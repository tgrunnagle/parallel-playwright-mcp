//go:build !integration

package tools

import (
	"os"
	"testing"
)

// TestMain turns off the post-action settle window for unit tests, whose pages
// are mocks with nothing to settle. Integration tests keep the real window.
func TestMain(m *testing.M) {
	settleDelay = 0
	os.Exit(m.Run())
}
