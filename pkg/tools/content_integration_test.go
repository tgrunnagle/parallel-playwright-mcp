//go:build integration

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// longPageText is the visible text of the /long page: numbered words, so every
// slice of it is distinct.
var longPageText = func() string {
	var b strings.Builder
	for i := 0; b.Len() < 3000; i++ {
		fmt.Fprintf(&b, "w%04d ", i)
	}
	return strings.TrimSpace(b.String())
}()

// startContentTestServer serves pages for the content pagination integration tests.
func startContentTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/long", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<!DOCTYPE html><html><body><p>%s</p></body></html>", longPageText)
	})
	mux.HandleFunc("/links", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		var b strings.Builder
		b.WriteString("<!DOCTYPE html><html><body>")
		for i := 0; i < 120; i++ {
			fmt.Fprintf(&b, `<a href="/page/%d">Link %d</a> `, i, i)
		}
		b.WriteString("</body></html>")
		fmt.Fprint(w, b.String())
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// openSessionAt creates a browser session and navigates it to url.
func openSessionAt(t *testing.T, mgr session.BrowserSessionManager, url string) string {
	t.Helper()
	ctx := context.Background()
	createResult := callIntegrationTool(t, SessionCreateHandler(mgr, DefaultTimeoutConfig()), "session_create", map[string]any{})
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	t.Cleanup(func() { _ = mgr.CloseSession(ctx, "", sessionID) })

	navResult := callIntegrationTool(t, NavigateHandler(mgr, DefaultTimeoutConfig(), nil), "navigate", map[string]any{
		"sessionId": sessionID,
		"url":       url,
	})
	if navResult.IsError {
		t.Fatalf("Navigate failed: %s", extractTextContent(navResult.Content))
	}
	return sessionID
}

// callIntegrationTool invokes a handler and fails the test on a handler error.
func callIntegrationTool(t *testing.T, handler server.ToolHandlerFunc, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: name, Arguments: args},
	})
	if err != nil {
		t.Fatalf("%s handler returned error: %v", name, err)
	}
	return result
}

// integrationTextBlocks returns the text of each content block of a result.
func integrationTextBlocks(t *testing.T, result *mcp.CallToolResult) []string {
	t.Helper()
	var blocks []string
	for _, c := range result.Content {
		tc, ok := mcp.AsTextContent(c)
		if !ok {
			t.Fatalf("content block %T is not text", c)
		}
		blocks = append(blocks, tc.Text)
	}
	return blocks
}

func TestExtractTextPaginationIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	ts := startContentTestServer(t)
	sessionID := openSessionAt(t, mgr, ts.URL+"/long")
	handler := ExtractTextHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())

	full := integrationTextBlocks(t, callIntegrationTool(t, handler, "extract_text", map[string]any{"sessionId": sessionID}))
	if len(full) != 1 || full[0] != longPageText {
		t.Fatalf("unpaged extract = %d blocks, want the page text in one block", len(full))
	}
	total := utf8.RuneCountInString(longPageText)

	const maxChars = 1000
	first := integrationTextBlocks(t, callIntegrationTool(t, handler, "extract_text", map[string]any{
		"sessionId": sessionID,
		"maxChars":  float64(maxChars),
	}))
	if len(first) != 2 {
		t.Fatalf("got %d blocks, want content and pagination block", len(first))
	}
	if n := utf8.RuneCountInString(first[0]); n != maxChars {
		t.Errorf("first block is %d runes, want %d", n, maxChars)
	}
	if first[0] != longPageText[:maxChars] {
		t.Errorf("first block is not the start of the page text")
	}
	wantNotice := fmt.Sprintf("[truncated: %d more characters; call again with offset=%d]", total-maxChars, maxChars)
	if first[1] != wantNotice {
		t.Fatalf("notice = %q, want %q", first[1], wantNotice)
	}

	match := regexp.MustCompile(`offset=(\d+)`).FindStringSubmatch(first[1])
	nextOffset, _ := strconv.Atoi(match[1])
	second := integrationTextBlocks(t, callIntegrationTool(t, handler, "extract_text", map[string]any{
		"sessionId": sessionID,
		"maxChars":  float64(maxChars),
		"offset":    float64(nextOffset),
	}))
	if len(second) != 2 {
		t.Fatalf("second call returned %d blocks, want slice and notice", len(second))
	}
	if second[0] != longPageText[maxChars:2*maxChars] {
		t.Errorf("second slice is not the next %d characters", maxChars)
	}
	wantSecond := fmt.Sprintf("[truncated: %d more characters; call again with offset=%d]", total-2*maxChars, 2*maxChars)
	if total <= 2*maxChars {
		wantSecond = fmt.Sprintf("[end of content: %d characters]", total)
	}
	if second[1] != wantSecond {
		t.Errorf("second notice = %q, want %q", second[1], wantSecond)
	}

	past := integrationTextBlocks(t, callIntegrationTool(t, handler, "extract_text", map[string]any{
		"sessionId": sessionID,
		"offset":    float64(total),
	}))
	if len(past) != 2 || past[0] != "" || past[1] != fmt.Sprintf("[end of content: %d characters]", total) {
		t.Errorf("offset at end = %q", past)
	}
}

func TestGetLinksPaginationIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	ts := startContentTestServer(t)
	sessionID := openSessionAt(t, mgr, ts.URL+"/links")
	handler := GetLinksHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())

	decode := func(t *testing.T, result *mcp.CallToolResult) ([]LinkInfo, listPageMeta) {
		t.Helper()
		if result.IsError {
			t.Fatalf("get_links failed: %s", extractTextContent(result.Content))
		}
		blocks := integrationTextBlocks(t, result)
		if len(blocks) != 2 {
			t.Fatalf("got %d blocks, want links and pagination block", len(blocks))
		}
		var links []LinkInfo
		if err := json.Unmarshal([]byte(blocks[0]), &links); err != nil {
			t.Fatalf("first block is not a bare JSON array: %v", err)
		}
		var meta listPageMeta
		if err := json.Unmarshal([]byte(blocks[1]), &meta); err != nil {
			t.Fatalf("second block is not pagination metadata: %v", err)
		}
		return links, meta
	}

	links, meta := decode(t, callIntegrationTool(t, handler, "get_links", map[string]any{"sessionId": sessionID}))
	if len(links) != 50 || meta.Total != 120 || !meta.HasMore {
		t.Errorf("first page: %d links, meta %+v; want 50 of 120 with more", len(links), meta)
	}
	if links[0].Href != ts.URL+"/page/0" || links[0].Label != "Link 0" {
		t.Errorf("links[0] = %+v", links[0])
	}

	links, meta = decode(t, callIntegrationTool(t, handler, "get_links", map[string]any{
		"sessionId": sessionID,
		"offset":    float64(100),
	}))
	if len(links) != 20 || meta.Returned != 20 || meta.HasMore {
		t.Errorf("offset 100: %d links, meta %+v; want 20 and no more", len(links), meta)
	}
	if links[0].Label != "Link 100" || links[19].Label != "Link 119" {
		t.Errorf("offset 100 returned %q..%q", links[0].Label, links[19].Label)
	}
}
