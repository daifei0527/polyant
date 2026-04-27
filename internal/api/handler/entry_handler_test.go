package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
)

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
