// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// Default viewport dimensions when not specified.
const (
	defaultViewportWidth  = 1280
	defaultViewportHeight = 720
)

// MCP error codes for session operations.
const (
	// ErrCodeSessionNotFound indicates the requested session does not exist or is inaccessible.
	ErrCodeSessionNotFound = -32001
)

// SessionCreateTool returns the session_create MCP tool definition.
func SessionCreateTool() mcp.Tool {
	return mcp.NewTool("session_create",
		mcp.WithDescription("Create a new isolated browser session"),
		mcp.WithString("browserType",
			mcp.Description("Browser engine to use (chromium, firefox, or webkit)"),
			mcp.Enum("chromium", "firefox", "webkit"),
		),
		mcp.WithBoolean("headless",
			mcp.Description("Run browser in headless mode (default: true)"),
		),
		mcp.WithObject("viewport",
			mcp.Description("Viewport size (default: 1280x720)"),
			mcp.Properties(map[string]any{
				"width": map[string]any{
					"type":        "number",
					"description": "Viewport width in pixels",
				},
				"height": map[string]any{
					"type":        "number",
					"description": "Viewport height in pixels",
				},
			}),
		),
	)
}

// SessionCreateHandler returns the handler function for session_create.
func SessionCreateHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership tracking
		mcpSessionID := getMCPSessionID(ctx)

		// Parse arguments with defaults
		opts := session.SessionOptions{
			BrowserType: browser.BrowserChromium,
			Headless:    true,
			Viewport:    &browser.Viewport{Width: defaultViewportWidth, Height: defaultViewportHeight},
		}

		// Parse browserType if provided
		browserType := req.GetString("browserType", "")
		if browserType != "" {
			switch browserType {
			case "chromium":
				opts.BrowserType = browser.BrowserChromium
			case "firefox":
				opts.BrowserType = browser.BrowserFirefox
			case "webkit":
				opts.BrowserType = browser.BrowserWebKit
			default:
				return mcp.NewToolResultError(fmt.Sprintf("Invalid browserType: %s. Must be chromium, firefox, or webkit", browserType)), nil
			}
		}

		// Parse headless if provided
		// GetArguments returns a map[string]any that we can check for key existence
		args := req.GetArguments()
		if _, ok := args["headless"]; ok {
			opts.Headless = req.GetBool("headless", true)
		}

		// Parse viewport if provided
		if vpArg, ok := args["viewport"]; ok && vpArg != nil {
			if vpMap, ok := vpArg.(map[string]any); ok {
				viewport := parseViewport(vpMap)
				if viewport != nil {
					opts.Viewport = viewport
				}
			}
		}

		// Create the browser session
		sess, err := mgr.CreateSession(ctx, mcpSessionID, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create session: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Created session: %s", sess.ID)), nil
	}
}

// SessionListTool returns the session_list MCP tool definition.
func SessionListTool() mcp.Tool {
	return mcp.NewTool("session_list",
		mcp.WithDescription("List all active browser sessions for this connection"),
	)
}

// SessionListHandler returns the handler function for session_list.
func SessionListHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership scoping
		mcpSessionID := getMCPSessionID(ctx)

		// Get all sessions for this MCP connection
		sessions := mgr.ListSessions(mcpSessionID)

		// Format response as JSON
		response, err := formatSessionList(sessions)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format session list: %v", err)), nil
		}

		return mcp.NewToolResultText(response), nil
	}
}

// SessionCloseTool returns the session_close MCP tool definition.
func SessionCloseTool() mcp.Tool {
	return mcp.NewTool("session_close",
		mcp.WithDescription("Close a browser session and release all associated resources"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID to close"),
		),
	)
}

// SessionCloseHandler returns the handler function for session_close.
func SessionCloseHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Close the session (manager handles ownership validation and cleanup)
		if err := mgr.CloseSession(ctx, mcpSessionID, sessionID); err != nil {
			// Return error with code and descriptive message
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Session not found: %s", ErrCodeSessionNotFound, sessionID)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Closed session: %s", sessionID)), nil
	}
}

// getMCPSessionID extracts the MCP session ID from the context.
// The mcp-go library stores session information in the context during request handling.
func getMCPSessionID(ctx context.Context) string {
	clientSession := server.ClientSessionFromContext(ctx)
	if clientSession != nil {
		return clientSession.SessionID()
	}
	return ""
}

// parseViewport extracts width and height from a viewport parameter object.
// Returns nil if the viewport cannot be parsed or is invalid.
func parseViewport(vp map[string]any) *browser.Viewport {
	var width, height int

	// Parse width (may come as float64 from JSON)
	switch w := vp["width"].(type) {
	case float64:
		width = int(w)
	case int:
		width = w
	default:
		return nil
	}

	// Parse height (may come as float64 from JSON)
	switch h := vp["height"].(type) {
	case float64:
		height = int(h)
	case int:
		height = h
	default:
		return nil
	}

	// Validate dimensions are positive
	if width <= 0 || height <= 0 {
		return nil
	}

	return &browser.Viewport{Width: width, Height: height}
}

// SessionListResponse represents the JSON response structure for session_list.
type SessionListResponse struct {
	Sessions []SessionListItem `json:"sessions"`
}

// SessionListItem represents a single session in the list response.
type SessionListItem struct {
	SessionID   string `json:"sessionId"`
	BrowserType string `json:"browserType"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	CreatedAt   string `json:"createdAt"`
}

// formatSessionList converts SessionInfo slice to JSON string response.
func formatSessionList(sessions []*session.SessionInfo) (string, error) {
	items := make([]SessionListItem, 0, len(sessions))

	for _, sess := range sessions {
		items = append(items, SessionListItem{
			SessionID:   sess.ID,
			BrowserType: string(sess.BrowserType),
			URL:         sess.URL,
			Title:       sess.Title,
			CreatedAt:   sess.CreatedAt.Format(time.RFC3339),
		})
	}

	response := SessionListResponse{Sessions: items}

	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}
