// Package browser provides browser instance pooling for Chromium, Firefox, and WebKit.
package browser

import (
	"context"

	"github.com/playwright-community/playwright-go"
)

// BrowserType represents supported browser engines.
type BrowserType string

const (
	// BrowserChromium represents the Chromium browser engine.
	BrowserChromium BrowserType = "chromium"
	// BrowserFirefox represents the Firefox browser engine.
	BrowserFirefox BrowserType = "firefox"
	// BrowserWebKit represents the WebKit browser engine.
	BrowserWebKit BrowserType = "webkit"
)

// Viewport defines browser viewport dimensions.
type Viewport struct {
	// Width is the viewport width in pixels.
	Width int
	// Height is the viewport height in pixels.
	Height int
}

// ContextOptions configures a new browser context.
type ContextOptions struct {
	// Headless determines whether the browser runs in headless mode.
	Headless bool
	// Viewport sets the browser viewport size. If nil, uses browser default.
	Viewport *Viewport
	// UserAgent overrides the default user agent string.
	UserAgent string
	// Locale sets the browser locale (e.g., "en-US").
	Locale string
	// TimezoneID sets the browser timezone (e.g., "America/New_York").
	TimezoneID string
	// Permissions grants browser permissions (e.g., "geolocation").
	Permissions []string
	// ExtraHeaders adds HTTP headers to all requests.
	ExtraHeaders map[string]string
}

// BrowserStats holds statistics for a single browser type.
type BrowserStats struct {
	// Running indicates whether the browser is currently launched.
	Running bool
	// ActiveContexts is the number of currently open browser contexts.
	ActiveContexts int
	// TotalCreated is the cumulative count of contexts created.
	TotalCreated int
	// TotalClosed is the cumulative count of contexts closed.
	TotalClosed int
}

// PoolStats aggregates statistics for all browser types.
type PoolStats struct {
	// Browsers maps each browser type to its statistics.
	Browsers map[BrowserType]BrowserStats
}

// BrowserPool manages shared browser instances with lazy initialization.
type BrowserPool interface {
	// Start initializes the Playwright runtime. Browsers are started lazily
	// on first NewContext call for each browser type.
	Start(ctx context.Context) error

	// Stop closes all browsers and the Playwright runtime. Should be called
	// during graceful shutdown.
	Stop(ctx context.Context) error

	// NewContext creates an isolated browser context for the specified browser
	// type. If the browser is not running, it will be launched first.
	NewContext(ctx context.Context, browserType BrowserType, opts ContextOptions) (playwright.BrowserContext, error)

	// CloseContext closes a browser context and updates pool statistics.
	CloseContext(ctx context.Context, browserContext playwright.BrowserContext) error

	// Stats returns current pool statistics per browser type.
	Stats() PoolStats
}

// NewBrowserPool creates a new browser pool instance.
func NewBrowserPool() BrowserPool {
	// Implementation will be added in TASK-006
	return nil
}
