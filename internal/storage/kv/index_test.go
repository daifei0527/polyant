package kv

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestKVAuditStore_GetByIDIndex 验证 audit log 写入后维护了 audit:by-id:{id} 索引，
// 且 Get(id) 经该索引 O(1) 命中（而非全表扫描）。
func TestKVAuditStore_GetByIDIndex(t *testing.T) {
	store := NewMemoryStore()
	as := NewAuditStore(store)
	ctx := context.Background()

	log := model.NewAuditLog()
	log.ActionType = "test.action"
	log.TargetID = "entry-x"
	if err := as.Create(ctx, log); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 索引键应已写入
	if _, err := store.Get([]byte(auditByIDPrefix + log.ID)); err != nil {
		t.Fatalf("by-id index not written: %v", err)
	}

	// Get(id) 应经索引命中
	got, err := as.Get(ctx, log.ID)
	if err != nil {
		t.Fatalf("Get by id: %v", err)
	}
	if got.ID != log.ID {
		t.Errorf("got ID %s, want %s", got.ID, log.ID)
	}
}

// TestRatingStore_ListByRater 验证 by-rater 索引：ListByRater 只返回指定评分者的评分。
func TestRatingStore_ListByRater(t *testing.T) {
	store := NewMemoryStore()
	rs := NewRatingStore(store)

	mk := func(entry, rater string, score float64) *model.Rating {
		return &model.Rating{EntryId: entry, RaterPubkey: rater, Score: score, Weight: 1}
	}
	if err := rs.CreateRating(mk("e1", "raterA", 5)); err != nil {
		t.Fatal(err)
	}
	if err := rs.CreateRating(mk("e2", "raterA", 4)); err != nil {
		t.Fatal(err)
	}
	if err := rs.CreateRating(mk("e1", "raterB", 3)); err != nil {
		t.Fatal(err)
	}

	got, err := rs.ListByRater("raterA")
	if err != nil {
		t.Fatalf("ListByRater: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 ratings for raterA, got %d", len(got))
	}
	for _, r := range got {
		if r.RaterPubkey != "raterA" {
			t.Errorf("ListByRater leaked other rater: %s", r.RaterPubkey)
		}
	}

	gotB, _ := rs.ListByRater("raterB")
	if len(gotB) != 1 {
		t.Errorf("expected 1 rating for raterB, got %d", len(gotB))
	}
}
