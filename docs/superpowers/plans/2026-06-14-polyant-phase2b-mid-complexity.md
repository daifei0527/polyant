# Polyant Phase 2B (mid) — 投票原子化 + isLocalRequest 配置化 + 清理

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development / executing-plans. Steps use `- [ ]`.

**Goal:** Phase 2 的 4 项中等复杂度修复：P2.3 投票计数原子化 + HasVoted 错误传播、P2.7 isLocalRequest 配置化、P2.8 level_checker 返回当前等级、P2.8 删 SimpleSearchEngine 死代码。

**Verified root causes (master `a8718a7`):**
- **P2.3a** `KVVoteStore.HasVoted` (vote_store.go:99): KV `Get` 出错时 `return false, nil`——吞错误当"未投"，KV 故障时允许重复投票。
- **P2.3b** `KVCandidateStore.UpdateVoteCount` (election_store.go:145): Get-then-Add-Put 非原子，并发投票丢票。
- **P2.7** `isLocalRequest` (admin/session.go:86): 硬编码 `127.0.0.1:18531`/`localhost:18531`，且 `r.RemoteAddr == "127.0.0.1"` 几乎永不匹配（RemoteAddr 实为 `ip:port`）。换端口/反代即失效。
- **P2.8a** `checkUpgrade` (level_checker.go:173): 未升级时 `return newLevel, upgraded`（newLevel=0），应返回 `user.UserLevel`。
- **P2.8b** `SimpleSearchEngine` (index/search.go): 死代码（`store.go` 用 `MemorySearchEngine`，无任何调用方）。

**Deferred:** P2.8 Windows `processExists`（需 syscall + 交叉编译验证）、选举 spec、env 文档对齐、P2.1/P2.2/P2.4/P2.5 大项。

**Gate:** `go build ./... && go vet ./... && go test ./cmd/... ./internal/... ./pkg/...`

---

## Task 1: P2.3 — HasVoted 错误传播 + UpdateVoteCount 原子化

**Files:** `internal/storage/kv/vote_store.go`, `internal/storage/kv/election_store.go`, tests.

- [ ] **Step 1 (HasVoted):** vote_store.go `HasVoted` 区分 `ErrKeyNotFound` 与其他错误：
  ```go
  func (s *KVVoteStore) HasVoted(ctx context.Context, voterID, electionID string) (bool, error) {
      indexKey := []byte(fmt.Sprintf("%s%s:%s", votesByVoterKey, electionID, voterID))
      _, err := s.kv.Get(indexKey)
      if err != nil {
          if err == ErrKeyNotFound {
              return false, nil // 确实没投过
          }
          return false, fmt.Errorf("check voted: %w", err) // 存储 error 必须传播，不能当"未投"
      }
      return true, nil
  }
  ```
  （确认 `ErrKeyNotFound` 是该包的哨兵 error——election_store.go 已用它。）

- [ ] **Step 2 (UpdateVoteCount 原子化):** election_store.go 给 `KVCandidateStore` 加 per-election 互斥锁：
  ```go
  type KVCandidateStore struct {
      kv    Store
      mu    sync.Map // electionID -> *sync.Mutex
  }
  ```
  加 helper：
  ```go
  func (s *KVCandidateStore) lockFor(electionID string) *sync.Mutex {
      actual, _ := s.mu.LoadOrStore(electionID, &sync.Mutex{})
      return actual.(*sync.Mutex)
  }
  ```
  `UpdateVoteCount` 加锁包裹 Get-Add-Put：
  ```go
  func (s *KVCandidateStore) UpdateVoteCount(ctx context.Context, electionID, userID string, delta int32) error {
      s.lockFor(electionID).Lock()
      defer s.lockFor(electionID).Unlock()
      candidate, err := s.Get(ctx, electionID, userID)
      if err != nil {
          return err
      }
      candidate.VoteCount += delta
      return s.Add(ctx, candidate)
  }
  ```
  （`UpdateStatus` 同理可选加锁；至少 `UpdateVoteCount` 必须原子。）加 `"sync"` import。

- [ ] **Step 3 (tests):** 加并发测试：N goroutine 各 `UpdateVoteCount(+1)` 同一候选人，断言最终 VoteCount == N（无丢票）。加 HasVoted 存储错误传播测试（注入返回非 ErrKeyNotFound 的 store，断言 HasVoted 返回 error 而非 false,nil）。

- [ ] **Step 4:** build/vet/test `./internal/storage/kv/...`，commit `fix(election): atomic vote counting + propagate HasVoted errors`.

## Task 2: P2.7 — isLocalRequest 配置化 + RemoteAddr 正确解析

**Files:** `internal/api/admin/session.go`, `internal/api/admin/middleware.go`, `internal/api/router/router.go`, tests.

- [ ] **Step 1:** admin 包加配置注入（避免全局状态）：`SessionHandler` 加 `localHost string` 字段；`NewSessionHandler(sessionMgr, userStore, localHost string)`。`isLocalRequest(r *http.Request, localHost string) bool` 用 `net.SplitHostPort` 正确解析 RemoteAddr 的 IP，比对 127.0.0.1/::1，并以 localHost 的端口做 Host 辅助检查。

- [ ] **Step 2:** `LocalOnlyMiddleware(next http.Handler, localHost string) http.Handler` 加参数；`CreateSessionHandler` 用 `h.localHost`。

- [ ] **Step 3:** router `Dependencies` 加 `AdminListenAddr string`；`registerAdminRoutes` 传 `deps.AdminListenAddr` 给 `admin.NewSessionHandler` 与 `admin.LocalOnlyMiddleware`；`NewRouter` 设 `AdminListenAddr: cfg.Admin.Listen`。

- [ ] **Step 4:** 测试：自定义端口下 isLocalRequest 正确判定；Host 伪造但 RemoteAddr 非本地应拒绝。

- [ ] **Step 5:** build/vet/test，commit `fix(admin): configurable isLocalRequest + correct RemoteAddr parsing`.

## Task 3: P2.8 — level_checker 返回当前等级

**Files:** `internal/core/user/level_checker.go`, test.

- [ ] **Step 1:** level_checker.go 末尾 `return newLevel, upgraded` → `return user.UserLevel, upgraded`（升级时 user.UserLevel 已=newLevel；未升级时=原等级，不再返回 0）。

- [ ] **Step 2:** 加测试：Lv1 用户不满足升级条件时 `checkUpgrade` 返回 `(Lv1, false)` 而非 `(0, false)`。

- [ ] **Step 3:** build/vet/test，commit `fix(level-checker): return current level when not upgrading`.

## Task 4: P2.8 — 删 SimpleSearchEngine 死代码

**Files:** `internal/storage/index/search.go`.

- [ ] **Step 1:** 读 search.go 全文，确认 `SimpleSearchEngine` 类型+方法+`NewSimpleSearchEngine`+其专用的内部接口定义均为死代码（`store.go` 用 `MemorySearchEngine`）。删除这些，保留文件里其他仍被使用的定义（如 `SearchEngine` 接口若在此）。

- [ ] **Step 2:** `grep -rn "SimpleSearchEngine"` → 空；`go build ./...` 通过。

- [ ] **Step 3:** commit `refactor(index): remove dead SimpleSearchEngine`.

## Gate & Deferred
- [ ] build/vet/test 全绿。
- Deferred: Windows processExists（需 syscall+交叉编译）、选举 spec、env 文档、P2.1/P2.2/P2.4/P2.5。
