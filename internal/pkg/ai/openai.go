// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"context"
	"errors"
	"time"

	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
	"github.com/tmc/langchaingo/llms/openai"
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

// OpenAIProvider implements the Provider interface for OpenAI using LangChain.
type OpenAIProvider struct {
	wrapper *LangChainWrapper
	config  ProviderConfig
}

// NewOpenAIProvider creates a new OpenAI provider using LangChain.
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

	// Build LangChain OpenAI options
	opts := []openai.Option{
		openai.WithToken(config.APIKey),
		openai.WithModel(config.Model),
	}

	// Support custom endpoints (for OpenAI-compatible APIs)
	if config.Endpoint != "" {
		opts = append(opts, openai.WithBaseURL(config.Endpoint))
	}

	// Create LangChain OpenAI LLM
	llm, err := openai.New(opts...)
	if err != nil {
		return nil, err
	}

	// Log provider creation
	apperrors.Debug("AI provider created: openai")

	// Create wrapper with the LLM
	wrapper := NewLangChainWrapper(llm, config, "openai")

	return &OpenAIProvider{
		wrapper: wrapper,
		config:  config,
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

// GenerateCommitMessage generates a commit message using OpenAI via LangChain.
func (p *OpenAIProvider) GenerateCommitMessage(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	return p.wrapper.GenerateWithRetry(ctx, req)
}

// SetPromptTemplate sets a custom prompt template.
func (p *OpenAIProvider) SetPromptTemplate(pt *LangChainPromptTemplate) {
	p.wrapper.SetPromptTemplate(pt)
}

// GetConfig returns the provider configuration (useful for testing).
func (p *OpenAIProvider) GetConfig() ProviderConfig {
	return p.config
}

// calculateBackoff calculates the backoff delay for a retry attempt.
// This is a shared utility function used by all providers.
func calculateBackoff(attempt int) time.Duration {
	delay := InitialRetryDelay * time.Duration(1<<uint(attempt))
	if delay > MaxRetryDelay {
		delay = MaxRetryDelay
	}
	return delay
}
