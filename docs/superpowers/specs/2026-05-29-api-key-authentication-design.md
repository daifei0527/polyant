# API Key 认证中间件设计文档

## 1. 概述

### 1.1 问题陈述

种子节点的公开 API（搜索、查看条目、分类列表等）目前无需认证即可访问，存在被外部应用滥用的风险。

### 1.2 设计目标

- 保护种子节点的公开 API，仅允许 Polyant 应用的节点访问
- 用户节点不需要预先注册
- 实现简单，符合 YAGNI 原则

### 1.3 设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 认证方式 | 静态共享密钥 | 用户节点无身份，无需多密钥管理 |
| 密钥传递 | HTTP 头 (`X-Polyant-Api-Key`) | 不会出现在 URL 日志中 |
| 密钥格式 | 随机字符串 (32+ 字节) | 强度足够，易于生成 |
| 错误响应 | 401 Unauthorized + JSON | 符合 HTTP 规范 |

## 2. 架构设计

### 2.1 认证流程

```
客户端请求
    ↓
携带 X-Polyant-Api-Key 头
    ↓
ApiKeyMiddleware 验证
    ↓
匹配？ → 继续处理 → 返回响应
不匹配？ → 返回 401 Unauthorized
```

### 2.2 路由保护范围

| 路由类型 | 认证方式 | 说明 |
|----------|----------|------|
| 公开路由 | API Key | 搜索、查看条目、分类列表、节点状态、用户注册 |
| 认证路由 | Ed25519 签名 | 创建/更新/删除条目、评分、用户信息、批量操作 |
| Admin 路由 | Session Token | 用户管理、统计、静态管理页面 |

### 2.3 组件交互

```
┌─────────────┐     ┌─────────────────┐     ┌─────────────┐
│  用户节点   │────→│ ApiKeyMiddleware │────→│   Handler   │
└─────────────┘     └─────────────────┘     └─────────────┘
                           │
                           ↓
                    ┌─────────────┐
                    │ 配置文件    │
                    │ (api_key)   │
                    └─────────────┘
```

## 3. 详细设计

### 3.1 配置结构

**文件：** `pkg/config/config.go`

```go
type NetworkConfig struct {
    // ... 现有字段
    ApiKey string `json:"api_key"` // API 访问密钥
}
```

**配置文件示例：**

```json
{
  "network": {
    "api_key": "sk_live_YOUR_API_KEY_HERE",
    "api_port": 8080
  }
}
```

### 3.2 中间件实现

**文件：** `internal/api/middleware/apikey.go`

```go
package middleware

import (
    "encoding/json"
    "net/http"
)

// ApiKeyMiddleware 验证 API Key
func ApiKeyMiddleware(validKey string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            apiKey := r.Header.Get("X-Polyant-Api-Key")
            if apiKey == "" || apiKey != validKey {
                writeJSONError(w, http.StatusUnauthorized, "Missing or invalid API key")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

func writeJSONError(w http.ResponseWriter, code int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "code":    code,
        "message": message,
    })
}
```

### 3.3 路由配置

**文件：** `internal/api/router/router.go`

```go
func SetupRouter(store storage.Store, config *config.Config) *chi.Mux {
    r := chi.NewRouter()

    // 中间件链
    r.Use(middleware.RequestID)
    r.Use(middleware.Logger)
    r.Use(middleware.Recovery)
    r.Use(middleware.CORS)
    r.Use(middleware.RateLimit)

    // 公开路由（需要 API key）
    public := r.Group("/api/v1")
    if config.Network.ApiKey != "" {
        public.Use(middleware.ApiKeyMiddleware(config.Network.ApiKey))
    }
    public.Get("/search", handler.Search)
    public.Get("/entries/{id}", handler.GetEntry)
    public.Get("/categories", handler.ListCategories)
    public.Get("/status", handler.Status)
    public.Post("/users/register", handler.Register)

    // 认证路由（需要 Ed25519 签名）
    auth := r.Group("/api/v1")
    auth.Use(middleware.AuthMiddleware(store))
    auth.Post("/entries", handler.CreateEntry)
    auth.Put("/entries/{id}", handler.UpdateEntry)
    auth.Delete("/entries/{id}", handler.DeleteEntry)
    // ...

    return r
}
```

### 3.4 错误响应格式

```json
{
  "code": 401,
  "message": "Missing or invalid API key"
}
```

## 4. 节点配置

### 4.1 种子节点配置

```json
{
  "node": {
    "type": "seed",
    "name": "polyant-seed-1"
  },
  "network": {
    "api_key": "sk_live_YOUR_API_KEY_HERE",
    "api_port": 8080,
    "listen_port": 9000
  }
}
```

### 4.2 用户节点配置

```json
{
  "node": {
    "type": "user",
    "name": "polyant-user-1"
  },
  "network": {
    "api_key": "sk_live_YOUR_API_KEY_HERE",
    "api_port": 8081,
    "seed_nodes": ["/ip4/114.116.242.84/tcp/9000/p2p/12D3Koo..."]
  }
}
```

## 5. 安全考虑

### 5.1 密钥存储

- API key 以明文存储在配置文件中
- 种子节点有 TLS 保护，密钥传输加密
- 建议使用强随机字符串（32+ 字节）

### 5.2 密钥轮换

- 密钥轮换需要重启所有节点
- 建议在维护窗口期间进行

### 5.3 速率限制

- 现有速率限制机制仍然有效
- 按 IP 或用户公钥限流

## 6. 测试计划

### 6.1 单元测试

- 测试 ApiKeyMiddleware 正确验证有效 key
- 测试 ApiKeyMiddleware 拒绝无效 key
- 测试 ApiKeyMiddleware 拒绝缺少 key 的请求

### 6.2 集成测试

- 测试公开路由需要 API key
- 测试认证路由不需要 API key（使用 Ed25519 签名）
- 测试错误响应格式正确

### 6.3 端到端测试

- 测试用户节点使用 API key 访问种子节点
- 测试外部应用无法访问种子节点

## 7. 部署计划

### 7.1 生成 API Key

```bash
openssl rand -hex 32
```

### 7.2 更新配置文件

在所有节点的配置文件中添加 `api_key` 字段。

### 7.3 重启节点

按顺序重启所有节点：
1. 种子节点
2. 用户节点

## 8. 未来扩展

### 8.1 多密钥支持

如果未来需要支持多密钥，可以扩展为：
- 配置文件支持数组格式
- 中间件验证 key 是否在有效列表中

### 8.2 密钥轮换

如果需要无重启密钥轮换，可以：
- 将密钥存储在数据库中
- 定期从数据库加载有效密钥

### 8.3 密钥撤销

如果需要撤销单个密钥，可以：
- 为每个密钥添加过期时间
- 支持密钥禁用功能

## 9. 参考资料

- [现有认证中间件](internal/api/middleware/auth.go)
- [路由配置](internal/api/router/router.go)
- [配置结构](pkg/config/config.go)
