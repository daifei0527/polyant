package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

func TestImporter_Import(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	// 创建测试 ZIP 数据
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 写入条目
	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "Test", Content: "Content", Category: "test", Status: "published"},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	// 写入 manifest
	manifest := &Manifest{Version: "1.0", NodeID: "test"}
	manifestData, _ := json.Marshal(manifest)
	w2, _ := zipWriter.Create("manifest.json")
	w2.Write(manifestData)

	zipWriter.Close()
	zipData := buf.Bytes()

	// 执行导入
	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(zipData, opts)

	if !result.Success {
		t.Errorf("Import failed: %v", result.Errors)
	}
	if result.Summary.EntriesImported != 1 {
		t.Errorf("Expected 1 entry imported, got %d", result.Summary.EntriesImported)
	}

	// 验证数据已导入
	entry, err := store.Entry.Get(nil, "entry-1")
	if err != nil {
		t.Fatalf("Failed to get imported entry: %v", err)
	}
	if entry.Title != "Test" {
		t.Errorf("Expected title 'Test', got '%s'", entry.Title)
	}
}

func TestImporter_ConflictSkip(t *testing.T) {
	store := newTestStore(t)

	// 预先创建条目
	existing := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Existing",
		Content:   "Old Content",
		Category:  "test",
		Status:    "published",
		Version:   1,
	}
	store.Entry.Create(nil, existing)

	importer := NewImporter(store)

	// 创建包含相同 ID 的导入数据
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "New Title", Content: "New Content", Category: "test", Status: "published", Version: 2},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Summary.EntriesSkipped != 1 {
		t.Errorf("Expected 1 entry skipped, got %d", result.Summary.EntriesSkipped)
	}

	// 验证现有数据未被修改
	entry, _ := store.Entry.Get(nil, "entry-1")
	if entry.Title != "Existing" {
		t.Errorf("Title should remain 'Existing', got '%s'", entry.Title)
	}
}

func TestImporter_ConflictOverwrite(t *testing.T) {
	store := newTestStore(t)

	existing := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Existing",
		Content:   "Old",
		Category:  "test",
		Status:    "published",
		Version:   1,
	}
	store.Entry.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "New Title", Content: "New", Category: "test", Status: "published", Version: 2},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictOverwrite,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Summary.EntriesUpdated != 1 {
		t.Errorf("Expected 1 entry updated, got %d", result.Summary.EntriesUpdated)
	}

	entry, _ := store.Entry.Get(nil, "entry-1")
	if entry.Title != "New Title" {
		t.Errorf("Title should be updated to 'New Title', got '%s'", entry.Title)
	}
}

func TestImporter_ConflictMerge(t *testing.T) {
	store := newTestStore(t)

	// 现有条目版本较高
	existing := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Existing",
		Content:   "Old",
		Category:  "test",
		Status:    "published",
		Version:   5,
	}
	store.Entry.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 导入条目版本较低
	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "New Title", Content: "New", Category: "test", Status: "published", Version: 2},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictMerge,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	// 应该跳过，因为现有版本更高
	if result.Summary.EntriesSkipped != 1 {
		t.Errorf("Expected 1 entry skipped, got %d", result.Summary.EntriesSkipped)
	}

	entry, _ := store.Entry.Get(nil, "entry-1")
	if entry.Title != "Existing" {
		t.Errorf("Title should remain 'Existing' (higher version), got '%s'", entry.Title)
	}
}

func TestImporter_UserLevelSecurity(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 尝试导入 Lv5 用户，但操作者只有 Lv4
	users := []ExportUser{
		{PublicKey: "user-1", AgentName: "Admin", UserLevel: 5, Status: "active"},
	}
	userData, _ := json.Marshal(users)
	w, _ := zipWriter.Create("users.json")
	w.Write(userData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    4, // Lv4 操作者
	}
	result := importer.Import(buf.Bytes(), opts)

	// 应该有错误
	if len(result.Errors) == 0 {
		t.Error("Expected error for importing higher level user")
	}

	found := false
	for _, e := range result.Errors {
		if e.Type == "user" && e.Message == "cannot import user with higher level" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected specific error message for higher level user")
	}
}

func TestImporter_InvalidZip(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import([]byte("not a zip file"), opts)

	if result.Success {
		t.Error("Expected import to fail with invalid zip")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors in result")
	}
}

func TestImporter_ImportCategories(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	categories := []*model.Category{
		{ID: "cat-1", Path: "tech", Name: "Tech", Level: 0},
		{ID: "cat-2", Path: "tech/ai", Name: "AI", Level: 1},
	}
	catData, _ := json.Marshal(categories)
	w, _ := zipWriter.Create("categories.json")
	w.Write(catData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Summary.CategoriesImported != 2 {
		t.Errorf("Expected 2 categories imported, got %d", result.Summary.CategoriesImported)
	}

	// 验证分类已导入
	cat, err := store.Category.Get(nil, "tech")
	if err != nil {
		t.Fatalf("Failed to get imported category: %v", err)
	}
	if cat.Name != "Tech" {
		t.Errorf("Expected name 'Tech', got '%s'", cat.Name)
	}
}

func TestImporter_ImportRatings(t *testing.T) {
	store := newTestStore(t)

	// 创建条目供评分关联
	entry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Test",
		Content:   "Content",
		Category:  "test",
		Status:    "published",
	}
	store.Entry.Create(nil, entry)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	ratings := []*model.Rating{
		{ID: "rating-1", EntryId: "entry-1", RaterPubkey: "user-1", Score: 5, Comment: "Great!"},
	}
	ratingData, _ := json.Marshal(ratings)
	w, _ := zipWriter.Create("ratings.json")
	w.Write(ratingData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Summary.RatingsImported != 1 {
		t.Errorf("Expected 1 rating imported, got %d", result.Summary.RatingsImported)
	}
}
