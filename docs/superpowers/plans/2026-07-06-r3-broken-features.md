# R3 坏功能修复实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 闭合 R3 spec 的 6 组坏功能缺陷：审计明文 body、panic 假成功、admin 内容审核死壳、CORS 混合配置反射、统计缺条目维度、SPA 刷新回归。

**Architecture:** 6 个独立可提交 task（R3-A 审计脱敏 / R3-B panic code / R3-C 清死壳+孤儿 / R3-D CORS 混合配置 / R3-E 统计条目维度 / R3-F SPA 刷新回归）。每组 TDD、独立测试周期、独立提交。核心原则：R3 只"修坏+清死壳"，不建新功能（完整审核流程归 R4）；存储/中间件层修复忠实语义、不引入新配置开关。

**Tech Stack:** Go 1.25.x / net/http / regexp / Vue 3 + Vite（admin SPA）/ 标准测试 + `-race`。

**Spec:** `docs/superpowers/specs/2026-07-06-polyant-r3-broken-features-design.md`

## Global Constraints

- Go 版本：`go 1.25.7`（go.mod）；CI `1.25.x`。
- 每个改动**先写失败测试再实现**（TDD），删除型 task（R3-C）以 build + grep 无引用为验证。
- 提交信息前缀：`fix(broken-feature)` / `feat(broken-feature)` / `chore(broken-feature)` / `refactor(broken-feature)`，尾部带 `(R3-X)` 标签。
- 每个 task 结束运行 `go build ./cmd/... ./internal/... ./pkg/... && go vet ./... && go test -race -count=1 <受影响包>`，全绿才提交。
- 前端改动（R3-C）额外 `cd web/admin && npm run build` 通过。
- 中文注释 OK，沿用周边文件风格。
- 本计划所有 file:line 引用以 2026-07-06 master `876495f`（R2 合入后）为准；实现时若行号漂移按符号名定位。

## File Structure

**R3-A（审计脱敏）：**
- Modify: `internal/core/audit/audit.go`（`sensitiveFields` :14-19、`init` :43-49、`Log` :62-69、`MaskSensitiveFields` :87-105）
- Test: `internal/core/audit/audit_test.go`（改 :27 用例、新增用例 + 新增 `TestService_Log_MasksBothBodies`）

**R3-B（panic code）：**
- Modify: `internal/api/middleware/logging.go`（`RecoveryMiddleware` :66-78）
- Test: `internal/api/middleware/logging_test.go`（`TestRecoveryMiddleware_WithPanic` :76-97 加 code 断言）

**R3-C（清死壳 + 孤儿）：**
- Modify: `web/admin/src/components/Sidebar.vue`（删内容审核菜单 :21-24）、`web/admin/src/router/index.js`（删 entries 路由 :38-49）
- Delete: `web/admin/src/views/entries/List.vue`、`web/admin/src/views/entries/Detail.vue`（及空目录）、`web/landing/index.html`、`web/static/css/style.css`（及空目录）
- Modify: `internal/storage/model/models.go`（`EntryStatusReview` :32 注释）

**R3-D（CORS 混合配置）：**
- Modify: `internal/api/middleware/cors.go`（`NewCORSMiddleware` :38-55）
- Test: `internal/api/middleware/cors_test.go`（新增 `TestCORSMiddleware_MixedWildcardNotReflected`）

**R3-E（统计条目维度）：**
- Create/Modify: `internal/storage/model/models.go`（新增 `EntryStats` + `CategoryCount`，接 `UserStats` :214 之后）
- Modify: `internal/core/user/stats_service.go`（新增 `GetEntryStats` + `topCategories` + 缓存字段）
- Modify: `internal/api/handler/stats_handler.go`（新增 `GetEntryStatsHandler`）
- Modify: `internal/api/admin/handler.go`（新增 `GetEntryStatsHandler` 委托）
- Modify: `internal/api/router/router.go`（`registerAdminRoutes` :487-495 后注册 `/stats/entries`）
- Test: `internal/core/user/stats_service_test.go`（新增 `TestStatsService_GetEntryStats`）

**R3-F（SPA 刷新回归）：**
- Test: `internal/api/admin/static_test.go`（新增 `TestStaticHandler_ServeHTTP_DeepRefresh`）

---

# R3-A：审计脱敏

闭合：ResponseBody 不脱敏（:65 仅 Truncate）、email 未入敏感字段表（:14-19）、正则只掩字符串值（:46）、字段表不全。

## Task A: 审计敏感字段全量脱敏

**Files:**
- Modify: `internal/core/audit/audit.go`（`sensitiveFields` :14-19、`init` :43-49、`Log` :62-69）
- Test: `internal/core/audit/audit_test.go`（:27 用例、新增用例、新增 `TestService_Log_MasksBothBodies`）

**Interfaces:**
- Produces: `MaskSensitiveFields` 行为增强（覆盖数字/布尔/null 标量值、字段表扩充 email 等）；签名不变（`func MaskSensitiveFields(jsonStr string) string`）。`Service.Log` 对 `ResponseBody` 也调 `MaskSensitiveFields`。
- Consumes: `kv.AuditStore` 接口（`Create/Get/List/DeleteBefore/GetStats`，见 `internal/storage/kv/audit_store.go:23-29`）。

- [ ] **Step 1: 改现有用例 + 写失败测试**（`audit_test.go`）

把 :27-29 的 `mask verification_code` 用例 expected 改为 email 也掩（因 R3-A 将 email 加入敏感字段），并在 `TestMaskSensitiveFields` 表驱动切片末尾追加数字/布尔/新字段用例：

```go
		{
			name:     "mask verification_code",
			input:    `{"email":"test@example.com","code":"123456"}`,
			expected: `{"email": "***","code": "***"}`, // R3-A: email 也脱敏
		},
		// R3-A: 标量值（数字/布尔/null）也要掩
		{
			name:     "mask numeric token",
			input:    `{"token":123456789}`,
			expected: `{"token": "***"}`,
		},
		{
			name:     "mask boolean secret",
			input:    `{"secret":true}`,
			expected: `{"secret": "***"}`,
		},
		{
			name:     "mask null api_key",
			input:    `{"api_key":null}`,
			expected: `{"api_key": "***"}`,
		},
		// R3-A: 新增字段表项
		{
			name:     "mask new_password",
			input:    `{"new_password":"abc"}`,
			expected: `{"new_password": "***"}`,
		},
```

在文件末尾追加 `TestService_Log_MasksBothBodies`（验证 ResponseBody 也被脱敏）：

```go
// fakeAuditStore 捕获 Create 收到的 AuditLog，用于断言脱敏后的 body。
type fakeAuditStore struct {
	got *model.AuditLog
}

func (s *fakeAuditStore) Create(ctx context.Context, log *model.AuditLog) error { s.got = log; return nil }
func (s *fakeAuditStore) Get(ctx context.Context, id string) (*model.AuditLog, error) { return nil, nil }
func (s *fakeAuditStore) List(ctx context.Context, filter model.AuditFilter) ([]*model.AuditLog, int64, error) { return nil, 0, nil }
func (s *fakeAuditStore) DeleteBefore(ctx context.Context, ts int64) (int64, error) { return 0, nil }
func (s *fakeAuditStore) GetStats(ctx context.Context) (*model.AuditStats, error) { return nil, nil }

// TestService_Log_MasksBothBodies: RequestBody 与 ResponseBody 都必须脱敏。
func TestService_Log_MasksBothBodies(t *testing.T) {
	store := &fakeAuditStore{}
	svc := NewService(store)

	err := svc.Log(context.Background(), &model.AuditLog{
		RequestBody:  `{"password":"secret","email":"u@example.com"}`,
		ResponseBody: `{"token":"abc123","api_key":"k"}`,
	})
	require.NoError(t, err)

	assert.Contains(t, store.got.RequestBody, `"password": "***"`)
	assert.NotContains(t, store.got.RequestBody, "secret")
	assert.Contains(t, store.got.RequestBody, `"email": "***"`)
	assert.NotContains(t, store.got.RequestBody, "u@example.com")

	assert.Contains(t, store.got.ResponseBody, `"token": "***"`)
	assert.NotContains(t, store.got.ResponseBody, "abc123")
	assert.Contains(t, store.got.ResponseBody, `"api_key": "***"`)
}
```

测试文件头补 import（`context`、`require`、`model`）：

```go
import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/core/audit/ -run 'TestMaskSensitiveFields|TestService_Log_MasksBothBodies' -v`
Expected: FAIL（email 未掩 → 用例期望不符；标量值未掩；`ResponseBody` 含明文 token）

- [ ] **Step 3: 扩充字段表 + 标量正则 + ResponseBody 脱敏**（`audit.go`）

`sensitiveFields`（:14-19）扩充：

```go
// 敏感字段脱敏规则
var sensitiveFields = []string{
	"password", "passwd", "pwd",
	"new_password", "old_password", "confirm_password",
	"private_key", "privateKey", "private-key",
	"secret", "token", "api_key", "apiKey",
	"access_token", "refresh_token",
	"code", "verification_code",
	"email",
	"passphrase", "mnemonic", "seed",
}
```

`init()`（:43-49）每字段编译两条模式（字符串值 + 标量值）：

```go
func init() {
	for _, field := range sensitiveFields {
		// 字符串值："field": "value"
		sensitivePatterns = append(sensitivePatterns,
			regexp.MustCompile(`(?i)"`+field+`"\s*:\s*"[^"]*"`))
		// 标量值（数字/布尔/null）："field": 123 / true / false / null
		sensitivePatterns = append(sensitivePatterns,
			regexp.MustCompile(`(?i)"`+field+`"\s*:\s*(?:-?\d+(?:\.\d+)?|true|false|null)`))
	}
}
```

`Log`（:62-69）对 `ResponseBody` 也脱敏（先 mask 再 truncate）：

```go
// Log 记录审计日志
func (s *Service) Log(ctx context.Context, log *model.AuditLog) error {
	// R3-A：RequestBody 与 ResponseBody 均脱敏（含 email/密钥/token 的标量与字符串形式）
	log.RequestBody = MaskSensitiveFields(log.RequestBody)
	log.ResponseBody = MaskSensitiveFields(log.ResponseBody)
	log.ResponseBody = TruncateString(log.ResponseBody, 4096) // 4KB
	log.RequestBody = TruncateString(log.RequestBody, 16384)  // 16KB

	return s.store.Create(ctx, log)
}
```

`MaskSensitiveFields`（:87-105）函数体无需改——`SplitN(match, ":", 2)` 对字符串与标量两种匹配都正确重写为 `"field": "***"`。

- [ ] **Step 4: 跑测试确认通过 + race**

Run: `go test -race -count=1 ./internal/core/audit/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/core/audit/audit.go internal/core/audit/audit_test.go
git commit -m "fix(broken-feature): audit masks response body, email, scalar secrets (R3-A)"
```

---

# R3-B：panic 不再假成功

闭合：`RecoveryMiddleware`（logging.go:73）recover 后写 `{"code":0,...}`，与 HTTP 500 矛盾，前端按业务 code 判错会当成功放行。

## Task B: panic 响应返回非 0 错误码

**Files:**
- Modify: `internal/api/middleware/logging.go`（`RecoveryMiddleware` :66-78）
- Test: `internal/api/middleware/logging_test.go`（`TestRecoveryMiddleware_WithPanic` :76-97）

**Interfaces:**
- Produces: panic 响应 body 的 `code` 由 `0` 改为 `500`（非 0）。响应仍为 `application/json`，HTTP 500。无新接口。
- Consumes: 无。middleware 不 import handler（避免循环依赖），直接写字面 JSON。

- [ ] **Step 1: 给现有 panic 测试加 code 断言**（`logging_test.go` `TestRecoveryMiddleware_WithPanic` :76-97）

在 message 断言后追加 code 断言：

```go
		if resp["code"] == nil || resp["code"].(float64) == 0 {
			t.Errorf("panic 响应 code 必须非 0（假成功），got %v", resp["code"])
		}
```

（`json.Unmarshal` 把 JSON number 解成 `float64`，故断言 `float64(0)`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/api/middleware/ -run TestRecoveryMiddleware_WithPanic -v`
Expected: FAIL（当前 code:0，断言 `== 0` 命中）

- [ ] **Step 3: 修 RecoveryMiddleware**（`logging.go:66-78`）

```go
// RecoveryMiddleware 异常恢复中间件
// 捕获 handler 中的 panic，防止服务崩溃
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC] %s %s: %v", r.Method, r.URL.Path, err)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				// R3-B：code 非 0（与 APIResponse 约定一致：0=成功），避免前端把 panic 当成功放行
				w.Write([]byte(`{"code":500,"message":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: 跑测试确认通过 + race**

Run: `go test -race -count=1 ./internal/api/middleware/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/api/middleware/logging.go internal/api/middleware/logging_test.go
git commit -m "fix(broken-feature): panic response returns non-zero code, not fake success (R3-B)"
```

---

# R3-C：清死壳 + 孤儿前端文件

闭合：admin SPA 的"内容审核"入口（Sidebar 菜单 + router 路由 + entries 视图）是死壳（无对应 API），点击无数据；`web/landing/index.html`、`web/static/css/style.css` 是 R1 删公开前端后的孤儿文件。完整审核流程归 R4。

## Task C: 删 admin 内容审核死壳 + 孤儿文件

**Files:**
- Modify: `web/admin/src/components/Sidebar.vue`（删 :21-24 内容审核菜单 + :22,32 `Document` 引用）
- Modify: `web/admin/src/router/index.js`（删 :38-49 entries/EntryDetail 路由）
- Delete: `web/admin/src/views/entries/List.vue`、`web/admin/src/views/entries/Detail.vue`、`web/admin/src/views/entries/`（空目录）
- Delete: `web/landing/index.html`、`web/landing/`（空目录）、`web/static/css/style.css`、`web/static/css/`（空目录，若空）
- Modify: `internal/storage/model/models.go`（`EntryStatusReview` :32 注释）

**Interfaces:**
- Produces: 无后端接口变化。admin SPA 不再暴露"内容审核"入口（R4 实现完整审核时重新加回）。
- Consumes: 无。

- [ ] **Step 1: 删 Sidebar.vue 内容审核菜单**（`web/admin/src/components/Sidebar.vue`）

删 :21-24 的菜单项：

```vue
      <el-menu-item index="/entries" v-if="hasPermission(4)">
        <el-icon><Document /></el-icon>
        <span>内容审核</span>
      </el-menu-item>
```

并把 :32 的 import 去掉 `Document`（保留 `DataLine, User`）：

```js
import { DataLine, User } from '@element-plus/icons-vue'
```

- [ ] **Step 2: 删 router/index.js entries 路由**（`web/admin/src/router/index.js`）

删 :38-49 的两个路由对象：

```js
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
```

注意删后 `users/:publicKey` 路由对象末尾的逗号需保留合法（其前为 `users` 路由，删 entries 后 `children` 数组以 `users/:publicKey` 收尾，确保无尾逗号）。

- [ ] **Step 3: 删 entries 视图目录**

```bash
rm -rf web/admin/src/views/entries
```

- [ ] **Step 4: 改后端 EntryStatusReview 注释**（`internal/storage/model/models.go:32`）

```go
	EntryStatusReview    = "review"    // 审核中（未实现，完整审核流程见 R4）
```

- [ ] **Step 5: 删孤儿前端文件**

```bash
rm -f web/landing/index.html
rmdir web/landing 2>/dev/null || true
rm -f web/static/css/style.css
rmdir web/static/css 2>/dev/null || true
```

- [ ] **Step 6: 验证——前端构建 + Go 构建 + 无悬空引用**

Run（前端）:
```bash
cd web/admin && (npm ci || npm install) && npm run build
```
Expected: 构建成功（无对已删 `entries` 视图的悬空 import）。

Run（后端 + 引用检查）:
```bash
cd /home/daifei/agentwiki
go build ./cmd/... ./internal/... ./pkg/...
grep -rn "views/entries\|web/landing\|static/css/style" --include="*.go" --include="*.js" --include="*.vue" . | grep -v node_modules
```
Expected: Go 构建绿；grep 无命中（无任何代码引用已删文件）。

- [ ] **Step 7: 提交**

```bash
git add -A web/admin/src web/landing web/static internal/storage/model/models.go
git commit -m "chore(broken-feature): remove dead content-review UI shell + orphan frontend (R3-C)"
```

---

# R3-D：CORS 混合配置加固

闭合：`NewCORSMiddleware` 对混合 `["*","https://x"]` 配置不规范化，`Middleware`（:64）走 else 反射任意 Origin（虽 credentials 已强关，反射路径仍是设计瑕疵）。

## Task D: 混合通配符配置剔除 `*`

**Files:**
- Modify: `internal/api/middleware/cors.go`（`NewCORSMiddleware` :38-55）
- Test: `internal/api/middleware/cors_test.go`（新增 `TestCORSMiddleware_MixedWildcardNotReflected`）

**Interfaces:**
- Produces: `NewCORSMiddleware` 规范化输入——若 `AllowedOrigins` 同时含 `*` 与具体 origin，`log.Warn` 并剔除 `*`（白名单优先）。纯 `["*"]` 与纯白名单行为不变。
- Consumes: `log`（标准库）。

- [ ] **Step 1: 写失败测试**（追加到 `cors_test.go`）

```go
// TestCORSMiddleware_MixedWildcardNotReflected: 混合 ["*","https://x"] 配置下，
// 任意 Origin 不得被反射（R3-D：NewCORSMiddleware 应剔除 *，等价纯白名单）。
func TestCORSMiddleware_MixedWildcardNotReflected(t *testing.T) {
	mw := NewCORSMiddleware(CORSConfig{
		AllowedOrigins: []string{"*", "https://allowed.com"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()
	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "https://evil.com" || got == "*" {
		t.Errorf("混合配置不得反射任意 origin，got %q", got)
	}
}

// TestCORSMiddleware_MixedWildcardStillAllowsWhitelisted: 混合配置剔除 * 后，
// 白名单内的 origin 仍正常反射。
func TestCORSMiddleware_MixedWildcardStillAllowsWhitelisted(t *testing.T) {
	mw := NewCORSMiddleware(CORSConfig{
		AllowedOrigins: []string{"*", "https://allowed.com"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Origin", "https://allowed.com")
	rec := httptest.NewRecorder()
	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.com" {
		t.Errorf("白名单 origin 应被反射，got %q", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/api/middleware/ -run TestCORSMiddleware_MixedWildcard -v`
Expected: FAIL（`MixedWildcardNotReflected`：当前 `isOriginAllowed("https://evil.com")` 因 `*` 返 true，`:64` 单一 `*` 判断不成立走 else 反射 `https://evil.com`）

- [ ] **Step 3: NewCORSMiddleware 规范化混合配置**（`cors.go:38-55`）

加 `log` import，并在 `NewCORSMiddleware` 开头（默认配置回填之后）加混合剔除：

```go
// NewCORSMiddleware 创建 CORS 中间件实例
func NewCORSMiddleware(config CORSConfig) *CORSMiddleware {
	if len(config.AllowedOrigins) == 0 {
		config = DefaultCORSConfig()
	}
	// R3-D：混合配置（同时含 * 与具体 origin）规范化——剔除 *，白名单优先。
	// 否则 isOriginAllowed 对任意 origin 返 true，而 Middleware(:64) 的"单一 *"判断
	// 不成立，会走 else 把任意 Origin 反射回 Access-Control-Allow-Origin。
	if hasOriginWildcard(config.AllowedOrigins) && len(config.AllowedOrigins) > 1 {
		log.Printf("[CORS] 配置同时含 \"*\" 与具体 origin，剔除 \"*\"（白名单优先）: %v", config.AllowedOrigins)
		filtered := make([]string, 0, len(config.AllowedOrigins)-1)
		for _, o := range config.AllowedOrigins {
			if o != "*" {
				filtered = append(filtered, o)
			}
		}
		config.AllowedOrigins = filtered
	}
	// CORS 规范不允许通配符 origin 与 credentials 同时启用——浏览器会拒绝。
	// 作为防线，即便配置错误也强制降级。
	if config.AllowCredentials {
		for _, o := range config.AllowedOrigins {
			if o == "*" {
				config.AllowCredentials = false
				break
			}
		}
	}
	return &CORSMiddleware{
		config: config,
	}
}

// hasOriginAllowedWildcard 报告 origins 是否含 "*"。
func hasOriginWildcard(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}
```

import 块加 `"log"`：

```go
import (
	"log"
	"net/http"
	"strconv"
	"strings"
)
```

- [ ] **Step 4: 跑测试确认通过 + race + 全量**

Run: `go test -race -count=1 ./internal/api/middleware/`
Expected: PASS（含既有 `TestCORSMiddleware_*` 全部不回归——纯 `["*"]` 与纯白名单路径未改）

- [ ] **Step 5: 提交**

```bash
git add internal/api/middleware/cors.go internal/api/middleware/cors_test.go
git commit -m "fix(broken-feature): CORS drops wildcard from mixed origin config (R3-D)"
```

---

# R3-E：统计条目维度

闭合：现有 4 个统计端点全为用户维度，缺条目维度（总数/状态分布/分类分布/评分分布）。

## Task E: 新增 `/admin/stats/entries` 条目统计

**Files:**
- Modify: `internal/storage/model/models.go`（接 `UserStats` :214 后新增 `EntryStats` + `CategoryCount`）
- Modify: `internal/core/user/stats_service.go`（新增 `GetEntryStats` + `topCategories` + 缓存字段 `entryStats/entryStatsAt` + `invalidateLocked` 清缓存）
- Modify: `internal/api/handler/stats_handler.go`（新增 `GetEntryStatsHandler`）
- Modify: `internal/api/admin/handler.go`（新增 `GetEntryStatsHandler` 委托）
- Modify: `internal/api/router/router.go`（`registerAdminRoutes` :495 后注册）
- Test: `internal/core/user/stats_service_test.go`（新增 `TestStatsService_GetEntryStats`）

**Interfaces:**
- Produces:
  - `model.EntryStats`（`TotalEntries/DraftCount/PublishedCount/ArchivedCount/DeletedCount/ReviewCount/TopCategories []CategoryCount/ScoreBuckets map[string]int64`）
  - `model.CategoryCount`（`Category/Count`）
  - `(*StatsService).GetEntryStats(ctx) (*model.EntryStats, error)`（60s 缓存）
  - `(*handler.StatsHandler).GetEntryStatsHandler(w, r)`
  - `(*admin.Handler).GetEntryStatsHandler(w, r)`
  - 路由 `GET /api/v1/admin/stats/entries`（admin session 认证）
- Consumes: `store.Entry.List(ctx, EntryFilter{Limit: 100000})`（既有）；`model.EntryStatus*` 常量。

- [ ] **Step 1: 写失败测试**（追加到 `stats_service_test.go`；若文件不存在则新建，包 `user`）

```go
func TestStatsService_GetEntryStats(t *testing.T) {
	store, err := storage.NewMemoryStore()
	require.NoError(t, err)
	ctx := context.Background()

	// 多状态 / 多分类 / 不同评分
	mustCreate := func(id, cat string, status model.EntryStatus, score float64) {
		_, e := store.Entry.Create(ctx, &model.KnowledgeEntry{
			ID: id, Title: "t", Content: "c", Category: cat, Status: status, Score: score,
		})
		require.NoError(t, e)
	}
	mustCreate("e1", "ai", model.EntryStatusPublished, 4.5)
	mustCreate("e2", "ai", model.EntryStatusPublished, 3.5)
	mustCreate("e3", "math", model.EntryStatusDraft, 0)
	mustCreate("e4", "ai", model.EntryStatusArchived, 2.0)

	svc := NewStatsService(store)
	svc.SetCacheTTL(0) // 禁用缓存，确保实时聚合

	stats, err := svc.GetEntryStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(4), stats.TotalEntries)
	assert.Equal(t, int64(2), stats.PublishedCount)
	assert.Equal(t, int64(1), stats.DraftCount)
	assert.Equal(t, int64(1), stats.ArchivedCount)
	assert.Equal(t, int64(0), stats.DeletedCount)

	// Top category: ai(3) > math(1)
	require.NotEmpty(t, stats.TopCategories)
	assert.Equal(t, "ai", stats.TopCategories[0].Category)
	assert.Equal(t, int64(3), stats.TopCategories[0].Count)

	// 评分分桶：4.5→4-5, 3.5→3-4, 2.0→2-3, e3 score=0 不入桶
	assert.Equal(t, int64(1), stats.ScoreBuckets["4-5"])
	assert.Equal(t, int64(1), stats.ScoreBuckets["3-4"])
	assert.Equal(t, int64(1), stats.ScoreBuckets["2-3"])
	assert.Equal(t, int64(0), stats.ScoreBuckets["0-1"])
}
```

文件头确保 import（`context`、`testing`、`require`/`assert`、`storage`、`model`）。若新建文件：

```go
package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/core/user/ -run TestStatsService_GetEntryStats -v`
Expected: FAIL（`GetEntryStats` undefined；`model.EntryStats` undefined）

- [ ] **Step 3: 新增 model 类型**（`internal/storage/model/models.go`，接 `UserStats` :214 后）

```go
// EntryStats 条目统计信息（admin 仪表盘 /stats/entries）
type EntryStats struct {
	TotalEntries   int64            `json:"totalEntries"`   // 总条目数
	DraftCount     int64            `json:"draftCount"`     // draft
	PublishedCount int64            `json:"publishedCount"` // published
	ArchivedCount  int64            `json:"archivedCount"`  // archived
	DeletedCount   int64            `json:"deletedCount"`   // deleted
	ReviewCount    int64            `json:"reviewCount"`    // review（未实现，预留）
	TopCategories  []CategoryCount  `json:"topCategories"`  // 条目数 Top-10 分类
	ScoreBuckets   map[string]int64 `json:"scoreBuckets"`   // 评分分桶："0-1".."4-5"
}

// CategoryCount 分类计数（TopCategories 元素）
type CategoryCount struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}
```

- [ ] **Step 4: 实现 GetEntryStats**（`internal/core/user/stats_service.go`）

`StatsService` 结构（:25-41）加两个缓存字段：

```go
	entryStats   *model.EntryStats
	entryStatsAt time.Time
```

`invalidateLocked`（:70-78）末尾追加：

```go
	s.entryStats = nil
```

在文件末尾追加 `GetEntryStats` 与 `topCategories`：

```go
// GetEntryStats 获取条目统计（按 TTL 缓存）。内存聚合全量条目。
func (s *StatsService) GetEntryStats(ctx context.Context) (*model.EntryStats, error) {
	s.mu.RLock()
	if s.entryStats != nil && s.fresh(s.entryStatsAt) {
		out := *s.entryStats
		s.mu.RUnlock()
		return &out, nil
	}
	s.mu.RUnlock()

	entries, total, err := s.store.Entry.List(ctx, storage.EntryFilter{Limit: 100000})
	if err != nil {
		return nil, err
	}

	stats := &model.EntryStats{
		TotalEntries: total,
		ScoreBuckets: map[string]int64{"0-1": 0, "1-2": 0, "2-3": 0, "3-4": 0, "4-5": 0},
	}
	catCount := make(map[string]int64)
	for _, e := range entries {
		switch e.Status {
		case model.EntryStatusDraft:
			stats.DraftCount++
		case model.EntryStatusPublished:
			stats.PublishedCount++
		case model.EntryStatusArchived:
			stats.ArchivedCount++
		case model.EntryStatusDeleted:
			stats.DeletedCount++
		case model.EntryStatusReview:
			stats.ReviewCount++
		}
		if e.Category != "" {
			catCount[e.Category]++
		}
		if e.Score > 0 {
			switch {
			case e.Score < 1:
				stats.ScoreBuckets["0-1"]++
			case e.Score < 2:
				stats.ScoreBuckets["1-2"]++
			case e.Score < 3:
				stats.ScoreBuckets["2-3"]++
			case e.Score < 4:
				stats.ScoreBuckets["3-4"]++
			default:
				stats.ScoreBuckets["4-5"]++
			}
		}
	}
	stats.TopCategories = topCategories(catCount, 10)

	s.mu.Lock()
	s.entryStats = stats
	s.entryStatsAt = time.Now()
	s.mu.Unlock()
	return stats, nil
}

// topCategories 按 count 降序取前 n 个分类。
func topCategories(counts map[string]int64, n int) []model.CategoryCount {
	type kv struct {
		cat string
		cnt int64
	}
	list := make([]kv, 0, len(counts))
	for k, v := range counts {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].cnt > list[j].cnt })
	if len(list) > n {
		list = list[:n]
	}
	out := make([]model.CategoryCount, len(list))
	for i, v := range list {
		out[i] = model.CategoryCount{Category: v.cat, Count: v.cnt}
	}
	return out
}
```

- [ ] **Step 5: 新增 handler + 委托 + 路由**

`stats_handler.go` 末尾追加：

```go
// GetEntryStatsHandler 获取条目统计
// GET /api/v1/admin/stats/entries
func (h *StatsHandler) GetEntryStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	stats, err := h.statsSvc.GetEntryStats(r.Context())
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
```

`admin/handler.go` 末尾追加委托：

```go
// GetEntryStatsHandler 获取条目统计处理器
func (h *Handler) GetEntryStatsHandler(w http.ResponseWriter, r *http.Request) {
	h.statsHandler.GetEntryStatsHandler(w, r)
}
```

`router.go` `registerAdminRoutes`（:494-495 后）追加：

```go
	mux.Handle("/api/v1/admin/stats/entries",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetEntryStatsHandler)))
```

- [ ] **Step 6: 跑测试确认通过 + race + 受影响包**

Run: `go test -race -count=1 ./internal/core/user/ ./internal/api/handler/ ./internal/api/admin/ ./internal/api/router/`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/storage/model/models.go internal/core/user/stats_service.go internal/core/user/stats_service_test.go internal/api/handler/stats_handler.go internal/api/admin/handler.go internal/api/router/router.go
git commit -m "feat(broken-feature): add /admin/stats/entries entry-dimension stats (R3-E)"
```

---

# R3-F：SPA 刷新回归测试

闭合：admin SPA 深层路径刷新已由 `static.go` fallback 修复，补一个针对深层路由的回归测试锁死。

## Task F: 深层 SPA 路由刷新回归测试

**Files:**
- Test: `internal/api/admin/static_test.go`（新增 `TestStaticHandler_ServeHTTP_DeepRefresh`）

**Interfaces:**
- Produces: 无代码改动，纯回归测试。
- Consumes: `admin.NewStaticHandler()`（既有）。

- [ ] **Step 1: 写回归测试**（追加到 `static_test.go`）

```go
// TestStaticHandler_ServeHTTP_DeepRefresh: 深层 SPA 路由刷新（如被删的内容审核入口
// /admin/entries）必须回 index.html（200，非 404）。锁死 R3-C 清理后仍能正常 fallback。
func TestStaticHandler_ServeHTTP_DeepRefresh(t *testing.T) {
	h := NewStaticHandler()
	req := httptest.NewRequest(http.MethodGet, "/admin/entries", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `<div id="app">`, "深层 SPA 路由刷新应回 index.html")
}
```

- [ ] **Step 2: 跑测试确认通过**（已修复行为，应直接 PASS；若 R3-C 已合则 /admin/entries 无前端路由，但 static fallback 仍返回 index.html）

Run: `go test -race -count=1 ./internal/api/admin/`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/admin/static_test.go
git commit -m "test(broken-feature): regression test for deep SPA route refresh (R3-F)"
```

---

# 验收（R3 收尾）

- [ ] `go build ./cmd/... ./internal/... ./pkg/...` 绿。
- [ ] `go vet ./...` 绿。
- [ ] `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` 全绿。
- [ ] `golangci-lint run --timeout=10m ./...` exit 0。
- [ ] `cd web/admin && npm run build` 通过。
- [ ] 手动核对（由自动化测试覆盖）：
  - 审计日志中 email / ResponseBody 敏感字段均脱敏（R3-A）。
  - panic 响应 `code == 500`（非 0）（R3-B）。
  - admin SPA 无"内容审核"入口（R3-C）。
  - 混合 CORS 配置不反射任意 Origin（R3-D）。
  - `/api/v1/admin/stats/entries` 返回正确聚合（R3-E）。
  - `/admin/entries` 刷新回 index.html（R3-F）。
- [ ] R3 合并到 master 后开 R4 cycle（完整内容审核流程 + KV backup/restore + 选举/导出导入 UI + MCP 扩展 + unused/staticcheck/gosec/errcheck linter 专项清理）。

# 自审清单（writing-plans）

- **Spec 覆盖**：spec R3-A→plan R3-A；R3-B→R3-B；R3-C→R3-C；R3-D→R3-D；R3-E→R3-E；R3-F→R3-F。spec 审计表的 8 项（SPA 刷新→R3-F、条目审核后端死常量→R3-C 注释保留、条目审核前端死壳→R3-C、统计缺条目维度→R3-E、UI 对齐→并入 R3-C、panic code:0→R3-B、CORS→R3-D、审计明文 body→R3-A、孤儿文件→R3-C）全覆盖。
- **类型一致**：`model.EntryStats`/`CategoryCount`（R3-E 定义）在 `GetEntryStats`、`GetEntryStatsHandler`、测试中一致；`GetEntryStats(ctx) (*model.EntryStats, error)` 在 service/handler/委托签名一致；`hasOriginWildcard`（R3-D）定义与调用一致；`fakeAuditStore`（R3-A）实现 `kv.AuditStore` 五方法签名与 `audit_store.go:23-29` 一致。
- **占位符**：无 TBD/TODO。R3-C 的 npm 构建给了 `npm ci || npm install` 容错；R3-E 测试 `mustCreate` 内联定义非占位。
- **TDD 顺序**：每 task 先写失败测试（含完整测试代码）→ 跑确认失败 → 实现（完整代码）→ 跑通过 → 提交。R3-C（纯删除）以 build + grep 无引用为验证，符合"删除型 task"特性。R3-F（锁死已修复行为）测试加完即 PASS，属回归测试。
- **风险已标**：R3-A 现有 audit_test.go :27 用例 expected 须同步改（email 入表后要掩）；R3-A 标量正则与字符串正则共用 `MaskSensitiveFields` 的 `SplitN` 重写逻辑（已验证对两种匹配都正确）；R3-B middleware 不 import handler（循环依赖），直接写字面 JSON；R3-C 删 router entries 路由后注意无尾逗号；R3-D 混合配置剔除 `*` 不影响纯 `["*"]`（既有测试 `TestCORSMiddleware_*` 不回归）；R3-E 评分 score=0 不入桶。
