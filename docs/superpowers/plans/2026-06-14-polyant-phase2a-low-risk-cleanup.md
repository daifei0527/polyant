# Polyant Phase 2A — Low-Risk Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Land the cleanest, lowest-risk Phase 2 items first: fix the misleading `UpdateEntryScore` name (P2.6), replace deprecated `ioutil` with `os` (P2.8), and make `config.Save` path-handling cross-platform (P2.8). No behavior change for callers; each is a TDD cycle with an atomic commit.

**Architecture:** Three independent, mechanical, fully code-verified fixes. Phase 2's heavier items (P2.3 atomic voting, P2.7 isLocalRequest config-wiring, P2.1 i18n, P2.4 NAT, P2.5 aggregation/paging, P2.2 persistence) are deferred to Plan 2B.

**Tech Stack:** Go, testify, standard library (`os`, `path/filepath`).

**Spec source:** `docs/superpowers/specs/2026-06-13-polyant-full-sweep-design.md` §5 (P2.6, P2.8). Root causes re-verified against master (HEAD `dabc8cf`).

**Verified code-truth notes:**
- **P2.6:** `RatingStore.UpdateEntryScore` (rating_store.go:111) computes the weighted average and returns it but persists nothing; `grep` shows **zero callers**. The name implies a side effect that doesn't exist. Rename → `ComputeEntryScore`.
- **P2.8 (ioutil):** `io/ioutil` is deprecated since Go 1.16; `os.ReadFile`/`os.WriteFile` are drop-in replacements. Sites: `internal/auth/ed25519/keys.go` (6), `pkg/config/config.go` (2).
- **P2.8 (filepath):** `config.Save` uses `path[:strings.LastIndex(path, "/")]` — breaks on Windows (`\`). Replace with `filepath.Dir`.

**Verification gate (after every task):** `go build ./... && go vet ./... && go test ./cmd/... ./internal/... ./pkg/...`

---

## Task 1: P2.6 — Rename UpdateEntryScore → ComputeEntryScore

**Files:**
- Modify: `internal/storage/kv/rating_store.go`
- Modify: `internal/storage/kv/rating_store_test.go` (if it references the old name)

- [ ] **Step 1: Rename the method + fix the doc comment**

In `internal/storage/kv/rating_store.go`, rename `UpdateEntryScore` → `ComputeEntryScore` and clarify the comment (it computes, does NOT persist):

```go
// ComputeEntryScore 重新计算指定条目的加权平均评分。
// 注意：本方法仅"计算"并返回分数，不持久化到条目；调用方需自行将返回值
// 写回 entry.Score 并保存（原方法名 UpdateEntryScore 有误导性——它并不 update 存储）。
// 返回新的加权平均分；无评分时返回 0。
func (rs *RatingStore) ComputeEntryScore(entryId string) (float64, error) {
```

(Only the function name + comment change; body unchanged.)

- [ ] **Step 2: Find and update any caller/reference**

Run: `grep -rn "UpdateEntryScore" --include="*.go" .`
Expected before: only the definition (verified zero external callers). Update any test that calls the old name to `ComputeEntryScore`. If `rating_store_test.go` references it, rename there too.

- [ ] **Step 3: Add/confirm a test asserting the computed score**

In `internal/storage/kv/rating_store_test.go` (create if absent), ensure a test verifies the weighted-average computation via `ComputeEntryScore`:

```go
func TestRatingStore_ComputeEntryScore_WeightedAverage(t *testing.T) {
	dir := t.TempDir()
	kvStore := newTestKVStore(t, dir) // use the existing test helper in this package
	rs := NewRatingStore(kvStore)

	// rater A: score 5 * weight 1 = 5 ; rater B: score 3 * weight 2 = 6
	require.NoError(t, rs.CreateRating(&model.Rating{EntryId: "e1", RaterPubkey: "A", Score: 5, Weight: 1}))
	require.NoError(t, rs.CreateRating(&model.Rating{EntryId: "e1", RaterPubkey: "B", Score: 3, Weight: 2}))

	score, err := rs.ComputeEntryScore("e1")
	require.NoError(t, err)
	// (5*1 + 3*2) / (1+2) = 11/3 ≈ 3.6667
	assert.InDelta(t, 3.6666667, score, 1e-6)
}
```

(Use whatever KV-store test constructor already exists in `package kv` tests — e.g. `newTestBadgerStore`/`NewMemoryStore`; match the existing pattern.)

- [ ] **Step 4: Build/vet/test and commit**

Run: `go build ./... && go vet ./... && go test ./internal/storage/kv/...`. Expected: PASS.

```bash
git add internal/storage/kv/rating_store.go internal/storage/kv/rating_store_test.go
git commit -m "refactor(rating): rename UpdateEntryScore to ComputeEntryScore

The method computes the weighted average but persists nothing (zero
callers). The old name implied a side effect that doesn't exist —
renamed to ComputeEntryScore with a doc note that callers must persist
the returned score themselves."
```

---

## Task 2: P2.8 — Replace deprecated ioutil with os

**Files:**
- Modify: `internal/auth/ed25519/keys.go`
- Modify: `pkg/config/config.go`

- [ ] **Step 1: Replace in keys.go**

In `internal/auth/ed25519/keys.go`, replace all 6 `ioutil.` calls:
- `ioutil.WriteFile(p, data, mode)` → `os.WriteFile(p, data, mode)`
- `ioutil.ReadFile(p)` → `os.ReadFile(p)`

Replace the import `"io/ioutil"` with `"os"` (if `os` is not already imported). Run `grep -n "ioutil" internal/auth/ed25519/keys.go` → must be empty.

- [ ] **Step 2: Replace in config.go**

In `pkg/config/config.go`, replace the 2 `ioutil.` calls (lines ~251, ~544):
- `ioutil.ReadFile(path)` → `os.ReadFile(path)`
- `ioutil.WriteFile(path, data, 0644)` → `os.WriteFile(path, data, 0644)`

Replace import `"io/ioutil"` with `"os"` (if not already imported).

- [ ] **Step 3: Confirm no ioutil remains + build**

Run: `grep -rn "io/ioutil\|ioutil\." --include="*.go" internal/ pkg/` → must be empty (excluding vendor). Then `go build ./... && go vet ./...`. Expected: clean.

- [ ] **Step 4: Test and commit**

Run: `go test ./internal/auth/... ./pkg/config/...`. Expected: PASS.

```bash
git add internal/auth/ed25519/keys.go pkg/config/config.go
git commit -m "chore: replace deprecated io/ioutil with os

ioutil is deprecated since Go 1.16; os.ReadFile/WriteFile are drop-in
replacements. Updates keys.go (6 sites) and config.go (2 sites)."
```

---

## Task 3: P2.8 — Cross-platform path handling in config.Save

**Files:**
- Modify: `pkg/config/config.go`

- [ ] **Step 1: Write a failing test (Windows-style path separator)**

In `pkg/config/config_test.go`, add a test that `Save` creates the parent dir on a path using `filepath.Join` (works on the host platform; the bug is the hard-coded `"/"` splitter):

```go
func TestSave_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "deep", "config.json")
	cfg := DefaultConfig()

	require.NoError(t, Save(cfg, target))

	// parent dir must have been created
	assert.DirExists(t, filepath.Dir(target))
	// file must exist and re-load round-trip
	loaded, err := Load(target)
	require.NoError(t, err)
	assert.Equal(t, cfg.Node.Name, loaded.Node.Name)
}
```

(Add `"path/filepath"`, `"github.com/stretchr/testify/assert"`, `"github.com/stretchr/testify/require"` to imports as needed.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/config/ -run TestSave_CreatesParentDir -v`
Expected: FAIL — current `path[:strings.LastIndex(path, "/")]` returns empty/wrong dir for a nested path on the host, so `MkdirAll` is skipped and `WriteFile` fails (no such dir).

- [ ] **Step 3: Fix with filepath.Dir**

In `pkg/config/config.go` `Save`, replace:

```go
	dir := path[:strings.LastIndex(path, "/")]
```

with:

```go
	dir := filepath.Dir(path)
```

Add the import `"path/filepath"`. (`strings` stays — still used elsewhere in the file.)

- [ ] **Step 4: Build/vet/test and commit**

Run: `go build ./... && go vet ./... && go test ./pkg/config/...`. Expected: PASS.

```bash
git add pkg/config/config.go pkg/config/config_test.go
git commit -m "fix(config): use filepath.Dir for cross-platform Save parent dir

config.Save split the parent dir with strings.LastIndex(path, \"/\"),
which is wrong on Windows (\\\\) and fragile. Use filepath.Dir. Adds a
test that Save creates nested parent dirs and round-trips."
```

---

## Phase 2A Verification Gate

- [ ] `go build ./...` ✓
- [ ] `go vet ./...` ✓
- [ ] `go test ./cmd/... ./internal/... ./pkg/...` ✓ (root `test/` excluded — Phase 3)
- [ ] P2.6: `ComputeEntryScore` computes weighted avg; zero callers of old name.
- [ ] P2.8: zero `ioutil` references; `config.Save` uses `filepath.Dir`.

## Deferred to Plan 2B
- **P2.3** atomic vote counting (per-election mutex + `HasVoted` error propagation).
- **P2.7** `isLocalRequest` config-wiring (thread `Admin.Listen` into the admin package).
- **P2.1** i18n pipeline, **P2.4** NAT detection, **P2.5** storage aggregation + cursor paging, **P2.2** in-memory persistence + election auto-close.
- **P2.8 remainder:** remove dead `SimpleSearchEngine`, `level_checker.checkUpgrade` return-current-level, Windows `processExists`, env-prefix doc alignment, election design spec.
