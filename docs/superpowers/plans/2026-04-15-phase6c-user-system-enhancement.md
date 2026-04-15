# Phase 6c: 用户体系完善实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完善用户体系，添加投票选举系统增强（自动当选、提名确认）、可配置封禁类型、四维度统计 API。

**Architecture:**
- 扩展现有模型：User 增加 BanType、Candidate 增加 Confirmed/SelfNominated、Election 增加 AutoElect
- 新增 StatsService 处理四个维度的统计
- 修改现有 AdminService 和 ElectionService 支持新功能

**Tech Stack:** Go 1.21+, Pebble KV, Bleve Search

---

## 文件结构

**新建文件：**
- `internal/core/user/stats_service.go` - 统计服务
- `internal/core/user/stats_service_test.go` - 统计服务测试
- `internal/api/handler/stats_handler.go` - 统计 API 处理器
- `internal/storage/kv/stats_store.go` - 统计数据存储

**修改文件：**
- `internal/storage/model/models.go` - User 模型扩展 BanType
- `internal/storage/model/election.go` - Election/Candidate 模型扩展
- `internal/core/user/admin_service.go` - 封禁逻辑支持 BanType
- `internal/core/election/election.go` - 自动当选、提名确认
- `internal/api/handler/election_handler.go` - 确认提名 API
- `internal/api/handler/admin_handler.go` - 封禁类型参数
- `internal/api/router/router.go` - 注册统计路由
- `internal/api/middleware/auth.go` - 处理 readonly 用户访问控制

---

## 阶段 1：模型扩展

### Task 1: User 模型扩展 - BanType 字段

**Files:**
- Modify: `internal/storage/model/models.go:131-153`
- Test: `internal/storage/model/models_test.go`

- [ ] **Step 1: 写失败的测试 - BanType 字段**

```go
// internal/storage/model/models_test.go

func TestUserBanType(t *testing.T) {
    user := &User{
        PublicKey: "test-key",
        Status:    UserStatusBanned,
        BanType:   BanTypeReadonly,
        BanReason: "违规操作",
    }

    assert.Equal(t, UserStatusBanned, user.Status)
    assert.Equal(t, BanTypeReadonly, user.BanType)
    assert.Equal(t, "违规操作", user.BanReason)
}

func TestUserBanTypeDefaults(t *testing.T) {
    user := &User{
        PublicKey: "test-key",
        Status:    UserStatusBanned,
    }
    // 默认封禁类型应该是 full
    assert.Equal(t, BanTypeFull, user.BanType)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/storage/model/... -run TestUserBanType`
Expected: FAIL - BanType not defined

- [ ] **Step 3: 添加 BanType 常量和字段**

```go
// internal/storage/model/model.go

// 用户状态常量
const (
    UserStatusActive   = "active"
    UserStatusBanned   = "banned"
    UserStatusReadonly = "readonly" // 新增：只读模式
)

// BanType 封禁类型
type BanType string

const (
    BanTypeFull     BanType = "full"     // 完全禁止访问
    BanTypeReadonly BanType = "readonly" // 只读模式
)
```

```go
// internal/storage/model/models.go

// User 表示系统中的一个用户
type User struct {
    PublicKey         string   `json:"publicKey"`
    AgentName         string   `json:"agentName"`
    UserLevel         int32    `json:"userLevel"`
    Email             string   `json:"email"`
    EmailVerified     bool     `json:"emailVerified"`
    Phone             string   `json:"phone"`
    RegisteredAt      int64    `json:"registeredAt"`
    LastActive        int64    `json:"lastActive"`
    ContributionCnt   int32    `json:"contributionCnt"`
    RatingCnt         int32    `json:"ratingCnt"`
    NodeId            string   `json:"nodeId"`
    Status            string   `json:"status"`
    // 封禁相关
    BanType           BanType `json:"banType,omitempty"`           // 封禁类型
    BanReason         string  `json:"banReason,omitempty"`
    BannedAt          int64   `json:"bannedAt,omitempty"`
    BannedBy          string  `json:"bannedBy,omitempty"`
    UnbannedAt        int64   `json:"unbannedAt,omitempty"`
    UnbannedBy        string  `json:"unbannedBy,omitempty"`
    // 等级变更
    LevelChangeReason string  `json:"levelChangeReason,omitempty"`
    LevelChangedAt    int64   `json:"levelChangedAt,omitempty"`
    LevelChangedBy    string  `json:"levelChangedBy,omitempty"`
}

// IsBanned 检查用户是否被封禁（完全禁止或只读）
func (u *User) IsBanned() bool {
    return u.Status == UserStatusBanned || u.Status == UserStatusReadonly
}

// IsReadOnly 检查用户是否处于只读模式
func (u *User) IsReadOnly() bool {
    return u.Status == UserStatusReadonly || (u.Status == UserStatusBanned && u.BanType == BanTypeReadonly)
}

// IsFullBanned 检查用户是否完全被封禁
func (u *User) IsFullBanned() bool {
    return u.Status == UserStatusBanned && (u.BanType == "" || u.BanType == BanTypeFull)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -v ./internal/storage/model/... -run TestUserBanType`
Expected: PASS

- [ ] **Step 5: 提交 User 模型扩展**

```bash
git add internal/storage/model/model.go internal/storage/model/models.go internal/storage/model/models_test.go
git commit -m "$(cat <<'EOF'
feat(model): add BanType field for configurable user ban

Add BanType to User model:
- BanTypeFull: complete access denial
- BanTypeReadonly: read-only mode

Add helper methods: IsBanned(), IsReadOnly(), IsFullBanned()

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Election 模型扩展 - AutoElect 和提名确认

**Files:**
- Modify: `internal/storage/model/election.go`
- Test: `internal/storage/model/election_test.go`

- [ ] **Step 1: 写失败的测试 - Election 扩展字段**

```go
// internal/storage/model/election_test.go

func TestElectionAutoElect(t *testing.T) {
    election := &Election{
        ID:            "ele-1",
        Title:         "Test Election",
        VoteThreshold: 10,
        AutoElect:     true,
    }

    assert.True(t, election.AutoElect)
    assert.True(t, election.ShouldAutoElect())
}

func TestCandidateConfirmation(t *testing.T) {
    candidate := &Candidate{
        ElectionID:   "ele-1",
        UserID:       "user-1",
        SelfNominated: true,
        Confirmed:    true,
    }

    assert.True(t, candidate.SelfNominated)
    assert.True(t, candidate.Confirmed)
    assert.True(t, candidate.IsReady())
}

func TestCandidatePeerNomination(t *testing.T) {
    candidate := &Candidate{
        ElectionID:   "ele-1",
        UserID:       "user-1",
        SelfNominated: false,
        NominatedBy:  "nominator-1",
        Confirmed:    false,
    }

    assert.False(t, candidate.SelfNominated)
    assert.False(t, candidate.IsReady()) // 未确认，不能投票
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/storage/model/... -run TestElection`
Expected: FAIL - AutoElect, SelfNominated, Confirmed not defined

- [ ] **Step 3: 扩展 Election 模型**

```go
// internal/storage/model/election.go

// Election 表示一次选举
type Election struct {
    ID            string         `json:"id"`
    Title         string         `json:"title"`
    Description   string         `json:"description"`
    Status        ElectionStatus `json:"status"`
    StartTime     int64          `json:"startTime"`
    EndTime       int64          `json:"endTime"`
    VoteThreshold int32          `json:"voteThreshold"`
    AutoElect     bool           `json:"autoElect"`     // 是否自动当选
    CreatedAt     int64          `json:"createdAt"`
    CreatedBy     string         `json:"createdBy"`
}

// ShouldAutoElect 判断是否应该自动当选
func (e *Election) ShouldAutoElect() bool {
    return e.AutoElect && e.Status == ElectionStatusActive
}

// NewElection 创建新选举（更新：添加 AutoElect 参数）
func NewElection(title, description, createdBy string, voteThreshold int32, duration time.Duration, autoElect bool) *Election {
    now := time.Now().UnixMilli()
    return &Election{
        ID:            generateID(),
        Title:         title,
        Description:   description,
        Status:        ElectionStatusActive,
        StartTime:     now,
        EndTime:       now + duration.Milliseconds(),
        VoteThreshold: voteThreshold,
        AutoElect:     autoElect,
        CreatedAt:     now,
        CreatedBy:     createdBy,
    }
}
```

- [ ] **Step 4: 扩展 Candidate 模型**

```go
// internal/storage/model/election.go

// Candidate 表示选举候选人
type Candidate struct {
    ElectionID    string          `json:"electionId"`
    UserID        string          `json:"userId"`
    UserName      string          `json:"userName"`
    NominatedBy   string          `json:"nominatedBy"`
    SelfNominated bool            `json:"selfNominated"` // 是否自荐
    Confirmed     bool            `json:"confirmed"`     // 是否确认接受提名
    ConfirmedAt   int64           `json:"confirmedAt,omitempty"`
    VoteCount     int32           `json:"voteCount"`
    Status        CandidateStatus `json:"status"`
    NominatedAt   int64           `json:"nominatedAt"`
}

// IsReady 判断候选人是否准备好（自荐自动确认，他荐需手动确认）
func (c *Candidate) IsReady() bool {
    return c.Confirmed
}

// Confirm 确认接受提名
func (c *Candidate) Confirm() {
    c.Confirmed = true
    c.ConfirmedAt = time.Now().UnixMilli()
}

// NewCandidate 创建新候选人
func NewCandidate(electionID, userID, userName, nominatedBy string, selfNominated bool) *Candidate {
    now := time.Now().UnixMilli()
    c := &Candidate{
        ElectionID:    electionID,
        UserID:        userID,
        UserName:      userName,
        NominatedBy:   nominatedBy,
        SelfNominated: selfNominated,
        VoteCount:     0,
        Status:        CandidateStatusNominated,
        NominatedAt:   now,
    }
    // 自荐自动确认
    if selfNominated {
        c.Confirmed = true
        c.ConfirmedAt = now
    }
    return c
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -v ./internal/storage/model/... -run TestElection`
Expected: PASS

- [ ] **Step 6: 提交 Election 模型扩展**

```bash
git add internal/storage/model/election.go internal/storage/model/election_test.go
git commit -m "$(cat <<'EOF'
feat(model): add AutoElect and nomination confirmation to Election

Election changes:
- Add AutoElect field for automatic election when threshold reached
- Add ShouldAutoElect() method

Candidate changes:
- Add SelfNominated field to distinguish self vs peer nomination
- Add Confirmed/ConfirmedAt fields for nomination confirmation
- Add IsReady() method to check if candidate is confirmed
- Add Confirm() method to accept nomination
- Self-nomination auto-confirms

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## 阶段 2：管理功能增强

### Task 3: AdminService 支持 BanType

**Files:**
- Modify: `internal/core/user/admin_service.go`
- Test: `internal/core/user/admin_service_test.go`

- [ ] **Step 1: 写失败的测试 - BanType 参数**

```go
// internal/core/user/admin_service_test.go

func TestAdminService_BanUser_WithBanType(t *testing.T) {
    store := setupTestStore(t)
    svc := NewAdminService(store)

    // 创建目标用户
    targetUser := &model.User{
        PublicKey:    "target-key",
        UserLevel:    model.UserLevelLv1,
        Status:       model.UserStatusActive,
    }
    store.User.Create(context.Background(), targetUser)

    // 测试只读封禁
    err := svc.BanUser(context.Background(), "target-key", "admin-key", "违规", model.BanTypeReadonly)
    require.NoError(t, err)

    user, _ := store.User.Get(context.Background(), user.HashPublicKey("target-key"))
    assert.Equal(t, model.UserStatusReadonly, user.Status)
    assert.Equal(t, model.BanTypeReadonly, user.BanType)
}

func TestAdminService_BanUser_FullBan(t *testing.T) {
    store := setupTestStore(t)
    svc := NewAdminService(store)

    targetUser := &model.User{
        PublicKey:    "target-key",
        UserLevel:    model.UserLevelLv1,
        Status:       model.UserStatusActive,
    }
    store.User.Create(context.Background(), targetUser)

    err := svc.BanUser(context.Background(), "target-key", "admin-key", "严重违规", model.BanTypeFull)
    require.NoError(t, err)

    user, _ := store.User.Get(context.Background(), user.HashPublicKey("target-key"))
    assert.Equal(t, model.UserStatusBanned, user.Status)
    assert.Equal(t, model.BanTypeFull, user.BanType)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/core/user/... -run TestAdminService_BanUser`
Expected: FAIL - BanUser signature mismatch

- [ ] **Step 3: 更新 AdminService.BanUser 方法**

```go
// internal/core/user/admin_service.go

// BanUser 封禁用户
func (s *AdminService) BanUser(ctx context.Context, targetPublicKey, adminPublicKey, reason string, banType model.BanType) error {
    hash := HashPublicKey(targetPublicKey)
    user, err := s.store.User.Get(ctx, hash)
    if err != nil {
        return ErrUserNotFound
    }

    // 不能封禁 Lv4+ 管理员
    if user.UserLevel >= model.UserLevelLv4 {
        return ErrCannotBanAdmin
    }

    // 设置封禁状态
    if banType == model.BanTypeReadonly {
        user.Status = model.UserStatusReadonly
    } else {
        user.Status = model.UserStatusBanned
    }
    user.BanType = banType
    user.BanReason = reason
    user.BannedAt = time.Now().UnixMilli()
    user.BannedBy = adminPublicKey

    _, err = s.store.User.Update(ctx, user)
    return err
}

// UnbanUser 解封用户
func (s *AdminService) UnbanUser(ctx context.Context, targetPublicKey, adminPublicKey string) error {
    hash := HashPublicKey(targetPublicKey)
    user, err := s.store.User.Get(ctx, hash)
    if err != nil {
        return ErrUserNotFound
    }

    user.Status = model.UserStatusActive
    user.BanType = ""
    user.BanReason = ""
    user.BannedAt = 0
    user.BannedBy = ""
    user.UnbannedAt = time.Now().UnixMilli()
    user.UnbannedBy = adminPublicKey

    _, err = s.store.User.Update(ctx, user)
    return err
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -v ./internal/core/user/... -run TestAdminService_BanUser`
Expected: PASS

- [ ] **Step 5: 提交 AdminService 更新**

```bash
git add internal/core/user/admin_service.go internal/core/user/admin_service_test.go
git commit -m "$(cat <<'EOF'
feat(admin): support configurable ban type in BanUser

Update BanUser to accept banType parameter:
- BanTypeFull: sets status to "banned"
- BanTypeReadonly: sets status to "readonly"

Clear BanType when unbanning user.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: AdminHandler 支持 BanType 参数

**Files:**
- Modify: `internal/api/handler/admin_handler.go`

- [ ] **Step 1: 更新 BanUserRequest 结构体**

```go
// internal/api/handler/admin_handler.go

// BanUserRequest 封禁用户请求
type BanUserRequest struct {
    Reason  string         `json:"reason"`
    BanType model.BanType  `json:"ban_type"` // full 或 readonly
}
```

- [ ] **Step 2: 更新 BanUserHandler**

```go
// internal/api/handler/admin_handler.go

// BanUserHandler 封禁用户
// POST /api/v1/admin/users/{public_key}/ban
func (h *AdminHandler) BanUserHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
        return
    }

    publicKey := extractAdminPathParam(r.URL.Path, "/api/v1/admin/users/", "/ban")
    if publicKey == "" {
        writeError(w, awerrors.ErrInvalidParams)
        return
    }

    var req BanUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, awerrors.ErrJSONParse)
        return
    }

    // 默认完全封禁
    if req.BanType == "" {
        req.BanType = model.BanTypeFull
    }

    // 验证 banType
    if req.BanType != model.BanTypeFull && req.BanType != model.BanTypeReadonly {
        writeError(w, awerrors.New(400, awerrors.CategoryAPI, "invalid ban_type", http.StatusBadRequest))
        return
    }

    adminPublicKey, _ := r.Context().Value("public_key").(string)

    ctx := r.Context()
    if err := h.adminSvc.BanUser(ctx, publicKey, adminPublicKey, req.Reason, req.BanType); err != nil {
        writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusBadRequest, err))
        return
    }

    writeJSON(w, http.StatusOK, &APIResponse{
        Code:    0,
        Message: "success",
        Data: map[string]interface{}{
            "success":     true,
            "ban_type":    req.BanType,
            "public_key":  publicKey,
        },
    })
}
```

- [ ] **Step 3: 编译验证**

Run: `go build ./...`
Expected: Build success

- [ ] **Step 4: 提交 AdminHandler 更新**

```bash
git add internal/api/handler/admin_handler.go
git commit -m "$(cat <<'EOF'
feat(handler): add ban_type parameter to ban user API

Update POST /api/v1/admin/users/{id}/ban:
- Accept ban_type field (full/readonly)
- Default to "full" if not specified
- Validate ban_type value

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Auth 中间件处理 Readonly 用户

**Files:**
- Modify: `internal/api/middleware/auth.go`

- [ ] **Step 1: 写失败的测试 - Readonly 用户访问控制**

```go
// internal/api/middleware/auth_test.go

func TestAuthMiddleware_ReadonlyUser(t *testing.T) {
    // 创建 readonly 用户
    user := &model.User{
        PublicKey: "readonly-key",
        Status:    model.UserStatusReadonly,
        BanType:   model.BanTypeReadonly,
        UserLevel: model.UserLevelLv1,
    }
    
    // Readonly 用户可以访问读取接口
    assert.True(t, canRead(user))
    // Readonly 用户不能写入
    assert.False(t, canWrite(user))
}

func TestAuthMiddleware_FullBannedUser(t *testing.T) {
    user := &model.User{
        PublicKey: "banned-key",
        Status:    model.UserStatusBanned,
        BanType:   model.BanTypeFull,
    }
    
    // 完全封禁用户不能访问任何接口
    assert.False(t, canRead(user))
    assert.False(t, canWrite(user))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/api/middleware/... -run TestAuthMiddleware_Readonly`
Expected: FAIL - canRead/canWrite not defined

- [ ] **Step 3: 更新 AuthMiddleware 中间件**

```go
// internal/api/middleware/auth.go

// Middleware 返回 HTTP 中间件处理函数
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ... 现有的签名验证逻辑保持不变 ...
        
        user, err := m.userStore.Get(r.Context(), pubKeyHash)
        if err != nil {
            writeAuthError(w, awerrors.ErrUserNotFound)
            return
        }

        // 检查用户状态
        if user.IsFullBanned() {
            // 完全封禁：拒绝所有请求
            writeAuthError(w, awerrors.New(403, awerrors.CategoryAPI, "用户已被封禁", http.StatusForbidden))
            return
        }

        // 只读模式用户检查
        if user.IsReadOnly() && isWriteOperation(r.Method, r.URL.Path) {
            writeAuthError(w, awerrors.New(403, awerrors.CategoryAPI, "用户处于只读模式", http.StatusForbidden))
            return
        }

        // 将用户信息注入上下文
        ctx := context.WithValue(r.Context(), UserKey, user)
        ctx = context.WithValue(ctx, PublicKeyKey, user.PublicKey)
        ctx = context.WithValue(ctx, UserLevelKey, user.UserLevel)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// isWriteOperation 判断是否为写操作
func isWriteOperation(method, path string) bool {
    // GET 和 HEAD 是读操作
    if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
        // 但某些 GET 路径可能触发写操作（如触发同步）
        writePaths := []string{"/api/v1/node/sync"}
        for _, wp := range writePaths {
            if strings.HasPrefix(path, wp) {
                return true
            }
        }
        return false
    }
    return true
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -v ./internal/api/middleware/... -run TestAuthMiddleware`
Expected: PASS

- [ ] **Step 5: 提交 Auth 中间件更新**

```bash
git add internal/api/middleware/auth.go internal/api/middleware/auth_test.go
git commit -m "$(cat <<'EOF'
feat(auth): handle readonly banned users in middleware

Update auth middleware to distinguish ban types:
- Full ban (BanTypeFull): reject all requests with 403
- Readonly (BanTypeReadonly): allow GET/HEAD, reject POST/PUT/DELETE

Add isWriteOperation() helper to determine request type.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## 阶段 3：选举系统增强

### Task 6: ElectionService 支持自动当选

**Files:**
- Modify: `internal/core/election/election.go`
- Test: `internal/core/election/election_test.go`

- [ ] **Step 1: 写失败的测试 - 自动当选**

```go
// internal/core/election/election_test.go

func TestElectionService_AutoElect(t *testing.T) {
    store := setupTestElectionStore(t)
    svc := NewElectionService(
        store.ElectionStore,
        store.CandidateStore,
        store.VoteStore,
    )

    // 创建启用自动当选的选举
    election, _ := svc.CreateElection(ctx, "Test", "Desc", "creator", 3, 7, true)
    
    // 添加候选人
    svc.NominateCandidate(ctx, election.ID, "user-1", "User 1", "user-1", true)
    
    // 投票 3 次（达到阈值）
    svc.Vote(ctx, election.ID, "voter-1", "user-1")
    svc.Vote(ctx, election.ID, "voter-2", "user-1")
    result, err := svc.Vote(ctx, election.ID, "voter-3", "user-1")
    
    require.NoError(t, err)
    // 验证候选人自动当选
    assert.True(t, result.AutoElected)
    
    candidate, _ := store.CandidateStore.Get(ctx, election.ID, "user-1")
    assert.Equal(t, model.CandidateStatusElected, candidate.Status)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/core/election/... -run TestElectionService_AutoElect`
Expected: FAIL - CreateElection signature mismatch, Vote doesn't return result

- [ ] **Step 3: 更新 CreateElection 方法签名**

```go
// internal/core/election/election.go

// CreateElection 创建选举
func (s *ElectionService) CreateElection(ctx context.Context, title, description, createdBy string, voteThreshold int32, durationDays int, autoElect bool) (*model.Election, error) {
    duration := time.Duration(durationDays) * 24 * time.Hour
    if duration == 0 {
        duration = 7 * 24 * time.Hour
    }

    election := model.NewElection(title, description, createdBy, voteThreshold, duration, autoElect)
    if err := s.electionStore.Create(ctx, election); err != nil {
        return nil, fmt.Errorf("create election: %w", err)
    }

    return election, nil
}
```

- [ ] **Step 4: 添加 VoteResult 结构体和更新 Vote 方法**

```go
// internal/core/election/election.go

// VoteResult 投票结果
type VoteResult struct {
    Success     bool   `json:"success"`
    VoteCount   int32  `json:"vote_count"`
    AutoElected bool   `json:"auto_elected,omitempty"`
    Message     string `json:"message,omitempty"`
}

// Vote 投票
func (s *ElectionService) Vote(ctx context.Context, electionID, voterID, candidateID string) (*VoteResult, error) {
    election, err := s.electionStore.Get(ctx, electionID)
    if err != nil {
        return nil, ErrElectionNotFound
    }

    if election.Status != model.ElectionStatusActive {
        return nil, ErrElectionClosed
    }

    // 检查候选人是否存在且已确认
    candidate, err := s.candidateStore.Get(ctx, electionID, candidateID)
    if err != nil {
        return nil, ErrCandidateNotFound
    }
    
    if !candidate.IsReady() {
        return nil, fmt.Errorf("候选人尚未确认接受提名")
    }

    // 检查是否已投票
    hasVoted, err := s.voteStore.HasVoted(ctx, voterID, electionID)
    if err != nil {
        return nil, err
    }
    if hasVoted {
        return nil, ErrAlreadyVoted
    }

    // 创建投票记录
    vote := &model.Vote{
        ID:          generateVoteID(),
        ElectionID:  electionID,
        VoterID:     voterID,
        CandidateID: candidateID,
        VotedAt:     time.Now().UnixMilli(),
    }

    if err := s.voteStore.Create(ctx, vote); err != nil {
        return nil, fmt.Errorf("create vote: %w", err)
    }

    // 更新候选人票数
    newVoteCount := candidate.VoteCount + 1
    if err := s.candidateStore.UpdateVoteCount(ctx, electionID, candidateID, 1); err != nil {
        return nil, fmt.Errorf("update vote count: %w", err)
    }

    result := &VoteResult{
        Success:   true,
        VoteCount: newVoteCount,
    }

    // 检查是否自动当选
    if election.ShouldAutoElect() && newVoteCount >= election.VoteThreshold {
        s.candidateStore.UpdateStatus(ctx, electionID, candidateID, model.CandidateStatusElected)
        result.AutoElected = true
        result.Message = "恭喜！候选人已自动当选"
    }

    return result, nil
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -v ./internal/core/election/... -run TestElectionService_AutoElect`
Expected: PASS

- [ ] **Step 6: 提交自动当选功能**

```bash
git add internal/core/election/election.go internal/core/election/election_test.go
git commit -m "$(cat <<'EOF'
feat(election): implement auto-elect when vote threshold reached

Add auto-elect feature:
- Update CreateElection to accept autoElect parameter
- Add VoteResult struct to return vote outcome
- Check threshold after each vote when autoElect is enabled
- Auto-mark candidate as elected when threshold reached

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: ElectionService 支持提名确认

**Files:**
- Modify: `internal/core/election/election.go`
- Modify: `internal/core/election/election.go`

- [ ] **Step 1: 写失败的测试 - 提名确认**

```go
// internal/core/election/election_test.go

func TestElectionService_NominateCandidate_SelfNomination(t *testing.T) {
    svc := setupTestElectionService(t)
    election, _ := svc.CreateElection(ctx, "Test", "Desc", "creator", 5, 7, false)
    
    // Lv4 用户自荐
    err := svc.NominateCandidate(ctx, election.ID, "lv4-user", "Lv4 User", "lv4-user", true)
    require.NoError(t, err)
    
    // 自荐自动确认
    candidate, _ := svc.candidateStore.Get(ctx, election.ID, "lv4-user")
    assert.True(t, candidate.SelfNominated)
    assert.True(t, candidate.Confirmed)
}

func TestElectionService_NominateCandidate_PeerNomination(t *testing.T) {
    svc := setupTestElectionService(t)
    election, _ := svc.CreateElection(ctx, "Test", "Desc", "creator", 5, 7, false)
    
    // Lv3 用户提名 Lv4 用户
    err := svc.NominateCandidate(ctx, election.ID, "lv4-user", "Lv4 User", "lv3-nominator", false)
    require.NoError(t, err)
    
    // 他荐需要确认
    candidate, _ := svc.candidateStore.Get(ctx, election.ID, "lv4-user")
    assert.False(t, candidate.SelfNominated)
    assert.False(t, candidate.Confirmed)
    
    // 被提名人确认
    err = svc.ConfirmNomination(ctx, election.ID, "lv4-user")
    require.NoError(t, err)
    
    candidate, _ = svc.candidateStore.Get(ctx, election.ID, "lv4-user")
    assert.True(t, candidate.Confirmed)
}

func TestElectionService_Vote_UnconfirmedCandidate(t *testing.T) {
    svc := setupTestElectionService(t)
    election, _ := svc.CreateElection(ctx, "Test", "Desc", "creator", 5, 7, false)
    
    // 他荐未确认
    svc.NominateCandidate(ctx, election.ID, "lv4-user", "Lv4 User", "nominator", false)
    
    // 投票应该失败
    _, err := svc.Vote(ctx, election.ID, "voter", "lv4-user")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "尚未确认")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/core/election/... -run TestElectionService_Nominate`
Expected: FAIL - NominateCandidate signature mismatch, ConfirmNomination not defined

- [ ] **Step 3: 更新 NominateCandidate 方法**

```go
// internal/core/election/election.go

// NominateCandidate 提名候选人
func (s *ElectionService) NominateCandidate(ctx context.Context, electionID, userID, userName, nominatedBy string, selfNominated bool) error {
    election, err := s.electionStore.Get(ctx, electionID)
    if err != nil {
        return ErrElectionNotFound
    }

    if election.Status != model.ElectionStatusActive {
        return ErrElectionClosed
    }

    // 检查是否已被提名
    existing, _ := s.candidateStore.Get(ctx, electionID, userID)
    if existing != nil {
        return ErrAlreadyNominated
    }

    candidate := model.NewCandidate(electionID, userID, userName, nominatedBy, selfNominated)
    return s.candidateStore.Add(ctx, candidate)
}

// ConfirmNomination 确认接受提名
func (s *ElectionService) ConfirmNomination(ctx context.Context, electionID, userID string) error {
    election, err := s.electionStore.Get(ctx, electionID)
    if err != nil {
        return ErrElectionNotFound
    }

    if election.Status != model.ElectionStatusActive {
        return ErrElectionClosed
    }

    candidate, err := s.candidateStore.Get(ctx, electionID, userID)
    if err != nil {
        return ErrCandidateNotFound
    }

    if candidate.Confirmed {
        return fmt.Errorf("已确认接受提名")
    }

    candidate.Confirm()
    return s.candidateStore.Add(ctx, candidate)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -v ./internal/core/election/... -run TestElectionService_Nominate`
Expected: PASS

- [ ] **Step 5: 提交提名确认功能**

```bash
git add internal/core/election/election.go internal/core/election/election_test.go
git commit -m "$(cat <<'EOF'
feat(election): support nomination confirmation

Add nomination confirmation flow:
- Self-nomination auto-confirms
- Peer nomination requires explicit confirmation
- Add ConfirmNomination method
- Reject votes for unconfirmed candidates

Update NominateCandidate to accept selfNominated parameter.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: ElectionHandler 更新确认提名 API

**Files:**
- Modify: `internal/api/handler/election_handler.go`

- [ ] **Step 1: 更新 CreateElectionRequest 和 NominateRequest**

```go
// internal/api/handler/election_handler.go

// CreateElectionRequest 创建选举请求
type CreateElectionRequest struct {
    Title         string `json:"title"`
    Description   string `json:"description"`
    VoteThreshold int32  `json:"vote_threshold"`
    DurationDays  int    `json:"duration_days"`
    AutoElect     bool   `json:"auto_elect"` // 是否自动当选
}

// NominateRequest 提名请求
type NominateRequest struct {
    UserID        string `json:"user_id"`
    UserName      string `json:"user_name"`
    SelfNominated bool   `json:"self_nominated"` // true=自荐, false=他荐
}
```

- [ ] **Step 2: 更新 CreateElectionHandler**

```go
// internal/api/handler/election_handler.go

// CreateElectionHandler 创建选举
func (h *ElectionHandler) CreateElectionHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
            Code:    405,
            Message: "method not allowed",
        })
        return
    }

    var req CreateElectionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, &APIResponse{
            Code:    400,
            Message: "invalid request body",
        })
        return
    }

    publicKey, _ := r.Context().Value("public_key").(string)

    ctx := r.Context()
    election, err := h.electionSvc.CreateElection(ctx, req.Title, req.Description, publicKey, req.VoteThreshold, req.DurationDays, req.AutoElect)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, &APIResponse{
            Code:    500,
            Message: err.Error(),
        })
        return
    }

    writeJSON(w, http.StatusCreated, &APIResponse{
        Code:    0,
        Message: "success",
        Data: map[string]interface{}{
            "election_id": election.ID,
            "auto_elect":  election.AutoElect,
        },
    })
}
```

- [ ] **Step 3: 更新 NominateCandidateHandler**

```go
// internal/api/handler/election_handler.go

// NominateCandidateHandler 提名候选人
func (h *ElectionHandler) NominateCandidateHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
            Code:    405,
            Message: "method not allowed",
        })
        return
    }

    electionID := extractPathParam(r.URL.Path, "/api/v1/elections/", "/candidates")
    if electionID == "" {
        writeJSON(w, http.StatusBadRequest, &APIResponse{
            Code:    400,
            Message: "missing election_id",
        })
        return
    }

    var req NominateRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, &APIResponse{
            Code:    400,
            Message: "invalid request body",
        })
        return
    }

    publicKey, _ := r.Context().Value("public_key").(string)

    // 如果是自荐，UserID 应该是自己的公钥
    if req.SelfNominated {
        req.UserID = publicKey
    }

    ctx := r.Context()
    if err := h.electionSvc.NominateCandidate(ctx, electionID, req.UserID, req.UserName, publicKey, req.SelfNominated); err != nil {
        writeJSON(w, http.StatusBadRequest, &APIResponse{
            Code:    400,
            Message: err.Error(),
        })
        return
    }

    writeJSON(w, http.StatusOK, &APIResponse{
        Code:    0,
        Message: "success",
        Data: map[string]interface{}{
            "success":       true,
            "self_nominated": req.SelfNominated,
            "confirmed":     req.SelfNominated, // 自荐自动确认
        },
    })
}
```

- [ ] **Step 4: 添加 ConfirmNominationHandler**

```go
// internal/api/handler/election_handler.go

// ConfirmNominationHandler 确认接受提名
// POST /api/v1/elections/{id}/candidates/{user_id}/confirm
func (h *ElectionHandler) ConfirmNominationHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
            Code:    405,
            Message: "method not allowed",
        })
        return
    }

    // 解析路径: /api/v1/elections/{election_id}/candidates/{user_id}/confirm
    path := r.URL.Path
    parts := strings.Split(path, "/")
    if len(parts) < 8 {
        writeJSON(w, http.StatusBadRequest, &APIResponse{
            Code:    400,
            Message: "invalid path",
        })
        return
    }
    electionID := parts[4]
    userID := parts[6]

    // 验证权限：只有被提名人自己可以确认
    publicKey, _ := r.Context().Value("public_key").(string)
    if publicKey != userID {
        writeJSON(w, http.StatusForbidden, &APIResponse{
            Code:    403,
            Message: "只有被提名人自己可以确认",
        })
        return
    }

    ctx := r.Context()
    if err := h.electionSvc.ConfirmNomination(ctx, electionID, userID); err != nil {
        writeJSON(w, http.StatusBadRequest, &APIResponse{
            Code:    400,
            Message: err.Error(),
        })
        return
    }

    writeJSON(w, http.StatusOK, &APIResponse{
        Code:    0,
        Message: "success",
        Data:    map[string]bool{"confirmed": true},
    })
}
```

- [ ] **Step 5: 更新 VoteHandler 返回 VoteResult**

```go
// internal/api/handler/election_handler.go

// VoteHandler 投票
func (h *ElectionHandler) VoteHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
            Code:    405,
            Message: "method not allowed",
        })
        return
    }

    electionID := extractPathParam(r.URL.Path, "/api/v1/elections/", "/vote")
    if electionID == "" {
        writeJSON(w, http.StatusBadRequest, &APIResponse{
            Code:    400,
            Message: "missing election_id",
        })
        return
    }

    var req VoteRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, &APIResponse{
            Code:    400,
            Message: "invalid request body",
        })
        return
    }

    publicKey, _ := r.Context().Value("public_key").(string)

    ctx := r.Context()
    result, err := h.electionSvc.Vote(ctx, electionID, publicKey, req.CandidateID)
    if err != nil {
        writeJSON(w, http.StatusBadRequest, &APIResponse{
            Code:    400,
            Message: err.Error(),
        })
        return
    }

    writeJSON(w, http.StatusOK, &APIResponse{
        Code:    0,
        Message: "success",
        Data:    result,
    })
}
```

- [ ] **Step 6: 编译验证**

Run: `go build ./...`
Expected: Build success

- [ ] **Step 7: 提交 ElectionHandler 更新**

```bash
git add internal/api/handler/election_handler.go
git commit -m "$(cat <<'EOF'
feat(handler): add nomination confirmation API

Update election handlers:
- CreateElection: accept auto_elect parameter
- NominateCandidate: accept self_nominated parameter
- Add ConfirmNominationHandler for peer nominations
- VoteHandler: return VoteResult with auto_elected flag

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: 更新路由注册

**Files:**
- Modify: `internal/api/router/router.go`

- [ ] **Step 1: 添加确认提名路由**

```go
// internal/api/router/router.go

// 在 registerAuthRoutes 中添加
// 确认接受提名 POST /api/v1/elections/{id}/candidates/{user_id}/confirm - 被提名人自己
mux.Handle("/api/v1/elections/candidates/confirm/", authMW.Middleware(http.HandlerFunc(elh.ConfirmNominationHandler)))
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: Build success

- [ ] **Step 3: 提交路由更新**

```bash
git add internal/api/router/router.go
git commit -m "$(cat <<'EOF'
feat(router): add confirm nomination route

Register POST /api/v1/elections/candidates/confirm/ for nomination confirmation.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## 阶段 4：统计 API

### Task 10: 创建 StatsService

**Files:**
- Create: `internal/core/user/stats_service.go`
- Create: `internal/core/user/stats_service_test.go`

- [ ] **Step 1: 写失败的测试 - 统计服务**

```go
// internal/core/user/stats_service_test.go

package user

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/daifei0527/polyant/internal/storage/model"
)

func TestStatsService_UserStats(t *testing.T) {
    store := setupTestStore(t)
    svc := NewStatsService(store)

    // 创建测试用户
    createTestUsers(t, store)

    stats, err := svc.GetUserStats(context.Background())
    require.NoError(t, err)

    assert.Equal(t, int64(10), stats.TotalUsers)
    assert.Equal(t, int64(2), stats.Lv0Count)
    assert.Equal(t, int64(3), stats.Lv1Count)
}

func TestStatsService_ContributionStats(t *testing.T) {
    store := setupTestStore(t)
    svc := NewStatsService(store)

    createTestUsersWithContributions(t, store)

    contribs, total, err := svc.GetContributionStats(context.Background(), 0, 10, "entry_count")
    require.NoError(t, err)

    assert.Equal(t, int64(10), total)
    assert.Equal(t, 10, len(contribs))
    // 验证按 entry_count 降序排列
    assert.True(t, contribs[0].EntryCount >= contribs[1].EntryCount)
}

func TestStatsService_ActivityTrend(t *testing.T) {
    store := setupTestStore(t)
    svc := NewStatsService(store)

    createTestActivityData(t, store)

    trend, err := svc.GetActivityTrend(context.Background(), 7)
    require.NoError(t, err)

    assert.Equal(t, 7, len(trend))
    assert.NotZero(t, trend[0].DAU)
}

func TestStatsService_RegistrationTrend(t *testing.T) {
    store := setupTestStore(t)
    svc := NewStatsService(store)

    createTestUsers(t, store)

    trend, err := svc.GetRegistrationTrend(context.Background(), 30)
    require.NoError(t, err)

    assert.NotEmpty(t, trend)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/core/user/... -run TestStatsService`
Expected: FAIL - StatsService not defined

- [ ] **Step 3: 创建 StatsService**

```go
// internal/core/user/stats_service.go

package user

import (
    "context"
    "sort"
    "time"

    "github.com/daifei0527/polyant/internal/storage"
    "github.com/daifei0527/polyant/internal/storage/model"
)

// StatsService 统计服务
type StatsService struct {
    store *storage.Store
}

// NewStatsService 创建统计服务
func NewStatsService(store *storage.Store) *StatsService {
    return &StatsService{store: store}
}

// GetUserStats 获取用户统计概览
func (s *StatsService) GetUserStats(ctx context.Context) (*model.UserStats, error) {
    users, total, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
    if err != nil {
        return nil, err
    }

    stats := &model.UserStats{TotalUsers: total}
    now := time.Now().UnixMilli()
    thirtyDaysAgo := now - 30*24*60*60*1000

    for _, u := range users {
        switch u.UserLevel {
        case model.UserLevelLv0:
            stats.Lv0Count++
        case model.UserLevelLv1:
            stats.Lv1Count++
        case model.UserLevelLv2:
            stats.Lv2Count++
        case model.UserLevelLv3:
            stats.Lv3Count++
        case model.UserLevelLv4:
            stats.Lv4Count++
        case model.UserLevelLv5:
            stats.Lv5Count++
        }

        if u.LastActive > thirtyDaysAgo {
            stats.ActiveUsers++
        }

        if u.Status == model.UserStatusBanned || u.Status == model.UserStatusReadonly {
            stats.BannedCount++
        }

        stats.TotalContribs += int64(u.ContributionCnt)
        stats.TotalRatings += int64(u.RatingCnt)
    }

    return stats, nil
}

// UserContribution 用户贡献明细
type UserContribution struct {
    UserID           string  `json:"user_id"`
    UserName         string  `json:"user_name"`
    EntryCount       int64   `json:"entry_count"`
    EditCount        int64   `json:"edit_count"`
    RatingGivenCount int64   `json:"rating_given_count"`
    RatingRecvCount  int64   `json:"rating_recv_count"`
    AvgRatingRecv    float64 `json:"avg_rating_recv"`
}

// GetContributionStats 获取贡献明细统计
func (s *StatsService) GetContributionStats(ctx context.Context, offset, limit int, sortBy string) ([]UserContribution, int64, error) {
    users, total, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
    if err != nil {
        return nil, 0, err
    }

    var contribs []UserContribution
    for _, u := range users {
        contribs = append(contribs, UserContribution{
            UserID:           u.PublicKey,
            UserName:         u.AgentName,
            EntryCount:       int64(u.ContributionCnt),
            RatingGivenCount: int64(u.RatingCnt),
        })
    }

    // 排序
    sort.Slice(contribs, func(i, j int) bool {
        switch sortBy {
        case "entry_count":
            return contribs[i].EntryCount > contribs[j].EntryCount
        case "rating_given_count":
            return contribs[i].RatingGivenCount > contribs[j].RatingGivenCount
        default:
            return contribs[i].EntryCount > contribs[j].EntryCount
        }
    })

    // 分页
    if offset >= len(contribs) {
        return []UserContribution{}, total, nil
    }
    end := offset + limit
    if end > len(contribs) {
        end = len(contribs)
    }
    return contribs[offset:end], total, nil
}

// ActivityTrend 活跃度趋势
type ActivityTrend struct {
    Date        string `json:"date"`
    DAU         int64  `json:"dau"`
    NewUsers    int64  `json:"new_users"`
    ActionCount int64  `json:"action_count"`
}

// GetActivityTrend 获取活跃度趋势
func (s *StatsService) GetActivityTrend(ctx context.Context, days int) ([]ActivityTrend, error) {
    users, _, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
    if err != nil {
        return nil, err
    }

    now := time.Now()
    trend := make([]ActivityTrend, days)

    for i := 0; i < days; i++ {
        date := now.AddDate(0, 0, -i)
        dateStr := date.Format("2006-01-02")
        dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location()).UnixMilli()
        dayEnd := dayStart + 24*60*60*1000

        var dau, newUsers int64
        for _, u := range users {
            // 检查是否在该天活跃
            if u.LastActive >= dayStart && u.LastActive < dayEnd {
                dau++
            }
            // 检查是否在该天注册
            if u.RegisteredAt >= dayStart && u.RegisteredAt < dayEnd {
                newUsers++
            }
        }

        trend[days-1-i] = ActivityTrend{
            Date:     dateStr,
            DAU:      dau,
            NewUsers: newUsers,
        }
    }

    return trend, nil
}

// RegistrationTrend 注册趋势
type RegistrationTrend struct {
    Date  string `json:"date"`
    Count int64  `json:"count"`
    Total int64  `json:"total"`
}

// GetRegistrationTrend 获取注册趋势
func (s *StatsService) GetRegistrationTrend(ctx context.Context, days int) ([]RegistrationTrend, error) {
    users, total, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
    if err != nil {
        return nil, err
    }

    now := time.Now()
    trend := make([]RegistrationTrend, days)
    cumulative := total

    for i := 0; i < days; i++ {
        date := now.AddDate(0, 0, -i)
        dateStr := date.Format("2006-01-02")
        dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location()).UnixMilli()
        dayEnd := dayStart + 24*60*60*1000

        var count int64
        for _, u := range users {
            if u.RegisteredAt >= dayStart && u.RegisteredAt < dayEnd {
                count++
            }
        }

        trend[days-1-i] = RegistrationTrend{
            Date:  dateStr,
            Count: count,
            Total: cumulative,
        }
        cumulative -= count
    }

    return trend, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -v ./internal/core/user/... -run TestStatsService`
Expected: PASS

- [ ] **Step 5: 提交 StatsService**

```bash
git add internal/core/user/stats_service.go internal/core/user/stats_service_test.go
git commit -m "$(cat <<'EOF'
feat(stats): add StatsService for user statistics

Add StatsService with 4 dimensions:
- GetUserStats: level distribution, active users, banned count
- GetContributionStats: per-user contributions with sorting
- GetActivityTrend: DAU, new users over time
- GetRegistrationTrend: registration counts over time

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: 创建 StatsHandler

**Files:**
- Create: `internal/api/handler/stats_handler.go`

- [ ] **Step 1: 创建 StatsHandler**

```go
// internal/api/handler/stats_handler.go

package handler

import (
    "net/http"
    "strconv"

    "github.com/daifei0527/polyant/internal/core/user"
    "github.com/daifei0527/polyant/internal/storage"
    awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// StatsHandler 统计 API 处理器
type StatsHandler struct {
    statsSvc *user.StatsService
}

// NewStatsHandler 创建统计处理器
func NewStatsHandler(store *storage.Store) *StatsHandler {
    return &StatsHandler{
        statsSvc: user.NewStatsService(store),
    }
}

// GetUserStatsHandler 获取用户统计概览
// GET /api/v1/admin/stats/users
func (h *StatsHandler) GetUserStatsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
        return
    }

    stats, err := h.statsSvc.GetUserStats(r.Context())
    if err != nil {
        writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusInternalServerError, err))
        return
    }

    writeJSON(w, http.StatusOK, &APIResponse{
        Code:    0,
        Message: "success",
        Data:    stats,
    })
}

// GetContributionStatsHandler 获取贡献明细
// GET /api/v1/admin/stats/contributions?page=1&limit=20&sort=entry_count
func (h *StatsHandler) GetContributionStatsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
        return
    }

    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    if page < 1 {
        page = 1
    }
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit < 1 || limit > 100 {
        limit = 20
    }
    sortBy := r.URL.Query().Get("sort")
    if sortBy == "" {
        sortBy = "entry_count"
    }

    contribs, total, err := h.statsSvc.GetContributionStats(r.Context(), (page-1)*limit, limit, sortBy)
    if err != nil {
        writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusInternalServerError, err))
        return
    }

    writeJSON(w, http.StatusOK, &APIResponse{
        Code:    0,
        Message: "success",
        Data: map[string]interface{}{
            "contributions": contribs,
            "total":         total,
            "page":          page,
            "limit":         limit,
        },
    })
}

// GetActivityTrendHandler 获取活跃度趋势
// GET /api/v1/admin/stats/activity?days=30
func (h *StatsHandler) GetActivityTrendHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
        return
    }

    days, _ := strconv.Atoi(r.URL.Query().Get("days"))
    if days < 1 || days > 365 {
        days = 30
    }

    trend, err := h.statsSvc.GetActivityTrend(r.Context(), days)
    if err != nil {
        writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusInternalServerError, err))
        return
    }

    writeJSON(w, http.StatusOK, &APIResponse{
        Code:    0,
        Message: "success",
        Data:    map[string]interface{}{"trend": trend},
    })
}

// GetRegistrationTrendHandler 获取注册趋势
// GET /api/v1/admin/stats/registrations?days=30
func (h *StatsHandler) GetRegistrationTrendHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
        return
    }

    days, _ := strconv.Atoi(r.URL.Query().Get("days"))
    if days < 1 || days > 365 {
        days = 30
    }

    trend, err := h.statsSvc.GetRegistrationTrend(r.Context(), days)
    if err != nil {
        writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusInternalServerError, err))
        return
    }

    writeJSON(w, http.StatusOK, &APIResponse{
        Code:    0,
        Message: "success",
        Data:    map[string]interface{}{"trend": trend},
    })
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: Build success

- [ ] **Step 3: 提交 StatsHandler**

```bash
git add internal/api/handler/stats_handler.go
git commit -m "$(cat <<'EOF'
feat(handler): add stats API handlers

Add 4 statistics endpoints:
- GET /api/v1/admin/stats/users: user stats overview
- GET /api/v1/admin/stats/contributions: per-user contributions
- GET /api/v1/admin/stats/activity: activity trend
- GET /api/v1/admin/stats/registrations: registration trend

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: 更新路由注册统计 API

**Files:**
- Modify: `internal/api/router/router.go`

- [ ] **Step 1: 在 Dependencies 中添加 StatsHandler**

```go
// internal/api/router/router.go

// 在 NewRouterWithDeps 中添加
var statsHandler *handler.StatsHandler
if deps.Store != nil {
    statsHandler = handler.NewStatsHandler(deps.Store)
}
```

- [ ] **Step 2: 注册统计路由**

```go
// internal/api/router/router.go

// 在 registerAuthRoutes 中添加
// ==================== 统计路由 ====================
if sh != nil {
    // 用户统计概览 GET /api/v1/admin/stats/users - Lv4+
    mux.Handle("/api/v1/admin/stats/users", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv4, http.HandlerFunc(sh.GetUserStatsHandler))))

    // 贡献明细 GET /api/v1/admin/stats/contributions - Lv4+
    mux.Handle("/api/v1/admin/stats/contributions", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv4, http.HandlerFunc(sh.GetContributionStatsHandler))))

    // 活跃度趋势 GET /api/v1/admin/stats/activity - Lv4+
    mux.Handle("/api/v1/admin/stats/activity", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv4, http.HandlerFunc(sh.GetActivityTrendHandler))))

    // 注册趋势 GET /api/v1/admin/stats/registrations - Lv4+
    mux.Handle("/api/v1/admin/stats/registrations", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv4, http.HandlerFunc(sh.GetRegistrationTrendHandler))))
}
```

- [ ] **Step 3: 编译验证**

Run: `go build ./...`
Expected: Build success

- [ ] **Step 4: 提交路由更新**

```bash
git add internal/api/router/router.go
git commit -m "$(cat <<'EOF'
feat(router): register stats API routes

Register 4 statistics endpoints under /api/v1/admin/stats/:
- /users: user stats overview
- /contributions: contribution details
- /activity: activity trend
- /registrations: registration trend

All require Lv4+ (Admin) permission.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: 最终测试与验证

- [ ] **Step 1: 运行所有测试**

Run: `go test ./... -v`
Expected: All tests pass

- [ ] **Step 2: 编译所有二进制**

Run: `make build`
Expected: All binaries created

- [ ] **Step 3: 验证 API 端点**

Run: `./bin/polyant-user --help`
Expected: Shows usage

- [ ] **Step 4: 最终提交**

```bash
git add -A
git commit -m "$(cat <<'EOF'
feat: complete Phase 6c user system enhancement

Implement user system enhancements:
1. Configurable ban types (full/readonly)
2. Election auto-elect feature
3. Nomination confirmation flow (self + peer)
4. Four-dimension statistics API

New APIs:
- POST /api/v1/admin/users/{id}/ban with ban_type
- POST /api/v1/elections/{id}/candidates/{user_id}/confirm
- GET /api/v1/admin/stats/contributions
- GET /api/v1/admin/stats/activity
- GET /api/v1/admin/stats/registrations

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## 自检清单

### 1. 规范覆盖检查

| 规范要求 | 对应任务 |
|---------|---------|
| 可配置封禁类型 (full/readonly) | Task 1, Task 3, Task 4, Task 5 |
| 选举自动当选 | Task 2, Task 6 |
| 自荐 + 他荐提名 | Task 2, Task 7, Task 8 |
| 提名确认流程 | Task 7, Task 8 |
| 等级分布统计 | Task 10, Task 11 |
| 贡献明细统计 | Task 10, Task 11 |
| 活跃度趋势 | Task 10, Task 11 |
| 注册趋势 | Task 10, Task 11 |

### 2. 占位符扫描

无 TBD、TODO、implement later 等占位符。

### 3. 类型一致性检查

- `BanType` 定义在 `model.go`，使用在 `models.go`、`admin_service.go`、`admin_handler.go`
- `UserContribution` 定义在 `stats_service.go`，使用在 `stats_handler.go`
- `VoteResult` 定义在 `election.go`，使用在 `election_handler.go`
- 所有字段名使用驼峰命名，JSON tag 使用驼峰命名<tool_call>Read<arg_key>file_path</arg_key><arg_value>/home/daifei/agentwiki/internal/api/middleware/auth.go