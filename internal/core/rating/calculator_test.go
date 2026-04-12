package rating

import (
	"context"
	"testing"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

func newTestStore(t *testing.T) *storage.Store {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return store
}

func TestGetLevelWeight(t *testing.T) {
	tests := []struct {
		level    int32
		expected float64
	}{
		{model.UserLevelLv0, 0.0},
		{model.UserLevelLv1, 1.0},
		{model.UserLevelLv2, 1.2},
		{model.UserLevelLv3, 1.5},
		{model.UserLevelLv4, 2.0},
		{model.UserLevelLv5, 3.0},
		{99, 0.0}, // Unknown level
	}

	for _, tt := range tests {
		result := GetLevelWeight(tt.level)
		if result != tt.expected {
			t.Errorf("GetLevelWeight(%d) = %f, expected %f", tt.level, result, tt.expected)
		}
	}
}

func TestRatingCalculator_SubmitRating(t *testing.T) {
	store := newTestStore(t)
	calc := NewRatingCalculator(store)

	// Create test entry
	entry := &model.KnowledgeEntry{
		ID:        "test-entry-1",
		Title:     "Test Entry",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
	}
	_, _ = store.Entry.Create(context.Background(), entry)

	// Create test user
	user := &model.User{
		PublicKey:     "dGVzdC1wdWJsaWMta2V5",
		AgentName:     "test-user",
		UserLevel:     model.UserLevelLv1,
		Status:        model.UserStatusActive,
	}

	// Test successful rating
	rating, err := calc.SubmitRating(context.Background(), "test-entry-1", user, 4.5, "Great article")
	if err != nil {
		t.Fatalf("SubmitRating failed: %v", err)
	}

	if rating.Score != 4.5 {
		t.Errorf("Expected score 4.5, got %f", rating.Score)
	}
	if rating.Weight != 1.0 {
		t.Errorf("Expected weight 1.0 for Lv1, got %f", rating.Weight)
	}
}

func TestRatingCalculator_SubmitRating_ScoreOutOfRange(t *testing.T) {
	store := newTestStore(t)
	calc := NewRatingCalculator(store)

	user := &model.User{
		PublicKey:     "test-key",
		UserLevel:     model.UserLevelLv1,
		Status:        model.UserStatusActive,
	}

	// Test score too low
	_, err := calc.SubmitRating(context.Background(), "entry-1", user, 0.5, "")
	if err != ErrScoreOutOfRange {
		t.Error("Expected ErrScoreOutOfRange for low score")
	}

	// Test score too high
	_, err = calc.SubmitRating(context.Background(), "entry-1", user, 5.5, "")
	if err != ErrScoreOutOfRange {
		t.Error("Expected ErrScoreOutOfRange for high score")
	}
}

func TestRatingCalculator_SubmitRating_PermissionDenied(t *testing.T) {
	store := newTestStore(t)
	calc := NewRatingCalculator(store)

	// Lv0 user cannot rate
	user := &model.User{
		PublicKey:     "test-key",
		UserLevel:     model.UserLevelLv0,
		Status:        model.UserStatusActive,
	}

	_, err := calc.SubmitRating(context.Background(), "entry-1", user, 4.0, "")
	if err != ErrPermissionDenied {
		t.Error("Expected ErrPermissionDenied for Lv0 user")
	}
}

func TestRatingCalculator_SubmitRating_DuplicateRating(t *testing.T) {
	store := newTestStore(t)
	calc := NewRatingCalculator(store)

	// Create test entry
	entry := &model.KnowledgeEntry{
		ID:        "test-entry-1",
		Title:     "Test",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
	}
	_, _ = store.Entry.Create(context.Background(), entry)

	user := &model.User{
		PublicKey:     "dGVzdC1wdWJsaWMta2V5",
		UserLevel:     model.UserLevelLv1,
		Status:        model.UserStatusActive,
	}

	// First rating
	_, err := calc.SubmitRating(context.Background(), "test-entry-1", user, 4.0, "")
	if err != nil {
		t.Fatalf("First rating failed: %v", err)
	}

	// Duplicate rating
	_, err = calc.SubmitRating(context.Background(), "test-entry-1", user, 5.0, "")
	if err != ErrDuplicateRating {
		t.Error("Expected ErrDuplicateRating for duplicate rating")
	}
}

func TestRatingCalculator_RecalculateEntryScore(t *testing.T) {
	store := newTestStore(t)
	calc := NewRatingCalculator(store)

	// Create test entry
	entry := &model.KnowledgeEntry{
		ID:        "test-entry-1",
		Title:     "Test",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
	}
	_, _ = store.Entry.Create(context.Background(), entry)

	// Add multiple ratings
	users := []*model.User{
		{PublicKey: "key1", UserLevel: model.UserLevelLv1}, // weight 1.0
		{PublicKey: "key2", UserLevel: model.UserLevelLv2}, // weight 1.2
		{PublicKey: "key3", UserLevel: model.UserLevelLv3}, // weight 1.5
	}

	scores := []float64{5.0, 4.0, 3.0}
	for i, user := range users {
		// Create rating directly in store
		rating := &model.Rating{
			ID:            string(rune('a' + i)),
			EntryId:       "test-entry-1",
			RaterPubkey:   user.PublicKey,
			Score:         scores[i],
			Weight:        GetLevelWeight(user.UserLevel),
			WeightedScore: scores[i] * GetLevelWeight(user.UserLevel),
		}
		_, _ = store.Rating.Create(context.Background(), rating)
	}

	// Calculate expected score
	// (5.0 * 1.0 + 4.0 * 1.2 + 3.0 * 1.5) / (1.0 + 1.2 + 1.5)
	// = (5.0 + 4.8 + 4.5) / 3.7
	// = 14.3 / 3.7 ≈ 3.86

	result := calc.RecalculateEntryScore(context.Background(), "test-entry-1")

	if result <= 0 {
		t.Error("Expected positive score")
	}

	// Verify reasonable range
	if result < 3.5 || result > 4.0 {
		t.Errorf("Expected score between 3.5 and 4.0, got %f", result)
	}
}

func TestRatingCalculator_RecalculateEntryScore_NoRatings(t *testing.T) {
	store := newTestStore(t)
	calc := NewRatingCalculator(store)

	// Entry with no ratings
	result := calc.RecalculateEntryScore(context.Background(), "non-existing-entry")
	if result != 0.0 {
		t.Errorf("Expected 0.0 for no ratings, got %f", result)
	}
}

func TestRatingCalculator_GetEntryRatings(t *testing.T) {
	store := newTestStore(t)
	calc := NewRatingCalculator(store)

	// Create test entry
	entry := &model.KnowledgeEntry{
		ID:        "test-entry-1",
		Title:     "Test",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
	}
	_, _ = store.Entry.Create(context.Background(), entry)

	// Add ratings
	for i := 0; i < 3; i++ {
		rating := &model.Rating{
			ID:          string(rune('a' + i)),
			EntryId:     "test-entry-1",
			RaterPubkey: string(rune('x' + i)),
			Score:       float64(i + 3),
			Weight:      1.0,
		}
		_, _ = store.Rating.Create(context.Background(), rating)
	}

	ratings, err := calc.GetEntryRatings(context.Background(), "test-entry-1")
	if err != nil {
		t.Fatalf("GetEntryRatings failed: %v", err)
	}

	if len(ratings) != 3 {
		t.Errorf("Expected 3 ratings, got %d", len(ratings))
	}
}

func TestRatingCalculator_WeightedScore(t *testing.T) {
	// Test that higher level users have more impact on score
	store := newTestStore(t)
	calc := NewRatingCalculator(store)

	// Create test entry
	entry := &model.KnowledgeEntry{
		ID:        "test-entry-1",
		Title:     "Test",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
	}
	_, _ = store.Entry.Create(context.Background(), entry)

	// Lv1 user rates 5.0 (weight 1.0)
	user1 := &model.User{
		PublicKey:     "key1",
		UserLevel:     model.UserLevelLv1,
		Status:        model.UserStatusActive,
	}

	// Lv5 user rates 1.0 (weight 3.0)
	user5 := &model.User{
		PublicKey:     "key5",
		UserLevel:     model.UserLevelLv5,
		Status:        model.UserStatusActive,
	}

	_, _ = calc.SubmitRating(context.Background(), "test-entry-1", user1, 5.0, "")
	_, _ = calc.SubmitRating(context.Background(), "test-entry-1", user5, 1.0, "")

	// Expected: (5.0 * 1.0 + 1.0 * 3.0) / (1.0 + 3.0) = 8.0 / 4.0 = 2.0
	result := calc.RecalculateEntryScore(context.Background(), "test-entry-1")

	// Score should be closer to Lv5's rating due to higher weight
	if result >= 3.0 {
		t.Errorf("Expected score closer to Lv5 rating (1.0), got %f", result)
	}

	// Should be exactly 2.0
	if result < 1.9 || result > 2.1 {
		t.Errorf("Expected score ~2.0, got %f", result)
	}
}
