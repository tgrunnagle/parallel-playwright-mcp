// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// settleDelay is how long an action tool waits after the action, and after any
// navigation it caused reaches domcontentloaded, before reading the page state.
// Console messages and requests that arrive later show up in the next summary.
// A variable so unit tests can turn it off.
var settleDelay = 150 * time.Millisecond

// maxSummaryEntries caps each list in a state summary.
const maxSummaryEntries = 5

// maxSummaryEntryChars caps each list entry in a state summary.
const maxSummaryEntryChars = 200

// includeSnapshotDescription describes the includeSnapshot argument of the action tools.
const includeSnapshotDescription = "When true, append a second content block with the page's ARIA snapshot " +
	"(the get_accessibility_tree format, with element refs) taken after the action settles, cut to the response budget"

// stateSummary describes the page after an action tool ran.
type stateSummary struct {
	// Action is the tool name.
	Action string `json:"action"`
	// Target is the selector or ref=<ref> acted on, empty for page-level actions.
	Target string `json:"target,omitempty"`
	// URL is the page URL after the action; for navigate, the final URL after redirects.
	URL string `json:"url"`
	// Title is the page title, omitted when it could not be read.
	Title *string `json:"title,omitempty"`
	// Navigated reports whether the action moved the page to another document or URL.
	Navigated bool `json:"navigated"`
	// HTTPStatus is the main document's response status, for navigation tools only.
	HTTPStatus *int `json:"httpStatus,omitempty"`
	// NewConsoleErrors lists console errors logged since the action started.
	NewConsoleErrors []string `json:"newConsoleErrors"`
	// FailedRequests lists requests since the action started that got status 400
	// or above or no response, without their query strings.
	FailedRequests []string `json:"failedRequests"`
	// More counts list entries left out by the cap, keyed by list name.
	More map[string]int `json:"more,omitempty"`
}

// actionRecorder captures the page state before an action so the summary after it
// can report what changed.
type actionRecorder struct {
	sess        *session.BrowserSession
	page        playwright.Page
	action      string
	target      string
	urlBefore   string
	consoleMark int
	networkMark int
	waitTimeout time.Duration

	// includeSnapshot appends a snapshot block to the result, cut to budget.
	includeSnapshot bool
	budget          int

	// navigation marks a navigation tool; response is its main document response.
	navigation bool
	response   playwright.Response
}

// startAction records the page state before an action. Call it after validation
// and before the action runs. args supplies includeSnapshot.
func startAction(sess *session.BrowserSession, page playwright.Page, action, target string,
	args map[string]any, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) *actionRecorder {
	if timeoutConfig == nil {
		timeoutConfig = DefaultTimeoutConfig()
	}
	includeSnapshot, _ := args["includeSnapshot"].(bool)
	r := &actionRecorder{
		sess:            sess,
		page:            page,
		action:          action,
		target:          target,
		urlBefore:       page.URL(),
		waitTimeout:     timeoutConfig.Element,
		includeSnapshot: includeSnapshot,
		budget:          responseConfig.budget(),
	}
	if sess.ConsoleLogs != nil {
		r.consoleMark = sess.ConsoleLogs.Total()
	}
	if sess.NetworkLogs != nil {
		r.networkMark = sess.NetworkLogs.Total()
	}
	return r
}

// navigated records the response of a navigation tool.
func (r *actionRecorder) navigated(resp playwright.Response) {
	r.navigation = true
	r.response = resp
}

// finish waits for the page to settle and returns the action's result: line, then
// a newline and the state summary JSON, as text, with the summary also as
// structured content. With includeSnapshot, a second block holds the snapshot.
func (r *actionRecorder) finish(ctx context.Context, line string) *mcp.CallToolResult {
	summary := r.summarize(ctx)
	var result *mcp.CallToolResult
	if data, err := json.Marshal(summary); err != nil {
		slog.Error("failed to serialize state summary", "action", r.action, "error", err)
		result = mcp.NewToolResultText(line)
	} else {
		result = mcp.NewToolResultStructured(summary, line+"\n"+string(data))
	}
	if r.includeSnapshot {
		result.Content = append(result.Content, r.snapshotBlocks(ctx)...)
	}
	return result
}

// snapshotBlocks takes the post-action snapshot and returns it as a content block
// cut to the budget, followed by a notice block when it was cut. The action has
// already succeeded, so a snapshot failure is reported in the block, not as an
// error result.
func (r *actionRecorder) snapshotBlocks(ctx context.Context) []mcp.Content {
	snapshot, err := takeSnapshot(r.page, "", 0, PlaywrightTimeoutFromContext(ctx))
	if err != nil {
		slog.Warn("snapshot after action failed", "action", r.action, "error", err)
		return []mcp.Content{mcp.NewTextContent("[snapshot unavailable: " + errors.FormatErrorForTool(err) + "]")}
	}
	if strings.TrimSpace(snapshot) == "" {
		return []mcp.Content{mcp.NewTextContent("No accessible elements found.")}
	}
	total := utf8.RuneCountInString(snapshot)
	if total <= r.budget {
		return []mcp.Content{mcp.NewTextContent(snapshot)}
	}
	return []mcp.Content{
		mcp.NewTextContent(runeSlice(snapshot, 0, r.budget)),
		mcp.NewTextContent(fmt.Sprintf("[truncated: %d more characters; call get_accessibility_tree with offset=%d]", total-r.budget, r.budget)),
	}
}

// summarize waits for the page to settle and builds the state summary.
func (r *actionRecorder) summarize(ctx context.Context) stateSummary {
	if r.page.URL() != r.urlBefore {
		r.waitForDOMContentLoaded(ctx)
	}
	r.settle(ctx)
	return buildStateSummary(r)
}

// waitForDOMContentLoaded waits for the new document after a URL change, bounded
// by the element timeout and the request deadline. A wait that fails leaves the
// summary to report whatever state the page is in.
func (r *actionRecorder) waitForDOMContentLoaded(ctx context.Context) {
	timeout := r.waitTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = min(timeout, time.Until(deadline))
	}
	if timeout <= 0 {
		return
	}
	err := r.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateDomcontentloaded,
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
	})
	if err != nil {
		slog.Debug("page did not reach domcontentloaded after action", "action", r.action, "error", err)
	}
}

// settle waits settleDelay, or until the request is cancelled.
func (r *actionRecorder) settle(ctx context.Context) {
	if settleDelay <= 0 {
		return
	}
	timer := time.NewTimer(settleDelay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// buildStateSummary reads the page state and the log entries since the action
// started. A title that cannot be read is omitted; it never fails the action.
func buildStateSummary(r *actionRecorder) stateSummary {
	summary := stateSummary{
		Action:           r.action,
		Target:           r.target,
		URL:              r.page.URL(),
		NewConsoleErrors: []string{},
		FailedRequests:   []string{},
	}
	if title, err := r.page.Title(); err == nil {
		summary.Title = &title
	} else {
		slog.Debug("could not read page title for state summary", "action", r.action, "error", err)
	}

	summary.Navigated = summary.URL != r.urlBefore
	if r.navigation && r.response != nil {
		summary.Navigated = true
		status := r.response.Status()
		summary.HTTPStatus = &status
	}

	var consoleErrors []string
	if r.sess.ConsoleLogs != nil {
		for _, entry := range r.sess.ConsoleLogs.Since(r.consoleMark) {
			if entry.Level == session.ConsoleLogLevelError {
				consoleErrors = append(consoleErrors, entry.Text)
			}
		}
	}
	var failed []string
	if r.sess.NetworkLogs != nil {
		for _, entry := range r.sess.NetworkLogs.Since(r.networkMark) {
			if entry.Status == 0 {
				failed = append(failed, fmt.Sprintf("%s %s (no response)", entry.Method, stripQuery(entry.URL)))
			} else if entry.Status >= 400 {
				failed = append(failed, fmt.Sprintf("%s %s %d", entry.Method, stripQuery(entry.URL), entry.Status))
			}
		}
	}

	summary.NewConsoleErrors = capEntries(consoleErrors, "newConsoleErrors", &summary.More)
	summary.FailedRequests = capEntries(failed, "failedRequests", &summary.More)
	return summary
}

// capEntries keeps the first maxSummaryEntries entries, each cut to
// maxSummaryEntryChars, and records how many were left out under name in more.
func capEntries(entries []string, name string, more *map[string]int) []string {
	kept := make([]string, 0, min(len(entries), maxSummaryEntries))
	for i, entry := range entries {
		if i == maxSummaryEntries {
			if *more == nil {
				*more = map[string]int{}
			}
			(*more)[name] = len(entries) - maxSummaryEntries
			break
		}
		kept = append(kept, cutRunes(entry, maxSummaryEntryChars))
	}
	return kept
}

// stripQuery removes the query string, fragment, and user info from a request
// URL, since they may carry tokens or credentials.
func stripQuery(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		if i := strings.IndexAny(raw, "?#"); i >= 0 {
			return raw[:i]
		}
		return raw
	}
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	u.User = nil
	return u.String()
}
