package errors

import (
	"errors"
	"testing"
)

func TestErrorCodeConstants(t *testing.T) {
	t.Run("standard JSON-RPC error codes have correct values", func(t *testing.T) {
		tests := []struct {
			name     string
			code     int
			expected int
		}{
			{"CodeParseError", CodeParseError, -32700},
			{"CodeInvalidRequest", CodeInvalidRequest, -32600},
			{"CodeMethodNotFound", CodeMethodNotFound, -32601},
			{"CodeInvalidParams", CodeInvalidParams, -32602},
			{"CodeInternalError", CodeInternalError, -32603},
		}

		for _, tt := range tests {
			if tt.code != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.expected)
			}
		}
	})

	t.Run("custom Playwright MCP error codes have correct values", func(t *testing.T) {
		tests := []struct {
			name     string
			code     int
			expected int
		}{
			{"CodeSessionNotFound", CodeSessionNotFound, -32001},
			{"CodeElementNotFound", CodeElementNotFound, -32002},
			{"CodeTimeout", CodeTimeout, -32003},
			{"CodeNavigationFailed", CodeNavigationFailed, -32004},
		}

		for _, tt := range tests {
			if tt.code != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.expected)
			}
		}
	})
}

func TestSessionNotFoundError(t *testing.T) {
	t.Run("implements error interface", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		var _ error = err
		if err.Error() == "" {
			t.Error("Error() should return non-empty string")
		}
	})

	t.Run("implements MCPError interface", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		var mcpErr MCPError = err
		if mcpErr.ErrorCode() != CodeSessionNotFound {
			t.Errorf("ErrorCode() = %d, want %d", mcpErr.ErrorCode(), CodeSessionNotFound)
		}
		data := mcpErr.ErrorData()
		if data == nil {
			t.Error("ErrorData() should not return nil")
		}
	})

	t.Run("ErrorCode returns correct code", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		if err.ErrorCode() != CodeSessionNotFound {
			t.Errorf("ErrorCode() = %d, want %d", err.ErrorCode(), CodeSessionNotFound)
		}
	})

	t.Run("Error message includes session ID", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-abc123")
		msg := err.Error()
		if msg != "session not found: sess-abc123" {
			t.Errorf("Error() = %q, want %q", msg, "session not found: sess-abc123")
		}
	})

	t.Run("ErrorData includes session ID", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-xyz")
		data := err.ErrorData()
		if data["sessionId"] != "sess-xyz" {
			t.Errorf("ErrorData()[sessionId] = %v, want %q", data["sessionId"], "sess-xyz")
		}
		if data["suggestion"] == nil {
			t.Error("ErrorData() should include suggestion")
		}
	})

	t.Run("empty session ID produces valid error message", func(t *testing.T) {
		err := NewSessionNotFoundError("")
		msg := err.Error()
		if msg != "session not found" {
			t.Errorf("Error() = %q, want %q", msg, "session not found")
		}
		data := err.ErrorData()
		if _, exists := data["sessionId"]; exists {
			t.Error("ErrorData() should not include sessionId when empty")
		}
	})

	t.Run("Unwrap returns wrapped error", func(t *testing.T) {
		underlying := errors.New("underlying error")
		err := WrapSessionNotFoundError("sess-123", underlying)
		if err.Unwrap() != underlying {
			t.Error("Unwrap() should return the wrapped error")
		}
	})

	t.Run("Unwrap returns nil when no wrapped error", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		if err.Unwrap() != nil {
			t.Error("Unwrap() should return nil when no wrapped error")
		}
	})

	t.Run("Error message includes wrapped error", func(t *testing.T) {
		underlying := errors.New("database connection lost")
		err := WrapSessionNotFoundError("sess-123", underlying)
		msg := err.Error()
		expected := "session not found: sess-123: database connection lost"
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})

	t.Run("errors.Is works with wrapped errors", func(t *testing.T) {
		underlying := errors.New("underlying error")
		err := WrapSessionNotFoundError("sess-123", underlying)
		if !errors.Is(err, underlying) {
			t.Error("errors.Is should find the underlying error")
		}
	})
}

func TestElementNotFoundError(t *testing.T) {
	t.Run("implements error interface", func(t *testing.T) {
		err := NewElementNotFoundError("#button", 5000)
		var _ error = err
		if err.Error() == "" {
			t.Error("Error() should return non-empty string")
		}
	})

	t.Run("implements MCPError interface", func(t *testing.T) {
		err := NewElementNotFoundError("#button", 5000)
		var mcpErr MCPError = err
		if mcpErr.ErrorCode() != CodeElementNotFound {
			t.Errorf("ErrorCode() = %d, want %d", mcpErr.ErrorCode(), CodeElementNotFound)
		}
		data := mcpErr.ErrorData()
		if data == nil {
			t.Error("ErrorData() should not return nil")
		}
	})

	t.Run("ErrorCode returns correct code", func(t *testing.T) {
		err := NewElementNotFoundError("#button", 5000)
		if err.ErrorCode() != CodeElementNotFound {
			t.Errorf("ErrorCode() = %d, want %d", err.ErrorCode(), CodeElementNotFound)
		}
	})

	t.Run("Error message includes selector and timeout", func(t *testing.T) {
		err := NewElementNotFoundError("#submit-btn", 3000)
		msg := err.Error()
		expected := `element not found: selector "#submit-btn" after 3000ms`
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})

	t.Run("ErrorData includes selector and timeout", func(t *testing.T) {
		err := NewElementNotFoundError(".my-class", 2500)
		data := err.ErrorData()
		if data["selector"] != ".my-class" {
			t.Errorf("ErrorData()[selector] = %v, want %q", data["selector"], ".my-class")
		}
		if data["timeout"] != 2500 {
			t.Errorf("ErrorData()[timeout] = %v, want %d", data["timeout"], 2500)
		}
		if data["suggestion"] == nil {
			t.Error("ErrorData() should include suggestion")
		}
	})

	t.Run("empty selector produces valid error message", func(t *testing.T) {
		err := NewElementNotFoundError("", 0)
		msg := err.Error()
		if msg != "element not found" {
			t.Errorf("Error() = %q, want %q", msg, "element not found")
		}
	})

	t.Run("zero timeout value omits timeout from message", func(t *testing.T) {
		err := NewElementNotFoundError("#btn", 0)
		msg := err.Error()
		expected := `element not found: selector "#btn"`
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
		data := err.ErrorData()
		if _, exists := data["timeout"]; exists {
			t.Error("ErrorData() should not include timeout when zero")
		}
	})

	t.Run("Unwrap returns wrapped error", func(t *testing.T) {
		underlying := errors.New("timeout while waiting")
		err := WrapElementNotFoundError("#btn", 5000, underlying)
		if err.Unwrap() != underlying {
			t.Error("Unwrap() should return the wrapped error")
		}
	})

	t.Run("Unwrap returns nil when no wrapped error", func(t *testing.T) {
		err := NewElementNotFoundError("#btn", 5000)
		if err.Unwrap() != nil {
			t.Error("Unwrap() should return nil when no wrapped error")
		}
	})

	t.Run("Error message includes wrapped error", func(t *testing.T) {
		underlying := errors.New("page was navigated away")
		err := WrapElementNotFoundError("#btn", 5000, underlying)
		msg := err.Error()
		expected := `element not found: selector "#btn" after 5000ms: page was navigated away`
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})
}

func TestTimeoutError(t *testing.T) {
	t.Run("implements error interface", func(t *testing.T) {
		err := NewTimeoutError("navigation", 30000)
		var _ error = err
		if err.Error() == "" {
			t.Error("Error() should return non-empty string")
		}
	})

	t.Run("implements MCPError interface", func(t *testing.T) {
		err := NewTimeoutError("navigation", 30000)
		var mcpErr MCPError = err
		if mcpErr.ErrorCode() != CodeTimeout {
			t.Errorf("ErrorCode() = %d, want %d", mcpErr.ErrorCode(), CodeTimeout)
		}
		data := mcpErr.ErrorData()
		if data == nil {
			t.Error("ErrorData() should not return nil")
		}
	})

	t.Run("ErrorCode returns correct code", func(t *testing.T) {
		err := NewTimeoutError("navigation", 30000)
		if err.ErrorCode() != CodeTimeout {
			t.Errorf("ErrorCode() = %d, want %d", err.ErrorCode(), CodeTimeout)
		}
	})

	t.Run("Error message includes operation and timeout", func(t *testing.T) {
		err := NewTimeoutError("page load", 60000)
		msg := err.Error()
		expected := "page load timed out after 60000ms"
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})

	t.Run("ErrorData includes operation and timeout", func(t *testing.T) {
		err := NewTimeoutError("click", 5000)
		data := err.ErrorData()
		if data["operation"] != "click" {
			t.Errorf("ErrorData()[operation] = %v, want %q", data["operation"], "click")
		}
		if data["timeout"] != 5000 {
			t.Errorf("ErrorData()[timeout] = %v, want %d", data["timeout"], 5000)
		}
		if data["suggestion"] == nil {
			t.Error("ErrorData() should include suggestion")
		}
	})

	t.Run("empty operation produces valid error message", func(t *testing.T) {
		err := NewTimeoutError("", 5000)
		msg := err.Error()
		expected := "operation timed out after 5000ms"
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})

	t.Run("zero timeout value produces valid error message", func(t *testing.T) {
		err := NewTimeoutError("navigation", 0)
		msg := err.Error()
		expected := "navigation timed out"
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
		data := err.ErrorData()
		if _, exists := data["timeout"]; exists {
			t.Error("ErrorData() should not include timeout when zero")
		}
	})

	t.Run("both empty operation and zero timeout", func(t *testing.T) {
		err := NewTimeoutError("", 0)
		msg := err.Error()
		if msg != "operation timed out" {
			t.Errorf("Error() = %q, want %q", msg, "operation timed out")
		}
	})

	t.Run("Unwrap returns wrapped error", func(t *testing.T) {
		underlying := errors.New("context deadline exceeded")
		err := WrapTimeoutError("navigation", 30000, underlying)
		if err.Unwrap() != underlying {
			t.Error("Unwrap() should return the wrapped error")
		}
	})

	t.Run("Unwrap returns nil when no wrapped error", func(t *testing.T) {
		err := NewTimeoutError("navigation", 30000)
		if err.Unwrap() != nil {
			t.Error("Unwrap() should return nil when no wrapped error")
		}
	})

	t.Run("Error message includes wrapped error", func(t *testing.T) {
		underlying := errors.New("context canceled")
		err := WrapTimeoutError("script execution", 10000, underlying)
		msg := err.Error()
		expected := "script execution timed out after 10000ms: context canceled"
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})
}

func TestNavigationError(t *testing.T) {
	t.Run("implements error interface", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "connection refused")
		var _ error = err
		if err.Error() == "" {
			t.Error("Error() should return non-empty string")
		}
	})

	t.Run("implements MCPError interface", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "connection refused")
		var mcpErr MCPError = err
		if mcpErr.ErrorCode() != CodeNavigationFailed {
			t.Errorf("ErrorCode() = %d, want %d", mcpErr.ErrorCode(), CodeNavigationFailed)
		}
		data := mcpErr.ErrorData()
		if data == nil {
			t.Error("ErrorData() should not return nil")
		}
	})

	t.Run("ErrorCode returns correct code", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "timeout")
		if err.ErrorCode() != CodeNavigationFailed {
			t.Errorf("ErrorCode() = %d, want %d", err.ErrorCode(), CodeNavigationFailed)
		}
	})

	t.Run("Error message includes URL and reason", func(t *testing.T) {
		err := NewNavigationError("https://test.com", "SSL certificate error")
		msg := err.Error()
		expected := `navigation to "https://test.com" failed: SSL certificate error`
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})

	t.Run("ErrorData includes URL and reason", func(t *testing.T) {
		err := NewNavigationError("https://api.example.com", "403 Forbidden")
		data := err.ErrorData()
		if data["url"] != "https://api.example.com" {
			t.Errorf("ErrorData()[url] = %v, want %q", data["url"], "https://api.example.com")
		}
		if data["reason"] != "403 Forbidden" {
			t.Errorf("ErrorData()[reason] = %v, want %q", data["reason"], "403 Forbidden")
		}
		if data["suggestion"] == nil {
			t.Error("ErrorData() should include suggestion")
		}
	})

	t.Run("empty URL produces valid error message", func(t *testing.T) {
		err := NewNavigationError("", "no URL provided")
		msg := err.Error()
		expected := "navigation failed: no URL provided"
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
		data := err.ErrorData()
		if _, exists := data["url"]; exists {
			t.Error("ErrorData() should not include url when empty")
		}
	})

	t.Run("empty reason produces valid error message", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "")
		msg := err.Error()
		expected := `navigation to "https://example.com" failed`
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
		data := err.ErrorData()
		if _, exists := data["reason"]; exists {
			t.Error("ErrorData() should not include reason when empty")
		}
	})

	t.Run("both empty URL and reason", func(t *testing.T) {
		err := NewNavigationError("", "")
		msg := err.Error()
		if msg != "navigation failed" {
			t.Errorf("Error() = %q, want %q", msg, "navigation failed")
		}
	})

	t.Run("Unwrap returns wrapped error", func(t *testing.T) {
		underlying := errors.New("net::ERR_CONNECTION_REFUSED")
		err := WrapNavigationError("http://localhost:9999", "connection refused", underlying)
		if err.Unwrap() != underlying {
			t.Error("Unwrap() should return the wrapped error")
		}
	})

	t.Run("Unwrap returns nil when no wrapped error", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "timeout")
		if err.Unwrap() != nil {
			t.Error("Unwrap() should return nil when no wrapped error")
		}
	})

	t.Run("Error message includes wrapped error", func(t *testing.T) {
		underlying := errors.New("TLS handshake failed")
		err := WrapNavigationError("https://expired.cert.com", "SSL error", underlying)
		msg := err.Error()
		expected := `navigation to "https://expired.cert.com" failed: SSL error: TLS handshake failed`
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})

	t.Run("errors.Is works with wrapped errors", func(t *testing.T) {
		underlying := errors.New("connection timeout")
		err := WrapNavigationError("https://slow.com", "timeout", underlying)
		if !errors.Is(err, underlying) {
			t.Error("errors.Is should find the underlying error")
		}
	})
}

func TestMCPErrorInterfaceCompliance(t *testing.T) {
	t.Run("all error types implement MCPError at compile time", func(t *testing.T) {
		// These assignments will fail to compile if the types don't implement MCPError
		var _ MCPError = (*SessionNotFoundError)(nil)
		var _ MCPError = (*ElementNotFoundError)(nil)
		var _ MCPError = (*TimeoutError)(nil)
		var _ MCPError = (*NavigationError)(nil)
	})

	t.Run("all error types return valid ErrorData", func(t *testing.T) {
		mcpErrors := []MCPError{
			NewSessionNotFoundError("sess-123"),
			NewElementNotFoundError("#btn", 5000),
			NewTimeoutError("navigation", 30000),
			NewNavigationError("https://example.com", "timeout"),
		}

		for _, err := range mcpErrors {
			data := err.ErrorData()
			if data == nil {
				t.Errorf("%T.ErrorData() should not return nil", err)
			}
			if data["suggestion"] == nil {
				t.Errorf("%T.ErrorData() should include suggestion", err)
			}
		}
	})

	t.Run("all error types return distinct error codes", func(t *testing.T) {
		mcpErrors := []MCPError{
			NewSessionNotFoundError("sess-123"),
			NewElementNotFoundError("#btn", 5000),
			NewTimeoutError("navigation", 30000),
			NewNavigationError("https://example.com", "timeout"),
		}

		codes := make(map[int]string)
		for _, err := range mcpErrors {
			code := err.ErrorCode()
			if existing, exists := codes[code]; exists {
				t.Errorf("error code %d used by both %s and %T", code, existing, err)
			}
			codes[code] = err.Error()
		}
	})
}

func TestErrorWrappingCompatibility(t *testing.T) {
	t.Run("errors.Unwrap works with all error types", func(t *testing.T) {
		underlying := errors.New("root cause")

		wrappedErrors := []error{
			WrapSessionNotFoundError("sess-123", underlying),
			WrapElementNotFoundError("#btn", 5000, underlying),
			WrapTimeoutError("navigation", 30000, underlying),
			WrapNavigationError("https://example.com", "error", underlying),
		}

		for _, err := range wrappedErrors {
			unwrapped := errors.Unwrap(err)
			if unwrapped != underlying {
				t.Errorf("errors.Unwrap(%T) = %v, want %v", err, unwrapped, underlying)
			}
		}
	})

	t.Run("error chain traversal works", func(t *testing.T) {
		root := errors.New("database error")
		middle := WrapSessionNotFoundError("sess-123", root)
		// Note: standard errors don't support wrapping in this way,
		// but we verify our errors work with errors.Is
		if !errors.Is(middle, root) {
			t.Error("errors.Is should find root error through chain")
		}
	})
}
