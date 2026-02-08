package session

import (
	"sync"
	"time"
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
