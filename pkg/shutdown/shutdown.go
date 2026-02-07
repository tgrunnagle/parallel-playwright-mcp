package shutdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// Coordinator orchestrates the ordered shutdown sequence.
// It manages the phased shutdown of HTTP server, browser sessions, and browser pool.
type Coordinator struct {
	config         Config
	httpServer     *http.Server
	sessionMgr     session.BrowserSessionManager
	browserPool    browser.BrowserPool
	requestTracker *RequestTracker

	mu          sync.RWMutex
	mcpSessions []string
	shutdownErr error
	shutdown    bool
}

// NewCoordinator creates a shutdown coordinator with the given dependencies.
// If requestTracker is nil, a new tracker will be created.
func NewCoordinator(
	config Config,
	httpServer *http.Server,
	sessionMgr session.BrowserSessionManager,
	browserPool browser.BrowserPool,
	requestTracker *RequestTracker,
) *Coordinator {
	if requestTracker == nil {
		requestTracker = NewRequestTracker()
	}
	return &Coordinator{
		config:         config,
		httpServer:     httpServer,
		sessionMgr:     sessionMgr,
		browserPool:    browserPool,
		requestTracker: requestTracker,
		mcpSessions:    make([]string, 0),
	}
}

// RegisterMCPSession adds an MCP session ID for tracking.
// Called when a new MCP connection is established.
func (c *Coordinator) RegisterMCPSession(mcpSessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mcpSessions = append(c.mcpSessions, mcpSessionID)
	slog.Debug("registered MCP session for shutdown tracking", "mcpSessionID", mcpSessionID)
}

// UnregisterMCPSession removes an MCP session ID from tracking.
// Called when an MCP connection is closed normally.
func (c *Coordinator) UnregisterMCPSession(mcpSessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, id := range c.mcpSessions {
		if id == mcpSessionID {
			c.mcpSessions = append(c.mcpSessions[:i], c.mcpSessions[i+1:]...)
			slog.Debug("unregistered MCP session from shutdown tracking", "mcpSessionID", mcpSessionID)
			return
		}
	}
}

// ActiveMCPSessionCount returns the number of tracked MCP sessions.
func (c *Coordinator) ActiveMCPSessionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.mcpSessions)
}

// RequestTracker returns the request tracker for middleware access.
func (c *Coordinator) RequestTracker() *RequestTracker {
	return c.requestTracker
}

// Shutdown executes the ordered shutdown sequence with connection draining.
// It proceeds through phases in order, continuing even if errors occur.
// Returns an aggregated error if any phase encountered errors.
func (c *Coordinator) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if c.shutdown {
		err := c.shutdownErr
		c.mu.Unlock()
		return err
	}
	c.shutdown = true
	c.mu.Unlock()

	shutdownCtx, cancel := context.WithTimeout(ctx, c.config.TotalTimeout)
	defer cancel()

	var errs []error

	// Phase 1: Drain HTTP connections
	slog.Info("shutdown: phase 1 - draining HTTP connections",
		"activeRequests", c.requestTracker.ActiveCount(),
		"drainTimeout", c.config.DrainTimeout,
	)

	drainStart := time.Now()
	if err := c.drainHTTPConnections(shutdownCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			slog.Warn("shutdown: drain timeout exceeded, forcing connection close",
				"remainingRequests", c.requestTracker.ActiveCount(),
				"elapsed", time.Since(drainStart),
			)
		} else {
			slog.Error("shutdown: HTTP drain failed", "error", err)
		}
		errs = append(errs, fmt.Errorf("phase 1 (drain): %w", err))
	} else {
		slog.Info("shutdown: phase 1 complete - HTTP connections drained",
			"elapsed", time.Since(drainStart),
		)
	}

	// Phase 2: Close all browser sessions
	slog.Info("shutdown: phase 2 - closing browser sessions")
	if err := c.closeBrowserSessions(shutdownCtx); err != nil {
		slog.Error("shutdown: browser session closure failed", "error", err)
		errs = append(errs, fmt.Errorf("phase 2 (sessions): %w", err))
	} else {
		slog.Info("shutdown: phase 2 complete - browser sessions closed")
	}

	// Phase 3: Stop browser pool
	slog.Info("shutdown: phase 3 - stopping browser pool")
	if err := c.stopBrowserPool(shutdownCtx); err != nil {
		slog.Error("shutdown: browser pool stop failed", "error", err)
		errs = append(errs, fmt.Errorf("phase 3 (browser pool): %w", err))
	} else {
		slog.Info("shutdown: phase 3 complete - browser pool stopped")
	}

	slog.Info("shutdown: sequence complete")

	c.mu.Lock()
	if len(errs) > 0 {
		c.shutdownErr = errors.Join(errs...)
	}
	c.mu.Unlock()

	return c.shutdownErr
}

// drainHTTPConnections stops accepting new connections and waits for
// in-flight requests to complete within the drain timeout.
func (c *Coordinator) drainHTTPConnections(ctx context.Context) error {
	if c.httpServer == nil {
		slog.Info("shutdown: no HTTP server to drain")
		return nil
	}

	drainCtx, cancel := context.WithTimeout(ctx, c.config.DrainTimeout)
	defer cancel()

	// http.Server.Shutdown() stops accepting new connections immediately
	// and waits for existing connections to complete (or context deadline)
	err := c.httpServer.Shutdown(drainCtx)

	// Log final request count
	remaining := c.requestTracker.ActiveCount()
	if remaining > 0 {
		slog.Warn("shutdown: drain completed with requests still active",
			"remainingRequests", remaining,
		)
	}

	return err
}

// closeBrowserSessions closes all browser sessions using the session manager's CloseAll method.
// This ensures all sessions are closed regardless of MCP session tracking.
func (c *Coordinator) closeBrowserSessions(ctx context.Context) error {
	if c.sessionMgr == nil {
		slog.Info("shutdown: no session manager to clean up")
		return nil
	}

	phaseCtx, cancel := context.WithTimeout(ctx, c.config.PhaseTimeout)
	defer cancel()

	slog.Info("shutdown: closing all browser sessions")
	return c.sessionMgr.CloseAll(phaseCtx)
}

// stopBrowserPool stops the browser pool with phase timeout.
func (c *Coordinator) stopBrowserPool(ctx context.Context) error {
	if c.browserPool == nil {
		slog.Info("shutdown: no browser pool to stop")
		return nil
	}

	phaseCtx, cancel := context.WithTimeout(ctx, c.config.PhaseTimeout)
	defer cancel()

	return c.browserPool.Stop(phaseCtx)
}
