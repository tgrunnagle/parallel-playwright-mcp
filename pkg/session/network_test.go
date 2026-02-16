package session

import (
	"sync"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// waitForEntries polls the buffer until it has at least n entries or timeout is reached.
// Network event handlers run in goroutines to avoid deadlocking the playwright-go
// driver, so tests must wait for async processing to complete.
func waitForEntries(t *testing.T, buffer *NetworkLogBuffer, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for buffer.Len() < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDefaultNetworkLogBufferSize(t *testing.T) {
	if DefaultNetworkLogBufferSize != 100 {
		t.Errorf("DefaultNetworkLogBufferSize = %d, want 100", DefaultNetworkLogBufferSize)
	}
}

func TestNewNetworkLogBuffer(t *testing.T) {
	tests := []struct {
		name            string
		maxSize         int
		expectedMaxSize int
	}{
		{
			name:            "valid size",
			maxSize:         100,
			expectedMaxSize: 100,
		},
		{
			name:            "zero uses default",
			maxSize:         0,
			expectedMaxSize: DefaultNetworkLogBufferSize,
		},
		{
			name:            "negative uses default",
			maxSize:         -5,
			expectedMaxSize: DefaultNetworkLogBufferSize,
		},
		{
			name:            "small size",
			maxSize:         1,
			expectedMaxSize: 1,
		},
		{
			name:            "large size",
			maxSize:         1000,
			expectedMaxSize: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := NewNetworkLogBuffer(tt.maxSize)
			if buf == nil {
				t.Fatal("NewNetworkLogBuffer returned nil")
			}
			if buf.maxSize != tt.expectedMaxSize {
				t.Errorf("maxSize = %d, want %d", buf.maxSize, tt.expectedMaxSize)
			}
			if buf.count != 0 {
				t.Errorf("count = %d, want 0", buf.count)
			}
			if buf.head != 0 {
				t.Errorf("head = %d, want 0", buf.head)
			}
			if len(buf.entries) != tt.expectedMaxSize {
				t.Errorf("entries length = %d, want %d", len(buf.entries), tt.expectedMaxSize)
			}
		})
	}
}

func TestNetworkLogBuffer_Add(t *testing.T) {
	t.Run("add single entry", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)
		entry := NetworkLogEntry{
			Timestamp:    time.Now(),
			Method:       "GET",
			URL:          "https://example.com",
			Status:       200,
			Duration:     100 * time.Millisecond,
			RequestSize:  0,
			ResponseSize: 1024,
		}

		buf.Add(entry)

		if buf.Len() != 1 {
			t.Errorf("Len() = %d, want 1", buf.Len())
		}
	})

	t.Run("add multiple entries", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		for i := 0; i < 5; i++ {
			entry := NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com",
				Status:    200,
			}
			buf.Add(entry)
		}

		if buf.Len() != 5 {
			t.Errorf("Len() = %d, want 5", buf.Len())
		}
	})

	t.Run("circular eviction when buffer full", func(t *testing.T) {
		buf := NewNetworkLogBuffer(3)

		// Add 5 entries to a buffer of size 3
		for i := 0; i < 5; i++ {
			entry := NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com",
			}
			buf.Add(entry)
		}

		// Count should be capped at maxSize
		if buf.Len() != 3 {
			t.Errorf("Len() = %d, want 3", buf.Len())
		}
	})

	t.Run("preserves chronological order after wrap-around", func(t *testing.T) {
		buf := NewNetworkLogBuffer(3)

		// Add 5 entries with distinct URLs
		urls := []string{"url1", "url2", "url3", "url4", "url5"}
		for _, url := range urls {
			entry := NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       url,
			}
			buf.Add(entry)
		}

		// Get all entries - should be the last 3 in order
		entries := buf.Entries(0)

		if len(entries) != 3 {
			t.Fatalf("Got %d entries, want 3", len(entries))
		}

		// Should have url3, url4, url5 in chronological order
		expected := []string{"url3", "url4", "url5"}
		for i, entry := range entries {
			if entry.URL != expected[i] {
				t.Errorf("entries[%d].URL = %q, want %q", i, entry.URL, expected[i])
			}
		}
	})
}

func TestNetworkLogBuffer_Entries(t *testing.T) {
	t.Run("get from empty buffer returns empty slice not nil", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)
		entries := buf.Entries(0)

		if entries == nil {
			t.Error("Entries() returned nil, want empty slice")
		}
		if len(entries) != 0 {
			t.Errorf("Got %d entries from empty buffer, want 0", len(entries))
		}
	})

	t.Run("get all entries with limit 0", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		for i := 0; i < 5; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com",
			})
		}

		entries := buf.Entries(0) // 0 means no limit

		if len(entries) != 5 {
			t.Errorf("Got %d entries, want 5", len(entries))
		}
	})

	t.Run("get all entries with negative limit", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		for i := 0; i < 5; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com",
			})
		}

		entries := buf.Entries(-1) // -1 means no limit

		if len(entries) != 5 {
			t.Errorf("Got %d entries, want 5", len(entries))
		}
	})

	t.Run("get with limit", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		for i := 0; i < 10; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com",
			})
		}

		entries := buf.Entries(5)

		if len(entries) != 5 {
			t.Errorf("Got %d entries, want 5", len(entries))
		}
	})

	t.Run("get with limit larger than count", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		for i := 0; i < 3; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com",
			})
		}

		entries := buf.Entries(100) // Limit larger than count

		if len(entries) != 3 {
			t.Errorf("Got %d entries, want 3", len(entries))
		}
	})

	t.Run("entries returned in chronological order oldest first", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		now := time.Now()
		for i := 0; i < 5; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: now.Add(time.Duration(i) * time.Second),
				Method:    "GET",
				URL:       "https://example.com",
			})
		}

		entries := buf.Entries(0)

		for i := 1; i < len(entries); i++ {
			if !entries[i].Timestamp.After(entries[i-1].Timestamp) {
				t.Error("Entries not in chronological order")
			}
		}
	})

	t.Run("limit returns most recent entries", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		// Add 10 entries with sequential URLs
		for i := 0; i < 10; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com/" + string(rune('a'+i)),
			})
		}

		// Get only 3 entries
		entries := buf.Entries(3)

		if len(entries) != 3 {
			t.Fatalf("Got %d entries, want 3", len(entries))
		}

		// Should be the last 3 entries (h, i, j)
		expectedSuffixes := []string{"h", "i", "j"}
		for i, entry := range entries {
			expected := "https://example.com/" + expectedSuffixes[i]
			if entry.URL != expected {
				t.Errorf("entries[%d].URL = %q, want %q", i, entry.URL, expected)
			}
		}
	})
}

func TestNetworkLogBuffer_Clear(t *testing.T) {
	t.Run("clear removes all entries", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		for i := 0; i < 5; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com",
			})
		}

		if buf.Len() != 5 {
			t.Fatalf("Len() = %d, want 5 before clear", buf.Len())
		}

		buf.Clear()

		if buf.Len() != 0 {
			t.Errorf("Len() = %d after clear, want 0", buf.Len())
		}
	})

	t.Run("entries after clear returns empty", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		buf.Add(NetworkLogEntry{Method: "GET", URL: "https://example.com"})
		buf.Clear()

		entries := buf.Entries(0)

		if len(entries) != 0 {
			t.Errorf("Got %d entries after clear, want 0", len(entries))
		}
	})

	t.Run("add after clear works correctly", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		buf.Add(NetworkLogEntry{Method: "GET", URL: "https://old.com"})
		buf.Clear()
		buf.Add(NetworkLogEntry{Method: "POST", URL: "https://new.com"})

		entries := buf.Entries(0)

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].URL != "https://new.com" {
			t.Errorf("Entry URL = %q, want %q", entries[0].URL, "https://new.com")
		}
		if entries[0].Method != "POST" {
			t.Errorf("Entry Method = %q, want %q", entries[0].Method, "POST")
		}
	})
}

func TestNetworkLogBuffer_Len(t *testing.T) {
	t.Run("len on empty buffer", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)
		if buf.Len() != 0 {
			t.Errorf("Len() = %d, want 0", buf.Len())
		}
	})

	t.Run("len after adds", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		for i := 0; i < 5; i++ {
			buf.Add(NetworkLogEntry{Method: "GET", URL: "https://example.com"})
		}

		if buf.Len() != 5 {
			t.Errorf("Len() = %d, want 5", buf.Len())
		}
	})

	t.Run("len capped at maxSize", func(t *testing.T) {
		buf := NewNetworkLogBuffer(5)

		for i := 0; i < 10; i++ {
			buf.Add(NetworkLogEntry{Method: "GET", URL: "https://example.com"})
		}

		if buf.Len() != 5 {
			t.Errorf("Len() = %d, want 5", buf.Len())
		}
	})
}

// TestNetworkLogBuffer_Concurrency validates thread-safety of the NetworkLogBuffer.
// NOTE: These tests are designed to catch race conditions and should be run with
// the Go race detector enabled: `go test -race ./pkg/session/...`
// The race detector requires CGO to be enabled, which may not be available on
// all platforms (e.g., Windows with CGO_ENABLED=0). CI pipelines should run
// these tests on Linux or macOS with CGO enabled to fully validate thread safety.
func TestNetworkLogBuffer_Concurrency(t *testing.T) {
	t.Run("concurrent adds are thread-safe", func(t *testing.T) {
		buf := NewNetworkLogBuffer(100)
		var wg sync.WaitGroup

		// Launch 10 goroutines each adding 10 entries
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					buf.Add(NetworkLogEntry{
						Timestamp: time.Now(),
						Method:    "GET",
						URL:       "https://example.com",
					})
				}
			}()
		}

		wg.Wait()

		// Should have exactly 100 entries
		if buf.Len() != 100 {
			t.Errorf("Len() = %d, want 100", buf.Len())
		}
	})

	t.Run("concurrent entries reads are thread-safe", func(t *testing.T) {
		buf := NewNetworkLogBuffer(100)

		// Pre-populate buffer
		for i := 0; i < 50; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com",
			})
		}

		var wg sync.WaitGroup

		// Launch 10 goroutines each doing 10 reads
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					entries := buf.Entries(0)
					if len(entries) != 50 {
						t.Errorf("Got %d entries, want 50", len(entries))
					}
				}
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent adds and entries reads are thread-safe", func(t *testing.T) {
		buf := NewNetworkLogBuffer(50)
		var wg sync.WaitGroup

		// Pre-populate with some entries
		for i := 0; i < 20; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://initial.com",
			})
		}

		// Launch adders
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					buf.Add(NetworkLogEntry{
						Timestamp: time.Now(),
						Method:    "POST",
						URL:       "https://new.com",
					})
				}
			}()
		}

		// Launch readers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					_ = buf.Entries(0)
				}
			}()
		}

		wg.Wait()

		// Buffer should be at max capacity
		if buf.Len() != 50 {
			t.Errorf("Len() = %d, want 50", buf.Len())
		}
	})
}

func TestNetworkLogBuffer_EdgeCases(t *testing.T) {
	t.Run("buffer with maxSize 1", func(t *testing.T) {
		buf := NewNetworkLogBuffer(1)

		buf.Add(NetworkLogEntry{Method: "GET", URL: "https://first.com"})
		buf.Add(NetworkLogEntry{Method: "POST", URL: "https://second.com"})

		entries := buf.Entries(0)

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].URL != "https://second.com" {
			t.Errorf("Entry URL = %q, want %q", entries[0].URL, "https://second.com")
		}
	})

	t.Run("circular wrap-around with maxSize+5 entries", func(t *testing.T) {
		maxSize := 10
		buf := NewNetworkLogBuffer(maxSize)

		// Add maxSize + 5 entries
		for i := 0; i < maxSize+5; i++ {
			buf.Add(NetworkLogEntry{
				Timestamp: time.Now(),
				Method:    "GET",
				URL:       "https://example.com/" + string(rune('a'+i)),
			})
		}

		entries := buf.Entries(0)

		if len(entries) != maxSize {
			t.Fatalf("Got %d entries, want %d", len(entries), maxSize)
		}

		// Verify oldest 5 are gone (entries a-e should be gone)
		// First entry should be 'f' (index 5)
		expectedFirst := "https://example.com/" + string(rune('a'+5))
		if entries[0].URL != expectedFirst {
			t.Errorf("First entry URL = %q, want %q", entries[0].URL, expectedFirst)
		}
	})

	t.Run("entry with empty URL", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		buf.Add(NetworkLogEntry{
			Timestamp: time.Now(),
			Method:    "GET",
			URL:       "",
		})

		entries := buf.Entries(0)

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].URL != "" {
			t.Errorf("Entry URL = %q, want empty string", entries[0].URL)
		}
	})

	t.Run("entry with all fields populated", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)
		now := time.Now()

		buf.Add(NetworkLogEntry{
			Timestamp:    now,
			Method:       "POST",
			URL:          "https://api.example.com/data",
			Status:       201,
			Duration:     250 * time.Millisecond,
			RequestSize:  512,
			ResponseSize: 2048,
		})

		entries := buf.Entries(0)

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		entry := entries[0]
		if entry.Method != "POST" {
			t.Errorf("Entry Method = %q, want POST", entry.Method)
		}
		if entry.URL != "https://api.example.com/data" {
			t.Errorf("Entry URL = %q, want https://api.example.com/data", entry.URL)
		}
		if entry.Status != 201 {
			t.Errorf("Entry Status = %d, want 201", entry.Status)
		}
		if entry.Duration != 250*time.Millisecond {
			t.Errorf("Entry Duration = %v, want 250ms", entry.Duration)
		}
		if entry.RequestSize != 512 {
			t.Errorf("Entry RequestSize = %d, want 512", entry.RequestSize)
		}
		if entry.ResponseSize != 2048 {
			t.Errorf("Entry ResponseSize = %d, want 2048", entry.ResponseSize)
		}
	})

	t.Run("entry with status 0 for failed request", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		buf.Add(NetworkLogEntry{
			Timestamp:    time.Now(),
			Method:       "GET",
			URL:          "https://unreachable.example.com",
			Status:       0,
			Duration:     5 * time.Second,
			RequestSize:  0,
			ResponseSize: 0,
		})

		entries := buf.Entries(0)

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].Status != 0 {
			t.Errorf("Entry Status = %d, want 0", entries[0].Status)
		}
	})

	t.Run("interleaved add and entries operations", func(t *testing.T) {
		buf := NewNetworkLogBuffer(5)

		// Add some entries
		for i := 0; i < 3; i++ {
			buf.Add(NetworkLogEntry{
				Method: "GET",
				URL:    "https://example.com/" + string(rune('a'+i)),
			})
		}

		// Read entries
		entries := buf.Entries(0)
		if len(entries) != 3 {
			t.Fatalf("Got %d entries after first batch, want 3", len(entries))
		}

		// Add more entries
		for i := 3; i < 7; i++ {
			buf.Add(NetworkLogEntry{
				Method: "GET",
				URL:    "https://example.com/" + string(rune('a'+i)),
			})
		}

		// Read entries again - should have 5 entries (buffer is full)
		entries = buf.Entries(0)
		if len(entries) != 5 {
			t.Fatalf("Got %d entries after second batch, want 5", len(entries))
		}

		// Verify chronological order - should be c, d, e, f, g
		expectedSuffixes := []string{"c", "d", "e", "f", "g"}
		for i, entry := range entries {
			expected := "https://example.com/" + expectedSuffixes[i]
			if entry.URL != expected {
				t.Errorf("entries[%d].URL = %q, want %q", i, entry.URL, expected)
			}
		}
	})
}

func TestNetworkLogEntry_Fields(t *testing.T) {
	t.Run("all HTTP methods", func(t *testing.T) {
		methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
		buf := NewNetworkLogBuffer(10)

		for _, method := range methods {
			buf.Add(NetworkLogEntry{
				Method: method,
				URL:    "https://example.com",
			})
		}

		entries := buf.Entries(0)
		if len(entries) != len(methods) {
			t.Fatalf("Got %d entries, want %d", len(entries), len(methods))
		}

		for i, entry := range entries {
			if entry.Method != methods[i] {
				t.Errorf("entries[%d].Method = %q, want %q", i, entry.Method, methods[i])
			}
		}
	})

	t.Run("various status codes", func(t *testing.T) {
		statusCodes := []int{200, 201, 301, 302, 400, 401, 403, 404, 500, 502, 503}
		buf := NewNetworkLogBuffer(20)

		for _, status := range statusCodes {
			buf.Add(NetworkLogEntry{
				Method: "GET",
				URL:    "https://example.com",
				Status: status,
			})
		}

		entries := buf.Entries(0)
		if len(entries) != len(statusCodes) {
			t.Fatalf("Got %d entries, want %d", len(entries), len(statusCodes))
		}

		for i, entry := range entries {
			if entry.Status != statusCodes[i] {
				t.Errorf("entries[%d].Status = %d, want %d", i, entry.Status, statusCodes[i])
			}
		}
	})

	t.Run("large request and response sizes", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		buf.Add(NetworkLogEntry{
			Timestamp:    time.Now(),
			Method:       "POST",
			URL:          "https://example.com/upload",
			Status:       200,
			RequestSize:  1024 * 1024 * 100, // 100MB
			ResponseSize: 1024 * 1024 * 50,  // 50MB
		})

		entries := buf.Entries(0)
		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].RequestSize != 1024*1024*100 {
			t.Errorf("RequestSize = %d, want %d", entries[0].RequestSize, 1024*1024*100)
		}
		if entries[0].ResponseSize != 1024*1024*50 {
			t.Errorf("ResponseSize = %d, want %d", entries[0].ResponseSize, 1024*1024*50)
		}
	})

	t.Run("very long duration", func(t *testing.T) {
		buf := NewNetworkLogBuffer(10)

		longDuration := 30 * time.Second
		buf.Add(NetworkLogEntry{
			Timestamp: time.Now(),
			Method:    "GET",
			URL:       "https://slow-server.example.com",
			Status:    200,
			Duration:  longDuration,
		})

		entries := buf.Entries(0)
		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].Duration != longDuration {
			t.Errorf("Duration = %v, want %v", entries[0].Duration, longDuration)
		}
	})
}

// mockNetworkPage is a mock implementation of playwright.Page for network event testing.
type mockNetworkPage struct {
	playwright.Page
	requestHandlers       []func(playwright.Request)
	responseHandlers      []func(playwright.Response)
	requestFailedHandlers []func(playwright.Request)
}

func (m *mockNetworkPage) On(event string, handler interface{}) {
	switch event {
	case "request":
		if h, ok := handler.(func(playwright.Request)); ok {
			m.requestHandlers = append(m.requestHandlers, h)
		}
	case "response":
		if h, ok := handler.(func(playwright.Response)); ok {
			m.responseHandlers = append(m.responseHandlers, h)
		}
	case "requestfailed":
		if h, ok := handler.(func(playwright.Request)); ok {
			m.requestFailedHandlers = append(m.requestFailedHandlers, h)
		}
	}
}

func (m *mockNetworkPage) RemoveListener(event string, handler interface{}) {
	// In a real implementation, we'd remove the specific handler
	// For testing, we just clear all handlers of that type
	switch event {
	case "request":
		m.requestHandlers = nil
	case "response":
		m.responseHandlers = nil
	case "requestfailed":
		m.requestFailedHandlers = nil
	}
}

func (m *mockNetworkPage) simulateRequest(req playwright.Request) {
	for _, h := range m.requestHandlers {
		h(req)
	}
}

func (m *mockNetworkPage) simulateResponse(resp playwright.Response) {
	for _, h := range m.responseHandlers {
		h(resp)
	}
}

func (m *mockNetworkPage) simulateRequestFailed(req playwright.Request) {
	for _, h := range m.requestFailedHandlers {
		h(req)
	}
}

// mockFrame is a mock implementation of playwright.Frame.
type mockFrame struct {
	playwright.Frame
	url string
}

func (m *mockFrame) URL() string {
	return m.url
}

// mockRequest is a mock implementation of playwright.Request.
type mockRequest struct {
	playwright.Request
	method       string
	url          string
	postData     []byte
	resourceType string
	frame        playwright.Frame
}

func (m *mockRequest) Method() string {
	return m.method
}

func (m *mockRequest) URL() string {
	return m.url
}

func (m *mockRequest) PostDataBuffer() ([]byte, error) {
	return m.postData, nil
}

func (m *mockRequest) ResourceType() string {
	return m.resourceType
}

func (m *mockRequest) Frame() playwright.Frame {
	if m.frame == nil {
		return &mockFrame{url: ""}
	}
	return m.frame
}

// mockResponse is a mock implementation of playwright.Response.
type mockResponse struct {
	playwright.Response
	request playwright.Request
	url     string
	status  int
	headers []playwright.NameValue
}

func (m *mockResponse) Request() playwright.Request {
	return m.request
}

func (m *mockResponse) URL() string {
	return m.url
}

func (m *mockResponse) Status() int {
	return m.status
}

func (m *mockResponse) HeadersArray() ([]playwright.NameValue, error) {
	return m.headers, nil
}

func TestRequestKey(t *testing.T) {
	t.Run("generates unique key for different methods to same URL", func(t *testing.T) {
		req1 := &mockRequest{method: "GET", url: "https://example.com/api"}
		req2 := &mockRequest{method: "POST", url: "https://example.com/api"}

		key1 := requestKey(req1)
		key2 := requestKey(req2)

		if key1 == key2 {
			t.Errorf("keys should be different: %s vs %s", key1, key2)
		}
	})

	t.Run("generates same key for identical requests", func(t *testing.T) {
		req1 := &mockRequest{method: "GET", url: "https://example.com/api"}
		req2 := &mockRequest{method: "GET", url: "https://example.com/api"}

		key1 := requestKey(req1)
		key2 := requestKey(req2)

		if key1 != key2 {
			t.Errorf("keys should be same: %s vs %s", key1, key2)
		}
	})

	t.Run("generates different keys for different URLs", func(t *testing.T) {
		req1 := &mockRequest{method: "GET", url: "https://example.com/api/1"}
		req2 := &mockRequest{method: "GET", url: "https://example.com/api/2"}

		key1 := requestKey(req1)
		key2 := requestKey(req2)

		if key1 == key2 {
			t.Errorf("keys should be different: %s vs %s", key1, key2)
		}
	})

	t.Run("generates different keys for same URL from different frames", func(t *testing.T) {
		frame1 := &mockFrame{url: "https://main.example.com"}
		frame2 := &mockFrame{url: "https://iframe.example.com"}

		req1 := &mockRequest{method: "GET", url: "https://example.com/api", frame: frame1}
		req2 := &mockRequest{method: "GET", url: "https://example.com/api", frame: frame2}

		key1 := requestKey(req1)
		key2 := requestKey(req2)

		if key1 == key2 {
			t.Errorf("keys should be different for different frames: %s vs %s", key1, key2)
		}
	})

	t.Run("generates same key for same URL from same frame", func(t *testing.T) {
		frame := &mockFrame{url: "https://main.example.com"}

		req1 := &mockRequest{method: "GET", url: "https://example.com/api", frame: frame}
		req2 := &mockRequest{method: "GET", url: "https://example.com/api", frame: frame}

		key1 := requestKey(req1)
		key2 := requestKey(req2)

		if key1 != key2 {
			t.Errorf("keys should be same for same frame: %s vs %s", key1, key2)
		}
	})
}

func TestSetupNetworkLogging(t *testing.T) {
	t.Run("returns non-nil cleanup function", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		cleanup := SetupNetworkLogging(page, buffer)
		if cleanup == nil {
			t.Error("SetupNetworkLogging should return non-nil cleanup function")
		}
	})

	t.Run("returns no-op cleanup function when buffer is nil", func(t *testing.T) {
		page := &mockNetworkPage{}

		cleanup := SetupNetworkLogging(page, nil)
		if cleanup == nil {
			t.Error("SetupNetworkLogging should return non-nil cleanup function even with nil buffer")
		}
		// Should not panic
		cleanup()
	})

	t.Run("attaches request event handler", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		SetupNetworkLogging(page, buffer)

		if len(page.requestHandlers) != 1 {
			t.Errorf("expected 1 request handler, got %d", len(page.requestHandlers))
		}
	})

	t.Run("attaches response event handler", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		SetupNetworkLogging(page, buffer)

		if len(page.responseHandlers) != 1 {
			t.Errorf("expected 1 response handler, got %d", len(page.responseHandlers))
		}
	})

	t.Run("attaches requestfailed event handler", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		SetupNetworkLogging(page, buffer)

		if len(page.requestFailedHandlers) != 1 {
			t.Errorf("expected 1 requestfailed handler, got %d", len(page.requestFailedHandlers))
		}
	})

	t.Run("cleanup function removes event listeners", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		cleanup := SetupNetworkLogging(page, buffer)
		cleanup()

		if len(page.requestHandlers) != 0 {
			t.Errorf("expected 0 request handlers after cleanup, got %d", len(page.requestHandlers))
		}
		if len(page.responseHandlers) != 0 {
			t.Errorf("expected 0 response handlers after cleanup, got %d", len(page.responseHandlers))
		}
		if len(page.requestFailedHandlers) != 0 {
			t.Errorf("expected 0 requestfailed handlers after cleanup, got %d", len(page.requestFailedHandlers))
		}
	})

	t.Run("captures successful request/response pair", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		SetupNetworkLogging(page, buffer)

		req := &mockRequest{
			method:       "GET",
			url:          "https://example.com/api",
			postData:     nil,
			resourceType: "document",
		}

		// Simulate request event
		page.simulateRequest(req)

		// Wait for async request handler goroutine to record the pending request,
		// then add delay to ensure duration is measurable
		time.Sleep(50 * time.Millisecond)

		// Simulate response event
		resp := &mockResponse{
			request: req,
			url:     "https://example.com/api",
			status:  200,
			headers: []playwright.NameValue{{Name: "content-length", Value: "1024"}},
		}
		page.simulateResponse(resp)

		// Wait for async handler goroutines to complete
		waitForEntries(t, buffer, 1)

		// Verify entry was captured
		entries := buffer.Entries(0)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]
		if entry.Method != "GET" {
			t.Errorf("Method = %q, want GET", entry.Method)
		}
		if entry.URL != "https://example.com/api" {
			t.Errorf("URL = %q, want https://example.com/api", entry.URL)
		}
		if entry.Status != 200 {
			t.Errorf("Status = %d, want 200", entry.Status)
		}
		if entry.Duration == 0 {
			t.Error("Duration should be non-zero for successful request")
		}
		if entry.ResponseSize != 1024 {
			t.Errorf("ResponseSize = %d, want 1024", entry.ResponseSize)
		}
		if entry.ResourceType != "document" {
			t.Errorf("ResourceType = %q, want document", entry.ResourceType)
		}
	})

	t.Run("captures POST request with body size", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		SetupNetworkLogging(page, buffer)

		postData := []byte(`{"name": "test"}`)
		req := &mockRequest{
			method:       "POST",
			url:          "https://example.com/api",
			postData:     postData,
			resourceType: "xhr",
		}

		page.simulateRequest(req)

		// Wait for async request handler goroutine to record the pending request
		// (including PostDataBuffer call) before simulating the response
		time.Sleep(50 * time.Millisecond)

		resp := &mockResponse{
			request: req,
			url:     "https://example.com/api",
			status:  201,
			headers: []playwright.NameValue{{Name: "content-length", Value: "256"}},
		}
		page.simulateResponse(resp)

		waitForEntries(t, buffer, 1)

		entries := buffer.Entries(0)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]
		if entry.Method != "POST" {
			t.Errorf("Method = %q, want POST", entry.Method)
		}
		if entry.RequestSize != int64(len(postData)) {
			t.Errorf("RequestSize = %d, want %d", entry.RequestSize, len(postData))
		}
		if entry.Status != 201 {
			t.Errorf("Status = %d, want 201", entry.Status)
		}
		if entry.ResourceType != "xhr" {
			t.Errorf("ResourceType = %q, want xhr", entry.ResourceType)
		}
	})

	t.Run("captures failed request with status 0", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		SetupNetworkLogging(page, buffer)

		req := &mockRequest{
			method:       "GET",
			url:          "https://unreachable.example.com",
			resourceType: "document",
		}

		page.simulateRequest(req)
		page.simulateRequestFailed(req)

		waitForEntries(t, buffer, 1)

		entries := buffer.Entries(0)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]
		if entry.Status != 0 {
			t.Errorf("Status = %d, want 0 for failed request", entry.Status)
		}
		if entry.Duration != 0 {
			t.Errorf("Duration = %v, want 0 for failed request", entry.Duration)
		}
		if entry.URL != "https://unreachable.example.com" {
			t.Errorf("URL = %q, want https://unreachable.example.com", entry.URL)
		}
		if entry.ResourceType != "document" {
			t.Errorf("ResourceType = %q, want document", entry.ResourceType)
		}
	})

	t.Run("handles response without tracked request", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		SetupNetworkLogging(page, buffer)

		req := &mockRequest{
			method:       "GET",
			url:          "https://example.com/untracked",
			resourceType: "script",
		}

		// Simulate response without prior request event (late attachment)
		resp := &mockResponse{
			request: req,
			url:     "https://example.com/untracked",
			status:  200,
			headers: []playwright.NameValue{},
		}
		page.simulateResponse(resp)

		waitForEntries(t, buffer, 1)

		entries := buffer.Entries(0)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]
		if entry.Status != 200 {
			t.Errorf("Status = %d, want 200", entry.Status)
		}
		// Duration will be 0 since no tracked request
		if entry.Duration != 0 {
			t.Errorf("Duration = %v, want 0 for untracked request", entry.Duration)
		}
	})

	t.Run("handles multiple concurrent requests", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(100)

		SetupNetworkLogging(page, buffer)

		// Simulate multiple requests starting
		reqs := []*mockRequest{
			{method: "GET", url: "https://example.com/1", resourceType: "script"},
			{method: "GET", url: "https://example.com/2", resourceType: "image"},
			{method: "POST", url: "https://example.com/3", resourceType: "xhr"},
		}

		for _, req := range reqs {
			page.simulateRequest(req)
		}

		// Simulate responses in different order
		for i := len(reqs) - 1; i >= 0; i-- {
			resp := &mockResponse{
				request: reqs[i],
				url:     reqs[i].url,
				status:  200,
				headers: []playwright.NameValue{},
			}
			page.simulateResponse(resp)
		}

		waitForEntries(t, buffer, 3)

		entries := buffer.Entries(0)
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
	})

	// NOTE: This test is designed to catch race conditions. Run with -race flag
	// on a platform with CGO enabled (Linux/macOS) for full validation.
	t.Run("concurrent access is thread-safe", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(1000)

		SetupNetworkLogging(page, buffer)

		var wg sync.WaitGroup
		requestCount := 100

		// Launch multiple goroutines simulating requests
		for i := 0; i < requestCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				req := &mockRequest{
					method:       "GET",
					url:          "https://example.com/" + string(rune('0'+idx%10)),
					resourceType: "document",
				}

				page.simulateRequest(req)

				resp := &mockResponse{
					request: req,
					url:     req.url,
					status:  200,
					headers: []playwright.NameValue{},
				}
				page.simulateResponse(resp)
			}(i)
		}

		wg.Wait()

		// Wait for all async handler goroutines to finish
		waitForEntries(t, buffer, requestCount)

		// All entries should have been captured
		if buffer.Len() != requestCount {
			t.Errorf("expected %d entries, got %d", requestCount, buffer.Len())
		}
	})

	t.Run("response without Content-Length header", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		SetupNetworkLogging(page, buffer)

		req := &mockRequest{
			method:       "GET",
			url:          "https://example.com/api",
			resourceType: "document",
		}

		page.simulateRequest(req)

		resp := &mockResponse{
			request: req,
			url:     "https://example.com/api",
			status:  200,
			headers: []playwright.NameValue{}, // No Content-Length
		}
		page.simulateResponse(resp)

		waitForEntries(t, buffer, 1)

		entries := buffer.Entries(0)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		if entries[0].ResponseSize != 0 {
			t.Errorf("ResponseSize = %d, want 0 when no Content-Length", entries[0].ResponseSize)
		}
	})

	t.Run("no entries added after cleanup", func(t *testing.T) {
		page := &mockNetworkPage{}
		buffer := NewNetworkLogBuffer(10)

		cleanup := SetupNetworkLogging(page, buffer)

		// Add one entry before cleanup
		req := &mockRequest{method: "GET", url: "https://example.com/before", resourceType: "document"}
		page.simulateRequest(req)
		resp := &mockResponse{request: req, url: req.url, status: 200, headers: []playwright.NameValue{}}
		page.simulateResponse(resp)

		waitForEntries(t, buffer, 1)

		if buffer.Len() != 1 {
			t.Fatalf("expected 1 entry before cleanup, got %d", buffer.Len())
		}

		// Call cleanup
		cleanup()

		// Handlers are cleared, so new events won't be processed by our handlers
		// (Since we cleared the handlers in RemoveListener mock)
		beforeCount := buffer.Len()

		// Try to add another entry after cleanup
		req2 := &mockRequest{method: "GET", url: "https://example.com/after", resourceType: "document"}
		page.simulateRequest(req2)
		resp2 := &mockResponse{request: req2, url: req2.url, status: 200, headers: []playwright.NameValue{}}
		page.simulateResponse(resp2)

		// Give any stray goroutines time to run (they shouldn't add entries)
		time.Sleep(50 * time.Millisecond)

		if buffer.Len() != beforeCount {
			t.Errorf("buffer length changed after cleanup: was %d, now %d", beforeCount, buffer.Len())
		}
	})
}
