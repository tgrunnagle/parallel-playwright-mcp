//go:build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/tgrunnagle/parallel-playwright-mcp/e2e/helpers"
)

// TestFullAutomationWorkflow validates the complete browser automation sequence:
// session_create -> navigate -> click -> type -> screenshot -> extract_text -> session_close
func TestFullAutomationWorkflow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Start test server
	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Stop()

	// Start fixture server
	fixtures := helpers.NewFixtureServer(t)
	defer fixtures.Close()

	// Create and initialize MCP client
	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize MCP client: %v", err)
	}
	defer client.Close(ctx)

	// Create browser session
	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create browser session: %v", err)
	}

	// Ensure session cleanup even if test fails partway through
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.CloseSession(cleanupCtx, sessionID)
	})

	// Navigate to test fixture
	formURL := fixtures.URL("form.html")
	result, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": sessionID,
		"url":       formURL,
		"waitUntil": "load",
	})
	if err != nil {
		t.Fatalf("navigate tool call failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("navigate returned error: %v", result.Content)
	}

	// Click on input field
	result, err = client.CallTool(ctx, "click", map[string]any{
		"sessionId": sessionID,
		"selector":  "#username",
	})
	if err != nil {
		t.Fatalf("click tool call failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("click returned error: %v", result.Content)
	}

	// Type text into the field
	testUsername := "e2e_test_user"
	result, err = client.CallTool(ctx, "type", map[string]any{
		"sessionId": sessionID,
		"selector":  "#username",
		"text":      testUsername,
	})
	if err != nil {
		t.Fatalf("type tool call failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("type returned error: %v", result.Content)
	}

	// Capture screenshot
	result, err = client.CallTool(ctx, "screenshot", map[string]any{
		"sessionId": sessionID,
	})
	if err != nil {
		t.Fatalf("screenshot tool call failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("screenshot returned error: %v", result.Content)
	}

	// Verify screenshot is valid base64 PNG
	screenshotData := extractImageData(result)
	if screenshotData == "" {
		t.Fatal("screenshot data should not be empty")
	}
	verifyPNGScreenshot(t, screenshotData)

	// Extract text from page
	result, err = client.CallTool(ctx, "extract_text", map[string]any{
		"sessionId": sessionID,
	})
	if err != nil {
		t.Fatalf("extract_text tool call failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("extract_text returned error: %v", result.Content)
	}

	extractedText := extractTextContent(result)
	if extractedText == "" {
		t.Fatal("extracted text should not be empty")
	}
	// The form.html page has title "Form Test Page" and labels like "Username"
	if !strings.Contains(strings.ToLower(extractedText), "form") {
		t.Errorf("extracted text should contain 'form', got: %.200s", extractedText)
	}

	// Close session
	if err := client.CloseSession(ctx, sessionID); err != nil {
		t.Fatalf("failed to close browser session: %v", err)
	}

	// Verify session is cleaned up
	result, err = client.CallTool(ctx, "session_list", nil)
	if err != nil {
		t.Fatalf("session_list tool call failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("session_list returned error: %v", result.Content)
	}

	sessionListText := extractTextContent(result)
	if strings.Contains(sessionListText, sessionID) {
		t.Errorf("closed session should not appear in session list")
	}
}

// TestFullAutomationWorkflowWithFullPageScreenshot tests the workflow with fullPage screenshot option.
func TestFullAutomationWorkflowWithFullPageScreenshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	fixtures := helpers.NewFixtureServer(t)
	defer fixtures.Close()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close(ctx)

	// Create session and navigate
	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.CloseSession(cleanupCtx, sessionID)
	})

	result, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": sessionID,
		"url":       fixtures.URL("form.html"),
		"waitUntil": "load",
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("navigate returned error: %v", result.Content)
	}

	// Take full page screenshot
	result, err = client.CallTool(ctx, "screenshot", map[string]any{
		"sessionId": sessionID,
		"fullPage":  true,
	})
	if err != nil {
		t.Fatalf("screenshot failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("screenshot returned error: %v", result.Content)
	}

	// Verify it's valid base64 PNG
	screenshotData := extractImageData(result)
	if screenshotData == "" {
		t.Fatal("full page screenshot data should not be empty")
	}
	verifyPNGScreenshot(t, screenshotData)
}

// TestWorkflowWithElementScreenshot tests taking a screenshot of a specific element.
func TestWorkflowWithElementScreenshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	fixtures := helpers.NewFixtureServer(t)
	defer fixtures.Close()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
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

	result, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": sessionID,
		"url":       fixtures.URL("form.html"),
		"waitUntil": "load",
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("navigate returned error: %v", result.Content)
	}

	// Take element-specific screenshot
	result, err = client.CallTool(ctx, "screenshot", map[string]any{
		"sessionId": sessionID,
		"selector":  "#username",
	})
	if err != nil {
		t.Fatalf("element screenshot failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("element screenshot returned error: %v", result.Content)
	}

	screenshotData := extractImageData(result)
	if screenshotData == "" {
		t.Fatal("element screenshot data should not be empty")
	}
	verifyPNGScreenshot(t, screenshotData)
}

// extractTextContent extracts text content from a ToolResult.
func extractTextContent(result *helpers.ToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	var texts []string
	for _, block := range result.Content {
		if block.Text != "" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// extractImageData extracts base64 image data from a ToolResult.
// Screenshot results use the Data field (image content blocks).
func extractImageData(result *helpers.ToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	for _, block := range result.Content {
		if block.Data != "" {
			return block.Data
		}
	}
	return ""
}

// verifyPNGScreenshot decodes and validates that the base64 data is a valid PNG image.
// Returns the decoded bytes on success.
func verifyPNGScreenshot(t *testing.T, base64Data string) []byte {
	t.Helper()

	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		t.Fatalf("screenshot should be valid base64: %v", err)
	}

	if len(decoded) < 8 {
		t.Fatalf("decoded screenshot too small: %d bytes", len(decoded))
	}

	// Verify PNG magic bytes (89 50 4E 47 0D 0A 1A 0A)
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range pngHeader {
		if decoded[i] != b {
			t.Fatalf("invalid PNG header byte at position %d: expected 0x%02X, got 0x%02X", i, b, decoded[i])
		}
	}

	return decoded
}
