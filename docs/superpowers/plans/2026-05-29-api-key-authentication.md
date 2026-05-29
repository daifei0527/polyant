# API Key 认证中间件实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 API Key 认证中间件，保护种子节点的公开 API，仅允许 Polyant 应用的节点访问。

**Architecture:** 新增 ApiKeyMiddleware 中间件，在配置文件中添加 api_key 字段，对公开路由进行 API Key 验证。

**Tech Stack:** Go, net/http, 中间件模式

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `pkg/config/config.go` | 修改 | 在 NetworkConfig 中添加 ApiKey 字段 |
| `internal/api/middleware/apikey.go` | 创建 | API Key 认证中间件实现 |
| `internal/api/middleware/apikey_test.go` | 创建 | API Key 中间件单元测试 |
| `internal/api/router/router.go` | 修改 | 在公开路由中应用 ApiKeyMiddleware |
| `configs/seed.json` | 修改 | 添加 api_key 配置示例 |
| `configs/user.json` | 修改 | 添加 api_key 配置示例 |

---

### Task 1: 添加配置字段

**Files:**
- Modify: `pkg/config/config.go:26-32`

- [ ] **Step 1: 在 NetworkConfig 中添加 ApiKey 字段**

```go
// NetworkConfig 网络配置
type NetworkConfig struct {
	ListenPort  int      `json:"listen_port"`  // P2P 监听端口
	APIPort     int      `json:"api_port"`     // API 服务端口
	SeedNodes   []string `json:"seed_nodes"`   // 种子节点列表
	DHTEnabled  bool     `json:"dht_enabled"`  // 是否启用 DHT
	MDNSEnabled bool     `json:"mdns_enabled"` // 是否启用 mDNS 发现
	ApiKey      string   `json:"api_key"`      // API 访问密钥，为空则不验证
}
```

- [ ] **Step 2: 运行测试验证配置加载**

Run: `go test ./pkg/config/... -v`
Expected: PASS（现有测试不受影响）

- [ ] **Step 3: 提交配置变更**

```bash
git add pkg/config/config.go
git commit -m "feat(config): add ApiKey field to NetworkConfig"
```

---

### Task 2: 创建 API Key 中间件

**Files:**
- Create: `internal/api/middleware/apikey.go`

- [ ] **Step 1: 创建 API Key 中间件文件**

```go
package middleware

import (
	"encoding/json"
	"net/http"
)

const (
	// headerApiKey API Key 请求头
	headerApiKey = "X-Polyant-Api-Key"
)

// ApiKeyMiddleware 验证 API Key
// 如果 validKey 为空，则跳过验证
func ApiKeyMiddleware(validKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果未配置 API Key，跳过验证
			if validKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			apiKey := r.Header.Get(headerApiKey)
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

- [ ] **Step 2: 提交中间件实现**

```bash
git add internal/api/middleware/apikey.go
git commit -m "feat(middleware): add ApiKeyMiddleware for API key authentication"
```

---

### Task 3: 创建 API Key 中间件测试

**Files:**
- Create: `internal/api/middleware/apikey_test.go`

- [ ] **Step 1: 创建测试文件**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApiKeyMiddleware_ValidKey(t *testing.T) {
	validKey := "sk_live_test123"
	handler := ApiKeyMiddleware(validKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Polyant-Api-Key", validKey)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestApiKeyMiddleware_InvalidKey(t *testing.T) {
	validKey := "sk_live_test123"
	handler := ApiKeyMiddleware(validKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Polyant-Api-Key", "wrong_key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestApiKeyMiddleware_MissingKey(t *testing.T) {
	validKey := "sk_live_test123"
	handler := ApiKeyMiddleware(validKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestApiKeyMiddleware_EmptyValidKey(t *testing.T) {
	handler := ApiKeyMiddleware("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 when valid key is empty, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: 运行测试**

Run: `go test ./internal/api/middleware/... -v -run TestApiKey`
Expected: PASS

- [ ] **Step 3: 提交测试**

```bash
git add internal/api/middleware/apikey_test.go
git commit -m "test(middleware): add ApiKeyMiddleware unit tests"
```

---

### Task 4: 在路由器中应用 API Key 中间件

**Files:**
- Modify: `internal/api/router/router.go`

- [ ] **Step 1: 修改 NewRouter 函数添加配置参数**

```go
// NewRouter 创建并配置 HTTP 路由
// 注册所有 API 端点，配置中间件链
// 中间件执行顺序: RequestID -> Logging -> Recovery -> CORS -> [ApiKey] -> [Auth] -> Handler
func NewRouter(store *storage.Store, cfg *config.Config) (http.Handler, error) {
	return NewRouterWithDeps(&Dependencies{
		Store:         store,
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		KVStore:       store.KVStore(),
		NodeID:        "local-node-1",
		NodeType:      cfg.Node.Type,
		Version:       "v0.1.0-dev",
		ApiKey:        cfg.Network.ApiKey,  // 添加这行
	})
}
```

- [ ] **Step 2: 在 Dependencies 结构体中添加 ApiKey 字段**

```go
// Dependencies 路由依赖注入容器
// 包含所有 handler 需要的存储和引擎实例
type Dependencies struct {
	Store           *storage.Store
	EntryStore      storage.EntryStore
	UserStore       storage.UserStore
	RatingStore     storage.RatingStore
	CategoryStore   storage.CategoryStore
	SearchEngine    index.SearchEngine
	Backlink        storage.BacklinkIndex
	EmailService    *email.Service
	VerificationMgr *email.VerificationManager
	RemoteQuerier   RemoteQuerier // 远程查询服务
	EntryPusher     EntryPusher   // 条目推送服务
	KVStore         kv.Store      // KV 存储（选举等功能需要）
	SessionManager  *coreadmin.SessionManager
	NodeID          string
	NodeType        string
	Version         string
	ApiKey          string        // API 访问密钥
}
```

- [ ] **Step 3: 在公开路由中应用 ApiKeyMiddleware**

在 `NewRouterWithDeps` 函数中找到公开路由注册部分，添加中间件：

```go
	// ==================== 公开 API（无需认证）====================
	// 注意: 这些路由通过 ApiKeyMiddleware 保护，需要在请求头中携带 X-Polyant-Api-Key

	// 创建公开路由处理器
	publicMux := http.NewServeMux()

	// 注册公开路由
	publicMux.HandleFunc("/api/v1/search", entryHandler.Search)
	publicMux.HandleFunc("/api/v1/entries/", entryHandler.GetEntry)
	publicMux.HandleFunc("/api/v1/categories", categoryHandler.ListCategories)
	publicMux.HandleFunc("/api/v1/categories/tree", categoryHandler.GetCategoryTree)
	publicMux.HandleFunc("/api/v1/status", statusHandler.GetStatus)
	publicMux.HandleFunc("/api/v1/users/register", userHandler.Register)
	publicMux.HandleFunc("/api/v1/users/check-email", userHandler.CheckEmail)
	publicMux.HandleFunc("/api/v1/users/verify-email", userHandler.VerifyEmail)

	// 应用 API Key 中间件到公开路由
	publicHandler := middleware.ApiKeyMiddleware(deps.ApiKey)(publicMux)
```

- [ ] **Step 4: 运行测试验证路由配置**

Run: `go test ./internal/api/... -v`
Expected: PASS

- [ ] **Step 5: 提交路由变更**

```bash
git add internal/api/router/router.go
git commit -m "feat(router): apply ApiKeyMiddleware to public routes"
```

---

### Task 5: 更新配置文件示例

**Files:**
- Modify: `configs/seed.json`
- Modify: `configs/user.json`

- [ ] **Step 1: 生成 API Key**

Run: `openssl rand -hex 32`
Expected: 输出类似 `a1b2c3d4e5f6...` 的 64 字符十六进制字符串

- [ ] **Step 2: 更新种子节点配置**

在 `configs/seed.json` 的 `network` 部分添加 `api_key`：

```json
{
  "network": {
    "listen_port": 9000,
    "api_port": 8080,
    "seed_nodes": [],
    "dht_enabled": true,
    "mdns_enabled": false,
    "api_key": "sk_live_a1b2c3d4e5f6..."
  }
}
```

- [ ] **Step 3: 更新用户节点配置**

在 `configs/user.json` 的 `network` 部分添加 `api_key`：

```json
{
  "network": {
    "listen_port": 0,
    "api_port": 8080,
    "seed_nodes": ["/dns4/seed.polyant.top/tcp/9000/p2p/12D3Koo..."],
    "dht_enabled": false,
    "mdns_enabled": true,
    "api_key": "sk_live_a1b2c3d4e5f6..."
  }
}
```

- [ ] **Step 4: 提交配置文件变更**

```bash
git add configs/seed.json configs/user.json
git commit -m "config: add api_key to seed and user configuration examples"
```

---

### Task 6: 端到端测试

- [ ] **Step 1: 启动种子节点**

Run: `./bin/seed -config configs/seed.json -domain localhost -tls-cert certs/cert.pem -tls-key certs/key.pem`

- [ ] **Step 2: 测试无 API Key 请求（应返回 401）**

Run: `curl -sk https://localhost:8080/api/v1/categories`
Expected: `{"code":401,"message":"Missing or invalid API key"}`

- [ ] **Step 3: 测试有效 API Key 请求（应返回 200）**

Run: `curl -sk -H "X-Polyant-Api-Key: sk_live_a1b2c3d4e5f6..." https://localhost:8080/api/v1/categories`
Expected: `{"code":0,"message":"success","data":[...]}`

- [ ] **Step 4: 测试无效 API Key 请求（应返回 401）**

Run: `curl -sk -H "X-Polyant-Api-Key: wrong_key" https://localhost:8080/api/v1/categories`
Expected: `{"code":401,"message":"Missing or invalid API key"}`

- [ ] **Step 5: 提交最终变更**

```bash
git add .
git commit -m "feat: complete API key authentication implementation"
```

---

## 安全注意事项

1. **API Key 生成**：使用 `openssl rand -hex 32` 生成强随机密钥
2. **密钥存储**：API Key 以明文存储在配置文件中，确保配置文件权限为 600
3. **密钥轮换**：轮换密钥需要重启所有节点
4. **传输安全**：种子节点使用 TLS，API Key 传输加密

## 未来扩展

1. **多密钥支持**：配置文件支持数组格式，中间件验证 key 是否在有效列表中
2. **动态密钥**：将密钥存储在数据库中，支持无重启轮换
3. **密钥撤销**：为每个密钥添加过期时间，支持密钥禁用功能
