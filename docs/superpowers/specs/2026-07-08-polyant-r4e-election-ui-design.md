# R4e 选举管理 UI 设计

**范围**：R4 第五个迷你轮——admin SPA 的选举管理界面 + 修复预存的 Ed25519 选举 handler pubkey bug。R4a/R4b/R4c/R4d 已合入 master（`3490838`）。选举后端完整（`ElectionService` + 7 端点），但：①admin 管理操作（create/close）挂 Ed25519，session-token SPA 够不到；②`election_handler.go` 4 处用裸字符串 `"public_key"` 读 context（无对应 setter）→ Ed25519 选举 API 完全失效（投票全归空 pubkey、`HasVoted("")` 阻断后续投票、创建者空）。本轮加 admin UI + 修 bug。

**轮次定位**：R4e 做 admin 选举管理 UI + bundled 修复。投票/提名/确认留 Ed25519（pactl/用户），不入 SPA。

## 目标

- **Admin SPA 选举管理**：创建选举、列表（active+closed）、查看详情（候选人 + 票数 + 状态，只读）、关闭选举。
- **新 session-token admin 端点**（4 个，挂 `adminAuthMW`），薄包装 `ElectionService`。
- **修复预存 bug**：`election_handler.go` 4 处裸字符串 `Value("public_key")` → `Value(mw.PublicKeyKey)`（typed），让 Ed25519 选举 API（create/nominate/vote/confirm）真正可用。

## 非目标

- SPA 内投票/提名/确认（留 Ed25519 pactl）。
- 候选人管理（admin 代提名等）。
- 选举通知/提醒/定时器 UI。
- 选举模型/服务改动（复用现成 `ElectionService`）。

## 现状核实（代码 grounded）

| 能力 | 现状 | 位置 |
|---|---|---|
| `ElectionService` | 完整（Create/Get/List/Nominate/Confirm/Vote/HasVoted/ListCandidates/Close） | `election/election.go:50+` |
| Ed25519 选举端点 | create/close=PermAdmin(Lv4+)；nominate/confirm=authed；vote=Lv3 | `router.go:405-417` |
| 选举 handler pubkey 读取 | **裸字符串 `"public_key"`，无 setter → 恒空（bug）** | `election_handler.go:70,193,249,330` |
| typed `PublicKeyKey` setter | admin middleware + Ed25519 auth 都设 | `admin/middleware.go:49`、`auth.go:208` |
| admin session context | 仅 `mw.PublicKeyKey`（typed） | `admin/middleware.go:48` |
| SPA 选举 UI/api | 无 | `web/admin/src/` |
| `CreateElectionHandler` createdBy | 从 context pubkey（裸字符串，bug） | `election_handler.go:70` |

## 架构：新 admin handler + bundled fix

**方案：新 admin 选举 handler（推荐）**。薄包装 `ElectionService`，4 端点挂 `adminAuthMW`，create 用 `mw.PublicKeyKey`。避开 Ed25519 handler 的硬编码路径解析，与 R4b/R4c/R4d 一致。

被否：复用现有 handler 挂 adminAuthMW（路径解析硬编码 `/api/v1/elections/`，与 `/admin/elections/` 不匹配）。

**bundled fix**：`election_handler.go` 4 处 `Value("public_key")` → `Value(mw.PublicKeyKey)`。这是独立修复（让 Ed25519 端点可用），不影响新 admin 端点（admin handler 本就用 typed key）。

## 组件

### 1. Bundled fix（`internal/api/handler/election_handler.go`）

4 处（行 70/193/249/330）：
```go
// 前：publicKey, _ := r.Context().Value("public_key").(string)
// 后：
publicKey, _ := r.Context().Value(mw.PublicKeyKey).(string)
```
加 `mw "github.com/daifei0527/polyant/internal/api/middleware"` import。这让：
- `CreateElectionHandler`（:70）：createdBy 拿到真实 admin pubkey。
- `NominateCandidateHandler`（:193）：nominatedBy 真实。
- `VoteHandler`（:249）：voterID 真实（修复"所有票归空 pubkey + HasVoted 阻断"）。
- `ConfirmNominationHandler`（:330）：userID 真实。

### 2. 新 admin 选举 handler（`internal/api/admin/election_handler.go`）

```go
type ElectionHandler struct {
	svc *election.ElectionService
}
func NewElectionHandler(svc *election.ElectionService) *ElectionHandler
func (h *ElectionHandler) CreateElectionHandler(w, r)    // POST /admin/elections，createdBy 从 mw.PublicKeyKey
func (h *ElectionHandler) ListElectionsHandler(w, r)     // GET /admin/elections?status=
func (h *ElectionHandler) GetElectionHandler(w, r)       // GET /admin/elections/{id}，含 candidates
func (h *ElectionHandler) CloseElectionHandler(w, r)     // POST /admin/elections/{id}/close
```
- create：解析 `CreateElectionRequest{Title,Description,VoteThreshold,DurationDays,AutoElect}`，`createdBy, _ := r.Context().Value(mw.PublicKeyKey).(string)`，调 `svc.CreateElection`。
- list：`?status=active|closed`（空=全部），调 `svc.ListElections`。
- detail：从路径取 id，调 `svc.GetElection` + `svc.ListCandidates`，返回 `{election, candidates}`。
- close：从路径取 id，调 `svc.CloseElection`，返回 `{elected}`。
- 路径解析用 admin 前缀（`/api/v1/admin/elections/`），照搬 R4b review handler 的 `entryIDFromPath` 模式。

### 3. admin.Handler 接线（`internal/api/admin/handler.go`）

- 加 `electionHandler *ElectionHandler` 字段 + 4 委托方法。
- `NewHandler` 用 `store.KVStore()` 构造 `ElectionService`（照搬 seed main `cmd/seed/main.go:273-278`）：
```go
kvStore := store.KVStore()
electionSvc := election.NewElectionService(
    kv.NewElectionStore(kvStore), kv.NewCandidateStore(kvStore), kv.NewVoteStore(kvStore),
)
h.electionHandler = NewElectionHandler(electionSvc)
```

### 4. 路由（`internal/api/router/router.go` `registerAdminRoutes`）

4 端点挂 `adminAuthMW`：
- `POST /api/v1/admin/elections` → CreateElectionHandler
- `GET /api/v1/admin/elections` → ListElectionsHandler
- `GET /api/v1/admin/elections/` → GetElectionHandler（{id} 后缀解析）
- `POST /api/v1/admin/elections/` → suffix 路由（`/{id}/close` → CloseElectionHandler）

照搬 R4b/R4d 的 admin 端点注册模式。

### 5. SPA

- **`api/elections.js`**：`listElections({status})`、`getElection(id)`、`createElection({title,description,voteThreshold,durationDays,autoElect})`、`closeElection(id)`。走 `request`（JSON envelope）。
- **`views/elections/List.vue`**：active/closed tab 切换 + "创建选举"按钮（弹 `ElMessageBox` 或 dialog 填表）+ 表格（标题/状态/阈值/时间/操作-查看/关闭）。
- **`views/elections/Detail.vue`**：election 信息（标题/描述/状态/阈值/时间/autoElect）+ 候选人表（UserName/VoteCount/Status nominated|elected|rejected/Confirmed）。
- **router**：`/elections` + `/elections/:id`（meta.permission 4）。
- **Sidebar**："选举管理"菜单项（v-if Lv4）+ 图标。

## 数据流

- **创建**：SPA 表单 → `POST /admin/elections`（Bearer）→ admin handler 读 `mw.PublicKeyKey` → `ElectionService.CreateElection` → election_id。
- **列表/详情**：SPA → `GET /admin/elections[?status=]` / `/{id}` → ElectionService → 渲染（详情含候选人票数，只读）。
- **关闭**：SPA → `POST /admin/elections/{id}/close` → `ElectionService.CloseElection` → 当选者。
- **投票/提名**（bundled fix 后）：用户经 Ed25519 pactl，handler 现在拿真实 pubkey。

## 测试

- **bundled fix**：扩 `election_handler_test.go`——context 设 `mw.PublicKeyKey`（typed）→ create/nominate/vote 拿到非空 createdBy/voterID/nominatedBy（之前裸字符串恒空）。现有测试多在 service 层；handler 层补 typed-key 覆盖。
- **admin handler**（`admin/election_handler_test.go`）：create（session context + mw.PublicKeyKey → createdBy 非空）、list（?status 过滤）、detail（含 candidates）、close；401 无 token。
- **SPA**：列表渲染、创建提交、详情候选人表、关闭（mock）。

## 接口变化

- 新增 4 个 admin 端点（`/api/v1/admin/elections*`，session-token）。
- bundled fix：Ed25519 选举端点行为修复（pubkey 不再空）——**修复性变更**，让失效功能可用。
- 无 ElectionService/模型改动。

## 风险与回退

- **bundled fix 改 Ed25519 行为**：之前 election API 静默拿空 pubkey（错），修复后真实。若有调用方依赖空值（极不可能），会变。CI 测试在 service 层不受影响；handler 层之前无覆盖（补测试）。
- **ElectionService 双构造**：admin.NewHandler 与 seed main 各构造一个实例（共享 KVStore）。ElectionStore 是 KV-backed 无内存状态，无冲突。
- **路径冲突**：`/api/v1/admin/elections`（新）与 `/api/v1/elections`（现有 public GET）路径不同（admin 前缀），无 mux 冲突。
- 每个 task 独立 commit + 验证；bundled fix 可单独 revert。

## 出范围跟踪

- SPA 内投票/提名/确认 UI。
- 候选人管理（admin 代提名）。
- 选举通知/定时。
