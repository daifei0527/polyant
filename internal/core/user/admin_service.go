package user

import (
	"context"
	"fmt"
	"time"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/daifei0527/polyant/pkg/crypto"
)

var (
	ErrCannotBanAdmin = fmt.Errorf("无法封禁管理员")
	ErrInvalidLevel   = fmt.Errorf("无效的用户等级")
)

// AdminService 管理员服务
type AdminService struct {
	store *storage.Store
	stats *StatsService // GetUserStats 委托给它（去重 + 共享 StatsService 的缓存）
}

// NewAdminService 创建管理员服务
func NewAdminService(store *storage.Store) *AdminService {
	return &AdminService{store: store, stats: NewStatsService(store)}
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
func (s *AdminService) BanUser(ctx context.Context, targetPublicKey, adminPublicKey, reason string, banType model.BanType) error {
	hash := HashPublicKey(targetPublicKey)
	user, err := s.store.User.Get(ctx, hash)
	if err != nil {
		return ErrUserNotFound
	}

	// 不能封禁 Lv4+ 管理员
	if user.UserLevel >= model.UserLevelLv4 {
		return ErrCannotBanAdmin
	}

	// 设置封禁状态
	user.Status = model.UserStatusBanned
	user.BanType = banType
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

	user.Status = model.UserStatusActive
	user.BanType = ""
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

// ErrPasswordTooShort 密码过短
var ErrPasswordTooShort = fmt.Errorf("密码至少 8 位")

// SetPassword 设置/重置目标用户的 Web admin 登录密码（bcrypt 哈希存储）。
// 密码至少 8 位。供 pactl（Ed25519 签名 + ManageUser 权限）调用。
func (s *AdminService) SetPassword(ctx context.Context, targetPublicKey, password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	hash := HashPublicKey(targetPublicKey)
	user, err := s.store.User.Get(ctx, hash)
	if err != nil {
		return ErrUserNotFound
	}
	hp, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	user.PasswordHash = hp
	_, err = s.store.User.Update(ctx, user)
	return err
}

// GetUserStats 获取用户统计
// GetUserStats 委托给 StatsService（去重：原为 StatsService.GetUserStats 的完整拷贝，
// 现二者共享同一实现与 TTL 缓存）。
func (s *AdminService) GetUserStats(ctx context.Context) (*model.UserStats, error) {
	return s.stats.GetUserStats(ctx)
}
