// Package email 提供邮件发送服务
package email

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// VerificationCode 验证码记录
type VerificationCode struct {
	Code      string
	Email     string
	CreatedAt time.Time
	ExpiresAt time.Time
	Used      bool
}

// VerificationManager 验证码管理器
type VerificationManager struct {
	mu    sync.RWMutex
	codes map[string]*VerificationCode // code -> record
	
	// 配置
	codeLength    int
	codeValidity  time.Duration
	cleanupPeriod time.Duration
}

// NewVerificationManager 创建验证码管理器
func NewVerificationManager() *VerificationManager {
	vm := &VerificationManager{
		codes:         make(map[string]*VerificationCode),
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
	
	vm.codes[code] = &VerificationCode{
		Code:      code,
		Email:     email,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(vm.codeValidity),
		Used:      false,
	}
	
	return code
}

// Verify 验证验证码
func (vm *VerificationManager) Verify(code, email string) bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	record, exists := vm.codes[code]
	if !exists {
		return false
	}
	
	// 检查是否已使用
	if record.Used {
		return false
	}
	
	// 检查是否过期
	if time.Now().After(record.ExpiresAt) {
		delete(vm.codes, code)
		return false
	}
	
	// 检查邮箱是否匹配
	if record.Email != email {
		return false
	}
	
	// 标记为已使用
	record.Used = true
	
	return true
}

// Invalidate 使验证码失效
func (vm *VerificationManager) Invalidate(code string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	delete(vm.codes, code)
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
	
	now := time.Now()
	for code, record := range vm.codes {
		if now.After(record.ExpiresAt) {
			delete(vm.codes, code)
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
