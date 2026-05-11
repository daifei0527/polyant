package storage

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestMemoryRatingStore_ListRatedAfter(t *testing.T) {
	store := NewMemoryRatingStore()
	ctx := context.Background()

	// Create two ratings with different timestamps
	rating1 := &model.Rating{
		ID:          "r1",
		EntryId:     "e1",
		RaterPubkey: "pub1",
		Score:       4.0,
		RatedAt:     1000,
	}
	rating2 := &model.Rating{
		ID:          "r2",
		EntryId:     "e1",
		RaterPubkey: "pub2",
		Score:       5.0,
		RatedAt:     2000,
	}

	store.Create(ctx, rating1)
	store.Create(ctx, rating2)

	// Query ratings after 1500 - should only get rating2
	ratings, err := store.ListRatedAfter(ctx, 1500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ratings) != 1 {
		t.Fatalf("expected 1 rating, got %d", len(ratings))
	}
	if ratings[0].ID != "r2" {
		t.Errorf("expected rating r2, got %s", ratings[0].ID)
	}
}

func TestMemoryRatingStore_ListRatedAfter_Empty(t *testing.T) {
	store := NewMemoryRatingStore()
	ctx := context.Background()

	ratings, err := store.ListRatedAfter(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ratings) != 0 {
		t.Fatalf("expected 0 ratings, got %d", len(ratings))
	}
}

func TestMemoryRatingStore_ListRatedAfter_AllMatch(t *testing.T) {
	store := NewMemoryRatingStore()
	ctx := context.Background()

	rating := &model.Rating{
		ID:          "r1",
		EntryId:     "e1",
		RaterPubkey: "pub1",
		Score:       4.0,
		RatedAt:     2000,
	}
	store.Create(ctx, rating)

	ratings, err := store.ListRatedAfter(ctx, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ratings) != 1 {
		t.Fatalf("expected 1 rating, got %d", len(ratings))
	}
}
