// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"fmt"

	"github.com/gitsage/gitsage/internal/pkg/config"
)

// ProviderName constants for supported providers.
const (
	ProviderNameOpenAI   = "openai"
	ProviderNameDeepSeek = "deepseek"
	ProviderNameOllama   = "ollama"
)

// NewProvider creates a new AI provider based on the configuration.
func NewProvider(cfg *config.ProviderConfig) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("provider configuration is required")
	}

	// Convert config.ProviderConfig to ai.ProviderConfig
	aiConfig := ProviderConfig{
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		Endpoint:    cfg.Endpoint,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	}

	switch cfg.Name {
	case ProviderNameOpenAI, "":
		// Default to OpenAI if no provider specified
		return NewOpenAIProvider(aiConfig)

	case ProviderNameDeepSeek:
		// DeepSeek uses OpenAI-compatible API with dedicated provider
		return NewDeepSeekProvider(aiConfig)

	case ProviderNameOllama:
		return NewOllamaProvider(aiConfig)

	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Name)
	}
}

// NewProviderWithCustomPrompt creates a new AI provider with a custom prompt template.
func NewProviderWithCustomPrompt(cfg *config.ProviderConfig, systemPrompt, userPrompt string) (Provider, error) {
	provider, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}

	// Set custom prompt template if provider supports it
	pt := NewPromptTemplateWithCustom(systemPrompt, userPrompt)

	switch p := provider.(type) {
	case *OpenAIProvider:
		p.SetPromptTemplate(pt)
	case *DeepSeekProvider:
		p.SetPromptTemplate(pt)
	case *OllamaProvider:
		p.SetPromptTemplate(pt)
	}

	return provider, nil
}
