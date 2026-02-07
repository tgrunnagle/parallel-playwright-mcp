// Package errors defines custom error codes and types for the Playwright MCP Server.
// These error types extend the standard JSON-RPC 2.0 error codes with application-specific
// codes for session, element, timeout, and navigation failures.
package errors

import "fmt"

// Standard JSON-RPC 2.0 error codes (for reference).
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Custom Playwright MCP error codes.
const (
	CodeSessionNotFound  = -32001
	CodeElementNotFound  = -32002
	CodeTimeout          = -32003
	CodeNavigationFailed = -32004
)

// MCPError is the interface all custom MCP errors implement.
// It extends the standard error interface with methods for JSON-RPC error responses.
type MCPError interface {
	error
	// ErrorCode returns the JSON-RPC error code for this error type.
	ErrorCode() int
	// ErrorData returns additional context for the error response data field.
	ErrorData() map[string]any
}

// SessionNotFoundError indicates the requested session ID does not exist.
type SessionNotFoundError struct {
	// SessionID is the browser session ID that was not found.
	SessionID string
	// Err is the optional underlying error for error wrapping.
	Err error
}

// Error implements the error interface.
func (e *SessionNotFoundError) Error() string {
	if e.SessionID == "" {
		if e.Err != nil {
			return fmt.Sprintf("session not found: %v", e.Err)
		}
		return "session not found"
	}
	if e.Err != nil {
		return fmt.Sprintf("session not found: %s: %v", e.SessionID, e.Err)
	}
	return fmt.Sprintf("session not found: %s", e.SessionID)
}

// ErrorCode returns the JSON-RPC error code for session not found errors.
func (e *SessionNotFoundError) ErrorCode() int {
	return CodeSessionNotFound
}

// ErrorData returns additional context for the error response.
func (e *SessionNotFoundError) ErrorData() map[string]any {
	data := map[string]any{
		"suggestion": "Verify the session ID or create a new session",
	}
	if e.SessionID != "" {
		data["sessionId"] = e.SessionID
	}
	return data
}

// Unwrap returns the underlying error for error chain traversal.
func (e *SessionNotFoundError) Unwrap() error {
	return e.Err
}

// NewSessionNotFoundError creates a new SessionNotFoundError with the given session ID.
func NewSessionNotFoundError(sessionID string) *SessionNotFoundError {
	return &SessionNotFoundError{SessionID: sessionID}
}

// WrapSessionNotFoundError creates a SessionNotFoundError that wraps an underlying error.
func WrapSessionNotFoundError(sessionID string, err error) *SessionNotFoundError {
	return &SessionNotFoundError{SessionID: sessionID, Err: err}
}

// ElementNotFoundError indicates a selector did not match any elements.
type ElementNotFoundError struct {
	// Selector is the CSS selector, XPath, or other selector that failed to match.
	Selector string
	// Timeout is the timeout in milliseconds that was used when waiting for the element.
	Timeout int
	// Err is the optional underlying error for error wrapping.
	Err error
}

// Error implements the error interface.
func (e *ElementNotFoundError) Error() string {
	if e.Selector == "" {
		if e.Err != nil {
			return fmt.Sprintf("element not found: %v", e.Err)
		}
		return "element not found"
	}
	if e.Timeout > 0 {
		if e.Err != nil {
			return fmt.Sprintf("element not found: selector %q after %dms: %v", e.Selector, e.Timeout, e.Err)
		}
		return fmt.Sprintf("element not found: selector %q after %dms", e.Selector, e.Timeout)
	}
	if e.Err != nil {
		return fmt.Sprintf("element not found: selector %q: %v", e.Selector, e.Err)
	}
	return fmt.Sprintf("element not found: selector %q", e.Selector)
}

// ErrorCode returns the JSON-RPC error code for element not found errors.
func (e *ElementNotFoundError) ErrorCode() int {
	return CodeElementNotFound
}

// ErrorData returns additional context for the error response.
func (e *ElementNotFoundError) ErrorData() map[string]any {
	data := map[string]any{
		"suggestion": "Verify the selector or increase timeout",
	}
	if e.Selector != "" {
		data["selector"] = e.Selector
	}
	if e.Timeout > 0 {
		data["timeout"] = e.Timeout
	}
	return data
}

// Unwrap returns the underlying error for error chain traversal.
func (e *ElementNotFoundError) Unwrap() error {
	return e.Err
}

// NewElementNotFoundError creates a new ElementNotFoundError with the given selector and timeout.
func NewElementNotFoundError(selector string, timeout int) *ElementNotFoundError {
	return &ElementNotFoundError{Selector: selector, Timeout: timeout}
}

// WrapElementNotFoundError creates an ElementNotFoundError that wraps an underlying error.
func WrapElementNotFoundError(selector string, timeout int, err error) *ElementNotFoundError {
	return &ElementNotFoundError{Selector: selector, Timeout: timeout, Err: err}
}

// TimeoutError indicates an operation exceeded its timeout.
type TimeoutError struct {
	// Operation describes the operation that timed out.
	Operation string
	// Timeout is the timeout value in milliseconds.
	Timeout int
	// Err is the optional underlying error for error wrapping.
	Err error
}

// Error implements the error interface.
func (e *TimeoutError) Error() string {
	if e.Operation == "" {
		if e.Timeout > 0 {
			if e.Err != nil {
				return fmt.Sprintf("operation timed out after %dms: %v", e.Timeout, e.Err)
			}
			return fmt.Sprintf("operation timed out after %dms", e.Timeout)
		}
		if e.Err != nil {
			return fmt.Sprintf("operation timed out: %v", e.Err)
		}
		return "operation timed out"
	}
	if e.Timeout > 0 {
		if e.Err != nil {
			return fmt.Sprintf("%s timed out after %dms: %v", e.Operation, e.Timeout, e.Err)
		}
		return fmt.Sprintf("%s timed out after %dms", e.Operation, e.Timeout)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s timed out: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("%s timed out", e.Operation)
}

// ErrorCode returns the JSON-RPC error code for timeout errors.
func (e *TimeoutError) ErrorCode() int {
	return CodeTimeout
}

// ErrorData returns additional context for the error response.
func (e *TimeoutError) ErrorData() map[string]any {
	data := map[string]any{
		"suggestion": "Increase timeout or verify the operation can complete",
	}
	if e.Operation != "" {
		data["operation"] = e.Operation
	}
	if e.Timeout > 0 {
		data["timeout"] = e.Timeout
	}
	return data
}

// Unwrap returns the underlying error for error chain traversal.
func (e *TimeoutError) Unwrap() error {
	return e.Err
}

// NewTimeoutError creates a new TimeoutError with the given operation and timeout.
func NewTimeoutError(operation string, timeout int) *TimeoutError {
	return &TimeoutError{Operation: operation, Timeout: timeout}
}

// WrapTimeoutError creates a TimeoutError that wraps an underlying error.
func WrapTimeoutError(operation string, timeout int, err error) *TimeoutError {
	return &TimeoutError{Operation: operation, Timeout: timeout, Err: err}
}

// NavigationError indicates a page load or navigation failure.
type NavigationError struct {
	// URL is the URL that failed to load.
	URL string
	// Reason describes why the navigation failed.
	Reason string
	// Err is the optional underlying error for error wrapping.
	Err error
}

// Error implements the error interface.
func (e *NavigationError) Error() string {
	if e.URL == "" {
		if e.Reason != "" {
			if e.Err != nil {
				return fmt.Sprintf("navigation failed: %s: %v", e.Reason, e.Err)
			}
			return fmt.Sprintf("navigation failed: %s", e.Reason)
		}
		if e.Err != nil {
			return fmt.Sprintf("navigation failed: %v", e.Err)
		}
		return "navigation failed"
	}
	if e.Reason != "" {
		if e.Err != nil {
			return fmt.Sprintf("navigation to %q failed: %s: %v", e.URL, e.Reason, e.Err)
		}
		return fmt.Sprintf("navigation to %q failed: %s", e.URL, e.Reason)
	}
	if e.Err != nil {
		return fmt.Sprintf("navigation to %q failed: %v", e.URL, e.Err)
	}
	return fmt.Sprintf("navigation to %q failed", e.URL)
}

// ErrorCode returns the JSON-RPC error code for navigation errors.
func (e *NavigationError) ErrorCode() int {
	return CodeNavigationFailed
}

// ErrorData returns additional context for the error response.
func (e *NavigationError) ErrorData() map[string]any {
	data := map[string]any{
		"suggestion": "Verify the URL is accessible and correctly formatted",
	}
	if e.URL != "" {
		data["url"] = e.URL
	}
	if e.Reason != "" {
		data["reason"] = e.Reason
	}
	return data
}

// Unwrap returns the underlying error for error chain traversal.
func (e *NavigationError) Unwrap() error {
	return e.Err
}

// NewNavigationError creates a new NavigationError with the given URL and reason.
func NewNavigationError(url, reason string) *NavigationError {
	return &NavigationError{URL: url, Reason: reason}
}

// WrapNavigationError creates a NavigationError that wraps an underlying error.
func WrapNavigationError(url, reason string, err error) *NavigationError {
	return &NavigationError{URL: url, Reason: reason, Err: err}
}

// Compile-time interface compliance checks.
var (
	_ MCPError = (*SessionNotFoundError)(nil)
	_ MCPError = (*ElementNotFoundError)(nil)
	_ MCPError = (*TimeoutError)(nil)
	_ MCPError = (*NavigationError)(nil)
)
