package shutdown

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
)

const defaultEmergencyTimeout = 10 * time.Second

// EmergencyCleanup represents resources that can be cleaned up on panic.
// It attempts to clean up browser resources when the application panics.
type EmergencyCleanup struct {
	coordinator *Coordinator
	browserPool browser.BrowserPool
	timeout     time.Duration
}

// NewEmergencyCleanup creates an emergency cleanup handler.
// It accepts either a coordinator (preferred) or a direct browser pool reference.
// If a coordinator is provided, it uses the full shutdown sequence.
// Otherwise, it directly stops the browser pool.
func NewEmergencyCleanup(coordinator *Coordinator, browserPool browser.BrowserPool) *EmergencyCleanup {
	return &EmergencyCleanup{
		coordinator: coordinator,
		browserPool: browserPool,
		timeout:     defaultEmergencyTimeout,
	}
}

// WithTimeout sets a custom timeout for emergency cleanup.
func (e *EmergencyCleanup) WithTimeout(timeout time.Duration) *EmergencyCleanup {
	e.timeout = timeout
	return e
}

// Execute performs best-effort resource cleanup.
// This should be called from a defer/recover block when a panic is caught.
func (e *EmergencyCleanup) Execute(panicValue any) {
	stack := debug.Stack()

	slog.Error("panic in main goroutine, executing emergency cleanup",
		"panic", panicValue,
		"stack", string(stack),
	)

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	// Prefer using the coordinator if available (handles full shutdown sequence)
	if e.coordinator != nil {
		slog.Info("emergency cleanup: using shutdown coordinator")
		if err := e.coordinator.Shutdown(ctx); err != nil {
			slog.Error("emergency cleanup: coordinator shutdown failed", "error", err)
		} else {
			slog.Info("emergency cleanup: coordinator shutdown complete")
		}
		return
	}

	// Fallback: directly stop the browser pool
	if e.browserPool != nil {
		slog.Info("emergency cleanup: stopping browser pool directly")
		if err := e.browserPool.Stop(ctx); err != nil {
			slog.Error("emergency cleanup: browser pool stop failed", "error", err)
		} else {
			slog.Info("emergency cleanup: browser pool stopped")
		}
		return
	}

	slog.Warn("emergency cleanup: no resources available for cleanup")
}

// RecoverAndCleanup is a helper that can be used in defer statements.
// It recovers from a panic and executes cleanup if a panic occurred.
// Example: defer cleanup.RecoverAndCleanup()
func (e *EmergencyCleanup) RecoverAndCleanup() bool {
	if r := recover(); r != nil {
		e.Execute(r)
		return true
	}
	return false
}
