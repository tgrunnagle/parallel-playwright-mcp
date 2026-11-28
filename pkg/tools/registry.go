package tools

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// AllToolNames contains the names of all available MCP tools.
// This is used for configuration validation and tool filtering, and must list
// exactly the tools Registry returns.
var AllToolNames = []string{
	// Session management
	"session_create",
	"session_list",
	"session_close",

	// Navigation
	"navigate",
	"go_back",
	"go_forward",
	"reload",

	// Interaction
	"click",
	"type",
	"fill",
	"select_option",
	"hover",
	"press_key",

	// Inspection
	"get_console_logs",
	"get_network_logs",
	"screenshot",
	"extract_text",
	"get_html",
	"evaluate",
	"query_selector",
	"get_accessibility_tree",
	"navigate_and_extract_text",
	"get_links",
}

// Registry returns every MCP tool the server offers, each paired with its
// handler. The server registers tools from this list, so it is the one place a
// new tool is added.
func Registry(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) []server.ServerTool {
	return []server.ServerTool{
		// Session management
		{Tool: SessionCreateTool(), Handler: SessionCreateHandler(mgr, timeoutConfig)},
		{Tool: SessionListTool(), Handler: SessionListHandler(mgr, timeoutConfig)},
		{Tool: SessionCloseTool(), Handler: SessionCloseHandler(mgr, timeoutConfig)},

		// Inspection
		{Tool: GetConsoleLogsTool(), Handler: GetConsoleLogsHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: GetNetworkLogsTool(), Handler: GetNetworkLogsHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: ScreenshotTool(), Handler: ScreenshotHandler(mgr, timeoutConfig)},
		{Tool: ExtractTextTool(), Handler: ExtractTextHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: GetHTMLTool(), Handler: GetHTMLHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: EvaluateTool(), Handler: EvaluateHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: QuerySelectorTool(), Handler: QuerySelectorHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: GetAccessibilityTreeTool(), Handler: GetAccessibilityTreeHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: NavigateAndExtractTextTool(), Handler: NavigateAndExtractTextHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: GetLinksTool(), Handler: GetLinksHandler(mgr, timeoutConfig, responseConfig)},

		// Navigation
		{Tool: NavigateTool(), Handler: NavigateHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: GoBackTool(), Handler: GoBackHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: GoForwardTool(), Handler: GoForwardHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: ReloadTool(), Handler: ReloadHandler(mgr, timeoutConfig, responseConfig)},

		// Interaction
		{Tool: ClickTool(), Handler: ClickHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: TypeTool(), Handler: TypeHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: FillTool(), Handler: FillHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: SelectOptionTool(), Handler: SelectOptionHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: HoverTool(), Handler: HoverHandler(mgr, timeoutConfig, responseConfig)},
		{Tool: PressKeyTool(), Handler: PressKeyHandler(mgr, timeoutConfig, responseConfig)},
	}
}
