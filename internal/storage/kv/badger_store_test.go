package kv

import (
	"testing"
)

// TestBadgerStore_Scan_PropagatesError 验证 Scan 在底层错误时传播错误，
// 而非静默返回空 map + nil（B3 契约测试：关库后 Scan 必须返回非 nil error）
func TestBadgerStore_Scan_PropagatesError(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatalf("NewBadgerStore failed: %v", err)
	}
	_ = s.Put([]byte("entry:a"), []byte("{}"))
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 关库后 Scan 应返回非 nil error（之前可能返回空 map+nil）
	_, err = s.Scan([]byte("entry:"))
	if err == nil {
		t.Fatal("Scan on closed store must return error, not silent empty result")
	}
}
