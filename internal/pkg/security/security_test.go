package security

import (
	"testing"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "normal key",
			key:      "sk-1234567890abcdef1234567890abcdef",
			expected: "*******************************cdef",
		},
		{
			name:     "short key",
			key:      "abc",
			expected: "****",
		},
		{
			name:     "exactly 4 chars",
			key:      "abcd",
			expected: "****",
		},
		{
			name:     "5 chars",
			key:      "abcde",
			expected: "*bcde",
		},
		{
			name:     "empty key",
			key:      "",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKey(tt.key)
			if result != tt.expected {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestValidateAPIKeyFormat(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		apiKey   string
		wantErr  bool
	}{
		{
			name:     "valid openai key",
			provider: "openai",
			apiKey:   "sk-1234567890abcdef1234567890abcdef",
			wantErr:  false,
		},
		{
			name:     "empty openai key",
			provider: "openai",
			apiKey:   "",
			wantErr:  true,
		},
		{
			name:     "short openai key",
			provider: "openai",
			apiKey:   "sk-short",
			wantErr:  true,
		},
		{
			name:     "ollama no key required",
			provider: "ollama",
			apiKey:   "",
			wantErr:  false,
		},
		{
			name:     "valid deepseek key",
			provider: "deepseek",
			apiKey:   "sk-1234567890abcdef1234567890abcdef",
			wantErr:  false,
		},
		{
			name:     "unknown provider with valid key",
			provider: "unknown",
			apiKey:   "some-long-api-key-that-is-valid",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIKeyFormat(tt.provider, tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAPIKeyFormat(%q, %q) error = %v, wantErr %v", tt.provider, tt.apiKey, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeForLogging(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "api key in text",
			input:    "Error with key sk-1234567890abcdef1234567890abcdef",
			expected: "Error with key sk-****",
		},
		{
			name:     "bearer token",
			input:    "Authorization: Bearer abc123token",
			expected: "Authorization: Bearer ****",
		},
		{
			name:     "api_key assignment",
			input:    "api_key=mysecretkey123",
			expected: "api_key=****",
		},
		{
			name:     "password in text",
			input:    "password=secret123",
			expected: "password=****",
		},
		{
			name:     "no sensitive data",
			input:    "This is a normal message",
			expected: "This is a normal message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLogging(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeForLogging(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
