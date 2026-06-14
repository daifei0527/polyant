# Polyant Phase 1A — Core Security & Data Integrity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close 7 verified security/correctness defects in Polyant v2.2.0 (verification-code leak, email-uniqueness gap, fragmented content-hash contract, invalid CORS default, auth-middleware goroutine leak, broken pactl hex export, nanosecond IDs) so the foundation is trustworthy before Phase 1B/2/3 build on it.

**Architecture:** All fixes are localized and independently shippable. Each is a TDD cycle (failing test → minimal fix → green → commit). Two cross-cutting threads: (1) the canonical content-hash contract becomes `SHA256(title + "\n" + content + "\n" + category)` in ONE place (`model.KnowledgeEntry.ComputeContentHash`), eliminating 4 divergent copies; (2) the auth middleware surfaces an `io.Closer` so the router can stop its cleanup goroutine on graceful shutdown. No new external dependencies. No storage-engine or protocol-format changes.

**Tech Stack:** Go 1.x, standard library `net/http`, Ed25519 auth, BadgerDB/Pebble KV, testify (`github.com/stretchr/testify`), cobra (pactl CLI).

**Spec source:** `docs/superpowers/specs/2026-06-13-polyant-full-sweep-design.md` §4 (P1.1, P1.2, P1.7, P1.8, P1.9, P1.10, P1.11). Root causes were re-verified against live code before this plan was written; notes inline where the spec's prose differed from code truth.

**Scope (this plan):** P1.1, P1.2, P1.7, P1.8, P1.9, P1.10, P1.11.
**Deferred to Plan 1B** (network/permissions cluster, benefits from P3.1 mocknet): P1.3 (RBAC), P1.4 (mirror dial), P1.5 (EntryPusher wiring), P1.6 (node/sync trigger).

---

## File Structure

| File | Responsibility | Touched by task |
|------|----------------|-----------------|
| `internal/storage/model/models.go` | Canonical `ComputeContentHash` (new contract); `generateID()` → UUID v4 | P1.7, P1.11 |
| `internal/api/handler/entry_handler.go` | Delete local `computeContentHash`, use model method | P1.7 |
| `internal/api/handler/batch_handler.go` | Delete local `computeContentHash`, use model method | P1.7 |
| `internal/storage/memory.go` | Delete local `computeContentHash`, use model method | P1.7 |
| `internal/api/handler/user_handler.go` | Verify-code dev gate; email-uniqueness dedup | P1.1, P1.2 |
| `pkg/config/config.go` | `DevConfig`; CORS origin/credential fields | P1.1, P1.8 |
| `internal/api/router/router.go` | Wire dev flag + CORS config; `Router.Close()` | P1.1, P1.8, P1.9 |
| `internal/api/middleware/cors.go` | Valid default; wildcard+credentials guard | P1.8 |
| `internal/api/middleware/auth.go` | (no change to Close itself; consumed by Router.Close) | P1.9 |
| `cmd/seed/main.go`, `cmd/user/main.go` | Store router, call `Close()` on shutdown; pass CORS config | P1.8, P1.9 |
| `cmd/pactl/key.go` | base64→hex public-key export | P1.10 |

**Verification gate (run after every task):** `go build ./... && go vet ./... && go test ./...`

---

## Task 1: P1.10 — Fix pactl public-key hex export (warmup, isolated)

**Root cause (verified):** `cmd/pactl/key.go` calls `hex.DecodeString(pubKey)` on `pubKey`, but `client.GetPublicKey()` returns a **base64** string. Hex-decoding a base64 string fails, so `key show --format hex` and `key export --format hex` produce wrong/empty output. Three sites: `key.go:80` (show hex), `key.go:90` (show default Hex line), `key.go:125` (export hex).

**Files:**
- Modify: `cmd/pactl/key.go`
- Create: `cmd/pactl/key_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/pactl/key_test.go`:

```go
package main

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubKeyToHex_FromBase64(t *testing.T) {
	// 32 raw bytes (Ed25519 public key length), base64-encoded as the client stores it
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	b64 := base64.StdEncoding.EncodeToString(raw)

	got, err := pubKeyToHex(b64)
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(raw), got)
}

func TestPubKeyToHex_InvalidBase64(t *testing.T) {
	_, err := pubKeyToHex("!!!not-base64!!!")
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/pactl/ -run TestPubKeyToHex -v`
Expected: FAIL / build error — `pubKeyToHex` undefined.

- [ ] **Step 3: Add the helper and fix the three call sites**

In `cmd/pactl/key.go`, add the import `"encoding/base64"` to the import block (alongside the existing `"encoding/hex"`), then add this helper near the bottom of the file (above `func init()`):

```go
// pubKeyToHex converts a base64-encoded Ed25519 public key to lowercase hex.
// The client stores/returns the public key as base64, so we must base64-decode
// before hex-encoding (the previous code hex-decoded a base64 string, which fails).
func pubKeyToHex(pubKeyBase64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	return hex.EncodeToString(b), nil
}
```

Replace the **show hex** branch (currently around `key.go:78-84`):

```go
		case "hex":
			h, err := pubKeyToHex(pubKey)
			if err != nil {
				return fmt.Errorf("解码公钥失败: %w", err)
			}
			fmt.Println(h)
```

Replace the **default show** Hex line (currently `decoded, err := hex.DecodeString(pubKey)` in the `default:` branch):

```go
		default:
			fmt.Println("公钥信息:")
			fmt.Printf("  Base64: %s\n", pubKey)
			if h, err := pubKeyToHex(pubKey); err == nil {
				fmt.Printf("  Hex:     %s\n", h)
			}
			fmt.Printf("  长度:    %d 字节\n", 32) // Ed25519 公钥固定 32 字节
```

Replace the **export hex** branch (currently around `key.go:124-129`):

```go
		case "hex":
			h, err := pubKeyToHex(pubKey)
			if err != nil {
				return fmt.Errorf("解码公钥失败: %w", err)
			}
			content = h
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/pactl/ -run TestPubKeyToHex -v`
Expected: PASS (both subtests).

- [ ] **Step 5: Verify build/vet and commit**

Run: `go build ./... && go vet ./... && go test ./cmd/pactl/...`
Expected: all PASS.

```bash
git add cmd/pactl/key.go cmd/pactl/key_test.go
git commit -m "fix(pactl): base64-decode public key before hex export

pubKeyToHex replaces hex.DecodeString(pubKey) which operated on a
base64 string and silently failed. key show/export --format hex now
produces correct output."
```

---

## Task 2: P1.11 — UUID v4 for model-generated IDs

**Root cause (verified):** `internal/storage/model/models.go:299` `generateID()` returns `fmt.Sprintf("%d", time.Now().UnixNano())` — a nanosecond timestamp, not UUID v4. P0 requires `entry-id-uuid-v4`; nanosecond IDs collide across goroutines/nodes and are guessable. Callers: `models.go:73` (entry), `model/audit.go:69` (`"audit_"+generateID()`), `model/election.go:48,158` (election/candidate). Note: API-created entries already use the handler's `generateUUID()` (helpers.go) — this fix closes the gap for `NewKnowledgeEntry`, audit, and election paths.

A correct UUID v4 already exists at `pkg/crypto/hash.go:48` (`crypto.GenerateUUID`, crypto/rand with timestamp fallback). Reuse it.

**Files:**
- Modify: `internal/storage/model/models.go`
- Modify: `internal/storage/model/models_test.go`

- [ ] **Step 1: Add a failing test asserting UUID v4 format**

In `internal/storage/model/models_test.go`, replace the existing `TestGenerateID` (lines ~373-384) with:

```go
func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Fatal("Generated ID should not be empty")
	}
	if id1 == id2 {
		t.Fatal("Generated IDs should be unique")
	}
}

// TestGenerateID_IsUUIDv4 asserts generateID now returns UUID v4, not a
// nanosecond timestamp (P0: entry-id-uuid-v4).
func TestGenerateID_IsUUIDv4(t *testing.T) {
	id := generateID()

	// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx (36 chars)
	if len(id) != 36 {
		t.Fatalf("expected UUID length 36, got %d (%q)", len(id), id)
	}
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Fatalf("not UUID format (bad hyphens): %q", id)
	}
	if id[14] != '4' {
		t.Fatalf("not UUID v4 (version char = %q): %q", string(id[14]), id)
	}
	switch id[19] {
	case '8', '9', 'a', 'b':
	default:
		t.Fatalf("invalid UUID variant char %q: %q", string(id[19]), id)
	}

	// Must not be a bare integer (the old nanosecond form).
	if _, err := strconv.ParseInt(id, 10, 64); err == nil {
		t.Fatalf("generateID still returns a bare integer: %q", id)
	}
}
```

Add `"strconv"` to the test file's imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/model/ -run TestGenerateID_IsUUIDv4 -v`
Expected: FAIL — old `generateID` returns a bare integer, length != 36.

- [ ] **Step 3: Implement — delegate to crypto.GenerateUUID**

In `internal/storage/model/models.go`, add the import:

```go
	"github.com/daifei0527/polyant/pkg/crypto"
```

Replace `generateID` (lines ~298-301):

```go
// generateID 生成一个 UUID v4 格式的唯一标识符。
// 使用 crypto/rand 保证跨协程/跨节点的唯一性与不可预测性（满足 P0: entry-id-uuid-v4）。
func generateID() string {
	return crypto.GenerateUUID()
}
```

Leave the existing `import "time"` — it is still used by `NewKnowledgeEntry` (`time.Now().Unix()`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/model/ -v`
Expected: PASS, including `TestGenerateID` and `TestGenerateID_IsUUIDv4`.

- [ ] **Step 5: Verify full suite and commit**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS (no test asserts IDs are pure digits — verified by grep).

```bash
git add internal/storage/model/models.go internal/storage/model/models_test.go
git commit -m "fix(model): generate UUID v4 IDs instead of nanosecond timestamps

generateID() now returns crypto.GenerateUUID(). Closes the P0
entry-id-uuid-v4 gap for NewKnowledgeEntry, audit, and election
ID generation (API-created entries already used UUID via the handler)."
```

---

## Task 3: P1.7 — Unify the content-hash contract to `SHA256(title\ncontent\ncategory)`

**Root cause (verified):** There are **four** divergent hash implementations:
1. `model.KnowledgeEntry.ComputeContentHash` (models.go:92) → `SHA256(title:content:version:jsonData)`
2. `handler.computeContentHash` (entry_handler.go:449) → `SHA256(title‖content‖category)` (no separators)
3. `handler.computeContentHash` (batch_handler.go) → same as #2
4. `storage.computeContentHash` (memory.go:615) → same as #2

None matches the documented contract (CLAUDE.md: *"Signature content: SHA256(title + \"\\n\" + content + \"\\n\" + category)"*). The content hash must equal the signed content so sync verification is consistent.

**Fix:** Make `model.KnowledgeEntry.ComputeContentHash` the single canonical source using the documented contract, then delete the three local copies and route all callers through the method.

Known vector (used in the test): `SHA256("Title\nContent\ntest-category")` = `49b12e97e9d0c4e09ab363e031293e37f90aa78fe6bb434c57bdab2b6eaba543`.

**Files:**
- Modify: `internal/storage/model/models.go`
- Modify: `internal/api/handler/entry_handler.go`
- Modify: `internal/api/handler/batch_handler.go`
- Modify: `internal/storage/memory.go`
- Modify: `internal/storage/model/models_test.go`

- [ ] **Step 1: Add a failing test pinning the canonical contract**

In `internal/storage/model/models_test.go`, add:

```go
// TestKnowledgeEntry_ComputeContentHash_Contract pins the documented hash
// contract: SHA256(title + "\n" + content + "\n" + category). This must match
// the entry content-signature scheme so sync/push verification is consistent.
func TestKnowledgeEntry_ComputeContentHash_Contract(t *testing.T) {
	e := &KnowledgeEntry{Title: "Title", Content: "Content", Category: "test-category"}
	want := "49b12e97e9d0c4e09ab363e031293e37f90aa78fe6bb434c57bdab2b6eaba543"
	if got := e.ComputeContentHash(); got != want {
		t.Errorf("ComputeContentHash = %q, want %q (contract: SHA256(title\\ncontent\\ncategory))", got, want)
	}

	// Separators matter: "a"+"bc" must differ from "ab"+"c".
	a := &KnowledgeEntry{Title: "a", Content: "bc", Category: "z"}
	b := &KnowledgeEntry{Title: "ab", Content: "c", Category: "z"}
	if a.ComputeContentHash() == b.ComputeContentHash() {
		t.Error("hash must distinguish a|bc from ab|c (separator-sensitive)")
	}

	// Version/JSONData must NOT affect the content hash.
	x := &KnowledgeEntry{Title: "T", Content: "C", Category: "cat", Version: 1, JSONData: []map[string]interface{}{{"k": "v"}}}
	y := &KnowledgeEntry{Title: "T", Content: "C", Category: "cat", Version: 99}
	if x.ComputeContentHash() != y.ComputeContentHash() {
		t.Error("content hash must be independent of Version/JSONData")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/model/ -run TestKnowledgeEntry_ComputeContentHash_Contract -v`
Expected: FAIL — current hash includes version/jsonData and uses `:` separators.

- [ ] **Step 3: Change the canonical method to the documented contract**

In `internal/storage/model/models.go`, replace `ComputeContentHash` (lines ~91-96):

```go
// ComputeContentHash 计算条目内容的 SHA256 哈希值。
// 契约：SHA256(title + "\n" + content + "\n" + category)
// 与条目内容签名方案一致，用于同步/push 的完整性校验。
// 注意：Version 与 JSONData 不参与内容哈希（仅 title/content/category 决定）。
func (e *KnowledgeEntry) ComputeContentHash() string {
	data := fmt.Sprintf("%s\n%s\n%s", e.Title, e.Content, e.Category)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
```

`fmt` is already imported.

- [ ] **Step 4: Run model tests to verify the contract test passes**

Run: `go test ./internal/storage/model/ -run TestKnowledgeEntry_ComputeContentHash -v`
Expected: PASS (both the existing determinism test and the new contract test — the existing test does not assert a literal value, only determinism/difference, so it stays green).

- [ ] **Step 5: Route handler callers through the method (CreateEntryHandler)**

In `internal/api/handler/entry_handler.go`, `CreateEntryHandler` currently computes `contentHash` at line 215 before building the entry. Remove that line and set the hash on the entry instead.

Delete the `// 计算内容哈希` block:
```go
	// 计算内容哈希
	contentHash := computeContentHash(req.Title, req.Content, req.Category)
```

In the entry literal, remove the `ContentHash: contentHash,` line, then immediately after the `if entry.License == "" { entry.License = "CC-BY-SA-4.0" }` block, add:

```go
	// 计算内容哈希（统一契约：SHA256(title\ncontent\ncategory)）
	entry.ContentHash = entry.ComputeContentHash()
```

In `UpdateEntryHandler`, replace the line `existing.ContentHash = computeContentHash(existing.Title, existing.Content, existing.Category)` with:

```go
	existing.ContentHash = existing.ComputeContentHash()
```

- [ ] **Step 6: Route batch_handler callers through the method**

In `internal/api/handler/batch_handler.go`, replace each occurrence of:
```go
contentHash := computeContentHash(entry.Title, entry.Content, entry.Category)
```
and
```go
existing.ContentHash = computeContentHash(existing.Title, existing.Content, existing.Category)
```
with the equivalent using the model method. For the `contentHash :=` form, replace the later assignment `ContentHash: contentHash,` by deleting the local and adding `entry.ContentHash = entry.ComputeContentHash()` after the entry struct is built (mirror exactly what Task 3 Step 5 does for entry_handler). For the `existing.ContentHash =` form, use `existing.ComputeContentHash()`.

Run: `grep -n "computeContentHash" internal/api/handler/batch_handler.go`
Expected: **no output** (all references removed).

- [ ] **Step 7: Route memory store caller through the method and delete its local copy**

In `internal/storage/memory.go`, find the local `computeContentHash` (line ~615) and its caller(s). Replace each call with the model method on the relevant entry (e.g. `entry.ComputeContentHash()`), then delete the local `func computeContentHash(...)` entirely.

Run: `grep -rn "func computeContentHash" internal/`
Expected: **no output** (the function now exists only as `model.KnowledgeEntry.ComputeContentHash`).

- [ ] **Step 8: Delete the handler-local computeContentHash**

In `internal/api/handler/entry_handler.go`, delete the whole `func computeContentHash(title, content, category string) string { ... }` (lines ~447-455).

Run: `grep -rn "computeContentHash" internal/api/`
Expected: **no output**.

Check for now-unused imports after deletion. `internal/storage/memory.go` may no longer use `crypto/sha256` and `encoding/hex` — remove them if `goimports`/`go vet` flags unused imports. `internal/api/handler/entry_handler.go` keeps `crypto/sha256`/`encoding/hex` only if still used elsewhere in the file — verify with the build in Step 9 and remove if unused.

- [ ] **Step 9: Verify build/vet/test and commit**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS. (Verified: no test asserts a literal content-hash value; existing hash tests only check `== ""` and determinism.)

```bash
git add internal/storage/model/models.go internal/storage/model/models_test.go \
        internal/api/handler/entry_handler.go internal/api/handler/batch_handler.go \
        internal/storage/memory.go
git commit -m "fix(model): unify content hash to SHA256(title\\ncontent\\ncategory)

ComputeContentHash is now the single canonical source matching the
documented signature contract. Removes 3 divergent copies in
entry_handler, batch_handler, and memory store. Unblocks the P3.2
integrity guardian daemon."
```

---

## Task 4: P1.1 — Stop leaking the email verification code by default

**Root cause (verified):** `internal/api/handler/user_handler.go:193` unconditionally includes `"code": code` in the `SendVerificationCodeHandler` response, bypassing email delivery entirely. Comment self-incriminates: *"仅用于测试，生产环境应删除此字段"*.

**Fix:** Gate the `code` field behind a new `DevConfig.ReturnVerificationCode` config (default false). Only when true does the response include the code (for test/dev environments).

**Test impact (verified):** `internal/api/handler/handler_test.go:519` extracts `sendData["code"]` to drive the verify-email flow. After this change that test must enable the dev flag on the handler.

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `internal/api/handler/user_handler.go`
- Modify: `internal/api/router/router.go`
- Modify: `internal/api/handler/handler_test.go`

- [ ] **Step 1: Add the DevConfig type and field**

In `pkg/config/config.go`, add a new struct after `AdminConfig` (around line 132):

```go
// DevConfig 开发/测试专用配置。
// 这些选项在生产环境中必须保持默认（关闭）。
type DevConfig struct {
	// ReturnVerificationCode 为 true 时，发送验证码接口在响应中返回明文验证码，
	// 仅供开发/测试使用（默认 false：验证码只通过邮件下发，绝不回传）。
	ReturnVerificationCode bool `json:"return_verification_code"`
}
```

Add the field to the `Config` struct (after `Admin`):

```go
	Admin   AdminConfig  `json:"admin"`   // 管理页面配置
	Dev     DevConfig    `json:"dev"`     // 开发/测试专用（默认全关闭）
```

No `DefaultConfig()` change needed — the zero value is `false`, which is the secure default.

- [ ] **Step 2: Write the failing test**

In `internal/api/handler/user_handler_test.go` (create if absent; otherwise append), add:

```go
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

func newSendVerifyHandler(t *testing.T) (*UserHandler, *storage.Store) {
	t.Helper()
	store, err := storage.NewMemoryStore()
	require.NoError(t, err)
	vm := email.NewVerificationManager()
	return NewUserHandler(store, store.User, store.Entry, store.Rating, nil, vm), store
}

// TestSendVerificationCodeHandler_CodeNotLeakedByDefault: with the dev flag off
// (the default) the response MUST NOT contain the plaintext code.
func TestSendVerificationCodeHandler_CodeNotLeakedByDefault(t *testing.T) {
	handler, store := newSendVerifyHandler(t)
	user, _ := createTestUser(t, store, "leak-agent", model.UserLevelLv0)

	body, _ := json.Marshal(map[string]string{"email": "leak@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/send-verification", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(context.Background(), user))
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
	handler, store := newSendVerifyHandler(t)
	handler.SetDevReturnVerificationCode(true)
	user, _ := createTestUser(t, store, "dev-agent", model.UserLevelLv0)

	body, _ := json.Marshal(map[string]string{"email": "dev@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/send-verification", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(context.Background(), user))
	rec := httptest.NewRecorder()

	handler.SendVerificationCodeHandler(rec, req)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, _ := resp.Data.(map[string]interface{})
	code, ok := data["code"].(string)
	assert.True(t, ok, "code must be present in dev mode")
	assert.NotEmpty(t, code)
}
```

(`createTestUser` and `setUserInContext` already exist in the handler test package.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/api/handler/ -run TestSendVerificationCodeHandler -v`
Expected: FAIL / build error — `SetDevReturnVerificationCode` undefined; and the default-leak test fails because code is present.

- [ ] **Step 4: Add the dev field + setter + gate in the handler**

In `internal/api/handler/user_handler.go`, add a field to `UserHandler` (after `verificationMgr`):

```go
	devReturnCode bool // 仅 dev/测试：SendVerificationCodeHandler 是否在响应中回传验证码
}
```

Add the setter after `NewUserHandler`:

```go
// SetDevReturnVerificationCode 控制发送验证码接口是否在响应中返回明文验证码。
// 默认 false（安全：验证码只通过邮件下发）。仅在开发/测试环境启用。
func (h *UserHandler) SetDevReturnVerificationCode(v bool) {
	h.devReturnCode = v
}
```

In `SendVerificationCodeHandler`, replace the `writeJSON(...)` response block (lines ~187-195) with:

```go
	respData := map[string]interface{}{
		"email":      req.Email,
		"expires_in": 1800, // 30 分钟
	}
	if h.devReturnCode {
		// 仅 dev/测试：回传验证码以便测试完成验证流程。生产环境必须保持关闭。
		respData["code"] = code
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "verification code sent to your email",
		Data:    respData,
	})
```

- [ ] **Step 5: Wire the config flag through the router**

In `internal/api/router/router.go`, add a field to `Dependencies` (after `ApiKey`):

```go
	DevReturnVerificationCode bool // dev/测试：发送验证码接口是否回传验证码（默认 false）
```

After `userHandler := handler.NewUserHandler(...)` in `NewRouterWithDeps`, add:

```go
	userHandler.SetDevReturnVerificationCode(deps.DevReturnVerificationCode)
```

In `NewRouter` (the cfg-based constructor), set it from config in the `Dependencies` literal:

```go
		DevReturnVerificationCode: cfg.Dev.ReturnVerificationCode,
```

- [ ] **Step 6: Fix the existing test that relied on the leaked code**

In `internal/api/handler/handler_test.go`, find the test that calls `SendVerificationCodeHandler` and then reads `sendData["code"]` (around line 506-522). Immediately after the `handler` is created (and before `handler.SendVerificationCodeHandler(sendRec, sendReq)`), add:

```go
	handler.SetDevReturnVerificationCode(true) // test needs the code to complete verify
```

(If the handler in that test is constructed via `newTestUserHandler`, add the line right after that call.)

- [ ] **Step 7: Run all handler tests to verify they pass**

Run: `go test ./internal/api/handler/ -v`
Expected: PASS, including the previously-leak-dependent test and the two new tests.

- [ ] **Step 8: Verify build/vet/test and commit**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS.

```bash
git add pkg/config/config.go internal/api/handler/user_handler.go \
        internal/api/router/router.go internal/api/handler/handler_test.go \
        internal/api/handler/user_handler_test.go
git commit -m "fix(auth): do not leak verification code in API response by default

SendVerificationCodeHandler now omits the plaintext code unless the
new DevConfig.ReturnVerificationCode flag is set (default false).
The code is delivered only via email in production. Tests opt in."
```

---

## Task 5: P1.2 — Enforce email uniqueness on send-verification (and tighten verify)

**Root cause (verified):** `SendVerificationCodeHandler` (user_handler.go:161-165) has an **empty** `if h.emailService != nil { }` block where email-uniqueness dedup was intended — it never checks. (`GetByEmail` already exists in both the `storage.UserStore` interface and the KV store; the spec's "add GetByEmail" was already done — the real bug is the empty block.) Separately, `VerifyEmailHandler` (line 249) rejects an email if *any* user holds it, even the same user re-verifying their own address.

**Fix:** In `SendVerificationCodeHandler`, reject the email if another user already owns it. In `VerifyEmailHandler`, only reject when a *different* user owns it (so self-reverification works). `ErrEmailAlreadyUsed` (code 805, 409) already exists.

**Files:**
- Modify: `internal/api/handler/user_handler.go`
- Modify: `internal/api/handler/verify_email_test.go`

- [ ] **Step 1: Write failing tests**

In `internal/api/handler/verify_email_test.go`, add (imports `context`, `model` already present):

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/handler/ -run TestSendVerificationCodeHandler_Email -v`
Expected: FAIL — `EmailTakenByOther` gets 200 (no dedup); `OwnEmailAllowed` passes already.

- [ ] **Step 3: Implement the dedup in SendVerificationCodeHandler**

In `internal/api/handler/user_handler.go`, replace the empty dedup block (lines ~161-165):

```go
	// 检查邮箱是否已被其他用户使用（防止抢占他人邮箱）
	if existing, _ := h.userStore.GetByEmail(r.Context(), req.Email); existing != nil && existing.PublicKey != user.PublicKey {
		writeError(w, awerrors.ErrEmailAlreadyUsed)
		return
	}
```

- [ ] **Step 4: Tighten VerifyEmailHandler to allow self-reverification**

In `VerifyEmailHandler`, replace the check at line ~248-252:

```go
	// 检查邮箱是否已被"其他"用户使用（本人重新验证自己的邮箱应放行）
	if existingUser, _ := h.userStore.GetByEmail(r.Context(), req.Email); existingUser != nil && existingUser.PublicKey != user.PublicKey {
		writeError(w, awerrors.ErrEmailAlreadyUsed)
		return
	}
```

(The existing `TestVerifyEmailHandler_EmailAlreadyUsed` uses two different users, so it still expects and gets 409 — verified.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/handler/ -v`
Expected: PASS, including the new dedup tests and the existing `TestVerifyEmailHandler_EmailAlreadyUsed`.

- [ ] **Step 6: Verify build/vet/test and commit**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS.

```bash
git add internal/api/handler/user_handler.go internal/api/handler/verify_email_test.go
git commit -m "fix(user): enforce email uniqueness on send-verification

SendVerificationCodeHandler now rejects an email already owned by
another user (was an empty stub). VerifyEmailHandler only rejects when
a different user owns the email, so self-reverification works."
```

---

## Task 6: P1.8 — Make the default CORS config spec-valid and configurable

**Root cause (verified):** `internal/api/middleware/cors.go` `DefaultCORSConfig()` returns `AllowedOrigins: ["*"]` together with `AllowCredentials: true`. The CORS spec forbids that combination (browsers reject it). The router (`router.go:146`) hardcodes this default.

**Fix:** (1) Default `AllowCredentials` to false. (2) Enforce the wildcard/credentials rule inside `NewCORSMiddleware` (defense in depth). (3) Make origins + credentials configurable via `APIConfig`.

**Files:**
- Modify: `internal/api/middleware/cors.go`
- Modify: `internal/api/middleware/cors_test.go`
- Modify: `pkg/config/config.go`
- Modify: `internal/api/router/router.go`
- Modify: `cmd/seed/main.go`, `cmd/user/main.go`

- [ ] **Step 1: Write failing tests**

In `internal/api/middleware/cors_test.go`, add (adjust imports to match the file's existing style — it uses `net/http/httptest` and `testing`):

```go
// TestDefaultCORSConfig_SpecValid: the default must not combine "*" with
// AllowCredentials=true (browsers reject that combination).
func TestDefaultCORSConfig_SpecValid(t *testing.T) {
	c := DefaultCORSConfig()
	if c.AllowCredentials {
		for _, o := range c.AllowedOrigins {
			if o == "*" {
				t.Fatal("default CORS combines wildcard origin with credentials, which the CORS spec forbids")
			}
		}
	}
}

// TestCORSMiddleware_WildcardWithCredentialsDowngraded: even if a caller
// (or config) asks for "*" + credentials, the middleware must drop credentials.
func TestCORSMiddleware_WildcardWithCredentialsDowngraded(t *testing.T) {
	mw := NewCORSMiddleware(CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("wildcard origin must not advertise credentials, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected wildcard origin, got %q", got)
	}
}
```

Add `"net/http"` and `"net/http/httptest"` to imports if not already present.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/middleware/ -run CORS -v`
Expected: FAIL — default has credentials=true with "*"; downgrade test sees the credentials header.

- [ ] **Step 3: Fix the default and add the guard**

In `internal/api/middleware/cors.go`, change `DefaultCORSConfig()` — set `AllowCredentials: false`:

```go
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Polyant-PublicKey", "X-Polyant-Timestamp", "X-Polyant-Signature"},
		ExposedHeaders:   []string{"Content-Length", "X-Request-Id"},
		AllowCredentials: false, // "*" + credentials is invalid per CORS spec
		MaxAge:           86400,
	}
}
```

In `NewCORSMiddleware`, after the `if len(config.AllowedOrigins) == 0` block, add the guard:

```go
	// CORS 规范不允许通配符 origin 与 credentials 同时启用——浏览器会拒绝。
	// 作为防线，即便配置错误也强制降级。
	if config.AllowCredentials {
		for _, o := range config.AllowedOrigins {
			if o == "*" {
				config.AllowCredentials = false
				break
			}
		}
	}
```

- [ ] **Step 4: Run middleware tests to verify they pass**

Run: `go test ./internal/api/middleware/ -run CORS -v`
Expected: PASS.

- [ ] **Step 5: Add configurable CORS fields**

In `pkg/config/config.go`, extend `APIConfig`:

```go
type APIConfig struct {
	Enabled              bool     `json:"enabled"`                // 是否启用 API 服务
	CORS                 bool     `json:"cors"`                   // 是否启用 CORS
	CORSAllowOrigins     []string `json:"cors_allow_origins"`     // 允许的源，空则等价于 ["*"]
	CORSAllowCredentials bool     `json:"cors_allow_credentials"` // 是否允许携带凭证（与 "*" 互斥）
}
```

- [ ] **Step 6: Add a config→CORSConfig helper in the router and use it**

In `internal/api/router/router.go`, add (this bridges `config.Config` ↔ `middleware.CORSConfig`):

```go
// CORSConfigFromConfig 根据应用配置构建 CORS 中间件配置。
// 未配置 origins 时退化为安全的通配符默认值（credentials=false）。
func CORSConfigFromConfig(cfg *config.Config) middleware.CORSConfig {
	c := middleware.DefaultCORSConfig()
	if cfg != nil && len(cfg.API.CORSAllowOrigins) > 0 {
		c.AllowedOrigins = cfg.API.CORSAllowOrigins
	}
	if cfg != nil && cfg.API.CORSAllowCredentials {
		c.AllowCredentials = true
	}
	return c
}
```

Add `DevReturnVerificationCode bool` wiring is already done in Task 4. Now add a `CORSConfig` field to `Dependencies` (after `DevReturnVerificationCode`):

```go
	CORSConfig middleware.CORSConfig // 可选；为零值时使用 DefaultCORSConfig
```

In `NewRouterWithDeps`, replace the hardcoded CORS line (`corsMW := middleware.NewCORSMiddleware(middleware.DefaultCORSConfig())`) with:

```go
	corsConf := deps.CORSConfig
	if len(corsConf.AllowedOrigins) == 0 && len(corsConf.AllowedMethods) == 0 {
		corsConf = middleware.DefaultCORSConfig()
	}
	corsMW := middleware.NewCORSMiddleware(corsConf)
```

In `NewRouter` (cfg-based), set it in the `Dependencies` literal:

```go
		CORSConfig:                CORSConfigFromConfig(cfg),
```

- [ ] **Step 7: Pass configured CORS into both nodes' routers**

In `cmd/seed/main.go` and `cmd/user/main.go`, in the `router.NewRouterWithDeps(&router.Dependencies{ ... })` literal, add:

```go
		CORSConfig: router.CORSConfigFromConfig(app.config),
```

- [ ] **Step 8: Verify build/vet/test and commit**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS. (router_test.go uses `NewRouter`/`NewRouterWithDeps` as values; adding fields to `Dependencies` and returning the same types does not break it.)

```bash
git add internal/api/middleware/cors.go internal/api/middleware/cors_test.go \
        pkg/config/config.go internal/api/router/router.go \
        cmd/seed/main.go cmd/user/main.go
git commit -m "fix(cors): valid default CORS (no wildcard+credentials) + config

DefaultCORSConfig no longer combines '*' with AllowCredentials (spec-
invalid). NewCORSMiddleware downgrades the invalid combo as defense in
depth. Origins/credentials are now configurable via APIConfig."
```

---

## Task 7: P1.9 — Stop the auth-middleware cleanup goroutine leak on shutdown

**Root cause (verified):** `internal/api/middleware/auth.go:66` launches `go m.cleanupSeenRequests()`; `Close()` (line 71) stops it, but `Close()` is **only ever called from tests**. In production the goroutine + `seenRequests` map live for the entire server lifetime with no shutdown hook. The router creates `authMW` locally (`router.go:143`) and never exposes it.

**Fix:** Introduce a `Router` type that implements `http.Handler` (via `ServeHTTP`) **and** exposes `Close()`, which stops the auth middleware. Both nodes store the router and call `Close()` in their graceful-shutdown path. `router_test.go` is unaffected because `*Router` satisfies `http.Handler`.

**Files:**
- Modify: `internal/api/router/router.go`
- Modify: `cmd/seed/main.go`, `cmd/user/main.go`
- Create: `internal/api/router/router_close_test.go`

- [ ] **Step 1: Write a failing test**

Create `internal/api/router/router_close_test.go`:

```go
package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
)

// TestRouter_CloseExists asserts the router exposes a Close hook so callers
// can release the auth middleware's background goroutine on shutdown.
func TestRouter_CloseExists(t *testing.T) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	r, err := NewRouterWithDeps(&Dependencies{
		Store:      store,
		EntryStore: store.Entry,
		UserStore:  store.User,
		NodeID:     "test",
		NodeType:   "local",
	})
	if err != nil {
		t.Fatalf("NewRouterWithDeps: %v", err)
	}
	if r == nil {
		t.Fatal("nil router")
	}

	// Router must be usable as an http.Handler.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Must be closeable without panic.
	r.Close()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/router/ -run TestRouter_CloseExists -v`
Expected: FAIL / build error — `NewRouterWithDeps` returns `http.Handler`, not a type with `.ServeHTTP`/`.Close()` directly addressable; `.Close()` undefined.

- [ ] **Step 3: Introduce the Router type**

In `internal/api/router/router.go`, add the `Router` type and change the two constructors' return types.

Add the type (near the top, after the `Dependencies` definition):

```go
// Router 包装已注册路由与中间件链，并暴露 Close 以便优雅停机时
// 释放后台资源（如认证中间件的重放保护清理 goroutine）。
type Router struct {
	handler http.Handler
	authMW  *middleware.AuthMiddleware
}

// ServeHTTP 让 *Router 满足 http.Handler（可直接作为 http.Server.Handler）。
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.handler.ServeHTTP(w, req)
}

// Close 释放路由持有的后台资源。应在 HTTP 服务器 Shutdown 之后调用。
func (r *Router) Close() {
	if r.authMW != nil {
		r.authMW.Close()
	}
}
```

Change `NewRouter` signature and tail:

```go
func NewRouter(store *storage.Store, cfg *config.Config) (*Router, error) {
	return NewRouterWithDeps(&Dependencies{
		// ... unchanged literal contents ...
	})
}
```

In `NewRouterWithDeps`, keep building the `httpHandler` chain as today, but capture `authMW` and return `*Router`. Change the final lines from `return httpHandler, nil` to:

```go
	return &Router{
		handler: httpHandler,
		authMW:  authMW,
	}, nil
```

- [ ] **Step 4: Run router tests to verify they pass**

Run: `go test ./internal/api/router/ -v`
Expected: PASS. (`router_test.go` assigns the result to `router` and uses it as an `http.Handler`; since `*Router` implements `ServeHTTP`, those usages — `router.ServeHTTP(...)`, passing as a handler, `router == nil` — all continue to compile and work.)

- [ ] **Step 5: Store the router and close it on shutdown (seed node)**

In `cmd/seed/main.go`:
- Add a field to `SeedApp`:
  ```go
  	apiRouter *router.Router
  ```
- In `Start()`, change `apiHandler, err := router.NewRouterWithDeps(...)` to also keep the reference:
  ```go
  	apiRouter, err := router.NewRouterWithDeps(&router.Dependencies{
  		// ... unchanged ...
  	})
  	if err != nil {
  		return fmt.Errorf("failed to create API router: %w", err)
  	}
  ```
  and set the http server handler to `app.apiRouter`:
  ```go
  	app.httpServer = &http.Server{
  		Addr:    httpAddr,
  		Handler: app.apiRouter,
  		// ... timeouts unchanged ...
  	}
  ```
- In `Stop()`, after the HTTP server shutdown block (and before stopping other components), add:
  ```go
  	if app.apiRouter != nil {
  		app.apiRouter.Close()
  	}
  ```

- [ ] **Step 6: Store the router and close it on shutdown (user node)**

In `cmd/user/main.go`, make the same three changes as Step 5 for the user-node app struct and its `Start`/`Stop` (the user node uses the same `router.NewRouterWithDeps` call at line ~407 and shuts down at ~478). Add the `apiRouter *router.Router` field, assign it, set `Handler: app.apiRouter`, and call `app.apiRouter.Close()` in the shutdown sequence after `httpServer.Shutdown`.

- [ ] **Step 7: Verify build/vet/test and commit**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS.

```bash
git add internal/api/router/router.go internal/api/router/router_close_test.go \
        cmd/seed/main.go cmd/user/main.go
git commit -m "fix(shutdown): close auth middleware cleanup goroutine on stop

Introduce router.Router (http.Handler + Close). Both nodes now call
Router.Close() during graceful shutdown, stopping the replay-protection
cleanup goroutine instead of leaking it for the process lifetime."
```

---

## Phase 1A Verification Gate

After Task 7, run the full gate and confirm all three are green:

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...` (all packages, including the previously-broken `test/` integration suite is NOT in scope here — that is fixed in Phase 3 per the spec; only the unit suites covered by Tasks 1–7 must be green)

- [ ] **Manual smoke (optional but recommended):**
  ```bash
  ./bin/seed -config configs/seed.json   # in one terminal (requires TLS flags per cmd/seed/main.go)
  ```
  Send a `POST /api/v1/user/send-verification` (signed) and confirm the response has **no** `code` field. Confirm the server starts and, on Ctrl-C, logs "Polyant Seed Node stopped" without hanging.

---

## Self-Review (completed by plan author)

**1. Spec coverage (Phase 1A subset):** P1.1 → Task 4 ✓; P1.2 → Task 5 ✓; P1.7 → Task 3 ✓; P1.8 → Task 6 ✓; P1.9 → Task 7 ✓; P1.10 → Task 1 ✓; P1.11 → Task 2 ✓. P1.3/P1.4/P1.5/P1.6 are explicitly deferred to Plan 1B (network/permissions cluster, verified by P3.1 mocknet) — documented in the plan header and roadmap.

**2. Placeholder scan:** No "TBD"/"TODO"/"handle edge cases" steps. Every code step contains the actual code or an exact find-and-replace instruction with a grep verification of the expected (empty) result. The only "run the suite and fix" instruction (Task 3 Step 8 unused-import cleanup) is bounded by `go vet` failing with a specific, mechanical remedy.

**3. Type/method consistency:** `SetDevReturnVerificationCode` (Task 4) is defined once and called from the test, router, and the fixed existing test. `pubKeyToHex` (Task 1) defined once, used at all three export sites. `Router.Close()`/`Router.ServeHTTP` (Task 7) defined once and consumed by both nodes. `CORSConfigFromConfig` (Task 6) defined once and used by both nodes + `NewRouter`. `crypto.GenerateUUID` (Task 2) is the pre-existing function reused, not redefined. The content-hash is canonicalized to exactly one function (`model.KnowledgeEntry.ComputeContentHash`); all four prior copies are removed.

**Cross-task ordering rationale:** Task 1 (pactl) and Task 2 (UUID) are isolated warmups. Task 3 (hash) is done before the guardian daemon (Phase 3) needs it and touches the same model file as Task 2, so it follows Task 2. Tasks 4–5 (user/auth) cluster on `user_handler.go`. Task 6 (CORS) and Task 7 (Router/Close) both modify `router.go` and both node mains; ordering CORS before Router keeps the `Dependencies` struct growth incremental and each commit coherent.
