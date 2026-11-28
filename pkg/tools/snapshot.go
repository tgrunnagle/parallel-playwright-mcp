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
	return redactPasswords(page, snapshot), nil
}

// passwordCheckTimeout bounds the type check of one textbox ref, in milliseconds.
// The ref was counted first, so the element exists and the check does not wait.
const passwordCheckTimeout = 1000

// redactPasswords replaces the value of every password textbox in snapshot with
// redactedValue. AI-mode snapshots print input values in clear text, passwords
// included. The renderer writes a textbox value in one of two shapes:
//
//   - textbox "Password" [ref=e3]: hunter2
//   - textbox "Password" [ref=e3]:
//   - /placeholder: Enter your password
//   - text: hunter2
//
// and wraps the key in single quotes when the name needs YAML quoting. Every
// textbox with a value in either shape is resolved through its ref; when the
// element is a password input, or its type cannot be read, the value is replaced.
// A textbox without a ref cannot be resolved, so its value is always replaced.
// In the block shape every child line except /property lines is replaced.
func redactPasswords(page playwright.Page, snapshot string) string {
	lines := strings.Split(snapshot, "\n")
	for i := 0; i < len(lines); i++ {
		tb, ok := parseTextboxLine(lines[i])
		if !ok {
			continue
		}
		var children []int
		if tb.valueStart < 0 {
			for j := i + 1; j < len(lines) && lineIndent(lines[j]) > tb.indent; j++ {
				if !strings.HasPrefix(lines[j][lineIndent(lines[j]):], "- /") {
					children = append(children, j)
				}
			}
			if len(children) == 0 {
				continue
			}
		}
		if refPattern.MatchString(tb.ref) && !isPasswordRef(page, tb.ref) {
			continue
		}
		if tb.valueStart >= 0 {
			lines[i] = lines[i][:tb.valueStart] + redactedValue
		}
		for _, j := range children {
			lines[j] = lines[j][:lineIndent(lines[j])] + "- text: " + redactedValue
		}
	}
	return strings.Join(lines, "\n")
}

// isPasswordRef reports whether the element ref resolves to a password input.
// It fails closed: a ref that matches nothing, or whose type cannot be read,
// counts as a password. The count does not wait, so a re-rendered element does
// not hold the snapshot for the request timeout.
func isPasswordRef(page playwright.Page, ref string) bool {
	loc := page.Locator(refSelector(ref))
	if n, err := loc.Count(); err != nil || n == 0 {
		slog.Debug("textbox ref did not resolve; redacting its value", "ref", ref, "error", err)
		return true
	}
	result, err := loc.Evaluate(isPasswordJS, nil, playwright.LocatorEvaluateOptions{Timeout: playwright.Float(passwordCheckTimeout)})
	if err != nil {
		slog.Debug("could not read textbox type; redacting its value", "ref", ref, "error", err)
		return true
	}
	isPassword, ok := result.(bool)
	return !ok || isPassword
}

// textboxLine is a parsed snapshot line for a textbox that has a value.
type textboxLine struct {
	// ref is the element ref from the line's attributes, empty when it has none.
	ref string
	// indent is the number of leading spaces.
	indent int
	// valueStart is the byte offset of an inline value, or -1 when the value is
	// in the child lines that follow.
	valueStart int
}

// lineIndent returns the number of leading spaces in line.
func lineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// parseTextboxLine parses one snapshot line for a textbox with a value, in either
// shape redactPasswords describes, with the key plain or single-quoted. ok is false
// for other roles, for textboxes without a value, and for lines it cannot parse.
func parseTextboxLine(line string) (textboxLine, bool) {
	indent := lineIndent(line)
	rest, found := strings.CutPrefix(line[indent:], "- ")
	if !found {
		return textboxLine{}, false
	}
	base := indent + 2

	var ref, after string
	var afterStart int
	if strings.HasPrefix(rest, "'") {
		// Single-quoted YAML key: '' is an escaped quote.
		end := -1
		for i := 1; i < len(rest); i++ {
			if rest[i] != '\'' {
				continue
			}
			if i+1 < len(rest) && rest[i+1] == '\'' {
				i++
				continue
			}
			end = i
			break
		}
		if end < 0 {
			return textboxLine{}, false
		}
		key := strings.ReplaceAll(rest[1:end], "''", "'")
		r, keyEnd, ok := textboxKeyRef(key)
		if !ok || keyEnd != len(key) {
			return textboxLine{}, false
		}
		ref, after, afterStart = r, rest[end+1:], base+end+1
	} else {
		r, keyEnd, ok := textboxKeyRef(rest)
		if !ok {
			return textboxLine{}, false
		}
		ref, after, afterStart = r, rest[keyEnd:], base+keyEnd
	}

	switch {
	case strings.HasPrefix(after, ": "):
		return textboxLine{ref: ref, indent: indent, valueStart: afterStart + 2}, true
	case after == ":":
		return textboxLine{ref: ref, indent: indent, valueStart: -1}, true
	default:
		return textboxLine{}, false
	}
}

// textboxKeyRef parses a snapshot key of the form
//
//	textbox "Name" [attr] [ref=e3]
//
// at the start of key and returns its ref and the byte offset where the key ends.
// ref is empty when the line has none: AI mode leaves refs off elements that are
// not visible or do not receive pointer events, but still prints their values.
// The quoted name is skipped with its escapes, so a name cannot pose as an
// attribute or a value.
func textboxKeyRef(key string) (string, int, bool) {
	const role = "textbox"
	rest, found := strings.CutPrefix(key, role)
	if !found {
		return "", 0, false
	}
	i := 0
	if strings.HasPrefix(rest, ` "`) {
		i = 2
		closed := false
		for i < len(rest) && !closed {
			switch rest[i] {
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

	ref := ""
	for strings.HasPrefix(rest[i:], " [") {
		end := strings.IndexByte(rest[i:], ']')
		if end < 0 {
			return "", 0, false
		}
		if r, found := strings.CutPrefix(rest[i+2:i+end], "ref="); found {
			ref = r
		}
		i += end + 1
	}
	return ref, len(role) + i, true
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
