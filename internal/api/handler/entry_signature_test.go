package handler

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/daifei0527/polyant/pkg/crypto"
)

// createTestUserWithKey 与 createTestUser 一致，但保留私钥以便测试中对内容签名。
func createTestUserWithKey(t *testing.T, store *storage.Store, name string, level int32) (*model.User, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pub)
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
	return created, priv
}

// signContentB64 用私钥对 (title,content,category) 签名，返回 base64 字符串。
func signContentB64(t *testing.T, priv ed25519.PrivateKey, title, content, category string) string {
	t.Helper()
	sig, err := crypto.SignContent(priv, title, content, category)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(sig)
}

// TestCreateEntryHandler_RejectsMissingSignature: 无内容签名 → 401（R1-B3）。
func TestCreateEntryHandler_RejectsMissingSignature(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	user, _ := createTestUserWithKey(t, memStore, "author", model.UserLevelLv1)
	body := `{"title":"T","content":"C","category":"cat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/create", bytes.NewBufferString(body))
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()

	h.CreateEntryHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode,
		"create without creator_signature must be rejected")
}

// TestCreateEntryHandler_RejectsForgedSignature: 签名内容与服务端存储不一致 → 401。
func TestCreateEntryHandler_RejectsForgedSignature(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	user, priv := createTestUserWithKey(t, memStore, "author", model.UserLevelLv1)
	// 签名的是 "forged" 内容，但请求体声明 "real" 内容
	forged := signContentB64(t, priv, "forged", "C", "cat")
	body := `{"title":"real","content":"C","category":"cat","creator_signature":"` + forged + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/create", bytes.NewBufferString(body))
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()

	h.CreateEntryHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode,
		"create with forged signature must be rejected")
}

// TestCreateEntryHandler_AcceptsValidSignature: 合法签名 → 201；条目落库带签名；
// 且推送到 seed 时携带真实签名（而非 nil）。
func TestCreateEntryHandler_AcceptsValidSignature(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)
	pusher := &sigCapturePusher{pushed: make(chan pushedItem, 1)}
	h.SetEntryPusher(pusher)

	user, priv := createTestUserWithKey(t, memStore, "author", model.UserLevelLv1)
	sig := signContentB64(t, priv, "T", "C", "cat")
	body := `{"title":"T","content":"C","category":"cat","creator_signature":"` + sig + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/create", bytes.NewBufferString(body))
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()

	h.CreateEntryHandler(rec, req)

	require.Equal(t, http.StatusCreated, rec.Result().StatusCode)

	// 落库条目应带签名
	stored, gerr := memStore.Entry.Get(context.Background(), extractIDFromCreateResp(t, rec.Body.Bytes()))
	require.NoError(t, gerr)
	assert.NotEmpty(t, stored.Signature, "stored entry must carry creator signature")
	assert.Equal(t, "ed25519", stored.SignAlgorithm)

	// 推送必须携带与落库一致的真实签名
	select {
	case got := <-pusher.pushed:
		require.NotNil(t, got.signature, "push must carry real signature, not nil")
		assert.Equal(t, stored.Signature, got.signature)
	case <-time.After(2 * time.Second):
		t.Fatal("entry was not pushed to seed within timeout")
	}
}

// TestUpdateEntryHandler_RequiresSignatureOnContentChange:
// 改 title/content/category 之一必须带 CreatedBy 私钥的新签名；缺失或伪造 → 401。
func TestUpdateEntryHandler_RequiresSignatureOnContentChange(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	user, priv := createTestUserWithKey(t, memStore, "author", model.UserLevelLv1)

	// 先创建一条已签名条目
	sig := signContentB64(t, priv, "T0", "C0", "cat")
	createBody := `{"title":"T0","content":"C0","category":"cat","creator_signature":"` + sig + `"}`
	creq := httptest.NewRequest(http.MethodPost, "/api/v1/entry/create", bytes.NewBufferString(createBody))
	creq = creq.WithContext(setUserInContext(creq.Context(), user))
	rec := httptest.NewRecorder()
	h.CreateEntryHandler(rec, creq)
	require.Equal(t, http.StatusCreated, rec.Result().StatusCode)
	entryID := extractIDFromCreateResp(t, rec.Body.Bytes())

	// 1) 改 title 但不带签名 → 401
	updReq := httptest.NewRequest(http.MethodPut, "/api/v1/entry/update/"+entryID,
		bytes.NewBufferString(`{"title":"T1"}`))
	updReq = updReq.WithContext(setUserInContext(updReq.Context(), user))
	updRec := httptest.NewRecorder()
	h.UpdateEntryHandler(updRec, updReq)
	assert.Equal(t, http.StatusUnauthorized, updRec.Result().StatusCode,
		"content change without new signature must be rejected")

	// 2) 改 title 带伪造签名（签的是旧内容）→ 401
	forged := signContentB64(t, priv, "T0", "C0", "cat") // 旧内容
	updReq2 := httptest.NewRequest(http.MethodPut, "/api/v1/entry/update/"+entryID,
		bytes.NewBufferString(`{"title":"T1","creator_signature":"`+forged+`"}`))
	updReq2 = updReq2.WithContext(setUserInContext(updReq2.Context(), user))
	updRec2 := httptest.NewRecorder()
	h.UpdateEntryHandler(updRec2, updReq2)
	assert.Equal(t, http.StatusUnauthorized, updRec2.Result().StatusCode,
		"content change with stale signature must be rejected")

	// 3) 改 title 带正确新签名 → 200
	good := signContentB64(t, priv, "T1", "C0", "cat")
	updReq3 := httptest.NewRequest(http.MethodPut, "/api/v1/entry/update/"+entryID,
		bytes.NewBufferString(`{"title":"T1","creator_signature":"`+good+`"}`))
	updReq3 = updReq3.WithContext(setUserInContext(updReq3.Context(), user))
	updRec3 := httptest.NewRecorder()
	h.UpdateEntryHandler(updRec3, updReq3)
	assert.Equal(t, http.StatusOK, updRec3.Result().StatusCode,
		"content change with valid new signature must succeed")
}

// TestUpdateEntryHandler_MetaChangeKeepsSignature:
// 仅改 tags（非内容字段）时无需新签名，原签名保留。
func TestUpdateEntryHandler_MetaChangeKeepsSignature(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)
	h := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	user, priv := createTestUserWithKey(t, memStore, "author", model.UserLevelLv1)
	sig := signContentB64(t, priv, "T0", "C0", "cat")
	createBody := `{"title":"T0","content":"C0","category":"cat","creator_signature":"` + sig + `"}`
	creq := httptest.NewRequest(http.MethodPost, "/api/v1/entry/create", bytes.NewBufferString(createBody))
	creq = creq.WithContext(setUserInContext(creq.Context(), user))
	rec := httptest.NewRecorder()
	h.CreateEntryHandler(rec, creq)
	require.Equal(t, http.StatusCreated, rec.Result().StatusCode)
	entryID := extractIDFromCreateResp(t, rec.Body.Bytes())

	// 仅改 tags，不带签名 → 200
	updReq := httptest.NewRequest(http.MethodPut, "/api/v1/entry/update/"+entryID,
		bytes.NewBufferString(`{"tags":["new"]}`))
	updReq = updReq.WithContext(setUserInContext(updReq.Context(), user))
	updRec := httptest.NewRecorder()
	h.UpdateEntryHandler(updRec, updReq)
	assert.Equal(t, http.StatusOK, updRec.Result().StatusCode,
		"meta-only change must not require a new signature")

	stored, gerr := memStore.Entry.Get(context.Background(), entryID)
	require.NoError(t, gerr)
	assert.NotEmpty(t, stored.Signature, "original signature must be preserved on meta-only update")
}

// pushedItem 记录被推送的条目及其签名。
type pushedItem struct {
	entry     *model.KnowledgeEntry
	signature []byte
}

// sigCapturePusher 捕获推送签名以便断言推送链携带真实签名。
type sigCapturePusher struct {
	pushed chan pushedItem
}

func (f *sigCapturePusher) PushEntry(entry *model.KnowledgeEntry, signature []byte) error {
	f.pushed <- pushedItem{entry: entry, signature: signature}
	return nil
}

// extractIDFromCreateResp 从 CreateEntry 响应体解析出条目 ID。
func extractIDFromCreateResp(t *testing.T, b []byte) string {
	t.Helper()
	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(b, &resp))
	require.NotEmpty(t, resp.Data.ID, "create response must contain entry id")
	return resp.Data.ID
}
