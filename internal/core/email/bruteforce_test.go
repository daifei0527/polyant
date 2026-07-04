package email

import (
	"testing"
	"time"
)

// TestVerificationBruteForceLockout: 连续失败达阈值→锁定；锁定内正确码也失败；过期后恢复。
func TestVerificationBruteForceLockout(t *testing.T) {
	vm := NewVerificationManager()
	vm.SetMaxAttempts(3)
	vm.SetLockDuration(50 * time.Millisecond)

	email := "victim@example.com"
	code := vm.GenerateCode(email) // 为该邮箱生成合法验证码

	// 3 次错误码 → 锁定
	for i := 0; i < 3; i++ {
		if vm.Verify("wrongcode", email) {
			t.Fatal("wrong code must not verify")
		}
	}
	if !vm.IsEmailLocked(email) {
		t.Fatal("email should be locked after 3 failed attempts")
	}

	// 锁定内：即便正确码也失败
	if vm.Verify(code, email) {
		t.Fatal("correct code must not verify while locked")
	}

	// 锁定过期后：正确码恢复有效
	time.Sleep(60 * time.Millisecond)
	if !vm.Verify(code, email) {
		t.Fatal("correct code should verify after lock expires")
	}
}

// TestVerificationSuccessResetsCounter: 验证成功后失败计数归零，不会因累计历史失败误锁。
func TestVerificationSuccessResetsCounter(t *testing.T) {
	vm := NewVerificationManager()
	vm.SetMaxAttempts(3)
	email := "user@example.com"
	code := vm.GenerateCode(email)

	vm.Verify("w1", email)
	vm.Verify("w2", email) // 2 次失败，未达阈值
	if vm.IsEmailLocked(email) {
		t.Fatal("should not be locked after only 2 failures")
	}
	if !vm.Verify(code, email) {
		t.Fatal("correct code must verify")
	}

	// 成功后计数归零：再 2 次失败不应锁定
	vm.Verify("w3", email)
	vm.Verify("w4", email)
	if vm.IsEmailLocked(email) {
		t.Fatal("counter should reset on success; not locked after 2 more fails")
	}
}

// TestVerificationLockoutPerEmail: A 邮箱被锁不影响 B 邮箱。
func TestVerificationLockoutPerEmail(t *testing.T) {
	vm := NewVerificationManager()
	vm.SetMaxAttempts(2)

	vm.GenerateCode("a@x.com")
	vm.Verify("bad", "a@x.com")
	vm.Verify("bad", "a@x.com")
	if !vm.IsEmailLocked("a@x.com") {
		t.Fatal("a@x.com should be locked")
	}
	if vm.IsEmailLocked("b@x.com") {
		t.Fatal("b@x.com must not be affected by a@x.com lockout")
	}
}
