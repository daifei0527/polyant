package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daifei0527/polyant/internal/core/email"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newTestVerifyHandler(t *testing.T) (*UserHandler, *storage.Store) {
	t.Helper()
	store, err := storage.NewMemoryStore()
	require.NoError(t, err)

	vm := email.NewVerificationManager()
	handler := NewUserHandler(store, store.User, store.Entry, store.Rating, nil, vm)
	return handler, store
}

func TestVerifyEmailHandler_Success(t *testing.T) {
	handler, store := newTestVerifyHandler(t)
	user, _ := createTestUser(t, store, "verify-agent", model.UserLevelLv0)

	code := handler.verificationMgr.GenerateCode("test@example.com")

	body, _ := json.Marshal(map[string]string{
		"email": "test@example.com",
		"code":  code,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.VerifyEmailHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(model.UserLevelLv1), data["user_level"])
	assert.Equal(t, true, data["email_verified"])
}

func TestVerifyEmailHandler_InvalidCode(t *testing.T) {
	handler, store := newTestVerifyHandler(t)
	user, _ := createTestUser(t, store, "verify-agent", model.UserLevelLv0)

	body, _ := json.Marshal(map[string]string{
		"email": "test@example.com",
		"code":  "wrong-code",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.VerifyEmailHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestVerifyEmailHandler_ExpiredCode(t *testing.T) {
	handler, store := newTestVerifyHandler(t)
	user, _ := createTestUser(t, store, "verify-agent", model.UserLevelLv0)

	code := handler.verificationMgr.GenerateCode("test@example.com")
	handler.verificationMgr.Invalidate(code)

	body, _ := json.Marshal(map[string]string{
		"email": "test@example.com",
		"code":  code,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.VerifyEmailHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestVerifyEmailHandler_NoAuth(t *testing.T) {
	handler, _ := newTestVerifyHandler(t)

	body, _ := json.Marshal(map[string]string{
		"email": "test@example.com",
		"code":  "123456",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.VerifyEmailHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestVerifyEmailHandler_MissingFields(t *testing.T) {
	handler, store := newTestVerifyHandler(t)
	user, _ := createTestUser(t, store, "verify-agent", model.UserLevelLv0)

	body, _ := json.Marshal(map[string]string{
		"email": "",
		"code":  "",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.VerifyEmailHandler(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Result().StatusCode)
}

func TestVerifyEmailHandler_EmailAlreadyUsed(t *testing.T) {
	handler, store := newTestVerifyHandler(t)

	user1, _ := createTestUser(t, store, "agent-1", model.UserLevelLv1)
	user1.Email = "taken@example.com"
	user1.EmailVerified = true
	store.User.Update(context.Background(), user1)

	user2, _ := createTestUser(t, store, "agent-2", model.UserLevelLv0)
	code := handler.verificationMgr.GenerateCode("taken@example.com")

	body, _ := json.Marshal(map[string]string{
		"email": "taken@example.com",
		"code":  code,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user2)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.VerifyEmailHandler(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Result().StatusCode)
}

// ---------- SendVerificationCodeHandler: email uniqueness dedup (P1.2) ----------

func TestSendVerificationCodeHandler_EmailTakenByOther(t *testing.T) {
	handler, store := newTestVerifyHandler(t)

	// Another user already owns this email.
	owner, _ := createTestUser(t, store, "owner", model.UserLevelLv1)
	owner.Email = "taken@example.com"
	owner.EmailVerified = true
	store.User.Update(context.Background(), owner)

	// A different user tries to claim the same email.
	other, _ := createTestUser(t, store, "other", model.UserLevelLv0)
	body, _ := json.Marshal(map[string]string{"email": "taken@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/send-verification", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), other))
	rec := httptest.NewRecorder()

	handler.SendVerificationCodeHandler(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Result().StatusCode)
}

func TestSendVerificationCodeHandler_OwnEmailAllowed(t *testing.T) {
	handler, store := newTestVerifyHandler(t)

	owner, _ := createTestUser(t, store, "owner", model.UserLevelLv1)
	owner.Email = "mine@example.com"
	owner.EmailVerified = true
	store.User.Update(context.Background(), owner)

	body, _ := json.Marshal(map[string]string{"email": "mine@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/send-verification", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), owner))
	rec := httptest.NewRecorder()

	handler.SendVerificationCodeHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)
}
