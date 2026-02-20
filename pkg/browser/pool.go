// Package browser provides browser instance pooling for Chromium, Firefox, and WebKit.
package browser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/playwright-community/playwright-go"
)

// Error definitions for browser pool operations.
var (
	// ErrPoolNotRunning indicates an operation was attempted on a stopped pool.
	ErrPoolNotRunning = errors.New("browser pool is not running")
	// ErrPoolAlreadyRunning indicates Start() was called on an already running pool.
	ErrPoolAlreadyRunning = errors.New("browser pool is already running")
	// ErrInvalidBrowserType indicates an unsupported browser type was requested.
	ErrInvalidBrowserType = errors.New("invalid browser type")
	// ErrNilBrowserContext indicates a nil browser context was provided.
	ErrNilBrowserContext = errors.New("browser context is nil")
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

// defaultUserAgents provides realistic user agent strings per browser type
// so that automated sessions appear as normal browser traffic.
var defaultUserAgents = map[BrowserType]string{
	BrowserChromium: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	BrowserFirefox:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
	BrowserWebKit:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7_2) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.2 Safari/605.1.15",
}

// Viewport defines browser viewport dimensions.
type Viewport struct {
	// Width is the viewport width in pixels.
	Width int
	// Height is the viewport height in pixels.
	Height int
}

// ContextOptions configures a new browser context.
type ContextOptions struct {
	// Headless is reserved for future use. In Playwright, headless mode is a
	// browser-level setting configured via BrowserLaunchOptions, not per-context.
	// This field is currently ignored.
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

// BrowserLaunchOptions configures browser launch behavior per browser type.
type BrowserLaunchOptions struct {
	// Args are command-line arguments passed to the browser process.
	Args []string
	// Headless runs the browser in headless mode. If nil, uses pool default.
	Headless *bool
	// SlowMo slows down operations by the specified milliseconds for debugging.
	SlowMo *float64
	// ExecutablePath specifies a custom browser executable path.
	ExecutablePath string
	// Channel specifies the browser distribution channel.
	// Use "chromium" to opt in to new headless mode (uses full Chromium instead of headless_shell).
	Channel string
	// Timeout specifies the maximum time to wait for browser launch in milliseconds.
	Timeout *float64
	// UserAgent overrides the default user agent string for contexts created with this browser.
	UserAgent string
}

// PoolOptions configures the browser pool.
type PoolOptions struct {
	// DefaultHeadless sets the default headless mode for all browsers (default: true).
	DefaultHeadless bool
	// ChromiumOptions are launch options specific to Chromium.
	ChromiumOptions *BrowserLaunchOptions
	// FirefoxOptions are launch options specific to Firefox.
	FirefoxOptions *BrowserLaunchOptions
	// WebKitOptions are launch options specific to WebKit.
	WebKitOptions *BrowserLaunchOptions
}

// DefaultPoolOptions returns sensible default options.
func DefaultPoolOptions() PoolOptions {
	return PoolOptions{
		DefaultHeadless: true,
	}
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

// browserPool is the concrete implementation of BrowserPool.
type browserPool struct {
	mu              sync.RWMutex
	playwright      *playwright.Playwright
	browsers        map[BrowserType]playwright.Browser
	stats           map[BrowserType]*BrowserStats
	contextTypes    map[playwright.BrowserContext]BrowserType
	launchOptions   map[BrowserType]*BrowserLaunchOptions
	defaultHeadless bool
	running         bool
}

// NewBrowserPool creates a new browser pool instance with default options.
func NewBrowserPool() BrowserPool {
	return NewBrowserPoolWithOptions(DefaultPoolOptions())
}

// NewBrowserPoolWithOptions creates a new browser pool instance with the given options.
func NewBrowserPoolWithOptions(opts PoolOptions) BrowserPool {
	launchOpts := make(map[BrowserType]*BrowserLaunchOptions)
	if opts.ChromiumOptions != nil {
		launchOpts[BrowserChromium] = opts.ChromiumOptions
	}
	if opts.FirefoxOptions != nil {
		launchOpts[BrowserFirefox] = opts.FirefoxOptions
	}
	if opts.WebKitOptions != nil {
		launchOpts[BrowserWebKit] = opts.WebKitOptions
	}

	return &browserPool{
		browsers:        make(map[BrowserType]playwright.Browser),
		stats:           make(map[BrowserType]*BrowserStats),
		contextTypes:    make(map[playwright.BrowserContext]BrowserType),
		launchOptions:   launchOpts,
		defaultHeadless: opts.DefaultHeadless,
	}
}

// Start initializes the Playwright runtime.
func (p *browserPool) Start(ctx context.Context) error {
	slog.Debug("browser pool starting")

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		slog.Error("browser pool already running")
		return ErrPoolAlreadyRunning
	}

	pw, err := playwright.Run()
	if err != nil {
		slog.Error("failed to start Playwright runtime", "error", err)
		return fmt.Errorf("failed to start Playwright: %w", err)
	}

	p.playwright = pw
	p.running = true
	slog.Info("browser pool started")
	return nil
}

// Stop closes all browsers and the Playwright runtime.
func (p *browserPool) Stop(ctx context.Context) error {
	slog.Debug("browser pool stopping")

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		slog.Error("browser pool not running")
		return ErrPoolNotRunning
	}

	var errs []error

	// Close all browser contexts first and update statistics
	slog.Debug("closing browser contexts", "count", len(p.contextTypes))
	for browserCtx, browserType := range p.contextTypes {
		if err := browserCtx.Close(); err != nil {
			slog.Error("failed to close browser context", "browserType", browserType, "error", err)
			errs = append(errs, fmt.Errorf("failed to close browser context: %w", err))
		}
		if stats, ok := p.stats[browserType]; ok {
			stats.TotalClosed++
		}
	}
	p.contextTypes = make(map[playwright.BrowserContext]BrowserType)

	// Close all running browsers
	for browserType, browser := range p.browsers {
		if browser != nil {
			slog.Debug("closing browser", "browserType", browserType)
			if err := browser.Close(); err != nil {
				slog.Error("failed to close browser", "browserType", browserType, "error", err)
				errs = append(errs, fmt.Errorf("failed to close %s: %w", browserType, err))
			}
			if stats, ok := p.stats[browserType]; ok {
				stats.Running = false
				stats.ActiveContexts = 0
			}
		}
	}
	p.browsers = make(map[BrowserType]playwright.Browser)

	// Stop Playwright runtime
	if p.playwright != nil {
		if err := p.playwright.Stop(); err != nil {
			slog.Error("failed to stop Playwright runtime", "error", err)
			errs = append(errs, fmt.Errorf("failed to stop Playwright: %w", err))
		}
		p.playwright = nil
	}

	p.running = false
	slog.Info("browser pool stopped")
	return errors.Join(errs...)
}

// NewContext creates an isolated browser context for the specified browser type.
func (p *browserPool) NewContext(ctx context.Context, browserType BrowserType, opts ContextOptions) (playwright.BrowserContext, error) {
	slog.Debug("creating new browser context", "browserType", browserType)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		slog.Error("cannot create context: pool not running", "browserType", browserType)
		return nil, ErrPoolNotRunning
	}

	if !isValidBrowserType(browserType) {
		slog.Error("invalid browser type requested", "browserType", browserType)
		return nil, fmt.Errorf("%w: %s", ErrInvalidBrowserType, browserType)
	}

	// Lazy launch browser if not already running
	browser, ok := p.browsers[browserType]
	if !ok {
		slog.Info("lazy-launching browser engine", "browserType", browserType)
		var err error
		browser, err = p.launchBrowser(browserType)
		if err != nil {
			slog.Error("failed to launch browser engine", "browserType", browserType, "error", err)
			return nil, fmt.Errorf("failed to launch %s: %w", browserType, err)
		}
		p.browsers[browserType] = browser

		// Initialize stats for this browser type
		if p.stats[browserType] == nil {
			p.stats[browserType] = &BrowserStats{}
		}
		p.stats[browserType].Running = true
		slog.Info("browser engine launched", "browserType", browserType)
	}

	// Build context options
	contextOpts := playwright.BrowserNewContextOptions{}
	if opts.Viewport != nil {
		contextOpts.Viewport = &playwright.Size{
			Width:  opts.Viewport.Width,
			Height: opts.Viewport.Height,
		}
	}
	// User agent priority: per-session opts > per-browser config > hardcoded default
	switch {
	case opts.UserAgent != "":
		contextOpts.UserAgent = playwright.String(opts.UserAgent)
	case p.launchOptions[browserType] != nil && p.launchOptions[browserType].UserAgent != "":
		contextOpts.UserAgent = playwright.String(p.launchOptions[browserType].UserAgent)
	default:
		if ua, ok := defaultUserAgents[browserType]; ok {
			contextOpts.UserAgent = playwright.String(ua)
		}
	}
	if opts.Locale != "" {
		contextOpts.Locale = playwright.String(opts.Locale)
	}
	if opts.TimezoneID != "" {
		contextOpts.TimezoneId = playwright.String(opts.TimezoneID)
	}
	if len(opts.Permissions) > 0 {
		contextOpts.Permissions = opts.Permissions
	}
	if len(opts.ExtraHeaders) > 0 {
		contextOpts.ExtraHttpHeaders = opts.ExtraHeaders
	}

	// Create the browser context
	browserContext, err := browser.NewContext(contextOpts)
	if err != nil {
		slog.Error("failed to create browser context", "browserType", browserType, "error", err)
		return nil, fmt.Errorf("failed to create browser context: %w", err)
	}

	// Track context ownership
	p.contextTypes[browserContext] = browserType

	// Update statistics
	if p.stats[browserType] == nil {
		p.stats[browserType] = &BrowserStats{Running: true}
	}
	p.stats[browserType].TotalCreated++
	p.stats[browserType].ActiveContexts++

	slog.Debug("browser context created", "browserType", browserType, "activeContexts", p.stats[browserType].ActiveContexts)
	return browserContext, nil
}

// CloseContext closes a browser context and updates pool statistics.
func (p *browserPool) CloseContext(ctx context.Context, browserContext playwright.BrowserContext) error {
	slog.Debug("closing browser context")

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if browserContext == nil {
		slog.Error("cannot close nil browser context")
		return ErrNilBrowserContext
	}

	// Get browser type before closing (while we can still identify it)
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		slog.Error("cannot close context: pool not running")
		return ErrPoolNotRunning
	}
	browserType, tracked := p.contextTypes[browserContext]
	p.mu.Unlock()

	// Close the context
	if err := browserContext.Close(); err != nil {
		slog.Error("failed to close browser context", "browserType", browserType, "error", err)
		return fmt.Errorf("failed to close browser context: %w", err)
	}

	// Update statistics if this context was tracked
	p.mu.Lock()
	defer p.mu.Unlock()

	if tracked {
		delete(p.contextTypes, browserContext)
		if stats, ok := p.stats[browserType]; ok {
			stats.ActiveContexts--
			stats.TotalClosed++
		}
		slog.Debug("browser context closed", "browserType", browserType)
	}

	return nil
}

// Stats returns current pool statistics per browser type.
func (p *browserPool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := PoolStats{
		Browsers: make(map[BrowserType]BrowserStats),
	}

	// Return stats for all browser types
	for _, bt := range []BrowserType{BrowserChromium, BrowserFirefox, BrowserWebKit} {
		if stats, ok := p.stats[bt]; ok {
			result.Browsers[bt] = BrowserStats{
				Running:        stats.Running,
				ActiveContexts: stats.ActiveContexts,
				TotalCreated:   stats.TotalCreated,
				TotalClosed:    stats.TotalClosed,
			}
		} else {
			result.Browsers[bt] = BrowserStats{}
		}
	}

	return result
}

// launchBrowser launches the specified browser type with configured options.
func (p *browserPool) launchBrowser(browserType BrowserType) (playwright.Browser, error) {
	launchOpts := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(p.defaultHeadless),
	}

	// Apply browser-specific options if configured
	if opts, ok := p.launchOptions[browserType]; ok && opts != nil {
		if len(opts.Args) > 0 {
			launchOpts.Args = opts.Args
			slog.Info("applying browser launch args", "browserType", browserType, "args", opts.Args)
		}
		if opts.Headless != nil {
			launchOpts.Headless = opts.Headless
		}
		if opts.SlowMo != nil {
			launchOpts.SlowMo = opts.SlowMo
		}
		if opts.ExecutablePath != "" {
			launchOpts.ExecutablePath = playwright.String(opts.ExecutablePath)
		}
		if opts.Channel != "" {
			launchOpts.Channel = playwright.String(opts.Channel)
			slog.Info("applying browser channel", "browserType", browserType, "channel", opts.Channel)
		}
		if opts.Timeout != nil {
			launchOpts.Timeout = opts.Timeout
		}
	}

	switch browserType {
	case BrowserChromium:
		return p.playwright.Chromium.Launch(launchOpts)
	case BrowserFirefox:
		return p.playwright.Firefox.Launch(launchOpts)
	case BrowserWebKit:
		return p.playwright.WebKit.Launch(launchOpts)
	default:
		return nil, ErrInvalidBrowserType
	}
}

// isValidBrowserType checks if the given browser type is supported.
func isValidBrowserType(bt BrowserType) bool {
	switch bt {
	case BrowserChromium, BrowserFirefox, BrowserWebKit:
		return true
	default:
		return false
	}
}
