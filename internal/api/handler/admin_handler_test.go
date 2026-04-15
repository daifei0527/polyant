package handler

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newTestAdminHandler(t *testing.T) (*AdminHandler, *storage.Store) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	handler := NewAdminHandler(store)
	return handler, store
}

func TestAdminHandler_BanUserHandler(t *testing.T) {
	handler, store := newTestAdminHandler(t)

	// Create target user
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	pubKeyHash := sha256.Sum256(pubKey)
	pubKeyHashStr := hex.EncodeToString(pubKeyHash[:])

	targetUser := &model.User{
		PublicKey: pubKeyB64,
		AgentName: "target-user",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusActive,
	}
	_, err := store.User.Create(context.Background(), targetUser)
	if err != nil {
		t.Fatalf("Failed to create target user: %v", err)
	}

	// Create admin user
	adminPubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	adminPubKeyB64 := base64.StdEncoding.EncodeToString(adminPubKey)

	// Test ban user with full ban type
	// URL should contain the original public key (Base64 encoded), not the hash
	body := `{"reason": "Violation of terms", "ban_type": "full"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+pubKeyB64+"/ban", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", adminPubKeyB64)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.BanUserHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["success"] != true {
		t.Error("Expected success to be true")
	}
	if data["ban_type"] != "full" {
		t.Errorf("Expected ban_type 'full', got '%s'", data["ban_type"])
	}

	// Verify user is banned
	bannedUser, _ := store.User.Get(context.Background(), pubKeyHashStr)
	if bannedUser.Status != model.UserStatusBanned {
		t.Error("Expected user status to be banned")
	}
	if bannedUser.BanType != model.BanTypeFull {
		t.Errorf("Expected ban_type full, got %s", bannedUser.BanType)
	}
}

func TestAdminHandler_BanUserHandler_ReadonlyBan(t *testing.T) {
	handler, store := newTestAdminHandler(t)

	// Create target user
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	pubKeyHash := sha256.Sum256(pubKey)
	pubKeyHashStr := hex.EncodeToString(pubKeyHash[:])

	targetUser := &model.User{
		PublicKey: pubKeyB64,
		AgentName: "target-user",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusActive,
	}
	store.User.Create(context.Background(), targetUser)

	adminPubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	adminPubKeyB64 := base64.StdEncoding.EncodeToString(adminPubKey)

	// Test ban user with readonly ban type
	// URL should contain the original public key (Base64 encoded), not the hash
	body := `{"reason": "Readonly mode", "ban_type": "readonly"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+pubKeyB64+"/ban", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", adminPubKeyB64)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.BanUserHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Verify user has readonly ban
	bannedUser, _ := store.User.Get(context.Background(), pubKeyHashStr)
	if bannedUser.BanType != model.BanTypeReadonly {
		t.Errorf("Expected ban_type readonly, got %s", bannedUser.BanType)
	}
}

func TestAdminHandler_BanUserHandler_DefaultBanType(t *testing.T) {
	handler, store := newTestAdminHandler(t)

	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	pubKeyHash := sha256.Sum256(pubKey)
	pubKeyHashStr := hex.EncodeToString(pubKeyHash[:])

	targetUser := &model.User{
		PublicKey: pubKeyB64,
		AgentName: "target-user",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusActive,
	}
	store.User.Create(context.Background(), targetUser)

	adminPubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	adminPubKeyB64 := base64.StdEncoding.EncodeToString(adminPubKey)

	// Test without specifying ban_type (should default to full)
	// URL should contain the original public key (Base64 encoded)
	body := `{"reason": "No ban type specified"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+pubKeyB64+"/ban", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", adminPubKeyB64)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.BanUserHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	bannedUser, _ := store.User.Get(context.Background(), pubKeyHashStr)
	if bannedUser.BanType != model.BanTypeFull {
		t.Errorf("Expected default ban_type full, got %s", bannedUser.BanType)
	}
}

func TestAdminHandler_BanUserHandler_InvalidBanType(t *testing.T) {
	handler, _ := newTestAdminHandler(t)

	adminPubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	adminPubKeyB64 := base64.StdEncoding.EncodeToString(adminPubKey)

	body := `{"reason": "Test", "ban_type": "invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/somekey/ban", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", adminPubKeyB64)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.BanUserHandler(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected error for invalid ban_type")
	}
}

func TestAdminHandler_BanUserHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestAdminHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/somekey/ban", nil)
	rec := httptest.NewRecorder()

	handler.BanUserHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestAdminHandler_UnbanUserHandler(t *testing.T) {
	handler, store := newTestAdminHandler(t)

	// Create banned user
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	pubKeyHash := sha256.Sum256(pubKey)
	pubKeyHashStr := hex.EncodeToString(pubKeyHash[:])

	targetUser := &model.User{
		PublicKey: pubKeyB64,
		AgentName: "banned-user",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusBanned,
		BanType:   model.BanTypeFull,
	}
	store.User.Create(context.Background(), targetUser)

	adminPubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	adminPubKeyB64 := base64.StdEncoding.EncodeToString(adminPubKey)

	// URL should contain the original public key (Base64 encoded)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+pubKeyB64+"/unban", nil)
	ctx := context.WithValue(req.Context(), "public_key", adminPubKeyB64)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.UnbanUserHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Verify user is unbanned
	unbannedUser, _ := store.User.Get(context.Background(), pubKeyHashStr)
	if unbannedUser.Status != model.UserStatusActive {
		t.Errorf("Expected user status to be active, got %s", unbannedUser.Status)
	}
	if unbannedUser.BanType != "" {
		t.Errorf("Expected ban_type to be empty, got %s", unbannedUser.BanType)
	}
}

func TestAdminHandler_SetUserLevelHandler(t *testing.T) {
	handler, store := newTestAdminHandler(t)

	// Create target user
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	pubKeyHash := sha256.Sum256(pubKey)
	pubKeyHashStr := hex.EncodeToString(pubKeyHash[:])

	targetUser := &model.User{
		PublicKey: pubKeyB64,
		AgentName: "target-user",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusActive,
	}
	store.User.Create(context.Background(), targetUser)

	adminPubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	adminPubKeyB64 := base64.StdEncoding.EncodeToString(adminPubKey)

	// URL should contain the original public key (Base64 encoded)
	body := `{"level": 3, "reason": "Promoted for contributions"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+pubKeyB64+"/level", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", adminPubKeyB64)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.SetUserLevelHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Verify level was updated
	updatedUser, _ := store.User.Get(context.Background(), pubKeyHashStr)
	if updatedUser.UserLevel != model.UserLevelLv3 {
		t.Errorf("Expected user level Lv3, got Lv%d", updatedUser.UserLevel)
	}
}

func TestAdminHandler_SetUserLevelHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestAdminHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/somekey/level", nil)
	rec := httptest.NewRecorder()

	handler.SetUserLevelHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestAdminHandler_GetUserStatsHandler(t *testing.T) {
	handler, store := newTestAdminHandler(t)

	// Create some test users with different levels
	levels := []int32{model.UserLevelLv0, model.UserLevelLv1, model.UserLevelLv2, model.UserLevelLv3, model.UserLevelLv4}
	for i := 0; i < 5; i++ {
		pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
		pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

		user := &model.User{
			PublicKey: pubKeyB64,
			AgentName: "user-" + string(rune('a'+i)),
			UserLevel: levels[i],
			Status:    model.UserStatusActive,
		}
		store.User.Create(context.Background(), user)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/users", nil)
	rec := httptest.NewRecorder()

	handler.GetUserStatsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	// Use the correct JSON field name (camelCase)
	if data["totalUsers"] == nil {
		t.Error("Expected totalUsers in response")
	}
}

func TestAdminHandler_GetUserStatsHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestAdminHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/stats/users", nil)
	rec := httptest.NewRecorder()

	handler.GetUserStatsHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestAdminHandler_ListUsersHandler(t *testing.T) {
	handler, store := newTestAdminHandler(t)

	// Create test users
	for i := 0; i < 10; i++ {
		pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
		pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

		user := &model.User{
			PublicKey: pubKeyB64,
			AgentName: "user-" + string(rune('a'+i)),
			UserLevel: model.UserLevelLv1,
			Status:    model.UserStatusActive,
		}
		store.User.Create(context.Background(), user)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1&limit=5", nil)
	rec := httptest.NewRecorder()

	handler.ListUsersHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	users, ok := data["users"].([]interface{})
	if !ok {
		t.Fatal("Users is not an array")
	}

	if len(users) > 5 {
		t.Errorf("Expected at most 5 users, got %d", len(users))
	}

	total := data["total"].(float64)
	if total < 10 {
		t.Errorf("Expected total >= 10, got %d", int(total))
	}
}

func TestAdminHandler_ListUsersHandler_WithFilters(t *testing.T) {
	handler, store := newTestAdminHandler(t)

	// Create users with different levels
	levels := []int32{model.UserLevelLv0, model.UserLevelLv1, model.UserLevelLv2, model.UserLevelLv0, model.UserLevelLv1}
	for i := 0; i < 5; i++ {
		pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
		pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

		user := &model.User{
			PublicKey: pubKeyB64,
			AgentName: "user-" + string(rune('a'+i)),
			UserLevel: levels[i % 3], // Lv0, Lv1, Lv2
			Status:    model.UserStatusActive,
		}
		store.User.Create(context.Background(), user)
	}

	// Filter by level
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?level=1", nil)
	rec := httptest.NewRecorder()

	handler.ListUsersHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_ListUsersHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestAdminHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	handler.ListUsersHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestExtractAdminPathParam(t *testing.T) {
	tests := []struct {
		path     string
		prefix   string
		suffix   string
		expected string
	}{
		{"/api/v1/admin/users/abc123/ban", "/api/v1/admin/users/", "/ban", "abc123"},
		{"/api/v1/admin/users/xyz789/unban", "/api/v1/admin/users/", "/unban", "xyz789"},
		{"/api/v1/admin/users/testkey/level", "/api/v1/admin/users/", "/level", "testkey"},
		{"/api/v1/admin/users//ban", "/api/v1/admin/users/", "/ban", ""},
		{"/short", "/api/v1/admin/users/", "/ban", ""},
	}

	for _, tt := range tests {
		result := extractAdminPathParam(tt.path, tt.prefix, tt.suffix)
		if result != tt.expected {
			t.Errorf("extractAdminPathParam(%q, %q, %q) = %q, expected %q",
				tt.path, tt.prefix, tt.suffix, result, tt.expected)
		}
	}
}
