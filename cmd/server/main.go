// Package main is the entry point for the Playwright MCP Server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/config"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/middleware"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/shutdown"
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
	// Exit code to use after panic cleanup
	exitCode := 0

	// Set up emergency cleanup (will be populated in run())
	var emergencyCleanup *shutdown.EmergencyCleanup

	defer func() {
		if r := recover(); r != nil {
			if emergencyCleanup != nil {
				emergencyCleanup.Execute(r)
			} else {
				slog.Error("panic before emergency cleanup was initialized",
					"panic", r,
				)
			}
			exitCode = 1
		}
		os.Exit(exitCode)
	}()

	// Create root context with signal handling for graceful shutdown.
	// Note: On Windows, SIGTERM is not natively supported. Only SIGINT (Ctrl+C) will
	// trigger graceful shutdown. On Unix-like systems, both SIGTERM and SIGINT work.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := run(ctx, &emergencyCleanup); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("server error", "error", err)
		exitCode = 1
	}
}

func run(ctx context.Context, emergencyCleanup **shutdown.EmergencyCleanup) error {
	// Load configuration from file and environment
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create and start browser pool
	poolOpts := browser.PoolOptions{
		DefaultHeadless: cfg.Browser.Headless,
	}
	pool := browser.NewBrowserPoolWithOptions(poolOpts)

	if err := pool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start browser pool: %w", err)
	}

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

	// Create request tracker for connection draining
	requestTracker := shutdown.NewRequestTracker()

	// Create custom HTTP mux with health endpoint and MCP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler(serverVersion))
	mux.Handle("/mcp", mcpHandler)

	// Wrap with middleware: panic recovery -> request tracking -> mux
	handler := middleware.PanicRecovery(requestTracker.Middleware(mux))

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Create shutdown coordinator
	shutdownCoord := shutdown.NewCoordinator(
		shutdown.DefaultConfig(),
		httpServer,
		sessionMgr,
		pool,
		requestTracker,
	)

	// Set up emergency cleanup now that we have the coordinator
	*emergencyCleanup = shutdown.NewEmergencyCleanup(shutdownCoord, pool)

	// Start HTTP server in goroutine
	errChan := make(chan error, 1)
	go func() {
		slog.Info("server starting", "name", serverName, "version", serverVersion, "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		// Use context.Background() instead of the cancelled ctx to allow the shutdown
		// sequence to complete without immediate cancellation. The coordinator has its
		// own timeouts (TotalTimeout, DrainTimeout, PhaseTimeout) to prevent hanging.
		// A second signal won't accelerate shutdown, but the timeouts ensure completion.
		return shutdownCoord.Shutdown(context.Background())
	case err := <-errChan:
		if err != nil {
			return err
		}
		return nil
	}
}
