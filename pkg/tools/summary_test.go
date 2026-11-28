package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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

		rec := startAction(sess, page, "click", "#go", nil)
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
		rec := startAction(sess, page, "click", "#go", nil)
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
		rec := startAction(sess, page, "click", "#go", nil)

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
		got := buildStateSummary(startAction(summarySession(page), page, "hover", "#x", nil))
		if got.Title == nil || *got.Title != "" {
			t.Errorf("Title = %v, want empty string", got.Title)
		}
	})

	t.Run("caps lists and cuts long entries", func(t *testing.T) {
		page := &mockInteractionPage{}
		sess := summarySession(page)
		rec := startAction(sess, page, "click", "#go", nil)
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
		rec := startAction(summarySession(page), page, "click", "a", nil)
		url = "https://example.com/b"
		got := buildStateSummary(rec)
		if !got.Navigated || got.URL != "https://example.com/b" {
			t.Errorf("Navigated = %v, URL = %s", got.Navigated, got.URL)
		}
	})

	t.Run("navigation tool reports the response status", func(t *testing.T) {
		page := &mockPage{}
		rec := startAction(summarySession(page), page, "reload", "", nil)
		rec.navigated(&mockResponse{statusFunc: func() int { return 201 }})
		got := buildStateSummary(rec)
		if !got.Navigated || got.HTTPStatus == nil || *got.HTTPStatus != 201 {
			t.Errorf("Navigated = %v, HTTPStatus = %v", got.Navigated, got.HTTPStatus)
		}
	})

	t.Run("navigation tool without a response has no status", func(t *testing.T) {
		page := &mockPage{}
		rec := startAction(summarySession(page), page, "go_back", "", nil)
		rec.navigated(nil)
		got := buildStateSummary(rec)
		if got.Navigated || got.HTTPStatus != nil {
			t.Errorf("Navigated = %v, HTTPStatus = %v", got.Navigated, got.HTTPStatus)
		}
	})

	t.Run("session without buffers", func(t *testing.T) {
		page := &mockInteractionPage{}
		sess := &session.BrowserSession{ID: "s", Pages: map[string]playwright.Page{}}
		got := buildStateSummary(startAction(sess, page, "click", "#go", nil))
		if got.NewConsoleErrors == nil || got.FailedRequests == nil {
			t.Error("lists should be empty, not null")
		}
	})
}

func TestActionResultShape(t *testing.T) {
	page := &mockInteractionPage{}
	rec := startAction(summarySession(page), page, "click", "#button", nil)
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
