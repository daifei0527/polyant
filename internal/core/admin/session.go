// internal/core/admin/session.go
package admin

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session 会话信息
type Session struct {
	PublicKey string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager 会话管理器
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	ttl      time.Duration
}

// NewSessionManager 创建会话管理器
func NewSessionManager(ttl time.Duration) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(publicKey string) (string, error) {
	token := generateToken()
	now := time.Now()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessions[token] = &Session{
		PublicKey: publicKey,
		CreatedAt: now,
		ExpiresAt: now.Add(sm.ttl),
	}

	return token, nil
}

// ValidateSession 验证会话
func (sm *SessionManager) ValidateSession(token string) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[token]
	if !exists {
		return "", false
	}

	if time.Now().After(session.ExpiresAt) {
		return "", false
	}

	return session.PublicKey, true
}

// DeleteSession 删除会话
func (sm *SessionManager) DeleteSession(token string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, token)
}

// generateToken 生成随机 Token
func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
