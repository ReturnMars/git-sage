// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
	"github.com/tmc/langchaingo/llms"
)

// LangChainWrapper wraps LangChain LLM calls with retry logic and error handling.
type LangChainWrapper struct {
	llm            llms.Model
	promptTemplate *PromptTemplate
	config         ProviderConfig
	providerName   string
}

// NewLangChainWrapper creates a new LangChain wrapper.
func NewLangChainWrapper(llm llms.Model, config ProviderConfig, providerName string) *LangChainWrapper {
	return &LangChainWrapper{
		llm:            llm,
		promptTemplate: NewPromptTemplate(),
		config:         config,
		providerName:   providerName,
	}
}

// SetPromptTemplate sets a custom prompt template.
func (w *LangChainWrapper) SetPromptTemplate(pt *PromptTemplate) {
	if pt != nil {
		w.promptTemplate = pt
	}
}

// generate performs a single LLM call without retry logic.
func (w *LangChainWrapper) generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	// Allow empty DiffChunks if CustomPrompt is provided
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
	userPrompt, err := w.promptTemplate.RenderUserPrompt(promptData)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	// Build LangChain message content
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, w.promptTemplate.GetSystemPrompt()),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	// Log API request
	apperrors.LogAPIRequest(w.providerName, w.config.Endpoint, w.config.Model, len(userPrompt))
	startTime := time.Now()

	// Call LangChain LLM
	resp, err := w.llm.GenerateContent(ctx, messages,
		llms.WithTemperature(float64(w.config.Temperature)),
		llms.WithMaxTokens(w.config.MaxTokens),
	)
	if err != nil {
		return nil, err
	}

	// Log API response
	duration := time.Since(startTime)
	responseLen := 0
	rawText := ""
	if len(resp.Choices) > 0 {
		rawText = resp.Choices[0].Content
		responseLen = len(rawText)
	}
	apperrors.LogAPIResponse(w.providerName, 200, responseLen, duration)

	// Check for empty response
	if rawText == "" {
		return nil, errors.New("no response from AI provider")
	}

	// Parse the response into structured format
	parsed := ParseCommitMessage(rawText)

	return parsed.ToGenerateResponse(rawText), nil
}


// GenerateWithRetry performs LLM call with retry logic and error handling.
func (w *LangChainWrapper) GenerateWithRetry(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		resp, err := w.generate(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if !w.isRetryableError(err) {
			return nil, w.wrapError(err)
		}

		// Calculate backoff delay using existing function
		delay := calculateBackoff(attempt)

		// Log retry attempt
		apperrors.LogRetry(attempt+1, MaxRetries, err, delay)

		// Wait before retry (respect context cancellation)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	return nil, w.wrapError(lastErr)
}

// isRetryableError checks if an error is retryable.
func (w *LangChainWrapper) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Retry on rate limit (429) and server errors (5xx)
	if strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") {
		return true
	}

	// Check for context deadline exceeded (timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// wrapError wraps an error with a user-friendly message.
func (w *LangChainWrapper) wrapError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Authentication error
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "Unauthorized") {
		return apperrors.NewAuthenticationError(w.providerName)
	}

	// Rate limit error
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "too many requests") {
		return apperrors.NewRateLimitError(60 * time.Second)
	}

	// Timeout error
	if errors.Is(err, context.DeadlineExceeded) {
		return apperrors.NewTimeoutError(err)
	}

	// Connection error (Ollama specific)
	if strings.Contains(errStr, "connection refused") {
		appErr := apperrors.NewNetworkError(err)
		appErr.Message = fmt.Sprintf("cannot connect to %s", w.providerName)
		if w.providerName == "ollama" {
			appErr.WithSuggestion("Please ensure Ollama is running using 'ollama serve'")
		}
		return appErr
	}

	return apperrors.NewAIProviderError(w.providerName, err)
}
