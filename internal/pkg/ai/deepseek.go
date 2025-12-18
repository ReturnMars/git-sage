// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
	"github.com/sashabaranov/go-openai"
)

const (
	// DefaultDeepSeekModel is the default model for DeepSeek.
	DefaultDeepSeekModel = "deepseek-chat"

	// DefaultDeepSeekEndpoint is the default API endpoint for DeepSeek.
	DefaultDeepSeekEndpoint = "https://api.deepseek.com/v1"
)

// DeepSeekProvider implements the Provider interface for DeepSeek.
// DeepSeek uses an OpenAI-compatible API, so we leverage the go-openai client.
type DeepSeekProvider struct {
	client         *openai.Client
	config         ProviderConfig
	promptTemplate *PromptTemplate
}

// NewDeepSeekProvider creates a new DeepSeek provider.
func NewDeepSeekProvider(config ProviderConfig) (*DeepSeekProvider, error) {
	if err := validateDeepSeekConfig(config); err != nil {
		return nil, err
	}

	// Set DeepSeek-specific defaults
	if config.Model == "" {
		config.Model = DefaultDeepSeekModel
	}
	if config.Endpoint == "" {
		config.Endpoint = DefaultDeepSeekEndpoint
	}
	if config.Temperature == 0 {
		config.Temperature = DefaultTemperature
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = DefaultMaxTokens
	}

	// Create OpenAI-compatible client configuration with DeepSeek endpoint
	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = config.Endpoint

	// Create HTTP client with timeout and connection pooling
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	}
	clientConfig.HTTPClient = &http.Client{
		Timeout:   DefaultTimeout,
		Transport: transport,
	}

	client := openai.NewClientWithConfig(clientConfig)

	return &DeepSeekProvider{
		client:         client,
		config:         config,
		promptTemplate: NewPromptTemplate(),
	}, nil
}

// validateDeepSeekConfig validates the DeepSeek provider configuration.
func validateDeepSeekConfig(config ProviderConfig) error {
	if config.APIKey == "" {
		return errors.New("API key is required for DeepSeek provider")
	}

	// DeepSeek API keys are typically longer than 20 characters
	if len(config.APIKey) < 20 {
		return errors.New("API key appears to be invalid (too short)")
	}

	return nil
}

// Name returns the provider name.
func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

// ValidateConfig validates the provider configuration.
func (p *DeepSeekProvider) ValidateConfig(config ProviderConfig) error {
	return validateDeepSeekConfig(config)
}

// GenerateCommitMessage generates a commit message using DeepSeek.
func (p *DeepSeekProvider) GenerateCommitMessage(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	// Allow empty DiffChunks if CustomPrompt is provided (for summary-based generation)
	if len(req.DiffChunks) == 0 && req.CustomPrompt == "" {
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

	// Create chat completion request
	chatReq := openai.ChatCompletionRequest{
		Model: p.config.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: p.promptTemplate.GetSystemPrompt(),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		Temperature: p.config.Temperature,
		MaxTokens:   p.config.MaxTokens,
	}

	// Log API request in verbose mode
	apperrors.LogAPIRequest("deepseek", p.config.Endpoint, p.config.Model, len(userPrompt))
	startTime := time.Now()

	// Call DeepSeek API with retry logic
	var resp openai.ChatCompletionResponse
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		resp, lastErr = p.client.CreateChatCompletion(ctx, chatReq)
		if lastErr == nil {
			break
		}

		// Check if error is retryable
		if !isDeepSeekRetryableError(lastErr) {
			return nil, wrapDeepSeekAPIError(lastErr)
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
		return nil, wrapDeepSeekAPIError(lastErr)
	}

	// Log API response
	duration := time.Since(startTime)
	responseLen := 0
	if len(resp.Choices) > 0 {
		responseLen = len(resp.Choices[0].Message.Content)
	}
	apperrors.LogAPIResponse("deepseek", 200, responseLen, duration)

	// Extract response content
	if len(resp.Choices) == 0 {
		return nil, errors.New("no response from DeepSeek provider")
	}

	rawText := resp.Choices[0].Message.Content

	// Parse the response into structured format
	parsed := ParseCommitMessage(rawText)

	return parsed.ToGenerateResponse(rawText), nil
}

// isDeepSeekRetryableError checks if an error is retryable for DeepSeek.
func isDeepSeekRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for OpenAI-compatible API errors
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		// Retry on rate limit (429) and server errors (5xx)
		switch apiErr.HTTPStatusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
	}

	// Check for context deadline exceeded (timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// wrapDeepSeekAPIError wraps a DeepSeek API error with a user-friendly message.
func wrapDeepSeekAPIError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.HTTPStatusCode {
		case http.StatusUnauthorized:
			return apperrors.NewAuthenticationError("DeepSeek")
		case http.StatusTooManyRequests:
			retryAfter := 60 * time.Second // Default to 60 seconds
			return apperrors.NewRateLimitError(retryAfter)
		case http.StatusBadRequest:
			return apperrors.Wrap(err, apperrors.ErrAIProviderFailed, fmt.Sprintf("DeepSeek invalid request: %s", apiErr.Message))
		case http.StatusPaymentRequired:
			appErr := apperrors.Wrap(err, apperrors.ErrAIProviderFailed, "DeepSeek payment required")
			appErr.WithSuggestion("Please check your DeepSeek account balance")
			return appErr
		case http.StatusForbidden:
			appErr := apperrors.Wrap(err, apperrors.ErrAIProviderFailed, "DeepSeek access forbidden")
			appErr.WithSuggestion("Please check your API key permissions")
			return appErr
		default:
			return apperrors.Wrap(err, apperrors.ErrAIProviderFailed, fmt.Sprintf("DeepSeek API error (status %d): %s", apiErr.HTTPStatusCode, apiErr.Message))
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return apperrors.NewTimeoutError(err)
	}

	return apperrors.NewAIProviderError("DeepSeek", err)
}

// SetPromptTemplate sets a custom prompt template.
func (p *DeepSeekProvider) SetPromptTemplate(pt *PromptTemplate) {
	if pt != nil {
		p.promptTemplate = pt
	}
}

// GetConfig returns the provider configuration (useful for testing).
func (p *DeepSeekProvider) GetConfig() ProviderConfig {
	return p.config
}
