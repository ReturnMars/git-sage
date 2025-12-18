package errors

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestLogger_Levels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, true) // verbose mode

	logger.Error("error message")
	logger.Warn("warn message")
	logger.Info("info message")
	logger.Debug("debug message")

	output := buf.String()

	if !strings.Contains(output, "ERROR") {
		t.Error("Output should contain ERROR")
	}
	if !strings.Contains(output, "WARN") {
		t.Error("Output should contain WARN")
	}
	if !strings.Contains(output, "INFO") {
		t.Error("Output should contain INFO")
	}
	if !strings.Contains(output, "DEBUG") {
		t.Error("Output should contain DEBUG")
	}
}

func TestLogger_NonVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false) // non-verbose mode

	logger.Error("error message")
	logger.Warn("warn message")
	logger.Info("info message")
	logger.Debug("debug message")

	output := buf.String()

	if !strings.Contains(output, "ERROR") {
		t.Error("Output should contain ERROR even in non-verbose mode")
	}
	if strings.Contains(output, "WARN") {
		t.Error("Output should not contain WARN in non-verbose mode")
	}
	if strings.Contains(output, "INFO") {
		t.Error("Output should not contain INFO in non-verbose mode")
	}
	if strings.Contains(output, "DEBUG") {
		t.Error("Output should not contain DEBUG in non-verbose mode")
	}
}

func TestLogger_LogAPIRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, true)

	logger.LogAPIRequest("openai", "https://api.openai.com", "gpt-4", 1000)

	output := buf.String()

	if !strings.Contains(output, "openai") {
		t.Error("Output should contain provider name")
	}
	if !strings.Contains(output, "gpt-4") {
		t.Error("Output should contain model name")
	}
}

func TestLogger_LogAPIResponse(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, true)

	logger.LogAPIResponse("openai", 200, 500, 100*time.Millisecond)

	output := buf.String()

	if !strings.Contains(output, "openai") {
		t.Error("Output should contain provider name")
	}
	if !strings.Contains(output, "200") {
		t.Error("Output should contain status code")
	}
}

func TestLogger_LogRetry(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, true)

	testErr := New(ErrNetworkError, "connection failed")
	logger.LogRetry(1, 3, testErr, 1*time.Second)

	output := buf.String()

	if !strings.Contains(output, "Retry") {
		t.Error("Output should contain 'Retry'")
	}
	if !strings.Contains(output, "1/3") {
		t.Error("Output should contain attempt count")
	}
}

func TestLogger_LogCircuitBreaker(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, true)

	logger.LogCircuitBreaker(CircuitOpen, 5)

	output := buf.String()

	if !strings.Contains(output, "open") {
		t.Error("Output should contain circuit state")
	}
	if !strings.Contains(output, "5") {
		t.Error("Output should contain failure count")
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "normal key",
			apiKey:   "sk-1234567890abcdef",
			expected: "***************cdef",
		},
		{
			name:     "short key",
			apiKey:   "abc",
			expected: "****",
		},
		{
			name:     "exactly 4 chars",
			apiKey:   "abcd",
			expected: "****",
		},
		{
			name:     "5 chars",
			apiKey:   "abcde",
			expected: "*bcde",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskAPIKey(tt.apiKey); got != tt.expected {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.apiKey, got, tt.expected)
			}
		})
	}
}

func TestSetVerbose(t *testing.T) {
	// Save original state
	originalVerbose := IsVerbose()
	defer SetVerbose(originalVerbose)

	SetVerbose(true)
	if !IsVerbose() {
		t.Error("IsVerbose() should return true after SetVerbose(true)")
	}

	SetVerbose(false)
	if IsVerbose() {
		t.Error("IsVerbose() should return false after SetVerbose(false)")
	}
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelError, "ERROR"},
		{LogLevelWarn, "WARN"},
		{LogLevelInfo, "INFO"},
		{LogLevelDebug, "DEBUG"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
