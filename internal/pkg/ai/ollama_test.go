package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/git"
)

func TestNewOllamaProvider_ValidConfig(t *testing.T) {
	config := ProviderConfig{}

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

	// Check default model
	if provider.config.Model != DefaultOllamaModel {
		t.Errorf("Model = %q, want %q", provider.config.Model, DefaultOllamaModel)
	}

	// Check default endpoint
	if provider.config.Endpoint != DefaultOllamaEndpoint {
		t.Errorf("Endpoint = %q, want %q", provider.config.Endpoint, DefaultOllamaEndpoint)
	}

	// Check default temperature
	if provider.config.Temperature != DefaultTemperature {
		t.Errorf("Temperature = %v, want %v", provider.config.Temperature, DefaultTemperature)
	}

	// Check default max tokens
	if provider.config.MaxTokens != DefaultMaxTokens {
		t.Errorf("MaxTokens = %d, want %d", provider.config.MaxTokens, DefaultMaxTokens)
	}
}

func TestNewOllamaProvider_CustomValues(t *testing.T) {
	config := ProviderConfig{
		Model:       "llama2",
		Endpoint:    "http://192.168.1.100:11434",
		Temperature: 0.5,
		MaxTokens:   1000,
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	if provider.config.Model != "llama2" {
		t.Errorf("Model = %q, want %q", provider.config.Model, "llama2")
	}

	if provider.config.Endpoint != "http://192.168.1.100:11434" {
		t.Errorf("Endpoint = %q, want %q", provider.config.Endpoint, "http://192.168.1.100:11434")
	}

	if provider.config.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want %v", provider.config.Temperature, 0.5)
	}

	if provider.config.MaxTokens != 1000 {
		t.Errorf("MaxTokens = %d, want %d", provider.config.MaxTokens, 1000)
	}
}

func TestNewOllamaProvider_InvalidEndpoint(t *testing.T) {
	config := ProviderConfig{
		Endpoint: "invalid",
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
			name:    "empty config (valid - uses defaults)",
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
			name: "invalid endpoint - no protocol",
			config: ProviderConfig{
				Endpoint: "localhost:11434",
			},
			wantErr: true,
		},
		{
			name: "invalid endpoint - too short",
			config: ProviderConfig{
				Endpoint: "http",
			},
			wantErr: true,
		},
	}

	provider := &OllamaProvider{}

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
	config := ProviderConfig{}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	customTemplate := NewPromptTemplateWithCustom("custom system", "custom user")
	provider.SetPromptTemplate(customTemplate)

	if provider.promptTemplate.SystemPrompt != "custom system" {
		t.Errorf("SystemPrompt = %q, want %q", provider.promptTemplate.SystemPrompt, "custom system")
	}

	if provider.promptTemplate.UserPrompt != "custom user" {
		t.Errorf("UserPrompt = %q, want %q", provider.promptTemplate.UserPrompt, "custom user")
	}
}

func TestOllamaProvider_SetPromptTemplate_Nil(t *testing.T) {
	config := ProviderConfig{}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	originalTemplate := provider.promptTemplate
	provider.SetPromptTemplate(nil)

	// Should not change when nil is passed
	if provider.promptTemplate != originalTemplate {
		t.Error("SetPromptTemplate(nil) should not change the template")
	}
}

func TestOllamaProvider_GetConfig(t *testing.T) {
	config := ProviderConfig{
		Model:       "llama2",
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

	if gotConfig.Temperature != config.Temperature {
		t.Errorf("GetConfig().Temperature = %v, want %v", gotConfig.Temperature, config.Temperature)
	}
}

func TestOllamaProvider_GenerateCommitMessage_NilRequest(t *testing.T) {
	config := ProviderConfig{}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	_, err = provider.GenerateCommitMessage(context.Background(), nil)
	if err == nil {
		t.Error("GenerateCommitMessage() should return error for nil request")
	}
}

func TestOllamaProvider_GenerateCommitMessage_EmptyDiffChunks(t *testing.T) {
	config := ProviderConfig{}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	req := &GenerateRequest{
		DiffChunks: nil,
	}

	_, err = provider.GenerateCommitMessage(context.Background(), req)
	if err == nil {
		t.Error("GenerateCommitMessage() should return error for empty diff chunks")
	}
}

func TestOllamaProvider_GenerateCommitMessage_Success(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != OllamaAPIPath {
			t.Errorf("Expected path %s, got %s", OllamaAPIPath, r.URL.Path)
		}

		// Parse request body
		var req OllamaChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Verify request structure
		if req.Model == "" {
			t.Error("Model should not be empty")
		}
		if len(req.Messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(req.Messages))
		}

		// Send response
		resp := OllamaChatResponse{
			Model: req.Model,
			Message: OllamaMessage{
				Role:    "assistant",
				Content: "feat(test): add new feature\n\nThis is the body of the commit message.",
			},
			Done: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		Endpoint: server.URL,
		Model:    "codellama",
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	req := &GenerateRequest{
		DiffChunks: []git.DiffChunk{
			{
				FilePath:   "test.go",
				ChangeType: git.ChangeTypeModified,
				Content:    "+// new comment",
			},
		},
		DiffStats: &git.DiffStats{},
	}

	resp, err := provider.GenerateCommitMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateCommitMessage() error = %v", err)
	}

	if resp == nil {
		t.Fatal("GenerateCommitMessage() returned nil response")
	}

	if resp.Subject == "" {
		t.Error("Subject should not be empty")
	}
}

func TestOllamaProvider_GenerateCommitMessage_ServerError(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	config := ProviderConfig{
		Endpoint: server.URL,
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	req := &GenerateRequest{
		DiffChunks: []git.DiffChunk{
			{
				FilePath: "test.go",
				Content:  "+// new comment",
			},
		},
	}

	_, err = provider.GenerateCommitMessage(context.Background(), req)
	if err == nil {
		t.Error("GenerateCommitMessage() should return error for server error")
	}
}

func TestOllamaProvider_GenerateCommitMessage_ModelNotFound(t *testing.T) {
	// Create a mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("model not found"))
	}))
	defer server.Close()

	config := ProviderConfig{
		Endpoint: server.URL,
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	req := &GenerateRequest{
		DiffChunks: []git.DiffChunk{
			{
				FilePath: "test.go",
				Content:  "+// new comment",
			},
		},
	}

	_, err = provider.GenerateCommitMessage(context.Background(), req)
	if err == nil {
		t.Error("GenerateCommitMessage() should return error for model not found")
	}
}

func TestOllamaAPIError_Error(t *testing.T) {
	err := &OllamaAPIError{
		StatusCode: 500,
		Message:    "internal error",
	}

	expected := "ollama API error (status 500): internal error"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestIsOllamaRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "500 error",
			err:      &OllamaAPIError{StatusCode: 500},
			expected: true,
		},
		{
			name:     "502 error",
			err:      &OllamaAPIError{StatusCode: 502},
			expected: true,
		},
		{
			name:     "503 error",
			err:      &OllamaAPIError{StatusCode: 503},
			expected: true,
		},
		{
			name:     "504 error",
			err:      &OllamaAPIError{StatusCode: 504},
			expected: true,
		},
		{
			name:     "400 error",
			err:      &OllamaAPIError{StatusCode: 400},
			expected: false,
		},
		{
			name:     "404 error",
			err:      &OllamaAPIError{StatusCode: 404},
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOllamaRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isOllamaRetryableError() = %v, want %v", result, tt.expected)
			}
		})
	}
}
