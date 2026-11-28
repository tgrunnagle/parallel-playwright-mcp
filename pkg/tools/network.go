// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// networkLogOutput represents a network log entry formatted for JSON output.
// Duration is converted to milliseconds for easier consumption by clients.
type networkLogOutput struct {
	Timestamp    string `json:"timestamp"`
	Method       string `json:"method"`
	URL          string `json:"url"`
	Status       int    `json:"status"`
	DurationMs   int64  `json:"durationMs"`
	RequestSize  int64  `json:"requestSize"`
	ResponseSize int64  `json:"responseSize"`
	ResourceType string `json:"resourceType"`
}

// GetNetworkLogsTool returns the get_network_logs MCP tool definition.
func GetNetworkLogsTool() mcp.Tool {
	return mcp.NewTool("get_network_logs",
		readOnlyLocalHints.option(),
		mcp.WithDescription("Retrieve captured network request/response logs from a browser session"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID to retrieve logs from"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of entries to return (default: 50)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Number of newest matching entries to skip (default: 0). Entries are selected newest first and returned oldest first; the buffer is live, so offsets and total shift as new entries arrive. Continue with offset + returned from the second content block while hasMore is true; fewer than limit items may be returned when the response budget is reached"),
		),
		mcp.WithObject("filter",
			mcp.Description("Filter criteria for network logs"),
			mcp.Properties(map[string]any{
				"urlPattern": map[string]any{
					"type":        "string",
					"description": "Regex pattern to match request URLs",
				},
				"status": map[string]any{
					"type":        "number",
					"description": "Filter by exact HTTP status code",
				},
				"statusMin": map[string]any{
					"type":        "number",
					"description": "Minimum HTTP status code (inclusive)",
				},
				"statusMax": map[string]any{
					"type":        "number",
					"description": "Maximum HTTP status code (inclusive)",
				},
			}),
		),
	)
}

// GetNetworkLogsHandler returns the handler function for get_network_logs.
func GetNetworkLogsHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "get_network_logs")

		// Apply timeout with default category (log retrieval is a quick operation)
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutDefault, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "get_network_logs")
			return requiredParamResult("sessionId"), nil
		}

		limit, offset, err := listPageParams(args)
		if err != nil {
			return errorResult(err), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "get_network_logs", "sessionID", sessionID)
			return errorResult(errors.NewSessionNotFoundError(sessionID)), nil
		}

		// Retrieve all entries; Entries() returns chronological order (oldest first)
		entries := []session.NetworkLogEntry{}
		if sess.NetworkLogs != nil {
			entries = sess.NetworkLogs.Entries(0)
		}

		// Select newest first: offset k skips the k newest matching entries
		entries = reverseNetworkEntries(entries)

		// Parse and apply filter if provided
		if filterVal, ok := args["filter"].(map[string]any); ok && len(filterVal) > 0 {
			filtered, err := applyNetworkFilters(entries, filterVal)
			if err != nil {
				slog.Error("invalid network log filter", "tool", "get_network_logs", "sessionID", sessionID, "error", err)
				return paramErrorResult("filter", fmt.Sprintf("Invalid filter: %v", err)), nil
			}
			entries = filtered
		}

		// Convert to output format
		output := make([]networkLogOutput, len(entries))
		for i, entry := range entries {
			output[i] = networkLogOutput{
				Timestamp:    entry.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
				Method:       entry.Method,
				URL:          entry.URL,
				Status:       entry.Status,
				DurationMs:   entry.Duration.Milliseconds(),
				RequestSize:  entry.RequestSize,
				ResponseSize: entry.ResponseSize,
				ResourceType: entry.ResourceType,
			}
		}

		// Page newest first, returned oldest first
		result, err := paginateItems(output, offset, limit, responseConfig.budget(), true)
		if err != nil {
			slog.Error("failed to format network logs", "tool", "get_network_logs", "sessionID", sessionID, "error", err)
			return internalErrorResult("Failed to format logs: %v", err), nil
		}

		slog.Debug("network logs retrieved", "tool", "get_network_logs", "sessionID", sessionID, "matchingEntries", len(output))
		return result, nil
	}
}

// reverseNetworkEntries returns a new slice with entries in reverse order.
func reverseNetworkEntries(entries []session.NetworkLogEntry) []session.NetworkLogEntry {
	n := len(entries)
	if n == 0 {
		return entries
	}
	reversed := make([]session.NetworkLogEntry, n)
	for i, entry := range entries {
		reversed[n-1-i] = entry
	}
	return reversed
}

// applyNetworkFilters filters entries based on the provided filter criteria.
// Returns an error if the urlPattern is an invalid regex.
func applyNetworkFilters(entries []session.NetworkLogEntry, filter map[string]any) ([]session.NetworkLogEntry, error) {
	// Compile URL pattern regex if provided
	var urlRegex *regexp.Regexp
	if pattern, ok := filter["urlPattern"].(string); ok && pattern != "" {
		var err error
		urlRegex, err = regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid urlPattern regex: %w", err)
		}
	}

	// Parse status filters
	var exactStatus *int
	var statusMin *int
	var statusMax *int

	if status, ok := filter["status"].(float64); ok {
		s := int(status)
		exactStatus = &s
	}
	if min, ok := filter["statusMin"].(float64); ok {
		m := int(min)
		statusMin = &m
	}
	if max, ok := filter["statusMax"].(float64); ok {
		m := int(max)
		statusMax = &m
	}

	// Filter entries
	var filtered []session.NetworkLogEntry
	for _, entry := range entries {
		// Apply URL pattern filter
		if urlRegex != nil && !urlRegex.MatchString(entry.URL) {
			continue
		}

		// Apply exact status filter
		if exactStatus != nil && entry.Status != *exactStatus {
			continue
		}

		// Apply status range filters
		if statusMin != nil && entry.Status < *statusMin {
			continue
		}
		if statusMax != nil && entry.Status > *statusMax {
			continue
		}

		filtered = append(filtered, entry)
	}

	// Return empty slice instead of nil for consistent JSON output
	if filtered == nil {
		filtered = []session.NetworkLogEntry{}
	}

	return filtered, nil
}
