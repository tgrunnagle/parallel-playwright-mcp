package session

import (
	"sync"
	"testing"
	"time"
)

func TestNewConsoleLogBuffer(t *testing.T) {
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
			expectedMaxSize: DefaultConsoleLogBufferSize,
		},
		{
			name:            "negative uses default",
			maxSize:         -5,
			expectedMaxSize: DefaultConsoleLogBufferSize,
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
			buf := NewConsoleLogBuffer(tt.maxSize)
			if buf == nil {
				t.Fatal("NewConsoleLogBuffer returned nil")
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

func TestConsoleLogBuffer_Add(t *testing.T) {
	t.Run("add single entry", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)
		entry := ConsoleLogEntry{
			Timestamp: time.Now(),
			Level:     ConsoleLogLevelLog,
			Text:      "test message",
		}

		buf.Add(entry)

		if buf.Count() != 1 {
			t.Errorf("Count() = %d, want 1", buf.Count())
		}
	})

	t.Run("add multiple entries", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		for i := 0; i < 5; i++ {
			entry := ConsoleLogEntry{
				Timestamp: time.Now(),
				Level:     ConsoleLogLevelLog,
				Text:      "message",
			}
			buf.Add(entry)
		}

		if buf.Count() != 5 {
			t.Errorf("Count() = %d, want 5", buf.Count())
		}
	})

	t.Run("circular eviction when buffer full", func(t *testing.T) {
		buf := NewConsoleLogBuffer(3)

		// Add 5 entries to a buffer of size 3
		for i := 0; i < 5; i++ {
			entry := ConsoleLogEntry{
				Timestamp: time.Now(),
				Level:     ConsoleLogLevelLog,
				Text:      "message",
			}
			buf.Add(entry)
		}

		// Count should be capped at maxSize
		if buf.Count() != 3 {
			t.Errorf("Count() = %d, want 3", buf.Count())
		}
	})

	t.Run("preserves chronological order after wrap-around", func(t *testing.T) {
		buf := NewConsoleLogBuffer(3)

		// Add 5 entries with distinct messages
		messages := []string{"msg1", "msg2", "msg3", "msg4", "msg5"}
		for _, msg := range messages {
			entry := ConsoleLogEntry{
				Timestamp: time.Now(),
				Level:     ConsoleLogLevelLog,
				Text:      msg,
			}
			buf.Add(entry)
		}

		// Get all entries - should be the last 3 in order
		entries := buf.Get(0, "")

		if len(entries) != 3 {
			t.Fatalf("Got %d entries, want 3", len(entries))
		}

		// Should have msg3, msg4, msg5 in chronological order
		expected := []string{"msg3", "msg4", "msg5"}
		for i, entry := range entries {
			if entry.Text != expected[i] {
				t.Errorf("entries[%d].Text = %q, want %q", i, entry.Text, expected[i])
			}
		}
	})
}

func TestConsoleLogBuffer_Get(t *testing.T) {
	t.Run("get from empty buffer", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)
		entries := buf.Get(0, "")

		if len(entries) != 0 {
			t.Errorf("Got %d entries from empty buffer, want 0", len(entries))
		}
	})

	t.Run("get all entries without limit", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		for i := 0; i < 5; i++ {
			buf.Add(ConsoleLogEntry{
				Timestamp: time.Now(),
				Level:     ConsoleLogLevelLog,
				Text:      "message",
			})
		}

		entries := buf.Get(0, "") // 0 means no limit

		if len(entries) != 5 {
			t.Errorf("Got %d entries, want 5", len(entries))
		}
	})

	t.Run("get with limit", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		for i := 0; i < 10; i++ {
			buf.Add(ConsoleLogEntry{
				Timestamp: time.Now(),
				Level:     ConsoleLogLevelLog,
				Text:      "message",
			})
		}

		entries := buf.Get(5, "")

		if len(entries) != 5 {
			t.Errorf("Got %d entries, want 5", len(entries))
		}
	})

	t.Run("get with limit larger than count", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		for i := 0; i < 3; i++ {
			buf.Add(ConsoleLogEntry{
				Timestamp: time.Now(),
				Level:     ConsoleLogLevelLog,
				Text:      "message",
			})
		}

		entries := buf.Get(100, "") // Limit larger than count

		if len(entries) != 3 {
			t.Errorf("Got %d entries, want 3", len(entries))
		}
	})

	t.Run("filter by level", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		// Add mixed level entries
		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "log1"})
		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelError, Text: "error1"})
		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "log2"})
		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelWarn, Text: "warn1"})
		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelError, Text: "error2"})

		// Filter only errors
		entries := buf.Get(0, ConsoleLogLevelError)

		if len(entries) != 2 {
			t.Fatalf("Got %d entries, want 2", len(entries))
		}

		for _, entry := range entries {
			if entry.Level != ConsoleLogLevelError {
				t.Errorf("Entry level = %q, want %q", entry.Level, ConsoleLogLevelError)
			}
		}
	})

	t.Run("filter by level with limit", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		// Add multiple error entries
		for i := 0; i < 5; i++ {
			buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelError, Text: "error"})
		}

		// Get only 2 errors
		entries := buf.Get(2, ConsoleLogLevelError)

		if len(entries) != 2 {
			t.Errorf("Got %d entries, want 2", len(entries))
		}
	})

	t.Run("filter with non-matching level returns empty", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "log"})
		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelWarn, Text: "warn"})

		entries := buf.Get(0, ConsoleLogLevelDebug)

		if len(entries) != 0 {
			t.Errorf("Got %d entries, want 0", len(entries))
		}
	})

	t.Run("entries returned in chronological order", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		now := time.Now()
		for i := 0; i < 5; i++ {
			buf.Add(ConsoleLogEntry{
				Timestamp: now.Add(time.Duration(i) * time.Second),
				Level:     ConsoleLogLevelLog,
				Text:      "message",
			})
		}

		entries := buf.Get(0, "")

		for i := 1; i < len(entries); i++ {
			if !entries[i].Timestamp.After(entries[i-1].Timestamp) {
				t.Error("Entries not in chronological order")
			}
		}
	})
}

func TestConsoleLogBuffer_Clear(t *testing.T) {
	t.Run("clear removes all entries", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		for i := 0; i < 5; i++ {
			buf.Add(ConsoleLogEntry{
				Timestamp: time.Now(),
				Level:     ConsoleLogLevelLog,
				Text:      "message",
			})
		}

		if buf.Count() != 5 {
			t.Fatalf("Count() = %d, want 5 before clear", buf.Count())
		}

		buf.Clear()

		if buf.Count() != 0 {
			t.Errorf("Count() = %d after clear, want 0", buf.Count())
		}
	})

	t.Run("get after clear returns empty", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "message"})
		buf.Clear()

		entries := buf.Get(0, "")

		if len(entries) != 0 {
			t.Errorf("Got %d entries after clear, want 0", len(entries))
		}
	})

	t.Run("add after clear works correctly", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "old"})
		buf.Clear()
		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "new"})

		entries := buf.Get(0, "")

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].Text != "new" {
			t.Errorf("Entry text = %q, want %q", entries[0].Text, "new")
		}
	})
}

func TestConsoleLogBuffer_Count(t *testing.T) {
	t.Run("count on empty buffer", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)
		if buf.Count() != 0 {
			t.Errorf("Count() = %d, want 0", buf.Count())
		}
	})

	t.Run("count after adds", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		for i := 0; i < 5; i++ {
			buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "message"})
		}

		if buf.Count() != 5 {
			t.Errorf("Count() = %d, want 5", buf.Count())
		}
	})

	t.Run("count capped at maxSize", func(t *testing.T) {
		buf := NewConsoleLogBuffer(5)

		for i := 0; i < 10; i++ {
			buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "message"})
		}

		if buf.Count() != 5 {
			t.Errorf("Count() = %d, want 5", buf.Count())
		}
	})
}

func TestConsoleLogBuffer_Concurrency(t *testing.T) {
	t.Run("concurrent adds are thread-safe", func(t *testing.T) {
		buf := NewConsoleLogBuffer(100)
		var wg sync.WaitGroup

		// Launch 10 goroutines each adding 10 entries
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					buf.Add(ConsoleLogEntry{
						Timestamp: time.Now(),
						Level:     ConsoleLogLevelLog,
						Text:      "concurrent message",
					})
				}
			}()
		}

		wg.Wait()

		// Should have exactly 100 entries
		if buf.Count() != 100 {
			t.Errorf("Count() = %d, want 100", buf.Count())
		}
	})

	t.Run("concurrent gets are thread-safe", func(t *testing.T) {
		buf := NewConsoleLogBuffer(100)

		// Pre-populate buffer
		for i := 0; i < 50; i++ {
			buf.Add(ConsoleLogEntry{
				Timestamp: time.Now(),
				Level:     ConsoleLogLevelLog,
				Text:      "message",
			})
		}

		var wg sync.WaitGroup

		// Launch 10 goroutines each doing 10 gets
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					entries := buf.Get(0, "")
					if len(entries) != 50 {
						t.Errorf("Got %d entries, want 50", len(entries))
					}
				}
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent adds and gets are thread-safe", func(t *testing.T) {
		buf := NewConsoleLogBuffer(50)
		var wg sync.WaitGroup

		// Pre-populate with some entries
		for i := 0; i < 20; i++ {
			buf.Add(ConsoleLogEntry{
				Timestamp: time.Now(),
				Level:     ConsoleLogLevelLog,
				Text:      "initial",
			})
		}

		// Launch adders
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					buf.Add(ConsoleLogEntry{
						Timestamp: time.Now(),
						Level:     ConsoleLogLevelLog,
						Text:      "new",
					})
				}
			}()
		}

		// Launch getters
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					_ = buf.Get(0, "")
				}
			}()
		}

		wg.Wait()

		// Buffer should be at max capacity
		if buf.Count() != 50 {
			t.Errorf("Count() = %d, want 50", buf.Count())
		}
	})
}

func TestConsoleLogBuffer_EdgeCases(t *testing.T) {
	t.Run("buffer with maxSize 1", func(t *testing.T) {
		buf := NewConsoleLogBuffer(1)

		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "first"})
		buf.Add(ConsoleLogEntry{Level: ConsoleLogLevelLog, Text: "second"})

		entries := buf.Get(0, "")

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].Text != "second" {
			t.Errorf("Entry text = %q, want %q", entries[0].Text, "second")
		}
	})

	t.Run("entry with empty text", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		buf.Add(ConsoleLogEntry{
			Timestamp: time.Now(),
			Level:     ConsoleLogLevelLog,
			Text:      "",
		})

		entries := buf.Get(0, "")

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].Text != "" {
			t.Errorf("Entry text = %q, want empty string", entries[0].Text)
		}
	})

	t.Run("entry with very long text", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		// Create a 1MB string
		longText := make([]byte, 1024*1024)
		for i := range longText {
			longText[i] = 'a'
		}

		buf.Add(ConsoleLogEntry{
			Timestamp: time.Now(),
			Level:     ConsoleLogLevelLog,
			Text:      string(longText),
		})

		entries := buf.Get(0, "")

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if len(entries[0].Text) != 1024*1024 {
			t.Errorf("Entry text length = %d, want %d", len(entries[0].Text), 1024*1024)
		}
	})

	t.Run("entry with all optional fields empty", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		buf.Add(ConsoleLogEntry{
			Timestamp: time.Now(),
			Level:     ConsoleLogLevelLog,
			Text:      "message",
			URL:       "",
			Line:      0,
			Column:    0,
		})

		entries := buf.Get(0, "")

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].URL != "" {
			t.Errorf("Entry URL = %q, want empty", entries[0].URL)
		}
		if entries[0].Line != 0 {
			t.Errorf("Entry Line = %d, want 0", entries[0].Line)
		}
		if entries[0].Column != 0 {
			t.Errorf("Entry Column = %d, want 0", entries[0].Column)
		}
	})

	t.Run("entry with all optional fields populated", func(t *testing.T) {
		buf := NewConsoleLogBuffer(10)

		buf.Add(ConsoleLogEntry{
			Timestamp: time.Now(),
			Level:     ConsoleLogLevelError,
			Text:      "error message",
			URL:       "https://example.com/app.js",
			Line:      42,
			Column:    15,
		})

		entries := buf.Get(0, "")

		if len(entries) != 1 {
			t.Fatalf("Got %d entries, want 1", len(entries))
		}

		if entries[0].URL != "https://example.com/app.js" {
			t.Errorf("Entry URL = %q, want %q", entries[0].URL, "https://example.com/app.js")
		}
		if entries[0].Line != 42 {
			t.Errorf("Entry Line = %d, want 42", entries[0].Line)
		}
		if entries[0].Column != 15 {
			t.Errorf("Entry Column = %d, want 15", entries[0].Column)
		}
	})
}

func TestMapConsoleType(t *testing.T) {
	tests := []struct {
		msgType  string
		expected ConsoleLogLevel
	}{
		{"log", ConsoleLogLevelLog},
		{"info", ConsoleLogLevelInfo},
		{"warning", ConsoleLogLevelWarn}, // Playwright uses "warning"
		{"error", ConsoleLogLevelError},
		{"debug", ConsoleLogLevelDebug},
		{"unknown", ConsoleLogLevelLog},    // Unknown defaults to log
		{"", ConsoleLogLevelLog},           // Empty defaults to log
		{"trace", ConsoleLogLevelLog},      // Unknown type defaults to log
		{"assert", ConsoleLogLevelLog},     // Unknown type defaults to log
		{"table", ConsoleLogLevelLog},      // Unknown type defaults to log
		{"startGroup", ConsoleLogLevelLog}, // Unknown type defaults to log
	}

	for _, tt := range tests {
		t.Run(tt.msgType, func(t *testing.T) {
			result := MapConsoleType(tt.msgType)
			if result != tt.expected {
				t.Errorf("MapConsoleType(%q) = %q, want %q", tt.msgType, result, tt.expected)
			}
		})
	}
}
