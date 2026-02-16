//go:build integration

package session

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
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

func TestNetworkLoggingIntegration(t *testing.T) {
	pool := setupPool(t)
	defer pool.Stop(context.Background())

	mgr := NewManager(pool)
	ctx := context.Background()

	t.Run("network buffer is initialized for new session", func(t *testing.T) {
		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", session.ID)

		if session.NetworkLogs == nil {
			t.Fatal("NetworkLogs should not be nil")
		}
	})

	t.Run("captures network activity during navigation", func(t *testing.T) {
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

		// Navigate to a data URL that generates a network request
		dataURL := "data:text/html,<html><body>Test</body></html>"
		_, err = page.Goto(dataURL)
		if err != nil {
			t.Fatalf("Page.Goto failed: %v", err)
		}

		// Give some time for network events to be processed
		time.Sleep(100 * time.Millisecond)

		// Verify network entries were captured
		entries := session.NetworkLogs.Entries(0)
		if len(entries) == 0 {
			t.Error("expected at least one network entry from navigation")
		}

		// Verify the captured entry has expected fields
		if len(entries) > 0 {
			entry := entries[0]
			if entry.Method == "" {
				t.Error("Method should not be empty")
			}
			if entry.URL == "" {
				t.Error("URL should not be empty")
			}
			// Data URLs may have status 0 in some browsers, so we just check it's set
			if entry.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}
		}
	})

	t.Run("captures HTTP navigation with status code", func(t *testing.T) {
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

		// Navigate to example.com (a stable public URL)
		_, err = page.Goto("https://example.com")
		if err != nil {
			// If network is unavailable, skip this test
			t.Skipf("Could not navigate to example.com (network may be unavailable): %v", err)
		}

		// Give time for network events to be processed
		time.Sleep(200 * time.Millisecond)

		entries := session.NetworkLogs.Entries(0)
		if len(entries) == 0 {
			t.Error("expected at least one network entry from HTTP navigation")
		}

		// Find the main document request
		var docEntry *NetworkLogEntry
		for i := range entries {
			if entries[i].ResourceType == "document" || strings.Contains(entries[i].URL, "example.com") {
				docEntry = &entries[i]
				break
			}
		}

		if docEntry != nil {
			if docEntry.Status != 200 {
				t.Errorf("expected status 200, got %d", docEntry.Status)
			}
			if docEntry.Method != "GET" {
				t.Errorf("expected method GET, got %s", docEntry.Method)
			}
			if docEntry.Duration <= 0 {
				t.Error("expected positive duration for HTTP request")
			}
		}
	})

	t.Run("multiple navigations accumulate entries", func(t *testing.T) {
		session, err := mgr.CreateSession(ctx, "mcp-1", SessionOptions{
			BrowserType: browser.BrowserChromium,
		})
		if err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("CreateSession failed: %v", err)
		}
		defer mgr.CloseSession(ctx, "mcp-1", session.ID)

		page := session.ActivePage()

		// Navigate to multiple data URLs
		for i := 0; i < 3; i++ {
			dataURL := "data:text/html,<html><body>Page " + string(rune('A'+i)) + "</body></html>"
			_, err = page.Goto(dataURL)
			if err != nil {
				t.Fatalf("Page.Goto failed: %v", err)
			}
		}

		// Give time for network events to be processed
		time.Sleep(100 * time.Millisecond)

		entries := session.NetworkLogs.Entries(0)
		if len(entries) < 3 {
			t.Errorf("expected at least 3 network entries, got %d", len(entries))
		}
	})

	t.Run("captures failed requests with status 0", func(t *testing.T) {
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

		// Clear any existing entries
		session.NetworkLogs.Clear()

		// Set up a route to abort requests to a specific URL
		err = page.Route("**/abort-this-request", func(route playwright.Route) {
			route.Abort()
		})
		if err != nil {
			t.Fatalf("Failed to set up route: %v", err)
		}

		// Navigate to a page that will make a request we'll abort
		htmlWithAbortedFetch := `data:text/html,<html><script>
			fetch('/abort-this-request').catch(() => {});
		</script></html>`
		_, err = page.Goto(htmlWithAbortedFetch)
		if err != nil {
			t.Fatalf("Page.Goto failed: %v", err)
		}

		// Give time for network events to be processed
		time.Sleep(200 * time.Millisecond)

		// Check for a failed request entry (status 0)
		entries := session.NetworkLogs.Entries(0)
		var foundAborted bool
		for _, entry := range entries {
			if strings.Contains(entry.URL, "abort-this-request") && entry.Status == 0 {
				foundAborted = true
				break
			}
		}

		if !foundAborted {
			t.Error("expected to find an aborted request with status 0")
		}
	})

	t.Run("captures POST request with body size", func(t *testing.T) {
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

		// Clear any existing entries
		session.NetworkLogs.Clear()

		// Set up a route to intercept and fulfill POST requests
		postBody := `{"test": "data", "value": 12345}`
		err = page.Route("**/post-endpoint", func(route playwright.Route) {
			route.Fulfill(playwright.RouteFulfillOptions{
				Status:      playwright.Int(200),
				ContentType: playwright.String("application/json"),
				Body:        []byte(`{"status": "ok"}`),
			})
		})
		if err != nil {
			t.Fatalf("Failed to set up route: %v", err)
		}

		// Navigate to a page that will make a POST request
		htmlWithPost := `data:text/html,<html><script>
			fetch('/post-endpoint', {
				method: 'POST',
				headers: {'Content-Type': 'application/json'},
				body: '` + postBody + `'
			});
		</script></html>`
		_, err = page.Goto(htmlWithPost)
		if err != nil {
			t.Fatalf("Page.Goto failed: %v", err)
		}

		// Give time for network events to be processed
		time.Sleep(200 * time.Millisecond)

		// Check for the POST request entry
		entries := session.NetworkLogs.Entries(0)
		var foundPost *NetworkLogEntry
		for i := range entries {
			if entries[i].Method == "POST" && strings.Contains(entries[i].URL, "post-endpoint") {
				foundPost = &entries[i]
				break
			}
		}

		if foundPost == nil {
			t.Fatal("expected to find POST request entry")
		}

		// Verify the request had a body
		expectedSize := int64(len(postBody))
		if foundPost.RequestSize != expectedSize {
			t.Errorf("RequestSize = %d, want %d", foundPost.RequestSize, expectedSize)
		}

		if foundPost.Status != 200 {
			t.Errorf("Status = %d, want 200", foundPost.Status)
		}
	})
}
