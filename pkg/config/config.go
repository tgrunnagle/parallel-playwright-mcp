// Package config handles server configuration loading from files and environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	pkgerrors "github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/tools"
	"gopkg.in/yaml.v3"
)

// Default configuration values.
const (
	DefaultHost             = "127.0.0.1"
	DefaultPort             = 3000
	DefaultBrowserType      = "chromium"
	DefaultHeadless         = true
	DefaultSlowMo           = 0
	DefaultMaxPerConnection = 10
	DefaultMaxTotal         = 50
	DefaultTimeout          = 30 * time.Second
	DefaultIdleTimeout      = 5 * time.Minute
	DefaultLogLevel         = "info"
	DefaultLogFormat        = "json"
	DefaultMaxAttempts      = 3
	DefaultInitialBackoff   = 100 * time.Millisecond
	DefaultMaxBackoff       = 5 * time.Second
	DefaultBackoffFactor    = 2.0
	DefaultJitter           = 0.1
)

// Valid values for validation.
var (
	ValidBrowserTypes = []string{"chromium", "firefox", "webkit"}
	ValidLogLevels    = []string{"debug", "info", "warn", "error"}
	ValidLogFormats   = []string{"json", "text"}
)

// Config represents the complete server configuration.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Browser BrowserConfig `yaml:"browser"`
	Session SessionConfig `yaml:"session"`
	Logging LoggingConfig `yaml:"logging"`
	Retry   RetrySettings `yaml:"retry"`
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// BrowserConfig contains browser-related settings.
type BrowserConfig struct {
	DefaultType string              `yaml:"defaultType"`
	Headless    bool                `yaml:"headless"`
	SlowMo      int                 `yaml:"slowMo"`
	Chromium    BrowserLaunchConfig `yaml:"chromium"`
	Firefox     BrowserLaunchConfig `yaml:"firefox"`
	WebKit      BrowserLaunchConfig `yaml:"webkit"`
}

// BrowserLaunchConfig contains per-browser launch arguments.
type BrowserLaunchConfig struct {
	Args    []string `yaml:"args"`
	Channel string   `yaml:"channel"`
}

// SessionConfig contains session management settings.
type SessionConfig struct {
	MaxPerConnection int             `yaml:"maxPerConnection"`
	MaxTotal         int             `yaml:"maxTotal"`
	DefaultTimeout   time.Duration   `yaml:"defaultTimeout"`
	IdleTimeout      time.Duration   `yaml:"idleTimeout"`
	Timeout          TimeoutSettings `yaml:"timeout"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// RetrySettings holds retry-related settings for transient failures.
// These settings are loaded from YAML configuration and can be converted
// to errors.RetryConfig using the ToRetryConfig() method.
//
// Example YAML configuration:
//
//	retry:
//	  maxAttempts: 3
//	  initialBackoff: 100ms
//	  maxBackoff: 5s
//	  backoffFactor: 2.0
//	  jitter: 0.1
//	  retryableErrors: [-32002, -32003]  # Optional, defaults to ElementNotFound and Timeout
type RetrySettings struct {
	MaxAttempts     int           `yaml:"maxAttempts"`
	InitialBackoff  time.Duration `yaml:"initialBackoff"`
	MaxBackoff      time.Duration `yaml:"maxBackoff"`
	BackoffFactor   float64       `yaml:"backoffFactor"`
	Jitter          float64       `yaml:"jitter"`
	RetryableErrors []int         `yaml:"retryableErrors,omitempty"`
}

// ToRetryConfig converts RetrySettings to errors.RetryConfig for use with retry operations.
// This method creates a defensive copy of the retryable errors slice to prevent mutations.
//
// Example usage:
//
//	cfg, err := config.Load("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	retryConfig := cfg.Retry.ToRetryConfig()
//	err = errors.WithRetry(ctx, retryConfig, func() error {
//	    return page.Goto(url)
//	})
func (rs *RetrySettings) ToRetryConfig() *pkgerrors.RetryConfig {
	// Create a copy of RetryableErrors to avoid mutations
	retryableErrors := make([]int, len(rs.RetryableErrors))
	copy(retryableErrors, rs.RetryableErrors)

	return &pkgerrors.RetryConfig{
		MaxAttempts:     rs.MaxAttempts,
		InitialBackoff:  rs.InitialBackoff,
		MaxBackoff:      rs.MaxBackoff,
		BackoffFactor:   rs.BackoffFactor,
		Jitter:          rs.Jitter,
		RetryableErrors: retryableErrors,
	}
}

// TimeoutSettings holds timeout-related settings (in milliseconds for YAML convenience).
// These settings are nested under the session configuration section.
//
// Example YAML configuration:
//
//	session:
//	  timeout:
//	    default: 30000      # 30 seconds
//	    navigation: 30000   # 30 seconds
//	    element: 5000       # 5 seconds
//	    script: 30000       # 30 seconds
type TimeoutSettings struct {
	DefaultMs    int `yaml:"default"`    // Default timeout (ms), default: 30000
	NavigationMs int `yaml:"navigation"` // Navigation timeout (ms), default: 30000
	ElementMs    int `yaml:"element"`    // Element wait timeout (ms), default: 5000
	ScriptMs     int `yaml:"script"`     // Script timeout (ms), default: 30000
}

// ToTimeoutConfig converts TimeoutSettings to tools.TimeoutConfig for use with timeout operations.
//
// Example usage:
//
//	cfg, err := config.Load("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	timeoutConfig := cfg.Session.Timeout.ToTimeoutConfig()
func (ts *TimeoutSettings) ToTimeoutConfig() *tools.TimeoutConfig {
	return &tools.TimeoutConfig{
		Default:    time.Duration(ts.DefaultMs) * time.Millisecond,
		Navigation: time.Duration(ts.NavigationMs) * time.Millisecond,
		Element:    time.Duration(ts.ElementMs) * time.Millisecond,
		Script:     time.Duration(ts.ScriptMs) * time.Millisecond,
	}
}

// LoadWithDefaults creates a configuration with all default values set.
func LoadWithDefaults() *Config {
	return &Config{
		Server: ServerConfig{
			Host: DefaultHost,
			Port: DefaultPort,
		},
		Browser: BrowserConfig{
			DefaultType: DefaultBrowserType,
			Headless:    DefaultHeadless,
			SlowMo:      DefaultSlowMo,
			Chromium:    BrowserLaunchConfig{Args: []string{}},
			Firefox:     BrowserLaunchConfig{Args: []string{}},
			WebKit:      BrowserLaunchConfig{Args: []string{}},
		},
		Session: SessionConfig{
			MaxPerConnection: DefaultMaxPerConnection,
			MaxTotal:         DefaultMaxTotal,
			DefaultTimeout:   DefaultTimeout,
			IdleTimeout:      DefaultIdleTimeout,
			Timeout: TimeoutSettings{
				DefaultMs:    30000, // 30 seconds
				NavigationMs: 30000, // 30 seconds
				ElementMs:    5000,  // 5 seconds
				ScriptMs:     30000, // 30 seconds
			},
		},
		Logging: LoggingConfig{
			Level:  DefaultLogLevel,
			Format: DefaultLogFormat,
		},
		Retry: RetrySettings{
			MaxAttempts:     DefaultMaxAttempts,
			InitialBackoff:  DefaultInitialBackoff,
			MaxBackoff:      DefaultMaxBackoff,
			BackoffFactor:   DefaultBackoffFactor,
			Jitter:          DefaultJitter,
			RetryableErrors: []int{-32002, -32003}, // CodeElementNotFound, CodeTimeout
		},
	}
}

// Load reads configuration from the specified path (or default paths) and applies
// environment variable overrides. If path is empty, it checks MCP_CONFIG_PATH env var,
// then falls back to "config.yaml" in the current directory. If the file doesn't exist,
// default values are used.
func Load(path string) (*Config, error) {
	cfg := LoadWithDefaults()

	// Determine config file path
	configPath := path
	if configPath == "" {
		configPath = os.Getenv("MCP_CONFIG_PATH")
	}
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Try to load config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		// File doesn't exist - use defaults
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	}

	// Apply environment variable overrides
	if errs := cfg.applyEnvironmentOverrides(); len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Validate configuration
	if errs := cfg.Validate(); len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return cfg, nil
}

// applyEnvironmentOverrides overlays environment variable values on the configuration.
// Returns a slice of errors for any invalid environment variable values.
func (c *Config) applyEnvironmentOverrides() []error {
	var errs []error

	if host := os.Getenv("MCP_HOST"); host != "" {
		c.Server.Host = host
	}

	if port := os.Getenv("MCP_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid MCP_PORT %q: must be a valid integer", port))
		} else {
			c.Server.Port = p
		}
	}

	if headless := os.Getenv("MCP_BROWSER_HEADLESS"); headless != "" {
		val, ok := parseBool(headless)
		if !ok {
			errs = append(errs, fmt.Errorf("invalid MCP_BROWSER_HEADLESS %q: must be true/false, 1/0, yes/no, or on/off", headless))
		} else {
			c.Browser.Headless = val
		}
	}

	if level := os.Getenv("MCP_LOG_LEVEL"); level != "" {
		c.Logging.Level = strings.ToLower(level)
	}

	// Timeout settings
	if defaultTimeout := os.Getenv("MCP_TIMEOUT_DEFAULT"); defaultTimeout != "" {
		val, err := strconv.Atoi(defaultTimeout)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid MCP_TIMEOUT_DEFAULT %q: must be a valid integer", defaultTimeout))
		} else {
			c.Session.Timeout.DefaultMs = val
		}
	}

	if navigationTimeout := os.Getenv("MCP_TIMEOUT_NAVIGATION"); navigationTimeout != "" {
		val, err := strconv.Atoi(navigationTimeout)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid MCP_TIMEOUT_NAVIGATION %q: must be a valid integer", navigationTimeout))
		} else {
			c.Session.Timeout.NavigationMs = val
		}
	}

	if elementTimeout := os.Getenv("MCP_TIMEOUT_ELEMENT"); elementTimeout != "" {
		val, err := strconv.Atoi(elementTimeout)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid MCP_TIMEOUT_ELEMENT %q: must be a valid integer", elementTimeout))
		} else {
			c.Session.Timeout.ElementMs = val
		}
	}

	if scriptTimeout := os.Getenv("MCP_TIMEOUT_SCRIPT"); scriptTimeout != "" {
		val, err := strconv.Atoi(scriptTimeout)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid MCP_TIMEOUT_SCRIPT %q: must be a valid integer", scriptTimeout))
		} else {
			c.Session.Timeout.ScriptMs = val
		}
	}

	return errs
}

// parseBool parses a string as a boolean. Returns the parsed value and true if valid,
// or false and false if the string is not a recognized boolean value.
func parseBool(s string) (bool, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "1", "yes", "on":
		return true, true
	case "false", "0", "no", "off":
		return false, true
	default:
		return false, false
	}
}

// Validate checks configuration values and returns a slice of errors for all invalid values.
// Note: Server.Host is not validated because empty string is valid (binds to all interfaces),
// and any other string will be validated by the network stack when the server starts.
func (c *Config) Validate() []error {
	var errs []error

	// Validate server port
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("invalid server port %d: must be between 1 and 65535", c.Server.Port))
	}

	// Validate browser type
	if !isValidValue(c.Browser.DefaultType, ValidBrowserTypes) {
		errs = append(errs, fmt.Errorf("invalid browser type %q: must be one of %v", c.Browser.DefaultType, ValidBrowserTypes))
	}

	// Validate slowMo is non-negative
	if c.Browser.SlowMo < 0 {
		errs = append(errs, fmt.Errorf("invalid slowMo %d: must be non-negative", c.Browser.SlowMo))
	}

	// Validate session limits
	if c.Session.MaxPerConnection < 1 {
		errs = append(errs, fmt.Errorf("invalid maxPerConnection %d: must be at least 1", c.Session.MaxPerConnection))
	}
	if c.Session.MaxTotal < 1 {
		errs = append(errs, fmt.Errorf("invalid maxTotal %d: must be at least 1", c.Session.MaxTotal))
	}
	if c.Session.MaxPerConnection > c.Session.MaxTotal {
		errs = append(errs, fmt.Errorf("maxPerConnection (%d) cannot exceed maxTotal (%d)", c.Session.MaxPerConnection, c.Session.MaxTotal))
	}

	// Validate timeout values
	if c.Session.DefaultTimeout <= 0 {
		errs = append(errs, fmt.Errorf("invalid defaultTimeout %v: must be positive", c.Session.DefaultTimeout))
	}
	if c.Session.IdleTimeout <= 0 {
		errs = append(errs, fmt.Errorf("invalid idleTimeout %v: must be positive", c.Session.IdleTimeout))
	}

	// Validate log level
	if !isValidValue(c.Logging.Level, ValidLogLevels) {
		errs = append(errs, fmt.Errorf("invalid log level %q: must be one of %v", c.Logging.Level, ValidLogLevels))
	}

	// Validate log format
	if !isValidValue(c.Logging.Format, ValidLogFormats) {
		errs = append(errs, fmt.Errorf("invalid log format %q: must be one of %v", c.Logging.Format, ValidLogFormats))
	}

	// Validate retry settings
	if c.Retry.MaxAttempts < 1 {
		errs = append(errs, fmt.Errorf("invalid retry maxAttempts %d: must be at least 1 (1 means no retries, just initial attempt)", c.Retry.MaxAttempts))
	}
	if c.Retry.InitialBackoff < 0 {
		errs = append(errs, fmt.Errorf("invalid retry initialBackoff %v: must be non-negative", c.Retry.InitialBackoff))
	}
	if c.Retry.MaxBackoff < 0 {
		errs = append(errs, fmt.Errorf("invalid retry maxBackoff %v: must be non-negative", c.Retry.MaxBackoff))
	}
	if c.Retry.BackoffFactor < 0 {
		errs = append(errs, fmt.Errorf("invalid retry backoffFactor %v: must be non-negative", c.Retry.BackoffFactor))
	}
	if c.Retry.Jitter < 0 || c.Retry.Jitter > 1 {
		errs = append(errs, fmt.Errorf("invalid retry jitter %v: must be between 0.0 and 1.0", c.Retry.Jitter))
	}

	// Validate timeout settings
	if c.Session.Timeout.DefaultMs <= 0 {
		errs = append(errs, fmt.Errorf("invalid timeout default %dms: must be positive", c.Session.Timeout.DefaultMs))
	}
	if c.Session.Timeout.NavigationMs <= 0 {
		errs = append(errs, fmt.Errorf("invalid timeout navigation %dms: must be positive", c.Session.Timeout.NavigationMs))
	}
	if c.Session.Timeout.ElementMs <= 0 {
		errs = append(errs, fmt.Errorf("invalid timeout element %dms: must be positive", c.Session.Timeout.ElementMs))
	}
	if c.Session.Timeout.ScriptMs <= 0 {
		errs = append(errs, fmt.Errorf("invalid timeout script %dms: must be positive", c.Session.Timeout.ScriptMs))
	}

	return errs
}

// isValidValue checks if a value is in the list of valid values.
func isValidValue(value string, valid []string) bool {
	for _, v := range valid {
		if value == v {
			return true
		}
	}
	return false
}
