# Web 管理页面实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Polyant 添加 Web 管理页面和 CLI 管理工具，实现对用户、内容、数据的可视化管理。

**Architecture:**
- Vue 3 + Vite + Element Plus 前端，静态文件嵌入 Go 二进制
- 管理页面仅限本地访问 (127.0.0.1)，使用 Ed25519 → Session Token 混合认证
- CLI 工具从 awctl 更名为 pactl，实现与 Web 管理页面对等的功能

**Tech Stack:** Vue 3, Vite, Element Plus, Go embed.FS, Ed25519, Session Token

---

## 文件结构

### 新建文件

```
web/admin/                          # Vue 前端项目
├── public/
│   └── favicon.ico
├── src/
│   ├── main.js                     # 入口
│   ├── App.vue                     # 根组件
│   ├── router/
│   │   └── index.js                # 路由配置 (懒加载)
│   ├── stores/
│   │   ├── admin.js                # 用户/权限状态
│   │   └── app.js                  # 应用状态
│   ├── api/
│   │   ├── request.js              # Axios 封装
│   │   ├── session.js              # 会话 API
│   │   ├── users.js                # 用户 API
│   │   ├── entries.js              # 条目 API
│   │   └── stats.js                # 统计 API
│   ├── views/
│   │   ├── Login.vue               # 登录页
│   │   ├── Layout.vue              # 管理布局
│   │   ├── users/
│   │   │   ├── List.vue            # 用户列表
│   │   │   └── Detail.vue          # 用户详情
│   │   ├── entries/
│   │   │   ├── List.vue            # 条目列表
│   │   │   └── Detail.vue          # 条目详情
│   │   └── stats/
│   │       └── Index.vue           # 统计首页
│   ├── components/
│   │   ├── PermissionGuard.vue     # 权限守卫
│   │   ├── Sidebar.vue             # 侧边栏
│   │   └── Header.vue              # 顶栏
│   └── styles/
│       ├── index.scss              # 全局样式
│       └── variables.scss          # 变量定义
├── index.html
├── vite.config.js
├── package.json
└── .env

internal/api/admin/                 # Admin API 处理器
├── handler.go                      # Admin 处理器
├── session.go                      # 会话管理
├── middleware.go                   # 权限中间件
└── static.go                       # 静态文件服务 (embed.FS)

cmd/pactl/                          # CLI 工具 (awctl 更名)
├── main.go                         # 入口
├── client.go                       # API 客户端
├── user.go                         # 用户命令
├── entry.go                        # 条目命令
├── category.go                     # 分类命令
├── sync.go                         # 同步命令
├── key.go                          # 密钥命令
├── status.go                       # 状态命令
├── service.go                      # 服务命令
├── admin.go                        # 管理命令 (新增)
├── admin_user.go                   # 用户管理命令 (新增)
├── admin_entry.go                  # 条目管理命令 (新增)
├── admin_stats.go                  # 统计命令 (新增)
└── process_*.go                    # 平台特定文件

internal/core/admin/                # 管理核心逻辑
└── session.go                      # Session Token 管理
```

### 修改文件

```
internal/api/router/router.go       # 注册 admin 路由
cmd/polyant/main.go                 # 添加 Admin 服务
pkg/config/config.go                # 添加 Admin 配置
Makefile                            # 添加 build-admin 目标
```

---

## Phase 1: 基础设施

### Task 1.1: 创建 Admin 配置结构

**Files:**
- Modify: `pkg/config/config.go`
- Test: `pkg/config/config_test.go`

- [ ] **Step 1: 添加 Admin 配置结构**

```go
// pkg/config/config.go 中添加

// AdminConfig 管理页面配置
type AdminConfig struct {
    Enabled bool   `json:"enabled" mapstructure:"enabled"`           // 是否启用管理页面
    Listen  string `json:"listen" mapstructure:"listen"`             // 监听地址，默认 127.0.0.1:18531
}

// 在 Config 结构体中添加
type Config struct {
    // ... 现有字段 ...
    Admin AdminConfig `json:"admin" mapstructure:"admin"` // 管理页面配置
}

// 在 DefaultConfig 中添加默认值
func DefaultConfig() *Config {
    return &Config{
        // ... 现有默认值 ...
        Admin: AdminConfig{
            Enabled: true,
            Listen:  "127.0.0.1:18531",
        },
    }
}
```

- [ ] **Step 2: 运行测试验证配置**

Run: `go test -v ./pkg/config/...`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add pkg/config/config.go
git commit -m "feat: add admin config structure"
```

---

### Task 1.2: 创建 Session Token 管理器

**Files:**
- Create: `internal/core/admin/session.go`
- Create: `internal/core/admin/session_test.go`

- [ ] **Step 1: 编写 Session 管理器测试**

```go
// internal/core/admin/session_test.go
package admin

import (
    "testing"
    "time"
)

func TestSessionManager_CreateSession(t *testing.T) {
    sm := NewSessionManager(time.Hour)
    publicKey := "test-pub-key-123"

    token, err := sm.CreateSession(publicKey)
    if err != nil {
        t.Fatalf("CreateSession failed: %v", err)
    }
    if token == "" {
        t.Fatal("token should not be empty")
    }
}

func TestSessionManager_ValidateSession(t *testing.T) {
    sm := NewSessionManager(time.Hour)
    publicKey := "test-pub-key-123"

    token, _ := sm.CreateSession(publicKey)

    pk, valid := sm.ValidateSession(token)
    if !valid {
        t.Fatal("session should be valid")
    }
    if pk != publicKey {
        t.Fatalf("expected %s, got %s", publicKey, pk)
    }
}

func TestSessionManager_InvalidToken(t *testing.T) {
    sm := NewSessionManager(time.Hour)

    _, valid := sm.ValidateSession("invalid-token")
    if valid {
        t.Fatal("invalid token should not be valid")
    }
}

func TestSessionManager_ExpiredSession(t *testing.T) {
    sm := NewSessionManager(100 * time.Millisecond)
    publicKey := "test-pub-key-123"

    token, _ := sm.CreateSession(publicKey)

    time.Sleep(150 * time.Millisecond)

    _, valid := sm.ValidateSession(token)
    if valid {
        t.Fatal("expired session should not be valid")
    }
}

func TestSessionManager_DeleteSession(t *testing.T) {
    sm := NewSessionManager(time.Hour)
    publicKey := "test-pub-key-123"

    token, _ := sm.CreateSession(publicKey)
    sm.DeleteSession(token)

    _, valid := sm.ValidateSession(token)
    if valid {
        t.Fatal("deleted session should not be valid")
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/core/admin/...`
Expected: FAIL - package does not exist

- [ ] **Step 3: 实现 Session 管理器**

```go
// internal/core/admin/session.go
package admin

import (
    "crypto/rand"
    "encoding/hex"
    "sync"
    "time"
)

// Session 会话信息
type Session struct {
    PublicKey string
    CreatedAt time.Time
    ExpiresAt time.Time
}

// SessionManager 会话管理器
type SessionManager struct {
    sessions map[string]*Session
    mu       sync.RWMutex
    ttl      time.Duration
}

// NewSessionManager 创建会话管理器
func NewSessionManager(ttl time.Duration) *SessionManager {
    return &SessionManager{
        sessions: make(map[string]*Session),
        ttl:      ttl,
    }
}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(publicKey string) (string, error) {
    token := generateToken()
    now := time.Now()

    sm.mu.Lock()
    defer sm.mu.Unlock()

    sm.sessions[token] = &Session{
        PublicKey: publicKey,
        CreatedAt: now,
        ExpiresAt: now.Add(sm.ttl),
    }

    return token, nil
}

// ValidateSession 验证会话
func (sm *SessionManager) ValidateSession(token string) (string, bool) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()

    session, exists := sm.sessions[token]
    if !exists {
        return "", false
    }

    if time.Now().After(session.ExpiresAt) {
        return "", false
    }

    return session.PublicKey, true
}

// DeleteSession 删除会话
func (sm *SessionManager) DeleteSession(token string) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    delete(sm.sessions, token)
}

// generateToken 生成随机 Token
func generateToken() string {
    bytes := make([]byte, 32)
    rand.Read(bytes)
    return hex.EncodeToString(bytes)
}
```

- [ ] **Step 4: 运行测试验证通过**

Run: `go test -v ./internal/core/admin/...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/core/admin/
git commit -m "feat: add session manager for admin authentication"
```

---

### Task 1.3: 创建 Admin API Handler

**Files:**
- Create: `internal/api/admin/handler.go`
- Create: `internal/api/admin/session.go`
- Create: `internal/api/admin/middleware.go`

- [ ] **Step 1: 创建 Session Handler**

```go
// internal/api/admin/session.go
package admin

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/daifei0527/polyant/internal/core/admin"
    "github.com/daifei0527/polyant/internal/storage"
    awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// SessionHandler 会话处理器
type SessionHandler struct {
    sessionMgr *admin.SessionManager
    userStore  storage.UserStore
}

// NewSessionHandler 创建会话处理器
func NewSessionHandler(sessionMgr *admin.SessionManager, userStore storage.UserStore) *SessionHandler {
    return &SessionHandler{
        sessionMgr: sessionMgr,
        userStore:  userStore,
    }
}

// CreateSessionHandler 创建会话 (仅限本地访问)
// POST /api/v1/admin/session/create
func (h *SessionHandler) CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        writeAdminError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
        return
    }

    // 检查是否为本地访问
    if !isLocalRequest(r) {
        writeAdminError(w, awerrors.New(403, awerrors.CategoryAPI, "仅限本地访问", http.StatusForbidden))
        return
    }

    // 解析请求
    var req struct {
        PublicKey string `json:"public_key"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeAdminError(w, awerrors.ErrJSONParse)
        return
    }

    if req.PublicKey == "" {
        writeAdminError(w, awerrors.ErrInvalidParams)
        return
    }

    // 验证用户是否存在
    user, err := h.userStore.Get(r.Context(), req.PublicKey)
    if err != nil {
        writeAdminError(w, awerrors.ErrUserNotFound)
        return
    }

    // 创建 Session Token
    token, err := h.sessionMgr.CreateSession(user.PublicKey)
    if err != nil {
        writeAdminError(w, awerrors.New(500, awerrors.CategoryAPI, "创建会话失败", http.StatusInternalServerError))
        return
    }

    writeAdminJSON(w, http.StatusOK, map[string]interface{}{
        "code":    0,
        "message": "success",
        "data": map[string]interface{}{
            "token":      token,
            "expires_at": time.Now().Add(24 * time.Hour).UnixMilli(),
            "user": map[string]interface{}{
                "public_key":  user.PublicKey,
                "agent_name":  user.AgentName,
                "user_level":  user.UserLevel,
            },
        },
    })
}

// isLocalRequest 检查是否为本地请求
func isLocalRequest(r *http.Request) bool {
    host := r.Host
    // 检查 Host 是否为 127.0.0.1 或 localhost
    return host == "127.0.0.1:18531" ||
           host == "localhost:18531" ||
           r.RemoteAddr == "127.0.0.1" ||
           r.RemoteAddr == "[::1]"
}

func writeAdminJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func writeAdminError(w http.ResponseWriter, err *awerrors.AWError) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(err.HTTPStatus)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "code":    err.Code,
        "message": err.Message,
    })
}
```

- [ ] **Step 2: 创建 Admin Middleware**

```go
// internal/api/admin/middleware.go
package admin

import (
    "encoding/json"
    "net/http"
    "strings"
)

// AuthMiddleware Admin 认证中间件
type AuthMiddleware struct {
    sessionMgr *SessionManager
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(sessionMgr *SessionManager) *AuthMiddleware {
    return &AuthMiddleware{sessionMgr: sessionMgr}
}

// Middleware 验证 Session Token
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 从 Header 获取 Token
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            writeAdminError(w, awerrors.ErrMissingAuth)
            return
        }

        // 解析 Bearer Token
        if !strings.HasPrefix(authHeader, "Bearer ") {
            writeAdminError(w, awerrors.ErrInvalidSignature)
            return
        }
        token := strings.TrimPrefix(authHeader, "Bearer ")

        // 验证 Token
        publicKey, valid := m.sessionMgr.ValidateSession(token)
        if !valid {
            writeAdminError(w, awerrors.New(401, awerrors.CategoryAPI, "会话已过期", http.StatusUnauthorized))
            return
        }

        // 将公钥注入上下文
        ctx := context.WithValue(r.Context(), "public_key", publicKey)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// LocalOnlyMiddleware 限制仅本地访问
func LocalOnlyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !isLocalRequest(r) {
            writeAdminError(w, awerrors.New(403, awerrors.CategoryAPI, "仅限本地访问", http.StatusForbidden))
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

- [ ] **Step 3: 创建 Admin Handler (复用现有逻辑)**

```go
// internal/api/admin/handler.go
package admin

import (
    "net/http"

    "github.com/daifei0527/polyant/internal/api/handler"
    "github.com/daifei0527/polyant/internal/storage"
)

// Handler Admin API 处理器
type Handler struct {
    adminHandler *handler.AdminHandler
    statsHandler *handler.StatsHandler
}

// NewHandler 创建 Admin 处理器
func NewHandler(store *storage.Store) *Handler {
    return &Handler{
        adminHandler: handler.NewAdminHandler(store),
        statsHandler: handler.NewStatsHandler(store),
    }
}

// 代理方法，复用现有 handler
func (h *Handler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
    h.adminHandler.ListUsersHandler(w, r)
}

func (h *Handler) BanUserHandler(w http.ResponseWriter, r *http.Request) {
    h.adminHandler.BanUserHandler(w, r)
}

func (h *Handler) UnbanUserHandler(w http.ResponseWriter, r *http.Request) {
    h.adminHandler.UnbanUserHandler(w, r)
}

func (h *Handler) SetUserLevelHandler(w http.ResponseWriter, r *http.Request) {
    h.adminHandler.SetUserLevelHandler(w, r)
}

func (h *Handler) GetUserStatsHandler(w http.ResponseWriter, r *http.Request) {
    h.adminHandler.GetUserStatsHandler(w, r)
}

func (h *Handler) GetContributionStatsHandler(w http.ResponseWriter, r *http.Request) {
    h.statsHandler.GetContributionStatsHandler(w, r)
}

func (h *Handler) GetActivityTrendHandler(w http.ResponseWriter, r *http.Request) {
    h.statsHandler.GetActivityTrendHandler(w, r)
}

func (h *Handler) GetRegistrationTrendHandler(w http.ResponseWriter, r *http.Request) {
    h.statsHandler.GetRegistrationTrendHandler(w, r)
}
```

- [ ] **Step 4: 提交**

```bash
git add internal/api/admin/
git commit -m "feat: add admin API handlers and session middleware"
```

---

### Task 1.4: 创建静态文件嵌入处理器

**Files:**
- Create: `internal/api/admin/static.go`

- [ ] **Step 1: 创建静态文件处理器**

```go
// internal/api/admin/static.go
package admin

import (
    "embed"
    "io/fs"
    "net/http"
    "strings"
)

//go:embed dist
var adminFS embed.FS

// StaticHandler 静态文件处理器
type StaticHandler struct {
    fileServer http.Handler
}

// NewStaticHandler 创建静态文件处理器
func NewStaticHandler() *StaticHandler {
    // 获取 dist 子目录
    distFS, err := fs.Sub(adminFS, "dist")
    if err != nil {
        panic(err)
    }

    return &StaticHandler{
        fileServer: http.FileServer(http.FS(distFS)),
    }
}

// ServeHTTP 处理静态文件请求
// 对于 SPA 应用，所有非文件请求返回 index.html
func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path

    // 移除 /admin 前缀
    path = strings.TrimPrefix(path, "/admin")
    if path == "" || path == "/" {
        path = "/index.html"
    }

    // 更新请求路径
    r.URL.Path = path

    // 检查文件是否存在
    if _, err := fs.Stat(adminFS, "dist"+path); err != nil {
        // 文件不存在，返回 index.html (SPA 路由)
        r.URL.Path = "/index.html"
    }

    h.fileServer.ServeHTTP(w, r)
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/api/admin/static.go
git commit -m "feat: add static file handler with embed.FS support"
```

---

### Task 1.5: 注册 Admin 路由

**Files:**
- Modify: `internal/api/router/router.go`

- [ ] **Step 1: 添加 Admin 路由注册函数**

```go
// internal/api/router/router.go 中添加

import (
    // ... 现有 imports ...
    "github.com/daifei0527/polyant/internal/api/admin"
    coreadmin "github.com/daifei0527/polyant/internal/core/admin"
)

// 在 Dependencies 中添加
type Dependencies struct {
    // ... 现有字段 ...
    SessionManager *coreadmin.SessionManager
}

// 在 NewRouterWithDeps 中添加
func NewRouterWithDeps(deps *Dependencies) (http.Handler, error) {
    mux := http.NewServeMux()

    // ... 现有代码 ...

    // 创建 Session Manager
    var sessionMgr *coreadmin.SessionManager
    if deps.SessionManager != nil {
        sessionMgr = deps.SessionManager
    } else {
        sessionMgr = coreadmin.NewSessionManager(24 * time.Hour)
    }

    // 注册 Admin 路由
    if deps.AdminEnabled {
        registerAdminRoutes(mux, deps, sessionMgr)
    }

    // ... 现有中间件 ...
}

// registerAdminRoutes 注册 Admin API 路由
func registerAdminRoutes(mux *http.ServeMux, deps *Dependencies, sessionMgr *coreadmin.SessionManager) {
    // 创建 handlers
    sessionHandler := admin.NewSessionHandler(sessionMgr, deps.UserStore)
    adminHandler := admin.NewHandler(deps.Store)
    adminAuthMW := admin.NewAuthMiddleware(sessionMgr)

    // Session API (仅本地访问)
    mux.Handle("/api/v1/admin/session/create",
        admin.LocalOnlyMiddleware(http.HandlerFunc(sessionHandler.CreateSessionHandler)))

    // Admin API (需要 Session Token 认证)
    // 用户管理
    mux.Handle("/api/v1/admin/users",
        adminAuthMW.Middleware(http.HandlerFunc(adminHandler.ListUsersHandler)))
    mux.Handle("/api/v1/admin/users/",
        adminAuthMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            path := r.URL.Path
            if strings.HasSuffix(path, "/ban") {
                adminHandler.BanUserHandler(w, r)
            } else if strings.HasSuffix(path, "/unban") {
                adminHandler.UnbanUserHandler(w, r)
            } else if strings.HasSuffix(path, "/level") {
                adminHandler.SetUserLevelHandler(w, r)
            } else {
                http.NotFound(w, r)
            }
        })))

    // 统计 API
    mux.Handle("/api/v1/admin/stats/users",
        adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetUserStatsHandler)))
    mux.Handle("/api/v1/admin/stats/contributions",
        adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetContributionStatsHandler)))
    mux.Handle("/api/v1/admin/stats/activity",
        adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetActivityTrendHandler)))
    mux.Handle("/api/v1/admin/stats/registrations",
        adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetRegistrationTrendHandler)))

    // 静态文件服务 (管理页面)
    staticHandler := admin.NewStaticHandler()
    mux.Handle("/admin/", http.StripPrefix("/admin", staticHandler))
    mux.Handle("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently))
}
```

- [ ] **Step 2: 运行测试**

Run: `go test -v ./internal/api/router/...`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/router/router.go
git commit -m "feat: register admin routes with session authentication"
```

---

## Phase 2: CLI 工具 (awctl → pactl)

### Task 2.1: 重命名 CLI 目录

**Files:**
- Rename: `cmd/awctl/` → `cmd/pactl/`

- [ ] **Step 1: 复制 awctl 到 pactl**

```bash
cp -r cmd/awctl cmd/pactl
```

- [ ] **Step 2: 修改 main.go 中的命令名称**

```go
// cmd/pactl/main.go
var rootCmd = &cobra.Command{
    Use:   "pactl",
    Short: "Polyant 管理工具",
    Long: `pactl 是 Polyant 的命令行管理工具。

用于管理知识库条目、用户、同步等功能。

示例:
  pactl key generate              生成密钥对
  pactl user register --name "my-agent"  注册用户
  pactl status                    查看服务器状态
  pactl search "人工智能"          搜索条目
  pactl entry get <id>            获取条目详情
  pactl admin users list          列出用户 (管理员)`,
    Version: version,
    // ... 其他代码不变 ...
}
```

- [ ] **Step 3: 更新所有命令中的 awctl 引用**

```bash
# 在 cmd/pactl/ 目录下替换所有 awctl 为 pactl
sed -i 's/awctl/pactl/g' cmd/pactl/*.go
```

- [ ] **Step 4: 运行测试**

Run: `go test -v ./cmd/pactl/...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add cmd/pactl/
git commit -m "feat: rename awctl to pactl CLI tool"
```

---

### Task 2.2: 添加 admin 命令组

**Files:**
- Create: `cmd/pactl/admin.go`
- Create: `cmd/pactl/admin_user.go`
- Create: `cmd/pactl/admin_entry.go`
- Create: `cmd/pactl/admin_stats.go`

- [ ] **Step 1: 创建 admin 根命令**

```go
// cmd/pactl/admin.go
package main

import (
    "github.com/spf13/cobra"
)

// adminCmd 管理员命令组
var adminCmd = &cobra.Command{
    Use:   "admin",
    Short: "管理员操作",
    Long: `管理员操作命令。

需要管理员 (Lv4+) 或超级管理员 (Lv5) 权限。

子命令:
  users    用户管理
  entries  内容审核
  stats    数据统计
  status   系统状态`,
}

func init() {
    rootCmd.AddCommand(adminCmd)
}
```

- [ ] **Step 2: 创建用户管理命令**

```go
// cmd/pactl/admin_user.go
package main

import (
    "context"
    "fmt"
    "os"
    "text/tabwriter"
    "time"

    "github.com/spf13/cobra"
)

// adminUsersCmd 用户管理命令组
var adminUsersCmd = &cobra.Command{
    Use:   "users",
    Short: "用户管理",
    Long:  "用户管理操作，包括列出、封禁、设置等级等",
}

// adminUsersListCmd 列出用户
var adminUsersListCmd = &cobra.Command{
    Use:   "list",
    Short: "列出用户",
    RunE: func(cmd *cobra.Command, args []string) error {
        level, _ := cmd.Flags().GetInt32("level")
        limit, _ := cmd.Flags().GetInt("limit")
        search, _ := cmd.Flags().GetString("search")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        users, total, err := client.ListUsers(ctx, 1, limit, level, search)
        if err != nil {
            return fmt.Errorf("获取用户列表失败: %w", err)
        }

        fmt.Printf("用户列表 (共 %d 个):\n\n", total)

        w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(w, "公钥\t名称\t等级\t状态\t贡献数\t评分数")
        fmt.Fprintln(w, "----\t----\t----\t----\t------\t------")

        for _, u := range users {
            pubKey := u.PublicKey
            if len(pubKey) > 20 {
                pubKey = pubKey[:20] + "..."
            }
            fmt.Fprintf(w, "%s\t%s\tLv%d\t%s\t%d\t%d\n",
                pubKey, u.AgentName, u.UserLevel, u.Status, u.ContributionCnt, u.RatingCnt)
        }
        w.Flush()

        return nil
    },
}

// adminUsersBanCmd 封禁用户
var adminUsersBanCmd = &cobra.Command{
    Use:   "ban <public-key>",
    Short: "封禁用户",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        pubKey := args[0]
        reason, _ := cmd.Flags().GetString("reason")
        banType, _ := cmd.Flags().GetString("type")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := client.BanUser(ctx, pubKey, reason, banType); err != nil {
            return fmt.Errorf("封禁用户失败: %w", err)
        }

        fmt.Printf("用户 %s 已被封禁\n", pubKey[:min(20, len(pubKey))])
        return nil
    },
}

// adminUsersUnbanCmd 解封用户
var adminUsersUnbanCmd = &cobra.Command{
    Use:   "unban <public-key>",
    Short: "解封用户",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        pubKey := args[0]

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := client.UnbanUser(ctx, pubKey); err != nil {
            return fmt.Errorf("解封用户失败: %w", err)
        }

        fmt.Printf("用户 %s 已解封\n", pubKey[:min(20, len(pubKey))])
        return nil
    },
}

// adminUsersLevelCmd 设置用户等级
var adminUsersLevelCmd = &cobra.Command{
    Use:   "level <public-key> <level>",
    Short: "设置用户等级 (需要 Lv5)",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        pubKey := args[0]
        level := args[1]
        reason, _ := cmd.Flags().GetString("reason")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := client.SetUserLevel(ctx, pubKey, parseLevel(level), reason); err != nil {
            return fmt.Errorf("设置等级失败: %w", err)
        }

        fmt.Printf("用户 %s 等级已设置为 %s\n", pubKey[:min(20, len(pubKey))], level)
        return nil
    },
}

func init() {
    adminCmd.AddCommand(adminUsersCmd)

    adminUsersCmd.AddCommand(adminUsersListCmd)
    adminUsersListCmd.Flags().Int32("level", -1, "按等级过滤")
    adminUsersListCmd.Flags().IntP("limit", "l", 20, "结果数量限制")
    adminUsersListCmd.Flags().String("search", "", "搜索关键词")

    adminUsersCmd.AddCommand(adminUsersBanCmd)
    adminUsersBanCmd.Flags().String("reason", "", "封禁原因")
    adminUsersBanCmd.Flags().String("type", "full", "封禁类型 (full/readonly)")

    adminUsersCmd.AddCommand(adminUsersUnbanCmd)

    adminUsersCmd.AddCommand(adminUsersLevelCmd)
    adminUsersLevelCmd.Flags().String("reason", "", "设置原因")
}

func parseLevel(s string) int32 {
    levels := map[string]int32{
        "0": 0, "lv0": 0, "Lv0": 0,
        "1": 1, "lv1": 1, "Lv1": 1,
        "2": 2, "lv2": 2, "Lv2": 2,
        "3": 3, "lv3": 3, "Lv3": 3,
        "4": 4, "lv4": 4, "Lv4": 4,
        "5": 5, "lv5": 5, "Lv5": 5,
    }
    if l, ok := levels[s]; ok {
        return l
    }
    return -1
}
```

- [ ] **Step 3: 添加 Client 方法**

```go
// cmd/pactl/client.go 中添加

// BanUser 封禁用户
func (c *Client) BanUser(ctx context.Context, publicKey, reason, banType string) error {
    req := map[string]interface{}{
        "reason":   reason,
        "ban_type": banType,
    }
    path := fmt.Sprintf("/api/v1/admin/users/%s/ban", publicKey)
    var resp APIResponse
    return c.doRequestWithAuth(ctx, http.MethodPost, path, req, &resp, true)
}

// UnbanUser 解封用户
func (c *Client) UnbanUser(ctx context.Context, publicKey string) error {
    path := fmt.Sprintf("/api/v1/admin/users/%s/unban", publicKey)
    var resp APIResponse
    return c.doRequestWithAuth(ctx, http.MethodPost, path, nil, &resp, true)
}
```

- [ ] **Step 4: 创建统计命令**

```go
// cmd/pactl/admin_stats.go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/spf13/cobra"
)

// adminStatsCmd 统计命令组
var adminStatsCmd = &cobra.Command{
    Use:   "stats",
    Short: "数据统计",
}

// adminStatsUsersCmd 用户统计
var adminStatsUsersCmd = &cobra.Command{
    Use:   "users",
    Short: "用户统计",
    RunE: func(cmd *cobra.Command, args []string) error {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        stats, err := client.GetUserStats(ctx)
        if err != nil {
            return fmt.Errorf("获取用户统计失败: %w", err)
        }

        fmt.Println("用户统计:")
        fmt.Printf("  总用户数: %d\n", stats.Total)
        fmt.Println("  等级分布:")
        for _, l := range stats.LevelDistribution {
            fmt.Printf("    Lv%d: %d\n", l.Level, l.Count)
        }
        return nil
    },
}

// adminStatsActivityCmd 活跃趋势
var adminStatsActivityCmd = &cobra.Command{
    Use:   "activity",
    Short: "活跃趋势",
    RunE: func(cmd *cobra.Command, args []string) error {
        days, _ := cmd.Flags().GetInt("days")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        trend, err := client.GetActivityTrend(ctx, days)
        if err != nil {
            return fmt.Errorf("获取活跃趋势失败: %w", err)
        }

        fmt.Printf("活跃趋势 (近 %d 天):\n", days)
        for _, d := range trend {
            fmt.Printf("  %s: %d 活跃用户\n", d.Date, d.ActiveUsers)
        }
        return nil
    },
}

func init() {
    adminCmd.AddCommand(adminStatsCmd)
    adminStatsCmd.AddCommand(adminStatsUsersCmd)
    adminStatsCmd.AddCommand(adminStatsActivityCmd)
    adminStatsActivityCmd.Flags().Int("days", 7, "统计天数")
}
```

- [ ] **Step 5: 运行测试**

Run: `go test -v ./cmd/pactl/...`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add cmd/pactl/admin*.go
git commit -m "feat: add admin commands to pactl CLI"
```

---

## Phase 3: Web 管理页面

### Task 3.1: 初始化 Vue 项目

**Files:**
- Create: `web/admin/package.json`
- Create: `web/admin/vite.config.js`
- Create: `web/admin/index.html`

- [ ] **Step 1: 创建 package.json**

```json
{
  "name": "polyant-admin",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.4.0",
    "vue-router": "^4.2.5",
    "pinia": "^2.1.7",
    "axios": "^1.6.2",
    "element-plus": "^2.4.4"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^4.5.2",
    "vite": "^5.0.10",
    "sass": "^1.69.5"
  }
}
```

- [ ] **Step 2: 创建 vite.config.js**

```javascript
// web/admin/vite.config.js
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src')
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  },
  base: '/admin/'
})
```

- [ ] **Step 3: 创建 index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Polyant 管理后台</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.js"></script>
</body>
</html>
```

- [ ] **Step 4: 提交**

```bash
git add web/admin/
git commit -m "feat: initialize Vue 3 admin project"
```

---

### Task 3.2: 创建 Vue 应用入口和路由

**Files:**
- Create: `web/admin/src/main.js`
- Create: `web/admin/src/App.vue`
- Create: `web/admin/src/router/index.js`

- [ ] **Step 1: 创建 main.js**

```javascript
// web/admin/src/main.js
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'

import App from './App.vue'
import router from './router'
import './styles/index.scss'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })

app.mount('#app')
```

- [ ] **Step 2: 创建 App.vue**

```vue
<!-- web/admin/src/App.vue -->
<template>
  <router-view />
</template>

<script setup>
</script>

<style>
#app {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
</style>
```

- [ ] **Step 3: 创建路由配置**

```javascript
// web/admin/src/router/index.js
import { createRouter, createWebHistory } from 'vue-router'
import { useAdminStore } from '@/stores/admin'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        redirect: '/stats'
      },
      {
        path: 'stats',
        name: 'Stats',
        component: () => import('@/views/stats/Index.vue'),
        meta: { permission: 4, title: '数据统计' }
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('@/views/users/List.vue'),
        meta: { permission: 4, title: '用户管理' }
      },
      {
        path: 'users/:publicKey',
        name: 'UserDetail',
        component: () => import('@/views/users/Detail.vue'),
        meta: { permission: 4, title: '用户详情' }
      },
      {
        path: 'entries',
        name: 'Entries',
        component: () => import('@/views/entries/List.vue'),
        meta: { permission: 4, title: '内容审核' }
      },
      {
        path: 'entries/:id',
        name: 'EntryDetail',
        component: () => import('@/views/entries/Detail.vue'),
        meta: { permission: 4, title: '条目详情' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  const adminStore = useAdminStore()

  if (to.meta.requiresAuth !== false && !adminStore.isLoggedIn) {
    next('/login')
    return
  }

  // 权限检查
  if (to.meta.permission && adminStore.userLevel < to.meta.permission) {
    next('/stats') // 跳转到权限允许的页面
    return
  }

  next()
})

export default router
```

- [ ] **Step 4: 提交**

```bash
git add web/admin/src/
git commit -m "feat: add Vue app entry and router config"
```

---

### Task 3.3: 创建状态管理

**Files:**
- Create: `web/admin/src/stores/admin.js`
- Create: `web/admin/src/stores/app.js`

- [ ] **Step 1: 创建 admin store**

```javascript
// web/admin/src/stores/admin.js
import { defineStore } from 'pinia'
import { createSession, getCurrentUser } from '@/api/session'

export const useAdminStore = defineStore('admin', {
  state: () => ({
    token: sessionStorage.getItem('admin_token') || '',
    user: null,
    userLevel: 0
  }),

  getters: {
    isLoggedIn: (state) => !!state.token,
    publicKey: (state) => state.user?.public_key || ''
  },

  actions: {
    async login(publicKey) {
      try {
        const res = await createSession(publicKey)
        this.token = res.token
        this.user = res.user
        this.userLevel = res.user.user_level
        sessionStorage.setItem('admin_token', res.token)
        return true
      } catch (error) {
        console.error('Login failed:', error)
        return false
      }
    },

    logout() {
      this.token = ''
      this.user = null
      this.userLevel = 0
      sessionStorage.removeItem('admin_token')
    },

    hasPermission(level) {
      return this.userLevel >= level
    }
  }
})
```

- [ ] **Step 2: 提交**

```bash
git add web/admin/src/stores/
git commit -m "feat: add admin store for auth state"
```

---

### Task 3.4: 创建 API 请求封装

**Files:**
- Create: `web/admin/src/api/request.js`
- Create: `web/admin/src/api/session.js`
- Create: `web/admin/src/api/users.js`
- Create: `web/admin/src/api/stats.js`

- [ ] **Step 1: 创建请求封装**

```javascript
// web/admin/src/api/request.js
import axios from 'axios'
import { ElMessage } from 'element-plus'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 30000
})

// 请求拦截器
request.interceptors.request.use(
  (config) => {
    const token = sessionStorage.getItem('admin_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// 响应拦截器
request.interceptors.response.use(
  (response) => {
    const { data } = response
    if (data.code !== 0) {
      ElMessage.error(data.message || '请求失败')
      return Promise.reject(new Error(data.message))
    }
    return data.data
  },
  (error) => {
    if (error.response?.status === 401) {
      sessionStorage.removeItem('admin_token')
      window.location.href = '/admin/login'
    }
    ElMessage.error(error.response?.data?.message || '网络错误')
    return Promise.reject(error)
  }
)

export default request
```

- [ ] **Step 2: 创建 session API**

```javascript
// web/admin/src/api/session.js
import request from './request'

export function createSession(publicKey) {
  return request.post('/admin/session/create', { public_key: publicKey })
}

export function getCurrentUser() {
  return request.get('/user/info')
}
```

- [ ] **Step 3: 创建 users API**

```javascript
// web/admin/src/api/users.js
import request from './request'

export function listUsers(params) {
  return request.get('/admin/users', { params })
}

export function banUser(publicKey, reason, banType = 'full') {
  return request.post(`/admin/users/${publicKey}/ban`, { reason, ban_type: banType })
}

export function unbanUser(publicKey) {
  return request.post(`/admin/users/${publicKey}/unban`)
}

export function setUserLevel(publicKey, level, reason) {
  return request.put(`/admin/users/${publicKey}/level`, { level, reason })
}
```

- [ ] **Step 4: 创建 stats API**

```javascript
// web/admin/src/api/stats.js
import request from './request'

export function getUserStats() {
  return request.get('/admin/stats/users')
}

export function getContributionStats(params) {
  return request.get('/admin/stats/contributions', { params })
}

export function getActivityTrend(days = 30) {
  return request.get('/admin/stats/activity', { params: { days } })
}

export function getRegistrationTrend(days = 30) {
  return request.get('/admin/stats/registrations', { params: { days } })
}
```

- [ ] **Step 5: 提交**

```bash
git add web/admin/src/api/
git commit -m "feat: add API request modules"
```

---

### Task 3.5: 创建登录页面

**Files:**
- Create: `web/admin/src/views/Login.vue`

- [ ] **Step 1: 创建登录页面**

```vue
<!-- web/admin/src/views/Login.vue -->
<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <h2>Polyant 管理后台</h2>
      </template>

      <el-form :model="form" :rules="rules" ref="formRef" label-position="top">
        <el-form-item label="公钥" prop="publicKey">
          <el-input
            v-model="form.publicKey"
            type="textarea"
            :rows="3"
            placeholder="请输入您的 Ed25519 公钥"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="handleLogin" :loading="loading" style="width: 100%">
            登录
          </el-button>
        </el-form-item>
      </el-form>

      <el-divider />

      <p class="hint">
        管理后台仅限本地访问，请使用已注册的 Ed25519 公钥登录
      </p>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAdminStore } from '@/stores/admin'

const router = useRouter()
const adminStore = useAdminStore()

const formRef = ref(null)
const loading = ref(false)

const form = reactive({
  publicKey: ''
})

const rules = {
  publicKey: [
    { required: true, message: '请输入公钥', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const success = await adminStore.login(form.publicKey)
    if (success) {
      ElMessage.success('登录成功')
      router.push('/')
    } else {
      ElMessage.error('登录失败，请检查公钥是否正确')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: #f5f7fa;
}

.login-card {
  width: 400px;
}

.login-card :deep(.el-card__header) {
  text-align: center;
}

.hint {
  color: #909399;
  font-size: 12px;
  text-align: center;
}
</style>
```

- [ ] **Step 2: 提交**

```bash
git add web/admin/src/views/Login.vue
git commit -m "feat: add login page"
```

---

### Task 3.6: 创建管理布局

**Files:**
- Create: `web/admin/src/views/Layout.vue`
- Create: `web/admin/src/components/Sidebar.vue`
- Create: `web/admin/src/components/Header.vue`

- [ ] **Step 1: 创建布局组件**

```vue
<!-- web/admin/src/views/Layout.vue -->
<template>
  <el-container class="layout-container">
    <el-aside width="200px">
      <Sidebar />
    </el-aside>
    <el-container>
      <el-header>
        <Header />
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import Sidebar from '@/components/Sidebar.vue'
import Header from '@/components/Header.vue'
</script>

<style scoped>
.layout-container {
  height: 100vh;
}

.el-header {
  padding: 0;
  background: #fff;
  border-bottom: 1px solid #e6e6e6;
}

.el-aside {
  background: #304156;
}

.el-main {
  background: #f5f7fa;
  padding: 20px;
}
</style>
```

- [ ] **Step 2: 创建侧边栏**

```vue
<!-- web/admin/src/components/Sidebar.vue -->
<template>
  <div class="sidebar">
    <div class="logo">
      <h1>Polyant</h1>
    </div>
    <el-menu
      :default-active="activeMenu"
      router
      background-color="#304156"
      text-color="#bfcbd9"
      active-text-color="#409EFF"
    >
      <el-menu-item index="/stats">
        <el-icon><DataLine /></el-icon>
        <span>数据统计</span>
      </el-menu-item>
      <el-menu-item index="/users" v-if="hasPermission(4)">
        <el-icon><User /></el-icon>
        <span>用户管理</span>
      </el-menu-item>
      <el-menu-item index="/entries" v-if="hasPermission(4)">
        <el-icon><Document /></el-icon>
        <span>内容审核</span>
      </el-menu-item>
    </el-menu>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { DataLine, User, Document } from '@element-plus/icons-vue'
import { useAdminStore } from '@/stores/admin'

const route = useRoute()
const adminStore = useAdminStore()

const activeMenu = computed(() => route.path)

const hasPermission = (level) => adminStore.hasPermission(level)
</script>

<style scoped>
.sidebar {
  height: 100%;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.logo h1 {
  font-size: 18px;
  margin: 0;
}

.el-menu {
  border-right: none;
}
</style>
```

- [ ] **Step 3: 创建头部**

```vue
<!-- web/admin/src/components/Header.vue -->
<template>
  <div class="header">
    <div class="breadcrumb">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
        <el-breadcrumb-item v-if="currentTitle">{{ currentTitle }}</el-breadcrumb-item>
      </el-breadcrumb>
    </div>
    <div class="user-info">
      <el-dropdown @command="handleCommand">
        <span class="user-dropdown">
          <el-avatar :size="32" icon="UserFilled" />
          <span class="user-name">{{ userName }}</span>
        </span>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item disabled>
              Lv{{ adminStore.userLevel }}
            </el-dropdown-item>
            <el-dropdown-item divided command="logout">
              退出登录
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAdminStore } from '@/stores/admin'

const route = useRoute()
const router = useRouter()
const adminStore = useAdminStore()

const currentTitle = computed(() => route.meta?.title || '')

const userName = computed(() => adminStore.user?.agent_name || '管理员')

const handleCommand = (command) => {
  if (command === 'logout') {
    adminStore.logout()
    router.push('/login')
  }
}
</script>

<style scoped>
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 100%;
  padding: 0 20px;
}

.user-dropdown {
  display: flex;
  align-items: center;
  cursor: pointer;
}

.user-name {
  margin-left: 8px;
}
</style>
```

- [ ] **Step 4: 提交**

```bash
git add web/admin/src/views/Layout.vue web/admin/src/components/
git commit -m "feat: add admin layout with sidebar and header"
```

---

### Task 3.7: 创建用户管理页面

**Files:**
- Create: `web/admin/src/views/users/List.vue`
- Create: `web/admin/src/views/users/Detail.vue`

- [ ] **Step 1: 创建用户列表页**

```vue
<!-- web/admin/src/views/users/List.vue -->
<template>
  <div class="users-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>用户列表</span>
          <el-input
            v-model="searchText"
            placeholder="搜索用户"
            style="width: 200px"
            clearable
            @clear="fetchUsers"
            @keyup.enter="fetchUsers"
          >
            <template #append>
              <el-button icon="Search" @click="fetchUsers" />
            </template>
          </el-input>
        </div>
      </template>

      <el-table :data="users" v-loading="loading">
        <el-table-column prop="public_key" label="公钥" width="200">
          <template #default="{ row }">
            <el-tooltip :content="row.public_key" placement="top">
              <span>{{ row.public_key.slice(0, 20) }}...</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="agent_name" label="名称" width="150" />
        <el-table-column prop="user_level" label="等级" width="80">
          <template #default="{ row }">
            <el-tag :type="getLevelType(row.user_level)">
              Lv{{ row.user_level }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'">
              {{ row.status === 'active' ? '正常' : '封禁' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="contribution_cnt" label="贡献数" width="100" />
        <el-table-column prop="rating_cnt" label="评分数" width="100" />
        <el-table-column label="操作" fixed="right" width="200">
          <template #default="{ row }">
            <el-button size="small" @click="showDetail(row)">详情</el-button>
            <el-button
              v-if="row.status === 'active'"
              size="small"
              type="danger"
              @click="handleBan(row)"
            >封禁</el-button>
            <el-button
              v-else
              size="small"
              type="success"
              @click="handleUnban(row)"
            >解封</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @size-change="fetchUsers"
          @current-change="fetchUsers"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listUsers, banUser, unbanUser } from '@/api/users'

const router = useRouter()

const loading = ref(false)
const users = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const searchText = ref('')

const fetchUsers = async () => {
  loading.value = true
  try {
    const res = await listUsers({
      page: currentPage.value,
      limit: pageSize.value,
      search: searchText.value
    })
    users.value = res.users || []
    total.value = res.total || 0
  } catch (error) {
    console.error('Failed to fetch users:', error)
  } finally {
    loading.value = false
  }
}

const showDetail = (row) => {
  router.push(`/users/${row.public_key}`)
}

const handleBan = async (row) => {
  const { value: reason } = await ElMessageBox.prompt('请输入封禁原因', '封禁用户', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputPattern: /\S+/,
    inputErrorMessage: '请输入封禁原因'
  }).catch(() => ({ value: null }))

  if (!reason) return

  try {
    await banUser(row.public_key, reason)
    ElMessage.success('封禁成功')
    fetchUsers()
  } catch (error) {
    console.error('Ban failed:', error)
  }
}

const handleUnban = async (row) => {
  try {
    await unbanUser(row.public_key)
    ElMessage.success('解封成功')
    fetchUsers()
  } catch (error) {
    console.error('Unban failed:', error)
  }
}

const getLevelType = (level) => {
  const types = { 0: 'info', 1: '', 2: 'success', 3: 'warning', 4: 'danger', 5: 'danger' }
  return types[level] || 'info'
}

onMounted(() => {
  fetchUsers()
})
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
```

- [ ] **Step 2: 提交**

```bash
git add web/admin/src/views/users/
git commit -m "feat: add users list page with ban/unban actions"
```

---

### Task 3.8: 创建统计页面

**Files:**
- Create: `web/admin/src/views/stats/Index.vue`

- [ ] **Step 1: 创建统计首页**

```vue
<!-- web/admin/src/views/stats/Index.vue -->
<template>
  <div class="stats-index">
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value">{{ userStats.total || 0 }}</div>
            <div class="stat-label">总用户数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value">{{ entryStats.total || 0 }}</div>
            <div class="stat-label">总条目数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value">{{ contributionStats.total || 0 }}</div>
            <div class="stat-label">总贡献数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value">{{ ratingStats.total || 0 }}</div>
            <div class="stat-label">总评分数</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>用户等级分布</span>
          </template>
          <div v-for="item in userStats.level_distribution" :key="item.level" class="level-item">
            <span>Lv{{ item.level }}</span>
            <el-progress :percentage="getPercentage(item.count)" :stroke-width="20" />
            <span>{{ item.count }} 人</span>
          </div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>活跃趋势 (近 7 天)</span>
          </template>
          <el-table :data="activityTrend" size="small">
            <el-table-column prop="date" label="日期" width="120" />
            <el-table-column prop="active_users" label="活跃用户" />
            <el-table-column prop="new_entries" label="新增条目" />
            <el-table-column prop="new_ratings" label="新增评分" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { getUserStats, getActivityTrend, getContributionStats } from '@/api/stats'

const userStats = ref({})
const activityTrend = ref([])
const contributionStats = ref({})

// 模拟数据
const entryStats = ref({ total: 0 })
const ratingStats = ref({ total: 0 })

const fetchData = async () => {
  try {
    const [userRes, activityRes, contribRes] = await Promise.all([
      getUserStats(),
      getActivityTrend(7),
      getContributionStats({ limit: 1 })
    ])
    userStats.value = userRes || {}
    activityTrend.value = activityRes?.trend || []
    contributionStats.value = contribRes || {}
  } catch (error) {
    console.error('Failed to fetch stats:', error)
  }
}

const getPercentage = (count) => {
  const total = userStats.value.total || 1
  return Math.round((count / total) * 100)
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.stat-card {
  text-align: center;
  padding: 20px 0;
}

.stat-value {
  font-size: 36px;
  font-weight: bold;
  color: #409EFF;
}

.stat-label {
  margin-top: 10px;
  color: #909399;
}

.level-item {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
}

.level-item span:first-child {
  width: 40px;
}

.level-item span:last-child {
  width: 60px;
  text-align: right;
}

.level-item .el-progress {
  flex: 1;
  margin: 0 10px;
}
</style>
```

- [ ] **Step 2: 提交**

```bash
git add web/admin/src/views/stats/
git commit -m "feat: add stats dashboard page"
```

---

## Phase 4: 构建与测试

### Task 4.1: 更新 Makefile

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: 添加管理页面构建目标**

```makefile
# Makefile 中添加

# 管理页面构建
.PHONY: build-admin
build-admin:
	cd web/admin && npm install && npm run build

# 完整构建 (包含管理页面)
.PHONY: build-full
build-full: build-admin
	go build -tags embed_admin -o bin/polyant ./cmd/polyant
	go build -o bin/pactl ./cmd/pactl

# 仅构建核心 (不含管理页面)
.PHONY: build
build:
	go build -o bin/polyant ./cmd/polyant
	go build -o bin/pactl ./cmd/pactl

# 开发模式运行管理页面
.PHONY: dev-admin
dev-admin:
	cd web/admin && npm run dev
```

- [ ] **Step 2: 提交**

```bash
git add Makefile
git commit -m "feat: add build targets for admin dashboard"
```

---

### Task 4.2: 编写集成测试

**Files:**
- Create: `internal/api/admin/handler_test.go`

- [ ] **Step 1: 编写 Session API 测试**

```go
// internal/api/admin/handler_test.go
package admin

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    coreadmin "github.com/daifei0527/polyant/internal/core/admin"
    "github.com/daifei0527/polyant/internal/storage"
)

func TestCreateSession_LocalOnly(t *testing.T) {
    store, _ := storage.NewMemoryStore()
    sessionMgr := coreadmin.NewSessionManager(time.Hour)
    handler := NewSessionHandler(sessionMgr, store.User)

    // 测试非本地请求
    body := map[string]string{"public_key": "test"}
    jsonBody, _ := json.Marshal(body)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session/create", bytes.NewReader(jsonBody))
    req.RemoteAddr = "192.168.1.1:12345"
    w := httptest.NewRecorder()

    handler.CreateSessionHandler(w, req)

    if w.Code != http.StatusForbidden {
        t.Fatalf("expected 403, got %d", w.Code)
    }
}

func TestSessionMiddleware(t *testing.T) {
    sessionMgr := coreadmin.NewSessionManager(time.Hour)
    mw := NewAuthMiddleware(sessionMgr)

    // 创建有效 token
    token, _ := sessionMgr.CreateSession("test-user")

    // 测试有效 token
    req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    w := httptest.NewRecorder()

    called := false
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        called = true
        w.WriteHeader(http.StatusOK)
    })

    mw.Middleware(next).ServeHTTP(w, req)

    if !called {
        t.Fatal("middleware should call next handler")
    }
}
```

- [ ] **Step 2: 运行测试**

Run: `go test -v ./internal/api/admin/...`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/admin/handler_test.go
git commit -m "test: add admin handler tests"
```

---

### Task 4.3: 更新文档

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-04-15-web-admin-dashboard-design.md`

- [ ] **Step 1: 更新 README 添加管理页面说明**

```markdown
## Web 管理后台

Polyant 提供 Web 管理后台用于可视化管理。

### 访问方式

管理后台仅限本地访问，默认地址：http://127.0.0.1:18531/admin/

### 认证方式

使用 Ed25519 公钥认证，登录后获取 Session Token (有效期 24 小时)。

### 功能模块

- **数据统计**: 用户统计、贡献统计、活跃趋势
- **用户管理**: 用户列表、封禁/解封、等级设置 (Lv5)
- **内容审核**: 条目列表、删除条目

### CLI 工具

`pactl` 是 Polyant 的命令行管理工具：

```bash
# 用户管理
pactl admin users list
pactl admin users ban <public-key> --reason "违规"
pactl admin users level <public-key> 2 --reason "贡献达标"

# 统计信息
pactl admin stats users
pactl admin stats activity --days 30
```
```

- [ ] **Step 2: 提交**

```bash
git add README.md
git commit -m "docs: add admin dashboard documentation"
```

---

## 最终步骤

### Task 4.4: 完整构建验证

- [ ] **Step 1: 安装前端依赖并构建**

Run: `cd web/admin && npm install && npm run build`
Expected: dist/ 目录生成成功

- [ ] **Step 2: 构建完整二进制**

Run: `make build-full`
Expected: bin/polyant 和 bin/pactl 生成成功

- [ ] **Step 3: 运行所有测试**

Run: `make test`
Expected: 所有测试通过

- [ ] **Step 4: 最终提交**

```bash
git add .
git commit -m "feat: complete web admin dashboard implementation

- Add Vue 3 + Element Plus admin frontend
- Add Session Token authentication for admin API
- Add admin commands to pactl CLI
- Add user management, content moderation, and stats pages
- Support local-only access for security"
```

---

## 自检清单

- [ ] Session Token 管理器实现完整，支持创建、验证、删除
- [ ] Admin API 仅限本地访问 (127.0.0.1)
- [ ] CLI 工具 pactl 具有与 Web 页面对等的功能
- [ ] Vue 前端使用懒加载，权限守卫正确
- [ ] 静态文件成功嵌入 Go 二进制
- [ ] 所有测试通过
- [ ] 文档更新完整
