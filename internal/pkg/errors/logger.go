// Package errors provides error types, handling utilities, and retry logic for GitSage.
package errors

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the logging level.
type LogLevel int

const (
	// LogLevelError logs only errors.
	LogLevelError LogLevel = iota
	// LogLevelWarn logs warnings and errors.
	LogLevelWarn
	// LogLevelInfo logs info, warnings, and errors.
	LogLevelInfo
	// LogLevelDebug logs everything including debug messages.
	LogLevelDebug
)

// String returns the string representation of LogLevel.
func (l LogLevel) String() string {
	switch l {
	case LogLevelError:
		return "ERROR"
	case LogLevelWarn:
		return "WARN"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging with verbose mode support.
type Logger struct {
	mu      sync.Mutex
	output  io.Writer
	level   LogLevel
	verbose bool
}

// Global logger instance
var defaultLogger = &Logger{
	output:  os.Stderr,
	level:   LogLevelError,
	verbose: false,
}

// SetVerbose enables or disables verbose logging.
func SetVerbose(verbose bool) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.verbose = verbose
	if verbose {
		defaultLogger.level = LogLevelDebug
	} else {
		defaultLogger.level = LogLevelError
	}
}

// IsVerbose returns whether verbose logging is enabled.
func IsVerbose() bool {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	return defaultLogger.verbose
}

// SetOutput sets the output writer for the logger.
func SetOutput(w io.Writer) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.output = w
}

// NewLogger creates a new logger with the given configuration.
func NewLogger(output io.Writer, verbose bool) *Logger {
	level := LogLevelError
	if verbose {
		level = LogLevelDebug
	}
	return &Logger{
		output:  output,
		level:   level,
		verbose: verbose,
	}
}

// log writes a log message at the given level.
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level > l.level {
		return
	}

	timestamp := time.Now().Format("15:04:05")
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.output, "[%s] %s: %s\n", timestamp, level.String(), message)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LogLevelError, format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LogLevelWarn, format, args...)
}

// Info logs an info message.
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LogLevelInfo, format, args...)
}

// Debug logs a debug message.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LogLevelDebug, format, args...)
}

// LogAPIRequest logs an API request in verbose mode.
func (l *Logger) LogAPIRequest(provider, endpoint, model string, promptLength int) {
	if !l.verbose {
		return
	}
	l.Debug("API Request: provider=%s, endpoint=%s, model=%s, prompt_length=%d",
		provider, endpoint, model, promptLength)
}

// LogAPIResponse logs an API response in verbose mode.
func (l *Logger) LogAPIResponse(provider string, statusCode int, responseLength int, duration time.Duration) {
	if !l.verbose {
		return
	}
	l.Debug("API Response: provider=%s, status=%d, response_length=%d, duration=%v",
		provider, statusCode, responseLength, duration)
}

// LogRetry logs a retry attempt in verbose mode.
func (l *Logger) LogRetry(attempt int, maxAttempts int, err error, delay time.Duration) {
	if !l.verbose {
		return
	}
	l.Debug("Retry attempt %d/%d after error: %v (waiting %v)", attempt, maxAttempts, err, delay)
}

// LogCircuitBreaker logs circuit breaker state changes.
func (l *Logger) LogCircuitBreaker(state CircuitState, failures int) {
	if !l.verbose {
		return
	}
	l.Debug("Circuit breaker state: %s (consecutive failures: %d)", state.String(), failures)
}

// Package-level logging functions using the default logger

// Error logs an error message.
func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

// Warn logs a warning message.
func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

// Info logs an info message.
func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

// Debug logs a debug message.
func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

// LogAPIRequest logs an API request in verbose mode.
func LogAPIRequest(provider, endpoint, model string, promptLength int) {
	defaultLogger.LogAPIRequest(provider, endpoint, model, promptLength)
}

// LogAPIResponse logs an API response in verbose mode.
func LogAPIResponse(provider string, statusCode int, responseLength int, duration time.Duration) {
	defaultLogger.LogAPIResponse(provider, statusCode, responseLength, duration)
}

// LogRetry logs a retry attempt in verbose mode.
func LogRetry(attempt int, maxAttempts int, err error, delay time.Duration) {
	defaultLogger.LogRetry(attempt, maxAttempts, err, delay)
}

// LogCircuitBreaker logs circuit breaker state changes.
func LogCircuitBreaker(state CircuitState, failures int) {
	defaultLogger.LogCircuitBreaker(state, failures)
}

// MaskAPIKey masks an API key for safe logging, showing only the last 4 characters.
func MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(apiKey)-4) + apiKey[len(apiKey)-4:]
}
