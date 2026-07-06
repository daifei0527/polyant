# R4a Linter Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clean all 97 distinct linter findings across `unused`/`staticcheck`/`gosec`/`errcheck`, then enable those 4 linters in `.golangci.yml` + `make lint` so CI enforces them.

**Architecture:** 10 independent tasks (A1, A2, A3, B-context, B-deprecation, B-relay, C-gosec, D-prod, D-tests, E-enable), each its own commit + verification cycle. Principle: fix root cause; `//nolint:<linter>` only for unfixable-by-design cases, each naming its linter + carrying a `//` justification. One real concurrency bug is fixed along the way (wire `RatingCalculator.mu`).

**Tech Stack:** Go 1.25.x / golangci-lint v1.64.x (`unused` `staticcheck` `gosec` `errcheck`) / standard testing + `-race` / GitHub Actions (`golangci-lint-action`).

## Global Constraints

- **Go 1.25.x**; module path `github.com/daifei0527/polyant`.
- **golangci-lint v1.64.x** is installed locally; invoke per-linter with `golangci-lint run --no-config --disable-all --enable <LINTER> ./...`.
- **nolint policy:** every `//nolint` MUST name its linter and carry a `//` justification on the same line. No bare `//nolint`. No blanket excludes for findings this plan addresses.
- **Every task ends with the canonical verification block** (run it before committing):
  ```bash
  go build ./cmd/... ./internal/... ./pkg/... ./scripts/...
  go vet ./...
  go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
  ```
- **Commit prefix convention:** `refactor(lint)` for dead-code removal, `fix(...)` for real bug fixes (rating race, error propagation, rand checks), `chore(lint)` for pure suppressions / config. End every commit message with:
  ```
  Co-Authored-By: Claude <noreply@anthropic.com>
  ```
- **Line numbers** in this plan reference current master (`48909de`); they shift as earlier tasks edit files — always locate the target by grepping the shown symbol/text, not by line number alone.
- **Spec:** `docs/superpowers/specs/2026-07-06-polyant-r4a-linter-cleanup-design.md`.

---

## Task 1 (A1): Delete unused test dead code

**Files:**
- Modify: `internal/network/protocol/codec_test.go` (delete `mockStream` type + methods, lines ~450-469)
- Modify: `internal/api/router/router_test.go` (delete `mockEntryPusher` type + method, lines ~384-388)
- Modify: `internal/api/handler/export_handler_test.go` (delete `createMultipartBody`, line ~179)

**Interfaces:** none (pure deletion of unreferenced symbols).

- [ ] **Step 1: Confirm zero references before deleting**

```bash
grep -rn "mockStream\b" --include="*.go" . | grep -v "codec_test.go"
grep -rn "mockEntryPusher" --include="*.go" . | grep -v "router_test.go"
grep -rn "createMultipartBody" --include="*.go" . | grep -v "export_handler_test.go"
```
Expected: all three empty (no references outside their defining test file). If any hit, STOP — the symbol is used; do not delete.

- [ ] **Step 2: Delete `mockStream` from `codec_test.go`**

Open `internal/network/protocol/codec_test.go`, locate `type mockStream struct {` (around line 450) and delete the entire type declaration plus all its methods through the closing brace of `func (m *mockStream) Stat() network.Stats ...` (through ~line 469). Delete any now-unused imports the block relied on (run `goimports`/`gofmt` after).

- [ ] **Step 3: Delete `mockEntryPusher` from `router_test.go`**

In `internal/api/router/router_test.go`, delete:
```go
type mockEntryPusher struct{}
func (m *mockEntryPusher) PushEntry(entry *model.KnowledgeEntry, signature []byte) error {
	...
}
```
(around lines 384-388). Remove the `model` import only if nothing else in the file uses it.

- [ ] **Step 4: Delete `createMultipartBody` from `export_handler_test.go`**

In `internal/api/handler/export_handler_test.go`, delete the whole `func createMultipartBody(...)` (around line 179). Remove now-unused imports (`io`, etc.) only if unused elsewhere in the file.

- [ ] **Step 5: Verify build + tests + unused drops by 17**

```bash
go build ./cmd/... ./internal/... ./pkg/...
go test -race -count=1 ./internal/network/protocol/... ./internal/api/router/... ./internal/api/handler/...
golangci-lint run --no-config --disable-all --enable unused ./... 2>&1 | grep -cE '^[^[:space:]].*:[0-9]+:[0-9]+:'
```
Expected: build OK, tests PASS, count = **2** (only the two production `mu` fields remain).

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "refactor(lint): drop unused test mocks/helpers (mockStream, mockEntryPusher, createMultipartBody)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2 (A2.1): Remove vestigial `Protocol.mu`

**Files:**
- Modify: `internal/network/protocol/protocol.go` (delete `mu sync.RWMutex` field at ~line 32)

**Interfaces:** none.

**Investigation result (already verified):** `grep "p\.handler\s*=\|p\.codec\s*=" internal/network/protocol/protocol.go` returns empty — `handler` and `codec` are set once in `NewProtocol` and never mutated. The `mu` field guards nothing → vestigial.

- [ ] **Step 1: Re-confirm no mutation of guarded fields**

```bash
grep -nE "p\.(handler|codec)\s*=|\.handler\s*=|\.codec\s*=" internal/network/protocol/protocol.go
```
Expected: empty. If non-empty, STOP and re-evaluate (the field may need wiring instead of deletion).

- [ ] **Step 2: Delete the field**

In `internal/network/protocol/protocol.go`, in the `type Protocol struct { ... }` block, delete the line:
```go
	mu      sync.RWMutex
```
Leave `wg sync.WaitGroup` (it is used).

- [ ] **Step 3: Remove unused `sync` import if now unused**

```bash
go build ./internal/network/protocol/...
```
If the build fails on `"sync"` import — check whether anything else in the file uses `sync` (the `wg sync.WaitGroup` does). `sync` stays. Expected: build OK, no import change needed.

- [ ] **Step 4: Verify**

```bash
go build ./cmd/... ./internal/... ./pkg/...
go test -race -count=1 ./internal/network/protocol/...
golangci-lint run --no-config --disable-all --enable unused ./... 2>&1 | grep "protocol.go"
```
Expected: build OK, tests PASS, no `protocol.go` unused finding.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "refactor(lint): remove vestigial Protocol.mu (handler/codec never mutated post-construction)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3 (A2.2): Wire `RatingCalculator.mu` — fix SubmitRating RMW race

**Files:**
- Modify: `internal/core/rating/calculator.go` (add lock to `SubmitRating`, ~line 49)
- Test: `internal/core/rating/calculator_test.go` (add concurrent test)

**Investigation result (already verified):** `SubmitRating` performs an unguarded read-modify-write chain: `Rating.ListByEntry` (dup check) → `Rating.Create` → `RecalculateEntryScore` (re-read) → `Entry.Get/Set/Update` → `rater.RatingCnt++; User.Update`. There is **no per-entry lock** in the rating path (R2-D1's lock was for election voting). `mu` was declared but never locked → real lost-update race on the entry's score/ScoreCount and the rater's RatingCnt under concurrent ratings.

- [ ] **Step 1: Write the failing concurrency test**

Append to `internal/core/rating/calculator_test.go`:
```go
func TestRatingCalculator_SubmitRating_ConcurrentSafe(t *testing.T) {
	store := newTestStore(t)
	calc := NewRatingCalculator(store)

	entry := &model.KnowledgeEntry{
		ID:       "entry-concurrent",
		Title:    "Concurrent",
		Content:  "x",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	if _, err := store.Entry.Create(context.Background(), entry); err != nil {
		t.Fatalf("create entry: %v", err)
	}

	const n = 25
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rater := &model.User{
				PublicKey: fmt.Sprintf("rater-%d", i),
				AgentName: fmt.Sprintf("rater-%d", i),
				UserLevel: model.UserLevelLv1,
				Status:    model.UserStatusActive,
			}
			if _, err := calc.SubmitRating(context.Background(), "entry-concurrent", rater, 4.0, ""); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("SubmitRating error under concurrency: %v", err)
	}

	ratings, err := store.Rating.ListByEntry(context.Background(), "entry-concurrent")
	if err != nil {
		t.Fatalf("ListByEntry: %v", err)
	}
	if len(ratings) != n {
		t.Fatalf("expected %d ratings, got %d (concurrent SubmitRating lost writes)", n, len(ratings))
	}

	got, _ := store.Entry.Get(context.Background(), "entry-concurrent")
	if got == nil || got.ScoreCount != int32(n) {
		t.Fatalf("expected ScoreCount=%d, got %v", n, got)
	}
}
```
Add `"sync"` and `"fmt"` to the test file's imports if not present.

- [ ] **Step 2: Run the test under -race to confirm it fails**

```bash
go test -race -count=1 -run 'TestRatingCalculator_SubmitRating_ConcurrentSafe' ./internal/core/rating/...
```
Expected: FAIL — either a `DATA RACE` report from `-race`, or `expected 25 ratings, got <less>` / `ScoreCount` mismatch. (If it happens to pass by luck, re-run with `-count=5`; the race is non-deterministic.)

- [ ] **Step 3: Wire the lock**

In `internal/core/rating/calculator.go`, modify `SubmitRating` to acquire the mutex for the whole RMW chain:
```go
func (rc *RatingCalculator) SubmitRating(ctx context.Context, entryID string, rater *model.User, score float64, comment string) (*model.Rating, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if score < 1.0 || score > 5.0 {
		return nil, ErrScoreOutOfRange
	}
	// ... rest unchanged
```
Add a comment above the struct field documenting intent:
```go
type RatingCalculator struct {
	store *storage.Store
	// mu 串行化 SubmitRating 的读-改-写链（评分查重 + 条目分数/计票重算 + rater 计数），
	// 防止并发评分丢失更新。注：全局锁简化实现；如成热点可改为 per-entry 锁。
	mu sync.RWMutex
}
```

- [ ] **Step 4: Run the test to confirm it passes**

```bash
go test -race -count=5 -run 'TestRatingCalculator_SubmitRating_ConcurrentSafe' ./internal/core/rating/...
```
Expected: PASS ×5, no race report.

- [ ] **Step 5: Verify unused drops + full suite green**

```bash
go build ./cmd/... ./internal/... ./pkg/...
go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
golangci-lint run --no-config --disable-all --enable unused ./... 2>&1 | grep -cE '^[^[:space:]].*:[0-9]+:[0-9]+:'
```
Expected: build OK, all tests PASS, unused count = **0**.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "fix(rating): serialize SubmitRating to close lost-update race (wire RatingCalculator.mu)

SubmitRating did ListByEntry->Create->Recalculate->Entry.Update->User.Update
with no lock; concurrent ratings lost entry ScoreCount and rater RatingCnt
updates. The struct's mu was declared but never locked.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4 (B-context): staticcheck context + key hygiene

**Files:**
- Modify: `internal/api/middleware/auth_test.go` (SA1012 line ~220, SA5011 lines ~239-242)
- Modify: `internal/core/export/exporter.go` (SA1012 lines ~73, ~87)
- Modify: `internal/storage/kv/store.go` (SA6001 line ~89)
- Modify: `internal/api/admin/middleware.go` (SA1029 line ~48 — writer)
- Modify: `internal/api/handler/admin_handler.go` (SA1029 lines ~105, ~139, ~174 — readers)
- Modify: `internal/api/handler/admin_handler_test.go` (SA1029 — all `WithValue(..., "public_key", ...)` sites)
- Modify: `internal/api/admin/handler_test.go` (line ~167 — reader in test)

**Interfaces:**
- Consumes: `middleware.PublicKeyKey` (existing exported const, `internal/api/middleware/auth.go:47`, type `userContextKey`, value `"public_key"`). No import cycle (verified: `admin` and `middleware` don't import each other).

- [ ] **Step 1: Fix SA1012 nil context in `exporter.go`**

In `internal/core/export/exporter.go`, the `Export` method receives a `ctx context.Context` but its internal calls pass `nil`. Change the two nil sites to use the received ctx:
```go
// line ~73:
entries, _, err := e.store.Entry.List(ctx, storage.EntryFilter{Limit: exportAllLimit})
// line ~87:
categories, err := e.store.Category.ListAll(ctx)
```
If `Export`'s signature is `func (e *Exporter) Export(opts ExportOptions) ([]byte, error)` (no ctx), change it to `func (e *Exporter) Export(ctx context.Context, opts ExportOptions) ([]byte, error)` and update all callers (grep `\.Export(` to find them — `export_handler.go` is the likely only caller; pass `r.Context()`).

- [ ] **Step 2: Fix SA1012 nil context in `auth_test.go:220`**

Change `GetUserFromContext(nil)` → `GetUserFromContext(context.TODO())`.

- [ ] **Step 3: Fix SA5011 nil-deref in `auth_test.go:239-242`**

The nil branch uses `t.Error` (which continues execution) then dereferences `user`:
```go
	user = GetUserFromContext(ctx)
	if user == nil {
		t.Error("Expected user from context")  // continues → next line derefs nil
	}
	if user.AgentName != "test-agent" {  // SA5011
```
Fix by terminating the nil branch so the deref is provably non-nil:
```go
	if user == nil {
		t.Fatal("Expected user from context")
	}
	if user.AgentName != "test-agent" {
```

- [ ] **Step 4: Fix SA6001 in `kv/store.go:89`**

Replace the two-line:
```go
keyStr := string(key)
m[keyStr] = ...
```
with the inlined `m[string(key)] = ...` (delete `keyStr`). If `keyStr` is used elsewhere in the same scope, keep a single assignment but staticcheck flags the specific pattern — inline at the flagged use.

- [ ] **Step 5: Reuse typed context key for the admin path (SA1029)**

In `internal/api/admin/middleware.go`, add the import and change the writer:
```go
import (
	...
	mw "github.com/daifei0527/polyant/internal/api/middleware"
)
// line ~48:
ctx := context.WithValue(r.Context(), mw.PublicKeyKey, publicKey)
```
In `internal/api/handler/admin_handler.go`, change the three readers (~lines 105, 139, 174):
```go
adminPublicKey, _ := r.Context().Value(mw.PublicKeyKey).(string)
```
(add the same `mw` import alias).

In `internal/api/admin/handler_test.go:167` (test reader):
```go
pk := r.Context().Value(mw.PublicKeyKey)
```

In `internal/api/handler/admin_handler_test.go`, replace ALL `context.WithValue(req.Context(), "public_key", adminPubKeyB64)` sites (grep says ~6: lines 58, 118, 159, 184, 231, 275) with `mw.PublicKeyKey`. Add the `mw` import to the test file.

- [ ] **Step 6: Verify**

```bash
go build ./cmd/... ./internal/... ./pkg/...
go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
golangci-lint run --no-config --disable-all --enable staticcheck ./... 2>&1 | grep -E 'SA1012|SA1029|SA5011|SA6001'
```
Expected: build OK (no import cycle), tests PASS (admin handler tests still find the pubkey), no SA1012/SA1029/SA5011/SA6001 findings.

- [ ] **Step 7: Commit**

```bash
git add -A && git commit -m "fix(lint): typed context keys + non-nil ctx (SA1012/SA1029/SA5011/SA6001)

Admin path now reuses middleware.PublicKeyKey instead of a bare string key.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5 (B-deprecation): Scope `internal/storage` deprecation to `NewMemoryStore`

**Files:**
- Modify: `internal/storage/memory.go` (lines 1-5 — package doc)
- Modify: `internal/storage/store.go` (~line 124 — `NewMemoryStore` doc)

**Investigation result (already verified):** the `// Deprecated:` lives in the **package doc** of `memory.go` (lines 1-5), which marks the entire `storage` package deprecated. Importers using the non-deprecated `NewPersistentStore` (`cmd/seed`, `cmd/user`, `internal/api/admin/handler.go`) get false SA1019. Fix: move the deprecation marker off the package doc onto the `NewMemoryStore` function.

- [ ] **Step 1: Rewrite the package doc in `memory.go`**

Replace lines 1-5:
```go
// Package storage 提供基于内存的存储实现。
// 适用于开发和测试环境。
//
// Deprecated: 生产环境应使用 NewPersistentStore 创建持久化存储。
// 内存存储不会持久化数据，重启后数据丢失。
package storage
```
with an accurate, non-deprecated package doc:
```go
// Package storage 提供知识库的存储聚合与多种后端实现。
//
// 生产环境使用 NewPersistentStore（Pebble/Badger 持久化）；开发与测试环境
// 可用 NewMemoryStore（纯内存，重启数据丢失）。
package storage
```

- [ ] **Step 2: Attach the deprecation to `NewMemoryStore`**

In `internal/storage/store.go`, find `func NewMemoryStore() (*Store, error)` (~line 124) and add a doc comment immediately above it:
```go
// NewMemoryStore 创建基于内存的存储聚合，仅适用于开发与测试。
//
// Deprecated: 生产环境应使用 NewPersistentStore 创建持久化存储。内存存储不会
// 持久化数据，重启后数据丢失。
func NewMemoryStore() (*Store, error) {
```

- [ ] **Step 3: Verify SA1019 cleared for the 3 import sites**

```bash
go build ./cmd/... ./internal/... ./pkg/...
golangci-lint run --no-config --disable-all --enable staticcheck ./... 2>&1 | grep "internal/storage\" is deprecated"
```
Expected: build OK, the grep empty (no more package-deprecated SA1019 on the import). `NewMemoryStore` callers (only tests) correctly still see its deprecation if they use it directly.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor(lint): scope storage deprecation to NewMemoryStore (SA1019)

Package-level // Deprecated: in memory.go falsely flagged every import,
including NewPersistentStore callers. Moved to the NewMemoryStore func doc.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6 (B-relay + empty branches): libp2p nolint + SA9003

**Files:**
- Modify: `internal/network/host/host.go` (~line 175)
- Modify: `cmd/pactl/main.go` (~line 60)
- Modify: `cmd/pactl/service.go` (~line 160)
- Modify: `internal/api/middleware/audit.go` (~line 107)

- [ ] **Step 1: nolint the libp2p relay call**

In `internal/network/host/host.go`, change:
```go
		if cfg.EnableAutoRelay {
			opts = append(opts, libp2p.EnableAutoRelay())
		}
```
to:
```go
		if cfg.EnableAutoRelay {
			// TODO(R4+): migrate to EnableAutoRelayWithStaticRelays (configured seed
			// nodes) or EnableAutoRelayWithPeerSource. Behaviour change — deferred to
			// a dedicated round with NAT/relay testing.
			opts = append(opts, libp2p.EnableAutoRelay()) //nolint:staticcheck // migration is a behaviour change, tracked above
		}
```

- [ ] **Step 2: Fill the empty branch in `audit.go:107`**

The current code:
```go
	if err := m.auditSvc.Log(ctx, log); err != nil {
	}
```
Replace with a real handler (audit write failure is a security concern — must not be silent):
```go
	if err := m.auditSvc.Log(ctx, log); err != nil {
		// 审计写入失败不得静默：记录到 stderr 供运维排查（不影响主请求路径）。
		fmt.Fprintf(os.Stderr, "audit log write failed: op=%s path=%s err=%v\n", log.Operation, log.Path, err)
	}
```
Add `"fmt"` and `"os"` to imports if not present.

- [ ] **Step 3: Fill the empty branches in `cmd/pactl/main.go:60` and `cmd/pactl/service.go:160`**

For each, the pattern is `if err := X(); err != nil { }`. Inspect intent; the safe fix is to log and continue (CLI tools shouldn't hard-fail on i18n/systemd stop warnings):
```go
		if err := i18n.Init(localesDir, i18n.Lang(langFlag)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: i18n init failed: %v\n", err)
		}
```
```go
		if err := stopViaSystemd(name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: systemd stop failed for %s: %v\n", name, err)
		}
```
Add `"fmt"`/`"os"` imports as needed.

- [ ] **Step 4: Verify**

```bash
go build ./cmd/... ./internal/... ./pkg/...
go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
golangci-lint run --no-config --disable-all --enable staticcheck ./... 2>&1 | grep -E 'SA1019|SA9003'
```
Expected: build OK, tests PASS, no SA1019 (libp2p) or SA9003 findings.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "chore(lint): nolint libp2p relay deprecation + fill empty error branches (SA1019/SA9003)

Audit-log write failure now surfaces to stderr instead of being swallowed.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7 (C-gosec): gosec fixes (13 findings)

**Files:**
- Create: `pkg/safeconv/safeconv.go` + `pkg/safeconv/safeconv_test.go`
- Modify: `internal/storage/index/bleve_engine.go` (~lines 357, 358 — G115)
- Modify: `internal/network/sync/remote_query.go` (~lines 211, 212 — G115)
- Modify: `internal/network/sync/sync.go` (~line 404 — G115)
- Modify: `internal/api/handler/admin_handler.go` (~line 236 — G109)
- Modify: `pkg/config/config.go` (lines 18, 566 — G101, G306)
- Modify: `internal/api/middleware/apikey.go` (line 11 — G101)
- Modify: `scripts/initdata/main.go` (lines 93, 106 — G306; lines 106, 111 — errcheck, owned here to avoid cross-task conflict)
- Modify: `internal/core/email/service.go` (line 154 — G402)
- Modify: `cmd/pactl/client.go` (line 51 — G402)

- [ ] **Step 1: Create the safe-conversion helper (TDD)**

Create `pkg/safeconv/safeconv.go`:
```go
// Package safeconv provides overflow-safe integer conversions for values that
// are provably small in practice (counts, limits) but trigger gosec G115.
package safeconv

import "math"

// IntFromUint64 converts a uint64 to int, clamping at math.MaxInt32 for safety
// on all platforms. Counts/limits in this codebase never approach MaxInt32.
func IntFromUint64(v uint64) int {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(v)
}

// Int32FromInt converts an int to int32, clamping at the int32 range.
func Int32FromInt(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
```
Create `pkg/safeconv/safeconv_test.go`:
```go
package safeconv

import "testing"

func TestIntFromUint64(t *testing.T) {
	if got := IntFromUint64(0); got != 0 {
		t.Errorf("0 -> %d", got)
	}
	if got := IntFromUint64(42); got != 42 {
		t.Errorf("42 -> %d", got)
	}
	if got := IntFromUint64(1 << 40); got != math.MaxInt32 {
		t.Errorf("overflow not clamped: %d", got)
	}
}

func TestInt32FromInt(t *testing.T) {
	if got := Int32FromInt(0); got != 0 {
		t.Errorf("0 -> %d", got)
	}
	if got := Int32FromInt(100); got != 100 {
		t.Errorf("100 -> %d", got)
	}
	if got := Int32FromInt(math.MaxInt); got != math.MaxInt32 {
		t.Errorf("overflow not clamped: %d", got)
	}
	if got := Int32FromInt(-math.MaxInt); got != math.MinInt32 {
		t.Errorf("underflow not clamped: %d", got)
	}
}
```
Add `"math"` to the test imports.

- [ ] **Step 2: Run the helper tests**

```bash
go test -race -count=1 ./pkg/safeconv/...
```
Expected: PASS.

- [ ] **Step 3: Apply G115 conversions**

`internal/storage/index/bleve_engine.go` (~357, ~358):
```go
		TotalCount: safeconv.IntFromUint64(searchResult.Total),
		HasMore:    safeconv.IntFromUint64(searchResult.Total) > query.Offset+query.Limit,
```
`internal/network/sync/remote_query.go` (~211, ~212):
```go
		Limit:      safeconv.Int32FromInt(query.Limit),
		Offset:     safeconv.Int32FromInt(query.Offset),
```
`internal/network/sync/sync.go` (~404):
```go
	entry.ScoreCount = safeconv.Int32FromInt(len(ratings))
```
Add the import `"github.com/daifei0527/polyant/pkg/safeconv"` to each file.

- [ ] **Step 4: Fix G109 in `admin_handler.go:236`**

The `level` comes from `strconv.Atoi`. Clamp before the `int32` cast:
```go
level, err := strconv.Atoi(levelStr)
if err != nil {
	// ... existing 400 handling
}
users, total, err := h.adminSvc.ListUsers(ctx, (page-1)*limit, limit, safeconv.Int32FromInt(level), search)
```
(Use `safeconv.Int32FromInt`; keep existing error handling for the parse failure.)

- [ ] **Step 5: Fix G306 file permissions (and initdata errcheck — same file, owned here)**

`pkg/config/config.go:566` (`config.Save` — config may contain secrets/API keys): change `0644` → `0600`.

`scripts/initdata/main.go` generates seed JSON for distribution (world-readable is intended) — keep `0644`, add `//nolint:gosec`, AND check the WriteFile errors (initdata's errcheck findings live on the same lines, so this task owns them to avoid a cross-task conflict). Three sites:
```go
	// :93 (outputPath) — already error-checked in surrounding code; only add gosec nolint to the WriteFile line:
	if err := os.WriteFile(outputPath, data, 0644); err != nil { //nolint:gosec // 分发的种子数据，刻意世界可读
		...existing handling...
	}
	// :106 (categoriesPath) — was a bare unchecked call; wrap it:
	if err := os.WriteFile(categoriesPath, categoriesData, 0644); err != nil { //nolint:gosec // 分发数据，刻意世界可读
		log.Fatalf("write categories: %v", err)
	}
	// :111 (entriesPath) — was a bare unchecked call; wrap it:
	if err := os.WriteFile(entriesPath, entriesData, 0644); err != nil { //nolint:gosec // 分发数据，刻意世界可读
		log.Fatalf("write entries: %v", err)
	}
```
Add `"log"` to imports if not present. (Read the current file first — :93 may already be a checked `if err :=` form needing only the nolint; :106/:111 are the bare calls to wrap.)

- [ ] **Step 6: nolint G101 (legitimate placeholders / header names)**

`pkg/config/config.go:18`:
```go
const PlaceholderApiKey = "sk_live_YOUR_API_KEY_HERE" //nolint:gosec // 占位示例常量，非真实凭据
```
`internal/api/middleware/apikey.go:11`:
```go
	headerApiKey = "X-Polyant-Api-Key" //nolint:gosec // HTTP 头名称，非凭据
```

- [ ] **Step 7: nolint G402**

`internal/core/email/service.go:154`:
```go
		InsecureSkipVerify: s.config.SkipTLSVerify, //nolint:gosec // 配置驱动，默认 false（仅测试/自签场景开启）
```
`cmd/pactl/client.go:51` — this is `NewInsecureClient`, an explicitly-named opt-in constructor whose doc comment already says "用于自签名证书" (for self-signed certs). The name declares the insecurity, so nolint+comment is correct (no behaviour change):
```go
				InsecureSkipVerify: true, //nolint:gosec // NewInsecureClient 是显式命名的 opt-in，仅用于自签名证书场景
```

- [ ] **Step 8: Verify**

```bash
go build ./cmd/... ./internal/... ./pkg/... ./scripts/...
go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
golangci-lint run --no-config --disable-all --enable gosec ./...
```
Expected: build OK, tests PASS, gosec exit 0. Verify all nolints are named + justified:
```bash
grep -rn "//nolint:gosec" --include="*.go" .
```
Each line must have a `//` reason.

- [ ] **Step 9: Commit**

```bash
git add -A && git commit -m "fix(lint): clamp int conversions, tighten file perms, justify gosec nolints (G101/G109/G115/G306/G402)

Adds pkg/safeconv for overflow-safe int32/uint64->int conversions.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8 (D-prod): errcheck production / non-test code (~19)

**Files:**
- Modify: `internal/core/election/election.go` (lines 179, 219, 223, 229)
- Modify: `internal/core/admin/session.go` (line 121)
- Modify: `internal/core/email/verification.go` (line 286)
- Modify: `pkg/logger/logger.go` (lines 157, 161)
- Modify: `internal/service/daemon/daemon.go` (line 26)
- Modify: `cmd/pactl/service.go` (lines 208, 241, 253, 317)
- Modify: `cmd/pactl/entry.go` (line 226)
- Modify: `scripts/integration/main.go` (lines 201, 220, 236)

- [ ] **Step 1: Propagate election result-persistence errors**

`internal/core/election/election.go` — `Vote` auto-elect (~line 179):
```go
		s.candidateStore.UpdateStatus(ctx, electionID, candidateID, model.CandidateStatusElected)
```
→
```go
		if err := s.candidateStore.UpdateStatus(ctx, electionID, candidateID, model.CandidateStatusElected); err != nil {
			return nil, fmt.Errorf("auto-elect: update candidate status: %w", err)
		}
```
`CloseElection` (~219, ~223, ~229) — inside the candidate loop:
```go
		if c.VoteCount >= election.VoteThreshold {
			c.Status = model.CandidateStatusElected
			if err := s.candidateStore.UpdateStatus(ctx, electionID, c.UserID, model.CandidateStatusElected); err != nil {
				return nil, fmt.Errorf("close election: mark elected: %w", err)
			}
			elected = append(elected, c)
		} else {
			c.Status = model.CandidateStatusRejected
			if err := s.candidateStore.UpdateStatus(ctx, electionID, c.UserID, model.CandidateStatusRejected); err != nil {
				return nil, fmt.Errorf("close election: mark rejected: %w", err)
			}
		}
```
and the election status update (~229):
```go
	election.Status = model.ElectionStatusClosed
	if err := s.electionStore.Update(ctx, election); err != nil {
		return nil, fmt.Errorf("close election: update election: %w", err)
	}
```

- [ ] **Step 2: Check crypto/rand errors (check-and-panic)**

Both sites generate security-critical secrets (session tokens, verification codes) from `crypto/rand`. On CSPRNG failure, returning a predictable token/code is a worse outcome than crashing, so check-and-panic (no signature ripple; `CreateSession`/`GenerateCode` keep returning non-error shapes).

`internal/core/admin/session.go` — `generateToken()` (called by `CreateSession`):
```go
// generateToken 生成随机 Token
func generateToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// CSPRNG 故障极罕见；退化为可预测 token 会让 admin 会话被伪造，
		// 直接 panic 由 supervisor 重启优于签发可预测会话。
		panic(fmt.Sprintf("crypto/rand for session token failed: %v", err))
	}
	return hex.EncodeToString(bytes)
}
```
`internal/core/email/verification.go` — `generateRandomCode()` (called by `GenerateCode`):
```go
// generateRandomCode 生成随机验证码
func (vm *VerificationManager) generateRandomCode() string {
	bytes := make([]byte, vm.codeLength/2+1)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("crypto/rand for verification code failed: %v", err))
	}
	return hex.EncodeToString(bytes)[:vm.codeLength]
}
```
Ensure `"fmt"` is imported in both files.

- [ ] **Step 3: Log logger-rotation rename failures**

`pkg/logger/logger.go` (~157, ~161):
```go
			os.Rename(oldName, newName)
```
→
```go
			if err := os.Rename(oldName, newName); err != nil {
				fmt.Fprintf(os.Stderr, "log rotate rename %s->%s failed: %v\n", oldName, newName, err)
			}
```
(and the same for the `l.filePath` → `l.filePath+".1"` rename at ~161).

- [ ] **Step 4: Make daemon StartFn explicit**

`internal/service/daemon/daemon.go:26` — read context. If `StartFn()` is best-effort (fire-and-forget daemon start), make the ignore explicit:
```go
		_ = p.StartFn() // best-effort; errors surface via process state
```
If it can fail meaningfully, propagate. Inspect `StartFn` signature first.

- [ ] **Step 5: pactl service.go — handle pid-parse, nolint best-effort exec**

`cmd/pactl/service.go` (~208, ~317) `fmt.Sscanf` pid parse:
```go
		if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
			return 0, fmt.Errorf("parse pid from %s: %w", pidFile, err)
		}
```
(adjust to the surrounding return shape).
`cmd/pactl/service.go` (~241, ~253) `exec.Command(...).Run()` (best-effort journalctl/tail log fetch):
```go
				exec.Command(journalctlPath, args...).Run() //nolint:errcheck // best-effort log fetch
```
```go
			exec.Command("tail", args...).Run() //nolint:errcheck // best-effort log fetch
```

- [ ] **Step 6: pactl entry.go Scanln — nolint**

`cmd/pactl/entry.go:226`:
```go
			fmt.Scanln(&confirm) //nolint:errcheck // 一次性确认提示，读取失败即视为否定
```

- [ ] **Step 7: scripts/integration — check json.Unmarshal**

`scripts/integration/main.go` (~201, ~220, ~236):
```go
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		log.Fatalf("unmarshal response: %v", err)
	}
```
(Add `"log"` import if not present. `scripts/initdata` is owned entirely by Task 7 Step 5 — do not touch it here.)

- [ ] **Step 8: Verify**

```bash
go build ./cmd/... ./internal/... ./pkg/... ./scripts/...
go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
golangci-lint run --no-config --disable-all --enable errcheck ./cmd/... ./internal/core/... ./internal/service/... ./pkg/... ./scripts/...
```
Expected: build OK, tests PASS, errcheck clean for production paths. (Test-only errcheck findings remain — handled in Task 9.)

- [ ] **Step 9: Commit**

```bash
git add -A && git commit -m "fix(lint): check ignored errors in production paths (election results, rand, log rotation, pid parse)

Unchecked UpdateStatus/Update on election close/auto-elect could silently
corrupt results. crypto/rand.Read for session tokens/verification codes now
checked.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9 (D-tests): errcheck test / benchmark code (~31)

**Files:** (test files only)
- `internal/service/daemon/daemon_test.go` (4)
- `pkg/logger/logger_test.go` (3)
- `pkg/polysdk/client_test.go` (2)
- `internal/storage/index/bleve_engine_test.go` (4)
- `internal/storage/index/title_index_test.go` (4)
- `internal/storage/kv/pebble_store_test.go` (4)
- `internal/network/protocol/benchmark_test.go` (3)
- `cmd/pactl/api_test.go` (1)
- `cmd/pactl/process_unix_test.go` (1)
- `internal/core/category/manager_test.go` (3)
- `internal/core/election/election_test.go` (2)

**Convention:** where the call MUST succeed or the test is meaningless → check with `if err := X(); err != nil { t.Fatal(err) }`. Where ignoring is idiomatic (benchmark loops, fatal-on-failure setup) → `//nolint:errcheck // <reason>`.

- [ ] **Step 1: Get the exact current list**

```bash
golangci-lint run --no-config --disable-all --enable errcheck ./... 2>&1 | grep "_test.go"
```
Expected: ~31 lines (production ones are already fixed in Task 8). Work through each.

- [ ] **Step 2: Fix the meaningful ones (t.Fatal)**

`election_test.go:111,135` — `NominateCandidate` failure makes the test meaningless:
```go
	if err := service.NominateCandidate(ctx, election.ID, "candidate1", "Candidate One", "nominator1", true); err != nil {
		t.Fatalf("NominateCandidate: %v", err)
	}
```
`category/manager_test.go:58,78,95` — `Initialize`:
```go
	if err := mgr.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
```
`title_index_test.go:298-301` — `Build`/`Add`:
```go
	if err := ti.Build([]TitleEntry{{ID: "e1", Title: "A"}}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, e := range []TitleEntry{{ID: "e2", Title: "B"}, {ID: "e3", Title: "AB"}, {ID: "e4", Title: "C"}} {
		if err := ti.Add(e); err != nil {
			t.Fatalf("Add %+v: %v", e, err)
		}
	}
```
`pebble_store_test.go:57,93,94,119` — `store.Put` setup:
```go
	if err := store.Put(key, value); err != nil {
		t.Fatalf("Put: %v", err)
	}
```

- [ ] **Step 3: nolint the idiomatic ones**

`benchmark_test.go:33,63,89` (benchmark encode/decode loops):
```go
		codec.Encode(msg)   //nolint:errcheck // benchmark
		codec.Decode(reader) //nolint:errcheck // benchmark
```
`bleve_engine_test.go:70,105,145,181` (`IndexEntry` setup — if the engine.IndexEntry error is asserted later, check it; if pure setup, nolint with reason):
```go
		engine.IndexEntry(e) //nolint:errcheck // test setup; subsequent assertions cover correctness
```
`client_test.go:26,232`, `api_test.go:33` (`json.NewEncoder(w).Encode` in test servers):
```go
	json.NewEncoder(w).Encode(data) //nolint:errcheck // test server response
```
`daemon_test.go:133,148` (`prg.Start`), `:221,253` (`p.Signal`) — these test daemon internals; check or nolint per intent (likely nolint: the test asserts process state elsewhere):
```go
		prg.Start(nil)              //nolint:errcheck // daemon test; asserts state via Wait
		p.Signal(syscall.SIGTERM)   //nolint:errcheck // signal delivery asserted via Wait
```
`process_unix_test.go:34` (`cmd.Wait`):
```go
	cmd.Wait() //nolint:errcheck // test asserts exit state separately
```
`logger_test.go:222,412` (`Close()`), `:387` (`os.WriteFile` setup):
```go
	defer func() { _ = Close() }()
	os.WriteFile(backupPath, []byte("backup"), 0644) //nolint:errcheck // test fixture
```
(For `defer Close()` prefer `defer func(){ _ = Close() }()` to stay explicit.)

- [ ] **Step 4: Verify**

```bash
go build ./cmd/... ./internal/... ./pkg/... ./scripts/...
go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
golangci-lint run --no-config --disable-all --enable errcheck ./...
```
Expected: build OK, tests PASS, errcheck exit 0 across all packages.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "chore(lint): check/nolint test error returns (errcheck)

Meaningful calls (NominateCandidate, Initialize, Build/Add, Put) now t.Fatal;
idiomatic ignores (benchmarks, test-server encode, signal/wait) nolint+reason.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10 (E): Enable the 4 linters in `.golangci.yml` + `make lint`

**Files:**
- Modify: `.golangci.yml`
- Modify: `Makefile` (`lint` target)
- Verify: `.github/workflows/ci.yml` (no change expected — `golangci-lint-action` auto-loads config)

- [ ] **Step 1: Confirm clean baseline before enabling**

```bash
for L in unused staticcheck gosec errcheck; do
  echo "=== $L ==="
  golangci-lint run --no-config --disable-all --enable $L ./... 2>&1 | tail -2
done
```
Expected: each linter reports zero issues (exit 0). If any still reports findings, return to the relevant task before enabling.

- [ ] **Step 2: Enable the linters in `.golangci.yml`**

In `.golangci.yml`, move the four linters from the commented "暂缓" block into the `enable:` list. The `linters:` section becomes:
```yaml
linters:
  disable-all: true
  enable:
    - gofmt
    - goimports
    - misspell
    - ineffassign
    - govet
    - gosec
    - staticcheck
    - unused
    - errcheck
```
Replace the long "以下 linter 暂缓启用" comment block with:
```yaml
    # gosec/staticcheck/unused/errcheck 于 R4a 清理后启用（2026-07-06）。
    # 唯一受控 nolint：libp2p EnableAutoRelay SA1019（host.go，迁移留独立轮）。
```
Keep `max-issues-per-linter: 50` / `max-same-issues: 10`.

- [ ] **Step 3: Extend `make lint`**

In `Makefile`, change:
```make
lint: fmt vet
```
to:
```make
lint: fmt vet
	golangci-lint run ./...
```

- [ ] **Step 4: Full project verification with config**

```bash
make lint
golangci-lint run ./...
go build ./cmd/... ./internal/... ./pkg/... ./scripts/...
go vet ./...
go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
```
Expected: `make lint` exit 0, `golangci-lint run ./...` exit 0, build/vet/test all green.

- [ ] **Step 5: Sanity-check the controlled nolint is the only one**

```bash
grep -rn "//nolint" --include="*.go" . | grep -v "_test.go" | grep -v "scripts/"
```
Review the list: each must name a linter and carry a reason. The libp2p `//nolint:staticcheck` should be present; everything else justified.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "chore(lint): enable gosec/staticcheck/unused/errcheck in golangci + make lint (R4a)

All four linters now pass on a clean tree and are enforced in CI via
golangci-lint-action. Only controlled nolint: libp2p relay SA1019 (tracked).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Final verification (after all 10 tasks)

- [ ] `golangci-lint run ./...` → exit 0.
- [ ] `make lint` → exit 0.
- [ ] `go build ./cmd/... ./internal/... ./pkg/... ./scripts/...` → OK.
- [ ] `go vet ./...` → OK.
- [ ] `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` → all PASS (38+ packages, 0 failures).
- [ ] Every `//nolint` names its linter + carries a `//` reason (`grep -rn "//nolint" --include="*.go" .`).
- [ ] The two mu-field outcomes are documented in their commit messages (Protocol.mu deleted as vestigial; RatingCalculator.mu wired with the race rationale).
- [ ] Branch `r4a-linter-cleanup` has 10 task commits + the spec commit, ready for review/merge.
