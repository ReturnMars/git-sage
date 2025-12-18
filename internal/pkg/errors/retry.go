// Package errors provides error types, handling utilities, and retry logic for GitSage.
package errors

import (
	"context"
	"math/rand"
	"time"
)

// RetryConfig contains configuration for retry logic.
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       bool // Add random jitter to delays
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// RetryFunc is a function that can be retried.
type RetryFunc func(ctx context.Context) error

// Retry executes a function with retry logic.
func Retry(ctx context.Context, config RetryConfig, fn RetryFunc) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Execute the function
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Check if error is retryable
		if !IsRetryable(lastErr) {
			return lastErr
		}

		// Check if this was the last attempt
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Calculate delay
		delay := calculateRetryDelay(config, attempt, lastErr)

		// Wait before retry (respect context cancellation)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	return lastErr
}

// calculateRetryDelay calculates the delay for a retry attempt.
func calculateRetryDelay(config RetryConfig, attempt int, err error) time.Duration {
	// Check if error has a specific retry-after duration
	if retryAfter := GetRetryAfter(err); retryAfter > 0 {
		return retryAfter
	}

	// Calculate exponential backoff
	delay := float64(config.InitialDelay) * pow(config.Multiplier, float64(attempt))

	// Apply max delay cap
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Add jitter if enabled (±25%)
	if config.Jitter {
		jitter := delay * 0.25 * (rand.Float64()*2 - 1)
		delay += jitter
	}

	return time.Duration(delay)
}

// pow calculates base^exp for float64.
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// RetryWithCallback executes a function with retry logic and a callback for each attempt.
type RetryCallback func(attempt int, err error, delay time.Duration)

// RetryWithNotify executes a function with retry logic and notifies on each retry.
func RetryWithNotify(ctx context.Context, config RetryConfig, fn RetryFunc, notify RetryCallback) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Execute the function
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Check if error is retryable
		if !IsRetryable(lastErr) {
			return lastErr
		}

		// Check if this was the last attempt
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Calculate delay
		delay := calculateRetryDelay(config, attempt, lastErr)

		// Notify about retry
		if notify != nil {
			notify(attempt+1, lastErr, delay)
		}

		// Wait before retry (respect context cancellation)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	return lastErr
}
