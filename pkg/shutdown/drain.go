package shutdown

import (
	"net/http"
	"sync/atomic"
)

// RequestTracker tracks the number of active HTTP requests.
// Used during shutdown to monitor drain progress.
type RequestTracker struct {
	activeRequests atomic.Int64
}

// NewRequestTracker creates a new request tracker.
func NewRequestTracker() *RequestTracker {
	return &RequestTracker{}
}

// Middleware returns an HTTP middleware that tracks active requests.
// The counter is incremented when a request starts and decremented when it completes,
// including when the handler panics.
func (t *RequestTracker) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.activeRequests.Add(1)
		defer t.activeRequests.Add(-1)
		next.ServeHTTP(w, r)
	})
}

// ActiveCount returns the current number of in-flight requests.
func (t *RequestTracker) ActiveCount() int64 {
	return t.activeRequests.Load()
}
