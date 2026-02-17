package tools

// AllToolNames contains the names of all available MCP tools.
// This is used for configuration validation and tool filtering.
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
}
