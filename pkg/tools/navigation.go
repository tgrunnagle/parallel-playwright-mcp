// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// NavigateTool returns the navigate MCP tool definition.
// Navigate loads a URL in the browser session with configurable wait conditions.
func NavigateTool() mcp.Tool {
	return mcp.NewTool("navigate",
		mcp.WithDescription("Navigate to a URL in the browser session"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("URL to navigate to"),
		),
		mcp.WithString("waitUntil",
			mcp.Description("Wait condition: load (default), domcontentloaded, or networkidle"),
			mcp.Enum("load", "domcontentloaded", "networkidle"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Navigation timeout in milliseconds (default: 30000)"),
		),
	)
}

// NavigateHandler returns the handler function for navigate.
func NavigateHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "navigate")

		// Apply timeout with navigation category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutNavigation, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate required sessionId parameter
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "navigate")
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Parse and validate required url parameter
		url := req.GetString("url", "")
		if url == "" {
			slog.Error("missing required url", "tool", "navigate", "sessionID", sessionID)
			return mcp.NewToolResultError("url is required"), nil
		}

		slog.Debug("navigating", "tool", "navigate", "sessionID", sessionID, "url", url)

		// Retrieve browser session (validates ownership)
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "navigate", "sessionID", sessionID)
			return newNavigationSessionNotFoundError(sessionID), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "navigate", "sessionID", sessionID)
			return mcp.NewToolResultError("no active page in session"), nil
		}

		// Build Playwright options from optional parameters
		opts := buildGotoOptions(args)
		if opts.Timeout == nil {
			opts.Timeout = PlaywrightTimeoutFromContext(ctx)
		}

		// Execute navigation
		if _, err := page.Goto(url, opts); err != nil {
			if ctxErr := HandleContextError(ctx, fmt.Sprintf("navigation to %s", url)); ctxErr != nil {
				slog.Error("navigation context error", "tool", "navigate", "sessionID", sessionID, "url", url, "error", ctxErr)
				return mcp.NewToolResultError(errors.FormatErrorForTool(ctxErr)), nil
			}
			slog.Error("navigation failed", "tool", "navigate", "sessionID", sessionID, "url", url, "error", err)
			return handleNavigationError(err, url), nil
		}

		slog.Info("navigation completed", "tool", "navigate", "sessionID", sessionID, "url", url)
		return mcp.NewToolResultText(fmt.Sprintf("Navigated to %s", url)), nil
	}
}

// GoBackTool returns the go_back MCP tool definition.
// GoBack navigates to the previous page in browser history.
func GoBackTool() mcp.Tool {
	return mcp.NewTool("go_back",
		mcp.WithDescription("Go back to the previous page in browser history"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("waitUntil",
			mcp.Description("Wait condition: load (default), domcontentloaded, or networkidle"),
			mcp.Enum("load", "domcontentloaded", "networkidle"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Navigation timeout in milliseconds (default: 30000)"),
		),
	)
}

// GoBackHandler returns the handler function for go_back.
func GoBackHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "go_back")

		// Apply timeout with navigation category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutNavigation, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate required sessionId parameter
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "go_back")
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Retrieve browser session (validates ownership)
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "go_back", "sessionID", sessionID)
			return newNavigationSessionNotFoundError(sessionID), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "go_back", "sessionID", sessionID)
			return mcp.NewToolResultError("no active page in session"), nil
		}

		// Build Playwright options from optional parameters
		opts := buildGoBackOptions(args)
		if opts.Timeout == nil {
			opts.Timeout = PlaywrightTimeoutFromContext(ctx)
		}

		// Execute back navigation
		resp, err := page.GoBack(opts)
		if err != nil {
			if ctxErr := HandleContextError(ctx, "go_back"); ctxErr != nil {
				slog.Error("go_back context error", "tool", "go_back", "sessionID", sessionID, "error", ctxErr)
				return mcp.NewToolResultError(errors.FormatErrorForTool(ctxErr)), nil
			}
			slog.Error("go_back failed", "tool", "go_back", "sessionID", sessionID, "error", err)
			return handleNavigationError(err, "back"), nil
		}

		// Handle case where no previous page exists
		if resp == nil {
			slog.Debug("no previous page in history", "tool", "go_back", "sessionID", sessionID)
			return mcp.NewToolResultText("No previous page in history"), nil
		}

		slog.Debug("navigated back", "tool", "go_back", "sessionID", sessionID, "url", resp.URL())
		return mcp.NewToolResultText(fmt.Sprintf("Navigated back to %s", resp.URL())), nil
	}
}

// GoForwardTool returns the go_forward MCP tool definition.
// GoForward navigates to the next page in browser history.
func GoForwardTool() mcp.Tool {
	return mcp.NewTool("go_forward",
		mcp.WithDescription("Go forward to the next page in browser history"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("waitUntil",
			mcp.Description("Wait condition: load (default), domcontentloaded, or networkidle"),
			mcp.Enum("load", "domcontentloaded", "networkidle"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Navigation timeout in milliseconds (default: 30000)"),
		),
	)
}

// GoForwardHandler returns the handler function for go_forward.
func GoForwardHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "go_forward")

		// Apply timeout with navigation category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutNavigation, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate required sessionId parameter
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "go_forward")
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Retrieve browser session (validates ownership)
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "go_forward", "sessionID", sessionID)
			return newNavigationSessionNotFoundError(sessionID), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "go_forward", "sessionID", sessionID)
			return mcp.NewToolResultError("no active page in session"), nil
		}

		// Build Playwright options from optional parameters
		opts := buildGoForwardOptions(args)
		if opts.Timeout == nil {
			opts.Timeout = PlaywrightTimeoutFromContext(ctx)
		}

		// Execute forward navigation
		resp, err := page.GoForward(opts)
		if err != nil {
			if ctxErr := HandleContextError(ctx, "go_forward"); ctxErr != nil {
				slog.Error("go_forward context error", "tool", "go_forward", "sessionID", sessionID, "error", ctxErr)
				return mcp.NewToolResultError(errors.FormatErrorForTool(ctxErr)), nil
			}
			slog.Error("go_forward failed", "tool", "go_forward", "sessionID", sessionID, "error", err)
			return handleNavigationError(err, "forward"), nil
		}

		// Handle case where no forward page exists
		if resp == nil {
			slog.Debug("no forward page in history", "tool", "go_forward", "sessionID", sessionID)
			return mcp.NewToolResultText("No forward page in history"), nil
		}

		slog.Debug("navigated forward", "tool", "go_forward", "sessionID", sessionID, "url", resp.URL())
		return mcp.NewToolResultText(fmt.Sprintf("Navigated forward to %s", resp.URL())), nil
	}
}

// ReloadTool returns the reload MCP tool definition.
// Reload refreshes the current page.
func ReloadTool() mcp.Tool {
	return mcp.NewTool("reload",
		mcp.WithDescription("Reload the current page in the browser session"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("waitUntil",
			mcp.Description("Wait condition: load (default), domcontentloaded, or networkidle"),
			mcp.Enum("load", "domcontentloaded", "networkidle"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Reload timeout in milliseconds"),
		),
	)
}

// ReloadHandler returns the handler function for reload.
func ReloadHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "reload")

		// Apply timeout with navigation category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutNavigation, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate required sessionId parameter
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "reload")
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Retrieve browser session (validates ownership)
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "reload", "sessionID", sessionID)
			return newNavigationSessionNotFoundError(sessionID), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "reload", "sessionID", sessionID)
			return mcp.NewToolResultError("no active page in session"), nil
		}

		// Build Playwright reload options from optional parameters
		opts := buildReloadOptions(args)
		if opts.Timeout == nil {
			opts.Timeout = PlaywrightTimeoutFromContext(ctx)
		}

		// Execute reload
		if _, err := page.Reload(opts); err != nil {
			if ctxErr := HandleContextError(ctx, "reload"); ctxErr != nil {
				slog.Error("reload context error", "tool", "reload", "sessionID", sessionID, "error", ctxErr)
				return mcp.NewToolResultError(errors.FormatErrorForTool(ctxErr)), nil
			}
			slog.Error("reload failed", "tool", "reload", "sessionID", sessionID, "error", err)
			return handleNavigationError(err, "reload"), nil
		}

		slog.Debug("page reloaded", "tool", "reload", "sessionID", sessionID, "url", page.URL())
		return mcp.NewToolResultText(fmt.Sprintf("Reloaded page: %s", page.URL())), nil
	}
}

// buildGotoOptions constructs Playwright PageGotoOptions from tool arguments.
func buildGotoOptions(args map[string]any) playwright.PageGotoOptions {
	opts := playwright.PageGotoOptions{}

	if waitUntil, ok := args["waitUntil"].(string); ok && waitUntil != "" {
		opts.WaitUntil = mapWaitUntil(waitUntil)
	}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// buildGoBackOptions constructs Playwright PageGoBackOptions from tool arguments.
func buildGoBackOptions(args map[string]any) playwright.PageGoBackOptions {
	opts := playwright.PageGoBackOptions{}

	if waitUntil, ok := args["waitUntil"].(string); ok && waitUntil != "" {
		opts.WaitUntil = mapWaitUntil(waitUntil)
	}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// buildGoForwardOptions constructs Playwright PageGoForwardOptions from tool arguments.
func buildGoForwardOptions(args map[string]any) playwright.PageGoForwardOptions {
	opts := playwright.PageGoForwardOptions{}

	if waitUntil, ok := args["waitUntil"].(string); ok && waitUntil != "" {
		opts.WaitUntil = mapWaitUntil(waitUntil)
	}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// buildReloadOptions constructs Playwright PageReloadOptions from tool arguments.
func buildReloadOptions(args map[string]any) playwright.PageReloadOptions {
	opts := playwright.PageReloadOptions{}

	if waitUntil, ok := args["waitUntil"].(string); ok && waitUntil != "" {
		opts.WaitUntil = mapWaitUntil(waitUntil)
	}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// mapWaitUntil converts string wait condition to Playwright WaitUntilState.
// Returns load state as default for unrecognized values.
func mapWaitUntil(waitUntil string) *playwright.WaitUntilState {
	switch waitUntil {
	case "domcontentloaded":
		return playwright.WaitUntilStateDomcontentloaded
	case "networkidle":
		return playwright.WaitUntilStateNetworkidle
	case "load":
		fallthrough
	default:
		return playwright.WaitUntilStateLoad
	}
}

// newNavigationSessionNotFoundError creates an error result for invalid session ID.
// Uses error code -32001 (Session Not Found).
func newNavigationSessionNotFoundError(sessionID string) *mcp.CallToolResult {
	return mcp.NewToolResultError(errors.FormatErrorForTool(errors.NewSessionNotFoundError(sessionID)))
}

// handleNavigationError classifies navigation errors and returns appropriate error results.
// Timeout errors get code -32003, other navigation failures get code -32004.
func handleNavigationError(err error, target string) *mcp.CallToolResult {
	if isTimeoutError(err) {
		slog.Error("navigation timeout", "target", target, "error", err)
		return mcp.NewToolResultError(errors.FormatErrorForTool(
			errors.WrapTimeoutError(fmt.Sprintf("navigation to %s", target), 0, err)))
	}
	slog.Error("navigation failed", "target", target, "error", err)
	return mcp.NewToolResultError(errors.FormatErrorForTool(
		errors.WrapNavigationError(target, "", err)))
}

// isTimeoutError checks if the error is a timeout error based on error message.
func isTimeoutError(err error) bool {
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "exceeded")
}
