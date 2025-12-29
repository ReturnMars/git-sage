package retry

import (
	"math"
	"time"
)

// BackoffStrategy defines how long to wait before retrying.
type BackoffStrategy interface {
	CalculateBackoff(attempt int) time.Duration
}

// ExponentialBackoff implements exponential backoff.
type ExponentialBackoff struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

// NewExponentialBackoff creates a new strategy with defaults.
func NewExponentialBackoff(initial, max time.Duration) *ExponentialBackoff {
	if initial <= 0 {
		initial = 1 * time.Second
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	return &ExponentialBackoff{
		InitialDelay: initial,
		MaxDelay:     max,
	}
}

// CalculateBackoff returns initial * 2^attempt, capped at max.
func (s *ExponentialBackoff) CalculateBackoff(attempt int) time.Duration {
	if attempt < 0 {
		return 0
	}
	// 2^attempt
	factor := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(s.InitialDelay) * factor)

	if delay > s.MaxDelay {
		return s.MaxDelay
	}
	return delay
}
