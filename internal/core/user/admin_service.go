package user

import (
	"context"
	"fmt"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

var (
	ErrCannotBanAdmin = fmt.Errorf("无法封禁管理员")
	ErrInvalidLevel   = fmt.Errorf("无效的用户等级")
)

// AdminService 管理员服务
type AdminService struct {
	store *storage.Store
}

// NewAdminService 创建管理员服务
func NewAdminService(store *storage.Store) *AdminService {
	return &AdminService{store: store}
}

// ListUsers 列出用户
func (s *AdminService) ListUsers(ctx context.Context, offset, limit int, level int32, search string) ([]*model.User, int64, error) {
	filter := storage.UserFilter{
		Offset: offset,
		Limit:  limit,
		Level:  level,
		Search: search,
	}

	users, total, err := s.store.User.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	return users, total, nil
}

// BanUser 封禁用户
func (s *AdminService) BanUser(ctx context.Context, targetPublicKey, adminPublicKey, reason string) error {
	hash := HashPublicKey(targetPublicKey)
	user, err := s.store.User.Get(ctx, hash)
	if err != nil {
		return ErrUserNotFound
	}

	// 不能封禁 Lv4+ 管理员
	if user.UserLevel >= model.UserLevelLv4 {
		return ErrCannotBanAdmin
	}

	user.Status = "banned"
	user.BanReason = reason
	user.BannedAt = time.Now().UnixMilli()
	user.BannedBy = adminPublicKey

	_, err = s.store.User.Update(ctx, user)
	return err
}

// UnbanUser 解封用户
func (s *AdminService) UnbanUser(ctx context.Context, targetPublicKey, adminPublicKey string) error {
	hash := HashPublicKey(targetPublicKey)
	user, err := s.store.User.Get(ctx, hash)
	if err != nil {
		return ErrUserNotFound
	}

	user.Status = "active"
	user.BanReason = ""
	user.BannedAt = 0
	user.BannedBy = ""
	user.UnbannedAt = time.Now().UnixMilli()
	user.UnbannedBy = adminPublicKey

	_, err = s.store.User.Update(ctx, user)
	return err
}

// SetUserLevel 设置用户等级
func (s *AdminService) SetUserLevel(ctx context.Context, targetPublicKey string, newLevel int32, adminPublicKey, reason string) error {
	if newLevel < model.UserLevelLv0 || newLevel > model.UserLevelLv5 {
		return ErrInvalidLevel
	}

	hash := HashPublicKey(targetPublicKey)
	user, err := s.store.User.Get(ctx, hash)
	if err != nil {
		return ErrUserNotFound
	}

	user.UserLevel = newLevel
	user.LevelChangeReason = reason
	user.LevelChangedAt = time.Now().UnixMilli()
	user.LevelChangedBy = adminPublicKey

	_, err = s.store.User.Update(ctx, user)
	return err
}

// GetUserStats 获取用户统计
func (s *AdminService) GetUserStats(ctx context.Context) (*model.UserStats, error) {
	users, total, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	stats := &model.UserStats{
		TotalUsers: total,
	}

	now := time.Now().UnixMilli()
	thirtyDaysAgo := now - 30*24*60*60*1000

	for _, u := range users {
		// 统计各级别用户数
		switch u.UserLevel {
		case model.UserLevelLv0:
			stats.Lv0Count++
		case model.UserLevelLv1:
			stats.Lv1Count++
		case model.UserLevelLv2:
			stats.Lv2Count++
		case model.UserLevelLv3:
			stats.Lv3Count++
		case model.UserLevelLv4:
			stats.Lv4Count++
		case model.UserLevelLv5:
			stats.Lv5Count++
		}

		// 统计活跃用户
		if u.LastActive > thirtyDaysAgo {
			stats.ActiveUsers++
		}

		// 统计被封禁用户
		if u.Status == "banned" {
			stats.BannedCount++
		}

		// 统计总贡献和评分
		stats.TotalContribs += int64(u.ContributionCnt)
		stats.TotalRatings += int64(u.RatingCnt)
	}

	return stats, nil
}
