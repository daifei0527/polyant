package kv

import (
	"testing"
)

func TestPebbleStore_PutGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewPebbleStore(dir)
	if err != nil {
		t.Fatalf("NewPebbleStore failed: %v", err)
	}
	defer store.Close()

	key := []byte("test-key")
	value := []byte("test-value")

	if err := store.Put(key, value); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(got) != string(value) {
		t.Errorf("Get returned wrong value: got %s, want %s", got, value)
	}
}

func TestPebbleStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewPebbleStore(dir)
	if err != nil {
		t.Fatalf("NewPebbleStore failed: %v", err)
	}
	defer store.Close()

	_, err = store.Get([]byte("nonexistent"))
	if err != ErrKeyNotFound {
		t.Errorf("Get nonexistent key should return ErrKeyNotFound, got: %v", err)
	}
}

func TestPebbleStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewPebbleStore(dir)
	if err != nil {
		t.Fatalf("NewPebbleStore failed: %v", err)
	}
	defer store.Close()

	key := []byte("test-key")
	value := []byte("test-value")

	store.Put(key, value)

	if err := store.Delete(key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(key)
	if err != ErrKeyNotFound {
		t.Errorf("After delete, Get should return ErrKeyNotFound, got: %v", err)
	}
}

func TestPebbleStore_Scan(t *testing.T) {
	dir := t.TempDir()
	store, err := NewPebbleStore(dir)
	if err != nil {
		t.Fatalf("NewPebbleStore failed: %v", err)
	}
	defer store.Close()

	// 添加测试数据
	store.Put([]byte("entry:1"), []byte("value1"))
	store.Put([]byte("entry:2"), []byte("value2"))
	store.Put([]byte("user:1"), []byte("user1"))

	// 扫描 entry: 前缀
	result, err := store.Scan([]byte("entry:"))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Scan should return 2 entries, got %d", len(result))
	}
}

func TestPebbleStore_Persistence(t *testing.T) {
	dir := t.TempDir()

	// 第一次写入
	store1, err := NewPebbleStore(dir)
	if err != nil {
		t.Fatalf("NewPebbleStore failed: %v", err)
	}

	key := []byte("persist-key")
	value := []byte("persist-value")
	store1.Put(key, value)
	store1.Close()

	// 重新打开验证数据持久化
	store2, err := NewPebbleStore(dir)
	if err != nil {
		t.Fatalf("NewPebbleStore failed on reopen: %v", err)
	}
	defer store2.Close()

	got, err := store2.Get(key)
	if err != nil {
		t.Fatalf("Get after reopen failed: %v", err)
	}

	if string(got) != string(value) {
		t.Errorf("Persisted value wrong: got %s, want %s", got, value)
	}
}
