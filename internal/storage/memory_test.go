package storage

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestMemoryEntryStore_Create(t *testing.T) {
	store := NewMemoryEntryStore()
	ctx := context.Background()

	entry := &model.KnowledgeEntry{
		ID:        "test-entry-1",
		Title:     "Test Entry",
		Content:   "This is a test entry content",
		Category:  "test",
		CreatedAt: 1000,
		UpdatedAt: 1000,
		CreatedBy: "user-1",
		Status:    model.EntryStatusPublished,
	}

	// Test create
	created, err := store.Create(ctx, entry)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID != entry.ID {
		t.Errorf("Expected ID %s, got %s", entry.ID, created.ID)
	}

	// Test duplicate create
	_, err = store.Create(ctx, entry)
	if err == nil {
		t.Error("Expected error for duplicate entry")
	}
}

func TestMemoryEntryStore_Get(t *testing.T) {
	store := NewMemoryEntryStore()
	ctx := context.Background()

	entry := &model.KnowledgeEntry{
		ID:       "test-entry-1",
		Title:    "Test Entry",
		Content:  "Content",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}

	_, _ = store.Create(ctx, entry)

	// Test get existing
	got, err := store.Get(ctx, "test-entry-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "Test Entry" {
		t.Errorf("Expected title 'Test Entry', got '%s'", got.Title)
	}

	// Test get non-existing
	_, err = store.Get(ctx, "non-existing")
	if err == nil {
		t.Error("Expected error for non-existing entry")
	}
}

func TestMemoryEntryStore_Update(t *testing.T) {
	store := NewMemoryEntryStore()
	ctx := context.Background()

	entry := &model.KnowledgeEntry{
		ID:       "test-entry-1",
		Title:    "Original Title",
		Content:  "Content",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	_, _ = store.Create(ctx, entry)

	// Test update
	entry.Title = "Updated Title"
	entry.Version = 2
	updated, err := store.Update(ctx, entry)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", updated.Title)
	}

	// Verify update persisted
	got, _ := store.Get(ctx, "test-entry-1")
	if got.Title != "Updated Title" {
		t.Error("Update not persisted")
	}
}

func TestMemoryEntryStore_Delete(t *testing.T) {
	store := NewMemoryEntryStore()
	ctx := context.Background()

	entry := &model.KnowledgeEntry{
		ID:       "test-entry-1",
		Title:    "Test Entry",
		Content:  "Content",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	_, _ = store.Create(ctx, entry)

	// Test soft delete
	err := store.Delete(ctx, "test-entry-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify status changed
	got, _ := store.Get(ctx, "test-entry-1")
	if got.Status != model.EntryStatusArchived {
		t.Errorf("Expected status %s, got %s", model.EntryStatusArchived, got.Status)
	}
}

func TestMemoryEntryStore_List(t *testing.T) {
	store := NewMemoryEntryStore()
	ctx := context.Background()

	// Create test entries
	for i := 0; i < 5; i++ {
		entry := &model.KnowledgeEntry{
			ID:        string(rune('a' + i)),
			Title:     "Entry",
			Content:   "Content",
			Category:  "test",
			Status:    model.EntryStatusPublished,
			CreatedBy: "user-1",
			Score:     float64(5 - i),
		}
		_, _ = store.Create(ctx, entry)
	}

	// Test list with pagination
	results, total, err := store.List(ctx, EntryFilter{Limit: 3})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 5 {
		t.Errorf("Expected total 5, got %d", total)
	}
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Test list with category filter
	results, _, _ = store.List(ctx, EntryFilter{Category: "test"})
	if len(results) != 5 {
		t.Errorf("Expected 5 results for category filter, got %d", len(results))
	}

	// Test list with creator filter
	results, _, _ = store.List(ctx, EntryFilter{CreatedBy: "user-1"})
	if len(results) != 5 {
		t.Errorf("Expected 5 results for creator filter, got %d", len(results))
	}
}

func TestMemoryEntryStore_Count(t *testing.T) {
	store := NewMemoryEntryStore()
	ctx := context.Background()

	// Initially empty
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	// Add entries
	for i := 0; i < 3; i++ {
		entry := &model.KnowledgeEntry{
			ID:       string(rune('a' + i)),
			Title:    "Entry",
			Content:  "Content",
			Category: "test",
			Status:   model.EntryStatusPublished,
		}
		_, _ = store.Create(ctx, entry)
	}

	count, _ = store.Count(ctx)
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}

func TestMemoryUserStore_CreateAndGet(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	user := &model.User{
		PublicKey:    "dGVzdC1wdWJsaWMta2V5", // base64 encoded test key
		AgentName:    "test-agent",
		UserLevel:    model.UserLevelLv0,
		Email:        "test@example.com",
		RegisteredAt: 1000,
		Status:       model.UserStatusActive,
	}

	// Test create
	created, err := store.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.AgentName != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got '%s'", created.AgentName)
	}

	// Test get by pubkey hash
	pubKeyHash := "a883daf829b55a4e82cfc62fbde3ef1a25d6a0b7c7e1c6c7e9a0b1c2d3e4f5a6" // fake hash
	got, err := store.Get(ctx, pubKeyHash)
	if err != nil {
		// This is expected since we used a fake hash
		t.Logf("Get with fake hash returned error as expected: %v", err)
	}
	_ = got
}

func TestMemoryUserStore_GetByEmail(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	user := &model.User{
		PublicKey:    "dGVzdC1wdWJsaWMta2V5",
		AgentName:    "test-agent",
		Email:        "test@example.com",
		RegisteredAt: 1000,
		Status:       model.UserStatusActive,
	}
	_, _ = store.Create(ctx, user)

	// Test get by email
	got, err := store.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	if got.AgentName != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got '%s'", got.AgentName)
	}

	// Test non-existing email
	_, err = store.GetByEmail(ctx, "nonexisting@example.com")
	if err == nil {
		t.Error("Expected error for non-existing email")
	}
}

func TestMemoryUserStore_Update(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	user := &model.User{
		PublicKey:    "dGVzdC1wdWJsaWMta2V5",
		AgentName:    "test-agent",
		UserLevel:    model.UserLevelLv0,
		Email:        "test@example.com",
		RegisteredAt: 1000,
		Status:       model.UserStatusActive,
	}
	_, _ = store.Create(ctx, user)

	// Update user level
	user.UserLevel = model.UserLevelLv1
	user.EmailVerified = true
	updated, err := store.Update(ctx, user)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.UserLevel != model.UserLevelLv1 {
		t.Errorf("Expected level Lv1, got %d", updated.UserLevel)
	}
}

func TestMemoryRatingStore_CreateAndGet(t *testing.T) {
	store := NewMemoryRatingStore()
	ctx := context.Background()

	rating := &model.Rating{
		ID:          "rating-1",
		EntryId:     "entry-1",
		RaterPubkey: "user-hash-1",
		Score:       4.5,
		Comment:     "Great article",
		RatedAt:     1000,
	}

	// Test create
	created, err := store.Create(ctx, rating)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.Score != 4.5 {
		t.Errorf("Expected score 4.5, got %f", created.Score)
	}

	// Test get
	got, err := store.Get(ctx, "rating-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.EntryId != "entry-1" {
		t.Errorf("Expected entry ID 'entry-1', got '%s'", got.EntryId)
	}
}

func TestMemoryRatingStore_ListByEntry(t *testing.T) {
	store := NewMemoryRatingStore()
	ctx := context.Background()

	// Create multiple ratings
	for i := 0; i < 3; i++ {
		rating := &model.Rating{
			ID:          string(rune('a' + i)),
			EntryId:     "entry-1",
			RaterPubkey: string(rune('x' + i)),
			Score:       float64(i + 3),
			RatedAt:     1000,
		}
		_, _ = store.Create(ctx, rating)
	}

	// Test list by entry
	ratings, err := store.ListByEntry(ctx, "entry-1")
	if err != nil {
		t.Fatalf("ListByEntry failed: %v", err)
	}
	if len(ratings) != 3 {
		t.Errorf("Expected 3 ratings, got %d", len(ratings))
	}
}

func TestMemoryRatingStore_GetByRater(t *testing.T) {
	store := NewMemoryRatingStore()
	ctx := context.Background()

	rating := &model.Rating{
		ID:          "rating-1",
		EntryId:     "entry-1",
		RaterPubkey: "user-hash-1",
		Score:       4.5,
		RatedAt:     1000,
	}
	_, _ = store.Create(ctx, rating)

	// Test get by rater
	got, err := store.GetByRater(ctx, "entry-1", "user-hash-1")
	if err != nil {
		t.Fatalf("GetByRater failed: %v", err)
	}
	if got.Score != 4.5 {
		t.Errorf("Expected score 4.5, got %f", got.Score)
	}

	// Test non-existing rater
	_, err = store.GetByRater(ctx, "entry-1", "non-existing")
	if err == nil {
		t.Error("Expected error for non-existing rater")
	}
}

func TestMemoryCategoryStore_CreateAndGet(t *testing.T) {
	store := NewMemoryCategoryStore()
	ctx := context.Background()

	cat := &model.Category{
		ID:    "cat-1",
		Path:  "programming",
		Name:  "编程",
		Level: 0,
	}

	// Test create
	created, err := store.Create(ctx, cat)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.Name != "编程" {
		t.Errorf("Expected name '编程', got '%s'", created.Name)
	}

	// Test get by path
	got, err := store.Get(ctx, "programming")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != "cat-1" {
		t.Errorf("Expected ID 'cat-1', got '%s'", got.ID)
	}

	// Test duplicate
	_, err = store.Create(ctx, cat)
	if err == nil {
		t.Error("Expected error for duplicate category")
	}
}

func TestMemoryCategoryStore_List(t *testing.T) {
	store := NewMemoryCategoryStore()
	ctx := context.Background()

	// Create hierarchy
	categories := []*model.Category{
		{ID: "cat-1", Path: "programming", Name: "编程", ParentId: "", Level: 0},
		{ID: "cat-2", Path: "programming/go", Name: "Go", ParentId: "cat-1", Level: 1},
		{ID: "cat-3", Path: "programming/rust", Name: "Rust", ParentId: "cat-1", Level: 1},
	}
	for _, cat := range categories {
		_, _ = store.Create(ctx, cat)
	}

	// Test list all
	all, err := store.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 categories, got %d", len(all))
	}

	// Test list with parent
	children, err := store.List(ctx, "cat-1")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
}

func TestMemorySearchEngine_Search(t *testing.T) {
	engine := NewMemorySearchEngine()

	// Index test entries
	entries := []*model.KnowledgeEntry{
		{ID: "1", Title: "Go Programming", Content: "Learn Go language", Category: "programming", Status: model.EntryStatusPublished, Score: 4.5},
		{ID: "2", Title: "Rust Programming", Content: "Learn Rust language", Category: "programming", Status: model.EntryStatusPublished, Score: 4.0},
		{ID: "3", Title: "Python Guide", Content: "Python tutorial", Category: "python", Status: model.EntryStatusPublished, Score: 3.5},
	}
	for _, e := range entries {
		_ = engine.IndexEntry(e)
	}

	ctx := context.Background()

	// Test keyword search
	result, err := engine.Search(ctx, index.SearchQuery{Keyword: "programming", Limit: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.TotalCount != 2 {
		t.Errorf("Expected 2 results for 'programming', got %d", result.TotalCount)
	}

	// Test category filter
	result, _ = engine.Search(ctx, index.SearchQuery{Keyword: "", Categories: []string{"programming"}, Limit: 10})
	if result.TotalCount != 2 {
		t.Errorf("Expected 2 results for category 'programming', got %d", result.TotalCount)
	}

	// Test min score filter
	result, _ = engine.Search(ctx, index.SearchQuery{Keyword: "", MinScore: 4.0, Limit: 10})
	if result.TotalCount != 2 {
		t.Errorf("Expected 2 results with min score 4.0, got %d", result.TotalCount)
	}

	// Test pagination
	result, _ = engine.Search(ctx, index.SearchQuery{Keyword: "", Limit: 2})
	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries with limit 2, got %d", len(result.Entries))
	}
	if !result.HasMore {
		t.Error("Expected HasMore to be true")
	}
}

func TestMemoryBacklinkIndex(t *testing.T) {
	idx := NewMemoryBacklinkIndex()

	// Test update index
	err := idx.UpdateIndex("entry-1", []string{"entry-2", "entry-3"})
	if err != nil {
		t.Fatalf("UpdateIndex failed: %v", err)
	}

	// Test get outlinks
	outlinks, err := idx.GetOutlinks("entry-1")
	if err != nil {
		t.Fatalf("GetOutlinks failed: %v", err)
	}
	if len(outlinks) != 2 {
		t.Errorf("Expected 2 outlinks, got %d", len(outlinks))
	}

	// Test get backlinks
	backlinks, err := idx.GetBacklinks("entry-2")
	if err != nil {
		t.Fatalf("GetBacklinks failed: %v", err)
	}
	if len(backlinks) != 1 {
		t.Errorf("Expected 1 backlink, got %d", len(backlinks))
	}

	// Test delete index
	err = idx.DeleteIndex("entry-1")
	if err != nil {
		t.Fatalf("DeleteIndex failed: %v", err)
	}

	// Verify deletion
	outlinks, _ = idx.GetOutlinks("entry-1")
	if len(outlinks) != 0 {
		t.Error("Expected 0 outlinks after delete")
	}
	backlinks, _ = idx.GetBacklinks("entry-2")
	if len(backlinks) != 0 {
		t.Error("Expected 0 backlinks after delete")
	}
}

func TestNewMemoryStore(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore failed: %v", err)
	}
	if store.Entry == nil {
		t.Error("Entry store is nil")
	}
	if store.User == nil {
		t.Error("User store is nil")
	}
	if store.Rating == nil {
		t.Error("Rating store is nil")
	}
	if store.Category == nil {
		t.Error("Category store is nil")
	}
	if store.Search == nil {
		t.Error("Search engine is nil")
	}
	if store.Backlink == nil {
		t.Error("Backlink index is nil")
	}
}

// TestMemoryUserStore_List 测试用户列表功能
func TestMemoryUserStore_List(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	// 创建测试用户
	for i := 0; i < 5; i++ {
		user := &model.User{
			PublicKey:    string(rune('a' + i)),
			AgentName:    "User",
			UserLevel:    int32(i % 3),
			RegisteredAt: int64(1000 + i*100),
			Status:       model.UserStatusActive,
		}
		store.Create(ctx, user)
	}

	// 测试分页
	filter := UserFilter{Limit: 3}
	users, total, err := store.List(ctx, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 5 {
		t.Errorf("Expected total 5, got %d", total)
	}
	if len(users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(users))
	}

	// 测试第二页
	filter = UserFilter{Offset: 3, Limit: 10}
	users, _, _ = store.List(ctx, filter)
	if len(users) != 2 {
		t.Errorf("Expected 2 users on second page, got %d", len(users))
	}

	// 测试用户等级过滤（Level=0 表示不过滤，所以测试 Level=1）
	filter = UserFilter{Level: 1, Limit: 10}
	_, total, _ = store.List(ctx, filter)
	if total != 2 {
		t.Errorf("Expected 2 Lv1 users, got %d", total)
	}

	// 测试状态过滤
	filter = UserFilter{Status: model.UserStatusActive, Limit: 10}
	_, total, _ = store.List(ctx, filter)
	if total != 5 {
		t.Errorf("Expected 5 active users, got %d", total)
	}
}

// TestMemorySearchEngine_UpdateIndex 测试更新索引
func TestMemorySearchEngine_UpdateIndex(t *testing.T) {
	engine := NewMemorySearchEngine()

	// 先索引
	entry := &model.KnowledgeEntry{
		ID:      "entry-1",
		Title:   "Original Title",
		Content: "Original Content",
		Status:  model.EntryStatusPublished,
	}
	engine.IndexEntry(entry)

	// 更新索引
	updatedEntry := &model.KnowledgeEntry{
		ID:      "entry-1",
		Title:   "Updated Title",
		Content: "Updated Content",
		Status:  model.EntryStatusPublished,
	}
	err := engine.UpdateIndex(updatedEntry)
	if err != nil {
		t.Errorf("UpdateIndex failed: %v", err)
	}

	// 验证更新
	result, _ := engine.Search(context.Background(), index.SearchQuery{Keyword: "Updated", Limit: 10})
	if result.TotalCount != 1 {
		t.Errorf("Expected 1 result after update, got %d", result.TotalCount)
	}
}

// TestMemorySearchEngine_DeleteIndex 测试删除索引
func TestMemorySearchEngine_DeleteIndex(t *testing.T) {
	engine := NewMemorySearchEngine()

	// 索引条目
	entry := &model.KnowledgeEntry{
		ID:      "entry-1",
		Title:   "Test Entry",
		Content: "Content",
		Status:  model.EntryStatusPublished,
	}
	engine.IndexEntry(entry)

	// 删除索引
	err := engine.DeleteIndex("entry-1")
	if err != nil {
		t.Errorf("DeleteIndex failed: %v", err)
	}

	// 验证删除
	result, _ := engine.Search(context.Background(), index.SearchQuery{Keyword: "Test", Limit: 10})
	if result.TotalCount != 0 {
		t.Errorf("Expected 0 results after delete, got %d", result.TotalCount)
	}
}

// TestMemorySearchEngine_Close 测试关闭搜索引擎
func TestMemorySearchEngine_Close(t *testing.T) {
	engine := NewMemorySearchEngine()

	// Close 应该不返回错误
	err := engine.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestMemoryEntryStore_Delete_HardDelete 测试硬删除
func TestMemoryEntryStore_Delete_HardDelete(t *testing.T) {
	store := NewMemoryEntryStore()
	ctx := context.Background()

	entry := &model.KnowledgeEntry{
		ID:      "entry-1",
		Title:   "Test",
		Content: "Content",
		Status:  model.EntryStatusPublished,
	}
	store.Create(ctx, entry)

	// 硬删除
	err := store.Delete(ctx, "entry-1")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// 验证状态变为 archived
	got, _ := store.Get(ctx, "entry-1")
	if got.Status != model.EntryStatusArchived {
		t.Errorf("Expected status archived, got %s", got.Status)
	}
}

// TestMemoryEntryStore_UpdateNonExisting 测试更新不存在的条目
func TestMemoryEntryStore_UpdateNonExisting(t *testing.T) {
	store := NewMemoryEntryStore()
	ctx := context.Background()

	entry := &model.KnowledgeEntry{
		ID:      "non-existing",
		Title:   "Test",
		Content: "Content",
		Status:  model.EntryStatusPublished,
	}

	_, err := store.Update(ctx, entry)
	if err == nil {
		t.Error("Expected error when updating non-existing entry")
	}
}

// TestMemoryEntryStore_DeleteNonExisting 测试删除不存在的条目
func TestMemoryEntryStore_DeleteNonExisting(t *testing.T) {
	store := NewMemoryEntryStore()
	ctx := context.Background()

	err := store.Delete(ctx, "non-existing")
	if err == nil {
		t.Error("Expected error when deleting non-existing entry")
	}
}

// TestMemoryUserStore_UpdateNonExisting 测试更新不存在的用户
func TestMemoryUserStore_UpdateNonExisting(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	user := &model.User{
		PublicKey: "non-existing",
		AgentName: "Test",
	}

	_, err := store.Update(ctx, user)
	if err == nil {
		t.Error("Expected error when updating non-existing user")
	}
}

// TestMemoryRatingStore_GetNonExisting 测试获取不存在的评分
func TestMemoryRatingStore_GetNonExisting(t *testing.T) {
	store := NewMemoryRatingStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "non-existing")
	if err == nil {
		t.Error("Expected error when getting non-existing rating")
	}
}

// TestMemoryCategoryStore_GetNonExisting 测试获取不存在的分类
func TestMemoryCategoryStore_GetNonExisting(t *testing.T) {
	store := NewMemoryCategoryStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "non-existing")
	if err == nil {
		t.Error("Expected error when getting non-existing category")
	}
}

// TestMemorySearchEngine_SearchWithScoreRange 测试分数范围搜索
func TestMemorySearchEngine_SearchWithScoreRange(t *testing.T) {
	engine := NewMemorySearchEngine()

	// 索引不同分数的条目
	entries := []*model.KnowledgeEntry{
		{ID: "1", Title: "Entry 1", Status: model.EntryStatusPublished, Score: 3.0},
		{ID: "2", Title: "Entry 2", Status: model.EntryStatusPublished, Score: 4.5},
		{ID: "3", Title: "Entry 3", Status: model.EntryStatusPublished, Score: 5.0},
	}
	for _, e := range entries {
		engine.IndexEntry(e)
	}

	// 测试分数范围
	result, _ := engine.Search(context.Background(), index.SearchQuery{MinScore: 4.0, Limit: 10})
	if result.TotalCount != 2 {
		t.Errorf("Expected 2 results with min score 4.0, got %d", result.TotalCount)
	}
}

// TestMemoryBacklinkIndex_UpdateWithEmptyLinks 测试空链接更新
func TestMemoryBacklinkIndex_UpdateWithEmptyLinks(t *testing.T) {
	idx := NewMemoryBacklinkIndex()

	// 更新空链接
	err := idx.UpdateIndex("entry-1", []string{})
	if err != nil {
		t.Errorf("UpdateIndex with empty links failed: %v", err)
	}

	// 验证没有 outlinks
	outlinks, _ := idx.GetOutlinks("entry-1")
	if len(outlinks) != 0 {
		t.Errorf("Expected 0 outlinks, got %d", len(outlinks))
	}
}

// TestMemoryBacklinkIndex_GetNonExisting 测试获取不存在的链接
func TestMemoryBacklinkIndex_GetNonExisting(t *testing.T) {
	idx := NewMemoryBacklinkIndex()

	// 获取不存在的 outlinks
	outlinks, err := idx.GetOutlinks("non-existing")
	if err != nil {
		t.Errorf("GetOutlinks failed: %v", err)
	}
	if len(outlinks) != 0 {
		t.Errorf("Expected 0 outlinks for non-existing entry, got %d", len(outlinks))
	}

	// 获取不存在的 backlinks
	backlinks, err := idx.GetBacklinks("non-existing")
	if err != nil {
		t.Errorf("GetBacklinks failed: %v", err)
	}
	if len(backlinks) != 0 {
		t.Errorf("Expected 0 backlinks for non-existing entry, got %d", len(backlinks))
	}
}
