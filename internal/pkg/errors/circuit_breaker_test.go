package errors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	if cb.State() != CircuitClosed {
		t.Errorf("Initial state = %v, want CircuitClosed", cb.State())
	}

	// Successful request should keep circuit closed
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if cb.State() != CircuitClosed {
		t.Errorf("State after success = %v, want CircuitClosed", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		ResetTimeout:        1 * time.Second,
		HalfOpenMaxRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	testErr := errors.New("test error")

	// Cause failures to open the circuit
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	if cb.State() != CircuitOpen {
		t.Errorf("State after failures = %v, want CircuitOpen", cb.State())
	}

	// Next request should fail immediately
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		t.Error("Function should not be called when circuit is open")
		return nil
	})

	if err == nil {
		t.Error("Execute() should return error when circuit is open")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	if cb.State() != CircuitOpen {
		t.Errorf("State = %v, want CircuitOpen", cb.State())
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Next request should transition to half-open
	called := false
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})

	if !called {
		t.Error("Function should be called in half-open state")
	}
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Successful request in half-open should close the circuit
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if cb.State() != CircuitClosed {
		t.Errorf("State after success in half-open = %v, want CircuitClosed", cb.State())
	}
}

func TestCircuitBreaker_ReopensAfterFailureInHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Failed request in half-open should reopen the circuit
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return testErr
	})

	if cb.State() != CircuitOpen {
		t.Errorf("State after failure in half-open = %v, want CircuitOpen", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		ResetTimeout:        1 * time.Second,
		HalfOpenMaxRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	if cb.State() != CircuitOpen {
		t.Errorf("State = %v, want CircuitOpen", cb.State())
	}

	// Reset the circuit
	cb.Reset()

	if cb.State() != CircuitClosed {
		t.Errorf("State after reset = %v, want CircuitClosed", cb.State())
	}
	if cb.ConsecutiveFailures() != 0 {
		t.Errorf("ConsecutiveFailures after reset = %d, want 0", cb.ConsecutiveFailures())
	}
}

func TestCircuitBreaker_ConsecutiveFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    5,
		ResetTimeout:        1 * time.Second,
		HalfOpenMaxRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	testErr := errors.New("test error")

	// Cause some failures
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	if cb.ConsecutiveFailures() != 3 {
		t.Errorf("ConsecutiveFailures = %d, want 3", cb.ConsecutiveFailures())
	}

	// Success should reset counter
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if cb.ConsecutiveFailures() != 0 {
		t.Errorf("ConsecutiveFailures after success = %d, want 0", cb.ConsecutiveFailures())
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
