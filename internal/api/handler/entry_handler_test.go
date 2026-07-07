package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// ========== Trust-based entry creation (R4b Task 2) ==========

// TestCreateEntry_LowLevelUserEntersReview: Lv1 creator → entry gets status "review"
// and must NOT appear in the search index.
func TestCreateEntry_LowLevelUserEntersReview(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	user, priv := createTestUserWithKey(t, memStore, "lv1-author", model.UserLevelLv1)
	sig := signContentB64(t, priv, "ReviewTitle", "ReviewContent", "test")
	body := `{"title":"ReviewTitle","content":"ReviewContent","category":"test","creator_signature":"` + sig + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()

	h.CreateEntryHandler(rec, req)

	require.Equal(t, http.StatusCreated, rec.Result().StatusCode, "create must succeed for Lv1 user")

	// Verify entry status is "review"
	entryID := extractIDFromCreateResp(t, rec.Body.Bytes())
	stored, err := memStore.Entry.Get(context.Background(), entryID)
	require.NoError(t, err)
	if stored.Status != model.EntryStatusReview {
		t.Fatalf("lv1 create: want status %q, got %q", model.EntryStatusReview, stored.Status)
	}

	// Review entries must NOT be in the search index
	res, _ := memStore.Search.Search(context.Background(), index.SearchQuery{Keyword: "ReviewTitle", Limit: 10})
	if res != nil && res.TotalCount > 0 {
		t.Errorf("review entry leaked into search index: total=%d", res.TotalCount)
	}
}

// TestCreateEntry_TrustedUserPublishes: Lv3+ creator → entry gets status "published".
func TestCreateEntry_TrustedUserPublishes(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	user, priv := createTestUserWithKey(t, memStore, "lv3-author", model.UserLevelLv3)
	sig := signContentB64(t, priv, "PubTitle", "PubContent", "test")
	body := `{"title":"PubTitle","content":"PubContent","category":"test","creator_signature":"` + sig + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()

	h.CreateEntryHandler(rec, req)

	require.Equal(t, http.StatusCreated, rec.Result().StatusCode, "create must succeed for Lv3 user")

	// Verify entry status is "published"
	entryID := extractIDFromCreateResp(t, rec.Body.Bytes())
	stored, err := memStore.Entry.Get(context.Background(), entryID)
	require.NoError(t, err)
	if stored.Status != model.EntryStatusPublished {
		t.Fatalf("lv3 create: want status %q, got %q", model.EntryStatusPublished, stored.Status)
	}
}

// ========== Search Integration Tests (Graph + Keyword Indexing) ==========

// TestSearchHandler_WithGraph is an end-to-end test: create cross-referencing entries,
// search, and verify the response contains a graph with correct nodes and edges.
func TestSearchHandler_WithGraph(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	ctx := context.Background()

	// Create entry A: 神经网络
	entryA := &model.KnowledgeEntry{
		ID:       "a1",
		Title:    "神经网络",
		Content:  "神经网络是一种计算模型。",
		Category: "AI",
		Status:   model.EntryStatusPublished,
	}
	_, err = memStore.Entry.Create(ctx, entryA)
	require.NoError(t, err)
	memStore.TitleIdx.Add(index.TitleEntry{ID: "a1", Title: "神经网络"})

	// Create entry B: 深度学习, content references 神经网络
	entryB := &model.KnowledgeEntry{
		ID:       "b1",
		Title:    "深度学习",
		Content:  "深度学习基于神经网络技术。",
		Category: "AI",
		Status:   model.EntryStatusPublished,
	}
	_, err = memStore.Entry.Create(ctx, entryB)
	require.NoError(t, err)
	memStore.Search.IndexEntry(entryB)
	memStore.TitleIdx.Add(index.TitleEntry{ID: "b1", Title: "深度学习"})

	handler := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=深度学习", nil)
	w := httptest.NewRecorder()
	handler.SearchHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var apiResp APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)

	pagedData, ok := apiResp.Data.(map[string]interface{})
	require.True(t, ok, "data should be a map")

	// Verify graph exists and has correct structure
	graph, ok := pagedData["graph"]
	require.True(t, ok, "graph should exist")
	require.NotNil(t, graph)

	graphMap := graph.(map[string]interface{})
	nodes, _ := graphMap["nodes"].([]interface{})
	edges, _ := graphMap["edges"].([]interface{})

	// Should have at least 2 nodes: result (b1: 深度学习) + reference (a1: 神经网络)
	assert.True(t, len(nodes) >= 2, "expected at least 2 nodes, got %d", len(nodes))
	assert.True(t, len(edges) >= 1, "expected at least 1 edge, got %d", len(edges))

	// Verify content has markdown link
	items, _ := pagedData["items"].([]interface{})
	require.NotEmpty(t, items, "items should not be empty")
	entryMap := items[0].(map[string]interface{})
	content := entryMap["content"].(string)
	assert.Contains(t, content, "[神经网络](entry://a1)")
}

// TestSearchHandler_NoGraphWhenEmpty verifies that when no external references exist,
// the graph still has the result node but no edges.
func TestSearchHandler_NoGraphWhenEmpty(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	ctx := context.Background()

	// Create entry that mentions nothing from the index
	entry := &model.KnowledgeEntry{
		ID:       "x1",
		Title:    "量子计算",
		Content:  "量子计算使用量子比特进行运算。",
		Category: "Physics",
		Status:   model.EntryStatusPublished,
	}
	_, err = memStore.Entry.Create(ctx, entry)
	require.NoError(t, err)
	memStore.TitleIdx.Add(index.TitleEntry{ID: "x1", Title: "量子计算"})
	memStore.Search.IndexEntry(entry)

	handler := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=量子计算", nil)
	w := httptest.NewRecorder()
	handler.SearchHandler(w, req)

	var apiResp APIResponse
	err = json.NewDecoder(w.Result().Body).Decode(&apiResp)
	require.NoError(t, err)

	pagedData := apiResp.Data.(map[string]interface{})
	graph := pagedData["graph"]
	require.NotNil(t, graph, "graph should exist even without external references")

	graphMap := graph.(map[string]interface{})
	nodes, _ := graphMap["nodes"].([]interface{})
	edges, _ := graphMap["edges"].([]interface{})

	// Should have 1 result node and 0 edges (self-reference filtered)
	assert.Equal(t, 1, len(nodes))
	assert.Equal(t, 0, len(edges))
}

// ========== EntryPusher wiring (P1.5) ==========

// fakeEntryPusher records pushed entries via a buffered channel (race-free).
type fakeEntryPusher struct {
	pushed chan *model.KnowledgeEntry
}

func (f *fakeEntryPusher) PushEntry(entry *model.KnowledgeEntry, signature []byte) error {
	f.pushed <- entry
	return nil
}

// TestCreateEntryHandler_IndexFailureDoesNotBlock: 搜索索引写入失败应仅记日志，
// 不得阻塞条目创建或改变 HTTP 状态码（best-effort 索引，R2-C2）。
func TestCreateEntryHandler_IndexFailureDoesNotBlock(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)
	mock := &mockSearchEngine{indexErr: assertErrIndexFail}
	h.searchEngine = mock

	user, priv := createTestUserWithKey(t, memStore, "author", model.UserLevelLv3) // Lv3 → published → indexing attempted
	sig := signContentB64(t, priv, "T", "C", "cat")
	body := `{"title":"T","content":"C","category":"cat","creator_signature":"` + sig + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()

	h.CreateEntryHandler(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Result().StatusCode, "entry create must succeed despite index error")
	assert.Greater(t, mock.indexCalls, 0, "IndexEntry must still be attempted")
}

// TestCreateEntryHandler_PushesToSeed: creating an entry must push it to the
// configured EntryPusher (the dormant push half-flow is now wired).
func TestCreateEntryHandler_PushesToSeed(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)
	pusher := &fakeEntryPusher{pushed: make(chan *model.KnowledgeEntry, 1)}
	h.SetEntryPusher(pusher)

	user, priv := createTestUserWithKey(t, memStore, "author", model.UserLevelLv1)
	sig := signContentB64(t, priv, "T", "C", "cat")
	body := `{"title":"T","content":"C","category":"cat","creator_signature":"` + sig + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()

	h.CreateEntryHandler(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Result().StatusCode)

	select {
	case got := <-pusher.pushed:
		assert.Equal(t, "T", got.Title)
	case <-time.After(2 * time.Second):
		t.Fatal("newly created entry was not pushed to seed within timeout")
	}
}

// TestGetEntryHandler_LangProjection: /entry/{id}?lang= 返回本地化 Title/Content。
func TestGetEntryHandler_LangProjection(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	entry := &model.KnowledgeEntry{
		ID:          "e-i18n",
		Title:       "Hello",
		Content:     "World",
		Category:    "test",
		Status:      model.EntryStatusPublished,
		TitleI18n:   map[string]string{"zh-CN": "你好"},
		ContentI18n: map[string]string{"zh-CN": "世界"},
	}
	_, err = memStore.Entry.Create(context.Background(), entry)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entry/e-i18n?lang=zh-CN", nil)
	rec := httptest.NewRecorder()
	h.GetEntryHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "你好", data["title"])
	assert.Equal(t, "世界", data["content"])
}
