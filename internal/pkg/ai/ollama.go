// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"context"
	"errors"

	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
	"github.com/tmc/langchaingo/llms/ollama"
)

const (
	// DefaultOllamaModel is the default model for Ollama.
	DefaultOllamaModel = "codellama"

	// DefaultOllamaEndpoint is the default API endpoint for Ollama.
	DefaultOllamaEndpoint = "http://localhost:11434"
)

// OllamaProvider implements the Provider interface for Ollama using LangChain.
type OllamaProvider struct {
	wrapper *LangChainWrapper
	config  ProviderConfig
}

// NewOllamaProvider creates a new Ollama provider using LangChain.
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

	// Create LangChain Ollama LLM
	llm, err := ollama.New(
		ollama.WithModel(config.Model),
		ollama.WithServerURL(config.Endpoint),
	)
	if err != nil {
		return nil, err
	}

	// Log provider creation
	apperrors.Debug("AI provider created: ollama")

	// Create wrapper with the LLM
	wrapper := NewLangChainWrapper(llm, config, "ollama")

	return &OllamaProvider{
		wrapper: wrapper,
		config:  config,
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

// GenerateCommitMessage generates a commit message using Ollama via LangChain.
func (p *OllamaProvider) GenerateCommitMessage(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	return p.wrapper.GenerateWithRetry(ctx, req)
}

// SetPromptTemplate sets a custom prompt template.
func (p *OllamaProvider) SetPromptTemplate(pt *LangChainPromptTemplate) {
	p.wrapper.SetPromptTemplate(pt)
}

// GetConfig returns the provider configuration (useful for testing).
func (p *OllamaProvider) GetConfig() ProviderConfig {
	return p.config
}
