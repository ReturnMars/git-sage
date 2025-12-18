package errors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetry_Success(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attempts := 0
	err := Retry(context.Background(), config, func(ctx context.Context) error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("Retry() error = %v, want nil", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attempts := 0
	err := Retry(context.Background(), config, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return NewNetworkError(errors.New("connection failed"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Retry() error = %v, want nil", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attempts := 0
	expectedErr := NewNoStagedChangesError()
	err := Retry(context.Background(), config, func(ctx context.Context) error {
		attempts++
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Retry() error = %v, want %v", err, expectedErr)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (should not retry non-retryable errors)", attempts)
	}
}

func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attempts := 0
	err := Retry(context.Background(), config, func(ctx context.Context) error {
		attempts++
		return NewNetworkError(errors.New("connection failed"))
	})

	if err == nil {
		t.Error("Retry() should return error after max attempts")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}

	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, config, func(ctx context.Context) error {
		attempts++
		return NewNetworkError(errors.New("connection failed"))
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Retry() error = %v, want context.Canceled", err)
	}
}

func TestRetry_RespectRetryAfter(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attempts := 0
	start := time.Now()

	err := Retry(context.Background(), config, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return NewRateLimitError(50 * time.Millisecond)
		}
		return nil
	})

	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Retry() error = %v, want nil", err)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("elapsed = %v, should be at least 50ms (retry-after)", elapsed)
	}
}

func TestRetryWithNotify(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attempts := 0
	notifications := 0

	err := RetryWithNotify(context.Background(), config, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return NewNetworkError(errors.New("connection failed"))
		}
		return nil
	}, func(attempt int, err error, delay time.Duration) {
		notifications++
	})

	if err != nil {
		t.Errorf("RetryWithNotify() error = %v, want nil", err)
	}
	if notifications != 2 {
		t.Errorf("notifications = %d, want 2", notifications)
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 10 * time.Second}, // Capped at MaxDelay
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := calculateRetryDelay(config, tt.attempt, nil)
			if delay != tt.expected {
				t.Errorf("calculateRetryDelay(%d) = %v, want %v", tt.attempt, delay, tt.expected)
			}
		})
	}
}

func TestCalculateRetryDelay_WithRetryAfter(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}

	err := NewRateLimitError(30 * time.Second)
	delay := calculateRetryDelay(config, 0, err)

	if delay != 30*time.Second {
		t.Errorf("calculateRetryDelay() = %v, want 30s (from RetryAfter)", delay)
	}
}
