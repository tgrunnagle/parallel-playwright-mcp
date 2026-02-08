// Package session implements the 1:N browser session management model.
package session

import (
	"sync"
	"time"
)

// ConsoleLogLevel represents the severity level of a console message.
type ConsoleLogLevel string

const (
	// ConsoleLogLevelLog represents console.log() messages.
	ConsoleLogLevelLog ConsoleLogLevel = "log"
	// ConsoleLogLevelInfo represents console.info() messages.
	ConsoleLogLevelInfo ConsoleLogLevel = "info"
	// ConsoleLogLevelWarn represents console.warn() messages.
	ConsoleLogLevelWarn ConsoleLogLevel = "warn"
	// ConsoleLogLevelError represents console.error() messages.
	ConsoleLogLevelError ConsoleLogLevel = "error"
	// ConsoleLogLevelDebug represents console.debug() messages.
	ConsoleLogLevelDebug ConsoleLogLevel = "debug"
)

// DefaultConsoleLogBufferSize is the default maximum number of entries in a console log buffer.
const DefaultConsoleLogBufferSize = 100

// ConsoleLogEntry represents a single browser console message.
type ConsoleLogEntry struct {
	// Timestamp is when the console message was captured.
	Timestamp time.Time `json:"timestamp"`
	// Level is the console message severity (log, info, warn, error, debug).
	Level ConsoleLogLevel `json:"level"`
	// Text is the console message content.
	Text string `json:"text"`
	// URL is the source URL where the console message originated, if available.
	URL string `json:"url,omitempty"`
	// Line is the source line number, if available.
	Line int `json:"line,omitempty"`
	// Column is the source column number, if available.
	Column int `json:"column,omitempty"`
}

// ConsoleLogBuffer is a thread-safe circular buffer for console log entries.
// It stores up to maxSize entries, evicting the oldest when capacity is reached.
type ConsoleLogBuffer struct {
	mu      sync.RWMutex
	entries []ConsoleLogEntry
	maxSize int
	head    int // Index where next entry will be written
	count   int // Current number of entries in buffer
}

// NewConsoleLogBuffer creates a new ConsoleLogBuffer with the specified capacity.
// If maxSize is <= 0, defaults to DefaultConsoleLogBufferSize (100 entries).
func NewConsoleLogBuffer(maxSize int) *ConsoleLogBuffer {
	if maxSize <= 0 {
		maxSize = DefaultConsoleLogBufferSize
	}
	return &ConsoleLogBuffer{
		entries: make([]ConsoleLogEntry, maxSize),
		maxSize: maxSize,
		head:    0,
		count:   0,
	}
}

// Add appends a console log entry to the buffer.
// If the buffer is at capacity, the oldest entry is evicted.
func (b *ConsoleLogBuffer) Add(entry ConsoleLogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.maxSize
	if b.count < b.maxSize {
		b.count++
	}
}

// Get returns console log entries in chronological order.
// If level is non-empty, only entries matching that level are returned.
// If limit is <= 0, all matching entries are returned.
func (b *ConsoleLogBuffer) Get(limit int, level ConsoleLogLevel) []ConsoleLogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return []ConsoleLogEntry{}
	}

	// Calculate starting index for chronological order
	start := (b.head - b.count + b.maxSize) % b.maxSize

	// Collect entries in chronological order with optional filtering
	result := make([]ConsoleLogEntry, 0, b.count)
	for i := 0; i < b.count; i++ {
		idx := (start + i) % b.maxSize
		entry := b.entries[idx]

		// Apply level filter if specified
		if level != "" && entry.Level != level {
			continue
		}
		result = append(result, entry)

		// Apply limit if specified
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// Clear removes all entries from the buffer.
func (b *ConsoleLogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.head = 0
	b.count = 0
	// Note: We don't need to zero out entries slice, just reset indices
}

// Len returns the current number of entries in the buffer.
func (b *ConsoleLogBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// MapConsoleType maps Playwright console message types to ConsoleLogLevel.
// Unknown types are mapped to ConsoleLogLevelLog as a safe default.
func MapConsoleType(msgType string) ConsoleLogLevel {
	switch msgType {
	case "log":
		return ConsoleLogLevelLog
	case "info":
		return ConsoleLogLevelInfo
	case "warning": // Playwright uses "warning", not "warn"
		return ConsoleLogLevelWarn
	case "error":
		return ConsoleLogLevelError
	case "debug":
		return ConsoleLogLevelDebug
	default:
		// Treat unknown types as log level
		return ConsoleLogLevelLog
	}
}
