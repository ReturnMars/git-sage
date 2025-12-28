package ai

import (
	"context"
	"testing"
)

func TestNewDeepSeekProvider_ValidConfig(t *testing.T) {
	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewDeepSeekProvider(config)
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("NewDeepSeekProvider() returned nil")
	}

	if provider.Name() != "deepseek" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "deepseek")
	}
}

func TestNewDeepSeekProvider_MissingAPIKey(t *testing.T) {
	config := ProviderConfig{}

	_, err := NewDeepSeekProvider(config)
	if err == nil {
		t.Error("NewDeepSeekProvider() should return error for missing API key")
	}
}

func TestNewDeepSeekProvider_ShortAPIKey(t *testing.T) {
	config := ProviderConfig{
		APIKey: "short",
	}

	_, err := NewDeepSeekProvider(config)
	if err == nil {
		t.Error("NewDeepSeekProvider() should return error for short API key")
	}
}

func TestNewDeepSeekProvider_DefaultValues(t *testing.T) {
	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewDeepSeekProvider(config)
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}

	providerConfig := provider.GetConfig()

	// Check default model
	if providerConfig.Model != DefaultDeepSeekModel {
		t.Errorf("Model = %q, want %q", providerConfig.Model, DefaultDeepSeekModel)
	}

	// Check default endpoint
	if providerConfig.Endpoint != DefaultDeepSeekEndpoint {
		t.Errorf("Endpoint = %q, want %q", providerConfig.Endpoint, DefaultDeepSeekEndpoint)
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

func TestNewDeepSeekProvider_CustomValues(t *testing.T) {
	config := ProviderConfig{
		APIKey:      "sk-test-key-that-is-long-enough-for-validation",
		Model:       "deepseek-coder",
		Endpoint:    "https://custom.deepseek.com/v1",
		Temperature: 0.5,
		MaxTokens:   1000,
	}

	provider, err := NewDeepSeekProvider(config)
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}

	providerConfig := provider.GetConfig()

	if providerConfig.Model != "deepseek-coder" {
		t.Errorf("Model = %q, want %q", providerConfig.Model, "deepseek-coder")
	}

	if providerConfig.Endpoint != "https://custom.deepseek.com/v1" {
		t.Errorf("Endpoint = %q, want %q", providerConfig.Endpoint, "https://custom.deepseek.com/v1")
	}

	if providerConfig.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want %v", providerConfig.Temperature, 0.5)
	}

	if providerConfig.MaxTokens != 1000 {
		t.Errorf("MaxTokens = %d, want %d", providerConfig.MaxTokens, 1000)
	}
}

func TestDeepSeekProvider_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  ProviderConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ProviderConfig{
				APIKey: "sk-test-key-that-is-long-enough-for-validation",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: ProviderConfig{
				APIKey: "",
			},
			wantErr: true,
		},
		{
			name: "short API key",
			config: ProviderConfig{
				APIKey: "short",
			},
			wantErr: true,
		},
	}

	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}
	provider, _ := NewDeepSeekProvider(config)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeepSeekProvider_SetPromptTemplate(t *testing.T) {
	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewDeepSeekProvider(config)
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}

	customTemplate := NewLangChainPromptTemplateWithCustom("custom system", "custom user")
	provider.SetPromptTemplate(customTemplate)

	// Verify template was set without error (internal state not accessible)
}

func TestDeepSeekProvider_SetPromptTemplate_Nil(t *testing.T) {
	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewDeepSeekProvider(config)
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}

	// Should not panic when nil is passed
	provider.SetPromptTemplate(nil)
}

func TestDeepSeekProvider_GetConfig(t *testing.T) {
	config := ProviderConfig{
		APIKey:      "sk-test-key-that-is-long-enough-for-validation",
		Model:       "deepseek-coder",
		Temperature: 0.3,
	}

	provider, err := NewDeepSeekProvider(config)
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}

	gotConfig := provider.GetConfig()

	if gotConfig.APIKey != config.APIKey {
		t.Errorf("GetConfig().APIKey = %q, want %q", gotConfig.APIKey, config.APIKey)
	}

	if gotConfig.Model != config.Model {
		t.Errorf("GetConfig().Model = %q, want %q", gotConfig.Model, config.Model)
	}

	if gotConfig.Temperature != config.Temperature {
		t.Errorf("GetConfig().Temperature = %v, want %v", gotConfig.Temperature, config.Temperature)
	}
}

func TestDeepSeekProvider_GenerateCommitMessage_NilRequest(t *testing.T) {
	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewDeepSeekProvider(config)
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}

	_, err = provider.GenerateCommitMessage(context.TODO(), nil)
	if err == nil {
		t.Error("GenerateCommitMessage() should return error for nil request")
	}
}

func TestDeepSeekProvider_GenerateCommitMessage_EmptyDiffChunks(t *testing.T) {
	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewDeepSeekProvider(config)
	if err != nil {
		t.Fatalf("NewDeepSeekProvider() error = %v", err)
	}

	req := &GenerateRequest{
		DiffChunks: nil,
	}

	_, err = provider.GenerateCommitMessage(context.TODO(), req)
	if err == nil {
		t.Error("GenerateCommitMessage() should return error for empty diff chunks")
	}
}
