# R4c KV Backup/Restore + GC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add online KV backup (Pebble Checkpoint / Badger Backup), offline restore (pactl engine-direct), and periodic GC — the three KV-maintenance pieces missing from the storage layer.

**Architecture:** 5 tasks — (1) extend `kv.Store` with `Backup`+`RunGC` across all 4 backends; (2) `BackupService` + manifest + config; (3) admin endpoints + SPA trigger; (4) `GarbageCollector` background task wired into both nodes; (5) pactl offline list + engine-direct restore. Restore stays off-interface (offline pactl).

**Tech Stack:** Go 1.25.x / Pebble v1.1.5 (`DB.Checkpoint`) / Badger v4.9.1 (`DB.Backup`/`DB.Load`) / Vue 3 SPA / cobra CLI / stdlib `log` + `time`.

## Global Constraints

- **Go 1.25.x**; module `github.com/daifei0527/polyant`.
- **`kv.Store` interface** (`internal/storage/kv/store.go:29-40`) currently: `Put/Get/Delete/Scan/Close`. This plan adds `Backup(destDir string) error` + `RunGC() error`. ALL implementations (PebbleStore, BadgerStore, MemoryStore, JSONFileStore) AND all test fakes must add both — the compiler (`var _ Store = ...` assertions) forces compliance; grep for other `kv.Store` fakes and update them.
- **Engine APIs** (verified): Pebble `(*pebble.DB).Checkpoint(destDir string, opts ...pebble.CheckpointOption) error`; Badger `(*badger.DB).Backup(w io.Writer, since uint64) (uint64, error)` and `(*badger.DB).Load(r io.Reader, maxPendingWrites int) error`.
- **Production backend = Pebble** (`NewPersistentStore` default → Pebble). KV path = `<DataDir>/kv` (`cmd/seed/main.go:222`, `cmd/user/main.go:232`).
- **Background-task pattern** to mirror: `internal/core/integrity/checker.go` (struct with `cancel context.CancelFunc` + `wg sync.WaitGroup`; `Start(ctx)/Stop()`; `loop` with `time.Ticker`+`select <-ctx.Done()`; per-cycle `recover()`). Uses stdlib `log.Printf` (not zap) in core packages.
- **Node wiring**: seed `cmd/seed/main.go` struct fields `:64-67`, Start `:273-298`, Stop block `:496-510` (levelChecker/electionCloser/integrityChecker). **User node has NO background tasks yet** — this plan adds the first one (GC) to `cmd/user/main.go` by mirroring seed's pattern.
- **admin.Handler** (`internal/api/admin/handler.go:20`): `NewHandler(store *storage.Store, entryPusher handler.EntryPusher)`. Routes registered in `registerAdminRoutes` (`router.go:453+`) under `adminAuthMW` (session-token): `mux.Handle("/api/v1/admin/backup", adminAuthMW.Middleware(http.HandlerFunc(...)))`.
- **Config** (`pkg/config/config.go`): `StorageConfig{KVType, SearchType}` at `:124-128`; defaults in `DefaultConfig()` `:236-239`; `NodeConfig.DataDir` at `:24`. `Load(path)` starts from `DefaultConfig()` then unmarshals, so new fields with zero-values need explicit defaults set in the consuming code.
- **pactl**: cobra; `--config` flag exists (`cmd/pactl/main.go:147`) but is currently unused. `cmd/pactl/admin.go` is the admin command group skeleton; `cmd/pactl/admin_user.go` is the subcommand pattern; `cmd/pactl/client.go` has `doRequestWithAuth`. **For offline ops, do NOT use the client** — read `--config` via `config.Load` and open the engine directly (precedent: `scripts/scan_pebble.go`).
- **Canonical verification block — run before every commit** (gofmt included):
  ```
  gofmt -l $(find . -name '*.go' -not -path './vendor/*')   # must print nothing
  go build ./cmd/... ./internal/... ./pkg/...
  go vet ./...
  go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
  golangci-lint run ./...                                    # exit 0
  ```
- **Commit prefixes**: `feat(kv-ops)` for features, `refactor(kv-ops)` for the interface change. End every message with a blank line then `Co-Authored-By: Claude <noreply@anthropic.com>`.
- **Line numbers** reference master `4771e53`; they shift — locate by symbol/text.
- **Spec**: `docs/superpowers/specs/2026-07-07-polyant-r4c-kv-backup-design.md`.
- **Scope refinement vs spec**: `pactl backup create` (online) is **deferred** — pactl's auth (Ed25519/API-key) can't easily reach session-token admin endpoints; the admin SPA endpoint is the online trigger. pactl does offline list (filesystem) + engine-direct restore. This preserves the spec's online-backup goal (via SPA) without the auth entanglement.

---

## Task 1: Extend `kv.Store` with `Backup` + `RunGC` (all 4 backends)

**Files:**
- Modify: `internal/storage/kv/store.go` (Store interface ~:29-40)
- Modify: `internal/storage/kv/pebble_store.go` (add Backup + RunGC)
- Modify: `internal/storage/kv/badger_store.go` (add Backup; RunGC exists, promote)
- Modify: `internal/storage/kv/memory_store.go` (add Backup + RunGC)
- Modify: `internal/storage/kv/store.go` JSONFileStore (add Backup + RunGC)
- Modify: any test fake implementing `kv.Store` (grep `kv.Store` in `*_test.go`)
- Test: `internal/storage/kv/backup_test.go` (new)

**Interfaces:**
- Produces: `kv.Store` now requires `Backup(destDir string) error` and `RunGC() error`. PebbleStore.Backup = `db.Checkpoint(destDir)`; BadgerStore.Backup = `db.Backup` to `destDir/backup.bak`; MemoryStore/JSONFileStore.Backup = dump to `destDir/dump.json`. RunGC: Pebble = `db.Compact(nil,nil,false)`, Badger = `db.RunValueLogGC` loop, Memory/JSONFile = no-op.

- [ ] **Step 1: Write the failing test**

Create `internal/storage/kv/backup_test.go`:
```go
package kv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestBackupRestoreRoundtrip_Pebble verifies PebbleStore.Backup produces a
// directory that can be reopened as a valid Pebble DB containing the same keys.
func TestBackupRestoreRoundtrip_Pebble(t *testing.T) {
	srcDir := t.TempDir()
	store, err := NewPebbleStore(srcDir)
	if err != nil { t.Fatalf("NewPebbleStore: %v", err) }
	defer store.Close()

	seed := []struct{ k, v string }{{"entry:a", "1"}, {"entry:b", "2"}, {"user:x", "9"}}
	for _, e := range seed {
		if err := store.Put([]byte(e.k), []byte(e.v)); err != nil {
			t.Fatalf("Put %s: %v", e.k, err)
		}
	}

	backupDir := filepath.Join(t.TempDir(), "bk")
	if err := store.Backup(backupDir); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	// backup dir must exist and be non-empty
	entries, err := os.ReadDir(backupDir)
	if err != nil { t.Fatalf("read backup dir: %v", err) }
	if len(entries) == 0 { t.Fatal("backup dir is empty") }

	// reopen the backup dir as a fresh PebbleStore and verify keys
	if err := store.Close(); err != nil { t.Fatalf("close src: %v", err) }
	restored, err := NewPebbleStore(backupDir)
	if err != nil { t.Fatalf("reopen backup: %v", err) }
	defer restored.Close()
	for _, e := range seed {
		got, err := restored.Get([]byte(e.k))
		if err != nil { t.Errorf("Get %s from restored: %v", e.k, err); continue }
		if string(got) != e.v { t.Errorf("Get %s = %q, want %q", e.k, got, e.v) }
	}
}

// TestRunGC_Pebble verifies RunGC does not panic on a small Pebble DB.
func TestRunGC_Pebble(t *testing.T) {
	store, err := NewPebbleStore(t.TempDir())
	if err != nil { t.Fatalf("NewPebbleStore: %v", err) }
	defer store.Close()
	store.Put([]byte("k"), []byte("v"))
	if err := store.RunGC(); err != nil {
		t.Fatalf("RunGC: %v", err)
	}
}

// TestBackup_Memory verifies the generic dump path (used by tests/dev).
func TestBackup_Memory(t *testing.T) {
	m := NewMemoryStore()
	m.Put([]byte("k1"), []byte("v1"))
	backupDir := filepath.Join(t.TempDir(), "bk")
	if err := m.Backup(backupDir); err != nil { t.Fatalf("Backup: %v", err) }
	if _, err := os.Stat(filepath.Join(backupDir, "dump.json")); err != nil {
		t.Fatalf("dump.json missing: %v", err)
	}
	if err := m.RunGC(); err != nil { t.Fatalf("Memory RunGC: %v", err) }
}

// Reference model package so the import isn't dropped if unused later.
var _ = model.EntryStatusPublished
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run 'TestBackup|TestRunGC' ./internal/storage/kv/...`
Expected: FAIL — compile error: `Backup`/`RunGC` not defined on the Store interface / impls.

- [ ] **Step 3: Extend the interface**

In `internal/storage/kv/store.go`, add the two methods to the `Store` interface:
```go
type Store interface {
	Put(key, value []byte) error
	Get(key []byte) ([]byte, error)
	Delete(key []byte) error
	Scan(prefix []byte) (map[string][]byte, error)
	Close() error
	// Backup 写入一致性快照到 destDir（目录格式）。R4c。
	Backup(destDir string) error
	// RunGC 周期空间回收（Pebble Compact / Badger RunValueLogGC）。R4c。
	RunGC() error
}
```

- [ ] **Step 4: Implement PebbleStore.Backup + RunGC**

In `internal/storage/kv/pebble_store.go`, add (after the existing `Compact` method):
```go
// Backup 创建一致性快照到 destDir（Pebble Checkpoint，在线、不停机）。
func (s *PebbleStore) Backup(destDir string) error {
	return s.db.Checkpoint(destDir)
}

// RunGC 触发全量压缩回收空间（满足 kv.Store 接口）。
func (s *PebbleStore) RunGC() error {
	return s.db.Compact(nil, nil, false)
}
```

- [ ] **Step 5: Implement BadgerStore.Backup (RunGC already exists at :92)**

In `internal/storage/kv/badger_store.go`, add a Backup method. The existing `RunGC` (line 92) already satisfies the interface; just ensure it's exported (it is). Add:
```go
// Backup 将数据库备份到 destDir/backup.bak（Badger 原生 backup 格式）。
func (s *BadgerStore) Backup(destDir string) error {
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	f, err := os.Create(filepath.Join(destDir, "backup.bak"))
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()
	_, err = s.db.Backup(f, 0)
	return err
}
```
Add `"os"`, `"path/filepath"`, `"fmt"` to imports if missing. **Verify the existing `RunGC`** loops `RunValueLogGC` until it returns < 1.0 (read lines 90-94; if it only calls once, wrap in a loop with a sane cap, e.g. 10 iterations):
```go
func (s *BadgerStore) RunGC() error {
	for i := 0; i < 10; i++ {
		n, err := s.db.RunValueLogGC(0.7)
		if err != nil {
			if err == badger.ErrNoRewrite { return nil }
			return err
		}
		if n < 1.0 { break }
	}
	return nil
}
```
(Use this form only if the current one doesn't loop; otherwise leave it. `badger.ErrNoRewrite` is the "nothing to GC" sentinel.)

- [ ] **Step 6: Implement MemoryStore.Backup + RunGC**

In `internal/storage/kv/memory_store.go`, add:
```go
// Backup dump 内存键值到 destDir/dump.json。
func (s *MemoryStore) Backup(destDir string) error {
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	dump := make(map[string]string, len(s.data))
	for k, v := range s.data {
		dump[k] = string(v)
	}
	b, err := json.MarshalIndent(dump, "", "  ")
	if err != nil { return err }
	return os.WriteFile(filepath.Join(destDir, "dump.json"), b, 0o600) //nolint:gosec // 备份目录内文件
}

// RunGC 内存存储无需回收。
func (s *MemoryStore) RunGC() error { return nil }
```
Add imports `"encoding/json"`, `"fmt"`, `"os"`, `"path/filepath"`.

- [ ] **Step 7: Implement JSONFileStore.Backup + RunGC**

In `internal/storage/kv/store.go` (JSONFileStore section), add:
```go
// Backup 复制当前 JSON 持久化文件到 destDir/。
func (s *JSONFileStore) Backup(destDir string) error {
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	in, err := os.ReadFile(s.filePath)
	if err != nil { return err }
	return os.WriteFile(filepath.Join(destDir, filepath.Base(s.filePath)), in, 0o600) //nolint:gosec // 备份目录内文件
}

// RunGC JSON 文件存储无需回收。
func (s *JSONFileStore) RunGC() error { return nil }
```
Ensure `"path/filepath"` is imported.

- [ ] **Step 8: Fix any other `kv.Store` fakes**

Run: `grep -rln "kv.Store\b" --include="*_test.go" .` and `go build ./...`. The compiler will error on any fake missing the two methods; add stub `Backup`/`RunGC` (Backup can `return errors.New("not implemented")`, RunGC `return nil`) to each test fake that's not meant to back up.

- [ ] **Step 9: Run tests + verify + commit**

Run the canonical verification block. Then:
```bash
git add internal/storage/kv/
git commit -m "refactor(kv-ops): add Backup + RunGC to kv.Store (Pebble Checkpoint / Badger Backup)

Pebble Backup=db.Checkpoint, RunGC=db.Compact; Badger Backup=db.Backup, RunGC
loops RunValueLogGC; Memory/JSONFile dump/no-op. Foundation for R4c backup/restore+GC.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: BackupService + manifest + config

**Files:**
- Create: `internal/core/backup/service.go`
- Test: `internal/core/backup/service_test.go`
- Modify: `pkg/config/config.go` (add `BackupDir` to StorageConfig + default)

**Interfaces:**
- Consumes: `kv.Store` (Backup), `storage.Store` (KVStore accessor), the known key prefixes.
- Produces: `backup.Service` with `NewService(kvStore kv.Store, backupDir, engine string)` + `CreateBackup(ctx) (*BackupResult, error)` + `ListBackups() ([]*BackupMeta, error)`. Types: `BackupResult{Dir, SizeBytes, KeyCount, CreatedAt}`, `BackupMeta{Dir, Engine, CreatedAt, SizeBytes, KeyCount}`.

- [ ] **Step 1: Add config field**

In `pkg/config/config.go`, extend `StorageConfig`:
```go
type StorageConfig struct {
	KVType     string `json:"kv_type"`
	SearchType string `json:"search_type"`
	BackupDir  string `json:"backup_dir"` // R4c：备份目录，默认 <DataDir>/backups（节点 main 兜底）
}
```

- [ ] **Step 2: Write the failing test**

Create `internal/core/backup/service_test.go`:
```go
package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/kv"
)

func newStore(t *testing.T) (*storage.Store, kv.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewPersistentStore(&storage.StoreConfig{
		KVType: "pebble", KVPath: filepath.Join(dir, "kv"),
		SearchType: "memory", SearchPath: filepath.Join(dir, "search"),
	})
	if err != nil { t.Fatalf("NewPersistentStore: %v", err) }
	t.Cleanup(func() { store.Close() })
	return store, store.KVStore()
}

func TestCreateBackup_WritesManifest(t *testing.T) {
	_, kvStore := newStore(t)
	kvStore.Put([]byte("entry:a"), []byte("1"))

	svc := NewService(kvStore, t.TempDir(), "pebble")
	res, err := svc.CreateBackup(context.Background())
	if err != nil { t.Fatalf("CreateBackup: %v", err) }
	if res.SizeBytes <= 0 { t.Error("SizeBytes should be > 0") }
	if res.KeyCount < 1 { t.Error("KeyCount should include entry:a") }
	if _, err := os.Stat(filepath.Join(res.Dir, "manifest.json")); err != nil {
		t.Errorf("manifest.json missing: %v", err)
	}
}

func TestListBackups_SortedDesc(t *testing.T) {
	_, kvStore := newStore(t)
	svc := NewService(kvStore, t.TempDir(), "pebble")
	r1, _ := svc.CreateBackup(context.Background())
	r2, _ := svc.CreateBackup(context.Background())
	list, err := svc.ListBackups()
	if err != nil { t.Fatalf("ListBackups: %v", err) }
	if len(list) != 2 { t.Fatalf("want 2 backups, got %d", len(list)) }
	if list[0].CreatedAt < list[1].CreatedAt {
		t.Error("ListBackups should be newest-first")
	}
	_ = r1; _ = r2
}
```
Note: confirm `storage.StoreConfig` field names (`KVType`/`KVPath`/`SearchType`/`SearchPath`) — they are at `internal/storage/store.go:161-170`. `SearchType: "memory"` uses the in-memory search engine (avoids CGO/bleve in tests).

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/core/backup/...`
Expected: FAIL — package doesn't exist.

- [ ] **Step 4: Implement the service**

Create `internal/core/backup/service.go`:
```go
package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/kv"
)

// BackupMeta is the on-disk manifest + listed-backup record.
type BackupMeta struct {
	Dir       string `json:"dir"`
	Engine    string `json:"engine"`
	CreatedAt int64  `json:"created_at"` // unix millis
	SizeBytes int64  `json:"size_bytes"`
	KeyCount  int64  `json:"key_count"`
}

// BackupResult is returned from CreateBackup.
type BackupResult struct {
	BackupMeta
}

// Service creates and lists raw-KV backups.
type Service struct {
	kvStore   kv.Store
	backupDir string
	engine    string
}

// NewService creates a backup service. backupDir is created on first CreateBackup.
func NewService(kvStore kv.Store, backupDir, engine string) *Service {
	return &Service{kvStore: kvStore, backupDir: backupDir, engine: engine}
}

// CreateBackup writes a consistent snapshot to <backupDir>/<unix-ms>/ + a manifest.
func (s *Service) CreateBackup(ctx context.Context) (*BackupResult, error) {
	ts := time.Now().UnixMilli()
	dir := filepath.Join(s.backupDir, fmt.Sprintf("%d", ts))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}
	if err := s.kvStore.Backup(dir); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("kv backup: %w", err)
	}
	size := dirSize(dir)
	count := s.estimateKeyCount()
	meta := BackupMeta{Dir: dir, Engine: s.engine, CreatedAt: ts, SizeBytes: size, KeyCount: count}
	mb, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), mb, 0o600); err != nil { //nolint:gosec // 备份目录内
		return nil, fmt.Errorf("write manifest: %w", err)
	}
	return &BackupResult{BackupMeta: meta}, nil
}

// ListBackups scans <backupDir>/*/manifest.json, newest-first.
func (s *Service) ListBackups() ([]*BackupMeta, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		if os.IsNotExist(err) { return []*BackupMeta{}, nil }
		return nil, err
	}
	var out []*BackupMeta
	for _, e := range entries {
		if !e.IsDir() { continue }
		mpath := filepath.Join(s.backupDir, e.Name(), "manifest.json")
		b, err := os.ReadFile(mpath)
		if err != nil { continue }
		var m BackupMeta
		if json.Unmarshal(b, &m) == nil {
			out = append(out, &m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() { total += info.Size() }
		return nil
	})
	return total
}

// estimateKeyCount sums Scan results across the known key prefixes.
func (s *Service) estimateKeyCount() int64 {
	var total int64
	for _, p := range []string{
		kv.EntryPrefix, kv.UserPrefix, kv.UserEmailPrefix, kv.UserHashPrefix,
		kv.RatingPrefix, kv.RatingByRaterPrefix, kv.CategoryPrefix,
		kv.NodePrefix, kv.MetaPrefix,
	} {
		m, err := s.kvStore.Scan([]byte(p))
		if err == nil { total += int64(len(m)) }
	}
	return total
}

// Reference storage so the import is intentional for future migration helpers.
var _ = storage.ErrKeyNotFound
```
**Verify the prefix constant names** in `internal/storage/kv/store.go:14-24` — they may be `kv.EntryPrefix` / `kv.PrefixEntry` etc. Read that block and use the exact exported names; if they are unexported string literals, use the literal strings (`"entry:"`, etc.) instead. Also confirm `storage.ErrKeyNotFound` exists or drop that line.

- [ ] **Step 5: Run tests + verify + commit**

Canonical verification block. Then:
```bash
git add internal/core/backup/ pkg/config/config.go
git commit -m "feat(kv-ops): BackupService (manifest + list) + BackupDir config

CreateBackup writes <backupDir>/<ts>/ via kvStore.Backup + manifest.json
(engine/ts/size/keyCount). ListBackups scans manifests newest-first.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Admin backup endpoints + SPA trigger

**Files:**
- Create: `internal/api/handler/backup_handler.go`
- Modify: `internal/api/admin/handler.go` (add backupHandler field + delegation + extend NewHandler)
- Modify: `internal/api/router/router.go` (registerAdminRoutes: 2 routes + pass backupDir)
- Modify: `cmd/seed/main.go` + `cmd/user/main.go` (pass backupDir through Deps/router)
- Create: `web/admin/src/api/backup.js`
- Modify: `web/admin/src/views/stats/Index.vue` (a backup panel: button + list)
- Test: `internal/api/handler/backup_handler_test.go`

**Interfaces:**
- Consumes: `backup.Service` (Task 2), `adminAuthMW`.
- Produces: `POST /api/v1/admin/backup` → `{dir,size_bytes,key_count,created_at}`; `GET /api/v1/admin/backups` → `{backups:[...]}`.

- [ ] **Step 1: Write the failing handler test**

Create `internal/api/handler/backup_handler_test.go`:
```go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/core/backup"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/kv"
)

func newBackupHandler(t *testing.T) (*BackupHandler, kv.Store) {
	t.Helper()
	dir := t.TempDir()
	store, _ := storage.NewPersistentStore(&storage.StoreConfig{
		KVType: "pebble", KVPath: dir + "/kv", SearchType: "memory", SearchPath: dir + "/s",
	})
	t.Cleanup(func() { store.Close() })
	svc := backup.NewService(store.KVStore(), t.TempDir(), "pebble")
	return NewBackupHandler(svc), store.KVStore()
}

func TestBackupHandler_Create(t *testing.T) {
	h, kvStore := newBackupHandler(t)
	kvStore.Put([]byte("entry:a"), []byte("1"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/backup", nil)
	req = req.WithContext(context.Background())
	rec := httptest.NewRecorder()
	h.CreateBackupHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBackupHandler_List(t *testing.T) {
	h, kvStore := newBackupHandler(t)
	kvStore.Put([]byte("entry:a"), []byte("1"))
	h.svc.CreateBackup(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups", nil)
	rec := httptest.NewRecorder()
	h.ListBackupsHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestBackupHandler ./internal/api/handler/...`
Expected: FAIL — `BackupHandler`/`NewBackupHandler` undefined.

- [ ] **Step 3: Implement the handler**

Create `internal/api/handler/backup_handler.go`:
```go
package handler

import (
	"net/http"

	"github.com/daifei0527/polyant/internal/core/backup"
)

// BackupHandler exposes the KV backup admin endpoints.
type BackupHandler struct {
	svc *backup.Service
}

func NewBackupHandler(svc *backup.Service) *BackupHandler {
	return &BackupHandler{svc: svc}
}

// CreateBackupHandler  POST /api/v1/admin/backup
func (h *BackupHandler) CreateBackupHandler(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.CreateBackup(r.Context())
	if err != nil {
		writeError(w, awerrors.Wrap(320, awerrors.CategoryStorage, "backup failed", 500, err))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"dir": res.Dir, "size_bytes": res.SizeBytes, "key_count": res.KeyCount, "created_at": res.CreatedAt,
	}})
}

// ListBackupsHandler  GET /api/v1/admin/backups
func (h *BackupHandler) ListBackupsHandler(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListBackups()
	if err != nil {
		writeError(w, awerrors.Wrap(321, awerrors.CategoryStorage, "list backups failed", 500, err))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"backups": list,
	}})
}
```
**Verify helpers** (`writeError`/`writeJSON`/`APIResponse`/`awerrors.Wrap`) exist in the `handler` package — they do (used by entry/admin handlers). Reuse them.

- [ ] **Step 4: Run handler tests + verify + commit (backend)**

Canonical verification block. Commit:
```bash
git add internal/api/handler/backup_handler.go internal/api/handler/backup_handler_test.go
git commit -m "feat(kv-ops): backup admin handler (create + list)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

- [ ] **Step 5: Wire into admin.Handler + router + node Deps**

(a) `internal/api/admin/handler.go`: add a `backupHandler` to the struct + extend `NewHandler` to take `backupDir, engine string`:
```go
type Handler struct {
	adminHandler  *handler.AdminHandler
	statsHandler  *handler.StatsHandler
	reviewHandler *handler.ReviewHandler
	backupHandler *handler.BackupHandler
}

func NewHandler(store *storage.Store, entryPusher handler.EntryPusher, backupDir, engine string) *Handler {
	backupSvc := backup.NewService(store.KVStore(), backupDir, engine)
	return &Handler{
		adminHandler:  handler.NewAdminHandler(store),
		statsHandler:  handler.NewStatsHandler(store),
		reviewHandler: handler.NewReviewHandler(review.NewService(store, entryPusher)),
		backupHandler: handler.NewBackupHandler(backupSvc),
	}
}
func (h *Handler) CreateBackupHandler(w http.ResponseWriter, r *http.Request) { h.backupHandler.CreateBackupHandler(w, r) }
func (h *Handler) ListBackupsHandler(w http.ResponseWriter, r *http.Request)  { h.backupHandler.ListBackupsHandler(w, r) }
```
(Adjust the review-handler construction to match the current R4b code — it may already build `reviewSvc` inline; keep that as-is and just add the backup field.)

(b) `internal/api/router/router.go` `registerAdminRoutes`: the `NewHandler` call site (was `admin.NewHandler(deps.Store, deps.EntryPusher)`) → add `deps.BackupDir, deps.KVType` (or read from a new Deps field). Add the Deps fields `BackupDir string` + `KVType string` to the `Dependencies` struct. Register routes:
```go
mux.Handle("/api/v1/admin/backup",
	adminAuthMW.Middleware(http.HandlerFunc(adminHandler.CreateBackupHandler)))
mux.Handle("/api/v1/admin/backups",
	adminAuthMW.Middleware(http.HandlerFunc(adminHandler.ListBackupsHandler)))
```

(c) `cmd/seed/main.go` + `cmd/user/main.go`: when building `Dependencies`, pass `BackupDir` (= `cfg.Storage.BackupDir` or default `dataDir+"/backups"`) and `KVType` (= `cfg.Storage.KVType`). Grep `Dependencies{` in both mains and add the two fields.

- [ ] **Step 6: Verify backend wiring**

Canonical verification block (the build catches any missed call-site). Commit:
```bash
git add internal/api/admin/handler.go internal/api/router/router.go cmd/seed/main.go cmd/user/main.go
git commit -m "feat(kv-ops): wire backup endpoints into admin routes (seed + user)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

- [ ] **Step 7: SPA backup panel**

Create `web/admin/src/api/backup.js` (mirror `api/users.js`):
```js
import request from './request'

export function createBackup() {
  return request.post('/admin/backup')
}
export function listBackups() {
  return request.get('/admin/backups')
}
```
In `web/admin/src/views/stats/Index.vue`, add a small backup card (import `createBackup, listBackups`; a button that calls `createBackup()` then refreshes a `backups` list; render the list with `el-table` or `<ul>`). Mirror the existing `fetchData`/`ElMessage.success` patterns. Keep it minimal — a card with a "创建备份" button + a list of `{dir, size_bytes, created_at}`.

- [ ] **Step 8: Build SPA + sync dist + commit**

```bash
cd web/admin && npm run build && cd -
rm -rf internal/api/admin/dist && cp -r web/admin/dist internal/api/admin/dist
```
Then canonical Go verification. Commit:
```bash
git add web/admin/src/ internal/api/admin/dist/
git commit -m "feat(kv-ops): admin SPA backup panel (create + list)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: GarbageCollector background task + node wiring

**Files:**
- Create: `internal/core/storage/gc.go`
- Test: `internal/core/storage/gc_test.go`
- Modify: `pkg/config/config.go` (add `GCIntervalS` to StorageConfig + default)
- Modify: `cmd/seed/main.go` (struct field + Start + Stop)
- Modify: `cmd/user/main.go` (struct field + Start + Stop — first bg task here)

**Interfaces:**
- Consumes: `kv.Store` (RunGC).
- Produces: `gc.GarbageCollector` with `NewGarbageCollector(kvStore kv.Store, interval time.Duration)` + `Start(ctx) error` + `Stop() error`.

- [ ] **Step 1: Add config field**

In `pkg/config/config.go` `StorageConfig`, add:
```go
GCIntervalS int    `json:"gc_interval_s"` // R4c：GC 间隔秒，默认 3600；<=0 禁用
```
And in `DefaultConfig()` Storage block set `GCIntervalS: 3600`.

- [ ] **Step 2: Write the failing test**

Create `internal/core/storage/gc_test.go`:
```go
package storage

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/storage/kv"
)

// countingStore wraps a real PebbleStore to count RunGC calls.
type countingStore struct {
	kv.Store
	gcCalls int32
}

func (c *countingStore) RunGC() error { atomic.AddInt32(&c.gcCalls, 1); return c.Store.RunGC() }

func TestGarbageCollector_RunsOnInterval(t *testing.T) {
	real, err := kv.NewPebbleStore(t.TempDir())
	if err != nil { t.Fatalf("NewPebbleStore: %v", err) }
	defer real.Close()
	cs := &countingStore{Store: real}

	gc := NewGarbageCollector(cs, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := gc.Start(ctx); err != nil { t.Fatalf("Start: %v", err) }
	defer gc.Stop()

	time.Sleep(180 * time.Millisecond) // ~3-4 ticks
	if got := atomic.LoadInt32(&cs.gcCalls); got < 2 {
		t.Errorf("RunGC called %d times, want >=2", got)
	}
}

func TestGarbageCollector_StopCancels(t *testing.T) {
	real, _ := kv.NewPebbleStore(t.TempDir())
	defer real.Close()
	gc := NewGarbageCollector(real, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gc.Start(ctx)
	done := make(chan struct{})
	go func() { gc.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return within 2s")
	}
}
```
Note: `countingStore` embeds `kv.Store` and overrides only `RunGC` — it satisfies the interface. The package is `storage` (internal/core/storage); ensure no name clash with `internal/storage`.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/core/storage/...`
Expected: FAIL — package/gc.go doesn't exist.

- [ ] **Step 4: Implement the GC**

Create `internal/core/storage/gc.go`:
```go
package storage

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/storage/kv"
)

// GarbageCollector periodically calls kv.Store.RunGC to reclaim space.
type GarbageCollector struct {
	kvStore  kv.Store
	interval time.Duration
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewGarbageCollector returns a GC runner. interval <=0 disables (Start is a no-op).
func NewGarbageCollector(kvStore kv.Store, interval time.Duration) *GarbageCollector {
	return &GarbageCollector{kvStore: kvStore, interval: interval}
}

func (g *GarbageCollector) Start(ctx context.Context) error {
	if g.interval <= 0 {
		log.Printf("[GarbageCollector] disabled (interval <= 0)")
		return nil
	}
	ctx, g.cancel = context.WithCancel(ctx)
	g.wg.Add(1)
	go g.loop(ctx)
	log.Printf("[GarbageCollector] started, interval: %v", g.interval)
	return nil
}

func (g *GarbageCollector) Stop() error {
	if g.cancel != nil { g.cancel() }
	g.wg.Wait()
	return nil
}

func (g *GarbageCollector) loop(ctx context.Context) {
	defer g.wg.Done()
	g.runOnce() // run on startup
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.runOnce()
		}
	}
}

func (g *GarbageCollector) runOnce() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[GarbageCollector] panic during GC, aborted this cycle: %v", r)
		}
	}()
	if err := g.kvStore.RunGC(); err != nil {
		log.Printf("[GarbageCollector] RunGC error: %v", err)
	}
}
```

- [ ] **Step 5: Run tests + verify + commit (component)**

Canonical verification block. Commit:
```bash
git add internal/core/storage/ pkg/config/config.go
git commit -m "feat(kv-ops): GarbageCollector background task + GCIntervalS config

Mirrors IntegrityChecker (ticker + select + per-cycle recover). interval<=0 disables.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

- [ ] **Step 6: Wire into seed node**

In `cmd/seed/main.go`: add import `"github.com/daifei0527/polyant/internal/core/storage"` (alias if it clashes with `"internal/storage"` — check existing imports; the core package is `internal/core/storage`, the data package is `internal/storage`; use alias `gcstorage "github.com/.../internal/core/storage"` if needed). Add field `garbageCollector *gcstorage.GarbageCollector` to `SeedApp`. In `Start()` (after `integrityChecker`):
```go
gcInterval := time.Duration(cfg.Storage.GCIntervalS) * time.Second
app.garbageCollector = gcstorage.NewGarbageCollector(app.store.KVStore(), gcInterval)
if err := app.garbageCollector.Start(ctx); err != nil {
	app.logger.Warn("Garbage collector start failed", zap.Error(err))
} else {
	app.logger.Info("Garbage collector started")
}
```
In the `Stop()` block (alongside the other checkers, BEFORE `cleanup()`):
```go
if app.garbageCollector != nil {
	if err := app.garbageCollector.Stop(); err != nil {
		app.logger.Warn("garbageCollector stop failed", zap.Error(err))
	}
}
```
**Resolve the config source for `GCIntervalS`**: `cfg` is the loaded `*config.Config` available in `Start()` (the seed main loads it). Read `cfg.Storage.GCIntervalS`.

- [ ] **Step 7: Wire into user node (first bg task here)**

In `cmd/user/main.go`: mirror seed — add the `garbageCollector` field, construct+`Start` in `Start()` (the user `Start()` currently has no bg tasks; add the GC block), and add the `Stop()` clause in `Stop()` before `cleanup()`.

- [ ] **Step 8: Verify node builds + runs**

```bash
go build ./cmd/seed ./cmd/user
```
Canonical verification block. Commit:
```bash
git add cmd/seed/main.go cmd/user/main.go
git commit -m "feat(kv-ops): wire periodic GC into seed + user nodes

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: pactl offline backup list + engine-direct restore

**Files:**
- Create: `cmd/pactl/backup.go`
- Modify: `cmd/pactl/main.go` (consume `--config` for backup subcommands; register `backupCmd`)

**Interfaces:**
- Consumes: `config.Load` (resolve KVType/KVPath), `kv.NewPebbleStore`/`NewBadgerStore` (engine-direct restore), `badger.Load` (Badger restore).
- Produces: `pactl backup list` (reads `<DataDir>/backups/*/manifest.json` from filesystem) + `pactl backup restore <dir>` (offline engine-direct: Pebble swap dir, Badger Load).

- [ ] **Step 1: Implement `backup list` + `backup restore`**

Create `cmd/pactl/backup.go`:
```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
	"github.com/spf13/cobra"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/pkg/config"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "KV 备份管理（离线；restore 需先停节点）",
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出本地备份（读 <DataDir>/backups）",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, dataDir, err := resolveKVMeta()
		if err != nil { return err }
		backupDir := filepath.Join(dataDir, "backups")
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			if os.IsNotExist(err) { fmt.Println("(无备份)"); return nil }
			return err
		}
		for _, e := range entries {
			if !e.IsDir() { continue }
			b, err := os.ReadFile(filepath.Join(backupDir, e.Name(), "manifest.json"))
			if err != nil { continue }
			var m struct {
				Engine    string `json:"engine"`
				CreatedAt int64  `json:"created_at"`
				SizeBytes int64  `json:"size_bytes"`
				KeyCount  int64  `json:"key_count"`
			}
			if json.Unmarshal(b, &m) == nil {
				fmt.Printf("%s  engine=%s  size=%d  keys=%d\n", e.Name(), m.Engine, m.SizeBytes, m.KeyCount)
			}
		}
		return nil
	},
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore <backup-dir>",
	Short: "从备份目录离线恢复 KV（必须先停节点）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		srcDir := args[0]
		kvType, dataDir, err := resolveKVMeta()
		if err != nil { return err }
		kvPath := filepath.Join(dataDir, "kv")
		if err := assertNodeStopped(kvPath, kvType); err != nil { return err }

		switch kvType {
		case "badger":
			return restoreBadger(kvPath, srcDir)
		default: // pebble
			return restorePebble(kvPath, srcDir)
		}
	},
}

func resolveKVMeta() (kvType, dataDir string, err error) {
	cfg, err := config.Load(configPath) // configPath is the global from --config
	if err != nil { return "", "", fmt.Errorf("load config: %w", err) }
	if cfg.Node.DataDir == "" { cfg.Node.DataDir = "./data" }
	return cfg.Storage.KVType, cfg.Node.DataDir, nil
}

// assertNodeStopped tries to open the KV read-only; if it's locked the node is running.
func assertNodeStopped(kvPath, kvType string) error {
	switch kvType {
	case "badger":
		db, err := badger.Open(badger.DefaultOptions(kvPath).WithReadOnly(true))
		if err != nil { return fmt.Errorf("无法打开 KV（节点可能正在运行，先停节点）: %w", err) }
		db.Close()
	default:
		s, err := kv.NewPebbleStore(kvPath)
		if err != nil { return fmt.Errorf("无法打开 KV（节点可能正在运行，先停节点）: %w", err) }
		s.Close()
	}
	return nil
}

func restorePebble(kvPath, srcDir string) error {
	// backup dir from Pebble Checkpoint IS a valid Pebble DB dir; swap.
	tmp := kvPath + ".old"
	if err := os.Rename(kvPath, tmp); err != nil { return fmt.Errorf("移开旧 KV: %w", err) }
	if err := os.Rename(srcDir, kvPath); err != nil {
		os.Rename(tmp, kvPath) // best-effort rollback
		return fmt.Errorf("移入备份: %w", err)
	}
	os.RemoveAll(tmp)
	fmt.Printf("Pebble 恢复完成: %s -> %s\n请重启节点。\n", srcDir, kvPath)
	return nil
}

func restoreBadger(kvPath, srcDir string) error {
	db, err := badger.Open(badger.DefaultOptions(kvPath))
	if err != nil { return fmt.Errorf("open KV for Load: %w", err) }
	defer db.Close()
	f, err := os.Open(filepath.Join(srcDir, "backup.bak"))
	if err != nil { return fmt.Errorf("open backup.bak: %w", err) }
	defer f.Close()
	if err := db.Load(f, 256); err != nil { return fmt.Errorf("badger Load: %w", err) }
	fmt.Printf("Badger 恢复完成: 从 %s 载入 %s\n请重启节点。\n", srcDir, kvPath)
	return nil
}

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.AddCommand(backupListCmd, backupRestoreCmd)
}
```
**Verify**: `configPath` is the global var bound to `--config` in `cmd/pactl/main.go:147` (read its exact name — could be `configPath`). `config.Load(path)` exists (`pkg/config/config.go:254`). Pebble restore swaps dirs (checkpoint is a valid DB); Badger Load merges into the existing KV (if you need a clean restore, `os.RemoveAll(kvPath)` before `badger.Open` — but that loses existing data; for MVP, Load merges, which is the standard Badger restore).

- [ ] **Step 2: Build + smoke-test pactl**

```bash
go build ./cmd/pactl
./bin/pactl backup list --config configs/seed.json   # or wherever a config exists
./bin/pactl backup --help
```
Expected: `list` prints (no error if no backups); `--help` shows `list` + `restore`.

- [ ] **Step 3: Verify + commit**

Canonical verification block (the new file is `cmd/pactl/`; `go vet`/lint apply). Commit:
```bash
git add cmd/pactl/backup.go
git commit -m "feat(kv-ops): pactl backup list (offline) + restore (engine-direct)

backup list reads <DataDir>/backups/*/manifest.json. backup restore <dir>
opens the engine directly (node must be stopped): Pebble swaps the checkpoint
dir in; Badger Load. --config resolves KVType/KVPath.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Final verification (after all 5 tasks)

- [ ] `gofmt -l` repo-wide empty; `go build ./cmd/... ./internal/... ./pkg/...` OK; `go vet ./...` OK.
- [ ] `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` all PASS.
- [ ] `golangci-lint run ./...` exit 0.
- [ ] `cd web/admin && npm run build` succeeds; `internal/api/admin/dist/index.html` exists.
- [ ] `./bin/pactl backup --help` shows list + restore.
- [ ] Manual smoke (optional): trigger `POST /api/v1/admin/backup` (or SPA button) → backup dir created with manifest; `pactl backup list` shows it; (with node stopped) `pactl backup restore <dir>` swaps KV.
- [ ] Branch `r4c-kv-backup` has 5 task commits + spec/plan, ready for review/merge.
