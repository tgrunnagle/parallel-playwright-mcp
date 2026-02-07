// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// GetConsoleLogsTool returns the get_console_logs MCP tool definition.
func GetConsoleLogsTool() mcp.Tool {
	return mcp.NewTool("get_console_logs",
		mcp.WithDescription("Retrieve browser console log messages from a session"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of entries to return (default: 50)"),
		),
		mcp.WithString("level",
			mcp.Enum("log", "info", "warn", "error", "debug"),
			mcp.Description("Filter by log level (returns all levels if not specified)"),
		),
	)
}

// GetConsoleLogsHandler returns the handler function for get_console_logs.
func GetConsoleLogsHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Session not found: %s", ErrCodeSessionNotFound, sessionID)), nil
		}

		// Check if ConsoleLogs buffer exists
		if sess.ConsoleLogs == nil {
			return mcp.NewToolResultError("Console log buffer not initialized for this session"), nil
		}

		// Extract limit parameter with default
		limit := 50
		args := req.GetArguments()
		if limitVal, ok := args["limit"]; ok && limitVal != nil {
			switch v := limitVal.(type) {
			case float64:
				limit = int(v)
			case int:
				limit = v
			}
		}

		// Extract level filter parameter
		var level session.ConsoleLogLevel
		levelStr := req.GetString("level", "")
		if levelStr != "" {
			level = session.ConsoleLogLevel(levelStr)
		}

		// Retrieve console logs from session buffer
		entries := sess.ConsoleLogs.Get(limit, level)

		// Serialize entries to JSON
		jsonBytes, err := json.Marshal(entries)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize console logs: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
