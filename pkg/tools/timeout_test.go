package tools

import (
	"context"
	"testing"
	"time"

	pkgerrors "github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
)

func TestDefaultTimeoutConfig(t *testing.T) {
	config := DefaultTimeoutConfig()

	if config.Default != 30*time.Second {
		t.Errorf("Expected Default timeout 30s, got %v", config.Default)
	}
	if config.Navigation != 30*time.Second {
		t.Errorf("Expected Navigation timeout 30s, got %v", config.Navigation)
	}
	if config.Element != 5*time.Second {
		t.Errorf("Expected Element timeout 5s, got %v", config.Element)
	}
	if config.Script != 30*time.Second {
		t.Errorf("Expected Script timeout 30s, got %v", config.Script)
	}
}

func TestGetCategoryTimeout(t *testing.T) {
	config := &TimeoutConfig{
		Default:    10 * time.Second,
		Navigation: 20 * time.Second,
		Element:    5 * time.Second,
		Script:     15 * time.Second,
	}

	tests := []struct {
		name     string
		category TimeoutCategory
		expected time.Duration
	}{
		{"navigation category", TimeoutNavigation, 20 * time.Second},
		{"element category", TimeoutElement, 5 * time.Second},
		{"script category", TimeoutScript, 15 * time.Second},
		{"default category", TimeoutDefault, 10 * time.Second},
		{"unknown category", TimeoutCategory("unknown"), 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetCategoryTimeout(tt.category)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetTimeout(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]any
		defaultMs  int
		expectedMs int
	}{
		{
			name:       "timeout present and valid",
			args:       map[string]any{"timeout": float64(5000)},
			defaultMs:  3000,
			expectedMs: 5000,
		},
		{
			name:       "timeout not present",
			args:       map[string]any{},
			defaultMs:  3000,
			expectedMs: 3000,
		},
		{
			name:       "timeout is zero",
			args:       map[string]any{"timeout": float64(0)},
			defaultMs:  3000,
			expectedMs: 3000,
		},
		{
			name:       "timeout is negative",
			args:       map[string]any{"timeout": float64(-100)},
			defaultMs:  3000,
			expectedMs: 3000,
		},
		{
			name:       "timeout is wrong type",
			args:       map[string]any{"timeout": "not a number"},
			defaultMs:  3000,
			expectedMs: 3000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTimeout(tt.args, tt.defaultMs)
			expected := time.Duration(tt.expectedMs) * time.Millisecond
			if result != expected {
				t.Errorf("Expected %v, got %v", expected, result)
			}
		})
	}
}

func TestGetTimeoutPtr(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		expectNil   bool
		expectedVal int
	}{
		{
			name:        "timeout present and valid",
			args:        map[string]any{"timeout": float64(5000)},
			expectNil:   false,
			expectedVal: 5000,
		},
		{
			name:      "timeout not present",
			args:      map[string]any{},
			expectNil: true,
		},
		{
			name:      "timeout is zero",
			args:      map[string]any{"timeout": float64(0)},
			expectNil: true,
		},
		{
			name:      "timeout is negative",
			args:      map[string]any{"timeout": float64(-100)},
			expectNil: true,
		},
		{
			name:      "timeout is wrong type",
			args:      map[string]any{"timeout": "not a number"},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTimeoutPtr(tt.args)
			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil, got %v", *result)
				}
			} else {
				if result == nil {
					t.Errorf("Expected %d, got nil", tt.expectedVal)
				} else if *result != tt.expectedVal {
					t.Errorf("Expected %d, got %d", tt.expectedVal, *result)
				}
			}
		})
	}
}

func TestWithTimeout(t *testing.T) {
	config := &TimeoutConfig{
		Default:    10 * time.Second,
		Navigation: 20 * time.Second,
		Element:    5 * time.Second,
		Script:     15 * time.Second,
	}

	t.Run("creates context with configured timeout", func(t *testing.T) {
		ctx := context.Background()
		newCtx, cancel := WithTimeout(ctx, config, TimeoutNavigation, nil)
		defer cancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Fatal("Expected context to have deadline")
		}

		remaining := time.Until(deadline)
		// Check that remaining time is approximately 20 seconds (allow some tolerance)
		if remaining < 19*time.Second || remaining > 21*time.Second {
			t.Errorf("Expected remaining time ~20s, got %v", remaining)
		}
	})

	t.Run("uses override when provided", func(t *testing.T) {
		ctx := context.Background()
		override := 8000 // 8 seconds in ms
		newCtx, cancel := WithTimeout(ctx, config, TimeoutNavigation, &override)
		defer cancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Fatal("Expected context to have deadline")
		}

		remaining := time.Until(deadline)
		// Check that remaining time is approximately 8 seconds
		if remaining < 7*time.Second || remaining > 9*time.Second {
			t.Errorf("Expected remaining time ~8s, got %v", remaining)
		}
	})

	t.Run("uses existing deadline when earlier", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		newCtx, newCancel := WithTimeout(ctx, config, TimeoutNavigation, nil)
		defer newCancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Fatal("Expected context to have deadline")
		}

		// The existing 2-second deadline should be preserved
		remaining := time.Until(deadline)
		if remaining > 3*time.Second {
			t.Errorf("Expected existing deadline to be preserved, got %v remaining", remaining)
		}
	})

	t.Run("uses new timeout when existing deadline is later", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		newCtx, newCancel := WithTimeout(ctx, config, TimeoutElement, nil)
		defer newCancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Fatal("Expected context to have deadline")
		}

		// The new 5-second timeout should be used
		remaining := time.Until(deadline)
		if remaining < 4*time.Second || remaining > 6*time.Second {
			t.Errorf("Expected new 5s deadline, got %v remaining", remaining)
		}
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		ctx := context.Background()
		newCtx, cancel := WithTimeout(ctx, nil, TimeoutNavigation, nil)
		defer cancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Fatal("Expected context to have deadline")
		}

		remaining := time.Until(deadline)
		// Should use default navigation timeout of 30s
		if remaining < 29*time.Second || remaining > 31*time.Second {
			t.Errorf("Expected default navigation timeout ~30s, got %v", remaining)
		}
	})

	t.Run("cancel function releases resources", func(t *testing.T) {
		ctx := context.Background()
		newCtx, cancel := WithTimeout(ctx, config, TimeoutElement, nil)

		// Cancel immediately
		cancel()

		// Check that context is cancelled
		select {
		case <-newCtx.Done():
			// Expected - context is cancelled
		case <-time.After(100 * time.Millisecond):
			t.Error("Expected context to be cancelled")
		}
	})
}

func TestHandleContextError(t *testing.T) {
	t.Run("returns TimeoutError for DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Wait for context to expire
		time.Sleep(10 * time.Millisecond)

		err := HandleContextError(ctx, "test_operation")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		timeoutErr, ok := err.(*pkgerrors.TimeoutError)
		if !ok {
			t.Fatalf("Expected TimeoutError, got %T", err)
		}

		if timeoutErr.Operation != "test_operation" {
			t.Errorf("Expected operation 'test_operation', got %s", timeoutErr.Operation)
		}
	})

	t.Run("returns TimeoutError for Canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := HandleContextError(ctx, "test_operation")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		timeoutErr, ok := err.(*pkgerrors.TimeoutError)
		if !ok {
			t.Fatalf("Expected TimeoutError, got %T", err)
		}

		if timeoutErr.Operation != "test_operation" {
			t.Errorf("Expected operation 'test_operation', got %s", timeoutErr.Operation)
		}
	})

	t.Run("returns nil when context has no error", func(t *testing.T) {
		ctx := context.Background()
		err := HandleContextError(ctx, "test_operation")
		if err != nil {
			t.Errorf("Expected nil, got %v", err)
		}
	})
}

func TestApplyTimeout(t *testing.T) {
	config := &TimeoutConfig{
		Default:    10 * time.Second,
		Navigation: 20 * time.Second,
		Element:    5 * time.Second,
		Script:     15 * time.Second,
	}

	t.Run("extracts timeout and applies it", func(t *testing.T) {
		ctx := context.Background()
		args := map[string]any{"timeout": float64(3000)}

		newCtx, cancel := ApplyTimeout(ctx, args, TimeoutNavigation, config)
		defer cancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Fatal("Expected context to have deadline")
		}

		remaining := time.Until(deadline)
		// Should use the 3-second override from args
		if remaining < 2*time.Second || remaining > 4*time.Second {
			t.Errorf("Expected ~3s timeout from args, got %v", remaining)
		}
	})

	t.Run("uses category default when no override in args", func(t *testing.T) {
		ctx := context.Background()
		args := map[string]any{}

		newCtx, cancel := ApplyTimeout(ctx, args, TimeoutElement, config)
		defer cancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Fatal("Expected context to have deadline")
		}

		remaining := time.Until(deadline)
		// Should use element timeout of 5s
		if remaining < 4*time.Second || remaining > 6*time.Second {
			t.Errorf("Expected ~5s element timeout, got %v", remaining)
		}
	})
}

