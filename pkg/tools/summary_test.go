package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// summarySession returns a session with real log buffers around page.
func summarySession(page playwright.Page) *session.BrowserSession {
	return &session.BrowserSession{
		ID:          "sess-1",
		ActiveTabID: "tab-1",
		Pages:       map[string]playwright.Page{"tab-1": page},
		ConsoleLogs: session.NewConsoleLogBuffer(0),
		NetworkLogs: session.NewNetworkLogBuffer(0),
	}
}

func TestBuildStateSummary(t *testing.T) {
	t.Run("reports only entries after the action started", func(t *testing.T) {
		page := &mockInteractionPage{}
		sess := summarySession(page)
		sess.ConsoleLogs.Add(session.ConsoleLogEntry{Level: session.ConsoleLogLevelError, Text: "before"})
		sess.NetworkLogs.Add(session.NetworkLogEntry{Method: "GET", URL: "http://h/old", Status: 500})

		rec := startAction(sess, page, "click", "#go", nil, nil, nil)
		sess.ConsoleLogs.Add(session.ConsoleLogEntry{Level: session.ConsoleLogLevelLog, Text: "just a log"})
		sess.ConsoleLogs.Add(session.ConsoleLogEntry{Level: session.ConsoleLogLevelError, Text: "after"})
		sess.NetworkLogs.Add(session.NetworkLogEntry{Method: "GET", URL: "http://h/ok", Status: 200})
		sess.NetworkLogs.Add(session.NetworkLogEntry{Method: "GET", URL: "http://h/redirect", Status: 302})
		sess.NetworkLogs.Add(session.NetworkLogEntry{Method: "POST", URL: "http://h/api?token=abc", Status: 500})
		sess.NetworkLogs.Add(session.NetworkLogEntry{Method: "GET", URL: "http://h/missing#frag", Status: 404})
		sess.NetworkLogs.Add(session.NetworkLogEntry{Method: "GET", URL: "http://user:pw@h/down", Status: 0})

		got := buildStateSummary(rec)

		if got.Action != "click" || got.Target != "#go" || got.URL != "https://example.com/" {
			t.Errorf("unexpected header fields: %+v", got)
		}
		if got.Title == nil || *got.Title != "Example" {
			t.Errorf("Title = %v, want Example", got.Title)
		}
		if got.Navigated {
			t.Error("Navigated = true, want false for an unchanged URL")
		}
		if got.HTTPStatus != nil {
			t.Errorf("HTTPStatus = %v, want nil for an interaction tool", *got.HTTPStatus)
		}
		if strings.Join(got.NewConsoleErrors, "|") != "after" {
			t.Errorf("NewConsoleErrors = %v, want [after]", got.NewConsoleErrors)
		}
		wantFailed := []string{
			"POST http://h/api 500",
			"GET http://h/missing 404",
			"GET http://h/down (no response)",
		}
		if strings.Join(got.FailedRequests, "|") != strings.Join(wantFailed, "|") {
			t.Errorf("FailedRequests = %v, want %v", got.FailedRequests, wantFailed)
		}
		if got.More != nil {
			t.Errorf("More = %v, want nil", got.More)
		}
	})

	t.Run("strips query strings from failed request URLs", func(t *testing.T) {
		page := &mockInteractionPage{}
		sess := summarySession(page)
		rec := startAction(sess, page, "click", "#go", nil, nil, nil)
		sess.NetworkLogs.Add(session.NetworkLogEntry{Method: "GET", URL: "http://h/api?token=abc", Status: 500})

		got := buildStateSummary(rec)
		if len(got.FailedRequests) != 1 || got.FailedRequests[0] != "GET http://h/api 500" {
			t.Errorf("FailedRequests = %v, want [GET http://h/api 500]", got.FailedRequests)
		}
		data, _ := json.Marshal(got)
		if strings.Contains(string(data), "token") {
			t.Errorf("summary leaks the query string: %s", data)
		}
	})

	t.Run("title failure omits title and keeps the summary", func(t *testing.T) {
		page := &mockInteractionPage{titleFunc: func() (string, error) { return "", errors.New("execution context was destroyed") }}
		sess := summarySession(page)
		rec := startAction(sess, page, "click", "#go", nil, nil, nil)

		got := buildStateSummary(rec)
		if got.Title != nil {
			t.Errorf("Title = %q, want omitted", *got.Title)
		}
		data, _ := json.Marshal(got)
		if strings.Contains(string(data), `"title"`) {
			t.Errorf("summary JSON has a title: %s", data)
		}
	})

	t.Run("empty title is kept", func(t *testing.T) {
		page := &mockInteractionPage{titleFunc: func() (string, error) { return "", nil }}
		got := buildStateSummary(startAction(summarySession(page), page, "hover", "#x", nil, nil, nil))
		if got.Title == nil || *got.Title != "" {
			t.Errorf("Title = %v, want empty string", got.Title)
		}
	})

	t.Run("caps lists and cuts long entries", func(t *testing.T) {
		page := &mockInteractionPage{}
		sess := summarySession(page)
		rec := startAction(sess, page, "click", "#go", nil, nil, nil)
		for range 8 {
			sess.ConsoleLogs.Add(session.ConsoleLogEntry{Level: session.ConsoleLogLevelError, Text: strings.Repeat("x", 500)})
		}
		for range 6 {
			sess.NetworkLogs.Add(session.NetworkLogEntry{Method: "GET", URL: "http://h/a", Status: 503})
		}

		got := buildStateSummary(rec)
		if len(got.NewConsoleErrors) != 5 || len(got.FailedRequests) != 5 {
			t.Fatalf("got %d console errors and %d failed requests, want 5 and 5", len(got.NewConsoleErrors), len(got.FailedRequests))
		}
		if n := len([]rune(got.NewConsoleErrors[0])); n != 200 || !strings.HasSuffix(got.NewConsoleErrors[0], "…") {
			t.Errorf("console entry has %d runes, want 200 ending in …", n)
		}
		if got.More["newConsoleErrors"] != 3 || got.More["failedRequests"] != 1 {
			t.Errorf("More = %v, want newConsoleErrors 3, failedRequests 1", got.More)
		}
	})

	t.Run("URL change counts as navigation", func(t *testing.T) {
		url := "https://example.com/a"
		page := &mockInteractionPage{urlFunc: func() string { return url }}
		rec := startAction(summarySession(page), page, "click", "a", nil, nil, nil)
		url = "https://example.com/b"
		got := buildStateSummary(rec)
		if !got.Navigated || got.URL != "https://example.com/b" {
			t.Errorf("Navigated = %v, URL = %s", got.Navigated, got.URL)
		}
	})

	t.Run("navigation tool reports the response status", func(t *testing.T) {
		page := &mockPage{}
		rec := startAction(summarySession(page), page, "reload", "", nil, nil, nil)
		rec.navigated(&mockResponse{statusFunc: func() int { return 201 }})
		got := buildStateSummary(rec)
		if !got.Navigated || got.HTTPStatus == nil || *got.HTTPStatus != 201 {
			t.Errorf("Navigated = %v, HTTPStatus = %v", got.Navigated, got.HTTPStatus)
		}
	})

	t.Run("navigation tool without a response has no status", func(t *testing.T) {
		page := &mockPage{}
		rec := startAction(summarySession(page), page, "go_back", "", nil, nil, nil)
		rec.navigated(nil)
		got := buildStateSummary(rec)
		if got.Navigated || got.HTTPStatus != nil {
			t.Errorf("Navigated = %v, HTTPStatus = %v", got.Navigated, got.HTTPStatus)
		}
	})

	t.Run("session without buffers", func(t *testing.T) {
		page := &mockInteractionPage{}
		sess := &session.BrowserSession{ID: "s", Pages: map[string]playwright.Page{}}
		got := buildStateSummary(startAction(sess, page, "click", "#go", nil, nil, nil))
		if got.NewConsoleErrors == nil || got.FailedRequests == nil {
			t.Error("lists should be empty, not null")
		}
	})
}

func TestActionResultShape(t *testing.T) {
	page := &mockInteractionPage{}
	rec := startAction(summarySession(page), page, "click", "#button", nil, nil, nil)
	result := rec.finish(context.Background(), "Clicked element: #button")

	text := extractTextContent(result.Content)
	line, rest, ok := strings.Cut(text, "\n")
	if !ok || line != "Clicked element: #button" {
		t.Fatalf("text = %q, want the action line first", text)
	}
	var fromText map[string]any
	if err := json.Unmarshal([]byte(rest), &fromText); err != nil {
		t.Fatalf("text after the line is not JSON: %v\n%s", err, rest)
	}
	for _, key := range []string{"action", "target", "url", "title", "navigated", "newConsoleErrors", "failedRequests"} {
		if _, ok := fromText[key]; !ok {
			t.Errorf("summary JSON lacks %q: %s", key, rest)
		}
	}
	if _, ok := fromText["httpStatus"]; ok {
		t.Errorf("interaction summary should not carry httpStatus: %s", rest)
	}
	if _, ok := result.StructuredContent.(stateSummary); !ok {
		t.Errorf("StructuredContent = %T, want stateSummary", result.StructuredContent)
	}
}

func TestStripQuery(t *testing.T) {
	tests := map[string]string{
		"http://h/api?token=abc":         "http://h/api",
		"https://h/p?a=1&b=2#section":    "https://h/p",
		"https://user:secret@h/p?x":      "https://h/p",
		"http://h/plain":                 "http://h/plain",
		"http://h/%zz?bad=escape":        "http://h/%zz",
		"data:text/html,<p>hi</p>?q=1":   "data:text/html,<p>hi</p>",
		"http://h/empty-query?":          "http://h/empty-query",
		"/relative/path?session=private": "/relative/path",
	}
	for in, want := range tests {
		if got := stripQuery(in); got != want {
			t.Errorf("stripQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

// urlSequence returns a urlFunc that yields urls in turn, then repeats the last.
func urlSequence(urls ...string) func() string {
	i := 0
	return func() string {
		u := urls[min(i, len(urls)-1)]
		i++
		return u
	}
}

func TestSummarizeWaits(t *testing.T) {
	// waits records the WaitForLoadState calls of a page.
	type waits struct {
		opts []playwright.PageWaitForLoadStateOptions
	}
	pageWith := func(w *waits, urls ...string) *mockInteractionPage {
		return &mockInteractionPage{
			urlFunc: urlSequence(urls...),
			waitFunc: func(opts playwright.PageWaitForLoadStateOptions) error {
				w.opts = append(w.opts, opts)
				return nil
			},
		}
	}

	t.Run("no URL change does not wait", func(t *testing.T) {
		var w waits
		page := pageWith(&w, "https://a/")
		startAction(summarySession(page), page, "click", "#go", nil, nil, nil).summarize(context.Background())
		if len(w.opts) != 0 {
			t.Errorf("waited %d times, want 0", len(w.opts))
		}
	})

	t.Run("URL change before the settle window waits once for domcontentloaded", func(t *testing.T) {
		var w waits
		page := pageWith(&w, "https://a/", "https://b/")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		got := startAction(summarySession(page), page, "click", "#go", nil, nil, nil).summarize(ctx)
		if len(w.opts) != 1 {
			t.Fatalf("waited %d times, want 1", len(w.opts))
		}
		if w.opts[0].State == nil || *w.opts[0].State != *playwright.LoadStateDomcontentloaded {
			t.Errorf("State = %v, want domcontentloaded", w.opts[0].State)
		}
		// The element timeout (default 10 s) is clamped to the request deadline.
		if w.opts[0].Timeout == nil || *w.opts[0].Timeout <= 0 || *w.opts[0].Timeout > 2000 {
			t.Errorf("Timeout = %v, want in (0, 2000]", w.opts[0].Timeout)
		}
		if !got.Navigated {
			t.Error("Navigated = false, want true")
		}
	})

	t.Run("URL change during the settle window waits after it", func(t *testing.T) {
		var w waits
		// startAction and the first check see the old URL; the check after the
		// settle window sees the new one.
		page := pageWith(&w, "https://a/", "https://a/", "https://b/")
		got := startAction(summarySession(page), page, "click", "#go", nil, nil, nil).summarize(context.Background())
		if len(w.opts) != 1 || !got.Navigated {
			t.Errorf("waited %d times, Navigated = %v; want 1, true", len(w.opts), got.Navigated)
		}
	})

	t.Run("a failed wait still yields the summary", func(t *testing.T) {
		page := &mockInteractionPage{
			urlFunc:  urlSequence("https://a/", "https://b/"),
			waitFunc: func(playwright.PageWaitForLoadStateOptions) error { return errors.New("timeout") },
		}
		got := startAction(summarySession(page), page, "click", "#go", nil, nil, nil).summarize(context.Background())
		if !got.Navigated || got.URL != "https://b/" {
			t.Errorf("summary = %+v", got)
		}
	})

	t.Run("under a millisecond left does not wait without a timeout", func(t *testing.T) {
		var w waits
		page := pageWith(&w, "https://a/", "https://b/")
		rec := startAction(summarySession(page), page, "click", "#go", nil, nil, nil)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(500*time.Microsecond))
		defer cancel()
		rec.waitForDOMContentLoaded(ctx)
		if len(w.opts) != 0 {
			t.Errorf("waited with Timeout = %v, want no wait", *w.opts[0].Timeout)
		}
	})
}

// setSettleDelay sets the settle window for one test.
func setSettleDelay(t *testing.T, d time.Duration) {
	t.Helper()
	old := settleDelay
	settleDelay = d
	t.Cleanup(func() { settleDelay = old })
}

func TestSettle(t *testing.T) {
	t.Run("waits the settle window", func(t *testing.T) {
		setSettleDelay(t, 30*time.Millisecond)
		start := time.Now()
		(&actionRecorder{}).settle(context.Background())
		if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
			t.Errorf("settle returned after %v, want at least 30ms", elapsed)
		}
	})

	t.Run("returns when the request is cancelled", func(t *testing.T) {
		setSettleDelay(t, time.Minute)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		start := time.Now()
		(&actionRecorder{}).settle(ctx)
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("settle returned after %v, want at once", elapsed)
		}
	})
}
