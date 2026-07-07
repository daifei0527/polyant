# R4b 内容审核流程设计

**范围**：R4 第二个迷你轮——内容审核流程（旗舰功能）。R4a（linter 清理）已合入 master（`d756d42`）。R3-C 清掉了 admin 内容审核的死 UI 壳，本轮建真流程：条目不再一律创建即发布，改为信任分级触发审核队列，admin 在后台审批/拒绝/下架。

**轮次定位**：R4b 只建内容审核流程（模型字段 + 创建分支 + 状态转换 API + admin 后台 UI + 审计），不做专用 reviewer 权限、修订循环、每节点审核策略、创建者通知。

## 目标

让条目发布受信任度门控，并提供 admin 审核闭环：

- **信任分级触发**：Lv1/Lv2 用户创建的条目进入 `review` 队列；Lv3+ 用户直接 `published`（行为不变）。
- **状态机**：`create(Lv1/2)→review`；`review→published`（approve）；`review→archived`（reject，终态）；`published→review`（admin 下架重审）。
- **Reviewer**：admin 会话持有者（Lv4+，由 session-token 签发保证），通过 admin SPA 操作。
- **审核闭环**：admin 后台有审核队列（列出 `review` 条目）、approve/reject、对已发布条目 takedown；统计页 `ReviewCount` 显示真实数据。
- **审计**：approve/reject/takedown 落审计，记录操作者/原因。
- **P2P**：状态经现有 sync 原样同步；seed 为隐式审核权威。

## 非目标

- 专用 `PermReview`/`PermApprove` 权限（MVP 用 Lv4+ session 门控；后续可加）。
- 修订循环（reject 为终态 `archived`，不退回 `draft` 重提；创建者需重新创建）。
- 每节点独立审核策略（seed 权威，peer 接受 seed 状态）。
- 创建者邮件/通知。
- P2P review 状态专门协商（原样携带，不改 sync 协议）。
- admin SPA Ed25519 签名能力（不引入；review 走 session-token 专用端点）。

## 现状核实（代码 grounded）

| 能力 | 现状 | 位置 |
|---|---|---|
| 状态常量 Draft/Published/Archived/Deleted/Review | 已定义，Review 标注"未实现" | `models.go:28-32` |
| `EntryStats.ReviewCount` | 已接线（统计时计入），当前恒 0 | `models.go:223`、`stats_service.go:358` |
| `EntryFilter.Status` 过滤 | store 层已支持 | `store.go:37`、`memory.go:100` |
| 搜索排除非 published | 已实现 | `bleve_engine.go:307`、`memory.go:160` |
| 创建硬编码 published | **待改**：信任分级分支 | `entry_handler.go:287` |
| Reviewer 字段（ReviewedBy/At/Reason） | **缺失**，需新增 | `models.go:47-70` |
| 状态转换 API/handler | **缺失** | 新建 |
| Reviewer 权限 | 不新增（用 Lv4+ session） | — |
| admin SPA 可达 entry 端点 | 否（两套认证分离） | 挂新端点到 `adminAuthMW` |
| admin 中间件注入 level | 否（仅 PublicKeyKey） | `admin/middleware.go:49`（不改，用 session 签发门控） |
| 审计类型化状态转换 | 否（仅 generic entry.update） | 新增 action 类型 |
| P2P sync 携带 status | 是（原样） | `sync.go:778`（不改） |

**两套认证（关键）**：Ed25519 签名（`authMW`）守 entry CRUD/选举/导出导入；session-token（`adminAuthMW`）守 admin SPA（users/stats）。admin SPA 无法调 Ed25519 端点（不会签名）。故 review 端点挂 `adminAuthMW`，复用 session 签发时的 Lv4+ 门控（`session.go:70,137`）。

## 架构：认证方案

**方案：adminAuthMW 下专用 review 端点**（推荐方案，详见决策）。

- 新端点 `/api/v1/admin/entries`（list，支持 `?status=review|published|archived`）与 `/api/v1/admin/entries/{id}/{approve|reject|takedown}`，挂 `adminAuthMW`（session-token）。
- reviewer = admin 会话持有者；其 pubkey 从 context 的 `PublicKeyKey` 取（`admin/middleware.go:49` 已注入），其 level 由 session 签发保证（Lv4+），**handler 不再做 level 校验**（session 签发即门控）。
- admin SPA（`api/request.js` 已带 `Authorization: Bearer <token>`）直接调用，零额外认证改动。

被否方案：① 桥接 admin session 注入 `UserLevelKey` 复用 Ed25519 entry 端点（混认证栈、SPA 不会签名，走不通）；② 给 SPA 加 Ed25519 签名（过重）。

## 组件

### 1. 模型（`internal/storage/model/models.go`）

`KnowledgeEntry` 新增 3 字段：
```go
ReviewedBy   string // 审核者 pubkey（最后一次 approve/reject/takedown 的操作者）
ReviewedAt   int64  // 审核时间（毫秒）
ReviewReason string // 拒绝/下架原因（approve 时可空）
```
3 字段随 P2P sync 原样同步到 peer（信息性元数据）。JSON tag 保持蛇形（`reviewed_by`/`reviewed_at`/`review_reason`），与现有字段风格一致。

### 2. 创建流（`internal/api/handler/entry_handler.go`）

`CreateEntryHandler:287` 的 `Status: model.EntryStatusPublished` 改为按创建者 level 分支：
```go
status := model.EntryStatusPublished
if creator.UserLevel < model.UserLevelLv3 {
    status = model.EntryStatusReview
}
entry := &model.KnowledgeEntry{ ..., Status: status }
```
- `Lv3` 常量在 `models.go:19`（`UserLevelLv3`，贡献者）。
- **仅 published 才进索引**：现有 `entry_handler.go:309-324` 的 search/backlink/title 索引逻辑加 `if status == EntryStatusPublished` 守卫；review 跳过索引（搜索本就排除非 published，一致）。
- 创建后照常 `entryPusher.PushEntry`（`entry_handler.go:327-333`），status 原样推给 peer。

### 3. Review service + handler（新建）

**Service**（`internal/core/review/service.go`，照搬 election/admin service 范式）：
```go
type Service struct {
    store    *storage.Store
    pusher   EntryPusher // 复用 entry_handler 的推送接口，审批后同步 peer
}
func (s *Service) ListQueue(ctx, status string, limit, offset int) ([]*model.KnowledgeEntry, int, error)
func (s *Service) Approve(ctx, entryID, reviewerPubkey string) (*model.KnowledgeEntry, error)
func (s *Service) Reject(ctx, entryID, reviewerPubkey, reason string) (*model.KnowledgeEntry, error)
func (s *Service) Takedown(ctx, entryID, reviewerPubkey, reason string) (*model.KnowledgeEntry, error)
```
每个动作：`Entry.Get` → 校验当前 status 合法（approve/takedown 要求源状态；详见状态机表）→ 设新 status + ReviewedBy/At/Reason → `Entry.Update` → 索引增删（approve 进索引、takedown 出索引）→ `pusher.PushEntry` 同步 peer。

**状态机校验**（service 内）：
| 动作 | 要求源 status | 目标 status |
|---|---|---|
| Approve | `review` | `published` |
| Reject | `review` | `archived`（终态） |
| Takedown | `published` | `review` |

非法转换返回 409/400 错误（如 approve 一个 archived 条目）。

**Handler**（`internal/api/admin/review_handler.go`，挂 admin 包）：
- `GET /api/v1/admin/entries?status=review&page=1&limit=20` → `ListQueue`
- `POST /api/v1/admin/entries/{id}/approve` → `Approve`
- `POST /api/v1/admin/entries/{id}/reject`（body: `{reason}`） → `Reject`
- `POST /api/v1/admin/entries/{id}/takedown`（body: `{reason}`） → `Takedown`

reviewer pubkey 从 `r.Context().Value(mw.PublicKeyKey)` 取（admin 中间件已注入）。

**EntryPusher 复用**：`entry_handler` 已有 `EntryPusher` 接口（`PushEntry(entry, signature)`）。review service 注入同一接口（router 装配时传同一个 pusher 实例）。审批后用条目现有 signature 推送（signature 覆盖 `title\ncontent\ncategory`，status 变更不破坏签名）。

### 4. 审计（`internal/api/middleware/audit.go`）

`sensitiveOps`（:29-45）与 prefix 规则（:124-165）新增：
- `POST /api/v1/admin/entries/:id/approve` → action `entry.approve`
- `POST /api/v1/admin/entries/:id/reject` → action `entry.reject`
- `POST /api/v1/admin/entries/:id/takedown` → action `entry.takedown`

操作者信息（pubkey/level/IP/UA）由现有审计中间件自动捕获（`audit.go:86-101`）。reason 在 request body 中，会被审计 body 脱敏（R3-A 后 `MaskSensitiveFields` 覆盖；reason 非敏感字段，原样记录）。

### 5. Admin SPA（`web/admin/src/`）

- **`api/reviews.js`**（新）：`getReviewQueue({status, page, limit})`、`approveEntry(id)`、`rejectEntry(id, reason)`、`takedownEntry(id, reason)`。baseURL `/api/v1`，Bearer token（`request.js` 已配）。
- **`views/reviews/Index.vue`**（新）：队列表格（标题/创建者/分类/提交时间），按 status tab 切换（默认 `review`）。每行操作按钮：approve / reject（弹窗输 reason）。
- **条目管理（takedown）**：在 `reviews/Index.vue` 加 `published` tab，列出已发布条目，每行 takedown 按钮（弹窗输 reason）。MVP 不做独立 entries 管理页，复用 reviews 视图的 tab。
- **Sidebar**（`components/Sidebar.vue`）：加"内容审核"菜单项 → `/reviews`，`v-if="hasPermission(4)"`。
- **Router**（`router/index.js`）：加 `/reviews` → `reviews/Index.vue`，`meta.permission: 4`。
- **Stats**（`views/stats/Index.vue:78`）：把 mock `entryStats = ref({ total: 0 })` 改为真调 `getEntryStats()`（`api/stats.js` 补 `getEntryStats` 导出，后端 R3-E 已有）。展示 `ReviewCount` 等。

### 6. P2P

不改 sync 协议。review 动作后 `Entry.Update` + `pusher.PushEntry` 触发 peer 同步（status 原样）。peer 接受 seed 推来的 status（已发布/已归档/重审）。`HandlePushEntry`（`sync.go:758-815`）现有 LWW 冲突解决（按 Version/UpdatedAt）继续生效——review 动作 `Update` 会 bump Version/UpdatedAt，确保覆盖旧状态。

## 数据流

1. **Lv1 用户创建** → status=`review` → 不进索引 → push 到 seed（review 状态）。
2. **seed admin 开队列** → 看到 review 条目 → approve（→`published` + 建索引 + push）或 reject（→`archived` + reason + push，终态）。
3. **事后下架**：admin 在 published tab → takedown（→`review` + 移出索引 + push，回队列）。
4. **Lv3+ 用户创建** → status=`published`（不变）→ 建索引 + push。

## 测试

- **后端**（`internal/core/review/service_test.go` + handler test）：
  - 状态转换正确（approve review→published、reject review→archived、takedown published→review）。
  - 非法转换报错（如 approve 已 archived）。
  - approve 后进搜索索引（可搜到）；takedown 后移出（搜不到）。
  - reviewer 字段落库（ReviewedBy/At/Reason）。
  - 审计 action 类型正确。
- **创建流**（扩展 `entry_handler_test.go`）：Lv1 创建→review、Lv3 创建→published；review 不进索引。
- **SPA**：队列渲染、approve/reject/takedown 调用端点（mock backend）、stats 显示 ReviewCount。

## 接口变化

- `KnowledgeEntry` 加 3 字段（向后兼容，旧数据零值为空）。
- 新增 4 个 admin 端点（`/api/v1/admin/entries*`）。
- 无外部 SDK 变化（`polysdk` 创建路径不变，status 由服务端按 level 定）。

## 风险与回退

- **创建流行为变化**：Lv1/Lv2 条目不再立即可见（进审核）。**回退**：CreateEntryHandler 的 level 分支加配置开关 `RequireReviewForLowLevel bool`（默认 true），紧急时可关。
- **P2P 状态传播**：若 user node 的 Lv1 条目先本地 review、后 seed 也 review，无冲突（同 status）。若 seed approve 后 push，user node LWW 接受（Version 更高）。风险低。
- **索引一致性**：approve/takedown 必须同步增删索引，否则 review 条目泄漏到搜索。测试覆盖。
- **每个 task 独立 commit + 验证**，可单独 revert。

## 出范围跟踪

- 专用 `PermReview` 权限（后续可加，让 Lv3 也能审核）。
- 退回草稿修订循环（reject→draft 重提）。
- 创建者通知（审核结果推送）。
- 每节点审核策略（非 seed 节点独立审核）。
