// Package session implements the 1:N browser session management model.
package session

import (
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
)

// BrowserSession represents an isolated browser session owned by an MCP connection.
// Each session contains a Playwright browser context with one or more pages (tabs).
type BrowserSession struct {
	// ID is the unique browser session identifier (e.g., "sess-abc123").
	ID string
	// MCPSessionID is the owning MCP connection identifier.
	MCPSessionID string
	// BrowserType is the browser engine (chromium, firefox, or webkit).
	BrowserType browser.BrowserType
	// Context is the Playwright browser context for this session.
	Context playwright.BrowserContext
	// Pages maps tab IDs to Playwright pages within this session.
	Pages map[string]playwright.Page
	// ActiveTabID is the currently active tab identifier.
	ActiveTabID string
	// ConsoleLogs is the circular buffer for captured console messages.
	ConsoleLogs *ConsoleLogBuffer
	// CreatedAt is the session creation timestamp.
	CreatedAt time.Time
	// LastAccess is the timestamp of the last activity in this session.
	LastAccess time.Time
	// Metadata holds additional session-specific data.
	Metadata map[string]any
	// mu protects concurrent access to Pages and ActiveTabID.
	mu sync.RWMutex
}

// ActivePage returns the currently active page in the session.
// Returns nil if no active page is set or if the active tab ID doesn't exist.
func (s *BrowserSession) ActivePage() playwright.Page {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Pages[s.ActiveTabID]
}

// ActivePageInfo returns the URL and title of the active page.
// This method safely captures page information while holding the session lock,
// preventing data races when the page might be closed concurrently.
// Returns empty strings if no active page exists or if page methods fail.
func (s *BrowserSession) ActivePageInfo() (url, title string) {
	s.mu.RLock()
	page := s.Pages[s.ActiveTabID]
	s.mu.RUnlock()

	if page == nil {
		return "", ""
	}

	// Capture URL and title - these methods are safe to call even on a closed page
	// (Playwright returns empty values rather than panicking)
	url = page.URL()
	title, _ = page.Title()
	return url, title
}

// UpdateLastAccess updates the LastAccess timestamp to the current time.
func (s *BrowserSession) UpdateLastAccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastAccess = time.Now()
}

// AddPage adds a new page to the session with the given tab ID.
// If a page with the same tab ID already exists, it will be replaced.
func (s *BrowserSession) AddPage(tabID string, page playwright.Page) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Pages[tabID] = page
}

// RemovePage removes a page from the session by tab ID.
// Returns the removed page, or nil if the tab ID didn't exist.
func (s *BrowserSession) RemovePage(tabID string) playwright.Page {
	s.mu.Lock()
	defer s.mu.Unlock()
	page, exists := s.Pages[tabID]
	if exists {
		delete(s.Pages, tabID)
	}
	return page
}

// GetPage retrieves a page by tab ID.
// Returns the page and true if found, or nil and false otherwise.
func (s *BrowserSession) GetPage(tabID string) (playwright.Page, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	page, exists := s.Pages[tabID]
	return page, exists
}

// SetActiveTab sets the active tab ID.
// Returns true if the tab ID exists in the Pages map, false otherwise.
func (s *BrowserSession) SetActiveTab(tabID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.Pages[tabID]; exists {
		s.ActiveTabID = tabID
		return true
	}
	return false
}

// PageCount returns the number of pages in the session.
func (s *BrowserSession) PageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Pages)
}

// TabIDs returns a slice of all tab IDs in the session.
func (s *BrowserSession) TabIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.Pages))
	for id := range s.Pages {
		ids = append(ids, id)
	}
	return ids
}
