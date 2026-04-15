package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestNewAuditMiddleware(t *testing.T) {
	store := kv.NewMemoryStore()
	auditStore := kv.NewAuditStore(store)
	m := NewAuditMiddleware(auditStore)
	if m == nil {
		t.Fatal("Expected non-nil AuditMiddleware")
	}
}

func TestAuditMiddleware_Middleware_NonSensitive(t *testing.T) {
	store := kv.NewMemoryStore()
	auditStore := kv.NewAuditStore(store)
	m := NewAuditMiddleware(auditStore)

	// Non-sensitive path - should not create audit log
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/info", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAuditMiddleware_Middleware_Sensitive(t *testing.T) {
	store := kv.NewMemoryStore()
	auditStore := kv.NewAuditStore(store)
	m := NewAuditMiddleware(auditStore)

	// Sensitive path - should create audit log
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", bytes.NewBufferString(`{"title": "test"}`))
	ctx := context.WithValue(req.Context(), PublicKeyKey, "test-pubkey")
	ctx = context.WithValue(ctx, UserLevelKey, int32(1))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Wait for async audit log write
	time.Sleep(100 * time.Millisecond)
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rec,
		status:         0,
	}

	rw.WriteHeader(http.StatusNotFound)
	if rw.status != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rw.status)
	}
}

func TestResponseWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rec,
		status:         0,
	}

	n, err := rw.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 4 {
		t.Errorf("Expected 4 bytes written, got %d", n)
	}
	if rw.body.String() != "test" {
		t.Errorf("Expected body 'test', got '%s'", rw.body.String())
	}
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")

	ip := getClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("Expected IP '192.168.1.1', got '%s'", ip)
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "192.168.1.2")

	ip := getClientIP(req)
	if ip != "192.168.1.2" {
		t.Errorf("Expected IP '192.168.1.2', got '%s'", ip)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.3:12345"

	ip := getClientIP(req)
	if ip != "192.168.1.3" {
		t.Errorf("Expected IP '192.168.1.3', got '%s'", ip)
	}
}

func TestGetErrorMessage_ValidJSON(t *testing.T) {
	body := `{"code":0,"message":"test error"}`
	msg := getErrorMessage(body)
	if msg != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", msg)
	}
}

func TestGetErrorMessage_EmptyBody(t *testing.T) {
	msg := getErrorMessage("")
	if msg != "" {
		t.Errorf("Expected empty message, got '%s'", msg)
	}
}

func TestGetErrorMessage_InvalidJSON(t *testing.T) {
	msg := getErrorMessage("not json")
	if msg != "" {
		t.Errorf("Expected empty message for invalid JSON, got '%s'", msg)
	}
}

func TestWriteAuditLog(t *testing.T) {
	store := kv.NewMemoryStore()
	auditStore := kv.NewAuditStore(store)
	m := NewAuditMiddleware(auditStore)

	log := &model.AuditLog{
		Timestamp:      time.Now().UnixMilli(),
		OperatorPubkey: "test-pubkey",
		ActionType:     "entry.create",
		Success:        true,
	}

	m.writeAuditLog(context.Background(), log)

	// Verify log was created (ID should be set)
	if log.ID == "" {
		t.Error("Expected log ID to be set")
	}
}
