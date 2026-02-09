package errors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts to be 3, got %d", config.MaxAttempts)
	}
	if config.InitialBackoff != 100*time.Millisecond {
		t.Errorf("expected InitialBackoff to be 100ms, got %v", config.InitialBackoff)
	}
	if config.MaxBackoff != 5*time.Second {
		t.Errorf("expected MaxBackoff to be 5s, got %v", config.MaxBackoff)
	}
	if config.BackoffFactor != 2.0 {
		t.Errorf("expected BackoffFactor to be 2.0, got %v", config.BackoffFactor)
	}
	if config.Jitter != 0.1 {
		t.Errorf("expected Jitter to be 0.1, got %v", config.Jitter)
	}
	if len(config.RetryableErrors) != 2 {
		t.Errorf("expected 2 retryable errors, got %d", len(config.RetryableErrors))
	}
	if !contains(config.RetryableErrors, CodeElementNotFound) {
		t.Error("expected ElementNotFound to be in retryable errors")
	}
	if !contains(config.RetryableErrors, CodeTimeout) {
		t.Error("expected Timeout to be in retryable errors")
	}
}

func TestIsRetryable_ElementNotFoundError(t *testing.T) {
	config := DefaultRetryConfig()
	err := NewElementNotFoundError("button", 5000)

	if !IsRetryable(err, config) {
		t.Error("expected ElementNotFoundError to be retryable")
	}
}

func TestIsRetryable_TimeoutError(t *testing.T) {
	config := DefaultRetryConfig()
	err := NewTimeoutError("navigation", 30000)

	if !IsRetryable(err, config) {
		t.Error("expected TimeoutError to be retryable")
	}
}

func TestIsRetryable_SessionNotFoundError(t *testing.T) {
	config := DefaultRetryConfig()
	err := NewSessionNotFoundError("session-123")

	if IsRetryable(err, config) {
		t.Error("expected SessionNotFoundError to not be retryable")
	}
}

func TestIsRetryable_NavigationError(t *testing.T) {
	config := DefaultRetryConfig()
	err := NewNavigationError("https://example.com", "404 not found")

	if IsRetryable(err, config) {
		t.Error("expected NavigationError to not be retryable")
	}
}

func TestIsRetryable_NilError(t *testing.T) {
	config := DefaultRetryConfig()

	if IsRetryable(nil, config) {
		t.Error("expected nil error to not be retryable")
	}
}

func TestIsRetryable_NilConfig(t *testing.T) {
	err := NewElementNotFoundError("button", 5000)

	if IsRetryable(err, nil) {
		t.Error("expected error with nil config to not be retryable")
	}
}

func TestIsRetryable_CustomRetryableErrors(t *testing.T) {
	config := &RetryConfig{
		RetryableErrors: []int{CodeSessionNotFound},
	}

	sessionErr := NewSessionNotFoundError("session-123")
	if !IsRetryable(sessionErr, config) {
		t.Error("expected SessionNotFoundError to be retryable with custom config")
	}

	elementErr := NewElementNotFoundError("button", 5000)
	if IsRetryable(elementErr, config) {
		t.Error("expected ElementNotFoundError to not be retryable with custom config")
	}
}

func TestIsRetryable_EmptyRetryableErrors(t *testing.T) {
	config := &RetryConfig{
		RetryableErrors: []int{},
	}

	err := NewElementNotFoundError("button", 5000)
	if IsRetryable(err, config) {
		t.Error("expected error to not be retryable with empty retryable errors list")
	}
}

func TestIsRetryable_WrappedError(t *testing.T) {
	config := DefaultRetryConfig()
	innerErr := NewElementNotFoundError("button", 5000)
	wrappedErr := errors.Join(errors.New("outer error"), innerErr)

	if !IsRetryable(wrappedErr, config) {
		t.Error("expected wrapped ElementNotFoundError to be retryable")
	}
}

func TestCalculateBackoff_Attempt0(t *testing.T) {
	config := &RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.0, // Disable jitter for predictable testing
	}

	backoff := calculateBackoff(0, config)
	expected := 100 * time.Millisecond

	if backoff != expected {
		t.Errorf("expected backoff to be %v, got %v", expected, backoff)
	}
}

func TestCalculateBackoff_Attempt1(t *testing.T) {
	config := &RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.0,
	}

	backoff := calculateBackoff(1, config)
	expected := 200 * time.Millisecond

	if backoff != expected {
		t.Errorf("expected backoff to be %v, got %v", expected, backoff)
	}
}

func TestCalculateBackoff_MaxBackoffCap(t *testing.T) {
	config := &RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
		BackoffFactor:  2.0,
		Jitter:         0.0,
	}

	// Attempt 10 would normally give 100ms * 2^10 = 102,400ms
	backoff := calculateBackoff(10, config)

	if backoff != config.MaxBackoff {
		t.Errorf("expected backoff to be capped at %v, got %v", config.MaxBackoff, backoff)
	}
}

func TestCalculateBackoff_WithJitter(t *testing.T) {
	config := &RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.1,
	}

	// Run multiple times to ensure jitter produces different values
	backoffs := make(map[time.Duration]bool)
	for i := 0; i < 20; i++ {
		backoff := calculateBackoff(1, config)
		backoffs[backoff] = true

		// Verify backoff is within expected range
		base := 200 * time.Millisecond
		minBackoff := base - time.Duration(float64(base)*0.1)
		maxBackoff := base + time.Duration(float64(base)*0.1)

		if backoff < minBackoff || backoff > maxBackoff {
			t.Errorf("backoff %v outside expected range [%v, %v]", backoff, minBackoff, maxBackoff)
		}
	}

	// With jitter, we should see some variation (not all the same)
	if len(backoffs) == 1 {
		t.Error("expected jitter to produce variation in backoff values")
	}
}

func TestWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	config := DefaultRetryConfig()
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		return nil
	}

	err := WithRetry(ctx, config, operation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestWithRetry_SuccessOnSecondAttempt(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  1 * time.Millisecond,
		MaxBackoff:      10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 2 {
			return NewElementNotFoundError("button", 5000)
		}
		return nil
	}

	err := WithRetry(ctx, config, operation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestWithRetry_SuccessOnFinalAttempt(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  1 * time.Millisecond,
		MaxBackoff:      10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return NewElementNotFoundError("button", 5000)
		}
		return nil
	}

	err := WithRetry(ctx, config, operation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetry_ExhaustedAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  1 * time.Millisecond,
		MaxBackoff:      10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx := context.Background()

	attempts := 0
	expectedErr := NewElementNotFoundError("button", 5000)
	operation := func() error {
		attempts++
		return expectedErr
	}

	err := WithRetry(ctx, config, operation)
	if err == nil {
		t.Error("expected error after exhausting retries")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	// Check that error is wrapped in RetryExhaustedError
	var retryErr *RetryExhaustedError
	if !errors.As(err, &retryErr) {
		t.Errorf("expected error to be RetryExhaustedError, got %T", err)
	} else {
		if retryErr.Attempts != 3 {
			t.Errorf("expected RetryExhaustedError.Attempts to be 3, got %d", retryErr.Attempts)
		}
		if !errors.Is(retryErr.Err, expectedErr) {
			t.Errorf("expected wrapped error to be %v, got %v", expectedErr, retryErr.Err)
		}
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	config := DefaultRetryConfig()
	ctx := context.Background()

	attempts := 0
	expectedErr := NewNavigationError("https://example.com", "404 not found")
	operation := func() error {
		attempts++
		return expectedErr
	}

	err := WithRetry(ctx, config, operation)
	if err == nil {
		t.Error("expected error to be returned")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestWithRetry_ContextCancelledBeforeFirstAttempt(t *testing.T) {
	config := DefaultRetryConfig()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	attempts := 0
	operation := func() error {
		attempts++
		return NewElementNotFoundError("button", 5000)
	}

	err := WithRetry(ctx, config, operation)
	if err == nil {
		t.Error("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
	if attempts != 0 {
		t.Errorf("expected 0 attempts, got %d", attempts)
	}
}

func TestWithRetry_ContextCancelledBetweenAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      1 * time.Second,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	operation := func() error {
		attempts++
		if attempts == 1 {
			// Cancel context after first attempt
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
		}
		return NewElementNotFoundError("button", 5000)
	}

	err := WithRetry(ctx, config, operation)
	if err == nil {
		t.Error("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt before cancellation, got %d", attempts)
	}
}

func TestWithRetry_NilConfig(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 2 {
			return NewElementNotFoundError("button", 5000)
		}
		return nil
	}

	err := WithRetry(ctx, nil, operation)
	if err != nil {
		t.Errorf("expected no error with nil config (should use defaults), got %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestWithRetry_MaxAttempts1(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     1,
		InitialBackoff:  1 * time.Millisecond,
		MaxBackoff:      10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		return NewElementNotFoundError("button", 5000)
	}

	err := WithRetry(ctx, config, operation)
	if err == nil {
		t.Error("expected error with MaxAttempts=1")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt with MaxAttempts=1, got %d", attempts)
	}
}

// TestWithRetry_MaxAttempts0 removed - MaxAttempts=0 is no longer valid.
// Configuration validation now requires MaxAttempts >= 1.

func TestRetryableOperation_SuccessOnFirstAttempt(t *testing.T) {
	config := DefaultRetryConfig()
	ctx := context.Background()

	attempts := 0
	operation := func() (string, error) {
		attempts++
		return "success", nil
	}

	result, err := RetryableOperation(ctx, config, operation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected result to be 'success', got %q", result)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetryableOperation_SuccessOnSecondAttempt(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  1 * time.Millisecond,
		MaxBackoff:      10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx := context.Background()

	attempts := 0
	operation := func() (int, error) {
		attempts++
		if attempts < 2 {
			return 0, NewElementNotFoundError("button", 5000)
		}
		return 42, nil
	}

	result, err := RetryableOperation(ctx, config, operation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("expected result to be 42, got %d", result)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryableOperation_ExhaustedAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  1 * time.Millisecond,
		MaxBackoff:      10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx := context.Background()

	attempts := 0
	expectedErr := NewElementNotFoundError("button", 5000)
	operation := func() (string, error) {
		attempts++
		return "", expectedErr
	}

	result, err := RetryableOperation(ctx, config, operation)
	if err == nil {
		t.Error("expected error after exhausting retries")
	}
	if result != "" {
		t.Errorf("expected empty result on error, got %q", result)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	// Check that error is wrapped in RetryExhaustedError
	var retryErr *RetryExhaustedError
	if !errors.As(err, &retryErr) {
		t.Errorf("expected error to be RetryExhaustedError, got %T", err)
	} else {
		if retryErr.Attempts != 3 {
			t.Errorf("expected RetryExhaustedError.Attempts to be 3, got %d", retryErr.Attempts)
		}
		if !errors.Is(retryErr.Err, expectedErr) {
			t.Errorf("expected wrapped error to be %v, got %v", expectedErr, retryErr.Err)
		}
	}
}

func TestRetryableOperation_NonRetryableError(t *testing.T) {
	config := DefaultRetryConfig()
	ctx := context.Background()

	attempts := 0
	operation := func() (bool, error) {
		attempts++
		return false, NewNavigationError("https://example.com", "404 not found")
	}

	result, err := RetryableOperation(ctx, config, operation)
	if err == nil {
		t.Error("expected error to be returned")
	}
	if result != false {
		t.Errorf("expected false result, got %v", result)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestRetryableOperation_ContextCancelled(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      1 * time.Second,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	operation := func() (int, error) {
		attempts++
		if attempts == 1 {
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
		}
		return 0, NewElementNotFoundError("button", 5000)
	}

	result, err := RetryableOperation(ctx, config, operation)
	if err == nil {
		t.Error("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
	if result != 0 {
		t.Errorf("expected zero value result, got %d", result)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt before cancellation, got %d", attempts)
	}
}

func TestRetryableOperation_NilConfig(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	operation := func() (string, error) {
		attempts++
		if attempts < 2 {
			return "", NewElementNotFoundError("button", 5000)
		}
		return "success", nil
	}

	result, err := RetryableOperation(ctx, nil, operation)
	if err != nil {
		t.Errorf("expected no error with nil config, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected result to be 'success', got %q", result)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []int
		val      int
		expected bool
	}{
		{"contains value", []int{1, 2, 3}, 2, true},
		{"does not contain value", []int{1, 2, 3}, 4, false},
		{"empty slice", []int{}, 1, false},
		{"single value match", []int{5}, 5, true},
		{"single value no match", []int{5}, 6, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.val)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestWithRetry_TimingBetweenRetries(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  50 * time.Millisecond,
		MaxBackoff:      1 * time.Second,
		BackoffFactor:   2.0,
		RetryableErrors: []int{CodeElementNotFound},
		Jitter:          0.0,
	}
	ctx := context.Background()

	attempts := 0
	attemptTimes := []time.Time{}
	operation := func() error {
		attempts++
		attemptTimes = append(attemptTimes, time.Now())
		return NewElementNotFoundError("button", 5000)
	}

	WithRetry(ctx, config, operation)

	if len(attemptTimes) != 3 {
		t.Fatalf("expected 3 attempts, got %d", len(attemptTimes))
	}

	// Check delay between attempt 0 and 1 (should be ~50ms)
	delay1 := attemptTimes[1].Sub(attemptTimes[0])
	if delay1 < 40*time.Millisecond || delay1 > 60*time.Millisecond {
		t.Errorf("expected first retry delay to be ~50ms, got %v", delay1)
	}

	// Check delay between attempt 1 and 2 (should be ~100ms)
	delay2 := attemptTimes[2].Sub(attemptTimes[1])
	if delay2 < 90*time.Millisecond || delay2 > 110*time.Millisecond {
		t.Errorf("expected second retry delay to be ~100ms, got %v", delay2)
	}
}

func TestRetryExhaustedError_ErrorMessage(t *testing.T) {
	innerErr := NewElementNotFoundError("button", 5000)

	tests := []struct {
		name     string
		attempts int
		want     string
	}{
		{
			name:     "single attempt",
			attempts: 1,
			want:     "operation failed after 1 attempt:",
		},
		{
			name:     "multiple attempts",
			attempts: 3,
			want:     "operation failed after 3 attempts:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RetryExhaustedError{
				Err:      innerErr,
				Attempts: tt.attempts,
			}

			msg := err.Error()
			if !errors.Is(err, innerErr) {
				t.Error("RetryExhaustedError should wrap inner error")
			}
			if len(msg) < len(tt.want) || msg[:len(tt.want)] != tt.want {
				t.Errorf("expected error message to start with %q, got %q", tt.want, msg)
			}
		})
	}
}

func TestRetryExhaustedError_Unwrap(t *testing.T) {
	innerErr := NewElementNotFoundError("button", 5000)
	err := &RetryExhaustedError{
		Err:      innerErr,
		Attempts: 3,
	}

	unwrapped := errors.Unwrap(err)
	if unwrapped != innerErr {
		t.Errorf("expected unwrapped error to be %v, got %v", innerErr, unwrapped)
	}
}

func TestRetryExhaustedError_ImplementsMCPError(t *testing.T) {
	innerErr := NewElementNotFoundError("button", 5000)
	err := &RetryExhaustedError{
		Err:      innerErr,
		Attempts: 3,
	}

	// Should implement MCPError interface
	var _ MCPError = err

	// ErrorCode should delegate to inner error
	if err.ErrorCode() != CodeElementNotFound {
		t.Errorf("expected error code %d, got %d", CodeElementNotFound, err.ErrorCode())
	}

	// ErrorData should include attempts
	data := err.ErrorData()
	if attempts, ok := data["attempts"].(int); !ok || attempts != 3 {
		t.Errorf("expected ErrorData to include attempts=3, got %v", data["attempts"])
	}

	// ErrorData should merge inner error data
	if selector, ok := data["selector"].(string); !ok || selector != "button" {
		t.Errorf("expected ErrorData to include selector from inner error, got %v", data["selector"])
	}
}

func TestRetryExhaustedError_WithNonMCPError(t *testing.T) {
	innerErr := errors.New("generic error")
	err := &RetryExhaustedError{
		Err:      innerErr,
		Attempts: 2,
	}

	// ErrorCode should return internal error for non-MCP errors
	if err.ErrorCode() != CodeInternalError {
		t.Errorf("expected error code %d for non-MCP error, got %d", CodeInternalError, err.ErrorCode())
	}

	// ErrorData should still include attempts
	data := err.ErrorData()
	if attempts, ok := data["attempts"].(int); !ok || attempts != 2 {
		t.Errorf("expected ErrorData to include attempts=2, got %v", data["attempts"])
	}
}

func TestCalculateBackoff_NoNegativeDelays(t *testing.T) {
	// Test with very small backoff and high jitter to ensure it never goes negative
	config := &RetryConfig{
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		BackoffFactor:  2.0,
		Jitter:         1.0, // Maximum jitter (100%)
	}

	// Run many iterations to catch edge cases with random jitter
	for i := 0; i < 1000; i++ {
		for attempt := 0; attempt < 10; attempt++ {
			backoff := calculateBackoff(attempt, config)
			if backoff < 0 {
				t.Errorf("calculateBackoff produced negative delay: %v (attempt %d, iteration %d)", backoff, attempt, i)
			}
		}
	}
}

func TestCalculateBackoff_ZeroInitialBackoff(t *testing.T) {
	config := &RetryConfig{
		InitialBackoff: 0,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.1,
	}

	backoff := calculateBackoff(0, config)
	if backoff < 0 {
		t.Errorf("calculateBackoff with zero InitialBackoff produced negative delay: %v", backoff)
	}
}
