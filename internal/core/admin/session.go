// internal/core/admin/session.go
package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Session 会话信息
type Session struct {
	PublicKey string    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionStore 会话持久化后端。kv.Store 隐式满足此接口（无需 core/admin 依赖 storage/kv）。
type SessionStore interface {
	Get(key []byte) ([]byte, error)
	Put(key, value []byte) error
	Delete(key []byte) error
}

// sessionKeyPrefix 会话在 KV 中的键前缀，与其他数据隔离。
const sessionKeyPrefix = "admin:session:"

// memSessionStore 内存会话存储（默认后端，重启丢失）。
type memSessionStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func (m *memSessionStore) Get(key []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[string(key)]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return append([]byte(nil), v...), nil
}
func (m *memSessionStore) Put(key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[string(key)] = append([]byte(nil), value...)
	return nil
}
func (m *memSessionStore) Delete(key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, string(key))
	return nil
}

// SessionManager 会话管理器
type SessionManager struct {
	store SessionStore
	ttl   time.Duration
}

// NewSessionManager 创建会话管理器（内存后端，重启丢失）
func NewSessionManager(ttl time.Duration) *SessionManager {
	return &SessionManager{store: &memSessionStore{data: make(map[string][]byte)}, ttl: ttl}
}

// NewSessionManagerWithStore 创建持久化会话管理器（KV 后端，重启不丢）。store 为 nil 时退化为内存后端。
func NewSessionManagerWithStore(ttl time.Duration, store SessionStore) *SessionManager {
	if store == nil {
		store = &memSessionStore{data: make(map[string][]byte)}
	}
	return &SessionManager{store: store, ttl: ttl}
}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(publicKey string) (string, error) {
	token := generateToken()
	now := time.Now()
	s := &Session{
		PublicKey: publicKey,
		CreatedAt: now,
		ExpiresAt: now.Add(sm.ttl),
	}
	data, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	if err := sm.store.Put([]byte(sessionKeyPrefix+token), data); err != nil {
		return "", fmt.Errorf("persist session: %w", err)
	}
	return token, nil
}

// ValidateSession 验证会话（过期则顺手删除）
func (sm *SessionManager) ValidateSession(token string) (string, bool) {
	data, err := sm.store.Get([]byte(sessionKeyPrefix + token))
	if err != nil || len(data) == 0 {
		return "", false
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return "", false
	}
	if time.Now().After(s.ExpiresAt) {
		_ = sm.store.Delete([]byte(sessionKeyPrefix + token))
		return "", false
	}
	return s.PublicKey, true
}

// DeleteSession 删除会话
func (sm *SessionManager) DeleteSession(token string) {
	_ = sm.store.Delete([]byte(sessionKeyPrefix + token))
}

// generateToken 生成随机 Token
func generateToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// CSPRNG 故障极罕见；退化为可预测 token 会让 admin 会话被伪造，
		// 直接 panic 由 supervisor 重启优于签发可预测会话。
		panic(fmt.Sprintf("crypto/rand for session token failed: %v", err))
	}
	return hex.EncodeToString(bytes)
}
