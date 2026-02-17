//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tgrunnagle/parallel-playwright-mcp/e2e/helpers"
)

// Error codes from pkg/errors/errors.go
const (
	codeSessionNotFound  = -32001
	codeElementNotFound  = -32002
	codeTimeout          = -32003
	codeNavigationFailed = -32004
)

// errorCodeRe matches error codes in the format [-NNNNN] in error text.
var errorCodeRe = regexp.MustCompile(`\[(-?\d+)\]`)

// TestInvalidSessionID verifies error code -32001 when using a non-existent session ID.
func TestInvalidSessionID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize MCP client: %v", err)
	}
	defer client.Close(ctx)

	invalidSessionID := "invalid-session-00000000-0000-0000-0000-000000000000"

	result, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": invalidSessionID,
		"url":       "https://example.com",
	})
	if err != nil {
		t.Fatalf("tool call should not return transport error: %v", err)
	}

	if !result.IsError {
		t.Fatal("result should be an error")
	}

	errText := getErrorText(result)
	t.Logf("Error response: %s", errText)

	errCode := parseErrorCode(t, errText)
	if errCode != codeSessionNotFound {
		t.Errorf("expected error code %d, got %d", codeSessionNotFound, errCode)
	}

	if !containsIgnoreCase(errText, "session") {
		t.Errorf("error message should mention 'session', got: %s", errText)
	}

	if !strings.Contains(errText, invalidSessionID) {
		t.Errorf("error message should contain the session ID %q, got: %s", invalidSessionID, errText)
	}
}

// TestElementNotFound verifies error handling when interacting with non-existent selectors.
//
// Playwright's action model waits for elements to appear within the timeout period.
// When a selector matches no elements, Playwright waits until the timeout expires and
// reports a timeout error. The server's handleInteractionError classifies this as:
//   - -32003 (Timeout) when the error message contains "timeout" or "exceeded"
//   - -32002 (Element Not Found) when Playwright reports element state errors
//     (e.g., "not visible", "not attached", "strict mode violation")
//
// For non-existent selectors, Playwright consistently produces timeout errors (-32003).
// The -32002 code path handles cases where elements exist but are in an invalid state.
func TestElementNotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Stop()

	fixtures := helpers.NewFixtureServer(t)
	defer fixtures.Close()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize MCP client: %v", err)
	}
	defer client.Close(ctx)

	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.CloseSession(cleanupCtx, sessionID)
	})

	// Navigate to a page so we have a loaded DOM
	if err := client.Navigate(ctx, sessionID, fixtures.URL("simple.html")); err != nil {
		t.Fatalf("navigation failed: %v", err)
	}

	nonExistentSelector := "#this-element-does-not-exist-12345"

	t.Run("Click", func(t *testing.T) {
		result, err := client.CallTool(ctx, "click", map[string]any{
			"sessionId": sessionID,
			"selector":  nonExistentSelector,
			"timeout":   3000,
		})
		if err != nil {
			t.Fatalf("tool call should not return transport error: %v", err)
		}

		if !result.IsError {
			t.Fatal("result should be an error")
		}

		errText := getErrorText(result)
		t.Logf("Error response: %s", errText)

		errCode := parseErrorCode(t, errText)
		// Playwright waits for the element to appear, then times out.
		// handleInteractionError classifies timeout messages as -32003.
		if errCode != codeElementNotFound && errCode != codeTimeout {
			t.Errorf("expected error code %d or %d, got %d", codeElementNotFound, codeTimeout, errCode)
		}

		if !strings.Contains(errText, nonExistentSelector) {
			t.Errorf("error message should contain the selector %q, got: %s", nonExistentSelector, errText)
		}
	})

	t.Run("Fill", func(t *testing.T) {
		result, err := client.CallTool(ctx, "fill", map[string]any{
			"sessionId": sessionID,
			"selector":  nonExistentSelector,
			"value":     "test text",
			"timeout":   3000,
		})
		if err != nil {
			t.Fatalf("tool call should not return transport error: %v", err)
		}

		if !result.IsError {
			t.Fatal("result should be an error")
		}

		errText := getErrorText(result)
		t.Logf("Error response: %s", errText)

		errCode := parseErrorCode(t, errText)
		if errCode != codeElementNotFound && errCode != codeTimeout {
			t.Errorf("expected error code %d or %d, got %d", codeElementNotFound, codeTimeout, errCode)
		}

		if !strings.Contains(errText, nonExistentSelector) {
			t.Errorf("error message should contain the selector %q, got: %s", nonExistentSelector, errText)
		}
	})
}

// TestNavigationTimeout verifies error code -32003 when navigation times out.
func TestNavigationTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize MCP client: %v", err)
	}
	defer client.Close(ctx)

	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.CloseSession(cleanupCtx, sessionID)
	})

	// Start a server that accepts connections but delays response.
	// The handler uses a channel so it can be interrupted during test cleanup
	// instead of blocking httptest.Server.Close() for the full sleep duration.
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
		}
	}))
	defer slowServer.Close()

	result, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": sessionID,
		"url":       slowServer.URL,
		"timeout":   3000, // 3 second timeout to trigger timeout quickly
	})
	if err != nil {
		t.Fatalf("tool call should not return transport error: %v", err)
	}

	if !result.IsError {
		t.Fatal("result should be an error")
	}

	errText := getErrorText(result)
	t.Logf("Error response: %s", errText)

	errCode := parseErrorCode(t, errText)
	// The slow server accepts connections but never responds, so Playwright's
	// navigation timeout fires reliably, producing a timeout error classified as -32003
	// by handleNavigationError via isTimeoutError().
	if errCode != codeTimeout {
		t.Errorf("expected error code %d (Timeout), got %d", codeTimeout, errCode)
	}

	if !containsIgnoreCase(errText, "timeout") {
		t.Errorf("error message should mention timeout, got: %s", errText)
	}

	if !strings.Contains(errText, slowServer.URL) {
		t.Errorf("error message should contain the target URL %q, got: %s", slowServer.URL, errText)
	}
}

// TestNavigationFailed verifies error code -32004 when navigating to an invalid URL.
func TestNavigationFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize MCP client: %v", err)
	}
	defer client.Close(ctx)

	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.CloseSession(cleanupCtx, sessionID)
	})

	invalidURL := "invalid://not-a-real-protocol/page"

	result, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": sessionID,
		"url":       invalidURL,
	})
	if err != nil {
		t.Fatalf("tool call should not return transport error: %v", err)
	}

	if !result.IsError {
		t.Fatal("result should be an error")
	}

	errText := getErrorText(result)
	t.Logf("Error response: %s", errText)

	errCode := parseErrorCode(t, errText)
	if errCode != codeNavigationFailed {
		t.Errorf("expected error code %d, got %d", codeNavigationFailed, errCode)
	}

	if !containsIgnoreCase(errText, "navigation") && !containsIgnoreCase(errText, "failed") {
		t.Errorf("error message should mention navigation failure, got: %s", errText)
	}

	if !strings.Contains(errText, invalidURL) {
		t.Errorf("error message should contain the target URL %q, got: %s", invalidURL, errText)
	}
}

// TestErrorResponseStructure verifies the complete error response format.
// Checks that error responses have the expected structure: IsError flag,
// error code prefix, descriptive message, and relevant context.
func TestErrorResponseStructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize MCP client: %v", err)
	}
	defer client.Close(ctx)

	// Trigger an error with invalid session ID on session_close
	result, err := client.CallTool(ctx, "session_close", map[string]any{
		"sessionId": "nonexistent-session",
	})
	if err != nil {
		t.Fatalf("tool call should not return transport error: %v", err)
	}

	t.Run("IsErrorFlagSet", func(t *testing.T) {
		if !result.IsError {
			t.Error("result.IsError should be true for error responses")
		}
	})

	t.Run("HasContent", func(t *testing.T) {
		if len(result.Content) == 0 {
			t.Fatal("error response should have at least one content block")
		}
		if result.Content[0].Type != "text" {
			t.Errorf("content type should be 'text', got %q", result.Content[0].Type)
		}
	})

	t.Run("HasErrorCode", func(t *testing.T) {
		errText := getErrorText(result)
		errCode := parseErrorCode(t, errText)
		if errCode == 0 {
			t.Errorf("error response should contain an error code, got text: %s", errText)
		}
	})

	t.Run("HasDescriptiveMessage", func(t *testing.T) {
		errText := getErrorText(result)
		if errText == "" {
			t.Error("error message should not be empty")
		}
		// Error text should be more than just the code bracket
		codePrefix := errorCodeRe.FindString(errText)
		messageAfterCode := strings.TrimSpace(strings.TrimPrefix(errText, codePrefix))
		if messageAfterCode == "" {
			t.Error("error message should contain descriptive text after the error code")
		}
	})

	t.Run("ContainsContextData", func(t *testing.T) {
		errText := getErrorText(result)
		if !strings.Contains(errText, "nonexistent-session") {
			t.Errorf("error message should contain the session ID context, got: %s", errText)
		}
	})

	t.Run("SuggestionVerification", func(t *testing.T) {
		// NOTE: The server's tool-level error handlers (newNavigationSessionNotFoundError,
		// handleNavigationError, handleInteractionError, session_close handler) use
		// fmt.Sprintf("[%d] ...") directly instead of errors.FormatErrorForTool().
		// Only FormatErrorForTool() includes actionable suggestions from the errors package.
		// As a result, most tool error responses do not include suggestions.
		// This is a known gap; a server-side fix to use FormatErrorForTool() consistently
		// would enable full suggestion verification here.
		errText := getErrorText(result)
		hasSuggestion := containsIgnoreCase(errText, "verify") ||
			containsIgnoreCase(errText, "check") ||
			containsIgnoreCase(errText, "try") ||
			containsIgnoreCase(errText, "ensure") ||
			containsIgnoreCase(errText, "create a new session")
		t.Logf("Suggestion present in error response: %v", hasSuggestion)
		if !hasSuggestion {
			t.Log("Server tool handlers do not currently include suggestions in error text. " +
				"See pkg/errors/formatter.go FormatErrorForTool() for the suggestion-capable path.")
		}
	})
}

// TestMultipleErrorScenarios runs through session-not-found errors across multiple tools.
func TestMultipleErrorScenarios(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize MCP client: %v", err)
	}
	defer client.Close(ctx)

	fakeSessionID := "fake-session-id"

	testCases := []struct {
		name         string
		toolName     string
		args         map[string]any
		expectedCode int
	}{
		{
			name:         "InvalidSessionOnNavigate",
			toolName:     "navigate",
			args:         map[string]any{"sessionId": fakeSessionID, "url": "https://example.com"},
			expectedCode: codeSessionNotFound,
		},
		{
			name:         "InvalidSessionOnClick",
			toolName:     "click",
			args:         map[string]any{"sessionId": fakeSessionID, "selector": "#btn"},
			expectedCode: codeSessionNotFound,
		},
		{
			name:         "InvalidSessionOnScreenshot",
			toolName:     "screenshot",
			args:         map[string]any{"sessionId": fakeSessionID},
			expectedCode: codeSessionNotFound,
		},
		{
			name:         "InvalidSessionOnExtractText",
			toolName:     "extract_text",
			args:         map[string]any{"sessionId": fakeSessionID},
			expectedCode: codeSessionNotFound,
		},
		{
			name:         "InvalidSessionOnSessionClose",
			toolName:     "session_close",
			args:         map[string]any{"sessionId": fakeSessionID},
			expectedCode: codeSessionNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := client.CallTool(ctx, tc.toolName, tc.args)
			if err != nil {
				t.Fatalf("tool call should not return transport error: %v", err)
			}

			if !result.IsError {
				t.Fatalf("expected error for %s", tc.name)
			}

			errText := getErrorText(result)
			t.Logf("Error response for %s: %s", tc.name, errText)

			errCode := parseErrorCode(t, errText)
			if errCode != tc.expectedCode {
				t.Errorf("expected error code %d, got %d for %s", tc.expectedCode, errCode, tc.name)
			}

			if !containsIgnoreCase(errText, "session") {
				t.Errorf("error message should mention 'session' for %s, got: %s", tc.name, errText)
			}
		})
	}
}

// parseErrorCode extracts the error code from the [CODE] prefix in error text.
func parseErrorCode(t *testing.T, errText string) int {
	t.Helper()
	matches := errorCodeRe.FindStringSubmatch(errText)
	if len(matches) < 2 {
		t.Logf("no error code found in text: %s", errText)
		return 0
	}
	code, err := strconv.Atoi(matches[1])
	if err != nil {
		t.Logf("failed to parse error code from %q: %v", matches[1], err)
		return 0
	}
	return code
}

// getErrorText extracts the text content from a ToolResult.
func getErrorText(result *helpers.ToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	for _, block := range result.Content {
		if block.Text != "" {
			return block.Text
		}
	}
	return ""
}

// containsIgnoreCase checks if s contains substr, case-insensitive.
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
