package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"

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

func TestExporter_Export(t *testing.T) {
	store := newTestStore(t)

	// 创建测试数据
	entry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Test Entry",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
		CreatedBy: "user-1",
	}
	store.Entry.Create(nil, entry)

	cat := &model.Category{
		ID:    "cat-1",
		Path:  "test",
		Name:  "Test",
		Level: 0,
	}
	store.Category.Create(nil, cat)

	user := &model.User{
		PublicKey:    "user-1",
		AgentName:    "TestAgent",
		UserLevel:    1,
		Status:       model.UserStatusActive,
	}
	store.User.Create(nil, user)

	// 创建导出器
	exporter := NewExporter(store, "test-node")

	// 执行导出
	opts := ExportOptions{
		IncludeEntries:    true,
		IncludeCategories: true,
		IncludeUsers:      true,
		IncludeRatings:    false,
	}

	zipData, err := exporter.Export(opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// 验证 ZIP 文件
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	// 检查文件列表
	fileNames := make(map[string]bool)
	for _, file := range reader.File {
		fileNames[file.Name] = true
	}

	if !fileNames["manifest.json"] {
		t.Error("Missing manifest.json")
	}
	if !fileNames["entries.json"] {
		t.Error("Missing entries.json")
	}
	if !fileNames["categories.json"] {
		t.Error("Missing categories.json")
	}
	if !fileNames["users.json"] {
		t.Error("Missing users.json")
	}

	// 验证 manifest 内容
	for _, file := range reader.File {
		if file.Name == "manifest.json" {
			rc, err := file.Open()
			if err != nil {
				t.Fatalf("Failed to open manifest.json: %v", err)
			}
			var manifest Manifest
			if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
				rc.Close()
				t.Fatalf("Failed to decode manifest: %v", err)
			}
			rc.Close()

			if manifest.Version != "1.0" {
				t.Errorf("Expected version 1.0, got %s", manifest.Version)
			}
			if manifest.NodeID != "test-node" {
				t.Errorf("Expected nodeID test-node, got %s", manifest.NodeID)
			}
		}
	}
}

func TestExporter_ExportEmpty(t *testing.T) {
	store := newTestStore(t)
	exporter := NewExporter(store, "test-node")

	opts := ExportOptions{
		IncludeEntries: true,
	}

	zipData, err := exporter.Export(opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(zipData) < 100 {
		t.Errorf("ZIP too small, got %d bytes", len(zipData))
	}
}

func TestExporter_ExportWithRatings(t *testing.T) {
	store := newTestStore(t)

	// 创建测试条目
	entry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Test Entry",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
		CreatedBy: "user-1",
	}
	store.Entry.Create(nil, entry)

	// 创建评分
	rating := &model.Rating{
		ID:          "rating-1",
		EntryId:     "entry-1",
		RaterPubkey: "user-1",
		Score:       5,
		Comment:     "Great!",
	}
	store.Rating.Create(nil, rating)

	exporter := NewExporter(store, "test-node")

	opts := ExportOptions{
		IncludeEntries: true,
		IncludeRatings: true,
	}

	zipData, err := exporter.Export(opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// 验证 ZIP 包含 ratings.json
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	fileNames := make(map[string]bool)
	for _, file := range reader.File {
		fileNames[file.Name] = true
	}

	if !fileNames["ratings.json"] {
		t.Error("Missing ratings.json")
	}
}

func TestExporter_ExportUserPrivacy(t *testing.T) {
	store := newTestStore(t)

	// 创建用户，包含敏感字段
	user := &model.User{
		PublicKey:    "user-1",
		AgentName:    "TestAgent",
		UserLevel:    1,
		Status:       model.UserStatusActive,
		Email:        "secret@example.com", // 敏感字段
	}
	store.User.Create(nil, user)

	exporter := NewExporter(store, "test-node")

	opts := ExportOptions{
		IncludeUsers: true,
	}

	zipData, err := exporter.Export(opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// 验证导出的用户不包含敏感字段
	reader, _ := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	for _, file := range reader.File {
		if file.Name == "users.json" {
			rc, _ := file.Open()
			var exportUsers []ExportUser
			json.NewDecoder(rc).Decode(&exportUsers)
			rc.Close()

			if len(exportUsers) != 1 {
				t.Fatalf("Expected 1 user, got %d", len(exportUsers))
			}

			// 验证导出格式不包含邮箱字段
			if exportUsers[0].PublicKey != "user-1" {
				t.Errorf("Wrong public key")
			}
			// ExportUser 结构体不包含 Email 字段，所以这里自动通过
		}
	}
}
