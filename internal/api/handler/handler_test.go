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

	"github.com/daifei0527/agentwiki/internal/core/email"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

func newTestStore(t *testing.T) *storage.Store {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return store
}

func newTestUserHandler(t *testing.T) (*UserHandler, *storage.Store) {
	store := newTestStore(t)
	verificationMgr := email.NewVerificationManager()
	handler := NewUserHandler(store.User, store.Entry, store.Rating, nil, verificationMgr)
	return handler, store
}

func TestUserHandler_RegisterHandler(t *testing.T) {
	handler, _ := newTestUserHandler(t)

	// Test successful registration
	body := `{"agent_name": "test-agent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterHandler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["public_key"] == "" {
		t.Error("Expected public_key in response")
	}
	if data["private_key"] == "" {
		t.Error("Expected private_key in response")
	}
	if data["agent_name"] != "test-agent" {
		t.Errorf("Expected agent_name 'test-agent', got '%s'", data["agent_name"])
	}
}

func TestUserHandler_RegisterHandler_MissingFields(t *testing.T) {
	handler, _ := newTestUserHandler(t)

	// Test missing agent_name
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterHandler(rec, req)

	if rec.Code == http.StatusCreated {
		t.Error("Expected error for missing agent_name")
	}
}

func TestUserHandler_RegisterHandler_InvalidJSON(t *testing.T) {
	handler, _ := newTestUserHandler(t)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterHandler(rec, req)

	if rec.Code == http.StatusCreated {
		t.Error("Expected error for invalid JSON")
	}
}

func TestUserHandler_GetUserInfoHandler(t *testing.T) {
	handler, store := newTestUserHandler(t)

	// Create test user
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	pubKeyHash := sha256.Sum256(pubKey)
	pubKeyHashStr := hex.EncodeToString(pubKeyHash[:])

	user := &model.User{
		PublicKey:     pubKeyB64,
		AgentName:     "test-user",
		UserLevel:     model.UserLevelLv1,
		Email:         "test@example.com",
		EmailVerified: true,
		Status:        model.UserStatusActive,
	}
	_, _ = store.User.Create(context.Background(), user)

	// Test get user info
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/info", nil)
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetUserInfoHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["public_key_hash"] != pubKeyHashStr {
		t.Errorf("Expected public_key_hash '%s', got '%s'", pubKeyHashStr, data["public_key_hash"])
	}
}

func TestUserHandler_GetUserInfoHandler_Unauthorized(t *testing.T) {
	handler, _ := newTestUserHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/info", nil)
	rec := httptest.NewRecorder()

	handler.GetUserInfoHandler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestEntryHandler_CreateEntryHandler(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	// Create test user
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

	user := &model.User{
		PublicKey:     pubKeyB64,
		AgentName:     "test-user",
		UserLevel:     model.UserLevelLv1,
		EmailVerified: true,
		Status:        model.UserStatusActive,
	}

	// Test create entry
	body := `{
		"title": "Test Entry",
		"content": "This is test content for the entry.",
		"category": "test",
		"tags": ["test", "example"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateEntryHandler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["id"] == "" {
		t.Error("Expected id in response")
	}
}

func TestEntryHandler_CreateEntryHandler_Unauthorized(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	body := `{"title": "Test", "content": "Content", "category": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.CreateEntryHandler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestEntryHandler_CreateEntryHandler_InsufficientPermission(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	// Lv0 user cannot create entries
	user := &model.User{
		PublicKey:     "test-key",
		AgentName:     "test-user",
		UserLevel:     model.UserLevelLv0,
		Status:        model.UserStatusActive,
	}

	body := `{"title": "Test", "content": "Content", "category": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateEntryHandler(rec, req)

	if rec.Code == http.StatusCreated {
		t.Error("Expected error for insufficient permission")
	}
}

func TestEntryHandler_GetEntryHandler(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	// Create test entry
	entry := &model.KnowledgeEntry{
		ID:        "test-entry-1",
		Title:     "Test Entry",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
	}
	_, _ = store.Entry.Create(context.Background(), entry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entry/test-entry-1", nil)
	rec := httptest.NewRecorder()

	handler.GetEntryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["title"] != "Test Entry" {
		t.Errorf("Expected title 'Test Entry', got '%s'", data["title"])
	}
}

func TestEntryHandler_GetEntryHandler_NotFound(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entry/non-existing", nil)
	rec := httptest.NewRecorder()

	handler.GetEntryHandler(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected error for non-existing entry")
	}
}

func TestEntryHandler_SearchHandler(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	// Create test entries
	for i := 0; i < 5; i++ {
		entry := &model.KnowledgeEntry{
			ID:        string(rune('a' + i)),
			Title:     "Go Programming Tutorial",
			Content:   "Learn Go programming language",
			Category:  "programming",
			Status:    model.EntryStatusPublished,
			Score:     float64(5 - i),
		}
		_, _ = store.Entry.Create(context.Background(), entry)
		_ = store.Search.IndexEntry(entry)
	}

	// Test search
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=go&limit=3", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	items, ok := data["items"].([]interface{})
	if !ok {
		t.Fatal("Items is not an array")
	}

	if len(items) == 0 {
		t.Error("Expected search results")
	}
}

func TestEntryHandler_SearchHandler_MissingQuery(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected error for missing query")
	}
}

func TestEntryHandler_DeleteEntryHandler(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	// Create test user and entry
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

	user := &model.User{
		PublicKey:     pubKeyB64,
		AgentName:     "test-user",
		UserLevel:     model.UserLevelLv1,
		Status:        model.UserStatusActive,
	}

	entry := &model.KnowledgeEntry{
		ID:        "test-entry-1",
		Title:     "Test Entry",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
		CreatedBy: pubKeyB64,
	}
	_, _ = store.Entry.Create(context.Background(), entry)

	// Test delete
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entry/test-entry-1", nil)
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.DeleteEntryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify soft delete
	deleted, _ := store.Entry.Get(context.Background(), "test-entry-1")
	if deleted.Status != model.EntryStatusArchived {
		t.Error("Entry should be archived after delete")
	}
}

func TestCategoryHandler_ListCategoriesHandler(t *testing.T) {
	store := newTestStore(t)
	handler := NewCategoryHandler(store.Category, store.Entry)

	// Create test categories
	categories := []*model.Category{
		{ID: "cat-1", Path: "programming", Name: "编程", Level: 0},
		{ID: "cat-2", Path: "programming/go", Name: "Go", ParentId: "cat-1", Level: 1},
	}
	for _, cat := range categories {
		_, _ = store.Category.Create(context.Background(), cat)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rec := httptest.NewRecorder()

	handler.ListCategoriesHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("Response data is not an array")
	}

	if len(data) < 2 {
		t.Errorf("Expected at least 2 categories, got %d", len(data))
	}
}
