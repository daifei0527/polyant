package handler

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newTestStatsHandler(t *testing.T) (*StatsHandler, *storage.Store) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	handler := NewStatsHandler(store)
	return handler, store
}

func TestStatsHandler_GetUserStatsHandler(t *testing.T) {
	handler, store := newTestStatsHandler(t)

	// Create test users with different levels
	levels := []int32{model.UserLevelLv0, model.UserLevelLv1, model.UserLevelLv2, model.UserLevelLv3, model.UserLevelLv4}
	for i := 0; i < 5; i++ {
		pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
		pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

		user := &model.User{
			PublicKey: pubKeyB64,
			AgentName: "user-" + string(rune('a'+i)),
			UserLevel: levels[i],
			Status:    model.UserStatusActive,
		}
		store.User.Create(context.Background(), user)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/users", nil)
	rec := httptest.NewRecorder()

	handler.GetUserStatsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["totalUsers"] == nil {
		t.Error("Expected totalUsers in response")
	}
}

func TestStatsHandler_GetUserStatsHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestStatsHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/stats/users", nil)
	rec := httptest.NewRecorder()

	handler.GetUserStatsHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestStatsHandler_GetContributionStatsHandler(t *testing.T) {
	handler, store := newTestStatsHandler(t)

	// Create test users with contributions
	for i := 0; i < 5; i++ {
		pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
		pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

		user := &model.User{
			PublicKey:     pubKeyB64,
			AgentName:     "contributor-" + string(rune('a'+i)),
			UserLevel:     model.UserLevelLv1,
			Status:        model.UserStatusActive,
			ContributionCnt: int32((i + 1) * 10),
			RatingCnt:     int32((i + 1) * 5),
		}
		store.User.Create(context.Background(), user)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/contributions?page=1&limit=10&sort=entry_count", nil)
	rec := httptest.NewRecorder()

	handler.GetContributionStatsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["contributions"] == nil {
		t.Error("Expected contributions in response")
	}
	if data["total"] == nil {
		t.Error("Expected total in response")
	}
}

func TestStatsHandler_GetContributionStatsHandler_DefaultPaging(t *testing.T) {
	handler, _ := newTestStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/contributions", nil)
	rec := httptest.NewRecorder()

	handler.GetContributionStatsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestStatsHandler_GetContributionStatsHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestStatsHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/stats/contributions", nil)
	rec := httptest.NewRecorder()

	handler.GetContributionStatsHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestStatsHandler_GetActivityTrendHandler(t *testing.T) {
	handler, _ := newTestStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/activity?days=7", nil)
	rec := httptest.NewRecorder()

	handler.GetActivityTrendHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["trend"] == nil {
		t.Error("Expected trend in response")
	}
}

func TestStatsHandler_GetActivityTrendHandler_DefaultDays(t *testing.T) {
	handler, _ := newTestStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/activity", nil)
	rec := httptest.NewRecorder()

	handler.GetActivityTrendHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestStatsHandler_GetActivityTrendHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestStatsHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/stats/activity", nil)
	rec := httptest.NewRecorder()

	handler.GetActivityTrendHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestStatsHandler_GetRegistrationTrendHandler(t *testing.T) {
	handler, _ := newTestStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/registrations?days=30", nil)
	rec := httptest.NewRecorder()

	handler.GetRegistrationTrendHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["trend"] == nil {
		t.Error("Expected trend in response")
	}
}

func TestStatsHandler_GetRegistrationTrendHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestStatsHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/stats/registrations", nil)
	rec := httptest.NewRecorder()

	handler.GetRegistrationTrendHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
