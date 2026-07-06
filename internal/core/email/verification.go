// Package email 提供邮件发送服务
package email

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	verificationCodeKeyPrefix = "vcode:"
	verificationFailKeyPrefix = "vcode:fail:" // R1-C3: per-email 失败计数/锁定
)

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

// emailFailRecord per-email 验证失败计数与锁定窗口（R1-C3 防暴破）。
type emailFailRecord struct {
	Count       int       `json:"count"`
	LockedUntil time.Time `json:"locked_until"`
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
	maxAttempts   int           // R1-C3: 触发锁定的失败次数阈值
	lockDuration  time.Duration // R1-C3: 锁定时长
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
		maxAttempts:   5,
		lockDuration:  15 * time.Minute,
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

// Verify 验证验证码。R1-C3：失败累计 per-email 计数，达阈值锁定；锁定内一律 false。
func (vm *VerificationManager) Verify(code, email string) bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// 锁定窗口内直接拒绝（即便提供正确码）
	if vm.isLocked(email) {
		return false
	}

	key := []byte(verificationCodeKeyPrefix + code)
	data, err := vm.store.Get(key)
	if err != nil {
		vm.recordFailure(email)
		return false
	}

	var rec VerificationCode
	if err := json.Unmarshal(data, &rec); err != nil {
		vm.recordFailure(email)
		return false
	}

	// 检查是否已使用
	if rec.Used {
		vm.recordFailure(email)
		return false
	}

	// 检查是否过期
	if time.Now().After(rec.ExpiresAt) {
		_ = vm.store.Delete(key)
		vm.recordFailure(email)
		return false
	}

	// 检查邮箱是否匹配
	if rec.Email != email {
		vm.recordFailure(email)
		return false
	}

	// 标记为已使用
	rec.Used = true
	if newData, err := json.Marshal(&rec); err == nil {
		_ = vm.store.Put(key, newData)
	}
	// 成功：清空失败计数
	vm.resetFailure(email)

	return true
}

// IsEmailLocked 报告某邮箱是否处于验证锁定窗口内（供 send-verification 拒绝发送）。
func (vm *VerificationManager) IsEmailLocked(email string) bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.isLocked(email)
}

// 以下失败计数辅助方法均假定调用方已持有 vm.mu。

// isLocked 报告 email 是否处于锁定窗口内。
func (vm *VerificationManager) isLocked(email string) bool {
	rec, err := vm.getFailRecord(email)
	if err != nil || rec == nil {
		return false
	}
	return rec.LockedUntil.After(time.Now())
}

// recordFailure 记录一次验证失败；达阈值则设置锁定窗口。
func (vm *VerificationManager) recordFailure(email string) {
	rec, _ := vm.getFailRecord(email)
	if rec == nil {
		rec = &emailFailRecord{}
	}
	// 若上一轮锁定已过期，重新开始计数
	if !rec.LockedUntil.IsZero() && time.Now().After(rec.LockedUntil) {
		rec.Count = 0
	}
	rec.Count++
	if rec.Count >= vm.maxAttempts {
		rec.LockedUntil = time.Now().Add(vm.lockDuration)
	}
	vm.putFailRecord(email, rec)
}

// resetFailure 清空某邮箱的失败计数（验证成功后调用）。
func (vm *VerificationManager) resetFailure(email string) {
	_ = vm.store.Delete([]byte(verificationFailKeyPrefix + email))
}

func (vm *VerificationManager) getFailRecord(email string) (*emailFailRecord, error) {
	data, err := vm.store.Get([]byte(verificationFailKeyPrefix + email))
	if err != nil {
		return nil, err
	}
	var rec emailFailRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (vm *VerificationManager) putFailRecord(email string, rec *emailFailRecord) {
	if data, err := json.Marshal(rec); err == nil {
		_ = vm.store.Put([]byte(verificationFailKeyPrefix+email), data)
	}
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
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("crypto/rand for verification code failed: %v", err))
	}
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

// SetMaxAttempts 设置触发锁定的失败次数阈值（R1-C3）。
func (vm *VerificationManager) SetMaxAttempts(n int) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	if n > 0 {
		vm.maxAttempts = n
	}
}

// SetLockDuration 设置锁定时长（R1-C3）。
func (vm *VerificationManager) SetLockDuration(d time.Duration) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	if d > 0 {
		vm.lockDuration = d
	}
}
