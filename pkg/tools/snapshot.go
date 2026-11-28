// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
)

// redactedValue replaces a password field's value in a snapshot.
const redactedValue = "•••"

// isPasswordJS reports whether an element is a password input.
const isPasswordJS = `e => String(e.type || '').toLowerCase() === 'password'`

// describeMatchesJS describes the first matches of a locator as tag and text.
const describeMatchesJS = `(els, n) => els.slice(0, n).map(e => ({
	tag: e.tagName.toLowerCase(),
	text: String(e.innerText || e.textContent || '').replace(/\s+/g, ' ').trim(),
}))`

// maxDescribedMatches is how many matches a multi-match error describes.
const maxDescribedMatches = 3

// maxMatchTextChars is how many characters of each match's text a multi-match
// error includes.
const maxMatchTextChars = 80

// takeSnapshot returns the AI-mode ARIA snapshot of the page, or of the element
// selector matches when selector is non-empty, with password values redacted.
// depth limits the tree depth when positive. A selector matching no element fails
// at once with -32002 instead of waiting for the timeout.
func takeSnapshot(page playwright.Page, selector string, depth int, timeout *float64) (string, error) {
	var depthOpt *int
	if depth > 0 {
		depthOpt = playwright.Int(depth)
	}

	var snapshot string
	var err error
	if selector == "" {
		snapshot, err = page.AriaSnapshot(playwright.PageAriaSnapshotOptions{
			Mode:    playwright.AriaSnapshotModeAi,
			Depth:   depthOpt,
			Timeout: timeout,
		})
	} else {
		loc := page.Locator(selector)
		count, countErr := loc.Count()
		if countErr != nil {
			return "", errors.WrapElementNotFoundError(selector, 0, countErr)
		}
		switch {
		case count == 0:
			return "", errors.NewElementNotFoundError(selector, 0)
		case count > 1:
			return "", multipleMatchError(loc, selector, count, nil)
		}
		snapshot, err = loc.AriaSnapshot(playwright.LocatorAriaSnapshotOptions{
			Mode:    playwright.AriaSnapshotModeAi,
			Depth:   depthOpt,
			Timeout: timeout,
		})
	}
	if err != nil {
		if isTimeoutError(err) {
			return "", errors.WrapTimeoutError("accessibility snapshot", 0, err)
		}
		return "", fmt.Errorf("failed to take accessibility snapshot: %w", err)
	}
	return redactPasswords(page, snapshot, timeout), nil
}

// redactPasswords replaces the value of every password textbox in snapshot with
// redactedValue. AI-mode snapshots print input values in clear text, passwords
// included. Each textbox line with a value is resolved through its ref; when the
// element's type cannot be read, the value is redacted anyway.
func redactPasswords(page playwright.Page, snapshot string, timeout *float64) string {
	lines := strings.Split(snapshot, "\n")
	for i, line := range lines {
		ref, valueStart, ok := textboxValue(line)
		if !ok {
			continue
		}
		if !refPattern.MatchString(ref) || isPasswordRef(page, ref, timeout) {
			lines[i] = line[:valueStart] + redactedValue
		}
	}
	return strings.Join(lines, "\n")
}

// isPasswordRef reports whether the element ref resolves to a password input.
// It fails closed: an error resolving the element counts as a password.
func isPasswordRef(page playwright.Page, ref string, timeout *float64) bool {
	result, err := page.Locator(refSelector(ref)).Evaluate(isPasswordJS, nil, playwright.LocatorEvaluateOptions{Timeout: timeout})
	if err != nil {
		slog.Debug("could not resolve textbox ref; redacting its value", "ref", ref, "error", err)
		return true
	}
	isPassword, ok := result.(bool)
	return !ok || isPassword
}

// textboxValue parses one snapshot line of the form
//
//   - textbox "Name" [attr] [ref=e3]: value
//
// and returns the ref and the byte offset where the value starts. ok is false for
// other roles, for textboxes without a value, and for lines it cannot parse. The
// quoted name is skipped with its escapes, so a name cannot pose as the value.
func textboxValue(line string) (ref string, valueStart int, ok bool) {
	i := len(line) - len(strings.TrimLeft(line, " "))
	const prefix = "- textbox"
	if !strings.HasPrefix(line[i:], prefix) {
		return "", 0, false
	}
	i += len(prefix)

	if strings.HasPrefix(line[i:], ` "`) {
		i += 2
		closed := false
		for i < len(line) && !closed {
			switch line[i] {
			case '\\':
				i += 2
			case '"':
				i++
				closed = true
			default:
				i++
			}
		}
		if !closed {
			return "", 0, false
		}
	}

	for strings.HasPrefix(line[i:], " [") {
		end := strings.IndexByte(line[i:], ']')
		if end < 0 {
			return "", 0, false
		}
		if r, found := strings.CutPrefix(line[i+2:i+end], "ref="); found {
			ref = r
		}
		i += end + 1
	}

	if ref == "" || !strings.HasPrefix(line[i:], ": ") {
		return "", 0, false
	}
	return ref, i + 2, true
}

// multipleMatchError builds the -32002 error for a selector that matched count
// elements, describing the first maxDescribedMatches by tag and text. The
// Playwright cause is kept only when the matches cannot be described, since it
// repeats them at length.
func multipleMatchError(loc playwright.Locator, selector string, count int, cause error) error {
	var described []string
	var result any
	if loc != nil {
		var err error
		if result, err = loc.EvaluateAll(describeMatchesJS, maxDescribedMatches); err != nil {
			slog.Debug("could not describe selector matches", "selector", selector, "error", err)
		}
	}
	if items, ok := result.([]any); ok {
		for _, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			tag, _ := m["tag"].(string)
			text, _ := m["text"].(string)
			described = append(described, fmt.Sprintf("<%s> %q", tag, cutRunes(text, maxMatchTextChars)))
		}
	}
	if len(described) > 0 {
		cause = nil
	}
	return errors.NewMultipleMatchError(selector, count, described, cause)
}

// cutRunes returns s cut to at most n runes, ending with "…" when cut.
func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return s[:runePrefixEnd(s, n-1)] + "…"
}

// refPattern is the form of an element ref in an AI-mode snapshot: e<n>, or
// f<k>e<n> for an element in an iframe or a later document.
var refPattern = regexp.MustCompile(`^(f[0-9]+)?e[0-9]+$`)

// refSelector is the Playwright selector that resolves an element ref.
func refSelector(ref string) string {
	return "aria-ref=" + ref
}
