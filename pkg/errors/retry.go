// Package errors provides retry logic for transient failures with exponential backoff.
package errors

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configures retry behavior for transient failures.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including initial attempt).
	// Default: 3 (1 initial + 2 retries)
	MaxAttempts int

	// InitialBackoff is the delay before the first retry.
	// Default: 100ms
	InitialBackoff time.Duration

	// MaxBackoff is the maximum delay between retries.
	// Default: 5s
	MaxBackoff time.Duration

	// BackoffFactor is the multiplier applied to backoff after each attempt.
	// Default: 2.0
	BackoffFactor float64

	// RetryableErrors is the list of error codes that should trigger retry.
	// Default: [-32002 (ElementNotFound), -32003 (Timeout)]
	RetryableErrors []int

	// Jitter adds randomization to backoff delays (0.0 to 1.0).
	// Default: 0.1 (10% jitter)
	Jitter float64
}

// DefaultRetryConfig returns a RetryConfig with sensible defaults.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		BackoffFactor:  2.0,
		RetryableErrors: []int{
			CodeElementNotFound, // -32002
			CodeTimeout,         // -32003
		},
		Jitter: 0.1,
	}
}

// IsRetryable checks if an error should be retried based on config.
func IsRetryable(err error, config *RetryConfig) bool {
	if err == nil || config == nil {
		return false
	}

	// Check if error implements MCPError interface
	if mcpErr, ok := err.(MCPError); ok {
		code := mcpErr.ErrorCode()
		for _, retryableCode := range config.RetryableErrors {
			if code == retryableCode {
				return true
			}
		}
	}

	// Also check for wrapped custom error types using errors.As
	var elementErr *ElementNotFoundError
	if errors.As(err, &elementErr) {
		return contains(config.RetryableErrors, CodeElementNotFound)
	}

	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return contains(config.RetryableErrors, CodeTimeout)
	}

	return false
}

// WithRetry wraps an operation with retry logic.
// Returns the last error if all attempts fail.
func WithRetry(ctx context.Context, config *RetryConfig, operation func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check context before attempting
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Execute operation
		lastErr = operation()
		if lastErr == nil {
			return nil // Success
		}

		// Check if error is retryable
		if !IsRetryable(lastErr, config) {
			return lastErr // Non-retryable error, return immediately
		}

		// Don't sleep after the last attempt
		if attempt < config.MaxAttempts-1 {
			backoff := calculateBackoff(attempt, config)
			select {
			case <-time.After(backoff):
				// Continue to next attempt
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return lastErr
}

// RetryableOperation wraps an operation that returns a value with retry logic.
func RetryableOperation[T any](ctx context.Context, config *RetryConfig, operation func() (T, error)) (T, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var zero T
	var lastErr error
	var result T

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}

		result, lastErr = operation()
		if lastErr == nil {
			return result, nil
		}

		if !IsRetryable(lastErr, config) {
			return zero, lastErr
		}

		if attempt < config.MaxAttempts-1 {
			backoff := calculateBackoff(attempt, config)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		}
	}

	return zero, lastErr
}

// calculateBackoff computes the backoff duration for a given attempt.
// Uses exponential backoff with optional jitter.
func calculateBackoff(attempt int, config *RetryConfig) time.Duration {
	// Exponential backoff: InitialBackoff * BackoffFactor^attempt
	backoff := float64(config.InitialBackoff) * math.Pow(config.BackoffFactor, float64(attempt))

	// Cap at MaxBackoff
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	// Add jitter
	if config.Jitter > 0 {
		jitter := backoff * config.Jitter * (rand.Float64()*2 - 1) // +/- jitter%
		backoff += jitter
	}

	return time.Duration(backoff)
}

// contains checks if a slice contains a value.
func contains(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
