package ai

import (
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/config"
)

func TestNewProvider_OpenAI(t *testing.T) {
	cfg := &config.ProviderConfig{
		Name:   "openai",
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if provider.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "openai")
	}
}

func TestNewProvider_DefaultToOpenAI(t *testing.T) {
	cfg := &config.ProviderConfig{
		Name:   "", // Empty should default to OpenAI
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if provider.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "openai")
	}
}

func TestNewProvider_DeepSeek(t *testing.T) {
	cfg := &config.ProviderConfig{
		Name:   "deepseek",
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	// DeepSeek has its own provider that returns "deepseek" as name
	if provider.Name() != "deepseek" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "deepseek")
	}

	// Check that DeepSeek-specific defaults are applied
	deepseekProvider, ok := provider.(*DeepSeekProvider)
	if !ok {
		t.Fatal("Expected DeepSeekProvider type")
	}

	providerConfig := deepseekProvider.GetConfig()
	if providerConfig.Endpoint != "https://api.deepseek.com/v1" {
		t.Errorf("Endpoint = %q, want %q", providerConfig.Endpoint, "https://api.deepseek.com/v1")
	}
	if providerConfig.Model != "deepseek-chat" {
		t.Errorf("Model = %q, want %q", providerConfig.Model, "deepseek-chat")
	}
}

func TestNewProvider_Ollama(t *testing.T) {
	cfg := &config.ProviderConfig{
		Name: "ollama",
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if provider.Name() != "ollama" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "ollama")
	}

	// Check that Ollama-specific defaults are applied
	ollamaProvider, ok := provider.(*OllamaProvider)
	if !ok {
		t.Fatal("Expected OllamaProvider type")
	}

	providerConfig := ollamaProvider.GetConfig()
	if providerConfig.Endpoint != DefaultOllamaEndpoint {
		t.Errorf("Endpoint = %q, want %q", providerConfig.Endpoint, DefaultOllamaEndpoint)
	}
	if providerConfig.Model != DefaultOllamaModel {
		t.Errorf("Model = %q, want %q", providerConfig.Model, DefaultOllamaModel)
	}
}

func TestNewProvider_UnknownProvider(t *testing.T) {
	cfg := &config.ProviderConfig{
		Name: "unknown",
	}

	_, err := NewProvider(cfg)
	if err == nil {
		t.Error("NewProvider() should return error for unknown provider")
	}
}

func TestNewProvider_NilConfig(t *testing.T) {
	_, err := NewProvider(nil)
	if err == nil {
		t.Error("NewProvider() should return error for nil config")
	}
}

func TestNewProviderWithCustomPrompt(t *testing.T) {
	cfg := &config.ProviderConfig{
		Name:   "openai",
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	customSystem := "Custom system prompt"
	customUser := "Custom user prompt"

	provider, err := NewProviderWithCustomPrompt(cfg, customSystem, customUser)
	if err != nil {
		t.Fatalf("NewProviderWithCustomPrompt() error = %v", err)
	}

	// Verify provider was created successfully
	if provider == nil {
		t.Fatal("NewProviderWithCustomPrompt() returned nil")
	}

	if provider.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "openai")
	}
}
