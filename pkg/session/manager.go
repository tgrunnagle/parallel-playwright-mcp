// Package session implements the 1:N browser session management model.
package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/playwright-community/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
)

// Error definitions for session operations.
var (
	// ErrSessionNotFound indicates the browser session ID does not exist.
	ErrSessionNotFound = errors.New("session not found")
	// ErrUnauthorized indicates the MCP session does not own the browser session.
	ErrUnauthorized = errors.New("unauthorized: MCP session does not own this browser session")
)

// SessionOptions configures a new browser session.
type SessionOptions struct {
	// BrowserType specifies the browser engine to use.
	BrowserType browser.BrowserType
	// Headless is reserved for future use. In Playwright, headless mode is a
	// browser-level setting configured via pool options, not per-context.
	// This field is currently ignored.
	Headless bool
	// Viewport sets the browser viewport size. If nil, uses browser default.
	Viewport *browser.Viewport
	// UserAgent overrides the default user agent string.
	UserAgent string
	// Locale sets the browser locale (e.g., "en-US").
	Locale string
	// TimezoneID sets the browser timezone (e.g., "America/New_York").
	TimezoneID string
	// ExtraHeaders adds HTTP headers to all requests.
	ExtraHeaders map[string]string
	// Timeout sets the default timeout for operations in this session.
	Timeout time.Duration
}

// SessionInfo provides metadata about a browser session for listing.
type SessionInfo struct {
	// ID is the browser session identifier.
	ID string
	// BrowserType is the browser engine type.
	BrowserType browser.BrowserType
	// URL is the current page URL.
	URL string
	// Title is the current page title.
	Title string
	// CreatedAt is the session creation timestamp.
	CreatedAt time.Time
	// LastAccess is the timestamp of the last activity.
	LastAccess time.Time
}

// BrowserSessionManager manages browser sessions within MCP connections.
// It implements the 1:N model where one MCP connection can own multiple
// browser sessions, each with full isolation.
type BrowserSessionManager interface {
	// CreateSession creates a new browser session for an MCP connection.
	// The session is associated with the given mcpSessionID for ownership tracking.
	CreateSession(ctx context.Context, mcpSessionID string, opts SessionOptions) (*BrowserSession, error)

	// GetSession retrieves a browser session by ID.
	// Returns the session and true if found and owned by the given MCP session,
	// or nil and false otherwise.
	GetSession(mcpSessionID, browserSessionID string) (*BrowserSession, bool)

	// CloseSession terminates a specific browser session.
	// Returns an error if the session doesn't exist or isn't owned by the MCP session.
	CloseSession(ctx context.Context, mcpSessionID, browserSessionID string) error

	// ListSessions returns metadata for all browser sessions owned by an MCP connection.
	ListSessions(mcpSessionID string) []*SessionInfo

	// CloseAllForMCP closes all browser sessions when an MCP connection ends.
	// This should be called during MCP session cleanup.
	CloseAllForMCP(ctx context.Context, mcpSessionID string) error

	// CloseAll closes all browser sessions regardless of MCP connection.
	// This should be called during server shutdown to ensure all sessions are closed.
	CloseAll(ctx context.Context) error

	// Cleanup removes expired/orphaned sessions that exceed the idle timeout.
	// Returns the number of sessions cleaned up.
	Cleanup(ctx context.Context, idleTimeout time.Duration) (int, error)
}

// manager is the concrete implementation of BrowserSessionManager.
type manager struct {
	// pool provides browser context creation capabilities.
	pool browser.BrowserPool
	// sessions maps browser session ID to BrowserSession.
	sessions map[string]*BrowserSession
	// mcpSessions maps MCP session ID to list of owned browser session IDs.
	mcpSessions map[string][]string
	// mu protects sessions and mcpSessions maps.
	mu sync.RWMutex
}

// NewManager creates a new browser session manager.
// The pool parameter provides browser context creation capabilities.
func NewManager(pool browser.BrowserPool) BrowserSessionManager {
	return &manager{
		pool:        pool,
		sessions:    make(map[string]*BrowserSession),
		mcpSessions: make(map[string][]string),
	}
}

// CreateSession creates a new browser session for an MCP connection.
// It generates a unique session ID, creates an isolated browser context via the pool,
// and establishes MCP ownership tracking.
func (m *manager) CreateSession(ctx context.Context, mcpSessionID string, opts SessionOptions) (*BrowserSession, error) {
	slog.Debug("creating browser session", "mcpSessionID", mcpSessionID, "browserType", opts.BrowserType)

	// Apply default browser type if not specified
	browserType := opts.BrowserType
	if browserType == "" {
		browserType = browser.BrowserChromium
	}

	// Generate unique session ID with "sess-" prefix
	sessionID := "sess-" + uuid.New().String()

	// Translate SessionOptions to ContextOptions
	// Note: Headless is passed but currently ignored by browser.ContextOptions
	contextOpts := browser.ContextOptions{
		Headless:     opts.Headless,
		Viewport:     opts.Viewport,
		UserAgent:    opts.UserAgent,
		Locale:       opts.Locale,
		TimezoneID:   opts.TimezoneID,
		ExtraHeaders: opts.ExtraHeaders,
	}

	// Create browser context via pool (may trigger lazy browser launch)
	browserContext, err := m.pool.NewContext(ctx, browserType, contextOpts)
	if err != nil {
		slog.Error("failed to create browser context for session", "mcpSessionID", mcpSessionID, "browserType", browserType, "error", err)
		return nil, fmt.Errorf("failed to create browser context: %w", err)
	}

	// Create initial page in the context
	page, err := browserContext.NewPage()
	if err != nil {
		slog.Error("failed to create initial page", "sessionID", sessionID, "error", err)
		// Clean up context if page creation fails - use pool.CloseContext to maintain proper tracking
		_ = m.pool.CloseContext(ctx, browserContext)
		return nil, fmt.Errorf("failed to create initial page: %w", err)
	}

	// Generate tab ID for initial page
	initialTabID := "tab-" + uuid.New().String()

	// Initialize console log buffer
	consoleLogs := NewConsoleLogBuffer(DefaultConsoleLogBufferSize)

	// Initialize network log buffer
	networkLogs := NewNetworkLogBuffer(DefaultNetworkLogBufferSize)

	// Attach console handler to initial page
	attachConsoleHandler(page, consoleLogs)

	// Setup network logging for initial page
	networkCleanup := SetupNetworkLogging(page, networkLogs)

	// Create the browser session
	now := time.Now()
	session := &BrowserSession{
		ID:              sessionID,
		MCPSessionID:    mcpSessionID,
		BrowserType:     browserType,
		Context:         browserContext,
		Pages:           map[string]playwright.Page{initialTabID: page},
		ActiveTabID:     initialTabID,
		ConsoleLogs:     consoleLogs,
		NetworkLogs:     networkLogs,
		CreatedAt:       now,
		LastAccess:      now,
		Metadata:        make(map[string]any),
		networkCleanups: map[string]func(){initialTabID: networkCleanup},
	}

	// Store session with write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Store session in sessions map
	m.sessions[sessionID] = session

	// Update MCP session ownership index
	m.mcpSessions[mcpSessionID] = append(m.mcpSessions[mcpSessionID], sessionID)

	slog.Info("browser session created", "sessionID", sessionID, "mcpSessionID", mcpSessionID, "browserType", browserType)
	return session, nil
}

// GetSession retrieves a browser session by ID.
// Returns the session and true if found and owned by the given MCP session,
// or nil and false otherwise. This method validates MCP ownership to ensure
// sessions cannot be accessed by unauthorized MCP connections.
func (m *manager) GetSession(mcpSessionID, browserSessionID string) (*BrowserSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Look up session by browser session ID
	session, exists := m.sessions[browserSessionID]
	if !exists {
		slog.Debug("session not found", "sessionID", browserSessionID, "mcpSessionID", mcpSessionID)
		return nil, false
	}

	// Validate MCP session ownership
	if session.MCPSessionID != mcpSessionID {
		slog.Warn("unauthorized session access attempt", "sessionID", browserSessionID, "mcpSessionID", mcpSessionID, "ownerMCPSessionID", session.MCPSessionID)
		return nil, false
	}

	// Update last access time for idle timeout tracking
	session.UpdateLastAccess()

	return session, true
}

// CloseSession terminates a specific browser session.
// It validates MCP ownership, closes all pages and the browser context,
// and removes the session from tracking maps.
func (m *manager) CloseSession(ctx context.Context, mcpSessionID, browserSessionID string) error {
	slog.Debug("closing session", "sessionID", browserSessionID, "mcpSessionID", mcpSessionID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Look up session
	session, exists := m.sessions[browserSessionID]
	if !exists {
		slog.Error("cannot close session: not found", "sessionID", browserSessionID)
		return ErrSessionNotFound
	}

	// Validate MCP ownership
	if session.MCPSessionID != mcpSessionID {
		slog.Warn("unauthorized session close attempt", "sessionID", browserSessionID, "mcpSessionID", mcpSessionID, "ownerMCPSessionID", session.MCPSessionID)
		return ErrUnauthorized
	}

	// Clean up network event listeners
	session.CleanupNetworkListeners()

	// Close all pages in the session
	for _, page := range session.Pages {
		_ = page.Close() // Log but continue - we still want to close the context
	}

	// Close the browser context
	if err := session.Context.Close(); err != nil {
		slog.Error("failed to close browser context during session close", "sessionID", browserSessionID, "error", err)
		return fmt.Errorf("failed to close browser context: %w", err)
	}

	// Remove from sessions map
	delete(m.sessions, browserSessionID)

	// Remove from mcpSessions index
	m.removeFromMCPIndex(mcpSessionID, browserSessionID)

	slog.Info("session closed", "sessionID", browserSessionID, "mcpSessionID", mcpSessionID)
	return nil
}

// ListSessions returns metadata for all browser sessions owned by an MCP connection.
// Returns an empty slice if the MCP session has no browser sessions or does not exist.
// This method provides a point-in-time snapshot; session state may change after return.
func (m *manager) ListSessions(mcpSessionID string) []*SessionInfo {
	slog.Debug("listing sessions", "mcpSessionID", mcpSessionID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get list of browser session IDs for this MCP connection
	sessionIDs, exists := m.mcpSessions[mcpSessionID]
	if !exists || len(sessionIDs) == 0 {
		slog.Debug("no sessions found for MCP connection", "mcpSessionID", mcpSessionID)
		return []*SessionInfo{}
	}

	// Collect metadata for each session
	result := make([]*SessionInfo, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		session, exists := m.sessions[sessionID]
		if !exists {
			// Session was removed but mcpSessions not yet updated (shouldn't happen)
			continue
		}

		// Get URL and title safely using ActivePageInfo to avoid data races
		url, title := session.ActivePageInfo()

		info := &SessionInfo{
			ID:          session.ID,
			BrowserType: session.BrowserType,
			URL:         url,
			Title:       title,
			CreatedAt:   session.CreatedAt,
			LastAccess:  session.LastAccess,
		}

		result = append(result, info)
	}

	return result
}

// CloseAllForMCP closes all browser sessions when an MCP connection ends.
// It continues closing remaining sessions even if some fail.
func (m *manager) CloseAllForMCP(ctx context.Context, mcpSessionID string) error {
	slog.Debug("closing all sessions for MCP connection", "mcpSessionID", mcpSessionID)

	m.mu.Lock()
	defer m.mu.Unlock()

	sessionIDs, exists := m.mcpSessions[mcpSessionID]
	if !exists || len(sessionIDs) == 0 {
		// No sessions to close - not an error
		return nil
	}

	slog.Info("closing sessions for MCP disconnect", "mcpSessionID", mcpSessionID, "sessionCount", len(sessionIDs))

	// Collect errors but continue closing all sessions
	var errs []error

	// Make a copy of session IDs since we'll be modifying the slice
	sessionIDsCopy := make([]string, len(sessionIDs))
	copy(sessionIDsCopy, sessionIDs)

	for _, sessionID := range sessionIDsCopy {
		session, exists := m.sessions[sessionID]
		if !exists {
			continue // Session already removed somehow
		}

		// Clean up network event listeners
		session.CleanupNetworkListeners()

		// Close all pages in the session
		for _, page := range session.Pages {
			_ = page.Close()
		}

		// Close the browser context
		if err := session.Context.Close(); err != nil {
			slog.Error("failed to close session during MCP cleanup", "sessionID", sessionID, "mcpSessionID", mcpSessionID, "error", err)
			errs = append(errs, fmt.Errorf("failed to close session %s: %w", sessionID, err))
		}

		// Remove from sessions map
		delete(m.sessions, sessionID)
	}

	// Remove MCP session entry entirely
	delete(m.mcpSessions, mcpSessionID)

	// Return combined error if any closures failed
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	slog.Info("all sessions closed for MCP connection", "mcpSessionID", mcpSessionID)
	return nil
}

// CloseAll closes all browser sessions regardless of MCP connection.
// It continues closing remaining sessions even if some fail.
// This is used during server shutdown to ensure all resources are released.
func (m *manager) CloseAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) == 0 {
		slog.Debug("no sessions to close")
		return nil
	}

	slog.Info("closing all browser sessions", "sessionCount", len(m.sessions))

	// Collect errors but continue closing all sessions
	var errs []error

	for sessionID, session := range m.sessions {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			slog.Warn("context cancelled during CloseAll", "error", ctx.Err())
			errs = append(errs, ctx.Err())
			return errors.Join(errs...)
		default:
		}

		slog.Debug("closing session", "sessionID", sessionID)

		// Clean up network event listeners
		session.CleanupNetworkListeners()

		// Close all pages in the session
		for _, page := range session.Pages {
			_ = page.Close()
		}

		// Close the browser context
		if err := session.Context.Close(); err != nil {
			slog.Error("failed to close session during shutdown", "sessionID", sessionID, "error", err)
			errs = append(errs, fmt.Errorf("failed to close session %s: %w", sessionID, err))
		}
	}

	// Clear all tracking maps
	m.sessions = make(map[string]*BrowserSession)
	m.mcpSessions = make(map[string][]string)

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	slog.Info("all browser sessions closed")
	return nil
}

// Cleanup removes expired/orphaned sessions based on idle timeout.
// It returns the number of sessions cleaned up and any errors encountered.
// Sessions with LastAccess older than idleTimeout are considered expired.
// A zero idleTimeout means no cleanup will be performed.
func (m *manager) Cleanup(ctx context.Context, idleTimeout time.Duration) (int, error) {
	slog.Debug("running session cleanup", "idleTimeout", idleTimeout)

	// Zero timeout means no cleanup
	if idleTimeout == 0 {
		return 0, nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	// Calculate cutoff time
	cutoff := time.Now().Add(-idleTimeout)

	// Identify expired sessions under read lock
	m.mu.RLock()
	var expiredSessions []struct {
		mcpSessionID     string
		browserSessionID string
	}
	for sessionID, session := range m.sessions {
		if session.LastAccess.Before(cutoff) {
			expiredSessions = append(expiredSessions, struct {
				mcpSessionID     string
				browserSessionID string
			}{
				mcpSessionID:     session.MCPSessionID,
				browserSessionID: sessionID,
			})
		}
	}
	m.mu.RUnlock()

	// No expired sessions found
	if len(expiredSessions) == 0 {
		slog.Debug("no expired sessions found during cleanup")
		return 0, nil
	}

	slog.Info("found expired sessions for cleanup", "count", len(expiredSessions))

	// Close each expired session
	var errs []error
	cleanedCount := 0

	for _, expired := range expiredSessions {
		// Check for context cancellation between operations
		select {
		case <-ctx.Done():
			if len(errs) > 0 {
				errs = append(errs, ctx.Err())
				return cleanedCount, errors.Join(errs...)
			}
			return cleanedCount, ctx.Err()
		default:
		}

		// Close the session (CloseSession handles its own locking)
		err := m.CloseSession(ctx, expired.mcpSessionID, expired.browserSessionID)
		if err != nil {
			// Session may have been closed/removed between identification and cleanup
			// ErrSessionNotFound is acceptable, other errors should be tracked
			if !errors.Is(err, ErrSessionNotFound) {
				slog.Error("failed to cleanup expired session", "sessionID", expired.browserSessionID, "error", err)
				errs = append(errs, fmt.Errorf("cleanup session %s: %w", expired.browserSessionID, err))
			}
			// Still count as "cleaned" if it no longer exists
			cleanedCount++
		} else {
			cleanedCount++
		}
	}

	if len(errs) > 0 {
		return cleanedCount, errors.Join(errs...)
	}

	slog.Info("session cleanup completed", "cleanedCount", cleanedCount)
	return cleanedCount, nil
}

// removeFromMCPIndex removes a browser session ID from the MCP session's list.
// Caller must hold the write lock.
func (m *manager) removeFromMCPIndex(mcpSessionID, browserSessionID string) {
	sessionIDs := m.mcpSessions[mcpSessionID]
	for i, id := range sessionIDs {
		if id == browserSessionID {
			// Remove by replacing with last element and truncating
			m.mcpSessions[mcpSessionID] = append(sessionIDs[:i], sessionIDs[i+1:]...)
			break
		}
	}
	// If MCP session has no more browser sessions, remove the entry
	if len(m.mcpSessions[mcpSessionID]) == 0 {
		delete(m.mcpSessions, mcpSessionID)
	}
}

// attachConsoleHandler attaches a console event handler to a page that captures
// console messages and stores them in the provided buffer.
// This function should be called for each new page (initial page and new tabs).
func attachConsoleHandler(page playwright.Page, buffer *ConsoleLogBuffer) {
	page.On("console", func(msg playwright.ConsoleMessage) {
		// Map Playwright console message type to ConsoleLogLevel
		level := MapConsoleType(msg.Type())

		// Create log entry with current timestamp
		entry := ConsoleLogEntry{
			Timestamp: time.Now(),
			Level:     level,
			Text:      msg.Text(),
		}

		// Get source location if available
		if location := msg.Location(); location != nil {
			entry.URL = location.URL
			entry.Line = location.LineNumber
			entry.Column = location.ColumnNumber
		}

		// Add to buffer (thread-safe)
		buffer.Add(entry)
	})
}

// AttachConsoleHandler is the exported version of attachConsoleHandler for use
// by external packages (e.g., tab management tools) when creating new pages.
func AttachConsoleHandler(page playwright.Page, buffer *ConsoleLogBuffer) {
	attachConsoleHandler(page, buffer)
}
