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
