//go:build integration

package tools

import (
	"context"
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

	navResult := callIntegrationTool(t, NavigateHandler(mgr, DefaultTimeoutConfig()), "navigate", map[string]any{
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
	if second[0] != longPageText[maxChars:2*maxChars] {
		t.Errorf("second slice is not the next %d characters", maxChars)
	}

	past := integrationTextBlocks(t, callIntegrationTool(t, handler, "extract_text", map[string]any{
		"sessionId": sessionID,
		"offset":    float64(total),
	}))
	if len(past) != 2 || past[0] != "" || past[1] != fmt.Sprintf("[end of content: %d characters]", total) {
		t.Errorf("offset at end = %q", past)
	}
}
