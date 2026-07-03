package crypto

import (
	"crypto/ed25519"
	"testing"
)

func TestSignVerifyContent(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	sig, err := SignContent(priv, "Title", "Body", "cat/x")
	if err != nil {
		t.Fatalf("SignContent: %v", err)
	}
	if !VerifyContent(pub, sig, "Title", "Body", "cat/x") {
		t.Fatal("valid signature must verify")
	}
	// 篡改任意字段应失败
	if VerifyContent(pub, sig, "Title!", "Body", "cat/x") {
		t.Fatal("tampered title must fail")
	}
	if VerifyContent(pub, sig, "Title", "Body2", "cat/x") {
		t.Fatal("tampered content must fail")
	}
	if VerifyContent(pub, sig, "Title", "Body", "cat/y") {
		t.Fatal("tampered category must fail")
	}
	// 错误公钥应失败
	pub2, _, _ := ed25519.GenerateKey(nil)
	if VerifyContent(pub2, sig, "Title", "Body", "cat/x") {
		t.Fatal("wrong pubkey must fail")
	}
	// 非法公钥/签名长度应失败（不 panic）
	if VerifyContent(ed25519.PublicKey{}, sig, "Title", "Body", "cat/x") {
		t.Fatal("empty pubkey must fail")
	}
	if VerifyContent(pub, []byte("short"), "Title", "Body", "cat/x") {
		t.Fatal("malformed signature must fail")
	}
}

func TestSignVerifyRating(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	sig, err := SignRating(priv, "entry-1", "rater-pk", 4.5)
	if err != nil {
		t.Fatalf("SignRating: %v", err)
	}
	if !VerifyRating(pub, sig, "entry-1", "rater-pk", 4.5) {
		t.Fatal("valid rating signature must verify")
	}
	if VerifyRating(pub, sig, "entry-1", "rater-pk", 4.0) {
		t.Fatal("tampered score must fail")
	}
	if VerifyRating(pub, sig, "entry-2", "rater-pk", 4.5) {
		t.Fatal("tampered entryID must fail")
	}
}
