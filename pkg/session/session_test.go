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
