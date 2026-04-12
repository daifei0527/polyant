// Package test 提供性能基准测试
package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/index"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// ==================== Entry 性能测试 ====================

// BenchmarkEntryCreate 条目创建性能测试
func BenchmarkEntryCreate(b *testing.B) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		b.Fatalf("NewMemoryStore 失败: %v", err)
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := &model.KnowledgeEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			Title:     "测试条目",
			Content:   "这是一个测试条目的内容，用于性能基准测试。",
			Category:  "test",
			Version:   1,
			Status:    model.EntryStatusPublished,
			CreatedBy: "benchmark",
		}
		store.Entry.Create(ctx, entry)
	}
}

// BenchmarkEntryGet 条目获取性能测试
func BenchmarkEntryGet(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	// 准备测试数据
	for i := 0; i < 1000; i++ {
		entry := &model.KnowledgeEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			Title:     "测试条目",
			Content:   "测试内容",
			Category:  "test",
			Status:    model.EntryStatusPublished,
		}
		store.Entry.Create(ctx, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Entry.Get(ctx, fmt.Sprintf("entry-%d", i%1000))
	}
}

// BenchmarkEntryUpdate 条目更新性能测试
func BenchmarkEntryUpdate(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	// 准备测试数据
	for i := 0; i < 100; i++ {
		entry := &model.KnowledgeEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			Title:     "原始标题",
			Content:   "原始内容",
			Category:  "test",
			Version:   1,
			Status:    model.EntryStatusPublished,
		}
		store.Entry.Create(ctx, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry, _ := store.Entry.Get(ctx, fmt.Sprintf("entry-%d", i%100))
		entry.Title = fmt.Sprintf("更新标题 %d", i)
		entry.Version++
		store.Entry.Update(ctx, entry)
	}
}

// BenchmarkEntryList 条目列表性能测试
func BenchmarkEntryList(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	// 准备测试数据
	for i := 0; i < 1000; i++ {
		entry := &model.KnowledgeEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			Title:     "测试条目",
			Content:   "测试内容",
			Category:  fmt.Sprintf("cat-%d", i%10),
			Status:    model.EntryStatusPublished,
			CreatedBy: fmt.Sprintf("user-%d", i%5),
		}
		store.Entry.Create(ctx, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Entry.List(ctx, storage.EntryFilter{
			Category: fmt.Sprintf("cat-%d", i%10),
			Limit:    20,
		})
	}
}

// ==================== Search 性能测试 ====================

// BenchmarkSearch 搜索性能测试
func BenchmarkSearch(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	// 准备测试数据
	for i := 0; i < 1000; i++ {
		entry := &model.KnowledgeEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			Title:     fmt.Sprintf("条目 %d 编程语言测试", i),
			Content:   fmt.Sprintf("这是第 %d 个条目的内容，包含编程、算法和数据结构相关信息。", i),
			Category:  "tech",
			Status:    model.EntryStatusPublished,
			Score:     float64(i % 5) + 1,
		}
		store.Entry.Create(ctx, entry)
		store.Search.IndexEntry(entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Search.Search(ctx, index.SearchQuery{
			Keyword: "编程",
			Limit:   10,
		})
	}
}

// BenchmarkSearchWithFilter 带过滤器的搜索性能测试
func BenchmarkSearchWithFilter(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	// 准备测试数据
	for i := 0; i < 1000; i++ {
		entry := &model.KnowledgeEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			Title:     fmt.Sprintf("条目 %d", i),
			Content:   fmt.Sprintf("内容 %d", i),
			Category:  fmt.Sprintf("cat-%d", i%5),
			Status:    model.EntryStatusPublished,
			Score:     float64(i%5) + 1,
		}
		store.Entry.Create(ctx, entry)
		store.Search.IndexEntry(entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Search.Search(ctx, index.SearchQuery{
			Categories: []string{"cat-1", "cat-2"},
			MinScore:   3.0,
			Limit:      20,
		})
	}
}

// ==================== User 性能测试 ====================

// BenchmarkUserCreate 用户创建性能测试
func BenchmarkUserCreate(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user := &model.User{
			PublicKey:    fmt.Sprintf("pubkey-%d", i),
			AgentName:    fmt.Sprintf("agent-%d", i),
			UserLevel:    model.UserLevelLv0,
			RegisteredAt: 1000,
			Status:       model.UserStatusActive,
		}
		store.User.Create(ctx, user)
	}
}

// BenchmarkUserGetByEmail 邮箱查询性能测试
func BenchmarkUserGetByEmail(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	// 准备测试数据
	for i := 0; i < 100; i++ {
		user := &model.User{
			PublicKey:    fmt.Sprintf("pubkey-%d", i),
			AgentName:    fmt.Sprintf("agent-%d", i),
			Email:        fmt.Sprintf("user%d@example.com", i),
			RegisteredAt: 1000,
			Status:       model.UserStatusActive,
		}
		store.User.Create(ctx, user)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.User.GetByEmail(ctx, fmt.Sprintf("user%d@example.com", i%100))
	}
}

// ==================== Rating 性能测试 ====================

// BenchmarkRatingCreate 评分创建性能测试
func BenchmarkRatingCreate(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rating := &model.Rating{
			ID:          fmt.Sprintf("rating-%d", i),
			EntryId:     fmt.Sprintf("entry-%d", i%100),
			RaterPubkey: fmt.Sprintf("user-%d", i),
			Score:       float64(i%5) + 1,
			Comment:     "这是一个测试评论",
			RatedAt:     1000,
		}
		store.Rating.Create(ctx, rating)
	}
}

// BenchmarkRatingListByEntry 条目评分列表性能测试
func BenchmarkRatingListByEntry(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	// 准备测试数据
	for i := 0; i < 500; i++ {
		rating := &model.Rating{
			ID:          fmt.Sprintf("rating-%d", i),
			EntryId:     fmt.Sprintf("entry-%d", i%10),
			RaterPubkey: fmt.Sprintf("user-%d", i),
			Score:       float64(i%5) + 1,
			RatedAt:     1000,
		}
		store.Rating.Create(ctx, rating)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Rating.ListByEntry(ctx, fmt.Sprintf("entry-%d", i%10))
	}
}

// ==================== Category 性能测试 ====================

// BenchmarkCategoryCreate 分类创建性能测试
func BenchmarkCategoryCreate(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cat := &model.Category{
			ID:       fmt.Sprintf("cat-%d", i),
			Path:     fmt.Sprintf("category-%d", i),
			Name:     fmt.Sprintf("分类 %d", i),
			ParentId: "",
			Level:    0,
		}
		store.Category.Create(ctx, cat)
	}
}

// BenchmarkCategoryList 分类列表性能测试
func BenchmarkCategoryList(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	// 准备层级分类数据
	for i := 0; i < 100; i++ {
		cat := &model.Category{
			ID:       fmt.Sprintf("cat-%d", i),
			Path:     fmt.Sprintf("cat-%d", i),
			Name:     fmt.Sprintf("分类 %d", i),
			ParentId: "",
			Level:    0,
		}
		store.Category.Create(ctx, cat)

		// 添加子分类
		for j := 0; j < 5; j++ {
			subCat := &model.Category{
				ID:       fmt.Sprintf("cat-%d-%d", i, j),
				Path:     fmt.Sprintf("cat-%d/sub-%d", i, j),
				Name:     fmt.Sprintf("子分类 %d", j),
				ParentId: fmt.Sprintf("cat-%d", i),
				Level:    1,
			}
			store.Category.Create(ctx, subCat)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Category.ListAll(ctx)
	}
}

// ==================== Backlink 性能测试 ====================

// BenchmarkBacklinkUpdate 反向链接更新性能测试
func BenchmarkBacklinkUpdate(b *testing.B) {
	store, _ := storage.NewMemoryStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outlinks := []string{
			fmt.Sprintf("entry-%d", i%10),
			fmt.Sprintf("entry-%d", (i+1)%10),
		}
		store.Backlink.UpdateIndex(fmt.Sprintf("entry-%d", i), outlinks)
	}
}

// BenchmarkBacklinkGet 反向链接查询性能测试
func BenchmarkBacklinkGet(b *testing.B) {
	store, _ := storage.NewMemoryStore()

	// 准备测试数据
	for i := 0; i < 100; i++ {
		outlinks := []string{
			fmt.Sprintf("entry-%d", (i+1)%100),
			fmt.Sprintf("entry-%d", (i+2)%100),
		}
		store.Backlink.UpdateIndex(fmt.Sprintf("entry-%d", i), outlinks)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Backlink.GetBacklinks(fmt.Sprintf("entry-%d", i%100))
	}
}

// ==================== 并发性能测试 ====================

// BenchmarkEntryCreateParallel 并发创建条目
func BenchmarkEntryCreateParallel(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			entry := &model.KnowledgeEntry{
				ID:        fmt.Sprintf("entry-parallel-%d", i),
				Title:     "并发测试条目",
				Content:   "测试内容",
				Category:  "test",
				Status:    model.EntryStatusPublished,
			}
			store.Entry.Create(ctx, entry)
			i++
		}
	})
}

// BenchmarkSearchParallel 并发搜索
func BenchmarkSearchParallel(b *testing.B) {
	store, _ := storage.NewMemoryStore()
	ctx := context.Background()

	// 准备测试数据
	for i := 0; i < 500; i++ {
		entry := &model.KnowledgeEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			Title:     fmt.Sprintf("条目 %d", i),
			Content:   "测试内容",
			Category:  "test",
			Status:    model.EntryStatusPublished,
		}
		store.Entry.Create(ctx, entry)
		store.Search.IndexEntry(entry)
	}

	keywords := []string{"条目", "测试", "内容", "entry", "test"}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			store.Search.Search(ctx, index.SearchQuery{
				Keyword: keywords[i%len(keywords)],
				Limit:   10,
			})
			i++
		}
	})
}
