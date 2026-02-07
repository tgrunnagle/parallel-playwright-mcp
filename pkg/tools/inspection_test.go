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

// TestGetConsoleLogsTool tests the get_console_logs tool definition.
func TestGetConsoleLogsTool(t *testing.T) {
	tool := GetConsoleLogsTool()

	if tool.Name != "get_console_logs" {
		t.Errorf("Expected tool name 'get_console_logs', got '%s'", tool.Name)
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
	if _, ok := props["level"]; !ok {
		t.Error("Expected 'level' property in input schema")
	}
}

// TestGetConsoleLogsHandler tests the get_console_logs handler.
func TestGetConsoleLogsHandler(t *testing.T) {
	// Helper to create a session with console logs
	createSessionWithLogs := func(entries []session.ConsoleLogEntry) *session.BrowserSession {
		buf := session.NewConsoleLogBuffer(100)
		for _, e := range entries {
			buf.Add(e)
		}
		return &session.BrowserSession{
			ID:          "sess-test",
			ConsoleLogs: buf,
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
			name: "empty console logs",
			arguments: map[string]any{
				"sessionId": "sess-test",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return createSessionWithLogs([]session.ConsoleLogEntry{}), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []session.ConsoleLogEntry
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
				entries := []session.ConsoleLogEntry{
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelLog, Text: "log message"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelError, Text: "error message"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelWarn, Text: "warn message"},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []session.ConsoleLogEntry
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 3 {
					t.Errorf("Expected 3 entries, got %d", len(entries))
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
				entries := []session.ConsoleLogEntry{
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelLog, Text: "message 1"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelLog, Text: "message 2"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelLog, Text: "message 3"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelLog, Text: "message 4"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelLog, Text: "message 5"},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []session.ConsoleLogEntry
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 2 {
					t.Errorf("Expected 2 entries (limited), got %d", len(entries))
				}
			},
		},
		{
			name: "retrieve with level filter",
			arguments: map[string]any{
				"sessionId": "sess-test",
				"level":     "error",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				entries := []session.ConsoleLogEntry{
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelLog, Text: "log"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelError, Text: "error1"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelWarn, Text: "warn"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelError, Text: "error2"},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []session.ConsoleLogEntry
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 2 {
					t.Errorf("Expected 2 error entries, got %d", len(entries))
				}
				for _, e := range entries {
					if e.Level != session.ConsoleLogLevelError {
						t.Errorf("Expected error level, got %s", e.Level)
					}
				}
			},
		},
		{
			name: "retrieve with both limit and level filter",
			arguments: map[string]any{
				"sessionId": "sess-test",
				"limit":     float64(1),
				"level":     "warn",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				entries := []session.ConsoleLogEntry{
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelWarn, Text: "warn1"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelWarn, Text: "warn2"},
					{Timestamp: time.Now(), Level: session.ConsoleLogLevelWarn, Text: "warn3"},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []session.ConsoleLogEntry
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 1 {
					t.Errorf("Expected 1 entry, got %d", len(entries))
				}
			},
		},
		{
			name: "default limit is 50",
			arguments: map[string]any{
				"sessionId": "sess-test",
				// no limit specified, should default to 50
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				// Add 100 entries
				entries := make([]session.ConsoleLogEntry, 100)
				for i := 0; i < 100; i++ {
					entries[i] = session.ConsoleLogEntry{
						Timestamp: time.Now(),
						Level:     session.ConsoleLogLevelLog,
						Text:      "message",
					}
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []session.ConsoleLogEntry
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 50 {
					t.Errorf("Expected 50 entries (default limit), got %d", len(entries))
				}
			},
		},
		{
			name: "entry includes all fields",
			arguments: map[string]any{
				"sessionId": "sess-test",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				entries := []session.ConsoleLogEntry{
					{
						Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 123000000, time.UTC),
						Level:     session.ConsoleLogLevelError,
						Text:      "Test error message",
						URL:       "https://example.com/app.js",
						Line:      42,
						Column:    15,
					},
				}
				return createSessionWithLogs(entries), true
			},
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text := extractTextContent(result.Content)
				var entries []session.ConsoleLogEntry
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if len(entries) != 1 {
					t.Fatalf("Expected 1 entry, got %d", len(entries))
				}
				e := entries[0]
				if e.Level != session.ConsoleLogLevelError {
					t.Errorf("Level = %s, want error", e.Level)
				}
				if e.Text != "Test error message" {
					t.Errorf("Text = %s, want 'Test error message'", e.Text)
				}
				if e.URL != "https://example.com/app.js" {
					t.Errorf("URL = %s, want 'https://example.com/app.js'", e.URL)
				}
				if e.Line != 42 {
					t.Errorf("Line = %d, want 42", e.Line)
				}
				if e.Column != 15 {
					t.Errorf("Column = %d, want 15", e.Column)
				}
			},
		},
		{
			name: "nil console buffer returns error",
			arguments: map[string]any{
				"sessionId": "sess-test",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return &session.BrowserSession{
					ID:          "sess-test",
					ConsoleLogs: nil, // nil buffer
				}, true
			},
			expectError:    true,
			expectedResult: "Console log buffer not initialized",
		},
		{
			name: "invalid level returns error",
			arguments: map[string]any{
				"sessionId": "sess-test",
				"level":     "invalid_level",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return createSessionWithLogs([]session.ConsoleLogEntry{}), true
			},
			expectError:    true,
			expectedResult: "Invalid level: invalid_level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}
			handler := GetConsoleLogsHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_console_logs",
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

// TestGetConsoleLogsHandler_AllLevels tests that all log levels work correctly.
func TestGetConsoleLogsHandler_AllLevels(t *testing.T) {
	levels := []session.ConsoleLogLevel{
		session.ConsoleLogLevelLog,
		session.ConsoleLogLevelInfo,
		session.ConsoleLogLevelWarn,
		session.ConsoleLogLevelError,
		session.ConsoleLogLevelDebug,
	}

	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			// Create a session with entries of all levels
			buf := session.NewConsoleLogBuffer(100)
			for _, l := range levels {
				buf.Add(session.ConsoleLogEntry{
					Timestamp: time.Now(),
					Level:     l,
					Text:      "message for " + string(l),
				})
			}

			mgr := &mockSessionManager{
				getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
					return &session.BrowserSession{
						ID:          "sess-test",
						ConsoleLogs: buf,
					}, true
				},
			}
			handler := GetConsoleLogsHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_console_logs",
					Arguments: map[string]any{
						"sessionId": "sess-test",
						"level":     string(level),
					},
				},
			}

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Handler error: %v", err)
			}

			if result.IsError {
				t.Errorf("Expected success, got error: %s", extractTextContent(result.Content))
				return
			}

			text := extractTextContent(result.Content)
			var entries []session.ConsoleLogEntry
			if err := json.Unmarshal([]byte(text), &entries); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if len(entries) != 1 {
				t.Errorf("Expected 1 entry for level %s, got %d", level, len(entries))
			}

			if len(entries) > 0 && entries[0].Level != level {
				t.Errorf("Expected level %s, got %s", level, entries[0].Level)
			}
		})
	}
}

// TestGetConsoleLogsHandler_MCPOwnership tests that the handler correctly validates session ownership.
func TestGetConsoleLogsHandler_MCPOwnership(t *testing.T) {
	var capturedMCPSessionID, capturedBrowserSessionID string

	mgr := &mockSessionManager{
		getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
			capturedMCPSessionID = mcpSessionID
			capturedBrowserSessionID = browserSessionID
			// Simulate ownership check - return false for unauthorized access
			return nil, false
		},
	}
	handler := GetConsoleLogsHandler(mgr)

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_console_logs",
			Arguments: map[string]any{
				"sessionId": "sess-123",
			},
		},
	}

	ctx := context.Background()
	result, _ := handler(ctx, req)

	// Verify both IDs were passed to manager
	if capturedMCPSessionID != "" {
		t.Errorf("Expected empty MCP session ID from context.Background(), got %q", capturedMCPSessionID)
	}
	if capturedBrowserSessionID != "sess-123" {
		t.Errorf("Expected browser session ID 'sess-123', got %q", capturedBrowserSessionID)
	}

	// Result should be error since session was not found
	if !result.IsError {
		t.Error("Expected error result when session not found")
	}

	text := extractTextContent(result.Content)
	if !strings.Contains(text, "Session not found") {
		t.Errorf("Expected 'Session not found' in error message, got %q", text)
	}
}
