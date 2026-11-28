//go:build integration

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// refTestPages serves the pages the ref and state-summary integration tests use.
func refTestPages(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	page := func(path, title, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<!DOCTYPE html><html><head><title>%s</title></head><body>%s</body></html>", title, body)
		})
	}
	page("/twins", "Twins", `<button onclick="this.textContent='Clicked'">Go</button>
		<button onclick="this.textContent='Clicked'">Go</button>`)
	page("/inputs", "Inputs", `<input aria-label="Field"><input aria-label="Field">`)
	page("/frame", "Frame", `<button>Outer</button>
		<iframe srcdoc="<button onclick=&quot;this.textContent='Inner clicked'&quot;>Inner</button>"></iframe>`)
	page("/other", "Other", `<button>Elsewhere</button>`)
	page("/three", "Three", `<button>Alpha</button><button>Beta</button><button>Gamma</button>`)
	page("/page1", "Page One", `<a href="/page2">Next</a>`)
	page("/page2", "Page Two", `<h1>Second page</h1>`)
	// Break misbehaves on its first click only, so a second click has nothing new.
	page("/errors", "Errors", `<button onclick="if (!this.dataset.done) { this.dataset.done = '1'; console.error('boom from click'); fetch('/fail?token=secret').catch(() => {}); }">Break</button>`)
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/page2?from=redirect", http.StatusFound)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// refHarness is a live browser session with the tool handlers the ref tests use.
type refHarness struct {
	t         *testing.T
	ctx       context.Context
	mgr       session.BrowserSessionManager
	sessionID string
	handlers  map[string]server.ToolHandlerFunc
}

// newRefHarness starts a pool and creates one browser session.
func newRefHarness(t *testing.T) *refHarness {
	t.Helper()
	pool, mgr := setupPoolAndManager(t)
	t.Cleanup(func() { pool.Stop(context.Background()) })

	h := &refHarness{t: t, ctx: context.Background(), mgr: mgr, handlers: allToolHandlers(mgr)}
	created := h.call("session_create", map[string]any{})
	if created.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(created))
		t.Fatalf("Create failed: %s", extractTextContent(created.Content))
	}
	h.sessionID = strings.TrimPrefix(extractTextContent(created.Content), "Created session: ")
	t.Cleanup(func() { mgr.CloseSession(context.Background(), "", h.sessionID) })
	return h
}

// call runs a tool with sessionId filled in once the session exists.
func (h *refHarness) call(tool string, args map[string]any) *mcp.CallToolResult {
	h.t.Helper()
	if h.sessionID != "" {
		args["sessionId"] = h.sessionID
	}
	result, err := h.handlers[tool](h.ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: tool, Arguments: args},
	})
	if err != nil {
		h.t.Fatalf("%s returned error: %v", tool, err)
	}
	return result
}

// mustCall runs a tool and fails the test on an error result.
func (h *refHarness) mustCall(tool string, args map[string]any) *mcp.CallToolResult {
	h.t.Helper()
	result := h.call(tool, args)
	if result.IsError {
		h.t.Fatalf("%s failed: %s", tool, extractTextContent(result.Content))
	}
	return result
}

// snapshot returns the page's snapshot.
func (h *refHarness) snapshot() string {
	h.t.Helper()
	return extractTextContent(h.mustCall("get_accessibility_tree", map[string]any{}).Content)
}

// page returns the session's active page for direct assertions.
func (h *refHarness) page() playwright.Page {
	h.t.Helper()
	sess, ok := h.mgr.GetSession("", h.sessionID)
	if !ok || sess.ActivePage() == nil {
		h.t.Fatal("Expected an active page")
	}
	return sess.ActivePage()
}

// eval evaluates an expression on the active page.
func (h *refHarness) eval(expression string) any {
	h.t.Helper()
	v, err := h.page().Evaluate(expression)
	if err != nil {
		h.t.Fatalf("evaluate %q failed: %v", expression, err)
	}
	return v
}

// refsFor returns, in document order, the refs of snapshot lines with the given
// role and accessible name.
func refsFor(snapshot, role, name string) []string {
	re := regexp.MustCompile(`(?m)^\s*- ` + regexp.QuoteMeta(role) + ` "` + regexp.QuoteMeta(name) + `"(?: \[[^\]]+\])* \[ref=((?:f\d+)?e\d+)\]`)
	var refs []string
	for _, m := range re.FindAllStringSubmatch(snapshot, -1) {
		refs = append(refs, m[1])
	}
	return refs
}

// mustRefs is refsFor that requires exactly n matches.
func (h *refHarness) mustRefs(snapshot, role, name string, n int) []string {
	h.t.Helper()
	refs := refsFor(snapshot, role, name)
	if len(refs) != n {
		h.t.Fatalf("Expected %d %s %q refs, found %v in:\n%s", n, role, name, refs, snapshot)
	}
	return refs
}

func TestClickByRefIntegration(t *testing.T) {
	h := newRefHarness(t)
	ts := refTestPages(t)
	h.mustCall("navigate", map[string]any{"url": ts.URL + "/twins"})

	refs := h.mustRefs(h.snapshot(), "button", "Go", 2)
	result := h.mustCall("click", map[string]any{"ref": refs[1]})
	if text := extractTextContent(result.Content); !strings.HasPrefix(text, "Clicked element: ref="+refs[1]) {
		t.Errorf("Unexpected click result: %s", text)
	}

	got := h.eval(`() => [...document.querySelectorAll('button')].map(b => b.textContent).join(',')`)
	if got != "Go,Clicked" {
		t.Errorf("Expected only the second button to change, got %q", got)
	}
}

func TestFillByRefIntegration(t *testing.T) {
	h := newRefHarness(t)
	ts := refTestPages(t)
	h.mustCall("navigate", map[string]any{"url": ts.URL + "/inputs"})

	refs := h.mustRefs(h.snapshot(), "textbox", "Field", 2)
	h.mustCall("fill", map[string]any{"ref": refs[1], "value": "second"})

	got := h.eval(`() => [...document.querySelectorAll('input')].map(i => i.value).join('|')`)
	if got != "|second" {
		t.Errorf("Expected only the second textbox filled, got %q", got)
	}
	if after := h.snapshot(); !strings.Contains(after, "[ref="+refs[1]+"]: second") {
		t.Errorf("Expected the snapshot to show the filled value under the same ref:\n%s", after)
	}
}

func TestClickFrameRefIntegration(t *testing.T) {
	h := newRefHarness(t)
	ts := refTestPages(t)
	h.mustCall("navigate", map[string]any{"url": ts.URL + "/frame"})

	refs := h.mustRefs(h.snapshot(), "button", "Inner", 1)
	if !regexp.MustCompile(`^f\d+e\d+$`).MatchString(refs[0]) {
		t.Fatalf("Expected an f<k>e<n> ref for the iframe button, got %s", refs[0])
	}
	h.mustCall("click", map[string]any{"ref": refs[0]})

	got := h.eval(`() => document.querySelector('iframe').contentDocument.querySelector('button').textContent`)
	if got != "Inner clicked" {
		t.Errorf("Expected the iframe button to be clicked, got %q", got)
	}
}

func TestStaleRefIntegration(t *testing.T) {
	h := newRefHarness(t)
	ts := refTestPages(t)
	h.mustCall("navigate", map[string]any{"url": ts.URL + "/twins"})
	oldRef := h.mustRefs(h.snapshot(), "button", "Go", 2)[0]

	h.mustCall("navigate", map[string]any{"url": ts.URL + "/other"})
	newSnapshot := h.snapshot()
	if strings.Contains(newSnapshot, "[ref="+oldRef+"]") {
		t.Fatalf("Expected the new document to use new refs, but %s is still present:\n%s", oldRef, newSnapshot)
	}

	start := time.Now()
	result := h.call("click", map[string]any{"ref": oldRef})
	elapsed := time.Since(start)

	text := extractTextContent(result.Content)
	if !result.IsError || !strings.HasPrefix(text, "[-32002] Stale element ref") {
		t.Fatalf("Expected a stale ref error, got: %s", text)
	}
	if !strings.Contains(text, "ref "+oldRef+" is not on the current page") || !strings.Contains(text, "Take a new snapshot with get_accessibility_tree") {
		t.Errorf("Expected the stale ref reason and suggestion, got: %s", text)
	}
	if elapsed > time.Second {
		t.Errorf("Expected the stale ref to fail within 1s, took %v", elapsed)
	}
}

// summaryOf parses the state summary from an action result's text: the line after
// the action line.
func summaryOf(t *testing.T, result *mcp.CallToolResult) stateSummary {
	t.Helper()
	text := extractTextContent(result.Content)
	_, rest, ok := strings.Cut(text, "\n")
	if !ok {
		t.Fatalf("Expected a state summary after the action line, got %q", text)
	}
	var summary stateSummary
	if err := json.Unmarshal([]byte(rest), &summary); err != nil {
		t.Fatalf("State summary is not JSON: %v\n%s", err, rest)
	}
	return summary
}

func TestActionStateSummaryIntegration(t *testing.T) {
	h := newRefHarness(t)
	ts := refTestPages(t)

	t.Run("link click", func(t *testing.T) {
		h.mustCall("navigate", map[string]any{"url": ts.URL + "/page1"})
		result := h.mustCall("click", map[string]any{"selector": "a"})

		summary := summaryOf(t, result)
		if !summary.Navigated {
			t.Errorf("Expected navigated: true, got %+v", summary)
		}
		if summary.URL != ts.URL+"/page2" {
			t.Errorf("URL = %q, want %q", summary.URL, ts.URL+"/page2")
		}
		if summary.Title == nil || *summary.Title != "Page Two" {
			t.Errorf("Title = %v, want Page Two", summary.Title)
		}
		if summary.HTTPStatus != nil {
			t.Errorf("Expected no httpStatus on click, got %d", *summary.HTTPStatus)
		}
	})

	t.Run("redirecting handler", func(t *testing.T) {
		result := h.mustCall("navigate", map[string]any{"url": ts.URL + "/redirect"})

		summary := summaryOf(t, result)
		if want := ts.URL + "/page2?from=redirect"; summary.URL != want {
			t.Errorf("URL = %q, want the final URL %q", summary.URL, want)
		}
		if summary.HTTPStatus == nil || *summary.HTTPStatus != 200 {
			t.Errorf("httpStatus = %v, want 200", summary.HTTPStatus)
		}
		if !summary.Navigated || summary.Action != "navigate" {
			t.Errorf("Unexpected summary: %+v", summary)
		}
	})

	t.Run("error-emitting button clicked twice", func(t *testing.T) {
		h.mustCall("navigate", map[string]any{"url": ts.URL + "/errors"})

		first := summaryOf(t, h.mustCall("click", map[string]any{"selector": "button"}))
		// Chromium also logs its own "Failed to load resource" error for the 500.
		if !slices.ContainsFunc(first.NewConsoleErrors, func(e string) bool { return strings.Contains(e, "boom from click") }) {
			t.Errorf("Expected the page's console error, got %v", first.NewConsoleErrors)
		}
		wantFailed := "GET " + ts.URL + "/fail 500"
		if len(first.FailedRequests) != 1 || first.FailedRequests[0] != wantFailed {
			t.Errorf("Expected failed request %q, got %v", wantFailed, first.FailedRequests)
		}

		second := summaryOf(t, h.mustCall("click", map[string]any{"selector": "button"}))
		if len(second.NewConsoleErrors) != 0 || len(second.FailedRequests) != 0 {
			t.Errorf("Expected nothing new on the second click, got errors %v and requests %v",
				second.NewConsoleErrors, second.FailedRequests)
		}
	})

	t.Run("includeSnapshot", func(t *testing.T) {
		h.mustCall("navigate", map[string]any{"url": ts.URL + "/page1"})
		result := h.mustCall("click", map[string]any{"selector": "a", "includeSnapshot": true})

		if len(result.Content) != 2 {
			t.Fatalf("Expected 2 content blocks, got %d", len(result.Content))
		}
		block, ok := mcp.AsTextContent(result.Content[1])
		if !ok {
			t.Fatal("Expected the second block to be text")
		}
		// The snapshot is of the page the click navigated to.
		refs := h.mustRefs(block.Text, "heading", "Second page", 1)

		// A ref from the post-action snapshot works on the next call.
		next := h.mustCall("click", map[string]any{"ref": refs[0]})
		if text := extractTextContent(next.Content); !strings.HasPrefix(text, "Clicked element: ref="+refs[0]) {
			t.Errorf("Unexpected result clicking a ref from includeSnapshot: %s", text)
		}
	})

	t.Run("navigation tool includeSnapshot", func(t *testing.T) {
		result := h.mustCall("navigate", map[string]any{"url": ts.URL + "/twins", "includeSnapshot": true})
		if len(result.Content) != 2 {
			t.Fatalf("Expected 2 content blocks, got %d", len(result.Content))
		}
		block, _ := mcp.AsTextContent(result.Content[1])
		refs := h.mustRefs(block.Text, "button", "Go", 2)
		h.mustCall("click", map[string]any{"ref": refs[0]})
		if got := h.eval(`() => document.querySelector('button').textContent`); got != "Clicked" {
			t.Errorf("Expected the first button clicked by its navigate-snapshot ref, got %q", got)
		}
	})
}

func TestAmbiguousSelectorIntegration(t *testing.T) {
	h := newRefHarness(t)
	ts := refTestPages(t)
	h.mustCall("navigate", map[string]any{"url": ts.URL + "/three"})

	start := time.Now()
	result := h.call("click", map[string]any{"selector": "button", "timeout": float64(5000)})
	elapsed := time.Since(start)

	text := extractTextContent(result.Content)
	if !result.IsError || !strings.HasPrefix(text, "[-32002] Selector matched 3 elements") {
		t.Fatalf("Expected the multi-match error, got: %s", text)
	}
	for _, want := range []string{`<button> "Alpha"`, `<button> "Beta"`, `<button> "Gamma"`} {
		if !strings.Contains(text, want) {
			t.Errorf("Expected the error to describe %s, got: %s", want, text)
		}
	}
	if elapsed > 3*time.Second {
		t.Errorf("Expected the strict mode violation to fail fast, took %v", elapsed)
	}
}
