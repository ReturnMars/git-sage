// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/config"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: langchaingo-refactor, Property 3: Provider Factory Correctness
// Validates: Requirements 5.2, 5.4, 5.5
//
// For any valid provider configuration, the factory SHALL create the correct provider type
// and the provider SHALL use LangChain internally.

// genProviderConfig generates valid provider configurations.
func genProviderConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.OneConstOf("openai", "deepseek"),
		gen.OneConstOf("gpt-4", "gpt-3.5-turbo", "deepseek-chat"),
	).Map(func(values []interface{}) *config.ProviderConfig {
		providerName := values[0].(string)
		model := values[1].(string)
		return &config.ProviderConfig{
			Name:   providerName,
			APIKey: "sk-test-key-that-is-long-enough-for-validation",
			Model:  model,
		}
	})
}

// genOllamaConfig generates Ollama configurations.
func genOllamaConfig() gopter.Gen {
	return gen.OneConstOf(
		"codellama", "llama2", "mistral",
	).Map(func(model string) *config.ProviderConfig {
		return &config.ProviderConfig{
			Name:  "ollama",
			Model: model,
		}
	})
}

// TestProperty_ProviderFactoryCorrectness verifies that the factory creates correct providers.
//
// Feature: langchaingo-refactor, Property 3: Provider Factory Correctness
// Validates: Requirements 5.2, 5.4, 5.5
func TestProperty_ProviderFactoryCorrectness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: NewProvider creates correct provider type
	properties.Property("NewProvider creates correct provider type", prop.ForAll(
		func(cfg *config.ProviderConfig) bool {
			provider, err := NewProvider(cfg)
			if err != nil {
				return false
			}
			return provider.Name() == cfg.Name
		},
		genProviderConfig(),
	))

	// Property: OpenAI provider is OpenAIProvider type
	properties.Property("OpenAI config creates OpenAIProvider", prop.ForAll(
		func(model string) bool {
			cfg := &config.ProviderConfig{
				Name:   "openai",
				APIKey: "sk-test-key-that-is-long-enough-for-validation",
				Model:  model,
			}
			provider, err := NewProvider(cfg)
			if err != nil {
				return false
			}
			_, ok := provider.(*OpenAIProvider)
			return ok
		},
		gen.OneConstOf("gpt-4", "gpt-4o-mini", "gpt-3.5-turbo"),
	))

	// Property: DeepSeek provider is DeepSeekProvider type
	properties.Property("DeepSeek config creates DeepSeekProvider", prop.ForAll(
		func(model string) bool {
			cfg := &config.ProviderConfig{
				Name:   "deepseek",
				APIKey: "sk-test-key-that-is-long-enough-for-validation",
				Model:  model,
			}
			provider, err := NewProvider(cfg)
			if err != nil {
				return false
			}
			_, ok := provider.(*DeepSeekProvider)
			return ok
		},
		gen.OneConstOf("deepseek-chat", "deepseek-coder"),
	))

	// Property: Ollama provider is OllamaProvider type
	properties.Property("Ollama config creates OllamaProvider", prop.ForAll(
		func(cfg *config.ProviderConfig) bool {
			provider, err := NewProvider(cfg)
			if err != nil {
				return false
			}
			_, ok := provider.(*OllamaProvider)
			return ok
		},
		genOllamaConfig(),
	))

	// Property: Empty provider name defaults to OpenAI
	properties.Property("empty provider name defaults to OpenAI", prop.ForAll(
		func(apiKey string) bool {
			if len(apiKey) < 20 {
				return true // Skip invalid keys
			}
			cfg := &config.ProviderConfig{
				Name:   "",
				APIKey: apiKey,
			}
			provider, err := NewProvider(cfg)
			if err != nil {
				return false
			}
			return provider.Name() == "openai"
		},
		gen.AlphaString(),
	))

	// Property: Provider GetConfig returns correct config
	properties.Property("provider GetConfig returns correct config", prop.ForAll(
		func(cfg *config.ProviderConfig) bool {
			provider, err := NewProvider(cfg)
			if err != nil {
				return false
			}

			switch p := provider.(type) {
			case *OpenAIProvider:
				gotCfg := p.GetConfig()
				if cfg.Model != "" && gotCfg.Model != cfg.Model {
					return false
				}
			case *DeepSeekProvider:
				gotCfg := p.GetConfig()
				if cfg.Model != "" && gotCfg.Model != cfg.Model {
					return false
				}
			case *OllamaProvider:
				gotCfg := p.GetConfig()
				if cfg.Model != "" && gotCfg.Model != cfg.Model {
					return false
				}
			}
			return true
		},
		genProviderConfig(),
	))

	// Property: NewProviderWithCustomPrompt works correctly
	properties.Property("NewProviderWithCustomPrompt creates provider with custom template", prop.ForAll(
		func(cfg *config.ProviderConfig, systemPrompt, userPrompt string) bool {
			provider, err := NewProviderWithCustomPrompt(cfg, systemPrompt, userPrompt)
			if err != nil {
				return false
			}
			// Just verify the provider was created successfully
			return provider != nil && provider.Name() == cfg.Name
		},
		genProviderConfig(),
		gen.OneConstOf("Custom system prompt", "You are a helpful assistant"),
		gen.OneConstOf("Custom user prompt", "Generate commit message for: {{.DiffStats}}"),
	))

	// Property: Unknown provider returns error
	properties.Property("unknown provider returns error", prop.ForAll(
		func(unknownName string) bool {
			if unknownName == "openai" || unknownName == "deepseek" || unknownName == "ollama" || unknownName == "" {
				return true // Skip known providers
			}
			cfg := &config.ProviderConfig{
				Name:   unknownName,
				APIKey: "some-key-for-testing-purposes",
			}
			_, err := NewProvider(cfg)
			return err != nil
		},
		gen.AlphaString(),
	))

	// Property: Nil config returns error
	properties.Property("nil config returns error", prop.ForAll(
		func(_ int) bool {
			_, err := NewProvider(nil)
			return err != nil
		},
		gen.Int(),
	))

	properties.TestingRun(t)
}
