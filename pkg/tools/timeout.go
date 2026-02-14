// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"context"
	"time"

	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
)

// TimeoutCategory represents different categories of timeouts.
type TimeoutCategory string

const (
	TimeoutDefault    TimeoutCategory = "default"
	TimeoutNavigation TimeoutCategory = "navigation"
	TimeoutElement    TimeoutCategory = "element"
	TimeoutScript     TimeoutCategory = "script"
)

// TimeoutConfig holds timeout settings for different operation categories.
type TimeoutConfig struct {
	// Default timeout for general operations
	Default time.Duration

	// Navigation timeout for page loads
	Navigation time.Duration

	// Element timeout for waiting on elements
	Element time.Duration

	// Script timeout for JavaScript execution
	Script time.Duration
}

// DefaultTimeoutConfig returns a TimeoutConfig with sensible defaults.
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		Default:    30 * time.Second,
		Navigation: 30 * time.Second,
		Element:    5 * time.Second,
		Script:     30 * time.Second,
	}
}

// GetCategoryTimeout returns the timeout for a specific category.
func (c *TimeoutConfig) GetCategoryTimeout(category TimeoutCategory) time.Duration {
	switch category {
	case TimeoutNavigation:
		return c.Navigation
	case TimeoutElement:
		return c.Element
	case TimeoutScript:
		return c.Script
	default:
		return c.Default
	}
}

// WithTimeout creates a context with a timeout, using the earlier of:
// - The existing context deadline (if any)
// - The configured timeout for the category
// - The override value (if provided and > 0)
//
// Always call the returned cancel function to release resources.
func WithTimeout(ctx context.Context, config *TimeoutConfig, category TimeoutCategory, overrideMs *int) (context.Context, context.CancelFunc) {
	if config == nil {
		config = DefaultTimeoutConfig()
	}

	// Determine the timeout to use
	timeout := config.GetCategoryTimeout(category)

	// Apply override if provided
	if overrideMs != nil && *overrideMs > 0 {
		timeout = time.Duration(*overrideMs) * time.Millisecond
	}

	// Check if context already has an earlier deadline
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < timeout {
			// Existing deadline is earlier, just return a cancel function
			// that doesn't change the deadline
			return context.WithCancel(ctx)
		}
	}

	return context.WithTimeout(ctx, timeout)
}

// GetTimeout extracts a timeout value from tool arguments.
// Returns the argument value if present and valid, otherwise returns default.
func GetTimeout(args map[string]any, defaultMs int) time.Duration {
	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		return time.Duration(timeout) * time.Millisecond
	}
	return time.Duration(defaultMs) * time.Millisecond
}

// GetTimeoutPtr extracts a timeout value from tool arguments as a pointer.
// Returns nil if timeout argument is not present.
// Useful for optional override detection.
func GetTimeoutPtr(args map[string]any) *int {
	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		ms := int(timeout)
		return &ms
	}
	return nil
}

// HandleContextError converts context errors to appropriate MCP errors.
// - context.DeadlineExceeded -> TimeoutError
// - context.Canceled -> TimeoutError (treat cancellation as timeout from client perspective)
// Returns nil if the context error is nil.
func HandleContextError(ctx context.Context, operation string, timeoutMs int) error {
	err := ctx.Err()
	if err == nil {
		return nil
	}

	switch err {
	case context.DeadlineExceeded:
		return errors.NewTimeoutError(operation, timeoutMs)
	case context.Canceled:
		// Client cancellation - report as timeout with context
		return &errors.TimeoutError{
			Operation: operation,
			Timeout:   timeoutMs,
			Err:       err,
		}
	default:
		return err
	}
}

// ApplyTimeout is a convenience function for tools to apply timeout handling.
// It extracts the timeout from args, applies the category default, and returns
// a context with the appropriate deadline.
//
// Usage in tool handler:
//
//	ctx, cancel := ApplyTimeout(ctx, req.Params.Arguments, tools.TimeoutNavigation, config)
//	defer cancel()
func ApplyTimeout(ctx context.Context, args map[string]any, category TimeoutCategory, config *TimeoutConfig) (context.Context, context.CancelFunc) {
	overrideMs := GetTimeoutPtr(args)
	return WithTimeout(ctx, config, category, overrideMs)
}

// MustComplete wraps an operation with timeout handling and error conversion.
// If the context deadline is exceeded or cancelled, returns a TimeoutError.
// Otherwise returns the operation's error (or nil on success).
func MustComplete(ctx context.Context, operation string, timeoutMs int, fn func() error) error {
	// Run the operation
	err := fn()

	// Check if context was cancelled/timed out
	if ctxErr := HandleContextError(ctx, operation, timeoutMs); ctxErr != nil {
		// If operation also returned an error, wrap it
		if err != nil {
			return &errors.TimeoutError{
				Operation: operation,
				Timeout:   timeoutMs,
				Err:       err,
			}
		}
		return ctxErr
	}

	return err
}
