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
			ContributionCnt: int32(i + 1),
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
