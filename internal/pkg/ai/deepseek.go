// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"context"
	"errors"

	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	// DefaultDeepSeekModel is the default model for DeepSeek.
	DefaultDeepSeekModel = "deepseek-chat"

	// DefaultDeepSeekEndpoint is the default API endpoint for DeepSeek.
	DefaultDeepSeekEndpoint = "https://api.deepseek.com/v1"
)

// DeepSeekProvider implements the Provider interface for DeepSeek using LangChain.
// DeepSeek uses an OpenAI-compatible API, so we leverage the langchaingo openai package.
type DeepSeekProvider struct {
	wrapper *LangChainWrapper
	config  ProviderConfig
}

// NewDeepSeekProvider creates a new DeepSeek provider using LangChain.
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

	// Create LangChain OpenAI LLM with DeepSeek endpoint
	// DeepSeek uses OpenAI-compatible API
	llm, err := openai.New(
		openai.WithToken(config.APIKey),
		openai.WithModel(config.Model),
		openai.WithBaseURL(config.Endpoint),
	)
	if err != nil {
		return nil, err
	}

	// Log provider creation
	apperrors.Debug("AI provider created: deepseek")

	// Create wrapper with the LLM
	wrapper := NewLangChainWrapper(llm, config, "deepseek")

	return &DeepSeekProvider{
		wrapper: wrapper,
		config:  config,
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

// GenerateCommitMessage generates a commit message using DeepSeek via LangChain.
func (p *DeepSeekProvider) GenerateCommitMessage(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	return p.wrapper.GenerateWithRetry(ctx, req)
}

// SetPromptTemplate sets a custom prompt template.
func (p *DeepSeekProvider) SetPromptTemplate(pt *LangChainPromptTemplate) {
	p.wrapper.SetPromptTemplate(pt)
}

// GetConfig returns the provider configuration (useful for testing).
func (p *DeepSeekProvider) GetConfig() ProviderConfig {
	return p.config
}
