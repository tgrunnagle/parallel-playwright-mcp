package browser

import "testing"

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
	t.Run("returns nil for now", func(t *testing.T) {
		pool := NewBrowserPool()
		if pool != nil {
			t.Error("NewBrowserPool should return nil until implementation")
		}
	})
}
