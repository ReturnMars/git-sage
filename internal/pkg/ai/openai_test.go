package ai

import (
	"testing"
)

func TestNewOpenAIProvider_ValidConfig(t *testing.T) {
	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewOpenAIProvider(config)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("NewOpenAIProvider() returned nil")
	}

	if provider.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "openai")
	}
}

func TestNewOpenAIProvider_MissingAPIKey(t *testing.T) {
	config := ProviderConfig{}

	_, err := NewOpenAIProvider(config)
	if err == nil {
		t.Error("NewOpenAIProvider() should return error for missing API key")
	}
}

func TestNewOpenAIProvider_ShortAPIKey(t *testing.T) {
	config := ProviderConfig{
		APIKey: "short",
	}

	_, err := NewOpenAIProvider(config)
	if err == nil {
		t.Error("NewOpenAIProvider() should return error for short API key")
	}
}

func TestNewOpenAIProvider_DefaultValues(t *testing.T) {
	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewOpenAIProvider(config)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	// Check that defaults are applied
	if provider.config.Model != DefaultOpenAIModel {
		t.Errorf("Model = %q, want %q", provider.config.Model, DefaultOpenAIModel)
	}
	if provider.config.Temperature != DefaultTemperature {
		t.Errorf("Temperature = %v, want %v", provider.config.Temperature, DefaultTemperature)
	}
	if provider.config.MaxTokens != DefaultMaxTokens {
		t.Errorf("MaxTokens = %d, want %d", provider.config.MaxTokens, DefaultMaxTokens)
	}
}

func TestNewOpenAIProvider_CustomValues(t *testing.T) {
	config := ProviderConfig{
		APIKey:      "sk-test-key-that-is-long-enough-for-validation",
		Model:       "gpt-4",
		Temperature: 0.5,
		MaxTokens:   1000,
	}

	provider, err := NewOpenAIProvider(config)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	if provider.config.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", provider.config.Model, "gpt-4")
	}
	if provider.config.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want %v", provider.config.Temperature, 0.5)
	}
	if provider.config.MaxTokens != 1000 {
		t.Errorf("MaxTokens = %d, want %d", provider.config.MaxTokens, 1000)
	}
}

func TestNewOpenAIProvider_CustomEndpoint(t *testing.T) {
	config := ProviderConfig{
		APIKey:   "sk-test-key-that-is-long-enough-for-validation",
		Endpoint: "https://custom.api.endpoint/v1",
	}

	provider, err := NewOpenAIProvider(config)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	if provider.config.Endpoint != "https://custom.api.endpoint/v1" {
		t.Errorf("Endpoint = %q, want %q", provider.config.Endpoint, "https://custom.api.endpoint/v1")
	}
}

func TestOpenAIProvider_ValidateConfig(t *testing.T) {
	provider := &OpenAIProvider{}

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
			name:    "missing API key",
			config:  ProviderConfig{},
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenAIProvider_SetPromptTemplate(t *testing.T) {
	config := ProviderConfig{
		APIKey: "sk-test-key-that-is-long-enough-for-validation",
	}

	provider, err := NewOpenAIProvider(config)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	customPT := NewPromptTemplateWithCustom("custom system", "custom user")
	provider.SetPromptTemplate(customPT)

	if provider.promptTemplate.SystemPrompt != "custom system" {
		t.Errorf("SystemPrompt = %q, want %q", provider.promptTemplate.SystemPrompt, "custom system")
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		want    string // Use string comparison for duration
	}{
		{0, "1s"},
		{1, "2s"},
		{2, "4s"},
		{3, "8s"},
		{4, "10s"}, // Should be capped at MaxRetryDelay
		{5, "10s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := calculateBackoff(tt.attempt)
			if got.String() != tt.want {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}
