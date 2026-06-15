package email

import (
	"testing"
	"time"
)

func TestNewVerificationManager(t *testing.T) {
	vm := NewVerificationManager()
	if vm == nil {
		t.Fatal("NewVerificationManager returned nil")
	}

	if vm.codeLength != 6 {
		t.Errorf("Default code length should be 6, got %d", vm.codeLength)
	}

	if vm.codeValidity != 30*time.Minute {
		t.Errorf("Default validity should be 30 minutes, got %v", vm.codeValidity)
	}
}

func TestVerificationManager_GenerateCode(t *testing.T) {
	vm := NewVerificationManager()

	code := vm.GenerateCode("test@example.com")

	if len(code) != vm.codeLength {
		t.Errorf("Code length should be %d, got %d", vm.codeLength, len(code))
	}

	// 生成的码应被存储且可验证、邮箱匹配
	if !vm.Verify(code, "test@example.com") {
		t.Error("Generated code should be stored and verifiable")
	}
}

func TestVerificationManager_Verify(t *testing.T) {
	vm := NewVerificationManager()

	email := "test@example.com"
	code := vm.GenerateCode(email)

	// 正确码 + 邮箱
	if !vm.Verify(code, email) {
		t.Error("Verify should return true for correct code and email")
	}

	// 验证后应标记为已使用：再次验证失败
	if vm.Verify(code, email) {
		t.Error("Code should be marked used after first verification")
	}
}

func TestVerificationManager_Verify_WrongCode(t *testing.T) {
	vm := NewVerificationManager()

	vm.GenerateCode("test@example.com")

	if vm.Verify("wrongcode", "test@example.com") {
		t.Error("Verify should return false for wrong code")
	}
}

func TestVerificationManager_Verify_WrongEmail(t *testing.T) {
	vm := NewVerificationManager()

	code := vm.GenerateCode("test@example.com")

	if vm.Verify(code, "other@example.com") {
		t.Error("Verify should return false for wrong email")
	}
}

func TestVerificationManager_Verify_AlreadyUsed(t *testing.T) {
	vm := NewVerificationManager()

	email := "test@example.com"
	code := vm.GenerateCode(email)

	// First verification
	vm.Verify(code, email)

	// Second verification should fail
	if vm.Verify(code, email) {
		t.Error("Verify should return false for already used code")
	}
}

func TestVerificationManager_Verify_Expired(t *testing.T) {
	vm := NewVerificationManager()
	vm.SetCodeValidity(1 * time.Millisecond) // Very short validity

	code := vm.GenerateCode("test@example.com")

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	if vm.Verify(code, "test@example.com") {
		t.Error("Verify should return false for expired code")
	}
}

func TestVerificationManager_Invalidate(t *testing.T) {
	vm := NewVerificationManager()

	code := vm.GenerateCode("test@example.com")

	vm.Invalidate(code)

	// 失效后验证应失败
	if vm.Verify(code, "test@example.com") {
		t.Error("Verify should fail after Invalidate")
	}
}

func TestVerificationManager_SetCodeLength(t *testing.T) {
	vm := NewVerificationManager()

	vm.SetCodeLength(8)

	if vm.codeLength != 8 {
		t.Errorf("Code length should be 8, got %d", vm.codeLength)
	}

	code := vm.GenerateCode("test@example.com")
	if len(code) != 8 {
		t.Errorf("Generated code length should be 8, got %d", len(code))
	}
}

func TestVerificationManager_SetCodeValidity(t *testing.T) {
	vm := NewVerificationManager()

	newValidity := 1 * time.Hour
	vm.SetCodeValidity(newValidity)

	if vm.codeValidity != newValidity {
		t.Errorf("Code validity should be %v, got %v", newValidity, vm.codeValidity)
	}
}

func TestVerificationManager_Cleanup(t *testing.T) {
	vm := NewVerificationManager()
	vm.SetCodeValidity(1 * time.Millisecond)

	// Generate codes
	code1 := vm.GenerateCode("test1@example.com")
	code2 := vm.GenerateCode("test2@example.com")

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Generate a new code that's not expired
	vm.SetCodeValidity(30 * time.Minute)
	code3 := vm.GenerateCode("test3@example.com")

	// Run cleanup
	vm.cleanup()

	// 过期码应被移除（验证失败），未过期码仍可验证
	if vm.Verify(code1, "test1@example.com") {
		t.Error("Expired code1 should be removed")
	}
	if vm.Verify(code2, "test2@example.com") {
		t.Error("Expired code2 should be removed")
	}
	if !vm.Verify(code3, "test3@example.com") {
		t.Error("Valid code3 should still verify")
	}
}

// TestVerificationManager_PersistsAcrossRestart 验证注入后端时验证码跨"重启"持久化：
// 两个共享同一 store 的 manager（模拟进程重启前后），后者仍能验证前者生成的码。
func TestVerificationManager_PersistsAcrossRestart(t *testing.T) {
	store := newMemCodeStore()

	vm1 := NewVerificationManagerWithStore(store)
	code := vm1.GenerateCode("persist@example.com")

	// 模拟重启：在同一 store 上新建 manager（旧进程已退出）
	vm2 := NewVerificationManagerWithStore(store)
	if !vm2.Verify(code, "persist@example.com") {
		t.Error("Code persisted via store should verify after simulated restart")
	}
}

// TestNewVerificationManagerWithStore_Nil 验证 nil store 退化为内存后端。
func TestNewVerificationManagerWithStore_Nil(t *testing.T) {
	vm := NewVerificationManagerWithStore(nil)
	if vm == nil {
		t.Fatal("nil store should fall back to memory manager")
	}
	code := vm.GenerateCode("nil@example.com")
	if !vm.Verify(code, "nil@example.com") {
		t.Error("Fallback memory manager should verify generated code")
	}
}
