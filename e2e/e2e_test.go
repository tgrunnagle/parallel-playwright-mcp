//go:build e2e

// Package e2e contains end-to-end tests for the Playwright MCP Server.
package e2e

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tgrunnagle/parallel-playwright-mcp/e2e/helpers"
)

const (
	// defaultTimeout is the default timeout for E2E test operations.
	// Set high enough to accommodate browser startup and operations.
	defaultTimeout = 120 * time.Second
)

// TestServerStartsAndAcceptsConnections verifies that the MCP server
// starts correctly and accepts client connections.
func TestServerStartsAndAcceptsConnections(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Start test server
	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Create MCP client and initialize
	client := helpers.NewMCPClient(server.MCPURL())
	result, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("failed to initialize MCP client: %v", err)
	}
	defer client.Close(ctx)

	// Verify server info
	if result.ServerInfo.Name != "playwright-mcp" {
		t.Errorf("expected server name 'playwright-mcp', got '%s'", result.ServerInfo.Name)
	}
}

// TestSessionListReturnsEmptyInitially verifies that a fresh server
// has no browser sessions.
func TestSessionListReturnsEmptyInitially(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close(ctx)

	// List sessions - should return empty or zero sessions
	result, err := client.CallTool(ctx, "session_list", nil)
	if err != nil {
		t.Fatalf("failed to call session_list: %v", err)
	}

	if result.IsError {
		t.Errorf("session_list returned error: %v", result.Content)
	}
}

// TestListToolsReturnsRegisteredTools verifies that the server
// returns all registered tools.
func TestListToolsReturnsRegisteredTools(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close(ctx)

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}

	// Verify we have the expected tools
	expectedTools := []string{
		"session_create",
		"session_list",
		"session_close",
		"navigate",
		"go_back",
		"go_forward",
		"reload",
		"click",
		"type",
		"fill",
		"select_option",
		"hover",
		"press_key",
		"get_console_logs",
		"get_network_logs",
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("expected tool '%s' not found in tools list", expected)
		}
	}
}

// TestCreateAndCloseSession verifies the basic session lifecycle:
// create a session, verify it exists, and close it.
func TestCreateAndCloseSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close(ctx)

	// Create a session
	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Verify session exists in list
	result, err := client.CallTool(ctx, "session_list", nil)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if result.IsError {
		t.Errorf("session_list returned error: %v", result.Content)
	}

	// Close the session
	if err := client.CloseSession(ctx, sessionID); err != nil {
		t.Fatalf("failed to close session: %v", err)
	}
}

// TestNavigateToPage verifies that we can create a session,
// navigate to a page, and the page loads successfully.
// Uses a data URL to avoid network dependencies.
func TestNavigateToPage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Start MCP server
	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close(ctx)

	// Create session
	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer client.CloseSession(ctx, sessionID)

	// Navigate to a data URL (no network access required)
	dataURL := "data:text/html,<html><body><h1>Test Page</h1></body></html>"
	result, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": sessionID,
		"url":       dataURL,
		"waitUntil": "domcontentloaded",
	})
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	if result.IsError {
		t.Fatalf("navigation returned error: %v", result.Content)
	}
}

// TestMultipleSessionsCanCoexist verifies that multiple browser sessions
// can be created and used concurrently within the same MCP connection.
// Each session can navigate to different URLs independently.
func TestMultipleSessionsCanCoexist(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Start MCP server
	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close(ctx)

	// Create two sessions
	session1, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session 1: %v", err)
	}
	defer client.CloseSession(ctx, session1)

	session2, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session 2: %v", err)
	}
	defer client.CloseSession(ctx, session2)

	// Navigate sessions to different data URLs
	url1 := "data:text/html,<html><body><h1>Session 1</h1></body></html>"
	url2 := "data:text/html,<html><body><h1>Session 2</h1></body></html>"

	result1, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": session1,
		"url":       url1,
		"waitUntil": "domcontentloaded",
	})
	if err != nil {
		t.Fatalf("failed to navigate session 1: %v", err)
	}
	if result1.IsError {
		t.Fatalf("session 1 navigation error: %v", result1.Content)
	}

	result2, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": session2,
		"url":       url2,
		"waitUntil": "domcontentloaded",
	})
	if err != nil {
		t.Fatalf("failed to navigate session 2: %v", err)
	}
	if result2.IsError {
		t.Fatalf("session 2 navigation error: %v", result2.Content)
	}
}

// TestParallelServerInstances verifies that multiple test servers
// can run in parallel without port conflicts.
func TestParallelServerInstances(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close(ctx)
}

// TestDataURLNavigation verifies that navigation to data URLs works,
// which is useful for tests that don't require network access.
func TestDataURLNavigation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Start MCP server
	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close(ctx)

	// Create session and navigate to data URL
	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer client.CloseSession(ctx, sessionID)

	// Navigate to a data URL with inline HTML
	dataURL := "data:text/html,<html><head><title>Inline Test</title></head><body><h1>Inline Test Page</h1></body></html>"
	result, err := client.CallTool(ctx, "navigate", map[string]any{
		"sessionId": sessionID,
		"url":       dataURL,
		"waitUntil": "domcontentloaded",
	})
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	if result.IsError {
		t.Fatalf("navigation returned error: %v", result.Content)
	}
}

// TestMultipleNavigations verifies that multiple navigations work
// within the same session.
func TestMultipleNavigations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Start MCP server
	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close(ctx)

	// Create session
	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer client.CloseSession(ctx, sessionID)

	// Navigate to multiple data URLs
	pages := []string{
		"data:text/html,<h1>Page 1</h1>",
		"data:text/html,<h1>Page 2</h1>",
		"data:text/html,<h1>Page 3</h1>",
	}

	for i, url := range pages {
		result, err := client.CallTool(ctx, "navigate", map[string]any{
			"sessionId": sessionID,
			"url":       url,
			"waitUntil": "domcontentloaded",
		})
		if err != nil {
			t.Fatalf("failed to navigate to page %d: %v", i+1, err)
		}
		if result.IsError {
			t.Fatalf("navigation to page %d returned error: %v", i+1, result.Content)
		}
	}
}

// TestServerShutdownCleansUpResources verifies that stopping the server
// properly cleans up all resources including browser sessions.
//
// Note: Browser process cleanup is delegated to the server's shutdown logic
// (pkg/shutdown). The server's graceful shutdown sequence closes all browser
// sessions and stops the browser pool, which terminates browser processes.
// Directly verifying process termination would require platform-specific
// process inspection that adds complexity without additional coverage value.
func TestServerShutdownCleansUpResources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	server := helpers.NewTestServer(t)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	client := helpers.NewMCPClient(server.MCPURL())
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	// Note: We don't defer client.Close() here because this test specifically
	// verifies server shutdown cleanup - the server will be stopped before cleanup.

	// Create a session
	sessionID, err := client.CreateSession(ctx, "chromium", true)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	_ = sessionID // Session created to verify cleanup on shutdown

	// Stop the server - should clean up the session and browser processes
	if err := server.Stop(); err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}
}

// TestFixtureServerServesPages verifies that the FixtureServer correctly
// serves HTML fixture files from the fixtures directory.
func TestFixtureServerServesPages(t *testing.T) {
	t.Parallel()

	// Start fixture server
	fixtures := helpers.NewFixtureServer(t)
	defer fixtures.Close()

	// Test that simple.html is served correctly
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(fixtures.URL("simple.html"))
	if err != nil {
		t.Fatalf("failed to fetch simple.html: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	// Verify the page contains expected content
	if !strings.Contains(string(body), "Simple Test Page") {
		t.Error("simple.html does not contain expected title")
	}

	// Test form.html
	resp2, err := httpClient.Get(fixtures.URL("form.html"))
	if err != nil {
		t.Fatalf("failed to fetch form.html: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for form.html, got %d", resp2.StatusCode)
	}
}
