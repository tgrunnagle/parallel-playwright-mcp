package tools

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// mockSnapshotPage is a page whose AI-mode snapshot is canned and whose locators
// come from a selector-to-locator map.
type mockSnapshotPage struct {
	playwright.Page
	snapshot    string
	snapshotErr error
	lastOpts    playwright.PageAriaSnapshotOptions
	locators    map[string]*mockLocator
	located     []string
}

func (m *mockSnapshotPage) AriaSnapshot(opts ...playwright.PageAriaSnapshotOptions) (string, error) {
	if len(opts) > 0 {
		m.lastOpts = opts[0]
	}
	return m.snapshot, m.snapshotErr
}

func (m *mockSnapshotPage) Locator(selector string, _ ...playwright.PageLocatorOptions) playwright.Locator {
	m.located = append(m.located, selector)
	if loc, ok := m.locators[selector]; ok {
		return loc
	}
	return &mockLocator{}
}

// passwordLocator resolves a ref whose element is (or is not) a password input.
func passwordLocator(isPassword bool) *mockLocator {
	return &mockLocator{evaluateFunc: func(string, any) (any, error) { return isPassword, nil }}
}

func TestTextboxValue(t *testing.T) {
	tests := []struct {
		line      string
		wantRef   string
		wantValue string
		wantOK    bool
	}{
		{`  - textbox "Email" [ref=e5]: a@b.c`, "e5", "a@b.c", true},
		{`- textbox [ref=f1e2]: plain`, "f1e2", "plain", true},
		{`    - textbox "Password" [ref=e4]: "s3cret: pass"`, "e4", `"s3cret: pass"`, true},
		{`- textbox "Bio" [active] [ref=e10]: "line1 line2"`, "e10", `"line1 line2"`, true},
		{`- textbox "Name \"x\" [ref=e9]: fake" [ref=e3]: real`, "e3", "real", true},
		{`- textbox "Empty" [ref=e6]`, "", "", false},
		{`- textbox "Kids" [ref=e6]:`, "", "", false},
		{`- button "Go" [ref=e8]: Go`, "", "", false},
		{`- textboxy "x" [ref=e1]: v`, "", "", false},
		{`- textbox "unterminated [ref=e1]: v`, "", "", false},
		{`- textbox "No ref": v`, "", "", false},
	}
	for _, tt := range tests {
		ref, start, ok := textboxValue(tt.line)
		if ok != tt.wantOK || ref != tt.wantRef {
			t.Errorf("textboxValue(%q) = (%q, %d, %v), want (%q, _, %v)", tt.line, ref, start, ok, tt.wantRef, tt.wantOK)
			continue
		}
		if ok && tt.line[start:] != tt.wantValue {
			t.Errorf("textboxValue(%q) value = %q, want %q", tt.line, tt.line[start:], tt.wantValue)
		}
	}
}

func TestRedactPasswords(t *testing.T) {
	snapshot := strings.Join([]string{
		`- generic [ref=e2]:`,
		`  - textbox "Name" [ref=e3]: alice`,
		`  - textbox "Password" [ref=e4]: "s3cret: pass"`,
		`  - textbox "Unknown" [ref=e5]: maybe-secret`,
		`  - checkbox "Agree" [checked] [ref=e6]`,
	}, "\n")
	page := &mockSnapshotPage{locators: map[string]*mockLocator{
		"aria-ref=e3": passwordLocator(false),
		"aria-ref=e4": passwordLocator(true),
		"aria-ref=e5": {evaluateFunc: func(string, any) (any, error) { return nil, errors.New("detached") }},
	}}

	got := redactPasswords(page, snapshot, nil)

	want := strings.Join([]string{
		`- generic [ref=e2]:`,
		`  - textbox "Name" [ref=e3]: alice`,
		`  - textbox "Password" [ref=e4]: •••`,
		`  - textbox "Unknown" [ref=e5]: •••`,
		`  - checkbox "Agree" [checked] [ref=e6]`,
	}, "\n")
	if got != want {
		t.Errorf("redactPasswords() =\n%s\nwant\n%s", got, want)
	}
	if strings.Contains(got, "s3cret") {
		t.Error("password value leaked into the snapshot")
	}
}

func TestTakeSnapshotScopedSelector(t *testing.T) {
	t.Run("no match fails at once with -32002", func(t *testing.T) {
		page := &mockSnapshotPage{locators: map[string]*mockLocator{
			"#missing": {countFunc: func() (int, error) { return 0, nil }},
		}}
		_, err := takeSnapshot(page, "#missing", 0, nil)
		if err == nil || !strings.HasPrefix(errorResultText(err), "[-32002] Element not found (selector: #missing)") {
			t.Errorf("takeSnapshot() error = %v", err)
		}
	})

	t.Run("several matches fail with the multi-match error", func(t *testing.T) {
		page := &mockSnapshotPage{locators: map[string]*mockLocator{
			"li": {
				countFunc: func() (int, error) { return 4, nil },
				evaluateAllFunc: func(string, ...any) (any, error) {
					return []any{map[string]any{"tag": "li", "text": "One"}}, nil
				},
			},
		}}
		_, err := takeSnapshot(page, "li", 0, nil)
		text := errorResultText(err)
		if !strings.HasPrefix(text, "[-32002] Selector matched 4 elements") || !strings.Contains(text, `<li> "One"`) {
			t.Errorf("takeSnapshot() error = %q", text)
		}
	})

	t.Run("one match snapshots that element with depth", func(t *testing.T) {
		var gotOpts playwright.LocatorAriaSnapshotOptions
		page := &mockSnapshotPage{locators: map[string]*mockLocator{
			"form": {
				countFunc: func() (int, error) { return 1, nil },
				ariaSnapshotFunc: func(opts playwright.LocatorAriaSnapshotOptions) (string, error) {
					gotOpts = opts
					return `- form [ref=e2]`, nil
				},
			},
		}}
		got, err := takeSnapshot(page, "form", 2, nil)
		if err != nil || got != `- form [ref=e2]` {
			t.Fatalf("takeSnapshot() = %q, %v", got, err)
		}
		if gotOpts.Mode == nil || *gotOpts.Mode != *playwright.AriaSnapshotModeAi {
			t.Error("expected AI snapshot mode")
		}
		if gotOpts.Depth == nil || *gotOpts.Depth != 2 {
			t.Errorf("Depth = %v, want 2", gotOpts.Depth)
		}
	})
}

func TestCutRunes(t *testing.T) {
	if got := cutRunes("short", 80); got != "short" {
		t.Errorf("cutRunes() = %q", got)
	}
	if got := cutRunes(strings.Repeat("é", 100), 80); got != strings.Repeat("é", 79)+"…" {
		t.Errorf("cutRunes() = %q", got)
	}
}

// errorResultText formats err the way a tool error result would.
func errorResultText(err error) string {
	if err == nil {
		return ""
	}
	return extractTextContent(errorResult(err).Content)
}

// TestGetAccessibilityTreeHandler covers the snapshot, the response budget, and errors.
func TestGetAccessibilityTreeHandler(t *testing.T) {
	newSession := func(page playwright.Page) func(string, string) (*session.BrowserSession, bool) {
		return func(string, string) (*session.BrowserSession, bool) {
			return &session.BrowserSession{
				ID:          "sess-1",
				ActiveTabID: "tab-1",
				Pages:       map[string]playwright.Page{"tab-1": page},
			}, true
		}
	}
	call := func(t *testing.T, page playwright.Page, rc *ResponseConfig, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		handler := GetAccessibilityTreeHandler(&mockSessionManager{getSessionFunc: newSession(page)}, DefaultTimeoutConfig(), rc)
		args["sessionId"] = "sess-1"
		result, err := handler(context.Background(), mcp.CallToolRequest{
			Params: mcp.CallToolParams{Name: "get_accessibility_tree", Arguments: args},
		})
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		return result
	}

	t.Run("returns the YAML snapshot in AI mode", func(t *testing.T) {
		page := &mockSnapshotPage{snapshot: `- button "Go" [ref=e2]`}
		result := call(t, page, nil, map[string]any{})
		if result.IsError || len(result.Content) != 1 || extractTextContent(result.Content) != `- button "Go" [ref=e2]` {
			t.Fatalf("unexpected result: %+v", result.Content)
		}
		if page.lastOpts.Mode == nil || *page.lastOpts.Mode != *playwright.AriaSnapshotModeAi {
			t.Error("expected AI snapshot mode")
		}
	})

	t.Run("snapshot over budget is cut with the pagination block", func(t *testing.T) {
		line := `- listitem "Item" [ref=e1]` + "\n"
		snapshot := strings.Repeat(line, 100)
		page := &mockSnapshotPage{snapshot: snapshot}

		result := call(t, page, &ResponseConfig{MaxResponseChars: 500}, map[string]any{})
		if result.IsError || len(result.Content) != 2 {
			t.Fatalf("expected content and pagination blocks, got %+v", result.Content)
		}
		first, _ := mcp.AsTextContent(result.Content[0])
		notice, _ := mcp.AsTextContent(result.Content[1])
		if len([]rune(first.Text)) != 500 || first.Text != snapshot[:500] {
			t.Errorf("first block has %d runes, want the first 500", len([]rune(first.Text)))
		}
		total := len([]rune(snapshot))
		wantNotice := "[truncated: " + strconv.Itoa(total-500) + " more characters; call again with offset=500]"
		if notice.Text != wantNotice {
			t.Errorf("notice = %q, want %q", notice.Text, wantNotice)
		}

		next := call(t, page, &ResponseConfig{MaxResponseChars: 500}, map[string]any{"offset": float64(500)})
		if got := extractTextContent(next.Content); got != snapshot[500:1000] {
			t.Errorf("offset page = %q", got)
		}
	})

	t.Run("snapshot failure is formatted", func(t *testing.T) {
		page := &mockSnapshotPage{snapshotErr: errors.New("target closed")}
		result := call(t, page, nil, map[string]any{})
		if !result.IsError || !strings.HasPrefix(extractTextContent(result.Content), "[-32603]") {
			t.Errorf("unexpected result: %s", extractTextContent(result.Content))
		}
	})

	t.Run("invalid depth is rejected", func(t *testing.T) {
		result := call(t, &mockSnapshotPage{}, nil, map[string]any{"depth": float64(0)})
		if !result.IsError || !strings.HasPrefix(extractTextContent(result.Content), "[-32602] Invalid parameters (parameter: depth)") {
			t.Errorf("unexpected result: %s", extractTextContent(result.Content))
		}
	})
}
