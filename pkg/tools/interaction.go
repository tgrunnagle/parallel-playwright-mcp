// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// ClickTool returns the click MCP tool definition.
// Click simulates a mouse click on an element identified by selector.
func ClickTool() mcp.Tool {
	return mcp.NewTool("click",
		mcp.WithDescription("Click an element on the page. Target it with exactly one of selector or ref; a ref comes from get_accessibility_tree and fails at once with 'Stale element ref' once the page has changed"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector, XPath, or text selector for element to click. Provide exactly one of selector or ref"),
		),
		mcp.WithString("ref",
			mcp.Description(refParamDescription),
		),
		mcp.WithString("button",
			mcp.Description("Mouse button to use (left, right, middle)"),
			mcp.Enum("left", "right", "middle"),
		),
		mcp.WithNumber("clickCount",
			mcp.Description("Number of clicks (1 for single, 2 for double)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Maximum time to wait for element in milliseconds"),
		),
	)
}

// ClickHandler returns the handler function for click.
func ClickHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "click")

		// Apply timeout with element category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutElement, timeoutConfig)
		defer cancel()

		mcpSessionID := getMCPSessionID(ctx)

		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "click")
			return requiredParamResult("sessionId"), nil
		}

		target, targetErr := resolveTarget(args, false)
		if targetErr != nil {
			slog.Error("invalid element target", "tool", "click", "sessionID", sessionID, "error", targetErr)
			return errorResult(targetErr), nil
		}
		selector := target.Label

		slog.Debug("clicking element", "tool", "click", "sessionID", sessionID, "selector", selector)

		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "click", "sessionID", sessionID)
			return newInteractionSessionNotFoundError(sessionID), nil
		}

		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "click", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		rec := startAction(sess, page, "click", target.Label, timeoutConfig)

		locator, locErr := target.locate(page)
		if locErr != nil {
			slog.Error("element target did not resolve", "sessionID", sessionID, "target", selector, "error", locErr)
			return errorResult(locErr), nil
		}
		opts := buildClickOptions(args)

		// Propagate context deadline to Playwright if no explicit timeout was set
		if opts.Timeout == nil {
			opts.Timeout = PlaywrightTimeoutFromContext(ctx)
		}

		if err := locator.Click(opts); err != nil {
			if ctxErr := HandleContextError(ctx, fmt.Sprintf("click on %s", selector)); ctxErr != nil {
				slog.Error("click context error", "tool", "click", "sessionID", sessionID, "selector", selector, "error", ctxErr)
				return errorResult(ctxErr), nil
			}
			return handleInteractionError(err, "click", selector), nil
		}

		slog.Debug("element clicked", "tool", "click", "sessionID", sessionID, "selector", selector)
		return rec.finish(ctx, fmt.Sprintf("Clicked element: %s", selector)), nil
	}
}

// TypeTool returns the type MCP tool definition.
// Type simulates typing text into an element with keystroke events.
func TypeTool() mcp.Tool {
	return mcp.NewTool("type",
		mcp.WithDescription("Type text into an element with keystroke events. Target it with exactly one of selector or ref; a ref comes from get_accessibility_tree and fails at once with 'Stale element ref' once the page has changed"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector for input element. Provide exactly one of selector or ref"),
		),
		mcp.WithString("ref",
			mcp.Description(refParamDescription),
		),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("Text to type"),
		),
		mcp.WithNumber("delay",
			mcp.Description("Delay between keystrokes in milliseconds"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Maximum time to wait for element in milliseconds"),
		),
	)
}

// TypeHandler returns the handler function for type.
func TypeHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "type")

		// Apply timeout with element category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutElement, timeoutConfig)
		defer cancel()

		mcpSessionID := getMCPSessionID(ctx)

		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "type")
			return requiredParamResult("sessionId"), nil
		}

		target, targetErr := resolveTarget(args, false)
		if targetErr != nil {
			slog.Error("invalid element target", "tool", "type", "sessionID", sessionID, "error", targetErr)
			return errorResult(targetErr), nil
		}
		selector := target.Label

		text := req.GetString("text", "")
		if text == "" {
			slog.Error("missing required text", "tool", "type", "sessionID", sessionID)
			return requiredParamResult("text"), nil
		}

		slog.Debug("typing into element", "tool", "type", "sessionID", sessionID, "selector", selector)

		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "type", "sessionID", sessionID)
			return newInteractionSessionNotFoundError(sessionID), nil
		}

		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "type", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		rec := startAction(sess, page, "type", target.Label, timeoutConfig)

		locator, locErr := target.locate(page)
		if locErr != nil {
			slog.Error("element target did not resolve", "sessionID", sessionID, "target", selector, "error", locErr)
			return errorResult(locErr), nil
		}
		opts := buildTypeOptions(args)

		if err := locator.PressSequentially(text, opts); err != nil {
			if ctxErr := HandleContextError(ctx, fmt.Sprintf("type on %s", selector)); ctxErr != nil {
				slog.Error("type context error", "tool", "type", "sessionID", sessionID, "selector", selector, "error", ctxErr)
				return errorResult(ctxErr), nil
			}
			return handleInteractionError(err, "type", selector), nil
		}

		slog.Debug("typed into element", "tool", "type", "sessionID", sessionID, "selector", selector)
		return rec.finish(ctx, fmt.Sprintf("Typed text into element: %s", selector)), nil
	}
}

// FillTool returns the fill MCP tool definition.
// Fill populates an input field with text without keystroke events.
func FillTool() mcp.Tool {
	return mcp.NewTool("fill",
		mcp.WithDescription("Fill an input field with text (fast, no keystroke events). Target it with exactly one of selector or ref; a ref comes from get_accessibility_tree and fails at once with 'Stale element ref' once the page has changed"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector for input element. Provide exactly one of selector or ref"),
		),
		mcp.WithString("ref",
			mcp.Description(refParamDescription),
		),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("Value to fill into the input"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Maximum time to wait for element in milliseconds"),
		),
	)
}

// FillHandler returns the handler function for fill.
func FillHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "fill")

		// Apply timeout with element category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutElement, timeoutConfig)
		defer cancel()

		mcpSessionID := getMCPSessionID(ctx)

		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "fill")
			return requiredParamResult("sessionId"), nil
		}

		target, targetErr := resolveTarget(args, false)
		if targetErr != nil {
			slog.Error("invalid element target", "tool", "fill", "sessionID", sessionID, "error", targetErr)
			return errorResult(targetErr), nil
		}
		selector := target.Label

		value := req.GetString("value", "")
		// Note: Allow empty value since user may want to clear input

		slog.Debug("filling element", "tool", "fill", "sessionID", sessionID, "selector", selector)

		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "fill", "sessionID", sessionID)
			return newInteractionSessionNotFoundError(sessionID), nil
		}

		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "fill", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		rec := startAction(sess, page, "fill", target.Label, timeoutConfig)

		locator, locErr := target.locate(page)
		if locErr != nil {
			slog.Error("element target did not resolve", "sessionID", sessionID, "target", selector, "error", locErr)
			return errorResult(locErr), nil
		}
		opts := buildFillOptions(args)

		if err := locator.Fill(value, opts); err != nil {
			if ctxErr := HandleContextError(ctx, fmt.Sprintf("fill on %s", selector)); ctxErr != nil {
				slog.Error("fill context error", "tool", "fill", "sessionID", sessionID, "selector", selector, "error", ctxErr)
				return errorResult(ctxErr), nil
			}
			return handleInteractionError(err, "fill", selector), nil
		}

		slog.Debug("element filled", "tool", "fill", "sessionID", sessionID, "selector", selector)
		return rec.finish(ctx, fmt.Sprintf("Filled element: %s", selector)), nil
	}
}

// SelectOptionTool returns the select_option MCP tool definition.
// SelectOption selects an option from a dropdown/select element.
func SelectOptionTool() mcp.Tool {
	return mcp.NewTool("select_option",
		mcp.WithDescription("Select an option from a dropdown/select element. Target it with exactly one of selector or ref; a ref comes from get_accessibility_tree and fails at once with 'Stale element ref' once the page has changed"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector for select element. Provide exactly one of selector or ref"),
		),
		mcp.WithString("ref",
			mcp.Description(refParamDescription),
		),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("Option value, label, or index to select"),
		),
		mcp.WithString("selectBy",
			mcp.Description("Selection method: value, label, or index (default: value)"),
			mcp.Enum("value", "label", "index"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Maximum time to wait for element in milliseconds"),
		),
	)
}

// SelectOptionHandler returns the handler function for select_option.
func SelectOptionHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "select_option")

		// Apply timeout with element category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutElement, timeoutConfig)
		defer cancel()

		mcpSessionID := getMCPSessionID(ctx)

		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "select_option")
			return requiredParamResult("sessionId"), nil
		}

		target, targetErr := resolveTarget(args, false)
		if targetErr != nil {
			slog.Error("invalid element target", "tool", "select_option", "sessionID", sessionID, "error", targetErr)
			return errorResult(targetErr), nil
		}
		selector := target.Label

		value := req.GetString("value", "")
		if value == "" {
			slog.Error("missing required value", "tool", "select_option", "sessionID", sessionID)
			return requiredParamResult("value"), nil
		}

		selectBy := req.GetString("selectBy", "value")

		slog.Debug("selecting option", "tool", "select_option", "sessionID", sessionID, "selector", selector, "selectBy", selectBy)

		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "select_option", "sessionID", sessionID)
			return newInteractionSessionNotFoundError(sessionID), nil
		}

		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "select_option", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		rec := startAction(sess, page, "select_option", target.Label, timeoutConfig)

		locator, locErr := target.locate(page)
		if locErr != nil {
			slog.Error("element target did not resolve", "sessionID", sessionID, "target", selector, "error", locErr)
			return errorResult(locErr), nil
		}
		opts := buildSelectOptionOptions(args)

		var selectedValues []string
		var err error

		switch selectBy {
		case "label":
			selectedValues, err = locator.SelectOption(playwright.SelectOptionValues{
				Labels: &[]string{value},
			}, opts)
		case "index":
			idx, parseErr := strconv.Atoi(value)
			if parseErr != nil {
				return paramErrorResult("value", fmt.Sprintf("Invalid index value: %s", value)), nil
			}
			selectedValues, err = locator.SelectOption(playwright.SelectOptionValues{
				Indexes: &[]int{idx},
			}, opts)
		default: // "value"
			selectedValues, err = locator.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{value},
			}, opts)
		}

		if err != nil {
			if ctxErr := HandleContextError(ctx, fmt.Sprintf("select_option on %s", selector)); ctxErr != nil {
				return errorResult(ctxErr), nil
			}
			return handleInteractionError(err, "select_option", selector), nil
		}

		if len(selectedValues) == 0 {
			return rec.finish(ctx, fmt.Sprintf("Selected option by %s in: %s (no value returned)", selectBy, selector)), nil
		}
		return rec.finish(ctx, fmt.Sprintf("Selected option '%s' by %s in: %s", selectedValues[0], selectBy, selector)), nil
	}
}

// HoverTool returns the hover MCP tool definition.
// Hover simulates a mouse hover over an element.
func HoverTool() mcp.Tool {
	return mcp.NewTool("hover",
		mcp.WithDescription("Hover over an element. Target it with exactly one of selector or ref; a ref comes from get_accessibility_tree and fails at once with 'Stale element ref' once the page has changed"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector for element to hover over. Provide exactly one of selector or ref"),
		),
		mcp.WithString("ref",
			mcp.Description(refParamDescription),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Maximum time to wait for element in milliseconds"),
		),
	)
}

// HoverHandler returns the handler function for hover.
func HoverHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "hover")

		// Apply timeout with element category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutElement, timeoutConfig)
		defer cancel()

		mcpSessionID := getMCPSessionID(ctx)

		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "hover")
			return requiredParamResult("sessionId"), nil
		}

		target, targetErr := resolveTarget(args, false)
		if targetErr != nil {
			slog.Error("invalid element target", "tool", "hover", "sessionID", sessionID, "error", targetErr)
			return errorResult(targetErr), nil
		}
		selector := target.Label

		slog.Debug("hovering over element", "tool", "hover", "sessionID", sessionID, "selector", selector)

		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "hover", "sessionID", sessionID)
			return newInteractionSessionNotFoundError(sessionID), nil
		}

		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "hover", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		rec := startAction(sess, page, "hover", target.Label, timeoutConfig)

		locator, locErr := target.locate(page)
		if locErr != nil {
			slog.Error("element target did not resolve", "sessionID", sessionID, "target", selector, "error", locErr)
			return errorResult(locErr), nil
		}
		opts := buildHoverOptions(args)

		if err := locator.Hover(opts); err != nil {
			if ctxErr := HandleContextError(ctx, fmt.Sprintf("hover on %s", selector)); ctxErr != nil {
				slog.Error("hover context error", "tool", "hover", "sessionID", sessionID, "selector", selector, "error", ctxErr)
				return errorResult(ctxErr), nil
			}
			return handleInteractionError(err, "hover", selector), nil
		}

		slog.Debug("hovered over element", "tool", "hover", "sessionID", sessionID, "selector", selector)
		return rec.finish(ctx, fmt.Sprintf("Hovered over element: %s", selector)), nil
	}
}

// PressKeyTool returns the press_key MCP tool definition.
// PressKey simulates pressing a keyboard key or key combination.
func PressKeyTool() mcp.Tool {
	return mcp.NewTool("press_key",
		mcp.WithDescription("Press a keyboard key or key combination"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Browser session ID"),
		),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("Key to press (e.g., 'Enter', 'Tab', 'a')"),
		),
		mcp.WithString("selector",
			mcp.Description("Optional CSS selector to focus before pressing key. Give at most one of selector or ref; with neither, the key goes to the page"),
		),
		mcp.WithString("ref",
			mcp.Description("Optional element ref from get_accessibility_tree to focus before pressing key, such as e3 or f1e2"),
		),
		mcp.WithArray("modifiers",
			mcp.Description("Modifier keys to hold during press"),
			mcp.Items(map[string]any{
				"type": "string",
				"enum": []string{"Control", "Shift", "Alt", "Meta"},
			}),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Maximum time to wait for element in milliseconds"),
		),
	)
}

// PressKeyHandler returns the handler function for press_key.
func PressKeyHandler(mgr session.BrowserSessionManager, timeoutConfig *TimeoutConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool handler called", "tool", "press_key")

		// Apply timeout with element category
		args := req.GetArguments()
		ctx, cancel := ApplyTimeout(ctx, args, TimeoutElement, timeoutConfig)
		defer cancel()

		mcpSessionID := getMCPSessionID(ctx)

		sessionID := req.GetString("sessionId", "")
		if sessionID == "" {
			slog.Error("missing required sessionId", "tool", "press_key")
			return requiredParamResult("sessionId"), nil
		}

		key := req.GetString("key", "")
		if key == "" {
			slog.Error("missing required key", "tool", "press_key", "sessionID", sessionID)
			return requiredParamResult("key"), nil
		}

		target, targetErr := resolveTarget(args, true)
		if targetErr != nil {
			slog.Error("invalid element target", "tool", "press_key", "sessionID", sessionID, "error", targetErr)
			return errorResult(targetErr), nil
		}
		selector := target.Label

		// Build key string with modifiers if provided
		keyWithModifiers := buildKeyWithModifiers(key, args)

		slog.Debug("pressing key", "tool", "press_key", "sessionID", sessionID, "key", keyWithModifiers, "selector", selector)

		sess, ok := mgr.GetSession(mcpSessionID, sessionID)
		if !ok {
			slog.Error("session not found", "tool", "press_key", "sessionID", sessionID)
			return newInteractionSessionNotFoundError(sessionID), nil
		}

		page := sess.ActivePage()
		if page == nil {
			slog.Error("no active page in session", "tool", "press_key", "sessionID", sessionID)
			return noActivePageResult(), nil
		}

		rec := startAction(sess, page, "press_key", target.Label, timeoutConfig)

		// If a target is provided, use locator.Press, otherwise use page.Keyboard.Press
		if !target.isZero() {
			locator, locErr := target.locate(page)
			if locErr != nil {
				return errorResult(locErr), nil
			}
			opts := buildPressOptions(args)

			if err := locator.Press(keyWithModifiers, opts); err != nil {
				if ctxErr := HandleContextError(ctx, fmt.Sprintf("press_key on %s", selector)); ctxErr != nil {
					slog.Error("press_key context error", "tool", "press_key", "sessionID", sessionID, "key", keyWithModifiers, "error", ctxErr)
					return errorResult(ctxErr), nil
				}
				return handleInteractionError(err, "press_key", selector), nil
			}

			slog.Debug("key pressed on element", "tool", "press_key", "sessionID", sessionID, "key", keyWithModifiers, "selector", selector)
			return rec.finish(ctx, fmt.Sprintf("Pressed key '%s' on element: %s", keyWithModifiers, selector)), nil
		}

		// Press key without focusing specific element
		if err := page.Keyboard().Press(keyWithModifiers); err != nil {
			if ctxErr := HandleContextError(ctx, "press_key"); ctxErr != nil {
				slog.Error("press_key context error", "tool", "press_key", "sessionID", sessionID, "key", keyWithModifiers, "error", ctxErr)
				return errorResult(ctxErr), nil
			}
			return handleInteractionError(err, "press_key", "page"), nil
		}

		slog.Debug("key pressed on page", "tool", "press_key", "sessionID", sessionID, "key", keyWithModifiers)
		return rec.finish(ctx, fmt.Sprintf("Pressed key '%s'", keyWithModifiers)), nil
	}
}

// buildClickOptions constructs Playwright LocatorClickOptions from tool arguments.
func buildClickOptions(args map[string]any) playwright.LocatorClickOptions {
	opts := playwright.LocatorClickOptions{}

	if button, ok := args["button"].(string); ok && button != "" {
		btn := playwright.MouseButton(button)
		opts.Button = &btn
	}

	if clickCount, ok := args["clickCount"].(float64); ok && clickCount > 0 {
		count := int(clickCount)
		opts.ClickCount = &count
	}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// buildTypeOptions constructs Playwright LocatorPressSequentiallyOptions from tool arguments.
func buildTypeOptions(args map[string]any) playwright.LocatorPressSequentiallyOptions {
	opts := playwright.LocatorPressSequentiallyOptions{}

	if delay, ok := args["delay"].(float64); ok && delay > 0 {
		opts.Delay = playwright.Float(delay)
	}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// buildFillOptions constructs Playwright LocatorFillOptions from tool arguments.
func buildFillOptions(args map[string]any) playwright.LocatorFillOptions {
	opts := playwright.LocatorFillOptions{}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// buildSelectOptionOptions constructs Playwright LocatorSelectOptionOptions from tool arguments.
func buildSelectOptionOptions(args map[string]any) playwright.LocatorSelectOptionOptions {
	opts := playwright.LocatorSelectOptionOptions{}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// buildHoverOptions constructs Playwright LocatorHoverOptions from tool arguments.
func buildHoverOptions(args map[string]any) playwright.LocatorHoverOptions {
	opts := playwright.LocatorHoverOptions{}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// buildPressOptions constructs Playwright LocatorPressOptions from tool arguments.
func buildPressOptions(args map[string]any) playwright.LocatorPressOptions {
	opts := playwright.LocatorPressOptions{}

	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		opts.Timeout = playwright.Float(timeout)
	}

	return opts
}

// buildKeyWithModifiers combines the key with any modifiers provided.
// Returns a key string in Playwright format (e.g., "Control+Shift+a").
func buildKeyWithModifiers(key string, args map[string]any) string {
	modifiersArg, ok := args["modifiers"]
	if !ok || modifiersArg == nil {
		return key
	}

	// modifiers comes as []interface{} from JSON
	modifiersSlice, ok := modifiersArg.([]interface{})
	if !ok || len(modifiersSlice) == 0 {
		return key
	}

	// Build modifier prefix
	var modifiers []string
	for _, m := range modifiersSlice {
		if modStr, ok := m.(string); ok && modStr != "" {
			modifiers = append(modifiers, modStr)
		}
	}

	if len(modifiers) == 0 {
		return key
	}

	// Combine modifiers with key (e.g., "Control+Shift+a")
	return strings.Join(append(modifiers, key), "+")
}

// newInteractionSessionNotFoundError creates an error result for invalid session ID.
// Uses error code -32001 (Session Not Found).
func newInteractionSessionNotFoundError(sessionID string) *mcp.CallToolResult {
	return mcp.NewToolResultError(errors.FormatErrorForTool(errors.NewSessionNotFoundError(sessionID)))
}

// handleInteractionError classifies interaction errors and returns appropriate error results.
// Element not found errors get code -32002, timeout errors get code -32003,
// unclassified interaction errors get code -32005.
func handleInteractionError(err error, operation, selector string) *mcp.CallToolResult {
	errMsg := strings.ToLower(err.Error())

	// Check for element not found or not actionable errors
	if strings.Contains(errMsg, "strict mode violation") ||
		strings.Contains(errMsg, "resolved to") ||
		strings.Contains(errMsg, "no element matches") ||
		strings.Contains(errMsg, "element is not attached") ||
		strings.Contains(errMsg, "element is not visible") ||
		strings.Contains(errMsg, "element is outside of the viewport") ||
		strings.Contains(errMsg, "element is not stable") ||
		strings.Contains(errMsg, "element is not editable") ||
		strings.Contains(errMsg, "element is disabled") {
		slog.Error("element not found or not actionable", "operation", operation, "selector", selector, "error", err)
		return mcp.NewToolResultError(errors.FormatErrorForTool(
			errors.WrapElementNotFoundError(selector, 0, err)))
	}

	// Check for timeout errors
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "exceeded") {
		slog.Error("interaction timeout", "operation", operation, "selector", selector, "error", err)
		return mcp.NewToolResultError(errors.FormatErrorForTool(
			errors.WrapTimeoutError(fmt.Sprintf("%s on %s", operation, selector), 0, err)))
	}

	// Return generic interaction error for unclassified errors
	slog.Error("interaction failed", "operation", operation, "selector", selector, "error", err)
	return errorResult(errors.WrapInteractionError(operation, selector, err))
}
