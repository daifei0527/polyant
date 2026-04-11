# P0 核心 API 完善设计

**日期**: 2026-04-11
**优先级**: P0
**状态**: 待实现

---

## 一、概述

完善 AgentWiki 核心 API，修复已发现的 bug，增强条目内容签名验证，确保 API 可用性和安全性。

---

## 二、变更清单

### 2.1 条目内容签名

**目标**: 条目创建时对核心内容签名，确保同步时可独立验证内容完整性。

**请求结构变更** (`handler/types.go`):
```go
type CreateEntryRequest struct {
    Title            string                   `json:"title"`
    Content          string                   `json:"content"`
    JsonData         []map[string]interface{} `json:"json_data,omitempty"`
    Category         string                   `json:"category"`
    Tags             []string                 `json:"tags,omitempty"`
    License          string                   `json:"license,omitempty"`
    SourceRef        string                   `json:"source_ref,omitempty"`
    CreatorSignature string                   `json:"creator_signature"` // 新增
}
```

**签名内容格式**:
```
签名输入 = title + "\n" + content + "\n" + category
签名内容 = SHA256(签名输入)
签名输出 = Ed25519.Sign(私钥, 签名内容)
```

**验证逻辑** (`handler/entry_handler.go`):
1. 从 context 获取用户公钥
2. 计算签名内容 SHA256(title + "\n" + content + "\n" + category)
3. 使用 Ed25519.Verify 验证签名
4. 签名验证失败返回 ErrInvalidSignature

**存储**: 条目已有的 `ContentHash` 字段保持不变，签名存储在请求处理过程中验证，不持久化。

---

### 2.2 UserStore GetByEmail 实现

**目标**: 支持通过邮箱查询用户，用于邮箱唯一性检查。

**文件**: `storage/kv/user_store.go`

**方法签名**:
```go
func (us *UserStore) GetByEmail(ctx context.Context, email string) (*model.User, error)
```

**实现方式**:
- 遍历 `PrefixUser` 前缀的所有用户
- 查找 Email 字段匹配的用户
- 找到返回用户，未找到返回 ErrUserNotFound

---

### 2.3 GetUserInfoHandler Bug 修复

**问题**: 当前代码传入 `user.PublicKey` 作为查询键，但 `UserStore.Get()` 期望 `pubkeyHash`。

**文件**: `handler/user_handler.go`

**修复方案**:
```go
// 方案：从 user.PublicKey 计算 pubkeyHash
pubKeyBytes, _ := base64.StdEncoding.DecodeString(user.PublicKey)
hash := sha256.Sum256(pubKeyBytes)
pubKeyHash := hex.EncodeToString(hash[:])
latest, err := h.userStore.Get(r.Context(), pubKeyHash)
```

---

### 2.4 Router 依赖注入完善

**目标**: 确保 EmailService 正确注入到 Handler。

**文件**: `router/router.go`

**变更**:
```go
// 修改函数签名
func NewRouter(store *storage.Store, cfg *config.Config, emailService *email.Service) (http.Handler, error) {
    return NewRouterWithDeps(&Dependencies{
        EntryStore:    store.Entry,
        UserStore:     store.User,
        RatingStore:   store.Rating,
        CategoryStore: store.Category,
        SearchEngine:  store.Search,
        Backlink:      store.Backlink,
        EmailService:  emailService,  // 新增
        NodeID:        "local-node-1",
        NodeType:      cfg.Node.Type,
        Version:       "v0.1.0-dev",
    })
}
```

---

## 三、影响文件

| 文件 | 变更类型 | 影响范围 |
|------|----------|----------|
| `handler/types.go` | 修改字段 | 条目创建 API |
| `handler/entry_handler.go` | 增加验证逻辑 | 条目创建流程 |
| `storage/kv/user_store.go` | 新增方法 | 用户查询 |
| `handler/user_handler.go` | Bug 修复 | 用户信息查询 |
| `router/router.go` | 参数变更 | 服务启动 |

---

## 四、错误码

无需新增错误码，现有错误码已覆盖：
- `ErrInvalidSignature` (201): 签名验证失败
- `ErrUserNotFound` (301): 用户不存在

---

## 五、测试要点

1. **条目创建签名验证**
   - 正确签名 → 创建成功
   - 错误签名 → 返回 401
   - 缺少签名字段 → 返回 400

2. **GetByEmail**
   - 邮箱存在 → 返回用户
   - 邮箱不存在 → 返回错误

3. **GetUserInfo**
   - 认证用户 → 返回正确用户信息

4. **EmailService 注入**
   - 配置邮件服务 → 发送验证码成功
