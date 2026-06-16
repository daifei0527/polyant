package storage

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestBadgerUserStore_GetByHashIndex 验证 BadgerUserStore.Create 维护 hash→pubkey 索引，
// 且按公钥哈希查找（API 主路径）经索引 O(1) 命中，而非回退的 10 万全表扫描。
func TestBadgerUserStore_GetByHashIndex(t *testing.T) {
	dir := t.TempDir()
	w, err := NewBadgerStoreWithCloser(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer w.Close()
	ctx := context.Background()

	pub := base64.StdEncoding.EncodeToString(make([]byte, 32)) // 合法的 base64 公钥
	u := &model.User{
		PublicKey:    pub,
		AgentName:    "hash-index-test",
		RegisteredAt: 1,
	}
	if _, err := w.User.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	// 索引键应已写入
	h := hashPublicKey(pub)
	if _, err := w.kvStore.Get([]byte(kv.PrefixUserHash + h)); err != nil {
		t.Fatalf("hash index not written: %v", err)
	}

	// 按哈希查找应经索引命中
	got, err := w.User.Get(ctx, h)
	if err != nil {
		t.Fatalf("Get by hash: %v", err)
	}
	if got.PublicKey != pub {
		t.Errorf("got PublicKey %s, want %s", got.PublicKey, pub)
	}

	// 按原始公钥查找也应 O(1) 命中
	got2, err := w.User.Get(ctx, pub)
	if err != nil {
		t.Fatalf("Get by raw pubkey: %v", err)
	}
	if got2.PublicKey != pub {
		t.Errorf("got PublicKey %s, want %s", got2.PublicKey, pub)
	}
}
