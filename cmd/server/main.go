// Package main is the entry point for the Playwright MCP Server.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "playwright-mcp"
	serverVersion = "0.1.0"
	defaultHost   = "127.0.0.1"
	defaultPort   = "3000"
)

func main() {
	// Create MCP server with tool capabilities
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	// Get configuration from environment
	host := getEnv("MCP_HOST", defaultHost)
	port := getEnv("MCP_PORT", defaultPort)

	// Validate port is a valid number
	if err := validatePort(port); err != nil {
		log.Fatalf("Invalid MCP_PORT value %q: %v", port, err)
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	// Create streamable HTTP server
	httpServer := server.NewStreamableHTTPServer(mcpServer)

	// Log startup
	log.Printf("Starting %s v%s on %s", serverName, serverVersion, addr)

	// Start server (blocks until error or shutdown)
	if err := httpServer.Start(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// getEnv returns the value of an environment variable or a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// validatePort validates that a port string is a valid port number.
// Returns an error if the port is not a valid number or out of range.
func validatePort(port string) error {
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("must be between 1 and 65535")
	}
	return nil
}
