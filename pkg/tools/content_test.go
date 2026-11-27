package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// textBlocks returns the text of every text content block in a result.
func textBlocks(t *testing.T, result *mcp.CallToolResult) []string {
	t.Helper()
	var out []string
	for _, c := range result.Content {
		tc, ok := mcp.AsTextContent(c)
		if !ok {
			t.Fatalf("content block %T is not text", c)
		}
		out = append(out, tc.Text)
	}
	return out
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		offset     int
		maxChars   int
		wantPage   string
		wantNotice string
	}{
		{
			name:     "within budget is unchanged with no notice",
			content:  "hello world",
			maxChars: 100,
			wantPage: "hello world",
		},
		{
			name:     "exactly at budget is unchanged",
			content:  "hello",
			maxChars: 5,
			wantPage: "hello",
		},
		{
			name:     "empty content is unchanged",
			content:  "",
			maxChars: 5,
			wantPage: "",
		},
		{
			name:       "over budget is cut with continuation notice",
			content:    "abcdefghij",
			maxChars:   4,
			wantPage:   "abcd",
			wantNotice: "[truncated: 6 more characters; call again with offset=4]",
		},
		{
			name:       "middle slice",
			content:    "abcdefghij",
			offset:     4,
			maxChars:   4,
			wantPage:   "efgh",
			wantNotice: "[truncated: 2 more characters; call again with offset=8]",
		},
		{
			name:       "last slice reports end of content",
			content:    "abcdefghij",
			offset:     8,
			maxChars:   4,
			wantPage:   "ij",
			wantNotice: "[end of content: 10 characters]",
		},
		{
			name:       "offset at end returns empty page",
			content:    "abcdefghij",
			offset:     10,
			maxChars:   4,
			wantPage:   "",
			wantNotice: "[end of content: 10 characters]",
		},
		{
			name:       "offset past end returns empty page",
			content:    "abcdefghij",
			offset:     50,
			maxChars:   4,
			wantPage:   "",
			wantNotice: "[end of content: 10 characters]",
		},
		{
			name:       "counts runes, not bytes",
			content:    "héllo wörld ✓",
			maxChars:   5,
			wantPage:   "héllo",
			wantNotice: "[truncated: 8 more characters; call again with offset=5]",
		},
		{
			name:       "rune offset into multibyte text",
			content:    "日本語のテキスト",
			offset:     3,
			maxChars:   2,
			wantPage:   "のテ",
			wantNotice: "[truncated: 3 more characters; call again with offset=5]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, notice := truncateContent(tt.content, tt.offset, tt.maxChars)
			if page != tt.wantPage {
				t.Errorf("page = %q, want %q", page, tt.wantPage)
			}
			if notice != tt.wantNotice {
				t.Errorf("notice = %q, want %q", notice, tt.wantNotice)
			}

			blocks := textBlocks(t, textPageResult(tt.content, tt.offset, tt.maxChars))
			if tt.wantNotice == "" {
				if len(blocks) != 1 {
					t.Fatalf("got %d blocks, want 1", len(blocks))
				}
			} else if len(blocks) != 2 || blocks[1] != tt.wantNotice {
				t.Fatalf("blocks = %q, want page and notice %q", blocks, tt.wantNotice)
			}
		})
	}

	t.Run("successive slices reassemble the content", func(t *testing.T) {
		content := strings.Repeat("áb✓", 37)
		var got strings.Builder
		offset := 0
		for i := 0; i < 100; i++ {
			page, notice := truncateContent(content, offset, 10)
			got.WriteString(page)
			if strings.HasPrefix(notice, "[end of content") {
				break
			}
			offset += utf8.RuneCountInString(page)
		}
		if got.String() != content {
			t.Errorf("reassembled content differs:\n%q\n%q", got.String(), content)
		}
	})
}

type pageItem struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

func makePageItems(n int) []pageItem {
	items := make([]pageItem, n)
	for i := range items {
		items[i] = pageItem{ID: i, Text: "item"}
	}
	return items
}

// decodePage decodes the item array and metadata of a paginateItems result.
func decodePage(t *testing.T, result *mcp.CallToolResult) ([]pageItem, listPageMeta) {
	t.Helper()
	blocks := textBlocks(t, result)
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	var items []pageItem
	if err := json.Unmarshal([]byte(blocks[0]), &items); err != nil {
		t.Fatalf("first block is not a JSON array: %v\n%s", err, blocks[0])
	}
	var meta listPageMeta
	if err := json.Unmarshal([]byte(blocks[1]), &meta); err != nil {
		t.Fatalf("second block is not metadata: %v\n%s", err, blocks[1])
	}
	return items, meta
}

func TestPaginateItems(t *testing.T) {
	t.Run("first page with more", func(t *testing.T) {
		result, err := paginateItems(makePageItems(120), 0, 50, DefaultMaxResponseChars, false)
		if err != nil {
			t.Fatal(err)
		}
		items, meta := decodePage(t, result)
		if len(items) != 50 || items[0].ID != 0 || items[49].ID != 49 {
			t.Errorf("got %d items from %d", len(items), items[0].ID)
		}
		want := listPageMeta{Total: 120, Offset: 0, Returned: 50, HasMore: true}
		if meta != want {
			t.Errorf("meta = %+v, want %+v", meta, want)
		}
	})

	t.Run("last page", func(t *testing.T) {
		result, err := paginateItems(makePageItems(120), 100, 50, DefaultMaxResponseChars, false)
		if err != nil {
			t.Fatal(err)
		}
		items, meta := decodePage(t, result)
		if len(items) != 20 || items[0].ID != 100 {
			t.Errorf("got %d items", len(items))
		}
		want := listPageMeta{Total: 120, Offset: 100, Returned: 20, HasMore: false}
		if meta != want {
			t.Errorf("meta = %+v, want %+v", meta, want)
		}
	})

	t.Run("offset past end is empty", func(t *testing.T) {
		result, err := paginateItems(makePageItems(3), 10, 50, DefaultMaxResponseChars, false)
		if err != nil {
			t.Fatal(err)
		}
		blocks := textBlocks(t, result)
		if blocks[0] != "[]" {
			t.Errorf("first block = %q, want []", blocks[0])
		}
		_, meta := decodePage(t, result)
		if meta != (listPageMeta{Total: 3, Offset: 10, Returned: 0, HasMore: false}) {
			t.Errorf("meta = %+v", meta)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		result, err := paginateItems([]pageItem{}, 0, 50, DefaultMaxResponseChars, false)
		if err != nil {
			t.Fatal(err)
		}
		if blocks := textBlocks(t, result); blocks[0] != "[]" {
			t.Errorf("first block = %q, want []", blocks[0])
		}
	})

	t.Run("reverse emits page in reverse selection order", func(t *testing.T) {
		result, err := paginateItems(makePageItems(5), 1, 3, DefaultMaxResponseChars, true)
		if err != nil {
			t.Fatal(err)
		}
		items, _ := decodePage(t, result)
		if len(items) != 3 || items[0].ID != 3 || items[2].ID != 1 {
			t.Errorf("items = %+v, want ids 3,2,1", items)
		}
	})

	t.Run("drops trailing whole items over budget", func(t *testing.T) {
		items := makePageItems(10)
		one, _ := json.Marshal(items[0]) // {"id":0,"text":"item"}
		budget := 2 + 3*len(one) + 2     // room for 3 items and 2 commas
		result, err := paginateItems(items, 0, 10, budget, false)
		if err != nil {
			t.Fatal(err)
		}
		got, meta := decodePage(t, result)
		if len(got) != 3 || meta.Returned != 3 || !meta.HasMore {
			t.Errorf("got %d items, meta %+v; want 3 with more", len(got), meta)
		}
		if n := utf8.RuneCountInString(textBlocks(t, result)[0]); n > budget {
			t.Errorf("first block is %d runes, budget %d", n, budget)
		}
	})

	t.Run("reverse keeps the items nearest the offset when dropping", func(t *testing.T) {
		items := makePageItems(10)
		one, _ := json.Marshal(items[0])
		budget := 2 + 2*len(one) + 1
		result, err := paginateItems(items, 0, 10, budget, true)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := decodePage(t, result)
		if len(got) != 2 || got[0].ID != 1 || got[1].ID != 0 {
			t.Errorf("items = %+v, want ids 1,0", got)
		}
	})

	t.Run("oversized item is returned alone with strings cut", func(t *testing.T) {
		items := []pageItem{
			{ID: 0, Text: strings.Repeat("x", 500)},
			{ID: 1, Text: "next"},
		}
		budget := 100
		result, err := paginateItems(items, 0, 50, budget, false)
		if err != nil {
			t.Fatal(err)
		}
		got, meta := decodePage(t, result)
		if meta.Returned != 1 || len(got) != 1 || got[0].ID != 0 {
			t.Fatalf("got %+v meta %+v, want only item 0", got, meta)
		}
		if !meta.HasMore {
			t.Error("hasMore = false, want true")
		}
		if !strings.HasSuffix(got[0].Text, "…") || len(got[0].Text) >= 500 {
			t.Errorf("text was not cut: %d bytes", len(got[0].Text))
		}
		if n := utf8.RuneCountInString(textBlocks(t, result)[0]); n > budget {
			t.Errorf("first block is %d runes, budget %d", n, budget)
		}

		// The next offset reaches the following item.
		next, err := paginateItems(items, meta.Offset+meta.Returned, 50, budget, false)
		if err != nil {
			t.Fatal(err)
		}
		got, meta = decodePage(t, next)
		if len(got) != 1 || got[0].ID != 1 || meta.HasMore {
			t.Errorf("next page = %+v meta %+v, want item 1 and no more", got, meta)
		}
	})

	t.Run("oversized item with escaped and multibyte strings fits", func(t *testing.T) {
		items := []map[string]any{{
			"a": strings.Repeat("<é>", 300),
			"b": strings.Repeat("✓", 300),
			"n": 12345,
		}}
		budget := 120
		result, err := paginateItems(items, 0, 50, budget, false)
		if err != nil {
			t.Fatal(err)
		}
		first := textBlocks(t, result)[0]
		if n := utf8.RuneCountInString(first); n > budget {
			t.Errorf("first block is %d runes, budget %d: %s", n, budget, first)
		}
		var decoded []map[string]any
		if err := json.Unmarshal([]byte(first), &decoded); err != nil {
			t.Fatalf("cut item is not valid JSON: %v", err)
		}
		if decoded[0]["n"] != float64(12345) {
			t.Errorf("non-string field changed: %v", decoded[0]["n"])
		}
	})

	t.Run("escaped string keeps the prefix that fits", func(t *testing.T) {
		// Each "<" serializes as a six-rune escape, so the item is about 6000 runes.
		items := []map[string]string{{"t": strings.Repeat("<", 1000)}}
		budget := 4000
		result, err := paginateItems(items, 0, 50, budget, false)
		if err != nil {
			t.Fatal(err)
		}
		first := textBlocks(t, result)[0]
		if n := utf8.RuneCountInString(first); n > budget {
			t.Errorf("first block is %d runes, budget %d", n, budget)
		}
		var decoded []map[string]string
		if err := json.Unmarshal([]byte(first), &decoded); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		cut := decoded[0]["t"]
		if !strings.HasSuffix(cut, "…") || utf8.RuneCountInString(cut) < 600 {
			t.Errorf("cut field = %d runes, want a prefix of at least 600 ending in …", utf8.RuneCountInString(cut))
		}
	})

	t.Run("item that does not fit with strings emptied becomes a placeholder", func(t *testing.T) {
		attrs := make(map[string]string, 5000)
		for i := 0; i < 5000; i++ {
			attrs[fmt.Sprintf("data-attribute-%04d", i)] = "v"
		}
		items := []map[string]any{{"tag": "div", "attributes": attrs}}
		budget := 500
		start := time.Now()
		result, err := paginateItems(items, 0, 50, budget, false)
		if err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("cutting took %v", elapsed)
		}
		first := textBlocks(t, result)[0]
		if n := utf8.RuneCountInString(first); n > budget {
			t.Errorf("first block is %d runes, budget %d", n, budget)
		}
		if first != "["+truncatedItem+"]" {
			t.Errorf("first block = %s, want the truncated placeholder", first)
		}
	})

	t.Run("oversized bare string item is cut", func(t *testing.T) {
		items := []string{strings.Repeat("z", 200)}
		result, err := paginateItems(items, 0, 50, 50, false)
		if err != nil {
			t.Fatal(err)
		}
		first := textBlocks(t, result)[0]
		var decoded []string
		if err := json.Unmarshal([]byte(first), &decoded); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if utf8.RuneCountInString(first) > 50 || len(decoded) != 1 {
			t.Errorf("first block = %q", first)
		}
	})
}

func TestPageParams(t *testing.T) {
	t.Run("text defaults to budget and offset 0", func(t *testing.T) {
		maxChars, offset, err := textPageParams(map[string]any{}, 1000)
		if err != nil || maxChars != 1000 || offset != 0 {
			t.Errorf("got %d, %d, %v", maxChars, offset, err)
		}
	})

	t.Run("maxChars above budget is clamped", func(t *testing.T) {
		maxChars, _, err := textPageParams(map[string]any{"maxChars": float64(5000)}, 1000)
		if err != nil || maxChars != 1000 {
			t.Errorf("got %d, %v", maxChars, err)
		}
	})

	t.Run("list defaults to 50 and offset 0", func(t *testing.T) {
		limit, offset, err := listPageParams(map[string]any{})
		if err != nil || limit != 50 || offset != 0 {
			t.Errorf("got %d, %d, %v", limit, offset, err)
		}
	})

	invalid := []struct {
		name  string
		args  map[string]any
		param string
		list  bool
	}{
		{"negative maxChars", map[string]any{"maxChars": float64(-1)}, "maxChars", false},
		{"zero maxChars", map[string]any{"maxChars": float64(0)}, "maxChars", false},
		{"fractional maxChars", map[string]any{"maxChars": 1.5}, "maxChars", false},
		{"string maxChars", map[string]any{"maxChars": "10"}, "maxChars", false},
		{"negative offset", map[string]any{"offset": float64(-3)}, "offset", false},
		{"negative list offset", map[string]any{"offset": float64(-3)}, "offset", true},
		{"zero limit", map[string]any{"limit": float64(0)}, "limit", true},
		{"negative limit", map[string]any{"limit": float64(-10)}, "limit", true},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.list {
				_, _, err = listPageParams(tt.args)
			} else {
				_, _, err = textPageParams(tt.args, 1000)
			}
			if err == nil {
				t.Fatal("expected error")
			}
			text := extractTextContent(invalidParamsResult(err).Content)
			if !strings.HasPrefix(text, "[-32602]") || !strings.Contains(text, "(parameter: "+tt.param+")") {
				t.Errorf("error = %q, want [-32602] naming %s", text, tt.param)
			}
		})
	}
}

// mockContentPage is a playwright.Page whose Evaluate returns canned values. Any
// other method panics through the nil embedded interface, so a test fails loudly
// if a handler touches the page in an unexpected way.
type mockContentPage struct {
	playwright.Page
	evaluateFunc func(expression string, arg ...any) (any, error)
	evaluated    int
}

func (m *mockContentPage) Evaluate(expression string, arg ...any) (any, error) {
	m.evaluated++
	if m.evaluateFunc == nil {
		panic("mockContentPage.Evaluate called without evaluateFunc")
	}
	return m.evaluateFunc(expression, arg...)
}

// sessionWithPage returns a session manager whose only session has page active.
func sessionWithPage(page playwright.Page) *mockSessionManager {
	return &mockSessionManager{
		getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
			return &session.BrowserSession{
				ID:          "sess-test",
				Pages:       map[string]playwright.Page{"tab-1": page},
				ActiveTabID: "tab-1",
			}, true
		},
	}
}

// callTool invokes a handler with the given arguments and returns its result.
func callTool(t *testing.T, handler server.ToolHandlerFunc, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: name, Arguments: args},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result == nil {
		t.Fatal("handler returned nil result")
	}
	return result
}

// testExtractTextPagination covers maxChars/offset validation, clamping, and
// slicing for extract_text. It runs as a subtest of TestExtractTextHandler.
func testExtractTextPagination(t *testing.T) {
	longText := strings.Repeat("0123456789", 30) // 300 runes

	for _, tc := range []struct {
		name  string
		args  map[string]any
		param string
	}{
		{"negative maxChars", map[string]any{"maxChars": float64(-1)}, "maxChars"},
		{"negative offset", map[string]any{"offset": float64(-5)}, "offset"},
	} {
		t.Run(tc.name+" is rejected before the page is touched", func(t *testing.T) {
			page := &mockContentPage{}
			getSessionCalled := false
			mgr := &mockSessionManager{
				getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
					getSessionCalled = true
					return &session.BrowserSession{
						ID:          "sess-test",
						Pages:       map[string]playwright.Page{"tab-1": page},
						ActiveTabID: "tab-1",
					}, true
				},
			}
			args := map[string]any{"sessionId": "sess-test"}
			for k, v := range tc.args {
				args[k] = v
			}
			result := callTool(t, ExtractTextHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig()), "extract_text", args)
			if !result.IsError {
				t.Fatal("expected error result")
			}
			text := extractTextContent(result.Content)
			if !strings.HasPrefix(text, "[-32602]") || !strings.Contains(text, tc.param) {
				t.Errorf("error = %q, want [-32602] naming %s", text, tc.param)
			}
			if page.evaluated != 0 || getSessionCalled {
				t.Error("page or session was touched before validation")
			}
		})
	}

	t.Run("maxChars above the budget is clamped to the budget", func(t *testing.T) {
		page := &mockContentPage{evaluateFunc: func(string, ...any) (any, error) { return longText, nil }}
		handler := ExtractTextHandler(sessionWithPage(page), DefaultTimeoutConfig(), &ResponseConfig{MaxResponseChars: 100})
		result := callTool(t, handler, "extract_text", map[string]any{"sessionId": "sess-test", "maxChars": float64(5000)})
		blocks := textBlocks(t, result)
		if len(blocks) != 2 {
			t.Fatalf("got %d blocks, want 2", len(blocks))
		}
		if n := utf8.RuneCountInString(blocks[0]); n != 100 {
			t.Errorf("first block is %d runes, want 100 (the budget)", n)
		}
		if blocks[1] != "[truncated: 200 more characters; call again with offset=100]" {
			t.Errorf("notice = %q", blocks[1])
		}
	})

	t.Run("offset returns the next slice", func(t *testing.T) {
		page := &mockContentPage{evaluateFunc: func(string, ...any) (any, error) { return longText, nil }}
		handler := ExtractTextHandler(sessionWithPage(page), DefaultTimeoutConfig(), DefaultResponseConfig())
		result := callTool(t, handler, "extract_text", map[string]any{"sessionId": "sess-test", "maxChars": float64(100), "offset": float64(250)})
		blocks := textBlocks(t, result)
		if blocks[0] != longText[250:] || blocks[1] != "[end of content: 300 characters]" {
			t.Errorf("blocks = %q", blocks)
		}
	})

	t.Run("text within budget is one block", func(t *testing.T) {
		page := &mockContentPage{evaluateFunc: func(string, ...any) (any, error) { return "short text", nil }}
		handler := ExtractTextHandler(sessionWithPage(page), DefaultTimeoutConfig(), nil)
		result := callTool(t, handler, "extract_text", map[string]any{"sessionId": "sess-test"})
		if blocks := textBlocks(t, result); len(blocks) != 1 || blocks[0] != "short text" {
			t.Errorf("blocks = %q", blocks)
		}
	})
}

// testEvaluateTruncation covers maxChars for evaluate. It runs as a subtest of
// TestEvaluateHandler.
func testEvaluateTruncation(t *testing.T) {
	large := strings.Repeat("x", 5000)

	t.Run("over budget is cut with incomplete-JSON notice", func(t *testing.T) {
		page := &mockContentPage{evaluateFunc: func(string, ...any) (any, error) { return large, nil }}
		handler := EvaluateHandler(sessionWithPage(page), DefaultTimeoutConfig(), &ResponseConfig{MaxResponseChars: 1000})
		result := callTool(t, handler, "evaluate", map[string]any{"sessionId": "sess-test", "expression": "big()"})
		blocks := textBlocks(t, result)
		if len(blocks) != 2 {
			t.Fatalf("got %d blocks, want 2", len(blocks))
		}
		if n := utf8.RuneCountInString(blocks[0]); n != 1000 {
			t.Errorf("first block is %d runes, want 1000", n)
		}
		if blocks[1] != "[truncated: result JSON is incomplete; narrow the expression]" {
			t.Errorf("notice = %q", blocks[1])
		}
	})

	t.Run("maxChars narrows the cut", func(t *testing.T) {
		page := &mockContentPage{evaluateFunc: func(string, ...any) (any, error) { return large, nil }}
		handler := EvaluateHandler(sessionWithPage(page), DefaultTimeoutConfig(), DefaultResponseConfig())
		result := callTool(t, handler, "evaluate", map[string]any{"sessionId": "sess-test", "expression": "big()", "maxChars": float64(10)})
		if blocks := textBlocks(t, result); blocks[0] != "\"xxxxxxxxx" {
			t.Errorf("first block = %q", blocks[0])
		}
	})

	t.Run("within budget is unchanged", func(t *testing.T) {
		page := &mockContentPage{evaluateFunc: func(string, ...any) (any, error) { return float64(2), nil }}
		handler := EvaluateHandler(sessionWithPage(page), DefaultTimeoutConfig(), DefaultResponseConfig())
		result := callTool(t, handler, "evaluate", map[string]any{"sessionId": "sess-test", "expression": "1+1"})
		if blocks := textBlocks(t, result); len(blocks) != 1 || blocks[0] != "2" {
			t.Errorf("blocks = %q", blocks)
		}
	})

	t.Run("negative maxChars is rejected", func(t *testing.T) {
		page := &mockContentPage{}
		handler := EvaluateHandler(sessionWithPage(page), DefaultTimeoutConfig(), DefaultResponseConfig())
		result := callTool(t, handler, "evaluate", map[string]any{"sessionId": "sess-test", "expression": "1", "maxChars": float64(-1)})
		if text := extractTextContent(result.Content); !result.IsError || !strings.HasPrefix(text, "[-32602]") {
			t.Errorf("result = %q, want [-32602]", text)
		}
		if page.evaluated != 0 {
			t.Error("page was evaluated before validation")
		}
	})

	t.Run("evaluate has no offset parameter", func(t *testing.T) {
		props := EvaluateTool().InputSchema.Properties
		if _, ok := props["offset"]; ok {
			t.Error("evaluate should not declare offset")
		}
		if _, ok := props["maxChars"]; !ok {
			t.Error("evaluate should declare maxChars")
		}
	})
}

// mockHTMLPage is a playwright.Page whose Content returns canned HTML.
type mockHTMLPage struct {
	playwright.Page
	html string
}

func (m *mockHTMLPage) Content() (string, error) { return m.html, nil }

// invalidTextParams are maxChars/offset values every text tool must reject with
// [-32602] before it touches a session.
var invalidTextParams = []struct {
	name  string
	args  map[string]any
	param string
}{
	{"zero maxChars", map[string]any{"maxChars": float64(0)}, "maxChars"},
	{"negative maxChars", map[string]any{"maxChars": float64(-1)}, "maxChars"},
	{"negative offset", map[string]any{"offset": float64(-5)}, "offset"},
}

// assertInvalidParams checks that result is a [-32602] error naming param.
func assertInvalidParams(t *testing.T, result *mcp.CallToolResult, param string) {
	t.Helper()
	if !result.IsError {
		t.Fatal("expected error result")
	}
	text := extractTextContent(result.Content)
	if !strings.HasPrefix(text, "[-32602]") || !strings.Contains(text, param) {
		t.Errorf("error = %q, want [-32602] naming %s", text, param)
	}
}

// testGetHTMLPagination covers maxChars/offset validation and slicing for
// get_html. It runs as a subtest of TestGetHTMLHandler.
func testGetHTMLPagination(t *testing.T) {
	for _, tc := range invalidTextParams {
		t.Run(tc.name+" is rejected before the session is looked up", func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
					t.Error("session was looked up before validation")
					return nil, false
				},
			}
			args := map[string]any{"sessionId": "sess-test"}
			for k, v := range tc.args {
				args[k] = v
			}
			assertInvalidParams(t, callTool(t, GetHTMLHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig()), "get_html", args), tc.param)
		})
	}

	t.Run("offset reads the next slice", func(t *testing.T) {
		page := &mockHTMLPage{html: strings.Repeat("<p>x</p>", 40)} // 320 runes
		handler := GetHTMLHandler(sessionWithPage(page), DefaultTimeoutConfig(), &ResponseConfig{MaxResponseChars: 100})
		blocks := textBlocks(t, callTool(t, handler, "get_html", map[string]any{"sessionId": "sess-test", "offset": float64(100)}))
		if len(blocks) != 2 {
			t.Fatalf("got %d blocks, want slice and notice", len(blocks))
		}
		if blocks[0] != page.html[100:200] {
			t.Errorf("slice = %q, want runes 100-200", blocks[0])
		}
		if want := "[truncated: 120 more characters; call again with offset=200]"; blocks[1] != want {
			t.Errorf("notice = %q, want %q", blocks[1], want)
		}
	})
}

// testNavigateAndExtractTextPagination checks that navigate_and_extract_text
// rejects bad maxChars/offset before it creates a browser session, so bad input
// cannot leak one. It runs as a subtest of TestNavigateAndExtractTextHandler.
func testNavigateAndExtractTextPagination(t *testing.T) {
	for _, tc := range invalidTextParams {
		t.Run(tc.name+" is rejected before a session is created", func(t *testing.T) {
			mgr := &mockSessionManager{
				createSessionFunc: func(ctx context.Context, mcpSessionID string, opts session.SessionOptions) (*session.BrowserSession, error) {
					t.Error("session was created before validation")
					return nil, fmt.Errorf("unexpected CreateSession")
				},
			}
			args := map[string]any{"url": "https://example.com"}
			for k, v := range tc.args {
				args[k] = v
			}
			assertInvalidParams(t, callTool(t, NavigateAndExtractTextHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig()), "navigate_and_extract_text", args), tc.param)
		})
	}
}

// testConsoleLogsPagination covers newest-first selection and paging for
// get_console_logs. It runs as a subtest of TestGetConsoleLogsHandler.
func testConsoleLogsPagination(t *testing.T) {
	newSession := func(n int, levelOf func(i int) session.ConsoleLogLevel) *mockSessionManager {
		buf := session.NewConsoleLogBuffer(100)
		for i := 1; i <= n; i++ {
			buf.Add(session.ConsoleLogEntry{Level: levelOf(i), Text: fmt.Sprintf("entry %d", i)})
		}
		return &mockSessionManager{
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return &session.BrowserSession{ID: "sess-test", ConsoleLogs: buf}, true
			},
		}
	}
	allLog := func(int) session.ConsoleLogLevel { return session.ConsoleLogLevelLog }

	decode := func(t *testing.T, result *mcp.CallToolResult) ([]session.ConsoleLogEntry, listPageMeta) {
		t.Helper()
		blocks := textBlocks(t, result)
		if len(blocks) != 2 {
			t.Fatalf("got %d blocks, want entries and pagination block: %q", len(blocks), blocks)
		}
		var entries []session.ConsoleLogEntry
		if err := json.Unmarshal([]byte(blocks[0]), &entries); err != nil {
			t.Fatalf("entries: %v", err)
		}
		var meta listPageMeta
		if err := json.Unmarshal([]byte(blocks[1]), &meta); err != nil {
			t.Fatalf("meta: %v", err)
		}
		return entries, meta
	}

	t.Run("no limit returns the newest 50 in chronological order", func(t *testing.T) {
		handler := GetConsoleLogsHandler(newSession(80, allLog), DefaultTimeoutConfig(), DefaultResponseConfig())
		entries, meta := decode(t, callTool(t, handler, "get_console_logs", map[string]any{"sessionId": "sess-test"}))
		if len(entries) != 50 {
			t.Fatalf("got %d entries, want 50", len(entries))
		}
		for i, e := range entries {
			if want := fmt.Sprintf("entry %d", 31+i); e.Text != want {
				t.Fatalf("entries[%d] = %q, want %q", i, e.Text, want)
			}
		}
		if meta != (listPageMeta{Total: 80, Offset: 0, Returned: 50, HasMore: true}) {
			t.Errorf("meta = %+v", meta)
		}
	})

	t.Run("small budget keeps the newest entries", func(t *testing.T) {
		budget := 300
		handler := GetConsoleLogsHandler(newSession(80, allLog), DefaultTimeoutConfig(), &ResponseConfig{MaxResponseChars: budget})
		result := callTool(t, handler, "get_console_logs", map[string]any{"sessionId": "sess-test"})
		if n := utf8.RuneCountInString(textBlocks(t, result)[0]); n > budget {
			t.Errorf("entries block is %d runes, budget %d", n, budget)
		}
		entries, meta := decode(t, result)
		if len(entries) == 0 || len(entries) >= 50 || !meta.HasMore || meta.Returned != len(entries) {
			t.Fatalf("got %d entries, meta %+v; want a budget-limited page with more to come", len(entries), meta)
		}
		first := 80 - len(entries) + 1
		for i, e := range entries {
			if want := fmt.Sprintf("entry %d", first+i); e.Text != want {
				t.Fatalf("entries[%d] = %q, want %q (newest entries, oldest first)", i, e.Text, want)
			}
		}
	})

	t.Run("offset skips the newest entries", func(t *testing.T) {
		handler := GetConsoleLogsHandler(newSession(80, allLog), DefaultTimeoutConfig(), DefaultResponseConfig())
		entries, meta := decode(t, callTool(t, handler, "get_console_logs", map[string]any{
			"sessionId": "sess-test", "limit": float64(10), "offset": float64(5),
		}))
		if len(entries) != 10 || entries[0].Text != "entry 66" || entries[9].Text != "entry 75" {
			t.Errorf("got %d entries %q..%q, want entry 66..75", len(entries), entries[0].Text, entries[len(entries)-1].Text)
		}
		if !meta.HasMore || meta.Offset != 5 {
			t.Errorf("meta = %+v", meta)
		}
	})

	t.Run("total counts entries after the level filter", func(t *testing.T) {
		levelOf := func(i int) session.ConsoleLogLevel {
			if i%4 == 0 {
				return session.ConsoleLogLevelError
			}
			return session.ConsoleLogLevelLog
		}
		handler := GetConsoleLogsHandler(newSession(80, levelOf), DefaultTimeoutConfig(), DefaultResponseConfig())
		entries, meta := decode(t, callTool(t, handler, "get_console_logs", map[string]any{
			"sessionId": "sess-test", "level": "error",
		}))
		if meta.Total != 20 || len(entries) != 20 || meta.HasMore {
			t.Errorf("got %d entries, meta %+v; want all 20 errors", len(entries), meta)
		}
	})

	t.Run("invalid limit is rejected", func(t *testing.T) {
		handler := GetConsoleLogsHandler(newSession(1, allLog), DefaultTimeoutConfig(), DefaultResponseConfig())
		result := callTool(t, handler, "get_console_logs", map[string]any{"sessionId": "sess-test", "limit": float64(-1)})
		if text := extractTextContent(result.Content); !result.IsError || !strings.Contains(text, "[-32602] Invalid parameters (parameter: limit)") {
			t.Errorf("result = %q", text)
		}
	})
}

// mockElementHandle is a playwright.ElementHandle with canned info.
type mockElementHandle struct {
	playwright.ElementHandle
	info map[string]any
}

func (m *mockElementHandle) Evaluate(expression string, arg ...any) (any, error) {
	return m.info, nil
}

func (m *mockElementHandle) BoundingBox() (*playwright.Rect, error) {
	return &playwright.Rect{X: 1, Y: 2, Width: 3, Height: 4}, nil
}

// mockQueryPage is a playwright.Page that answers QuerySelector(All) with mocks.
type mockQueryPage struct {
	playwright.Page
	elements []playwright.ElementHandle
}

func (m *mockQueryPage) QuerySelector(selector string, options ...playwright.PageQuerySelectorOptions) (playwright.ElementHandle, error) {
	if len(m.elements) == 0 {
		return nil, nil
	}
	return m.elements[0], nil
}

func (m *mockQueryPage) QuerySelectorAll(selector string) ([]playwright.ElementHandle, error) {
	return m.elements, nil
}

// testQuerySelectorPagination covers query_selector with and without all. It
// runs as a subtest of TestQuerySelectorHandler.
func testQuerySelectorPagination(t *testing.T) {
	page := &mockQueryPage{}
	for i := 0; i < 70; i++ {
		page.elements = append(page.elements, &mockElementHandle{info: map[string]any{
			"tag":        "li",
			"attributes": map[string]any{"id": fmt.Sprintf("item-%d", i)},
			"text":       fmt.Sprintf("Item %d", i),
		}})
	}
	handler := QuerySelectorHandler(sessionWithPage(page), DefaultTimeoutConfig(), DefaultResponseConfig())

	t.Run("without all returns a single element object", func(t *testing.T) {
		result := callTool(t, handler, "query_selector", map[string]any{"sessionId": "sess-test", "selector": "li"})
		blocks := textBlocks(t, result)
		if len(blocks) != 1 {
			t.Fatalf("got %d blocks, want 1", len(blocks))
		}
		var info ElementInfo
		if err := json.Unmarshal([]byte(blocks[0]), &info); err != nil {
			t.Fatalf("not a single element object: %v\n%s", err, blocks[0])
		}
		if info.Tag != "li" || info.Text != "Item 0" || info.BoundingBox == nil {
			t.Errorf("info = %+v", info)
		}
	})

	t.Run("without all cuts an element larger than the budget", func(t *testing.T) {
		big := &mockQueryPage{elements: []playwright.ElementHandle{&mockElementHandle{info: map[string]any{
			"tag":        "img",
			"attributes": map[string]any{"data-x": strings.Repeat("a", 10000)},
			"text":       "",
		}}}}
		budget := 2000
		h := QuerySelectorHandler(sessionWithPage(big), DefaultTimeoutConfig(), &ResponseConfig{MaxResponseChars: budget})
		blocks := textBlocks(t, callTool(t, h, "query_selector", map[string]any{"sessionId": "sess-test", "selector": "img"}))
		if len(blocks) != 1 {
			t.Fatalf("got %d blocks, want 1", len(blocks))
		}
		if n := utf8.RuneCountInString(blocks[0]); n > budget {
			t.Errorf("result is %d runes, budget %d", n, budget)
		}
		var info ElementInfo
		if err := json.Unmarshal([]byte(blocks[0]), &info); err != nil {
			t.Fatalf("not a single element object: %v\n%s", err, blocks[0])
		}
		if v := info.Attributes["data-x"]; !strings.HasSuffix(v, "…") {
			t.Errorf("data-x was not cut: %d runes", utf8.RuneCountInString(v))
		}
	})

	t.Run("with all pages by item", func(t *testing.T) {
		result := callTool(t, handler, "query_selector", map[string]any{
			"sessionId": "sess-test", "selector": "li", "all": true, "offset": float64(60),
		})
		blocks := textBlocks(t, result)
		if len(blocks) != 2 {
			t.Fatalf("got %d blocks, want 2", len(blocks))
		}
		var infos []ElementInfo
		if err := json.Unmarshal([]byte(blocks[0]), &infos); err != nil {
			t.Fatalf("first block is not a bare array: %v", err)
		}
		if len(infos) != 10 || infos[0].Text != "Item 60" {
			t.Errorf("got %d infos", len(infos))
		}
		var meta listPageMeta
		if err := json.Unmarshal([]byte(blocks[1]), &meta); err != nil {
			t.Fatal(err)
		}
		if meta != (listPageMeta{Total: 70, Offset: 60, Returned: 10, HasMore: false}) {
			t.Errorf("meta = %+v", meta)
		}
	})
}

// testGetLinksPagination covers get_links paging. It runs as a subtest of
// TestGetLinksHandler.
func testGetLinksPagination(t *testing.T) {
	links := make([]any, 120)
	for i := range links {
		links[i] = map[string]any{"href": fmt.Sprintf("http://host/%d", i), "label": fmt.Sprintf("Link %d", i)}
	}
	page := &mockContentPage{evaluateFunc: func(string, ...any) (any, error) { return links, nil }}
	handler := GetLinksHandler(sessionWithPage(page), DefaultTimeoutConfig(), DefaultResponseConfig())

	result := callTool(t, handler, "get_links", map[string]any{"sessionId": "sess-test"})
	blocks := textBlocks(t, result)
	var got []LinkInfo
	if err := json.Unmarshal([]byte(blocks[0]), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 50 || blocks[1] != `{"total":120,"offset":0,"returned":50,"hasMore":true}` {
		t.Errorf("got %d links, meta %s", len(got), blocks[1])
	}

	result = callTool(t, handler, "get_links", map[string]any{"sessionId": "sess-test", "limit": float64(0)})
	if !result.IsError || !strings.Contains(extractTextContent(result.Content), "(parameter: limit)") {
		t.Errorf("limit 0 = %q, want invalid params", extractTextContent(result.Content))
	}
	if page.evaluated != 1 {
		t.Errorf("page evaluated %d times, want 1 (invalid call must not touch it)", page.evaluated)
	}
}
