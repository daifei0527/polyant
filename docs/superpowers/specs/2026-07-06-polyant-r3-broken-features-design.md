# R3 坏功能修复设计

**范围**: 4 轮"分阶段全扫荡"迭代的第三轮——坏功能修复（Tier 3）。R1（安全）/R2（正确性）已合并入 master，本轮闭合 R1 spec 备忘的 R3 坏功能清单经审计核实后的真实缺陷：审计明文落库、panic 假成功、admin 内容审核死壳、CORS 混合配置反射、统计缺条目维度，以及复核已修复的 SPA 刷新与清理孤儿前端文件。

**轮次定位**：R3 只"修坏"与"清死壳"，不"建新功能"。完整的内容审核流程（draft→review→published + 审核者角色 + 审核队列 + admin 审核后台）属新功能，留给 R4。本轮把"内容审核"对应的死壳清除，使其不再误导。

## 目标

闭合 6 组共 8 项坏功能缺陷，使：

- 审计日志不再明文留存敏感字段（含 ResponseBody、email、数字/布尔型密钥）。
- panic 不再返回 `code:0` 假成功，前端可正确判错。
- admin 后台不再有"内容审核"死菜单/死视图（点击无数据）。
- CORS 在混合 `["*","https://x"]` 配置下不再反射任意 Origin。
- admin 统计补齐条目维度（总数/状态/分类/评分分布）。
- SPA 刷新行为有回归测试锁死；孤儿前端文件清除。

## 非目标（R4 范围）

- 完整内容审核流程（审核 handler、审核者 RBAC、审核队列、draft→review→published 转换、admin 审核后台 UI）。
- KV backup/restore + GC 调度、选举/导出导入 UI、MCP 工具扩展等新功能。
- `EntryStatusReview` 常量保留（R4 审核流程要用），仅补注释标注"未实现"。

## 审计核实结果（R1 spec 备忘 7 项 + 新发现）

| 备忘项 | 核实结论 | R3 处置 |
|---|---|---|
| admin SPA 刷新 | **已修复**（`static.go` fallback + sessionStorage 会话恢复） | R3-F 加回归测试锁死 |
| 条目审核实现 | 后端 `EntryStatusReview` 死常量；前端 admin `/entries` 死菜单+死视图 | R3-C 清死壳；后端常量保留+注释 |
| 统计端点 | 用户维度真统计 OK，**缺条目维度** | R3-E 补 `/stats/entries` |
| UI 对齐 | 具象化为"内容审核 UI 死壳"（R3-C） | 并入 R3-C |
| panic code:0 | 真坏（`logging.go:73`） | R3-B 修 |
| CORS | 默认安全，混合配置反射小瑕疵（`cors.go:64`） | R3-D 加固 |
| 审计明文 body | 真坏，中高（**新发现，最严重**） | R3-A 脱敏 |
| _(新发现)_ 孤儿前端文件 | `web/landing/index.html`、`web/static/css/style.css` 无引用 | R3-C 顺手清 |

## 架构：6 个 task 组

每个 task 组独立可提交（TDD，独立测试周期，独立 commit）。前缀：`fix(broken-feature)` / `feat(broken-feature)` / `chore(broken-feature)` / `refactor(broken-feature)`。

- **R3-A 审计脱敏**（隐私/安全，中高）
- **R3-B panic 不假成功**（中间件，中）
- **R3-C 清死壳 + 孤儿文件**（前端 + 清理，中/低）
- **R3-D CORS 混合配置加固**（中间件，低）
- **R3-E 统计条目维度**（功能补全，低）
- **R3-F SPA 刷新回归测试**（测试加固，低）

## 技术栈

Go 1.25.x / net/http / regexp / Vue 3 + Vite（admin SPA）/ 标准测试 + `-race`。

---

## R3-A：审计脱敏（闭合明文 body）

**根因**（`internal/core/audit/audit.go`）：

1. `Log`（:62-69）只对 `RequestBody` 调 `MaskSensitiveFields`，`ResponseBody` 仅 `TruncateString`（:65）——登录返回的 token、admin/export 导出的私钥/密码、批量操作回显的敏感数据**明文落库**。
2. `sensitiveFields`（:14-19）缺 `email`——而 register/verify-email/update 恰在 `sensitiveOps` 强制审计列表（:27-29），邮箱 PII 明文记录。
3. `init()` 正则（:46）`"[^"]*"` 只掩**字符串值**——数字/布尔类型的 password/token 漏掩。
4. 字段表不全：缺 `new_password`/`old_password`/`confirm_password`/`passphrase`/`mnemonic`/`seed`/`refresh_token`/`access_token`。

**方案**：

1. 扩充 `sensitiveFields`：加入 `email`、`new_password`、`old_password`、`confirm_password`、`passphrase`、`mnemonic`、`seed`、`refresh_token`、`access_token`。
2. 正则增强：将 `"[^"]*"`（仅字符串）改为匹配任意标量值（字符串/数字/布尔/`null`）。每个敏感字段编译两条模式：字符串值 `(?i)"field"\s*:\s*"[^"]*"` 与标量值 `(?i)"field"\s*:\s*(?:-?\d+(?:\.\d+)?|true|false|null)`。所有命中统一替换为 `"field": "***"`（无论原值类型，审计 body 是字符串，统一 `"***"` 最简且安全）。
3. `Log` 对 `ResponseBody` 也调 `MaskSensitiveFields`（先脱敏再截断，顺序：mask → truncate）。
4. `MaskSensitiveFields` 函数签名不变（输入/输出 `string`），保持调用方零改动。

**接口变化**：无外部接口变化；`MaskSensitiveFields` 行为增强（覆盖更全）。`audit` 中间件（`internal/api/middleware/audit.go`）抓取 body 后调 `Service.Log`，无需改。

**测试**（新增 `internal/core/audit/audit_mask_test.go`）：

- 字符串型敏感字段（password/token/email）→ 全部被掩为 `"***"`，字段名保留。
- 数字型/布尔型敏感字段（如 `"code":123456`、`"remember":true`）→ 被掩。
- `email` 字段 → 被掩（回归 R3-A 核心）。
- 非敏感字段（agent_name/title）→ 原样保留。
- 嵌套 JSON / 多字段同行 → 各自独立掩。
- `Log` 同时传 RequestBody + ResponseBody 含敏感字段 → 两者落库前均被掩（用 spy AuditStore 断言）。

---

## R3-B：panic 不再假成功

**根因**（`internal/api/middleware/logging.go:66-78`）：`RecoveryMiddleware` recover 后写 `{"code":0,"message":"internal server error"}`。按本仓库 `APIResponse` 约定（`internal/api/handler/types.go:6-10`），`code:0` = 成功——HTTP 500 但 body code:0 自相矛盾，前端按业务 code 判错会把 panic 当成功放行，造成假成功静默失败。

**方案**：

1. recover 后改写非 0 错误码（`code:500`），message 保持 `"internal server error"`。
2. 直接在 middleware 内序列化 JSON（middleware 不可 import handler——会循环依赖；handler 已 import middleware）。JSON 字符串字面量或 `encoding/json` + 局部 struct 均可。
3. 排障：`X-Request-Id` 由外层 `RequestIDMiddleware` 已设入 `w.Header()`（中间件装配顺序保证 RequestID 在 Recovery 之外），panic 响应自动携带，无需额外生成。

**接口变化**：panic 响应 body 的 `code` 由 `0` 改为 `500`（破坏性仅对"错误地依赖 panic 返回 code:0"的调用方，不存在该调用方）。

**测试**（`internal/api/middleware/recovery_test.go` 新增）：

- 注入会 panic 的 handler，经 `RecoveryMiddleware` 包装，断言：HTTP 500、body `code == 500`（非 0）、`message` 非空。
- 正常 handler（不 panic）透传不受影响。

---

## R3-C：清死壳 + 孤儿前端文件

**根因**：

- admin SPA 有"内容审核"入口（`web/admin/src/components/Sidebar.vue` 菜单项 → `/entries`；`router/index.js` 注册 `/entries`、`/entries/:id`；`views/entries/List.vue` + `Detail.vue`），但 `web/admin/src/api/entries.js` 不存在、`List.vue` 是 `// Placeholder` 注释、后端 `router.go` 无 `/api/v1/admin/entries*` 路由——点击无数据，纯死壳。
- 孤儿文件：`web/landing/index.html`（53KB）、`web/static/css/style.css`（544 行）——R1 删公开阅读前端后的遗留，无任何 Go 路由/embed 引用。

**方案**：

1. 前端死壳清除：删 `Sidebar.vue` 的"内容审核"菜单项；删 `router/index.js` 的 `/entries`、`/entries/:id` 路由；删 `views/entries/List.vue`、`views/entries/Detail.vue` 整个目录（若有 `api/entries.js` 残留一并删）。
2. 后端 `EntryStatusReview` 常量（`internal/storage/model/models.go:32`）**保留**，注释改为 `// 审核中（未实现，完整审核流程见 R4）`。
3. 孤儿文件清除：删 `web/landing/index.html`、`web/static/css/style.css`；若 `web/landing/`、`web/static/css/` 目录清空则删目录。

**接口变化**：无后端接口变化。admin SPA 不再暴露"内容审核"入口（R4 实现完整审核时重新加回）。

**测试**：

- 前端：`cd web/admin && npm install && npm run build` 通过（无悬空 import）。
- 后端：`go build ./cmd/... ./internal/... ./pkg/...` 绿。
- 确认无 Go 代码引用被删前端文件（`grep` 验证）。

---

## R3-D：CORS 混合配置加固

**根因**（`internal/api/middleware/cors.go`）：`isOriginAllowed`（:108-118）遇 `*` 即返 true；`Middleware`（:64）仅在"单一 `*`"时回 `*`，否则反射 Origin。当配置为混合列表 `["*","https://x"]` 时，`:64` 判断不成立走 else，把任意 Origin 反射回 `Access-Control-Allow-Origin`。虽 `NewCORSMiddleware`（:44-51）已强关 credentials 限制危害，但反射路径本身是设计瑕疵。

**方案**：

`NewCORSMiddleware` 规范化配置：若 `AllowedOrigins` 同时含 `*` 与具体 origin，启动时 `log.Warn` 提示，并**剔除 `*`（具体白名单优先）**。规范化后：

- 纯 `["*"]` → 旧行为（回 `*`）。
- 纯白名单 → `:64` 走 else 反射命中的 origin（正确）。
- 混合 → 剔除 `*` 后等价纯白名单（不再反射任意 origin）。

**接口变化**：`NewCORSMiddleware` 输入规范化（混合配置剔除 `*`）；默认 `DefaultCORSConfig`（纯 `*`）行为不变。

**测试**（`internal/api/middleware/cors_test.go` 新增/扩展）：

- 混合配置 `["*","https://x"]` + Origin `https://evil.com` → `Access-Control-Allow-Origin` 不反射 `https://evil.com`（为空或 `https://x`）。
- 纯 `["*"]` + 任意 Origin → 回 `*`（不回归）。
- 纯白名单 + 命中 Origin → 反射该 origin（不回归）。

---

## R3-E：统计条目维度

**根因**（`internal/api/handler/stats_handler.go` + `internal/core/user/stats_service.go`）：现有 4 个统计端点（users/contributions/activity/registrations）全部用户维度，**完全缺条目维度**（无条目总数、状态分布、分类分布、评分分布）。

**方案**：

1. `stats_service.go` 新增 `GetEntryStats(ctx) (*EntryStats, error)`：调 `entryStore.List(全量)` 内存聚合——
   - 总数、按 `Status` 分布（draft/published/archived/deleted/review）、Top-N `Category`、评分分布（`Score` 分桶：0-1/1-2/2-3/3-4/4-5）。
   - 复用现有 60s TTL 缓存模式（`stats_service.go:13,81-83`）。
2. `stats_handler.go` 新增 `GetEntryStatsHandler`：`GET /api/v1/admin/stats/entries`，沿用 `writeJSON` + `APIResponse` 模式。
3. `router.go` `registerAdminRoutes` 注册 `/stats/entries`（与既有 `/stats/users` 等同级）。
4. `EntryStats` 结构定义在 `user` 包或 `model` 包（与 `UserStats` 同位置）。

**接口变化**：新增只读端点 `GET /api/v1/admin/stats/entries`（admin 权限）。无破坏性。

**测试**（`stats_handler_test.go` / `stats_service` 测试扩展）：

- 构造多状态/多分类/不同评分的条目，断言聚合：状态分布计数正确、Top-N 分类正确、评分分桶正确。
- 缓存命中：第二次调用不重算（可选）。

---

## R3-F：SPA 刷新回归测试

**根因**：admin SPA 刷新已修复（`internal/api/admin/static.go:36-56` fallback + sessionStorage 会话恢复），`static_test.go` 已覆盖 root/SPARoute/StaticFile/index 四场景，但缺"深层路径刷新回 index.html"的显式回归。

**方案**：`internal/api/admin/static_test.go` 补一个用例：请求 `/admin/entries`（不存在的深层 SPA 路由）→ 响应 index.html 内容 + 200（非 404）。锁死已修复行为，防回归。

**测试**：即上述用例。

---

## 全局约束

- Go 版本：`go 1.25.7`（go.mod）；CI `1.25.x`。
- 每个改动**先写失败测试再实现**（TDD）。
- 提交信息前缀：`fix(broken-feature)` / `feat(broken-feature)` / `chore(broken-feature)` / `refactor(broken-feature)`，尾部带 `(R3-X)` 标签。
- 每个 task 结束运行 `go build ./cmd/... ./internal/... ./pkg/... && go vet ./... && go test -race -count=1 <受影响包>`，全绿才提交。
- 前端改动（R3-C）额外 `cd web/admin && npm run build` 通过。
- 中文注释 OK，沿用周边文件风格。
- 本设计所有 file:line 引用以 2026-07-06 master `876495f`（R2 合入后）为准；实现时若行号漂移按符号名定位。

## 验收

- `go build ./cmd/... ./internal/... ./pkg/...` 绿。
- `go vet ./...` 绿。
- `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` 全绿。
- `golangci-lint run --timeout=10m ./...` exit 0。
- `cd web/admin && npm run build` 通过（R3-C 后）。
- 手动核对（由自动化测试覆盖）：
  - 审计日志中敏感字段（含 email、ResponseBody）均脱敏（R3-A）。
  - panic 响应 `code != 0`（R3-B）。
  - admin SPA 无"内容审核"死入口（R3-C）。
  - 混合 CORS 配置不反射任意 Origin（R3-D）。
  - `/api/v1/admin/stats/entries` 返回正确聚合（R3-E）。
  - `/admin/<deep>` 刷新回 index.html（R3-F）。

## 后续轮次（仅备忘）

- **R4 新功能**：完整内容审核流程（draft→review→published + 审核者 RBAC + 审核队列 + admin 审核后台 UI，复用 R3-C 清理出的入口位置）；KV backup/restore + GC 调度；选举/导出导入 UI；MCP 工具扩展；`unused`/`staticcheck`/`gosec`/`errcheck` linter 专项清理（见 `.golangci.yml` 注释）。
