package session

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

// DefaultNetworkLogBufferSize is the default maximum number of network log entries.
const DefaultNetworkLogBufferSize = 100

// NetworkLogEntry represents a single captured HTTP request/response.
type NetworkLogEntry struct {
	// Timestamp is when the request was initiated.
	Timestamp time.Time `json:"timestamp"`
	// Method is the HTTP method (GET, POST, etc.).
	Method string `json:"method"`
	// URL is the full request URL.
	URL string `json:"url"`
	// Status is the HTTP response status code (0 if request failed).
	Status int `json:"status"`
	// Duration is the time from request start to response complete.
	Duration time.Duration `json:"duration"`
	// RequestSize is the size of the request body in bytes.
	RequestSize int64 `json:"requestSize"`
	// ResponseSize is the size of the response body in bytes.
	ResponseSize int64 `json:"responseSize"`
	// ResourceType is the type of resource (document, script, image, etc.).
	ResourceType string `json:"resourceType,omitempty"`
}

// NetworkLogBuffer is a thread-safe circular buffer for network log entries.
// It maintains a fixed maximum size, automatically discarding oldest entries
// when capacity is reached.
type NetworkLogBuffer struct {
	entries    []NetworkLogEntry
	maxSize    int
	head int // Index where next entry will be written
	count      int // Number of valid entries (0 to maxSize)
	mu         sync.RWMutex
}

// NewNetworkLogBuffer creates a new NetworkLogBuffer with the specified maximum size.
// If maxSize <= 0, DefaultNetworkLogBufferSize is used.
func NewNetworkLogBuffer(maxSize int) *NetworkLogBuffer {
	if maxSize <= 0 {
		maxSize = DefaultNetworkLogBufferSize
	}
	return &NetworkLogBuffer{
		entries: make([]NetworkLogEntry, maxSize),
		maxSize: maxSize,
	}
}

// Add appends a network log entry to the buffer.
// If the buffer is full, the oldest entry is overwritten.
func (b *NetworkLogBuffer) Add(entry NetworkLogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.maxSize
	if b.count < b.maxSize {
		b.count++
	}
}

// Entries returns up to limit entries in chronological order (oldest first).
// If limit <= 0, all entries are returned.
func (b *NetworkLogBuffer) Entries(limit int) []NetworkLogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return []NetworkLogEntry{}
	}

	// Determine how many entries to return
	n := b.count
	if limit > 0 && limit < n {
		n = limit
	}

	result := make([]NetworkLogEntry, n)

	// Calculate start index for reading (oldest entry we want)
	// Skip (count - n) oldest entries if limit is less than count
	startOffset := b.count - n
	startIndex := (b.head - b.count + startOffset + b.maxSize) % b.maxSize

	for i := 0; i < n; i++ {
		result[i] = b.entries[(startIndex+i)%b.maxSize]
	}

	return result
}

// Clear removes all entries from the buffer.
func (b *NetworkLogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.head = 0
	b.count = 0
}

// Len returns the current number of entries in the buffer.
func (b *NetworkLogBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// pendingRequest tracks an in-flight HTTP request for duration calculation.
type pendingRequest struct {
	StartTime    time.Time
	Method       string
	URL          string
	RequestSize  int64
	ResourceType string
}

// requestKey generates a key for correlating requests with responses.
// Uses method + URL + frame URL to handle concurrent identical requests from
// different frames or iframes.
//
// Limitation: Truly concurrent identical requests from the SAME frame (e.g., two
// simultaneous GETs to the same URL triggered by the same script) may still
// collide. In this case, the first response would match and remove the pending
// entry, causing subsequent responses to have Duration=0. Playwright does not
// expose a unique request ID that could be used to avoid this edge case.
// This limitation is acceptable for typical browsing scenarios where such
// truly concurrent identical requests are rare.
func requestKey(request playwright.Request) string {
	frameURL := ""
	if frame := request.Frame(); frame != nil {
		frameURL = frame.URL()
	}
	return request.Method() + "|" + request.URL() + "|" + frameURL
}

// SetupNetworkLogging attaches request/response event listeners to a page
// and routes captured network activity to the provided buffer.
// Returns a cleanup function that removes the event listeners.
func SetupNetworkLogging(page playwright.Page, buffer *NetworkLogBuffer) func() {
	if buffer == nil {
		return func() {} // No-op if buffer is nil
	}

	pending := make(map[string]*pendingRequest)
	var mu sync.Mutex

	// Handler for request events
	requestHandler := func(request playwright.Request) {
		key := requestKey(request)

		var requestSize int64
		if postData, err := request.PostDataBuffer(); err == nil && postData != nil {
			requestSize = int64(len(postData))
		}

		mu.Lock()
		pending[key] = &pendingRequest{
			StartTime:    time.Now(),
			Method:       request.Method(),
			URL:          request.URL(),
			RequestSize:  requestSize,
			ResourceType: request.ResourceType(),
		}
		mu.Unlock()
	}

	// Handler for response events
	responseHandler := func(response playwright.Response) {
		request := response.Request()
		key := requestKey(request)

		mu.Lock()
		pendingReq, found := pending[key]
		if found {
			delete(pending, key)
		}
		mu.Unlock()

		// Calculate duration if we have the matching request
		var duration time.Duration
		var requestSize int64
		var resourceType string
		var timestamp time.Time

		if found {
			duration = time.Since(pendingReq.StartTime)
			requestSize = pendingReq.RequestSize
			resourceType = pendingReq.ResourceType
			timestamp = pendingReq.StartTime
		} else {
			// Request wasn't tracked, use current time and what we can get from response
			resourceType = request.ResourceType()
			timestamp = time.Now()
		}

		// Get response size from Content-Length header.
		// Note: We intentionally don't fall back to reading the response body for size
		// because response.Body() is a blocking operation that would wait for the entire
		// response to be received, potentially causing significant delays for large
		// responses or slow connections. The Content-Length header provides the size
		// without this overhead. For responses without Content-Length (e.g., chunked
		// transfer encoding), ResponseSize will be 0.
		var responseSize int64
		headers, err := response.HeadersArray()
		if err == nil {
			for _, h := range headers {
				if strings.EqualFold(h.Name, "content-length") {
					if size, parseErr := strconv.ParseInt(h.Value, 10, 64); parseErr == nil {
						responseSize = size
					}
					break
				}
			}
		}

		entry := NetworkLogEntry{
			Timestamp:    timestamp,
			Method:       request.Method(),
			URL:          response.URL(),
			Status:       response.Status(),
			Duration:     duration,
			RequestSize:  requestSize,
			ResponseSize: responseSize,
			ResourceType: resourceType,
		}

		buffer.Add(entry)
	}

	// Handler for failed requests (no response received)
	requestFailedHandler := func(request playwright.Request) {
		key := requestKey(request)

		mu.Lock()
		pendingReq, found := pending[key]
		if found {
			delete(pending, key)
		}
		mu.Unlock()

		// Record failed request with status 0
		var requestSize int64
		var resourceType string
		var timestamp time.Time

		if found {
			requestSize = pendingReq.RequestSize
			resourceType = pendingReq.ResourceType
			timestamp = pendingReq.StartTime
		} else {
			resourceType = request.ResourceType()
			timestamp = time.Now()
		}

		entry := NetworkLogEntry{
			Timestamp:    timestamp,
			Method:       request.Method(),
			URL:          request.URL(),
			Status:       0, // No response received
			Duration:     0,
			RequestSize:  requestSize,
			ResponseSize: 0,
			ResourceType: resourceType,
		}

		buffer.Add(entry)
	}

	// Attach event listeners
	page.On("request", requestHandler)
	page.On("response", responseHandler)
	page.On("requestfailed", requestFailedHandler)

	// Return cleanup function
	return func() {
		page.RemoveListener("request", requestHandler)
		page.RemoveListener("response", responseHandler)
		page.RemoveListener("requestfailed", requestFailedHandler)
	}
}
