package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// ========== Batch Handler Helper ==========

func newTestBatchHandler(t *testing.T) (*BatchHandler, *storage.Store) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	handler := NewBatchHandler(store.Entry, store.Search, store.Backlink, store.User)
	return handler, store
}

// ========== Batch Create Tests ==========

func TestBatchCreateHandler_Success(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	// Create test user
	user := &model.User{
		PublicKey: "test-pk",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusActive,
	}
	store.User.Create(context.Background(), user)

	// Create request with 2 entries
	entries := []BatchEntry{
		{Title: "Entry 1", Content: "Content 1", Category: "test"},
		{Title: "Entry 2", Content: "Content 2", Category: "test"},
	}
	reqBody := BatchCreateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchCreateHandler(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var resp BatchResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if !resp.Success {
		t.Error("Expected success")
	}
	if resp.Summary.Created != 2 {
		t.Errorf("Expected 2 created, got %d", resp.Summary.Created)
	}
	if len(resp.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(resp.Results))
	}
}

func TestBatchCreateHandler_TooManyEntries(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Create 101 entries (exceeds limit)
	entries := make([]BatchEntry, 101)
	for i := range entries {
		entries[i] = BatchEntry{Title: "Entry", Content: "Content", Category: "test"}
	}
	reqBody := BatchCreateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchCreateHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBatchCreateHandler_ValidationFailure(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Entries with missing required fields
	entries := []BatchEntry{
		{Title: "Valid", Content: "Content", Category: "test"},
		{Title: "", Content: "Content", Category: "test"},        // Missing title
		{Title: "No Content", Content: "", Category: "test"},     // Missing content
		{Title: "No Category", Content: "Content", Category: ""}, // Missing category
	}
	reqBody := BatchCreateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchCreateHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var resp BatchResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp.Success {
		t.Error("Expected failure")
	}
	if len(resp.Errors) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(resp.Errors))
	}
}

func TestBatchCreateHandler_Unauthorized(t *testing.T) {
	handler, _ := newTestBatchHandler(t)

	entries := []BatchEntry{{Title: "Test", Content: "Content", Category: "test"}}
	reqBody := BatchCreateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.BatchCreateHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBatchCreateHandler_InsufficientPermission(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	// Lv0 user cannot create entries
	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv0, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	entries := []BatchEntry{{Title: "Test", Content: "Content", Category: "test"}}
	reqBody := BatchCreateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchCreateHandler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

// ========== Batch Update Tests ==========

func TestBatchUpdateHandler_Success(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Create entries owned by user
	entry1 := &model.KnowledgeEntry{ID: "id1", Title: "Old 1", Content: "Content", Category: "test", CreatedBy: "test-pk", Version: 1}
	entry2 := &model.KnowledgeEntry{ID: "id2", Title: "Old 2", Content: "Content", Category: "test", CreatedBy: "test-pk", Version: 1}
	store.Entry.Create(context.Background(), entry1)
	store.Entry.Create(context.Background(), entry2)

	// Update request
	newTitle := "Updated Title"
	entries := []BatchUpdateEntry{
		{ID: "id1", Title: &newTitle},
		{ID: "id2", Title: &newTitle},
	}
	reqBody := BatchUpdateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchUpdateHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp BatchResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if !resp.Success {
		t.Error("Expected success")
	}
	if resp.Summary.Updated != 2 {
		t.Errorf("Expected 2 updated, got %d", resp.Summary.Updated)
	}
}

func TestBatchUpdateHandler_NotOwner(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Entry owned by different user
	entry := &model.KnowledgeEntry{ID: "id1", Title: "Test", Content: "Content", Category: "test", CreatedBy: "other-user"}
	store.Entry.Create(context.Background(), entry)

	newTitle := "Updated"
	entries := []BatchUpdateEntry{{ID: "id1", Title: &newTitle}}
	reqBody := BatchUpdateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchUpdateHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBatchUpdateHandler_Lv3CanUpdateAny(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	// Lv3 user
	user := &model.User{PublicKey: "lv3-user", UserLevel: model.UserLevelLv3, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Entry owned by different user
	entry := &model.KnowledgeEntry{ID: "id1", Title: "Test", Content: "Content", Category: "test", CreatedBy: "other-user"}
	store.Entry.Create(context.Background(), entry)

	newTitle := "Updated by Lv3"
	entries := []BatchUpdateEntry{{ID: "id1", Title: &newTitle}}
	reqBody := BatchUpdateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchUpdateHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d for Lv3 update, got %d", http.StatusOK, rr.Code)
	}
}

// ========== Batch Delete Tests ==========

func TestBatchDeleteHandler_Success(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Create entries owned by user
	entry1 := &model.KnowledgeEntry{ID: "id1", Title: "Test", Content: "Content", Category: "test", CreatedBy: "test-pk"}
	entry2 := &model.KnowledgeEntry{ID: "id2", Title: "Test", Content: "Content", Category: "test", CreatedBy: "test-pk"}
	store.Entry.Create(context.Background(), entry1)
	store.Entry.Create(context.Background(), entry2)

	reqBody := BatchDeleteRequest{IDs: []string{"id1", "id2"}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchDeleteHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp BatchResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if !resp.Success {
		t.Error("Expected success")
	}
	if resp.Summary.Deleted != 2 {
		t.Errorf("Expected 2 deleted, got %d", resp.Summary.Deleted)
	}
}

func TestBatchDeleteHandler_NotOwner(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Entry owned by different user
	entry := &model.KnowledgeEntry{ID: "id1", Title: "Test", Content: "Content", Category: "test", CreatedBy: "other-user"}
	store.Entry.Create(context.Background(), entry)

	reqBody := BatchDeleteRequest{IDs: []string{"id1"}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchDeleteHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBatchDeleteHandler_Lv4CanDeleteAny(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	// Lv4 user
	user := &model.User{PublicKey: "lv4-user", UserLevel: model.UserLevelLv4, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Entry owned by different user
	entry := &model.KnowledgeEntry{ID: "id1", Title: "Test", Content: "Content", Category: "test", CreatedBy: "other-user"}
	store.Entry.Create(context.Background(), entry)

	reqBody := BatchDeleteRequest{IDs: []string{"id1"}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchDeleteHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d for Lv4 delete, got %d", http.StatusOK, rr.Code)
	}
}

func TestBatchDeleteHandler_EntryNotFound(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	reqBody := BatchDeleteRequest{IDs: []string{"nonexistent"}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchDeleteHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// ========== Additional Boundary Tests ==========

func TestBatchCreateHandler_EmptyEntries(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Empty entries list
	reqBody := BatchCreateRequest{Entries: []BatchEntry{}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchCreateHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for empty entries, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBatchUpdateHandler_TooManyEntries(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Create 101 update entries (exceeds limit)
	entries := make([]BatchUpdateEntry, 101)
	newTitle := "Updated"
	for i := range entries {
		entries[i] = BatchUpdateEntry{ID: "id", Title: &newTitle}
	}
	reqBody := BatchUpdateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchUpdateHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for too many entries, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBatchUpdateHandler_EmptyEntries(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Empty entries list
	reqBody := BatchUpdateRequest{Entries: []BatchUpdateEntry{}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchUpdateHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for empty entries, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBatchDeleteHandler_TooManyEntries(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Create 101 IDs (exceeds limit)
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "id"
	}
	reqBody := BatchDeleteRequest{IDs: ids}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchDeleteHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for too many IDs, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBatchDeleteHandler_EmptyIDs(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Empty IDs list
	reqBody := BatchDeleteRequest{IDs: []string{}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchDeleteHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for empty IDs, got %d", http.StatusBadRequest, rr.Code)
	}
}

// ========== Permission Tests ==========

func TestBatchUpdateHandler_Unauthorized(t *testing.T) {
	handler, _ := newTestBatchHandler(t)

	newTitle := "Updated"
	entries := []BatchUpdateEntry{{ID: "id1", Title: &newTitle}}
	reqBody := BatchUpdateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/entries/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.BatchUpdateHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBatchUpdateHandler_InsufficientPermission(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	// Lv0 user cannot update entries
	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv0, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	newTitle := "Updated"
	entries := []BatchUpdateEntry{{ID: "id1", Title: &newTitle}}
	reqBody := BatchUpdateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchUpdateHandler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestBatchUpdateHandler_EntryNotFound(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	// Update non-existent entry
	newTitle := "Updated"
	entries := []BatchUpdateEntry{{ID: "nonexistent-id", Title: &newTitle}}
	reqBody := BatchUpdateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchUpdateHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var resp BatchResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Success {
		t.Error("Expected failure")
	}
}

func TestBatchDeleteHandler_Unauthorized(t *testing.T) {
	handler, _ := newTestBatchHandler(t)

	reqBody := BatchDeleteRequest{IDs: []string{"id1"}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entries/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.BatchDeleteHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBatchDeleteHandler_InsufficientPermission(t *testing.T) {
	handler, store := newTestBatchHandler(t)

	// Lv0 user cannot delete entries
	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv0, Status: model.UserStatusActive}
	store.User.Create(context.Background(), user)

	reqBody := BatchDeleteRequest{IDs: []string{"id1"}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchDeleteHandler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}
