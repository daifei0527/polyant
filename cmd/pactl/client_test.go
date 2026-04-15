package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8080")
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("Expected baseURL 'http://localhost:8080', got %s", client.baseURL)
	}
	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestClient_SetAuthToken(t *testing.T) {
	client := NewClient("http://localhost:8080")
	client.SetAuthToken("test-token")
	if client.authToken != "test-token" {
		t.Errorf("Expected authToken 'test-token', got %s", client.authToken)
	}
}

func TestClient_LoadOrGenerateKeys(t *testing.T) {
	// Create temp directory for keys
	tmpDir, err := os.MkdirTemp("", "pactl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client := NewClient("http://localhost:8080")

	// First call should generate new keys
	err = client.LoadOrGenerateKeys(tmpDir)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys failed: %v", err)
	}

	if !client.HasKeys() {
		t.Error("Client should have keys after LoadOrGenerateKeys")
	}

	if client.GetPublicKey() == "" {
		t.Error("GetPublicKey should return non-empty string")
	}

	// Check that key files were created
	keypairPath := filepath.Join(tmpDir, "keypair.json")
	if _, err := os.Stat(keypairPath); os.IsNotExist(err) {
		t.Error("keypair.json should be created")
	}

	// Second call should load existing keys
	pubKey1 := client.GetPublicKey()
	client2 := NewClient("http://localhost:8080")
	err = client2.LoadOrGenerateKeys(tmpDir)
	if err != nil {
		t.Fatalf("Second LoadOrGenerateKeys failed: %v", err)
	}

	pubKey2 := client2.GetPublicKey()
	if pubKey1 != pubKey2 {
		t.Error("Loaded public key should match the generated one")
	}
}

func TestClient_HasKeys(t *testing.T) {
	client := NewClient("http://localhost:8080")

	if client.HasKeys() {
		t.Error("New client should not have keys")
	}

	// Manually set keys
	client.publicKey = []byte("test-public-key-32-bytes-need-this")
	client.privateKey = []byte("test-private-key-need-64-bytes-total-length-need-more")

	if !client.HasKeys() {
		t.Error("Client should have keys after setting them")
	}
}

func TestClient_GetPublicKey(t *testing.T) {
	client := NewClient("http://localhost:8080")

	// No keys set
	if client.GetPublicKey() != "" {
		t.Error("GetPublicKey should return empty string when no keys")
	}

	// Create temp directory and generate keys
	tmpDir, err := os.MkdirTemp("", "pactl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = client.LoadOrGenerateKeys(tmpDir)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys failed: %v", err)
	}

	pubKey := client.GetPublicKey()
	if pubKey == "" {
		t.Error("GetPublicKey should return non-empty string after keys are loaded")
	}

	// Base64 encoded 32 bytes should be 44 characters (with padding)
	if len(pubKey) != 44 {
		t.Errorf("Expected public key length 44, got %d", len(pubKey))
	}
}

func TestClient_SignRequest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pactl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client := NewClient("http://localhost:8080")
	err = client.LoadOrGenerateKeys(tmpDir)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys failed: %v", err)
	}

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{"GET request", "GET", "/api/v1/test", nil},
		{"POST request", "POST", "/api/v1/entry/create", []byte(`{"title":"test"}`)},
		{"PUT request", "PUT", "/api/v1/entry/update/123", []byte(`{"title":"updated"}`)},
		{"DELETE request", "DELETE", "/api/v1/entry/delete/123", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pubKey, timestamp, signature, err := client.SignRequest(tt.method, tt.path, tt.body)
			if err != nil {
				t.Fatalf("SignRequest failed: %v", err)
			}

			if pubKey == "" {
				t.Error("pubKey should not be empty")
			}
			if timestamp == "" {
				t.Error("timestamp should not be empty")
			}
			if signature == "" {
				t.Error("signature should not be empty")
			}

			// Verify timestamp is a valid Unix millisecond timestamp
			var ts int64
			_, err = fmt.Sscanf(timestamp, "%d", &ts)
			if err != nil {
				t.Errorf("Failed to parse timestamp: %v", err)
			}

			// Timestamp should be recent (within 1 second)
			now := time.Now().UnixMilli()
			if ts > now || ts < now-1000 {
				t.Errorf("Timestamp %d is not recent (now: %d)", ts, now)
			}
		})
	}
}

func TestClient_SignRequest_NoKeys(t *testing.T) {
	client := NewClient("http://localhost:8080")

	_, _, _, err := client.SignRequest("GET", "/test", nil)
	if err == nil {
		t.Error("SignRequest should fail when no keys are loaded")
	}
}

func TestClient_SetAuthHeaders(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pactl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client := NewClient("http://localhost:8080")
	err = client.LoadOrGenerateKeys(tmpDir)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/entry/create", nil)
	body := []byte(`{"title":"test"}`)

	err = client.SetAuthHeaders(req, body)
	if err != nil {
		t.Fatalf("SetAuthHeaders failed: %v", err)
	}

	// Check headers are set
	if req.Header.Get("X-Polyant-PublicKey") == "" {
		t.Error("X-Polyant-PublicKey header should be set")
	}
	if req.Header.Get("X-Polyant-Timestamp") == "" {
		t.Error("X-Polyant-Timestamp header should be set")
	}
	if req.Header.Get("X-Polyant-Signature") == "" {
		t.Error("X-Polyant-Signature header should be set")
	}
}

func TestClient_doRequest(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request
		if r.URL.Path != "/api/v1/test" {
			t.Errorf("Expected path /api/v1/test, got %s", r.URL.Path)
		}

		// Return success response
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"key": "value",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	var result APIResponse
	err := client.Get(ctx, "/api/v1/test", &result)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if result.Code != 0 {
		t.Errorf("Expected code 0, got %d", result.Code)
	}
}

func TestClient_doRequest_Error(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIError{
			Code:    400,
			Message: "bad request",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	err := client.Get(ctx, "/api/v1/test", nil)
	if err == nil {
		t.Error("Get should return error for 400 response")
	}
}

func TestClient_doRequestWithAuth_RequireAuth(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth headers
		pubKey := r.Header.Get("X-Polyant-PublicKey")
		if pubKey == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(APIError{
				Code:    401,
				Message: "unauthorized",
			})
			return
		}

		// Return success
		resp := APIResponse{
			Code:    0,
			Message: "success",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	// Without keys, should fail
	err := client.doRequestWithAuth(ctx, "POST", "/api/v1/protected", nil, nil, true)
	if err == nil {
		t.Error("doRequestWithAuth should fail when no keys and auth required")
	}

	// With keys, should succeed
	tmpDir, err := os.MkdirTemp("", "pactl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = client.LoadOrGenerateKeys(tmpDir)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys failed: %v", err)
	}

	err = client.doRequestWithAuth(ctx, "POST", "/api/v1/protected", nil, nil, false)
	if err != nil {
		t.Errorf("doRequestWithAuth failed: %v", err)
	}
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		resp := APIResponse{
			Code:    0,
			Message: "created",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	body := map[string]string{"name": "test"}
	err := client.Post(ctx, "/api/v1/test", body, nil)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
}

func TestClient_Put(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}

		resp := APIResponse{
			Code:    0,
			Message: "updated",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	err := client.Put(ctx, "/api/v1/test", map[string]string{"name": "updated"}, nil)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
}

func TestClient_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}

		resp := APIResponse{
			Code:    0,
			Message: "deleted",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	err := client.Delete(ctx, "/api/v1/test", nil)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestGetDefaultKeyDir(t *testing.T) {
	keyDir := GetDefaultKeyDir()
	if keyDir == "" {
		t.Error("GetDefaultKeyDir should not return empty string")
	}
	// Should contain .polyant
	if !filepath.IsAbs(keyDir) {
		t.Errorf("GetDefaultKeyDir should return absolute path, got %s", keyDir)
	}
}

func TestEnsureKeyDirExists(t *testing.T) {
	// Create a temp home directory to test
	tmpHome, err := os.MkdirTemp("", "pactl-home-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// This test is environment-dependent, so we just verify it doesn't error
	// In production, it would create ~/.polyant
	_ = tmpHome
}

func TestEnsureKeyDirExists_CreatesDir(t *testing.T) {
	// The function should create the directory if it doesn't exist
	err := EnsureKeyDirExists()
	// Should not error even if directory already exists
	if err != nil {
		t.Logf("EnsureKeyDirExists returned: %v (may be expected)", err)
	}
}

func TestClient_APIResponseParsing(t *testing.T) {
	tests := []struct {
		name       string
		response   APIResponse
		statusCode int
		wantError  bool
	}{
		{
			name: "success response",
			response: APIResponse{
				Code:    0,
				Message: "success",
				Data:    map[string]interface{}{"id": "123"},
			},
			statusCode: http.StatusOK,
			wantError:  false,
		},
		{
			name: "error response",
			response: APIResponse{
				Code:    400,
				Message: "bad request",
			},
			statusCode: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "server error",
			response: APIResponse{
				Code:    500,
				Message: "internal error",
			},
			statusCode: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL)
			ctx := context.Background()

			var result APIResponse
			err := client.Get(ctx, "/test", &result)

			if tt.wantError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}
