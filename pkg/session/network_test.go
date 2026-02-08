package session

import (
	"sync"
	"testing"
	"time"
)

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
			if buf.writeIndex != 0 {
				t.Errorf("writeIndex = %d, want 0", buf.writeIndex)
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
