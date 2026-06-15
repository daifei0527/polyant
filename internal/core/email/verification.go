// Package email 提供邮件发送服务
package email

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

const verificationCodeKeyPrefix = "vcode:"

// errCodeNotFound 表示验证码不存在（内存后端语义；KV 后端返回其自身的 not-found 错误）。
var errCodeNotFound = errors.New("verification code not found")

// VerificationCode 验证码记录
type VerificationCode struct {
	Code      string    `json:"code"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}

// codeStore 是验证码持久化后端。kv.Store 隐式满足此接口（email 包无需依赖
// storage/kv）；memCodeStore 作为默认内存后端（重启丢失）。注入 KV 后端后，
// 验证码在节点重启后仍可用。
type codeStore interface {
	Put(key, value []byte) error
	Get(key []byte) ([]byte, error)
	Delete(key []byte) error
	Scan(prefix []byte) (map[string][]byte, error)
}

// memCodeStore 内存验证码存储（默认后端）。
type memCodeStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemCodeStore() *memCodeStore {
	return &memCodeStore{data: make(map[string][]byte)}
}

func (m *memCodeStore) Put(key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := make([]byte, len(value))
	copy(v, value)
	m.data[string(key)] = v
	return nil
}

func (m *memCodeStore) Get(key []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[string(key)]
	if !ok {
		return nil, errCodeNotFound
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (m *memCodeStore) Delete(key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, string(key))
	return nil
}

func (m *memCodeStore) Scan(prefix []byte) (map[string][]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := string(prefix)
	out := make(map[string][]byte)
	for k, v := range m.data {
		if strings.HasPrefix(k, p) {
			cv := make([]byte, len(v))
			copy(cv, v)
			out[k] = cv
		}
	}
	return out, nil
}

// VerificationManager 验证码管理器
type VerificationManager struct {
	mu    sync.Mutex
	store codeStore

	// 配置
	codeLength    int
	codeValidity  time.Duration
	cleanupPeriod time.Duration
}

// NewVerificationManager 创建内存验证码管理器（验证码仅存内存，重启丢失）。
func NewVerificationManager() *VerificationManager {
	return newVerificationManager(newMemCodeStore())
}

// NewVerificationManagerWithStore 创建持久化验证码管理器。store 为 KV 后端时
// 验证码落盘，节点重启后仍在有效期内可验证；store 为 nil 时退化为内存后端。
func NewVerificationManagerWithStore(store codeStore) *VerificationManager {
	if store == nil {
		store = newMemCodeStore()
	}
	return newVerificationManager(store)
}

func newVerificationManager(store codeStore) *VerificationManager {
	vm := &VerificationManager{
		store:         store,
		codeLength:    6,
		codeValidity:  30 * time.Minute,
		cleanupPeriod: 5 * time.Minute,
	}

	// 启动定期清理
	go vm.cleanupLoop()

	return vm
}

// GenerateCode 生成验证码
func (vm *VerificationManager) GenerateCode(email string) string {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	code := vm.generateRandomCode()
	rec := &VerificationCode{
		Code:      code,
		Email:     email,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(vm.codeValidity),
		Used:      false,
	}

	if data, err := json.Marshal(rec); err == nil {
		_ = vm.store.Put([]byte(verificationCodeKeyPrefix+code), data)
	}

	return code
}

// Verify 验证验证码
func (vm *VerificationManager) Verify(code, email string) bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	key := []byte(verificationCodeKeyPrefix + code)
	data, err := vm.store.Get(key)
	if err != nil {
		return false
	}

	var rec VerificationCode
	if err := json.Unmarshal(data, &rec); err != nil {
		return false
	}

	// 检查是否已使用
	if rec.Used {
		return false
	}

	// 检查是否过期
	if time.Now().After(rec.ExpiresAt) {
		_ = vm.store.Delete(key)
		return false
	}

	// 检查邮箱是否匹配
	if rec.Email != email {
		return false
	}

	// 标记为已使用
	rec.Used = true
	if newData, err := json.Marshal(&rec); err == nil {
		_ = vm.store.Put(key, newData)
	}

	return true
}

// Invalidate 使验证码失效
func (vm *VerificationManager) Invalidate(code string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	_ = vm.store.Delete([]byte(verificationCodeKeyPrefix + code))
}

// generateRandomCode 生成随机验证码
func (vm *VerificationManager) generateRandomCode() string {
	bytes := make([]byte, vm.codeLength/2+1)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:vm.codeLength]
}

// cleanupLoop 定期清理过期验证码
func (vm *VerificationManager) cleanupLoop() {
	ticker := time.NewTicker(vm.cleanupPeriod)
	defer ticker.Stop()

	for range ticker.C {
		vm.cleanup()
	}
}

// cleanup 清理过期验证码
func (vm *VerificationManager) cleanup() {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	items, err := vm.store.Scan([]byte(verificationCodeKeyPrefix))
	if err != nil {
		return
	}

	now := time.Now()
	for key, data := range items {
		var rec VerificationCode
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		if now.After(rec.ExpiresAt) {
			_ = vm.store.Delete([]byte(key))
		}
	}
}

// SetCodeLength 设置验证码长度
func (vm *VerificationManager) SetCodeLength(length int) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.codeLength = length
}

// SetCodeValidity 设置验证码有效期
func (vm *VerificationManager) SetCodeValidity(d time.Duration) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.codeValidity = d
}
