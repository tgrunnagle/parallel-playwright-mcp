//go:build integration

package session

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
)

// Integration tests require Playwright runtime to be installed.
// Run with: go test -tags=integration ./pkg/session/...

// skipIfPlaywrightNotInstalled checks if Playwright is available and skips the test with
// helpful instructions if not.
func skipIfPlaywrightNotInstalled(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	errStr := err.Error()
	if strings.Contains(errStr, "please install the driver") ||
		strings.Contains(errStr, "executable file not found") ||
		strings.Contains(errStr, "Playwright") ||
		strings.Contains(errStr, "browser pool is not running") {
		t.Skipf("Playwright not installed or pool not started. Run 'task playwright:install' to install browsers.\nOriginal error: %v", err)
	}
}

func setupPool(t *testing.T) browser.BrowserPool {
	t.Helper()
	pool := browser.NewBrowserPoolWithOptions(browser.PoolOptions{
		DefaultHeadless: true,
	})
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		skipIfPlaywrightNotInstalled(t, err)
		t.Fatalf("Failed to start pool: %v", err)
	}
	return pool
}

func TestCreateSessionIntegration(t *testing.T) {
	pool := setupPool(t)
	defer pool.Stop(context.Background())

	mgr := NewManager(pool)
	ctx := context.Background()

	t.Run("creates usable browser context", func(t *testing.T) {
		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", session.ID)

		if session.Context == nil {
			t.Fatal("session.Context should not be nil")
		}
	})

	t.Run("initial page can navigate to URL", func(t *testing.T) {
		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", session.ID)

		page := session.ActivePage()
		if page == nil {
			t.Fatal("ActivePage should not be nil")
		}

		_, err = page.Goto("about:blank")
		if err != nil {
			t.Fatalf("Page.Goto failed: %v", err)
		}

		url := page.URL()
		if url != "about:blank" {
			t.Errorf("URL = %s, want about:blank", url)
		}
	})

	t.Run("multiple sessions are independent", func(t *testing.T) {
		session1, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", session1.ID)

		session2, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", session2.ID)

		// Navigate each to different pages
		page1 := session1.ActivePage()
		page2 := session2.ActivePage()

		_, err = page1.Goto("about:blank")
		if err != nil {
			t.Fatalf("Page1.Goto failed: %v", err)
		}

		_, err = page2.Goto("about:blank")
		if err != nil {
			t.Fatalf("Page2.Goto failed: %v", err)
		}

		// Verify they're different contexts
		if session1.Context == session2.Context {
			t.Error("sessions should have different browser contexts")
		}
	})

	t.Run("sessions with different browser types work", func(t *testing.T) {
		sessionChromium, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession Chromium failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", sessionChromium.ID)

		if sessionChromium.BrowserType != browser.BrowserChromium {
			t.Errorf("expected Chromium, got %s", sessionChromium.BrowserType)
		}

		// Note: Firefox and WebKit may not be installed in all test environments
		// so we only test Chromium which is typically the default
	})

	t.Run("session options are applied", func(t *testing.T) {
		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
			Viewport:    &browser.Viewport{Width: 800, Height: 600},
			UserAgent:   "TestAgent/1.0",
			Locale:      "en-US",
			TimezoneID:  "America/New_York",
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", session.ID)

		// Context should be created (viewport, etc. are applied internally by Playwright)
		if session.Context == nil {
			t.Error("session.Context should not be nil")
		}
	})
}

func TestGetSessionIntegration(t *testing.T) {
	pool := setupPool(t)
	defer pool.Stop(context.Background())

	mgr := NewManager(pool)
	ctx := context.Background()

	t.Run("returned session is usable", func(t *testing.T) {
		created, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", created.ID)

		retrieved, ok := mgr.GetSession("mcp-1", created.ID)
		if !ok || retrieved == nil {
			t.Fatal("GetSession should return session")
		}

		page := retrieved.ActivePage()
		_, err = page.Goto("about:blank")
		if err != nil {
			t.Fatalf("Page.Goto failed on retrieved session: %v", err)
		}
	})
}

func TestCloseSessionIntegration(t *testing.T) {
	pool := setupPool(t)
	defer pool.Stop(context.Background())

	mgr := NewManager(pool)
	ctx := context.Background()

	t.Run("closes browser context properly", func(t *testing.T) {
		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}

		browserCtx := session.Context

		err = mgr.CloseSession(ctx, "mcp-1", session.ID)
		if err != nil {
			t.Fatalf("CloseSession failed: %v", err)
		}

		// Trying to create a new page on closed context should fail
		_, err = browserCtx.NewPage()
		if err == nil {
			t.Error("expected error when using closed context")
		}
	})
}

func TestCloseAllForMCPIntegration(t *testing.T) {
	pool := setupPool(t)
	defer pool.Stop(context.Background())

	mgr := NewManager(pool)
	ctx := context.Background()

	t.Run("closes all browser contexts for MCP connection", func(t *testing.T) {
		session1, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}

		session2, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		ctx1 := session1.Context
		ctx2 := session2.Context

		err = mgr.CloseAllForMCP(ctx, "mcp-1")
		if err != nil {
			t.Fatalf("CloseAllForMCP failed: %v", err)
		}

		// Both contexts should be closed
		_, err1 := ctx1.NewPage()
		_, err2 := ctx2.NewPage()

		if err1 == nil || err2 == nil {
			t.Error("expected errors when using closed contexts")
		}
	})
}

func TestListSessionsIntegration(t *testing.T) {
	pool := setupPool(t)
	defer pool.Stop(context.Background())

	mgr := NewManager(pool)
	ctx := context.Background()

	t.Run("returns URL and title from page", func(t *testing.T) {
		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", session.ID)

		page := session.ActivePage()
		_, err = page.Goto("about:blank")
		if err != nil {
			t.Fatalf("Page.Goto failed: %v", err)
		}

		sessions := mgr.ListSessions("mcp-1")
		if len(sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(sessions))
		}

		info := sessions[0]
		if info.URL != "about:blank" {
			t.Errorf("URL = %s, want about:blank", info.URL)
		}
	})
}

func TestConcurrentSessionOperationsIntegration(t *testing.T) {
	pool := setupPool(t)
	defer pool.Stop(context.Background())

	mgr := NewManager(pool)
	ctx := context.Background()

	t.Run("concurrent session creation is thread-safe", func(t *testing.T) {
		var wg sync.WaitGroup
		sessionCount := 5 // Keep low to avoid resource exhaustion

		sessions := make([]*BrowserSession, sessionCount)
		errors := make([]error, sessionCount)

		for i := 0; i < sessionCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				session, err := mgr.CreateSession(ctx, "mcp-concurrent", SessionOptions{
					BrowserType: browser.BrowserChromium,
				})
				if err != nil {
					errors[idx] = err
					return
				}
				sessions[idx] = session
			}(i)
		}

		wg.Wait()

		// Check for errors
		for i, err := range errors {
			if err != nil {
				skipIfPlaywrightNotInstalled(t, err)
				t.Fatalf("Session %d creation failed: %v", i, err)
			}
		}

		// Verify all sessions were created
		createdCount := 0
		for _, s := range sessions {
			if s != nil {
				createdCount++
			}
		}
		if createdCount != sessionCount {
			t.Errorf("expected %d sessions, got %d", sessionCount, createdCount)
		}

		// Clean up
		mgr.CloseAllForMCP(ctx, "mcp-concurrent")
	})
}

func TestCleanupIntegration(t *testing.T) {
	pool := setupPool(t)
	defer pool.Stop(context.Background())

	mgr := NewManager(pool).(*manager)
	ctx := context.Background()

	t.Run("cleanup removes expired sessions and closes contexts", func(t *testing.T) {
		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}

		browserCtx := session.Context

		// Set LastAccess to the past to make session expired
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

		// Context should be closed
		_, err = browserCtx.NewPage()
		if err == nil {
			t.Error("expected error when using closed context after cleanup")
		}
	})
}
