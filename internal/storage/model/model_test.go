package model

import (
	"testing"
	"time"
)

func TestNowMillis(t *testing.T) {
	before := time.Now().UnixMilli()
	result := NowMillis()
	after := time.Now().UnixMilli()

	if result < before || result > after {
		t.Errorf("NowMillis result %d should be between %d and %d", result, before, after)
	}
}

func TestGetLevelWeight(t *testing.T) {
	tests := []struct {
		level    int32
		expected float64
	}{
		{UserLevelLv0, 0.0},
		{UserLevelLv1, 1.0},
		{UserLevelLv2, 1.2},
		{UserLevelLv3, 1.5},
		{UserLevelLv4, 2.0},
		{UserLevelLv5, 3.0},
		{99, 0.0}, // Unknown level
		{-1, 0.0}, // Negative level
	}

	for _, tt := range tests {
		result := GetLevelWeight(tt.level)
		if result != tt.expected {
			t.Errorf("GetLevelWeight(%d) = %f, expected %f", tt.level, result, tt.expected)
		}
	}
}

func TestUserStatusConstants(t *testing.T) {
	if UserStatusActive != "active" {
		t.Errorf("UserStatusActive should be 'active', got %s", UserStatusActive)
	}

	if UserStatusSuspended != "suspended" {
		t.Errorf("UserStatusSuspended should be 'suspended', got %s", UserStatusSuspended)
	}
}

func TestKnowledgeEntry_SignatureRoundtrip(t *testing.T) {
	e := NewKnowledgeEntry("t", "c", "cat", "creator")
	if e.Signature != nil || e.SignAlgorithm != "" {
		t.Fatal("new entry must have empty signature by default")
	}
	e.Signature = []byte("sig-bytes")
	e.SignAlgorithm = "ed25519"
	data, err := e.ToJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got KnowledgeEntry
	if err := got.FromJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(got.Signature) != "sig-bytes" || got.SignAlgorithm != "ed25519" {
		t.Fatalf("signature not preserved: sig=%v algo=%q", got.Signature, got.SignAlgorithm)
	}
}

func TestRating_SignatureRoundtrip(t *testing.T) {
	r := &Rating{ID: "r1", EntryId: "e1", RaterPubkey: "pk", Score: 4}
	if r.Signature != nil || r.SignAlgorithm != "" {
		t.Fatal("new rating must have empty signature by default")
	}
	r.Signature = []byte("r-sig")
	r.SignAlgorithm = "ed25519"
	data, err := r.ToJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Rating
	if err := got.FromJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(got.Signature) != "r-sig" || got.SignAlgorithm != "ed25519" {
		t.Fatalf("signature not preserved: sig=%v algo=%q", got.Signature, got.SignAlgorithm)
	}
}
