# Polyant 全量扫荡迭代设计文档 (Full-Sweep Iteration)

| 字段 | 值 |
|------|-----|
| 日期 | 2026-06-13 |
| 版本 | v1.0 |
| 状态 | 待实现 |
| 关联 | 2026-06-13 全面审计（15-agent 工作流，结果存于会话工作流输出）；v2.2.0 |
| 执行方案 | A — 分阶段纵切 + 阶段内并行 |

---

## 1. 背景与动机

2026-06-13 对 Polyant v2.2.0 做了一次全面审计（需求文档对照 + 7 子系统实现审计 + 完整性批判 + 合成）。结论：**项目"已实现但有结构性漏洞"**——构建通过、vet 干净、单元测试全绿，但绿测试背后隐藏着安全漏洞与失效路径。

**关键洞察：markdown 是不可信的真相源。** 标注"待实现"的 spec 实际已实现；标注 0/49 完成的计划实际 100% 完成。**只有代码与 git log 可信。** 因此本迭代以代码核实为准，不以文档 checkbox 为准。

审计共发现 30+ 漏洞（4 严重、11 高危、9 中危、8 低危）与 8 个新功能机会。最严重的 5 个判断已逐一手动核实，全部属实（见 §3.1）。

**本里程碑目标：** 把 Polyant 从"已实现但有洞"推进到"稳固且已验证"，并补齐一个有真实安全价值的新功能。用户选择"全量 ultracode 扫荡"——修复所有严重/高危、完成所有半成品功能、再加新功能。

## 2. 范围（YAGNI 边界）

**包含：**
- Phase 1：11 项安全与正确性必修（§4）
- Phase 2：8 项半成品功能完成（§5）
- Phase 3：P2P mocknet 测试台 + 知识完整性守护进程（旗舰新功能）+ 测试覆盖回填与 CI 文档一致性（§6）

**不包含（明确排除）：**
- 不重写存储引擎（BadgerDB/Pebble/Bleve 并存是既成事实，本迭代仅在 KV 层补测试与聚合查询，不做引擎统一）
- 不做 JSON→Protobuf 全量存储迁移（仅收口死代码 codec，见 P2.8）
- 不引入新外部依赖（STUN 优先用 libp2p AutoNAT，否则诚实降级）
- 不做大范围无关重构

## 3. 执行方案

### 3.1 已核实的严重判断（实现依据）

| 判断 | 核实证据 |
|------|----------|
| 验证码泄露 | `internal/api/handler/user_handler.go:193` 无条件返回 `code`，注释自承"仅用于测试，生产应删除"，无 dev 门控 |
| 邮箱唯一性空 stub | `user_handler.go:161-165` 空 `if` 块 |
| RBAC 零强制 | `grep` 显示 `internal/auth/rbac` 零非测试引用 |
| EntryPusher 未接线 | `cmd/{seed,user}/main.go` 注入，但无 handler 读取 |
| 镜像同步拨错 peer | `internal/network/protocol/protocol.go:121` 把 `r.RequestID` 当 `peer.ID` 拨号 |

### 3.2 执行模型（方案 A）

- **顺序阶段**：Phase 1 → 2 → 3。依赖正确：基础 bug 先修（P1.7 哈希统一解锁 P3.2 完整性守护；P3.1 mocknet 解锁 P1.4/P1.5 的真实验证）。
- **阶段内并行**：每阶段内互相独立的项用工作流（workflow）并行子代理实现；有依赖的项串行。
- **验证门（每阶段末）**：`go build ./...` + `go vet ./...` + `go test ./...` 全绿方可进入下一阶段。
- **纪律**：
  - 每个 bug 修复前用 `systematic-debugging` 复现并定位根因（审计已核实根因，但实现时仍以复现为准）。
  - 每个修复配套测试（先写/改测试，TDD 优先）。
  - 每个原子修复独立提交。

## 4. Phase 1 — 安全与正确性（11 项，必修基础）

### P1.1 验证码泄露修复
- **根因**：`SendVerificationCodeHandler` 响应体含 `"code": code`（`user_handler.go:193`），无条件返回，绕过邮件。
- **修复**：移除响应中的 `code` 字段；新增配置项 `dev.return_verification_code`（默认 false），仅当为 true 时才在响应附 code，供测试使用。
- **测试**：默认配置下响应不含 code；dev=true 时含 code；端到端注册→验证→升级 Lv1 仍通过。
- **文件**：`internal/api/handler/user_handler.go`、`pkg/config`、相关测试。

### P1.2 邮箱唯一性
- **根因**：`user_handler.go:161-165` 邮箱查重 `if` 块为空。
- **修复**：在 `UserStore` 增加带前缀索引的 `GetByEmail`（避免全表扫描）；注册/发码前查重，重复返 `err-email-taken`（409/805）。
- **测试**：重复邮箱被拒；不同邮箱通过；并发注册同邮箱只成功一个。
- **文件**：`internal/storage/kv/user_store.go`、`user_handler.go`。

### P1.3 RBAC 强制执行
- **根因**：`internal/auth/rbac` 完整构建且测试通过，但零非测试代码引用；权限检查散落在各 handler 内为 ad-hoc 级别判断。
- **修复**：在 auth 中间件增加 `RequirePermission(perm)` 包装器，调用 `rbac.HasPermission(level, perm)` 作为唯一访问控制源；迁移散落的 per-handler 级别检查到权限矩阵调用。
- **测试**：表驱动矩阵，断言每个 (level × endpoint) → allow/deny 符合需求文档的累积权限图（Lv0 读、Lv1 写/评分、Lv2 提分类、Lv3 维护分类、Lv4 审计/管用户、Lv5 全权）。
- **文件**：`internal/api/middleware/auth.go`、`internal/auth/rbac`、各 handler、矩阵测试。

### P1.4 镜像同步 peer 拨号
- **根因**：`protocol.go:121` 用 `peer.ID(r.RequestID)` 拨号——`RequestID` 是同步关联 id，不是 peer id；新节点全量镜像同步失效。
- **修复**：把请求方的真实 `peer.ID` 从 stream/connection 上下文传入 `HandleMirrorRequest`，用它拨回数据流；`RequestID` 仅作关联。
- **测试**：P3.1 mocknet 端到端覆盖（handshake→mirror_request→mirror_data→ack）；单元测试断言拨号目标是请求方 peer。
- **文件**：`internal/network/protocol/protocol.go`、`internal/network/sync/`。

### P1.5 EntryPusher 接线
- **根因**：`Deps.EntryPusher` 在 seed/user main 注入但无 handler 读取；新建/更新条目不推送种子节点，破坏"推送到种子"半边流程。
- **修复**：在 entry Create/Update handler 中，本地持久化+索引后异步调用 `EntryPusher.Push(ctx, entry)` 推送到种子节点；失败仅记日志不阻塞主流程。
- **测试**：创建条目后断言 Push 被调用；P3.1 mocknet 验证种子节点收到。
- **文件**：`internal/api/handler/entry_handler.go`。

### P1.6 `/node/sync` 触发器
- **根因**：`TriggerSyncHandler` 只更新时间戳、返回 `{status:"syncing"}`，从不调用 `SyncEngine.IncrementalSync`。
- **修复**：向 NodeHandler 注入 sync 触发接口，异步调用 `IncrementalSync`，返回真实同步结果/状态。
- **测试**：调用后断言 IncrementalSync 被触发；返回结构含真实 peer 数/条目数。
- **文件**：`internal/api/handler/node_handler.go`、router 依赖注入。

### P1.7 content_hash / 签名方案统一
- **根因**：`ComputeContentHash = SHA256(title:content:version:jsonData)`（`models.go:93`），与文档定义的条目签名 `SHA256(title+"\n"+content+"\n"+category)` 不一致；同步/push 校验用哪个不一致，是静默损坏来源。
- **修复**：统一为文档契约 `SHA256(title + "\n" + content + "\n" + category)`；`IncrementalSync`/push 在写入前用同一哈希校验；旧条目提供一次性重算迁移。
- **测试**：创建→哈希→签名→push→对端校验全链路一致；迁移后哈希符合新契约。
- **文件**：`internal/storage/model/models.go`、`internal/network/sync/`、handler。

### P1.8 CORS 配置化
- **根因**：`DefaultCORSConfig` 用 `AllowedOrigins=["*"]` + `AllowCredentials=true`（浏览器拒绝该组合），且硬编码。
- **修复**：origins/credentials 从 `config.Config` 读取；默认 credentials=false 配通配符，或显式 origin 列表配 credentials=true。
- **测试**：默认配置合法；自定义 origin 生效。
- **文件**：`internal/api/router/router.go`、`pkg/config`。

### P1.9 AuthMiddleware.Close() 优雅停机
- **根因**：replay 保护清理 goroutine + `seenRequests` map 在服务器生命周期内泄漏（`auth.go:66,71`），仅测试调用 Close()。
- **修复**：在服务器/router 优雅停机流程中调用 `AuthMiddleware.Close()`，与组件有序停止并行。
- **测试**：停机后 goroutine 退出（goroutine 计数或 leaktest）。
- **文件**：`internal/api/middleware/auth.go`、`cmd/{seed,user}/main.go` 停机流程。

### P1.10 pactl hex 导出修复
- **根因**：`key.go:80,90,125` 对 base64 编码的公钥直接用 `hex.DecodeString`，hex/default 格式输出错误/为空。
- **修复**：先 `base64.StdEncoding.DecodeString` 再编码为 hex。
- **测试**：base64 输入 → 正确 hex 输出；往返一致。
- **文件**：`cmd/pactl/.../key.go`。

### P1.11 UUID v4 ID
- **根因**：`models.go:299` `generateID()` 返回 `time.Now().UnixNano()` 字符串，非 UUID v4（P0 要求 `entry-id-uuid-v4`），跨协程/跨节点有碰撞风险。
- **修复**：storage/kv 条目创建路径改用 `pkg/crypto/hash.go` 已有的 `GenerateUUID`（正确 UUID v4）；旧时间戳式 ID 一次性回填/别名兼容。
- **测试**：新 ID 为 UUID v4 格式；并发生成无碰撞；旧 ID 仍可读。
- **文件**：`internal/storage/model/models.go`、`internal/storage/kv/entry_store.go`。

## 5. Phase 2 — 完成半成品功能（8 项）

### P2.1 i18n 管线
- **现状**：`TitleI18n/ContentI18n/NameI18n` 在 models 定义且有 `GetXByLang` 访问器，但零消费者。
- **修复**：BleveEngine 索引本地化字段；`/search` 与 `/entry/{id}` 增 `lang` 查询参数，经访问器选本地化字段；polysdk 与 MCP `polyant_search` 暴露 lang。
- **测试**：多语言条目按 lang 返回正确字段；搜索命中本地化内容。

### P2.2 内存态持久化 + 选举自动关闭
- **现状**：email VerificationManager、admin SessionManager、BacklinkIndex（持久模式）、选举截止——重启即丢；选举从不自动关闭。
- **修复**：验证码/session 落 KV（已有 KV 后端接口）；BacklinkIndex 用持久存储 backing；增选举截止自动关闭后台任务。
- **测试**：重启后 session/验证码/backlink 仍在；选举到期自动关闭并结算。

### P2.3 投票计数原子化
- **现状**：`KVCandidateStore.UpdateVoteCount`、`ElectionStore.Update` 为 Get-then-Add 非原子，并发投票有竞态；`HasVoted` 把存储错误当"未投"。
- **修复**：用 Batch/事务写 + 版本比较（或每选举 mutex）做票数自增；`HasVoted` 区分存储错误与未投。
- **测试**：并发投票计数正确；存储错误正确传播。

### P2.4 NAT 检测
- **现状**：`host.detectNATType` 硬编码 Symmetric；`detect.detectNATType` 返 Unknown；`testPortReachability` 自拨本端口（假阳性）。
- **修复**：接入 libp2p AutoNAT 结果，或 STUN 探测；移除自拨端口测试；诚实降级标注。
- **测试**：mock AutoNAT 返回 → 正确分类；无 STUN 时降级为 Unknown 而非假 Symmetric。

### P2.5 可扩展性：存储聚合 + 游标分页
- **现状**：stats/admin/export/rating 用 `Limit:100000` 全表扫描 + 客户端聚合/排序/分页；`AuditStore.Get`、`BadgerRatingStore.Get` O(n)。
- **修复**：存储层增聚合/计数查询与游标分页；为 `user_level`、`rating_by_rater` 等加前缀索引键（design.md 已规定但未实现）；`GetByEmail`/`GetRating` 直查索引。
- **测试**：大数据集下查询耗时与结果正确；分页边界。

### P2.6 RatingStore 命名/持久化
- **现状**：`UpdateEntryScore` 计算加权均分并返回但不持久化到条目 Score（`rating_store.go:111`），易误用、分数静默过期。
- **修复**：同事务持久化 Score，或重命名为 `ComputeEntryScore`；核实所有调用方持久化。
- **测试**：评分后条目 Score 更新且持久。

### P2.7 isLocalRequest 配置化
- **现状**：`session.go:89-90` 比对固定端口串 18531，换端口或反代后失效；RemoteAddr 回退可伪造。
- **修复**：期望本地端口从 config 取；用连接级本地地址校验（`r.Context` 网络值）而非 Host 头；文档标注"不支持反代"约束。
- **测试**：默认端口与自定义端口均正确判定；Host 伪造无效。

### P2.8 低悬果（批量）
- `ioutil` → `os`（auth/ed25519/keys.go、config.go）
- `config.Save()` 路径用 `filepath.Dir/Split`（跨平台，`config.go:520`）
- Windows `processExists` 改非破坏探测（`OpenProcess + PROCESS_QUERY_LIMITED_INFORMATION`）
- env 前缀文档对齐（design.md `AW_` → `POLYANT_` 或加别名；在 SKILL.md 记录）
- 移除死代码 `SimpleSearchEngine`（或改造为内存引擎并修 Search 签名）
- `LevelUpgradeChecker.checkUpgrade` 未升级时返回当前等级（非 0）+ 透传 context（`level_checker.go:165`）
- protobuf/JSON codec 收口：正式弃用/移除死 JSON codec 或文档确认 JSON 存储为有意
- 补 **选举功能设计 spec**（`docs/superpowers/specs/2026-06-13-election-design.md`）

## 6. Phase 3 — 新功能 + 测试台（3 项）

### P3.1 P2P mocknet 集成测试台（必需基建）
- **目的**：同时关闭 P1.4（镜像拨号）、P1.5（EntryPusher）、P2.3 等同步类漏洞，并补齐 P2P 收发路径零验证。
- **设计**：用 `libp2p mocknet` 起 seed↔user↔user 三节点，复用现有 `mock_host`/`protocol` 测试件，组合 SyncEngine+Protocol，断言 `handshake → 增量同步 → push-entry → LWW 合并` 往返；CI 中作为"P2P 可用"门。
- **测试**：即本测试台本身。

### P3.2 知识完整性守护进程（旗舰新功能）
- **动机**：`rel-data-integrity-sha256` 与 `risk-consistency-check-15min` 要求周期性完整性校验；P1.7 哈希修复后此功能才有意义。安全价值真实。
- **架构**：
  - 后台周期任务（接入 `cmd/{seed,user}/main.go` 现有后台调度器，与 level-checker/rating-recompute 并列）。
  - 周期（默认 15min，可配）遍历条目，重算 `ComputeContentHash`，与存储的 `content_hash` 比对；校验 `creator_signature`。
  - 不一致 → 写 `audit.Service` 记录（篡改/损坏事件），可选标记条目 `status=archived`。
- **数据流**：调度器触发 → 守护进程读条目批次 → 重算哈希/验签 → 比对 → 异常入 audit。
- **错误处理**：单条异常不中断批次；批次大小与周期可配；自身 panic 不杀进程（recover）。
- **配置**：`integrity.enabled`、`integrity.interval_seconds`、`integrity.batch_size`、`integrity.archive_on_tamper`。
- **测试**：篡改条目内容 → 守护进程检出并入 audit；正常条目无误报；批次/周期可配。

### P3.3 测试覆盖回填 + CI 文档一致性
- **测试回填**：KV 层 9 文件直接 `_test.go`（CRUD/前缀扫描/边界/哈希）；polysdk `setAuthHeaders` 签名往返 + URL 用 `net/url.Values` 转义修复；entry CRUD 表驱动测试；seed 节点 default 数据集 + fake SMTP 的 register→verify→Lv1 e2e。
- **CI 一致性**：加检查——spec 的 checkbox/`状态:` 与代码矛盾时失败（如步骤未勾但符号已实现）；扫除陈旧文档（标 2026-06-01-unified-agent-skills、2026-05-29-api-key-authentication 为已完成；audit-log spec 状态改为已实现）。

## 7. 依赖与排序

```
P1.7(哈希统一) ──► P3.2(完整性守护)
P3.1(mocknet)  ──► 真实验证 P1.4 / P1.5 / P2.x(sync)
P1.3(RBAC)     ──► 简化 P2.x 中权限相关 handler
P1.2(GetByEmail索引) ──► P2.5(聚合索引复用前缀方案)
其余 P1/P2 项相互独立，阶段内可并行
```

Phase 1 内建议先做 P1.1（安全）→ P1.7（解锁 P3.2）→ 其余并行；P3.1 在 Phase 1 末或 Phase 3 初落地，用以验证 P1.4/P1.5。

## 8. 测试策略

- **单元**：每个修复配套 `_test.go`；KV 层、polysdk、entry CRUD 重点补齐。
- **集成**：修复 `test/integration_test.go`（v2.2.0 客户端生成密钥适配）——`Register()` 改为客户端本地生成密钥对、注册时提交公钥。
- **P2P**：P3.1 mocknet 三节点往返。
- **e2e**：seed 节点 SMTP 验证邮件全链路。
- **覆盖率**：目标 >80%（需求 `roadmap-phase5-test-release`）；阶段末 `go test -cover` 报告。

## 9. 成功标准

- [ ] `go build ./...` + `go vet ./...` + `go test ./...`（含修复后的 `test/` 包）全绿。
- [ ] Phase 1 全部 11 项修复 + 测试通过；验证码不再默认泄露；RBAC 矩阵测试通过。
- [ ] Phase 2 全部 8 项完成；选举 spec 补齐。
- [ ] P3.1 mocknet 测试台通过；P3.2 完整性守护进程检出篡改并入 audit。
- [ ] 测试覆盖率 >80%；CI 文档一致性检查就位、陈旧文档已扫除。
- [ ] 每项修复独立提交；CHANGELOG 更新；版本号 bump。

## 10. 风险与缓解

| 风险 | 缓解 |
|------|------|
| P1.7 哈希契约变更破坏旧数据 | 提供一次性迁移 + 旧哈希别名；灰度（先记录不一致不拦截） |
| P1.3 RBAC 接入改变现有可见行为 | 矩阵测试先固化"现状"，再收紧到需求契约；分批迁移 |
| mocknet 测试在 CI 不稳定 | 超时 + 重试 + 标记 P2P 测试为可隔离 |
| 范围过大导致半途而废 | 顺序阶段 + 每阶段验证门；每阶段独立可发布 |

## 11. 开放问题

- P1.1 dev 门控：用配置项还是编译标签（build tag）？→ 倾向配置项 `dev.return_verification_code`，便于测试环境启用。
- P2.4 NAT：是否引入第三方 STUN 库？→ 优先 libp2p AutoNAT，避免新依赖。
- P3.2 篡改处置：仅 audit 还是同时归档？→ 默认仅 audit + 可配归档。

---

**下一步：** 本 spec 经用户复核后，进入 `writing-plans` 生成实现计划（建议按 Phase 拆分多个计划），再以 `subagent-driven-development` / `executing-plans` 执行。
