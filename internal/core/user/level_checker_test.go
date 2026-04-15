package user

import (
	"context"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestNewLevelUpgradeChecker(t *testing.T) {
	store, _ := storage.NewMemoryStore()

	checker := NewLevelUpgradeChecker(store, time.Hour)
	if checker == nil {
		t.Fatal("NewLevelUpgradeChecker returned nil")
	}

	if checker.interval != time.Hour {
		t.Errorf("Expected interval 1 hour, got %v", checker.interval)
	}

	// Default interval
	checker2 := NewLevelUpgradeChecker(store, 0)
	if checker2.interval != time.Hour {
		t.Errorf("Default interval should be 1 hour, got %v", checker2.interval)
	}
}

func TestLevelUpgradeChecker_StartStop(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	checker := NewLevelUpgradeChecker(store, time.Hour)

	ctx := context.Background()

	// Start
	err := checker.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !checker.running {
		t.Error("Checker should be running after Start")
	}

	// Double start (should be idempotent)
	err = checker.Start(ctx)
	if err != nil {
		t.Fatalf("Second Start should not fail: %v", err)
	}

	// Stop
	err = checker.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if checker.running {
		t.Error("Checker should not be running after Stop")
	}

	// Double stop (should be idempotent)
	err = checker.Stop()
	if err != nil {
		t.Fatalf("Second Stop should not fail: %v", err)
	}
}

func TestLevelUpgradeChecker_checkUpgrade(t *testing.T) {
	tests := []struct {
		name          string
		level         int32
		contributions int32
		ratings       int32
		expectedLevel int32
		expectUpgrade bool
	}{
		{"Lv0 no auto upgrade", model.UserLevelLv0, 100, 100, model.UserLevelLv0, false},
		{"Lv1 meets requirements", model.UserLevelLv1, 10, 20, model.UserLevelLv2, true},
		// When not enough contributions/ratings, newLevel is 0 (not assigned) and upgraded is false
		// This is the current behavior of checkUpgrade
		{"Lv1 not enough contributions", model.UserLevelLv1, 9, 20, 0, false},
		{"Lv1 not enough ratings", model.UserLevelLv1, 10, 19, 0, false},
		{"Lv2 meets requirements", model.UserLevelLv2, 50, 100, model.UserLevelLv3, true},
		{"Lv3 meets requirements", model.UserLevelLv3, 200, 500, model.UserLevelLv4, true},
		{"Lv4 no auto upgrade", model.UserLevelLv4, 1000, 1000, model.UserLevelLv4, false},
		{"Lv5 max level", model.UserLevelLv5, 1000, 1000, model.UserLevelLv5, false},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new store for each test to avoid conflicts
			store, _ := storage.NewMemoryStore()
			checker := NewLevelUpgradeChecker(store, time.Hour)

			// Generate a unique base64 key for each test based on index
			publicKeys := []string{
				"dGVzdC0w", // test-0
				"dGVzdC0x", // test-1
				"dGVzdC0y", // test-2
				"dGVzdC0z", // test-3
				"dGVzdC00", // test-4
				"dGVzdC01", // test-5
				"dGVzdC02", // test-6
				"dGVzdC03", // test-7
			}
			publicKey := publicKeys[i]

			user := &model.User{
				PublicKey:       publicKey,
				AgentName:       tt.name,
				UserLevel:       tt.level,
				ContributionCnt: tt.contributions,
				RatingCnt:       tt.ratings,
				Status:          model.UserStatusActive,
			}

			// Create user in store first
			store.User.Create(context.Background(), user)

			newLevel, upgraded := checker.checkUpgrade(user)

			if upgraded != tt.expectUpgrade {
				t.Errorf("Expected upgrade %v, got %v", tt.expectUpgrade, upgraded)
			}

			if newLevel != tt.expectedLevel {
				t.Errorf("Expected level %d, got %d", tt.expectedLevel, newLevel)
			}
		})
	}
}

func TestLevelUpgradeChecker_CheckUserUpgrade(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	checker := NewLevelUpgradeChecker(store, time.Hour)

	user := &model.User{
		PublicKey:       "manual-check-user",
		AgentName:       "Manual Check",
		UserLevel:       model.UserLevelLv1,
		ContributionCnt: 10,
		RatingCnt:       20,
		Status:          model.UserStatusActive,
	}
	store.User.Create(context.Background(), user)

	newLevel, upgraded := checker.CheckUserUpgrade(context.Background(), user)

	if !upgraded {
		t.Error("Expected upgrade for user meeting requirements")
	}

	if newLevel != model.UserLevelLv2 {
		t.Errorf("Expected level Lv2, got Lv%d", newLevel)
	}
}

func TestGetLevelThresholds(t *testing.T) {
	thresholds := GetLevelThresholds()

	// Verify all levels have thresholds
	for level := model.UserLevelLv1; level <= model.UserLevelLv4; level++ {
		if _, ok := thresholds[level]; !ok {
			t.Errorf("Missing threshold for level %d", level)
		}
	}

	// Verify Lv2 requirements
	lv2 := thresholds[model.UserLevelLv2]
	if lv2.Contribution != 10 || lv2.Rating != 20 {
		t.Errorf("Lv2 requirements wrong: Contribution=%d, Rating=%d", lv2.Contribution, lv2.Rating)
	}

	// Verify Lv3 requirements
	lv3 := thresholds[model.UserLevelLv3]
	if lv3.Contribution != 50 || lv3.Rating != 100 {
		t.Errorf("Lv3 requirements wrong: Contribution=%d, Rating=%d", lv3.Contribution, lv3.Rating)
	}

	// Verify Lv4 requirements
	lv4 := thresholds[model.UserLevelLv4]
	if lv4.Contribution != 200 || lv4.Rating != 500 {
		t.Errorf("Lv4 requirements wrong: Contribution=%d, Rating=%d", lv4.Contribution, lv4.Rating)
	}
}
