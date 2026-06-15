// internal/core/admin/session_test.go
package admin

import (
	"testing"
	"time"
)

func TestSessionManager_CreateSession(t *testing.T) {
	sm := NewSessionManager(time.Hour)
	publicKey := "test-pub-key-123"

	token, err := sm.CreateSession(publicKey)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}
}

func TestSessionManager_ValidateSession(t *testing.T) {
	sm := NewSessionManager(time.Hour)
	publicKey := "test-pub-key-123"

	token, _ := sm.CreateSession(publicKey)

	pk, valid := sm.ValidateSession(token)
	if !valid {
		t.Fatal("session should be valid")
	}
	if pk != publicKey {
		t.Fatalf("expected %s, got %s", publicKey, pk)
	}
}

func TestSessionManager_InvalidToken(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	_, valid := sm.ValidateSession("invalid-token")
	if valid {
		t.Fatal("invalid token should not be valid")
	}
}

func TestSessionManager_ExpiredSession(t *testing.T) {
	sm := NewSessionManager(100 * time.Millisecond)
	publicKey := "test-pub-key-123"

	token, _ := sm.CreateSession(publicKey)

	time.Sleep(150 * time.Millisecond)

	_, valid := sm.ValidateSession(token)
	if valid {
		t.Fatal("expired session should not be valid")
	}
}

func TestSessionManager_DeleteSession(t *testing.T) {
	sm := NewSessionManager(time.Hour)
	publicKey := "test-pub-key-123"

	token, _ := sm.CreateSession(publicKey)
	sm.DeleteSession(token)

	_, valid := sm.ValidateSession(token)
	if valid {
		t.Fatal("deleted session should not be valid")
	}
}

// TestSessionManager_PersistentAcrossRestart: 持久化后端的 session 应在"重启"（新 manager，同 store）后仍有效。
func TestSessionManager_PersistentAcrossRestart(t *testing.T) {
	store := &memSessionStore{data: make(map[string][]byte)}
	sm1 := NewSessionManagerWithStore(time.Hour, store)
	token, err := sm1.CreateSession("pk-restart")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// 模拟重启：用同一 store 创建新 manager
	sm2 := NewSessionManagerWithStore(time.Hour, store)
	pubkey, ok := sm2.ValidateSession(token)
	if !ok {
		t.Fatal("session should survive restart via persistent store")
	}
	if pubkey != "pk-restart" {
		t.Errorf("got %s, want pk-restart", pubkey)
	}

	// 内存后端（默认）的 session 不应在重启后存活
	sm3 := NewSessionManager(time.Hour)
	tk, _ := sm3.CreateSession("pk-mem")
	sm4 := NewSessionManager(time.Hour)
	if _, ok := sm4.ValidateSession(tk); ok {
		t.Error("memory-backed session should NOT survive restart")
	}
}
