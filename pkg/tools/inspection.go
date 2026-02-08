// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/playwright-community/playwright-go"
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
		mcp.WithString("level",
			mcp.Enum("log", "info", "warn", "error", "debug"),
			mcp.Description("Filter by log level (returns all levels if not specified)"),
		),
	)
}

// GetConsoleLogsHandler returns the handler function for get_console_logs.
func GetConsoleLogsHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Session not found: %s", errors.CodeSessionNotFound, sessionID)), nil
		}

		// Check if ConsoleLogs buffer exists
		if sess.ConsoleLogs == nil {
			return mcp.NewToolResultError("Console log buffer not initialized for this session"), nil
		}

		// Extract limit parameter with default
		limit := 50
		args := req.GetArguments()
		if limitVal, ok := args["limit"]; ok && limitVal != nil {
			switch v := limitVal.(type) {
			case float64:
				limit = int(v)
			case int:
				limit = v
			}
		}

		// Extract and validate level filter parameter
		var level session.ConsoleLogLevel
		levelStr := req.GetString("level", "")
		if levelStr != "" {
			switch levelStr {
			case "log", "info", "warn", "error", "debug":
				level = session.ConsoleLogLevel(levelStr)
			default:
				return mcp.NewToolResultError(fmt.Sprintf("Invalid level: %s. Must be one of: log, info, warn, error, debug", levelStr)), nil
			}
		}

		// Retrieve console logs from session buffer
		entries := sess.ConsoleLogs.Get(limit, level)

		// Serialize entries to JSON
		jsonBytes, err := json.Marshal(entries)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize console logs: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
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
func ScreenshotHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Session not found: %s", errors.CodeSessionNotFound, sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			return mcp.NewToolResultError("no active page in session"), nil
		}

		args := req.GetArguments()

		// Check for selector parameter - element screenshot
		selector := req.GetString("selector", "")
		if selector != "" {
			// Element-specific screenshot
			element, err := page.QuerySelector(selector)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("[%d] Failed to find element: %v", errors.CodeElementNotFound, err)), nil
			}
			if element == nil {
				return mcp.NewToolResultError(fmt.Sprintf("[%d] Element not found: %s", errors.CodeElementNotFound, selector)), nil
			}

			screenshotBytes, err := element.Screenshot()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to capture element screenshot: %v", err)), nil
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
			return mcp.NewToolResultError(fmt.Sprintf("Failed to capture screenshot: %v", err)), nil
		}

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

// TextNode represents a node in the hierarchical text structure.
type TextNode struct {
	Tag      string     `json:"tag"`
	Text     string     `json:"text,omitempty"`
	Children []TextNode `json:"children,omitempty"`
}

// ExtractTextTool returns the extract_text MCP tool definition.
func ExtractTextTool() mcp.Tool {
	return mcp.NewTool("extract_text",
		mcp.WithDescription("Extract visible text from page in hierarchical structure"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector to extract from (defaults to body)"),
		),
	)
}

// ExtractTextHandler returns the handler function for extract_text.
func ExtractTextHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Session not found: %s", errors.CodeSessionNotFound, sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			return mcp.NewToolResultError("no active page in session"), nil
		}

		// Get selector or default to body
		selector := req.GetString("selector", "body")

		// Execute JavaScript to extract hierarchical text structure
		result, err := page.Evaluate(extractTextJS, selector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to extract text: %v", err)), nil
		}

		// Check if element was found (JavaScript returns null if selector doesn't match)
		if result == nil {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Element not found: %s", errors.CodeElementNotFound, selector)), nil
		}

		// Serialize result to JSON
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize text structure: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// extractTextJS is the JavaScript code that extracts hierarchical text from the DOM.
const extractTextJS = `(selector) => {
	function extractNode(element) {
		const node = { tag: element.tagName.toLowerCase() };
		const children = [];
		let directText = '';

		for (const child of element.childNodes) {
			if (child.nodeType === Node.TEXT_NODE) {
				const text = child.textContent.trim();
				if (text) directText += (directText ? ' ' : '') + text;
			} else if (child.nodeType === Node.ELEMENT_NODE) {
				const style = window.getComputedStyle(child);
				if (style.display !== 'none' && style.visibility !== 'hidden') {
					const childNode = extractNode(child);
					if (childNode.text || (childNode.children && childNode.children.length > 0)) {
						children.push(childNode);
					}
				}
			}
		}

		if (directText) node.text = directText;
		if (children.length > 0) node.children = children;

		return node;
	}

	const element = document.querySelector(selector);
	if (!element) return null;
	return extractNode(element);
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
	)
}

// GetHTMLHandler returns the handler function for get_html.
func GetHTMLHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Session not found: %s", errors.CodeSessionNotFound, sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			return mcp.NewToolResultError("no active page in session"), nil
		}

		args := req.GetArguments()
		selector := req.GetString("selector", "")

		// Check outer parameter
		outer := false
		if outerVal, ok := args["outer"]; ok && outerVal != nil {
			if outerBool, ok := outerVal.(bool); ok {
				outer = outerBool
			}
		}

		var html string
		var err error

		if selector == "" {
			// Get entire page content
			html, err = page.Content()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get page content: %v", err)), nil
			}
		} else {
			// Get element HTML
			element, err := page.QuerySelector(selector)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("[%d] Failed to find element: %v", errors.CodeElementNotFound, err)), nil
			}
			if element == nil {
				return mcp.NewToolResultError(fmt.Sprintf("[%d] Element not found: %s", errors.CodeElementNotFound, selector)), nil
			}

			if outer {
				// Get outerHTML
				result, err := element.Evaluate("el => el.outerHTML", nil)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to get outerHTML: %v", err)), nil
				}
				html, _ = result.(string)
			} else {
				// Get innerHTML
				html, err = element.InnerHTML()
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to get innerHTML: %v", err)), nil
				}
			}
		}

		return mcp.NewToolResultText(html), nil
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
	)
}

// EvaluateHandler returns the handler function for evaluate.
func EvaluateHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Parse and validate expression argument
		expression := req.GetString("expression", "")
		if expression == "" {
			return mcp.NewToolResultError("expression is required"), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Session not found: %s", errors.CodeSessionNotFound, sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			return mcp.NewToolResultError("no active page in session"), nil
		}

		// Execute JavaScript expression
		result, err := page.Evaluate(expression)
		if err != nil {
			// Check for syntax errors or evaluation errors
			errStr := err.Error()
			if strings.Contains(errStr, "SyntaxError") || strings.Contains(errStr, "ReferenceError") {
				return mcp.NewToolResultError(fmt.Sprintf("JavaScript error: %v", err)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to evaluate expression: %v", err)), nil
		}

		// Serialize result to JSON
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
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
	)
}

// QuerySelectorHandler returns the handler function for query_selector.
func QuerySelectorHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Parse and validate selector argument
		selector := req.GetString("selector", "")
		if selector == "" {
			return mcp.NewToolResultError("selector is required"), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Session not found: %s", errors.CodeSessionNotFound, sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			return mcp.NewToolResultError("no active page in session"), nil
		}

		args := req.GetArguments()

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
				return mcp.NewToolResultError(fmt.Sprintf("Failed to query elements: %v", err)), nil
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

			jsonBytes, err := json.Marshal(infos)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize element info: %v", err)), nil
			}

			return mcp.NewToolResultText(string(jsonBytes)), nil
		}

		// Query single element
		element, err := page.QuerySelector(selector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Failed to find element: %v", errors.CodeElementNotFound, err)), nil
		}
		if element == nil {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Element not found: %s", errors.CodeElementNotFound, selector)), nil
		}

		info, err := extractElementInfo(element)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to extract element info: %v", err)), nil
		}

		jsonBytes, err := json.Marshal(info)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize element info: %v", err)), nil
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

// GetAccessibilityTreeTool returns the get_accessibility_tree MCP tool definition.
// NOTE: This tool uses JavaScript-based accessibility extraction because playwright-go
// does not expose the native Accessibility().Snapshot() API. The JavaScript implementation
// provides a simplified accessibility tree based on ARIA attributes and semantic HTML roles,
// which may differ from the browser's native accessibility tree in some edge cases.
func GetAccessibilityTreeTool() mcp.Tool {
	return mcp.NewTool("get_accessibility_tree",
		mcp.WithDescription("Get accessibility snapshot of the page (JavaScript-based extraction using ARIA attributes and semantic roles)"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
	)
}

// GetAccessibilityTreeHandler returns the handler function for get_accessibility_tree.
func GetAccessibilityTreeHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership validation
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate sessionId argument
		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			return mcp.NewToolResultError("sessionId is required"), nil
		}

		// Get session with ownership validation
		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Session not found: %s", errors.CodeSessionNotFound, sessionID)), nil
		}

		// Get the active page from the session
		page := sess.ActivePage()
		if page == nil {
			return mcp.NewToolResultError("no active page in session"), nil
		}

		// Get accessibility tree using JavaScript
		// Note: playwright-go doesn't expose Accessibility().Snapshot() directly,
		// so we use JavaScript to extract accessibility information
		result, err := page.Evaluate(accessibilityTreeJS)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get accessibility tree: %v", err)), nil
		}

		// Serialize result to JSON
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize accessibility tree: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// accessibilityTreeJS extracts accessibility information from the DOM using JavaScript.
//
// LIMITATION: playwright-go v0.5200.1 does not expose the native Accessibility().Snapshot() API.
// This JavaScript-based implementation provides a simplified accessibility tree by:
// - Extracting explicit ARIA attributes (aria-label, aria-labelledby, aria-expanded, etc.)
// - Mapping semantic HTML elements to their implicit ARIA roles (button, link, heading, etc.)
// - Traversing visible elements and building a hierarchical tree structure
//
// Differences from native accessibility API:
// - May not capture all computed accessible names (e.g., from complex label algorithms)
// - Does not include accessibility properties from browser internals
// - Simplified role mapping compared to full ARIA specification
//
// This is suitable for most inspection and testing use cases but may not match
// screen reader behavior exactly.
const accessibilityTreeJS = `() => {
	function getAccessibleName(element) {
		// Try aria-label first
		if (element.getAttribute('aria-label')) {
			return element.getAttribute('aria-label');
		}
		// Try aria-labelledby
		const labelledBy = element.getAttribute('aria-labelledby');
		if (labelledBy) {
			const labels = labelledBy.split(' ').map(id => {
				const el = document.getElementById(id);
				return el ? el.textContent.trim() : '';
			}).filter(Boolean);
			if (labels.length) return labels.join(' ');
		}
		// Try label element for form controls
		if (element.id) {
			const label = document.querySelector('label[for="' + element.id + '"]');
			if (label) return label.textContent.trim();
		}
		// Try alt for images
		if (element.tagName === 'IMG' && element.alt) {
			return element.alt;
		}
		// Try title
		if (element.title) {
			return element.title;
		}
		// Try placeholder for inputs
		if (element.placeholder) {
			return element.placeholder;
		}
		// Use text content for simple elements
		const text = element.textContent.trim();
		return text.length < 100 ? text : text.substring(0, 100) + '...';
	}

	function getRole(element) {
		// Explicit role
		const explicitRole = element.getAttribute('role');
		if (explicitRole) return explicitRole;

		// Implicit roles based on tag
		const implicitRoles = {
			'A': element.href ? 'link' : null,
			'BUTTON': 'button',
			'INPUT': {
				'button': 'button',
				'checkbox': 'checkbox',
				'radio': 'radio',
				'text': 'textbox',
				'password': 'textbox',
				'email': 'textbox',
				'search': 'searchbox',
				'submit': 'button',
				'reset': 'button'
			}[element.type] || 'textbox',
			'SELECT': 'combobox',
			'TEXTAREA': 'textbox',
			'IMG': 'img',
			'NAV': 'navigation',
			'MAIN': 'main',
			'HEADER': 'banner',
			'FOOTER': 'contentinfo',
			'ASIDE': 'complementary',
			'ARTICLE': 'article',
			'SECTION': 'region',
			'FORM': 'form',
			'TABLE': 'table',
			'UL': 'list',
			'OL': 'list',
			'LI': 'listitem',
			'H1': 'heading',
			'H2': 'heading',
			'H3': 'heading',
			'H4': 'heading',
			'H5': 'heading',
			'H6': 'heading'
		};
		return implicitRoles[element.tagName] || null;
	}

	function buildNode(element) {
		const role = getRole(element);
		const name = getAccessibleName(element);

		// Skip elements without role and name, unless they have children with roles
		const node = {
			role: role || 'generic',
			name: name || ''
		};

		// Add relevant properties
		if (element.disabled) node.disabled = true;
		if (element.checked) node.checked = true;
		if (element.selected) node.selected = true;
		if (element.getAttribute('aria-expanded') !== null) {
			node.expanded = element.getAttribute('aria-expanded') === 'true';
		}
		if (element.getAttribute('aria-level')) {
			node.level = parseInt(element.getAttribute('aria-level'));
		}
		if (element.tagName.match(/^H[1-6]$/)) {
			node.level = parseInt(element.tagName[1]);
		}

		// Process children
		const children = [];
		for (const child of element.children) {
			const style = window.getComputedStyle(child);
			if (style.display !== 'none' && style.visibility !== 'hidden') {
				const childNode = buildNode(child);
				if (childNode.role !== 'generic' || childNode.name || childNode.children) {
					children.push(childNode);
				}
			}
		}
		if (children.length > 0) {
			node.children = children;
		}

		return node;
	}

	return buildNode(document.body);
}`

// NavigateAndExtractTextTool returns the navigate_and_extract_text MCP tool definition.
func NavigateAndExtractTextTool() mcp.Tool {
	return mcp.NewTool("navigate_and_extract_text",
		mcp.WithDescription("Navigate to URL and extract text (creates temporary session)"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("URL to navigate to"),
		),
		mcp.WithString("waitUntil",
			mcp.Description("Wait condition: load, domcontentloaded, networkidle"),
			mcp.Enum("load", "domcontentloaded", "networkidle"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector to extract from (defaults to body)"),
		),
		mcp.WithString("browserType",
			mcp.Description("Browser to use: chromium, firefox, webkit"),
			mcp.Enum("chromium", "firefox", "webkit"),
		),
	)
}

// NavigateAndExtractTextHandler returns the handler function for navigate_and_extract_text.
func NavigateAndExtractTextHandler(mgr session.BrowserSessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract MCP session ID from context for ownership tracking
		mcpSessionID := getMCPSessionID(ctx)

		// Parse and validate url argument
		url := req.GetString("url", "")
		if url == "" {
			return mcp.NewToolResultError("url is required"), nil
		}

		// Parse optional arguments
		args := req.GetArguments()

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
				return mcp.NewToolResultError(fmt.Sprintf("Invalid browserType: %s. Must be chromium, firefox, or webkit", btStr)), nil
			}
		}

		// Create temporary session
		opts := session.SessionOptions{
			BrowserType: browserType,
			Headless:    true,
		}

		sess, err := mgr.CreateSession(ctx, mcpSessionID, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create temporary session: %v", err)), nil
		}

		// Ensure session cleanup on exit
		defer func() {
			_ = mgr.CloseSession(ctx, mcpSessionID, sess.ID)
		}()

		// Get the page from the session
		page := sess.ActivePage()
		if page == nil {
			return mcp.NewToolResultError("no active page in temporary session"), nil
		}

		// Build navigation options
		gotoOpts := playwright.PageGotoOptions{}

		if waitUntil, ok := args["waitUntil"].(string); ok && waitUntil != "" {
			gotoOpts.WaitUntil = mapWaitUntil(waitUntil)
		}

		// Navigate to URL
		if _, err := page.Goto(url, gotoOpts); err != nil {
			if isTimeoutError(err) {
				return mcp.NewToolResultError(fmt.Sprintf("[%d] Navigation timeout for %s: %v", errors.CodeTimeout, url, err)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Navigation failed for %s: %v", errors.CodeNavigationFailed, url, err)), nil
		}

		// Get selector or default to body
		selector := req.GetString("selector", "body")

		// Execute JavaScript to extract hierarchical text structure
		result, err := page.Evaluate(extractTextJS, selector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to extract text: %v", err)), nil
		}

		// Check if element was found (JavaScript returns null if selector doesn't match)
		if result == nil {
			return mcp.NewToolResultError(fmt.Sprintf("[%d] Element not found: %s", errors.CodeElementNotFound, selector)), nil
		}

		// Serialize result to JSON
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize text structure: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
