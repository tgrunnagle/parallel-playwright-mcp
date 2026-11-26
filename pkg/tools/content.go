// Package tools contains MCP tool implementations for browser automation.
package tools

// DefaultMaxResponseChars is the default response budget: the maximum number of
// characters (Unicode code points) a content tool returns in one call.
const DefaultMaxResponseChars = 40000

// ResponseConfig holds the response budget applied to content tools.
type ResponseConfig struct {
	// MaxResponseChars is the maximum number of characters (runes) a content tool
	// returns in its content block. Per-call maxChars values above it are clamped.
	MaxResponseChars int
}

// DefaultResponseConfig returns a ResponseConfig with the default budget.
func DefaultResponseConfig() *ResponseConfig {
	return &ResponseConfig{MaxResponseChars: DefaultMaxResponseChars}
}

// budget returns the configured response budget, falling back to the default
// when the config is nil or unset.
func (c *ResponseConfig) budget() int {
	if c == nil || c.MaxResponseChars <= 0 {
		return DefaultMaxResponseChars
	}
	return c.MaxResponseChars
}
