package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coreadmin "github.com/daifei0527/polyant/internal/core/admin"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/daifei0527/polyant/pkg/crypto"
)

// fakeUserStore 最小 UserStore，按 pubkey 哈希槽位返回预设用户，便于隔离测试 handler。
type fakeUserStore struct {
	byKey map[string]*model.User
}

func (f *fakeUserStore) Create(ctx context.Context, u *model.User) (*model.User, error) {
	f.byKey[u.PublicKey] = u
	return u, nil
}
func (f *fakeUserStore) Get(ctx context.Context, key string) (*model.User, error) {
	if u, ok := f.byKey[key]; ok {
		return u, nil
	}
	return nil, errNotFound
}
func (f *fakeUserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	for _, u := range f.byKey {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errNotFound
}
func (f *fakeUserStore) Update(ctx context.Context, u *model.User) (*model.User, error) {
	f.byKey[u.PublicKey] = u
	return u, nil
}
func (f *fakeUserStore) List(ctx context.Context, flt storage.UserFilter) ([]*model.User, int64, error) {
	return nil, 0, nil
}

var errNotFound = &simpleErr{"user not found"}

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }

// newHandler 构造带 fake store 的 SessionHandler。
func newHandler(t *testing.T, users ...*model.User) *SessionHandler {
	t.Helper()
	store := &fakeUserStore{byKey: map[string]*model.User{}}
	for _, u := range users {
		store.byKey[u.PublicKey] = u
	}
	sm := coreadmin.NewSessionManager(time.Hour)
	return NewSessionHandler(sm, store, "127.0.0.1:18531")
}

func TestCreateSession_rejectsLowLevel(t *testing.T) {
	// Lv1 用户即使从 localhost 请求，也不得签发 admin 会话
	h := newHandler(t, &model.User{PublicKey: "lv1-key", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive})
	body, _ := json.Marshal(map[string]string{"public_key": "lv1-key"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/create", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:55555"
	w := httptest.NewRecorder()
	h.CreateSessionHandler(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Lv1 user must be rejected with 403, got %d", w.Code)
	}
}

func TestCreateSession_acceptsLv4FromLocalhost(t *testing.T) {
	h := newHandler(t, &model.User{PublicKey: "lv4-key", UserLevel: model.UserLevelLv4, Status: model.UserStatusActive, AgentName: "admin"})
	body, _ := json.Marshal(map[string]string{"public_key": "lv4-key"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/create", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:55555"
	w := httptest.NewRecorder()
	h.CreateSessionHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Lv4 local user must get 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Token == "" {
		t.Fatal("expected non-empty token")
	}
}

// ---- R1-A3: password login + session self-check ----

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := crypto.HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return h
}

func TestLoginHandler_success(t *testing.T) {
	u := &model.User{
		PublicKey:    "admin-pk",
		Email:        "admin@example.com",
		UserLevel:    model.UserLevelLv5,
		Status:       model.UserStatusActive,
		AgentName:    "the-admin",
		PasswordHash: mustHash(t, "correct-horse"),
	}
	h := newHandler(t, u)
	body, _ := json.Marshal(map[string]string{"identifier": "admin@example.com", "password": "correct-horse"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid login must be 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Token string `json:"token"`
			User  struct {
				Level int32 `json:"user_level"`
			} `json:"user"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Token == "" {
		t.Fatal("expected token")
	}
	if resp.Data.User.Level != 5 {
		t.Fatalf("expected level 5, got %d", resp.Data.User.Level)
	}
}

func TestLoginHandler_wrongPassword(t *testing.T) {
	u := &model.User{PublicKey: "admin-pk", Email: "a@x.com", UserLevel: model.UserLevelLv5, Status: model.UserStatusActive, PasswordHash: mustHash(t, "right")}
	h := newHandler(t, u)
	body, _ := json.Marshal(map[string]string{"identifier": "a@x.com", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password must be 401, got %d", w.Code)
	}
}

func TestLoginHandler_unknownUser_noEnumeration(t *testing.T) {
	h := newHandler(t) // 空存储
	body, _ := json.Marshal(map[string]string{"identifier": "ghost@x.com", "password": "whatever"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user must be 401 (no enumeration), got %d", w.Code)
	}
}

func TestLoginHandler_lowLevel(t *testing.T) {
	u := &model.User{PublicKey: "lv1-pk", Email: "u@x.com", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive, PasswordHash: mustHash(t, "pw")}
	h := newHandler(t, u)
	body, _ := json.Marshal(map[string]string{"identifier": "u@x.com", "password": "pw"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Lv1 login must be 403, got %d", w.Code)
	}
}

func TestGetSessionHandler_valid(t *testing.T) {
	u := &model.User{PublicKey: "admin-pk", Email: "a@x.com", UserLevel: model.UserLevelLv5, Status: model.UserStatusActive, AgentName: "ad"}
	h := newHandler(t, u)
	token, _ := h.sessionMgr.CreateSession("admin-pk")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.GetSessionHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid token must be 200, got %d", w.Code)
	}
	var resp struct {
		Data struct {
			Level int32 `json:"user_level"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Level != 5 {
		t.Fatalf("expected level 5, got %d", resp.Data.Level)
	}
}

func TestGetSessionHandler_invalidToken(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/session", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	w := httptest.NewRecorder()
	h.GetSessionHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token must be 401, got %d", w.Code)
	}
}
