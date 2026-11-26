// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
)

// DefaultMaxResponseChars is the default response budget: the maximum number of
// characters (Unicode code points) a content tool returns in one call.
const DefaultMaxResponseChars = 40000

// DefaultListLimit is the default number of items a list content tool returns.
const DefaultListLimit = 50

// evaluateTruncatedNotice is the pagination block evaluate returns when its result is cut.
const evaluateTruncatedNotice = "[truncated: result JSON is incomplete; narrow the expression]"

// ResponseConfig holds the response budget applied to content tools.
type ResponseConfig struct {
	// MaxResponseChars is the maximum number of characters (runes) a content tool
	// returns in its content block. Per-call maxChars values above it are clamped.
	MaxResponseChars int
}

// DefaultResponseConfig returns a ResponseConfig with the default budget.
func DefaultResponseConfig() *ResponseConfig {
	return &ResponseConfig{MaxResponseChars: DefaultMaxResponseChars}
}

// budget returns the configured response budget, falling back to the default
// when the config is nil or unset.
func (c *ResponseConfig) budget() int {
	if c == nil || c.MaxResponseChars <= 0 {
		return DefaultMaxResponseChars
	}
	return c.MaxResponseChars
}

// intParam reads an optional integer argument. It returns def when the argument is
// absent or null, and an InvalidParamsError when the value is not an integer or is
// below minValue.
func intParam(args map[string]any, name string, def, minValue int) (int, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return def, nil
	}
	reason := "must be a non-negative integer"
	if minValue > 0 {
		reason = "must be a positive integer"
	}
	var v int
	switch n := raw.(type) {
	case float64:
		if n != math.Trunc(n) || n > math.MaxInt32 || n < math.MinInt32 {
			return 0, errors.NewInvalidParamsError(name, reason)
		}
		v = int(n)
	case int:
		v = n
	case int64:
		v = int(n)
	default:
		return 0, errors.NewInvalidParamsError(name, reason)
	}
	if v < minValue {
		return 0, errors.NewInvalidParamsError(name, reason)
	}
	return v, nil
}

// textPageParams parses maxChars and offset for a text content tool. maxChars
// defaults to the budget and is clamped to it.
func textPageParams(args map[string]any, budget int) (maxChars, offset int, err error) {
	maxChars, err = maxCharsParam(args, budget)
	if err != nil {
		return 0, 0, err
	}
	offset, err = intParam(args, "offset", 0, 0)
	if err != nil {
		return 0, 0, err
	}
	return maxChars, offset, nil
}

// maxCharsParam parses maxChars, defaulting to the budget and clamping to it.
func maxCharsParam(args map[string]any, budget int) (int, error) {
	maxChars, err := intParam(args, "maxChars", budget, 1)
	if err != nil {
		return 0, err
	}
	return min(maxChars, budget), nil
}

// listPageParams parses limit and offset for a list content tool.
func listPageParams(args map[string]any) (limit, offset int, err error) {
	limit, err = intParam(args, "limit", DefaultListLimit, 1)
	if err != nil {
		return 0, 0, err
	}
	offset, err = intParam(args, "offset", 0, 0)
	if err != nil {
		return 0, 0, err
	}
	return limit, offset, nil
}

// invalidParamsResult renders a parameter validation error as a tool error result.
func invalidParamsResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(errors.FormatErrorForTool(err))
}

// runePrefixEnd returns the byte index just past the first n runes of s.
func runePrefixEnd(s string, n int) int {
	i := 0
	for n > 0 && i < len(s) {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		n--
	}
	return i
}

// runeSlice returns the runes of s in [start, end).
func runeSlice(s string, start, end int) string {
	from := runePrefixEnd(s, start)
	to := from + runePrefixEnd(s[from:], end-start)
	return s[from:to]
}

// truncateContent slices content to at most maxChars runes starting at the rune
// offset. It returns the slice and the pagination notice; the notice is empty when
// the whole content fits from offset 0, meaning the result is a single block.
func truncateContent(content string, offset, maxChars int) (page, notice string) {
	total := utf8.RuneCountInString(content)
	if offset == 0 && total <= maxChars {
		return content, ""
	}
	if offset >= total {
		return "", fmt.Sprintf("[end of content: %d characters]", total)
	}
	end := offset + maxChars
	if end >= total {
		return runeSlice(content, offset, total), fmt.Sprintf("[end of content: %d characters]", total)
	}
	return runeSlice(content, offset, end),
		fmt.Sprintf("[truncated: %d more characters; call again with offset=%d]", total-end, end)
}

// pagedResult builds a tool result from a content block and an optional
// pagination block. The pagination block is a separate text content so page
// content can never write into it.
func pagedResult(content, notice string) *mcp.CallToolResult {
	if notice == "" {
		return mcp.NewToolResultText(content)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(content),
			mcp.NewTextContent(notice),
		},
	}
}

// textPageResult slices text by offset and maxChars and builds the paged result.
func textPageResult(text string, offset, maxChars int) *mcp.CallToolResult {
	page, notice := truncateContent(text, offset, maxChars)
	return pagedResult(page, notice)
}

// listPageMeta is the pagination block of a list content tool.
type listPageMeta struct {
	Total    int  `json:"total"`
	Offset   int  `json:"offset"`
	Returned int  `json:"returned"`
	HasMore  bool `json:"hasMore"`
}

// paginateItems returns the page of items starting at offset, at most limit items,
// whose serialized JSON array fits in budget runes. items is in selection order:
// offset k skips the first k items. Trailing items that do not fit are dropped; a
// single item larger than the budget is returned alone with its string fields cut,
// so the next offset always advances. When reverse is true the page is emitted in
// reverse selection order (used by the log tools, which select newest first but
// return entries oldest first).
//
// The first content block is the bare JSON array; the second is listPageMeta.
func paginateItems[T any](items []T, offset, limit, budget int, reverse bool) (*mcp.CallToolResult, error) {
	total := len(items)
	start := min(offset, total)
	end := start + min(limit, total-start)

	chosen := make([][]byte, 0, end-start)
	size := 2 // brackets
	for i := start; i < end; i++ {
		raw, err := json.Marshal(items[i])
		if err != nil {
			return nil, err
		}
		itemSize := utf8.RuneCount(raw)
		if len(chosen) > 0 {
			itemSize++ // comma
		}
		if size+itemSize > budget {
			if len(chosen) == 0 {
				cut, err := cutItemToBudget(raw, budget-2)
				if err != nil {
					return nil, err
				}
				chosen = append(chosen, cut)
			}
			break
		}
		size += itemSize
		chosen = append(chosen, raw)
	}

	if reverse {
		for i, j := 0, len(chosen)-1; i < j; i, j = i+1, j-1 {
			chosen[i], chosen[j] = chosen[j], chosen[i]
		}
	}

	meta, err := json.Marshal(listPageMeta{
		Total:    total,
		Offset:   offset,
		Returned: len(chosen),
		HasMore:  start+len(chosen) < total,
	})
	if err != nil {
		return nil, err
	}

	array := "[" + string(bytes.Join(chosen, []byte(","))) + "]"
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(array),
			mcp.NewTextContent(string(meta)),
		},
	}, nil
}

// stringRef points at a string value inside a decoded JSON tree.
type stringRef struct {
	get func() string
	set func(string)
}

// collectStrings records a reference to every string value (not object key) in v.
func collectStrings(v any, refs *[]stringRef) {
	switch node := v.(type) {
	case map[string]any:
		for k, child := range node {
			if _, ok := child.(string); ok {
				key := k
				*refs = append(*refs, stringRef{
					get: func() string { return node[key].(string) },
					set: func(s string) { node[key] = s },
				})
				continue
			}
			collectStrings(child, refs)
		}
	case []any:
		for i, child := range node {
			if _, ok := child.(string); ok {
				idx := i
				*refs = append(*refs, stringRef{
					get: func() string { return node[idx].(string) },
					set: func(s string) { node[idx] = s },
				})
				continue
			}
			collectStrings(child, refs)
		}
	}
}

// cutItemToBudget shortens the string fields of one serialized item, longest
// first, until it fits in maxRunes. Each cut string ends with "…". If the item
// still does not fit once every string is empty, it is returned as small as it got.
func cutItemToBudget(raw []byte, maxRunes int) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}

	// Wrap the item so a bare string item is reachable through a reference too.
	holder := []any{tree}
	var refs []stringRef
	collectStrings(holder, &refs)

	for {
		out, err := json.Marshal(tree)
		if err != nil {
			return nil, err
		}
		size := utf8.RuneCount(out)
		if size <= maxRunes {
			return out, nil
		}

		var longest *stringRef
		longestLen := 0
		for i := range refs {
			if n := utf8.RuneCountInString(refs[i].get()); n > longestLen {
				longest, longestLen = &refs[i], n
			}
		}
		if longest == nil {
			return out, nil
		}

		keep := longestLen - (size - maxRunes) - 1 // leave room for the ellipsis
		if keep < 0 {
			longest.set("")
		} else {
			s := longest.get()
			longest.set(s[:runePrefixEnd(s, keep)] + "…")
		}
		tree = holder[0]
	}
}
