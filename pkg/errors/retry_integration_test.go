//go:build integration

package errors

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockNavigationError simulates navigation timeout errors
type MockNavigationError struct {
	attempts int
}

func (m *MockNavigationError) Navigate() error {
	m.attempts++
	if m.attempts < 3 {
		return NewTimeoutError("navigation", 30000)
	}
	return nil
}

// MockElementFinder simulates element not found errors
type MockElementFinder struct {
	attempts int
}

func (m *MockElementFinder) FindElement() error {
	m.attempts++
	if m.attempts < 2 {
		return NewElementNotFoundError("button#submit", 5000)
	}
	return nil
}

func TestRetryIntegration_NavigationTimeout(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      100 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeTimeout},
		Jitter:          0.0,
	}
	ctx := context.Background()

	mockNav := &MockNavigationError{}
	operation := func() error {
		return mockNav.Navigate()
	}

	err := WithRetry(ctx, config, operation)
	if err != nil {
		t.Errorf("expected navigation to succeed after retry, got error: %v", err)
	}
	if mockNav.attempts != 3 {
		t.Errorf("expected 3 navigation attempts, got %d", mockNav.attempts)
	}
}

func TestRetryIntegration_ElementNotFound(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      100 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx := context.Background()

	mockFinder := &MockElementFinder{}
	operation := func() error {
		return mockFinder.FindElement()
	}

	err := WithRetry(ctx, config, operation)
	if err != nil {
		t.Errorf("expected element find to succeed after retry, got error: %v", err)
	}
	if mockFinder.attempts != 2 {
		t.Errorf("expected 2 find attempts, got %d", mockFinder.attempts)
	}
}

func TestRetryIntegration_NavigationFailsAllAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      100 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeTimeout},
		Jitter:          0.0,
	}
	ctx := context.Background()

	// Mock that always fails
	operation := func() error {
		return NewTimeoutError("navigation", 30000)
	}

	err := WithRetry(ctx, config, operation)
	if err == nil {
		t.Error("expected error after exhausting retries")
	}

	// Verify it's a RetryExhaustedError
	var retryErr *RetryExhaustedError
	if !errors.As(err, &retryErr) {
		t.Errorf("expected RetryExhaustedError, got %T", err)
	} else {
		if retryErr.Attempts != 3 {
			t.Errorf("expected 3 attempts in error, got %d", retryErr.Attempts)
		}

		// Verify underlying error is still a TimeoutError
		var timeoutErr *TimeoutError
		if !errors.As(retryErr.Err, &timeoutErr) {
			t.Errorf("expected underlying error to be TimeoutError, got %T", retryErr.Err)
		}
	}
}

func TestRetryIntegration_NonRetryableNavigationError(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      100 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound, CodeTimeout}, // NavigationFailed not retryable
		Jitter:          0.0,
	}
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		return NewNavigationError("https://example.com", "404 not found")
	}

	err := WithRetry(ctx, config, operation)
	if err == nil {
		t.Error("expected navigation error to be returned")
	}

	// Should only attempt once (no retry for non-retryable error)
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}

	// Should not be wrapped in RetryExhaustedError
	var retryErr *RetryExhaustedError
	if errors.As(err, &retryErr) {
		t.Error("non-retryable error should not be wrapped in RetryExhaustedError")
	}
}

func TestRetryIntegration_ExponentialBackoffTiming(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  50 * time.Millisecond,
		MaxBackoff:      500 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeTimeout},
		Jitter:          0.0, // No jitter for predictable timing
	}
	ctx := context.Background()

	attemptTimes := []time.Time{}
	operation := func() error {
		attemptTimes = append(attemptTimes, time.Now())
		return NewTimeoutError("navigation", 30000)
	}

	WithRetry(ctx, config, operation)

	if len(attemptTimes) != 3 {
		t.Fatalf("expected 3 attempts, got %d", len(attemptTimes))
	}

	// Verify exponential backoff timing
	// First backoff: 50ms (attempt=0)
	delay1 := attemptTimes[1].Sub(attemptTimes[0])
	if delay1 < 40*time.Millisecond || delay1 > 60*time.Millisecond {
		t.Errorf("expected first backoff ~50ms, got %v", delay1)
	}

	// Second backoff: 100ms (attempt=1, 50ms * 2^1)
	delay2 := attemptTimes[2].Sub(attemptTimes[1])
	if delay2 < 90*time.Millisecond || delay2 > 110*time.Millisecond {
		t.Errorf("expected second backoff ~100ms, got %v", delay2)
	}
}

func TestRetryIntegration_ContextCancellation(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     5,
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      500 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeTimeout},
		Jitter:          0.0,
	}

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	operation := func() error {
		attempts++
		if attempts == 2 {
			// Cancel context after second attempt
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
		}
		return NewTimeoutError("navigation", 30000)
	}

	err := WithRetry(ctx, config, operation)

	// Should stop retrying due to context cancellation
	if err == nil {
		t.Error("expected context cancellation error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}

	// Should not complete all 5 attempts
	if attempts >= 5 {
		t.Errorf("expected fewer than 5 attempts due to cancellation, got %d", attempts)
	}
}

func TestRetryIntegration_DefaultConfig(t *testing.T) {
	ctx := context.Background()

	mockFinder := &MockElementFinder{}
	operation := func() error {
		return mockFinder.FindElement()
	}

	// Use nil config to test defaults
	err := WithRetry(ctx, nil, operation)
	if err != nil {
		t.Errorf("expected operation to succeed with default config, got error: %v", err)
	}
	if mockFinder.attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", mockFinder.attempts)
	}
}
