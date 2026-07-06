package user

import (
	"context"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestStatsService_GetUserStats(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	// Create users at different levels
	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv0})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", UserLevel: model.UserLevelLv1})
	store.User.Create(ctx, &model.User{PublicKey: "pk3", AgentName: "User3", UserLevel: model.UserLevelLv2})
	store.User.Create(ctx, &model.User{PublicKey: "pk4", AgentName: "User4", UserLevel: model.UserLevelLv3})
	store.User.Create(ctx, &model.User{PublicKey: "pk5", AgentName: "User5", UserLevel: model.UserLevelLv4})
	store.User.Create(ctx, &model.User{PublicKey: "pk6", AgentName: "User6", UserLevel: model.UserLevelLv5})

	stats, err := service.GetUserStats(ctx)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}

	if stats.TotalUsers != 6 {
		t.Errorf("Expected TotalUsers 6, got %d", stats.TotalUsers)
	}
	if stats.Lv0Count != 1 {
		t.Errorf("Expected Lv0Count 1, got %d", stats.Lv0Count)
	}
	if stats.Lv1Count != 1 {
		t.Errorf("Expected Lv1Count 1, got %d", stats.Lv1Count)
	}
	if stats.Lv2Count != 1 {
		t.Errorf("Expected Lv2Count 1, got %d", stats.Lv2Count)
	}
	if stats.Lv3Count != 1 {
		t.Errorf("Expected Lv3Count 1, got %d", stats.Lv3Count)
	}
	if stats.Lv4Count != 1 {
		t.Errorf("Expected Lv4Count 1, got %d", stats.Lv4Count)
	}
	if stats.Lv5Count != 1 {
		t.Errorf("Expected Lv5Count 1, got %d", stats.Lv5Count)
	}
}

func TestStatsService_GetUserStats_ActiveUsers(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	// Create user active within 30 days
	now := time.Now().UnixMilli()
	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "Active", LastActive: now})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "Inactive", LastActive: now - 31*24*60*60*1000})

	stats, err := service.GetUserStats(ctx)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}

	if stats.ActiveUsers != 1 {
		t.Errorf("Expected ActiveUsers 1, got %d", stats.ActiveUsers)
	}
}

func TestStatsService_GetUserStats_BannedUsers(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "Active1", Status: model.UserStatusActive})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "Banned", Status: model.UserStatusBanned})

	stats, err := service.GetUserStats(ctx)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}

	if stats.BannedCount != 1 {
		t.Errorf("Expected BannedCount 1, got %d", stats.BannedCount)
	}
}

func TestStatsService_GetUserStats_Contributions(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", ContributionCnt: 10, RatingCnt: 5})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", ContributionCnt: 20, RatingCnt: 15})

	stats, err := service.GetUserStats(ctx)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}

	if stats.TotalContribs != 30 {
		t.Errorf("Expected TotalContribs 30, got %d", stats.TotalContribs)
	}
	if stats.TotalRatings != 20 {
		t.Errorf("Expected TotalRatings 20, got %d", stats.TotalRatings)
	}
}

func TestStatsService_GetContributionStats(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", ContributionCnt: 10, RatingCnt: 5})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", ContributionCnt: 20, RatingCnt: 15})

	contribs, total, err := service.GetContributionStats(ctx, 0, 10, "entry_count")
	if err != nil {
		t.Fatalf("GetContributionStats failed: %v", err)
	}

	if total != 2 {
		t.Errorf("Expected total 2, got %d", total)
	}
	if len(contribs) != 2 {
		t.Errorf("Expected 2 contributions, got %d", len(contribs))
	}

	// Should be sorted by entry count (descending)
	if contribs[0].EntryCount < contribs[1].EntryCount {
		t.Error("Contributions should be sorted by entry_count descending")
	}
}

func TestStatsService_GetContributionStats_SortByRating(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", ContributionCnt: 10, RatingCnt: 20})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", ContributionCnt: 20, RatingCnt: 5})

	contribs, _, err := service.GetContributionStats(ctx, 0, 10, "rating_given_count")
	if err != nil {
		t.Fatalf("GetContributionStats failed: %v", err)
	}

	// Should be sorted by rating_given_count (descending)
	if contribs[0].RatingGivenCount < contribs[1].RatingGivenCount {
		t.Error("Contributions should be sorted by rating_given_count descending")
	}
}

func TestStatsService_GetContributionStats_Pagination(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	// Create 5 users
	for i := 0; i < 5; i++ {
		store.User.Create(ctx, &model.User{
			PublicKey:       string(rune('a' + i)),
			AgentName:       "User",
			ContributionCnt: int32(i + 1), //nolint:gosec // 测试中 i 范围极小，不会溢出
		})
	}

	// Get first page
	contribs, total, err := service.GetContributionStats(ctx, 0, 2, "entry_count")
	if err != nil {
		t.Fatalf("GetContributionStats failed: %v", err)
	}

	if total != 5 {
		t.Errorf("Expected total 5, got %d", total)
	}
	if len(contribs) != 2 {
		t.Errorf("Expected 2 contributions on first page, got %d", len(contribs))
	}

	// Get second page
	contribs, _, err = service.GetContributionStats(ctx, 2, 2, "entry_count")
	if err != nil {
		t.Fatalf("GetContributionStats failed: %v", err)
	}

	if len(contribs) != 2 {
		t.Errorf("Expected 2 contributions on second page, got %d", len(contribs))
	}
}

func TestStatsService_GetContributionStats_OffsetTooLarge(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1"})

	contribs, _, err := service.GetContributionStats(ctx, 100, 10, "entry_count")
	if err != nil {
		t.Fatalf("GetContributionStats failed: %v", err)
	}

	if len(contribs) != 0 {
		t.Errorf("Expected empty result for large offset, got %d", len(contribs))
	}
}

func TestStatsService_GetActivityTrend(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	// Create user active today
	now := time.Now()
	store.User.Create(ctx, &model.User{
		PublicKey:    "pk1",
		AgentName:    "Active",
		LastActive:   now.UnixMilli(),
		RegisteredAt: now.UnixMilli(),
	})

	trend, err := service.GetActivityTrend(ctx, 7)
	if err != nil {
		t.Fatalf("GetActivityTrend failed: %v", err)
	}

	if len(trend) != 7 {
		t.Errorf("Expected 7 days of trend, got %d", len(trend))
	}

	// Most recent day should have 1 active user
	if trend[6].DAU != 1 {
		t.Errorf("Expected DAU 1 for today, got %d", trend[6].DAU)
	}
	if trend[6].NewUsers != 1 {
		t.Errorf("Expected NewUsers 1 for today, got %d", trend[6].NewUsers)
	}
}

func TestStatsService_GetRegistrationTrend(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	// Create user registered today
	now := time.Now()
	store.User.Create(ctx, &model.User{
		PublicKey:    "pk1",
		AgentName:    "NewUser",
		RegisteredAt: now.UnixMilli(),
	})

	trend, err := service.GetRegistrationTrend(ctx, 30)
	if err != nil {
		t.Fatalf("GetRegistrationTrend failed: %v", err)
	}

	if len(trend) != 30 {
		t.Errorf("Expected 30 days of trend, got %d", len(trend))
	}

	// Total should equal total users
	if trend[29].Total != 1 {
		t.Errorf("Expected Total 1 for today, got %d", trend[29].Total)
	}
}

func TestStatsService_GetActivityTrend_EmptyStore(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	trend, err := service.GetActivityTrend(ctx, 7)
	if err != nil {
		t.Fatalf("GetActivityTrend failed: %v", err)
	}

	if len(trend) != 7 {
		t.Errorf("Expected 7 days of trend, got %d", len(trend))
	}

	// All DAU should be 0
	for i, day := range trend {
		if day.DAU != 0 {
			t.Errorf("Day %d: Expected DAU 0, got %d", i, day.DAU)
		}
	}
}

func TestStatsService_GetRegistrationTrend_EmptyStore(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)

	ctx := context.Background()

	trend, err := service.GetRegistrationTrend(ctx, 7)
	if err != nil {
		t.Fatalf("GetRegistrationTrend failed: %v", err)
	}

	if len(trend) != 7 {
		t.Errorf("Expected 7 days of trend, got %d", len(trend))
	}

	// All counts should be 0
	for i, day := range trend {
		if day.Count != 0 {
			t.Errorf("Day %d: Expected Count 0, got %d", i, day.Count)
		}
	}
}

// TestStatsService_CacheTTL 验证 TTL 缓存：TTL 内命中陈旧快照，过期后重算。
func TestStatsService_CacheTTL(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)
	service.SetCacheTTL(100 * time.Millisecond) // 短 TTL 便于测试
	ctx := context.Background()

	if _, err := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "U1", UserLevel: model.UserLevelLv0}); err != nil {
		t.Fatal(err)
	}
	stats1, err := service.GetUserStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats1.TotalUsers != 1 {
		t.Fatalf("initial TotalUsers = %d, want 1", stats1.TotalUsers)
	}

	// TTL 内再创建用户：缓存命中，应仍返回陈旧的 1（证明未重算）
	if _, err := store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "U2", UserLevel: model.UserLevelLv0}); err != nil {
		t.Fatal(err)
	}
	stats2, err := service.GetUserStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats2.TotalUsers != 1 {
		t.Errorf("within TTL cache should serve stale 1, got %d", stats2.TotalUsers)
	}

	// TTL 过期后应重算为 2
	time.Sleep(120 * time.Millisecond)
	stats3, err := service.GetUserStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats3.TotalUsers != 2 {
		t.Errorf("after TTL expiry should recompute to 2, got %d", stats3.TotalUsers)
	}
}

// TestStatsService_CacheDisabled 验证 TTL<=0 禁用缓存（每次重算）。
func TestStatsService_CacheDisabled(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)
	service.SetCacheTTL(0) // 禁用
	ctx := context.Background()

	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "U1", UserLevel: model.UserLevelLv0})
	if s, _ := service.GetUserStats(ctx); s.TotalUsers != 1 {
		t.Fatal("first call")
	}
	// 禁用缓存 → 立即反映新用户
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "U2", UserLevel: model.UserLevelLv0})
	if s, _ := service.GetUserStats(ctx); s.TotalUsers != 2 {
		t.Errorf("cache disabled: expected immediate 2, got %d", s.TotalUsers)
	}
}

// TestStatsService_GetEntryStats 验证条目维度统计聚合（R3-E）。
func TestStatsService_GetEntryStats(t *testing.T) {
	store := newTestStore(t)
	service := NewStatsService(store)
	service.SetCacheTTL(0) // 禁用缓存，确保实时聚合
	ctx := context.Background()

	mustEntry := func(id, cat string, status string, score float64) {
		_, err := store.Entry.Create(ctx, &model.KnowledgeEntry{
			ID: id, Title: "t", Content: "c", Category: cat, Status: status, Score: score,
		})
		if err != nil {
			t.Fatalf("create entry %s: %v", id, err)
		}
	}
	mustEntry("e1", "ai", model.EntryStatusPublished, 4.5)
	mustEntry("e2", "ai", model.EntryStatusPublished, 3.5)
	mustEntry("e3", "math", model.EntryStatusDraft, 0)
	mustEntry("e4", "ai", model.EntryStatusArchived, 2.0)

	stats, err := service.GetEntryStats(ctx)
	if err != nil {
		t.Fatalf("GetEntryStats failed: %v", err)
	}

	if stats.TotalEntries != 4 {
		t.Errorf("TotalEntries: got %d want 4", stats.TotalEntries)
	}
	if stats.PublishedCount != 2 {
		t.Errorf("PublishedCount: got %d want 2", stats.PublishedCount)
	}
	if stats.DraftCount != 1 {
		t.Errorf("DraftCount: got %d want 1", stats.DraftCount)
	}
	if stats.ArchivedCount != 1 {
		t.Errorf("ArchivedCount: got %d want 1", stats.ArchivedCount)
	}
	if len(stats.TopCategories) == 0 || stats.TopCategories[0].Category != "ai" || stats.TopCategories[0].Count != 3 {
		t.Errorf("TopCategories[0]: got %+v want {ai 3}", stats.TopCategories)
	}
	if stats.ScoreBuckets["4-5"] != 1 || stats.ScoreBuckets["3-4"] != 1 || stats.ScoreBuckets["2-3"] != 1 {
		t.Errorf("ScoreBuckets: got %+v want 4-5=1,3-4=1,2-3=1", stats.ScoreBuckets)
	}
	if stats.ScoreBuckets["0-1"] != 0 {
		t.Errorf("ScoreBuckets[0-1]: got %d want 0 (score=0 不入桶)", stats.ScoreBuckets["0-1"])
	}
}
