// Package middleware provides HTTP middleware for the Playwright MCP server.
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// PanicRecovery wraps HTTP handlers to recover from panics.
// On panic, it logs the error with stack trace and returns 500 to the client.
// The server continues running after recovering from a request panic.
func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				slog.Error("panic recovered in HTTP handler",
					"panic", err,
					"method", r.Method,
					"path", r.URL.Path,
					"remoteAddr", r.RemoteAddr,
					"stack", string(stack),
				)

				// Return 500 Internal Server Error
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// PanicRecoveryMiddleware is an alias for PanicRecovery for consistency with common naming.
var PanicRecoveryMiddleware = PanicRecovery
