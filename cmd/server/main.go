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

	"github.com/mark3labs/mcp-go/mcp"
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

	// Create root context with cancellation for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown.
	// First SIGTERM/SIGINT triggers graceful shutdown via context cancellation.
	// Second signal forces immediate termination.
	// Note: On Windows, SIGTERM is not natively supported. Only SIGINT (Ctrl+C) will
	// trigger graceful shutdown. On Unix-like systems, both SIGTERM and SIGINT work.
	setupSignalHandler(cancel)

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

	// Configure logging level based on config
	var logLevel slog.Level
	switch cfg.Logging.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// Set up slog handler with configured level
	var logHandler slog.Handler
	if cfg.Logging.Format == "text" {
		logHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	} else {
		logHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	}
	slog.SetDefault(slog.New(logHandler))

	// Create and start browser pool with configured launch options
	poolOpts := browser.PoolOptions{
		DefaultHeadless: cfg.Browser.Headless,
	}

	// Pass browser-specific launch options from config
	if len(cfg.Browser.Chromium.Args) > 0 || cfg.Browser.Chromium.Channel != "" {
		poolOpts.ChromiumOptions = &browser.BrowserLaunchOptions{
			Args:    cfg.Browser.Chromium.Args,
			Channel: cfg.Browser.Chromium.Channel,
		}
	}
	if len(cfg.Browser.Firefox.Args) > 0 || cfg.Browser.Firefox.Channel != "" {
		poolOpts.FirefoxOptions = &browser.BrowserLaunchOptions{
			Args:    cfg.Browser.Firefox.Args,
			Channel: cfg.Browser.Firefox.Channel,
		}
	}
	if len(cfg.Browser.WebKit.Args) > 0 || cfg.Browser.WebKit.Channel != "" {
		poolOpts.WebKitOptions = &browser.BrowserLaunchOptions{
			Args:    cfg.Browser.WebKit.Args,
			Channel: cfg.Browser.WebKit.Channel,
		}
	}

	// Pass slowMo if configured
	if cfg.Browser.SlowMo > 0 {
		slowMo := float64(cfg.Browser.SlowMo)
		if poolOpts.ChromiumOptions == nil {
			poolOpts.ChromiumOptions = &browser.BrowserLaunchOptions{}
		}
		poolOpts.ChromiumOptions.SlowMo = &slowMo
		if poolOpts.FirefoxOptions == nil {
			poolOpts.FirefoxOptions = &browser.BrowserLaunchOptions{}
		}
		poolOpts.FirefoxOptions.SlowMo = &slowMo
		if poolOpts.WebKitOptions == nil {
			poolOpts.WebKitOptions = &browser.BrowserLaunchOptions{}
		}
		poolOpts.WebKitOptions.SlowMo = &slowMo
	}

	pool := browser.NewBrowserPoolWithOptions(poolOpts)

	if err := pool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start browser pool: %w", err)
	}

	// Create session manager
	sessionMgr := session.NewManager(pool)

	// Create timeout config from loaded configuration
	timeoutConfig := cfg.Session.Timeout.ToTimeoutConfig()

	// Create MCP server with tool capabilities
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	// Build enabled tools set for filtering
	enabledSet := make(map[string]bool, len(cfg.EnabledTools))
	for _, name := range cfg.EnabledTools {
		enabledSet[name] = true
	}
	addTool := func(tool mcp.Tool, handler server.ToolHandlerFunc) {
		if len(enabledSet) > 0 && !enabledSet[tool.Name] {
			slog.Debug("tool disabled by configuration", "tool", tool.Name)
			return
		}
		mcpServer.AddTool(tool, handler)
	}

	// Register session management tools
	addTool(tools.SessionCreateTool(), tools.SessionCreateHandler(sessionMgr, timeoutConfig))
	addTool(tools.SessionListTool(), tools.SessionListHandler(sessionMgr, timeoutConfig))
	addTool(tools.SessionCloseTool(), tools.SessionCloseHandler(sessionMgr, timeoutConfig))

	// Register inspection tools
	addTool(tools.GetConsoleLogsTool(), tools.GetConsoleLogsHandler(sessionMgr, timeoutConfig))
	addTool(tools.GetNetworkLogsTool(), tools.GetNetworkLogsHandler(sessionMgr, timeoutConfig))
	addTool(tools.ScreenshotTool(), tools.ScreenshotHandler(sessionMgr, timeoutConfig))
	addTool(tools.ExtractTextTool(), tools.ExtractTextHandler(sessionMgr, timeoutConfig))
	addTool(tools.GetHTMLTool(), tools.GetHTMLHandler(sessionMgr, timeoutConfig))
	addTool(tools.EvaluateTool(), tools.EvaluateHandler(sessionMgr, timeoutConfig))
	addTool(tools.QuerySelectorTool(), tools.QuerySelectorHandler(sessionMgr, timeoutConfig))
	addTool(tools.GetAccessibilityTreeTool(), tools.GetAccessibilityTreeHandler(sessionMgr, timeoutConfig))
	addTool(tools.NavigateAndExtractTextTool(), tools.NavigateAndExtractTextHandler(sessionMgr, timeoutConfig))

	// Register navigation tools
	addTool(tools.NavigateTool(), tools.NavigateHandler(sessionMgr, timeoutConfig))
	addTool(tools.GoBackTool(), tools.GoBackHandler(sessionMgr, timeoutConfig))
	addTool(tools.GoForwardTool(), tools.GoForwardHandler(sessionMgr, timeoutConfig))
	addTool(tools.ReloadTool(), tools.ReloadHandler(sessionMgr, timeoutConfig))

	// Register interaction tools
	addTool(tools.ClickTool(), tools.ClickHandler(sessionMgr, timeoutConfig))
	addTool(tools.TypeTool(), tools.TypeHandler(sessionMgr, timeoutConfig))
	addTool(tools.FillTool(), tools.FillHandler(sessionMgr, timeoutConfig))
	addTool(tools.SelectOptionTool(), tools.SelectOptionHandler(sessionMgr, timeoutConfig))
	addTool(tools.HoverTool(), tools.HoverHandler(sessionMgr, timeoutConfig))
	addTool(tools.PressKeyTool(), tools.PressKeyHandler(sessionMgr, timeoutConfig))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// Create HTTP server (supports both JSON-RPC POST and SSE)
	mcpHandler := server.NewStreamableHTTPServer(mcpServer)

	// Create request tracker for connection draining
	requestTracker := shutdown.NewRequestTracker()

	// Create custom HTTP mux with health endpoint and MCP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler(serverVersion))
	mux.Handle("/mcp", mcpHandler)

	// Wrap with middleware: CORS -> panic recovery -> request tracking -> mux
	handler := middleware.CORS(middleware.PanicRecovery(requestTracker.Middleware(mux)))

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
		// Note: The actual signal type is logged by setupSignalHandler.
		// Use context.Background() instead of the cancelled ctx to allow the shutdown
		// sequence to complete without immediate cancellation. The coordinator has its
		// own timeouts (TotalTimeout, DrainTimeout, PhaseTimeout) to prevent hanging.
		return shutdownCoord.Shutdown(context.Background())
	case err := <-errChan:
		if err != nil {
			return err
		}
		return nil
	}
}

// setupSignalHandler configures OS signal handling for graceful shutdown.
// First SIGTERM/SIGINT triggers graceful shutdown via context cancellation.
// Second signal forces immediate termination with exit code 1.
func setupSignalHandler(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		// Wait for first signal
		sig := <-sigChan
		slog.Info("received shutdown signal, initiating graceful shutdown", "signal", sig)
		cancel()

		// Wait for second signal (force quit)
		sig = <-sigChan
		slog.Warn("received second signal, forcing immediate shutdown", "signal", sig)
		os.Exit(1)
	}()
}
