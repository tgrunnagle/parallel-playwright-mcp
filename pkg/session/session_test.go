package session

import (
	"sync"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
)

// mockPageForSession is a mock implementation of playwright.Page for session testing.
type mockPageForSession struct {
	playwright.Page
	id    string
	url   string
	title string
}

func (m *mockPageForSession) URL() string {
	return m.url
}

func (m *mockPageForSession) Title() (string, error) {
	return m.title, nil
}

func TestBrowserSession_ActivePage(t *testing.T) {
	t.Run("returns page from Pages map using ActiveTabID", func(t *testing.T) {
		page := &mockPageForSession{id: "test-page"}
		session := &BrowserSession{
			Pages:       map[string]playwright.Page{"tab-1": page},
			ActiveTabID: "tab-1",
		}

		result := session.ActivePage()
		if result != page {
			t.Error("ActivePage should return the page at ActiveTabID")
		}
	})

	t.Run("returns nil when ActiveTabID not in Pages map", func(t *testing.T) {
		session := &BrowserSession{
			Pages:       map[string]playwright.Page{},
			ActiveTabID: "nonexistent",
		}

		result := session.ActivePage()
		if result != nil {
			t.Error("ActivePage should return nil for missing ActiveTabID")
		}
	})

	t.Run("returns nil when Pages map is nil", func(t *testing.T) {
		session := &BrowserSession{
			ActiveTabID: "tab-1",
		}

		result := session.ActivePage()
		if result != nil {
			t.Error("ActivePage should return nil when Pages is nil")
		}
	})
}

func TestBrowserSession_ActivePageInfo(t *testing.T) {
	t.Run("returns URL and title from active page", func(t *testing.T) {
		page := &mockPageForSession{
			id:    "test-page",
			url:   "https://example.com",
			title: "Example Title",
		}
		session := &BrowserSession{
			Pages:       map[string]playwright.Page{"tab-1": page},
			ActiveTabID: "tab-1",
		}

		url, title := session.ActivePageInfo()
		if url != "https://example.com" {
			t.Errorf("URL = %q, want %q", url, "https://example.com")
		}
		if title != "Example Title" {
			t.Errorf("Title = %q, want %q", title, "Example Title")
		}
	})

	t.Run("returns empty strings when no active page", func(t *testing.T) {
		session := &BrowserSession{
			Pages:       map[string]playwright.Page{},
			ActiveTabID: "nonexistent",
		}

		url, title := session.ActivePageInfo()
		if url != "" {
			t.Errorf("URL should be empty, got %q", url)
		}
		if title != "" {
			t.Errorf("Title should be empty, got %q", title)
		}
	})

	t.Run("returns empty strings when Pages map is nil", func(t *testing.T) {
		session := &BrowserSession{
			ActiveTabID: "tab-1",
		}

		url, title := session.ActivePageInfo()
		if url != "" {
			t.Errorf("URL should be empty, got %q", url)
		}
		if title != "" {
			t.Errorf("Title should be empty, got %q", title)
		}
	})
}

func TestBrowserSession_UpdateLastAccess(t *testing.T) {
	t.Run("updates LastAccess to current time", func(t *testing.T) {
		originalTime := time.Now().Add(-time.Hour)
		session := &BrowserSession{
			LastAccess: originalTime,
		}

		before := time.Now()
		session.UpdateLastAccess()
		after := time.Now()

		if session.LastAccess.Before(before) || session.LastAccess.After(after) {
			t.Error("LastAccess should be updated to current time")
		}
		if session.LastAccess.Equal(originalTime) {
			t.Error("LastAccess should be changed from original value")
		}
	})
}

func TestBrowserSession_AddPage(t *testing.T) {
	t.Run("adds new page to Pages map", func(t *testing.T) {
		page := &mockPageForSession{id: "new-page"}
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
		}

		session.AddPage("tab-new", page)

		if stored, exists := session.Pages["tab-new"]; !exists || stored != page {
			t.Error("AddPage should add page to Pages map")
		}
	})

	t.Run("replaces existing page with same tab ID", func(t *testing.T) {
		oldPage := &mockPageForSession{id: "old-page"}
		newPage := &mockPageForSession{id: "new-page"}
		session := &BrowserSession{
			Pages: map[string]playwright.Page{"tab-1": oldPage},
		}

		session.AddPage("tab-1", newPage)

		if session.Pages["tab-1"] != newPage {
			t.Error("AddPage should replace existing page")
		}
	})
}

func TestBrowserSession_RemovePage(t *testing.T) {
	t.Run("removes page from Pages map and returns it", func(t *testing.T) {
		page := &mockPageForSession{id: "test-page"}
		session := &BrowserSession{
			Pages: map[string]playwright.Page{"tab-1": page},
		}

		removed := session.RemovePage("tab-1")

		if removed != page {
			t.Error("RemovePage should return the removed page")
		}
		if _, exists := session.Pages["tab-1"]; exists {
			t.Error("RemovePage should remove page from map")
		}
	})

	t.Run("returns nil for nonexistent tab ID", func(t *testing.T) {
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
		}

		removed := session.RemovePage("nonexistent")

		if removed != nil {
			t.Error("RemovePage should return nil for nonexistent tab ID")
		}
	})
}

func TestBrowserSession_GetPage(t *testing.T) {
	t.Run("returns page and true when tab exists", func(t *testing.T) {
		page := &mockPageForSession{id: "test-page"}
		session := &BrowserSession{
			Pages: map[string]playwright.Page{"tab-1": page},
		}

		retrieved, exists := session.GetPage("tab-1")

		if !exists {
			t.Error("GetPage should return true for existing tab")
		}
		if retrieved != page {
			t.Error("GetPage should return the correct page")
		}
	})

	t.Run("returns nil and false for nonexistent tab", func(t *testing.T) {
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
		}

		retrieved, exists := session.GetPage("nonexistent")

		if exists {
			t.Error("GetPage should return false for nonexistent tab")
		}
		if retrieved != nil {
			t.Error("GetPage should return nil for nonexistent tab")
		}
	})
}

func TestBrowserSession_SetActiveTab(t *testing.T) {
	t.Run("sets ActiveTabID when tab exists", func(t *testing.T) {
		page := &mockPageForSession{id: "test-page"}
		session := &BrowserSession{
			Pages:       map[string]playwright.Page{"tab-1": page, "tab-2": &mockPageForSession{}},
			ActiveTabID: "tab-1",
		}

		result := session.SetActiveTab("tab-2")

		if !result {
			t.Error("SetActiveTab should return true when tab exists")
		}
		if session.ActiveTabID != "tab-2" {
			t.Error("SetActiveTab should update ActiveTabID")
		}
	})

	t.Run("returns false and doesn't change ActiveTabID when tab doesn't exist", func(t *testing.T) {
		session := &BrowserSession{
			Pages:       make(map[string]playwright.Page),
			ActiveTabID: "original",
		}

		result := session.SetActiveTab("nonexistent")

		if result {
			t.Error("SetActiveTab should return false for nonexistent tab")
		}
		if session.ActiveTabID != "original" {
			t.Error("SetActiveTab should not change ActiveTabID when tab doesn't exist")
		}
	})
}

func TestBrowserSession_PageCount(t *testing.T) {
	tests := []struct {
		name     string
		pages    map[string]playwright.Page
		expected int
	}{
		{
			name:     "empty pages map",
			pages:    make(map[string]playwright.Page),
			expected: 0,
		},
		{
			name: "single page",
			pages: map[string]playwright.Page{
				"tab-1": &mockPageForSession{},
			},
			expected: 1,
		},
		{
			name: "multiple pages",
			pages: map[string]playwright.Page{
				"tab-1": &mockPageForSession{},
				"tab-2": &mockPageForSession{},
				"tab-3": &mockPageForSession{},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &BrowserSession{Pages: tt.pages}
			if count := session.PageCount(); count != tt.expected {
				t.Errorf("PageCount() = %d, want %d", count, tt.expected)
			}
		})
	}
}

func TestBrowserSession_TabIDs(t *testing.T) {
	t.Run("returns all tab IDs", func(t *testing.T) {
		session := &BrowserSession{
			Pages: map[string]playwright.Page{
				"tab-1": &mockPageForSession{},
				"tab-2": &mockPageForSession{},
				"tab-3": &mockPageForSession{},
			},
		}

		ids := session.TabIDs()

		if len(ids) != 3 {
			t.Errorf("TabIDs should return 3 IDs, got %d", len(ids))
		}

		idSet := make(map[string]bool)
		for _, id := range ids {
			idSet[id] = true
		}
		for _, expected := range []string{"tab-1", "tab-2", "tab-3"} {
			if !idSet[expected] {
				t.Errorf("TabIDs should contain %s", expected)
			}
		}
	})

	t.Run("returns empty slice for empty Pages", func(t *testing.T) {
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
		}

		ids := session.TabIDs()

		if ids == nil {
			t.Error("TabIDs should return empty slice, not nil")
		}
		if len(ids) != 0 {
			t.Errorf("TabIDs should return empty slice, got %d items", len(ids))
		}
	})
}

// TestBrowserSession_ConcurrentAccess validates thread-safety of BrowserSession methods.
// NOTE: These tests are designed to catch race conditions and should be run with
// the Go race detector enabled: `go test -race ./pkg/session/...`
// The race detector requires CGO to be enabled, which may not be available on
// all platforms (e.g., Windows with CGO_ENABLED=0). CI pipelines should run
// these tests on Linux or macOS with CGO enabled to fully validate thread safety.
func TestBrowserSession_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent page operations are thread-safe", func(t *testing.T) {
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
		}

		var wg sync.WaitGroup

		// Concurrent adds
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				tabID := string(rune('a' + (idx % 26)))
				session.AddPage(tabID, &mockPageForSession{})
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = session.PageCount()
				_ = session.TabIDs()
				_ = session.ActivePage()
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent UpdateLastAccess is thread-safe", func(t *testing.T) {
		session := &BrowserSession{
			LastAccess: time.Now(),
		}

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				session.UpdateLastAccess()
			}()
		}
		wg.Wait()
	})
}

func TestBrowserSessionStruct(t *testing.T) {
	t.Run("can instantiate with all field types", func(t *testing.T) {
		now := time.Now()
		page := &mockPageForSession{}
		session := &BrowserSession{
			ID:           "sess-123",
			MCPSessionID: "mcp-456",
			BrowserType:  browser.BrowserChromium,
			Pages:        map[string]playwright.Page{"tab-1": page},
			ActiveTabID:  "tab-1",
			CreatedAt:    now,
			LastAccess:   now,
			Metadata:     map[string]any{"key": "value"},
		}

		if session.ID != "sess-123" {
			t.Error("ID not set correctly")
		}
		if session.MCPSessionID != "mcp-456" {
			t.Error("MCPSessionID not set correctly")
		}
		if session.BrowserType != browser.BrowserChromium {
			t.Error("BrowserType not set correctly")
		}
		if len(session.Pages) != 1 {
			t.Error("Pages not set correctly")
		}
		if session.ActiveTabID != "tab-1" {
			t.Error("ActiveTabID not set correctly")
		}
		if !session.CreatedAt.Equal(now) {
			t.Error("CreatedAt not set correctly")
		}
		if !session.LastAccess.Equal(now) {
			t.Error("LastAccess not set correctly")
		}
		if session.Metadata["key"] != "value" {
			t.Error("Metadata not set correctly")
		}
	})
}

// mockPageWithEvents is a mock implementation of playwright.Page that tracks
// both console and network event handlers for testing AddPageWithLogging.
type mockPageWithEvents struct {
	playwright.Page
	id                    string
	url                   string
	title                 string
	consoleHandlers       []func(playwright.ConsoleMessage)
	requestHandlers       []func(playwright.Request)
	responseHandlers      []func(playwright.Response)
	requestFailedHandlers []func(playwright.Request)
}

func (m *mockPageWithEvents) URL() string {
	return m.url
}

func (m *mockPageWithEvents) Title() (string, error) {
	return m.title, nil
}

func (m *mockPageWithEvents) On(event string, handler interface{}) {
	switch event {
	case "console":
		if h, ok := handler.(func(playwright.ConsoleMessage)); ok {
			m.consoleHandlers = append(m.consoleHandlers, h)
		}
	case "request":
		if h, ok := handler.(func(playwright.Request)); ok {
			m.requestHandlers = append(m.requestHandlers, h)
		}
	case "response":
		if h, ok := handler.(func(playwright.Response)); ok {
			m.responseHandlers = append(m.responseHandlers, h)
		}
	case "requestfailed":
		if h, ok := handler.(func(playwright.Request)); ok {
			m.requestFailedHandlers = append(m.requestFailedHandlers, h)
		}
	}
}

func (m *mockPageWithEvents) RemoveListener(event string, handler interface{}) {
	switch event {
	case "console":
		m.consoleHandlers = nil
	case "request":
		m.requestHandlers = nil
	case "response":
		m.responseHandlers = nil
	case "requestfailed":
		m.requestFailedHandlers = nil
	}
}

func TestBrowserSession_CleanupNetworkListeners(t *testing.T) {
	t.Run("calls all cleanup functions", func(t *testing.T) {
		cleanup1Called := false
		cleanup2Called := false

		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
			networkCleanups: map[string]func(){
				"tab-1": func() { cleanup1Called = true },
				"tab-2": func() { cleanup2Called = true },
			},
		}

		session.CleanupNetworkListeners()

		if !cleanup1Called {
			t.Error("Cleanup1 should have been called")
		}
		if !cleanup2Called {
			t.Error("Cleanup2 should have been called")
		}
	})

	t.Run("removes all cleanup functions from map", func(t *testing.T) {
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
			networkCleanups: map[string]func(){
				"tab-1": func() {},
				"tab-2": func() {},
			},
		}

		session.CleanupNetworkListeners()

		if len(session.networkCleanups) != 0 {
			t.Errorf("Expected empty networkCleanups, got %d entries", len(session.networkCleanups))
		}
	})

	t.Run("handles nil networkCleanups map gracefully", func(t *testing.T) {
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
			// networkCleanups is nil
		}

		// Should not panic
		session.CleanupNetworkListeners()
	})

	t.Run("handles empty networkCleanups map gracefully", func(t *testing.T) {
		session := &BrowserSession{
			Pages:           make(map[string]playwright.Page),
			networkCleanups: make(map[string]func()),
		}

		// Should not panic
		session.CleanupNetworkListeners()
	})

	t.Run("handles nil cleanup function gracefully", func(t *testing.T) {
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
			networkCleanups: map[string]func(){
				"tab-1": nil,
				"tab-2": func() {},
			},
		}

		// Should not panic
		session.CleanupNetworkListeners()

		if len(session.networkCleanups) != 0 {
			t.Error("All entries should be removed even if cleanup was nil")
		}
	})
}

func TestBrowserSession_AddNetworkCleanup(t *testing.T) {
	t.Run("stores cleanup function in map", func(t *testing.T) {
		session := &BrowserSession{
			Pages:           make(map[string]playwright.Page),
			networkCleanups: make(map[string]func()),
		}

		cleanupCalled := false
		cleanup := func() { cleanupCalled = true }

		session.AddNetworkCleanup("tab-1", cleanup)

		if _, exists := session.networkCleanups["tab-1"]; !exists {
			t.Error("AddNetworkCleanup should store cleanup function in map")
		}

		// Verify the stored function works
		session.networkCleanups["tab-1"]()
		if !cleanupCalled {
			t.Error("Stored cleanup function should be callable")
		}
	})

	t.Run("initializes map if nil", func(t *testing.T) {
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
			// networkCleanups is nil
		}

		cleanup := func() {}
		session.AddNetworkCleanup("tab-1", cleanup)

		if session.networkCleanups == nil {
			t.Error("AddNetworkCleanup should initialize networkCleanups map if nil")
		}
		if _, exists := session.networkCleanups["tab-1"]; !exists {
			t.Error("AddNetworkCleanup should store cleanup function after initializing map")
		}
	})

	t.Run("replaces existing cleanup function", func(t *testing.T) {
		session := &BrowserSession{
			Pages:           make(map[string]playwright.Page),
			networkCleanups: make(map[string]func()),
		}

		firstCalled := false
		secondCalled := false

		session.AddNetworkCleanup("tab-1", func() { firstCalled = true })
		session.AddNetworkCleanup("tab-1", func() { secondCalled = true })

		// Call the stored cleanup
		session.networkCleanups["tab-1"]()

		if firstCalled {
			t.Error("First cleanup should have been replaced")
		}
		if !secondCalled {
			t.Error("Second cleanup should be called")
		}
	})

	t.Run("handles multiple tabs", func(t *testing.T) {
		session := &BrowserSession{
			Pages:           make(map[string]playwright.Page),
			networkCleanups: make(map[string]func()),
		}

		tab1Called := false
		tab2Called := false

		session.AddNetworkCleanup("tab-1", func() { tab1Called = true })
		session.AddNetworkCleanup("tab-2", func() { tab2Called = true })

		if len(session.networkCleanups) != 2 {
			t.Errorf("Expected 2 cleanup functions, got %d", len(session.networkCleanups))
		}

		session.networkCleanups["tab-1"]()
		session.networkCleanups["tab-2"]()

		if !tab1Called || !tab2Called {
			t.Error("Both cleanup functions should be callable")
		}
	})
}

func TestBrowserSession_AddPageWithLogging(t *testing.T) {
	t.Run("adds page to Pages map", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}
		session := &BrowserSession{
			Pages: make(map[string]playwright.Page),
		}

		session.AddPageWithLogging("tab-1", page)

		if stored, exists := session.Pages["tab-1"]; !exists || stored != page {
			t.Error("AddPageWithLogging should add page to Pages map")
		}
	})

	t.Run("attaches console handler when ConsoleLogs exists", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}
		session := &BrowserSession{
			Pages:       make(map[string]playwright.Page),
			ConsoleLogs: NewConsoleLogBuffer(10),
		}

		session.AddPageWithLogging("tab-1", page)

		if len(page.consoleHandlers) != 1 {
			t.Errorf("Expected 1 console handler, got %d", len(page.consoleHandlers))
		}
	})

	t.Run("attaches network handlers when NetworkLogs exists", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}
		session := &BrowserSession{
			Pages:       make(map[string]playwright.Page),
			NetworkLogs: NewNetworkLogBuffer(10),
		}

		session.AddPageWithLogging("tab-1", page)

		if len(page.requestHandlers) != 1 {
			t.Errorf("Expected 1 request handler, got %d", len(page.requestHandlers))
		}
		if len(page.responseHandlers) != 1 {
			t.Errorf("Expected 1 response handler, got %d", len(page.responseHandlers))
		}
		if len(page.requestFailedHandlers) != 1 {
			t.Errorf("Expected 1 requestfailed handler, got %d", len(page.requestFailedHandlers))
		}
	})

	t.Run("stores network cleanup function", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}
		session := &BrowserSession{
			Pages:       make(map[string]playwright.Page),
			NetworkLogs: NewNetworkLogBuffer(10),
		}

		session.AddPageWithLogging("tab-1", page)

		if session.networkCleanups == nil {
			t.Error("networkCleanups map should be initialized")
		}
		if _, exists := session.networkCleanups["tab-1"]; !exists {
			t.Error("Cleanup function should be stored for tab")
		}
	})

	t.Run("attaches both console and network handlers", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}
		session := &BrowserSession{
			Pages:       make(map[string]playwright.Page),
			ConsoleLogs: NewConsoleLogBuffer(10),
			NetworkLogs: NewNetworkLogBuffer(10),
		}

		session.AddPageWithLogging("tab-1", page)

		if len(page.consoleHandlers) != 1 {
			t.Errorf("Expected 1 console handler, got %d", len(page.consoleHandlers))
		}
		if len(page.requestHandlers) != 1 {
			t.Errorf("Expected 1 request handler, got %d", len(page.requestHandlers))
		}
	})

	t.Run("handles nil ConsoleLogs gracefully", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}
		session := &BrowserSession{
			Pages:       make(map[string]playwright.Page),
			ConsoleLogs: nil, // explicitly nil
			NetworkLogs: NewNetworkLogBuffer(10),
		}

		// Should not panic
		session.AddPageWithLogging("tab-1", page)

		if len(page.consoleHandlers) != 0 {
			t.Errorf("Expected 0 console handlers when ConsoleLogs is nil, got %d", len(page.consoleHandlers))
		}
	})

	t.Run("handles nil NetworkLogs gracefully", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}
		session := &BrowserSession{
			Pages:       make(map[string]playwright.Page),
			ConsoleLogs: NewConsoleLogBuffer(10),
			NetworkLogs: nil, // explicitly nil
		}

		// Should not panic
		session.AddPageWithLogging("tab-1", page)

		if len(page.requestHandlers) != 0 {
			t.Errorf("Expected 0 request handlers when NetworkLogs is nil, got %d", len(page.requestHandlers))
		}
		if session.networkCleanups != nil && len(session.networkCleanups) > 0 {
			t.Error("No cleanup should be stored when NetworkLogs is nil")
		}
	})

	t.Run("cleans up existing page cleanup before replacement", func(t *testing.T) {
		oldCleanupCalled := false
		oldPage := &mockPageWithEvents{id: "old-page"}
		newPage := &mockPageWithEvents{id: "new-page"}

		session := &BrowserSession{
			Pages:           map[string]playwright.Page{"tab-1": oldPage},
			NetworkLogs:     NewNetworkLogBuffer(10),
			networkCleanups: map[string]func(){"tab-1": func() { oldCleanupCalled = true }},
		}

		session.AddPageWithLogging("tab-1", newPage)

		if !oldCleanupCalled {
			t.Error("Old cleanup function should be called when replacing page")
		}
		if session.Pages["tab-1"] != newPage {
			t.Error("Page should be replaced with new page")
		}
	})

	t.Run("handles nil buffers with no handlers attached", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}
		session := &BrowserSession{
			Pages:       make(map[string]playwright.Page),
			ConsoleLogs: nil,
			NetworkLogs: nil,
		}

		// Should not panic
		session.AddPageWithLogging("tab-1", page)

		if len(page.consoleHandlers) != 0 {
			t.Error("No console handlers should be attached")
		}
		if len(page.requestHandlers) != 0 {
			t.Error("No request handlers should be attached")
		}
	})

	t.Run("concurrent AddPageWithLogging is thread-safe", func(t *testing.T) {
		session := &BrowserSession{
			Pages:       make(map[string]playwright.Page),
			ConsoleLogs: NewConsoleLogBuffer(100),
			NetworkLogs: NewNetworkLogBuffer(100),
		}

		var wg sync.WaitGroup

		// Concurrent adds
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				tabID := string(rune('a' + (idx % 26)))
				page := &mockPageWithEvents{id: tabID}
				session.AddPageWithLogging(tabID, page)
			}(i)
		}

		wg.Wait()

		// Should not panic and should have some pages
		if session.PageCount() == 0 {
			t.Error("Should have added some pages")
		}
	})
}

func TestBrowserSession_RemovePage_WithNetworkCleanup(t *testing.T) {
	t.Run("calls cleanup function when removing page", func(t *testing.T) {
		cleanupCalled := false
		page := &mockPageWithEvents{id: "test-page"}

		session := &BrowserSession{
			Pages:           map[string]playwright.Page{"tab-1": page},
			networkCleanups: map[string]func(){"tab-1": func() { cleanupCalled = true }},
		}

		session.RemovePage("tab-1")

		if !cleanupCalled {
			t.Error("Network cleanup should be called when removing page")
		}
		if _, exists := session.networkCleanups["tab-1"]; exists {
			t.Error("Cleanup function should be removed from map")
		}
	})

	t.Run("handles missing cleanup function gracefully", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}

		session := &BrowserSession{
			Pages:           map[string]playwright.Page{"tab-1": page},
			networkCleanups: make(map[string]func()), // Empty map, no cleanup registered
		}

		// Should not panic
		removed := session.RemovePage("tab-1")

		if removed != page {
			t.Error("Should return the removed page")
		}
	})

	t.Run("handles nil networkCleanups map gracefully", func(t *testing.T) {
		page := &mockPageWithEvents{id: "test-page"}

		session := &BrowserSession{
			Pages: map[string]playwright.Page{"tab-1": page},
			// networkCleanups is nil
		}

		// Should not panic
		removed := session.RemovePage("tab-1")

		if removed != page {
			t.Error("Should return the removed page")
		}
	})
}
