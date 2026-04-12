package user

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

// validBase64Key generates a valid base64 encoded public key for testing
func validBase64Key(name string) string {
	// Use a simple base64 encoded string that can be decoded
	// "test-" + name padded to make valid base64
	return "dGVzdC1rZXk=" // base64 of "test-key"
}

func TestNewUserManager(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	if mgr == nil {
		t.Fatal("NewUserManager returned nil")
	}
}

func TestUserManager_Register(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	publicKey := validBase64Key("test")
	user, err := mgr.Register(context.Background(), publicKey, "test-agent")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if user.PublicKey != publicKey {
		t.Errorf("Expected public key '%s', got %s", publicKey, user.PublicKey)
	}

	if user.AgentName != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got %s", user.AgentName)
	}

	if user.UserLevel != model.UserLevelLv0 {
		t.Errorf("New user should be Lv0, got %d", user.UserLevel)
	}
}

func TestUserManager_Register_Duplicate(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	publicKey := validBase64Key("dup")

	// First registration
	_, err := mgr.Register(context.Background(), publicKey, "agent1")
	if err != nil {
		t.Fatalf("First Register failed: %v", err)
	}

	// Duplicate registration
	_, err = mgr.Register(context.Background(), publicKey, "agent2")
	if err == nil {
		t.Error("Duplicate registration should return error")
	}
}

func TestUserManager_GetByPublicKey(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	publicKey := validBase64Key("get")

	// Register a user
	mgr.Register(context.Background(), publicKey, "test-agent")

	// Get the user
	user, err := mgr.GetByPublicKey(context.Background(), publicKey)
	if err != nil {
		t.Fatalf("GetByPublicKey failed: %v", err)
	}

	if user.AgentName != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got %s", user.AgentName)
	}
}

func TestUserManager_GetByPublicKey_NotFound(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	_, err := mgr.GetByPublicKey(context.Background(), "nonexistent-key")
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserManager_UpdateLastActive(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	publicKey := validBase64Key("active")

	// Register a user
	mgr.Register(context.Background(), publicKey, "test-agent")

	// Update last active
	err := mgr.UpdateLastActive(context.Background(), publicKey)
	if err != nil {
		t.Fatalf("UpdateLastActive failed: %v", err)
	}

	// Verify last active was updated
	user, _ := mgr.GetByPublicKey(context.Background(), publicKey)
	if user.LastActive == 0 {
		t.Error("LastActive should be updated")
	}
}

func TestUserManager_SetEmail(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	publicKey := validBase64Key("email")

	// Register a user
	mgr.Register(context.Background(), publicKey, "test-agent")

	// Set email
	err := mgr.SetEmail(context.Background(), publicKey, "test@example.com")
	if err != nil {
		t.Fatalf("SetEmail failed: %v", err)
	}

	// Verify email was set
	user, _ := mgr.GetByPublicKey(context.Background(), publicKey)
	if user.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got %s", user.Email)
	}

	if user.EmailVerified {
		t.Error("Email should not be verified after SetEmail")
	}
}

func TestUserManager_VerifyEmail(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	publicKey := validBase64Key("verify")

	// Register a user and set email
	mgr.Register(context.Background(), publicKey, "test-agent")
	mgr.SetEmail(context.Background(), publicKey, "test@example.com")

	// Verify email
	err := mgr.VerifyEmail(context.Background(), publicKey, "test@example.com")
	if err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	// Verify email is verified and level upgraded
	user, _ := mgr.GetByPublicKey(context.Background(), publicKey)
	if !user.EmailVerified {
		t.Error("Email should be verified")
	}

	if user.UserLevel != model.UserLevelLv1 {
		t.Errorf("User should be upgraded to Lv1, got %d", user.UserLevel)
	}
}

func TestUserManager_VerifyEmail_WrongEmail(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	publicKey := validBase64Key("wrong-email")

	// Register a user and set email
	mgr.Register(context.Background(), publicKey, "test-agent")
	mgr.SetEmail(context.Background(), publicKey, "correct@example.com")

	// Try to verify with wrong email
	err := mgr.VerifyEmail(context.Background(), publicKey, "wrong@example.com")
	if err != ErrInvalidEmail {
		t.Errorf("Expected ErrInvalidEmail, got %v", err)
	}
}

func TestUserManager_CheckLevelUpgrade(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	tests := []struct {
		name           string
		level          int32
		contributions  int32
		ratings        int32
		expectedLevel  int32
		expectUpgrade  bool
	}{
		{"Lv0 stays Lv0", model.UserLevelLv0, 0, 0, model.UserLevelLv0, false},
		{"Lv1 -> Lv2", model.UserLevelLv1, 10, 20, model.UserLevelLv2, true},
		{"Lv1 not enough contributions", model.UserLevelLv1, 5, 20, model.UserLevelLv1, false},
		{"Lv1 not enough ratings", model.UserLevelLv1, 10, 10, model.UserLevelLv1, false},
		{"Lv2 -> Lv3", model.UserLevelLv2, 50, 100, model.UserLevelLv3, true},
		{"Lv3 -> Lv4", model.UserLevelLv3, 200, 500, model.UserLevelLv4, true},
		{"Lv4 stays Lv4", model.UserLevelLv4, 1000, 1000, model.UserLevelLv4, false},
		{"Lv5 stays Lv5", model.UserLevelLv5, 1000, 1000, model.UserLevelLv5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publicKey := validBase64Key(tt.name)
			user := &model.User{
				PublicKey:       publicKey,
				AgentName:       tt.name,
				UserLevel:       tt.level,
				ContributionCnt: tt.contributions,
				RatingCnt:       tt.ratings,
			}
			store.User.Create(context.Background(), user)

			newLevel, upgraded := mgr.CheckLevelUpgrade(context.Background(), user)

			if upgraded != tt.expectUpgrade {
				t.Errorf("Expected upgrade %v, got %v", tt.expectUpgrade, upgraded)
			}

			if newLevel != tt.expectedLevel {
				t.Errorf("Expected level %d, got %d", tt.expectedLevel, newLevel)
			}
		})
	}
}

func TestUserManager_IncrementContribution(t *testing.T) {
	store := newTestStore(t)
	mgr := NewUserManager(store)

	publicKey := validBase64Key("contrib")

	// Register a user
	mgr.Register(context.Background(), publicKey, "test-agent")

	// Increment contribution
	err := mgr.IncrementContribution(context.Background(), publicKey)
	if err != nil {
		t.Fatalf("IncrementContribution failed: %v", err)
	}

	// Verify contribution count
	user, _ := mgr.GetByPublicKey(context.Background(), publicKey)
	if user.ContributionCnt != 1 {
		t.Errorf("Expected ContributionCnt 1, got %d", user.ContributionCnt)
	}
}

func TestHashPublicKey(t *testing.T) {
	// Test with base64 encoded string
	hash1 := HashPublicKey("dGVzdC1rZXk=") // base64 of "test-key"
	if len(hash1) != 64 { // SHA256 produces 64 hex characters
		t.Errorf("Hash length should be 64, got %d", len(hash1))
	}

	// Test with non-base64 string
	hash2 := HashPublicKey("plain-string")
	if len(hash2) != 64 {
		t.Errorf("Hash length should be 64, got %d", len(hash2))
	}

	// Same input should produce same hash
	hash3 := HashPublicKey("dGVzdC1rZXk=")
	if hash1 != hash3 {
		t.Error("Same input should produce same hash")
	}
}
