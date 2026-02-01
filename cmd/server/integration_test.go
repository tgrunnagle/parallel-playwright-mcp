package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// getFreePort returns an available port for testing
func getFreePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	return port
}

// sendInitializeRequest sends an MCP initialize request and returns the response
func sendInitializeRequest(t *testing.T, addr string) (*http.Response, map[string]any) {
	t.Helper()

	initRequest := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	body, err := json.Marshal(initRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("http://%s/mcp", addr)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	return resp, result
}

// startTestServer starts an MCP server on a free port and returns the address.
// Note: The mcp-go StreamableHTTPServer.Start() blocks indefinitely and doesn't
// expose a shutdown method. The cancel function signals intent to stop but the
// server goroutine continues until the test process ends. Test isolation is
// achieved through dynamic port allocation (getFreePort).
func startTestServer(t *testing.T) (string, func()) {
	t.Helper()

	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%s", port)

	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	httpServer := server.NewStreamableHTTPServer(mcpServer)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := httpServer.Start(addr); err != nil && ctx.Err() == nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Wait for server to be ready
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return addr, cancel
}

func TestServerStartsOnSpecifiedPort(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	// Verify server is listening
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server at %s: %v", addr, err)
	}
	conn.Close()
}

func TestMCPInitializeRequest(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	resp, _ := sendInitializeRequest(t, addr)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestMCPInitializeResponseContainsServerInfo(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	resp, result := sendInitializeRequest(t, addr)
	defer resp.Body.Close()

	// Verify JSON-RPC response structure
	if result["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", result["jsonrpc"])
	}

	resultData, ok := result["result"].(map[string]any)
	if !ok {
		t.Fatalf("Expected result object, got %T", result["result"])
	}

	// Verify server info
	serverInfo, ok := resultData["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("Expected serverInfo object, got %T", resultData["serverInfo"])
	}

	if serverInfo["name"] != serverName {
		t.Errorf("Expected server name %q, got %v", serverName, serverInfo["name"])
	}

	if serverInfo["version"] != serverVersion {
		t.Errorf("Expected server version %q, got %v", serverVersion, serverInfo["version"])
	}
}

func TestMCPInitializeResponseIncludesToolCapabilities(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	resp, result := sendInitializeRequest(t, addr)
	defer resp.Body.Close()

	resultData, ok := result["result"].(map[string]any)
	if !ok {
		t.Fatalf("Expected result object, got %T", result["result"])
	}

	// Verify capabilities include tools
	capabilities, ok := resultData["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("Expected capabilities object, got %T", resultData["capabilities"])
	}

	if _, hasTools := capabilities["tools"]; !hasTools {
		t.Error("Expected capabilities to include 'tools'")
	}
}

// Edge case tests for port validation

func TestValidatePort_InvalidNonNumericValue(t *testing.T) {
	testCases := []struct {
		name string
		port string
	}{
		{"alphabetic", "abc"},
		{"alphanumeric", "80abc"},
		{"empty string", ""},
		{"special chars", "80:80"},
		{"float", "80.5"},
		{"negative with text", "-abc"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePort(tc.port)
			if err == nil {
				t.Errorf("validatePort(%q) should return error for non-numeric value", tc.port)
			}
		})
	}
}

func TestValidatePort_OutOfRangeValues(t *testing.T) {
	testCases := []struct {
		name string
		port string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"too high", "65536"},
		{"way too high", "100000"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePort(tc.port)
			if err == nil {
				t.Errorf("validatePort(%q) should return error for out-of-range value", tc.port)
			}
		})
	}
}

func TestValidatePort_ValidValues(t *testing.T) {
	testCases := []struct {
		name string
		port string
	}{
		{"minimum valid", "1"},
		{"common port", "80"},
		{"common port", "443"},
		{"default port", "3000"},
		{"high port", "8080"},
		{"maximum valid", "65535"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePort(tc.port)
			if err != nil {
				t.Errorf("validatePort(%q) should not return error for valid value: %v", tc.port, err)
			}
		})
	}
}

func TestServerFailsOnPortInUse(t *testing.T) {
	// Start a listener to occupy a port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	addr := fmt.Sprintf("127.0.0.1:%s", port)

	// Try to start an MCP server on the same port
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)
	httpServer := server.NewStreamableHTTPServer(mcpServer)

	// Start should fail because port is in use
	errChan := make(chan error, 1)
	go func() {
		errChan <- httpServer.Start(addr)
	}()

	// Wait for error with timeout
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected error when starting server on port in use, got nil")
		}
		// Success - server correctly failed with an error
	case <-time.After(2 * time.Second):
		t.Error("Server did not fail within timeout when port is in use")
	}
}
