// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// targetParams names both targeting parameters in validation errors.
const targetParams = "selector, ref"

// refParamDescription describes the ref argument of the interaction tools.
const refParamDescription = "Element ref from get_accessibility_tree (or an action's includeSnapshot), such as e3 or f1e2. Use instead of selector"

// elementTarget is the element an interaction tool acts on: a selector, or an
// element ref from a snapshot.
type elementTarget struct {
	// Selector is the Playwright selector that locates the element: the caller's
	// selector, or aria-ref=<ref> for a ref.
	Selector string
	// Ref is the element ref, empty for a selector target.
	Ref string
	// Label names the target in messages: the selector, or ref=<ref>.
	Label string
}

// isZero reports whether no target was given.
func (t elementTarget) isZero() bool {
	return t.Selector == ""
}

// resolveTarget reads the selector and ref arguments. Exactly one is required;
// when optional is true, neither is also accepted and yields the zero target. A
// ref must have the e<n> or f<k>e<n> form, so it cannot smuggle in another selector
// engine or a chained selector. Validation happens before any page access.
func resolveTarget(args map[string]any, optional bool) (elementTarget, error) {
	selector, _ := args["selector"].(string)
	ref, _ := args["ref"].(string)

	switch {
	case selector != "" && ref != "":
		return elementTarget{}, errors.NewInvalidParamsError(targetParams, "provide exactly one of selector or ref, not both")
	case selector == "" && ref == "":
		if optional {
			return elementTarget{}, nil
		}
		return elementTarget{}, errors.NewInvalidParamsError(targetParams, "exactly one of selector or ref is required")
	case ref != "":
		if !refPattern.MatchString(ref) {
			return elementTarget{}, errors.NewInvalidParamsError("ref",
				fmt.Sprintf("%q is not an element ref; refs look like e3 or f1e2", ref))
		}
		return elementTarget{Selector: refSelector(ref), Ref: ref, Label: "ref=" + ref}, nil
	default:
		return elementTarget{Selector: selector, Label: selector}, nil
	}
}

// locate returns the locator for the target. A ref that resolves to no element
// fails at once with the stale-ref error, instead of waiting out the timeout, and
// so does one whose frame has detached. Any other failure to count the ref, such
// as a closed page, is an interaction error: a new snapshot would not help.
func (t elementTarget) locate(page playwright.Page) (playwright.Locator, error) {
	loc := page.Locator(t.Selector)
	if t.Ref == "" {
		return loc, nil
	}
	count, err := loc.Count()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "detached") {
			stale := errors.NewStaleRefError(t.Ref)
			stale.Err = err
			return nil, stale
		}
		return nil, errors.WrapInteractionError("resolve ref", t.Label, err)
	}
	if count == 0 {
		return nil, errors.NewStaleRefError(t.Ref)
	}
	return loc, nil
}

// targetFromArgs reads the target of an interaction tool call, as resolveTarget
// does, and returns a logged error result when the arguments are invalid.
func targetFromArgs(args map[string]any, optional bool, tool, sessionID string) (elementTarget, *mcp.CallToolResult) {
	target, err := resolveTarget(args, optional)
	if err != nil {
		slog.Error("invalid element target", "tool", tool, "sessionID", sessionID, "error", err)
		return elementTarget{}, errorResult(err)
	}
	return target, nil
}

// elementAction is an interaction tool call that is ready to act on its target.
type elementAction struct {
	// page is the session's active page.
	page playwright.Page
	// locator locates the target; nil for the zero target.
	locator playwright.Locator
	// rec records the action for its state summary.
	rec *actionRecorder
}

// beginElementAction looks up the session's active page, starts recording the
// action, and locates target unless it is the zero target. When a step fails it
// returns a logged error result instead.
func beginElementAction(mgr session.BrowserSessionManager, mcpSessionID, sessionID, tool string, target elementTarget,
	args map[string]any, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) (*elementAction, *mcp.CallToolResult) {
	sess, ok := mgr.GetSession(mcpSessionID, sessionID)
	if !ok {
		slog.Error("session not found", "tool", tool, "sessionID", sessionID)
		return nil, newInteractionSessionNotFoundError(sessionID)
	}

	page := sess.ActivePage()
	if page == nil {
		slog.Error("no active page in session", "tool", tool, "sessionID", sessionID)
		return nil, noActivePageResult()
	}

	act := &elementAction{page: page, rec: startAction(sess, page, tool, target.Label, args, timeoutConfig, responseConfig)}
	if target.isZero() {
		return act, nil
	}
	locator, err := target.locate(page)
	if err != nil {
		slog.Error("element target did not resolve", "tool", tool, "sessionID", sessionID, "target", target.Label, "error", err)
		return nil, errorResult(err)
	}
	act.locator = locator
	return act, nil
}
