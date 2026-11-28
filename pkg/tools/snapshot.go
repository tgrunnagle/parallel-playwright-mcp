// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
)

// redactedValue replaces a password field's value in a snapshot.
const redactedValue = "•••"

// passwordPredicateJS is true when the element e is a password input. The ref
// check and the value scrub share it, so they always agree on what a password is.
const passwordPredicateJS = `String(e.type || '').toLowerCase() === 'password'`

// isPasswordJS reports whether an element is a password input.
const isPasswordJS = `e => ` + passwordPredicateJS

// passwordValuesJS returns the non-empty values of the password inputs among els.
const passwordValuesJS = `els => els.filter(e => ` + passwordPredicateJS + ` && e.value).map(e => e.value)`

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

	// Password values are read before and after the snapshot, so a value typed
	// while it is taken is still scrubbed. The reads share the snapshot's timeout.
	var deadline time.Time
	if timeout != nil {
		deadline = time.Now().Add(time.Duration(*timeout * float64(time.Millisecond)))
	}
	before, err := passwordValues(page, deadline)
	if err != nil {
		return "", err
	}

	var snapshot string
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
	after, err := passwordValues(page, deadline)
	if err != nil {
		return "", err
	}
	return redactPasswords(page, snapshot, append(before, after...)), nil
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
// A textbox without a ref cannot be resolved, so its value is always replaced,
// and so is the value of a textbox line that cannot be parsed. In the block
// shape every child line except /property lines is replaced.
//
// The renderer prints the value of every input whatever its role, so a password
// input with an explicit role is not a textbox line. Every occurrence of a value
// in passwords, the values of the page's password inputs, is therefore also
// scrubbed from the whole snapshot (see scrubValues).
func redactPasswords(page playwright.Page, snapshot string, passwords []string) string {
	lines := strings.Split(snapshot, "\n")
	for i := 0; i < len(lines); i++ {
		tb, status := parseTextboxLine(lines[i])
		switch status {
		case textboxValue:
		case textboxUnparsed:
			var ok bool
			if tb, ok = unparsedTextboxLine(lines[i]); !ok {
				continue
			}
		default:
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
	scrubValues(lines, passwords)
	return strings.Join(lines, "\n")
}

// passwordReadAttempts is how many times passwordValues reads a frame whose
// document is replaced while it is read.
const passwordReadAttempts = 3

// passwordValues returns the non-empty values of the password inputs in every
// frame of the page, open shadow roots included. A value made only of whitespace
// is left out: scrubbing it would blank the snapshot's indentation, and a textbox
// holding it is still redacted through its ref. A frame that detached while it
// was read is skipped, and a frame that navigated while it was read is read
// again, up to passwordReadAttempts times. Any other failure, or a deadline
// (when not zero) passed before every frame was read, is returned, so no
// snapshot is served whose password values are unknown.
func passwordValues(page playwright.Page, deadline time.Time) ([]string, error) {
	var values []string
	for _, frame := range page.Frames() {
		var result any
		var err error
		for attempt := 1; ; attempt++ {
			if !deadline.IsZero() && time.Now().After(deadline) {
				return nil, errors.NewTimeoutError("reading password fields for the accessibility snapshot", 0)
			}
			result, err = frame.Locator("input").EvaluateAll(passwordValuesJS)
			if err == nil || frame.IsDetached() || attempt == passwordReadAttempts || !isNavigationRace(err) {
				break
			}
			slog.Debug("frame navigated while its password fields were read; reading again", "attempt", attempt, "error", err)
		}
		if err != nil {
			if frame.IsDetached() {
				continue
			}
			return nil, fmt.Errorf("failed to read password fields for the accessibility snapshot: %w", err)
		}
		items, _ := result.([]any)
		for _, item := range items {
			if v, ok := item.(string); ok && strings.TrimSpace(v) != "" {
				values = append(values, v)
			}
		}
	}
	return values, nil
}

// isNavigationRace reports whether err is Playwright's error for a script whose
// document was replaced while it ran.
func isNavigationRace(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "execution context was destroyed") || strings.Contains(msg, "because of a navigation")
}

// refToken matches the ref attribute of a snapshot line.
var refToken = regexp.MustCompile(`\[ref=[^\]]*\]`)

// scrubValues replaces every occurrence of a value in passwords, in each form the
// renderer can print it, with redactedValue. A line's indentation, list marker and
// ref attribute are left alone, so the tree and its refs stay usable even when a
// password is short enough to match them. A short password therefore also blanks
// matching text elsewhere in the snapshot: redaction errs towards hiding.
func scrubValues(lines []string, passwords []string) {
	forms := passwordForms(passwords)
	if len(forms) == 0 {
		return
	}
	pairs := make([]string, 0, 2*len(forms))
	for _, f := range forms {
		pairs = append(pairs, f, redactedValue)
	}
	replacer := strings.NewReplacer(pairs...)

	for i, line := range lines {
		start := lineIndent(line)
		if strings.HasPrefix(line[start:], "- ") {
			start += 2
		}
		var b strings.Builder
		b.WriteString(line[:start])
		pos := start
		for _, m := range refToken.FindAllStringIndex(line[start:], -1) {
			b.WriteString(replacer.Replace(line[pos : start+m[0]]))
			b.WriteString(line[start+m[0] : start+m[1]])
			pos = start + m[1]
		}
		b.WriteString(replacer.Replace(line[pos:]))
		lines[i] = b.String()
	}
}

// passwordForms returns each password as the renderer can print it: as typed and
// with whitespace normalized (see normalizeWhiteSpace), each raw, escaped as a
// double-quoted YAML value or a JSON name, and with single quotes doubled as in a
// quoted key. The forms are distinct and longest first, so a longer form wins
// over a shorter one at the same position.
func passwordForms(passwords []string) []string {
	seen := map[string]bool{}
	var forms []string
	add := func(f string) {
		if f != "" && !seen[f] {
			seen[f] = true
			forms = append(forms, f)
		}
	}
	for _, p := range passwords {
		for _, v := range []string{p, normalizeWhiteSpace(p)} {
			for _, f := range []string{v, yamlEscapeValue(v), jsonEscapeName(v)} {
				add(f)
				add(strings.ReplaceAll(f, "'", "''"))
			}
		}
	}
	sort.SliceStable(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })
	return forms
}

// normalizeWhiteSpace normalizes s as the renderer does input values and names
// (normalizeWhiteSpace in Playwright's stringUtils): zero-width spaces and soft
// hyphens are removed, then JavaScript whitespace is trimmed and each run of it
// collapsed to one space.
func normalizeWhiteSpace(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\u200b' || r == '\u00ad' {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.FieldsFunc(s, isJSSpace), " ")
}

// isJSSpace reports whether r matches \s in a JavaScript regular expression.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\u00a0', '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	}
	return r >= '\u2000' && r <= '\u200a'
}

// yamlEscapeValue escapes s as the renderer does inside a double-quoted value:
// backslash, double quote and control characters.
func yamlEscapeValue(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// jsonEscapeName escapes s as JSON.stringify does inside the quoted name of a key.
func jsonEscapeName(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return s
	}
	quoted := strings.TrimSuffix(b.String(), "\n")
	return quoted[1 : len(quoted)-1]
}

// unparsedTextboxLine handles a line that parseTextboxLine reports as
// textboxUnparsed. It fails closed: what follows the line's last ref attribute,
// or, when the line has no ref, its first ": ", is treated as the value of a
// textbox without a ref, which redactPasswords always replaces. ok is false when
// the line has no value: nothing follows its last ref, or it has no ": " and
// does not end with ":".
func unparsedTextboxLine(line string) (textboxLine, bool) {
	indent := lineIndent(line)
	rest, found := strings.CutPrefix(line[indent:], "- ")
	if !found {
		return textboxLine{}, false
	}
	base := indent + 2
	if ms := refToken.FindAllStringIndex(rest, -1); len(ms) > 0 {
		after := ms[len(ms)-1][1]
		tail := strings.TrimPrefix(rest[after:], "'")
		tailStart := base + len(rest) - len(tail)
		switch {
		case tail == "":
			return textboxLine{}, false
		case tail == ":":
			return textboxLine{indent: indent, valueStart: -1}, true
		case strings.HasPrefix(tail, ": "):
			return textboxLine{indent: indent, valueStart: tailStart + 2}, true
		}
	}
	if idx := strings.Index(rest, ": "); idx >= 0 {
		return textboxLine{indent: indent, valueStart: base + idx + 2}, true
	}
	if strings.HasSuffix(rest, ":") {
		return textboxLine{indent: indent, valueStart: -1}, true
	}
	return textboxLine{}, false
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

// textboxStatus classifies a snapshot line for redactPasswords.
type textboxStatus int

const (
	// notTextbox is a line for another role, or not a node line.
	notTextbox textboxStatus = iota
	// textboxEmpty is a textbox key with no value.
	textboxEmpty
	// textboxValue is a textbox with an inline or block value.
	textboxValue
	// textboxUnparsed is a line whose key starts with the textbox role but
	// cannot be read.
	textboxUnparsed
)

// hasTextboxRole reports whether key starts with the textbox role.
func hasTextboxRole(key string) bool {
	rest, found := strings.CutPrefix(key, "textbox")
	return found && (rest == "" || strings.ContainsAny(rest[:1], " :'"))
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
// shape redactPasswords describes, with the key plain or single-quoted. The line
// is returned only with textboxValue; the status tells the other cases apart.
func parseTextboxLine(line string) (textboxLine, textboxStatus) {
	indent := lineIndent(line)
	rest, found := strings.CutPrefix(line[indent:], "- ")
	if !found || !hasTextboxRole(strings.TrimPrefix(rest, "'")) {
		return textboxLine{}, notTextbox
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
			return textboxLine{}, textboxUnparsed
		}
		key := strings.ReplaceAll(rest[1:end], "''", "'")
		r, keyEnd, ok := textboxKeyRef(key)
		if !ok || keyEnd != len(key) {
			return textboxLine{}, textboxUnparsed
		}
		ref, after, afterStart = r, rest[end+1:], base+end+1
	} else {
		r, keyEnd, ok := textboxKeyRef(rest)
		if !ok {
			return textboxLine{}, textboxUnparsed
		}
		ref, after, afterStart = r, rest[keyEnd:], base+keyEnd
	}

	switch {
	case strings.HasPrefix(after, ": "):
		return textboxLine{ref: ref, indent: indent, valueStart: afterStart + 2}, textboxValue
	case after == ":":
		return textboxLine{ref: ref, indent: indent, valueStart: -1}, textboxValue
	case after == "":
		return textboxLine{}, textboxEmpty
	default:
		return textboxLine{}, textboxUnparsed
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
	if strings.HasPrefix(rest, " /") {
		// A name that starts and ends with "/" is printed without quotes.
		end, ok := regexNameEnd(rest)
		if !ok {
			return "", 0, false
		}
		i = end
	} else if strings.HasPrefix(rest, ` "`) {
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

// regexNameEnd returns the offset just past an unquoted /.../ name at the start
// of rest (` /name/ [attr]: value`): the first closing "/" that is followed by
// attributes and then the end of the key.
func regexNameEnd(rest string) (int, bool) {
	for j := 3; j < len(rest); j++ {
		if rest[j] != '/' {
			continue
		}
		k := j + 1
		for strings.HasPrefix(rest[k:], " [") {
			end := strings.IndexByte(rest[k:], ']')
			if end < 0 {
				break
			}
			k += end + 1
		}
		if t := rest[k:]; t == "" || t == ":" || strings.HasPrefix(t, ": ") || strings.HasPrefix(t, "'") {
			return j + 1, true
		}
	}
	return 0, false
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
// f<k>e<n> for an element in an iframe or a later document. This is the format of
// the Playwright driver that go.mod pins through playwright-go (1.62.1). Older
// drivers emitted refs such as s1e3, which this rejects, and redactPasswords then
// treats every textbox as ref-less, so check the format when upgrading.
var refPattern = regexp.MustCompile(`^(f[0-9]+)?e[0-9]+$`)

// refSelector is the Playwright selector that resolves an element ref.
func refSelector(ref string) string {
	return "aria-ref=" + ref
}
