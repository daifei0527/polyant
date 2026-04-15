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
				"version":        "v1.0.0",
				"uptime_seconds": float64(3600),
				"node_id":        "node-123",
				"node_type":      "seed",
				"nat_type":       "public",
				"peer_count":     float64(5),
				"entry_count":    float64(100),
				"user_count":     float64(10),
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
				"public_key":     "test-pubkey",
				"agent_name":     "Test Agent",
				"email":          "test@example.com",
				"user_level":     float64(2),
				"contrib_count":  float64(10),
				"rating_count":   float64(5),
				"created_at":     float64(1700000000000),
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
		if r.Header.Get("X-Polyant-PublicKey") == "" {
			t.Error("X-Polyant-PublicKey header should be set")
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"id":          "new-entry-id",
				"title":       "New Entry",
				"content":     "Content here",
				"category":    "tech",
				"score":       float64(0),
				"score_count": float64(0),
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

func TestClient_UpdateEntry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT method, got %s", r.Method)
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"id":          "entry-123",
				"title":       "Updated Title",
				"content":     "Updated content",
				"category":    "tech",
				"score":       float64(4.5),
				"score_count": float64(10),
				"created_at":  float64(1700000000000),
				"updated_at":  float64(1700000001000),
				"created_by":  "user-1",
			},
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
	req := &UpdateEntryRequest{
		Title:   "Updated Title",
		Content: "Updated content",
	}

	entry, err := client.UpdateEntry(ctx, "entry-123", req)
	if err != nil {
		t.Fatalf("UpdateEntry failed: %v", err)
	}

	if entry.ID != "entry-123" {
		t.Errorf("Expected ID 'entry-123', got %s", entry.ID)
	}
	if entry.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %s", entry.Title)
	}
}

func TestClient_GetCurrentUserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Polyant-PublicKey") == "" {
			t.Error("X-Polyant-PublicKey header should be set")
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"public_key":     "test-pubkey",
				"agent_name":     "Test Agent",
				"email":          "test@example.com",
				"user_level":     float64(2),
				"contrib_count":  float64(15),
				"rating_count":   float64(8),
				"created_at":     float64(1700000000000),
				"last_active_at": float64(1700000000000),
			},
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
	user, err := client.GetCurrentUserInfo(ctx)
	if err != nil {
		t.Fatalf("GetCurrentUserInfo failed: %v", err)
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
	if user.ContribCount != 15 {
		t.Errorf("Expected contrib_count 15, got %d", user.ContribCount)
	}
}

func TestClient_UpdateUserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT method, got %s", r.Method)
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
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
	err = client.UpdateUserInfo(ctx, "New Agent Name")
	if err != nil {
		t.Fatalf("UpdateUserInfo failed: %v", err)
	}
}

func TestClient_SendVerificationCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		resp := APIResponse{
			Code:    0,
			Message: "verification code sent",
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
	err = client.SendVerificationCode(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("SendVerificationCode failed: %v", err)
	}
}

func TestClient_VerifyEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		resp := APIResponse{
			Code:    0,
			Message: "email verified",
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
	err = client.VerifyEmail(ctx, "test@example.com", "123456")
	if err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}
}

func TestClient_CreateCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"id":         "tech/ai",
				"name":       "人工智能",
				"parent_id":  "tech",
				"created_at": float64(1700000000000),
			},
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
	cat, err := client.CreateCategory(ctx, "tech/ai", "人工智能", "tech")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	if cat.ID != "tech/ai" {
		t.Errorf("Expected ID 'tech/ai', got %s", cat.ID)
	}
	if cat.Name != "人工智能" {
		t.Errorf("Expected name '人工智能', got %s", cat.Name)
	}
	if cat.ParentID != "tech" {
		t.Errorf("Expected parent_id 'tech', got %s", cat.ParentID)
	}
}

func TestClient_TriggerSync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		resp := APIResponse{
			Code:    0,
			Message: "sync triggered",
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
	err = client.TriggerSync(ctx)
	if err != nil {
		t.Fatalf("TriggerSync failed: %v", err)
	}
}

func TestClient_GetStatus_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIError{
			Code:    500,
			Message: "internal error",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetStatus(ctx)
	if err == nil {
		t.Error("GetStatus should return error for 500 response")
	}
}

func TestClient_ListEntries_Error(t *testing.T) {
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

	_, _, err := client.ListEntries(ctx, "", 10, 0)
	if err == nil {
		t.Error("ListEntries should return error for 400 response")
	}
}

func TestClient_GetUser_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{
			Code:    404,
			Message: "user not found",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetUser(ctx, "nonexistent")
	if err == nil {
		t.Error("GetUser should return error for 404 response")
	}
}

func TestClient_ListCategories_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIError{
			Code:    500,
			Message: "internal error",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.ListCategories(ctx)
	if err == nil {
		t.Error("ListCategories should return error for 500 response")
	}
}

func TestClient_GetBacklinks_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{
			Code:    404,
			Message: "entry not found",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetBacklinks(ctx, "nonexistent")
	if err == nil {
		t.Error("GetBacklinks should return error for 404 response")
	}
}

func TestClient_SearchEntries_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIError{
			Code:    400,
			Message: "invalid query",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, _, err := client.SearchEntries(ctx, "", 10)
	if err == nil {
		t.Error("SearchEntries should return error for 400 response")
	}
}

func TestClient_GetCategoryEntries_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{
			Code:    404,
			Message: "category not found",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, _, err := client.GetCategoryEntries(ctx, "nonexistent", 10, 0)
	if err == nil {
		t.Error("GetCategoryEntries should return error for 404 response")
	}
}

// Helper functions
func createTempKeyDir() (string, error) {
	return os.MkdirTemp("", "pactl-keys-*")
}

func cleanupTempDir(dir string) {
	os.RemoveAll(dir)
}

// TestFormatDuration 测试格式化持续时间
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  int64
		expected string
	}{
		{0, "0 秒"},
		{30, "30 秒"},
		{59, "59 秒"},
		{60, "1 分钟"},
		{120, "2 分钟"},
		{3599, "59 分钟"},
		{3600, "1 小时 0 分钟"},
		{3661, "1 小时 1 分钟"},
		{7200, "2 小时 0 分钟"},
		{86399, "23 小时 59 分钟"},
		{86400, "1 天 0 小时"},
		{90000, "1 天 1 小时"},
		{172800, "2 天 0 小时"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.seconds)
			if result != tt.expected {
				t.Errorf("formatDuration(%d) = %s, want %s", tt.seconds, result, tt.expected)
			}
		})
	}
}

// TestMaskEmail 测试邮箱掩码
func TestMaskEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected string
	}{
		{"a@b.c", "a***@b.c"},    // at=1 <= 2, return email[:at] + "***" + email[at:]
		{"ab@c.d", "ab***@c.d"},  // at=2 <= 2, return email[:at] + "***" + email[at:]
		{"abc@d.e", "ab***@d.e"}, // at=3 > 2, return email[:2] + "***" + email[at:]
		{"test@example.com", "te***@example.com"},
		{"user.name@domain.org", "us***@domain.org"},
		{"x@y", "***"},   // len=3 < 5
		{"xyz", "***"},   // no @
		{"", "***"},      // empty
		{"short", "***"}, // no @
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := maskEmail(tt.email)
			if result != tt.expected {
				t.Errorf("maskEmail(%s) = %s, want %s", tt.email, result, tt.expected)
			}
		})
	}
}

// TestMin 测试 min 函数
func TestMin(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{0, 10, 0},
		{-1, 1, -1},
		{-5, -3, -5},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

// TestClient_doRequestWithAuth_NoKeys 测试无密钥时的认证请求
func TestClient_doRequestWithAuth_NoKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Code: 0, Message: "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	// Without keys and requireAuth=true, should fail
	err := client.doRequestWithAuth(ctx, "POST", "/api/v1/test", nil, nil, true)
	if err == nil {
		t.Error("doRequestWithAuth should fail when no keys and requireAuth=true")
	}
}

// TestClient_doRequestWithAuth_WithKeys 测试有密钥时的认证请求
func TestClient_doRequestWithAuth_WithKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Polyant-PublicKey") == "" {
			t.Error("Expected X-Polyant-PublicKey header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Code: 0, Message: "success"})
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

	// With keys, should succeed
	var result APIResponse
	err = client.doRequestWithAuth(ctx, "POST", "/api/v1/test", nil, &result, false)
	if err != nil {
		t.Errorf("doRequestWithAuth failed: %v", err)
	}
}

// TestClient_GetSyncStatus_Error 测试同步状态错误
func TestClient_GetSyncStatus_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIError{Code: 500, Message: "internal error"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetSyncStatus(ctx)
	if err == nil {
		t.Error("GetSyncStatus should return error for 500 response")
	}
}

// TestClient_GetEntry_Error 测试获取条目错误
func TestClient_GetEntry_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{Code: 404, Message: "entry not found"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetEntry(ctx, "nonexistent")
	if err == nil {
		t.Error("GetEntry should return error for 404 response")
	}
}

// TestClient_GetOutlinks_Error 测试获取出链错误
func TestClient_GetOutlinks_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{Code: 404, Message: "entry not found"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetOutlinks(ctx, "nonexistent")
	if err == nil {
		t.Error("GetOutlinks should return error for 404 response")
	}
}

// TestClient_CreateEntry_Error 测试创建条目错误
func TestClient_CreateEntry_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIError{Code: 400, Message: "invalid request"})
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
	req := &CreateEntryRequest{
		Title:    "Test",
		Content:  "Content",
		Category: "tech",
	}

	_, err = client.CreateEntry(ctx, req)
	if err == nil {
		t.Error("CreateEntry should return error for 400 response")
	}
}

// TestClient_RegisterUser_Error 测试注册用户错误
func TestClient_RegisterUser_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(APIError{Code: 409, Message: "user already exists"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	req := &RegisterRequest{
		PublicKey: "existing-pubkey",
		AgentName: "Test",
	}

	_, err := client.RegisterUser(ctx, req)
	if err == nil {
		t.Error("RegisterUser should return error for 409 response")
	}
}

// TestClient_DeleteEntry_Error 测试删除条目错误
func TestClient_DeleteEntry_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{Code: 404, Message: "entry not found"})
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
	err = client.DeleteEntry(ctx, "nonexistent")
	if err == nil {
		t.Error("DeleteEntry should return error for 404 response")
	}
}

// TestClient_RateEntry_Error 测试评分错误
func TestClient_RateEntry_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{Code: 404, Message: "entry not found"})
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
	err = client.RateEntry(ctx, "nonexistent", 5.0, "comment")
	if err == nil {
		t.Error("RateEntry should return error for 404 response")
	}
}

// TestClient_UpdateEntry_Error 测试更新条目错误
func TestClient_UpdateEntry_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{Code: 404, Message: "entry not found"})
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
	req := &UpdateEntryRequest{Title: "Updated"}

	_, err = client.UpdateEntry(ctx, "nonexistent", req)
	if err == nil {
		t.Error("UpdateEntry should return error for 404 response")
	}
}

// TestClient_CreateCategory_Error 测试创建分类错误
func TestClient_CreateCategory_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(APIError{Code: 409, Message: "category already exists"})
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
	_, err = client.CreateCategory(ctx, "tech", "技术", "")
	if err == nil {
		t.Error("CreateCategory should return error for 409 response")
	}
}

// TestClient_TriggerSync_Error 测试触发同步错误
func TestClient_TriggerSync_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIError{Code: 500, Message: "sync failed"})
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
	err = client.TriggerSync(ctx)
	if err == nil {
		t.Error("TriggerSync should return error for 500 response")
	}
}

// TestClient_GetCurrentUserInfo_Error 测试获取当前用户信息错误
func TestClient_GetCurrentUserInfo_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(APIError{Code: 401, Message: "unauthorized"})
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
	_, err = client.GetCurrentUserInfo(ctx)
	if err == nil {
		t.Error("GetCurrentUserInfo should return error for 401 response")
	}
}

// TestClient_UpdateUserInfo_Error 测试更新用户信息错误
func TestClient_UpdateUserInfo_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(APIError{Code: 401, Message: "unauthorized"})
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
	err = client.UpdateUserInfo(ctx, "New Name")
	if err == nil {
		t.Error("UpdateUserInfo should return error for 401 response")
	}
}

// TestClient_SendVerificationCode_Error 测试发送验证码错误
func TestClient_SendVerificationCode_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIError{Code: 400, Message: "invalid email"})
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
	err = client.SendVerificationCode(ctx, "invalid-email")
	if err == nil {
		t.Error("SendVerificationCode should return error for 400 response")
	}
}

// TestClient_VerifyEmail_Error 测试验证邮箱错误
func TestClient_VerifyEmail_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIError{Code: 400, Message: "invalid code"})
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
	err = client.VerifyEmail(ctx, "test@example.com", "wrong-code")
	if err == nil {
		t.Error("VerifyEmail should return error for 400 response")
	}
}

// TestGetLang 测试语言设置
func TestGetLang(t *testing.T) {
	// 默认语言
	langFlag = ""
	lang := getLang()
	if lang != "zh-CN" {
		t.Errorf("Expected default lang 'zh-CN', got %s", lang)
	}

	// 英文
	langFlag = "en-US"
	lang = getLang()
	if lang != "en-US" {
		t.Errorf("Expected lang 'en-US', got %s", lang)
	}

	// 恢复默认
	langFlag = ""
}

// TestClient_LoadOrGenerateKeys_InvalidPath 测试无效路径加载密钥
func TestClient_LoadOrGenerateKeys_InvalidPath(t *testing.T) {
	client := NewClient("http://localhost:8080")

	// 使用无效路径（父目录不存在）
	err := client.LoadOrGenerateKeys("/nonexistent/path/keys")
	if err == nil {
		t.Error("LoadOrGenerateKeys should fail with invalid path")
	}
}

// TestClient_CreateEntry_NoKeys 测试无密钥时创建条目
func TestClient_CreateEntry_NoKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Code: 0, Message: "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	req := &CreateEntryRequest{
		Title:    "Test",
		Content:  "Content",
		Category: "tech",
	}

	// Without keys, should fail
	_, err := client.CreateEntry(ctx, req)
	if err == nil {
		t.Error("CreateEntry should fail when no keys are loaded")
	}
}

// TestClient_DeleteEntry_NoKeys 测试无密钥时删除条目
func TestClient_DeleteEntry_NoKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Code: 0, Message: "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	// Without keys, should fail
	err := client.DeleteEntry(ctx, "entry-1")
	if err == nil {
		t.Error("DeleteEntry should fail when no keys are loaded")
	}
}

// TestClient_RateEntry_NoKeys 测试无密钥时评分
func TestClient_RateEntry_NoKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Code: 0, Message: "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	// Without keys, should fail
	err := client.RateEntry(ctx, "entry-1", 5.0, "comment")
	if err == nil {
		t.Error("RateEntry should fail when no keys are loaded")
	}
}

// TestClient_CreateCategory_NoKeys 测试无密钥时创建分类
func TestClient_CreateCategory_NoKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Code: 0, Message: "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	// Without keys, should fail
	_, err := client.CreateCategory(ctx, "tech", "技术", "")
	if err == nil {
		t.Error("CreateCategory should fail when no keys are loaded")
	}
}

// TestClient_UpdateEntry_NoKeys 测试无密钥时更新条目
func TestClient_UpdateEntry_NoKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Code: 0, Message: "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	req := &UpdateEntryRequest{Title: "Updated"}

	// Without keys, should fail
	_, err := client.UpdateEntry(ctx, "entry-1", req)
	if err == nil {
		t.Error("UpdateEntry should fail when no keys are loaded")
	}
}

// TestClient_GetCurrentUserInfo_NoKeys 测试无密钥时获取用户信息
func TestClient_GetCurrentUserInfo_NoKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Code: 0, Message: "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	// Without keys, should fail
	_, err := client.GetCurrentUserInfo(ctx)
	if err == nil {
		t.Error("GetCurrentUserInfo should fail when no keys are loaded")
	}
}

// TestProcessExists 测试进程存在检查
func TestProcessExists(t *testing.T) {
	// 测试当前进程（应该存在）
	currentPid := os.Getpid()
	if !processExists(currentPid) {
		t.Errorf("processExists(%d) should return true for current process", currentPid)
	}

	// 测试不存在的进程（使用一个非常大的 PID）
	if processExists(99999999) {
		t.Error("processExists should return false for non-existent process")
	}

	// 测试 PID 1（通常是 init 进程，应该存在）
	if !processExists(1) {
		t.Log("processExists(1) returned false, which may be expected in some environments")
	}
}

// TestShowDefaultCategories 测试显示默认分类
func TestShowDefaultCategories(t *testing.T) {
	// 测试 JSON 输出
	err := showDefaultCategories(false, true)
	if err != nil {
		t.Errorf("showDefaultCategories(json) failed: %v", err)
	}

	// 测试树形输出
	err = showDefaultCategories(true, false)
	if err != nil {
		t.Errorf("showDefaultCategories(tree) failed: %v", err)
	}

	// 测试普通输出
	err = showDefaultCategories(false, false)
	if err != nil {
		t.Errorf("showDefaultCategories(plain) failed: %v", err)
	}
}

// TestShowCategoriesTree 测试显示分类树
func TestShowCategoriesTree(t *testing.T) {
	categories := []CategoryInfo{
		{ID: "tech", Name: "技术", ParentID: ""},
		{ID: "tech/ai", Name: "人工智能", ParentID: "tech"},
		{ID: "science", Name: "科学", ParentID: ""},
	}

	err := showCategoriesTree(categories)
	if err != nil {
		t.Errorf("showCategoriesTree failed: %v", err)
	}
}

// TestCategoryInfo 测试分类信息结构
func TestCategoryInfo(t *testing.T) {
	cat := CategoryInfo{
		ID:          "tech",
		Name:        "技术",
		Description: "技术相关",
		ParentID:    "",
		CreatedAt:   1700000000000,
	}

	if cat.ID != "tech" {
		t.Errorf("Expected ID 'tech', got %s", cat.ID)
	}
	if cat.Name != "技术" {
		t.Errorf("Expected Name '技术', got %s", cat.Name)
	}
}

// TestUserInfo 测试用户信息结构
func TestUserInfo(t *testing.T) {
	user := UserInfo{
		PublicKey:    "test-pubkey",
		AgentName:    "Test Agent",
		Email:        "test@example.com",
		UserLevel:    2,
		ContribCount: 10,
		RatingCount:  5,
		CreatedAt:    1700000000000,
		LastActiveAt: 1700000000000,
	}

	if user.PublicKey != "test-pubkey" {
		t.Errorf("Expected PublicKey 'test-pubkey', got %s", user.PublicKey)
	}
	if user.UserLevel != 2 {
		t.Errorf("Expected UserLevel 2, got %d", user.UserLevel)
	}
}

// TestEntryInfo 测试条目信息结构
func TestEntryInfo(t *testing.T) {
	entry := EntryInfo{
		ID:         "entry-1",
		Title:      "Test Entry",
		Category:   "tech",
		Tags:       []string{"go", "test"},
		Score:      4.5,
		ScoreCount: 10,
		CreatedAt:  1700000000000,
		UpdatedAt:  1700000000000,
		CreatedBy:  "user-1",
	}

	if entry.ID != "entry-1" {
		t.Errorf("Expected ID 'entry-1', got %s", entry.ID)
	}
	if len(entry.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(entry.Tags))
	}
}

// TestServerStatus 测试服务器状态结构
func TestServerStatus(t *testing.T) {
	status := ServerStatus{
		Version:    "v1.0.0",
		Uptime:     3600,
		NodeID:     "node-123",
		NodeType:   "seed",
		NATType:    "public",
		PeerCount:  5,
		EntryCount: 100,
		UserCount:  10,
	}

	if status.Version != "v1.0.0" {
		t.Errorf("Expected Version 'v1.0.0', got %s", status.Version)
	}
	if status.PeerCount != 5 {
		t.Errorf("Expected PeerCount 5, got %d", status.PeerCount)
	}
}

// TestSyncStatus 测试同步状态结构
func TestSyncStatus(t *testing.T) {
	status := SyncStatus{
		Running:       true,
		LastSync:      1700000000000,
		SyncedEntries: 100,
	}

	if !status.Running {
		t.Error("Expected Running to be true")
	}
	if status.SyncedEntries != 100 {
		t.Errorf("Expected SyncedEntries 100, got %d", status.SyncedEntries)
	}
}

// TestRegisterRequest 测试注册请求结构
func TestRegisterRequest(t *testing.T) {
	req := RegisterRequest{
		PublicKey: "test-pubkey",
		AgentName: "Test Agent",
	}

	if req.PublicKey != "test-pubkey" {
		t.Errorf("Expected PublicKey 'test-pubkey', got %s", req.PublicKey)
	}
}

// TestCreateEntryRequest 测试创建条目请求结构
func TestCreateEntryRequest(t *testing.T) {
	req := CreateEntryRequest{
		Title:    "Test Title",
		Content:  "Test Content",
		Category: "tech",
		Tags:     []string{"go", "test"},
	}

	if req.Title != "Test Title" {
		t.Errorf("Expected Title 'Test Title', got %s", req.Title)
	}
	if len(req.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(req.Tags))
	}
}

// TestUpdateEntryRequest 测试更新条目请求结构
func TestUpdateEntryRequest(t *testing.T) {
	req := UpdateEntryRequest{
		Title:   "Updated Title",
		Content: "Updated Content",
	}

	if req.Title != "Updated Title" {
		t.Errorf("Expected Title 'Updated Title', got %s", req.Title)
	}
}

// TestRegisterResponse 测试注册响应结构
func TestRegisterResponse(t *testing.T) {
	resp := RegisterResponse{
		PublicKey: "new-pubkey",
		AgentName: "New Agent",
		UserLevel: 0,
		CreatedAt: 1700000000000,
	}

	if resp.PublicKey != "new-pubkey" {
		t.Errorf("Expected PublicKey 'new-pubkey', got %s", resp.PublicKey)
	}
}

// TestAPIResponse 测试 API 响应结构
func TestAPIResponse(t *testing.T) {
	resp := APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]interface{}{"key": "value"},
	}

	if resp.Code != 0 {
		t.Errorf("Expected Code 0, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("Expected Message 'success', got %s", resp.Message)
	}
}

// TestPeerInfo 测试节点信息结构
func TestPeerInfo(t *testing.T) {
	peer := PeerInfo{
		ID:      "peer-123",
		Address: "192.168.1.1:8080",
	}

	if peer.ID != "peer-123" {
		t.Errorf("Expected ID 'peer-123', got %s", peer.ID)
	}
	if peer.Address != "192.168.1.1:8080" {
		t.Errorf("Expected Address '192.168.1.1:8080', got %s", peer.Address)
	}
}

// TestEntryDetail 测试条目详情结构
func TestEntryDetail(t *testing.T) {
	detail := EntryDetail{
		ID:         "entry-1",
		Title:      "Test Entry",
		Category:   "tech",
		Tags:       []string{"go", "test"},
		Score:      4.5,
		ScoreCount: 10,
		CreatedAt:  1700000000000,
		UpdatedAt:  1700000000000,
		CreatedBy:  "user-1",
		Content:    "Test content",
	}

	if detail.Content != "Test content" {
		t.Errorf("Expected Content 'Test content', got %s", detail.Content)
	}
	if detail.ID != "entry-1" {
		t.Errorf("Expected ID 'entry-1', got %s", detail.ID)
	}
	if len(detail.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(detail.Tags))
	}
}

// TestClient_GetStatus_Complete 测试完整状态响应
func TestClient_GetStatus_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"version":        "v1.0.0",
				"uptime_seconds": float64(3600),
				"node_id":        "node-123",
				"node_type":      "seed",
				"nat_type":       "public",
				"peer_count":     float64(5),
				"entry_count":    float64(100),
				"user_count":     float64(10),
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

	// 验证所有字段
	if status.Uptime != 3600 {
		t.Errorf("Expected Uptime 3600, got %d", status.Uptime)
	}
	if status.NATType != "public" {
		t.Errorf("Expected NATType 'public', got %s", status.NATType)
	}
}

// TestClient_ListEntries_WithCategory 测试带分类的条目列表
func TestClient_ListEntries_WithCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查分类参数 (使用 "cat" 参数名)
		category := r.URL.Query().Get("cat")
		if category != "tech" {
			t.Errorf("Expected category 'tech', got %s", category)
		}

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"total_count": float64(1),
				"items": []interface{}{
					map[string]interface{}{
						"id":          "entry-1",
						"title":       "Tech Entry",
						"category":    "tech",
						"score":       float64(4.5),
						"score_count": float64(10),
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

	entries, total, err := client.ListEntries(ctx, "tech", 10, 0)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
}

// TestClient_SearchEntries_WithCategory 测试带分类的搜索
func TestClient_SearchEntries_WithCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"total_count": float64(1),
				"items": []interface{}{
					map[string]interface{}{
						"id":          "entry-1",
						"title":       "Go Programming",
						"category":    "tech",
						"score":       float64(5.0),
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

	entries, total, err := client.SearchEntries(ctx, "go", 10)
	if err != nil {
		t.Fatalf("SearchEntries failed: %v", err)
	}

	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if entries[0].Title != "Go Programming" {
		t.Errorf("Expected title 'Go Programming', got %s", entries[0].Title)
	}
}

// TestClient_GetSyncStatus_Complete 测试完整的同步状态
func TestClient_GetSyncStatus_Complete(t *testing.T) {
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
		t.Error("Expected Running to be true")
	}
	if status.LastSync != 1700000000000 {
		t.Errorf("Expected LastSync 1700000000000, got %d", status.LastSync)
	}
	if status.SyncedEntries != 100 {
		t.Errorf("Expected SyncedEntries 100, got %d", status.SyncedEntries)
	}
}
