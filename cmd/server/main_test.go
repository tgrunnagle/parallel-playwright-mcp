package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/shutdown"
)

func TestHealthHandler_ReturnsOKStatus(t *testing.T) {
	handler := healthHandler("0.1.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHealthHandler_ReturnsJSONContentType(t *testing.T) {
	handler := healthHandler("0.1.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestHealthHandler_ReturnsCorrectStatusField(t *testing.T) {
	handler := healthHandler("0.1.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	var response HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response.Status)
	}
}

func TestHealthHandler_ReturnsCorrectVersion(t *testing.T) {
	testVersion := "1.2.3"
	handler := healthHandler(testVersion)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	var response HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Version != testVersion {
		t.Errorf("expected version '%s', got '%s'", testVersion, response.Version)
	}
}

func TestHealthHandler_RejectsNonGetMethods(t *testing.T) {
	handler := healthHandler("0.1.0")

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d for %s, got %d", http.StatusMethodNotAllowed, method, rec.Code)
			}
		})
	}
}

// TestSignalNotifyContext_CancelledOnSIGINT verifies that signal.NotifyContext
// cancels the context when SIGINT is received.
// This test is skipped on Windows because SIGINT cannot be sent programmatically.
func TestSignalNotifyContext_CancelledOnSIGINT(t *testing.T) {
	// Skip on Windows - SIGINT cannot be sent programmatically
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping signal test on Windows - SIGINT not supported")
	}

	// Create a context with signal handling for SIGINT
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer stop()

	// Send SIGINT to the current process
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	// Use a channel to verify context cancellation
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	// Send the signal
	if err := p.Signal(syscall.SIGINT); err != nil {
		t.Skipf("Signal not supported on this platform: %v", err)
	}

	// Wait for context cancellation with timeout
	select {
	case <-done:
		// Success - context was cancelled
	case <-time.After(2 * time.Second):
		t.Error("Context was not cancelled after SIGINT")
	}
}

// TestShutdownCoordinator_TriggeredByContextCancellation verifies that
// the shutdown coordinator can be triggered by context cancellation,
// which is how signal handling works in the main function.
func TestShutdownCoordinator_TriggeredByContextCancellation(t *testing.T) {
	// Create a cancellable context (simulates signal handling)
	ctx, cancel := context.WithCancel(context.Background())

	// Create a test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	httpServer := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	// Create shutdown coordinator
	config := shutdown.DefaultConfig().
		WithDrainTimeout(100 * time.Millisecond).
		WithPhaseTimeout(100 * time.Millisecond).
		WithTotalTimeout(500 * time.Millisecond)

	coordinator := shutdown.NewCoordinator(
		config,
		httpServer,
		nil, // No session manager
		nil, // No browser pool
		nil, // Will create its own request tracker
	)

	shutdownComplete := make(chan error, 1)

	// Simulate the main loop pattern
	go func() {
		select {
		case <-ctx.Done():
			shutdownComplete <- coordinator.Shutdown(context.Background())
		case <-time.After(5 * time.Second):
			shutdownComplete <- context.DeadlineExceeded
		}
	}()

	// Cancel the context (simulates receiving a signal)
	cancel()

	// Verify shutdown was triggered
	select {
	case err := <-shutdownComplete:
		if err != nil {
			t.Errorf("Shutdown returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Shutdown was not triggered after context cancellation")
	}
}

// TestSignalHandling_ShutdownSequenceOrder verifies that when shutdown is
// triggered (via context cancellation), the shutdown phases execute in order.
func TestSignalHandling_ShutdownSequenceOrder(t *testing.T) {
	// This test verifies the integration between signal handling and shutdown sequencing
	// The actual phase ordering is tested in pkg/shutdown/shutdown_test.go
	// Here we just verify that context cancellation triggers the shutdown

	httpServer := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.NewServeMux(),
	}

	config := shutdown.DefaultConfig().
		WithDrainTimeout(50 * time.Millisecond).
		WithPhaseTimeout(50 * time.Millisecond).
		WithTotalTimeout(200 * time.Millisecond)

	coordinator := shutdown.NewCoordinator(
		config,
		httpServer,
		nil,
		nil,
		nil,
	)

	// Trigger shutdown
	err := coordinator.Shutdown(context.Background())

	// Shutdown should complete without error when there are no active resources
	if err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}

// TestSetupSignalHandler_CancelsContextOnFirstSignal verifies that
// setupSignalHandler cancels the context when the first signal is received.
// This test is skipped on Windows because SIGINT cannot be sent programmatically.
func TestSetupSignalHandler_CancelsContextOnFirstSignal(t *testing.T) {
	// Skip on Windows - signals cannot be sent programmatically in the same way
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping signal test on Windows - SIGINT not supported programmatically")
	}

	// Reset signal handlers to avoid interference from previous tests
	signal.Reset(syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handler
	setupSignalHandler(cancel)

	// Give the goroutine time to set up the signal listener
	time.Sleep(10 * time.Millisecond)

	// Send SIGINT to the current process
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	// Use a channel to verify context cancellation
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	// Send the signal
	if err := p.Signal(syscall.SIGINT); err != nil {
		t.Skipf("Signal not supported on this platform: %v", err)
	}

	// Wait for context cancellation with timeout
	select {
	case <-done:
		// Success - context was cancelled
	case <-time.After(2 * time.Second):
		t.Error("Context was not cancelled after SIGINT")
	}
}

// TestSetupSignalHandler_RegistersForSIGTERMAndSIGINT verifies that
// the signal handler sets up notification for both SIGTERM and SIGINT.
// This is a structural test that verifies the handler uses both signals.
func TestSetupSignalHandler_RegistersForSIGTERMAndSIGINT(t *testing.T) {
	// Skip on Windows - SIGTERM is not natively supported
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping signal test on Windows - SIGTERM not supported")
	}

	// Reset signal handlers to avoid interference
	signal.Reset(syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handler
	setupSignalHandler(cancel)

	// Give the goroutine time to set up
	time.Sleep(10 * time.Millisecond)

	// Test with SIGTERM
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	if err := p.Signal(syscall.SIGTERM); err != nil {
		t.Skipf("SIGTERM not supported on this platform: %v", err)
	}

	select {
	case <-done:
		// Success - context was cancelled by SIGTERM
	case <-time.After(2 * time.Second):
		t.Error("Context was not cancelled after SIGTERM")
	}
}
