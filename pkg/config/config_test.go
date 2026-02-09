package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadWithDefaults(t *testing.T) {
	cfg := LoadWithDefaults()

	// Server defaults
	if cfg.Server.Host != DefaultHost {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, DefaultHost)
	}
	if cfg.Server.Port != DefaultPort {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, DefaultPort)
	}

	// Browser defaults
	if cfg.Browser.DefaultType != DefaultBrowserType {
		t.Errorf("Browser.DefaultType = %q, want %q", cfg.Browser.DefaultType, DefaultBrowserType)
	}
	if cfg.Browser.Headless != DefaultHeadless {
		t.Errorf("Browser.Headless = %v, want %v", cfg.Browser.Headless, DefaultHeadless)
	}
	if cfg.Browser.SlowMo != DefaultSlowMo {
		t.Errorf("Browser.SlowMo = %d, want %d", cfg.Browser.SlowMo, DefaultSlowMo)
	}

	// Session defaults
	if cfg.Session.MaxPerConnection != DefaultMaxPerConnection {
		t.Errorf("Session.MaxPerConnection = %d, want %d", cfg.Session.MaxPerConnection, DefaultMaxPerConnection)
	}
	if cfg.Session.MaxTotal != DefaultMaxTotal {
		t.Errorf("Session.MaxTotal = %d, want %d", cfg.Session.MaxTotal, DefaultMaxTotal)
	}
	if cfg.Session.DefaultTimeout != DefaultTimeout {
		t.Errorf("Session.DefaultTimeout = %v, want %v", cfg.Session.DefaultTimeout, DefaultTimeout)
	}
	if cfg.Session.IdleTimeout != DefaultIdleTimeout {
		t.Errorf("Session.IdleTimeout = %v, want %v", cfg.Session.IdleTimeout, DefaultIdleTimeout)
	}

	// Logging defaults
	if cfg.Logging.Level != DefaultLogLevel {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, DefaultLogLevel)
	}
	if cfg.Logging.Format != DefaultLogFormat {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, DefaultLogFormat)
	}

	// Retry defaults
	if cfg.Retry.MaxAttempts != DefaultMaxAttempts {
		t.Errorf("Retry.MaxAttempts = %d, want %d", cfg.Retry.MaxAttempts, DefaultMaxAttempts)
	}
	if cfg.Retry.InitialBackoff != DefaultInitialBackoff {
		t.Errorf("Retry.InitialBackoff = %v, want %v", cfg.Retry.InitialBackoff, DefaultInitialBackoff)
	}
	if cfg.Retry.MaxBackoff != DefaultMaxBackoff {
		t.Errorf("Retry.MaxBackoff = %v, want %v", cfg.Retry.MaxBackoff, DefaultMaxBackoff)
	}
	if cfg.Retry.BackoffFactor != DefaultBackoffFactor {
		t.Errorf("Retry.BackoffFactor = %v, want %v", cfg.Retry.BackoffFactor, DefaultBackoffFactor)
	}
	if cfg.Retry.Jitter != DefaultJitter {
		t.Errorf("Retry.Jitter = %v, want %v", cfg.Retry.Jitter, DefaultJitter)
	}
}

func TestLoadValidYAMLFile(t *testing.T) {
	yamlContent := `
server:
  host: "0.0.0.0"
  port: 8080
browser:
  defaultType: "firefox"
  headless: false
  slowMo: 100
session:
  maxPerConnection: 5
  maxTotal: 25
  defaultTimeout: 60s
  idleTimeout: 10m
logging:
  level: "debug"
  format: "text"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Browser.DefaultType != "firefox" {
		t.Errorf("Browser.DefaultType = %q, want %q", cfg.Browser.DefaultType, "firefox")
	}
	if cfg.Browser.Headless != false {
		t.Errorf("Browser.Headless = %v, want %v", cfg.Browser.Headless, false)
	}
	if cfg.Browser.SlowMo != 100 {
		t.Errorf("Browser.SlowMo = %d, want %d", cfg.Browser.SlowMo, 100)
	}
	if cfg.Session.MaxPerConnection != 5 {
		t.Errorf("Session.MaxPerConnection = %d, want %d", cfg.Session.MaxPerConnection, 5)
	}
	if cfg.Session.MaxTotal != 25 {
		t.Errorf("Session.MaxTotal = %d, want %d", cfg.Session.MaxTotal, 25)
	}
	if cfg.Session.DefaultTimeout != 60*time.Second {
		t.Errorf("Session.DefaultTimeout = %v, want %v", cfg.Session.DefaultTimeout, 60*time.Second)
	}
	if cfg.Session.IdleTimeout != 10*time.Minute {
		t.Errorf("Session.IdleTimeout = %v, want %v", cfg.Session.IdleTimeout, 10*time.Minute)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, "text")
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v, expected nil (should use defaults)", err)
	}

	if cfg.Server.Port != DefaultPort {
		t.Errorf("Server.Port = %d, want %d (default)", cfg.Server.Port, DefaultPort)
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("invalid: [yaml: content"), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for malformed YAML, got nil")
	}
}

func TestLoadPartialYAMLMergesWithDefaults(t *testing.T) {
	yamlContent := `
server:
  port: 9000
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Specified value
	if cfg.Server.Port != 9000 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9000)
	}
	// Default value (not specified in YAML)
	if cfg.Server.Host != DefaultHost {
		t.Errorf("Server.Host = %q, want %q (default)", cfg.Server.Host, DefaultHost)
	}
	if cfg.Browser.DefaultType != DefaultBrowserType {
		t.Errorf("Browser.DefaultType = %q, want %q (default)", cfg.Browser.DefaultType, DefaultBrowserType)
	}
}

func TestLoadEmptyYAMLUsesDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != DefaultPort {
		t.Errorf("Server.Port = %d, want %d (default)", cfg.Server.Port, DefaultPort)
	}
}

func TestLoadConfigPathFromEnv(t *testing.T) {
	yamlContent := `
server:
  port: 7777
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom-config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	t.Setenv("MCP_CONFIG_PATH", configPath)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 7777 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 7777)
	}
}

func TestLoadConfigPathFromEnvNonExistentFileUsesDefaults(t *testing.T) {
	// When MCP_CONFIG_PATH points to a non-existent file, defaults are used.
	// This is intentional to allow configuration paths to be set in deployment
	// environments where the config file might not always exist.
	t.Setenv("MCP_CONFIG_PATH", "/nonexistent/path/config.yaml")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v, expected nil (should use defaults)", err)
	}

	if cfg.Server.Port != DefaultPort {
		t.Errorf("Server.Port = %d, want %d (default)", cfg.Server.Port, DefaultPort)
	}
	if cfg.Server.Host != DefaultHost {
		t.Errorf("Server.Host = %q, want %q (default)", cfg.Server.Host, DefaultHost)
	}
}

func TestEnvironmentVariableOverrides(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		checkFn  func(*Config) bool
		wantDesc string
	}{
		{
			name:     "MCP_HOST override",
			envVars:  map[string]string{"MCP_HOST": "192.168.1.100"},
			checkFn:  func(c *Config) bool { return c.Server.Host == "192.168.1.100" },
			wantDesc: "Server.Host = 192.168.1.100",
		},
		{
			name:     "MCP_PORT override",
			envVars:  map[string]string{"MCP_PORT": "9999"},
			checkFn:  func(c *Config) bool { return c.Server.Port == 9999 },
			wantDesc: "Server.Port = 9999",
		},
		{
			name:     "MCP_BROWSER_HEADLESS=false",
			envVars:  map[string]string{"MCP_BROWSER_HEADLESS": "false"},
			checkFn:  func(c *Config) bool { return c.Browser.Headless == false },
			wantDesc: "Browser.Headless = false",
		},
		{
			name:     "MCP_BROWSER_HEADLESS=true",
			envVars:  map[string]string{"MCP_BROWSER_HEADLESS": "true"},
			checkFn:  func(c *Config) bool { return c.Browser.Headless == true },
			wantDesc: "Browser.Headless = true",
		},
		{
			name:     "MCP_BROWSER_HEADLESS=0",
			envVars:  map[string]string{"MCP_BROWSER_HEADLESS": "0"},
			checkFn:  func(c *Config) bool { return c.Browser.Headless == false },
			wantDesc: "Browser.Headless = false (from 0)",
		},
		{
			name:     "MCP_BROWSER_HEADLESS=1",
			envVars:  map[string]string{"MCP_BROWSER_HEADLESS": "1"},
			checkFn:  func(c *Config) bool { return c.Browser.Headless == true },
			wantDesc: "Browser.Headless = true (from 1)",
		},
		{
			name:     "MCP_LOG_LEVEL override",
			envVars:  map[string]string{"MCP_LOG_LEVEL": "DEBUG"},
			checkFn:  func(c *Config) bool { return c.Logging.Level == "debug" },
			wantDesc: "Logging.Level = debug (lowercased)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg, err := Load("/nonexistent/path/config.yaml")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if !tt.checkFn(cfg) {
				t.Errorf("Expected %s", tt.wantDesc)
			}
		})
	}
}

func TestEnvironmentVariableOverridesYAML(t *testing.T) {
	yamlContent := `
server:
  host: "10.0.0.1"
  port: 5000
browser:
  headless: false
logging:
  level: "warn"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	// Set env vars to override YAML values
	t.Setenv("MCP_HOST", "envhost")
	t.Setenv("MCP_PORT", "6000")
	t.Setenv("MCP_BROWSER_HEADLESS", "true")
	t.Setenv("MCP_LOG_LEVEL", "error")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Env vars should take precedence
	if cfg.Server.Host != "envhost" {
		t.Errorf("Server.Host = %q, want %q (env override)", cfg.Server.Host, "envhost")
	}
	if cfg.Server.Port != 6000 {
		t.Errorf("Server.Port = %d, want %d (env override)", cfg.Server.Port, 6000)
	}
	if cfg.Browser.Headless != true {
		t.Errorf("Browser.Headless = %v, want %v (env override)", cfg.Browser.Headless, true)
	}
	if cfg.Logging.Level != "error" {
		t.Errorf("Logging.Level = %q, want %q (env override)", cfg.Logging.Level, "error")
	}
}

func TestInvalidEnvVarPortReturnsError(t *testing.T) {
	t.Setenv("MCP_PORT", "not-a-number")

	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() expected error for invalid MCP_PORT, got nil")
	}
}

func TestInvalidEnvVarHeadlessReturnsError(t *testing.T) {
	t.Setenv("MCP_BROWSER_HEADLESS", "not-a-bool")

	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() expected error for invalid MCP_BROWSER_HEADLESS, got nil")
	}
}

func TestValidateValidConfig(t *testing.T) {
	cfg := LoadWithDefaults()
	errs := cfg.Validate()
	if len(errs) > 0 {
		t.Errorf("Validate() returned errors for valid config: %v", errs)
	}
}

func TestValidatePortRange(t *testing.T) {
	tests := []struct {
		port    int
		wantErr bool
	}{
		{port: 0, wantErr: true},
		{port: -1, wantErr: true},
		{port: 1, wantErr: false},
		{port: 3000, wantErr: false},
		{port: 65535, wantErr: false},
		{port: 65536, wantErr: true},
		{port: 100000, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("port_%d", tt.port), func(t *testing.T) {
			cfg := LoadWithDefaults()
			cfg.Server.Port = tt.port
			errs := cfg.Validate()

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("Validate() for port %d: hasErr = %v, wantErr = %v, errs = %v", tt.port, hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestValidateBrowserType(t *testing.T) {
	tests := []struct {
		browserType string
		wantErr     bool
	}{
		{browserType: "chromium", wantErr: false},
		{browserType: "firefox", wantErr: false},
		{browserType: "webkit", wantErr: false},
		{browserType: "chrome", wantErr: true},
		{browserType: "safari", wantErr: true},
		{browserType: "", wantErr: true},
		{browserType: "invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.browserType, func(t *testing.T) {
			cfg := LoadWithDefaults()
			cfg.Browser.DefaultType = tt.browserType
			errs := cfg.Validate()

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("Validate() for browserType %q: hasErr = %v, wantErr = %v", tt.browserType, hasErr, tt.wantErr)
			}
		})
	}
}

func TestValidateTimeouts(t *testing.T) {
	tests := []struct {
		name           string
		defaultTimeout time.Duration
		idleTimeout    time.Duration
		wantErr        bool
	}{
		{name: "valid timeouts", defaultTimeout: 30 * time.Second, idleTimeout: 5 * time.Minute, wantErr: false},
		{name: "zero defaultTimeout", defaultTimeout: 0, idleTimeout: 5 * time.Minute, wantErr: true},
		{name: "negative defaultTimeout", defaultTimeout: -1 * time.Second, idleTimeout: 5 * time.Minute, wantErr: true},
		{name: "zero idleTimeout", defaultTimeout: 30 * time.Second, idleTimeout: 0, wantErr: true},
		{name: "negative idleTimeout", defaultTimeout: 30 * time.Second, idleTimeout: -1 * time.Minute, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoadWithDefaults()
			cfg.Session.DefaultTimeout = tt.defaultTimeout
			cfg.Session.IdleTimeout = tt.idleTimeout
			errs := cfg.Validate()

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("Validate(): hasErr = %v, wantErr = %v, errs = %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		level   string
		wantErr bool
	}{
		{level: "debug", wantErr: false},
		{level: "info", wantErr: false},
		{level: "warn", wantErr: false},
		{level: "error", wantErr: false},
		{level: "trace", wantErr: true},
		{level: "fatal", wantErr: true},
		{level: "", wantErr: true},
		{level: "DEBUG", wantErr: true}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			cfg := LoadWithDefaults()
			cfg.Logging.Level = tt.level
			errs := cfg.Validate()

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("Validate() for level %q: hasErr = %v, wantErr = %v", tt.level, hasErr, tt.wantErr)
			}
		})
	}
}

func TestValidateLogFormat(t *testing.T) {
	tests := []struct {
		format  string
		wantErr bool
	}{
		{format: "json", wantErr: false},
		{format: "text", wantErr: false},
		{format: "xml", wantErr: true},
		{format: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			cfg := LoadWithDefaults()
			cfg.Logging.Format = tt.format
			errs := cfg.Validate()

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("Validate() for format %q: hasErr = %v, wantErr = %v", tt.format, hasErr, tt.wantErr)
			}
		})
	}
}

func TestValidateSessionLimits(t *testing.T) {
	tests := []struct {
		name             string
		maxPerConnection int
		maxTotal         int
		wantErr          bool
	}{
		{name: "valid limits", maxPerConnection: 10, maxTotal: 50, wantErr: false},
		{name: "equal limits", maxPerConnection: 10, maxTotal: 10, wantErr: false},
		{name: "zero maxPerConnection", maxPerConnection: 0, maxTotal: 50, wantErr: true},
		{name: "negative maxPerConnection", maxPerConnection: -1, maxTotal: 50, wantErr: true},
		{name: "zero maxTotal", maxPerConnection: 10, maxTotal: 0, wantErr: true},
		{name: "maxPerConnection > maxTotal", maxPerConnection: 100, maxTotal: 50, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoadWithDefaults()
			cfg.Session.MaxPerConnection = tt.maxPerConnection
			cfg.Session.MaxTotal = tt.maxTotal
			errs := cfg.Validate()

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("Validate(): hasErr = %v, wantErr = %v, errs = %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestValidateNegativeSlowMo(t *testing.T) {
	cfg := LoadWithDefaults()
	cfg.Browser.SlowMo = -100
	errs := cfg.Validate()

	if len(errs) == 0 {
		t.Error("Validate() expected error for negative slowMo, got nil")
	}
}

func TestValidateReturnsAllErrors(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "",
			Port: 0, // invalid
		},
		Browser: BrowserConfig{
			DefaultType: "invalid", // invalid
			Headless:    true,
			SlowMo:      -1, // invalid
		},
		Session: SessionConfig{
			MaxPerConnection: 0, // invalid
			MaxTotal:         0, // invalid
			DefaultTimeout:   0, // invalid
			IdleTimeout:      0, // invalid
		},
		Logging: LoggingConfig{
			Level:  "invalid", // invalid
			Format: "invalid", // invalid
		},
	}

	errs := cfg.Validate()

	// Should have multiple errors
	if len(errs) < 5 {
		t.Errorf("Validate() returned %d errors, expected at least 5 for multiple invalid fields", len(errs))
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
		valid    bool
	}{
		{"true", true, true},
		{"TRUE", true, true},
		{"True", true, true},
		{"1", true, true},
		{"yes", true, true},
		{"YES", true, true},
		{"on", true, true},
		{"ON", true, true},
		{"false", false, true},
		{"FALSE", false, true},
		{"False", false, true},
		{"0", false, true},
		{"no", false, true},
		{"NO", false, true},
		{"off", false, true},
		{"OFF", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := parseBool(tt.input)
			if ok != tt.valid {
				t.Errorf("parseBool(%q) ok = %v, want %v", tt.input, ok, tt.valid)
			}
			if result != tt.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseBoolInvalidReturnsFalse(t *testing.T) {
	_, ok := parseBool("invalid")
	if ok {
		t.Error("parseBool with invalid input should return ok=false")
	}
	_, ok = parseBool("maybe")
	if ok {
		t.Error("parseBool with 'maybe' should return ok=false")
	}
}

func TestValidateRetrySettings(t *testing.T) {
	tests := []struct {
		name    string
		retry   RetrySettings
		wantErr bool
	}{
		{
			name: "valid default settings",
			retry: RetrySettings{
				MaxAttempts:    3,
				InitialBackoff: 100 * time.Millisecond,
				MaxBackoff:     5 * time.Second,
				BackoffFactor:  2.0,
				Jitter:         0.1,
			},
			wantErr: false,
		},
		{
			name: "zero max attempts valid",
			retry: RetrySettings{
				MaxAttempts:    0,
				InitialBackoff: 100 * time.Millisecond,
				MaxBackoff:     5 * time.Second,
				BackoffFactor:  2.0,
				Jitter:         0.1,
			},
			wantErr: false,
		},
		{
			name: "negative max attempts invalid",
			retry: RetrySettings{
				MaxAttempts:    -1,
				InitialBackoff: 100 * time.Millisecond,
				MaxBackoff:     5 * time.Second,
				BackoffFactor:  2.0,
				Jitter:         0.1,
			},
			wantErr: true,
		},
		{
			name: "negative initial backoff invalid",
			retry: RetrySettings{
				MaxAttempts:    3,
				InitialBackoff: -100 * time.Millisecond,
				MaxBackoff:     5 * time.Second,
				BackoffFactor:  2.0,
				Jitter:         0.1,
			},
			wantErr: true,
		},
		{
			name: "negative max backoff invalid",
			retry: RetrySettings{
				MaxAttempts:    3,
				InitialBackoff: 100 * time.Millisecond,
				MaxBackoff:     -5 * time.Second,
				BackoffFactor:  2.0,
				Jitter:         0.1,
			},
			wantErr: true,
		},
		{
			name: "negative backoff factor invalid",
			retry: RetrySettings{
				MaxAttempts:    3,
				InitialBackoff: 100 * time.Millisecond,
				MaxBackoff:     5 * time.Second,
				BackoffFactor:  -2.0,
				Jitter:         0.1,
			},
			wantErr: true,
		},
		{
			name: "jitter below 0 invalid",
			retry: RetrySettings{
				MaxAttempts:    3,
				InitialBackoff: 100 * time.Millisecond,
				MaxBackoff:     5 * time.Second,
				BackoffFactor:  2.0,
				Jitter:         -0.1,
			},
			wantErr: true,
		},
		{
			name: "jitter above 1 invalid",
			retry: RetrySettings{
				MaxAttempts:    3,
				InitialBackoff: 100 * time.Millisecond,
				MaxBackoff:     5 * time.Second,
				BackoffFactor:  2.0,
				Jitter:         1.5,
			},
			wantErr: true,
		},
		{
			name: "jitter at 0 valid",
			retry: RetrySettings{
				MaxAttempts:    3,
				InitialBackoff: 100 * time.Millisecond,
				MaxBackoff:     5 * time.Second,
				BackoffFactor:  2.0,
				Jitter:         0.0,
			},
			wantErr: false,
		},
		{
			name: "jitter at 1 valid",
			retry: RetrySettings{
				MaxAttempts:    3,
				InitialBackoff: 100 * time.Millisecond,
				MaxBackoff:     5 * time.Second,
				BackoffFactor:  2.0,
				Jitter:         1.0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoadWithDefaults()
			cfg.Retry = tt.retry
			errs := cfg.Validate()

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("Validate(): hasErr = %v, wantErr = %v, errs = %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestLoadValidYAMLWithRetrySettings(t *testing.T) {
	yamlContent := `
retry:
  maxAttempts: 5
  initialBackoff: 200ms
  maxBackoff: 10s
  backoffFactor: 3.0
  jitter: 0.2
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Retry.MaxAttempts != 5 {
		t.Errorf("Retry.MaxAttempts = %d, want %d", cfg.Retry.MaxAttempts, 5)
	}
	if cfg.Retry.InitialBackoff != 200*time.Millisecond {
		t.Errorf("Retry.InitialBackoff = %v, want %v", cfg.Retry.InitialBackoff, 200*time.Millisecond)
	}
	if cfg.Retry.MaxBackoff != 10*time.Second {
		t.Errorf("Retry.MaxBackoff = %v, want %v", cfg.Retry.MaxBackoff, 10*time.Second)
	}
	if cfg.Retry.BackoffFactor != 3.0 {
		t.Errorf("Retry.BackoffFactor = %v, want %v", cfg.Retry.BackoffFactor, 3.0)
	}
	if cfg.Retry.Jitter != 0.2 {
		t.Errorf("Retry.Jitter = %v, want %v", cfg.Retry.Jitter, 0.2)
	}
}
