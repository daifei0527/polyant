package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClient_GetStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/status" {
			t.Errorf("Expected path /api/v1/status, got %s", r.URL.Path)
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"version":       "v1.0.0",
				"uptime_seconds": float64(3600),
				"node_id":       "node-123",
				"node_type":     "seed",
				"nat_type":      "public",
				"peer_count":    float64(5),
				"entry_count":   float64(100),
				"user_count":    float64(10),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	status, err := client.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got %s", status.Version)
	}
	if status.NodeID != "node-123" {
		t.Errorf("Expected node_id 'node-123', got %s", status.NodeID)
	}
	if status.NodeType != "seed" {
		t.Errorf("Expected node_type 'seed', got %s", status.NodeType)
	}
	if status.PeerCount != 5 {
		t.Errorf("Expected peer_count 5, got %d", status.PeerCount)
	}
	if status.EntryCount != 100 {
		t.Errorf("Expected entry_count 100, got %d", status.EntryCount)
	}
	if status.UserCount != 10 {
		t.Errorf("Expected user_count 10, got %d", status.UserCount)
	}
}

func TestClient_ListEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/entries" {
			t.Errorf("Expected path /api/v1/entries, got %s", r.URL.Path)
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"total_count": float64(2),
				"items": []interface{}{
					map[string]interface{}{
						"id":          "entry-1",
						"title":       "Test Entry 1",
						"category":    "tech",
						"score":       float64(4.5),
						"score_count": float64(10),
						"created_at":  float64(1700000000000),
						"updated_at":  float64(1700000000000),
						"created_by":  "user-1",
						"tags":        []interface{}{"go", "test"},
					},
					map[string]interface{}{
						"id":          "entry-2",
						"title":       "Test Entry 2",
						"category":    "science",
						"score":       float64(3.0),
						"score_count": float64(5),
						"created_at":  float64(1700000001000),
						"updated_at":  float64(1700000001000),
						"created_by":  "user-2",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	entries, total, err := client.ListEntries(ctx, "", 20, 0)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if total != 2 {
		t.Errorf("Expected total 2, got %d", total)
	}
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
	if entries[0].ID != "entry-1" {
		t.Errorf("Expected first entry ID 'entry-1', got %s", entries[0].ID)
	}
	if entries[0].Title != "Test Entry 1" {
		t.Errorf("Expected title 'Test Entry 1', got %s", entries[0].Title)
	}
	if len(entries[0].Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(entries[0].Tags))
	}
}

func TestClient_GetEntry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"id":          "entry-123",
				"title":       "Test Entry",
				"category":    "tech",
				"score":       float64(4.5),
				"score_count": float64(10),
				"created_at":  float64(1700000000000),
				"updated_at":  float64(1700000000000),
				"created_by":  "user-1",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	entry, err := client.GetEntry(ctx, "entry-123")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if entry.ID != "entry-123" {
		t.Errorf("Expected ID 'entry-123', got %s", entry.ID)
	}
	if entry.Title != "Test Entry" {
		t.Errorf("Expected title 'Test Entry', got %s", entry.Title)
	}
}

func TestClient_SearchEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "test" {
			t.Errorf("Expected query 'test', got %s", r.URL.Query().Get("q"))
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"total_count": float64(1),
				"items": []interface{}{
					map[string]interface{}{
						"id":          "entry-1",
						"title":       "Test Result",
						"category":    "tech",
						"score":       float64(5.0),
						"score_count": float64(1),
						"created_at":  float64(1700000000000),
						"updated_at":  float64(1700000000000),
						"created_by":  "user-1",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	entries, total, err := client.SearchEntries(ctx, "test", 10)
	if err != nil {
		t.Fatalf("SearchEntries failed: %v", err)
	}

	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
	if entries[0].Title != "Test Result" {
		t.Errorf("Expected title 'Test Result', got %s", entries[0].Title)
	}
}

func TestClient_GetUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"public_key":    "test-pubkey",
				"agent_name":    "Test Agent",
				"email":         "test@example.com",
				"user_level":    float64(2),
				"contrib_count": float64(10),
				"rating_count":  float64(5),
				"created_at":    float64(1700000000000),
				"last_active_at": float64(1700000000000),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	user, err := client.GetUser(ctx, "test-pubkey")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if user.PublicKey != "test-pubkey" {
		t.Errorf("Expected public_key 'test-pubkey', got %s", user.PublicKey)
	}
	if user.AgentName != "Test Agent" {
		t.Errorf("Expected agent_name 'Test Agent', got %s", user.AgentName)
	}
	if user.UserLevel != 2 {
		t.Errorf("Expected user_level 2, got %d", user.UserLevel)
	}
}

func TestClient_ListCategories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"id":          "tech",
						"name":        "技术",
						"description": "技术相关",
						"parent_id":   "",
						"created_at":  float64(1700000000000),
					},
					map[string]interface{}{
						"id":          "tech/ai",
						"name":        "人工智能",
						"description": "AI 相关",
						"parent_id":   "tech",
						"created_at":  float64(1700000000000),
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	categories, err := client.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories failed: %v", err)
	}

	if len(categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(categories))
	}
	if categories[0].ID != "tech" {
		t.Errorf("Expected first category ID 'tech', got %s", categories[0].ID)
	}
	if categories[1].ParentID != "tech" {
		t.Errorf("Expected second category parent_id 'tech', got %s", categories[1].ParentID)
	}
}

func TestClient_RegisterUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/register" {
			t.Errorf("Expected path /api/v1/user/register, got %s", r.URL.Path)
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"public_key": "new-pubkey",
				"agent_name": "New Agent",
				"user_level": float64(0),
				"created_at": float64(1700000000000),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	req := &RegisterRequest{
		PublicKey: "new-pubkey",
		AgentName: "New Agent",
	}

	resp, err := client.RegisterUser(ctx, req)
	if err != nil {
		t.Fatalf("RegisterUser failed: %v", err)
	}

	if resp.PublicKey != "new-pubkey" {
		t.Errorf("Expected public_key 'new-pubkey', got %s", resp.PublicKey)
	}
	if resp.AgentName != "New Agent" {
		t.Errorf("Expected agent_name 'New Agent', got %s", resp.AgentName)
	}
	if resp.UserLevel != 0 {
		t.Errorf("Expected user_level 0, got %d", resp.UserLevel)
	}
}

func TestClient_CreateEntry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth headers are present
		if r.Header.Get("X-AgentWiki-PublicKey") == "" {
			t.Error("X-AgentWiki-PublicKey header should be set")
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"id":         "new-entry-id",
				"title":      "New Entry",
				"content":    "Content here",
				"category":   "tech",
				"score":      float64(0),
				"score_count": float64(0),
				"created_at": float64(1700000000000),
				"updated_at": float64(1700000000000),
				"created_by": "user-1",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	// Generate keys for auth
	tmpDir, err := createTempKeyDir()
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanupTempDir(tmpDir)

	err = client.LoadOrGenerateKeys(tmpDir)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys failed: %v", err)
	}

	ctx := context.Background()
	req := &CreateEntryRequest{
		Title:    "New Entry",
		Content:  "Content here",
		Category: "tech",
	}

	entry, err := client.CreateEntry(ctx, req)
	if err != nil {
		t.Fatalf("CreateEntry failed: %v", err)
	}

	if entry.ID != "new-entry-id" {
		t.Errorf("Expected ID 'new-entry-id', got %s", entry.ID)
	}
}

func TestClient_DeleteEntry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE method, got %s", r.Method)
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

	tmpDir, err := createTempKeyDir()
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanupTempDir(tmpDir)

	err = client.LoadOrGenerateKeys(tmpDir)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys failed: %v", err)
	}

	ctx := context.Background()
	err = client.DeleteEntry(ctx, "entry-to-delete")
	if err != nil {
		t.Fatalf("DeleteEntry failed: %v", err)
	}
}

func TestClient_RateEntry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		resp := APIResponse{
			Code:    0,
			Message: "rated",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	tmpDir, err := createTempKeyDir()
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanupTempDir(tmpDir)

	err = client.LoadOrGenerateKeys(tmpDir)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys failed: %v", err)
	}

	ctx := context.Background()
	err = client.RateEntry(ctx, "entry-123", 4.5, "Great entry!")
	if err != nil {
		t.Fatalf("RateEntry failed: %v", err)
	}
}

func TestClient_GetBacklinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"backlinks": []interface{}{"entry-1", "entry-2", "entry-3"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	backlinks, err := client.GetBacklinks(ctx, "entry-123")
	if err != nil {
		t.Fatalf("GetBacklinks failed: %v", err)
	}

	if len(backlinks) != 3 {
		t.Errorf("Expected 3 backlinks, got %d", len(backlinks))
	}
}

func TestClient_GetOutlinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"outlinks": []interface{}{"entry-a", "entry-b"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	outlinks, err := client.GetOutlinks(ctx, "entry-123")
	if err != nil {
		t.Fatalf("GetOutlinks failed: %v", err)
	}

	if len(outlinks) != 2 {
		t.Errorf("Expected 2 outlinks, got %d", len(outlinks))
	}
}

func TestClient_GetCategoryEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"total_count": float64(1),
				"items": []interface{}{
					map[string]interface{}{
						"id":          "entry-1",
						"title":       "Tech Entry",
						"category":    "tech/ai",
						"score":       float64(4.0),
						"score_count": float64(5),
						"created_at":  float64(1700000000000),
						"updated_at":  float64(1700000000000),
						"created_by":  "user-1",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	entries, total, err := client.GetCategoryEntries(ctx, "tech/ai", 20, 0)
	if err != nil {
		t.Fatalf("GetCategoryEntries failed: %v", err)
	}

	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
}

func TestClient_GetSyncStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"running":        true,
				"last_sync":      float64(1700000000000),
				"synced_entries": float64(100),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	status, err := client.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus failed: %v", err)
	}

	if !status.Running {
		t.Error("Expected running to be true")
	}
	if status.SyncedEntries != 100 {
		t.Errorf("Expected synced_entries 100, got %d", status.SyncedEntries)
	}
}

// Helper functions
func createTempKeyDir() (string, error) {
	return os.MkdirTemp("", "awctl-keys-*")
}

func cleanupTempDir(dir string) {
	os.RemoveAll(dir)
}
