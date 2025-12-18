// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
)

const (
	// DefaultOllamaModel is the default model for Ollama.
	DefaultOllamaModel = "codellama"

	// DefaultOllamaEndpoint is the default API endpoint for Ollama.
	DefaultOllamaEndpoint = "http://localhost:11434"

	// OllamaAPIPath is the API path for chat completions.
	OllamaAPIPath = "/api/chat"
)

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	httpClient     *http.Client
	config         ProviderConfig
	promptTemplate *PromptTemplate
}

// OllamaChatRequest represents a request to the Ollama chat API.
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *OllamaOptions  `json:"options,omitempty"`
}

// OllamaMessage represents a message in the Ollama chat API.
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaOptions represents optional parameters for Ollama requests.
type OllamaOptions struct {
	Temperature float32 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// OllamaChatResponse represents a response from the Ollama chat API.
type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
	Error     string        `json:"error,omitempty"`
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(config ProviderConfig) (*OllamaProvider, error) {
	if err := validateOllamaConfig(config); err != nil {
		return nil, err
	}

	// Set Ollama-specific defaults
	if config.Model == "" {
		config.Model = DefaultOllamaModel
	}
	if config.Endpoint == "" {
		config.Endpoint = DefaultOllamaEndpoint
	}
	if config.Temperature == 0 {
		config.Temperature = DefaultTemperature
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = DefaultMaxTokens
	}

	// Create HTTP client with timeout and connection pooling
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	}
	httpClient := &http.Client{
		Timeout:   DefaultTimeout,
		Transport: transport,
	}

	return &OllamaProvider{
		httpClient:     httpClient,
		config:         config,
		promptTemplate: NewPromptTemplate(),
	}, nil
}

// validateOllamaConfig validates the Ollama provider configuration.
func validateOllamaConfig(config ProviderConfig) error {
	// Ollama doesn't require an API key (it's local)
	// Just validate that endpoint is reasonable if provided
	if config.Endpoint != "" {
		// Basic validation - endpoint should start with http:// or https://
		if len(config.Endpoint) >= 8 && config.Endpoint[:8] == "https://" {
			return nil
		}
		if len(config.Endpoint) >= 7 && config.Endpoint[:7] == "http://" {
			return nil
		}
		return errors.New("endpoint must start with http:// or https://")
	}

	return nil
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// ValidateConfig validates the provider configuration.
func (p *OllamaProvider) ValidateConfig(config ProviderConfig) error {
	return validateOllamaConfig(config)
}

// GenerateCommitMessage generates a commit message using Ollama.
func (p *OllamaProvider) GenerateCommitMessage(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if len(req.DiffChunks) == 0 {
		return nil, errors.New("no diff chunks provided")
	}

	// Determine if chunking is required based on total diff size
	totalSize := 0
	for _, chunk := range req.DiffChunks {
		totalSize += len(chunk.Content)
	}
	requiresChunking := totalSize > 10*1024 // 10KB threshold

	// Build prompt data
	promptData := BuildPromptData(req, requiresChunking)

	// Render user prompt
	userPrompt, err := p.promptTemplate.RenderUserPrompt(promptData)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	// Create Ollama chat request
	chatReq := OllamaChatRequest{
		Model: p.config.Model,
		Messages: []OllamaMessage{
			{
				Role:    "system",
				Content: p.promptTemplate.GetSystemPrompt(),
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Stream: false, // We don't need streaming for commit messages
		Options: &OllamaOptions{
			Temperature: p.config.Temperature,
			NumPredict:  p.config.MaxTokens,
		},
	}

	// Log API request in verbose mode
	apperrors.LogAPIRequest("ollama", p.config.Endpoint, p.config.Model, len(userPrompt))
	startTime := time.Now()

	// Call Ollama API with retry logic
	var resp *OllamaChatResponse
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		resp, lastErr = p.doRequest(ctx, chatReq)
		if lastErr == nil {
			break
		}

		// Check if error is retryable
		if !isOllamaRetryableError(lastErr) {
			return nil, wrapOllamaAPIError(lastErr)
		}

		// Calculate backoff delay
		delay := calculateBackoff(attempt)

		// Log retry attempt
		apperrors.LogRetry(attempt+1, MaxRetries, lastErr, delay)

		// Wait before retry (respect context cancellation)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	if lastErr != nil {
		return nil, wrapOllamaAPIError(lastErr)
	}

	// Log API response
	duration := time.Since(startTime)
	apperrors.LogAPIResponse("ollama", 200, len(resp.Message.Content), duration)

	// Check for error in response
	if resp.Error != "" {
		return nil, fmt.Errorf("ollama error: %s", resp.Error)
	}

	rawText := resp.Message.Content

	// Parse the response into structured format
	parsed := ParseCommitMessage(rawText)

	return parsed.ToGenerateResponse(rawText), nil
}

// doRequest performs the HTTP request to Ollama API.
func (p *OllamaProvider) doRequest(ctx context.Context, chatReq OllamaChatRequest) (*OllamaChatResponse, error) {
	// Marshal request body
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	url := p.config.Endpoint + OllamaAPIPath

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status code
	if httpResp.StatusCode != http.StatusOK {
		return nil, &OllamaAPIError{
			StatusCode: httpResp.StatusCode,
			Message:    string(respBody),
		}
	}

	// Parse response
	var resp OllamaChatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// OllamaAPIError represents an error from the Ollama API.
type OllamaAPIError struct {
	StatusCode int
	Message    string
}

func (e *OllamaAPIError) Error() string {
	return fmt.Sprintf("ollama API error (status %d): %s", e.StatusCode, e.Message)
}

// isOllamaRetryableError checks if an error is retryable for Ollama.
func isOllamaRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for Ollama API errors
	var apiErr *OllamaAPIError
	if errors.As(err, &apiErr) {
		// Retry on server errors (5xx)
		switch apiErr.StatusCode {
		case http.StatusInternalServerError, // 500
			http.StatusBadGateway,         // 502
			http.StatusServiceUnavailable, // 503
			http.StatusGatewayTimeout:     // 504
			return true
		}
	}

	// Check for context deadline exceeded (timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// wrapOllamaAPIError wraps an Ollama API error with a user-friendly message.
func wrapOllamaAPIError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *OllamaAPIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusNotFound:
			appErr := apperrors.Wrap(err, apperrors.ErrAIProviderFailed, "Ollama model not found")
			appErr.WithSuggestion("Please ensure the model is pulled using 'ollama pull <model>'")
			return appErr
		case http.StatusBadRequest:
			return apperrors.Wrap(err, apperrors.ErrAIProviderFailed, fmt.Sprintf("Ollama invalid request: %s", apiErr.Message))
		case http.StatusServiceUnavailable:
			appErr := apperrors.Wrap(err, apperrors.ErrAIProviderFailed, "Ollama service unavailable")
			appErr.WithSuggestion("Please ensure Ollama is running using 'ollama serve'")
			return appErr
		default:
			return apperrors.Wrap(err, apperrors.ErrAIProviderFailed, fmt.Sprintf("Ollama API error (status %d): %s", apiErr.StatusCode, apiErr.Message))
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		appErr := apperrors.NewTimeoutError(err)
		appErr.WithSuggestion("Please check if Ollama is running")
		return appErr
	}

	// Check for connection refused (Ollama not running)
	errStr := err.Error()
	if contains(errStr, "connection refused") || contains(errStr, "connect: connection refused") {
		appErr := apperrors.NewNetworkError(err)
		appErr.Message = "cannot connect to Ollama"
		appErr.WithSuggestion("Please ensure Ollama is running using 'ollama serve'")
		return appErr
	}

	return apperrors.NewAIProviderError("Ollama", err)
}

// contains checks if a string contains a substring (case-insensitive would be better but keeping it simple).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// SetPromptTemplate sets a custom prompt template.
func (p *OllamaProvider) SetPromptTemplate(pt *PromptTemplate) {
	if pt != nil {
		p.promptTemplate = pt
	}
}

// GetConfig returns the provider configuration (useful for testing).
func (p *OllamaProvider) GetConfig() ProviderConfig {
	return p.config
}
