package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseRecorder_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rr := &responseRecorder{
		ResponseWriter: rec,
		statusCode:     0,
	}

	rr.WriteHeader(http.StatusCreated)
	if rr.statusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rr.statusCode)
	}
}

func TestResponseRecorder_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	rr := &responseRecorder{
		ResponseWriter: rec,
		statusCode:     0,
	}

	n, err := rr.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 4 {
		t.Errorf("Expected 4 bytes written, got %d", n)
	}
	if rr.written != 4 {
		t.Errorf("Expected written 4, got %d", rr.written)
	}
	// Write should set default status code
	if rr.statusCode != http.StatusOK {
		t.Errorf("Expected default status %d, got %d", http.StatusOK, rr.statusCode)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRecoveryMiddleware_WithPanic(t *testing.T) {
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	// Check response body
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if resp["message"] != "internal server error" {
		t.Errorf("Expected error message, got '%v'", resp["message"])
	}
}

func TestRequestIDMiddleware_ExistingID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-Id", "existing-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-Id") != "existing-id" {
		t.Errorf("Expected request ID 'existing-id', got '%s'", rec.Header().Get("X-Request-Id"))
	}
}

func TestRequestIDMiddleware_GenerateID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	requestID := rec.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Error("Expected non-empty request ID")
	}
	// Check format: YYYYMMDD-HHMMSS.mmm-xxxxxxxx
	if !strings.Contains(requestID, "-") {
		t.Errorf("Request ID format unexpected: %s", requestID)
	}
}

func TestGenerateRequestID(t *testing.T) {
	id := generateRequestID()
	if id == "" {
		t.Error("Expected non-empty request ID")
	}
	// Should contain timestamp and random hex
	parts := strings.Split(id, "-")
	if len(parts) < 2 {
		t.Errorf("Expected at least 2 parts in ID, got: %s", id)
	}
}

func TestRandomHex(t *testing.T) {
	hex1 := randomHex(4)
	if len(hex1) != 8 { // 4 bytes = 8 hex chars
		t.Errorf("Expected 8 hex chars, got %d", len(hex1))
	}

	hex2 := randomHex(4)
	if hex1 == hex2 {
		t.Error("Expected different random values")
	}
}
