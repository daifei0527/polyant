package crypto

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	h, err := HashPassword("s3cret-pw")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if h == "" || h == "s3cret-pw" {
		t.Fatal("hash empty or equals plain")
	}
	if !CheckPassword(h, "s3cret-pw") {
		t.Fatal("valid password rejected")
	}
	if CheckPassword(h, "wrong") {
		t.Fatal("wrong password accepted")
	}
}

func TestCheckPassword_emptyHash(t *testing.T) {
	// 未设密码的用户：任何明文都应返回 false
	if CheckPassword("", "anything") {
		t.Fatal("empty hash must reject")
	}
}

func TestHashPassword_uniqueSalts(t *testing.T) {
	// bcrypt 每次哈希带随机盐，相同明文应产生不同哈希
	a, _ := HashPassword("same-pw")
	b, _ := HashPassword("same-pw")
	if a == b {
		t.Fatal("two hashes of same password must differ (random salt)")
	}
}
