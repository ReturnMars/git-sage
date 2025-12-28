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
	promptTemplate *LangChainPromptTemplate
	config         ProviderConfig
	providerName   string
}

// NewLangChainWrapper creates a new LangChain wrapper.
func NewLangChainWrapper(llm llms.Model, config ProviderConfig, providerName string) *LangChainWrapper {
	return &LangChainWrapper{
		llm:            llm,
		promptTemplate: NewLangChainPromptTemplate(),
		config:         config,
		providerName:   providerName,
	}
}

// SetPromptTemplate sets a custom prompt template.
func (w *LangChainWrapper) SetPromptTemplate(pt *LangChainPromptTemplate) {
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

	// Render messages using LangChain prompt template
	messages, err := w.promptTemplate.RenderMessages(promptData)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	// Get user prompt length for logging
	userPromptLen := 0
	if len(messages) > 1 {
		for _, part := range messages[1].Parts {
			if textPart, ok := part.(llms.TextContent); ok {
				userPromptLen = len(textPart.Text)
				break
			}
		}
	}

	// Log API request
	apperrors.LogAPIRequest(w.providerName, w.config.Endpoint, w.config.Model, userPromptLen)
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
// 优先使用 LangChain 提供的类型化错误检查，同时保留字符串检查作为后备方案。
func (w *LangChainWrapper) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 使用 LangChain 类型化错误检查（优先）
	// 速率限制错误 - 可重试
	if llms.IsRateLimitError(err) {
		return true
	}

	// 配额超限错误 - 可重试（可能是临时的）
	if llms.IsQuotaExceededError(err) {
		return true
	}

	// 服务不可用错误 - 可重试
	if llms.IsProviderUnavailableError(err) {
		return true
	}

	// 超时错误 - 可重试
	if llms.IsTimeoutError(err) {
		return true
	}

	// 请求被取消 - 不重试（用户主动取消）
	if llms.IsCanceledError(err) {
		return false
	}

	// 认证错误 - 不重试（需要用户干预）
	if llms.IsAuthenticationError(err) {
		return false
	}

	// 无效请求错误 - 不重试（请求本身有问题）
	if llms.IsInvalidRequestError(err) {
		return false
	}

	// 后备方案：字符串检查（用于 LangChain 未捕获的错误）
	errStr := err.Error()
	if strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") {
		return true
	}

	// Context deadline exceeded (timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// wrapError wraps an error with a user-friendly message.
// 优先使用 LangChain 类型化错误检查，提供更准确的错误分类。
func (w *LangChainWrapper) wrapError(err error) error {
	if err == nil {
		return nil
	}

	// 使用 LangChain 类型化错误检查（优先）
	// 认证错误
	if llms.IsAuthenticationError(err) {
		return apperrors.NewAuthenticationError(w.providerName)
	}

	// 速率限制错误
	if llms.IsRateLimitError(err) {
		return apperrors.NewRateLimitError(60 * time.Second)
	}

	// 配额超限错误
	if llms.IsQuotaExceededError(err) {
		appErr := apperrors.Wrap(err, apperrors.ErrAIProviderFailed, fmt.Sprintf("%s quota exceeded", w.providerName))
		appErr.WithSuggestion("Please check your API quota or billing status")
		return appErr
	}

	// 超时错误
	if llms.IsTimeoutError(err) {
		return apperrors.NewTimeoutError(err)
	}

	// 内容过滤错误
	if llms.IsContentFilterError(err) {
		appErr := apperrors.Wrap(err, apperrors.ErrAIProviderFailed, "content was filtered by safety policy")
		appErr.WithSuggestion("Please modify your input to comply with content policies")
		return appErr
	}

	// 无效请求错误
	if llms.IsInvalidRequestError(err) {
		return apperrors.Wrap(err, apperrors.ErrAIProviderFailed, fmt.Sprintf("%s invalid request", w.providerName))
	}

	// Token 限制错误
	if llms.IsTokenLimitError(err) {
		appErr := apperrors.Wrap(err, apperrors.ErrAIProviderFailed, "input or output exceeded token limit")
		appErr.WithSuggestion("Please reduce the size of your diff or use a model with larger context window")
		return appErr
	}

	// 服务不可用错误
	if llms.IsProviderUnavailableError(err) {
		appErr := apperrors.Wrap(err, apperrors.ErrAIProviderFailed, fmt.Sprintf("%s service is temporarily unavailable", w.providerName))
		appErr.WithSuggestion("Please try again later")
		return appErr
	}

	// 后备方案：字符串检查（用于 LangChain 未捕获的错误）
	errStr := err.Error()

	// 认证错误 - 后备检查
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "Unauthorized") {
		return apperrors.NewAuthenticationError(w.providerName)
	}

	// 速率限制错误 - 后备检查
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "too many requests") {
		return apperrors.NewRateLimitError(60 * time.Second)
	}

	// 超时错误 - 后备检查
	if errors.Is(err, context.DeadlineExceeded) {
		return apperrors.NewTimeoutError(err)
	}

	// 连接错误 (Ollama 特有)
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
