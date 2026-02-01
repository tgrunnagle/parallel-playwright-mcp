//go:build integration

package browser

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/playwright-community/playwright-go"
)

// Integration tests require Playwright runtime to be installed.
// Run with: go test -tags=integration ./pkg/browser/...

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
		strings.Contains(errStr, "Playwright") {
		t.Skipf("Playwright not installed. Run 'task playwright:install' to install browsers.\nOriginal error: %v", err)
	}
}

func TestStartStopIntegration(t *testing.T) {
	t.Run("Start returns error if already running", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		// First start should succeed
		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("First Start failed: %v", err)
		}

		// Second start should fail with ErrPoolAlreadyRunning
		err := pool.Start(ctx)
		if err != ErrPoolAlreadyRunning {
			t.Errorf("Second Start error = %v, want %v", err, ErrPoolAlreadyRunning)
		}

		// Cleanup
		_ = pool.Stop(ctx)
	})

	t.Run("Start then Stop works correctly", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Start failed: %v", err)
		}

		if err := pool.Stop(ctx); err != nil {
			t.Errorf("Stop failed: %v", err)
		}
	})

	t.Run("concurrent Start calls only one succeeds", func(t *testing.T) {
		// Pre-check: verify Playwright is installed
		preCheckPool := NewBrowserPool()
		ctx := context.Background()
		if err := preCheckPool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Pre-check Start failed: %v", err)
		}
		_ = preCheckPool.Stop(ctx)

		// Actual test
		pool := NewBrowserPool()
		var wg sync.WaitGroup
		var successCount int
		var mu sync.Mutex

		// Try to start the pool from multiple goroutines
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := pool.Start(ctx); err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		if successCount != 1 {
			t.Errorf("Expected exactly 1 successful Start, got %d", successCount)
		}

		// Cleanup
		_ = pool.Stop(ctx)
	})
}

func TestNewContextIntegration(t *testing.T) {
	t.Run("returns error for invalid browser type", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Start failed: %v", err)
		}
		defer pool.Stop(ctx)

		_, err := pool.NewContext(ctx, BrowserType("safari"), ContextOptions{Headless: true})
		if err == nil {
			t.Error("NewContext should return error for invalid browser type")
		}
	})

	t.Run("creates chromium context with lazy launch", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Start failed: %v", err)
		}
		defer pool.Stop(ctx)

		// Check that chromium is not running initially
		stats := pool.Stats()
		if stats.Browsers[BrowserChromium].Running {
			t.Error("Chromium should not be running before NewContext")
		}

		// Create a context
		browserCtx, err := pool.NewContext(ctx, BrowserChromium, ContextOptions{Headless: true})
		if err != nil {
			t.Fatalf("NewContext failed: %v", err)
		}

		// Verify stats updated
		stats = pool.Stats()
		if !stats.Browsers[BrowserChromium].Running {
			t.Error("Chromium should be running after NewContext")
		}
		if stats.Browsers[BrowserChromium].TotalCreated != 1 {
			t.Errorf("TotalCreated = %d, want 1", stats.Browsers[BrowserChromium].TotalCreated)
		}
		if stats.Browsers[BrowserChromium].ActiveContexts != 1 {
			t.Errorf("ActiveContexts = %d, want 1", stats.Browsers[BrowserChromium].ActiveContexts)
		}

		// Close the context
		if err := pool.CloseContext(ctx, browserCtx); err != nil {
			t.Fatalf("CloseContext failed: %v", err)
		}

		// Verify stats updated
		stats = pool.Stats()
		if stats.Browsers[BrowserChromium].TotalClosed != 1 {
			t.Errorf("TotalClosed = %d, want 1", stats.Browsers[BrowserChromium].TotalClosed)
		}
		if stats.Browsers[BrowserChromium].ActiveContexts != 0 {
			t.Errorf("ActiveContexts = %d, want 0", stats.Browsers[BrowserChromium].ActiveContexts)
		}
	})

	t.Run("reuses browser on subsequent context creation", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Start failed: %v", err)
		}
		defer pool.Stop(ctx)

		// Create first context
		ctx1, err := pool.NewContext(ctx, BrowserChromium, ContextOptions{Headless: true})
		if err != nil {
			t.Fatalf("First NewContext failed: %v", err)
		}

		// Create second context
		ctx2, err := pool.NewContext(ctx, BrowserChromium, ContextOptions{Headless: true})
		if err != nil {
			t.Fatalf("Second NewContext failed: %v", err)
		}

		// Verify stats
		stats := pool.Stats()
		if stats.Browsers[BrowserChromium].TotalCreated != 2 {
			t.Errorf("TotalCreated = %d, want 2", stats.Browsers[BrowserChromium].TotalCreated)
		}
		if stats.Browsers[BrowserChromium].ActiveContexts != 2 {
			t.Errorf("ActiveContexts = %d, want 2", stats.Browsers[BrowserChromium].ActiveContexts)
		}

		// Cleanup
		_ = pool.CloseContext(ctx, ctx1)
		_ = pool.CloseContext(ctx, ctx2)
	})

	t.Run("creates contexts with custom options", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Start failed: %v", err)
		}
		defer pool.Stop(ctx)

		// Create context with custom options
		opts := ContextOptions{
			Headless:   true,
			Viewport:   &Viewport{Width: 800, Height: 600},
			UserAgent:  "Test Agent/1.0",
			Locale:     "en-US",
			TimezoneID: "America/New_York",
		}
		browserCtx, err := pool.NewContext(ctx, BrowserChromium, opts)
		if err != nil {
			t.Fatalf("NewContext with options failed: %v", err)
		}
		defer pool.CloseContext(ctx, browserCtx)

		// Verify context was created (basic sanity check)
		if browserCtx == nil {
			t.Error("BrowserContext should not be nil")
		}
	})
}

func TestMultiBrowserIntegration(t *testing.T) {
	t.Run("can create contexts for different browser types", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Start failed: %v", err)
		}
		defer pool.Stop(ctx)

		// Create Chromium context
		chromiumCtx, err := pool.NewContext(ctx, BrowserChromium, ContextOptions{Headless: true})
		if err != nil {
			t.Fatalf("Chromium NewContext failed: %v", err)
		}

		// Verify only Chromium is running
		stats := pool.Stats()
		if !stats.Browsers[BrowserChromium].Running {
			t.Error("Chromium should be running")
		}
		if stats.Browsers[BrowserFirefox].Running {
			t.Error("Firefox should not be running yet")
		}

		// Create Firefox context
		firefoxCtx, err := pool.NewContext(ctx, BrowserFirefox, ContextOptions{Headless: true})
		if err != nil {
			t.Fatalf("Firefox NewContext failed: %v", err)
		}

		// Verify both are running
		stats = pool.Stats()
		if !stats.Browsers[BrowserChromium].Running {
			t.Error("Chromium should still be running")
		}
		if !stats.Browsers[BrowserFirefox].Running {
			t.Error("Firefox should now be running")
		}

		// Cleanup
		_ = pool.CloseContext(ctx, chromiumCtx)
		_ = pool.CloseContext(ctx, firefoxCtx)
	})
}

func TestContextIsolationIntegration(t *testing.T) {
	t.Run("contexts from same browser are isolated", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Start failed: %v", err)
		}
		defer pool.Stop(ctx)

		// Create two contexts
		ctx1, err := pool.NewContext(ctx, BrowserChromium, ContextOptions{Headless: true})
		if err != nil {
			t.Fatalf("First NewContext failed: %v", err)
		}
		defer pool.CloseContext(ctx, ctx1)

		ctx2, err := pool.NewContext(ctx, BrowserChromium, ContextOptions{Headless: true})
		if err != nil {
			t.Fatalf("Second NewContext failed: %v", err)
		}
		defer pool.CloseContext(ctx, ctx2)

		// Create pages in each context
		page1, err := ctx1.NewPage()
		if err != nil {
			t.Fatalf("Failed to create page1: %v", err)
		}

		page2, err := ctx2.NewPage()
		if err != nil {
			t.Fatalf("Failed to create page2: %v", err)
		}

		// Set a cookie in context 1
		_, err = page1.Goto("about:blank")
		if err != nil {
			t.Fatalf("Failed to navigate page1: %v", err)
		}

		url := "https://example.com"
		err = ctx1.AddCookies([]playwright.OptionalCookie{
			{
				Name:  "test_cookie",
				Value: "context1_value",
				URL:   &url,
			},
		})
		if err != nil {
			t.Fatalf("Failed to add cookie in context1: %v", err)
		}

		// Check that context 2 doesn't have the cookie
		_, err = page2.Goto("about:blank")
		if err != nil {
			t.Fatalf("Failed to navigate page2: %v", err)
		}

		cookies2, err := ctx2.Cookies()
		if err != nil {
			t.Fatalf("Failed to get cookies from context2: %v", err)
		}

		for _, cookie := range cookies2 {
			if cookie.Name == "test_cookie" {
				t.Error("Context2 should not have context1's cookie - isolation broken")
			}
		}
	})
}

func TestPoolOptionsIntegration(t *testing.T) {
	t.Run("respects custom browser launch options", func(t *testing.T) {
		headless := true
		opts := PoolOptions{
			DefaultHeadless: true,
			ChromiumOptions: &BrowserLaunchOptions{
				Args:     []string{"--disable-gpu"},
				Headless: &headless,
			},
		}
		pool := NewBrowserPoolWithOptions(opts)
		ctx := context.Background()

		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Start failed: %v", err)
		}
		defer pool.Stop(ctx)

		// Create context - should use custom args
		browserCtx, err := pool.NewContext(ctx, BrowserChromium, ContextOptions{})
		if err != nil {
			t.Fatalf("NewContext failed: %v", err)
		}
		defer pool.CloseContext(ctx, browserCtx)

		// Basic check that browser works
		if browserCtx == nil {
			t.Error("BrowserContext should not be nil")
		}
	})
}

func TestStopCleansUpAllResources(t *testing.T) {
	t.Run("Stop closes all contexts and browsers", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		if err := pool.Start(ctx); err != nil {
			skipIfPlaywrightNotInstalled(t, err)
			t.Fatalf("Start failed: %v", err)
		}

		// Create contexts for multiple browsers
		_, err := pool.NewContext(ctx, BrowserChromium, ContextOptions{Headless: true})
		if err != nil {
			t.Fatalf("Chromium NewContext failed: %v", err)
		}

		_, err = pool.NewContext(ctx, BrowserChromium, ContextOptions{Headless: true})
		if err != nil {
			t.Fatalf("Second Chromium NewContext failed: %v", err)
		}

		// Verify active contexts
		stats := pool.Stats()
		if stats.Browsers[BrowserChromium].ActiveContexts != 2 {
			t.Errorf("ActiveContexts before Stop = %d, want 2", stats.Browsers[BrowserChromium].ActiveContexts)
		}

		// Stop should clean everything
		if err := pool.Stop(ctx); err != nil {
			t.Errorf("Stop failed: %v", err)
		}

		// Verify all cleaned up
		stats = pool.Stats()
		if stats.Browsers[BrowserChromium].Running {
			t.Error("Chromium should not be running after Stop")
		}
		if stats.Browsers[BrowserChromium].ActiveContexts != 0 {
			t.Errorf("ActiveContexts after Stop = %d, want 0", stats.Browsers[BrowserChromium].ActiveContexts)
		}
	})
}
