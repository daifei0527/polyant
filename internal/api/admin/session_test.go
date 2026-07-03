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
