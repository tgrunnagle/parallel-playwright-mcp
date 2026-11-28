package tools

import (
	"context"
	"regexp"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// errorCodePrefix matches the "[<code>] " prefix FormatErrorForTool writes.
var errorCodePrefix = regexp.MustCompile(`^\[-\d+\] `)

// toolErrorCase drives one handler down one validation path.
type toolErrorCase struct {
	tool string
	args map[string]any
}

// allToolHandlers builds every tool handler against mgr.
func allToolHandlers(mgr session.BrowserSessionManager) map[string]server.ToolHandlerFunc {
	tc := DefaultTimeoutConfig()
	rc := DefaultResponseConfig()
	return map[string]server.ToolHandlerFunc{
		"session_create":            SessionCreateHandler(mgr, tc),
		"session_list":              SessionListHandler(mgr, tc),
		"session_close":             SessionCloseHandler(mgr, tc),
		"navigate":                  NavigateHandler(mgr, tc, rc),
		"go_back":                   GoBackHandler(mgr, tc, rc),
		"go_forward":                GoForwardHandler(mgr, tc, rc),
		"reload":                    ReloadHandler(mgr, tc, rc),
		"click":                     ClickHandler(mgr, tc, rc),
		"type":                      TypeHandler(mgr, tc, rc),
		"fill":                      FillHandler(mgr, tc, rc),
		"select_option":             SelectOptionHandler(mgr, tc, rc),
		"hover":                     HoverHandler(mgr, tc, rc),
		"press_key":                 PressKeyHandler(mgr, tc, rc),
		"get_console_logs":          GetConsoleLogsHandler(mgr, tc, rc),
		"get_network_logs":          GetNetworkLogsHandler(mgr, tc, rc),
		"screenshot":                ScreenshotHandler(mgr, tc),
		"extract_text":              ExtractTextHandler(mgr, tc, rc),
		"get_html":                  GetHTMLHandler(mgr, tc, rc),
		"evaluate":                  EvaluateHandler(mgr, tc, rc),
		"query_selector":            QuerySelectorHandler(mgr, tc, rc),
		"get_accessibility_tree":    GetAccessibilityTreeHandler(mgr, tc, rc),
		"get_links":                 GetLinksHandler(mgr, tc, rc),
		"navigate_and_extract_text": NavigateAndExtractTextHandler(mgr, tc, rc),
	}
}

// TestToolErrorsAreFormatted checks that every tool error string starts with its
// [<code>] prefix: missing parameters, invalid values, unknown sessions, and
// sessions without an active page.
func TestToolErrorsAreFormatted(t *testing.T) {
	sid := "sess-123"
	cases := []toolErrorCase{
		// Missing sessionId on every tool that takes one.
		{"session_close", map[string]any{}},
		{"navigate", map[string]any{"url": "https://example.com"}},
		{"go_back", map[string]any{}},
		{"go_forward", map[string]any{}},
		{"reload", map[string]any{}},
		{"click", map[string]any{"selector": "#b"}},
		{"type", map[string]any{"selector": "#b", "text": "x"}},
		{"fill", map[string]any{"selector": "#b", "value": "x"}},
		{"select_option", map[string]any{"selector": "#b", "value": "x"}},
		{"hover", map[string]any{"selector": "#b"}},
		{"press_key", map[string]any{"key": "Enter"}},
		{"get_console_logs", map[string]any{}},
		{"get_network_logs", map[string]any{}},
		{"screenshot", map[string]any{}},
		{"extract_text", map[string]any{}},
		{"get_html", map[string]any{}},
		{"evaluate", map[string]any{"expression": "1"}},
		{"query_selector", map[string]any{"selector": "#b"}},
		{"get_accessibility_tree", map[string]any{}},
		{"get_links", map[string]any{}},

		// Other missing or invalid parameters.
		{"navigate", map[string]any{"sessionId": sid}},
		{"type", map[string]any{"sessionId": sid, "selector": "#b"}},
		{"select_option", map[string]any{"sessionId": sid, "selector": "#b"}},
		{"press_key", map[string]any{"sessionId": sid}},
		{"evaluate", map[string]any{"sessionId": sid}},
		{"query_selector", map[string]any{"sessionId": sid}},
		{"navigate_and_extract_text", map[string]any{}},
		{"navigate_and_extract_text", map[string]any{"url": "https://example.com", "browserType": "netscape"}},
		{"session_create", map[string]any{"browserType": "netscape"}},
		{"extract_text", map[string]any{"sessionId": sid, "offset": float64(-1)}},
		{"click", map[string]any{"sessionId": sid, "selector": "#b", "ref": "e1"}},
		{"fill", map[string]any{"sessionId": sid, "ref": "css=div", "value": "x"}},
		{"press_key", map[string]any{"sessionId": sid, "key": "Enter", "selector": "#b", "ref": "e1"}},

		// Unknown session.
		{"click", map[string]any{"sessionId": "sess-unknown", "selector": "#b"}},
		{"get_html", map[string]any{"sessionId": "sess-unknown"}},

		// Session with no active page.
		{"navigate", map[string]any{"sessionId": sid, "url": "https://example.com"}},
		{"go_back", map[string]any{"sessionId": sid}},
		{"go_forward", map[string]any{"sessionId": sid}},
		{"reload", map[string]any{"sessionId": sid}},
		{"click", map[string]any{"sessionId": sid, "selector": "#b"}},
		{"type", map[string]any{"sessionId": sid, "selector": "#b", "text": "x"}},
		{"fill", map[string]any{"sessionId": sid, "selector": "#b", "value": "x"}},
		{"select_option", map[string]any{"sessionId": sid, "selector": "#b", "value": "x"}},
		{"hover", map[string]any{"sessionId": sid, "selector": "#b"}},
		{"press_key", map[string]any{"sessionId": sid, "key": "Enter"}},
		{"screenshot", map[string]any{"sessionId": sid}},
		{"extract_text", map[string]any{"sessionId": sid}},
		{"get_html", map[string]any{"sessionId": sid}},
		{"evaluate", map[string]any{"sessionId": sid, "expression": "1"}},
		{"query_selector", map[string]any{"sessionId": sid, "selector": "#b"}},
		{"get_accessibility_tree", map[string]any{"sessionId": sid}},
		{"get_links", map[string]any{"sessionId": sid}},

		// Console buffer missing on the session.
		{"get_console_logs", map[string]any{"sessionId": sid}},
	}

	mgr := &mockSessionManager{
		getSessionFunc: func(_, browserSessionID string) (*session.BrowserSession, bool) {
			if browserSessionID != sid {
				return nil, false
			}
			return &session.BrowserSession{
				ID:          sid,
				ActiveTabID: "tab-1",
				Pages:       map[string]playwright.Page{},
			}, true
		},
	}
	handlers := allToolHandlers(mgr)

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			handler, ok := handlers[tc.tool]
			if !ok {
				t.Fatalf("no handler for %s", tc.tool)
			}
			result, err := handler(context.Background(), mcp.CallToolRequest{
				Params: mcp.CallToolParams{Name: tc.tool, Arguments: tc.args},
			})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result for args %v", tc.args)
			}
			text := extractTextContent(result.Content)
			if !errorCodePrefix.MatchString(text) {
				t.Errorf("error for args %v does not start with [<code>]: %q", tc.args, text)
			}
		})
	}

	// Every registered tool is covered by at least one case.
	covered := map[string]bool{}
	for _, tc := range cases {
		covered[tc.tool] = true
	}
	for _, name := range AllToolNames {
		if _, ok := handlers[name]; !ok {
			t.Errorf("tool %s has no handler in allToolHandlers", name)
		}
		if !covered[name] && name != "session_list" {
			t.Errorf("tool %s has no error case", name)
		}
	}
}
