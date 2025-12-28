package ai

import (
	"context"
	"testing"
)

func TestNewOllamaProvider_ValidConfig(t *testing.T) {
	config := ProviderConfig{
		Model: "codellama",
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("NewOllamaProvider() returned nil")
	}

	if provider.Name() != "ollama" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "ollama")
	}
}

func TestNewOllamaProvider_DefaultValues(t *testing.T) {
	config := ProviderConfig{}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	providerConfig := provider.GetConfig()

	// Check default model
	if providerConfig.Model != DefaultOllamaModel {
		t.Errorf("Model = %q, want %q", providerConfig.Model, DefaultOllamaModel)
	}

	// Check default endpoint
	if providerConfig.Endpoint != DefaultOllamaEndpoint {
		t.Errorf("Endpoint = %q, want %q", providerConfig.Endpoint, DefaultOllamaEndpoint)
	}

	// Check default temperature
	if providerConfig.Temperature != DefaultTemperature {
		t.Errorf("Temperature = %v, want %v", providerConfig.Temperature, DefaultTemperature)
	}

	// Check default max tokens
	if providerConfig.MaxTokens != DefaultMaxTokens {
		t.Errorf("MaxTokens = %d, want %d", providerConfig.MaxTokens, DefaultMaxTokens)
	}
}

func TestNewOllamaProvider_CustomEndpoint(t *testing.T) {
	config := ProviderConfig{
		Endpoint: "http://custom-ollama:11434",
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	providerConfig := provider.GetConfig()
	if providerConfig.Endpoint != "http://custom-ollama:11434" {
		t.Errorf("Endpoint = %q, want %q", providerConfig.Endpoint, "http://custom-ollama:11434")
	}
}

func TestNewOllamaProvider_InvalidEndpoint(t *testing.T) {
	config := ProviderConfig{
		Endpoint: "invalid-endpoint",
	}

	_, err := NewOllamaProvider(config)
	if err == nil {
		t.Error("NewOllamaProvider() should return error for invalid endpoint")
	}
}

func TestOllamaProvider_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  ProviderConfig
		wantErr bool
	}{
		{
			name:    "valid empty config",
			config:  ProviderConfig{},
			wantErr: false,
		},
		{
			name: "valid http endpoint",
			config: ProviderConfig{
				Endpoint: "http://localhost:11434",
			},
			wantErr: false,
		},
		{
			name: "valid https endpoint",
			config: ProviderConfig{
				Endpoint: "https://ollama.example.com",
			},
			wantErr: false,
		},
		{
			name: "invalid endpoint",
			config: ProviderConfig{
				Endpoint: "invalid-endpoint",
			},
			wantErr: true,
		},
	}

	provider, _ := NewOllamaProvider(ProviderConfig{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOllamaProvider_SetPromptTemplate(t *testing.T) {
	provider, err := NewOllamaProvider(ProviderConfig{})
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	customTemplate := NewLangChainPromptTemplateWithCustom("custom system", "custom user")
	provider.SetPromptTemplate(customTemplate)

	// Verify template was set without error
}

func TestOllamaProvider_SetPromptTemplate_Nil(t *testing.T) {
	provider, err := NewOllamaProvider(ProviderConfig{})
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	// Should not panic when nil is passed
	provider.SetPromptTemplate(nil)
}

func TestOllamaProvider_GetConfig(t *testing.T) {
	config := ProviderConfig{
		Model:       "llama2",
		Endpoint:    "http://localhost:11434",
		Temperature: 0.3,
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	gotConfig := provider.GetConfig()

	if gotConfig.Model != config.Model {
		t.Errorf("GetConfig().Model = %q, want %q", gotConfig.Model, config.Model)
	}

	if gotConfig.Endpoint != config.Endpoint {
		t.Errorf("GetConfig().Endpoint = %q, want %q", gotConfig.Endpoint, config.Endpoint)
	}

	if gotConfig.Temperature != config.Temperature {
		t.Errorf("GetConfig().Temperature = %v, want %v", gotConfig.Temperature, config.Temperature)
	}
}

func TestOllamaProvider_GenerateCommitMessage_NilRequest(t *testing.T) {
	provider, err := NewOllamaProvider(ProviderConfig{})
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	_, err = provider.GenerateCommitMessage(context.TODO(), nil)
	if err == nil {
		t.Error("GenerateCommitMessage() should return error for nil request")
	}
}

func TestOllamaProvider_GenerateCommitMessage_EmptyDiffChunks(t *testing.T) {
	provider, err := NewOllamaProvider(ProviderConfig{})
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	req := &GenerateRequest{
		DiffChunks: nil,
	}

	_, err = provider.GenerateCommitMessage(context.TODO(), req)
	if err == nil {
		t.Error("GenerateCommitMessage() should return error for empty diff chunks")
	}
}
