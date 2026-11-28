// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
)

// errorResult renders any error as a tool error result through FormatErrorForTool,
// so every tool error string starts with its [<code>].
func errorResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(errors.FormatErrorForTool(err))
}

// requiredParamResult is the -32602 error for a missing required parameter.
func requiredParamResult(name string) *mcp.CallToolResult {
	return errorResult(errors.NewInvalidParamsError(name, name+" is required"))
}

// paramErrorResult is the -32602 error for a parameter with a rejected value.
func paramErrorResult(name, reason string) *mcp.CallToolResult {
	return errorResult(errors.NewInvalidParamsError(name, reason))
}

// internalErrorResult is the -32603 error for an unexpected server-side failure.
func internalErrorResult(format string, args ...any) *mcp.CallToolResult {
	return errorResult(fmt.Errorf(format, args...))
}

// noActivePageResult is the -32603 error for a session without an active page.
func noActivePageResult() *mcp.CallToolResult {
	return internalErrorResult("no active page in session")
}
