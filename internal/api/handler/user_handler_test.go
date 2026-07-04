package handler

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// ---------- helpers ----------

// createTestUser stores a user at the given level and returns it together
// with its public key hash. The user's PublicKey is a real Ed25519 public key
// encoded as base64, which GetUserInfoHandler needs to recompute the hash.
func createTestUser(t *testing.T, store *storage.Store, name string, level int32) (*model.User, string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	pubKeyB64 := base64.StdEncoding.EncodeToString(pub)
	hash := sha256.Sum256(pub)
	pubKeyHash := hex.EncodeToString(hash[:])

	user := &model.User{
		PublicKey:    pubKeyB64,
		AgentName:    name,
		UserLevel:    level,
		Status:       model.UserStatusActive,
		RegisteredAt: model.NowMillis(),
		LastActive:   model.NowMillis(),
	}
	created, err := store.User.Create(context.Background(), user)
	require.NoError(t, err)
	return created, pubKeyHash
}

// createTestEntry stores a published entry and returns it.
func createTestEntry(t *testing.T, store *storage.Store, id, title string) *model.KnowledgeEntry {
	t.Helper()
	entry := &model.KnowledgeEntry{
		ID:       id,
		Title:    title,
		Content:  "test content",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	created, err := store.Entry.Create(context.Background(), entry)
	require.NoError(t, err)
	return created
}

// ---------- RegisterHandler (additional cases) ----------

func TestUserHandler_RegisterHandler_MissingAgentName(t *testing.T) {
	handler, _ := newTestUserHandler(t)

	body := `{"agent_name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterHandler(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Result().StatusCode)
}

// ---------- GetUserInfoHandler (additional cases) ----------

func TestUserHandler_GetUserInfoHandler_NoAuth(t *testing.T) {
	handler, _ := newTestUserHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/info", nil)
	rec := httptest.NewRecorder()

	handler.GetUserInfoHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestUserHandler_GetUserInfoHandler_Success(t *testing.T) {
	handler, store := newTestUserHandler(t)
	user, _ := createTestUser(t, store, "info-agent", model.UserLevelLv1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/info", nil)
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetUserInfoHandler(rec, req)

	resp := rec.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var apiResp APIResponse
	err := json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)
	assert.Equal(t, 0, apiResp.Code)

	data, ok := apiResp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "info-agent", data["agent_name"])
	assert.Equal(t, float64(model.UserLevelLv1), data["user_level"])
	assert.Equal(t, user.PublicKey, data["public_key"])
}

// ---------- UpdateUserInfoHandler (additional cases) ----------

func TestUserHandler_UpdateUserInfoHandler_NoAuth(t *testing.T) {
	handler, _ := newTestUserHandler(t)

	body := `{"agent_name":"new-name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateUserInfoHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestUserHandler_UpdateUserInfoHandler_Success(t *testing.T) {
	handler, store := newTestUserHandler(t)
	user, _ := createTestUser(t, store, "old-name", model.UserLevelLv1)

	body := `{"agent_name":"new-name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.UpdateUserInfoHandler(rec, req)

	resp := rec.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var apiResp APIResponse
	err := json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)
	assert.Equal(t, 0, apiResp.Code)
}

// ---------- RateEntryHandler (additional cases) ----------

func TestUserHandler_RateEntryHandler_Success(t *testing.T) {
	handler, store := newTestUserHandler(t)
	user, _ := createTestUser(t, store, "rater", model.UserLevelLv1)
	entry := createTestEntry(t, store, "e1", "Test Entry")

	body := `{"score":4.5,"comment":"great"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/e1/rate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.RateEntryHandler(rec, req)

	resp := rec.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var apiResp APIResponse
	err := json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)
	assert.Equal(t, 0, apiResp.Code)

	data, ok := apiResp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, entry.ID, data["entryId"])
	assert.Equal(t, 4.5, data["score"])
}

func TestUserHandler_RateEntryHandler_Lv0Denied(t *testing.T) {
	handler, store := newTestUserHandler(t)
	user, _ := createTestUser(t, store, "basic-user", model.UserLevelLv0)
	createTestEntry(t, store, "e1", "Test Entry")

	body := `{"score":3.0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/e1/rate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.RateEntryHandler(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Result().StatusCode)
}

func TestUserHandler_RateEntryHandler_NoAuth(t *testing.T) {
	handler, _ := newTestUserHandler(t)

	body := `{"score":3.0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/e1/rate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RateEntryHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

// TestRateEntryHandler_ConcurrentSameRaterNoDup 验证同一用户并发评分只成功一次。
// 重构前 GetByRater 查重与 Create 之间无锁，并发双重投票可全部成功 → ok > 1。
func TestRateEntryHandler_ConcurrentSameRaterNoDup(t *testing.T) {
	handler, store := newTestUserHandler(t)
	user, _ := createTestUser(t, store, "rater", model.UserLevelLv1)
	createTestEntry(t, store, "e1", "Test")

	const n = 50
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		ok, dup int
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := `{"score":4.0,"comment":"x"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/e1/rate", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(setUserInContext(req.Context(), user))
			rec := httptest.NewRecorder()
			handler.RateEntryHandler(rec, req)
			mu.Lock()
			defer mu.Unlock()
			if rec.Code == http.StatusCreated {
				ok++
			} else {
				dup++
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("expected exactly 1 successful rating, got %d (race: double voting)", ok)
	}
	if dup != n-1 {
		t.Fatalf("expected %d rejections, got %d", n-1, dup)
	}
}

// TestRateEntryHandler_UpdatesEntryScore 验证评分后 entry.Score/ScoreCount 被重算并持久化。
// 重构前 RateEntryHandler 不重算，评分后 entry.Score 永远陈旧（0）。
func TestRateEntryHandler_UpdatesEntryScore(t *testing.T) {
	handler, store := newTestUserHandler(t)
	user, _ := createTestUser(t, store, "rater", model.UserLevelLv1)
	createTestEntry(t, store, "e1", "Test")

	body := `{"score":4.0,"comment":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/e1/rate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()
	handler.RateEntryHandler(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// 评分后 entry.ScoreCount 应为 1，Score 应为该评分（weight=GetLevelWeight(Lv1)）
	updated, err := store.Entry.Get(context.Background(), "e1")
	require.NoError(t, err)
	assert.Equal(t, int32(1), updated.ScoreCount, "ScoreCount must be recomputed after rating")
	assert.Greater(t, updated.Score, 0.0, "Score must be recomputed (not stale 0)")
}

// ---------- SendVerificationCodeHandler: verify-code leak gate (P1.1) ----------

// TestSendVerificationCodeHandler_CodeNotLeakedByDefault: with the dev flag off
// (the default) the response MUST NOT contain the plaintext code.
func TestSendVerificationCodeHandler_CodeNotLeakedByDefault(t *testing.T) {
	handler, store := newTestUserHandler(t)
	user, _ := createTestUser(t, store, "leak-agent", model.UserLevelLv0)

	body, _ := json.Marshal(map[string]string{"email": "leak@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/send-verification", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()

	handler.SendVerificationCodeHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "data is a map")

	_, hasCode := data["code"]
	assert.False(t, hasCode, "verification code must NOT be leaked by default")
}

// TestSendVerificationCodeHandler_CodeReturnedInDevMode: with the dev flag on
// (test environments) the code IS returned so tests can complete the flow.
func TestSendVerificationCodeHandler_CodeReturnedInDevMode(t *testing.T) {
	handler, store := newTestUserHandler(t)
	handler.SetDevReturnVerificationCode(true)
	user, _ := createTestUser(t, store, "dev-agent", model.UserLevelLv0)

	body, _ := json.Marshal(map[string]string{"email": "dev@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/send-verification", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()

	handler.SendVerificationCodeHandler(rec, req)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, _ := resp.Data.(map[string]interface{})
	code, ok := data["code"].(string)
	assert.True(t, ok, "code must be present in dev mode")
	assert.NotEmpty(t, code)
}
