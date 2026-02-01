// Package main is the entry point for the Playwright MCP Server.
package main

import (
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/server"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/config"
)

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

	// Create MCP server with tool capabilities
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// Create streamable HTTP server
	httpServer := server.NewStreamableHTTPServer(mcpServer)

	// Log startup
	log.Printf("Starting %s v%s on %s", serverName, serverVersion, addr)

	// Start server (blocks until error or shutdown)
	if err := httpServer.Start(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
