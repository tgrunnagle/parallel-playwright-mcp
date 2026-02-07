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

// mockPage is a mock implementation of playwright.Page for testing.
type mockPage struct {
	playwright.Page
	gotoFunc      func(url string, opts playwright.PageGotoOptions) (playwright.Response, error)
	goBackFunc    func(opts playwright.PageGoBackOptions) (playwright.Response, error)
	goForwardFunc func(opts playwright.PageGoForwardOptions) (playwright.Response, error)
	reloadFunc    func(opts playwright.PageReloadOptions) (playwright.Response, error)
	urlFunc       func() string
}

func (m *mockPage) Goto(url string, opts ...playwright.PageGotoOptions) (playwright.Response, error) {
	if m.gotoFunc != nil {
		opt := playwright.PageGotoOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.gotoFunc(url, opt)
	}
	return nil, nil
}

func (m *mockPage) GoBack(opts ...playwright.PageGoBackOptions) (playwright.Response, error) {
	if m.goBackFunc != nil {
		opt := playwright.PageGoBackOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.goBackFunc(opt)
	}
	return nil, nil
}

func (m *mockPage) GoForward(opts ...playwright.PageGoForwardOptions) (playwright.Response, error) {
	if m.goForwardFunc != nil {
		opt := playwright.PageGoForwardOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.goForwardFunc(opt)
	}
	return nil, nil
}

func (m *mockPage) Reload(opts ...playwright.PageReloadOptions) (playwright.Response, error) {
	if m.reloadFunc != nil {
		opt := playwright.PageReloadOptions{}
		if len(opts) > 0 {
			opt = opts[0]
		}
		return m.reloadFunc(opt)
	}
	return nil, nil
}

func (m *mockPage) URL() string {
	if m.urlFunc != nil {
		return m.urlFunc()
	}
	return "https://example.com"
}

// mockResponse is a mock implementation of playwright.Response for testing.
type mockResponse struct {
	playwright.Response
	urlFunc func() string
}

func (m *mockResponse) URL() string {
	if m.urlFunc != nil {
		return m.urlFunc()
	}
	return "https://example.com"
}

// TestNavigateTool tests the navigate tool definition.
func TestNavigateTool(t *testing.T) {
	tool := NavigateTool()

	if tool.Name != "navigate" {
		t.Errorf("Expected tool name 'navigate', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Verify input schema has expected properties
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
	if _, ok := props["url"]; !ok {
		t.Error("Expected 'url' property in input schema")
	}
	// Check optional parameters
	if _, ok := props["waitUntil"]; !ok {
		t.Error("Expected 'waitUntil' property in input schema")
	}
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify sessionId and url are in required
	required := schema.Required
	foundSessionId := false
	foundUrl := false
	for _, r := range required {
		if r == "sessionId" {
			foundSessionId = true
		}
		if r == "url" {
			foundUrl = true
		}
	}
	if !foundSessionId {
		t.Error("Expected 'sessionId' to be in required properties")
	}
	if !foundUrl {
		t.Error("Expected 'url' to be in required properties")
	}
}

// TestGoBackTool tests the go_back tool definition.
func TestGoBackTool(t *testing.T) {
	tool := GoBackTool()

	if tool.Name != "go_back" {
		t.Errorf("Expected tool name 'go_back', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	schema := tool.InputSchema
	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["waitUntil"]; !ok {
		t.Error("Expected 'waitUntil' property in input schema")
	}
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify sessionId is in required
	required := schema.Required
	found := false
	for _, r := range required {
		if r == "sessionId" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'sessionId' to be in required properties")
	}
}

// TestGoForwardTool tests the go_forward tool definition.
func TestGoForwardTool(t *testing.T) {
	tool := GoForwardTool()

	if tool.Name != "go_forward" {
		t.Errorf("Expected tool name 'go_forward', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	schema := tool.InputSchema
	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["waitUntil"]; !ok {
		t.Error("Expected 'waitUntil' property in input schema")
	}
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify sessionId is in required
	required := schema.Required
	found := false
	for _, r := range required {
		if r == "sessionId" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'sessionId' to be in required properties")
	}
}

// TestReloadTool tests the reload tool definition.
func TestReloadTool(t *testing.T) {
	tool := ReloadTool()

	if tool.Name != "reload" {
		t.Errorf("Expected tool name 'reload', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	schema := tool.InputSchema
	props := schema.Properties
	if props == nil {
		t.Fatal("Input schema properties should not be nil")
	}

	if _, ok := props["sessionId"]; !ok {
		t.Error("Expected 'sessionId' property in input schema")
	}
	if _, ok := props["waitUntil"]; !ok {
		t.Error("Expected 'waitUntil' property in input schema")
	}
	if _, ok := props["timeout"]; !ok {
		t.Error("Expected 'timeout' property in input schema")
	}

	// Verify sessionId is in required
	required := schema.Required
	found := false
	for _, r := range required {
		if r == "sessionId" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'sessionId' to be in required properties")
	}
}

// TestNavigateHandler tests the navigate handler.
func TestNavigateHandler(t *testing.T) {
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
				"url": "https://example.com",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "empty sessionId",
			arguments: map[string]any{
				"sessionId": "",
				"url":       "https://example.com",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "missing url",
			arguments: map[string]any{
				"sessionId": "sess-123",
			},
			expectError:    true,
			expectedResult: "url is required",
		},
		{
			name: "empty url",
			arguments: map[string]any{
				"sessionId": "sess-123",
				"url":       "",
			},
			expectError:    true,
			expectedResult: "url is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
				"url":       "https://example.com",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}
			handler := NavigateHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "navigate",
					Arguments: tt.arguments,
				},
			}

			ctx := context.Background()
			result, err := handler(ctx, req)

			if err != nil {
				t.Fatalf("Handler returned unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			text := extractTextContent(result.Content)

			if tt.expectError {
				if !result.IsError {
					t.Error("Expected error result")
				}
			}

			if tt.expectedResult != "" && !strings.Contains(text, tt.expectedResult) {
				t.Errorf("Expected result to contain '%s', got '%s'", tt.expectedResult, text)
			}
		})
	}
}

// TestGoBackHandler tests the go_back handler.
func TestGoBackHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
	}{
		{
			name:           "missing sessionId",
			arguments:      map[string]any{},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "empty sessionId",
			arguments: map[string]any{
				"sessionId": "",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}
			handler := GoBackHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "go_back",
					Arguments: tt.arguments,
				},
			}

			ctx := context.Background()
			result, err := handler(ctx, req)

			if err != nil {
				t.Fatalf("Handler returned unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			text := extractTextContent(result.Content)

			if tt.expectError {
				if !result.IsError {
					t.Error("Expected error result")
				}
			}

			if tt.expectedResult != "" && !strings.Contains(text, tt.expectedResult) {
				t.Errorf("Expected result to contain '%s', got '%s'", tt.expectedResult, text)
			}
		})
	}
}

// TestGoForwardHandler tests the go_forward handler.
func TestGoForwardHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
	}{
		{
			name:           "missing sessionId",
			arguments:      map[string]any{},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "empty sessionId",
			arguments: map[string]any{
				"sessionId": "",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}
			handler := GoForwardHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "go_forward",
					Arguments: tt.arguments,
				},
			}

			ctx := context.Background()
			result, err := handler(ctx, req)

			if err != nil {
				t.Fatalf("Handler returned unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			text := extractTextContent(result.Content)

			if tt.expectError {
				if !result.IsError {
					t.Error("Expected error result")
				}
			}

			if tt.expectedResult != "" && !strings.Contains(text, tt.expectedResult) {
				t.Errorf("Expected result to contain '%s', got '%s'", tt.expectedResult, text)
			}
		})
	}
}

// TestReloadHandler tests the reload handler.
func TestReloadHandler(t *testing.T) {
	tests := []struct {
		name           string
		arguments      map[string]any
		getSessionFunc func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool)
		expectError    bool
		expectedResult string
	}{
		{
			name:           "missing sessionId",
			arguments:      map[string]any{},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "empty sessionId",
			arguments: map[string]any{
				"sessionId": "",
			},
			expectError:    true,
			expectedResult: "sessionId is required",
		},
		{
			name: "session not found",
			arguments: map[string]any{
				"sessionId": "sess-invalid",
			},
			getSessionFunc: func(mcpSessionID, browserSessionID string) (*session.BrowserSession, bool) {
				return nil, false
			},
			expectError:    true,
			expectedResult: "[-32001] Session not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &mockSessionManager{
				getSessionFunc: tt.getSessionFunc,
			}
			handler := ReloadHandler(mgr)

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "reload",
					Arguments: tt.arguments,
				},
			}

			ctx := context.Background()
			result, err := handler(ctx, req)

			if err != nil {
				t.Fatalf("Handler returned unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			text := extractTextContent(result.Content)

			if tt.expectError {
				if !result.IsError {
					t.Error("Expected error result")
				}
			}

			if tt.expectedResult != "" && !strings.Contains(text, tt.expectedResult) {
				t.Errorf("Expected result to contain '%s', got '%s'", tt.expectedResult, text)
			}
		})
	}
}

// TestBuildGotoOptions tests the buildGotoOptions helper function.
func TestBuildGotoOptions(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		expectWait   bool
		waitValue    string
		expectTime   bool
		timeoutValue float64
	}{
		{
			name:       "empty args",
			args:       map[string]any{},
			expectWait: false,
			expectTime: false,
		},
		{
			name: "waitUntil load",
			args: map[string]any{
				"waitUntil": "load",
			},
			expectWait: true,
			waitValue:  "load",
		},
		{
			name: "waitUntil domcontentloaded",
			args: map[string]any{
				"waitUntil": "domcontentloaded",
			},
			expectWait: true,
			waitValue:  "domcontentloaded",
		},
		{
			name: "waitUntil networkidle",
			args: map[string]any{
				"waitUntil": "networkidle",
			},
			expectWait: true,
			waitValue:  "networkidle",
		},
		{
			name: "timeout only",
			args: map[string]any{
				"timeout": float64(5000),
			},
			expectTime:   true,
			timeoutValue: 5000,
		},
		{
			name: "all options",
			args: map[string]any{
				"waitUntil": "networkidle",
				"timeout":   float64(10000),
			},
			expectWait:   true,
			waitValue:    "networkidle",
			expectTime:   true,
			timeoutValue: 10000,
		},
		{
			name: "empty waitUntil",
			args: map[string]any{
				"waitUntil": "",
			},
			expectWait: false,
		},
		{
			name: "zero timeout",
			args: map[string]any{
				"timeout": float64(0),
			},
			expectTime: false,
		},
		{
			name: "negative timeout",
			args: map[string]any{
				"timeout": float64(-100),
			},
			expectTime: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildGotoOptions(tt.args)

			if tt.expectWait {
				if opts.WaitUntil == nil {
					t.Error("Expected WaitUntil to be set")
				}
			} else {
				if opts.WaitUntil != nil {
					t.Error("Expected WaitUntil to be nil")
				}
			}

			if tt.expectTime {
				if opts.Timeout == nil {
					t.Error("Expected Timeout to be set")
				} else if *opts.Timeout != tt.timeoutValue {
					t.Errorf("Expected timeout %f, got %f", tt.timeoutValue, *opts.Timeout)
				}
			} else {
				if opts.Timeout != nil {
					t.Error("Expected Timeout to be nil")
				}
			}
		})
	}
}

// TestBuildGoBackOptions tests the buildGoBackOptions helper function.
func TestBuildGoBackOptions(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		expectWait   bool
		expectTime   bool
		timeoutValue float64
	}{
		{
			name:       "empty args",
			args:       map[string]any{},
			expectWait: false,
			expectTime: false,
		},
		{
			name: "waitUntil set",
			args: map[string]any{
				"waitUntil": "domcontentloaded",
			},
			expectWait: true,
		},
		{
			name: "timeout set",
			args: map[string]any{
				"timeout": float64(3000),
			},
			expectTime:   true,
			timeoutValue: 3000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildGoBackOptions(tt.args)

			if tt.expectWait {
				if opts.WaitUntil == nil {
					t.Error("Expected WaitUntil to be set")
				}
			} else {
				if opts.WaitUntil != nil {
					t.Error("Expected WaitUntil to be nil")
				}
			}

			if tt.expectTime {
				if opts.Timeout == nil {
					t.Error("Expected Timeout to be set")
				} else if *opts.Timeout != tt.timeoutValue {
					t.Errorf("Expected timeout %f, got %f", tt.timeoutValue, *opts.Timeout)
				}
			}
		})
	}
}

// TestBuildGoForwardOptions tests the buildGoForwardOptions helper function.
func TestBuildGoForwardOptions(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		expectWait   bool
		expectTime   bool
		timeoutValue float64
	}{
		{
			name:       "empty args",
			args:       map[string]any{},
			expectWait: false,
			expectTime: false,
		},
		{
			name: "waitUntil set",
			args: map[string]any{
				"waitUntil": "networkidle",
			},
			expectWait: true,
		},
		{
			name: "timeout set",
			args: map[string]any{
				"timeout": float64(7500),
			},
			expectTime:   true,
			timeoutValue: 7500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildGoForwardOptions(tt.args)

			if tt.expectWait {
				if opts.WaitUntil == nil {
					t.Error("Expected WaitUntil to be set")
				}
			} else {
				if opts.WaitUntil != nil {
					t.Error("Expected WaitUntil to be nil")
				}
			}

			if tt.expectTime {
				if opts.Timeout == nil {
					t.Error("Expected Timeout to be set")
				} else if *opts.Timeout != tt.timeoutValue {
					t.Errorf("Expected timeout %f, got %f", tt.timeoutValue, *opts.Timeout)
				}
			}
		})
	}
}

// TestBuildReloadOptions tests the buildReloadOptions helper function.
func TestBuildReloadOptions(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		expectWait   bool
		expectTime   bool
		timeoutValue float64
	}{
		{
			name:       "empty args",
			args:       map[string]any{},
			expectWait: false,
			expectTime: false,
		},
		{
			name: "waitUntil set",
			args: map[string]any{
				"waitUntil": "load",
			},
			expectWait: true,
		},
		{
			name: "timeout set",
			args: map[string]any{
				"timeout": float64(15000),
			},
			expectTime:   true,
			timeoutValue: 15000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildReloadOptions(tt.args)

			if tt.expectWait {
				if opts.WaitUntil == nil {
					t.Error("Expected WaitUntil to be set")
				}
			} else {
				if opts.WaitUntil != nil {
					t.Error("Expected WaitUntil to be nil")
				}
			}

			if tt.expectTime {
				if opts.Timeout == nil {
					t.Error("Expected Timeout to be set")
				} else if *opts.Timeout != tt.timeoutValue {
					t.Errorf("Expected timeout %f, got %f", tt.timeoutValue, *opts.Timeout)
				}
			}
		})
	}
}

// TestMapWaitUntil tests the mapWaitUntil helper function.
func TestMapWaitUntil(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected playwright.WaitUntilState
	}{
		{
			name:     "load",
			input:    "load",
			expected: *playwright.WaitUntilStateLoad,
		},
		{
			name:     "domcontentloaded",
			input:    "domcontentloaded",
			expected: *playwright.WaitUntilStateDomcontentloaded,
		},
		{
			name:     "networkidle",
			input:    "networkidle",
			expected: *playwright.WaitUntilStateNetworkidle,
		},
		{
			name:     "unknown defaults to load",
			input:    "unknown",
			expected: *playwright.WaitUntilStateLoad,
		},
		{
			name:     "empty defaults to load",
			input:    "",
			expected: *playwright.WaitUntilStateLoad,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapWaitUntil(tt.input)
			if result == nil {
				t.Fatal("Expected non-nil result")
			}
			if *result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, *result)
			}
		})
	}
}

// TestIsTimeoutError tests the isTimeoutError helper function.
func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "contains timeout lowercase",
			err:      errors.New("navigation timeout"),
			expected: true,
		},
		{
			name:     "contains Timeout uppercase",
			err:      errors.New("Timeout waiting for page"),
			expected: true,
		},
		{
			name:     "contains exceeded",
			err:      errors.New("time exceeded"),
			expected: true,
		},
		{
			name:     "contains EXCEEDED uppercase",
			err:      errors.New("TIMEOUT EXCEEDED"),
			expected: true,
		},
		{
			name:     "regular error",
			err:      errors.New("network error"),
			expected: false,
		},
		{
			name:     "connection error",
			err:      errors.New("connection refused"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeoutError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for error '%s'", tt.expected, result, tt.err.Error())
			}
		})
	}
}

// TestHandleNavigationError tests the handleNavigationError helper function.
func TestHandleNavigationError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		target      string
		expectCode  string
		expectInMsg string
	}{
		{
			name:        "timeout error",
			err:         errors.New("navigation timeout"),
			target:      "https://example.com",
			expectCode:  "[-32003]",
			expectInMsg: "Navigation timeout",
		},
		{
			name:        "navigation failure",
			err:         errors.New("net::ERR_NAME_NOT_RESOLVED"),
			target:      "https://invalid.example",
			expectCode:  "[-32004]",
			expectInMsg: "Navigation failed",
		},
		{
			name:        "connection error",
			err:         errors.New("connection refused"),
			target:      "back",
			expectCode:  "[-32004]",
			expectInMsg: "Navigation failed for back",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleNavigationError(tt.err, tt.target)

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			if !result.IsError {
				t.Error("Expected error result")
			}

			text := extractTextContent(result.Content)

			if !strings.Contains(text, tt.expectCode) {
				t.Errorf("Expected result to contain code '%s', got '%s'", tt.expectCode, text)
			}

			if !strings.Contains(text, tt.expectInMsg) {
				t.Errorf("Expected result to contain '%s', got '%s'", tt.expectInMsg, text)
			}

			if !strings.Contains(text, tt.target) {
				t.Errorf("Expected result to contain target '%s', got '%s'", tt.target, text)
			}
		})
	}
}

// TestNewNavigationSessionNotFoundError tests the newNavigationSessionNotFoundError helper.
func TestNewNavigationSessionNotFoundError(t *testing.T) {
	sessionID := "sess-test-123"
	result := newNavigationSessionNotFoundError(sessionID)

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if !result.IsError {
		t.Error("Expected error result")
	}

	text := extractTextContent(result.Content)

	if !strings.Contains(text, "[-32001]") {
		t.Errorf("Expected result to contain error code '[-32001]', got '%s'", text)
	}

	if !strings.Contains(text, sessionID) {
		t.Errorf("Expected result to contain session ID '%s', got '%s'", sessionID, text)
	}

	if !strings.Contains(text, "Session not found") {
		t.Errorf("Expected result to contain 'Session not found', got '%s'", text)
	}
}
