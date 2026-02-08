// Package errors provides error formatting for MCP-compliant JSON-RPC 2.0 error responses.
package errors

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrorResponse represents a JSON-RPC 2.0 error response.
// This struct is used to format errors into the standard MCP error format
// with code, message, and optional data fields.
type ErrorResponse struct {
	// Code is the JSON-RPC error code.
	Code int `json:"code"`
	// Message is the human-readable error message.
	Message string `json:"message"`
	// Data contains additional context for the error.
	Data *ErrorData `json:"data,omitempty"`
}

// ErrorData contains additional context for the error response.
// All fields are optional and included when relevant to the error type.
type ErrorData struct {
	// SessionID is the browser session ID (for session-related errors).
	SessionID string `json:"sessionId,omitempty"`
	// Selector is the CSS/XPath selector (for element-related errors).
	Selector string `json:"selector,omitempty"`
	// URL is the target URL (for navigation-related errors).
	URL string `json:"url,omitempty"`
	// Timeout is the timeout value in milliseconds.
	Timeout int `json:"timeout,omitempty"`
	// Operation describes the operation that failed.
	Operation string `json:"operation,omitempty"`
	// Reason describes why the operation failed.
	Reason string `json:"reason,omitempty"`
	// Suggestion provides actionable guidance for resolving the error.
	Suggestion string `json:"suggestion,omitempty"`
	// Cause is the underlying error message if wrapped.
	Cause string `json:"cause,omitempty"`
}

// Error messages for each error type.
const (
	msgSessionNotFound  = "Session not found"
	msgElementNotFound  = "Element not found"
	msgTimeout          = "Operation timed out"
	msgNavigationFailed = "Navigation failed"
	msgInternalError    = "Internal server error"
	msgParseError       = "Parse error"
	msgInvalidRequest   = "Invalid request"
	msgMethodNotFound   = "Method not found"
	msgInvalidParams    = "Invalid parameters"
)

// Suggestion messages for each error type.
// These are exported so they can be used consistently in ErrorData() methods.
const (
	SuggestSessionNotFound  = "Verify the session ID or create a new session with session_create"
	SuggestElementNotFound  = "Verify the selector is correct, ensure the element exists, or increase timeout"
	SuggestTimeout          = "Increase the timeout value or check if the page/element is responsive"
	SuggestNavigationFailed = "Verify the URL is accessible and correctly formatted"
	SuggestInternalError    = "An unexpected error occurred. Check server logs for details."
	SuggestParseError       = "Check that the request body is valid JSON"
	SuggestInvalidRequest   = "Verify the request follows JSON-RPC 2.0 format"
	SuggestMethodNotFound   = "Check the method name is correct and the tool is registered"
	SuggestInvalidParams    = "Verify all required parameters are provided with correct types"
)

// FormatError converts any error to an MCP-compliant ErrorResponse.
// It handles custom MCP error types, standard JSON-RPC errors, and generic errors.
// Uses errors.As to properly handle wrapped errors in the error chain.
// Returns nil if the input error is nil.
func FormatError(err error) *ErrorResponse {
	if err == nil {
		return nil
	}

	// Handle custom MCP error types using errors.As to support wrapped errors
	var sessionErr *SessionNotFoundError
	if errors.As(err, &sessionErr) {
		return formatSessionNotFoundError(sessionErr)
	}

	var elementErr *ElementNotFoundError
	if errors.As(err, &elementErr) {
		return formatElementNotFoundError(elementErr)
	}

	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return formatTimeoutError(timeoutErr)
	}

	var navErr *NavigationError
	if errors.As(err, &navErr) {
		return formatNavigationError(navErr)
	}

	// Handle as generic internal error
	return &ErrorResponse{
		Code:    CodeInternalError,
		Message: msgInternalError,
		Data: &ErrorData{
			Suggestion: SuggestInternalError,
			Cause:      err.Error(),
		},
	}
}

// formatSessionNotFoundError formats a SessionNotFoundError into an ErrorResponse.
func formatSessionNotFoundError(e *SessionNotFoundError) *ErrorResponse {
	data := &ErrorData{
		Suggestion: SuggestSessionNotFound,
	}

	if e.SessionID != "" {
		data.SessionID = e.SessionID
	}
	if cause := unwrapMessage(e.Err); cause != "" {
		data.Cause = cause
	}

	return &ErrorResponse{
		Code:    CodeSessionNotFound,
		Message: msgSessionNotFound,
		Data:    data,
	}
}

// formatElementNotFoundError formats an ElementNotFoundError into an ErrorResponse.
func formatElementNotFoundError(e *ElementNotFoundError) *ErrorResponse {
	data := &ErrorData{
		Suggestion: SuggestElementNotFound,
	}

	if e.Selector != "" {
		data.Selector = e.Selector
	}
	if e.Timeout > 0 {
		data.Timeout = e.Timeout
	}
	if cause := unwrapMessage(e.Err); cause != "" {
		data.Cause = cause
	}

	return &ErrorResponse{
		Code:    CodeElementNotFound,
		Message: msgElementNotFound,
		Data:    data,
	}
}

// formatTimeoutError formats a TimeoutError into an ErrorResponse.
func formatTimeoutError(e *TimeoutError) *ErrorResponse {
	data := &ErrorData{
		Suggestion: SuggestTimeout,
	}

	if e.Operation != "" {
		data.Operation = e.Operation
	}
	if e.Timeout > 0 {
		data.Timeout = e.Timeout
	}
	if cause := unwrapMessage(e.Err); cause != "" {
		data.Cause = cause
	}

	return &ErrorResponse{
		Code:    CodeTimeout,
		Message: msgTimeout,
		Data:    data,
	}
}

// formatNavigationError formats a NavigationError into an ErrorResponse.
func formatNavigationError(e *NavigationError) *ErrorResponse {
	data := &ErrorData{
		Suggestion: SuggestNavigationFailed,
	}

	if e.URL != "" {
		data.URL = e.URL
	}
	if e.Reason != "" {
		data.Reason = e.Reason
	}
	if cause := unwrapMessage(e.Err); cause != "" {
		data.Cause = cause
	}

	return &ErrorResponse{
		Code:    CodeNavigationFailed,
		Message: msgNavigationFailed,
		Data:    data,
	}
}

// FormatErrorWithCode formats an error with a specific JSON-RPC error code.
// This is useful for standard JSON-RPC errors (parse, invalid request, etc.)
// that don't have custom error types.
func FormatErrorWithCode(code int, message string) *ErrorResponse {
	data := &ErrorData{}

	switch code {
	case CodeParseError:
		data.Suggestion = SuggestParseError
	case CodeInvalidRequest:
		data.Suggestion = SuggestInvalidRequest
	case CodeMethodNotFound:
		data.Suggestion = SuggestMethodNotFound
	case CodeInvalidParams:
		data.Suggestion = SuggestInvalidParams
	case CodeInternalError:
		data.Suggestion = SuggestInternalError
	default:
		data.Suggestion = SuggestInternalError
	}

	if message != "" {
		data.Cause = message
	}

	return &ErrorResponse{
		Code:    code,
		Message: standardMessage(code),
		Data:    data,
	}
}

// standardMessage returns the standard message for a JSON-RPC error code.
func standardMessage(code int) string {
	switch code {
	case CodeParseError:
		return msgParseError
	case CodeInvalidRequest:
		return msgInvalidRequest
	case CodeMethodNotFound:
		return msgMethodNotFound
	case CodeInvalidParams:
		return msgInvalidParams
	case CodeInternalError:
		return msgInternalError
	case CodeSessionNotFound:
		return msgSessionNotFound
	case CodeElementNotFound:
		return msgElementNotFound
	case CodeTimeout:
		return msgTimeout
	case CodeNavigationFailed:
		return msgNavigationFailed
	default:
		return msgInternalError
	}
}

// FormatErrorForTool returns a formatted error string suitable for mcp.NewToolResultError.
// The format includes the error code, message, and context in a human-readable format.
func FormatErrorForTool(err error) string {
	if err == nil {
		return ""
	}

	resp := FormatError(err)
	if resp == nil {
		return ""
	}

	// Build a formatted string with code and context
	result := fmt.Sprintf("[%d] %s", resp.Code, resp.Message)

	if resp.Data != nil {
		// Add context fields
		if resp.Data.SessionID != "" {
			result += fmt.Sprintf(" (sessionId: %s)", resp.Data.SessionID)
		}
		if resp.Data.Selector != "" {
			result += fmt.Sprintf(" (selector: %s)", resp.Data.Selector)
		}
		if resp.Data.URL != "" {
			result += fmt.Sprintf(" (url: %s)", resp.Data.URL)
		}
		if resp.Data.Operation != "" {
			result += fmt.Sprintf(" (operation: %s)", resp.Data.Operation)
		}
		if resp.Data.Timeout > 0 {
			result += fmt.Sprintf(" (timeout: %dms)", resp.Data.Timeout)
		}
		if resp.Data.Reason != "" {
			result += fmt.Sprintf(" (reason: %s)", resp.Data.Reason)
		}
		if resp.Data.Cause != "" {
			result += fmt.Sprintf(": %s", resp.Data.Cause)
		}

		// Add suggestion
		if resp.Data.Suggestion != "" {
			result += fmt.Sprintf(". %s", resp.Data.Suggestion)
		}
	}

	return result
}

// ToJSON serializes an ErrorResponse to JSON bytes.
// Returns an error if serialization fails.
func (r *ErrorResponse) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// unwrapMessage extracts the error message from a wrapped error.
// Returns an empty string if the error is nil.
func unwrapMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
