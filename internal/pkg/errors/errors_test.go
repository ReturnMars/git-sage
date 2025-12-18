package errors

import (
	"errors"
	"testing"
	"time"
)

func TestErrorCode_ExitCode(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected int
	}{
		{"NoStagedChanges", ErrNoStagedChanges, 1},
		{"InvalidConfig", ErrInvalidConfig, 1},
		{"MissingAPIKey", ErrMissingAPIKey, 1},
		{"GitCommandFailed", ErrGitCommandFailed, 2},
		{"FileSystemError", ErrFileSystemError, 2},
		{"AIProviderFailed", ErrAIProviderFailed, 3},
		{"NetworkError", ErrNetworkError, 3},
		{"RateLimited", ErrRateLimited, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.code.ExitCode(); got != tt.expected {
				t.Errorf("ExitCode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name: "without cause",
			err: &AppError{
				Code:    ErrNoStagedChanges,
				Message: "no staged changes",
			},
			expected: "no staged changes",
		},
		{
			name: "with cause",
			err: &AppError{
				Code:    ErrGitCommandFailed,
				Message: "git command failed",
				Cause:   errors.New("exit status 1"),
			},
			expected: "git command failed: exit status 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppError_IsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected bool
	}{
		{
			name:     "rate limited",
			err:      &AppError{Code: ErrRateLimited},
			expected: true,
		},
		{
			name:     "network error",
			err:      &AppError{Code: ErrNetworkError},
			expected: true,
		},
		{
			name:     "timeout",
			err:      &AppError{Code: ErrTimeout},
			expected: true,
		},
		{
			name:     "no staged changes",
			err:      &AppError{Code: ErrNoStagedChanges},
			expected: false,
		},
		{
			name:     "invalid config",
			err:      &AppError{Code: ErrInvalidConfig},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsRetryable(); got != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppError_WithContext(t *testing.T) {
	err := New(ErrGitCommandFailed, "git failed")
	err.WithContext("command", "git commit")
	err.WithContext("exit_code", 1)

	if err.Context["command"] != "git commit" {
		t.Errorf("Context[command] = %v, want 'git commit'", err.Context["command"])
	}
	if err.Context["exit_code"] != 1 {
		t.Errorf("Context[exit_code] = %v, want 1", err.Context["exit_code"])
	}
}

func TestAppError_WithSuggestion(t *testing.T) {
	err := New(ErrNoStagedChanges, "no staged changes")
	err.WithSuggestion("Use 'git add' to stage changes")

	if err.Suggestion != "Use 'git add' to stage changes" {
		t.Errorf("Suggestion = %v, want 'Use 'git add' to stage changes'", err.Suggestion)
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("underlying error")
	wrapped := Wrap(cause, ErrGitCommandFailed, "git command failed")

	if wrapped.Code != ErrGitCommandFailed {
		t.Errorf("Code = %v, want %v", wrapped.Code, ErrGitCommandFailed)
	}
	if wrapped.Message != "git command failed" {
		t.Errorf("Message = %v, want 'git command failed'", wrapped.Message)
	}
	if !errors.Is(wrapped, cause) {
		t.Error("Wrapped error should contain the cause")
	}
}

func TestIsAppError(t *testing.T) {
	appErr := New(ErrNoStagedChanges, "no staged changes")
	regularErr := errors.New("regular error")

	if !IsAppError(appErr) {
		t.Error("IsAppError should return true for AppError")
	}
	if IsAppError(regularErr) {
		t.Error("IsAppError should return false for regular error")
	}
}

func TestGetExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "app error user",
			err:      New(ErrNoStagedChanges, "no staged changes"),
			expected: 1,
		},
		{
			name:     "app error system",
			err:      New(ErrGitCommandFailed, "git failed"),
			expected: 2,
		},
		{
			name:     "app error external",
			err:      New(ErrNetworkError, "network error"),
			expected: 3,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetExitCode(tt.err); got != tt.expected {
				t.Errorf("GetExitCode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseRetryAfterHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected time.Duration
	}{
		{
			name:     "empty",
			header:   "",
			expected: 0,
		},
		{
			name:     "seconds",
			header:   "60",
			expected: 60 * time.Second,
		},
		{
			name:     "invalid",
			header:   "invalid",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseRetryAfterHeader(tt.header); got != tt.expected {
				t.Errorf("ParseRetryAfterHeader() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewNoStagedChangesError(t *testing.T) {
	err := NewNoStagedChangesError()

	if err.Code != ErrNoStagedChanges {
		t.Errorf("Code = %v, want %v", err.Code, ErrNoStagedChanges)
	}
	if err.Suggestion == "" {
		t.Error("Suggestion should not be empty")
	}
}

func TestNewMissingAPIKeyError(t *testing.T) {
	err := NewMissingAPIKeyError("openai")

	if err.Code != ErrMissingAPIKey {
		t.Errorf("Code = %v, want %v", err.Code, ErrMissingAPIKey)
	}
	if err.Suggestion == "" {
		t.Error("Suggestion should not be empty")
	}
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError(30 * time.Second)

	if err.Code != ErrRateLimited {
		t.Errorf("Code = %v, want %v", err.Code, ErrRateLimited)
	}
	if err.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want %v", err.RetryAfter, 30*time.Second)
	}
	if !err.IsRetryable() {
		t.Error("Rate limit error should be retryable")
	}
}

func TestFormatError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains []string
	}{
		{
			name:     "nil error",
			err:      nil,
			contains: []string{},
		},
		{
			name: "app error with suggestion",
			err: &AppError{
				Code:       ErrNoStagedChanges,
				Message:    "no staged changes",
				Suggestion: "Use git add",
			},
			contains: []string{"Error:", "no staged changes", "Suggestion:", "Use git add"},
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			contains: []string{"Error:", "regular error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatError(tt.err)
			for _, s := range tt.contains {
				if len(s) > 0 && !contains(result, s) {
					t.Errorf("FormatError() should contain %q, got %q", s, result)
				}
			}
		})
	}
}

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
