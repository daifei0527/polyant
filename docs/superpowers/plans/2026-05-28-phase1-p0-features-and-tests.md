# Phase 1: P0 Features + Test Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete P0 features (email verification flow, API auth middleware) and improve test coverage for seed/user/pactl commands to 70%+.

**Architecture:** Three parallel workstreams: (1) email verification uses VerificationManager with expiration and stores codes in memory, (2) auth middleware adds replay protection via timestamp+nonce tracking, (3) command tests use interface mocking to avoid real P2P/network dependencies.

**Tech Stack:** Go 1.22+, testify, httptest, ed25519, sync.Map

---

## File Structure

### Email Verification

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 修改 | `internal/core/email/verification.go` | 添加 nonce 存储和过期验证码清理 |
| 修改 | `internal/api/handler/user_handler.go` | 完善 VerifyEmailHandler 逻辑 |
| 修改 | `internal/api/handler/types.go` | 添加 VerifyEmailRequest 类型（如缺失） |
| 修改 | `internal/storage/store.go` | 添加 GetByEmail 到 UserStore 接口（如缺失） |
| 修改 | `internal/storage/memory.go` | 实现 GetByEmail |
| 修改 | `internal/storage/kv/badger_store.go` | 实现 GetByEmail |
| 创建 | `internal/api/handler/verify_email_test.go` | 邮箱验证端点测试 |

### API Auth Middleware

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 修改 | `internal/api/middleware/auth.go` | 添加重放攻击防护（nonce + timestamp） |
| 修改 | `internal/api/middleware/auth_test.go` | 添加重放攻击和边界测试 |

### Command Tests

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 创建 | `cmd/seed/main_test.go` | SeedApp 配置加载和初始化测试 |
| 创建 | `cmd/user/main_test.go` | UserApp 配置加载和初始化测试 |
| 创建 | `cmd/pactl/main_test.go` | pactl CLI 命令测试 |

---

## Task 1: Add GetByEmail to UserStore Interface

**Files:**
- Modify: `internal/storage/store.go:43-54`
- Modify: `internal/storage/memory.go`
- Modify: `internal/storage/kv/badger_store.go`

- [ ] **Step 1: Add GetByEmail to UserStore interface**

In `internal/storage/store.go`, add `GetByEmail` method to the `UserStore` interface:

```go
type UserStore interface {
    Create(ctx context.Context, user *model.User) (*model.User, error)
    Get(ctx context.Context, pubkeyHash string) (*model.User, error)
    GetByEmail(ctx context.Context, email string) (*model.User, error)
    Update(ctx context.Context, user *model.User) (*model.User, error)
    List(ctx context.Context, filter UserFilter) ([]*model.User, int64, error)
}
```

- [ ] **Step 2: Implement GetByEmail in MemoryUserStore**

In `internal/storage/memory.go`, add the implementation to `MemoryUserStore`:

```go
func (s *MemoryUserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, u := range s.users {
        if u.Email == email {
            return u, nil
        }
    }
    return nil, fmt.Errorf("user not found with email: %s", email)
}
```

- [ ] **Step 3: Implement GetByEmail in BadgerUserStore**

In `internal/storage/kv/badger_store.go`, add the implementation. This requires iterating over users since BadgerDB doesn't have a secondary index on email:

```go
func (s *BadgerUserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
    var found *model.User
    err := s.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchSize = 10
        it := txn.NewIterator(opts)
        defer it.Close()
        prefix := []byte("user:")
        for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
            item := it.Item()
            var user model.User
            if err := item.Value(func(val []byte) error {
                return json.Unmarshal(val, &user)
            }); err != nil {
                continue
            }
            if user.Email == email {
                found = &user
                return nil
            }
        }
        return nil
    })
    if err != nil {
        return nil, err
    }
    if found == nil {
        return nil, fmt.Errorf("user not found with email: %s", email)
    }
    return found, nil
}
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./...`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add internal/storage/store.go internal/storage/memory.go internal/storage/kv/badger_store.go
git commit -m "feat(storage): add GetByEmail to UserStore interface and implementations"
```

---

## Task 2: Complete Email Verification Flow

**Files:**
- Modify: `internal/api/handler/user_handler.go:191-270`
- Modify: `internal/api/handler/types.go`
- Create: `internal/api/handler/verify_email_test.go`

- [ ] **Step 1: Write failing tests for VerifyEmailHandler**

Create `internal/api/handler/verify_email_test.go`:

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

func newTestVerifyHandler(t *testing.T) (*UserHandler, *storage.Store) {
    t.Helper()
    store, err := storage.NewMemoryStore()
    require.NoError(t, err)

    vm := email.NewVerificationManager()
    handler := NewUserHandler(store, store.User, store.Entry, store.Rating, nil, vm)
    return handler, store
}

func TestVerifyEmailHandler_Success(t *testing.T) {
    handler, store := newTestVerifyHandler(t)
    user, _ := createTestUser(t, store, "verify-agent", model.UserLevelLv0)

    // Generate a verification code
    code := handler.verificationMgr.GenerateCode("test@example.com")

    body, _ := json.Marshal(map[string]string{
        "email": "test@example.com",
        "code":  code,
    })
    req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    ctx := setUserInContext(req.Context(), user)
    req = req.WithContext(ctx)
    rec := httptest.NewRecorder()

    handler.VerifyEmailHandler(rec, req)

    assert.Equal(t, http.StatusOK, rec.Result().StatusCode)

    var resp map[string]interface{}
    json.NewDecoder(rec.Body).Decode(&resp)
    assert.Equal(t, float64(0), resp["code"])

    // Verify user was upgraded
    data := resp["data"].(map[string]interface{})
    assert.Equal(t, float64(model.UserLevelLv1), data["user_level"])
    assert.Equal(t, true, data["email_verified"])
}

func TestVerifyEmailHandler_InvalidCode(t *testing.T) {
    handler, store := newTestVerifyHandler(t)
    user, _ := createTestUser(t, store, "verify-agent", model.UserLevelLv0)

    body, _ := json.Marshal(map[string]string{
        "email": "test@example.com",
        "code":  "wrong-code",
    })
    req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    ctx := setUserInContext(req.Context(), user)
    req = req.WithContext(ctx)
    rec := httptest.NewRecorder()

    handler.VerifyEmailHandler(rec, req)

    assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestVerifyEmailHandler_ExpiredCode(t *testing.T) {
    handler, store := newTestVerifyHandler(t)
    user, _ := createTestUser(t, store, "verify-agent", model.UserLevelLv0)

    // Generate code then invalidate it
    code := handler.verificationMgr.GenerateCode("test@example.com")
    handler.verificationMgr.Invalidate(code)

    body, _ := json.Marshal(map[string]string{
        "email": "test@example.com",
        "code":  code,
    })
    req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    ctx := setUserInContext(req.Context(), user)
    req = req.WithContext(ctx)
    rec := httptest.NewRecorder()

    handler.VerifyEmailHandler(rec, req)

    assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestVerifyEmailHandler_NoAuth(t *testing.T) {
    handler, _ := newTestVerifyHandler(t)

    body, _ := json.Marshal(map[string]string{
        "email": "test@example.com",
        "code":  "123456",
    })
    req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()

    handler.VerifyEmailHandler(rec, req)

    assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestVerifyEmailHandler_MissingFields(t *testing.T) {
    handler, store := newTestVerifyHandler(t)
    user, _ := createTestUser(t, store, "verify-agent", model.UserLevelLv0)

    body, _ := json.Marshal(map[string]string{
        "email": "",
        "code":  "",
    })
    req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    ctx := setUserInContext(req.Context(), user)
    req = req.WithContext(ctx)
    rec := httptest.NewRecorder()

    handler.VerifyEmailHandler(rec, req)

    assert.Equal(t, http.StatusBadRequest, rec.Result().StatusCode)
}

func TestVerifyEmailHandler_EmailAlreadyUsed(t *testing.T) {
    handler, store := newTestVerifyHandler(t)

    // Create first user with verified email
    user1, _ := createTestUser(t, store, "agent-1", model.UserLevelLv1)
    user1.Email = "taken@example.com"
    user1.EmailVerified = true
    store.User.Update(context.Background(), user1)

    // Create second user trying to verify same email
    user2, _ := createTestUser(t, store, "agent-2", model.UserLevelLv0)
    code := handler.verificationMgr.GenerateCode("taken@example.com")

    body, _ := json.Marshal(map[string]string{
        "email": "taken@example.com",
        "code":  code,
    })
    req := httptest.NewRequest(http.MethodPost, "/api/v1/user/verify-email", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    ctx := setUserInContext(req.Context(), user2)
    req = req.WithContext(ctx)
    rec := httptest.NewRecorder()

    handler.VerifyEmailHandler(rec, req)

    assert.Equal(t, http.StatusConflict, rec.Result().StatusCode)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run TestVerifyEmail ./internal/api/handler/`
Expected: Compilation errors or test failures (VerifyEmailHandler needs fixes)

- [ ] **Step 3: Fix VerifyEmailHandler implementation**

Update `internal/api/handler/user_handler.go` VerifyEmailHandler to:

1. Use `GetByEmail` to check if email is already taken by another user
2. Properly handle the case where user is nil from context
3. Return appropriate error codes

```go
func (h *UserHandler) VerifyEmailHandler(w http.ResponseWriter, r *http.Request) {
    var req VerifyEmailRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, awerrors.ErrJSONParse)
        return
    }

    req.Email = strings.TrimSpace(strings.ToLower(req.Email))
    if req.Email == "" || req.Code == "" {
        writeError(w, awerrors.ErrInvalidParams)
        return
    }

    user := getUserFromContext(r.Context())
    if user == nil {
        writeError(w, awerrors.ErrMissingAuth)
        return
    }

    // Verify the code
    if h.verificationMgr == nil {
        writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, "verification service unavailable", 500, nil))
        return
    }

    if !h.verificationMgr.Verify(req.Code, req.Email) {
        writeError(w, awerrors.ErrInvalidEmailToken)
        return
    }

    // Check if email is already used by another user
    existing, _ := h.userStore.GetByEmail(r.Context(), req.Email)
    if existing != nil && existing.PublicKey != user.PublicKey {
        writeError(w, awerrors.New(409, awerrors.CategoryUser, "email already registered by another user", http.StatusConflict, nil))
        return
    }

    // Update user
    user.Email = req.Email
    user.EmailVerified = true
    if user.UserLevel < model.UserLevelLv1 {
        user.UserLevel = model.UserLevelLv1
    }
    user.LastActive = model.NowMillis()

    updated, err := h.userStore.Update(r.Context(), user)
    if err != nil {
        writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, "failed to update user", 500, err))
        return
    }

    if h.emailService != nil && updated.AgentName != "" {
        go h.emailService.SendWelcomeEmail(updated.Email, updated.AgentName)
    }

    writeJSON(w, http.StatusOK, &APIResponse{
        Code:    0,
        Message: "email verified, upgraded to verified user",
        Data: map[string]interface{}{
            "public_key":     updated.PublicKey,
            "user_level":     updated.UserLevel,
            "email":          updated.Email,
            "email_verified": updated.EmailVerified,
        },
    })
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run TestVerifyEmail ./internal/api/handler/`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/user_handler.go internal/api/handler/verify_email_test.go
git commit -m "feat(handler): complete email verification flow with duplicate email check"
```

---

## Task 3: Add Replay Attack Protection to Auth Middleware

**Files:**
- Modify: `internal/api/middleware/auth.go`
- Modify: `internal/api/middleware/auth_test.go`

- [ ] **Step 1: Write failing tests for replay attack protection**

Add to `internal/api/middleware/auth_test.go`:

```go
func TestAuthMiddleware_ReplayAttack(t *testing.T) {
    store, user, privKey := newTestUserStore(t)
    authMW := NewAuthMiddleware(store)

    testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    body := []byte(`{"test":"data"}`)
    timestamp := time.Now().UnixMilli()
    bodyHash := sha256.Sum256(body)
    signContent := fmt.Sprintf("POST\n/api/v1/test\n%d\n%s", timestamp, hex.EncodeToString(bodyHash[:]))
    signature := ed25519.Sign(privKey, []byte(signContent))

    // First request should succeed
    req1 := httptest.NewRequest(http.MethodPost, "/api/v1/test", bytes.NewBuffer(body))
    req1.Header.Set("X-Polyant-PublicKey", user.PublicKey)
    req1.Header.Set("X-Polyant-Timestamp", fmt.Sprintf("%d", timestamp))
    req1.Header.Set("X-Polyant-Signature", base64.StdEncoding.EncodeToString(signature))
    rec1 := httptest.NewRecorder()
    authMW.Middleware(testHandler).ServeHTTP(rec1, req1)
    assert.Equal(t, http.StatusOK, rec1.Result().StatusCode)

    // Replay request with same timestamp+body should be rejected
    req2 := httptest.NewRequest(http.MethodPost, "/api/v1/test", bytes.NewBuffer(body))
    req2.Header.Set("X-Polyant-PublicKey", user.PublicKey)
    req2.Header.Set("X-Polyant-Timestamp", fmt.Sprintf("%d", timestamp))
    req2.Header.Set("X-Polyant-Signature", base64.StdEncoding.EncodeToString(signature))
    rec2 := httptest.NewRecorder()
    authMW.Middleware(testHandler).ServeHTTP(rec2, req2)
    assert.Equal(t, http.StatusTooManyRequests, rec2.Result().StatusCode)
}

func TestAuthMiddleware_TimestampExpired(t *testing.T) {
    store, user, privKey := newTestUserStore(t)
    authMW := NewAuthMiddleware(store)

    testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    body := []byte(`{"test":"data"}`)
    expiredTimestamp := time.Now().Add(-10 * time.Minute).UnixMilli() // 10 minutes ago
    bodyHash := sha256.Sum256(body)
    signContent := fmt.Sprintf("POST\n/api/v1/test\n%d\n%s", expiredTimestamp, hex.EncodeToString(bodyHash[:]))
    signature := ed25519.Sign(privKey, []byte(signContent))

    req := httptest.NewRequest(http.MethodPost, "/api/v1/test", bytes.NewBuffer(body))
    req.Header.Set("X-Polyant-PublicKey", user.PublicKey)
    req.Header.Set("X-Polyant-Timestamp", fmt.Sprintf("%d", expiredTimestamp))
    req.Header.Set("X-Polyant-Signature", base64.StdEncoding.EncodeToString(signature))
    rec := httptest.NewRecorder()
    authMW.Middleware(testHandler).ServeHTTP(rec, req)
    assert.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestAuthMiddleware_SuspendedUser(t *testing.T) {
    store := storage.NewMemoryUserStore()

    pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
    pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

    user := &model.User{
        PublicKey: pubKeyB64,
        AgentName: "suspended-user",
        UserLevel: model.UserLevelLv1,
        Status:    model.UserStatusSuspended,
    }
    store.Create(context.Background(), user)

    authMW := NewAuthMiddleware(store)
    testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    body := []byte(`{}`)
    timestamp := time.Now().UnixMilli()
    bodyHash := sha256.Sum256(body)
    signContent := fmt.Sprintf("POST\n/api/v1/test\n%d\n%s", timestamp, hex.EncodeToString(bodyHash[:]))
    signature := ed25519.Sign(privKey, []byte(signContent))

    req := httptest.NewRequest(http.MethodPost, "/api/v1/test", bytes.NewBuffer(body))
    req.Header.Set("X-Polyant-PublicKey", pubKeyB64)
    req.Header.Set("X-Polyant-Timestamp", fmt.Sprintf("%d", timestamp))
    req.Header.Set("X-Polyant-Signature", base64.StdEncoding.EncodeToString(signature))
    rec := httptest.NewRecorder()
    authMW.Middleware(testHandler).ServeHTTP(rec, req)
    assert.Equal(t, http.StatusForbidden, rec.Result().StatusCode)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run TestAuthMiddleware_Replay ./internal/api/middleware/`
Expected: FAIL (replay protection not implemented)

- [ ] **Step 3: Add replay protection to AuthMiddleware**

In `internal/api/middleware/auth.go`, add nonce tracking:

```go
// Add to imports
import "sync"

// Add to AuthMiddleware struct
type AuthMiddleware struct {
    userStore    storage.UserStore
    seenRequests sync.Map // map[string]time.Time - tracks request signatures
}

// Add cleanup goroutine in constructor
func NewAuthMiddleware(userStore storage.UserStore) *AuthMiddleware {
    m := &AuthMiddleware{
        userStore: userStore,
    }
    go m.cleanupSeenRequests()
    return m
}

// Add cleanup method
func (m *AuthMiddleware) cleanupSeenRequests() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        now := time.Now()
        m.seenRequests.Range(func(key, value interface{}) bool {
            if t, ok := value.(time.Time); ok && now.Sub(t) > 10*time.Minute {
                m.seenRequests.Delete(key)
            }
            return true
        })
    }
}
```

In the `Middleware` method, after signature verification but before user lookup, add:

```go
// Generate request fingerprint for replay detection
reqFingerprint := fmt.Sprintf("%s:%d:%s", pubKeyHash, timestamp, hex.EncodeToString(bodyHash[:]))
if _, loaded := m.seenRequests.LoadOrStore(reqFingerprint, time.Now()); loaded {
    writeAuthError(w, awerrors.New(429, awerrors.CategoryAPI, "duplicate request detected", http.StatusTooManyRequests))
    return
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run TestAuthMiddleware ./internal/api/middleware/`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/middleware/auth.go internal/api/middleware/auth_test.go
git commit -m "feat(middleware): add replay attack protection to auth middleware"
```

---

## Task 4: Add Seed Command Tests

**Files:**
- Create: `cmd/seed/main_test.go`

- [ ] **Step 1: Write seed command tests**

Create `cmd/seed/main_test.go`:

```go
package main

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/daifei0527/polyant/pkg/config"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLoadConfig_Default(t *testing.T) {
    // Reset flags
    *configFile = ""
    cfg := config.DefaultConfig()

    assert.NotNil(t, cfg)
    assert.Equal(t, "seed", cfg.Node.Type)
}

func TestLoadConfig_FromFile(t *testing.T) {
    // Create temp config file
    tmpDir := t.TempDir()
    cfgPath := filepath.Join(tmpDir, "test-config.json")

    cfgContent := `{
        "node": {"type": "seed", "name": "test-seed", "data_dir": "` + tmpDir + `"},
        "network": {"listen_port": 9000, "api_port": 8080},
        "seed": {"domain": "test.example.com"}
    }`
    err := os.WriteFile(cfgPath, []byte(cfgContent), 0644)
    require.NoError(t, err)

    *configFile = cfgPath
    defer func() { *configFile = "" }()

    cfg, err := config.Load(cfgPath)
    require.NoError(t, err)
    assert.Equal(t, "test-seed", cfg.Node.Name)
}

func TestSeedApp_Validation(t *testing.T) {
    // Seed node requires domain
    cfg := config.DefaultConfig()
    cfg.Node.Type = "seed"
    cfg.Seed.Domain = ""

    err := cfg.Seed.Validate()
    assert.Error(t, err)
}

func TestVersion(t *testing.T) {
    assert.Equal(t, "1.0.0", Version)
}
```

- [ ] **Step 2: Run tests**

Run: `go test -v ./cmd/seed/`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/seed/main_test.go
git commit -m "test(seed): add seed command configuration tests"
```

---

## Task 5: Add User Command Tests

**Files:**
- Create: `cmd/user/main_test.go`

- [ ] **Step 1: Write user command tests**

Create `cmd/user/main_test.go`:

```go
package main

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/daifei0527/polyant/pkg/config"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLoadConfig_Default(t *testing.T) {
    *configFile = ""
    cfg := config.DefaultConfig()

    assert.NotNil(t, cfg)
}

func TestLoadConfig_FromFile(t *testing.T) {
    tmpDir := t.TempDir()
    cfgPath := filepath.Join(tmpDir, "test-config.json")

    cfgContent := `{
        "node": {"type": "user", "name": "test-user", "data_dir": "` + tmpDir + `"},
        "network": {"listen_port": 0, "api_port": 8080}
    }`
    err := os.WriteFile(cfgPath, []byte(cfgContent), 0644)
    require.NoError(t, err)

    *configFile = cfgPath
    defer func() { *configFile = "" }()

    cfg, err := config.Load(cfgPath)
    require.NoError(t, err)
    assert.Equal(t, "test-user", cfg.Node.Name)
}

func TestParseSeedNodes(t *testing.T) {
    nodes := parseSeedNodes("/ip4/1.2.3.4/tcp/9000/p2p/abc,/ip4/5.6.7.8/tcp/9000/p2p/def", nil)
    assert.Len(t, nodes, 2)
    assert.Equal(t, "/ip4/1.2.3.4/tcp/9000/p2p/abc", nodes[0])
}

func TestParseSeedNodes_FromConfig(t *testing.T) {
    configNodes := []string{"/ip4/1.2.3.4/tcp/9000/p2p/abc"}
    nodes := parseSeedNodes("", configNodes)
    assert.Equal(t, configNodes, nodes)
}

func TestVersion(t *testing.T) {
    assert.Equal(t, "1.0.0", Version)
}
```

- [ ] **Step 2: Run tests**

Run: `go test -v ./cmd/user/`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/user/main_test.go
git commit -m "test(user): add user command configuration tests"
```

---

## Task 6: Add Pactl CLI Tests

**Files:**
- Create: `cmd/pactl/main_test.go`

- [ ] **Step 1: Write pactl CLI tests**

Create `cmd/pactl/main_test.go`:

```go
package main

import (
    "bytes"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
    buf := new(bytes.Buffer)
    rootCmd.SetOut(buf)
    rootCmd.SetArgs([]string{"--help"})

    err := rootCmd.Execute()
    require.NoError(t, err)

    output := buf.String()
    assert.Contains(t, output, "pactl")
    assert.Contains(t, output, "Polyant")
}

func TestVersionFlag(t *testing.T) {
    buf := new(bytes.Buffer)
    rootCmd.SetOut(buf)
    rootCmd.SetArgs([]string{"--version"})

    err := rootCmd.Execute()
    require.NoError(t, err)

    output := buf.String()
    assert.Contains(t, output, version)
}

func TestDefaultServerAddress(t *testing.T) {
    // Reset to defaults
    serverAddr = "http://localhost:8080"
    assert.Equal(t, "http://localhost:8080", serverAddr)
}

func TestClientCreation(t *testing.T) {
    c := NewClient("http://localhost:8080")
    assert.NotNil(t, c)
    assert.Equal(t, "http://localhost:8080", c.baseURL)
}
```

- [ ] **Step 2: Run tests**

Run: `go test -v ./cmd/pactl/`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/pactl/main_test.go
git commit -m "test(pactl): add CLI command tests"
```

---

## Task 7: Verify Coverage and Final Cleanup

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: All tests PASS

- [ ] **Step 2: Check coverage for modified packages**

Run: `go test ./internal/api/handler/ ./internal/api/middleware/ ./cmd/seed/ ./cmd/user/ ./cmd/pactl/ -coverprofile=coverage_check.out`
Expected:
- handler: ≥ 80%
- middleware: ≥ 85%
- seed: ≥ 70%
- user: ≥ 70%
- pactl: ≥ 70%

- [ ] **Step 3: Run linter**

Run: `make lint`
Expected: No errors

- [ ] **Step 4: Final commit if needed**

```bash
git add -A
git commit -m "chore: Phase 1 P0 features and test coverage complete"
```

---

## Summary

| Task | Description | Estimated Time |
|------|-------------|----------------|
| 1 | Add GetByEmail to UserStore | 15 min |
| 2 | Complete email verification flow | 30 min |
| 3 | Add replay attack protection | 30 min |
| 4 | Seed command tests | 15 min |
| 5 | User command tests | 15 min |
| 6 | Pactl CLI tests | 15 min |
| 7 | Verify coverage and cleanup | 15 min |
| **Total** | | **~2.5 hours** |
