// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gitsage/gitsage/internal/pkg/git"
	"github.com/tmc/langchaingo/llms"
)

// MockLLM implements llms.Model for testing
type MockLLM struct {
	GenerateContentFunc func(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error)
	CallCount           int
}

func (m *MockLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	m.CallCount++
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, messages, options...)
	}
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{Content: "feat(test): test commit message\n\n- test: added test functionality"},
		},
	}, nil
}

func (m *MockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", nil
}

func TestNewLangChainWrapper(t *testing.T) {
	mockLLM := &MockLLM{}
	config := ProviderConfig{
		APIKey:      "test-key",
		Model:       "test-model",
		Temperature: 0.5,
		MaxTokens:   100,
	}

	wrapper := NewLangChainWrapper(mockLLM, config, "test-provider")

	if wrapper == nil {
		t.Fatal("NewLangChainWrapper() returned nil")
	}
	if wrapper.llm != mockLLM {
		t.Error("LLM not set correctly")
	}
	if wrapper.providerName != "test-provider" {
		t.Errorf("providerName = %q, want %q", wrapper.providerName, "test-provider")
	}
	if wrapper.promptTemplate == nil {
		t.Error("promptTemplate should not be nil")
	}
}

func TestLangChainWrapper_SetPromptTemplate(t *testing.T) {
	mockLLM := &MockLLM{}
	wrapper := NewLangChainWrapper(mockLLM, ProviderConfig{}, "test")

	customPT := NewLangChainPromptTemplateWithCustom("custom system", "custom user")
	wrapper.SetPromptTemplate(customPT)

	// Setting nil should not change the template
	originalPT := wrapper.promptTemplate
	wrapper.SetPromptTemplate(nil)
	if wrapper.promptTemplate != originalPT {
		t.Error("SetPromptTemplate(nil) should not change the template")
	}
}

func TestLangChainWrapper_Generate_NilRequest(t *testing.T) {
	mockLLM := &MockLLM{}
	wrapper := NewLangChainWrapper(mockLLM, ProviderConfig{}, "test")

	_, err := wrapper.generate(context.Background(), nil)
	if err == nil {
		t.Error("generate() should return error for nil request")
	}
	if err.Error() != "request cannot be nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLangChainWrapper_Generate_EmptyDiffChunks(t *testing.T) {
	mockLLM := &MockLLM{}
	wrapper := NewLangChainWrapper(mockLLM, ProviderConfig{}, "test")

	req := &GenerateRequest{
		DiffChunks: nil,
	}

	_, err := wrapper.generate(context.Background(), req)
	if err == nil {
		t.Error("generate() should return error for empty diff chunks")
	}
	if err.Error() != "no diff chunks provided" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLangChainWrapper_Generate_WithCustomPrompt(t *testing.T) {
	mockLLM := &MockLLM{}
	wrapper := NewLangChainWrapper(mockLLM, ProviderConfig{
		Temperature: 0.2,
		MaxTokens:   500,
	}, "test")

	req := &GenerateRequest{
		CustomPrompt: "Generate a commit message for this change",
	}

	resp, err := wrapper.generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate() error = %v", err)
	}
	if resp == nil {
		t.Fatal("generate() returned nil response")
	}
	if mockLLM.CallCount != 1 {
		t.Errorf("LLM CallCount = %d, want 1", mockLLM.CallCount)
	}
}

func TestLangChainWrapper_Generate_Success(t *testing.T) {
	mockLLM := &MockLLM{
		GenerateContentFunc: func(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
			return &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{Content: "feat(api): add new endpoint\n\n- api: added GET /users endpoint"},
				},
			}, nil
		},
	}
	wrapper := NewLangChainWrapper(mockLLM, ProviderConfig{
		Model:       "test-model",
		Temperature: 0.2,
		MaxTokens:   500,
	}, "test")

	req := &GenerateRequest{
		DiffChunks: []git.DiffChunk{
			{FilePath: "api/users.go", ChangeType: git.ChangeTypeAdded, Content: "func GetUsers() {}"},
		},
		DiffStats: &git.DiffStats{
			TotalFiles:     1,
			TotalAdditions: 10,
			TotalDeletions: 0,
		},
	}

	resp, err := wrapper.generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate() error = %v", err)
	}
	if resp == nil {
		t.Fatal("generate() returned nil response")
	}
	if resp.Subject == "" {
		t.Error("response Subject should not be empty")
	}
}

func TestLangChainWrapper_Generate_EmptyResponse(t *testing.T) {
	mockLLM := &MockLLM{
		GenerateContentFunc: func(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
			return &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{Content: ""},
				},
			}, nil
		},
	}
	wrapper := NewLangChainWrapper(mockLLM, ProviderConfig{}, "test")

	req := &GenerateRequest{
		DiffChunks: []git.DiffChunk{
			{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "test"},
		},
		DiffStats: &git.DiffStats{TotalFiles: 1},
	}

	_, err := wrapper.generate(context.Background(), req)
	if err == nil {
		t.Error("generate() should return error for empty response")
	}
}

func TestLangChainWrapper_GenerateWithRetry_Success(t *testing.T) {
	callCount := 0
	mockLLM := &MockLLM{
		GenerateContentFunc: func(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
			callCount++
			return &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{Content: "feat(test): success"},
				},
			}, nil
		},
	}
	wrapper := NewLangChainWrapper(mockLLM, ProviderConfig{}, "test")

	req := &GenerateRequest{
		DiffChunks: []git.DiffChunk{
			{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "test"},
		},
		DiffStats: &git.DiffStats{TotalFiles: 1},
	}

	resp, err := wrapper.GenerateWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateWithRetry() error = %v", err)
	}
	if resp == nil {
		t.Fatal("GenerateWithRetry() returned nil response")
	}
	if callCount != 1 {
		t.Errorf("LLM should be called once on success, got %d", callCount)
	}
}

func TestLangChainWrapper_GenerateWithRetry_ContextCancelled(t *testing.T) {
	mockLLM := &MockLLM{
		GenerateContentFunc: func(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
			return nil, errors.New("429 rate limit exceeded")
		},
	}
	wrapper := NewLangChainWrapper(mockLLM, ProviderConfig{}, "test")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &GenerateRequest{
		DiffChunks: []git.DiffChunk{
			{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "test"},
		},
		DiffStats: &git.DiffStats{TotalFiles: 1},
	}

	_, err := wrapper.GenerateWithRetry(ctx, req)
	if err == nil {
		t.Error("GenerateWithRetry() should return error for cancelled context")
	}
}

func TestLangChainWrapper_IsRetryableError(t *testing.T) {
	wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, "test")

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"429 error", errors.New("429 Too Many Requests"), true},
		{"500 error", errors.New("500 Internal Server Error"), true},
		{"502 error", errors.New("502 Bad Gateway"), true},
		{"503 error", errors.New("503 Service Unavailable"), true},
		{"504 error", errors.New("504 Gateway Timeout"), true},
		{"rate limit", errors.New("rate limit exceeded"), true},
		{"too many requests", errors.New("too many requests"), true},
		{"401 error", errors.New("401 Unauthorized"), false},
		{"400 error", errors.New("400 Bad Request"), false},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"generic error", errors.New("some error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapper.isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestLangChainWrapper_WrapError(t *testing.T) {
	wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, "test-provider")

	tests := []struct {
		name            string
		err             error
		expectNil       bool
		containsMessage string
	}{
		{"nil error", nil, true, ""},
		{"401 error", errors.New("401 Unauthorized"), false, "authentication"},
		{"unauthorized", errors.New("unauthorized access"), false, "authentication"},
		{"429 error", errors.New("429 rate limit"), false, "rate limit"},
		{"rate limit", errors.New("rate limit exceeded"), false, "rate limit"},
		{"connection refused", errors.New("connection refused"), false, "cannot connect"},
		{"generic error", errors.New("some error"), false, "test-provider"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapper.wrapError(tt.err)
			if tt.expectNil {
				if result != nil {
					t.Errorf("wrapError(%v) = %v, want nil", tt.err, result)
				}
			} else {
				if result == nil {
					t.Errorf("wrapError(%v) = nil, want error", tt.err)
				} else if tt.containsMessage != "" {
					errStr := result.Error()
					if !containsIgnoreCase(errStr, tt.containsMessage) {
						t.Errorf("wrapError(%v) error = %q, should contain %q", tt.err, errStr, tt.containsMessage)
					}
				}
			}
		})
	}
}

func TestLangChainWrapper_WrapError_OllamaConnectionRefused(t *testing.T) {
	wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, "ollama")

	err := wrapper.wrapError(errors.New("connection refused"))
	if err == nil {
		t.Fatal("wrapError() should return error")
	}

	errStr := err.Error()
	if !containsIgnoreCase(errStr, "ollama") {
		t.Errorf("error should mention ollama, got: %s", errStr)
	}
}

func TestLangChainWrapper_WrapError_Timeout(t *testing.T) {
	wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, "test")

	err := wrapper.wrapError(context.DeadlineExceeded)
	if err == nil {
		t.Fatal("wrapError() should return error")
	}

	errStr := err.Error()
	// Check for either "timeout" or "timed out" in the error message
	if !containsIgnoreCase(errStr, "timeout") && !containsIgnoreCase(errStr, "timed out") {
		t.Errorf("error should mention timeout, got: %s", errStr)
	}
}

func TestLangChainWrapper_ChunkingThreshold(t *testing.T) {
	var receivedMessages []llms.MessageContent
	mockLLM := &MockLLM{
		GenerateContentFunc: func(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
			receivedMessages = messages
			return &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{Content: "feat(test): large commit"},
				},
			}, nil
		},
	}
	wrapper := NewLangChainWrapper(mockLLM, ProviderConfig{}, "test")

	// Create a large diff (>10KB)
	largeContent := make([]byte, 15*1024) // 15KB
	for i := range largeContent {
		largeContent[i] = 'x'
	}

	req := &GenerateRequest{
		DiffChunks: []git.DiffChunk{
			{FilePath: "large.go", ChangeType: git.ChangeTypeModified, Content: string(largeContent)},
		},
		DiffStats: &git.DiffStats{TotalFiles: 1, TotalAdditions: 100},
	}

	_, err := wrapper.generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate() error = %v", err)
	}

	// Verify messages were created
	if len(receivedMessages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(receivedMessages))
	}
}

// Helper function for case-insensitive contains
func containsIgnoreCase(s, substr string) bool {
	return contains(strings.ToLower(s), strings.ToLower(substr))
}

// The contains function should be available from ollama.go, but adding local version for safety
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCalculateBackoff_Values(t *testing.T) {
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 10 * time.Second}, // Capped at MaxRetryDelay
		{10, 10 * time.Second},
	}

	for _, tt := range tests {
		result := calculateBackoff(tt.attempt)
		if result != tt.expected {
			t.Errorf("calculateBackoff(%d) = %v, want %v", tt.attempt, result, tt.expected)
		}
	}
}
