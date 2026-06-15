# Polyant 选举功能设计 (Election Design)

| 字段 | 值 |
|------|-----|
| 日期 | 2026-06-13 |
| 版本 | v1.0 |
| 状态 | 已实现（回溯性 spec） |
| 关联 | 代码先于文档落地；本 spec 由现有代码回溯整理。`internal/core/election/`、`internal/api/handler/election_handler.go`、`internal/storage/kv/election_store.go` |

---

## 1. 背景与动机

Polyant 的分类维护权限按用户等级（Lv0–Lv5）分层。等级提升、维护权交接需要一种**去中心化的链上共识机制**：由社区（达到一定等级的节点运营者/贡献者）投票选出候选人，达到阈值即当选，获得相应维护权。

选举功能提供这一共识载体：任何超级管理员（Lv5）可发起一次选举，达到 Lv3 的用户可投票，候选人达到 `VoteThreshold` 即当选。选举有明确截止时间，到期由后台任务自动关闭结算，避免过期选举永久挂起。

> **本 spec 为回溯性文档。** 选举代码已先于本 spec 实现并运行；本文件依据现有代码整理设计意图与契约，供后续维护与审计对照（"只信代码"原则下，代码为真相源，本 spec 与代码冲突时以代码为准）。

## 2. 范围

**包含：**
- 选举的创建、提名（自荐/他荐）、确认接受提名、投票、关闭结算。
- 候选人票数原子计票（防并发竞态）。
- 到期选举的后台自动关闭。
- 选举数据的 KV 持久化与按选举维度索引。

**不包含（已知边界）：**
- 选举数据**不参与 P2P 同步**——选举状态仅存于创建/操作所在节点（通常为种子节点）。跨节点选举共识不在当前实现范围。
- 当选结果**不自动授予链上权限**——`CandidateStatusElected` 仅标记当选，是否据此调整 `user_level`/维护权由上层（人工或后续自动化）决定。
- 选举**不签名、不上链**，无防篡改审计（区别于知识条目的 content_hash/creator_signature 完整性机制）。

## 3. 数据模型

定义于 `internal/storage/model/election.go`。

### 3.1 Election（一次选举）

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | string | UUID v4（`generateID()` → `crypto.GenerateUUID`） |
| `Title` / `Description` | string | 选举标题与描述 |
| `Status` | `ElectionStatus` | `active` / `closed` |
| `StartTime` / `EndTime` | int64 (Unix 毫秒) | 选举窗口；`EndTime = StartTime + duration` |
| `VoteThreshold` | int32 | 当选所需票数 |
| `AutoElect` | bool | 投票中达到阈值即即时当选（无需等关闭结算） |
| `CreatedAt` / `CreatedBy` | int64 / string | 创建时间 / 创建者公钥 |

派生判断：`IsClosed()`、`IsExpired()`（`now > EndTime`）、`ShouldAutoElect()`（`AutoElect && status==active`）。

### 3.2 Candidate（候选人）

| 字段 | 说明 |
|------|------|
| `ElectionID` / `UserID` / `UserName` | 所属选举 / 候选人公钥 / 姓名 |
| `NominatedBy` | 提名人公钥 |
| `SelfNominated` | 是否自荐（自荐时 `Confirmed` 自动置 true） |
| `Confirmed` / `ConfirmedAt` | 是否确认接受提名 |
| `VoteCount` | 当前得票（原子维护） |
| `Status` | `CandidateStatus`：`nominated` / `elected` / `rejected` |

`IsReady()` = `Confirmed`：只有确认接受提名的候选人才可被投票。

### 3.3 Vote（投票记录）

| 字段 | 说明 |
|------|------|
| `ID` | **`vote_<UnixNano>`**（见 §10 已知限制：非 UUID v4） |
| `ElectionID` / `VoterID` / `CandidateID` | 选举 / 投票人公钥 / 候选人公钥 |
| `VotedAt` | 投票时间 |

## 4. 选举生命周期

```
            create(Lv5)
               │
               ▼
        ┌─────────────┐   nominate(认证用户)    ┌──────────────┐
        │   active    │ ─────────────────────▶ │  Candidate   │
        │             │   confirm(被提名人)     │  nominated   │
        │             │ ─────────────────────▶ │  confirmed   │
        │             │                         └──────┬───────┘
        │             │   vote(Lv3, 每人一票)          │
        │             │ ◀─────────────────────────────┘
        │             │
        │   AutoElect │  票数≥阈值 → 候选人即时 elected
        │   =true 时  │
        └──────┬──────┘
               │  close(Lv5) 或 到期自动关闭
               ▼
        ┌─────────────┐
        │   closed    │  结算：票数≥阈值 → elected；否则 rejected
        └─────────────┘
```

- **创建**：Lv5 发起，`Status=active`，`EndTime=now+duration`（默认 7 天）。
- **提名**：任何已认证用户可提名（自荐或他荐）。自荐自动 `Confirmed`；他荐需被提名人单独确认。
- **确认**：被提名人本人确认接受（`publicKey == userID` 校验）。
- **投票**：Lv3+ 用户，每场选举每人一票（`HasVoted` 去重），只能投给已确认的候选人。
- **即时当选**（`AutoElect=true`）：投票后若 `VoteCount ≥ VoteThreshold`，候选人立即 `elected`。
- **关闭结算**：手动（Lv5）或到期自动。遍历候选人：`VoteCount ≥ threshold → elected`，否则 `rejected`；选举 `Status=closed`。

## 5. 角色与权限

路由权限定义于 `internal/api/router/router.go`（`registerAuthRoutes`）。

| 操作 | 端点 | 权限 |
|------|------|------|
| 列出选举 | `GET /api/v1/elections?status=` | 公开 |
| 选举详情 | `GET /api/v1/elections/{id}` | 公开 |
| 创建选举 | `POST /api/v1/elections/create` | `rbac.PermAdmin`（Lv5） |
| 提名候选人 | `POST /api/v1/elections/{id}/candidates` | 已认证（任意等级） |
| 确认接受提名 | `POST /api/v1/elections/{id}/candidates/{user_id}/confirm` | 已认证 + handler 内自校验（仅被提名人本人） |
| 投票 | `POST /api/v1/elections/{id}/vote` | `RequireLevel(Lv3)` |
| 关闭选举 | `POST /api/v1/elections/{id}/close` | `rbac.PermAdmin`（Lv5） |

注：投票需 Lv3 与需求文档的累积权限图一致（Lv0 读、Lv1 写/评分、Lv2 提分类、Lv3 维护类操作含投票）。

## 6. API 契约

请求/响应结构定义于 `election_handler.go`。

- **POST `/api/v1/elections/create`** — body: `{title, description, vote_threshold, duration_days, auto_elect}` → `{election_id, auto_elect}`。
- **GET `/api/v1/elections?status=active|closed`** — `{elections: [...]}`。
- **GET `/api/v1/elections/{id}`** — `{election, candidates: [...]}`。
- **POST `/api/v1/elections/{id}/candidates`** — body: `{user_id, user_name, self_nominated}`（自荐时 `user_id` 以请求方公钥覆盖）→ `{success, self_nominated, confirmed}`。
- **POST `/api/v1/elections/{id}/candidates/{user_id}/confirm`** — 仅被提名人本人 → `{confirmed: true}`。
- **POST `/api/v1/elections/{id}/vote`** — body: `{candidate_id}` → `VoteResult{success, vote_count, auto_elected?, message?}`。
- **POST `/api/v1/elections/{id}/close`** — `{elected: [...]}`。

所有写端点需 Ed25519 请求签名认证（`AuthMiddleware`）；错误以 `APIResponse{code, message}` 返回，业务错误多为 400、不存在 404、权限 403。

## 7. 存储布局

KV 前缀（`internal/storage/kv/election_store.go`、`vote_store.go`），全部为 O(1) 直查或前缀扫描：

| 键 | 值 | 用途 |
|----|----|------|
| `election:{id}` | Election JSON | 选举主记录（Get/Create/Update 直查） |
| `election:` (前缀扫描) | — | `List(status)` 列出全部 |
| `candidate:{electionID}:{userID}` | Candidate JSON | 候选人主记录 |
| `candidate:{electionID}:` (前缀扫描) | — | `ListByElection` |
| `vote:{id}` | Vote JSON | 投票主记录 |
| `votes:voter:{electionID}:{voterID}` | vote id | **投票人去重索引**（`HasVoted` / `GetByVoterAndElection` O(1)） |

## 8. 后台自动关闭

`ElectionAutoCloser`（`internal/core/election/auto_closer.go`）照搬 `LevelUpgradeChecker` 的后台任务范式：

- 结构：`{svc, interval, cancel, wg}`，`interval<=0` 默认 5 分钟。
- `Start(ctx)`：派生 cancelable ctx，`wg.Add(1)`，`go loop(ctx)`。`loop` 启动时**立即跑一次** `closeExpired`，随后 `time.NewTicker(interval)` 周期执行。
- `closeExpired`：`ListElections(active)` → 对每个 `IsExpired()` 的选举调用 `CloseElection` 结算；失败仅记日志不中断。
- `Stop()`：`cancel()` + `wg.Wait()`。
- 接线（种子节点 `cmd/seed/main.go`）：`Start()` 内 `NewElectionAutoCloser(electionSvc, 5min).Start(ctx)`；`Stop()` 内 `electionCloser.Stop()`。

> **注**：`cmd/user/main.go` 当前**未**注册任何后台 checker（无 levelChecker、无 electionCloser）。选举自动关闭仅在种子节点生效——与 §2 "选举状态仅存于操作节点"一致。

## 9. 并发与完整性

- **原子计票**（P2.3）：`KVCandidateStore.UpdateVoteCount` 通过 per-election mutex（`sync.Map[electionID]→*sync.Mutex`，`lockFor()`）串行化，保证并发投票下 `VoteCount` 正确自增；`HasVoted` 的存储错误正确传播（不再被当"未投"吞掉）。
- **幂等**：`HasVoted` 去重保证每场选举每人一票；重复提名返回 `ErrAlreadyNominated`；重复确认返回错误。
- **状态守卫**：提名/确认/投票均校验 `election.Status == active`，关闭后操作返 `ErrElectionClosed`。

## 10. 已知限制与未来工作

- **Vote ID 非 UUID v4**：`generateVoteID()` 仍用 `fmt.Sprintf("vote_%d", time.Now().UnixNano())`（`election.go:234`），与 Election/Candidate 已迁移到 `crypto.GenerateUUID` 不一致。高并发下存在碰撞风险，应统一改用 `crypto.GenerateUUID`。
- **无 P2P 同步**：选举数据不随 AWSP 同步，跨节点不可见。若需分布式选举共识，需为选举/候选人/投票增加 sync 消息类型与 LWW/合并策略。
- **当选不联动权限**：`CandidateStatusElected` 不自动改写 `user_level` 或分类维护权；需上层裁决。
- **无防篡改审计**：选举记录无 content_hash/签名，不进完整性守护进程（P3.2）覆盖范围。
- **ListByElection 已有前缀索引**（`candidate:{eid}:`）为 O(候选人数)，无需额外优化；但 `election:` 全前缀扫描 + 客户端 status 过滤随选举数增长会变慢（属 P2.5 可扩展性范畴）。
