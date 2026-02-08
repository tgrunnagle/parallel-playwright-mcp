package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// TestGetNetworkLogsTool tests the get_network_logs tool definition.
func TestGetNetworkLogsTool(t *testing.T) {
	tool := GetNetworkLogsTool()

	if tool.Name != "get_network_logs" {
		t.Errorf("Expected tool name 'get_network_logs', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	if !strings.Contains(tool.Description, "network") {
		t.Error("Tool description should mention network request/response logs")
	}

	// Verify sessionId is in required
	required := tool.InputSchema.Required
	found := false
	for _, r := range required {
		if r == "sessionId" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'sessionId' to be in required properties")
	}

	// Verify properties exist
	props := tool.InputSchema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["limit"]; !ok {
		t.Error("Expected 'limit' property in input schema")
	}
	if _, ok := props["filter"]; !ok {
		t.Error("Expected 'filter' property in input schema")
	}
}

// TestGetNetworkLogsHandler tests the get_network_logs handler.
func TestGetNetworkLogsHandler(t *testing.T) {
	// Helper to create a session with network logs
	createSessionWithLogs := func(entries []session.NetworkLogEntry) *session.BrowserSession {
		buf := session.NewNetworkLogBuffer(100)
		for _, e := range entries {
			buf.Add(e)
		}
		return &session.BrowserSession{
			ID:          "sess-test",
			NetworkLogs: buf,
		}
	}

	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
		checkResult    func(t *testing.T, result *mcp.CallToolResult)
	}{
		{
			name: "missing sessionId",
			arguments: map[string]any{
				"limit": float64(10),
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "empty sessionId",
			arguments: map[string]any{
				"sessionId": "",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
		{
			name: "nil NetworkLogs buffer returns empty array",
			arguments: map[string]any{
				"sessionId": "sess-test",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return &session.BrowserSession{
					ID:          "sess-test",
					NetworkLogs: nil,
				}, true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				if text != "[]" {
					t.Errorf("Expected empty array '[]', got '%s'", text)
				}
			},
		},
		{
			name: "empty network logs",
			arguments: map[string]any{
				"sessionId": "sess-test",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return createSessionWithLogs([]session.NetworkLogEntry{}), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []networkLogOutput
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 0 {
					t.Errorf("Expected empty entries, got %d", len(entries))
				}
			},
		},
		{
			name: "retrieve all logs without filters",
			arguments: map[string]any{
				"sessionId": "sess-test",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				entries := []session.NetworkLogEntry{
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/1", Status: 200},
					{Timestamp: time.Now(), Method: "POST", URL: "https://example.com/2", Status: 201},
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/3", Status: 404},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []networkLogOutput
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 3 {
					t.Errorf("Expected 3 entries, got %d", len(entries))
				}
			},
		},
		{
			name: "entries returned in reverse chronological order (most recent first)",
			arguments: map[string]any{
				"sessionId": "sess-test",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				entries := []session.NetworkLogEntry{
					{Timestamp: time.Now().Add(-2 * time.Second), Method: "GET", URL: "https://example.com/first", Status: 200},
					{Timestamp: time.Now().Add(-1 * time.Second), Method: "GET", URL: "https://example.com/second", Status: 200},
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/third", Status: 200},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []networkLogOutput
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 3 {
					t.Fatalf("Expected 3 entries, got %d", len(entries))
				}
				// Most recent should be first (third was added last)
				if !strings.Contains(entries[0].URL, "third") {
					t.Errorf("Expected most recent entry first (third), got %s", entries[0].URL)
				}
				if !strings.Contains(entries[2].URL, "first") {
					t.Errorf("Expected oldest entry last (first), got %s", entries[2].URL)
				}
			},
		},
		{
			name: "retrieve with limit",
			arguments: map[string]any{
				"sessionId": "sess-test",
				"limit":     float64(2),
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				entries := []session.NetworkLogEntry{
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/1", Status: 200},
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/2", Status: 200},
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/3", Status: 200},
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/4", Status: 200},
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/5", Status: 200},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []networkLogOutput
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 2 {
					t.Errorf("Expected 2 entries (limited), got %d", len(entries))
				}
			},
		},
		{
			name: "limit of 0 returns all entries",
			arguments: map[string]any{
				"sessionId": "sess-test",
				"limit":     float64(0),
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				entries := []session.NetworkLogEntry{
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/1", Status: 200},
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/2", Status: 200},
					{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/3", Status: 200},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []networkLogOutput
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 3 {
					t.Errorf("Expected 3 entries (all), got %d", len(entries))
				}
			},
		},
		{
			name: "entry includes all fields with correct formatting",
			arguments: map[string]any{
				"sessionId": "sess-test",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				entries := []session.NetworkLogEntry{
					{
						Timestamp:    time.Date(2024, 1, 15, 10, 30, 45, 123000000, time.UTC),
						Method:       "POST",
						URL:          "https://api.example.com/data",
						Status:       201,
						Duration:     150 * time.Millisecond,
						RequestSize:  1024,
						ResponseSize: 2048,
						ResourceType: "fetch",
					},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []networkLogOutput
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 1 {
					t.Fatalf("Expected 1 entry, got %d", len(entries))
				}
				e := entries[0]
				if e.Method != "POST" {
					t.Errorf("Method = %s, want POST", e.Method)
				}
				if e.URL != "https://api.example.com/data" {
					t.Errorf("URL = %s, want https://api.example.com/data", e.URL)
				}
				if e.Status != 201 {
					t.Errorf("Status = %d, want 201", e.Status)
				}
				if e.DurationMs != 150 {
					t.Errorf("DurationMs = %d, want 150", e.DurationMs)
				}
				if e.RequestSize != 1024 {
					t.Errorf("RequestSize = %d, want 1024", e.RequestSize)
				}
				if e.ResponseSize != 2048 {
					t.Errorf("ResponseSize = %d, want 2048", e.ResponseSize)
				}
				if e.ResourceType != "fetch" {
					t.Errorf("ResourceType = %s, want fetch", e.ResourceType)
				}
				// Check timestamp is ISO 8601 format
				if !strings.Contains(e.Timestamp, "2024-01-15T10:30:45") {
					t.Errorf("Timestamp not in expected format, got %s", e.Timestamp)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}
			handler := GetNetworkLogsHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_network_logs",
					Arguments: tt.arguments,
				},
			}

			ctx := context.Background()
			result, err := handler(ctx, req)

			if err != nil {
				t.Fatalf("Handler returned unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			if tt.expectError {
				if !result.IsError {
					t.Error("Expected error result")
				}
				text := extractTextContent(result.Content)
				if tt.expectedResult != "" && !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectedResult, text)
				}
			} else {
				if result.IsError {
					text := extractTextContent(result.Content)
					t.Errorf("Expected success result, got error: %s", text)
				}
				if tt.checkResult != nil {
					tt.checkResult(t, result)
				}
			}
		})
	}
}

// TestGetNetworkLogsHandler_URLPatternFilter tests URL pattern filtering.
func TestGetNetworkLogsHandler_URLPatternFilter(t *testing.T) {
	createSessionWithLogs := func() *session.BrowserSession {
		buf := session.NewNetworkLogBuffer(100)
		entries := []session.NetworkLogEntry{
			{Timestamp: time.Now(), Method: "GET", URL: "https://api.example.com/users", Status: 200},
			{Timestamp: time.Now(), Method: "GET", URL: "https://api.example.com/posts", Status: 200},
			{Timestamp: time.Now(), Method: "GET", URL: "https://cdn.example.com/image.png", Status: 200},
			{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/about", Status: 200},
		}
		for _, e := range entries {
			buf.Add(e)
		}
		return &session.BrowserSession{
			ID:          "sess-test",
			NetworkLogs: buf,
		}
	}

	tests := []struct {
		name          string
		filter        map[string]any
		expectError   bool
		expectedCount int
		errorContains string
	}{
		{
			name:          "filter by URL pattern - api subdomain",
			filter:        map[string]any{"urlPattern": "api\\.example\\.com"},
			expectedCount: 2,
		},
		{
			name:          "filter by URL pattern - users endpoint",
			filter:        map[string]any{"urlPattern": "/users$"},
			expectedCount: 1,
		},
		{
			name:          "filter by URL pattern - no matches",
			filter:        map[string]any{"urlPattern": "nonexistent"},
			expectedCount: 0,
		},
		{
			name:          "filter by URL pattern - match all",
			filter:        map[string]any{"urlPattern": "example\\.com"},
			expectedCount: 4,
		},
		{
			name:          "invalid regex pattern",
			filter:        map[string]any{"urlPattern": "[invalid(regex"},
			expectError:   true,
			errorContains: "invalid urlPattern regex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
					return createSessionWithLogs(), true
				},
			}
			handler := GetNetworkLogsHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_network_logs",
					Arguments: map[string]any{
						"sessionId": "sess-test",
						"filter":    tt.filter,
					},
				},
			}

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Handler error: %v", err)
			}

			if tt.expectError {
				if !result.IsError {
					t.Error("Expected error result")
				}
				text := extractTextContent(result.Content)
				if !strings.Contains(text, tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorContains, text)
				}
				return
			}

			if result.IsError {
				t.Errorf("Unexpected error: %s", extractTextContent(result.Content))
				return
			}

			text := extractTextContent(result.Content)
			var entries []networkLogOutput
			if err := json.Unmarshal([]byte(text), &entries); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}
			if len(entries) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(entries))
			}
		})
	}
}

// TestGetNetworkLogsHandler_StatusFilters tests status code filtering.
func TestGetNetworkLogsHandler_StatusFilters(t *testing.T) {
	createSessionWithLogs := func() *session.BrowserSession {
		buf := session.NewNetworkLogBuffer(100)
		entries := []session.NetworkLogEntry{
			{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/1", Status: 200},
			{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/2", Status: 201},
			{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/3", Status: 301},
			{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/4", Status: 404},
			{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/5", Status: 500},
			{Timestamp: time.Now(), Method: "GET", URL: "https://example.com/6", Status: 0}, // Failed request
		}
		for _, e := range entries {
			buf.Add(e)
		}
		return &session.BrowserSession{
			ID:          "sess-test",
			NetworkLogs: buf,
		}
	}

	tests := []struct {
		name          string
		filter        map[string]any
		expectedCount int
	}{
		{
			name:          "filter by exact status 200",
			filter:        map[string]any{"status": float64(200)},
			expectedCount: 1,
		},
		{
			name:          "filter by exact status 404",
			filter:        map[string]any{"status": float64(404)},
			expectedCount: 1,
		},
		{
			name:          "filter by statusMin 400 (error statuses)",
			filter:        map[string]any{"statusMin": float64(400)},
			expectedCount: 2, // 404, 500
		},
		{
			name:          "filter by statusMax 299 (success statuses)",
			filter:        map[string]any{"statusMax": float64(299)},
			expectedCount: 3, // 200, 201, 0 (failed request has status 0)
		},
		{
			name: "filter by status range 200-299",
			filter: map[string]any{
				"statusMin": float64(200),
				"statusMax": float64(299),
			},
			expectedCount: 2, // 200, 201
		},
		{
			name: "filter by status range 300-399 (redirects)",
			filter: map[string]any{
				"statusMin": float64(300),
				"statusMax": float64(399),
			},
			expectedCount: 1, // 301
		},
		{
			name:          "filter for failed requests (status 0)",
			filter:        map[string]any{"status": float64(0)},
			expectedCount: 1,
		},
		{
			name: "statusMin equals statusMax (exact match via range)",
			filter: map[string]any{
				"statusMin": float64(404),
				"statusMax": float64(404),
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
					return createSessionWithLogs(), true
				},
			}
			handler := GetNetworkLogsHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_network_logs",
					Arguments: map[string]any{
						"sessionId": "sess-test",
						"filter":    tt.filter,
					},
				},
			}

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Handler error: %v", err)
			}

			if result.IsError {
				t.Errorf("Unexpected error: %s", extractTextContent(result.Content))
				return
			}

			text := extractTextContent(result.Content)
			var entries []networkLogOutput
			if err := json.Unmarshal([]byte(text), &entries); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}
			if len(entries) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(entries))
			}
		})
	}
}

// TestGetNetworkLogsHandler_CombinedFilters tests combining multiple filter criteria.
func TestGetNetworkLogsHandler_CombinedFilters(t *testing.T) {
	createSessionWithLogs := func() *session.BrowserSession {
		buf := session.NewNetworkLogBuffer(100)
		entries := []session.NetworkLogEntry{
			{Timestamp: time.Now(), Method: "GET", URL: "https://api.example.com/users", Status: 200},
			{Timestamp: time.Now(), Method: "GET", URL: "https://api.example.com/posts", Status: 404},
			{Timestamp: time.Now(), Method: "GET", URL: "https://cdn.example.com/image.png", Status: 200},
			{Timestamp: time.Now(), Method: "GET", URL: "https://api.example.com/comments", Status: 500},
		}
		for _, e := range entries {
			buf.Add(e)
		}
		return &session.BrowserSession{
			ID:          "sess-test",
			NetworkLogs: buf,
		}
	}

	tests := []struct {
		name          string
		filter        map[string]any
		expectedCount int
	}{
		{
			name: "URL pattern AND status 200",
			filter: map[string]any{
				"urlPattern": "api\\.example\\.com",
				"status":     float64(200),
			},
			expectedCount: 1, // Only api.example.com with 200
		},
		{
			name: "URL pattern AND status range 400+",
			filter: map[string]any{
				"urlPattern": "api\\.example\\.com",
				"statusMin":  float64(400),
			},
			expectedCount: 2, // api.example.com with 404 and 500
		},
		{
			name: "URL pattern AND status range 400-499",
			filter: map[string]any{
				"urlPattern": "api\\.example\\.com",
				"statusMin":  float64(400),
				"statusMax":  float64(499),
			},
			expectedCount: 1, // api.example.com with 404 only
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
					return createSessionWithLogs(), true
				},
			}
			handler := GetNetworkLogsHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_network_logs",
					Arguments: map[string]any{
						"sessionId": "sess-test",
						"filter":    tt.filter,
					},
				},
			}

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Handler error: %v", err)
			}

			if result.IsError {
				t.Errorf("Unexpected error: %s", extractTextContent(result.Content))
				return
			}

			text := extractTextContent(result.Content)
			var entries []networkLogOutput
			if err := json.Unmarshal([]byte(text), &entries); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}
			if len(entries) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(entries))
			}
		})
	}
}

// TestReverseNetworkEntries tests the reverseNetworkEntries helper function.
func TestReverseNetworkEntries(t *testing.T) {
	tests := []struct {
		name     string
		input    []session.NetworkLogEntry
		expected []string // Expected URLs in order
	}{
		{
			name:     "empty slice",
			input:    []session.NetworkLogEntry{},
			expected: []string{},
		},
		{
			name: "single entry",
			input: []session.NetworkLogEntry{
				{URL: "https://example.com/1"},
			},
			expected: []string{"https://example.com/1"},
		},
		{
			name: "multiple entries",
			input: []session.NetworkLogEntry{
				{URL: "https://example.com/1"},
				{URL: "https://example.com/2"},
				{URL: "https://example.com/3"},
			},
			expected: []string{
				"https://example.com/3",
				"https://example.com/2",
				"https://example.com/1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reverseNetworkEntries(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d entries, got %d", len(tt.expected), len(result))
			}

			for i, url := range tt.expected {
				if result[i].URL != url {
					t.Errorf("Entry %d: expected URL %s, got %s", i, url, result[i].URL)
				}
			}
		})
	}
}

// TestApplyNetworkFilters tests the applyNetworkFilters helper function.
func TestApplyNetworkFilters(t *testing.T) {
	baseEntries := []session.NetworkLogEntry{
		{URL: "https://api.example.com/users", Status: 200},
		{URL: "https://api.example.com/posts", Status: 404},
		{URL: "https://cdn.example.com/image.png", Status: 200},
	}

	tests := []struct {
		name          string
		filter        map[string]any
		expectError   bool
		expectedCount int
	}{
		{
			name:          "empty filter returns all",
			filter:        map[string]any{},
			expectedCount: 3,
		},
		{
			name:          "urlPattern filter",
			filter:        map[string]any{"urlPattern": "api\\.example\\.com"},
			expectedCount: 2,
		},
		{
			name:          "status filter",
			filter:        map[string]any{"status": float64(200)},
			expectedCount: 2,
		},
		{
			name:          "statusMin filter",
			filter:        map[string]any{"statusMin": float64(400)},
			expectedCount: 1,
		},
		{
			name:          "statusMax filter",
			filter:        map[string]any{"statusMax": float64(299)},
			expectedCount: 2,
		},
		{
			name:        "invalid regex",
			filter:      map[string]any{"urlPattern": "[invalid"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applyNetworkFilters(baseEntries, tt.filter)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(result))
			}
		})
	}
}

// TestApplyNetworkFilters_EmptyResult tests that empty results return empty slice, not nil.
func TestApplyNetworkFilters_EmptyResult(t *testing.T) {
	entries := []session.NetworkLogEntry{
		{URL: "https://example.com", Status: 200},
	}
	filter := map[string]any{"status": float64(404)}

	result, err := applyNetworkFilters(entries, filter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Error("Expected empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(result))
	}
}

// TestGetNetworkLogsHandler_EmptyFilter tests that empty filter object returns all entries.
func TestGetNetworkLogsHandler_EmptyFilter(t *testing.T) {
	buf := session.NewNetworkLogBuffer(100)
	for i := 0; i < 5; i++ {
		buf.Add(session.NetworkLogEntry{
			Timestamp: time.Now(),
			Method:    "GET",
			URL:       "https://example.com",
			Status:    200,
		})
	}

	mgr := &mockSessionManager{
		getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
			return &session.BrowserSession{
				ID:          "sess-test",
				NetworkLogs: buf,
			}, true
		},
	}
	handler := GetNetworkLogsHandler(mgr)

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_network_logs",
			Arguments: map[string]any{
				"sessionId": "sess-test",
				"filter":    map[string]any{}, // Empty filter
			},
		},
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler error: %v", err)
	}

	if result.IsError {
		t.Errorf("Unexpected error: %s", extractTextContent(result.Content))
		return
	}

	text := extractTextContent(result.Content)
	var entries []networkLogOutput
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("Expected 5 entries with empty filter, got %d", len(entries))
	}
}

// TestNetworkLogOutputSerialization tests JSON serialization of networkLogOutput.
func TestNetworkLogOutputSerialization(t *testing.T) {
	output := networkLogOutput{
		Timestamp:    "2024-01-15T10:30:45.123Z",
		Method:       "POST",
		URL:          "https://api.example.com/data",
		Status:       201,
		DurationMs:   150,
		RequestSize:  1024,
		ResponseSize: 2048,
		ResourceType: "fetch",
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("Failed to marshal networkLogOutput: %v", err)
	}

	var decoded networkLogOutput
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal networkLogOutput: %v", err)
	}

	if decoded.Timestamp != output.Timestamp {
		t.Errorf("Timestamp = %s, want %s", decoded.Timestamp, output.Timestamp)
	}
	if decoded.Method != output.Method {
		t.Errorf("Method = %s, want %s", decoded.Method, output.Method)
	}
	if decoded.URL != output.URL {
		t.Errorf("URL = %s, want %s", decoded.URL, output.URL)
	}
	if decoded.Status != output.Status {
		t.Errorf("Status = %d, want %d", decoded.Status, output.Status)
	}
	if decoded.DurationMs != output.DurationMs {
		t.Errorf("DurationMs = %d, want %d", decoded.DurationMs, output.DurationMs)
	}
	if decoded.RequestSize != output.RequestSize {
		t.Errorf("RequestSize = %d, want %d", decoded.RequestSize, output.RequestSize)
	}
	if decoded.ResponseSize != output.ResponseSize {
		t.Errorf("ResponseSize = %d, want %d", decoded.ResponseSize, output.ResponseSize)
	}
	if decoded.ResourceType != output.ResourceType {
		t.Errorf("ResourceType = %s, want %s", decoded.ResourceType, output.ResourceType)
	}
}
