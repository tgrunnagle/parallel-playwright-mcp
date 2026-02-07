package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/middleware"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/shutdown"
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
// The server includes both the MCP handler at /mcp and the health endpoint at /health.
// Note: The cancel function signals intent to stop but the server goroutine continues
// until the test process ends. Test isolation is achieved through dynamic port allocation.
func startTestServer(t *testing.T) (string, func()) {
	t.Helper()

	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%s", port)

	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	mcpHandler := server.NewStreamableHTTPServer(mcpServer)

	// Create custom HTTP mux with health endpoint and MCP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler(serverVersion))
	mux.Handle("/mcp", mcpHandler)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil && ctx.Err() == nil {
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

func TestHealthEndpoint_IsAccessibleOnRunningServer(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	url := fmt.Sprintf("http://%s/health", addr)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Failed to send health request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHealthEndpoint_ReturnsExpectedJSONSchema(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	url := fmt.Sprintf("http://%s/health", addr)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Failed to send health request: %v", err)
	}
	defer resp.Body.Close()

	// Verify Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Verify response body
	var response HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response.Status)
	}

	if response.Version != serverVersion {
		t.Errorf("Expected version '%s', got '%s'", serverVersion, response.Version)
	}
}

func TestHealthEndpoint_RespondsWhileMCPOperationsInProgress(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	// Start an MCP initialize request in background (which may take longer)
	go func() {
		url := fmt.Sprintf("http://%s/mcp", addr)
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
		body, _ := json.Marshal(initRequest)
		http.Post(url, "application/json", bytes.NewReader(body))
	}()

	// Immediately check the health endpoint - it should respond quickly
	healthURL := fmt.Sprintf("http://%s/health", addr)
	client := &http.Client{Timeout: 100 * time.Millisecond}

	resp, err := client.Get(healthURL)
	if err != nil {
		t.Fatalf("Health endpoint did not respond within 100ms: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// startTestServerWithShutdown starts an MCP server with proper shutdown support.
// It returns the address, a shutdown function, and a wait function that blocks until shutdown completes.
func startTestServerWithShutdown(t *testing.T) (addr string, triggerShutdown func(), waitForShutdown func() error) {
	t.Helper()

	port := getFreePort(t)
	addr = fmt.Sprintf("127.0.0.1:%s", port)

	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)
	mcpHandler := server.NewStreamableHTTPServer(mcpServer)

	// Create request tracker and middleware
	requestTracker := shutdown.NewRequestTracker()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler(serverVersion))
	mux.Handle("/mcp", mcpHandler)

	// Wrap with middleware
	handler := middleware.PanicRecovery(requestTracker.Middleware(mux))

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Create shutdown coordinator with short timeouts for testing
	config := shutdown.DefaultConfig().
		WithDrainTimeout(2 * time.Second).
		WithPhaseTimeout(2 * time.Second).
		WithTotalTimeout(5 * time.Second)

	coordinator := shutdown.NewCoordinator(
		config,
		httpServer,
		nil, // No session manager for basic tests
		nil, // No browser pool for basic tests
		requestTracker,
	)

	shutdownChan := make(chan struct{})
	errChan := make(chan error, 1)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
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

	var shutdownErr error
	var shutdownOnce sync.Once

	triggerShutdown = func() {
		shutdownOnce.Do(func() {
			shutdownErr = coordinator.Shutdown(context.Background())
			close(shutdownChan)
		})
	}

	waitForShutdown = func() error {
		<-shutdownChan
		return shutdownErr
	}

	return addr, triggerShutdown, waitForShutdown
}

func TestGracefulShutdown_ServerStopsAcceptingConnections(t *testing.T) {
	addr, triggerShutdown, waitForShutdown := startTestServerWithShutdown(t)

	// Verify server is responding
	healthURL := fmt.Sprintf("http://%s/health", addr)
	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(healthURL)
	if err != nil {
		t.Fatalf("Initial request failed: %v", err)
	}
	resp.Body.Close()

	// Trigger shutdown
	triggerShutdown()

	// Wait for shutdown to complete
	if err := waitForShutdown(); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}

	// Verify server is no longer accepting connections
	time.Sleep(100 * time.Millisecond)
	_, err = client.Get(healthURL)
	if err == nil {
		t.Error("Expected request to fail after shutdown, but it succeeded")
	}
}

func TestGracefulShutdown_InFlightRequestCompletes(t *testing.T) {
	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%s", port)

	// Create a handler that simulates a slow request
	requestStarted := make(chan struct{})
	requestCanProceed := make(chan struct{})
	requestCompleted := make(chan struct{})

	requestTracker := shutdown.NewRequestTracker()

	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-requestCanProceed
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("completed"))
		close(requestCompleted)
	})
	mux.HandleFunc("/health", healthHandler(serverVersion))

	handler := requestTracker.Middleware(mux)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	config := shutdown.DefaultConfig().
		WithDrainTimeout(5 * time.Second).
		WithPhaseTimeout(2 * time.Second).
		WithTotalTimeout(10 * time.Second)

	coordinator := shutdown.NewCoordinator(
		config,
		httpServer,
		nil,
		nil,
		requestTracker,
	)

	go httpServer.ListenAndServe()

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

	// Start a slow request
	responseChan := make(chan *http.Response, 1)
	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/slow", addr))
		if err != nil {
			t.Logf("Slow request error: %v", err)
			return
		}
		responseChan <- resp
	}()

	// Wait for request to start
	<-requestStarted

	// Trigger shutdown while request is in-flight
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- coordinator.Shutdown(context.Background())
	}()

	// Let the request complete
	close(requestCanProceed)

	// Wait for request to complete
	select {
	case resp := <-responseChan:
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 status, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	case <-time.After(3 * time.Second):
		t.Error("In-flight request did not complete within timeout")
	}

	// Wait for shutdown to complete
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Errorf("Shutdown returned error: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Error("Shutdown did not complete within timeout")
	}
}

func TestGracefulShutdown_CompletesWithinTimeout(t *testing.T) {
	addr, triggerShutdown, waitForShutdown := startTestServerWithShutdown(t)

	// Verify server is responding
	healthURL := fmt.Sprintf("http://%s/health", addr)
	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(healthURL)
	if err != nil {
		t.Fatalf("Initial request failed: %v", err)
	}
	resp.Body.Close()

	// Trigger shutdown and measure time
	start := time.Now()
	triggerShutdown()

	if err := waitForShutdown(); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
	elapsed := time.Since(start)

	// Shutdown should complete quickly when there are no active requests
	if elapsed > 3*time.Second {
		t.Errorf("Shutdown took too long: %v", elapsed)
	}
}
