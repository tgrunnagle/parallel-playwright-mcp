//go:build integration

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// Integration tests require Playwright runtime to be installed.
// Run with: go test -tags=integration ./pkg/tools/...

// skipIfPlaywrightNotInstalled checks if Playwright is available and skips the test with
// helpful instructions if not.
func skipIfPlaywrightNotInstalled(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	errStr := err.Error()
	if strings.Contains(errStr, "please install the driver") ||
		strings.Contains(errStr, "executable file not found") ||
		strings.Contains(errStr, "Playwright") ||
		strings.Contains(errStr, "browser pool is not running") {
		t.Skipf("Playwright not installed or pool not started. Run 'task playwright:install' to install browsers.\nOriginal error: %v", err)
	}
}

func setupPoolAndManager(t *testing.T) (browser.BrowserPool, session.BrowserSessionManager) {
	t.Helper()
	pool := browser.NewBrowserPoolWithOptions(browser.PoolOptions{
		DefaultHeadless: true,
	})
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		skipIfPlaywrightNotInstalled(t, err)
		t.Fatalf("Failed to start pool: %v", err)
	}
	mgr := session.NewManager(pool)
	return pool, mgr
}

func TestSessionCreateToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	handler := SessionCreateHandler(mgr)
	ctx := context.Background()

	t.Run("creates session with default options", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}

		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.HasPrefix(text, "Created session: sess-") {
			t.Errorf("Expected session ID in response, got: %s", text)
		}

		// Extract session ID and verify it exists
		sessionID := strings.TrimPrefix(text, "Created session: ")
		sessions := mgr.ListSessions("")
		found := false
		for _, s := range sessions {
			if s.ID == sessionID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Session %s not found in manager", sessionID)
		}

		// Clean up
		mgr.CloseSession(ctx, "", sessionID)
	})

	t.Run("creates session with chromium", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_create",
				Arguments: map[string]any{
					"browserType": "chromium",
				},
			},
		}

		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(result))
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		sessionID := strings.TrimPrefix(text, "Created session: ")

		// Verify browser type
		sessions := mgr.ListSessions("")
		for _, s := range sessions {
			if s.ID == sessionID {
				if s.BrowserType != browser.BrowserChromium {
					t.Errorf("Expected chromium, got %s", s.BrowserType)
				}
				break
			}
		}

		// Clean up
		mgr.CloseSession(ctx, "", sessionID)
	})

	t.Run("creates session with custom viewport", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_create",
				Arguments: map[string]any{
					"viewport": map[string]any{
						"width":  float64(1920),
						"height": float64(1080),
					},
				},
			},
		}

		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(result))
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		sessionID := strings.TrimPrefix(text, "Created session: ")

		// Clean up
		mgr.CloseSession(ctx, "", sessionID)
	})
}

func TestSessionListToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	listHandler := SessionListHandler(mgr)
	ctx := context.Background()

	t.Run("lists empty sessions for new connection", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_list",
				Arguments: map[string]any{},
			},
		}

		result, err := listHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var response SessionListResponse
		if err := json.Unmarshal([]byte(text), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(response.Sessions) != 0 {
			t.Errorf("Expected empty sessions, got %d", len(response.Sessions))
		}
	})

	t.Run("lists created sessions", func(t *testing.T) {
		// Create a session
		createReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}

		createResult, err := createHandler(ctx, createReq)
		if err != nil {
			t.Fatalf("Create handler returned error: %v", err)
		}
		if createResult.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
			t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
		}

		sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
		defer mgr.CloseSession(ctx, "", sessionID)

		// List sessions
		listReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_list",
				Arguments: map[string]any{},
			},
		}

		listResult, err := listHandler(ctx, listReq)
		if err != nil {
			t.Fatalf("List handler returned error: %v", err)
		}

		if listResult.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(listResult.Content))
		}

		text := extractTextContent(listResult.Content)
		var response SessionListResponse
		if err := json.Unmarshal([]byte(text), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(response.Sessions) != 1 {
			t.Errorf("Expected 1 session, got %d", len(response.Sessions))
		}

		if response.Sessions[0].SessionID != sessionID {
			t.Errorf("Expected session ID %s, got %s", sessionID, response.Sessions[0].SessionID)
		}
	})
}

func TestSessionCloseToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	closeHandler := SessionCloseHandler(mgr)
	listHandler := SessionListHandler(mgr)
	ctx := context.Background()

	t.Run("closes session and removes from list", func(t *testing.T) {
		// Create a session
		createReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}

		createResult, err := createHandler(ctx, createReq)
		if err != nil {
			t.Fatalf("Create handler returned error: %v", err)
		}
		if createResult.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
			t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
		}

		sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")

		// Close the session
		closeReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		closeResult, err := closeHandler(ctx, closeReq)
		if err != nil {
			t.Fatalf("Close handler returned error: %v", err)
		}

		if closeResult.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(closeResult.Content))
		}

		text := extractTextContent(closeResult.Content)
		if !strings.Contains(text, sessionID) {
			t.Errorf("Expected session ID in close response, got: %s", text)
		}

		// Verify session is removed from list
		listReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_list",
				Arguments: map[string]any{},
			},
		}

		listResult, err := listHandler(ctx, listReq)
		if err != nil {
			t.Fatalf("List handler returned error: %v", err)
		}

		listText := extractTextContent(listResult.Content)
		var response SessionListResponse
		if err := json.Unmarshal([]byte(listText), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		for _, s := range response.Sessions {
			if s.SessionID == sessionID {
				t.Errorf("Session %s should have been removed", sessionID)
			}
		}
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		closeReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": "sess-non-existent",
				},
			},
		}

		closeResult, err := closeHandler(ctx, closeReq)
		if err != nil {
			t.Fatalf("Close handler returned error: %v", err)
		}

		if !closeResult.IsError {
			t.Error("Expected error for non-existent session")
		}

		text := extractTextContent(closeResult.Content)
		if !strings.Contains(text, "not found") {
			t.Errorf("Expected 'not found' in error message, got: %s", text)
		}
	})
}

func TestFullSessionLifecycleIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	listHandler := SessionListHandler(mgr)
	closeHandler := SessionCloseHandler(mgr)
	ctx := context.Background()

	t.Run("full create -> list -> close flow", func(t *testing.T) {
		// Step 1: List sessions (should be empty)
		listReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_list",
				Arguments: map[string]any{},
			},
		}
		listResult, _ := listHandler(ctx, listReq)
		text := extractTextContent(listResult.Content)
		var initialList SessionListResponse
		json.Unmarshal([]byte(text), &initialList)
		initialCount := len(initialList.Sessions)

		// Step 2: Create first session
		createReq1 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_create",
				Arguments: map[string]any{
					"browserType": "chromium",
				},
			},
		}
		createResult1, err := createHandler(ctx, createReq1)
		if err != nil || createResult1.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult1))
			t.Fatalf("Create first session failed: %v", extractTextContent(createResult1.Content))
		}
		sessionID1 := strings.TrimPrefix(extractTextContent(createResult1.Content), "Created session: ")

		// Step 3: Create second session
		createReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}
		createResult2, err := createHandler(ctx, createReq2)
		if err != nil || createResult2.IsError {
			t.Fatalf("Create second session failed: %v", extractTextContent(createResult2.Content))
		}
		sessionID2 := strings.TrimPrefix(extractTextContent(createResult2.Content), "Created session: ")

		// Step 4: List sessions (should have 2 new sessions)
		listResult2, _ := listHandler(ctx, listReq)
		text = extractTextContent(listResult2.Content)
		var afterCreate SessionListResponse
		json.Unmarshal([]byte(text), &afterCreate)
		if len(afterCreate.Sessions) != initialCount+2 {
			t.Errorf("Expected %d sessions after create, got %d", initialCount+2, len(afterCreate.Sessions))
		}

		// Step 5: Close first session
		closeReq1 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": sessionID1,
				},
			},
		}
		closeResult1, _ := closeHandler(ctx, closeReq1)
		if closeResult1.IsError {
			t.Errorf("Close first session failed: %s", extractTextContent(closeResult1.Content))
		}

		// Step 6: List sessions (should have 1 less)
		listResult3, _ := listHandler(ctx, listReq)
		text = extractTextContent(listResult3.Content)
		var afterClose1 SessionListResponse
		json.Unmarshal([]byte(text), &afterClose1)
		if len(afterClose1.Sessions) != initialCount+1 {
			t.Errorf("Expected %d sessions after first close, got %d", initialCount+1, len(afterClose1.Sessions))
		}

		// Verify first session is gone
		for _, s := range afterClose1.Sessions {
			if s.SessionID == sessionID1 {
				t.Error("First session should have been removed")
			}
		}

		// Verify second session still exists
		found := false
		for _, s := range afterClose1.Sessions {
			if s.SessionID == sessionID2 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Second session should still exist")
		}

		// Step 7: Close second session
		closeReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": sessionID2,
				},
			},
		}
		closeResult2, _ := closeHandler(ctx, closeReq2)
		if closeResult2.IsError {
			t.Errorf("Close second session failed: %s", extractTextContent(closeResult2.Content))
		}

		// Step 8: List sessions (should be back to initial)
		listResult4, _ := listHandler(ctx, listReq)
		text = extractTextContent(listResult4.Content)
		var finalList SessionListResponse
		json.Unmarshal([]byte(text), &finalList)
		if len(finalList.Sessions) != initialCount {
			t.Errorf("Expected %d sessions at end, got %d", initialCount, len(finalList.Sessions))
		}
	})
}

// extractErrorFromResult extracts an error from a tool result for skip checking.
func extractErrorFromResult(result *mcp.CallToolResult) error {
	if result == nil || !result.IsError {
		return nil
	}
	text := extractTextContent(result.Content)
	return &resultError{message: text}
}

type resultError struct {
	message string
}

func (e *resultError) Error() string {
	return e.message
}

// TestNavigateToolIntegration tests the navigate tool with real browser instances.
func TestNavigateToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	ctx := context.Background()

	// Create a session for all tests
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("navigates to valid URL", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "about:blank",
				},
			},
		}

		result, err := navigateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated to") {
			t.Errorf("Expected navigation success message, got: %s", text)
		}
	})

	t.Run("navigates with waitUntil option", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "about:blank",
					"waitUntil": "load",
				},
			},
		}

		result, err := navigateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"url":       "about:blank",
				},
			},
		}

		result, err := navigateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32001]") {
			t.Errorf("Expected session not found error code, got: %s", text)
		}
	})
}

// TestGoBackToolIntegration tests the go_back tool with real browser instances.
func TestGoBackToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	goBackHandler := GoBackHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("returns no history message on fresh session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_back",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := goBackHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "No previous page") {
			t.Errorf("Expected 'No previous page' message, got: %s", text)
		}
	})

	t.Run("navigates back after page navigation", func(t *testing.T) {
		// Navigate to first page
		navReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "about:blank",
				},
			},
		}
		navigateHandler(ctx, navReq)

		// Navigate to second page (using data URL for consistent testing)
		navReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "data:text/html,<h1>Second Page</h1>",
				},
			},
		}
		navigateHandler(ctx, navReq2)

		// Go back
		backReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_back",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := goBackHandler(ctx, backReq)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated back") {
			t.Errorf("Expected 'Navigated back' message, got: %s", text)
		}
	})
}

// TestGoForwardToolIntegration tests the go_forward tool with real browser instances.
func TestGoForwardToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	goBackHandler := GoBackHandler(mgr)
	goForwardHandler := GoForwardHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("returns no history message on fresh session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_forward",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := goForwardHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "No forward page") {
			t.Errorf("Expected 'No forward page' message, got: %s", text)
		}
	})

	t.Run("navigates forward after go_back", func(t *testing.T) {
		// Navigate to first page
		navReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "data:text/html,<h1>First Page</h1>",
				},
			},
		}
		navigateHandler(ctx, navReq)

		// Navigate to second page
		navReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "data:text/html,<h1>Second Page</h1>",
				},
			},
		}
		navigateHandler(ctx, navReq2)

		// Go back
		backReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_back",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}
		goBackHandler(ctx, backReq)

		// Go forward
		forwardReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_forward",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := goForwardHandler(ctx, forwardReq)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated forward") {
			t.Errorf("Expected 'Navigated forward' message, got: %s", text)
		}
	})
}

// TestReloadToolIntegration tests the reload tool with real browser instances.
func TestReloadToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	reloadHandler := ReloadHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page first
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "about:blank",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("reloads current page", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "reload",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := reloadHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Reloaded page") {
			t.Errorf("Expected 'Reloaded page' message, got: %s", text)
		}
	})

	t.Run("reloads with waitUntil option", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "reload",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"waitUntil": "domcontentloaded",
				},
			},
		}

		result, err := reloadHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}
	})
}

// TestNavigationFlowIntegration tests a complete navigation workflow.
func TestNavigationFlowIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	goBackHandler := GoBackHandler(mgr)
	goForwardHandler := GoForwardHandler(mgr)
	reloadHandler := ReloadHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("full navigation flow: navigate -> navigate -> back -> forward -> reload", func(t *testing.T) {
		// Step 1: Navigate to first page
		navReq1 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "data:text/html,<h1>Page 1</h1>",
				},
			},
		}
		result, _ := navigateHandler(ctx, navReq1)
		if result.IsError {
			t.Fatalf("Navigate to page 1 failed: %s", extractTextContent(result.Content))
		}

		// Step 2: Navigate to second page
		navReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "data:text/html,<h1>Page 2</h1>",
				},
			},
		}
		result, _ = navigateHandler(ctx, navReq2)
		if result.IsError {
			t.Fatalf("Navigate to page 2 failed: %s", extractTextContent(result.Content))
		}

		// Step 3: Navigate to third page
		navReq3 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "data:text/html,<h1>Page 3</h1>",
				},
			},
		}
		result, _ = navigateHandler(ctx, navReq3)
		if result.IsError {
			t.Fatalf("Navigate to page 3 failed: %s", extractTextContent(result.Content))
		}

		// Step 4: Go back (should be at page 2)
		backReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_back",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}
		result, _ = goBackHandler(ctx, backReq)
		if result.IsError {
			t.Fatalf("Go back failed: %s", extractTextContent(result.Content))
		}
		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated back") {
			t.Errorf("Expected back navigation message, got: %s", text)
		}

		// Step 5: Go forward (should be at page 3)
		forwardReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_forward",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}
		result, _ = goForwardHandler(ctx, forwardReq)
		if result.IsError {
			t.Fatalf("Go forward failed: %s", extractTextContent(result.Content))
		}
		text = extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated forward") {
			t.Errorf("Expected forward navigation message, got: %s", text)
		}

		// Step 6: Reload current page
		reloadReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "reload",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}
		result, _ = reloadHandler(ctx, reloadReq)
		if result.IsError {
			t.Fatalf("Reload failed: %s", extractTextContent(result.Content))
		}
		text = extractTextContent(result.Content)
		if !strings.Contains(text, "Reloaded page") {
			t.Errorf("Expected reload message, got: %s", text)
		}
	})
}

// TestScreenshotToolIntegration tests the screenshot tool with real browser instances.
func TestScreenshotToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	screenshotHandler := ScreenshotHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<h1>Screenshot Test</h1><p>Test content</p>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("captures page screenshot", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "screenshot",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := screenshotHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		// Verify we got image content
		if len(result.Content) == 0 {
			t.Error("Expected content in result")
		}
	})

	t.Run("captures fullPage screenshot", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "screenshot",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"fullPage":  true,
				},
			},
		}

		result, err := screenshotHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}
	})
}

// TestExtractTextToolIntegration tests the extract_text tool with real browser instances.
func TestExtractTextToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	extractTextHandler := ExtractTextHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page with structured content
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><h1>Title</h1><p>Paragraph text</p><div><span>Nested content</span></div></body></html>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("extracts text from body", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "extract_text",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := extractTextHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Title") {
			t.Errorf("Expected 'Title' in response, got: %s", text)
		}
		if !strings.Contains(text, "Paragraph text") {
			t.Errorf("Expected 'Paragraph text' in response, got: %s", text)
		}
	})

	t.Run("extracts text from specific selector", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "extract_text",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "h1",
				},
			},
		}

		result, err := extractTextHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Title") {
			t.Errorf("Expected 'Title' in response, got: %s", text)
		}
	})

	t.Run("returns error for non-matching selector", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "extract_text",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#non-existent-element",
				},
			},
		}

		result, err := extractTextHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for non-matching selector")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32002]") {
			t.Errorf("Expected element not found error code, got: %s", text)
		}
	})
}

// TestGetHTMLToolIntegration tests the get_html tool with real browser instances.
func TestGetHTMLToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	getHTMLHandler := GetHTMLHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><div id=\"content\"><p>Test paragraph</p></div></body></html>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("gets full page HTML", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_html",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := getHTMLHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Test paragraph") {
			t.Errorf("Expected 'Test paragraph' in HTML, got: %s", text)
		}
	})

	t.Run("gets element innerHTML", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_html",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#content",
				},
			},
		}

		result, err := getHTMLHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "<p>Test paragraph</p>") {
			t.Errorf("Expected '<p>Test paragraph</p>' in innerHTML, got: %s", text)
		}
	})

	t.Run("gets element outerHTML", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_html",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#content",
					"outer":     true,
				},
			},
		}

		result, err := getHTMLHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "id=\"content\"") {
			t.Errorf("Expected 'id=\"content\"' in outerHTML, got: %s", text)
		}
	})
}

// TestEvaluateToolIntegration tests the evaluate tool with real browser instances.
func TestEvaluateToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	evaluateHandler := EvaluateHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><h1 id=\"title\">Hello World</h1></body></html>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("evaluates simple expression", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "evaluate",
				Arguments: map[string]any{
					"sessionId":  sessionID,
					"expression": "1 + 2",
				},
			},
		}

		result, err := evaluateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if text != "3" {
			t.Errorf("Expected '3', got: %s", text)
		}
	})

	t.Run("evaluates DOM query", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "evaluate",
				Arguments: map[string]any{
					"sessionId":  sessionID,
					"expression": "document.getElementById('title').textContent",
				},
			},
		}

		result, err := evaluateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Hello World") {
			t.Errorf("Expected 'Hello World' in response, got: %s", text)
		}
	})

	t.Run("returns object as JSON", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "evaluate",
				Arguments: map[string]any{
					"sessionId":  sessionID,
					"expression": "({ name: 'test', value: 42 })",
				},
			},
		}

		result, err := evaluateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(text), &obj); err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}
		if obj["name"] != "test" || obj["value"] != float64(42) {
			t.Errorf("Unexpected object values: %v", obj)
		}
	})
}

// TestQuerySelectorToolIntegration tests the query_selector tool with real browser instances.
func TestQuerySelectorToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	querySelectorHandler := QuerySelectorHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page with multiple elements
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><button id=\"btn1\" class=\"primary\">Submit</button><button id=\"btn2\" class=\"secondary\">Cancel</button></body></html>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("queries single element", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "query_selector",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#btn1",
				},
			},
		}

		result, err := querySelectorHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var info ElementInfo
		if err := json.Unmarshal([]byte(text), &info); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if info.Tag != "button" {
			t.Errorf("Expected tag 'button', got '%s'", info.Tag)
		}
		if info.Attributes["id"] != "btn1" {
			t.Errorf("Expected id 'btn1', got '%s'", info.Attributes["id"])
		}
	})

	t.Run("queries all matching elements", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "query_selector",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "button",
					"all":       true,
				},
			},
		}

		result, err := querySelectorHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var infos []ElementInfo
		if err := json.Unmarshal([]byte(text), &infos); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(infos) != 2 {
			t.Errorf("Expected 2 buttons, got %d", len(infos))
		}
	})

	t.Run("returns error for non-matching selector", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "query_selector",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#non-existent",
				},
			},
		}

		result, err := querySelectorHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for non-matching selector")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32002]") {
			t.Errorf("Expected element not found error code, got: %s", text)
		}
	})
}

// TestGetAccessibilityTreeToolIntegration tests the get_accessibility_tree tool with real browser instances.
func TestGetAccessibilityTreeToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	accessibilityHandler := GetAccessibilityTreeHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page with accessible elements
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><nav><a href=\"#\">Home</a></nav><main><h1>Title</h1><button aria-label=\"Submit Form\">Submit</button></main></body></html>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("gets accessibility tree", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_accessibility_tree",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := accessibilityHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)

		// Verify it returns a valid JSON structure
		var tree map[string]interface{}
		if err := json.Unmarshal([]byte(text), &tree); err != nil {
			t.Fatalf("Failed to parse accessibility tree: %v", err)
		}

		// Verify role is present
		if tree["role"] == nil {
			t.Error("Expected 'role' in accessibility tree")
		}

		// Check that key elements are captured
		if !strings.Contains(text, "button") {
			t.Error("Expected 'button' role in accessibility tree")
		}
		if !strings.Contains(text, "Submit Form") {
			t.Error("Expected 'Submit Form' aria-label in accessibility tree")
		}
	})
}

// TestGetConsoleLogsToolIntegration tests the get_console_logs tool with real browser instances.
func TestGetConsoleLogsToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	evaluateHandler := EvaluateHandler(mgr)
	consoleLogsHandler := GetConsoleLogsHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "about:blank",
			},
		},
	}
	navigateHandler(ctx, navReq)

	// Generate console logs using evaluate
	evalReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "evaluate",
			Arguments: map[string]any{
				"sessionId":  sessionID,
				"expression": "console.log('test log'); console.warn('test warning'); console.error('test error'); true",
			},
		},
	}
	evaluateHandler(ctx, evalReq)

	t.Run("retrieves console logs", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_console_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := consoleLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []session.ConsoleLogEntry
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse console logs: %v", err)
		}

		// Verify we captured logs
		if len(logs) < 3 {
			t.Errorf("Expected at least 3 log entries, got %d", len(logs))
		}
	})

	t.Run("filters by level", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_console_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"level":     "error",
				},
			},
		}

		result, err := consoleLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []session.ConsoleLogEntry
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse console logs: %v", err)
		}

		// All returned logs should be error level
		for _, log := range logs {
			if log.Level != session.ConsoleLogLevelError {
				t.Errorf("Expected error level, got %s", log.Level)
			}
		}
	})
}

// TestClickToolIntegration tests the click tool with real browser instances.
func TestClickToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	clickHandler := ClickHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with a clickable button
	testPage := `data:text/html,<html><body><button id="btn" onclick="this.textContent='clicked'">Click me</button></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("clicks element successfully", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "click",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#btn",
				},
			},
		}

		result, err := clickHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Clicked element") {
			t.Errorf("Expected click success message, got: %s", text)
		}
	})

	t.Run("returns error for non-existent element", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "click",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#non-existent",
					"timeout":   float64(1000),
				},
			},
		}

		result, err := clickHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for non-existent element")
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "click",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#btn",
				},
			},
		}

		result, err := clickHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32001]") {
			t.Errorf("Expected session not found error code, got: %s", text)
		}
	})
}

// TestTypeToolIntegration tests the type tool with real browser instances.
func TestTypeToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	typeHandler := TypeHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with an input field
	testPage := `data:text/html,<html><body><input id="input" type="text" /></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("types text into input", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "type",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#input",
					"text":      "Hello World",
				},
			},
		}

		result, err := typeHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Typed text into element") {
			t.Errorf("Expected type success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "type",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#input",
					"text":      "test",
				},
			},
		}

		result, err := typeHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}

// TestFillToolIntegration tests the fill tool with real browser instances.
func TestFillToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	fillHandler := FillHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with an input field
	testPage := `data:text/html,<html><body><input id="input" type="text" value="existing" /></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("fills input with text", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "fill",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#input",
					"value":     "New Value",
				},
			},
		}

		result, err := fillHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Filled element") {
			t.Errorf("Expected fill success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "fill",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#input",
					"value":     "test",
				},
			},
		}

		result, err := fillHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}

// TestSelectOptionToolIntegration tests the select_option tool with real browser instances.
func TestSelectOptionToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	selectHandler := SelectOptionHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with a select dropdown
	testPage := `data:text/html,<html><body><select id="select"><option value="a">Option A</option><option value="b">Option B</option><option value="c">Option C</option></select></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("selects option by value", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "select_option",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#select",
					"values":    []any{"b"},
				},
			},
		}

		result, err := selectHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Selected option") {
			t.Errorf("Expected select success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "select_option",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#select",
					"values":    []any{"a"},
				},
			},
		}

		result, err := selectHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}

// TestHoverToolIntegration tests the hover tool with real browser instances.
func TestHoverToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	hoverHandler := HoverHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with a hoverable element
	testPage := `data:text/html,<html><body><div id="hover-target" style="width:100px;height:100px;background:blue;">Hover me</div></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("hovers over element successfully", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "hover",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#hover-target",
				},
			},
		}

		result, err := hoverHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Hovered over element") {
			t.Errorf("Expected hover success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "hover",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#hover-target",
				},
			},
		}

		result, err := hoverHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}

// TestPressKeyToolIntegration tests the press_key tool with real browser instances.
func TestPressKeyToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr)
	navigateHandler := NavigateHandler(mgr)
	pressKeyHandler := PressKeyHandler(mgr)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with a text area
	testPage := `data:text/html,<html><body><textarea id="textarea"></textarea></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("presses key successfully", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "press_key",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"key":       "Tab",
				},
			},
		}

		result, err := pressKeyHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Pressed key") {
			t.Errorf("Expected press key success message, got: %s", text)
		}
	})

	t.Run("presses key with modifiers", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "press_key",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"key":       "a",
					"modifiers": []any{"Control"},
				},
			},
		}

		result, err := pressKeyHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Pressed key") {
			t.Errorf("Expected press key success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "press_key",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"key":       "Enter",
				},
			},
		}

		result, err := pressKeyHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}
