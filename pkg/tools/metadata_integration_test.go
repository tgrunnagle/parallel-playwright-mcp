//go:build integration

package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestNavigateAndExtractTextTimeoutIntegration checks that the timeout argument
// bounds navigate_and_extract_text's navigation, not just its context.
func TestNavigateAndExtractTextTimeoutIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/fast", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><p>fast</p></body></html>")
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><p>slow</p></body></html>")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	handler := NavigateAndExtractTextHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	call := func(args map[string]any) *mcp.CallToolResult {
		t.Helper()
		result, err := handler(context.Background(), mcp.CallToolRequest{
			Params: mcp.CallToolParams{Name: "navigate_and_extract_text", Arguments: args},
		})
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		return result
	}

	// Launch the browser first, so the timed call measures only its navigation.
	if warm := call(map[string]any{"url": ts.URL + "/fast"}); warm.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(warm))
		t.Fatalf("warm-up call failed: %s", extractTextContent(warm.Content))
	}

	start := time.Now()
	result := call(map[string]any{"url": ts.URL + "/slow", "timeout": float64(500)})
	elapsed := time.Since(start)

	text := extractTextContent(result.Content)
	if !result.IsError || !strings.HasPrefix(text, "[-32003]") {
		t.Errorf("result = %q (error %v), want a [-32003] timeout error", text, result.IsError)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("call took %v, want about 1 second or less", elapsed)
	}
}
