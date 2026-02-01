package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
