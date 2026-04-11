# P0 核心 API 完善实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完善 AgentWiki 核心 API，修复 bug，增加条目内容签名验证

**Architecture:** 修复 UserStore 缺失方法，增加条目创建时的内容签名验证，修复用户信息查询 bug，完善路由依赖注入

**Tech Stack:** Go 1.22, Ed25519 签名, SHA256 哈希

---

## Task 1: 实现 UserStore.GetByEmail 方法

**Files:**
- Modify: `internal/storage/kv/user_store.go`

- [ ] **Step 1: 添加 GetByEmail 方法**

在 `internal/storage/kv/user_store.go` 文件末尾添加以下代码：

```go
// GetByEmail 根据邮箱获取用户
func (us *UserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	users, err := ScanAndParse(us.store, PrefixUser, func(data []byte) (*model.User, error) {
		user := &model.User{}
		if err := user.FromJSON(data); err != nil {
			return nil, err
		}
		return user, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	for _, user := range users {
		if user.Email == email {
			return user, nil
		}
	}

	return nil, fmt.Errorf("user with email %s not found", email)
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
git add internal/storage/kv/user_store.go
git commit -m "feat(storage): 实现 UserStore.GetByEmail 方法"
```

---

## Task 2: 添加 creator_signature 字段到 CreateEntryRequest

**Files:**
- Modify: `internal/api/handler/types.go`

- [ ] **Step 1: 添加 CreatorSignature 字段**

修改 `internal/api/handler/types.go` 中的 `CreateEntryRequest` 结构体，添加 `CreatorSignature` 字段：

```go
// CreateEntryRequest 创建条目请求体
type CreateEntryRequest struct {
	Title            string                   `json:"title"`
	Content          string                   `json:"content"`
	JsonData         []map[string]interface{} `json:"json_data,omitempty"`
	Category         string                   `json:"category"`
	Tags             []string                 `json:"tags,omitempty"`
	License          string                   `json:"license,omitempty"`
	SourceRef        string                   `json:"source_ref,omitempty"`
	CreatorSignature string                   `json:"creator_signature"` // 条目内容签名
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/types.go
git commit -m "feat(api): 添加 CreateEntryRequest.CreatorSignature 字段"
```

---

## Task 3: 实现条目创建签名验证

**Files:**
- Modify: `internal/api/handler/entry_handler.go`

- [ ] **Step 1: 添加必要的 import**

在 `internal/api/handler/entry_handler.go` 的 import 块中添加：

```go
import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/linkparser"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)
```

注意：检查现有 import，添加缺失的 `crypto/ed25519` 和 `encoding/base64`。

- [ ] **Step 2: 添加签名验证辅助函数**

在 `CreateEntryHandler` 函数之前添加验证函数：

```go
// verifyCreatorSignature 验证条目创建者的内容签名
// 签名内容 = SHA256(title + "\n" + content + "\n" + category)
func verifyCreatorSignature(publicKey ed25519.PublicKey, title, content, category string, signature []byte) bool {
	// 构造签名内容
	signContent := title + "\n" + content + "\n" + category
	hash := sha256.Sum256([]byte(signContent))

	// Ed25519 验证签名
	return ed25519.Verify(publicKey, hash[:], signature)
}
```

- [ ] **Step 3: 在 CreateEntryHandler 中添加签名验证**

在 `CreateEntryHandler` 函数中，在权限检查之后、创建条目之前添加签名验证逻辑：

找到以下代码块：
```go
	// 检查用户权限（Lv1及以上可创建条目）
	if user.UserLevel < model.UserLevelLv1 {
		writeError(w, awerrors.ErrBasicUserDenied)
		return
	}
```

在其后添加：
```go
	// 验证条目内容签名
	if req.CreatorSignature == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 解码用户公钥
	pubKeyBytes, err := base64.StdEncoding.DecodeString(user.PublicKey)
	if err != nil {
		writeError(w, awerrors.ErrInvalidSignature)
		return
	}

	// 解码签名
	signature, err := base64.StdEncoding.DecodeString(req.CreatorSignature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		writeError(w, awerrors.ErrInvalidSignature)
		return
	}

	// 验证签名
	if !verifyCreatorSignature(pubKeyBytes, req.Title, req.Content, req.Category, signature) {
		writeError(w, awerrors.ErrInvalidSignature)
		return
	}
```

- [ ] **Step 4: 验证编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 5: 提交**

```bash
git add internal/api/handler/entry_handler.go
git commit -m "feat(api): 添加条目创建内容签名验证"
```

---

## Task 4: 修复 GetUserInfoHandler Bug

**Files:**
- Modify: `internal/api/handler/user_handler.go`

- [ ] **Step 1: 添加必要的 import**

检查 `internal/api/handler/user_handler.go` 的 import 块，确保包含：
```go
import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/daifei0527/agentwiki/internal/core/email"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
)
```

- [ ] **Step 2: 修复 GetUserInfoHandler**

找到 `GetUserInfoHandler` 函数中的以下代码：
```go
	// 从存储中获取最新的用户信息
	latest, err := h.userStore.Get(r.Context(), user.PublicKey)
	if err != nil {
		writeError(w, awerrors.ErrUserNotFound)
		return
	}
```

替换为：
```go
	// 从存储中获取最新的用户信息
	// UserStore.Get 需要 pubkeyHash，从 user.PublicKey 计算
	pubKeyBytes, err := base64.StdEncoding.DecodeString(user.PublicKey)
	if err != nil {
		writeError(w, awerrors.ErrInternal)
		return
	}
	hash := sha256.Sum256(pubKeyBytes)
	pubKeyHash := hex.EncodeToString(hash[:])

	latest, err := h.userStore.Get(r.Context(), pubKeyHash)
	if err != nil {
		writeError(w, awerrors.ErrUserNotFound)
		return
	}
```

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add internal/api/handler/user_handler.go
git commit -m "fix(api): 修复 GetUserInfoHandler 查询参数错误"
```

---

## Task 5: 完善 Router EmailService 注入

**Files:**
- Modify: `internal/api/router/router.go`

- [ ] **Step 1: 修改 NewRouter 函数签名**

找到 `NewRouter` 函数：
```go
func NewRouter(store *storage.Store, cfg *config.Config) (http.Handler, error) {
	return NewRouterWithDeps(&Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		NodeID:        "local-node-1",
		NodeType:      cfg.Node.Type,
		Version:       "v0.1.0-dev",
	})
}
```

替换为：
```go
func NewRouter(store *storage.Store, cfg *config.Config, emailService *email.Service) (http.Handler, error) {
	return NewRouterWithDeps(&Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		EmailService:  emailService,
		NodeID:        "local-node-1",
		NodeType:      cfg.Node.Type,
		Version:       "v0.1.0-dev",
	})
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: 编译错误，提示 `NewRouter` 调用参数不匹配

- [ ] **Step 3: 更新 NewRouter 调用方**

查找调用 `NewRouter` 的地方并更新。运行：
```bash
grep -rn "router.NewRouter" --include="*.go" .
```

找到调用位置后，添加 `nil` 作为 `emailService` 参数（如果暂时没有邮件服务）：
```go
// 例如在 cmd/agentwiki 或 internal/service/daemon 中
httpHandler, err := router.NewRouter(store, cfg, nil)
```

- [ ] **Step 4: 验证编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 5: 提交**

```bash
git add internal/api/router/router.go
git commit -m "feat(router): 完善 EmailService 依赖注入"
```

---

## Task 6: 最终验证

- [ ] **Step 1: 完整编译**

Run: `go build ./...`
Expected: 编译成功，无警告

- [ ] **Step 2: 运行测试（如有）**

Run: `go test ./...`
Expected: 所有测试通过

- [ ] **Step 3: 提交所有更改**

```bash
git status
# 确认所有更改已提交
```

---

## 验收清单

- [ ] `UserStore.GetByEmail` 可正常查询用户
- [ ] `CreateEntryRequest.CreatorSignature` 字段存在
- [ ] 条目创建时验证内容签名
- [ ] `GetUserInfoHandler` 返回正确用户信息
- [ ] `NewRouter` 接受 `emailService` 参数
- [ ] 编译通过
