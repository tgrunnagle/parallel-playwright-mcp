//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tgrunnagle/parallel-playwright-mcp/e2e/helpers"
)

// TestParallelSessionsWithDifferentBrowsers validates that multiple browser sessions
// can run concurrently across Chromium, Firefox, and WebKit while maintaining isolation.
func TestParallelSessionsWithDifferentBrowsers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Start test server
	server := helpers.NewTestServer(t)
	require.NoError(t, server.Start(ctx), "failed to start test server")
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Logf("warning: failed to stop test server: %v", err)
		}
	})

	// Start fixture server
	fixtures := helpers.NewFixtureServer(t)
	t.Cleanup(func() { fixtures.Close() })

	// Create and initialize MCP client
	client := helpers.NewMCPClient(server.MCPURL())
	_, err := client.Initialize(ctx)
	require.NoError(t, err, "failed to initialize MCP client")
	t.Cleanup(func() {
		if err := client.Close(ctx); err != nil {
			t.Logf("warning: failed to close MCP client: %v", err)
		}
	})

	// Define browser types to test
	browserTypes := []string{"chromium", "firefox", "webkit"}
	sessions := make(map[string]string) // browserType -> sessionID

	// Step 1: Create sessions for all browser types
	t.Log("Creating browser sessions for all browser types...")
	for _, browserType := range browserTypes {
		sessionID, err := client.CreateSession(ctx, browserType, true)
		require.NoError(t, err, "failed to create %s session", browserType)
		require.NotEmpty(t, sessionID, "%s session ID should not be empty", browserType)
		sessions[browserType] = sessionID
		t.Logf("Created %s session: %s", browserType, sessionID)
	}

	// Register cleanup for all sessions
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		for browserType, sessionID := range sessions {
			if err := client.CloseSession(cleanupCtx, sessionID); err != nil {
				t.Logf("warning: failed to close %s session %s: %v", browserType, sessionID, err)
			}
		}
	})

	// Step 1b: Verify each session reports the correct browser type
	t.Log("Verifying browser types for all sessions...")
	listResult, err := client.CallTool(ctx, "session_list", nil)
	require.NoError(t, err, "session_list failed")
	require.False(t, listResult.IsError, "session_list returned error")
	sessionListJSON := extractTextContent(listResult)

	var sessionList struct {
		Sessions []struct {
			SessionID   string `json:"sessionId"`
			BrowserType string `json:"browserType"`
		} `json:"sessions"`
	}
	require.NoError(t, json.Unmarshal([]byte(sessionListJSON), &sessionList),
		"failed to parse session_list JSON")

	// Build a lookup from sessionID -> browserType from the server response
	reportedBrowserTypes := make(map[string]string)
	for _, s := range sessionList.Sessions {
		reportedBrowserTypes[s.SessionID] = s.BrowserType
	}

	for expectedBrowser, sessionID := range sessions {
		reportedBrowser, ok := reportedBrowserTypes[sessionID]
		require.True(t, ok, "session %s not found in session_list", sessionID)
		assert.Equal(t, expectedBrowser, reportedBrowser,
			"session %s should report browser type %s", sessionID, expectedBrowser)
		t.Logf("Verified %s session %s reports correct browser type", expectedBrowser, sessionID)
	}

	// Step 2: Run parallel workflows on all sessions
	t.Log("Running parallel workflows on all browser sessions...")
	var wg sync.WaitGroup
	errCh := make(chan error, len(browserTypes))

	for browserType, sessionID := range sessions {
		wg.Add(1)
		go func(bt, sid string) {
			defer wg.Done()
			if err := runBrowserWorkflow(ctx, client, fixtures, bt, sid); err != nil {
				errCh <- fmt.Errorf("%s session %s: %w", bt, sid, err)
			}
		}(browserType, sessionID)
	}

	// Wait for all workflows to complete
	wg.Wait()
	close(errCh)

	// Collect and report all errors
	var workflowErrors []error
	for err := range errCh {
		workflowErrors = append(workflowErrors, err)
		t.Errorf("parallel workflow failed: %v", err)
	}

	// Step 3: Verify session isolation (only if workflows succeeded)
	// Use evaluate to read input values because extract_text returns DOM text content
	// which does not include form input values.
	if len(workflowErrors) == 0 {
		t.Log("Verifying session isolation...")
		for browserType, sessionID := range sessions {
			result, err := client.CallTool(ctx, "evaluate", map[string]any{
				"sessionId":  sessionID,
				"expression": "document.querySelector('#username').value",
			})
			require.NoError(t, err, "failed to evaluate in %s session", browserType)
			require.False(t, result.IsError, "evaluate returned error for %s session", browserType)

			inputValue := extractTextContent(result)
			expectedText := fmt.Sprintf("parallel_test_%s", browserType)
			assert.Contains(t, inputValue, expectedText,
				"%s session should contain its unique typed text in input value", browserType)
			t.Logf("%s session contains expected text", browserType)
		}
	}

	// Step 4: Close all sessions explicitly
	t.Log("Closing all browser sessions...")
	closedIDs := make([]string, 0, len(sessions))
	for browserType, sessionID := range sessions {
		err := client.CloseSession(ctx, sessionID)
		require.NoError(t, err, "failed to close %s session", browserType)
		closedIDs = append(closedIDs, sessionID)
		t.Logf("Closed %s session: %s", browserType, sessionID)
	}

	// Clear sessions map to prevent double-close in cleanup
	for k := range sessions {
		delete(sessions, k)
	}

	// Step 5: Verify all sessions are cleaned up
	t.Log("Verifying session cleanup...")
	result, err := client.CallTool(ctx, "session_list", nil)
	require.NoError(t, err, "session_list tool call failed")
	require.False(t, result.IsError, "session_list returned error")
	sessionListText := extractTextContent(result)
	for _, sid := range closedIDs {
		assert.NotContains(t, sessionListText, sid,
			"closed session should not appear in session list")
	}

	if len(workflowErrors) == 0 {
		t.Log("Parallel sessions with different browsers test completed successfully!")
	}
}

// runBrowserWorkflow executes a standard automation workflow on a single browser session.
// It navigates to a page, interacts with form elements, and types browser-specific text.
func runBrowserWorkflow(ctx context.Context, client *helpers.MCPClient, fixtures *helpers.FixtureServer, browserType, sessionID string) error {
	// Navigate to test fixture
	formURL := fixtures.URL("form.html")
	result, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": sessionID,
		"url":       formURL,
		"waitUntil": "load",
	})
	if err != nil {
		return fmt.Errorf("navigate failed: %w", err)
	}
	if result.IsError {
		return fmt.Errorf("navigate returned error: %s", extractTextContent(result))
	}

	// Click on username input field
	result, err = client.CallTool(ctx, "click", map[string]any{
		"sessionId": sessionID,
		"selector":  "#username",
	})
	if err != nil {
		return fmt.Errorf("click failed: %w", err)
	}
	if result.IsError {
		return fmt.Errorf("click returned error: %s", extractTextContent(result))
	}

	// Type browser-specific text to verify isolation
	uniqueText := fmt.Sprintf("parallel_test_%s", browserType)
	result, err = client.CallTool(ctx, "type", map[string]any{
		"sessionId": sessionID,
		"selector":  "#username",
		"text":      uniqueText,
	})
	if err != nil {
		return fmt.Errorf("type failed: %w", err)
	}
	if result.IsError {
		return fmt.Errorf("type returned error: %s", extractTextContent(result))
	}

	// Extract text to verify the operation succeeded
	result, err = client.CallTool(ctx, "extract_text", map[string]any{
		"sessionId": sessionID,
	})
	if err != nil {
		return fmt.Errorf("extract_text failed: %w", err)
	}
	if result.IsError {
		return fmt.Errorf("extract_text returned error: %s", extractTextContent(result))
	}

	return nil
}

// TestParallelSessionsIsolation verifies that sessions don't interfere with each other
// by checking that form input values are isolated between sessions.
func TestParallelSessionsIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	server := helpers.NewTestServer(t)
	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() { server.Stop() })

	fixtures := helpers.NewFixtureServer(t)
	t.Cleanup(func() { fixtures.Close() })

	client := helpers.NewMCPClient(server.MCPURL())
	_, err := client.Initialize(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close(ctx) })

	// Create two Chromium sessions to test same-browser isolation
	session1, err := client.CreateSession(ctx, "chromium", true)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.CloseSession(cleanupCtx, session1)
	})

	session2, err := client.CreateSession(ctx, "chromium", true)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.CloseSession(cleanupCtx, session2)
	})

	// Navigate both sessions to the same page
	formURL := fixtures.URL("form.html")

	require.NoError(t, client.Navigate(ctx, session1, formURL))
	require.NoError(t, client.Navigate(ctx, session2, formURL))

	// Fill different values in each session
	result, err := client.CallTool(ctx, "fill", map[string]any{
		"sessionId": session1,
		"selector":  "#username",
		"value":     "user_session_1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "fill session 1 failed: %s", extractTextContent(result))

	result, err = client.CallTool(ctx, "fill", map[string]any{
		"sessionId": session2,
		"selector":  "#username",
		"value":     "user_session_2",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "fill session 2 failed: %s", extractTextContent(result))

	// Verify each session has its own value using evaluate
	result1, err := client.CallTool(ctx, "evaluate", map[string]any{
		"sessionId":  session1,
		"expression": "document.getElementById('username').value",
	})
	require.NoError(t, err)
	require.False(t, result1.IsError)
	value1 := extractTextContent(result1)
	assert.Contains(t, value1, "user_session_1", "session 1 should have its own value")
	assert.NotContains(t, value1, "user_session_2", "session 1 should not have session 2's value")

	result2, err := client.CallTool(ctx, "evaluate", map[string]any{
		"sessionId":  session2,
		"expression": "document.getElementById('username').value",
	})
	require.NoError(t, err)
	require.False(t, result2.IsError)
	value2 := extractTextContent(result2)
	assert.Contains(t, value2, "user_session_2", "session 2 should have its own value")
	assert.NotContains(t, value2, "user_session_1", "session 2 should not have session 1's value")

	t.Log("Session isolation verified: each session maintains independent state")
}

// TestParallelSessionsConcurrentCreation tests creating multiple sessions concurrently.
func TestParallelSessionsConcurrentCreation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	server := helpers.NewTestServer(t)
	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() { server.Stop() })

	client := helpers.NewMCPClient(server.MCPURL())
	_, err := client.Initialize(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close(ctx) })

	// Create 5 sessions concurrently (mix of browser types)
	browserTypes := []string{"chromium", "firefox", "webkit", "chromium", "firefox"}

	type sessionResult struct {
		browserType string
		sessionID   string
		err         error
	}

	var wg sync.WaitGroup
	resultCh := make(chan sessionResult, len(browserTypes))

	for _, bt := range browserTypes {
		wg.Add(1)
		go func(browserType string) {
			defer wg.Done()
			sessionID, err := client.CreateSession(ctx, browserType, true)
			resultCh <- sessionResult{browserType, sessionID, err}
		}(bt)
	}

	wg.Wait()
	close(resultCh)

	// Collect results
	var createdSessions []string
	for res := range resultCh {
		if res.err != nil {
			t.Errorf("failed to create %s session: %v", res.browserType, res.err)
			continue
		}
		require.NotEmpty(t, res.sessionID, "%s session ID should not be empty", res.browserType)
		createdSessions = append(createdSessions, res.sessionID)
		t.Logf("Concurrently created %s session: %s", res.browserType, res.sessionID)
	}

	// Cleanup all created sessions
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		for _, sessionID := range createdSessions {
			client.CloseSession(cleanupCtx, sessionID)
		}
	})

	// Verify all sessions appear in session_list
	result, err := client.CallTool(ctx, "session_list", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)

	sessionListText := extractTextContent(result)
	for _, sessionID := range createdSessions {
		assert.Contains(t, sessionListText, sessionID,
			"created session should appear in session list")
	}

	t.Logf("Successfully created %d sessions concurrently", len(createdSessions))
}

// TestParallelSessionsResourceCleanup verifies that closing sessions releases resources properly.
func TestParallelSessionsResourceCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	server := helpers.NewTestServer(t)
	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() { server.Stop() })

	client := helpers.NewMCPClient(server.MCPURL())
	_, err := client.Initialize(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close(ctx) })

	// Create sessions
	browserTypes := []string{"chromium", "firefox", "webkit"}
	sessions := make([]string, 0, len(browserTypes))

	for _, bt := range browserTypes {
		sessionID, err := client.CreateSession(ctx, bt, true)
		require.NoError(t, err)
		sessions = append(sessions, sessionID)
	}

	// Verify all sessions are listed
	result, err := client.CallTool(ctx, "session_list", nil)
	require.NoError(t, err)
	sessionListBefore := extractTextContent(result)
	for _, sessionID := range sessions {
		require.Contains(t, sessionListBefore, sessionID,
			"session should be in list before close")
	}

	// Close all sessions in parallel
	var wg sync.WaitGroup
	closeErrors := make(chan error, len(sessions))

	for _, sessionID := range sessions {
		wg.Add(1)
		go func(sid string) {
			defer wg.Done()
			if err := client.CloseSession(ctx, sid); err != nil {
				closeErrors <- fmt.Errorf("failed to close session %s: %w", sid, err)
			}
		}(sessionID)
	}

	wg.Wait()
	close(closeErrors)

	// Check for close errors
	for err := range closeErrors {
		t.Errorf("session close error: %v", err)
	}

	// Verify all sessions are removed from the list
	result, err = client.CallTool(ctx, "session_list", nil)
	require.NoError(t, err)
	sessionListAfter := extractTextContent(result)

	for _, sessionID := range sessions {
		assert.NotContains(t, sessionListAfter, sessionID,
			"closed session should not appear in list")
	}

	t.Log("Resource cleanup verified: all sessions removed after parallel close")
}

// TestParallelWorkflowsPartialFailure verifies that if one session's workflow fails,
// other sessions continue unaffected.
func TestParallelWorkflowsPartialFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	server := helpers.NewTestServer(t)
	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() { server.Stop() })

	fixtures := helpers.NewFixtureServer(t)
	t.Cleanup(func() { fixtures.Close() })

	client := helpers.NewMCPClient(server.MCPURL())
	_, err := client.Initialize(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close(ctx) })

	// Create two sessions
	goodSession, err := client.CreateSession(ctx, "chromium", true)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.CloseSession(cleanupCtx, goodSession)
	})

	otherSession, err := client.CreateSession(ctx, "chromium", true)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.CloseSession(cleanupCtx, otherSession)
	})

	// Navigate good session to fixture
	require.NoError(t, client.Navigate(ctx, goodSession, fixtures.URL("form.html")))

	// Navigate other session to fixture too
	require.NoError(t, client.Navigate(ctx, otherSession, fixtures.URL("form.html")))

	// Run parallel operations: good session fills form, other session clicks nonexistent element
	var wg sync.WaitGroup
	goodErrCh := make(chan error, 1)
	badFailed := make(chan bool, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		// This should succeed
		result, err := client.CallTool(ctx, "fill", map[string]any{
			"sessionId": goodSession,
			"selector":  "#username",
			"value":     "success_test",
		})
		if err != nil {
			goodErrCh <- fmt.Errorf("good session fill error: %w", err)
			return
		}
		if result.IsError {
			goodErrCh <- fmt.Errorf("good session fill returned error: %s", extractTextContent(result))
		}
	}()

	go func() {
		defer wg.Done()
		// This should fail - clicking a nonexistent element
		result, err := client.CallTool(ctx, "click", map[string]any{
			"sessionId": otherSession,
			"selector":  "#nonexistent-element-that-does-not-exist",
		})
		if err != nil {
			t.Logf("Expected failure on other session (transport error): %v", err)
			badFailed <- true
		} else if result.IsError {
			t.Logf("Expected failure on other session (tool error): %s", extractTextContent(result))
			badFailed <- true
		} else {
			badFailed <- false
		}
	}()

	wg.Wait()
	close(goodErrCh)
	close(badFailed)

	// Verify good session was unaffected by the failure
	for err := range goodErrCh {
		t.Fatalf("good session should have succeeded: %v", err)
	}

	// Verify the bad session actually failed
	if failed := <-badFailed; !failed {
		t.Error("expected click on nonexistent element to fail, but it succeeded")
	}

	// Verify good session still works and has correct value
	result, err := client.CallTool(ctx, "evaluate", map[string]any{
		"sessionId":  goodSession,
		"expression": "document.getElementById('username').value",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	value := extractTextContent(result)
	assert.Contains(t, value, "success_test",
		"good session should retain its value despite other session's failure")

	t.Log("Partial failure test passed: good session unaffected by other session's error")
}

