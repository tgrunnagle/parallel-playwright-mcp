package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/config"
)

// TestFullConfigPipeline tests the complete config loading process:
// YAML file loading + environment variable overrides + validation.
func TestFullConfigPipeline(t *testing.T) {
	// Create a sample config file
	yamlContent := `
server:
  host: "0.0.0.0"
  port: 8080

browser:
  defaultType: "firefox"
  headless: false
  slowMo: 50
  chromium:
    args: ["--disable-gpu", "--no-sandbox"]
  firefox:
    args: []
  webkit:
    args: []

session:
  maxPerConnection: 5
  maxTotal: 20
  defaultTimeout: 45s
  idleTimeout: 3m

logging:
  level: "debug"
  format: "text"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set environment variable overrides
	t.Setenv("MCP_HOST", "127.0.0.1")       // Override host from file
	t.Setenv("MCP_BROWSER_HEADLESS", "true") // Override headless from file
	t.Setenv("MCP_LOG_LEVEL", "info")        // Override log level from file

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify env var overrides took precedence
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, want %q (env override)", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Browser.Headless != true {
		t.Errorf("Browser.Headless = %v, want %v (env override)", cfg.Browser.Headless, true)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want %q (env override)", cfg.Logging.Level, "info")
	}

	// Verify file values that weren't overridden
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d (from file)", cfg.Server.Port, 8080)
	}
	if cfg.Browser.DefaultType != "firefox" {
		t.Errorf("Browser.DefaultType = %q, want %q (from file)", cfg.Browser.DefaultType, "firefox")
	}
	if cfg.Browser.SlowMo != 50 {
		t.Errorf("Browser.SlowMo = %d, want %d (from file)", cfg.Browser.SlowMo, 50)
	}
	if cfg.Session.MaxPerConnection != 5 {
		t.Errorf("Session.MaxPerConnection = %d, want %d (from file)", cfg.Session.MaxPerConnection, 5)
	}
	if cfg.Session.MaxTotal != 20 {
		t.Errorf("Session.MaxTotal = %d, want %d (from file)", cfg.Session.MaxTotal, 20)
	}
	if cfg.Session.DefaultTimeout != 45*time.Second {
		t.Errorf("Session.DefaultTimeout = %v, want %v (from file)", cfg.Session.DefaultTimeout, 45*time.Second)
	}
	if cfg.Session.IdleTimeout != 3*time.Minute {
		t.Errorf("Session.IdleTimeout = %v, want %v (from file)", cfg.Session.IdleTimeout, 3*time.Minute)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %q, want %q (from file)", cfg.Logging.Format, "text")
	}

	// Verify browser args from file
	expectedChromiumArgs := []string{"--disable-gpu", "--no-sandbox"}
	if len(cfg.Browser.Chromium.Args) != len(expectedChromiumArgs) {
		t.Errorf("Browser.Chromium.Args = %v, want %v", cfg.Browser.Chromium.Args, expectedChromiumArgs)
	} else {
		for i, arg := range expectedChromiumArgs {
			if cfg.Browser.Chromium.Args[i] != arg {
				t.Errorf("Browser.Chromium.Args[%d] = %q, want %q", i, cfg.Browser.Chromium.Args[i], arg)
			}
		}
	}
}

// TestConfigPipelineWithValidationError tests that validation errors are
// properly returned through the full pipeline.
func TestConfigPipelineWithValidationError(t *testing.T) {
	yamlContent := `
server:
  port: 99999
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := config.Load(configPath)
	if err == nil {
		t.Error("Load() expected error for invalid port, got nil")
	}
}

// TestConfigPipelineDefaultsOnly tests loading with no file and no env vars.
func TestConfigPipelineDefaultsOnly(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify all defaults are applied
	if cfg.Server.Host != config.DefaultHost {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, config.DefaultHost)
	}
	if cfg.Server.Port != config.DefaultPort {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, config.DefaultPort)
	}
	if cfg.Browser.DefaultType != config.DefaultBrowserType {
		t.Errorf("Browser.DefaultType = %q, want %q", cfg.Browser.DefaultType, config.DefaultBrowserType)
	}
	if cfg.Browser.Headless != config.DefaultHeadless {
		t.Errorf("Browser.Headless = %v, want %v", cfg.Browser.Headless, config.DefaultHeadless)
	}
	if cfg.Session.MaxPerConnection != config.DefaultMaxPerConnection {
		t.Errorf("Session.MaxPerConnection = %d, want %d", cfg.Session.MaxPerConnection, config.DefaultMaxPerConnection)
	}
	if cfg.Session.MaxTotal != config.DefaultMaxTotal {
		t.Errorf("Session.MaxTotal = %d, want %d", cfg.Session.MaxTotal, config.DefaultMaxTotal)
	}
	if cfg.Session.DefaultTimeout != config.DefaultTimeout {
		t.Errorf("Session.DefaultTimeout = %v, want %v", cfg.Session.DefaultTimeout, config.DefaultTimeout)
	}
	if cfg.Session.IdleTimeout != config.DefaultIdleTimeout {
		t.Errorf("Session.IdleTimeout = %v, want %v", cfg.Session.IdleTimeout, config.DefaultIdleTimeout)
	}
	if cfg.Logging.Level != config.DefaultLogLevel {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, config.DefaultLogLevel)
	}
	if cfg.Logging.Format != config.DefaultLogFormat {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, config.DefaultLogFormat)
	}
}
