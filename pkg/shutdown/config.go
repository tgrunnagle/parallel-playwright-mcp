// Package shutdown provides graceful shutdown coordination for the Playwright MCP server.
package shutdown

import "time"

// Config holds shutdown timing configuration.
type Config struct {
	// DrainTimeout is the maximum time to wait for in-flight requests to complete.
	DrainTimeout time.Duration
	// PhaseTimeout is the timeout for each individual shutdown phase (after drain).
	PhaseTimeout time.Duration
	// TotalTimeout is the maximum time for the entire shutdown sequence.
	TotalTimeout time.Duration
}

// DefaultConfig returns sensible default shutdown configuration.
func DefaultConfig() Config {
	return Config{
		DrainTimeout: 30 * time.Second,
		PhaseTimeout: 30 * time.Second,
		TotalTimeout: 90 * time.Second, // DrainTimeout + 2*PhaseTimeout
	}
}

// WithDrainTimeout returns a new Config with the specified drain timeout.
func (c Config) WithDrainTimeout(d time.Duration) Config {
	c.DrainTimeout = d
	return c
}

// WithPhaseTimeout returns a new Config with the specified phase timeout.
func (c Config) WithPhaseTimeout(d time.Duration) Config {
	c.PhaseTimeout = d
	return c
}

// WithTotalTimeout returns a new Config with the specified total timeout.
func (c Config) WithTotalTimeout(d time.Duration) Config {
	c.TotalTimeout = d
	return c
}
