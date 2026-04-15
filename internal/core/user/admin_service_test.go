package user

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestAdminService_ListUsers(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()

	// 创建测试用户
	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", UserLevel: model.UserLevelLv2})

	users, total, err := service.ListUsers(ctx, 0, 10, 0, "")
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected total 2, got %d", total)
	}
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestAdminService_ListUsers_WithFilter(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()

	// 创建测试用户
	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", UserLevel: model.UserLevelLv2})
	store.User.Create(ctx, &model.User{PublicKey: "pk3", AgentName: "User3", UserLevel: model.UserLevelLv1})

	// 测试等级过滤
	users, total, err := service.ListUsers(ctx, 0, 10, model.UserLevelLv1, "")
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected total 2 for Lv1, got %d", total)
	}
	if len(users) != 2 {
		t.Errorf("Expected 2 Lv1 users, got %d", len(users))
	}

	// 测试搜索过滤
	users, total, err = service.ListUsers(ctx, 0, 10, 0, "User1")
	if err != nil {
		t.Fatalf("ListUsers with search failed: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected total 1 for search 'User1', got %d", total)
	}
}

func TestAdminService_BanUser(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()
	user, _ := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1})

	err := service.BanUser(ctx, user.PublicKey, "admin1", "违规操作", model.BanTypeFull)
	if err != nil {
		t.Fatalf("BanUser failed: %v", err)
	}

	updated, _ := store.User.Get(ctx, HashPublicKey(user.PublicKey))
	if updated.Status != model.UserStatusBanned {
		t.Errorf("Expected status %q, got %s", model.UserStatusBanned, updated.Status)
	}
	if updated.BanReason != "违规操作" {
		t.Errorf("Expected ban reason '违规操作', got %s", updated.BanReason)
	}
	if updated.BannedBy != "admin1" {
		t.Errorf("Expected banned by 'admin1', got %s", updated.BannedBy)
	}
	if updated.BanType != model.BanTypeFull {
		t.Errorf("Expected BanType %q, got %s", model.BanTypeFull, updated.BanType)
	}
}

func TestAdminService_BanUser_Readonly(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()
	user, _ := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1})

	err := service.BanUser(ctx, user.PublicKey, "admin1", "轻度违规", model.BanTypeReadonly)
	if err != nil {
		t.Fatalf("BanUser failed: %v", err)
	}

	updated, _ := store.User.Get(ctx, HashPublicKey(user.PublicKey))
	if updated.Status != model.UserStatusBanned {
		t.Errorf("Expected status %q, got %s", model.UserStatusBanned, updated.Status)
	}
	if updated.BanType != model.BanTypeReadonly {
		t.Errorf("Expected BanType %q, got %s", model.BanTypeReadonly, updated.BanType)
	}
	if !updated.IsReadOnly() {
		t.Error("User should be in readonly mode")
	}
}

func TestAdminService_BanUser_CannotBanAdmin(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()
	user, _ := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "Admin", UserLevel: model.UserLevelLv4})

	err := service.BanUser(ctx, user.PublicKey, "admin1", "尝试封禁", model.BanTypeFull)
	if err != ErrCannotBanAdmin {
		t.Errorf("Expected ErrCannotBanAdmin, got %v", err)
	}
}

func TestAdminService_BanUser_NotFound(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()

	err := service.BanUser(ctx, "nonexistent", "admin1", "reason", model.BanTypeFull)
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestAdminService_UnbanUser(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()
	user, _ := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1, Status: "banned"})

	err := service.UnbanUser(ctx, user.PublicKey, "admin1")
	if err != nil {
		t.Fatalf("UnbanUser failed: %v", err)
	}

	updated, _ := store.User.Get(ctx, HashPublicKey(user.PublicKey))
	if updated.Status != "active" {
		t.Errorf("Expected status 'active', got %s", updated.Status)
	}
	if updated.UnbannedBy != "admin1" {
		t.Errorf("Expected unbanned by 'admin1', got %s", updated.UnbannedBy)
	}
}

func TestAdminService_SetUserLevel(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()
	user, _ := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1})

	err := service.SetUserLevel(ctx, user.PublicKey, model.UserLevelLv3, "admin1", "贡献突出")
	if err != nil {
		t.Fatalf("SetUserLevel failed: %v", err)
	}

	updated, _ := store.User.Get(ctx, HashPublicKey(user.PublicKey))
	if updated.UserLevel != model.UserLevelLv3 {
		t.Errorf("Expected level Lv3, got %d", updated.UserLevel)
	}
	if updated.LevelChangeReason != "贡献突出" {
		t.Errorf("Expected reason '贡献突出', got %s", updated.LevelChangeReason)
	}
	if updated.LevelChangedBy != "admin1" {
		t.Errorf("Expected changed by 'admin1', got %s", updated.LevelChangedBy)
	}
}

func TestAdminService_SetUserLevel_InvalidLevel(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()
	user, _ := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1})

	err := service.SetUserLevel(ctx, user.PublicKey, 10, "admin1", "invalid")
	if err != ErrInvalidLevel {
		t.Errorf("Expected ErrInvalidLevel, got %v", err)
	}
}

func TestAdminService_SetUserLevel_NotFound(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()

	err := service.SetUserLevel(ctx, "nonexistent", model.UserLevelLv2, "admin1", "reason")
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestAdminService_GetUserStats(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()
	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv0})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", UserLevel: model.UserLevelLv1})
	store.User.Create(ctx, &model.User{PublicKey: "pk3", AgentName: "User3", UserLevel: model.UserLevelLv2})

	stats, err := service.GetUserStats(ctx)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}
	if stats.TotalUsers != 3 {
		t.Errorf("Expected total users 3, got %d", stats.TotalUsers)
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
}

func TestAdminService_GetUserStats_WithBanned(t *testing.T) {
	store := newTestStore(t)
	service := NewAdminService(store)

	ctx := context.Background()
	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1, Status: "active"})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", UserLevel: model.UserLevelLv1, Status: "banned"})

	stats, err := service.GetUserStats(ctx)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}
	if stats.TotalUsers != 2 {
		t.Errorf("Expected total users 2, got %d", stats.TotalUsers)
	}
	if stats.BannedCount != 1 {
		t.Errorf("Expected BannedCount 1, got %d", stats.BannedCount)
	}
}
