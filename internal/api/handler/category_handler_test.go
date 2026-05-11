package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newTestCategoryHandler(t *testing.T) (*CategoryHandler, *storage.Store) {
	t.Helper()
	store, err := storage.NewMemoryStore()
	require.NoError(t, err)
	handler := NewCategoryHandler(store.Category, store.Entry)
	return handler, store
}

func decodeAPIResponse(t *testing.T, rec *httptest.ResponseRecorder) APIResponse {
	t.Helper()
	var resp APIResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	return resp
}

// ========== ListCategoriesHandler ==========

func TestCategoryHandler_ListCategoriesHandler(t *testing.T) {
	handler, store := newTestCategoryHandler(t)
	ctx := context.Background()

	// Create parent and child categories
	// Note: buildCategoryTree looks up ParentId in a map keyed by Path,
	// so ParentId must be the parent's Path, not its ID.
	parent := &model.Category{
		ID:    "cat-prog",
		Path:  "programming",
		Name:  "Programming",
		Level: 0,
	}
	child := &model.Category{
		ID:       "cat-go",
		Path:     "programming/go",
		Name:     "Go",
		ParentId: "programming",
		Level:    1,
	}
	_, err := store.Category.Create(ctx, parent)
	require.NoError(t, err)
	_, err = store.Category.Create(ctx, child)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rec := httptest.NewRecorder()

	handler.ListCategoriesHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	resp := decodeAPIResponse(t, rec)
	assert.Equal(t, 0, resp.Code)

	// Response data should be an array of tree nodes
	data, ok := resp.Data.([]interface{})
	require.True(t, ok, "data should be an array")
	assert.Len(t, data, 1, "should have 1 root node")

	// Verify the root node
	root := data[0].(map[string]interface{})
	assert.Equal(t, "programming", root["path"])
	assert.Equal(t, "Programming", root["name"])

	// Verify the child node
	children := root["children"].([]interface{})
	require.Len(t, children, 1)
	childNode := children[0].(map[string]interface{})
	assert.Equal(t, "programming/go", childNode["path"])
	assert.Equal(t, "Go", childNode["name"])
}

func TestCategoryHandler_ListCategoriesHandler_Empty(t *testing.T) {
	handler, _ := newTestCategoryHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rec := httptest.NewRecorder()

	handler.ListCategoriesHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	resp := decodeAPIResponse(t, rec)
	assert.Equal(t, 0, resp.Code)

	// When no categories exist, buildCategoryTree returns nil which serializes to JSON null
	// The response data is either null or an empty array
	if resp.Data != nil {
		data, ok := resp.Data.([]interface{})
		require.True(t, ok, "data should be an array")
		assert.Empty(t, data, "data should be empty when no categories exist")
	}
	// nil is also acceptable (JSON null)
}

// ========== GetCategoryEntriesHandler ==========

func TestCategoryHandler_GetCategoryEntriesHandler(t *testing.T) {
	handler, store := newTestCategoryHandler(t)
	ctx := context.Background()

	// Create category
	cat := &model.Category{
		ID:    "cat-ai",
		Path:  "ai",
		Name:  "AI",
		Level: 0,
	}
	_, err := store.Category.Create(ctx, cat)
	require.NoError(t, err)

	// Create entries in the category
	for i := 0; i < 25; i++ {
		entry := &model.KnowledgeEntry{
			ID:       fmt.Sprintf("entry-%d", i),
			Title:    fmt.Sprintf("Entry %d", i),
			Content:  "Content",
			Category: "ai",
			Status:   model.EntryStatusPublished,
			Score:    float64(i),
		}
		_, err := store.Entry.Create(ctx, entry)
		require.NoError(t, err)
	}

	// First page with limit=10
	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories/ai/entries?limit=10&offset=0", nil)
	rec := httptest.NewRecorder()

	handler.GetCategoryEntriesHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	resp := decodeAPIResponse(t, rec)
	assert.Equal(t, 0, resp.Code)

	pagedData, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "data should be a paged data object")

	assert.Equal(t, float64(25), pagedData["total_count"])
	assert.Equal(t, true, pagedData["has_more"])

	items, ok := pagedData["items"].([]interface{})
	require.True(t, ok, "items should be an array")
	assert.Len(t, items, 10, "first page should have 10 items")

	// Second page with offset=10
	req = httptest.NewRequest(http.MethodGet, "/api/v1/categories/ai/entries?limit=10&offset=10", nil)
	rec = httptest.NewRecorder()

	handler.GetCategoryEntriesHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	resp = decodeAPIResponse(t, rec)
	pagedData = resp.Data.(map[string]interface{})

	items = pagedData["items"].([]interface{})
	assert.Len(t, items, 10, "second page should have 10 items")

	// Third page with offset=20
	req = httptest.NewRequest(http.MethodGet, "/api/v1/categories/ai/entries?limit=10&offset=20", nil)
	rec = httptest.NewRecorder()

	handler.GetCategoryEntriesHandler(rec, req)

	resp = decodeAPIResponse(t, rec)
	pagedData = resp.Data.(map[string]interface{})

	items = pagedData["items"].([]interface{})
	assert.Len(t, items, 5, "third page should have 5 items")
	assert.Equal(t, false, pagedData["has_more"])
}

func TestCategoryHandler_GetCategoryEntriesHandler_NotFound(t *testing.T) {
	handler, _ := newTestCategoryHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories/nonexistent/entries", nil)
	rec := httptest.NewRecorder()

	handler.GetCategoryEntriesHandler(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	resp := decodeAPIResponse(t, rec)
	assert.NotEqual(t, 0, resp.Code)
}

func TestCategoryHandler_GetCategoryEntriesHandler_EmptyPath(t *testing.T) {
	handler, _ := newTestCategoryHandler(t)

	// URL without a proper path segment between categories and entries
	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories//entries", nil)
	rec := httptest.NewRecorder()

	handler.GetCategoryEntriesHandler(rec, req)

	// Empty path returns 400 (invalid params)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	resp := decodeAPIResponse(t, rec)
	assert.NotEqual(t, 0, resp.Code)
}

// ========== CreateCategoryHandler ==========

func TestCategoryHandler_CreateCategoryHandler_NoAuth(t *testing.T) {
	handler, _ := newTestCategoryHandler(t)

	body := `{"path": "tech", "name": "Technology"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/categories/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.CreateCategoryHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	resp := decodeAPIResponse(t, rec)
	assert.NotEqual(t, 0, resp.Code)
}

func TestCategoryHandler_CreateCategoryHandler_Lv0Denied(t *testing.T) {
	handler, _ := newTestCategoryHandler(t)

	user := &model.User{
		PublicKey: "test-key",
		AgentName: "basic-user",
		UserLevel: model.UserLevelLv0,
		Status:    model.UserStatusActive,
	}

	body := `{"path": "tech", "name": "Technology"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/categories/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateCategoryHandler(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	resp := decodeAPIResponse(t, rec)
	assert.NotEqual(t, 0, resp.Code)
}

func TestCategoryHandler_CreateCategoryHandler_Success(t *testing.T) {
	handler, _ := newTestCategoryHandler(t)

	user := &model.User{
		PublicKey: "test-key",
		AgentName: "contributor",
		UserLevel: model.UserLevelLv2,
		Status:    model.UserStatusActive,
	}

	body := `{"path": "tech/programming/rust", "name": "Rust"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/categories/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateCategoryHandler(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	resp := decodeAPIResponse(t, rec)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "data should be a category object")
	assert.Equal(t, "tech/programming/rust", data["path"])
	assert.Equal(t, "Rust", data["name"])
	// model.Category uses camelCase JSON tags
	assert.Equal(t, false, data["isBuiltin"])
}

func TestCategoryHandler_CreateCategoryHandler_Duplicate(t *testing.T) {
	handler, store := newTestCategoryHandler(t)
	ctx := context.Background()

	// Pre-create a category
	existing := &model.Category{
		ID:    "existing-cat",
		Path:  "tech",
		Name:  "Technology",
		Level: 0,
	}
	_, err := store.Category.Create(ctx, existing)
	require.NoError(t, err)

	user := &model.User{
		PublicKey: "test-key",
		AgentName: "contributor",
		UserLevel: model.UserLevelLv2,
		Status:    model.UserStatusActive,
	}

	body := `{"path": "tech", "name": "Technology"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/categories/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx = setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateCategoryHandler(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	resp := decodeAPIResponse(t, rec)
	assert.NotEqual(t, 0, resp.Code)
}
