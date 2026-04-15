// internal/api/admin/handler_test.go
package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coreadmin "github.com/daifei0527/polyant/internal/core/admin"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestCreateSession_LocalOnly tests that non-local requests are rejected
func TestCreateSession_LocalOnly(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	sessionMgr := coreadmin.NewSessionManager(time.Hour)
	handler := NewSessionHandler(sessionMgr, store.User)

	// Create a test user first
	testUser := &model.User{
		PublicKey: "test-public-key-12345",
		AgentName: "test-agent",
		UserLevel: 1,
		Status:    model.UserStatusActive,
	}
	_, err := store.User.Create(context.Background(), testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Test non-local request (should be forbidden)
	body := map[string]string{"public_key": "test-public-key-12345"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/create", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler.CreateSessionHandler(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-local request, got %d", w.Code)
	}

	// Verify the error response
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["code"].(float64) != 403 {
		t.Errorf("Expected error code 403, got %v", resp["code"])
	}
}

// TestCreateSession_Valid tests successful session creation for local requests
func TestCreateSession_Valid(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	sessionMgr := coreadmin.NewSessionManager(time.Hour)
	handler := NewSessionHandler(sessionMgr, store.User)

	// Create a test user first
	testUser := &model.User{
		PublicKey: "test-public-key-local",
		AgentName: "local-agent",
		UserLevel: 1,
		Status:    model.UserStatusActive,
	}
	_, err := store.User.Create(context.Background(), testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Test local request
	body := map[string]string{"public_key": "test-public-key-local"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/create", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "127.0.0.1:18531" // Simulate local request
	w := httptest.NewRecorder()

	handler.CreateSessionHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for local request, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the response contains a token
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["token"] == "" {
		t.Error("Expected token in response")
	}
}

// TestCreateSession_UserNotFound tests that non-existent user returns error
func TestCreateSession_UserNotFound(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	sessionMgr := coreadmin.NewSessionManager(time.Hour)
	handler := NewSessionHandler(sessionMgr, store.User)

	// Test with non-existent user
	body := map[string]string{"public_key": "non-existent-key"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/create", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "127.0.0.1:18531" // Simulate local request
	w := httptest.NewRecorder()

	handler.CreateSessionHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent user, got %d", w.Code)
	}
}

// TestCreateSession_MissingPublicKey tests that missing public_key returns error
func TestCreateSession_MissingPublicKey(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	sessionMgr := coreadmin.NewSessionManager(time.Hour)
	handler := NewSessionHandler(sessionMgr, store.User)

	// Test with missing public_key
	body := map[string]string{}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/create", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "127.0.0.1:18531"
	w := httptest.NewRecorder()

	handler.CreateSessionHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing public_key, got %d", w.Code)
	}
}

// TestSessionMiddleware tests the session authentication middleware
func TestSessionMiddleware(t *testing.T) {
	sessionMgr := coreadmin.NewSessionManager(time.Hour)
	mw := NewAuthMiddleware(sessionMgr)

	// Create a valid token
	token, _ := sessionMgr.CreateSession("test-user-public-key")

	// Test valid token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// Verify public key is in context
		pk := r.Context().Value("public_key")
		if pk == nil {
			t.Error("public_key not found in context")
		}
		if pk.(string) != "test-user-public-key" {
			t.Errorf("Expected public_key 'test-user-public-key', got '%s'", pk)
		}
		w.WriteHeader(http.StatusOK)
	})

	mw.Middleware(next).ServeHTTP(w, req)

	if !called {
		t.Fatal("middleware should call next handler for valid token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}
}

// TestSessionMiddleware_MissingToken tests middleware rejects requests without token
func TestSessionMiddleware_MissingToken(t *testing.T) {
	sessionMgr := coreadmin.NewSessionManager(time.Hour)
	mw := NewAuthMiddleware(sessionMgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	w := httptest.NewRecorder()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw.Middleware(next).ServeHTTP(w, req)

	if called {
		t.Fatal("middleware should NOT call next handler when token is missing")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}
}

// TestSessionMiddleware_InvalidToken tests middleware rejects invalid tokens
func TestSessionMiddleware_InvalidToken(t *testing.T) {
	sessionMgr := coreadmin.NewSessionManager(time.Hour)
	mw := NewAuthMiddleware(sessionMgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-12345")
	w := httptest.NewRecorder()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw.Middleware(next).ServeHTTP(w, req)

	if called {
		t.Fatal("middleware should NOT call next handler for invalid token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}
}

// TestSessionMiddleware_InvalidBearerFormat tests middleware rejects malformed Authorization header
func TestSessionMiddleware_InvalidBearerFormat(t *testing.T) {
	sessionMgr := coreadmin.NewSessionManager(time.Hour)
	mw := NewAuthMiddleware(sessionMgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Basic some-credentials") // Not Bearer
	w := httptest.NewRecorder()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw.Middleware(next).ServeHTTP(w, req)

	if called {
		t.Fatal("middleware should NOT call next handler for invalid auth format")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}
}

// TestLocalOnlyMiddleware tests the local-only restriction middleware
func TestLocalOnlyMiddleware(t *testing.T) {
	// Test non-local request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/session/create", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	LocalOnlyMiddleware(next).ServeHTTP(w, req)

	if called {
		t.Fatal("LocalOnlyMiddleware should NOT call next handler for non-local request")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden, got %d", w.Code)
	}

	// Verify error message
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["code"].(float64) != 403 {
		t.Errorf("Expected error code 403, got %v", resp["code"])
	}
}

// TestLocalOnlyMiddleware_LocalRequest tests local requests pass through
func TestLocalOnlyMiddleware_LocalRequest(t *testing.T) {
	// Test local request (using localhost Host)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/session/create", nil)
	req.Host = "localhost:18531"
	w := httptest.NewRecorder()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	LocalOnlyMiddleware(next).ServeHTTP(w, req)

	if !called {
		t.Fatal("LocalOnlyMiddleware should call next handler for local request")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}
}
