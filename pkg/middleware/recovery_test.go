package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPanicRecovery_NoPanic(t *testing.T) {
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if rr.Body.String() != "success" {
		t.Errorf("expected body 'success', got '%s'", rr.Body.String())
	}
}

func TestPanicRecovery_WithPanic(t *testing.T) {
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestPanicRecovery_WithPanicError(t *testing.T) {
	testErr := "custom error message"
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(testErr)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestPanicRecovery_AllowsSubsequentRequests(t *testing.T) {
	callCount := 0
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			panic("first request panics")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("second request ok"))
	}))

	// First request - should panic and recover
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusInternalServerError {
		t.Errorf("first request: expected status 500, got %d", rr1.Code)
	}

	// Second request - should succeed
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("second request: expected status 200, got %d", rr2.Code)
	}
	if rr2.Body.String() != "second request ok" {
		t.Errorf("second request: expected body 'second request ok', got '%s'", rr2.Body.String())
	}
}

func TestPanicRecovery_MethodAndPathAvailable(t *testing.T) {
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/resource", nil)
	rr := httptest.NewRecorder()

	// Should not panic - the request info should be available for logging
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestPanicRecoveryMiddleware_Alias(t *testing.T) {
	// PanicRecoveryMiddleware should be an alias for PanicRecovery
	if PanicRecoveryMiddleware == nil {
		t.Error("expected PanicRecoveryMiddleware to be defined")
	}

	handler := PanicRecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}
