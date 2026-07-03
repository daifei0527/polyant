package sync

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	protocolpkg "github.com/daifei0527/polyant/internal/network/protocol"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/daifei0527/polyant/pkg/crypto"
)

// newPushTestEngine 构造一个带内存存储的 SyncEngine（p2pHost/protocol 为 nil，push handler 不依赖它们）。
func newPushTestEngine(t *testing.T, requireSigs bool) (*SyncEngine, *storage.Store) {
	t.Helper()
	store, err := storage.NewMemoryStore()
	require.NoError(t, err)
	se := NewSyncEngine(nil, nil, store, &SyncConfig{AutoSync: false, RequireEntrySignatures: requireSigs})
	return se, store
}

// TestHandlePushEntry_ValidSignatureAccepted: 合法签名推送 → 接收方存储成功。
func TestHandlePushEntry_ValidSignatureAccepted(t *testing.T) {
	se, store := newPushTestEngine(t, false)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	sig, err := crypto.SignContent(priv, "T", "C", "cat")
	require.NoError(t, err)

	entry := &model.KnowledgeEntry{
		ID:       "e1",
		Title:    "T",
		Content:  "C",
		Category: "cat",
		CreatedBy: pubB64,
		Version:  1,
		Status:   model.EntryStatusPublished,
		Signature: sig,
	}
	entryJSON, _ := entry.ToJSON()

	ack, err := se.HandlePushEntry(context.Background(), &protocolpkg.PushEntry{
		EntryID: entry.ID, Entry: entryJSON, CreatorSignature: sig,
	})
	require.NoError(t, err)
	assert.True(t, ack.Accepted, "valid signature must be accepted")

	got, gerr := store.Entry.Get(context.Background(), "e1")
	require.NoError(t, gerr)
	assert.NotEmpty(t, got.Signature, "stored entry must carry signature")
}

// TestHandlePushEntry_ForgedSignatureRejected: 篡改 content 但沿用旧签名 → 拒绝、不写库。
func TestHandlePushEntry_ForgedSignatureRejected(t *testing.T) {
	se, store := newPushTestEngine(t, false)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	sig, _ := crypto.SignContent(priv, "T", "C-real", "cat") // 签的是 C-real

	// 但条目声明 C-forged → 验签失败
	entry := &model.KnowledgeEntry{
		ID: "e2", Title: "T", Content: "C-forged", Category: "cat",
		CreatedBy: pubB64, Version: 1, Status: model.EntryStatusPublished, Signature: sig,
	}
	entryJSON, _ := entry.ToJSON()

	ack, err := se.HandlePushEntry(context.Background(), &protocolpkg.PushEntry{
		EntryID: entry.ID, Entry: entryJSON, CreatorSignature: sig,
	})
	require.NoError(t, err)
	assert.False(t, ack.Accepted, "forged signature must be rejected")
	assert.Contains(t, ack.RejectReason, "forged")

	_, gerr := store.Entry.Get(context.Background(), "e2")
	assert.Error(t, gerr, "forged entry must not be stored")
}

// TestHandlePushEntry_UnsignedDefault: 默认配置无签名 → 接受并记日志（兼容历史数据）。
func TestHandlePushEntry_UnsignedDefault(t *testing.T) {
	se, store := newPushTestEngine(t, false)
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	entry := &model.KnowledgeEntry{
		ID: "e3", Title: "T", Content: "C", Category: "cat",
		CreatedBy: pubB64, Version: 1, Status: model.EntryStatusPublished, // 无 Signature
	}
	entryJSON, _ := entry.ToJSON()

	ack, err := se.HandlePushEntry(context.Background(), &protocolpkg.PushEntry{EntryID: entry.ID, Entry: entryJSON})
	require.NoError(t, err)
	assert.True(t, ack.Accepted, "unsigned entry must be accepted in default (soft) mode")

	_, gerr := store.Entry.Get(context.Background(), "e3")
	assert.NoError(t, gerr, "unsigned entry must be stored in default mode")
}

// TestHandlePushEntry_UnsignedRejectedWhenRequired: RequireEntrySignatures=true 时无签名 → 拒绝。
func TestHandlePushEntry_UnsignedRejectedWhenRequired(t *testing.T) {
	se, store := newPushTestEngine(t, true)
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	entry := &model.KnowledgeEntry{
		ID: "e4", Title: "T", Content: "C", Category: "cat",
		CreatedBy: pubB64, Version: 1, Status: model.EntryStatusPublished,
	}
	entryJSON, _ := entry.ToJSON()

	ack, err := se.HandlePushEntry(context.Background(), &protocolpkg.PushEntry{EntryID: entry.ID, Entry: entryJSON})
	require.NoError(t, err)
	assert.False(t, ack.Accepted, "unsigned entry must be rejected when require_entry_signatures=true")
	assert.Contains(t, ack.RejectReason, "unsigned")

	_, gerr := store.Entry.Get(context.Background(), "e4")
	assert.Error(t, gerr, "unsigned entry must not be stored when required")
}

// TestHandleRatingPush_SignatureEnforced: 评分签名校验：合法→接受；伪造→拒绝；无签名默认接受/required 拒绝。
func TestHandleRatingPush_SignatureEnforced(t *testing.T) {
	// 合法签名 → 接受
	{
		se, store := newPushTestEngine(t, false)
		pub, priv, _ := ed25519.GenerateKey(rand.Reader)
		pubB64 := base64.StdEncoding.EncodeToString(pub)
		sig, _ := crypto.SignRating(priv, "entry-x", pubB64, 4.5)
		rating := &model.Rating{
			ID: "r1", EntryId: "entry-x", RaterPubkey: pubB64, Score: 4.5,
			Signature: sig,
		}
		ratingJSON, _ := rating.ToJSON()
		ack, err := se.HandleRatingPush(context.Background(), &protocolpkg.RatingPush{
			Rating: ratingJSON, RaterSignature: sig,
		})
		require.NoError(t, err)
		assert.True(t, ack.Accepted, "valid rating signature must be accepted")
		_, rerr := store.Rating.Get(context.Background(), "r1")
		assert.NoError(t, rerr)
	}

	// 伪造签名（改 score）→ 拒绝
	{
		se, store := newPushTestEngine(t, false)
		pub, priv, _ := ed25519.GenerateKey(rand.Reader)
		pubB64 := base64.StdEncoding.EncodeToString(pub)
		sig, _ := crypto.SignRating(priv, "entry-x", pubB64, 4.5) // 签的是 4.5
		rating := &model.Rating{
			ID: "r2", EntryId: "entry-x", RaterPubkey: pubB64, Score: 1.0, // 声明 1.0
			Signature: sig,
		}
		ratingJSON, _ := rating.ToJSON()
		ack, err := se.HandleRatingPush(context.Background(), &protocolpkg.RatingPush{
			Rating: ratingJSON, RaterSignature: sig,
		})
		require.NoError(t, err)
		assert.False(t, ack.Accepted, "forged rating signature must be rejected")
		_, rerr := store.Rating.Get(context.Background(), "r2")
		assert.Error(t, rerr, "forged rating must not be stored")
	}

	// 无签名 + required → 拒绝
	{
		se, store := newPushTestEngine(t, true)
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		pubB64 := base64.StdEncoding.EncodeToString(pub)
		rating := &model.Rating{ID: "r3", EntryId: "entry-x", RaterPubkey: pubB64, Score: 4.5}
		ratingJSON, _ := rating.ToJSON()
		ack, err := se.HandleRatingPush(context.Background(), &protocolpkg.RatingPush{Rating: ratingJSON})
		require.NoError(t, err)
		assert.False(t, ack.Accepted, "unsigned rating must be rejected when required")
		_, rerr := store.Rating.Get(context.Background(), "r3")
		assert.Error(t, rerr)
	}
}
