package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// actionTool is one action tool with arguments for a call that succeeds.
type actionTool struct {
	name    string
	handler func(session.BrowserSessionManager, *TimeoutConfig, *ResponseConfig) server.ToolHandlerFunc
	args    map[string]any
	// navigation is true for the navigation tools, which run on a mockPage.
	navigation bool
}

// allActionTools lists the ten tools that return a state summary.
var allActionTools = []actionTool{
	{"click", ClickHandler, map[string]any{"selector": "#go"}, false},
	{"type", TypeHandler, map[string]any{"selector": "#in", "text": "hi"}, false},
	{"fill", FillHandler, map[string]any{"selector": "#in", "value": "hi"}, false},
	{"select_option", SelectOptionHandler, map[string]any{"selector": "#sel", "value": "a"}, false},
	{"hover", HoverHandler, map[string]any{"selector": "#go"}, false},
	{"press_key", PressKeyHandler, map[string]any{"selector": "#in", "key": "Enter"}, false},
	{"navigate", NavigateHandler, map[string]any{"url": "https://example.com/next"}, true},
	{"go_back", GoBackHandler, map[string]any{}, true},
	{"go_forward", GoForwardHandler, map[string]any{}, true},
	{"reload", ReloadHandler, map[string]any{}, true},
}

// snapshotPage is a mockPage whose snapshot is canned, for navigation tools
// called with includeSnapshot.
type snapshotPage struct {
	*mockPage
	snapshot string
}

func (p *snapshotPage) AriaSnapshot(...playwright.PageAriaSnapshotOptions) (string, error) {
	return p.snapshot, nil
}

func (p *snapshotPage) Frames() []playwright.Frame { return nil }

// runActionTool calls tool on a page whose snapshot is snapshot and whose
// navigations return a response with status. extra is merged into its arguments.
func runActionTool(t *testing.T, tool actionTool, rc *ResponseConfig, snapshot string, status int, extra map[string]any) *mcp.CallToolResult {
	t.Helper()
	resp := &mockResponse{statusFunc: func() int { return status }}
	var page playwright.Page
	if tool.navigation {
		page = &snapshotPage{snapshot: snapshot, mockPage: &mockPage{
			gotoFunc: func(string, playwright.PageGotoOptions) (playwright.Response, error) { return resp, nil },
			goBackFunc: func(playwright.PageGoBackOptions) (playwright.Response, error) {
				return resp, nil
			},
			goForwardFunc: func(playwright.PageGoForwardOptions) (playwright.Response, error) {
				return resp, nil
			},
			reloadFunc: func(playwright.PageReloadOptions) (playwright.Response, error) { return resp, nil },
		}}
	} else {
		page = &mockInteractionPage{
			locatorFunc: func(string, ...playwright.PageLocatorOptions) playwright.Locator { return &mockLocator{} },
			snapshotFunc: func(playwright.PageAriaSnapshotOptions) (string, error) {
				return snapshot, nil
			},
		}
	}
	sess := &session.BrowserSession{ID: "sess-123", ActiveTabID: "tab-1", Pages: map[string]playwright.Page{"tab-1": page}}
	mgr := &mockSessionManager{getSessionFunc: func(string, string) (*session.BrowserSession, bool) { return sess, true }}

	args := map[string]any{"sessionId": "sess-123"}
	for k, v := range tool.args {
		args[k] = v
	}
	for k, v := range extra {
		args[k] = v
	}
	result, err := tool.handler(mgr, DefaultTimeoutConfig(), rc)(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: tool.name, Arguments: args},
	})
	if err != nil || result == nil || result.IsError {
		t.Fatalf("%s failed: %v %s", tool.name, err, extractTextContent(result.Content))
	}
	return result
}

// summaryOf parses the state summary that follows the first line of a result.
func summaryOf(t *testing.T, result *mcp.CallToolResult) stateSummary {
	t.Helper()
	_, summaryJSON, ok := strings.Cut(extractTextContent(result.Content), "\n")
	if !ok {
		t.Fatalf("no summary after the action line: %q", extractTextContent(result.Content))
	}
	var summary stateSummary
	if err := json.Unmarshal([]byte(summaryJSON), &summary); err != nil {
		t.Fatalf("summary is not JSON: %v\n%s", err, summaryJSON)
	}
	return summary
}

// blockText returns the text of content block i.
func blockText(t *testing.T, result *mcp.CallToolResult, i int) string {
	t.Helper()
	if len(result.Content) <= i {
		t.Fatalf("got %d content blocks, want more than %d", len(result.Content), i)
	}
	text, ok := mcp.AsTextContent(result.Content[i])
	if !ok {
		t.Fatalf("block %d is not text", i)
	}
	return text.Text
}

func TestNavigationToolsReportStatus(t *testing.T) {
	for _, tool := range allActionTools {
		if !tool.navigation {
			continue
		}
		t.Run(tool.name, func(t *testing.T) {
			summary := summaryOf(t, runActionTool(t, tool, nil, "", 404, nil))
			if summary.Action != tool.name || !summary.Navigated {
				t.Errorf("summary = %+v, want action %s and navigated", summary, tool.name)
			}
			if summary.HTTPStatus == nil || *summary.HTTPStatus != 404 {
				t.Errorf("httpStatus = %v, want 404", summary.HTTPStatus)
			}
		})
	}
}

func TestActionToolsIncludeSnapshot(t *testing.T) {
	const snapshot = `- button "Go" [ref=e2]`
	for _, tool := range allActionTools {
		t.Run(tool.name, func(t *testing.T) {
			if got := runActionTool(t, tool, nil, snapshot, 200, nil); len(got.Content) != 1 {
				t.Errorf("without includeSnapshot: %d blocks, want 1", len(got.Content))
			}
			got := runActionTool(t, tool, nil, snapshot, 200, map[string]any{"includeSnapshot": true})
			if len(got.Content) != 2 || blockText(t, got, 1) != snapshot {
				t.Errorf("with includeSnapshot: %d blocks, want the summary then the snapshot", len(got.Content))
			}
		})
	}

	click := allActionTools[0]
	withSnapshot := map[string]any{"includeSnapshot": true}

	t.Run("empty snapshot", func(t *testing.T) {
		got := runActionTool(t, click, nil, " \n", 200, withSnapshot)
		if len(got.Content) != 2 || blockText(t, got, 1) != "No accessible elements found." {
			t.Errorf("got %d blocks, second %q", len(got.Content), blockText(t, got, 1))
		}
	})

	t.Run("snapshot exactly at the budget is not cut", func(t *testing.T) {
		exact := strings.Repeat("é", 50)
		got := runActionTool(t, click, &ResponseConfig{MaxResponseChars: 50}, exact, 200, withSnapshot)
		if len(got.Content) != 2 || blockText(t, got, 1) != exact {
			t.Errorf("got %d blocks, want the whole snapshot and no notice", len(got.Content))
		}
	})

	t.Run("multibyte snapshot over the budget is cut by characters", func(t *testing.T) {
		long := strings.Repeat("é", 60)
		got := runActionTool(t, click, &ResponseConfig{MaxResponseChars: 50}, long, 200, withSnapshot)
		if len(got.Content) != 3 {
			t.Fatalf("got %d blocks, want summary, snapshot and notice", len(got.Content))
		}
		if cut := blockText(t, got, 1); cut != strings.Repeat("é", 50) {
			t.Errorf("snapshot block has %d bytes, want 50 characters", len(cut))
		}
		want := "[truncated: " + strconv.Itoa(10) + " more characters; call get_accessibility_tree with offset=50]"
		if notice := blockText(t, got, 2); notice != want {
			t.Errorf("notice = %q, want %q", notice, want)
		}
	})
}
