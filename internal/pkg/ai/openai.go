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
	// DefaultOpenAIModel is the default model for OpenAI.
	DefaultOpenAIModel = "gpt-4o-mini"

	// DefaultTemperature is the default temperature for AI generation.
	DefaultTemperature = 0.2

	// DefaultMaxTokens is the default max tokens for AI generation.
	DefaultMaxTokens = 500

	// DefaultTimeout is the default timeout for API calls.
	DefaultTimeout = 30 * time.Second

	// MaxRetries is the maximum number of retries for API calls.
	MaxRetries = 3

	// InitialRetryDelay is the initial delay for exponential backoff.
	InitialRetryDelay = 1 * time.Second

	// MaxRetryDelay is the maximum delay for exponential backoff.
	MaxRetryDelay = 10 * time.Second
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	client         *openai.Client
	config         ProviderConfig
	promptTemplate *PromptTemplate
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(config ProviderConfig) (*OpenAIProvider, error) {
	if err := validateOpenAIConfig(config); err != nil {
		return nil, err
	}

	// Set defaults
	if config.Model == "" {
		config.Model = DefaultOpenAIModel
	}
	if config.Temperature == 0 {
		config.Temperature = DefaultTemperature
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = DefaultMaxTokens
	}

	// Create OpenAI client configuration
	clientConfig := openai.DefaultConfig(config.APIKey)

	// Support custom endpoints (for OpenAI-compatible APIs)
	if config.Endpoint != "" {
		clientConfig.BaseURL = config.Endpoint
	}

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

	return &OpenAIProvider{
		client:         client,
		config:         config,
		promptTemplate: NewPromptTemplate(),
	}, nil
}

// validateOpenAIConfig validates the OpenAI provider configuration.
func validateOpenAIConfig(config ProviderConfig) error {
	if config.APIKey == "" {
		return errors.New("API key is required for OpenAI provider")
	}

	// Basic API key format validation
	// OpenAI keys typically start with "sk-"
	if len(config.APIKey) < 20 {
		return errors.New("API key appears to be invalid (too short)")
	}

	return nil
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// ValidateConfig validates the provider configuration.
func (p *OpenAIProvider) ValidateConfig(config ProviderConfig) error {
	return validateOpenAIConfig(config)
}

// GenerateCommitMessage generates a commit message using OpenAI.
func (p *OpenAIProvider) GenerateCommitMessage(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
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
	apperrors.LogAPIRequest("openai", p.config.Endpoint, p.config.Model, len(userPrompt))
	startTime := time.Now()

	// Call OpenAI API with retry logic
	var resp openai.ChatCompletionResponse
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		resp, lastErr = p.client.CreateChatCompletion(ctx, chatReq)
		if lastErr == nil {
			break
		}

		// Check if error is retryable
		if !isRetryableError(lastErr) {
			return nil, wrapAPIError(lastErr)
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
		return nil, wrapAPIError(lastErr)
	}

	// Log API response
	duration := time.Since(startTime)
	responseLen := 0
	if len(resp.Choices) > 0 {
		responseLen = len(resp.Choices[0].Message.Content)
	}
	apperrors.LogAPIResponse("openai", 200, responseLen, duration)

	// Extract response content
	if len(resp.Choices) == 0 {
		return nil, errors.New("no response from AI provider")
	}

	rawText := resp.Choices[0].Message.Content

	// Parse the response into structured format
	parsed := ParseCommitMessage(rawText)

	return parsed.ToGenerateResponse(rawText), nil
}

// isRetryableError checks if an error is retryable.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for OpenAI API errors
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

// calculateBackoff calculates the backoff delay for a retry attempt.
func calculateBackoff(attempt int) time.Duration {
	delay := InitialRetryDelay * time.Duration(1<<uint(attempt))
	if delay > MaxRetryDelay {
		delay = MaxRetryDelay
	}
	return delay
}

// wrapAPIError wraps an API error with a user-friendly message.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.HTTPStatusCode {
		case http.StatusUnauthorized:
			return apperrors.NewAuthenticationError("OpenAI")
		case http.StatusTooManyRequests:
			// Try to parse Retry-After from the error message or use default
			retryAfter := 60 * time.Second // Default to 60 seconds
			return apperrors.NewRateLimitError(retryAfter)
		case http.StatusBadRequest:
			return apperrors.Wrap(err, apperrors.ErrAIProviderFailed, fmt.Sprintf("invalid request: %s", apiErr.Message))
		default:
			return apperrors.Wrap(err, apperrors.ErrAIProviderFailed, fmt.Sprintf("API error (status %d): %s", apiErr.HTTPStatusCode, apiErr.Message))
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return apperrors.NewTimeoutError(err)
	}

	return apperrors.NewAIProviderError("OpenAI", err)
}

// SetPromptTemplate sets a custom prompt template.
func (p *OpenAIProvider) SetPromptTemplate(pt *PromptTemplate) {
	if pt != nil {
		p.promptTemplate = pt
	}
}
