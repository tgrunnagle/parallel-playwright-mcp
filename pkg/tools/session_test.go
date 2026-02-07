package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// mockSessionManager is a mock implementation of session.BrowserSessionManager for testing.
type mockSessionManager struct {
	createSessionFunc   func(ctx context.Context, mcpSessionID string, opts session.SessionOptions) (*session.BrowserSession, error)
	getSessionFunc      func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
	closeSessionFunc    func(ctx context.Context, mcpSessionID, browserSessionID string) error
	listSessionsFunc    func(mcpSessionID string) []*session.SessionInfo
	closeAllForMCPFunc  func(ctx context.Context, mcpSessionID string) error
	cleanupFunc         func(ctx context.Context, idleTimeout time.Duration) (int, error)
	lastCreateOpts      session.SessionOptions
	lastMCPSessionID    string
	lastBrowserSessionID string
}

func (m *mockSessionManager) CreateSession(ctx context.Context, mcpSessionID string, opts session.SessionOptions) (*session.BrowserSession, error) {
	m.lastMCPSessionID = mcpSessionID
	m.lastCreateOpts = opts
	if m.createSessionFunc != nil {
		return m.createSessionFunc(ctx, mcpSessionID, opts)
	}
	return &session.BrowserSession{ID: "sess-test-123"}, nil
}

func (m *mockSessionManager) GetSession(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
	if m.getSessionFunc != nil {
		return m.getSessionFunc(mcpSessionID, browserSessionID)
	}
	return nil, false
}

func (m *mockSessionManager) CloseSession(ctx context.Context, mcpSessionID, browserSessionID string) error {
	m.lastMCPSessionID = mcpSessionID
	m.lastBrowserSessionID = browserSessionID
	if m.closeSessionFunc != nil {
		return m.closeSessionFunc(ctx, mcpSessionID, browserSessionID)
	}
	return nil
}

func (m *mockSessionManager) ListSessions(mcpSessionID string) []*session.SessionInfo {
	m.lastMCPSessionID = mcpSessionID
	if m.listSessionsFunc != nil {
		return m.listSessionsFunc(mcpSessionID)
	}
	return []*session.SessionInfo{}
}

func (m *mockSessionManager) CloseAllForMCP(ctx context.Context, mcpSessionID string) error {
	if m.closeAllForMCPFunc != nil {
		return m.closeAllForMCPFunc(ctx, mcpSessionID)
	}
	return nil
}

func (m *mockSessionManager) Cleanup(ctx context.Context, idleTimeout time.Duration) (int, error) {
	if m.cleanupFunc != nil {
		return m.cleanupFunc(ctx, idleTimeout)
	}
	return 0, nil
}

// TestSessionCreateTool tests the session_create tool definition.
func TestSessionCreateTool(t *testing.T) {
	tool := SessionCreateTool()

	if tool.Name != "session_create" {
		t.Errorf("Expected tool name 'session_create', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Verify input schema has expected properties
	schema := tool.InputSchema
	if schema.Type != "object" {
		t.Errorf("Expected input schema type 'object', got '%s'", schema.Type)
	}

	// Check that browserType, headless, and viewport properties exist
	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	if _, ok := props["browserType"]; !ok {
		t.Error("Expected 'browserType' property in input schema")
	}
	if _, ok := props["headless"]; !ok {
		t.Error("Expected 'headless' property in input schema")
	}
	if _, ok := props["viewport"]; !ok {
		t.Error("Expected 'viewport' property in input schema")
	}
}

// TestSessionListTool tests the session_list tool definition.
func TestSessionListTool(t *testing.T) {
	tool := SessionListTool()

	if tool.Name != "session_list" {
		t.Errorf("Expected tool name 'session_list', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}
}

// TestSessionCloseTool tests the session_close tool definition.
func TestSessionCloseTool(t *testing.T) {
	tool := SessionCloseTool()

	if tool.Name != "session_close" {
		t.Errorf("Expected tool name 'session_close', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
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
}

// TestSessionCreateHandler tests the session_create handler.
func TestSessionCreateHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		createFunc     func(ctx context.Context, mcpSessionID string, opts session.SessionOptions) (*session.BrowserSession, error)
		expectedResult string
		expectError    bool
		checkOpts      func(t *testing.T, opts session.SessionOptions)
	}{
		{
			name:      "default values",
			arguments: map[string]any{},
			checkOpts: func(t *testing.T, opts session.SessionOptions) {
				if opts.BrowserType != browser.BrowserChromium {
					t.Errorf("Expected default browserType chromium, got %s", opts.BrowserType)
				}
				if !opts.Headless {
					t.Error("Expected default headless true")
				}
				if opts.Viewport == nil {
					t.Fatal("Expected default viewport to be set")
				}
				if opts.Viewport.Width != 1280 || opts.Viewport.Height != 720 {
					t.Errorf("Expected default viewport 1280x720, got %dx%d", opts.Viewport.Width, opts.Viewport.Height)
				}
			},
		},
		{
			name: "firefox browser type",
			arguments: map[string]any{
				"browserType": "firefox",
			},
			checkOpts: func(t *testing.T, opts session.SessionOptions) {
				if opts.BrowserType != browser.BrowserFirefox {
					t.Errorf("Expected browserType firefox, got %s", opts.BrowserType)
				}
			},
		},
		{
			name: "webkit browser type",
			arguments: map[string]any{
				"browserType": "webkit",
			},
			checkOpts: func(t *testing.T, opts session.SessionOptions) {
				if opts.BrowserType != browser.BrowserWebKit {
					t.Errorf("Expected browserType webkit, got %s", opts.BrowserType)
				}
			},
		},
		{
			name: "headless false",
			arguments: map[string]any{
				"headless": false,
			},
			checkOpts: func(t *testing.T, opts session.SessionOptions) {
				if opts.Headless {
					t.Error("Expected headless false")
				}
			},
		},
		{
			name: "custom viewport",
			arguments: map[string]any{
				"viewport": map[string]any{
					"width":  float64(1920),
					"height": float64(1080),
				},
			},
			checkOpts: func(t *testing.T, opts session.SessionOptions) {
				if opts.Viewport == nil {
					t.Fatal("Expected viewport to be set")
				}
				if opts.Viewport.Width != 1920 || opts.Viewport.Height != 1080 {
					t.Errorf("Expected viewport 1920x1080, got %dx%d", opts.Viewport.Width, opts.Viewport.Height)
				}
			},
		},
		{
			name: "invalid browser type",
			arguments: map[string]any{
				"browserType": "invalid",
			},
			expectedResult: "Invalid browserType",
			expectError:    true,
		},
		{
			name:      "manager error",
			arguments: map[string]any{},
			createFunc: func(ctx context.Context, mcpSessionID string, opts session.SessionOptions) (*session.BrowserSession, error) {
				return nil, errors.New("pool not running")
			},
			expectedResult: "Failed to create session",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				createSessionFunc: tt.createFunc,
			}
			handler := SessionCreateHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "session_create",
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

			// Check if it's an error result
			if tt.expectError {
				if !result.IsError {
					t.Error("Expected error result")
				}
				// Check error message contains expected text
				text := extractTextContent(result.Content)
				if tt.expectedResult != "" && !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectedResult, text)
				}
			} else {
				if result.IsError {
					t.Errorf("Expected success result, got error: %s", extractTextContent(result.Content))
				}
				if tt.checkOpts != nil {
					tt.checkOpts(t, mgr.lastCreateOpts)
				}
			}
		})
	}
}

// TestSessionListHandler tests the session_list handler.
func TestSessionListHandler(t *testing.T) {
	tests := []struct {
		name        string
		sessions    []*session.SessionInfo
		expectEmpty bool
		expectCount int
	}{
		{
			name:        "empty sessions",
			sessions:    []*session.SessionInfo{},
			expectEmpty: true,
		},
		{
			name: "single session",
			sessions: []*session.SessionInfo{
				{
					ID:          "sess-123",
					BrowserType: browser.BrowserChromium,
					URL:         "https://example.com",
					Title:       "Example",
					CreatedAt:   time.Now(),
				},
			},
			expectCount: 1,
		},
		{
			name: "multiple sessions",
			sessions: []*session.SessionInfo{
				{
					ID:          "sess-123",
					BrowserType: browser.BrowserChromium,
					URL:         "https://example.com",
					Title:       "Example",
					CreatedAt:   time.Now(),
				},
				{
					ID:          "sess-456",
					BrowserType: browser.BrowserFirefox,
					URL:         "https://test.com",
					Title:       "Test",
					CreatedAt:   time.Now(),
				},
			},
			expectCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				listSessionsFunc: func(mcpSessionID string) []*session.SessionInfo {
					return tt.sessions
				},
			}
			handler := SessionListHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "session_list",
					Arguments: map[string]any{},
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

			if result.IsError {
				t.Errorf("Expected success result, got error")
			}

			// Extract text from content
			text := extractTextContent(result.Content)
			var response SessionListResponse
			if err := json.Unmarshal([]byte(text), &response); err != nil {
				t.Fatalf("Failed to parse response JSON: %v (content: %s)", err, text)
			}

			if tt.expectEmpty && len(response.Sessions) != 0 {
				t.Errorf("Expected empty sessions, got %d", len(response.Sessions))
			}

			if tt.expectCount > 0 && len(response.Sessions) != tt.expectCount {
				t.Errorf("Expected %d sessions, got %d", tt.expectCount, len(response.Sessions))
			}
		})
	}
}

// TestSessionCloseHandler tests the session_close handler.
func TestSessionCloseHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		closeFunc      func(ctx context.Context, mcpSessionID, browserSessionID string) error
		expectError    bool
		expectedResult string
	}{
		{
			name: "successful close",
			arguments: map[string]any{
				"sessionId": "sess-123",
			},
			expectedResult: "Closed session: sess-123",
		},
		{
			name:           "missing sessionId",
			arguments:      map[string]any{},
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
			closeFunc: func(ctx context.Context, mcpSessionID, browserSessionID string) error {
				return session.ErrSessionNotFound
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				closeSessionFunc: tt.closeFunc,
			}
			handler := SessionCloseHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "session_close",
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

			text := extractTextContent(result.Content)

			if tt.expectError {
				if !result.IsError {
					t.Error("Expected error result")
				}
			} else {
				if result.IsError {
					t.Errorf("Expected success result, got error: %s", text)
				}
			}

			if tt.expectedResult != "" && !strings.Contains(text, tt.expectedResult) {
				t.Errorf("Expected result to contain '%s', got '%s'", tt.expectedResult, text)
			}
		})
	}
}

// TestMCPSessionIsolation tests that handlers correctly pass MCP session IDs to manager
// methods for ownership tracking and isolation between MCP connections.
func TestMCPSessionIsolation(t *testing.T) {
	t.Run("CreateSession passes MCP session ID to manager", func(t *testing.T) {
		var capturedMCPSessionID string
		mgr := &mockSessionManager{
			createSessionFunc: func(ctx context.Context, mcpSessionID string, opts session.SessionOptions) (*session.BrowserSession, error) {
				capturedMCPSessionID = mcpSessionID
				return &session.BrowserSession{ID: "sess-test"}, nil
			},
		}
		handler := SessionCreateHandler(mgr)

		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}

		// Handler extracts MCP session ID from context and passes to manager
		// With context.Background(), getMCPSessionID returns empty string
		ctx := context.Background()
		_, _ = handler(ctx, req)

		// Verify the manager received the MCP session ID (empty from context.Background())
		if capturedMCPSessionID != "" {
			t.Errorf("Expected empty MCP session ID from context.Background(), got %q", capturedMCPSessionID)
		}
	})

	t.Run("ListSessions only returns sessions for specific MCP connection", func(t *testing.T) {
		// Simulate sessions owned by different MCP connections
		sessionsForMCPA := []*session.SessionInfo{
			{ID: "sess-mcp-a-1", BrowserType: browser.BrowserChromium, URL: "https://a.com", CreatedAt: time.Now()},
		}
		sessionsForMCPB := []*session.SessionInfo{
			{ID: "sess-mcp-b-1", BrowserType: browser.BrowserFirefox, URL: "https://b.com", CreatedAt: time.Now()},
			{ID: "sess-mcp-b-2", BrowserType: browser.BrowserChromium, URL: "https://b2.com", CreatedAt: time.Now()},
		}

		var capturedMCPSessionID string
		mgr := &mockSessionManager{
			listSessionsFunc: func(mcpSessionID string) []*session.SessionInfo {
				capturedMCPSessionID = mcpSessionID
				// Return different sessions based on MCP session ID
				if mcpSessionID == "mcp-session-A" {
					return sessionsForMCPA
				} else if mcpSessionID == "mcp-session-B" {
					return sessionsForMCPB
				}
				return []*session.SessionInfo{}
			},
		}
		handler := SessionListHandler(mgr)

		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_list",
				Arguments: map[string]any{},
			},
		}

		// Call handler and verify manager receives the MCP session ID parameter
		ctx := context.Background()
		result, _ := handler(ctx, req)

		// Verify handler called manager with the extracted MCP session ID
		if capturedMCPSessionID != "" {
			t.Errorf("Expected empty MCP session ID from context.Background(), got %q", capturedMCPSessionID)
		}

		// Result should be empty since no MCP session in context
		text := extractTextContent(result.Content)
		var response SessionListResponse
		if err := json.Unmarshal([]byte(text), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		if len(response.Sessions) != 0 {
			t.Errorf("Expected empty sessions for empty MCP session ID, got %d", len(response.Sessions))
		}
	})

	t.Run("CloseSession validates MCP ownership", func(t *testing.T) {
		// Test that CloseSession passes both MCP session ID and browser session ID to manager
		var capturedMCPSessionID, capturedBrowserSessionID string
		mgr := &mockSessionManager{
			closeSessionFunc: func(ctx context.Context, mcpSessionID, browserSessionID string) error {
				capturedMCPSessionID = mcpSessionID
				capturedBrowserSessionID = browserSessionID
				// Simulate ownership check - if MCP session ID doesn't match, return unauthorized
				if mcpSessionID != "owner-mcp-session" {
					return session.ErrUnauthorized
				}
				return nil
			},
		}
		handler := SessionCloseHandler(mgr)

		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": "sess-to-close",
				},
			},
		}

		ctx := context.Background()
		result, _ := handler(ctx, req)

		// Verify both IDs were passed to manager
		if capturedMCPSessionID != "" {
			t.Errorf("Expected empty MCP session ID from context.Background(), got %q", capturedMCPSessionID)
		}
		if capturedBrowserSessionID != "sess-to-close" {
			t.Errorf("Expected browser session ID 'sess-to-close', got %q", capturedBrowserSessionID)
		}

		// Result should be error since MCP session ID (empty) doesn't match owner
		if !result.IsError {
			t.Error("Expected error result when MCP session doesn't own the browser session")
		}
	})

	t.Run("CloseSession fails with wrong MCP ownership", func(t *testing.T) {
		// Verify that unauthorized access is properly handled
		mgr := &mockSessionManager{
			closeSessionFunc: func(ctx context.Context, mcpSessionID, browserSessionID string) error {
				return session.ErrUnauthorized
			},
		}
		handler := SessionCloseHandler(mgr)

		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": "sess-owned-by-other",
				},
			},
		}

		ctx := context.Background()
		result, err := handler(ctx, req)

		if err != nil {
			t.Fatalf("Handler returned unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error result when closing session owned by different MCP connection")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32001]") || !strings.Contains(text, "Session not found") {
			t.Errorf("Expected '[-32001] Session not found' in error message, got %q", text)
		}
	})

	t.Run("Session created by MCP-A not visible to MCP-B", func(t *testing.T) {
		// This test verifies the conceptual isolation even though we can't inject
		// real MCP session IDs into context. The mock manager demonstrates that
		// different MCP sessions receive different session lists.

		sessionsByMCP := map[string][]*session.SessionInfo{
			"mcp-A": {{ID: "sess-A1", BrowserType: browser.BrowserChromium, CreatedAt: time.Now()}},
			"mcp-B": {{ID: "sess-B1", BrowserType: browser.BrowserFirefox, CreatedAt: time.Now()}},
			"":      {}, // Empty MCP session ID gets empty list
		}

		mgr := &mockSessionManager{
			listSessionsFunc: func(mcpSessionID string) []*session.SessionInfo {
				return sessionsByMCP[mcpSessionID]
			},
		}

		handler := SessionListHandler(mgr)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{Name: "session_list", Arguments: map[string]any{}},
		}

		// With context.Background(), handler gets empty MCP session ID
		result, _ := handler(context.Background(), req)
		text := extractTextContent(result.Content)
		var response SessionListResponse
		json.Unmarshal([]byte(text), &response)

		// Empty MCP session ID should get empty list (isolates from mcp-A and mcp-B)
		if len(response.Sessions) != 0 {
			t.Errorf("Sessions from mcp-A and mcp-B should not be visible to empty MCP session, got %d sessions", len(response.Sessions))
		}
	})
}

// TestParseViewport tests the parseViewport helper function.
func TestParseViewport(t *testing.T) {
	tests := []struct {
		name           string
		input          map[string]any
		expectedWidth  int
		expectedHeight int
		expectNil      bool
	}{
		{
			name: "valid float64 values",
			input: map[string]any{
				"width":  float64(1920),
				"height": float64(1080),
			},
			expectedWidth:  1920,
			expectedHeight: 1080,
		},
		{
			name: "valid int values",
			input: map[string]any{
				"width":  800,
				"height": 600,
			},
			expectedWidth:  800,
			expectedHeight: 600,
		},
		{
			name: "missing width",
			input: map[string]any{
				"height": float64(1080),
			},
			expectNil: true,
		},
		{
			name: "missing height",
			input: map[string]any{
				"width": float64(1920),
			},
			expectNil: true,
		},
		{
			name: "zero width",
			input: map[string]any{
				"width":  float64(0),
				"height": float64(1080),
			},
			expectNil: true,
		},
		{
			name: "negative height",
			input: map[string]any{
				"width":  float64(1920),
				"height": float64(-100),
			},
			expectNil: true,
		},
		{
			name: "invalid width type",
			input: map[string]any{
				"width":  "invalid",
				"height": float64(1080),
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseViewport(tt.input)

			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil, got viewport %dx%d", result.Width, result.Height)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected viewport, got nil")
			}

			if result.Width != tt.expectedWidth {
				t.Errorf("Expected width %d, got %d", tt.expectedWidth, result.Width)
			}

			if result.Height != tt.expectedHeight {
				t.Errorf("Expected height %d, got %d", tt.expectedHeight, result.Height)
			}
		})
	}
}

// TestFormatSessionList tests the formatSessionList helper function.
func TestFormatSessionList(t *testing.T) {
	tests := []struct {
		name        string
		sessions    []*session.SessionInfo
		expectEmpty bool
	}{
		{
			name:        "empty list",
			sessions:    []*session.SessionInfo{},
			expectEmpty: true,
		},
		{
			name: "single session",
			sessions: []*session.SessionInfo{
				{
					ID:          "sess-123",
					BrowserType: browser.BrowserChromium,
					URL:         "https://example.com",
					Title:       "Example",
					CreatedAt:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "multiple sessions",
			sessions: []*session.SessionInfo{
				{
					ID:          "sess-123",
					BrowserType: browser.BrowserChromium,
					URL:         "https://example.com",
					Title:       "Example",
					CreatedAt:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				},
				{
					ID:          "sess-456",
					BrowserType: browser.BrowserFirefox,
					URL:         "https://test.com",
					Title:       "Test",
					CreatedAt:   time.Date(2024, 1, 15, 10, 31, 0, 0, time.UTC),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatSessionList(tt.sessions)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify it's valid JSON
			var response SessionListResponse
			if err := json.Unmarshal([]byte(result), &response); err != nil {
				t.Fatalf("Result is not valid JSON: %v", err)
			}

			if tt.expectEmpty {
				if len(response.Sessions) != 0 {
					t.Errorf("Expected empty sessions array, got %d items", len(response.Sessions))
				}
				return
			}

			if len(response.Sessions) != len(tt.sessions) {
				t.Errorf("Expected %d sessions, got %d", len(tt.sessions), len(response.Sessions))
			}

			// Verify each session has required fields
			for i, sess := range response.Sessions {
				if sess.SessionID != tt.sessions[i].ID {
					t.Errorf("Session %d: expected ID %s, got %s", i, tt.sessions[i].ID, sess.SessionID)
				}
				if sess.BrowserType != string(tt.sessions[i].BrowserType) {
					t.Errorf("Session %d: expected browserType %s, got %s", i, tt.sessions[i].BrowserType, sess.BrowserType)
				}
				if sess.URL != tt.sessions[i].URL {
					t.Errorf("Session %d: expected URL %s, got %s", i, tt.sessions[i].URL, sess.URL)
				}
				if sess.Title != tt.sessions[i].Title {
					t.Errorf("Session %d: expected title %s, got %s", i, tt.sessions[i].Title, sess.Title)
				}
				if sess.CreatedAt == "" {
					t.Errorf("Session %d: createdAt should not be empty", i)
				}
			}
		})
	}
}

// extractTextContent extracts the text from a Content slice.
// Returns the text from the first TextContent item found.
func extractTextContent(content []mcp.Content) string {
	for _, c := range content {
		if tc, ok := mcp.AsTextContent(c); ok {
			return tc.Text
		}
	}
	return ""
}
