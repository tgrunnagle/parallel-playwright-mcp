package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestFormatError_NilError(t *testing.T) {
	t.Run("returns nil for nil error", func(t *testing.T) {
		result := FormatError(nil)
		if result != nil {
			t.Error("FormatError(nil) should return nil")
		}
	})
}

func TestFormatError_SessionNotFoundError(t *testing.T) {
	t.Run("formats with correct code", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		result := FormatError(err)

		if result.Code != CodeSessionNotFound {
			t.Errorf("Code = %d, want %d", result.Code, CodeSessionNotFound)
		}
	})

	t.Run("formats with correct message", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		result := FormatError(err)

		if result.Message != msgSessionNotFound {
			t.Errorf("Message = %q, want %q", result.Message, msgSessionNotFound)
		}
	})

	t.Run("includes sessionId in data", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-abc")
		result := FormatError(err)

		if result.Data == nil {
			t.Fatal("Data should not be nil")
		}
		if result.Data.SessionID != "sess-abc" {
			t.Errorf("Data.SessionID = %q, want %q", result.Data.SessionID, "sess-abc")
		}
	})

	t.Run("includes non-empty suggestion", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		result := FormatError(err)

		if result.Data == nil {
			t.Fatal("Data should not be nil")
		}
		if result.Data.Suggestion == "" {
			t.Error("Data.Suggestion should not be empty")
		}
	})

	t.Run("includes wrapped error in cause", func(t *testing.T) {
		underlying := errors.New("connection lost")
		err := WrapSessionNotFoundError("sess-123", underlying)
		result := FormatError(err)

		if result.Data == nil {
			t.Fatal("Data should not be nil")
		}
		if result.Data.Cause != "connection lost" {
			t.Errorf("Data.Cause = %q, want %q", result.Data.Cause, "connection lost")
		}
	})

	t.Run("omits cause when no wrapped error", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		result := FormatError(err)

		if result.Data.Cause != "" {
			t.Errorf("Data.Cause = %q, want empty string", result.Data.Cause)
		}
	})

	t.Run("handles empty sessionId", func(t *testing.T) {
		err := NewSessionNotFoundError("")
		result := FormatError(err)

		if result.Data.SessionID != "" {
			t.Errorf("Data.SessionID = %q, want empty string", result.Data.SessionID)
		}
		// Should still have valid code and message
		if result.Code != CodeSessionNotFound {
			t.Errorf("Code = %d, want %d", result.Code, CodeSessionNotFound)
		}
	})
}

func TestFormatError_ElementNotFoundError(t *testing.T) {
	t.Run("formats with correct code", func(t *testing.T) {
		err := NewElementNotFoundError("#button", 5000)
		result := FormatError(err)

		if result.Code != CodeElementNotFound {
			t.Errorf("Code = %d, want %d", result.Code, CodeElementNotFound)
		}
	})

	t.Run("formats with correct message", func(t *testing.T) {
		err := NewElementNotFoundError("#button", 5000)
		result := FormatError(err)

		if result.Message != msgElementNotFound {
			t.Errorf("Message = %q, want %q", result.Message, msgElementNotFound)
		}
	})

	t.Run("includes selector in data", func(t *testing.T) {
		err := NewElementNotFoundError(".my-class", 3000)
		result := FormatError(err)

		if result.Data == nil {
			t.Fatal("Data should not be nil")
		}
		if result.Data.Selector != ".my-class" {
			t.Errorf("Data.Selector = %q, want %q", result.Data.Selector, ".my-class")
		}
	})

	t.Run("includes timeout in data", func(t *testing.T) {
		err := NewElementNotFoundError("#btn", 2500)
		result := FormatError(err)

		if result.Data == nil {
			t.Fatal("Data should not be nil")
		}
		if result.Data.Timeout != 2500 {
			t.Errorf("Data.Timeout = %d, want %d", result.Data.Timeout, 2500)
		}
	})

	t.Run("includes non-empty suggestion", func(t *testing.T) {
		err := NewElementNotFoundError("#btn", 5000)
		result := FormatError(err)

		if result.Data.Suggestion == "" {
			t.Error("Data.Suggestion should not be empty")
		}
	})

	t.Run("includes wrapped error in cause", func(t *testing.T) {
		underlying := errors.New("page was navigated away")
		err := WrapElementNotFoundError("#btn", 5000, underlying)
		result := FormatError(err)

		if result.Data.Cause != "page was navigated away" {
			t.Errorf("Data.Cause = %q, want %q", result.Data.Cause, "page was navigated away")
		}
	})

	t.Run("handles empty selector", func(t *testing.T) {
		err := NewElementNotFoundError("", 5000)
		result := FormatError(err)

		if result.Data.Selector != "" {
			t.Errorf("Data.Selector = %q, want empty string", result.Data.Selector)
		}
	})

	t.Run("handles zero timeout", func(t *testing.T) {
		err := NewElementNotFoundError("#btn", 0)
		result := FormatError(err)

		if result.Data.Timeout != 0 {
			t.Errorf("Data.Timeout = %d, want 0", result.Data.Timeout)
		}
	})
}

func TestFormatError_TimeoutError(t *testing.T) {
	t.Run("formats with correct code", func(t *testing.T) {
		err := NewTimeoutError("navigation", 30000)
		result := FormatError(err)

		if result.Code != CodeTimeout {
			t.Errorf("Code = %d, want %d", result.Code, CodeTimeout)
		}
	})

	t.Run("formats with correct message", func(t *testing.T) {
		err := NewTimeoutError("navigation", 30000)
		result := FormatError(err)

		if result.Message != msgTimeout {
			t.Errorf("Message = %q, want %q", result.Message, msgTimeout)
		}
	})

	t.Run("includes operation in data", func(t *testing.T) {
		err := NewTimeoutError("click", 5000)
		result := FormatError(err)

		if result.Data.Operation != "click" {
			t.Errorf("Data.Operation = %q, want %q", result.Data.Operation, "click")
		}
	})

	t.Run("includes timeout in data", func(t *testing.T) {
		err := NewTimeoutError("page load", 60000)
		result := FormatError(err)

		if result.Data.Timeout != 60000 {
			t.Errorf("Data.Timeout = %d, want %d", result.Data.Timeout, 60000)
		}
	})

	t.Run("includes non-empty suggestion", func(t *testing.T) {
		err := NewTimeoutError("navigation", 30000)
		result := FormatError(err)

		if result.Data.Suggestion == "" {
			t.Error("Data.Suggestion should not be empty")
		}
	})

	t.Run("includes wrapped error in cause", func(t *testing.T) {
		underlying := errors.New("context deadline exceeded")
		err := WrapTimeoutError("navigation", 30000, underlying)
		result := FormatError(err)

		if result.Data.Cause != "context deadline exceeded" {
			t.Errorf("Data.Cause = %q, want %q", result.Data.Cause, "context deadline exceeded")
		}
	})

	t.Run("handles empty operation", func(t *testing.T) {
		err := NewTimeoutError("", 5000)
		result := FormatError(err)

		if result.Data.Operation != "" {
			t.Errorf("Data.Operation = %q, want empty string", result.Data.Operation)
		}
	})
}

func TestFormatError_NavigationError(t *testing.T) {
	t.Run("formats with correct code", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "connection refused")
		result := FormatError(err)

		if result.Code != CodeNavigationFailed {
			t.Errorf("Code = %d, want %d", result.Code, CodeNavigationFailed)
		}
	})

	t.Run("formats with correct message", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "timeout")
		result := FormatError(err)

		if result.Message != msgNavigationFailed {
			t.Errorf("Message = %q, want %q", result.Message, msgNavigationFailed)
		}
	})

	t.Run("includes URL in data", func(t *testing.T) {
		err := NewNavigationError("https://test.com", "error")
		result := FormatError(err)

		if result.Data.URL != "https://test.com" {
			t.Errorf("Data.URL = %q, want %q", result.Data.URL, "https://test.com")
		}
	})

	t.Run("includes reason in data", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "SSL certificate error")
		result := FormatError(err)

		if result.Data.Reason != "SSL certificate error" {
			t.Errorf("Data.Reason = %q, want %q", result.Data.Reason, "SSL certificate error")
		}
	})

	t.Run("includes non-empty suggestion", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "error")
		result := FormatError(err)

		if result.Data.Suggestion == "" {
			t.Error("Data.Suggestion should not be empty")
		}
	})

	t.Run("includes wrapped error in cause", func(t *testing.T) {
		underlying := errors.New("TLS handshake failed")
		err := WrapNavigationError("https://expired.cert.com", "SSL error", underlying)
		result := FormatError(err)

		if result.Data.Cause != "TLS handshake failed" {
			t.Errorf("Data.Cause = %q, want %q", result.Data.Cause, "TLS handshake failed")
		}
	})

	t.Run("handles empty URL", func(t *testing.T) {
		err := NewNavigationError("", "no URL")
		result := FormatError(err)

		if result.Data.URL != "" {
			t.Errorf("Data.URL = %q, want empty string", result.Data.URL)
		}
	})

	t.Run("handles empty reason", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "")
		result := FormatError(err)

		if result.Data.Reason != "" {
			t.Errorf("Data.Reason = %q, want empty string", result.Data.Reason)
		}
	})
}

func TestFormatError_GenericError(t *testing.T) {
	t.Run("formats as internal error", func(t *testing.T) {
		err := errors.New("unexpected error")
		result := FormatError(err)

		if result.Code != CodeInternalError {
			t.Errorf("Code = %d, want %d", result.Code, CodeInternalError)
		}
	})

	t.Run("uses internal error message", func(t *testing.T) {
		err := errors.New("something went wrong")
		result := FormatError(err)

		if result.Message != msgInternalError {
			t.Errorf("Message = %q, want %q", result.Message, msgInternalError)
		}
	})

	t.Run("includes original error in cause", func(t *testing.T) {
		err := errors.New("database connection failed")
		result := FormatError(err)

		if result.Data.Cause != "database connection failed" {
			t.Errorf("Data.Cause = %q, want %q", result.Data.Cause, "database connection failed")
		}
	})

	t.Run("includes suggestion", func(t *testing.T) {
		err := errors.New("unknown error")
		result := FormatError(err)

		if result.Data.Suggestion == "" {
			t.Error("Data.Suggestion should not be empty")
		}
	})
}

func TestFormatError_WrappedErrors(t *testing.T) {
	t.Run("handles wrapped SessionNotFoundError", func(t *testing.T) {
		inner := NewSessionNotFoundError("sess-123")
		wrapped := fmt.Errorf("handler context: %w", inner)
		result := FormatError(wrapped)

		if result.Code != CodeSessionNotFound {
			t.Errorf("Code = %d, want %d", result.Code, CodeSessionNotFound)
		}
		if result.Message != msgSessionNotFound {
			t.Errorf("Message = %q, want %q", result.Message, msgSessionNotFound)
		}
		if result.Data.SessionID != "sess-123" {
			t.Errorf("Data.SessionID = %q, want %q", result.Data.SessionID, "sess-123")
		}
	})

	t.Run("handles wrapped ElementNotFoundError", func(t *testing.T) {
		inner := NewElementNotFoundError("#button", 5000)
		wrapped := fmt.Errorf("click handler failed: %w", inner)
		result := FormatError(wrapped)

		if result.Code != CodeElementNotFound {
			t.Errorf("Code = %d, want %d", result.Code, CodeElementNotFound)
		}
		if result.Message != msgElementNotFound {
			t.Errorf("Message = %q, want %q", result.Message, msgElementNotFound)
		}
		if result.Data.Selector != "#button" {
			t.Errorf("Data.Selector = %q, want %q", result.Data.Selector, "#button")
		}
		if result.Data.Timeout != 5000 {
			t.Errorf("Data.Timeout = %d, want %d", result.Data.Timeout, 5000)
		}
	})

	t.Run("handles wrapped TimeoutError", func(t *testing.T) {
		inner := NewTimeoutError("navigation", 30000)
		wrapped := fmt.Errorf("page load failed: %w", inner)
		result := FormatError(wrapped)

		if result.Code != CodeTimeout {
			t.Errorf("Code = %d, want %d", result.Code, CodeTimeout)
		}
		if result.Message != msgTimeout {
			t.Errorf("Message = %q, want %q", result.Message, msgTimeout)
		}
		if result.Data.Operation != "navigation" {
			t.Errorf("Data.Operation = %q, want %q", result.Data.Operation, "navigation")
		}
		if result.Data.Timeout != 30000 {
			t.Errorf("Data.Timeout = %d, want %d", result.Data.Timeout, 30000)
		}
	})

	t.Run("handles wrapped NavigationError", func(t *testing.T) {
		inner := NewNavigationError("https://example.com", "connection refused")
		wrapped := fmt.Errorf("navigate tool failed: %w", inner)
		result := FormatError(wrapped)

		if result.Code != CodeNavigationFailed {
			t.Errorf("Code = %d, want %d", result.Code, CodeNavigationFailed)
		}
		if result.Message != msgNavigationFailed {
			t.Errorf("Message = %q, want %q", result.Message, msgNavigationFailed)
		}
		if result.Data.URL != "https://example.com" {
			t.Errorf("Data.URL = %q, want %q", result.Data.URL, "https://example.com")
		}
		if result.Data.Reason != "connection refused" {
			t.Errorf("Data.Reason = %q, want %q", result.Data.Reason, "connection refused")
		}
	})

	t.Run("handles double-wrapped errors", func(t *testing.T) {
		inner := NewSessionNotFoundError("sess-abc")
		wrapped1 := fmt.Errorf("handler failed: %w", inner)
		wrapped2 := fmt.Errorf("tool execution error: %w", wrapped1)
		result := FormatError(wrapped2)

		if result.Code != CodeSessionNotFound {
			t.Errorf("Code = %d, want %d", result.Code, CodeSessionNotFound)
		}
		if result.Data.SessionID != "sess-abc" {
			t.Errorf("Data.SessionID = %q, want %q", result.Data.SessionID, "sess-abc")
		}
	})

	t.Run("wrapped errors preserve suggestion", func(t *testing.T) {
		inner := NewSessionNotFoundError("sess-123")
		wrapped := fmt.Errorf("context: %w", inner)
		result := FormatError(wrapped)

		if result.Data.Suggestion == "" {
			t.Error("Data.Suggestion should not be empty for wrapped error")
		}
		if !strings.Contains(result.Data.Suggestion, "session_create") {
			t.Errorf("Suggestion %q should mention session_create", result.Data.Suggestion)
		}
	})

	t.Run("wrapped error with additional context in inner error", func(t *testing.T) {
		underlying := errors.New("database error")
		inner := WrapSessionNotFoundError("sess-123", underlying)
		wrapped := fmt.Errorf("handler context: %w", inner)
		result := FormatError(wrapped)

		if result.Code != CodeSessionNotFound {
			t.Errorf("Code = %d, want %d", result.Code, CodeSessionNotFound)
		}
		if result.Data.Cause != "database error" {
			t.Errorf("Data.Cause = %q, want %q", result.Data.Cause, "database error")
		}
	})
}

func TestFormatErrorWithCode(t *testing.T) {
	t.Run("formats parse error", func(t *testing.T) {
		result := FormatErrorWithCode(CodeParseError, "invalid JSON at position 10")

		if result.Code != CodeParseError {
			t.Errorf("Code = %d, want %d", result.Code, CodeParseError)
		}
		if result.Message != msgParseError {
			t.Errorf("Message = %q, want %q", result.Message, msgParseError)
		}
		if result.Data.Suggestion != SuggestParseError {
			t.Errorf("Suggestion = %q, want %q", result.Data.Suggestion, SuggestParseError)
		}
		if result.Data.Cause != "invalid JSON at position 10" {
			t.Errorf("Cause = %q, want %q", result.Data.Cause, "invalid JSON at position 10")
		}
	})

	t.Run("formats invalid request error", func(t *testing.T) {
		result := FormatErrorWithCode(CodeInvalidRequest, "missing jsonrpc field")

		if result.Code != CodeInvalidRequest {
			t.Errorf("Code = %d, want %d", result.Code, CodeInvalidRequest)
		}
		if result.Message != msgInvalidRequest {
			t.Errorf("Message = %q, want %q", result.Message, msgInvalidRequest)
		}
		if result.Data.Suggestion != SuggestInvalidRequest {
			t.Errorf("Suggestion = %q, want %q", result.Data.Suggestion, SuggestInvalidRequest)
		}
	})

	t.Run("formats method not found error", func(t *testing.T) {
		result := FormatErrorWithCode(CodeMethodNotFound, "unknown_tool")

		if result.Code != CodeMethodNotFound {
			t.Errorf("Code = %d, want %d", result.Code, CodeMethodNotFound)
		}
		if result.Message != msgMethodNotFound {
			t.Errorf("Message = %q, want %q", result.Message, msgMethodNotFound)
		}
		if result.Data.Suggestion != SuggestMethodNotFound {
			t.Errorf("Suggestion = %q, want %q", result.Data.Suggestion, SuggestMethodNotFound)
		}
	})

	t.Run("formats invalid params error", func(t *testing.T) {
		result := FormatErrorWithCode(CodeInvalidParams, "sessionId is required")

		if result.Code != CodeInvalidParams {
			t.Errorf("Code = %d, want %d", result.Code, CodeInvalidParams)
		}
		if result.Message != msgInvalidParams {
			t.Errorf("Message = %q, want %q", result.Message, msgInvalidParams)
		}
		if result.Data.Suggestion != SuggestInvalidParams {
			t.Errorf("Suggestion = %q, want %q", result.Data.Suggestion, SuggestInvalidParams)
		}
	})

	t.Run("formats internal error", func(t *testing.T) {
		result := FormatErrorWithCode(CodeInternalError, "null pointer exception")

		if result.Code != CodeInternalError {
			t.Errorf("Code = %d, want %d", result.Code, CodeInternalError)
		}
		if result.Message != msgInternalError {
			t.Errorf("Message = %q, want %q", result.Message, msgInternalError)
		}
	})

	t.Run("handles unknown code as internal error", func(t *testing.T) {
		result := FormatErrorWithCode(-99999, "unknown error")

		// Should use internal error suggestion as fallback
		if result.Data.Suggestion != SuggestInternalError {
			t.Errorf("Suggestion = %q, want %q", result.Data.Suggestion, SuggestInternalError)
		}
	})

	t.Run("handles empty message", func(t *testing.T) {
		result := FormatErrorWithCode(CodeInternalError, "")

		if result.Data.Cause != "" {
			t.Errorf("Cause = %q, want empty string", result.Data.Cause)
		}
	})
}

func TestFormatErrorForTool(t *testing.T) {
	t.Run("returns empty string for nil error", func(t *testing.T) {
		result := FormatErrorForTool(nil)
		if result != "" {
			t.Errorf("FormatErrorForTool(nil) = %q, want empty string", result)
		}
	})

	t.Run("includes error code in output", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "[-32001]") {
			t.Errorf("result %q should contain error code [-32001]", result)
		}
	})

	t.Run("includes message in output", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "Session not found") {
			t.Errorf("result %q should contain message", result)
		}
	})

	t.Run("includes sessionId context", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-xyz")
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "sessionId: sess-xyz") {
			t.Errorf("result %q should contain sessionId context", result)
		}
	})

	t.Run("includes selector context", func(t *testing.T) {
		err := NewElementNotFoundError("#missing-btn", 5000)
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "selector: #missing-btn") {
			t.Errorf("result %q should contain selector context", result)
		}
	})

	t.Run("includes URL context", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "timeout")
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "url: https://example.com") {
			t.Errorf("result %q should contain URL context", result)
		}
	})

	t.Run("includes operation context", func(t *testing.T) {
		err := NewTimeoutError("page load", 30000)
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "operation: page load") {
			t.Errorf("result %q should contain operation context", result)
		}
	})

	t.Run("includes timeout context", func(t *testing.T) {
		err := NewTimeoutError("navigation", 5000)
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "timeout: 5000ms") {
			t.Errorf("result %q should contain timeout context", result)
		}
	})

	t.Run("includes reason context", func(t *testing.T) {
		err := NewNavigationError("https://example.com", "SSL error")
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "reason: SSL error") {
			t.Errorf("result %q should contain reason context", result)
		}
	})

	t.Run("includes cause from wrapped error", func(t *testing.T) {
		underlying := errors.New("underlying cause")
		err := WrapSessionNotFoundError("sess-123", underlying)
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "underlying cause") {
			t.Errorf("result %q should contain underlying cause", result)
		}
	})

	t.Run("includes suggestion", func(t *testing.T) {
		err := NewSessionNotFoundError("sess-123")
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "session_create") {
			t.Errorf("result %q should contain suggestion mentioning session_create", result)
		}
	})

	t.Run("formats generic error correctly", func(t *testing.T) {
		err := errors.New("unexpected failure")
		result := FormatErrorForTool(err)

		if !strings.Contains(result, "[-32603]") {
			t.Errorf("result %q should contain internal error code", result)
		}
		if !strings.Contains(result, "unexpected failure") {
			t.Errorf("result %q should contain error message", result)
		}
	})
}

func TestErrorResponse_ToJSON(t *testing.T) {
	t.Run("serializes complete error response", func(t *testing.T) {
		resp := &ErrorResponse{
			Code:    CodeSessionNotFound,
			Message: msgSessionNotFound,
			Data: &ErrorData{
				SessionID:  "sess-123",
				Suggestion: "Create a new session",
			},
		}

		jsonBytes, err := resp.ToJSON()
		if err != nil {
			t.Fatalf("ToJSON() returned error: %v", err)
		}

		// Verify it's valid JSON
		var parsed map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}

		// Verify fields
		if parsed["code"].(float64) != float64(CodeSessionNotFound) {
			t.Errorf("code = %v, want %d", parsed["code"], CodeSessionNotFound)
		}
		if parsed["message"] != msgSessionNotFound {
			t.Errorf("message = %v, want %q", parsed["message"], msgSessionNotFound)
		}
	})

	t.Run("omits nil data field", func(t *testing.T) {
		resp := &ErrorResponse{
			Code:    CodeInternalError,
			Message: "Error",
			Data:    nil,
		}

		jsonBytes, err := resp.ToJSON()
		if err != nil {
			t.Fatalf("ToJSON() returned error: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}

		if _, exists := parsed["data"]; exists {
			t.Error("data field should be omitted when nil")
		}
	})

	t.Run("omits empty string fields in data", func(t *testing.T) {
		resp := &ErrorResponse{
			Code:    CodeSessionNotFound,
			Message: msgSessionNotFound,
			Data: &ErrorData{
				SessionID:  "sess-123",
				Suggestion: "Create a new session",
				// Other fields left empty
			},
		}

		jsonBytes, err := resp.ToJSON()
		if err != nil {
			t.Fatalf("ToJSON() returned error: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}

		data := parsed["data"].(map[string]any)
		if _, exists := data["selector"]; exists {
			t.Error("empty selector field should be omitted")
		}
		if _, exists := data["url"]; exists {
			t.Error("empty url field should be omitted")
		}
	})
}

func TestAllErrorTypesHaveActionableSuggestions(t *testing.T) {
	t.Run("all custom error types include actionable suggestions", func(t *testing.T) {
		testCases := []struct {
			name   string
			err    error
			action string // Expected action word in suggestion
		}{
			{
				name:   "SessionNotFoundError",
				err:    NewSessionNotFoundError("sess-123"),
				action: "session_create",
			},
			{
				name:   "ElementNotFoundError",
				err:    NewElementNotFoundError("#btn", 5000),
				action: "selector",
			},
			{
				name:   "TimeoutError",
				err:    NewTimeoutError("navigation", 30000),
				action: "timeout",
			},
			{
				name:   "NavigationError",
				err:    NewNavigationError("https://example.com", "error"),
				action: "URL",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := FormatError(tc.err)
				if result.Data == nil {
					t.Fatal("Data should not be nil")
				}
				if result.Data.Suggestion == "" {
					t.Error("Suggestion should not be empty")
				}
				if !strings.Contains(result.Data.Suggestion, tc.action) {
					t.Errorf("Suggestion %q should contain actionable word %q", result.Data.Suggestion, tc.action)
				}
			})
		}
	})
}

func TestVeryLongErrorMessages(t *testing.T) {
	t.Run("handles very long error messages without truncation", func(t *testing.T) {
		longMessage := strings.Repeat("a", 10000)
		err := errors.New(longMessage)
		result := FormatError(err)

		if result.Data.Cause != longMessage {
			t.Error("Long error messages should not be truncated")
		}
	})

	t.Run("handles very long session ID", func(t *testing.T) {
		longID := strings.Repeat("x", 1000)
		err := NewSessionNotFoundError(longID)
		result := FormatError(err)

		if result.Data.SessionID != longID {
			t.Error("Long session ID should not be truncated")
		}
	})

	t.Run("handles very long selector", func(t *testing.T) {
		longSelector := "#" + strings.Repeat("a", 1000)
		err := NewElementNotFoundError(longSelector, 5000)
		result := FormatError(err)

		if result.Data.Selector != longSelector {
			t.Error("Long selector should not be truncated")
		}
	})
}

func TestUnwrapMessage(t *testing.T) {
	t.Run("returns empty string for nil error", func(t *testing.T) {
		result := unwrapMessage(nil)
		if result != "" {
			t.Errorf("unwrapMessage(nil) = %q, want empty string", result)
		}
	})

	t.Run("returns error message", func(t *testing.T) {
		err := errors.New("test error")
		result := unwrapMessage(err)
		if result != "test error" {
			t.Errorf("unwrapMessage(err) = %q, want %q", result, "test error")
		}
	})
}

func TestStandardMessage(t *testing.T) {
	testCases := []struct {
		code     int
		expected string
	}{
		{CodeParseError, msgParseError},
		{CodeInvalidRequest, msgInvalidRequest},
		{CodeMethodNotFound, msgMethodNotFound},
		{CodeInvalidParams, msgInvalidParams},
		{CodeInternalError, msgInternalError},
		{CodeSessionNotFound, msgSessionNotFound},
		{CodeElementNotFound, msgElementNotFound},
		{CodeTimeout, msgTimeout},
		{CodeNavigationFailed, msgNavigationFailed},
		{-99999, msgInternalError}, // Unknown code defaults to internal error
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := standardMessage(tc.code)
			if result != tc.expected {
				t.Errorf("standardMessage(%d) = %q, want %q", tc.code, result, tc.expected)
			}
		})
	}
}
