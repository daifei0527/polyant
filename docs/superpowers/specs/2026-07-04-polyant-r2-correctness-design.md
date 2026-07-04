# R2 正确性加固设计

> 日期：2026-07-04。基线：master `877ce1a`（R1 安全加固已合并）。工作分支：`r2-correctness`。
> 关联：spec `docs/superpowers/specs/2026-07-03-polyant-r1-security-hardening-design.md`（R1，已完成）。

## 1. 目标

闭合 Polyant v2.3.0 审计的 Tier-2 正确性缺陷：并发数据竞争、版本/时间戳语义错乱、镜像同步失效、搜索索引与存储不一致、评分/选举 lost-update、资源泄漏、静默吞错。修复后数据完整性、P2P 冲突仲裁、搜索一致性在生产后端（Badger/Pebble + bleve）下成立，且不破坏历史数据。

## 2. 范围

### 2.1 纳入（12 项，7 高 + 4 中 + 1 低）

5 个并行只读子代理于 2026-07-04 在 master `877ce1a` 上逐项核实，全部仍成立。证据见各节。

| 分组 | 编号 | 缺陷 | 严重度 |
|---|---|---|---|
| R2-A sync 层 | A1 | versionVec 并发竞争（runtime fatal panic） | 高 |
| R2-A sync 层 | A2 | goroutine 泄漏（残留 3 处） | 中 |
| R2-A sync 层 | A3 | 镜像同步接收端未实现（MirrorData 被丢弃） | 高 |
| R2-B 存储层 | B1 | Update 双重 Version++（跳 +2） | 高 |
| R2-B 存储层 | B2 | 时间戳单位错配（秒覆盖毫秒）+ 历史迁移 | 高 |
| R2-B 存储层 | B3 | Scan 吞掉读取错误 | 中 |
| R2-C 搜索索引 | C1 | bleve 重启不核对（索引↔store 漂移） | 高 |
| R2-C 搜索索引 | C2 | 索引写错误被静默吞掉（`_ =`） | 高 |
| R2-C 搜索索引 | C3 | bleve Search 不过滤 status=published | 中 |
| R2-D 评分/选举 | D1 | 双重投票竞态（非原子 upsert + 双计票） | 高 |
| R2-D 评分/选举 | D2 | 候选人 UpdateStatus lost-update | 中 |
| R2-E 网络死代码 | E1 | DHT seed 未 dial（误导性死代码） | 低 |

### 2.2 排除（留 R4 专项清理）

- `BadgerSearchEngine`（备用实现）重启后搜索全空（`badger_adapter.go:296-327`，工厂路径不填充）——低危，R4。
- `RatingCalculator`（`internal/core/rating/calculator.go`）未挂路由的死代码及其死字段 `mu`——低危，R4。R2 仅修线上评分路径（`UserHandler`）。
- `internal/core/seed/initializer.go` 无法兜底修复 bleve 漂移——被 C1 启动 rebuild 覆盖，不单独处理。

### 2.3 非目标

- 不做性能优化（除非是正确性修复的副产品）。
- 不改 P2P 协议线格式（protobuf message 结构不变，仅 `processMessage` 增加已有 `MessageTypeMirrorData` 的路由分支）。
- 不加镜像 ack/重试/反压（YAGNI，先实现基本落库）。
- 不做增量索引核对（YAGNI，启动全量 rebuild 最稳；数据量评估见 C1）。

## 3. 设计原则与跨组约束

1. **存储层忠实持久化调用方语义**：`kv` 层不再自作主张改 `Version`/`UpdatedAt`，由调用方（handler/sync）显式赋值，存储层忠实写入。这是 B1/B2 的统一原则，也消除"Memory 后端掩盖 Badger 路径 bug"的测试盲区。
2. **并发修复优先用封装类型 / 复用现有锁原语**：`versionVec` 封装为自带锁类型；评分/选举复用 `lockFor(key)` per-key 互斥模式（选举模块已有）。
3. **索引与存储最终一致**：启动 rebuild 兜底 + 写错误不再静默，双管齐下保证索引↔store 一致。
4. **历史数据迁移幂等**：时间戳迁移只动可疑值（< 1e12），重复运行不变。
5. **每 task TDD，先写失败测试再实现**；并发项必须 `-race` 通过；B1 必须补 Badger 后端测试（现仅 Memory 路径有测试）。
6. **纯 bugfix，默认无配置开关**；行为变化（镜像接收端"启用"）视为修 bug，默认开。
7. **每组独立提交、独立测试周期、可独立回滚**（与 R1 一致）。

## 4. 分组设计

### R2-A：P2P sync 层并发与正确性

#### A1. versionVec 并发竞争（高）

**证据**：`internal/network/sync/sync.go` 的 `VersionVector`（`map[string]int64`）在多 goroutine 间访问，R1 给部分热点加了 `se.mu` 锁但覆盖不一致。`IncrementalSync`（`:192-201`）对每 peer 起 goroutine，`processSyncResponse` 在并发 goroutine 内被调用，以下点位裸奔：`:224`（`ToProto` 无锁读+遍历）、`:258/264`、`:279/285`、`:382/387`（`MergeEntries`）。更糟的是 `HandleSyncRequest`（`:394` RLock）与并发 `processSyncResponse`（无锁写）同时进行——RLock 挡不住无锁写。Go runtime 会 `fatal error: concurrent map read and map write`，进程崩。

**方案**：把 `VersionVector` 从裸 map 重构为**自带 `sync.RWMutex` 的封装类型**，所有方法（`Get`/`Set`/`ToProto`/`Merge`/`Range`/`Clone`）内部加锁，遍历在持锁期间完成。`SyncEngine.versionVec` 改为该类型。这样新增访问点无法绕过锁（编译期保证），彻底消除"漏加锁"。

**接口**（概念）：
```go
type versionVector struct {
    mu sync.RWMutex
    m  map[string]int64
}
func (v *versionVector) Get(id string) int64        // RLock
func (v *versionVector) Set(id string, ver int64)   // Lock
func (v *versionVector) Merge(other map[string]int64) // Lock，取 max
func (v *versionVector) Clone() map[string]int64    // RLock + 深拷贝
func (v *versionVector) ToProto() *protocolpkg.VersionVector // RLock + 深拷贝
```
现有的 `se.mu` RLock 包裹 `versionVec.ToProto()` 的代码可简化（封装类型自保护），但保留 `se.mu` 保护其他字段无妨。

**测试**：并发 `IncrementalSync`（多 peer 同时）+ `HandleSyncRequest` + `MergeEntries` 交错，`go test -race` 无 fatal panic、无 race 报告。

#### A2. goroutine 泄漏（中）

**证据**：周期 sync loop 与 push worker 在 R1 期间已加 `ctx+cancel+wg`（已修复）。残留 3 处 fire-and-forget：
1. `sync.go:494` `HandleMirrorRequest` 生产者 goroutine——未注册 `se.wg`，无 `ctx.Done()` select，channel 缓冲 10 而条目上限 10000，消费者弃读则永久阻塞在 `dataCh <-`。
2. `protocol.go:124-137` 镜像消费者 goroutine——无 wg、无 ctx 取消，`NewStream`/`WriteMessage` 阻塞无超时兜底。
3. `remote_query.go:95` `go s.cacheRemoteEntries(context.Background(), ...)`——脱离生命周期，未注册 wg，与 `Close()` 并发时 use-after-close / race。**最危险**。

**方案**：所有长期/异步 goroutine 统一注册到所属服务的 `wg` 并传入可被 `Stop()` 取消的服务级 ctx：
- `HandleMirrorRequest` 生产者：`se.wg.Add(1)` + `select { case dataCh <- x: case <-ctx.Done(): }`。
- protocol 镜像消费者：注册 `wg` + `ctx` 超时 select 兜底（`NewStream` 用带超时的 ctx）。
- `cacheRemoteEntries`：去掉 `context.Background()`，继承服务级 ctx + 注册 `wg`。

**测试**：`Start()` 后触发各路径，`Stop()` + 短等待后 `runtime.NumGoroutine()` 回落到基线；异常断连（消费者提前退出）下生产者不挂起。

#### A3. 镜像同步接收端（高）

**证据**：发送端 `HandleMirrorRequest`（`sync.go:491-558`）产出 `MirrorData`，协议层（`protocol.go:118-138`）把它推回请求方；但接收端 `processMessage`（`protocol.go:84-172`）的 switch **没有 `case MessageTypeMirrorData`**，走 `default` 报 `unknown message type` 后 stream 关闭、数据丢弃，从不落库。`Handler` 接口（`protocol.go:15-24`）只有 `HandleMirrorRequest`，无 `HandleMirrorData`。镜像同步（`CapabilityMirror`，种子节点核心能力）整条链路形同虚设，且无错误日志。

**方案**：
1. `processMessage` 增加 `case MessageTypeMirrorData: return p.handler.HandleMirrorData(ctx, data.(*protocolpkg.MirrorData))`。
2. `Handler` 接口增加 `HandleMirrorData(ctx context.Context, data *protocolpkg.MirrorData) (*Message, error)`。
3. `SyncEngine.HandleMirrorData`：反序列化 `data.Entries` → 对每个 entry 走**现有** `resolveConflictAndMerge`（复用 versionVec + last-write-wins 仲裁）落库 + 建索引。返回 `MirrorAck`（含本批接收计数）。
4.（可选）补一个客户端编排方法 `SendMirrorRequest(peer)` 便于测试与未来 CLI。

**测试**：扩展 `mocknet_e2e_test.go`：节点 A（镜像源）↔ 节点 B（镜像方），B 发 MirrorRequest → A 推 MirrorData → B 的 `HandleMirrorData` 落库，B 的 store 出现镜像条目；篡改某 entry 的 signature 走 R1 验签路径（若 `RequireEntrySignatures=true` 则拒）。

### R2-B：存储层语义统一

#### B1. 双重 Version++（高）

**证据**：update 路径 `UpdateEntryHandler`（`entry_handler.go:409` `existing.Version++`）→ `BadgerEntryStore.Update` → `kv.EntryStore.UpdateEntry`（`entry_store.go:154` 又 `entry.Version++`）→ 净 +2。批量更新 `batch_handler.go:481` 同病。sync 接收端 `sync.go:586/599` 先 `max(remote,local)` 再调 `Update`，kv 层的二次 ++ 会把 `max` 再顶高一格，导致同步版本漂移。`MemoryEntryStore.Update`（`memory.go:66-77`）不自增 → 测试用内存后端掩盖了 Badger 生产路径的双重 ++。

**方案**（落实"存储层忠实持久化调用方语义"）：**删除 `kv/entry_store.go:154` 的 `entry.Version++`**。Version 由调用方负责：handler/batch 自增一次（+1），sync 显式设 `max(remote,local)`。`MemoryEntryStore.Update` 已是该语义（存入参），不动。`ContentHash` 重算（`entry_store.go:155`）保留（幂等，存储层确保 hash 正确无害）。

**接口变更**：`kv.EntryStore.UpdateEntry` 不再副作用自增 Version，纯写入。这是 bugfix，所有调用方语义不变（handler 已自增、sync 已设 max）。

**测试**：**补 Badger 后端测试**（现仅 Memory 路径）——`BadgerEntryStore.Update` 一次调用 Version 恰好 +1；sync 端设 max 后 Update 落库 Version == max。并并发 update 无 race。

#### B2. 时间戳单位错配 + 历史迁移（高）

**证据**：`kv/entry_store.go:153` `entry.UpdatedAt = time.Now().Unix()`（秒）覆盖了 handler（`entry_handler.go:410` `NowMillis()` 毫秒）和 batch（`batch_handler.go:482`）设的毫秒值。`NewKnowledgeEntry`（`models.go:75`）也用 `Unix()`（潜伏隐患，生产 Create 未调用但需修）。净效果：Create 后 CreatedAt/UpdatedAt 均毫秒（~1.7e12），Update 后 UpdatedAt 变秒（~1.7e9）。sync 的 LWW 仲裁 `if remoteEntry.UpdatedAt > localEntry.UpdatedAt`（`sync.go:594`）在"毫秒 vs 秒"间比较，毫秒永远 >> 秒，仲裁失效。

**方案**：
1. **删除 `kv/entry_store.go:153` 的 `UpdatedAt` 覆盖**（B1 同一代码块）。UpdatedAt 由调用方设毫秒（handler/batch 已是 `NowMillis()`）。
2. `model.NewKnowledgeEntry`（`models.go:75,83-84`）`Unix()` → `UnixMilli()`。
3. **历史数据迁移**（幂等）：`NewPersistentStore` 启动时扫描所有 entries，对 `UpdatedAt`/`CreatedAt` 值 `< 1e12` 的视为秒 × 1000 → 毫秒。判定阈值 1e12：毫秒值 ~1.7e12、秒值 ~1.7e9，阈值居中无歧义。已毫秒的不动（幂等）。实现为 `migrateTimestampsToMillis(entryStore)`，启动调用一次。

**测试**：构造含秒值（`1.7e9`）与毫秒值（`1.7e12`）混合的 fixture，迁移后全部毫秒、再跑一次不变（幂等）；迁移后 LWW 仲裁在"旧秒值条目 vs 新毫秒条目"间按真实时间判定。

#### B3. Scan 吞掉读取错误（中）

**证据**：`BadgerStore.Scan`（`badger_store.go:71-79`）单键 `ValueCopy` 错误 `continue` 且 View 永远返回 nil；`PebbleStore.Scan`（`pebble_store.go:74-96`）单键 `ValueAndErr` 错误 `continue` + 循环后**从不检查 `iter.Error()`**。瞬时 IO 故障下条目/评分从 Scan 结果静默消失，破坏同步/列表/聚合一致性。`JSONFileStore`/`MemoryStore` 无 IO，不适用。

**方案**：
- Badger：单键 `ValueCopy` 错误 `return err`（传播给外层 View）。
- Pebble：单键错误 `return err` + 循环结束后 `if err := iter.Error(); err != nil { return nil, err }`。

**测试**：注入坏 value（mock 迭代器返回 err），`Scan` 返回非 nil error 且不返回残缺结果。

### R2-C：搜索索引一致性

C1/C2/C3 是索引↔store 不一致的三个表现，同组修复。

#### C1. bleve 重启不核对（高）

**证据**：`NewBleveEngine`（`bleve_engine.go:104-117`）reopen 时 `bleveIndexExists` 仅 `os.Stat`+`Open`+`Close`，不读 EntryStore、不比对；`NewPersistentStore`（`store.go:186-214`）启动重建 `titleIdx`/`backlinkIdx` 但**唯独 bleve 无重建**；全仓 0 reindex/rebuild 代码。索引文件损坏/丢失/陈旧时搜索静默失真。

**方案**：`NewPersistentStore` 打开 bleve 引擎后，执行一次**全量 rebuild**：遍历 EntryStore 已发布条目，清空 bleve + 全量 `IndexEntry`。暴露 `BleveEngine.Rebuild(entries []*model.KnowledgeEntry) error`，启动调用。Polyant 知识库条目量级可控（参考 seed 数据 + 用户贡献，估算 < 数万），启动全量 rebuild 秒级可接受。

> 备注：若实测数据量大导致启动慢，可降级为"启动 count 对账（bleve 文档数 vs EntryStore published 数），不一致才 rebuild"。本 spec 默认全量 rebuild（最稳），实现时按实测耗时定夺并在 plan 中记录。

**测试**：手动破坏 bleve 索引（删文件 / 删部分文档）后重启，搜得到全部 published 条目、搜不到已删条目。

#### C2. 索引写错误被静默吞掉（高）

**证据**：所有业务路径对 `IndexEntry/UpdateIndex/DeleteIndex` 返回值 `_ =` 丢弃：`entry_handler.go:310,436,514`、`batch_handler.go:407,521,596`、`sync.go:267,288`（连 `_ =` 都没有）、`seed/initializer.go:103`。唯一记录错误的是 `sync.go:304`。任何一次 bleve 写失败（磁盘满 / bolt 锁竞争 / 写中途被杀）永久制造漂移且无日志——日常漂移主因。

**方案**：所有调用点检查 err：`if err := h.searchEngine.IndexEntry(...); err != nil { log.Error("索引写失败", ...) }`。策略 **best-effort**：entry 写入成功为主（不阻塞、不回滚 entry），索引失败 `log.Error`。运行时漂移由 C1 启动 rebuild 兜底——**若 C1 采用默认全量 rebuild，则 log 即可（下次启动自愈）**；**若 C1 降级为对账模式（count 一致才跳过 rebuild），则需额外置进程内"索引脏"标志以强制下次 rebuild**。实现时与 C1 选定策略对齐，二选一，不重复。

**测试**：注入返回 err 的 mock 搜索引擎，验证：(a) 有 `log.Error`；(b) entry 操作仍成功返回；(c) "索引脏"标志被置。

#### C3. bleve Search 不过滤 status（中）

**证据**：`bleve_engine.go:190-308` 的 `Search` 索引了 `status` 字段（`:90,156`）但查询无 `status=published` 过滤，依赖"软删除必成功调 DeleteIndex"不变式。`MemorySearchEngine.Search`（`memory.go:335`）显式过滤。叠加 C2：软删除（`BadgerEntryStore.Delete` 置 `EntryStatusArchived`）时若 `DeleteIndex` 静默失败，archived 条目会被 bleve 搜出，两后端行为不一致。

**方案**：bleve `Search` 的 bool query 增加 `status=published` 的 must 条件。

**测试**：archived 条目搜不到；published 条目正常搜到。

### R2-D：评分/选举并发

#### D1. 双重投票竞态（高）

**证据**：线上路径 `UserHandler.RateEntryHandler`（`user_handler.go:435-474`）：`GetByRater`（READ 重复检查）→ `Create`（WRITE），中间无锁无事务，同用户并发可双双通过再双双 Create。`kv.RatingStore.CreateRating`（`rating_store.go:26-58`）自带一轮 check-then-write 也非原子。badger 后端 key=`PrefixRating+entryId:raterPubkey`，并发 Put 同 key 走 last-write-wins（不产重复记录，但两次请求都返 success 且 ScoreCount 重算双计）；Memory 后端以 `rating.ID`（UUID）为 key 会产真正重复记录。`RatingCalculator`（`calculator.go`）三层非原子且 `mu` 死字段，但属未挂路由死代码（R4），R2 仅修线上 handler。

**方案**：per-entry 互斥锁，临界区内完成"重复检查 + 写 rating + 重算 entry.Score/ScoreCount"。复用选举模块的 `lockFor` 模式：在 rating 路径引入 `lockForEntry(entryID) *sync.Mutex`（`sync.Map` 惰性 per-key 锁）。临界区：`GetByRater` 查重 → 已存在返 `ErrDuplicateRating`；否则 `Create` + 重算 Score/ScoreCount + `entryStore.Update`。

**测试**：同用户对同条目并发 N（如 50）次评分，最终恰 1 条 Rating、ScoreCount 恰好 +1、无 race；不同用户互不阻塞。

#### D2. 候选人 UpdateStatus lost-update（中）

**证据**：`KVCandidateStore.UpdateStatus`（`election_store.go:166-173`）`Get`→改 Status→`Add`（Put 全量覆写），无锁无 CAS。紧邻的 `UpdateVoteCount`（`:147-164`）已用 `lockFor(electionID)` 修复且有 `election_atomic_test.go` 回归，但 `UpdateStatus` 漏了且无测试。并发触发点：`Vote` 自动当选调 `UpdateStatus(Elected)`（`election.go:179`）与 `CloseElection` 批量置状态（`:219,223`）竞争，后者以过期快照把已达阈值候选人置 Rejected，覆写 Elected。

**方案**：`UpdateStatus` 复用 `lockFor(electionID)`（与 `UpdateVoteCount` 同把锁）。

**测试**：扩展 `election_atomic_test.go`——并发 `UpdateStatus` + `UpdateVoteCount` 不丢更新；`Vote` 自动当选与 `CloseElection` 并发，最终状态正确。

### R2-E：网络死代码清理

#### E1. DHT seed 未 dial（低）

**证据**：审计对 dht/host 包字面观察属实——`dht.go:48-77` `Bootstrap` 解析 `SeedNodes` 进本地 `peers` 切片但只用于日志（`:75`），从未传 `host.Connect`；`host.go:61-62` `SeedPeers` 字段被 `NewHost` 忽略（只消费 `RelayPeers`）。但**实际拨号在 app 层**：`cmd/user/main.go:386-406` 的 `ConnectToPeer` 循环、`cmd/seed/main.go:381-393` 同理。节点能连上、路由表不会空。属误导性死代码（运维看日志会误以为 DHT 已连 seed）。

**方案**：删除 `dht.go:52-69,75` 的 `peers` 解析 + 误导性 "bootstrap completed with N peers" 日志（app 层 dial 是唯一真相）；删除 `host.Config.SeedPeers` 未消费字段（或让 `NewHost` 消费它——推荐删字段，单一数据源）。

**测试**：连通性行为不变（mocknet e2e 仍通）；`grep` 确认死代码已清。

## 5. 测试策略

- 每 task TDD：先写失败测试（精确命中缺陷），再实现。
- 并发项（A1/A2/D1/D2）必须 `go test -race` 通过，且用并发用例（多 goroutine 交错）。
- **补 Badger/Pebble 后端测试**（B1/B3 关键）：现测试多用 Memory 后端，掩盖生产路径 bug。B1 的 Version 测试、B3 的 Scan 错误测试必须在真实后端上跑。
- 迁移测试（B2）：幂等性 + 混合 fixture。
- 索引测试（C1/C3）：破坏索引 / archived 条目场景。
- 复用既有测试基建：`mocknet_e2e_test.go`（A3）、`election_atomic_test.go`（D2）。

## 6. 向后兼容

- **时间戳迁移幂等**（B2）：只动 < 1e12 的值，重复运行不变。
- **Version/UpdatedAt 由调用方设**（B1/B2）：存储层语义更纯，所有现有调用方语义不变。
- **镜像接收端"启用"**（A3）：之前失效，修复后默认开。历史无镜像数据，无迁移问题。
- **无配置开关**：纯 bugfix。`RequireEntrySignatures`（R1 开关）仍适用于镜像接收的 entries。
- **协议不变**：`MessageTypeMirrorData` 常量与 `MirrorData` 结构早已定义（`types.go:16,125-131`），仅增加路由分支。

## 7. 验收

- `go build ./cmd/... ./internal/... ./pkg/...` 绿。
- `go vet ./...` 绿。
- `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` 全绿。
- 手动核对（由自动化测试覆盖）：
  - versionVec 并发不崩（A1）。
  - Badger 后端 update Version 恰好 +1（B1）。
  - 历史秒值条目迁移后 LWW 仲裁正确（B2）。
  - Scan 在 IO 错误下返 error（B3）。
  - 索引损坏重启后搜得到全部 published（C1）。
  - 镜像数据接收端落库（A3）。
  - 并发评分不双计（D1）。
- R2 合并到 master 后开 R3 cycle（坏功能修复）。

## 8. 风险

- **B2 迁移阈值误判**：< 1e12 判秒。理论上早期毫秒值（2001 年前）会被误判，但 Polyant 2026 年的数据不会出现。低风险。
- **C1 启动 rebuild 耗时**：若实测数据量大，启动慢。降级方案见 C1 备注。
- **A3 镜像落库风暴**：镜像源条目多时，接收端一次性大量 `resolveConflictAndMerge`。已有批量处理（`MirrorData` 分批），且走现有仲裁路径，风险可控。
- **D1 per-entry 锁内存**：`sync.Map` 惰性锁不清理会缓增。选举模块同模式已有，可接受；若担心可加 LRU（YAGNI，先不加）。

## 9. 自审清单（占位符/一致性/范围/歧义）

- 无 TBD/TODO；C1 的"全量 rebuild vs 对账"给了默认（全量）+ 降级路径，非占位符。
- 分组与 2.1 表一致；A-E 各 task 映射明确。
- B1/B2 共改 `entry_store.go:152-155` 同一代码块，分两个 task 但实现紧邻，plan 阶段会明确先后。
- 严重度、证据 file:line 均来自 2026-07-04 子代理核实（HEAD `877ce1a`）。
