package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/playwright-community/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// mockLocator is a mock implementation of playwright.Locator for testing.
// Note: playwright.Locator has a Locator() method which conflicts with embedding,
// so we don't embed it and instead implement only the methods we need for testing.
type mockLocator struct {
	clickFunc             func(opts playwright.LocatorClickOptions) error
	fillFunc              func(value string, opts playwright.LocatorFillOptions) error
	pressSequentiallyFunc func(text string, opts playwright.LocatorPressSequentiallyOptions) error
	hoverFunc             func(opts playwright.LocatorHoverOptions) error
	pressFunc             func(key string, opts playwright.LocatorPressOptions) error
	selectOptionFunc      func(values playwright.SelectOptionValues, opts playwright.LocatorSelectOptionOptions) ([]string, error)
}

func (m *mockLocator) Click(opts ...playwright.LocatorClickOptions) error {
	if m.clickFunc != nil {
		opt := playwright.LocatorClickOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.clickFunc(opt)
	}
	return nil
}

func (m *mockLocator) Fill(value string, opts ...playwright.LocatorFillOptions) error {
	if m.fillFunc != nil {
		opt := playwright.LocatorFillOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.fillFunc(value, opt)
	}
	return nil
}

func (m *mockLocator) PressSequentially(text string, opts ...playwright.LocatorPressSequentiallyOptions) error {
	if m.pressSequentiallyFunc != nil {
		opt := playwright.LocatorPressSequentiallyOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.pressSequentiallyFunc(text, opt)
	}
	return nil
}

func (m *mockLocator) Hover(opts ...playwright.LocatorHoverOptions) error {
	if m.hoverFunc != nil {
		opt := playwright.LocatorHoverOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.hoverFunc(opt)
	}
	return nil
}

func (m *mockLocator) Press(key string, opts ...playwright.LocatorPressOptions) error {
	if m.pressFunc != nil {
		opt := playwright.LocatorPressOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.pressFunc(key, opt)
	}
	return nil
}

func (m *mockLocator) SelectOption(values playwright.SelectOptionValues, opts ...playwright.LocatorSelectOptionOptions) ([]string, error) {
	if m.selectOptionFunc != nil {
		opt := playwright.LocatorSelectOptionOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.selectOptionFunc(values, opt)
	}
	return []string{}, nil
}

// Stub implementations for playwright.Locator interface methods not used in tests
func (m *mockLocator) All() ([]playwright.Locator, error)                { return nil, nil }
func (m *mockLocator) AllInnerTexts() ([]string, error)                  { return nil, nil }
func (m *mockLocator) AllTextContents() ([]string, error)                { return nil, nil }
func (m *mockLocator) And(locator playwright.Locator) playwright.Locator { return m }
func (m *mockLocator) AriaSnapshot(options ...playwright.LocatorAriaSnapshotOptions) (string, error) {
	return "", nil
}
func (m *mockLocator) Blur(options ...playwright.LocatorBlurOptions) error { return nil }
func (m *mockLocator) BoundingBox(options ...playwright.LocatorBoundingBoxOptions) (*playwright.Rect, error) {
	return nil, nil
}
func (m *mockLocator) Check(options ...playwright.LocatorCheckOptions) error       { return nil }
func (m *mockLocator) Clear(options ...playwright.LocatorClearOptions) error       { return nil }
func (m *mockLocator) Count() (int, error)                                         { return 0, nil }
func (m *mockLocator) Dblclick(options ...playwright.LocatorDblclickOptions) error { return nil }
func (m *mockLocator) DispatchEvent(typ string, eventInit interface{}, options ...playwright.LocatorDispatchEventOptions) error {
	return nil
}
func (m *mockLocator) DragTo(target playwright.Locator, options ...playwright.LocatorDragToOptions) error {
	return nil
}
func (m *mockLocator) ElementHandle(options ...playwright.LocatorElementHandleOptions) (playwright.ElementHandle, error) {
	return nil, nil
}
func (m *mockLocator) ElementHandles() ([]playwright.ElementHandle, error) { return nil, nil }
func (m *mockLocator) ContentFrame() playwright.FrameLocator               { return nil }
func (m *mockLocator) Evaluate(expression string, arg interface{}, options ...playwright.LocatorEvaluateOptions) (interface{}, error) {
	return nil, nil
}
func (m *mockLocator) EvaluateAll(expression string, arg ...interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockLocator) EvaluateHandle(expression string, arg interface{}, options ...playwright.LocatorEvaluateHandleOptions) (playwright.JSHandle, error) {
	return nil, nil
}
func (m *mockLocator) Filter(options ...playwright.LocatorFilterOptions) playwright.Locator { return m }
func (m *mockLocator) First() playwright.Locator                                            { return m }
func (m *mockLocator) Focus(options ...playwright.LocatorFocusOptions) error                { return nil }
func (m *mockLocator) FrameLocator(selector string) playwright.FrameLocator                 { return nil }
func (m *mockLocator) GetAttribute(name string, options ...playwright.LocatorGetAttributeOptions) (string, error) {
	return "", nil
}
func (m *mockLocator) GetByAltText(text interface{}, options ...playwright.LocatorGetByAltTextOptions) playwright.Locator {
	return m
}
func (m *mockLocator) GetByLabel(text interface{}, options ...playwright.LocatorGetByLabelOptions) playwright.Locator {
	return m
}
func (m *mockLocator) GetByPlaceholder(text interface{}, options ...playwright.LocatorGetByPlaceholderOptions) playwright.Locator {
	return m
}
func (m *mockLocator) GetByRole(role playwright.AriaRole, options ...playwright.LocatorGetByRoleOptions) playwright.Locator {
	return m
}
func (m *mockLocator) GetByTestId(testId interface{}) playwright.Locator { return m }
func (m *mockLocator) GetByText(text interface{}, options ...playwright.LocatorGetByTextOptions) playwright.Locator {
	return m
}
func (m *mockLocator) GetByTitle(text interface{}, options ...playwright.LocatorGetByTitleOptions) playwright.Locator {
	return m
}
func (m *mockLocator) Highlight() error { return nil }
func (m *mockLocator) InnerHTML(options ...playwright.LocatorInnerHTMLOptions) (string, error) {
	return "", nil
}
func (m *mockLocator) InnerText(options ...playwright.LocatorInnerTextOptions) (string, error) {
	return "", nil
}
func (m *mockLocator) InputValue(options ...playwright.LocatorInputValueOptions) (string, error) {
	return "", nil
}
func (m *mockLocator) IsChecked(options ...playwright.LocatorIsCheckedOptions) (bool, error) {
	return false, nil
}
func (m *mockLocator) IsDisabled(options ...playwright.LocatorIsDisabledOptions) (bool, error) {
	return false, nil
}
func (m *mockLocator) IsEditable(options ...playwright.LocatorIsEditableOptions) (bool, error) {
	return false, nil
}
func (m *mockLocator) IsEnabled(options ...playwright.LocatorIsEnabledOptions) (bool, error) {
	return false, nil
}
func (m *mockLocator) IsHidden(options ...playwright.LocatorIsHiddenOptions) (bool, error) {
	return false, nil
}
func (m *mockLocator) IsVisible(options ...playwright.LocatorIsVisibleOptions) (bool, error) {
	return false, nil
}
func (m *mockLocator) Last() playwright.Locator { return m }
func (m *mockLocator) Locator(selectorOrLocator interface{}, options ...playwright.LocatorLocatorOptions) playwright.Locator {
	return m
}
func (m *mockLocator) Nth(index int) playwright.Locator                 { return m }
func (m *mockLocator) Or(locator playwright.Locator) playwright.Locator { return m }
func (m *mockLocator) Page() (playwright.Page, error)                   { return nil, nil }
func (m *mockLocator) Screenshot(options ...playwright.LocatorScreenshotOptions) ([]byte, error) {
	return nil, nil
}
func (m *mockLocator) ScrollIntoViewIfNeeded(options ...playwright.LocatorScrollIntoViewIfNeededOptions) error {
	return nil
}
func (m *mockLocator) SelectText(options ...playwright.LocatorSelectTextOptions) error { return nil }
func (m *mockLocator) SetChecked(checked bool, options ...playwright.LocatorSetCheckedOptions) error {
	return nil
}
func (m *mockLocator) SetInputFiles(files interface{}, options ...playwright.LocatorSetInputFilesOptions) error {
	return nil
}
func (m *mockLocator) Tap(options ...playwright.LocatorTapOptions) error { return nil }
func (m *mockLocator) TextContent(options ...playwright.LocatorTextContentOptions) (string, error) {
	return "", nil
}
func (m *mockLocator) Type(text string, options ...playwright.LocatorTypeOptions) error { return nil }
func (m *mockLocator) Uncheck(options ...playwright.LocatorUncheckOptions) error        { return nil }
func (m *mockLocator) WaitFor(options ...playwright.LocatorWaitForOptions) error        { return nil }
func (m *mockLocator) Err() error                                                       { return nil }

// mockInteractionPage is a mock implementation of playwright.Page for interaction testing.
type mockInteractionPage struct {
	playwright.Page
	locatorFunc  func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator
	keyboardFunc func() playwright.Keyboard
}

func (m *mockInteractionPage) Locator(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
	if m.locatorFunc != nil {
		return m.locatorFunc(selector, opts...)
	}
	panic("mockInteractionPage.Locator called without locatorFunc set")
}

func (m *mockInteractionPage) Keyboard() playwright.Keyboard {
	if m.keyboardFunc != nil {
		return m.keyboardFunc()
	}
	panic("mockInteractionPage.Keyboard called without keyboardFunc set")
}

// mockKeyboard is a mock implementation of playwright.Keyboard for testing.
type mockKeyboard struct {
	playwright.Keyboard
	pressFunc func(key string, opts ...playwright.KeyboardPressOptions) error
}

func (m *mockKeyboard) Press(key string, opts ...playwright.KeyboardPressOptions) error {
	if m.pressFunc != nil {
		return m.pressFunc(key, opts...)
	}
	return nil
}

// createInteractionSessionWithMockPage creates a BrowserSession with a mock page for interaction testing.
func createInteractionSessionWithMockPage(page *mockInteractionPage) *session.BrowserSession {
	sess := &session.BrowserSession{
		ID:          "sess-123",
		ActiveTabID: "tab-1",
		Pages:       make(map[string]playwright.Page),
	}
	if page != nil {
		sess.Pages["tab-1"] = page
	}
	return sess
}

// TestClickTool tests the click tool definition.
func TestClickTool(t *testing.T) {
	tool := ClickTool()

	if tool.Name != "click" {
		t.Errorf("Expected tool name 'click', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	schema := tool.InputSchema
	if schema.Type != "object" {
		t.Errorf("Expected input schema type 'object', got '%s'", schema.Type)
	}

	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	// Check required parameters
	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["selector"]; !ok {
		t.Error("Expected 'selector' property in input schema")
	}
	// Check optional parameters
	if _, ok := props["button"]; !ok {
		t.Error("Expected 'button' property in input schema")
	}
	if _, ok := props["clickCount"]; !ok {
		t.Error("Expected 'clickCount' property in input schema")
	}
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify required fields
	required := schema.Required
	foundSessionId := false
	foundSelector := false
	for _, r := range required {
		if r == "sessionId" {
			foundSessionId = true
		}
		if r == "selector" {
			foundSelector = true
		}
	}
	if !foundSessionId {
		t.Error("Expected 'sessionId' to be in required properties")
	}
	if !foundSelector {
		t.Error("Expected 'selector' to be in required properties")
	}
}

// TestTypeTool tests the type tool definition.
func TestTypeTool(t *testing.T) {
	tool := TypeTool()

	if tool.Name != "type" {
		t.Errorf("Expected tool name 'type', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	schema := tool.InputSchema
	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	// Check required parameters
	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["selector"]; !ok {
		t.Error("Expected 'selector' property in input schema")
	}
	if _, ok := props["text"]; !ok {
		t.Error("Expected 'text' property in input schema")
	}
	// Check optional parameters
	if _, ok := props["delay"]; !ok {
		t.Error("Expected 'delay' property in input schema")
	}
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify required fields
	required := schema.Required
	foundSessionId := false
	foundSelector := false
	foundText := false
	for _, r := range required {
		switch r {
		case "sessionId":
			foundSessionId = true
		case "selector":
			foundSelector = true
		case "text":
			foundText = true
		}
	}
	if !foundSessionId {
		t.Error("Expected 'sessionId' to be in required properties")
	}
	if !foundSelector {
		t.Error("Expected 'selector' to be in required properties")
	}
	if !foundText {
		t.Error("Expected 'text' to be in required properties")
	}
}

// TestFillTool tests the fill tool definition.
func TestFillTool(t *testing.T) {
	tool := FillTool()

	if tool.Name != "fill" {
		t.Errorf("Expected tool name 'fill', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	schema := tool.InputSchema
	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	// Check required parameters
	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["selector"]; !ok {
		t.Error("Expected 'selector' property in input schema")
	}
	if _, ok := props["value"]; !ok {
		t.Error("Expected 'value' property in input schema")
	}
	// Check optional parameters
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify required fields
	required := schema.Required
	foundSessionId := false
	foundSelector := false
	foundValue := false
	for _, r := range required {
		switch r {
		case "sessionId":
			foundSessionId = true
		case "selector":
			foundSelector = true
		case "value":
			foundValue = true
		}
	}
	if !foundSessionId {
		t.Error("Expected 'sessionId' to be in required properties")
	}
	if !foundSelector {
		t.Error("Expected 'selector' to be in required properties")
	}
	if !foundValue {
		t.Error("Expected 'value' to be in required properties")
	}
}

// TestSelectOptionTool tests the select_option tool definition.
func TestSelectOptionTool(t *testing.T) {
	tool := SelectOptionTool()

	if tool.Name != "select_option" {
		t.Errorf("Expected tool name 'select_option', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	schema := tool.InputSchema
	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	// Check required parameters
	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["selector"]; !ok {
		t.Error("Expected 'selector' property in input schema")
	}
	if _, ok := props["value"]; !ok {
		t.Error("Expected 'value' property in input schema")
	}
	// Check optional parameters
	if _, ok := props["selectBy"]; !ok {
		t.Error("Expected 'selectBy' property in input schema")
	}
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify required fields
	required := schema.Required
	foundSessionId := false
	foundSelector := false
	foundValue := false
	for _, r := range required {
		switch r {
		case "sessionId":
			foundSessionId = true
		case "selector":
			foundSelector = true
		case "value":
			foundValue = true
		}
	}
	if !foundSessionId {
		t.Error("Expected 'sessionId' to be in required properties")
	}
	if !foundSelector {
		t.Error("Expected 'selector' to be in required properties")
	}
	if !foundValue {
		t.Error("Expected 'value' to be in required properties")
	}
}

// TestHoverTool tests the hover tool definition.
func TestHoverTool(t *testing.T) {
	tool := HoverTool()

	if tool.Name != "hover" {
		t.Errorf("Expected tool name 'hover', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	schema := tool.InputSchema
	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	// Check required parameters
	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["selector"]; !ok {
		t.Error("Expected 'selector' property in input schema")
	}
	// Check optional parameters
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify required fields
	required := schema.Required
	foundSessionId := false
	foundSelector := false
	for _, r := range required {
		switch r {
		case "sessionId":
			foundSessionId = true
		case "selector":
			foundSelector = true
		}
	}
	if !foundSessionId {
		t.Error("Expected 'sessionId' to be in required properties")
	}
	if !foundSelector {
		t.Error("Expected 'selector' to be in required properties")
	}
}

// TestPressKeyTool tests the press_key tool definition.
func TestPressKeyTool(t *testing.T) {
	tool := PressKeyTool()

	if tool.Name != "press_key" {
		t.Errorf("Expected tool name 'press_key', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	schema := tool.InputSchema
	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	// Check required parameters
	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["key"]; !ok {
		t.Error("Expected 'key' property in input schema")
	}
	// Check optional parameters
	if _, ok := props["selector"]; !ok {
		t.Error("Expected 'selector' property in input schema")
	}
	if _, ok := props["modifiers"]; !ok {
		t.Error("Expected 'modifiers' property in input schema")
	}
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify required fields
	required := schema.Required
	foundSessionId := false
	foundKey := false
	for _, r := range required {
		switch r {
		case "sessionId":
			foundSessionId = true
		case "key":
			foundKey = true
		}
	}
	if !foundSessionId {
		t.Error("Expected 'sessionId' to be in required properties")
	}
	if !foundKey {
		t.Error("Expected 'key' to be in required properties")
	}
}

// TestClickHandler tests the click handler.
func TestClickHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
	}{
		{
			name: "missing sessionId",
			arguments: map[string]any{
				"selector": "#button",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "empty sessionId",
			arguments: map[string]any{
				"sessionId": "",
				"selector":  "#button",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "missing selector",
			arguments: map[string]any{
				"sessionId": "sess-123",
			},
			expectError:    true,
			expectedResult: "selector is required",
		},
		{
			name: "empty selector",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "",
			},
			expectError:    true,
			expectedResult: "selector is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
				"selector":  "#button",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
		{
			name: "nil page in session",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#button",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return createInteractionSessionWithMockPage(nil), true
			},
			expectError:    true,
			expectedResult: "no active page in session",
		},
		{
			name: "successful click",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#button",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					clickFunc: func(opts playwright.LocatorClickOptions) error {
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Clicked element: #button",
		},
		{
			name: "successful click with options",
			arguments: map[string]any{
				"sessionId":  "sess-123",
				"selector":   "#button",
				"button":     "right",
				"clickCount": float64(2),
				"timeout":    float64(5000),
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					clickFunc: func(opts playwright.LocatorClickOptions) error {
						if opts.Button == nil || *opts.Button != *playwright.MouseButtonRight {
							return errors.New("expected right button")
						}
						if opts.ClickCount == nil || *opts.ClickCount != 2 {
							return errors.New("expected clickCount 2")
						}
						if opts.Timeout == nil || *opts.Timeout != 5000 {
							return errors.New("expected timeout 5000")
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Clicked element: #button",
		},
		{
			name: "click timeout error",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#button",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					clickFunc: func(opts playwright.LocatorClickOptions) error {
						return errors.New("timeout exceeded")
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    true,
			expectedResult: "[-32003] Timeout",
		},
		{
			name: "click element not found error",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#nonexistent",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					clickFunc: func(opts playwright.LocatorClickOptions) error {
						return errors.New("no element matches selector")
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    true,
			expectedResult: "[-32002] Element not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}

			handler := ClickHandler(mgr)
			req := mcp.CallToolRequest{}
			req.Params.Name = "click"
			req.Params.Arguments = tt.arguments

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			text := extractTextContent(result.Content)
			if tt.expectError {
				if !result.IsError {
					t.Errorf("Expected error result, got success: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedResult, text)
				}
			} else {
				if result.IsError {
					t.Errorf("Expected success result, got error: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected result containing '%s', got '%s'", tt.expectedResult, text)
				}
			}
		})
	}
}

// TestTypeHandler tests the type handler.
func TestTypeHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
	}{
		{
			name: "missing sessionId",
			arguments: map[string]any{
				"selector": "#input",
				"text":     "hello",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "missing selector",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"text":      "hello",
			},
			expectError:    true,
			expectedResult: "selector is required",
		},
		{
			name: "missing text",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#input",
			},
			expectError:    true,
			expectedResult: "text is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
				"selector":  "#input",
				"text":      "hello",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
		{
			name: "nil page in session",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#input",
				"text":      "hello",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return createInteractionSessionWithMockPage(nil), true
			},
			expectError:    true,
			expectedResult: "no active page in session",
		},
		{
			name: "successful type",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#input",
				"text":      "hello world",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					pressSequentiallyFunc: func(text string, opts playwright.LocatorPressSequentiallyOptions) error {
						if text != "hello world" {
							return errors.New("unexpected text")
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Typed text into element: #input",
		},
		{
			name: "type with delay option",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#input",
				"text":      "test",
				"delay":     float64(100),
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					pressSequentiallyFunc: func(text string, opts playwright.LocatorPressSequentiallyOptions) error {
						if opts.Delay == nil || *opts.Delay != 100 {
							return errors.New("expected delay 100")
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Typed text into element: #input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}

			handler := TypeHandler(mgr)
			req := mcp.CallToolRequest{}
			req.Params.Name = "type"
			req.Params.Arguments = tt.arguments

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			text := extractTextContent(result.Content)
			if tt.expectError {
				if !result.IsError {
					t.Errorf("Expected error result, got success: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedResult, text)
				}
			} else {
				if result.IsError {
					t.Errorf("Expected success result, got error: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected result containing '%s', got '%s'", tt.expectedResult, text)
				}
			}
		})
	}
}

// TestFillHandler tests the fill handler.
func TestFillHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
	}{
		{
			name: "missing sessionId",
			arguments: map[string]any{
				"selector": "#input",
				"value":    "test value",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "missing selector",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"value":     "test value",
			},
			expectError:    true,
			expectedResult: "selector is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
				"selector":  "#input",
				"value":     "test value",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
		{
			name: "nil page in session",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#input",
				"value":     "test value",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return createInteractionSessionWithMockPage(nil), true
			},
			expectError:    true,
			expectedResult: "no active page in session",
		},
		{
			name: "successful fill",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#input",
				"value":     "test value",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					fillFunc: func(value string, opts playwright.LocatorFillOptions) error {
						if value != "test value" {
							return errors.New("unexpected value")
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Filled element: #input",
		},
		{
			name: "fill with empty value (clear input)",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#input",
				"value":     "",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					fillFunc: func(value string, opts playwright.LocatorFillOptions) error {
						if value != "" {
							return errors.New("expected empty value")
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Filled element: #input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}

			handler := FillHandler(mgr)
			req := mcp.CallToolRequest{}
			req.Params.Name = "fill"
			req.Params.Arguments = tt.arguments

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			text := extractTextContent(result.Content)
			if tt.expectError {
				if !result.IsError {
					t.Errorf("Expected error result, got success: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedResult, text)
				}
			} else {
				if result.IsError {
					t.Errorf("Expected success result, got error: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected result containing '%s', got '%s'", tt.expectedResult, text)
				}
			}
		})
	}
}

// TestSelectOptionHandler tests the select_option handler.
func TestSelectOptionHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
	}{
		{
			name: "missing sessionId",
			arguments: map[string]any{
				"selector": "#select",
				"value":    "option1",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "missing selector",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"value":     "option1",
			},
			expectError:    true,
			expectedResult: "selector is required",
		},
		{
			name: "missing value",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#select",
			},
			expectError:    true,
			expectedResult: "value is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
				"selector":  "#select",
				"value":     "option1",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
		{
			name: "nil page in session",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#select",
				"value":     "option1",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return createInteractionSessionWithMockPage(nil), true
			},
			expectError:    true,
			expectedResult: "no active page in session",
		},
		{
			name: "successful select by value",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#select",
				"value":     "option1",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					selectOptionFunc: func(values playwright.SelectOptionValues, opts playwright.LocatorSelectOptionOptions) ([]string, error) {
						if values.Values == nil || len(*values.Values) != 1 || (*values.Values)[0] != "option1" {
							return nil, errors.New("expected value option1")
						}
						return []string{"option1"}, nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Selected option 'option1' by value",
		},
		{
			name: "successful select by label",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#select",
				"value":     "Option One",
				"selectBy":  "label",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					selectOptionFunc: func(values playwright.SelectOptionValues, opts playwright.LocatorSelectOptionOptions) ([]string, error) {
						if values.Labels == nil || len(*values.Labels) != 1 || (*values.Labels)[0] != "Option One" {
							return nil, errors.New("expected label Option One")
						}
						return []string{"option1"}, nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Selected option 'option1' by label",
		},
		{
			name: "successful select by index",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#select",
				"value":     "2",
				"selectBy":  "index",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					selectOptionFunc: func(values playwright.SelectOptionValues, opts playwright.LocatorSelectOptionOptions) ([]string, error) {
						if values.Indexes == nil || len(*values.Indexes) != 1 || (*values.Indexes)[0] != 2 {
							return nil, errors.New("expected index 2")
						}
						return []string{"option3"}, nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Selected option 'option3' by index",
		},
		{
			name: "invalid index value",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#select",
				"value":     "not-a-number",
				"selectBy":  "index",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    true,
			expectedResult: "Invalid index value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}

			handler := SelectOptionHandler(mgr)
			req := mcp.CallToolRequest{}
			req.Params.Name = "select_option"
			req.Params.Arguments = tt.arguments

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			text := extractTextContent(result.Content)
			if tt.expectError {
				if !result.IsError {
					t.Errorf("Expected error result, got success: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedResult, text)
				}
			} else {
				if result.IsError {
					t.Errorf("Expected success result, got error: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected result containing '%s', got '%s'", tt.expectedResult, text)
				}
			}
		})
	}
}

// TestHoverHandler tests the hover handler.
func TestHoverHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
	}{
		{
			name: "missing sessionId",
			arguments: map[string]any{
				"selector": "#element",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "missing selector",
			arguments: map[string]any{
				"sessionId": "sess-123",
			},
			expectError:    true,
			expectedResult: "selector is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
				"selector":  "#element",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
		{
			name: "nil page in session",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#element",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return createInteractionSessionWithMockPage(nil), true
			},
			expectError:    true,
			expectedResult: "no active page in session",
		},
		{
			name: "successful hover",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#element",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					hoverFunc: func(opts playwright.LocatorHoverOptions) error {
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Hovered over element: #element",
		},
		{
			name: "hover with timeout",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"selector":  "#element",
				"timeout":   float64(3000),
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					hoverFunc: func(opts playwright.LocatorHoverOptions) error {
						if opts.Timeout == nil || *opts.Timeout != 3000 {
							return errors.New("expected timeout 3000")
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Hovered over element: #element",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}

			handler := HoverHandler(mgr)
			req := mcp.CallToolRequest{}
			req.Params.Name = "hover"
			req.Params.Arguments = tt.arguments

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			text := extractTextContent(result.Content)
			if tt.expectError {
				if !result.IsError {
					t.Errorf("Expected error result, got success: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedResult, text)
				}
			} else {
				if result.IsError {
					t.Errorf("Expected success result, got error: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected result containing '%s', got '%s'", tt.expectedResult, text)
				}
			}
		})
	}
}

// TestPressKeyHandler tests the press_key handler.
func TestPressKeyHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
	}{
		{
			name: "missing sessionId",
			arguments: map[string]any{
				"key": "Enter",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "missing key",
			arguments: map[string]any{
				"sessionId": "sess-123",
			},
			expectError:    true,
			expectedResult: "key is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
				"key":       "Enter",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
		{
			name: "nil page in session",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"key":       "Enter",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return createInteractionSessionWithMockPage(nil), true
			},
			expectError:    true,
			expectedResult: "no active page in session",
		},
		{
			name: "successful press key on page (no selector)",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"key":       "Enter",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				keyboard := &mockKeyboard{
					pressFunc: func(key string, opts ...playwright.KeyboardPressOptions) error {
						if key != "Enter" {
							return errors.New("expected key Enter")
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					keyboardFunc: func() playwright.Keyboard {
						return keyboard
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Pressed key 'Enter'",
		},
		{
			name: "successful press key on element (with selector)",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"key":       "Tab",
				"selector":  "#input",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					pressFunc: func(key string, opts playwright.LocatorPressOptions) error {
						if key != "Tab" {
							return errors.New("expected key Tab")
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Pressed key 'Tab' on element: #input",
		},
		{
			name: "press key combination",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"key":       "Control+a",
				"selector":  "#input",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					pressFunc: func(key string, opts playwright.LocatorPressOptions) error {
						if key != "Control+a" {
							return errors.New("expected key Control+a")
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Pressed key 'Control+a' on element: #input",
		},
		{
			name: "press key with modifiers array",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"key":       "a",
				"selector":  "#input",
				"modifiers": []interface{}{"Control", "Shift"},
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				loc := &mockLocator{
					pressFunc: func(key string, opts playwright.LocatorPressOptions) error {
						if key != "Control+Shift+a" {
							return errors.New("expected key Control+Shift+a, got " + key)
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					locatorFunc: func(selector string, opts ...playwright.PageLocatorOptions) playwright.Locator {
						return loc
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Pressed key 'Control+Shift+a' on element: #input",
		},
		{
			name: "press key with single modifier",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"key":       "c",
				"modifiers": []interface{}{"Meta"},
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				keyboard := &mockKeyboard{
					pressFunc: func(key string, opts ...playwright.KeyboardPressOptions) error {
						if key != "Meta+c" {
							return errors.New("expected key Meta+c, got " + key)
						}
						return nil
					},
				}
				page := &mockInteractionPage{
					keyboardFunc: func() playwright.Keyboard {
						return keyboard
					},
				}
				return createInteractionSessionWithMockPage(page), true
			},
			expectError:    false,
			expectedResult: "Pressed key 'Meta+c'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}

			handler := PressKeyHandler(mgr)
			req := mcp.CallToolRequest{}
			req.Params.Name = "press_key"
			req.Params.Arguments = tt.arguments

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			text := extractTextContent(result.Content)
			if tt.expectError {
				if !result.IsError {
					t.Errorf("Expected error result, got success: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedResult, text)
				}
			} else {
				if result.IsError {
					t.Errorf("Expected success result, got error: %s", text)
				}
				if !strings.Contains(text, tt.expectedResult) {
					t.Errorf("Expected result containing '%s', got '%s'", tt.expectedResult, text)
				}
			}
		})
	}
}

// TestHandleInteractionError tests the error classification logic.
func TestHandleInteractionError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		operation    string
		selector     string
		expectedCode string
	}{
		{
			name:         "timeout error",
			err:          errors.New("timeout exceeded waiting for element"),
			operation:    "click",
			selector:     "#button",
			expectedCode: "[-32003]",
		},
		{
			name:         "element not found",
			err:          errors.New("no element matches selector"),
			operation:    "click",
			selector:     "#button",
			expectedCode: "[-32002]",
		},
		{
			name:         "strict mode violation",
			err:          errors.New("strict mode violation: selector resolved to 3 elements"),
			operation:    "click",
			selector:     "button",
			expectedCode: "[-32002]",
		},
		{
			name:         "element not visible",
			err:          errors.New("element is not visible"),
			operation:    "hover",
			selector:     "#hidden",
			expectedCode: "[-32002]",
		},
		{
			name:         "element not attached",
			err:          errors.New("element is not attached to the DOM"),
			operation:    "fill",
			selector:     "#detached",
			expectedCode: "[-32002]",
		},
		{
			name:         "generic error uses interaction failed code",
			err:          errors.New("some other error"),
			operation:    "type",
			selector:     "#input",
			expectedCode: "[-32005]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleInteractionError(tt.err, tt.operation, tt.selector)
			text := extractTextContent(result.Content)
			if !strings.Contains(text, tt.expectedCode) {
				t.Errorf("Expected error code %s in result, got: %s", tt.expectedCode, text)
			}
		})
	}
}

// TestBuildClickOptions tests the click options builder.
func TestBuildClickOptions(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		checkOpts func(opts playwright.LocatorClickOptions) error
	}{
		{
			name: "empty args",
			args: map[string]any{},
			checkOpts: func(opts playwright.LocatorClickOptions) error {
				if opts.Button != nil {
					return errors.New("expected nil button")
				}
				if opts.ClickCount != nil {
					return errors.New("expected nil clickCount")
				}
				if opts.Timeout != nil {
					return errors.New("expected nil timeout")
				}
				return nil
			},
		},
		{
			name: "with all options",
			args: map[string]any{
				"button":     "right",
				"clickCount": float64(2),
				"timeout":    float64(5000),
			},
			checkOpts: func(opts playwright.LocatorClickOptions) error {
				if opts.Button == nil || *opts.Button != *playwright.MouseButtonRight {
					return errors.New("expected right button")
				}
				if opts.ClickCount == nil || *opts.ClickCount != 2 {
					return errors.New("expected clickCount 2")
				}
				if opts.Timeout == nil || *opts.Timeout != 5000 {
					return errors.New("expected timeout 5000")
				}
				return nil
			},
		},
		{
			name: "with middle button",
			args: map[string]any{
				"button": "middle",
			},
			checkOpts: func(opts playwright.LocatorClickOptions) error {
				if opts.Button == nil || *opts.Button != *playwright.MouseButtonMiddle {
					return errors.New("expected middle button")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildClickOptions(tt.args)
			if err := tt.checkOpts(opts); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestBuildTypeOptions tests the type options builder.
func TestBuildTypeOptions(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		checkOpts func(opts playwright.LocatorPressSequentiallyOptions) error
	}{
		{
			name: "empty args",
			args: map[string]any{},
			checkOpts: func(opts playwright.LocatorPressSequentiallyOptions) error {
				if opts.Delay != nil {
					return errors.New("expected nil delay")
				}
				if opts.Timeout != nil {
					return errors.New("expected nil timeout")
				}
				return nil
			},
		},
		{
			name: "with delay and timeout",
			args: map[string]any{
				"delay":   float64(100),
				"timeout": float64(3000),
			},
			checkOpts: func(opts playwright.LocatorPressSequentiallyOptions) error {
				if opts.Delay == nil || *opts.Delay != 100 {
					return errors.New("expected delay 100")
				}
				if opts.Timeout == nil || *opts.Timeout != 3000 {
					return errors.New("expected timeout 3000")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildTypeOptions(tt.args)
			if err := tt.checkOpts(opts); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestBuildKeyWithModifiers tests the key with modifiers builder.
func TestBuildKeyWithModifiers(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		args     map[string]any
		expected string
	}{
		{
			name:     "no modifiers",
			key:      "Enter",
			args:     map[string]any{},
			expected: "Enter",
		},
		{
			name:     "nil modifiers",
			key:      "Tab",
			args:     map[string]any{"modifiers": nil},
			expected: "Tab",
		},
		{
			name:     "empty modifiers array",
			key:      "a",
			args:     map[string]any{"modifiers": []interface{}{}},
			expected: "a",
		},
		{
			name:     "single modifier",
			key:      "c",
			args:     map[string]any{"modifiers": []interface{}{"Control"}},
			expected: "Control+c",
		},
		{
			name:     "multiple modifiers",
			key:      "a",
			args:     map[string]any{"modifiers": []interface{}{"Control", "Shift"}},
			expected: "Control+Shift+a",
		},
		{
			name:     "all modifiers",
			key:      "x",
			args:     map[string]any{"modifiers": []interface{}{"Control", "Shift", "Alt", "Meta"}},
			expected: "Control+Shift+Alt+Meta+x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildKeyWithModifiers(tt.key, tt.args)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
