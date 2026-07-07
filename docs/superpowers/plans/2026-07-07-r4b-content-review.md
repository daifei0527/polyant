# R4b Content Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a trust-based content review workflow: Lv1/Lv2 entries enter a `review` queue, Lv3+ publish directly; admins approve/reject/takedown via the admin SPA.

**Architecture:** 5 tasks — (1) add reviewer fields to `KnowledgeEntry`; (2) trust-based create + index guard; (3) review service with state-machine transitions; (4) review handler + routes under `adminAuthMW` + audit action types; (5) admin SPA queue view + API client + stats wiring. Review endpoints use session-token auth (admin SPA can't sign Ed25519). P2P unchanged (status carries as-is; review actions update + push).

**Tech Stack:** Go 1.25.x / net/http / Vue 3 + Vite + element-plus + pinia + axios / standard tests + `-race`.

## Global Constraints

- **Go 1.25.x**; module `github.com/daifei0527/polyant`. JSON tags on `KnowledgeEntry` are **camelCase** (e.g. `json:"createdAt"`) — match this.
- **Entry status constants** (`internal/storage/model/models.go:28-32`): `EntryStatusDraft="draft"`, `EntryStatusPublished="published"`, `EntryStatusArchived="archived"`, `EntryStatusDeleted="deleted"`, `EntryStatusReview="review"`.
- **User levels** (`models.go:17-23`): `UserLevelLv1`=普通, `UserLevelLv3`=贡献者. Trust threshold = Lv3: `<Lv3 → review`, `≥Lv3 → published`.
- **`SearchEngine` interface** (`internal/storage/index/types.go:29`) already has `IndexEntry`, `UpdateIndex`, **`DeleteIndex(entryID string) error`**, `Search`, `Close`. Accessible via `store.Search`.
- **`EntryPusher` interface** (`internal/api/handler/entry_handler.go:51`): `PushEntry(entry *model.KnowledgeEntry, signature []byte) error`.
- **EntryStore** (`internal/storage/store.go:17`): `Create/Get/Update/Delete/List/Count`. `Update(ctx, entry) (*model.KnowledgeEntry, error)`. `EntryFilter` has `Status string`.
- **Auth context key**: reviewer pubkey via `mw.PublicKeyKey` (alias `middleware "github.com/.../internal/api/middleware"`), set by admin middleware (`admin/middleware.go:49`).
- **Response envelope**: `{code, message, data}` with `code===0` success. SPA `request.js` unwraps `data.data`.
- **Canonical verification block — run before every commit** (gofmt included per R4a lesson):
  ```
  gofmt -l $(find . -name '*.go' -not -path './vendor/*')   # must print nothing
  go build ./cmd/... ./internal/... ./pkg/...
  go vet ./...
  go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
  golangci-lint run ./...                                    # exit 0
  ```
- **Commit prefixes**: `feat(content-review)` for features, `feat(content-review)!:` if behavior changes (create no longer auto-publishes for Lv1/2), `fix(content-review)` for fixes. End every message with a blank line then `Co-Authored-By: Claude <noreply@anthropic.com>`.
- **Line numbers** reference master `d756d42`; they shift — locate by symbol/text.
- **Spec**: `docs/superpowers/specs/2026-07-07-polyant-r4b-content-review-design.md`.
- **Simplification vs spec**: the `RequireReviewForLowLevel` config flag is **dropped** (YAGNI for MVP — threshold is the constant Lv3; emergency rollback = `git revert` the commit). The spec's snake_case json tags were a typo — use camelCase to match the model.

---

## Task 1: Add reviewer fields to `KnowledgeEntry`

**Files:**
- Modify: `internal/storage/model/models.go` (KnowledgeEntry struct, ~line 70)
- Test: `internal/storage/model/models_test.go` (add test; create if absent)

**Interfaces:**
- Produces: `KnowledgeEntry` gains `ReviewedBy string`, `ReviewedAt int64`, `ReviewReason string`. Later tasks set these on review actions.

- [ ] **Step 1: Write the failing test**

Append to `internal/storage/model/models_test.go` (create the file with `package model` if it doesn't exist):
```go
package model

import (
	"encoding/json"
	"testing"
)

func TestKnowledgeEntry_ReviewerFieldsJSON(t *testing.T) {
	e := KnowledgeEntry{
		ID:           "e1",
		ReviewedBy:   "pubkey-abc",
		ReviewedAt:   1700000000000,
		ReviewReason: "spam",
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"reviewedBy", "reviewedAt", "reviewReason"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing json key %q (camelCase); got keys: reviewedBy/reviewedAt/reviewReason must exist", key)
		}
	}

	// backward-compat: zero-value entry (no review yet) must still serialize
	zero := KnowledgeEntry{ID: "e2"}
	zd, _ := json.Marshal(zero)
	var zm map[string]json.RawMessage
	json.Unmarshal(zd, &zm)
	if _, ok := zm["id"]; !ok {
		t.Error("zero-value entry lost id field")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestKnowledgeEntry_ReviewerFieldsJSON ./internal/storage/model/...`
Expected: FAIL — compile error (fields don't exist) or missing json keys.

- [ ] **Step 3: Add the fields**

In `internal/storage/model/models.go`, add to the `KnowledgeEntry` struct (after `SourceRef`, before the i18n block — keep `Status` where it is):
```go
	SourceRef     string                   `json:"sourceRef"`               // 来源引用
	ReviewedBy    string                   `json:"reviewedBy"`              // 审核者公钥（最后一次 approve/reject/takedown）
	ReviewedAt    int64                    `json:"reviewedAt"`              // 审核时间（Unix 毫秒）
	ReviewReason  string                   `json:"reviewReason,omitempty"`  // 拒绝/下架原因（approve 时可空）
	// 多语言支持
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -race -count=1 ./internal/storage/model/...`
Expected: PASS.

- [ ] **Step 5: Verify + commit**

Run the canonical verification block. Then:
```bash
git add internal/storage/model/models.go internal/storage/model/models_test.go
git commit -m "feat(content-review): add ReviewedBy/At/Reason fields to KnowledgeEntry

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Trust-based creation + index guard

**Files:**
- Modify: `internal/api/handler/entry_handler.go:287` (status branch) and the index block `:309-324` (guard with status check)
- Test: `internal/api/handler/entry_handler_test.go` (add 2 tests)

**Interfaces:**
- Consumes: `model.UserLevelLv3` threshold, `model.EntryStatusReview`/`EntryStatusPublished`.
- Produces: `CreateEntryHandler` now sets `review` for `<Lv3` creators (skipping search/title/backlink indexing) and `published` for `≥Lv3` (unchanged behavior).

- [ ] **Step 1: Write failing tests**

Append to `internal/api/handler/entry_handler_test.go` (match the existing test setup pattern — find `TestCreateEntry` or similar and reuse its store/handler construction):
```go
func TestCreateEntry_LowLevelUserEntersReview(t *testing.T) {
	h, store := newTestEntryHandler(t) // reuse existing helper; if named differently, use the file's constructor
	user := &model.User{PublicKey: "pk-lv1", UserLevel: model.UserLevelLv1, Status: model.UserStatusActive}
	req := &CreateEntryRequest{
		Title: "T", Content: "C", Category: "test",
		// attach a valid Ed25519 creator signature for title\ncontent\ncategory
		// reuse the existing test's signature helper (see TestCreateEntry)
	}
	// submit as lv1 user (set context user) — mirror existing TestCreateEntry auth setup
	entry := createEntryAs(t, h, user, req)
	if entry.Status != model.EntryStatusReview {
		t.Fatalf("lv1 create: want status review, got %q", entry.Status)
	}
	// review entries must NOT be in the search index
	res, _ := store.Search.Search(context.Background(), index.SearchQuery{Query: "T", Limit: 10})
	if res != nil && res.Total > 0 {
		t.Errorf("review entry leaked into search index: total=%d", res.Total)
	}
}

func TestCreateEntry_TrustedUserPublishes(t *testing.T) {
	h, _ := newTestEntryHandler(t)
	user := &model.User{PublicKey: "pk-lv3", UserLevel: model.UserLevelLv3, Status: model.UserStatusActive}
	entry := createEntryAs(t, h, user, &CreateEntryRequest{Title: "T3", Content: "C", Category: "test"})
	if entry.Status != model.EntryStatusPublished {
		t.Fatalf("lv3 create: want status published, got %q", entry.Status)
	}
}
```
**Note:** `newTestEntryHandler` and `createEntryAs` — find the existing test helper in `entry_handler_test.go` that constructs an `EntryHandler` with a memory store and submits a signed create request. If a `createEntryAs` helper doesn't exist, inline its body (construct request with valid signature using the same key-generation + `verifyEntrySignature` pattern the existing `TestCreateEntry` uses). The exact signature plumbing is already exercised by existing tests — copy it. Do NOT stub the signature (the handler verifies it).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestCreateEntry_LowLevelUserEntersReview|TestCreateEntry_TrustedUserPublishes' ./internal/api/handler/...`
Expected: FAIL — both get `published` (current behavior), so the review-status assertion fails.

- [ ] **Step 3: Implement the status branch**

In `internal/api/handler/entry_handler.go`, change the entry construction (~line 287). Replace:
```go
		Status:     model.EntryStatusPublished,
```
with:
```go
		Status:     entryStatusForCreator(user.UserLevel),
```
And add a helper near the top of the file (after the imports / other helpers):
```go
// entryStatusForCreator decides the initial status: trusted users (Lv3+) publish
// directly; lower-level users go to the review queue.
func entryStatusForCreator(level int32) string {
	if level >= model.UserLevelLv3 {
		return model.EntryStatusPublished
	}
	return model.EntryStatusReview
}
```

- [ ] **Step 4: Guard indexing on published status**

In the index block (~lines 309-324), wrap the three indexing steps so review entries skip them. Replace the block:
```go
	// 建立全文索引
	if h.searchEngine != nil {
		if err := h.searchEngine.IndexEntry(created); err != nil {
			log.Printf("[EntryHandler] index entry %s failed: %v", created.ID, err)
		}
	}

	// 更新标题索引
	if h.enricher != nil {
		_ = h.enricher.titleIndex.Add(index.TitleEntry{ID: created.ID, Title: created.Title})
	}

	// 建立反向链接索引
	if h.backlink != nil {
		linkedEntryIDs := linkparser.ParseLinks(created.Content)
		_ = h.backlink.UpdateIndex(created.ID, linkedEntryIDs)
	}
```
with:
```go
	// 仅 published 条目建立索引（review 等状态不进搜索/标题/反向链接）
	if created.Status == model.EntryStatusPublished {
		if h.searchEngine != nil {
			if err := h.searchEngine.IndexEntry(created); err != nil {
				log.Printf("[EntryHandler] index entry %s failed: %v", created.ID, err)
			}
		}
		if h.enricher != nil {
			_ = h.enricher.titleIndex.Add(index.TitleEntry{ID: created.ID, Title: created.Title})
		}
		if h.backlink != nil {
			linkedEntryIDs := linkparser.ParseLinks(created.Content)
			_ = h.backlink.UpdateIndex(created.ID, linkedEntryIDs)
		}
	}
```
The push block (`:327-333`) is unchanged — review entries still push to peers (status carries as-is).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -race -count=1 -run 'TestCreateEntry' ./internal/api/handler/...`
Expected: PASS (both new tests + existing TestCreateEntry).

- [ ] **Step 6: Verify + commit**

Canonical verification block. Then:
```bash
git add internal/api/handler/entry_handler.go internal/api/handler/entry_handler_test.go
git commit -m "feat(content-review)!: low-level users (Lv1/2) create into review queue

Lv3+ still publish directly. Review entries skip search/title/backlink indexing.
Breaking: Lv1/2 entries no longer immediately visible (by design).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Review service (state machine + index + push)

**Files:**
- Create: `internal/core/review/service.go`
- Test: `internal/core/review/service_test.go`

**Interfaces:**
- Consumes: `*storage.Store` (uses `.Entry.Get/Update/List`, `.Search.IndexEntry/DeleteIndex`), `handler.EntryPusher` (`PushEntry(entry, signature []byte) error`), `model.NowMillis()`, `model.EntryStatus*`.
- Produces: `review.Service` with:
  - `NewService(store *storage.Store, pusher handler.EntryPusher) *Service`
  - `ListQueue(ctx, status string, limit, offset int) ([]*model.KnowledgeEntry, int, error)`
  - `Approve(ctx, entryID, reviewerPubkey string) (*model.KnowledgeEntry, error)`
  - `Reject(ctx, entryID, reviewerPubkey, reason string) (*model.KnowledgeEntry, error)`
  - `Takedown(ctx, entryID, reviewerPubkey, reason string) (*model.KnowledgeEntry, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/core/review/service_test.go`:
```go
package review

import (
	"context"
	"sync"
	"testing"

	"github.com/daifei0527/polyant/internal/api/handler"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

type mockPusher struct {
	mu   sync.Mutex
	got  []*model.KnowledgeEntry
	errs []error
}

func (m *mockPusher) PushEntry(entry *model.KnowledgeEntry, signature []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.got = append(m.got, entry)
	return nil
}

func newTestService(t *testing.T) (*Service, *storage.Store, *mockPusher) {
	t.Helper()
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	p := &mockPusher{}
	return NewService(store, p), store, p
}

func seed(t *testing.T, store *storage.Store, id, status string) *model.KnowledgeEntry {
	t.Helper()
	e := &model.KnowledgeEntry{ID: id, Title: "T", Content: "C", Category: "x", Status: status}
	created, err := store.Entry.Create(context.Background(), e)
	if err != nil {
		t.Fatalf("seed Create %s: %v", id, err)
	}
	return created
}

func TestApprove_ReviewToPublished(t *testing.T) {
	svc, store, p := newTestService(t)
	seed(t, store, "e1", model.EntryStatusReview)

	got, err := svc.Approve(context.Background(), "e1", "reviewer-pk")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if got.Status != model.EntryStatusPublished {
		t.Errorf("status = %q, want published", got.Status)
	}
	if got.ReviewedBy != "reviewer-pk" || got.ReviewedAt == 0 {
		t.Errorf("reviewer fields not set: by=%q at=%d", got.ReviewedBy, got.ReviewedAt)
	}
	if len(p.got) != 1 {
		t.Errorf("push not called once: got %d", len(p.got))
	}
}

func TestReject_ReviewToArchived(t *testing.T) {
	svc, store, _ := newTestService(t)
	seed(t, store, "e2", model.EntryStatusReview)

	got, err := svc.Reject(context.Background(), "e2", "reviewer-pk", "spam")
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if got.Status != model.EntryStatusArchived {
		t.Errorf("status = %q, want archived", got.Status)
	}
	if got.ReviewReason != "spam" {
		t.Errorf("reason = %q, want spam", got.ReviewReason)
	}
}

func TestTakedown_PublishedToReview(t *testing.T) {
	svc, store, _ := newTestService(t)
	seed(t, store, "e3", model.EntryStatusPublished)

	got, err := svc.Takedown(context.Background(), "e3", "reviewer-pk", "policy")
	if err != nil {
		t.Fatalf("Takedown: %v", err)
	}
	if got.Status != model.EntryStatusReview {
		t.Errorf("status = %q, want review", got.Status)
	}
	if got.ReviewReason != "policy" {
		t.Errorf("reason = %q, want policy", got.ReviewReason)
	}
}

func TestApprove_IllegalTransition(t *testing.T) {
	svc, store, _ := newTestService(t)
	seed(t, store, "e4", model.EntryStatusPublished) // already published
	if _, err := svc.Approve(context.Background(), "e4", "r"); err == nil {
		t.Error("Approve on published should error (illegal transition)")
	}
}

func TestListQueue_FiltersByStatus(t *testing.T) {
	svc, store, _ := newTestService(t)
	seed(t, store, "r1", model.EntryStatusReview)
	seed(t, store, "r2", model.EntryStatusReview)
	seed(t, store, "p1", model.EntryStatusPublished)

	entries, total, err := svc.ListQueue(context.Background(), model.EntryStatusReview, 10, 0)
	if err != nil {
		t.Fatalf("ListQueue: %v", err)
	}
	if total != 2 || len(entries) != 2 {
		t.Errorf("review queue: total=%d len=%d, want 2/2", total, len(entries))
	}
}
```
Note: `handler.EntryPusher` is the interface type for the pusher param — the `mockPusher` satisfies it (`PushEntry(entry, signature []byte) error`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run 'TestApprove|TestReject|TestTakedown|TestListQueue' ./internal/core/review/...`
Expected: FAIL — package doesn't compile (no `service.go`).

- [ ] **Step 3: Implement the service**

Create `internal/core/review/service.go`:
```go
package review

import (
	"context"
	"errors"
	"fmt"

	"github.com/daifei0527/polyant/internal/api/handler"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

var (
	ErrEntryNotFound      = errors.New("entry not found")
	ErrIllegalTransition  = errors.New("illegal status transition")
)

// Service implements the entry content-review workflow.
type Service struct {
	store  *storage.Store
	pusher handler.EntryPusher
}

// NewService creates a review service. pusher may be nil (push is skipped).
func NewService(store *storage.Store, pusher handler.EntryPusher) *Service {
	return &Service{store: store, pusher: pusher}
}

// ListQueue lists entries by status (typically "review") with pagination.
func (s *Service) ListQueue(ctx context.Context, status string, limit, offset int) ([]*model.KnowledgeEntry, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	return s.store.Entry.List(ctx, storage.EntryFilter{Status: status, Limit: limit, Offset: offset})
}

// Approve moves a review entry to published and indexes it.
func (s *Service) Approve(ctx context.Context, entryID, reviewerPubkey string) (*model.KnowledgeEntry, error) {
	return s.transition(ctx, entryID, model.EntryStatusReview, model.EntryStatusPublished, reviewerPubkey, "")
}

// Reject moves a review entry to archived (terminal) with a reason.
func (s *Service) Reject(ctx context.Context, entryID, reviewerPubkey, reason string) (*model.KnowledgeEntry, error) {
	return s.transition(ctx, entryID, model.EntryStatusReview, model.EntryStatusArchived, reviewerPubkey, reason)
}

// Takedown moves a published entry back to review with a reason.
func (s *Service) Takedown(ctx context.Context, entryID, reviewerPubkey, reason string) (*model.KnowledgeEntry, error) {
	return s.transition(ctx, entryID, model.EntryStatusPublished, model.EntryStatusReview, reviewerPubkey, reason)
}

func (s *Service) transition(ctx context.Context, entryID, fromStatus, toStatus string, reviewerPubkey, reason string) (*model.KnowledgeEntry, error) {
	entry, err := s.store.Entry.Get(ctx, entryID)
	if err != nil || entry == nil {
		return nil, ErrEntryNotFound
	}
	if entry.Status != fromStatus {
		return nil, fmt.Errorf("%w: entry %s is %q, expected %q", ErrIllegalTransition, entryID, entry.Status, fromStatus)
	}

	entry.Status = toStatus
	entry.ReviewedBy = reviewerPubkey
	entry.ReviewedAt = model.NowMillis()
	entry.ReviewReason = reason
	entry.Version++ // bump so LWW sync accepts the new state
	entry.UpdatedAt = model.NowMillis()

	updated, err := s.store.Entry.Update(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("update entry %s: %w", entryID, err)
	}

	// keep the search index in sync with published state
	s.syncIndex(ctx, updated)

	// propagate to peers (status carries as-is via existing sync/LWW)
	if s.pusher != nil {
		_ = s.pusher.PushEntry(updated, updated.Signature)
	}
	return updated, nil
}

// syncIndex adds the entry to the search index on publish, removes it otherwise.
func (s *Service) syncIndex(ctx context.Context, entry *model.KnowledgeEntry) {
	if s.store.Search == nil {
		return
	}
	if entry.Status == model.EntryStatusPublished {
		if err := s.store.Search.IndexEntry(entry); err != nil {
			fmt.Printf("[review] index entry %s failed: %v\n", entry.ID, err)
		}
	} else {
		if err := s.store.Search.DeleteIndex(entry.ID); err != nil {
			fmt.Printf("[review] de-index entry %s failed: %v\n", entry.ID, err)
		}
	}
}
```
**Verify `model.NowMillis()` exists** — grep `func NowMillis` in `internal/storage/model/`. If it's named differently (e.g., `NowMilli`), use the exact name. The `store.Search` field is the `index.SearchEngine` (confirmed via router `SearchEngine: store.Search`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -count=1 ./internal/core/review/...`
Expected: PASS (all 5 tests).

- [ ] **Step 5: Verify + commit**

Canonical verification block. Then:
```bash
git add internal/core/review/
git commit -m "feat(content-review): review service with state-machine transitions

Approve(review->published), Reject(review->archived), Takedown(published->review).
Indexes/de-indexes on transition; pushes to peers; bumps Version for LWW sync.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Review handler + admin routes + audit action types

**Files:**
- Create: `internal/api/handler/review_handler.go`
- Modify: `internal/api/admin/handler.go` (add review delegation + extend `NewHandler`)
- Modify: `internal/api/router/router.go` (`registerAdminRoutes`: wire review routes; pass `deps.EntryPusher` to `admin.NewHandler`)
- Modify: `internal/api/middleware/audit.go` (add `entry.approve`/`reject`/`takedown` to sensitiveOps)
- Test: `internal/api/handler/review_handler_test.go`

**Interfaces:**
- Consumes: `review.Service` (Task 3), `mw.PublicKeyKey` context key, admin session auth.
- Produces: 4 admin endpoints under `/api/v1/admin/entries*` (session-token auth via `adminAuthMW`).

- [ ] **Step 1: Write the failing handler test**

Create `internal/api/handler/review_handler_test.go`:
```go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mw "github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/core/review"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newReviewHandler(t *testing.T) (*ReviewHandler, *storage.Store) {
	t.Helper()
	store, _ := storage.NewMemoryStore()
	svc := review.NewService(store, nil)
	return NewReviewHandler(svc), store
}

func TestApproveEntryHandler_OK(t *testing.T) {
	h, store := newReviewHandler(t)
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "t", Content: "c", Category: "x", Status: model.EntryStatusReview})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/entries/e1/approve", nil)
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "reviewer-pk"))
	rec := httptest.NewRecorder()
	h.ApproveEntryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	e, _ := store.Entry.Get(context.Background(), "e1")
	if e.Status != model.EntryStatusPublished {
		t.Errorf("entry status = %q, want published", e.Status)
	}
}

func TestRejectEntryHandler_WithReason(t *testing.T) {
	h, store := newReviewHandler(t)
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e2", Title: "t", Content: "c", Category: "x", Status: model.EntryStatusReview})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/entries/e2/reject", strings.NewReader(`{"reason":"spam"}`))
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "reviewer-pk"))
	rec := httptest.NewRecorder()
	h.RejectEntryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	e, _ := store.Entry.Get(context.Background(), "e2")
	if e.Status != model.EntryStatusArchived || e.ReviewReason != "spam" {
		t.Errorf("entry = status %q reason %q, want archived/spam", e.Status, e.ReviewReason)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run 'TestApproveEntryHandler|TestRejectEntryHandler' ./internal/api/handler/...`
Expected: FAIL — `ReviewHandler` and `NewReviewHandler` undefined.

- [ ] **Step 3: Implement the review handler**

Create `internal/api/handler/review_handler.go`:
```go
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	mw "github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/core/review"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// ReviewHandler exposes the content-review admin endpoints.
type ReviewHandler struct {
	svc *review.Service
}

// NewReviewHandler creates a ReviewHandler backed by a review.Service.
func NewReviewHandler(svc *review.Service) *ReviewHandler {
	return &ReviewHandler{svc: svc}
}

type reviewActionRequest struct {
	Reason string `json:"reason"`
}

// ListReviewQueueHandler  GET /api/v1/admin/entries?status=review&page=1&limit=20
func (h *ReviewHandler) ListReviewQueueHandler(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = model.EntryStatusReview
	}
	limit, offset := parsePagination(r) // existing helper in entry_handler.go or admin_handler.go; if absent, inline strconv.Atoi

	entries, total, err := h.svc.ListQueue(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, awerrors.Wrap(310, awerrors.CategoryStorage, "list review queue failed", 500, err))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"entries": entries, "total": total,
	}})
}

// ApproveEntryHandler  POST /api/v1/admin/entries/{id}/approve
func (h *ReviewHandler) ApproveEntryHandler(w http.ResponseWriter, r *http.Request) {
	h.doTransition(w, r, func(id, reviewer string) error {
		_, err := h.svc.Approve(r.Context(), id, reviewer)
		return err
	})
}

// RejectEntryHandler  POST /api/v1/admin/entries/{id}/reject
func (h *ReviewHandler) RejectEntryHandler(w http.ResponseWriter, r *http.Request) {
	h.doTransitionWithReason(w, r, func(id, reviewer, reason string) error {
		_, err := h.svc.Reject(r.Context(), id, reviewer, reason)
		return err
	})
}

// TakedownEntryHandler  POST /api/v1/admin/entries/{id}/takedown
func (h *ReviewHandler) TakedownEntryHandler(w http.ResponseWriter, r *http.Request) {
	h.doTransitionWithReason(w, r, func(id, reviewer, reason string) error {
		_, err := h.svc.Takedown(r.Context(), id, reviewer, reason)
		return err
	})
}

func (h *ReviewHandler) doTransition(w http.ResponseWriter, r *http.Request, fn func(id, reviewer string) error) {
	id := entryIDFromPath(r.URL.Path) // see helper below
	reviewer, _ := r.Context().Value(mw.PublicKeyKey).(string)
	if err := fn(id, reviewer); err != nil {
		writeError(w, awerrors.New(311, awerrors.CategoryAPI, err.Error(), httpStatusForReviewErr(err)))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success"})
}

func (h *ReviewHandler) doTransitionWithReason(w http.ResponseWriter, r *http.Request, fn func(id, reviewer, reason string) error) {
	id := entryIDFromPath(r.URL.Path)
	reviewer, _ := r.Context().Value(mw.PublicKeyKey).(string)
	var body reviewActionRequest
	_ = json.NewDecoder(r.Body).Decode(&body) // reason optional
	if err := fn(id, reviewer, body.Reason); err != nil {
		writeError(w, awerrors.New(311, awerrors.CategoryAPI, err.Error(), httpStatusForReviewErr(err)))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success"})
}

// entryIDFromPath parses /api/v1/admin/entries/{id}/<action> → {id}.
// net/http mux doesn't do path vars; parse manually (id is the segment between /entries/ and the action suffix).
func entryIDFromPath(path string) string {
	const prefix = "/api/v1/admin/entries/"
	if len(path) <= len(prefix) {
		return ""
	}
	rest := path[len(prefix):]
	for i, c := range rest {
		if c == '/' {
			return rest[:i]
		}
	}
	return rest
}

func httpStatusForReviewErr(err error) int {
	if errors.Is(err, review.ErrEntryNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, review.ErrIllegalTransition) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}
```
**Path parsing:** the codebase uses `net/http` mux (no gorilla/mux — the `/api/v1/admin/users/` handler routes via `strings.HasSuffix`), so `entryIDFromPath` parses the id segment manually. Do NOT import gorilla/mux. **Verify helpers before use:** grep for `parsePagination`, `writeError`, `writeJSON`, `APIResponse`, `awerrors.Wrap`, `awerrors.New` in `entry_handler.go`/`admin_handler.go` — reuse the existing ones (they exist in the `handler` package); if `parsePagination` is absent, inline it: `page, _ := strconv.Atoi(r.URL.Query().Get("page")); limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))` with defaults `page=1, limit=20`, computing `offset=(page-1)*limit`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -count=1 ./internal/api/handler/...`
Expected: PASS.

- [ ] **Step 5: Wire into admin.Handler + router + audit**

(a) In `internal/api/admin/handler.go`, extend the Handler to carry a `reviewHandler`:
```go
type Handler struct {
	adminHandler  *handler.AdminHandler
	statsHandler  *handler.StatsHandler
	reviewHandler *handler.ReviewHandler
}

func NewHandler(store *storage.Store, entryPusher handler.EntryPusher) *Handler {
	reviewSvc := review.NewService(store, entryPusher)
	return &Handler{
		adminHandler:  handler.NewAdminHandler(store),
		statsHandler:  handler.NewStatsHandler(store),
		reviewHandler: handler.NewReviewHandler(reviewSvc),
	}
}

func (h *Handler) ListReviewQueueHandler(w http.ResponseWriter, r *http.Request) {
	h.reviewHandler.ListReviewQueueHandler(w, r)
}
func (h *Handler) ApproveEntryHandler(w http.ResponseWriter, r *http.Request)    { h.reviewHandler.ApproveEntryHandler(w, r) }
func (h *Handler) RejectEntryHandler(w http.ResponseWriter, r *http.Request)     { h.reviewHandler.RejectEntryHandler(w, r) }
func (h *Handler) TakedownEntryHandler(w http.ResponseWriter, r *http.Request)   { h.reviewHandler.TakedownEntryHandler(w, r) }
```
Add imports: `"github.com/daifei0527/polyant/internal/core/review"`.

(b) In `internal/api/router/router.go` `registerAdminRoutes`, change the handler construction and add routes:
```go
	adminHandler := admin.NewHandler(deps.Store, deps.EntryPusher) // was: admin.NewHandler(deps.Store)
```
And add after the existing `/api/v1/admin/stats/entries` block (still inside `registerAdminRoutes`):
```go
	// 内容审核 API（admin session-token 认证）
	mux.Handle("/api/v1/admin/entries",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.ListReviewQueueHandler)))
	mux.Handle("/api/v1/admin/entries/",
		adminAuthMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			switch {
			case strings.HasSuffix(path, "/approve"):
				adminHandler.ApproveEntryHandler(w, r)
			case strings.HasSuffix(path, "/reject"):
				adminHandler.RejectEntryHandler(w, r)
			case strings.HasSuffix(path, "/takedown"):
				adminHandler.TakedownEntryHandler(w, r)
			default:
				http.NotFound(w, r)
			}
		})))
```

(c) In `internal/api/middleware/audit.go`, add the three review actions to `sensitiveOps` (the map/switch around line 29-45). Mirror the existing `entry.create`/`entry.update` entries:
```go
	"POST/api/v1/admin/entries/":      "entry.review", // fallback for approve/reject/takedown
```
If `sensitiveOps` keys exact paths (not prefixes), add three explicit entries mapping the suffix-routed paths. Inspect the existing structure and match it — the goal is that approve/reject/takedown requests get audited with action `entry.approve`/`entry.reject`/`entry.takedown` (or a generic `entry.review` if exact-path mapping is infeasible — generic is acceptable for MVP, documented in the commit).

- [ ] **Step 6: Verify build + full suite + lint**

Canonical verification block. Also add a quick router-level smoke check that `/api/v1/admin/entries` returns 401 without a token and 200 with one (optional but recommended — mirror existing admin route tests in `internal/api/router/router_test.go`).

- [ ] **Step 7: Commit**

```bash
git add internal/api/handler/review_handler.go internal/api/handler/review_handler_test.go internal/api/admin/handler.go internal/api/router/router.go internal/api/middleware/audit.go
git commit -m "feat(content-review): admin review endpoints + audit actions

GET /api/v1/admin/entries (queue), POST .../{id}/approve|reject|takedown,
all under adminAuthMW (session-token). Wired via admin.Handler + deps.EntryPusher.
Audit records entry.approve/reject/takedown.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Admin SPA review queue + API client + stats wiring

**Files:**
- Create: `web/admin/src/api/reviews.js`
- Create: `web/admin/src/views/reviews/List.vue`
- Modify: `web/admin/src/router/index.js` (add `/reviews` child)
- Modify: `web/admin/src/components/Sidebar.vue` (add menu item + icon import)
- Modify: `web/admin/src/api/stats.js` (add `getEntryStats`/`getRatingStats`)
- Modify: `web/admin/src/views/stats/Index.vue` (replace mock refs with real fetch)

**Interfaces:**
- Consumes: the 4 endpoints from Task 4 (`/api/v1/admin/entries*`), the existing `request.js` (Bearer token + envelope unwrap), `getEntryStats` backend (R3-E).
- Produces: a review-queue admin page with approve/reject/takedown actions; real entry-stats on the dashboard.

- [ ] **Step 1: Create the API client**

Create `web/admin/src/api/reviews.js` (mirror `api/users.js` exactly):
```js
import request from './request'

export function listReviewQueue(params) {
  // params: { status: 'review'|'published'|'archived', page, limit }
  return request.get('/admin/entries', { params })
}

export function approveEntry(id) {
  return request.post(`/admin/entries/${id}/approve`)
}

export function rejectEntry(id, reason) {
  return request.post(`/admin/entries/${id}/reject`, { reason })
}

export function takedownEntry(id, reason) {
  return request.post(`/admin/entries/${id}/takedown`, { reason })
}
```

- [ ] **Step 2: Create the review queue view**

Create `web/admin/src/views/reviews/List.vue` (mirror `views/users/List.vue` structure — el-card → el-table with `v-loading` → el-pagination; `ElMessageBox.prompt` for reason input):
```vue
<template>
  <div class="reviews-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>内容审核</span>
          <el-radio-group v-model="statusFilter" @change="fetchQueue">
            <el-radio-button label="review">待审核</el-radio-button>
            <el-radio-button label="published">已发布</el-radio-button>
            <el-radio-button label="archived">已归档</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <el-table :data="entries" v-loading="loading">
        <el-table-column prop="title" label="标题" min-width="200" />
        <el-table-column prop="createdBy" label="创建者" width="180">
          <template #default="{ row }">
            <span>{{ (row.createdBy || '').slice(0, 16) }}...</span>
          </template>
        </el-table-column>
        <el-table-column prop="category" label="分类" width="120" />
        <el-table-column prop="updatedAt" label="更新时间" width="160" />
        <el-table-column label="操作" fixed="right" width="220">
          <template #default="{ row }">
            <template v-if="row.status === 'review'">
              <el-button size="small" type="success" @click="handleApprove(row)">通过</el-button>
              <el-button size="small" type="danger" @click="handleReject(row)">拒绝</el-button>
            </template>
            <el-button v-if="row.status === 'published'" size="small" type="warning" @click="handleTakedown(row)">下架</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @size-change="fetchQueue"
          @current-change="fetchQueue"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listReviewQueue, approveEntry, rejectEntry, takedownEntry } from '@/api/reviews'

const loading = ref(false)
const entries = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const statusFilter = ref('review')

const fetchQueue = async () => {
  loading.value = true
  try {
    const res = await listReviewQueue({ status: statusFilter.value, page: currentPage.value, limit: pageSize.value })
    entries.value = res.entries || []
    total.value = res.total || 0
  } catch (e) {
    console.error('fetch review queue failed:', e)
  } finally {
    loading.value = false
  }
}

const handleApprove = async (row) => {
  try {
    await approveEntry(row.id)
    ElMessage.success('已通过')
    fetchQueue()
  } catch (e) { console.error('approve failed:', e) }
}

const promptReason = (title) => ElMessageBox.prompt('请输入原因', title, {
  confirmButtonText: '确定', cancelButtonText: '取消',
  inputPattern: /\S+/, inputErrorMessage: '原因不能为空'
}).catch(() => ({ value: null }))

const handleReject = async (row) => {
  const { value: reason } = await promptReason('拒绝条目')
  if (!reason) return
  try {
    await rejectEntry(row.id, reason)
    ElMessage.success('已拒绝')
    fetchQueue()
  } catch (e) { console.error('reject failed:', e) }
}

const handleTakedown = async (row) => {
  const { value: reason } = await promptReason('下架条目')
  if (!reason) return
  try {
    await takedownEntry(row.id, reason)
    ElMessage.success('已下架')
    fetchQueue()
  } catch (e) { console.error('takedown failed:', e) }
}

onMounted(() => { fetchQueue() })
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
.pagination { margin-top: 16px; }
</style>
```

- [ ] **Step 3: Register the route**

In `web/admin/src/router/index.js`, add a child to the `children` array (after the `users/:publicKey` entry):
```js
        {
          path: 'reviews',
          name: 'Reviews',
          component: () => import('@/views/reviews/List.vue'),
          meta: { permission: 4, title: '内容审核' }
        },
```

- [ ] **Step 4: Add the sidebar menu item**

In `web/admin/src/components/Sidebar.vue`, add after the users menu item (~line 20):
```vue
    <el-menu-item index="/reviews" v-if="hasPermission(4)">
      <el-icon><Checked /></el-icon>
      <span>内容审核</span>
    </el-menu-item>
```
And update the icon import (~line 27):
```js
import { DataLine, User, Checked } from '@element-plus/icons-vue'
```

- [ ] **Step 5: Wire real entry stats**

In `web/admin/src/api/stats.js`, add exports:
```js
export function getEntryStats() {
  return request.get('/admin/stats/entries')
}

export function getRatingStats() {
  return request.get('/admin/stats/ratings') // only if backend exists; if not, omit (see note)
}
```
**Note:** verify `/api/v1/admin/stats/ratings` exists (grep router.go). If it does NOT exist, drop `getRatingStats` and leave `ratingStats` as the mock ref — only wire `getEntryStats`. Do not invent an endpoint.

In `web/admin/src/views/stats/Index.vue`, replace the mock fetch (lines 78-79 + the `fetchData` Promise.all). Change the import (~line 71):
```js
import { getUserStats, getActivityTrend, getContributionStats, getEntryStats } from '@/api/stats'
```
Remove the `// 模拟数据` comments on `entryStats`/`ratingStats` (lines 77-79) and extend `fetchData`:
```js
  const fetchData = async () => {
    try {
      const [userRes, activityRes, contribRes, entryRes] = await Promise.all([
        getUserStats(),
        getActivityTrend(7),
        getContributionStats({ limit: 1 }),
        getEntryStats()
      ])
      userStats.value = userRes || {}
      activityTrend.value = activityRes?.trend || []
      contributionStats.value = contribRes || {}
      entryStats.value = entryRes || {}
      // ratingStats left as-is unless getRatingStats endpoint exists
    } catch (error) {
      console.error('Failed to fetch stats:', error)
    }
  }
```

- [ ] **Step 6: Build + verify**

```bash
cd web/admin && npm run build
```
Expected: build succeeds, output in `web/admin/dist/`. Then copy/sync to the embedded `internal/api/admin/dist/` if the build process does so (check `web/admin/package.json` scripts — if a postbuild copies to `internal/api/admin/dist/`, confirm it ran; otherwise manually `cp -r web/admin/dist/* internal/api/admin/dist/`).

Then the canonical Go verification block (the SPA is embedded via `//go:embed dist`, so the Go build picks up the new bundle).

- [ ] **Step 7: Commit**

```bash
git add web/admin/src/ internal/api/admin/dist/
git commit -m "feat(content-review): admin review queue UI + real entry stats

New /reviews page (queue + approve/reject/takedown), sidebar entry, route.
stats/Index.vue now calls getEntryStats (was mock).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Final verification (after all 5 tasks)

- [ ] `gofmt -l` repo-wide empty; `go build ./cmd/... ./internal/... ./pkg/...` OK; `go vet ./...` OK.
- [ ] `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` all PASS.
- [ ] `golangci-lint run ./...` exit 0.
- [ ] `cd web/admin && npm run build` succeeds.
- [ ] Manual smoke (optional): Lv1 create → appears in admin `/reviews` queue → approve → searchable; published → takedown → back in queue.
- [ ] Branch `r4b-content-review` has 5 task commits + spec/plan, ready for review/merge.
