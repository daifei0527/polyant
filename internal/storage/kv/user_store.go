package kv

import (
	"context"
	"fmt"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// UserStore 提供用户的CRUD操作
type UserStore struct {
	store Store
}

// NewUserStore 创建一个新的用户存储实例
func NewUserStore(store Store) *UserStore {
	return &UserStore{store: store}
}

// CreateUser 创建一个新用户
func (us *UserStore) CreateUser(user *model.User) error {
	if user.PublicKey == "" {
		return fmt.Errorf("user public key must not be empty")
	}

	key := []byte(PrefixUser + user.PublicKey)

	// 检查是否已存在
	_, err := us.store.Get(key)
	if err == nil {
		return fmt.Errorf("user with public key %s already exists", user.PublicKey)
	}

	data, err := user.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize user: %w", err)
	}

	if err := us.store.Put(key, data); err != nil {
		return err
	}
	// 维护 email→pubkey 索引，使 GetByEmail 成为 O(1) 直查而非全表扫描
	if user.Email != "" {
		if err := us.store.Put([]byte(PrefixUserEmail+user.Email), []byte(user.PublicKey)); err != nil {
			return fmt.Errorf("failed to index user email: %w", err)
		}
	}
	return nil
}

// GetUser 根据公钥获取用户
func (us *UserStore) GetUser(publicKey string) (*model.User, error) {
	key := []byte(PrefixUser + publicKey)

	data, err := us.store.Get(key)
	if err != nil {
		if err == ErrKeyNotFound {
			return nil, fmt.Errorf("user %s not found", publicKey)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user := &model.User{}
	if err := user.FromJSON(data); err != nil {
		return nil, fmt.Errorf("failed to deserialize user: %w", err)
	}

	return user, nil
}

// UpdateUser 更新用户信息
func (us *UserStore) UpdateUser(user *model.User) error {
	key := []byte(PrefixUser + user.PublicKey)

	// 读旧用户（用于维护 email 索引：email 变更时删旧索引；不存在则 GetUser 已返回 not-found）
	old, err := us.GetUser(user.PublicKey)
	if err != nil {
		return err
	}

	data, err := user.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize user: %w", err)
	}

	if err := us.store.Put(key, data); err != nil {
		return err
	}

	// 维护 email 索引：仅在 email 变更时更新
	if old.Email != user.Email {
		if old.Email != "" {
			_ = us.store.Delete([]byte(PrefixUserEmail + old.Email))
		}
		if user.Email != "" {
			if err := us.store.Put([]byte(PrefixUserEmail+user.Email), []byte(user.PublicKey)); err != nil {
				return fmt.Errorf("failed to index user email: %w", err)
			}
		}
	}
	return nil
}

// ListUsers 列出用户，支持分页
func (us *UserStore) ListUsers(offset, limit int) ([]*model.User, error) {
	users, err := ScanAndParse(us.store, PrefixUser, func(data []byte) (*model.User, error) {
		user := &model.User{}
		if err := user.FromJSON(data); err != nil {
			return nil, err
		}
		return user, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// 按注册时间倒序排列
	sortUsersByRegistered(users)

	// 应用分页
	return paginateUsers(users, offset, limit), nil
}

// sortUsersByRegistered 按注册时间倒序排列用户
func sortUsersByRegistered(users []*model.User) {
	for i := 0; i < len(users)-1; i++ {
		for j := i + 1; j < len(users); j++ {
			if users[j].RegisteredAt > users[i].RegisteredAt {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
}

// paginateUsers 对用户列表进行分页
func paginateUsers(users []*model.User, offset, limit int) []*model.User {
	if offset >= len(users) {
		return []*model.User{}
	}

	end := offset + limit
	if end > len(users) {
		end = len(users)
	}

	return users[offset:end]
}

// GetByEmail 根据邮箱获取用户（O(1) 索引直查，非全表扫描）
func (us *UserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	pubkey, err := us.store.Get([]byte(PrefixUserEmail + email))
	if err != nil {
		if err == ErrKeyNotFound {
			return nil, fmt.Errorf("user with email %s not found", email)
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return us.GetUser(string(pubkey))
}
