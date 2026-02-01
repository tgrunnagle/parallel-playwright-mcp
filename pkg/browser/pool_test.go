package browser

import (
	"context"
	"sync"
	"testing"
)

func TestBrowserTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		bt       BrowserType
		expected string
	}{
		{
			name:     "chromium has correct string value",
			bt:       BrowserChromium,
			expected: "chromium",
		},
		{
			name:     "firefox has correct string value",
			bt:       BrowserFirefox,
			expected: "firefox",
		},
		{
			name:     "webkit has correct string value",
			bt:       BrowserWebKit,
			expected: "webkit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.bt) != tt.expected {
				t.Errorf("BrowserType = %q, want %q", tt.bt, tt.expected)
			}
		})
	}
}

func TestViewport(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{
			name:   "standard desktop viewport",
			width:  1920,
			height: 1080,
		},
		{
			name:   "mobile viewport",
			width:  375,
			height: 812,
		},
		{
			name:   "zero viewport",
			width:  0,
			height: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Viewport{Width: tt.width, Height: tt.height}
			if v.Width != tt.width {
				t.Errorf("Viewport.Width = %d, want %d", v.Width, tt.width)
			}
			if v.Height != tt.height {
				t.Errorf("Viewport.Height = %d, want %d", v.Height, tt.height)
			}
		})
	}
}

func TestContextOptions(t *testing.T) {
	t.Run("can instantiate with all field types", func(t *testing.T) {
		viewport := &Viewport{Width: 1280, Height: 720}
		opts := ContextOptions{
			Headless:     true,
			Viewport:     viewport,
			UserAgent:    "Test Agent/1.0",
			Locale:       "en-US",
			TimezoneID:   "America/New_York",
			Permissions:  []string{"geolocation", "notifications"},
			ExtraHeaders: map[string]string{"X-Custom-Header": "value"},
		}

		if !opts.Headless {
			t.Error("Headless should be true")
		}
		if opts.Viewport != viewport {
			t.Error("Viewport not set correctly")
		}
		if opts.UserAgent != "Test Agent/1.0" {
			t.Errorf("UserAgent = %q, want %q", opts.UserAgent, "Test Agent/1.0")
		}
		if opts.Locale != "en-US" {
			t.Errorf("Locale = %q, want %q", opts.Locale, "en-US")
		}
		if opts.TimezoneID != "America/New_York" {
			t.Errorf("TimezoneID = %q, want %q", opts.TimezoneID, "America/New_York")
		}
		if len(opts.Permissions) != 2 {
			t.Errorf("Permissions length = %d, want %d", len(opts.Permissions), 2)
		}
		if opts.ExtraHeaders["X-Custom-Header"] != "value" {
			t.Error("ExtraHeaders not set correctly")
		}
	})

	t.Run("can instantiate with zero values", func(t *testing.T) {
		opts := ContextOptions{}

		if opts.Headless {
			t.Error("Headless should be false by default")
		}
		if opts.Viewport != nil {
			t.Error("Viewport should be nil by default")
		}
		if opts.UserAgent != "" {
			t.Error("UserAgent should be empty by default")
		}
		if opts.Permissions != nil {
			t.Error("Permissions should be nil by default")
		}
		if opts.ExtraHeaders != nil {
			t.Error("ExtraHeaders should be nil by default")
		}
	})
}

func TestBrowserStats(t *testing.T) {
	t.Run("can instantiate with all fields", func(t *testing.T) {
		stats := BrowserStats{
			Running:        true,
			ActiveContexts: 5,
			TotalCreated:   100,
			TotalClosed:    95,
		}

		if !stats.Running {
			t.Error("Running should be true")
		}
		if stats.ActiveContexts != 5 {
			t.Errorf("ActiveContexts = %d, want %d", stats.ActiveContexts, 5)
		}
		if stats.TotalCreated != 100 {
			t.Errorf("TotalCreated = %d, want %d", stats.TotalCreated, 100)
		}
		if stats.TotalClosed != 95 {
			t.Errorf("TotalClosed = %d, want %d", stats.TotalClosed, 95)
		}
	})

	t.Run("can instantiate with zero values", func(t *testing.T) {
		stats := BrowserStats{}

		if stats.Running {
			t.Error("Running should be false by default")
		}
		if stats.ActiveContexts != 0 {
			t.Error("ActiveContexts should be 0 by default")
		}
		if stats.TotalCreated != 0 {
			t.Error("TotalCreated should be 0 by default")
		}
		if stats.TotalClosed != 0 {
			t.Error("TotalClosed should be 0 by default")
		}
	})
}

func TestPoolStats(t *testing.T) {
	t.Run("can hold BrowserStats for all browser types", func(t *testing.T) {
		poolStats := PoolStats{
			Browsers: map[BrowserType]BrowserStats{
				BrowserChromium: {Running: true, ActiveContexts: 3},
				BrowserFirefox:  {Running: true, ActiveContexts: 2},
				BrowserWebKit:   {Running: false, ActiveContexts: 0},
			},
		}

		if len(poolStats.Browsers) != 3 {
			t.Errorf("Browsers map length = %d, want %d", len(poolStats.Browsers), 3)
		}

		chromiumStats := poolStats.Browsers[BrowserChromium]
		if !chromiumStats.Running {
			t.Error("Chromium should be running")
		}
		if chromiumStats.ActiveContexts != 3 {
			t.Errorf("Chromium ActiveContexts = %d, want %d", chromiumStats.ActiveContexts, 3)
		}

		firefoxStats := poolStats.Browsers[BrowserFirefox]
		if !firefoxStats.Running {
			t.Error("Firefox should be running")
		}
		if firefoxStats.ActiveContexts != 2 {
			t.Errorf("Firefox ActiveContexts = %d, want %d", firefoxStats.ActiveContexts, 2)
		}

		webkitStats := poolStats.Browsers[BrowserWebKit]
		if webkitStats.Running {
			t.Error("WebKit should not be running")
		}
		if webkitStats.ActiveContexts != 0 {
			t.Errorf("WebKit ActiveContexts = %d, want %d", webkitStats.ActiveContexts, 0)
		}
	})

	t.Run("can instantiate with empty Browsers map", func(t *testing.T) {
		poolStats := PoolStats{Browsers: make(map[BrowserType]BrowserStats)}

		if poolStats.Browsers == nil {
			t.Error("Browsers map should not be nil")
		}
		if len(poolStats.Browsers) != 0 {
			t.Errorf("Browsers map length = %d, want %d", len(poolStats.Browsers), 0)
		}
	})
}

func TestNewBrowserPool(t *testing.T) {
	t.Run("returns non-nil pool instance", func(t *testing.T) {
		pool := NewBrowserPool()
		if pool == nil {
			t.Error("NewBrowserPool should return non-nil instance")
		}
	})

	t.Run("pool is in stopped state initially", func(t *testing.T) {
		pool := NewBrowserPool()
		stats := pool.Stats()

		// All browser types should have zero stats and not be running
		for _, bt := range []BrowserType{BrowserChromium, BrowserFirefox, BrowserWebKit} {
			browserStats := stats.Browsers[bt]
			if browserStats.Running {
				t.Errorf("%s should not be running initially", bt)
			}
			if browserStats.ActiveContexts != 0 {
				t.Errorf("%s ActiveContexts should be 0 initially", bt)
			}
			if browserStats.TotalCreated != 0 {
				t.Errorf("%s TotalCreated should be 0 initially", bt)
			}
			if browserStats.TotalClosed != 0 {
				t.Errorf("%s TotalClosed should be 0 initially", bt)
			}
		}
	})
}

func TestNewBrowserPoolWithOptions(t *testing.T) {
	t.Run("accepts custom options", func(t *testing.T) {
		headless := true
		opts := PoolOptions{
			DefaultHeadless: true,
			ChromiumOptions: &BrowserLaunchOptions{
				Args:     []string{"--disable-gpu"},
				Headless: &headless,
			},
		}
		pool := NewBrowserPoolWithOptions(opts)
		if pool == nil {
			t.Error("NewBrowserPoolWithOptions should return non-nil instance")
		}
	})

	t.Run("accepts empty options", func(t *testing.T) {
		pool := NewBrowserPoolWithOptions(PoolOptions{})
		if pool == nil {
			t.Error("NewBrowserPoolWithOptions with empty options should return non-nil instance")
		}
	})
}

func TestDefaultPoolOptions(t *testing.T) {
	opts := DefaultPoolOptions()

	if !opts.DefaultHeadless {
		t.Error("DefaultHeadless should be true")
	}
	if opts.ChromiumOptions != nil {
		t.Error("ChromiumOptions should be nil by default")
	}
	if opts.FirefoxOptions != nil {
		t.Error("FirefoxOptions should be nil by default")
	}
	if opts.WebKitOptions != nil {
		t.Error("WebKitOptions should be nil by default")
	}
}

func TestBrowserLaunchOptions(t *testing.T) {
	t.Run("can instantiate with all fields", func(t *testing.T) {
		headless := false
		slowMo := 100.0
		timeout := 30000.0
		opts := BrowserLaunchOptions{
			Args:           []string{"--disable-gpu", "--no-sandbox"},
			Headless:       &headless,
			SlowMo:         &slowMo,
			ExecutablePath: "/usr/bin/chromium",
			Timeout:        &timeout,
		}

		if len(opts.Args) != 2 {
			t.Errorf("Args length = %d, want %d", len(opts.Args), 2)
		}
		if *opts.Headless != false {
			t.Error("Headless should be false")
		}
		if *opts.SlowMo != 100.0 {
			t.Errorf("SlowMo = %f, want %f", *opts.SlowMo, 100.0)
		}
		if opts.ExecutablePath != "/usr/bin/chromium" {
			t.Errorf("ExecutablePath = %q, want %q", opts.ExecutablePath, "/usr/bin/chromium")
		}
		if *opts.Timeout != 30000.0 {
			t.Errorf("Timeout = %f, want %f", *opts.Timeout, 30000.0)
		}
	})

	t.Run("can instantiate with zero values", func(t *testing.T) {
		opts := BrowserLaunchOptions{}

		if opts.Args != nil {
			t.Error("Args should be nil by default")
		}
		if opts.Headless != nil {
			t.Error("Headless should be nil by default")
		}
		if opts.SlowMo != nil {
			t.Error("SlowMo should be nil by default")
		}
		if opts.ExecutablePath != "" {
			t.Error("ExecutablePath should be empty by default")
		}
		if opts.Timeout != nil {
			t.Error("Timeout should be nil by default")
		}
	})
}

func TestStartStop(t *testing.T) {
	t.Run("Stop returns error if not running", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		// Stop without Start should fail
		err := pool.Stop(ctx)
		if err != ErrPoolNotRunning {
			t.Errorf("Stop error = %v, want %v", err, ErrPoolNotRunning)
		}
	})
}

func TestNewContext(t *testing.T) {
	t.Run("returns error if pool not running", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		_, err := pool.NewContext(ctx, BrowserChromium, ContextOptions{Headless: true})
		if err != ErrPoolNotRunning {
			t.Errorf("NewContext error = %v, want %v", err, ErrPoolNotRunning)
		}
	})
}

func TestCloseContext(t *testing.T) {
	t.Run("returns error for nil context", func(t *testing.T) {
		pool := NewBrowserPool()
		ctx := context.Background()

		err := pool.CloseContext(ctx, nil)
		if err != ErrNilBrowserContext {
			t.Errorf("CloseContext error = %v, want %v", err, ErrNilBrowserContext)
		}
	})
}

func TestStats(t *testing.T) {
	t.Run("returns all browser types in map", func(t *testing.T) {
		pool := NewBrowserPool()
		stats := pool.Stats()

		if len(stats.Browsers) != 3 {
			t.Errorf("Browsers map length = %d, want %d", len(stats.Browsers), 3)
		}

		for _, bt := range []BrowserType{BrowserChromium, BrowserFirefox, BrowserWebKit} {
			if _, ok := stats.Browsers[bt]; !ok {
				t.Errorf("Browsers map missing key %q", bt)
			}
		}
	})

	t.Run("returns copy of stats (immutable)", func(t *testing.T) {
		pool := NewBrowserPool()
		stats1 := pool.Stats()
		stats2 := pool.Stats()

		// Modify the first stats
		stats1.Browsers[BrowserChromium] = BrowserStats{Running: true, TotalCreated: 999}

		// Second stats should not be affected
		if stats2.Browsers[BrowserChromium].Running {
			t.Error("Stats should return a copy, not a reference")
		}
		if stats2.Browsers[BrowserChromium].TotalCreated != 0 {
			t.Error("Stats should return a copy, not a reference")
		}
	})

	t.Run("is thread-safe", func(t *testing.T) {
		pool := NewBrowserPool()
		var wg sync.WaitGroup

		// Call Stats concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = pool.Stats()
			}()
		}

		wg.Wait()
	})
}

func TestIsValidBrowserType(t *testing.T) {
	tests := []struct {
		name     string
		bt       BrowserType
		expected bool
	}{
		{"chromium is valid", BrowserChromium, true},
		{"firefox is valid", BrowserFirefox, true},
		{"webkit is valid", BrowserWebKit, true},
		{"safari is invalid", BrowserType("safari"), false},
		{"empty is invalid", BrowserType(""), false},
		{"unknown is invalid", BrowserType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidBrowserType(tt.bt); got != tt.expected {
				t.Errorf("isValidBrowserType(%q) = %v, want %v", tt.bt, got, tt.expected)
			}
		})
	}
}

