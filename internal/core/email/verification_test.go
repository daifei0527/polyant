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

	if vm.codes == nil {
		t.Error("codes map should be initialized")
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

	// Verify code is stored
	vm.mu.RLock()
	record, exists := vm.codes[code]
	vm.mu.RUnlock()

	if !exists {
		t.Error("Code should be stored")
	}

	if record.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", record.Email)
	}

	if record.Used {
		t.Error("New code should not be marked as used")
	}
}

func TestVerificationManager_Verify(t *testing.T) {
	vm := NewVerificationManager()

	email := "test@example.com"
	code := vm.GenerateCode(email)

	// Verify correct code and email
	if !vm.Verify(code, email) {
		t.Error("Verify should return true for correct code and email")
	}

	// Code should be marked as used after verification
	vm.mu.RLock()
	record := vm.codes[code]
	vm.mu.RUnlock()

	if !record.Used {
		t.Error("Code should be marked as used after verification")
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

	// Code should be deleted
	vm.mu.RLock()
	_, exists := vm.codes[code]
	vm.mu.RUnlock()

	if exists {
		t.Error("Code should be deleted after Invalidate")
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

	// Expired codes should be removed
	vm.mu.RLock()
	_, exists1 := vm.codes[code1]
	_, exists2 := vm.codes[code2]
	_, exists3 := vm.codes[code3]
	vm.mu.RUnlock()

	if exists1 {
		t.Error("Expired code1 should be removed")
	}
	if exists2 {
		t.Error("Expired code2 should be removed")
	}
	if !exists3 {
		t.Error("Valid code3 should still exist")
	}
}
