// Package main is the entry point for the Playwright MCP Server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/mark3labs/mcp-go/server"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/config"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/tools"
)

// HealthResponse represents the health check response structure.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// healthHandler returns an HTTP handler function for the /health endpoint.
func healthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := HealthResponse{
			Status:  "ok",
			Version: version,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

const (
	serverName    = "playwright-mcp"
	serverVersion = "0.1.0"
)

func main() {
	// Load configuration from file and environment
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create and start browser pool
	poolOpts := browser.PoolOptions{
		DefaultHeadless: cfg.Browser.Headless,
	}
	pool := browser.NewBrowserPoolWithOptions(poolOpts)

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		log.Fatalf("Failed to start browser pool: %v", err)
	}
	defer func() {
		if err := pool.Stop(context.Background()); err != nil {
			log.Printf("Error stopping browser pool: %v", err)
		}
	}()

	// Create session manager
	sessionMgr := session.NewManager(pool)

	// Create MCP server with tool capabilities
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	// Register session management tools
	mcpServer.AddTool(tools.SessionCreateTool(), tools.SessionCreateHandler(sessionMgr))
	mcpServer.AddTool(tools.SessionListTool(), tools.SessionListHandler(sessionMgr))
	mcpServer.AddTool(tools.SessionCloseTool(), tools.SessionCloseHandler(sessionMgr))

	// Register inspection tools
	mcpServer.AddTool(tools.GetConsoleLogsTool(), tools.GetConsoleLogsHandler(sessionMgr))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// Create streamable HTTP server (implements http.Handler)
	mcpHandler := server.NewStreamableHTTPServer(mcpServer)

	// Create custom HTTP mux with health endpoint and MCP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler(serverVersion))
	mux.Handle("/mcp", mcpHandler)

	// Log startup
	log.Printf("Starting %s v%s on %s", serverName, serverVersion, addr)

	// Start server (blocks until error or shutdown)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
