package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
)

// mockBrowserContext is a mock implementation of playwright.BrowserContext for testing.
type mockBrowserContext struct {
	playwright.BrowserContext
	pages       []playwright.Page
	closed      bool
	closeErr    error
	newPageErr  error
	newPageFunc func() (playwright.Page, error)
}

func (m *mockBrowserContext) NewPage() (playwright.Page, error) {
	if m.newPageFunc != nil {
		return m.newPageFunc()
	}
	if m.newPageErr != nil {
		return nil, m.newPageErr
	}
	page := &mockPage{}
	m.pages = append(m.pages, page)
	return page, nil
}

func (m *mockBrowserContext) Close(options ...playwright.BrowserContextCloseOptions) error {
	m.closed = true
	return m.closeErr
}

// mockPage is a mock implementation of playwright.Page for testing.
type mockPage struct {
	playwright.Page
	url       string
	title     string
	titleErr  error
	closed    bool
	closeErr  error
}

func (m *mockPage) URL() string {
	return m.url
}

func (m *mockPage) Title() (string, error) {
	return m.title, m.titleErr
}

func (m *mockPage) Close(options ...playwright.PageCloseOptions) error {
	m.closed = true
	return m.closeErr
}

// mockBrowserPool is a mock implementation of browser.BrowserPool for testing.
type mockBrowserPool struct {
	newContextFunc func(ctx context.Context, browserType browser.BrowserType, opts browser.ContextOptions) (playwright.BrowserContext, error)
	newContextErr  error
}

func (m *mockBrowserPool) Start(ctx context.Context) error {
	return nil
}

func (m *mockBrowserPool) Stop(ctx context.Context) error {
	return nil
}

func (m *mockBrowserPool) NewContext(ctx context.Context, browserType browser.BrowserType, opts browser.ContextOptions) (playwright.BrowserContext, error) {
	if m.newContextFunc != nil {
		return m.newContextFunc(ctx, browserType, opts)
	}
	if m.newContextErr != nil {
		return nil, m.newContextErr
	}
	return &mockBrowserContext{}, nil
}

func (m *mockBrowserPool) CloseContext(ctx context.Context, browserContext playwright.BrowserContext) error {
	return browserContext.Close()
}

func (m *mockBrowserPool) Stats() browser.PoolStats {
	return browser.PoolStats{}
}

func TestNewManager(t *testing.T) {
	t.Run("returns non-nil manager", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		if mgr == nil {
			t.Error("NewManager returned nil")
		}
	})

	t.Run("initializes with empty session maps", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)

		if len(mgr.sessions) != 0 {
			t.Errorf("sessions map should be empty, got %d entries", len(mgr.sessions))
		}
		if len(mgr.mcpSessions) != 0 {
			t.Errorf("mcpSessions map should be empty, got %d entries", len(mgr.mcpSessions))
		}
	})
}

func TestCreateSession(t *testing.T) {
	t.Run("generates unique session IDs with sess- prefix", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		sessionIDs := make(map[string]bool)
		for i := 0; i < 100; i++ {
			session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
			if err != nil {
				t.Fatalf("CreateSession failed: %v", err)
			}
			if sessionIDs[session.ID] {
				t.Errorf("duplicate session ID generated: %s", session.ID)
			}
			sessionIDs[session.ID] = true

			if len(session.ID) < 5 || session.ID[:5] != "sess-" {
				t.Errorf("session ID should start with 'sess-', got %s", session.ID)
			}
		}
	})

	t.Run("stores session in sessions map", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		stored, exists := mgr.sessions[session.ID]
		if !exists {
			t.Error("session not stored in sessions map")
		}
		if stored != session {
			t.Error("stored session does not match returned session")
		}
	})

	t.Run("adds session ID to mcpSessions index", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		mcpID := "mcp-test-1"
		session, err := mgr.CreateSession(ctx, mcpID, SessionOptions{})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		sessionIDs, exists := mgr.mcpSessions[mcpID]
		if !exists {
			t.Error("mcpSessions entry not created")
		}
		found := false
		for _, id := range sessionIDs {
			if id == session.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("session ID not added to mcpSessions index")
		}
	})

	t.Run("appends to existing mcpSessions slice", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		mcpID := "mcp-multi"
		session1, _ := mgr.CreateSession(ctx, mcpID, SessionOptions{})
		session2, _ := mgr.CreateSession(ctx, mcpID, SessionOptions{})

		sessionIDs := mgr.mcpSessions[mcpID]
		if len(sessionIDs) != 2 {
			t.Errorf("expected 2 sessions for MCP, got %d", len(sessionIDs))
		}
		hasSession1 := false
		hasSession2 := false
		for _, id := range sessionIDs {
			if id == session1.ID {
				hasSession1 = true
			}
			if id == session2.ID {
				hasSession2 = true
			}
		}
		if !hasSession1 || !hasSession2 {
			t.Error("not all session IDs found in mcpSessions index")
		}
	})

	t.Run("sets CreatedAt and LastAccess timestamps", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		before := time.Now()
		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		after := time.Now()

		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		if session.CreatedAt.Before(before) || session.CreatedAt.After(after) {
			t.Error("CreatedAt not set to current time")
		}
		if session.LastAccess.Before(before) || session.LastAccess.After(after) {
			t.Error("LastAccess not set to current time")
		}
	})

	t.Run("creates initial page with tab ID", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		if len(session.Pages) != 1 {
			t.Errorf("expected 1 page, got %d", len(session.Pages))
		}
		if session.ActiveTabID == "" {
			t.Error("ActiveTabID not set")
		}
		if len(session.ActiveTabID) < 4 || session.ActiveTabID[:4] != "tab-" {
			t.Errorf("ActiveTabID should start with 'tab-', got %s", session.ActiveTabID)
		}
		if _, exists := session.Pages[session.ActiveTabID]; !exists {
			t.Error("ActiveTabID not found in Pages map")
		}
	})

	t.Run("defaults to chromium when BrowserType is empty", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		if session.BrowserType != browser.BrowserChromium {
			t.Errorf("expected BrowserChromium, got %s", session.BrowserType)
		}
	})

	t.Run("uses specified BrowserType", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserFirefox,
		})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		if session.BrowserType != browser.BrowserFirefox {
			t.Errorf("expected BrowserFirefox, got %s", session.BrowserType)
		}
	})

	t.Run("returns error when pool.NewContext fails", func(t *testing.T) {
		expectedErr := errors.New("pool error")
		pool := &mockBrowserPool{newContextErr: expectedErr}
		mgr := NewManager(pool)
		ctx := context.Background()

		_, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("error should wrap pool error: %v", err)
		}
	})

	t.Run("returns error when initial page creation fails", func(t *testing.T) {
		pageErr := errors.New("page creation failed")
		mockCtx := &mockBrowserContext{newPageErr: pageErr}
		pool := &mockBrowserPool{
			newContextFunc: func(ctx context.Context, bt browser.BrowserType, opts browser.ContextOptions) (playwright.BrowserContext, error) {
				return mockCtx, nil
			},
		}
		mgr := NewManager(pool)
		ctx := context.Background()

		_, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !mockCtx.closed {
			t.Error("context should be closed when page creation fails")
		}
	})

	t.Run("translates SessionOptions to ContextOptions", func(t *testing.T) {
		var capturedOpts browser.ContextOptions
		pool := &mockBrowserPool{
			newContextFunc: func(ctx context.Context, bt browser.BrowserType, opts browser.ContextOptions) (playwright.BrowserContext, error) {
				capturedOpts = opts
				return &mockBrowserContext{}, nil
			},
		}
		mgr := NewManager(pool)
		ctx := context.Background()

		viewport := &browser.Viewport{Width: 1920, Height: 1080}
		opts := SessionOptions{
			BrowserType:  browser.BrowserWebKit,
			Headless:     true,
			Viewport:     viewport,
			UserAgent:    "Test Agent",
			Locale:       "en-GB",
			TimezoneID:   "Europe/London",
			ExtraHeaders: map[string]string{"X-Test": "value"},
		}

		_, err := mgr.CreateSession(ctx, "mcp-1", opts)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		if capturedOpts.Headless != opts.Headless {
			t.Error("Headless not translated")
		}
		if capturedOpts.Viewport != viewport {
			t.Error("Viewport not translated")
		}
		if capturedOpts.UserAgent != opts.UserAgent {
			t.Error("UserAgent not translated")
		}
		if capturedOpts.Locale != opts.Locale {
			t.Error("Locale not translated")
		}
		if capturedOpts.TimezoneID != opts.TimezoneID {
			t.Error("TimezoneID not translated")
		}
		if capturedOpts.ExtraHeaders["X-Test"] != "value" {
			t.Error("ExtraHeaders not translated")
		}
	})
}

func TestGetSession(t *testing.T) {
	t.Run("returns session when ownership matches", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		created, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		retrieved, ok := mgr.GetSession("mcp-1", created.ID)

		if !ok {
			t.Error("GetSession should return true")
		}
		if retrieved == nil {
			t.Error("GetSession should return session")
		}
		if retrieved.ID != created.ID {
			t.Error("returned session ID mismatch")
		}
	})

	t.Run("returns nil,false when session does not exist", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)

		session, ok := mgr.GetSession("mcp-1", "nonexistent")
		if ok {
			t.Error("GetSession should return false for nonexistent session")
		}
		if session != nil {
			t.Error("GetSession should return nil for nonexistent session")
		}
	})

	t.Run("returns nil,false when MCP ownership does not match", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		created, _ := mgr.CreateSession(ctx, "mcp-owner", SessionOptions{})
		session, ok := mgr.GetSession("mcp-other", created.ID)

		if ok {
			t.Error("GetSession should return false for wrong owner")
		}
		if session != nil {
			t.Error("GetSession should return nil for wrong owner")
		}
	})

	t.Run("updates LastAccess on successful retrieval", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		created, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		originalAccess := created.LastAccess

		time.Sleep(10 * time.Millisecond)

		retrieved, ok := mgr.GetSession("mcp-1", created.ID)
		if !ok || retrieved == nil {
			t.Fatal("GetSession failed")
		}
		if !retrieved.LastAccess.After(originalAccess) {
			t.Error("LastAccess should be updated")
		}
	})
}

func TestCloseSession(t *testing.T) {
	t.Run("returns ErrSessionNotFound for nonexistent session", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		err := mgr.CloseSession(ctx, "mcp-1", "nonexistent")
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("expected ErrSessionNotFound, got %v", err)
		}
	})

	t.Run("returns ErrUnauthorized when ownership does not match", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		created, _ := mgr.CreateSession(ctx, "mcp-owner", SessionOptions{})
		err := mgr.CloseSession(ctx, "mcp-other", created.ID)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("removes session from sessions map", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		created, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		err := mgr.CloseSession(ctx, "mcp-1", created.ID)

		if err != nil {
			t.Fatalf("CloseSession failed: %v", err)
		}
		if _, exists := mgr.sessions[created.ID]; exists {
			t.Error("session should be removed from sessions map")
		}
	})

	t.Run("removes session ID from mcpSessions index", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		created, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		_ = mgr.CloseSession(ctx, "mcp-1", created.ID)

		sessionIDs, exists := mgr.mcpSessions["mcp-1"]
		if exists && len(sessionIDs) > 0 {
			for _, id := range sessionIDs {
				if id == created.ID {
					t.Error("session ID should be removed from mcpSessions index")
				}
			}
		}
	})

	t.Run("removes empty MCP session entry", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		created, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		_ = mgr.CloseSession(ctx, "mcp-1", created.ID)

		if _, exists := mgr.mcpSessions["mcp-1"]; exists {
			t.Error("empty MCP session entry should be removed")
		}
	})

	t.Run("only removes target session when MCP has multiple sessions", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		session1, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		session2, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})

		_ = mgr.CloseSession(ctx, "mcp-1", session1.ID)

		if _, exists := mgr.sessions[session2.ID]; !exists {
			t.Error("other sessions should not be affected")
		}
		sessionIDs := mgr.mcpSessions["mcp-1"]
		if len(sessionIDs) != 1 || sessionIDs[0] != session2.ID {
			t.Error("only target session should be removed from index")
		}
	})
}

func TestListSessions(t *testing.T) {
	t.Run("returns empty slice for unknown MCP session", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)

		result := mgr.ListSessions("unknown-mcp")
		if result == nil {
			t.Error("should return empty slice, not nil")
		}
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %d items", len(result))
		}
	})

	t.Run("returns session info for MCP connection", func(t *testing.T) {
		mockCtx := &mockBrowserContext{}
		mockPage := &mockPage{url: "https://example.com", title: "Example"}
		mockCtx.newPageFunc = func() (playwright.Page, error) {
			return mockPage, nil
		}
		pool := &mockBrowserPool{
			newContextFunc: func(ctx context.Context, bt browser.BrowserType, opts browser.ContextOptions) (playwright.BrowserContext, error) {
				return mockCtx, nil
			},
		}
		mgr := NewManager(pool)
		ctx := context.Background()

		created, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserFirefox,
		})

		result := mgr.ListSessions("mcp-1")
		if len(result) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result))
		}

		info := result[0]
		if info.ID != created.ID {
			t.Errorf("ID mismatch: got %s, want %s", info.ID, created.ID)
		}
		if info.BrowserType != browser.BrowserFirefox {
			t.Errorf("BrowserType mismatch: got %s, want %s", info.BrowserType, browser.BrowserFirefox)
		}
		if info.URL != "https://example.com" {
			t.Errorf("URL mismatch: got %s, want %s", info.URL, "https://example.com")
		}
		if info.Title != "Example" {
			t.Errorf("Title mismatch: got %s, want %s", info.Title, "Example")
		}
	})

	t.Run("returns multiple sessions for MCP connection", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		mgr.CreateSession(ctx, "mcp-1", SessionOptions{})

		result := mgr.ListSessions("mcp-1")
		if len(result) != 3 {
			t.Errorf("expected 3 sessions, got %d", len(result))
		}
	})
}

func TestCloseAllForMCP(t *testing.T) {
	t.Run("returns nil when MCP session has no browser sessions", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		err := mgr.CloseAllForMCP(ctx, "nonexistent")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("closes all sessions for MCP connection", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		mgr.CreateSession(ctx, "mcp-1", SessionOptions{})

		err := mgr.CloseAllForMCP(ctx, "mcp-1")
		if err != nil {
			t.Fatalf("CloseAllForMCP failed: %v", err)
		}

		if len(mgr.sessions) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(mgr.sessions))
		}
	})

	t.Run("removes MCP session entry from mcpSessions", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		_ = mgr.CloseAllForMCP(ctx, "mcp-1")

		if _, exists := mgr.mcpSessions["mcp-1"]; exists {
			t.Error("MCP session entry should be removed")
		}
	})

	t.Run("does not affect other MCP connections", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		session1, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		session2, _ := mgr.CreateSession(ctx, "mcp-2", SessionOptions{})

		_ = mgr.CloseAllForMCP(ctx, "mcp-1")

		if _, exists := mgr.sessions[session1.ID]; exists {
			t.Error("mcp-1 session should be closed")
		}
		if _, exists := mgr.sessions[session2.ID]; !exists {
			t.Error("mcp-2 session should not be affected")
		}
	})
}

func TestCleanup(t *testing.T) {
	t.Run("returns 0 when idleTimeout is 0", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		mgr.CreateSession(ctx, "mcp-1", SessionOptions{})

		count, err := mgr.Cleanup(ctx, 0)
		if err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 cleaned, got %d", count)
		}
	})

	t.Run("returns 0 when no sessions exist", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		count, err := mgr.Cleanup(ctx, time.Minute)
		if err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 cleaned, got %d", count)
		}
	})

	t.Run("cleans up expired sessions", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		session, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})

		// Manually set LastAccess to the past
		mgr.mu.Lock()
		session.LastAccess = time.Now().Add(-time.Hour)
		mgr.mu.Unlock()

		count, err := mgr.Cleanup(ctx, time.Minute)
		if err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 cleaned, got %d", count)
		}
		if _, exists := mgr.sessions[session.ID]; exists {
			t.Error("expired session should be removed")
		}
	})

	t.Run("does not clean active sessions", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		session, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})

		count, err := mgr.Cleanup(ctx, time.Hour)
		if err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 cleaned, got %d", count)
		}
		if _, exists := mgr.sessions[session.ID]; !exists {
			t.Error("active session should not be removed")
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := mgr.Cleanup(ctx, time.Minute)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("concurrent CreateSession calls are thread-safe", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		var wg sync.WaitGroup
		sessionIDs := make(chan string, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
				if err != nil {
					t.Errorf("CreateSession failed: %v", err)
					return
				}
				sessionIDs <- session.ID
			}()
		}

		wg.Wait()
		close(sessionIDs)

		// Check all session IDs are unique
		ids := make(map[string]bool)
		for id := range sessionIDs {
			if ids[id] {
				t.Errorf("duplicate session ID: %s", id)
			}
			ids[id] = true
		}
		if len(ids) != 100 {
			t.Errorf("expected 100 unique sessions, got %d", len(ids))
		}
	})

	t.Run("concurrent GetSession calls are thread-safe", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		session, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				retrieved, ok := mgr.GetSession("mcp-1", session.ID)
				if !ok || retrieved == nil {
					t.Error("GetSession should succeed")
				}
			}()
		}
		wg.Wait()
	})

	t.Run("concurrent operations on different sessions are thread-safe", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool)
		ctx := context.Background()

		var wg sync.WaitGroup

		// Create sessions
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(mcpID string) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					_, err := mgr.CreateSession(ctx, mcpID, SessionOptions{})
					if err != nil {
						t.Errorf("CreateSession failed: %v", err)
					}
				}
			}(string(rune('a' + i)))
		}

		wg.Wait()

		// Verify all sessions were created
		totalSessions := 0
		for i := 0; i < 10; i++ {
			mcpID := string(rune('a' + i))
			sessions := mgr.ListSessions(mcpID)
			totalSessions += len(sessions)
		}
		if totalSessions != 100 {
			t.Errorf("expected 100 total sessions, got %d", totalSessions)
		}
	})
}

func TestRemoveFromMCPIndex(t *testing.T) {
	t.Run("removes session ID from middle of slice", func(t *testing.T) {
		pool := &mockBrowserPool{}
		mgr := NewManager(pool).(*manager)
		ctx := context.Background()

		s1, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		s2, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})
		s3, _ := mgr.CreateSession(ctx, "mcp-1", SessionOptions{})

		mgr.mu.Lock()
		mgr.removeFromMCPIndex("mcp-1", s2.ID)
		mgr.mu.Unlock()

		sessionIDs := mgr.mcpSessions["mcp-1"]
		if len(sessionIDs) != 2 {
			t.Errorf("expected 2 sessions, got %d", len(sessionIDs))
		}
		for _, id := range sessionIDs {
			if id == s2.ID {
				t.Error("removed session ID should not be in slice")
			}
		}
		hasS1, hasS3 := false, false
		for _, id := range sessionIDs {
			if id == s1.ID {
				hasS1 = true
			}
			if id == s3.ID {
				hasS3 = true
			}
		}
		if !hasS1 || !hasS3 {
			t.Error("remaining sessions should still be in slice")
		}
	})
}
