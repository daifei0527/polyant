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
