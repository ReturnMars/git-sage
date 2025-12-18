// Package errors provides error types, handling utilities, and retry logic for GitSage.
package errors

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrorCode represents the category of an error.
type ErrorCode int

const (
	// User errors (Exit Code 1)
	ErrNoStagedChanges ErrorCode = iota + 100
	ErrInvalidConfig
	ErrMissingAPIKey
	ErrInvalidArguments

	// System errors (Exit Code 2)
	ErrGitCommandFailed ErrorCode = iota + 200
	ErrFileSystemError
	ErrConfigCorruption

	// External errors (Exit Code 3)
	ErrAIProviderFailed ErrorCode = iota + 300
	ErrNetworkError
	ErrRateLimited
	ErrTimeout
	ErrAuthenticationFailed
)

// ExitCode returns the appropriate exit code for an error code.
func (c ErrorCode) ExitCode() int {
	switch {
	case c >= 100 && c < 200:
		return 1 // User errors
	case c >= 200 && c < 300:
		return 2 // System errors
	case c >= 300:
		return 3 // External errors
	default:
		return 1
	}
}

// String returns a human-readable name for the error code.
func (c ErrorCode) String() string {
	switch c {
	case ErrNoStagedChanges:
		return "NoStagedChanges"
	case ErrInvalidConfig:
		return "InvalidConfig"
	case ErrMissingAPIKey:
		return "MissingAPIKey"
	case ErrInvalidArguments:
		return "InvalidArguments"
	case ErrGitCommandFailed:
		return "GitCommandFailed"
	case ErrFileSystemError:
		return "FileSystemError"
	case ErrConfigCorruption:
		return "ConfigCorruption"
	case ErrAIProviderFailed:
		return "AIProviderFailed"
	case ErrNetworkError:
		return "NetworkError"
	case ErrRateLimited:
		return "RateLimited"
	case ErrTimeout:
		return "Timeout"
	case ErrAuthenticationFailed:
		return "AuthenticationFailed"
	default:
		return "Unknown"
	}
}

// AppError represents an application error with context.
type AppError struct {
	Code       ErrorCode
	Message    string
	Cause      error
	Context    map[string]interface{}
	Suggestion string
	RetryAfter time.Duration // For rate limit errors
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error can be retried.
func (e *AppError) IsRetryable() bool {
	switch e.Code {
	case ErrRateLimited, ErrNetworkError, ErrTimeout:
		return true
	case ErrAIProviderFailed:
		// Check if the underlying cause is retryable
		if e.Cause != nil {
			var retryable RetryableError
			if errors.As(e.Cause, &retryable) {
				return retryable.IsRetryable()
			}
		}
		return false
	default:
		return false
	}
}

// GetRetryAfter returns the duration to wait before retrying.
func (e *AppError) GetRetryAfter() time.Duration {
	if e.RetryAfter > 0 {
		return e.RetryAfter
	}
	return 0
}

// WithContext adds context to the error.
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithSuggestion adds a suggestion to the error.
func (e *AppError) WithSuggestion(suggestion string) *AppError {
	e.Suggestion = suggestion
	return e
}

// RetryableError is an interface for errors that can be retried.
type RetryableError interface {
	error
	IsRetryable() bool
	GetRetryAfter() time.Duration
}

// Ensure AppError implements RetryableError
var _ RetryableError = (*AppError)(nil)

// New creates a new AppError.
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an error with context.
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// WrapWithContext wraps an error with a context message.
func WrapWithContext(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// IsAppError checks if an error is an AppError.
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetAppError extracts an AppError from an error chain.
func GetAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

// GetExitCode returns the appropriate exit code for an error.
func GetExitCode(err error) int {
	if appErr := GetAppError(err); appErr != nil {
		return appErr.Code.ExitCode()
	}
	return 1 // Default to user error
}

// IsRetryable checks if an error is retryable.
func IsRetryable(err error) bool {
	var retryable RetryableError
	if errors.As(err, &retryable) {
		return retryable.IsRetryable()
	}
	return false
}

// GetRetryAfter returns the retry-after duration for an error.
func GetRetryAfter(err error) time.Duration {
	var retryable RetryableError
	if errors.As(err, &retryable) {
		return retryable.GetRetryAfter()
	}
	return 0
}

// Common error constructors with suggestions

// NewNoStagedChangesError creates an error for no staged changes.
func NewNoStagedChangesError() *AppError {
	return &AppError{
		Code:       ErrNoStagedChanges,
		Message:    "no staged changes found",
		Suggestion: "Use 'git add <files>' to stage changes before generating a commit message",
	}
}

// NewMissingAPIKeyError creates an error for missing API key.
func NewMissingAPIKeyError(provider string) *AppError {
	return &AppError{
		Code:       ErrMissingAPIKey,
		Message:    fmt.Sprintf("API key is required for %s provider", provider),
		Suggestion: "Set your API key using 'gitsage config set provider.api_key <your-key>' or set the GITSAGE_API_KEY environment variable",
	}
}

// NewInvalidConfigError creates an error for invalid configuration.
func NewInvalidConfigError(message string) *AppError {
	return &AppError{
		Code:       ErrInvalidConfig,
		Message:    message,
		Suggestion: "Run 'gitsage config init' to create a valid configuration file",
	}
}

// NewGitError creates an error for git command failures.
func NewGitError(err error, output string) *AppError {
	appErr := &AppError{
		Code:    ErrGitCommandFailed,
		Message: "git command failed",
		Cause:   err,
	}
	if output != "" {
		appErr.Context = map[string]interface{}{
			"output": output,
		}
	}
	return appErr
}

// NewNetworkError creates an error for network failures.
func NewNetworkError(err error) *AppError {
	return &AppError{
		Code:       ErrNetworkError,
		Message:    "network error occurred",
		Cause:      err,
		Suggestion: "Please check your network connection and try again",
	}
}

// NewRateLimitError creates an error for rate limiting.
func NewRateLimitError(retryAfter time.Duration) *AppError {
	suggestion := "Please wait and try again later"
	if retryAfter > 0 {
		suggestion = fmt.Sprintf("Please wait %v and try again", retryAfter)
	}
	return &AppError{
		Code:       ErrRateLimited,
		Message:    "rate limit exceeded",
		RetryAfter: retryAfter,
		Suggestion: suggestion,
	}
}

// NewTimeoutError creates an error for timeouts.
func NewTimeoutError(err error) *AppError {
	return &AppError{
		Code:       ErrTimeout,
		Message:    "request timed out",
		Cause:      err,
		Suggestion: "Please check your network connection or try again later",
	}
}

// NewAuthenticationError creates an error for authentication failures.
func NewAuthenticationError(provider string) *AppError {
	return &AppError{
		Code:       ErrAuthenticationFailed,
		Message:    fmt.Sprintf("authentication failed with %s", provider),
		Suggestion: "Please check your API key is valid and has not expired",
	}
}

// NewAIProviderError creates an error for AI provider failures.
func NewAIProviderError(provider string, err error) *AppError {
	return &AppError{
		Code:       ErrAIProviderFailed,
		Message:    fmt.Sprintf("%s provider error", provider),
		Cause:      err,
		Suggestion: "Please check your API key and network connectivity",
	}
}

// ParseRetryAfterHeader parses the Retry-After header value.
// It handles both seconds (integer) and HTTP-date formats.
func ParseRetryAfterHeader(header string) time.Duration {
	if header == "" {
		return 0
	}

	// Try parsing as seconds
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP-date
	if t, err := http.ParseTime(header); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return duration
		}
	}

	return 0
}

// FormatError formats an error for user display.
// API keys and other sensitive data are automatically masked.
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	var sb strings.Builder

	appErr := GetAppError(err)
	if appErr != nil {
		sb.WriteString("Error: ")
		sb.WriteString(SanitizeErrorMessage(appErr.Message))

		if appErr.Cause != nil {
			sb.WriteString("\n  Cause: ")
			sb.WriteString(SanitizeErrorMessage(appErr.Cause.Error()))
		}

		if appErr.Suggestion != "" {
			sb.WriteString("\n  Suggestion: ")
			sb.WriteString(appErr.Suggestion)
		}
	} else {
		sb.WriteString("Error: ")
		sb.WriteString(SanitizeErrorMessage(err.Error()))
	}

	return sb.String()
}

// FormatErrorVerbose formats an error with full details for verbose mode.
// API keys and other sensitive data are automatically masked.
func FormatErrorVerbose(err error) string {
	if err == nil {
		return ""
	}

	var sb strings.Builder

	appErr := GetAppError(err)
	if appErr != nil {
		sb.WriteString(fmt.Sprintf("Error [%s]: %s\n", appErr.Code.String(), SanitizeErrorMessage(appErr.Message)))

		if appErr.Cause != nil {
			sb.WriteString(fmt.Sprintf("  Cause: %v\n", SanitizeErrorMessage(appErr.Cause.Error())))
			// Print the full error chain
			sb.WriteString("  Error chain:\n")
			printErrorChain(&sb, appErr.Cause, 2)
		}

		if len(appErr.Context) > 0 {
			sb.WriteString("  Context:\n")
			for k, v := range appErr.Context {
				// Sanitize context values as well
				sb.WriteString(fmt.Sprintf("    %s: %v\n", k, SanitizeErrorMessage(fmt.Sprintf("%v", v))))
			}
		}

		if appErr.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("  Suggestion: %s\n", appErr.Suggestion))
		}

		if appErr.RetryAfter > 0 {
			sb.WriteString(fmt.Sprintf("  Retry after: %v\n", appErr.RetryAfter))
		}
	} else {
		sb.WriteString(fmt.Sprintf("Error: %v\n", SanitizeErrorMessage(err.Error())))
		sb.WriteString("  Error chain:\n")
		printErrorChain(&sb, err, 2)
	}

	return sb.String()
}

// printErrorChain prints the error chain with indentation.
func printErrorChain(sb *strings.Builder, err error, indent int) {
	if err == nil {
		return
	}

	prefix := strings.Repeat("  ", indent)
	// Sanitize error message to mask any API keys
	errMsg := SanitizeErrorMessage(err.Error())
	sb.WriteString(fmt.Sprintf("%s- %T: %v\n", prefix, err, errMsg))

	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		printErrorChain(sb, unwrapped, indent+1)
	}
}

// SanitizeErrorMessage masks any API keys or sensitive data in error messages.
func SanitizeErrorMessage(msg string) string {
	// Mask API keys that look like sk-...
	result := apiKeyPattern.ReplaceAllStringFunc(msg, func(match string) string {
		if len(match) <= 4 {
			return "****"
		}
		return strings.Repeat("*", len(match)-4) + match[len(match)-4:]
	})
	return result
}

// apiKeyPattern matches common API key patterns.
var apiKeyPattern = regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`)
