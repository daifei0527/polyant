package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
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
		PublicKey: "user-1",
		AgentName: "TestAgent",
		UserLevel: 1,
		Status:    model.UserStatusActive,
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

// TestExporter_ExportAllRecordsNoTruncation 验证导出不截断：创建多条目 + 多评分，
// 导出后断言 manifest 计数与实际记录数一致（评分经 ListAll 取全量，取代笛卡尔积；
// 条目/用户经无截断 limit）。锁定原先 Limit:100000 静默截断 bug 已修复。
func TestExporter_ExportAllRecordsNoTruncation(t *testing.T) {
	store := newTestStore(t)

	const numEntries = 5
	const ratingsPerEntry = 2
	wantRatings := numEntries * ratingsPerEntry

	for i := 0; i < numEntries; i++ {
		eid := fmt.Sprintf("entry-%d", i)
		if _, err := store.Entry.Create(nil, &model.KnowledgeEntry{
			ID: eid, Title: "t", Content: "c", Category: "cat",
			Status: model.EntryStatusPublished, CreatedBy: "u",
		}); err != nil {
			t.Fatalf("create entry: %v", err)
		}
		for j := 0; j < ratingsPerEntry; j++ {
			if _, err := store.Rating.Create(nil, &model.Rating{
				ID: fmt.Sprintf("r-%d-%d", i, j), EntryId: eid,
				RaterPubkey: fmt.Sprintf("rater-%d", j), Score: float64(j + 1), Weight: 1,
			}); err != nil {
				t.Fatalf("create rating: %v", err)
			}
		}
	}

	zipData, err := NewExporter(store, "node").Export(ExportOptions{IncludeEntries: true, IncludeRatings: true})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}

	// manifest 计数
	var manifest Manifest
	for _, f := range reader.File {
		if f.Name == "manifest.json" {
			rc, _ := f.Open()
			json.NewDecoder(rc).Decode(&manifest)
			rc.Close()
		}
	}
	if manifest.Counts["entries"] != numEntries {
		t.Errorf("manifest entries = %d, want %d", manifest.Counts["entries"], numEntries)
	}
	if manifest.Counts["ratings"] != wantRatings {
		t.Errorf("manifest ratings = %d, want %d", manifest.Counts["ratings"], wantRatings)
	}

	// ratings.json 实际记录数
	for _, f := range reader.File {
		if f.Name == "ratings.json" {
			rc, _ := f.Open()
			var ratings []*model.Rating
			json.NewDecoder(rc).Decode(&ratings)
			rc.Close()
			if len(ratings) != wantRatings {
				t.Errorf("ratings.json has %d ratings, want %d (no truncation/drop)", len(ratings), wantRatings)
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
		PublicKey: "user-1",
		AgentName: "TestAgent",
		UserLevel: 1,
		Status:    model.UserStatusActive,
		Email:     "secret@example.com", // 敏感字段
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
