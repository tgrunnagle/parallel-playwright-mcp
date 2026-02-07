package shutdown

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewRequestTracker(t *testing.T) {
	tracker := NewRequestTracker()
	if tracker == nil {
		t.Fatal("expected non-nil tracker")
	}
	if tracker.ActiveCount() != 0 {
		t.Errorf("expected initial count 0, got %d", tracker.ActiveCount())
	}
}

func TestRequestTrackerActiveCount(t *testing.T) {
	tracker := NewRequestTracker()

	if count := tracker.ActiveCount(); count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestRequestTrackerMiddleware_IncrementsOnRequestStart(t *testing.T) {
	tracker := NewRequestTracker()
	started := make(chan struct{})
	proceed := make(chan struct{})

	handler := tracker.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-proceed
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	go handler.ServeHTTP(rr, req)

	<-started
	if count := tracker.ActiveCount(); count != 1 {
		t.Errorf("expected count 1 during request, got %d", count)
	}

	close(proceed)
	// Give goroutine time to complete
	time.Sleep(10 * time.Millisecond)
}

func TestRequestTrackerMiddleware_DecrementsOnRequestComplete(t *testing.T) {
	tracker := NewRequestTracker()

	handler := tracker.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if count := tracker.ActiveCount(); count != 0 {
		t.Errorf("expected count 0 after request, got %d", count)
	}
}

func TestRequestTrackerMiddleware_DecrementsOnPanic(t *testing.T) {
	tracker := NewRequestTracker()

	handler := tracker.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	// The panic will propagate, but defer should still run
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate")
		}
		if count := tracker.ActiveCount(); count != 0 {
			t.Errorf("expected count 0 after panic, got %d", count)
		}
	}()

	handler.ServeHTTP(rr, req)
}

func TestRequestTrackerMiddleware_ConcurrentRequests(t *testing.T) {
	tracker := NewRequestTracker()
	numRequests := 10
	started := make(chan struct{})
	allStarted := sync.WaitGroup{}
	allStarted.Add(numRequests)
	proceed := make(chan struct{})

	handler := tracker.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allStarted.Done()
		<-proceed
		w.WriteHeader(http.StatusOK)
	}))

	// Start concurrent requests
	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}()
	}

	// Wait for all to start
	go func() {
		allStarted.Wait()
		close(started)
	}()

	<-started
	if count := tracker.ActiveCount(); count != int64(numRequests) {
		t.Errorf("expected count %d during concurrent requests, got %d", numRequests, count)
	}

	close(proceed)
	// Give goroutines time to complete
	time.Sleep(50 * time.Millisecond)

	if count := tracker.ActiveCount(); count != 0 {
		t.Errorf("expected count 0 after all requests complete, got %d", count)
	}
}

func TestRequestTrackerMiddleware_PassesThroughResponse(t *testing.T) {
	tracker := NewRequestTracker()

	handler := tracker.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("test response"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}
	if rr.Body.String() != "test response" {
		t.Errorf("expected body 'test response', got '%s'", rr.Body.String())
	}
}
