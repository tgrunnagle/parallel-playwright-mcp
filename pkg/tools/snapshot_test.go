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
	frames      []playwright.Frame
}

func (m *mockSnapshotPage) Frames() []playwright.Frame { return m.frames }

// mockFrame is a frame whose password inputs hold values, or whose read fails.
type mockFrame struct {
	playwright.Frame
	values   []any
	err      error
	detached bool
}

func (f *mockFrame) Locator(string, ...playwright.FrameLocatorOptions) playwright.Locator {
	return &mockLocator{evaluateAllFunc: func(expression string, _ ...any) (any, error) {
		if expression != passwordValuesJS {
			return nil, errors.New("unexpected expression")
		}
		return f.values, f.err
	}}
}

func (f *mockFrame) IsDetached() bool { return f.detached }

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
	return &mockLocator{countFunc: one, evaluateFunc: func(string, any) (any, error) { return isPassword, nil }}
}

// one is a countFunc for a locator that matches one element.
func one() (int, error) { return 1, nil }

// zero is a countFunc for a locator that matches nothing.
func zero() (int, error) { return 0, nil }

func TestParseTextboxLine(t *testing.T) {
	tests := []struct {
		line      string
		wantRef   string
		wantValue string // "" with wantOK means the value is in child lines
		wantOK    bool
	}{
		{`  - textbox "Email" [ref=e5]: a@b.c`, "e5", "a@b.c", true},
		{`- textbox [ref=f1e2]: plain`, "f1e2", "plain", true},
		{`    - textbox "Password" [ref=e4]: "s3cret: pass"`, "e4", `"s3cret: pass"`, true},
		{`- textbox "Bio" [active] [ref=e10]: "line1 line2"`, "e10", `"line1 line2"`, true},
		{`- textbox "Name \"x\" [ref=e9]: fake" [ref=e3]: real`, "e3", "real", true},
		{`  - textbox "Password" [ref=e2]:`, "e2", "", true},
		{`  - 'textbox "Password: 8+ chars" [ref=e3]': "s3cret: x"`, "e3", `"s3cret: x"`, true},
		{`  - 'textbox "Pin {x}" [ref=e4]':`, "e4", "", true},
		{`  - 'textbox "It''s: secret" [ref=e7]': v`, "e7", "v", true},
		{`- textbox "Empty" [ref=e6]`, "", "", false},
		{`- button "Go" [ref=e8]: Go`, "", "", false},
		{`- 'button "a: b" [ref=e8]': Go`, "", "", false},
		{`- textboxy "x" [ref=e1]: v`, "", "", false},
		{`- textbox "unterminated [ref=e1]: v`, "", "", false},
		{`- 'textbox "unterminated [ref=e1]: v`, "", "", false},
		{`- textbox /unterminated [ref=e1]: v`, "", "", false},
		{`- textbox "No ref": v`, "", "v", true},
		{`- textbox /pw/ [ref=e3]: secret`, "e3", "secret", true},
		{`- textbox /a/b [x]/ [ref=e4]:`, "e4", "", true},
		{`- 'textbox /pw: x/ [ref=e5]': v`, "e5", "v", true},
		{`  - textbox "No ref block":`, "", "", true},
		{`- text: hunter2`, "", "", false},
	}
	for _, tt := range tests {
		tb, ok := parseTextboxLine(tt.line)
		if ok != tt.wantOK || tb.ref != tt.wantRef {
			t.Errorf("parseTextboxLine(%q) = (%+v, %v), want ref %q, ok %v", tt.line, tb, ok, tt.wantRef, tt.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if tt.wantValue == "" {
			if tb.valueStart != -1 {
				t.Errorf("parseTextboxLine(%q) valueStart = %d, want -1 for a block value", tt.line, tb.valueStart)
			}
		} else if tb.valueStart < 0 || tt.line[tb.valueStart:] != tt.wantValue {
			t.Errorf("parseTextboxLine(%q) value at %d, want %q", tt.line, tb.valueStart, tt.wantValue)
		}
		if tb.indent != lineIndent(tt.line) {
			t.Errorf("parseTextboxLine(%q) indent = %d", tt.line, tb.indent)
		}
	}
}

func TestRedactPasswords(t *testing.T) {
	// Shapes taken from the playwright-go v0.6201.1 AI-mode renderer.
	snapshot := strings.Join([]string{
		`- generic [ref=e1]:`,
		`  - textbox "Name" [ref=e3]: alice`,
		`  - textbox "Password" [ref=e4]: "s3cret: pass"`,
		`  - textbox "Unknown" [ref=e5]: maybe-secret`,
		`  - textbox "Gone" [ref=e11]: vanished-secret`,
		`  - checkbox "Agree" [checked] [ref=e6]`,
		`  - textbox "Login password" [ref=e7]:`,
		`    - /placeholder: Enter your password`,
		`    - text: hunter2`,
		`  - 'textbox "Password: 8+ chars" [ref=e8]': "colon: secret"`,
		`  - 'textbox "Pin {x}" [ref=e9]':`,
		`    - /placeholder: pin here`,
		`    - text: "- text: 1234"`,
		`  - textbox "Note" [ref=e10]:`,
		`    - /placeholder: "type: here"`,
		`    - text: visible note`,
		`  - textbox "Search" [ref=e12]:`,
		`    - /placeholder: Search`,
		`  - button "Go" [ref=e13]`,
		// AI mode leaves refs off covered or zero-size elements but prints their values.
		`  - textbox "Covered": overlay-secret`,
		`  - textbox "Covered block":`,
		`    - /placeholder: hidden`,
		`    - text: zero-size-secret`,
		`  - textbox "Count fails" [ref=e14]: count-secret`,
		`  - textbox "Not bool" [ref=e15]: not-bool-secret`,
		`  - textbox "Old ref" [ref=s1e3]: old-ref-secret`,
		`  - textbox /pw/ [ref=e16]: regex-secret`,
		`  - textbox "unterminated [ref=e17]: unparsed-secret`,
		`  - 'textbox "unterminated: x [ref=e18]':`,
		`    - text: unparsed-block-secret`,
	}, "\n")
	page := &mockSnapshotPage{locators: map[string]*mockLocator{
		"aria-ref=e3":  passwordLocator(false),
		"aria-ref=e4":  passwordLocator(true),
		"aria-ref=e5":  {countFunc: one, evaluateFunc: func(string, any) (any, error) { return nil, errors.New("detached") }},
		"aria-ref=e7":  passwordLocator(true),
		"aria-ref=e8":  passwordLocator(true),
		"aria-ref=e9":  passwordLocator(true),
		"aria-ref=e10": passwordLocator(false),
		"aria-ref=e11": {countFunc: zero},
		"aria-ref=e12": passwordLocator(true),
		"aria-ref=e14": {countFunc: func() (int, error) { return 0, errors.New("target closed") }},
		"aria-ref=e15": {countFunc: one, evaluateFunc: func(string, any) (any, error) { return "password", nil }},
		"aria-ref=e16": passwordLocator(true),
	}}

	got := redactPasswords(page, snapshot, nil)

	want := strings.Join([]string{
		`- generic [ref=e1]:`,
		`  - textbox "Name" [ref=e3]: alice`,
		`  - textbox "Password" [ref=e4]: •••`,
		`  - textbox "Unknown" [ref=e5]: •••`,
		`  - textbox "Gone" [ref=e11]: •••`,
		`  - checkbox "Agree" [checked] [ref=e6]`,
		`  - textbox "Login password" [ref=e7]:`,
		`    - /placeholder: Enter your password`,
		`    - text: •••`,
		`  - 'textbox "Password: 8+ chars" [ref=e8]': •••`,
		`  - 'textbox "Pin {x}" [ref=e9]':`,
		`    - /placeholder: pin here`,
		`    - text: •••`,
		`  - textbox "Note" [ref=e10]:`,
		`    - /placeholder: "type: here"`,
		`    - text: visible note`,
		`  - textbox "Search" [ref=e12]:`,
		`    - /placeholder: Search`,
		`  - button "Go" [ref=e13]`,
		`  - textbox "Covered": •••`,
		`  - textbox "Covered block":`,
		`    - /placeholder: hidden`,
		`    - text: •••`,
		`  - textbox "Count fails" [ref=e14]: •••`,
		`  - textbox "Not bool" [ref=e15]: •••`,
		`  - textbox "Old ref" [ref=s1e3]: •••`,
		`  - textbox /pw/ [ref=e16]: •••`,
		`  - textbox "unterminated [ref=e17]: •••`,
		`  - 'textbox "unterminated: x [ref=e18]':`,
		`    - text: •••`,
	}, "\n")
	if got != want {
		t.Errorf("redactPasswords() =\n%s\nwant\n%s", got, want)
	}
	for _, secret := range []string{"s3cret", "maybe-secret", "vanished-secret", "hunter2", "colon: secret", "1234", "overlay-secret", "zero-size-secret",
		"count-secret", "not-bool-secret", "old-ref-secret", "regex-secret", "unparsed-secret", "unparsed-block-secret"} {
		if strings.Contains(got, secret) {
			t.Errorf("password value %q leaked into the snapshot", secret)
		}
	}
	// A block textbox with only property lines has no value and is not resolved,
	// and a ref in another driver's format is never looked up.
	for _, sel := range page.located {
		if sel == "aria-ref=e12" || sel == "aria-ref=s1e3" {
			t.Errorf("resolved %s", sel)
		}
	}
}

func TestScrubValues(t *testing.T) {
	tests := []struct {
		name      string
		passwords []string
		line      string
		want      string
	}{
		{"explicit role", []string{"hunter2"}, `  - searchbox "Pw" [ref=e3]: hunter2`, `  - searchbox "Pw" [ref=e3]: •••`},
		{"combobox", []string{"hunter2"}, `- combobox "Pw" [ref=e3]: hunter2`, `- combobox "Pw" [ref=e3]: •••`},
		{"block text", []string{"hunter2"}, `    - text: hunter2`, `    - text: •••`},
		{"merged into text", []string{"hunter2"}, `- paragraph [ref=e2]: Key hunter2 end`, `- paragraph [ref=e2]: Key ••• end`},
		{"in a name", []string{"hunter 2"}, `- button "Show hunter 2" [ref=e4]`, `- button "Show •••" [ref=e4]`},
		{"collapsed whitespace", []string{"a  b\tc"}, `- button "x a b c" [ref=e4]`, `- button "x •••" [ref=e4]`},
		{"quoted value", []string{`say "hi": now`}, `- searchbox [ref=e3]: "say \"hi\": now"`, `- searchbox [ref=e3]: "•••"`},
		{"control character", []string{"a\x01b"}, `- searchbox [ref=e3]: "a\x01b"`, `- searchbox [ref=e3]: "•••"`},
		{"quoted key", []string{"it's: x"}, `- 'button "it''s: x" [ref=e4]'`, `- 'button "•••" [ref=e4]'`},
		{"short password keeps ref and marker", []string{"e"}, `  - generic "e-mail" [ref=e3]: e`, `  - g•••n•••ric "•••-mail" [ref=e3]: •••`},
		{"longest form first", []string{"ab", "abc"}, `- text: abcd`, `- text: •••d`},
		{"no passwords", nil, `- searchbox [ref=e3]: hunter2`, `- searchbox [ref=e3]: hunter2`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := []string{tt.line}
			scrubValues(lines, tt.passwords)
			if lines[0] != tt.want {
				t.Errorf("scrubValues(%q) = %q, want %q", tt.line, lines[0], tt.want)
			}
		})
	}
}

func TestPasswordValues(t *testing.T) {
	t.Run("reads every frame and skips blank values", func(t *testing.T) {
		page := &mockSnapshotPage{frames: []playwright.Frame{
			&mockFrame{values: []any{"main-secret", "   "}},
			&mockFrame{values: []any{"frame-secret"}},
			&mockFrame{err: errors.New("frame was detached"), detached: true},
		}}
		got, err := passwordValues(page)
		if err != nil || strings.Join(got, ",") != "main-secret,frame-secret" {
			t.Errorf("passwordValues() = %q, %v", got, err)
		}
	})

	t.Run("a failed read withholds the snapshot", func(t *testing.T) {
		page := &mockSnapshotPage{
			snapshot: `- searchbox [ref=e3]: hunter2`,
			frames:   []playwright.Frame{&mockFrame{err: errors.New("execution context was destroyed")}},
		}
		got, err := takeSnapshot(page, "", 0, nil)
		if err == nil || got != "" {
			t.Errorf("takeSnapshot() = %q, %v, want an error", got, err)
		}
	})

	t.Run("takeSnapshot scrubs a role-overridden password", func(t *testing.T) {
		page := &mockSnapshotPage{
			snapshot: `- searchbox "Pw" [ref=e3]: hunter2`,
			frames:   []playwright.Frame{&mockFrame{values: []any{"hunter2"}}},
		}
		got, err := takeSnapshot(page, "", 0, nil)
		if err != nil || got != `- searchbox "Pw" [ref=e3]: •••` {
			t.Errorf("takeSnapshot() = %q, %v", got, err)
		}
	})
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
