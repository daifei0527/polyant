package index

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBleveEngine_IndexAndSearch(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	engine, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}
	defer engine.Close()

	// 索引测试条目
	entry := &model.KnowledgeEntry{
		ID:       "entry-1",
		Title:    "Go 语言编程",
		Content:  "Go 是一种开源编程语言，由 Google 开发",
		Category: "tech/programming",
		Tags:     []string{"go", "programming"},
		Status:   model.EntryStatusPublished,
		Score:    4.5,
	}

	if err := engine.IndexEntry(entry); err != nil {
		t.Fatalf("IndexEntry failed: %v", err)
	}

	// 搜索
	result, err := engine.Search(context.Background(), SearchQuery{
		Keyword: "Go 语言",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if result.TotalCount == 0 {
		t.Error("Search should find the indexed entry")
	}
}

func TestBleveEngine_SearchWithCategory(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	engine, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}
	defer engine.Close()

	// 索引多个条目
	entries := []*model.KnowledgeEntry{
		{ID: "1", Title: "Go 编程", Content: "Go 语言教程", Category: "tech/go", Status: model.EntryStatusPublished},
		{ID: "2", Title: "Python 编程", Content: "Python 语言教程", Category: "tech/python", Status: model.EntryStatusPublished},
		{ID: "3", Title: "烹饪教程", Content: "学习烹饪", Category: "life/cooking", Status: model.EntryStatusPublished},
	}

	for _, e := range entries {
		engine.IndexEntry(e)
	}

	// 搜索并按分类过滤
	result, err := engine.Search(context.Background(), SearchQuery{
		Keyword:    "编程",
		Categories: []string{"tech"},
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if result.TotalCount != 2 {
		t.Errorf("Search with category filter should return 2 results, got %d", result.TotalCount)
	}
}

func TestBleveEngine_UpdateIndex(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	engine, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}
	defer engine.Close()

	// 初始索引
	entry := &model.KnowledgeEntry{
		ID:      "entry-1",
		Title:   "原标题",
		Content: "原内容",
		Status:  model.EntryStatusPublished,
	}
	engine.IndexEntry(entry)

	// 更新索引
	entry.Title = "更新后的标题"
	entry.Content = "更新后的内容"
	if err := engine.UpdateIndex(entry); err != nil {
		t.Fatalf("UpdateIndex failed: %v", err)
	}

	// 搜索更新后的内容
	result, err := engine.Search(context.Background(), SearchQuery{
		Keyword: "更新",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if result.TotalCount == 0 {
		t.Error("Search should find the updated entry")
	}
}

func TestBleveEngine_DeleteIndex(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	engine, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}
	defer engine.Close()

	// 索引并删除
	entry := &model.KnowledgeEntry{
		ID:      "entry-1",
		Title:   "测试条目",
		Content: "测试内容",
		Status:  model.EntryStatusPublished,
	}
	engine.IndexEntry(entry)

	if err := engine.DeleteIndex("entry-1"); err != nil {
		t.Fatalf("DeleteIndex failed: %v", err)
	}

	// 搜索不应找到已删除条目
	result, err := engine.Search(context.Background(), SearchQuery{
		Keyword: "测试条目",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if result.TotalCount != 0 {
		t.Error("Deleted entry should not be found")
	}
}

func TestBleveEngine_Persistence(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	// 创建并索引
	engine1, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}

	entry := &model.KnowledgeEntry{
		ID:      "entry-1",
		Title:   "持久化测试",
		Content: "测试内容",
		Status:  model.EntryStatusPublished,
	}
	engine1.IndexEntry(entry)
	engine1.Close()

	// 重新打开验证持久化
	engine2, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine on reopen failed: %v", err)
	}
	defer engine2.Close()

	result, err := engine2.Search(context.Background(), SearchQuery{
		Keyword: "持久化",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if result.TotalCount == 0 {
		t.Error("Persisted index should be searchable")
	}
}

func TestBleveEngine_ChineseSearch(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	engine, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}
	defer engine.Close()

	// 中文分词测试
	entry := &model.KnowledgeEntry{
		ID:      "entry-1",
		Title:   "人工智能发展现状",
		Content: "机器学习是人工智能的重要分支，深度学习在自然语言处理领域取得突破",
		Status:  model.EntryStatusPublished,
	}
	engine.IndexEntry(entry)

	// 测试中文关键词搜索
	tests := []struct {
		keyword string
		wantHit bool
	}{
		{"人工智能", true},
		{"机器学习", true},
		{"深度学习", true},
		{"自然语言处理", true},
		{"不存在的关键词", false},
	}

	for _, tt := range tests {
		result, err := engine.Search(context.Background(), SearchQuery{
			Keyword: tt.keyword,
			Limit:   10,
		})
		if err != nil {
			t.Fatalf("Search failed for %s: %v", tt.keyword, err)
		}

		if (result.TotalCount > 0) != tt.wantHit {
			t.Errorf("Search %s: got hits=%d, want hit=%v", tt.keyword, result.TotalCount, tt.wantHit)
		}
	}
}

func TestBleveEngine_MinScore(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	engine, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}
	defer engine.Close()

	// 索引不同评分的条目
	entries := []*model.KnowledgeEntry{
		{ID: "1", Title: "高质量文章", Content: "优秀内容", Score: 4.5, Status: model.EntryStatusPublished},
		{ID: "2", Title: "普通文章", Content: "普通内容", Score: 3.0, Status: model.EntryStatusPublished},
		{ID: "3", Title: "低质量文章", Content: "较差内容", Score: 2.0, Status: model.EntryStatusPublished},
	}

	for _, e := range entries {
		engine.IndexEntry(e)
	}

	// 搜索并过滤低评分
	result, err := engine.Search(context.Background(), SearchQuery{
		Keyword:  "文章",
		MinScore: 3.5,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// 只应该返回评分 >= 3.5 的条目
	for _, e := range result.Entries {
		if e.Score < 3.5 {
			t.Errorf("Should not return entries with score < 3.5, got score=%.1f", e.Score)
		}
	}
}

func TestBleveEngine_Pagination(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	engine, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}
	defer engine.Close()

	// 索引多个条目
	for i := 0; i < 15; i++ {
		entry := &model.KnowledgeEntry{
			ID:      string(rune('a' + i)),
			Title:   "测试文章",
			Content: "内容",
			Status:  model.EntryStatusPublished,
		}
		engine.IndexEntry(entry)
	}

	// 测试分页
	result1, err := engine.Search(context.Background(), SearchQuery{
		Keyword: "测试",
		Limit:   5,
		Offset:  0,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(result1.Entries) != 5 {
		t.Errorf("First page should have 5 entries, got %d", len(result1.Entries))
	}
	if !result1.HasMore {
		t.Error("HasMore should be true when there are more results")
	}

	// 第二页
	result2, err := engine.Search(context.Background(), SearchQuery{
		Keyword: "测试",
		Limit:   5,
		Offset:  5,
	})
	if err != nil {
		t.Fatalf("Search page 2 failed: %v", err)
	}

	if len(result2.Entries) != 5 {
		t.Errorf("Second page should have 5 entries, got %d", len(result2.Entries))
	}

	// 确保两个页面的条目不同
	if result1.Entries[0].ID == result2.Entries[0].ID {
		t.Error("Different pages should return different entries")
	}
}

func TestBleveEngine_EmptyQuery(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	engine, err := NewBleveEngine(indexPath)
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}
	defer engine.Close()

	// 索引测试条目
	entry := &model.KnowledgeEntry{
		ID:      "entry-1",
		Title:   "测试文章",
		Content: "测试内容",
		Status:  model.EntryStatusPublished,
	}
	engine.IndexEntry(entry)

	// 空关键词搜索
	result, err := engine.Search(context.Background(), SearchQuery{
		Keyword: "",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// 空关键词应返回空结果
	if result.TotalCount != 0 {
		t.Errorf("Empty query should return 0 results, got %d", result.TotalCount)
	}
}

// TestBleveEngine_SearchI18n: 搜索应能经由本地化（i18n）字段命中条目——
// 主文本为英文，但中文关键词仅存在于 TitleI18n/ContentI18n，通过 all_text 命中。
func TestBleveEngine_SearchI18n(t *testing.T) {
	dir := t.TempDir()
	engine, err := NewBleveEngine(filepath.Join(dir, "test.bleve"))
	if err != nil {
		t.Fatalf("NewBleveEngine failed: %v", err)
	}
	defer engine.Close()

	entry := &model.KnowledgeEntry{
		ID:          "entry-i18n",
		Title:       "Go Programming",
		Content:     "Go is an open source programming language",
		TitleI18n:   map[string]string{"zh-CN": "Go 语言编程"},
		ContentI18n: map[string]string{"zh-CN": "Go 是一种开源编程语言"},
		Category:    "tech",
		Status:      model.EntryStatusPublished,
	}
	if err := engine.IndexEntry(entry); err != nil {
		t.Fatalf("IndexEntry failed: %v", err)
	}

	// "开源" 仅出现在中文本地化内容中
	result, err := engine.Search(context.Background(), SearchQuery{
		Keyword: "开源",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.TotalCount == 0 {
		t.Error("Search should hit the entry via localized content indexed in all_text")
	}
}

// TestBleveEngine_Rebuild_FixesStaleIndex 验证索引陈旧/损坏时 Rebuild 后内容与给定 entries 一致。
func TestBleveEngine_Rebuild_FixesStaleIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bleve")

	// 第一次：索引一个会随后被"丢弃"的陈旧条目 + 一个 published
	e1 := &model.KnowledgeEntry{ID: "e1", Title: "alpha", Content: "alpha", Category: "c", Status: model.EntryStatusPublished, CreatedBy: "x"}
	e1.ContentHash = e1.ComputeContentHash()
	stale := &model.KnowledgeEntry{ID: "stale", Title: "beta gone", Content: "beta", Category: "c", Status: model.EntryStatusPublished, CreatedBy: "x"}
	stale.ContentHash = stale.ComputeContentHash()

	eng1, err := NewBleveEngine(path)
	require.NoError(t, err)
	require.NoError(t, eng1.IndexEntry(e1))
	require.NoError(t, eng1.IndexEntry(stale))
	require.NoError(t, eng1.Close())

	// 第二次：reopen，调 Rebuild 只喂 e1（模拟 stale 已从 store 删除）
	eng2, err := NewBleveEngine(path)
	require.NoError(t, err)
	defer eng2.Close()
	require.NoError(t, eng2.Rebuild([]*model.KnowledgeEntry{e1}))

	// 陈旧条目搜不到
	res, err := eng2.Search(context.Background(), SearchQuery{Keyword: "beta"})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount, "stale entry should be gone after rebuild")

	// e1 仍可搜
	res2, err := eng2.Search(context.Background(), SearchQuery{Keyword: "alpha"})
	require.NoError(t, err)
	assert.Equal(t, 1, res2.TotalCount, "e1 should be searchable after rebuild")

	// 自检：DocCount == entries 数
	cnt, err := eng2.IndexCount()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), cnt)
}

// TestBleveEngine_SearchFiltersDraftStatus 验证 draft/archived 条目不被搜出。
func TestBleveEngine_SearchFiltersDraftStatus(t *testing.T) {
	eng, err := NewBleveEngine(filepath.Join(t.TempDir(), "t.bleve"))
	require.NoError(t, err)
	defer eng.Close()

	pub := &model.KnowledgeEntry{ID: "pub", Title: "shared keyword", Content: "x", Category: "c", Status: model.EntryStatusPublished, CreatedBy: "x"}
	pub.ContentHash = pub.ComputeContentHash()
	draft := &model.KnowledgeEntry{ID: "draft", Title: "shared keyword", Content: "x", Category: "c", Status: model.EntryStatusDraft, CreatedBy: "x"}
	draft.ContentHash = draft.ComputeContentHash()
	require.NoError(t, eng.IndexEntry(pub))
	require.NoError(t, eng.IndexEntry(draft))

	res, err := eng.Search(context.Background(), SearchQuery{Keyword: "shared"})
	require.NoError(t, err)
	assert.Equal(t, 1, res.TotalCount, "only published entry should be searchable; draft filtered")
}
