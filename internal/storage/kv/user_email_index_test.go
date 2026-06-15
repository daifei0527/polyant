package kv_test

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestUserStore_GetByEmail_Indexed: GetByEmail 经 email→pubkey 索引 O(1) 直查。
func TestUserStore_GetByEmail_Indexed(t *testing.T) {
	store := NewMemoryStore()
	us := kv.NewUserStore(store)
	ctx := context.Background()

	if err := us.CreateUser(&model.User{PublicKey: "pk-1", Email: "a@example.com", AgentName: "A"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := us.GetByEmail(ctx, "a@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.PublicKey != "pk-1" {
		t.Errorf("GetByEmail returned %s, want pk-1", got.PublicKey)
	}

	// 未索引的 email 应返回 not-found 错误
	if _, err := us.GetByEmail(ctx, "unknown@example.com"); err == nil {
		t.Error("GetByEmail for unknown email should error")
	}
}

// TestUserStore_GetByEmail_UpdateEmail: 更换 email 后，旧 email 不再解析，新 email 命中。
func TestUserStore_GetByEmail_UpdateEmail(t *testing.T) {
	store := NewMemoryStore()
	us := kv.NewUserStore(store)

	if err := us.CreateUser(&model.User{PublicKey: "pk-1", Email: "old@example.com"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := us.UpdateUser(&model.User{PublicKey: "pk-1", Email: "new@example.com"}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	if _, err := us.GetByEmail(context.Background(), "old@example.com"); err == nil {
		t.Error("old email should no longer resolve after update")
	}
	got, err := us.GetByEmail(context.Background(), "new@example.com")
	if err != nil {
		t.Fatalf("GetByEmail new: %v", err)
	}
	if got.PublicKey != "pk-1" {
		t.Errorf("new email returned %s, want pk-1", got.PublicKey)
	}
}

// TestUserStore_ListUsers_NotPollutedByEmailIndex: email 索引使用 user-email: 前缀，
// 不应被 ListUsers 的 user: 前缀扫描误当用户。
func TestUserStore_ListUsers_NotPollutedByEmailIndex(t *testing.T) {
	store := NewMemoryStore()
	us := kv.NewUserStore(store)

	if err := us.CreateUser(&model.User{PublicKey: "pk-1", Email: "a@example.com"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	users, err := us.ListUsers(0, 100)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("ListUsers returned %d users, want 1 (email index must not pollute user scan)", len(users))
	}
}
