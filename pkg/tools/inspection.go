// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// GetConsoleLogsTool returns the get_console_logs MCP tool definition.
func GetConsoleLogsTool() mcp.Tool {
	return mcp.NewTool("get_console_logs",
		mcp.WithDescription("Retrieve browser console log messages from a session"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of entries to return (default: 50)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Number of newest matching entries to skip (default: 0). Entries are selected newest first and returned oldest first; the buffer is live, so offsets and total shift as new entries arrive. Continue with offset + returned from the second content block while hasMore is true; fewer than limit items may be returned when the response budget is reached"),
		),
		mcp.WithString("level",
			mcp.Enum("log", "info", "warn", "error", "debug"),
			mcp.Description("Filter by log level (returns all levels if not specified)"),
		),
	)
}

// GetConsoleLogsHandler returns the handler function for get_console_logs.
func GetConsoleLogsHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "get_console_logs")

		// Apply timeout with default category (log retrieval is a quick operation)
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutDefault, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "get_console_logs")
			return requiredParamResult("sessionId"), nil
		}

		limit, offset, err := listPageParams(args)
		if err != nil {
			return errorResult(err), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "get_console_logs", "sessionID", sessionID)
			return errorResult(errors.NewSessionNotFoundError(sessionID)), nil
		}

		// Check if ConsoleLogs buffer exists
		if sess.ConsoleLogs == nil {
			return internalErrorResult("Console log buffer not initialized for this session"), nil
		}

		// Extract and validate level filter parameter
		var level session.ConsoleLogLevel
		levelStr := req.GetString("level", "")
		if levelStr != "" {
			switch levelStr {
			case "log", "info", "warn", "error", "debug":
				level = session.ConsoleLogLevel(levelStr)
			default:
				return paramErrorResult("level", fmt.Sprintf("Invalid level: %s. Must be one of: log, info, warn, error, debug", levelStr)), nil
			}
		}

		// Retrieve all matching entries (chronological), then select newest first:
		// offset k skips the k newest entries. The page is returned oldest first.
		entries := sess.ConsoleLogs.Get(0, level)
		slices.Reverse(entries)

		result, err := paginateItems(entries, offset, limit, responseConfig.budget(), true)
		if err != nil {
			return internalErrorResult("Failed to serialize console logs: %v", err), nil
		}
		return result, nil
	}
}

// ScreenshotTool returns the screenshot MCP tool definition.
func ScreenshotTool() mcp.Tool {
	return mcp.NewTool("screenshot",
		mcp.WithDescription("Capture a PNG screenshot of the page or element"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithBoolean("fullPage",
			mcp.Description("Capture full scrollable page (default: false)"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector to capture specific element"),
		),
		mcp.WithObject("viewport",
			mcp.Description("Clip region to capture (x, y, width, height in pixels)"),
			mcp.Properties(map[string]any{
				"x": map[string]any{
					"type":        "number",
					"description": "X coordinate of clip region",
				},
				"y": map[string]any{
					"type":        "number",
					"description": "Y coordinate of clip region",
				},
				"width": map[string]any{
					"type":        "number",
					"description": "Width of clip region in pixels",
				},
				"height": map[string]any{
					"type":        "number",
					"description": "Height of clip region in pixels",
				},
			}),
		),
	)
}

// ScreenshotHandler returns the handler function for screenshot.
func ScreenshotHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "screenshot")

		// Apply timeout with default category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutDefault, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "screenshot")
			return requiredParamResult("sessionId"), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "screenshot", "sessionID", sessionID)
			return errorResult(errors.NewSessionNotFoundError(sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "screenshot", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		// Check for selector parameter - element screenshot
		selector := req.GetString("selector", "")
		if selector != "" {
			// Element-specific screenshot
			element, err := page.QuerySelector(selector)
			if err != nil {
				return errorResult(errors.WrapElementNotFoundError("", 0, err)), nil
			}
			if element == nil {
				return errorResult(errors.NewElementNotFoundError(selector, 0)), nil
			}

			screenshotBytes, err := element.Screenshot()
			if err != nil {
				return internalErrorResult("Failed to capture element screenshot: %v", err), nil
			}

			base64Data := base64.StdEncoding.EncodeToString(screenshotBytes)
			return mcp.NewToolResultImage("Screenshot of element "+selector, base64Data, "image/png"), nil
		}

		// Page screenshot
		opts := playwright.PageScreenshotOptions{}

		// Parse fullPage option
		if fullPageVal, ok := args["fullPage"]; ok && fullPageVal != nil {
			if fullPage, ok := fullPageVal.(bool); ok && fullPage {
				opts.FullPage = playwright.Bool(true)
			}
		}

		// Parse viewport/clip option
		if vpArg, ok := args["viewport"]; ok && vpArg != nil {
			if vpMap, ok := vpArg.(map[string]any); ok {
				clip := parseClipRegion(vpMap)
				if clip != nil {
					opts.Clip = clip
				}
			}
		}

		screenshotBytes, err := page.Screenshot(opts)
		if err != nil {
			slog.Error("failed to capture screenshot", "tool", "screenshot", "sessionID", sessionID, "error", err)
			return internalErrorResult("Failed to capture screenshot: %v", err), nil
		}

		slog.Debug("screenshot captured", "tool", "screenshot", "sessionID", sessionID, "bytes", len(screenshotBytes))
		base64Data := base64.StdEncoding.EncodeToString(screenshotBytes)
		return mcp.NewToolResultImage("Page screenshot", base64Data, "image/png"), nil
	}
}

// parseClipRegion extracts x, y, width, height from a viewport/clip parameter object.
// Returns nil if the clip region cannot be parsed or is invalid.
func parseClipRegion(vp map[string]any) *playwright.Rect {
	var x, y, width, height float64
	var hasX, hasY, hasWidth, hasHeight bool

	// Parse x
	switch v := vp["x"].(type) {
	case float64:
		x = v
		hasX = true
	case int:
		x = float64(v)
		hasX = true
	}

	// Parse y
	switch v := vp["y"].(type) {
	case float64:
		y = v
		hasY = true
	case int:
		y = float64(v)
		hasY = true
	}

	// Parse width
	switch v := vp["width"].(type) {
	case float64:
		width = v
		hasWidth = true
	case int:
		width = float64(v)
		hasWidth = true
	}

	// Parse height
	switch v := vp["height"].(type) {
	case float64:
		height = v
		hasHeight = true
	case int:
		height = float64(v)
		hasHeight = true
	}

	// Require all four parameters and positive dimensions
	if !hasX || !hasY || !hasWidth || !hasHeight {
		return nil
	}
	if width <= 0 || height <= 0 {
		return nil
	}

	return &playwright.Rect{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	}
}

// ExtractTextTool returns the extract_text MCP tool definition.
func ExtractTextTool() mcp.Tool {
	return mcp.NewTool("extract_text",
		mcp.WithDescription("Extract visible text from page as markdown"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector to extract from (defaults to body)"),
		),
		mcp.WithNumber("maxChars",
			mcp.Description("Maximum characters to return (default and upper bound: the server's response budget)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Character offset to start from, for reading the next slice of truncated output (default: 0)"),
		),
	)
}

// ExtractTextHandler returns the handler function for extract_text.
func ExtractTextHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "extract_text")

		// Apply timeout with element category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutElement, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "extract_text")
			return requiredParamResult("sessionId"), nil
		}

		maxChars, offset, err := textPageParams(args, responseConfig.budget())
		if err != nil {
			return errorResult(err), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "extract_text", "sessionID", sessionID)
			return errorResult(errors.NewSessionNotFoundError(sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "extract_text", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		// Get selector or default to body
		selector := req.GetString("selector", "body")

		// Execute JavaScript to extract visible text as markdown
		result, err := page.Evaluate(extractTextAsMarkdownJS, selector)
		if err != nil {
			return internalErrorResult("Failed to extract text: %v", err), nil
		}

		// Check if element was found (JavaScript returns null if selector doesn't match)
		if result == nil {
			return errorResult(errors.NewElementNotFoundError(selector, 0)), nil
		}

		text, ok := result.(string)
		if !ok {
			return internalErrorResult("Failed to extract text: unexpected result type"), nil
		}

		text = strings.TrimSpace(text)
		// An empty page reports itself only on the first call; at a non-zero offset
		// it falls through so the caller gets the usual end-of-content notice.
		if text == "" && offset == 0 {
			return mcp.NewToolResultText("No visible text found on the page."), nil
		}

		return textPageResult(text, offset, maxChars), nil
	}
}

// extractTextAsMarkdownJS is the JavaScript code that extracts visible text from the DOM
// and formats it as markdown. Shared by extract_text and navigate_and_extract_text.
//
// Relative link and image URLs are resolved against the page, so they come out
// absolute; absolute URLs are kept as written.
// Heading and table cell content goes through the inline renderer, so links inside
// them survive. Tables read table.rows, so a nested table renders once, flattened
// into its cell. Subtrees under aria-hidden="true" are treated as hidden.
//
// Form controls render inline as [<role> "<label>": <value>]; secret values are
// never emitted, only whether the field is set. A control is secret when it is a
// password field, its autocomplete names a credential or card field, or its text
// is masked with -webkit-text-security. Open shadow roots are walked in
// place of the host's light DOM, with each slot rendering its assigned nodes.
// Iframes render as an [iframe "<title or src>"] placeholder without their content.
const extractTextAsMarkdownJS = `(selector) => {
	const BT = String.fromCharCode(96);
	const SKIP = new Set(['script','style','noscript','svg','template','meta','link']);
	const TEXTBOX_TYPES = new Set(['text','email','search','tel','url']);
	const BUTTON_TYPES = new Set(['submit','button','reset','image']);
	const SECRET_AUTOCOMPLETE = new Set(['current-password','new-password','one-time-code','cc-number','cc-csc']);
	// Code blocks are wrapped in these sentinels so the final tidy pass can tell code
	// from prose without parsing the rendered markdown. Page content never carries
	// them: clean() strips them from every string taken from the page (text nodes,
	// code bodies, attribute values, control values and labels).
	const CODE_OPEN = String.fromCharCode(0);
	const CODE_CLOSE = String.fromCharCode(1);
	const SENTINELS = /[\u0000\u0001]/g;
	const clean = s => String(s).replace(SENTINELS, '');
	let cellDepth = 0;

	function isHidden(node) {
		if (node.getAttribute('aria-hidden') === 'true') return true;
		try {
			const style = window.getComputedStyle(node);
			if (style.display === 'none' || style.visibility === 'hidden') return true;
		} catch(e) {}
		return false;
	}

	// resolveURL returns an absolute URL for a link or image attribute. Absolute
	// values are kept as written; relative ones use the browser-resolved property.
	function resolveURL(raw, resolved) {
		if (!raw) return '';
		return clean(/^[a-z][a-z0-9+.-]*:/i.test(raw.trim()) ? raw.trim() : (resolved || ''));
	}

	// kids returns the nodes rendered in place of an element's children: an open
	// shadow root's children, a slot's assigned nodes (or its fallback content),
	// or the light-DOM children.
	function kids(node) {
		if (node.shadowRoot) return Array.from(node.shadowRoot.childNodes);
		if (node.tagName === 'SLOT') {
			const assigned = node.assignedNodes();
			if (assigned.length) return assigned;
		}
		return Array.from(node.childNodes);
	}

	// labelText returns an element's text without the text of controls or secret
	// content inside it. A value-bearing control or a secret element has no label
	// text, so aria-labelledby pointing at one cannot read its content. A button
	// keeps its text, which is visible content.
	function labelText(el) {
		if (/^(INPUT|SELECT|TEXTAREA)$/i.test(el.tagName) || isSecretContent(el)) return '';
		const clone = withoutSecrets(el);
		clone.querySelectorAll('input,select,textarea,button').forEach(c => c.remove());
		return clone.textContent.replace(/\s+/g, ' ').trim();
	}

	// controlLabel takes the label from the first of: aria-labelledby, aria-label,
	// intrinsic (a button's own text), label[for=id], a wrapping label,
	// placeholder, name.
	function controlLabel(node, intrinsic) {
		const root = node.getRootNode();
		const byId = id => (root.getElementById && root.getElementById(id)) || document.getElementById(id);
		const labelledBy = (node.getAttribute('aria-labelledby') || '').trim();
		if (labelledBy) {
			const t = labelledBy.split(/\s+/).map(byId).filter(Boolean).map(labelText).filter(Boolean).join(' ');
			if (t) return t;
		}
		const ariaLabel = (node.getAttribute('aria-label') || '').trim();
		if (ariaLabel) return ariaLabel;
		if (intrinsic) return intrinsic;
		if (node.id && root.querySelector) {
			const forLabel = root.querySelector('label[for="' + CSS.escape(node.id) + '"]');
			const t = forLabel ? labelText(forLabel) : '';
			if (t) return t;
		}
		const wrapping = node.closest('label');
		const wrapped = wrapping ? labelText(wrapping) : '';
		if (wrapped) return wrapped;
		const placeholder = (node.getAttribute('placeholder') || '').trim();
		if (placeholder) return placeholder;
		return (node.getAttribute('name') || '').trim();
	}

	function quote(s) {
		return '"' + clean(s).replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"';
	}

	function token(role, label, value) {
		const v = value === undefined ? '' : ': ' + clean(value).replace(/\s+/g, ' ').trim();
		return ' [' + role + ' ' + quote(label) + v + '] ';
	}

	function isMasked(node) {
		try {
			const style = window.getComputedStyle(node);
			const mask = style.webkitTextSecurity || style.getPropertyValue('-webkit-text-security');
			return !!mask && mask !== 'none';
		} catch(e) {
			return false;
		}
	}

	function isSecret(node, type) {
		if (type === 'password') return true;
		const ac = (node.getAttribute('autocomplete') || '').toLowerCase().split(/\s+/);
		if (ac.some(t => SECRET_AUTOCOMPLETE.has(t))) return true;
		return isMasked(node);
	}

	// isSecretContent reports whether an element's text is secret: a secret textarea,
	// or a masked contenteditable or an element inside one (both properties inherit).
	function isSecretContent(el) {
		if (/^textarea$/i.test(el.tagName)) return isSecret(el, 'textarea');
		return el.isContentEditable && isMasked(el);
	}

	// withoutSecrets returns a clone of node with its secret descendants removed.
	// They are found on the original tree, where computed style is available, and
	// the clone node at the same document-order index is dropped.
	function withoutSecrets(node) {
		const clone = node.cloneNode(true);
		if (!node.isContentEditable && !node.querySelector('textarea,[contenteditable]')) return clone;
		const copies = clone.querySelectorAll('*');
		node.querySelectorAll('*').forEach((d, i) => {
			if (isSecretContent(d)) copies[i].remove();
		});
		return clone;
	}

	// codeText returns the text of a code element without secret content, so a
	// secret textarea or masked contenteditable cannot leak through a surrounding
	// pre or code.
	function codeText(node) {
		return clean(withoutSecrets(node).textContent);
	}

	// secretToken renders a secret control with only whether it holds a value.
	function secretToken(node) {
		return token('password', controlLabel(node), node.value ? 'set' : 'empty');
	}

	function renderInput(node) {
		const type = (node.getAttribute('type') || 'text').toLowerCase();
		if (type === 'hidden') return '';
		if (type === 'checkbox' || type === 'radio') {
			return token(type, controlLabel(node), node.checked ? 'checked' : 'unchecked');
		}
		if (BUTTON_TYPES.has(type)) {
			let own = node.value;
			if (!own && type === 'submit') own = 'Submit';
			if (!own && type === 'reset') own = 'Reset';
			if (type === 'image') own = node.alt || own;
			return token('button', controlLabel(node, (own || '').trim()));
		}
		// Only text-entry controls can be secret: -webkit-text-security inherits, so a
		// checkbox or button inside a masked wrapper keeps its own token.
		if (isSecret(node, type)) return secretToken(node);
		const role = type === 'number' ? 'spinbutton' : 'textbox';
		return token(role, controlLabel(node), node.value);
	}

	function renderSelect(node) {
		const selected = Array.from(node.selectedOptions || []).map(o => (o.label || o.text || '').trim());
		return token('combobox', controlLabel(node), selected.join(', '));
	}

	// oneLine collapses rendered content onto a single line.
	function oneLine(s) {
		return s.replace(/\s*\n\s*/g, ' ').replace(/[ \t]+/g, ' ').trim();
	}

	function renderCell(cell) {
		cellDepth++;
		try {
			return oneLine(kids(cell).map(md).join('')).replace(/\|/g, '\\|');
		} finally {
			cellDepth--;
		}
	}

	function md(node) {
		if (node.nodeType === Node.TEXT_NODE) {
			return clean(node.textContent).replace(/[ \t]+/g, ' ');
		}
		if (node.nodeType !== Node.ELEMENT_NODE) return '';

		const tag = node.tagName.toLowerCase();
		if (SKIP.has(tag)) return '';
		if (isHidden(node)) return '';
		// A masked contenteditable is a custom secret field: report only whether it
		// holds text, without walking its children.
		if (node.isContentEditable && isMasked(node)) {
			return token('password', controlLabel(node), node.textContent.trim() ? 'set' : 'empty');
		}

		const inner = () => kids(node).map(md).join('');

		switch(tag) {
		case 'input': return renderInput(node);
		case 'textarea':
			return isSecret(node, 'textarea') ? secretToken(node) : token('textbox', controlLabel(node), node.value);
		case 'select': return renderSelect(node);
		case 'button': return token('button', controlLabel(node, node.textContent.replace(/\s+/g, ' ').trim()));
		case 'iframe': {
			const src = resolveURL(node.getAttribute('src'), node.src);
			return ' [iframe ' + quote((node.getAttribute('title') || '').trim() || src) + '] ';
		}
		case 'h1': case 'h2': case 'h3': case 'h4': case 'h5': case 'h6': {
			const t = oneLine(inner());
			return t ? '\n\n' + '#'.repeat(+tag[1]) + ' ' + t + '\n\n' : '';
		}
		case 'p': {
			const t = inner().trim();
			return t ? '\n\n' + t + '\n\n' : '';
		}
		case 'br': return '\n';
		case 'hr': return '\n\n---\n\n';
		case 'strong': case 'b': {
			const t = inner().trim();
			return t ? '**' + t + '**' : '';
		}
		case 'em': case 'i': {
			const t = inner().trim();
			return t ? '*' + t + '*' : '';
		}
		case 'a': {
			const t = inner().trim();
			const href = resolveURL(node.getAttribute('href'), node.href);
			if (!t) return '';
			if (!href || /^javascript:/i.test(href)) return t;
			return '[' + t + '](' + href + ')';
		}
		case 'img': {
			const alt = clean(node.alt || node.getAttribute('aria-label') || '');
			if (!alt) return '';
			const src = resolveURL(node.getAttribute('src'), node.currentSrc || node.src);
			return src ? '![' + alt + '](' + src + ')' : alt;
		}
		case 'code': {
			if (node.parentElement && node.parentElement.tagName.toLowerCase() === 'pre') {
				return codeText(node);
			}
			const t = codeText(node);
			return t ? BT + t + BT : '';
		}
		case 'pre': {
			const t = codeText(node);
			if (!t) return '';
			return '\n\n' + CODE_OPEN + BT.repeat(3) + '\n' + t + '\n' + BT.repeat(3) + CODE_CLOSE + '\n\n';
		}
		case 'blockquote': {
			const t = inner().trim();
			return t ? '\n\n' + t.split('\n').map(l => '> ' + l).join('\n') + '\n\n' : '';
		}
		case 'ul': case 'ol':
			return '\n\n' + inner() + '\n';
		case 'li': {
			const parent = node.parentElement;
			const ordered = parent && parent.tagName.toLowerCase() === 'ol';
			const t = inner().trim();
			if (!t) return '';
			let marker = '- ';
			if (ordered) {
				marker = (Array.from(parent.children).filter(c => c.tagName === 'LI').indexOf(node) + 1) + '. ';
			}
			// Indent continuation lines to the item's content column, so a code block
			// inside the item stays part of it. The tidy pass trims prose lines, so only
			// code keeps the indent.
			const pad = ' '.repeat(marker.length);
			return marker + t.split('\n').map((l, k) => k ? pad + l : l).join('\n') + '\n';
		}
		case 'table': {
			const rows = Array.from(node.rows).filter(r => !isHidden(r));
			if (!rows.length) return '';
			const rendered = rows.map(r => Array.from(r.cells).filter(c => !isHidden(c)).map(renderCell));
			if (cellDepth > 0) {
				// A table nested in a cell flattens into that cell's text.
				return ' ' + rendered.map(cells => cells.filter(Boolean).join(' ')).filter(Boolean).join(' ') + ' ';
			}
			let out = '\n\n';
			rendered.forEach((cells, i) => {
				out += '| ' + cells.join(' | ') + ' |\n';
				if (i === 0) out += '| ' + cells.map(() => '---').join(' | ') + ' |\n';
			});
			return out + '\n';
		}
		case 'thead': case 'tbody': case 'tfoot': case 'tr': case 'th': case 'td':
			return '';
		default: {
			const content = inner();
			try {
				const display = window.getComputedStyle(node).display;
				if (display === 'block' || display === 'flex' || display === 'grid') {
					return '\n' + content + '\n';
				}
			} catch(e) {}
			return content;
		}
		}
	}

	const el = document.querySelector(selector);
	if (!el) return null;

	// Tidy whitespace outside code blocks: collapse runs of spaces and trim each
	// line, so whitespace-only lines collapse with the surrounding blank lines. Code
	// blocks are the sentinel-delimited segments and keep their text as written. A
	// prose line that continues into or out of a code block is not trimmed on that
	// side. The line just before a code block can only hold a wrapper prefix (a list
	// marker, a list item's continuation indent, "> ", or a mix of them), because pre
	// emits a blank line before its sentinel, so that prefix is kept as is. Runs of
	// blank quote lines collapse like runs of blank lines.
	const parts = md(el).split(/(\u0000[^\u0000\u0001]*\u0001)/);
	const text = parts.map((part, i) => {
		if (i % 2 === 1) return part.slice(1, -1);
		const lines = part.split('\n');
		const last = lines.length - 1;
		const beforeCode = i < parts.length - 1;
		const tidied = lines.map((line, j) => {
			if (j === last && beforeCode) {
				const indent = line.match(/^(?:[ \t]*>[ \t]?)*[ \t]*/)[0];
				return indent + line.slice(indent.length).replace(/[ \t]+/g, ' ');
			}
			line = line.replace(/[ \t]+/g, ' ');
			if (j > 0 || i === 0) line = line.trimStart();
			if (j < last || i === parts.length - 1) line = line.trimEnd();
			return line;
		}).join('\n');
		return tidied.replace(/\n{3,}/g, '\n\n').replace(/^(>(?: >)*)(?:\n\1)+$/gm, '$1').replace(SENTINELS, '');
	}).join('').trim();
	return text;
}`

// GetHTMLTool returns the get_html MCP tool definition.
func GetHTMLTool() mcp.Tool {
	return mcp.NewTool("get_html",
		mcp.WithDescription("Get HTML source of page or element"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector (defaults to entire page)"),
		),
		mcp.WithBoolean("outer",
			mcp.Description("Return outerHTML instead of innerHTML (default: false)"),
		),
		mcp.WithNumber("maxChars",
			mcp.Description("Maximum characters to return (default and upper bound: the server's response budget)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Character offset to start from, for reading the next slice of truncated output (default: 0)"),
		),
	)
}

// GetHTMLHandler returns the handler function for get_html.
func GetHTMLHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "get_html")

		// Apply timeout with element category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutElement, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "get_html")
			return requiredParamResult("sessionId"), nil
		}

		maxChars, offset, err := textPageParams(args, responseConfig.budget())
		if err != nil {
			return errorResult(err), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "get_html", "sessionID", sessionID)
			return errorResult(errors.NewSessionNotFoundError(sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "get_html", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		selector := req.GetString("selector", "")

		// Check outer parameter
		outer := false
		if outerVal, ok := args["outer"]; ok && outerVal != nil {
			if outerBool, ok := outerVal.(bool); ok {
				outer = outerBool
			}
		}

		var html string

		if selector == "" {
			// Get entire page content
			html, err = page.Content()
			if err != nil {
				return internalErrorResult("Failed to get page content: %v", err), nil
			}
		} else {
			// Get element HTML
			element, err := page.QuerySelector(selector)
			if err != nil {
				return errorResult(errors.WrapElementNotFoundError("", 0, err)), nil
			}
			if element == nil {
				return errorResult(errors.NewElementNotFoundError(selector, 0)), nil
			}

			if outer {
				// Get outerHTML
				result, err := element.Evaluate("el => el.outerHTML", nil)
				if err != nil {
					return internalErrorResult("Failed to get outerHTML: %v", err), nil
				}
				html, _ = result.(string)
			} else {
				// Get innerHTML
				html, err = element.InnerHTML()
				if err != nil {
					return internalErrorResult("Failed to get innerHTML: %v", err), nil
				}
			}
		}

		return textPageResult(html, offset, maxChars), nil
	}
}

// EvaluateTool returns the evaluate MCP tool definition.
func EvaluateTool() mcp.Tool {
	return mcp.NewTool("evaluate",
		mcp.WithDescription("Execute JavaScript expression in page context"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("expression",
			mcp.Required(),
			mcp.Description("JavaScript expression to evaluate"),
		),
		mcp.WithNumber("maxChars",
			mcp.Description("Maximum characters of result JSON to return (default and upper bound: the server's response budget). Output over the limit is cut, not paged"),
		),
	)
}

// EvaluateHandler returns the handler function for evaluate.
func EvaluateHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "evaluate")

		// Apply timeout with script category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutScript, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "evaluate")
			return requiredParamResult("sessionId"), nil
		}

		// Parse and validate expression argument
		expression := req.GetString("expression", "")
		if expression == "" {
			slog.Error("missing required expression", "tool", "evaluate", "sessionID", sessionID)
			return requiredParamResult("expression"), nil
		}

		maxChars, err := maxCharsParam(args, responseConfig.budget())
		if err != nil {
			return errorResult(err), nil
		}

		slog.Debug("evaluating expression", "tool", "evaluate", "sessionID", sessionID)

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "evaluate", "sessionID", sessionID)
			return errorResult(errors.NewSessionNotFoundError(sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "evaluate", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		// Execute JavaScript expression with context deadline enforcement.
		// page.Evaluate() doesn't accept a timeout option in playwright-go, so
		// we run it in a goroutine and select on context cancellation.
		//
		// Known limitation: when context is cancelled, the goroutine running
		// page.Evaluate() continues until the Playwright operation completes or
		// the browser context is closed. Under normal conditions Playwright's
		// own default timeout (30s) bounds the goroutine lifetime.
		type evalResult struct {
			value interface{}
			err   error
		}
		ch := make(chan evalResult, 1)
		go func() {
			v, evalErr := page.Evaluate(expression)
			ch <- evalResult{v, evalErr}
		}()

		select {
		case r := <-ch:
			if r.err != nil {
				errStr := r.err.Error()
				if strings.Contains(errStr, "SyntaxError") || strings.Contains(errStr, "ReferenceError") {
					slog.Error("JavaScript error", "tool", "evaluate", "sessionID", sessionID, "error", r.err)
					return paramErrorResult("expression", fmt.Sprintf("JavaScript error: %v", r.err)), nil
				}
				slog.Error("failed to evaluate expression", "tool", "evaluate", "sessionID", sessionID, "error", r.err)
				return internalErrorResult("Failed to evaluate expression: %v", r.err), nil
			}

			// Serialize result to JSON
			jsonBytes, err := json.Marshal(r.value)
			if err != nil {
				return internalErrorResult("Failed to serialize result: %v", err), nil
			}

			resultJSON := string(jsonBytes)
			if utf8.RuneCountInString(resultJSON) > maxChars {
				return pagedResult(runeSlice(resultJSON, 0, maxChars), evaluateTruncatedNotice), nil
			}
			return mcp.NewToolResultText(resultJSON), nil
		case <-ctx.Done():
			slog.Warn("evaluate returning due to context cancellation; Playwright goroutine still running",
				"tool", "evaluate", "sessionID", sessionID, "error", ctx.Err())
			return errorResult(errors.WrapTimeoutError("evaluate", 0, ctx.Err())), nil
		}
	}
}

// ElementInfo represents information about a DOM element.
type ElementInfo struct {
	Tag         string            `json:"tag"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Text        string            `json:"text,omitempty"`
	BoundingBox *BoundingBox      `json:"boundingBox,omitempty"`
}

// BoundingBox represents the bounding box of an element.
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// QuerySelectorTool returns the query_selector MCP tool definition.
func QuerySelectorTool() mcp.Tool {
	return mcp.NewTool("query_selector",
		mcp.WithDescription("Query element info by CSS selector"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Required(),
			mcp.Description("CSS selector to query"),
		),
		mcp.WithBoolean("all",
			mcp.Description("Return all matching elements (default: false, first match only)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("With all: maximum number of elements to return (default: 50)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("With all: number of matching elements to skip (default: 0). Continue with offset + returned from the second content block while hasMore is true; fewer than limit items may be returned when the response budget is reached"),
		),
	)
}

// QuerySelectorHandler returns the handler function for query_selector.
func QuerySelectorHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "query_selector")

		// Apply timeout with element category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutElement, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "query_selector")
			return requiredParamResult("sessionId"), nil
		}

		// Parse and validate selector argument
		selector := req.GetString("selector", "")
		if selector == "" {
			slog.Error("missing required selector", "tool", "query_selector", "sessionID", sessionID)
			return requiredParamResult("selector"), nil
		}

		limit, offset, err := listPageParams(args)
		if err != nil {
			return errorResult(err), nil
		}

		slog.Debug("querying selector", "tool", "query_selector", "sessionID", sessionID, "selector", selector)

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "query_selector", "sessionID", sessionID)
			return errorResult(errors.NewSessionNotFoundError(sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "query_selector", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		// Check if all parameter is set
		all := false
		if allVal, ok := args["all"]; ok && allVal != nil {
			if allBool, ok := allVal.(bool); ok {
				all = allBool
			}
		}

		if all {
			// Query all matching elements
			elements, err := page.QuerySelectorAll(selector)
			if err != nil {
				return internalErrorResult("Failed to query elements: %v", err), nil
			}

			infos := make([]ElementInfo, 0, len(elements))
			for i, elem := range elements {
				info, err := extractElementInfo(elem)
				if err != nil {
					slog.Warn("Skipping element in QuerySelectorAll due to extraction error",
						"selector", selector,
						"elementIndex", i,
						"error", err)
					continue
				}
				infos = append(infos, *info)
			}

			result, err := paginateItems(infos, offset, limit, responseConfig.budget(), false)
			if err != nil {
				return internalErrorResult("Failed to serialize element info: %v", err), nil
			}
			return result, nil
		}

		// Query single element
		element, err := page.QuerySelector(selector)
		if err != nil {
			return errorResult(errors.WrapElementNotFoundError("", 0, err)), nil
		}
		if element == nil {
			return errorResult(errors.NewElementNotFoundError(selector, 0)), nil
		}

		info, err := extractElementInfo(element)
		if err != nil {
			return internalErrorResult("Failed to extract element info: %v", err), nil
		}

		jsonBytes, err := json.Marshal(info)
		if err != nil {
			return internalErrorResult("Failed to serialize element info: %v", err), nil
		}
		if budget := responseConfig.budget(); utf8.RuneCount(jsonBytes) > budget {
			// Cut long attribute values and text so the single object fits the budget.
			if jsonBytes, err = cutItemToBudget(jsonBytes, budget); err != nil {
				return internalErrorResult("Failed to serialize element info: %v", err), nil
			}
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// extractElementInfo extracts information from a DOM element.
func extractElementInfo(element playwright.ElementHandle) (*ElementInfo, error) {
	// Get element tag name and attributes via JavaScript
	result, err := element.Evaluate(`el => ({
		tag: el.tagName.toLowerCase(),
		attributes: Object.fromEntries([...el.attributes].map(a => [a.name, a.value])),
		text: el.textContent.trim().substring(0, 500)
	})`, nil)
	if err != nil {
		return nil, err
	}

	data, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	info := &ElementInfo{
		Tag: data["tag"].(string),
	}

	if attrs, ok := data["attributes"].(map[string]interface{}); ok && len(attrs) > 0 {
		info.Attributes = make(map[string]string)
		for k, v := range attrs {
			if s, ok := v.(string); ok {
				info.Attributes[k] = s
			}
		}
	}

	if text, ok := data["text"].(string); ok && text != "" {
		info.Text = text
	}

	// Get bounding box
	box, err := element.BoundingBox()
	if err == nil && box != nil {
		info.BoundingBox = &BoundingBox{
			X:      box.X,
			Y:      box.Y,
			Width:  box.Width,
			Height: box.Height,
		}
	}

	return info, nil
}

// snapshotRefsDescription explains element refs; tool descriptions that return or
// accept refs share it.
const snapshotRefsDescription = "Each element carries a ref like [ref=e3], or [ref=f1e2] inside an iframe or a document loaded after the first. " +
	"Pass a ref as the ref argument of click, type, fill, select_option, hover, or press_key to act on exactly that element. " +
	"Refs stay the same while the document is unchanged and go stale when the page navigates; a stale ref fails at once with 'Stale element ref', so take a new snapshot."

// GetAccessibilityTreeTool returns the get_accessibility_tree MCP tool definition.
func GetAccessibilityTreeTool() mcp.Tool {
	return mcp.NewTool("get_accessibility_tree",
		mcp.WithDescription("Get an ARIA snapshot of the page: a compact YAML tree of roles, accessible names, states such as [checked], and form values (password values show as •••). "+
			snapshotRefsDescription+
			" Content hidden with display:none or visibility:hidden is left out; content that is on screen but aria-hidden is kept."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("Playwright selector (CSS, XPath, or text) matching exactly one element to snapshot instead of the whole page"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Maximum depth of the tree (default: unlimited)"),
		),
		mcp.WithNumber("maxChars",
			mcp.Description("Maximum characters to return (default and upper bound: the server's response budget)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Character offset to start from, for reading the next slice of truncated output (default: 0)"),
		),
	)
}

// GetAccessibilityTreeHandler returns the handler function for get_accessibility_tree.
func GetAccessibilityTreeHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "get_accessibility_tree")

		// Apply timeout with default category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutDefault, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "get_accessibility_tree")
			return requiredParamResult("sessionId"), nil
		}

		maxChars, offset, err := textPageParams(args, responseConfig.budget())
		if err != nil {
			return errorResult(err), nil
		}
		depth, err := intParam(args, "depth", 0, 1)
		if err != nil {
			return errorResult(err), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "get_accessibility_tree", "sessionID", sessionID)
			return errorResult(errors.NewSessionNotFoundError(sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "get_accessibility_tree", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		selector := req.GetString("selector", "")
		snapshot, err := takeSnapshot(page, selector, depth, PlaywrightTimeoutFromContext(ctx))
		if err != nil {
			if ctxErr := HandleContextError(ctx, "get_accessibility_tree"); ctxErr != nil {
				return errorResult(ctxErr), nil
			}
			slog.Error("failed to get accessibility snapshot", "tool", "get_accessibility_tree", "sessionID", sessionID, "error", err)
			return errorResult(err), nil
		}

		if strings.TrimSpace(snapshot) == "" && offset == 0 {
			return mcp.NewToolResultText("No accessible elements found."), nil
		}
		return textPageResult(snapshot, offset, maxChars), nil
	}
}

// LinkInfo represents information about a hyperlink on the page.
type LinkInfo struct {
	ID    string `json:"id,omitempty"`
	Href  string `json:"href"`
	Label string `json:"label"`
}

// GetLinksTool returns the get_links MCP tool definition.
func GetLinksTool() mcp.Tool {
	return mcp.NewTool("get_links",
		mcp.WithDescription("Get all hyperlinks from the page with their IDs and labels"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("url",
			mcp.Description("Optional URL to navigate to before extracting links"),
		),
		mcp.WithString("waitUntil",
			mcp.Enum("load", "domcontentloaded", "networkidle"),
			mcp.Description("Wait condition for navigation (default: load)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Navigation timeout in milliseconds"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of links to return (default: 50)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Number of links to skip (default: 0). Continue with offset + returned from the second content block while hasMore is true; fewer than limit items may be returned when the response budget is reached"),
		),
	)
}

// GetLinksHandler returns the handler function for get_links.
func GetLinksHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "get_links")

		// Apply timeout with navigation category (may include navigation)
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutNavigation, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "get_links")
			return requiredParamResult("sessionId"), nil
		}

		limit, offset, err := listPageParams(args)
		if err != nil {
			return errorResult(err), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "get_links", "sessionID", sessionID)
			return errorResult(errors.NewSessionNotFoundError(sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "get_links", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		// If url param provided, navigate first
		url := req.GetString("url", "")
		if url != "" {
			opts := buildGotoOptions(args)
			if opts.Timeout == nil {
				opts.Timeout = PlaywrightTimeoutFromContext(ctx)
			}
			if _, err := page.Goto(url, opts); err != nil {
				if ctxErr := HandleContextError(ctx, fmt.Sprintf("navigation to %s", url)); ctxErr != nil {
					slog.Error("get_links navigation context error", "tool", "get_links", "sessionID", sessionID, "url", url, "error", ctxErr)
					return errorResult(ctxErr), nil
				}
				return handleNavigationError(err, url), nil
			}
		}

		// Extract links using JavaScript
		result, err := page.Evaluate(getLinksJS)
		if err != nil {
			slog.Error("failed to extract links", "tool", "get_links", "sessionID", sessionID, "error", err)
			return internalErrorResult("Failed to extract links: %v", err), nil
		}

		links, ok := result.([]any)
		if !ok && result != nil {
			return internalErrorResult("Failed to extract links: unexpected result type"), nil
		}

		linksPage, err := paginateItems(links, offset, limit, responseConfig.budget(), false)
		if err != nil {
			return internalErrorResult("Failed to serialize links: %v", err), nil
		}
		return linksPage, nil
	}
}

// getLinksJS is the JavaScript code that extracts hyperlink information from the DOM.
const getLinksJS = `() => {
	return Array.from(document.querySelectorAll('a[href]')).map(a => ({
		id: a.id || undefined,
		href: a.href,
		label: (a.textContent || a.getAttribute('aria-label') || '').trim().substring(0, 200)
	})).filter(link => link.href);
}`

// NavigateAndExtractTextTool returns the navigate_and_extract_text MCP tool definition.
func NavigateAndExtractTextTool() mcp.Tool {
	return mcp.NewTool("navigate_and_extract_text",
		mcp.WithDescription("Navigate to URL and extract visible text as markdown (creates temporary session)"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("URL to navigate to"),
		),
		mcp.WithString("waitUntil",
			mcp.Description("Wait condition: load (default), domcontentloaded, networkidle"),
			mcp.Enum("load", "domcontentloaded", "networkidle"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector to extract from (defaults to body)"),
		),
		mcp.WithString("browserType",
			mcp.Description("Browser to use: chromium, firefox, webkit"),
			mcp.Enum("chromium", "firefox", "webkit"),
		),
		mcp.WithNumber("maxChars",
			mcp.Description("Maximum characters to return (default and upper bound: the server's response budget)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Character offset to start from, for reading the next slice of truncated output (default: 0)"),
		),
	)
}

// NavigateAndExtractTextHandler returns the handler function for navigate_and_extract_text.
func NavigateAndExtractTextHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig, responseConfig *ResponseConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "navigate_and_extract_text")

		// Apply timeout with navigation category (includes both navigation and extraction)
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutNavigation, timeoutConfig)
		defer cancel()

		// Extract MCP session ID from context for ownership tracking
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate url argument
		url := req.GetString("url", "")
		if url == "" {
			slog.Error("missing required url", "tool", "navigate_and_extract_text")
			return requiredParamResult("url"), nil
		}

		maxChars, offset, err := textPageParams(args, responseConfig.budget())
		if err != nil {
			return errorResult(err), nil
		}

		// Parse browserType with default
		browserType := browser.BrowserChromium
		if btStr := req.GetString("browserType", ""); btStr != "" {
			switch btStr {
			case "chromium":
				browserType = browser.BrowserChromium
			case "firefox":
				browserType = browser.BrowserFirefox
			case "webkit":
				browserType = browser.BrowserWebKit
			default:
				return paramErrorResult("browserType", fmt.Sprintf("Invalid browserType: %s. Must be chromium, firefox, or webkit", btStr)), nil
			}
		}

		// Create temporary session
		opts := session.SessionOptions{
			BrowserType: browserType,
			Headless:    true,
		}

		slog.Debug("creating temporary session", "tool", "navigate_and_extract_text", "url", url, "browserType", browserType)

		sess, err := mgr.CreateSession(ctx, mcpSessionID, opts)
		if err != nil {
			slog.Error("failed to create temporary session", "tool", "navigate_and_extract_text", "error", err)
			return internalErrorResult("Failed to create temporary session: %v", err), nil
		}

		// Ensure session cleanup on exit
		defer func() {
			_ = mgr.CloseSession(ctx, mcpSessionID, sess.ID)
		}()

		// Get the page from the session
		page := sess.ActivePage()
		if page == nil {
			return internalErrorResult("no active page in temporary session"), nil
		}

		// Build navigation options
		gotoOpts := playwright.PageGotoOptions{}

		if waitUntil, ok := args["waitUntil"].(string); ok && waitUntil != "" {
			gotoOpts.WaitUntil = mapWaitUntil(waitUntil)
		}

		// Navigate to URL
		if _, err := page.Goto(url, gotoOpts); err != nil {
			if isTimeoutError(err) {
				slog.Error("navigation timeout", "tool", "navigate_and_extract_text", "url", url, "error", err)
				return mcp.NewToolResultError(errors.FormatErrorForTool(
					errors.WrapTimeoutError(fmt.Sprintf("navigation to %s", url), 0, err))), nil
			}
			slog.Error("navigation failed", "tool", "navigate_and_extract_text", "url", url, "error", err)
			return mcp.NewToolResultError(errors.FormatErrorForTool(
				errors.WrapNavigationError(url, "", err))), nil
		}

		// Get selector or default to body
		selector := req.GetString("selector", "body")

		// Execute JavaScript to extract visible text as markdown
		result, err := page.Evaluate(extractTextAsMarkdownJS, selector)
		if err != nil {
			return internalErrorResult("Failed to extract text: %v", err), nil
		}

		// Check if element was found (JavaScript returns null if selector doesn't match)
		if result == nil {
			return errorResult(errors.NewElementNotFoundError(selector, 0)), nil
		}

		text, ok := result.(string)
		if !ok {
			return internalErrorResult("Failed to extract text: unexpected result type"), nil
		}

		text = strings.TrimSpace(text)
		// An empty page reports itself only on the first call; at a non-zero offset
		// it falls through so the caller gets the usual end-of-content notice.
		if text == "" && offset == 0 {
			return mcp.NewToolResultText("No visible text found on the page."), nil
		}

		return textPageResult(text, offset, maxChars), nil
	}
}
