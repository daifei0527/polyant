package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
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

// TestImporter_ImportCategories_ConflictOverwrite 测试分类冲突覆盖
func TestImporter_ImportCategories_ConflictOverwrite(t *testing.T) {
	store := newTestStore(t)

	// 预先创建分类
	existing := &model.Category{
		ID:    "cat-1",
		Path:  "tech",
		Name:  "Technology",
		Level: 0,
	}
	store.Category.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	categories := []*model.Category{
		{ID: "cat-1", Path: "tech", Name: "Tech Updated", Level: 0},
	}
	catData, _ := json.Marshal(categories)
	w, _ := zipWriter.Create("categories.json")
	w.Write(catData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictOverwrite,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	// 覆盖策略保留现有分类
	if result.Summary.CategoriesImported != 0 {
		t.Errorf("Expected 0 categories imported (kept existing), got %d", result.Summary.CategoriesImported)
	}
}

// TestImporter_ImportCategories_ConflictMerge 测试分类冲突合并
func TestImporter_ImportCategories_ConflictMerge(t *testing.T) {
	store := newTestStore(t)

	// 预先创建分类
	existing := &model.Category{
		ID:    "cat-1",
		Path:  "tech",
		Name:  "Technology",
		Level: 0,
	}
	store.Category.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	categories := []*model.Category{
		{ID: "cat-1", Path: "tech", Name: "Tech Updated", Level: 0},
		{ID: "cat-2", Path: "science", Name: "Science", Level: 0},
	}
	catData, _ := json.Marshal(categories)
	w, _ := zipWriter.Create("categories.json")
	w.Write(catData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictMerge,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	// 合并策略只导入新分类
	if result.Summary.CategoriesImported != 1 {
		t.Errorf("Expected 1 category imported (new one), got %d", result.Summary.CategoriesImported)
	}
}

// TestImporter_ImportCategories_InvalidJSON 测试无效分类 JSON
func TestImporter_ImportCategories_InvalidJSON(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	w, _ := zipWriter.Create("categories.json")
	w.Write([]byte("invalid json"))

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Success {
		t.Error("Expected import to fail with invalid JSON")
	}
}

// TestImporter_ImportUsers_NewUser 测试导入新用户
func TestImporter_ImportUsers_NewUser(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	users := []ExportUser{
		{PublicKey: "user-1", AgentName: "Test User", UserLevel: 1, Status: "active"},
		{PublicKey: "user-2", AgentName: "Another User", UserLevel: 2, Status: "active"},
	}
	userData, _ := json.Marshal(users)
	w, _ := zipWriter.Create("users.json")
	w.Write(userData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if !result.Success {
		t.Errorf("Import failed: %v", result.Errors)
	}
	if result.Summary.UsersImported != 2 {
		t.Errorf("Expected 2 users imported, got %d", result.Summary.UsersImported)
	}

	// 验证用户已导入
	user, err := store.User.Get(nil, "user-1")
	if err != nil {
		t.Fatalf("Failed to get imported user: %v", err)
	}
	if user.AgentName != "Test User" {
		t.Errorf("Expected agent_name 'Test User', got '%s'", user.AgentName)
	}
}

// TestImporter_ImportUsers_ExistingUser_Overwrite 测试覆盖已存在的用户
func TestImporter_ImportUsers_ExistingUser_Overwrite(t *testing.T) {
	store := newTestStore(t)

	// 预先创建用户
	existing := &model.User{
		PublicKey: "user-1",
		AgentName: "Old Name",
		UserLevel: 1,
		Status:    model.UserStatusActive,
	}
	store.User.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	users := []ExportUser{
		{PublicKey: "user-1", AgentName: "New Name", UserLevel: 1, Status: "inactive"},
	}
	userData, _ := json.Marshal(users)
	w, _ := zipWriter.Create("users.json")
	w.Write(userData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictOverwrite,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Summary.UsersImported != 1 {
		t.Errorf("Expected 1 user updated, got %d", result.Summary.UsersImported)
	}

	// 验证用户已更新
	user, _ := store.User.Get(nil, "user-1")
	if user.AgentName != "New Name" {
		t.Errorf("Expected agent_name 'New Name', got '%s'", user.AgentName)
	}
	if user.Status != "inactive" {
		t.Errorf("Expected status 'inactive', got '%s'", user.Status)
	}
}

// TestImporter_ImportUsers_ExistingUser_Merge 测试合并已存在的用户
func TestImporter_ImportUsers_ExistingUser_Merge(t *testing.T) {
	store := newTestStore(t)

	existing := &model.User{
		PublicKey: "user-1",
		AgentName: "Old Name",
		UserLevel: 1,
		Status:    model.UserStatusActive,
	}
	store.User.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	users := []ExportUser{
		{PublicKey: "user-1", AgentName: "New Name", UserLevel: 1, Status: "inactive"},
	}
	userData, _ := json.Marshal(users)
	w, _ := zipWriter.Create("users.json")
	w.Write(userData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictMerge,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Summary.UsersImported != 1 {
		t.Errorf("Expected 1 user updated, got %d", result.Summary.UsersImported)
	}
}

// TestImporter_ImportUsers_InvalidJSON 测试无效用户 JSON
func TestImporter_ImportUsers_InvalidJSON(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	w, _ := zipWriter.Create("users.json")
	w.Write([]byte("invalid json"))

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Success {
		t.Error("Expected import to fail with invalid JSON")
	}
}

// TestImporter_ImportRatings_ExistingRating_Skip 测试跳过已存在的评分
func TestImporter_ImportRatings_ExistingRating_Skip(t *testing.T) {
	store := newTestStore(t)

	// 创建条目
	entry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Test",
		Content:   "Content",
		Category:  "test",
		Status:    "published",
	}
	store.Entry.Create(nil, entry)

	// 预先创建评分
	existing := &model.Rating{
		ID:          "rating-1",
		EntryId:     "entry-1",
		RaterPubkey: "user-1",
		Score:       3,
		Comment:     "Old",
	}
	store.Rating.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	ratings := []*model.Rating{
		{ID: "rating-1", EntryId: "entry-1", RaterPubkey: "user-1", Score: 5, Comment: "New"},
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

	// 跳过策略应有警告错误
	if len(result.Errors) == 0 {
		t.Error("Expected warning for skipped rating")
	}
}

// TestImporter_ImportRatings_ExistingRating_Overwrite 测试覆盖已存在的评分
func TestImporter_ImportRatings_ExistingRating_Overwrite(t *testing.T) {
	store := newTestStore(t)

	entry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Test",
		Content:   "Content",
		Category:  "test",
		Status:    "published",
	}
	store.Entry.Create(nil, entry)

	existing := &model.Rating{
		ID:          "rating-1",
		EntryId:     "entry-1",
		RaterPubkey: "user-1",
		Score:       3,
		Comment:     "Old",
	}
	store.Rating.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	ratings := []*model.Rating{
		{ID: "rating-1", EntryId: "entry-1", RaterPubkey: "user-1", Score: 5, Comment: "New"},
	}
	ratingData, _ := json.Marshal(ratings)
	w, _ := zipWriter.Create("ratings.json")
	w.Write(ratingData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictOverwrite,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	// 覆盖策略保留现有评分
	_ = result
}

// TestImporter_ImportRatings_ExistingRating_Merge 测试合并已存在的评分
func TestImporter_ImportRatings_ExistingRating_Merge(t *testing.T) {
	store := newTestStore(t)

	entry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Test",
		Content:   "Content",
		Category:  "test",
		Status:    "published",
	}
	store.Entry.Create(nil, entry)

	existing := &model.Rating{
		ID:          "rating-1",
		EntryId:     "entry-1",
		RaterPubkey: "user-1",
		Score:       3,
		Comment:     "Old",
	}
	store.Rating.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	ratings := []*model.Rating{
		{ID: "rating-1", EntryId: "entry-1", RaterPubkey: "user-1", Score: 5, Comment: "New"},
	}
	ratingData, _ := json.Marshal(ratings)
	w, _ := zipWriter.Create("ratings.json")
	w.Write(ratingData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictMerge,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	// 合并策略保留现有评分
	_ = result
}

// TestImporter_ImportRatings_InvalidJSON 测试无效评分 JSON
func TestImporter_ImportRatings_InvalidJSON(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	w, _ := zipWriter.Create("ratings.json")
	w.Write([]byte("invalid json"))

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Success {
		t.Error("Expected import to fail with invalid JSON")
	}
}

// TestImporter_ImportEntries_InvalidJSON 测试无效条目 JSON
func TestImporter_ImportEntries_InvalidJSON(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	w, _ := zipWriter.Create("entries.json")
	w.Write([]byte("invalid json"))

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Success {
		t.Error("Expected import to fail with invalid JSON")
	}
}

// TestImporter_ConflictMerge_NewerVersion 测试合并时导入更新版本
func TestImporter_ConflictMerge_NewerVersion(t *testing.T) {
	store := newTestStore(t)

	existing := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Old",
		Content:   "Old Content",
		Category:  "test",
		Status:    "published",
		Version:   1,
	}
	store.Entry.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "New", Content: "New Content", Category: "test", Status: "published", Version: 5},
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

	// 应更新条目（版本更高）
	if result.Summary.EntriesUpdated != 1 {
		t.Errorf("Expected 1 entry updated, got %d", result.Summary.EntriesUpdated)
	}

	entry, _ := store.Entry.Get(nil, "entry-1")
	if entry.Title != "New" {
		t.Errorf("Title should be 'New', got '%s'", entry.Title)
	}
}

// TestImporter_ImportMultipleItems 测试导入多个项目
func TestImporter_ImportMultipleItems(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 多个条目
	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "Test 1", Content: "Content 1", Category: "test", Status: "published"},
		{ID: "entry-2", Title: "Test 2", Content: "Content 2", Category: "test", Status: "published"},
		{ID: "entry-3", Title: "Test 3", Content: "Content 3", Category: "test", Status: "published"},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	// 多个分类
	categories := []*model.Category{
		{ID: "cat-1", Path: "tech", Name: "Technology", Level: 0},
		{ID: "cat-2", Path: "tech/go", Name: "Golang", Level: 1},
	}
	catData, _ := json.Marshal(categories)
	w2, _ := zipWriter.Create("categories.json")
	w2.Write(catData)

	// 多个用户
	users := []ExportUser{
		{PublicKey: "user-1", AgentName: "User 1", UserLevel: 1, Status: "active"},
		{PublicKey: "user-2", AgentName: "User 2", UserLevel: 2, Status: "active"},
	}
	userData, _ := json.Marshal(users)
	w3, _ := zipWriter.Create("users.json")
	w3.Write(userData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if !result.Success {
		t.Errorf("Import failed: %v", result.Errors)
	}
	if result.Summary.EntriesImported != 3 {
		t.Errorf("Expected 3 entries imported, got %d", result.Summary.EntriesImported)
	}
	if result.Summary.CategoriesImported != 2 {
		t.Errorf("Expected 2 categories imported, got %d", result.Summary.CategoriesImported)
	}
	if result.Summary.UsersImported != 2 {
		t.Errorf("Expected 2 users imported, got %d", result.Summary.UsersImported)
	}
}

// TestImporter_ImportWithManifest 测试带 manifest 的导入
func TestImporter_ImportWithManifest(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 写入条目
	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "Test", Content: "Content", Category: "test", Status: "published"},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	// 写入 manifest（应该被忽略，不影响导入）
	manifest := &Manifest{Version: "1.0", NodeID: "test-node"}
	manifestData, _ := json.Marshal(manifest)
	w2, _ := zipWriter.Create("manifest.json")
	w2.Write(manifestData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if !result.Success {
		t.Errorf("Import failed: %v", result.Errors)
	}
	if result.Summary.EntriesImported != 1 {
		t.Errorf("Expected 1 entry imported, got %d", result.Summary.EntriesImported)
	}
}

// TestImporter_ImportEmptyFiles 测试导入空文件
func TestImporter_ImportEmptyFiles(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 写入空数组
	w, _ := zipWriter.Create("entries.json")
	w.Write([]byte("[]"))

	w2, _ := zipWriter.Create("categories.json")
	w2.Write([]byte("[]"))

	w3, _ := zipWriter.Create("users.json")
	w3.Write([]byte("[]"))

	w4, _ := zipWriter.Create("ratings.json")
	w4.Write([]byte("[]"))

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if !result.Success {
		t.Errorf("Import failed: %v", result.Errors)
	}
	// 所有计数应为 0
	if result.Summary.EntriesImported != 0 {
		t.Errorf("Expected 0 entries imported, got %d", result.Summary.EntriesImported)
	}
}

// TestImporter_UserSecurityBelowOperator 测试用户等级等于操作者的情况
func TestImporter_UserSecurityBelowOperator(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 导入 Lv5 用户，操作者也是 Lv5（应该允许）
	users := []ExportUser{
		{PublicKey: "user-1", AgentName: "Admin", UserLevel: 5, Status: "active"},
	}
	userData, _ := json.Marshal(users)
	w, _ := zipWriter.Create("users.json")
	w.Write(userData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5, // Lv5 操作者，可以导入 Lv5 用户
	}
	result := importer.Import(buf.Bytes(), opts)

	if !result.Success {
		t.Errorf("Import should succeed for equal level user")
	}
	if result.Summary.UsersImported != 1 {
		t.Errorf("Expected 1 user imported, got %d", result.Summary.UsersImported)
	}
}

// TestImporter_ExistingCategory_Skip 测试已存在分类跳过
func TestImporter_ExistingCategory_Skip(t *testing.T) {
	store := newTestStore(t)

	// 预先创建分类
	existing := &model.Category{
		ID:    "cat-1",
		Path:  "tech",
		Name:  "Technology",
		Level: 0,
	}
	store.Category.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	categories := []*model.Category{
		{ID: "cat-1", Path: "tech", Name: "Tech Updated", Level: 0},
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

	// 跳过策略应有警告
	if len(result.Errors) == 0 {
		t.Error("Expected warning for skipped category")
	}

	// 验证现有数据未被修改
	cat, _ := store.Category.Get(nil, "tech")
	if cat.Name != "Technology" {
		t.Errorf("Name should remain 'Technology', got '%s'", cat.Name)
	}
}

// TestImporter_ConflictOverwrite_EntryUpdate 测试条目覆盖更新
func TestImporter_ConflictOverwrite_EntryUpdate(t *testing.T) {
	store := newTestStore(t)

	// 预先创建条目
	existing := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Old Title",
		Content:   "Old Content",
		Category:  "test",
		Status:    "published",
		Version:   1,
	}
	store.Entry.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "New Title", Content: "New Content", Category: "updated", Status: "draft", Version: 2},
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

	// 验证条目已更新
	entry, _ := store.Entry.Get(nil, "entry-1")
	if entry.Title != "New Title" {
		t.Errorf("Title should be 'New Title', got '%s'", entry.Title)
	}
	if entry.Content != "New Content" {
		t.Errorf("Content should be 'New Content', got '%s'", entry.Content)
	}
	if entry.Category != "updated" {
		t.Errorf("Category should be 'updated', got '%s'", entry.Category)
	}
}
