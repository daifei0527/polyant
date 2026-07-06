# R4a Linter 清理设计

**范围**：4 轮"分阶段全扫荡"迭代的第四轮（R4）的第一个迷你轮——Linter 清理。R1（安全）/R2（正确性）/R3（坏功能）已合并入 master。R4 经评估横跨 5 个基本独立的领域（内容审核 / KV 备份恢复 / 选举&导出导入 UI / MCP 扩展 / linter 清理），过大且异质，不适合作单一 spec，故拆成多个迷你轮，每个各自走 design→plan→implement→verify，做完一个再开下一个。本迷你轮（**R4a**）只做 linter 清理。

**轮次定位**：R4a 是"清欠债 + 上门控"——清理 R1-E2 暂缓启用的 `gosec`/`staticcheck`/`unused`/`errcheck` 四个 linter 的既有发现，使其可在 `.golangci.yml` 与 CI 中硬启，作为后续 R4 功能开发的质量基线。**不建新功能**；任何"看起来像功能"的改动（如发现真实并发 bug）只做最小根因修复，功能扩展留给后续迷你轮。

## 目标

清理 4 个 linter 当前全部发现，使其全绿并硬启：

- `unused`（19）、`staticcheck`（15）、`gosec`（13）、`errcheck`（50）——合计 **97 处独立发现**（golangci 原始输出 98 行，含 1 条 SA5011 关联信息行；R1-E2 记录为 134，R2/R3 实现已自然减少）。
- 能修则修根因；仅对"设计上不可修"的命中用 `//nolint:<linter>` 压制，每处必须具名 linter + 文字理由。
- `.golangci.yml` 启用上述 4 个 linter；CI lint job 随之强制执行。
- `go build` / `go vet` / `go test -race ./...` 无回归。
- 两处生产 `mu` 字段（`Protocol.mu`、`RatingCalculator.mu`）做根因调查并记录结论（删 vs 加锁）。

## 非目标（后续迷你轮或主动不做）

- **libp2p relay API 迁移**（`EnableAutoRelay` → `WithStaticRelays`/`WithPeerSource`）：行为变更，本轮 `nolint` + TODO 跟踪，留给独立轮。
- R4 其他领域：内容审核流程、KV backup/restore + GC、选举/导出导入 UI、MCP 工具扩展——各自独立迷你轮。
- `goconst`/`gocyclo`/`funlen` 等其他 linter 的引入（不在 R1-E2 暂缓清单内）。

## 原则：nolint 策略

1. **能修则修**：linter 指向真实缺陷（nil ctx、字符串 ctx key、整型溢出、静默丢错误、空分支）一律根因修复。
2. **nolint 仅用于设计上不可修**：占位常量（`PlaceholderApiKey`）、HTTP 头名（非凭据）、配置驱动的可选不安全行为（`InsecureSkipVerify` 受配置关闭）、CLI 一次性忽略（确认提示 `Scanln`、尽力而为的日志抓取 `exec.Run`）。
3. **每处 nolint 必须具名 + 理由**：`//nolint:gosec // 占位常量，非真实凭据`。禁止裸 `//nolint`。
4. **被 nolint 的项建立跟踪**：libp2p relay 迁移记 TODO，后续轮复审。

## 技术栈

Go 1.25.x / golangci-lint v1.64.x（`unused` `staticcheck` `gosec` `errcheck`）/ 标准测试 + `-race` / GitHub Actions（`golangci-lint-action`）。

---

## 架构：5 个 task 组

每组独立可提交（独立 commit + 验证）。前缀：`chore(lint)` 为主，真实 bug 修复用 `fix(...)`，删死代码用 `refactor(...)`。

- **R4a-A 删死代码 + 解决 2 个生产 mu 字段**（`unused`，19）
- **R4a-B staticcheck 根因修复**（`staticcheck`，15）
- **R4a-C gosec 逐处处置**（`gosec`，13）
- **R4a-D errcheck 逐处处置**（`errcheck`，50）
- **R4a-E 启用 4 个 linter + CI 门控**（配置/CI）

---

## R4a-A：删死代码 + 解决生产 mu 字段（unused，19）

### A1 删除测试死代码（17 处）

| 位置 | 内容 | 处置 |
|---|---|---|
| `internal/network/protocol/codec_test.go:450-469` | `mockStream` 类型 + 15 个方法（R2.8 删 JSON codec 后遗留的 mock，无引用） | 整块删除 |
| `internal/api/router/router_test.go:384-388` | `mockEntryPusher` 类型 + `PushEntry` 方法（无引用） | 删除 |
| `internal/api/handler/export_handler_test.go:179` | `createMultipartBody` 辅助函数（无引用） | 删除 |

**验证**：删后 `go test ./...` 仍绿（确认确无引用）。

### A2 解决 2 个生产 `mu sync.RWMutex` 字段（根因调查，非盲删）

R2 并发审计曾标记此类"字段存在但从未上锁"为疑似"忘记加锁"。两处必须逐个核实：

**A2.1 `Protocol.mu`（`internal/network/protocol/protocol.go:32`）**

- 结构体另有 `wg sync.WaitGroup`（已用，跟踪异步 goroutine）。
- 调查：`NewProtocol` 之后是否有任何方法并发改写 `handler` 或 `codec` 字段（`grep` 全部赋值点）。
- **若无为并发保护而设的可变状态** → `mu` 是重构遗留死字段 → **删除字段**。
- **若有**（如运行时换 handler/codec）→ 接线锁（真实并发修复）。
- 预期：codec/handler 在 `NewProtocol` 一次性注入后不变 → 删除。但以代码核实为准。

**A2.2 `RatingCalculator.mu`（`internal/core/rating/calculator.go:41`）**

- R2-D2 在评分后重算条目分数（`recompute entry score`）。调查 `SubmitRating` 是否对"读旧分数→写新分数"做未受保护的 RMW。
- **若是未保护 RMW** → 接线 `mu.Lock()`（真实并发 bug 修复，并发评分会丢增量）；同时确认与 `RatingStore`/`EntryStore` 层级锁的关系，避免双重锁。
- **若 store 层已串行化（如 per-entry lock，R2-D1 已加）** → `mu` 冗余 → 删除字段。
- 预期：R2-D1 已在评分路径加 per-entry 锁，`mu` 大概率冗余 → 删除。但以代码核实为准。

**两处都必须在 commit message / 代码注释记录调查结论与依据**（删的理由或加锁的路径）。

---

## R4a-B：staticcheck 根因修复（15）

| 规则 | 位置 | 处置 |
|---|---|---|
| SA1012 nil context | `internal/api/middleware/auth_test.go:220`、`internal/core/export/exporter.go:73,87` | 改 `context.TODO()`（`exporter.go` 顶部 `ctx context.Context` 入参缺省传 `nil`，改为签名内 `context.TODO()` 或由调用方透传） |
| SA1019 弃用 import（`internal/storage`） | `cmd/seed/main.go:30`、`cmd/user/main.go:25`、`internal/api/admin/handler.go:8` | **根因修复**：`internal/storage/memory.go:4` 的 `// Deprecated:` 注解位置泄漏到包级，导致为 `NewPersistentStore`（非弃用）的 import 也被标记。把注解改为 `NewMemoryStore` 函数级 doc comment，使包 import 不再误报 |
| SA1019 libp2p `EnableAutoRelay` | `internal/network/host/host.go:175` | `//nolint:staticcheck // libp2p relay API 迁移（WithStaticRelays/WithPeerSource）为行为变更，留独立轮` + 行内 TODO |
| SA1029 string context key | `internal/api/admin/middleware.go:48`、`internal/api/handler/admin_handler_test.go:58,118` | 定义 `type contextKey string` + `const publicKeyCtxKey contextKey = "public_key"`；先 grep `auth.go` 是否已有 typed key 可复用，避免重复定义 |
| SA5011 可能 nil 解引用 | `internal/api/middleware/auth_test.go:242` | 修测试逻辑（`if user == nil` 之后又解引用 `user.AgentName`，逻辑矛盾） |
| SA6001 低效 string key | `internal/storage/kv/store.go:89` | 内联 `m[string(key)]`，去掉中间变量 |
| SA9003 空分支 | `cmd/pactl/main.go:60`、`cmd/pactl/service.go:160`、`internal/api/middleware/audit.go:107` | 在分支内记录错误（`logger.Warn`/`log.Printf`）；`audit.go:107` 审计写入失败应记日志（审计静默失败是安全隐患） |

**接口变化**：`exporter.go` 若改签名加 `ctx` 入参属内部调用方改动（非外部 API）；context key 类型化为内部重构。均无外部 API 变化。

---

## R4a-C：gosec 逐处处置（13）

| 规则 | 位置 | 处置 |
|---|---|---|
| G101 疑似硬编码凭据 | `pkg/config/config.go:18`（`PlaceholderApiKey` 占位常量）、`internal/api/middleware/apikey.go:11`（`headerApiKey` 是 HTTP 头名，非凭据） | `//nolint:gosec // 占位/头名，非真实凭据` |
| G306 文件权限 >0600 | `pkg/config/config.go:566`（`config.Save`）、`scripts/initdata/main.go:93,106` | 逐处判断：可能含敏感数据（配置/密钥落盘）→ `0600`；刻意世界可读（生成的种子数据 JSON 分发用）→ 保留 `0644` 并注释理由 |
| G402 `InsecureSkipVerify` | `internal/core/email/service.go:154`（配置驱动 `SkipTLSVerify`）、`cmd/pactl/client.go:51`（硬编码 `true`） | email：`//nolint:gosec // 配置驱动，默认关`；pactl：调查是否仅连 localhost（CLI 工具）→ 若是 nolint+注释，若可达生产则改 |
| G115 整型溢出转换 | `internal/storage/index/bleve_engine.go:357,358`（uint64→int）、`internal/network/sync/remote_query.go:211,212`、`internal/network/sync/sync.go:404`（int→int32） | 加受保护转换（`math.MaxInt32` clamp 或 `if v > maxInt32` 守卫）。计数字段实际不会溢出，但转换须显式安全 |
| G109 `Atoi`→int32 溢出 | `internal/api/handler/admin_handler.go:236`（`int32(level)`） | 解析后校验/钳制 `level` 范围再转换 |

**约定**：受保护转换抽一个小 helper（如 `safeInt32(int)`）置于 `pkg/` 或就近，避免散落重复 clamp 逻辑。

---

## R4a-D：errcheck 逐处处置（50）

### D1 生产/非测试代码（实质性，~19 处）

| 位置 | 风险 | 处置 |
|---|---|---|
| `internal/core/election/election.go:179,219,223,229` | **高**——选举结果状态（当选/落选/选举关闭）的 `UpdateStatus`/`Update` 未检查错误，静默失败会**破坏选举结果一致性** | 传播错误（返回）或在不能改签名处 `logger.Error` + 写审计 |
| `internal/core/admin/session.go:121`、`internal/core/email/verification.go:286` | 中高——`crypto/rand.Read` 未检查（会话 token / 验证码熵源失败） | `if _, err := rand.Read(bytes); err != nil { return ..., err }` |
| `pkg/logger/logger.go:157,161` | 中——日志轮转 `os.Rename` 失败被吞 | 记录 rename 失败（`fmt.Fprintf(os.Stderr, ...)` 或内部日志） |
| `internal/service/daemon/daemon.go:26` | 低中——`p.StartFn()` 未检查 | 调查语义；显式 `_ =` + 注释，或传播 |
| `cmd/pactl/service.go:208,317` | 低中——`fmt.Sscanf` 解析 pid 未检查 | 处理解析失败（pid 解析错应返回明确错误而非用零值） |
| `cmd/pactl/service.go:241,253` | 低——`exec.Command(...).Run()` 抓 journalctl/tail 日志 | `//nolint:errcheck // 尽力而为的日志抓取` |
| `cmd/pactl/entry.go:226` | 低——`fmt.Scanln` 确认提示 | `//nolint:errcheck // 一次性确认输入` |
| `scripts/integration/main.go:201,220,236` | 中——`json.Unmarshal` 响应未检查 | 检查（集成脚本静默反序列化失败掩盖断联） |

### D2 测试代码（~31 处）

**约定**：

- **调用必须成功否则测试无意义** → 检查错误 `if err := X(); err != nil { t.Fatal(err) }`。例：`election_test.go:111,135`（`NominateCandidate` 失败则后续断言无意义）、`category/manager_test.go:58,78,95`（`Initialize`）、`title_index_test.go:298-301`（`Build`/`Add`）。
- **惯用忽略**（benchmark 循环、失败即 fatal 的 setup）→ `//nolint:errcheck // reason`。例：`benchmark_test.go:33,63,89`（benchmark 内 encode/decode）、`bleve_engine_test.go` 多处 `IndexEntry` setup。
- 测试 helper 关闭：`defer Close()` / `defer x.Close()` → `defer func(){ _ = x.Close() }()` 或 nolint。
- 统一用 `//nolint:errcheck` 风格（非 `_ =`），与 golangci-native 一致；`_ =` 仅用于生产代码显式丢弃。

**清单**（按文件）：`daemon_test.go`(4)、`logger_test.go`(3)、`client_test.go`(2)、`bleve_engine_test.go`(4)、`title_index_test.go`(4)、`pebble_store_test.go`(4)、`benchmark_test.go`(3)、`api_test.go`(1)、`process_unix_test.go`(1)、`manager_test.go`(3)、`election_test.go`(2)。

---

## R4a-E：启用 4 个 linter + CI 门控

1. **`.golangci.yml`**：把 `gosec`/`staticcheck`/`unused`/`errcheck` 从注释"暂缓"块移入 `enable:` 列表（保留现有 `gofmt`/`goimports`/`misspell`/`ineffassign`/`govet`）。保留 `max-issues-per-linter: 50` / `max-same-issues: 10`（清理后命中数已为 0，余量充足）。更新头部注释说明各 linter 已启用。
2. **CI**（`.github/workflows/ci.yml` 的 `lint` job）：已用 `golangci-lint-action`，自动加载 `.golangci.yml` → 启用后 PR 即被强制。无需改 workflow，但需确认 action 版本支持启用的 linter（v1.64.x 支持）。
3. **`make lint`**：当前仅 `fmt vet`。**可选**追加 `golangci-lint run ./...`（本地预提交便利），CI 仍是最终门控。决策：追加（让本地 `make lint` 与 CI 一致，早暴露）。

---

## 验证

- `golangci-lint run --no-config --disable-all --enable unused,staticcheck,gosec,errcheck ./...` → **exit 0**（逐 linter 也各 exit 0）。
- `golangci-lint run ./...`（用项目 `.golangci.yml`，含新启用的 4 个）→ **exit 0**。
- `make lint` 绿（若追加 golangci）。
- `.github/workflows/ci.yml` lint job 配置正确（action 能加载 4 个新 linter）。
- `go build ./cmd/... ./internal/... ./pkg/...` 绿。
- `go vet ./...` 绿。
- `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` 绿（无回归；删死代码 / 修 errcheck / 加锁均不破坏测试）。
- 两处 mu 字段调查结论写入 commit message（删的理由 / 加锁的路径）。
- 所有 `//nolint` 均具名 + 理由（`grep -rn "//nolint" --include="*.go"` 逐条核对）。

## 风险与回退

- **A2 mu 字段**：若 `RatingCalculator.mu` 调查发现真实未保护 RMW，加锁可能影响并发吞吐（评分热路径）。缓解：确认 per-entry 锁（R2-D1）覆盖后再删 `mu`；若必须加锁，用 `sync.Mutex` 保护最小临界区。
- **B 的 `internal/storage` 弃用注解迁移**：移动注解位置须确保 `NewMemoryStore` 仍被标记、`NewPersistentStore` 不再被标记（用 `staticcheck` 单测验证）。
- **D1 election 错误传播**：若改 `CloseElection`/计票方法签名影响调用方，优先 `logger.Error` + 审计而非改签名（降低爆炸半径）。
- **回退**：每个 task 组独立 commit，任一组引入回归可单独 revert，不影响其余。

## 出范围跟踪

- libp2p `EnableAutoRelay` → SA1019 nolint + TODO，独立轮迁移。
- 其余 R4 迷你轮（内容审核 / KV 备份 / 选举&导出导入 UI / MCP）按推荐顺序后续开。
