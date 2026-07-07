# R4c KV 备份/恢复 + GC 设计

**范围**：R4 第三个迷你轮——KV 层运维（备份/恢复 + 周期 GC）。R4a（linter）/R4b（内容审核）已合入 master（`4771e53`）。当前 KV 层零备份/恢复能力；`BadgerStore.RunGC`/`PebbleStore.Compact` 定义了却从未调用。本轮补齐：在线备份、离线恢复、周期 GC。

**轮次定位**：R4c 只做 KV 运维三件套，不动业务逻辑。备份是 **raw-KV 全量快照**（全部键，含 `user-email:`/`user-hash:`/`meta:`/audit/选举等索引与状态），与既有的 entry 级 ZIP 导出（`internal/core/export/`，丢失索引/计数器/审计/选举）明确区分。

## 目标

- **在线备份**：运行中的节点调引擎一致性快照（Pebble `Checkpoint` / Badger `Backup`），写到 `<DataDir>/backups/<ts>/`，不停机。手动触发（admin 端点 + pactl）。
- **离线恢复**：`pactl backup restore <dir>`（节点已停）引擎直连恢复（Pebble 替换 kv 目录 / Badger `Load`），操作者重启节点。
- **周期 GC**：seed/user 节点后台任务按可配间隔（默认 1h）调空间回收（Pebble `Compact` / Badger `RunValueLogGC`），照搬 `ElectionAutoCloser`/`IntegrityChecker` 范式。
- **备份元数据**：每次备份写 manifest（引擎/timestamp/键数/大小），便于 list 与恢复校验。

## 非目标

- 自动定期备份 + 保留清理（本轮纯手动触发；`Retention` 字段预留但不实现清理逻辑）。
- 在线恢复（离线专用，避免并发写冲突）。
- 增量备份（每次全量快照）。
- 跨引擎恢复（Pebble 备份恢复到 Pebble，Badger 恢复到 Badger）。
- 备份加密/再压缩（Pebble checkpoint 已 Snappy 压缩；Badger Backup 已是紧凑格式）。
- 备份打包成单 archive（保留目录格式，与引擎原生输出一致）。
- bleve 搜索索引的备份/恢复（可从 KV 重建，`NewPersistentStore` 启动已重建；KV 备份恢复后重启自动重建 bleve）。

## 现状核实（代码 grounded）

| 能力 | 现状 | 位置 |
|---|---|---|
| `kv.Store` 接口 | Put/Get/Delete/Scan(prefix)/Close，无 backup/restore/gc | `kv/store.go:29-40` |
| Pebble 引擎句柄 | `db *pebble.DB` 未导出；有 `Flush`/`Compact` 但不在接口 | `kv/pebble_store.go:11,122,127` |
| Badger 引擎句柄 | `db *badger.DB` 未导出；有 `RunGC` 但不在接口 | `kv/badger_store.go:11,92` |
| `storage.Store.KVStore()` | 访问器存在，返回 `kv.Store` 接口 | `store.go:270-272` |
| 生产默认后端 | Pebble（`NewPersistentStore` default→Pebble） | `store.go:184` |
| 节点 KV 路径 | `<DataDir>/kv`（seed `./data/seed/kv`，user `./data/user/kv`） | `cmd/seed/main.go:222`、`cmd/user/main.go:232` |
| 备份/恢复代码 | **完全缺失** | 新建 |
| GC 调度 | `RunGC`/`Compact` 定义但零调用，无周期任务 | 新建 |
| 引擎直连先例 | `scripts/scan_{pebble,badger}.go`（`//go:build ignore`）离线打开引擎 | 照搬范式 |
| pactl ops CLI | cobra；有 `admin export/import`、`service`、`sync` 等，无 `backup` | `cmd/pactl/` |
| Config | `StorageConfig{KVType,SearchType}`，无 backup/gc 字段 | `pkg/config/config.go:125-128` |

## 架构：接口演进

**方案：`kv.Store` 接口加 2 个方法（推荐方案，详见决策）**。

- `Backup(destDir string) error` — 在线一致性快照到目录。
- `RunGC() error` — 周期空间回收（统一命名；Pebble 内部 `db.Compact`，Badger `db.RunValueLogGC`）。

被否方案：① 暴露引擎句柄访问器（泄漏底层类型，破坏抽象）；② 通用 Scan-dump（非原子、更慢更大）；③ 类型断言到具体 store 调 GC（运行时断言脆弱，不如接口方法）。

**恢复不走接口**：离线恢复由 pactl 引擎直连（节点已停），不经运行时 `kv.Store`，故不加 `Restore` 到接口（YAGNI）。

## 组件

### 1. `kv.Store` 接口（`internal/storage/kv/store.go`）

加 2 方法：
```go
type Store interface {
    Put(key, value []byte) error
    Get(key []byte) ([]byte, error)
    Delete(key []byte) error
    Scan(prefix []byte) (map[string][]byte, error)
    Close() error
    Backup(destDir string) error   // R4c：一致性快照到目录
    RunGC() error                   // R4c：周期空间回收
}
```

**实现**：
- **PebbleStore**：`Backup` = `s.db.Checkpoint(destDir)`（Pebble 一致性在线快照）；`RunGC` = `s.db.Compact(nil, nil, false)`（复用现有 `Compact` 逻辑，新增 `RunGC` 方法委托）。
- **BadgerStore**：`Backup` = `s.db.Backup(file, ...)`（写到 `destDir/backup.bak`）；`RunGC` = 复用现有 `RunGC`（`db.RunValueLogGC(0.7)`，循环至无回收）。
- **MemoryStore / JSONFileStore**：`Backup` = dump `map` 到 `destDir/dump.json`；`RunGC` = no-op。生产不用，仅为接口完整 + 测试。

**编译器强制**：所有 `kv.Store` 实现（4 个）必须补全 2 方法；测试 fake 同步。`var _ Store = (*XxxStore)(nil)` 断言已在各文件，编译期暴露遗漏。

### 2. `BackupService`（`internal/core/backup/service.go`）

```go
type Service struct {
    kvStore   kv.Store
    backupDir string  // <DataDir>/backups
    engine    string  // "pebble"|"badger"（写 manifest）
}
func (s *Service) CreateBackup(ctx context.Context) (*BackupResult, error)
func (s *Service) ListBackups() ([]*BackupMeta, error)
```
- `CreateBackup`：生成 `<backupDir>/<unix-ms>/`，调 `kvStore.Backup(dir)`，统计键数（`kvStore.Scan` 各已知前缀求和，或备份后目录大小），写 `dir/manifest.json`（engine/timestamp/keyCount/sizeBytes）。返回 `BackupResult{Dir, SizeBytes, KeyCount, CreatedAt}`。
- `ListBackups`：扫 `<backupDir>/*/manifest.json`，按时间倒序。
- `engine` 字段从 config `KVType` 取。

### 3. Admin 端点（`adminAuthMW`，session-token）

- `POST /api/v1/admin/backup` → `CreateBackup`，返回 `{dir, size_bytes, key_count, created_at}`。
- `GET /api/v1/admin/backups` → `ListBackups`，返回 `{backups: [...]}`。

挂 `internal/api/admin/handler.go`（加 `backupService` + 委托方法），router `registerAdminRoutes` 注册（照搬 R4b review 端点模式）。

### 4. pactl（`cmd/pactl/backup.go`，新）

- `pactl backup create` — **在线**：调运行节点 `POST /api/v1/admin/backup`（复用 `cmd/pactl/client.go` 的 admin 客户端 + admin session/bearer）。打印结果。
- `pactl backup list` — 调 `GET /api/v1/admin/backups`。
- `pactl backup restore <dir>` — **离线**：读节点 config（`--config` 或默认 `DataDir`）得 `KVType`/`KVPath`；检查节点未运行（锁文件/PID，若运行则拒绝并提示先停）；引擎直连恢复：
  - Pebble：`rm -rf <KVPath> && cp -r <dir> <KVPath>`（checkpoint 目录即合法 Pebble DB）。
  - Badger：`badger.Open(KVPath)` + `db.Load(<dir>/backup.bak)`（先清空 KVPath 再 Load，或 Load 到新目录再替换）。
  - 完成后提示操作者重启节点。
- 照搬 `scripts/scan_*.go` 的离线引擎打开方式（`pebble.Open`/`badger.Open` at `KVPath`）。

### 5. GC 后台任务（`internal/core/storage/gc.go`，新）

```go
type GarbageCollector struct {
    kvStore  kv.Store
    interval time.Duration
}
func (g *GarbageCollector) Start(ctx context.Context)  // 周期调 kvStore.RunGC()
func (g *GarbageCollector) Stop()
```
照搬 `ElectionAutoCloser`/`IntegrityChecker` 范式（`time.Ticker` + ctx.Done + recover 防 panic）。seed/user 节点 `main` 启动时 spawn（`app.gc` 字段 + Start/Stop 接入 `Stop()` 关闭顺序）。

### 6. Config（`pkg/config/config.go`）

`StorageConfig` 加：
```go
type StorageConfig struct {
    KVType      string `json:"kv_type"`      // 默认 "pebble"
    SearchType  string `json:"search_type"`
    BackupDir   string `json:"backup_dir"`   // R4c：默认 "<DataDir>/backups"（节点 main 兜底）
    GCIntervalS int    `json:"gc_interval_s"` // R4c：默认 3600（1h）；<=0 禁用
}
```

## 数据流

1. **备份**：admin SPA/`pactl backup create` → `POST /api/v1/admin/backup` → `BackupService.CreateBackup` → `kvStore.Backup(<DataDir>/backups/<ts>/)` → manifest → 返回。
2. **恢复**：操作者停节点 → `pactl backup restore <dir>` → 读 config → Pebble 替换目录 / Badger Load → 提示重启 → 节点启动重建 bleve（`NewPersistentStore` 现有逻辑）。
3. **GC**：节点启动 → spawn `GarbageCollector` → 每 `GCIntervalS` → `kvStore.RunGC()`（Pebble Compact / Badger RunValueLogGC）→ recover 防 panic。

## 测试

- **`kv.Store.Backup/RunGC` 各后端**（`kv/*_store_test.go`）：Pebble `Backup` 产生合法 checkpoint（重新打开可读全部键）；Badger `Backup`→`Load` 往返；Memory dump/load；`RunGC` 不 panic 且调底层（mock 或用真实引擎小数据集）。
- **`BackupService`**：`CreateBackup` 写 manifest、目录创建、keyCount/size 非零；`ListBackups` 倒序、解析 manifest。
- **GC**：`GarbageCollector` 间隔受控（短间隔测试）、调 `RunGC`（计数 mock）、ctx 取消退出、panic 不崩溃。
- **Admin 端点**：`POST /backup` 返回 200 + 结构；`GET /backups` 列表；session-token 鉴权（401 无 token）。
- **pactl**：`backup create/list` 调端点（mock client）；`backup restore` 离线（临时 KVPath + 备份目录，Pebble 往返）。

## 接口变化

- `kv.Store` 加 2 方法（所有实现 + fake 必须补全；编译期强制）。
- 新增 2 admin 端点（`/api/v1/admin/backup`、`/backups`）。
- 新增 pactl `backup` 命令组。
- `StorageConfig` 加 2 字段（向后兼容，零值有默认）。
- 无外部 SDK 变化。

## 风险与回退

- **接口演进**：`kv.Store` 加方法波及所有实现 + 测试 fake。编译器强制补全；逐实现 TDD。**回退**：每个实现独立 commit，可单独 revert。
- **Pebble Checkpoint 在线一致性**：Pebble `Checkpoint` 保证 point-in-time 一致快照（官方文档），无需停机。风险低。
- **离线恢复期间误启动节点**：pactl `restore` 检查节点未运行（锁文件/PID），运行中拒绝。恢复后提示重启。
- **GC 长时间 Compact 阻塞**：Pebble `Compact(nil,nil)` 在大库上可能耗时；放后台 goroutine 不阻塞主路径，间隔默认 1h 保守。可配 `GCIntervalS<=0` 禁用。
- **Badger RunGC 循环**：`RunValueLogGC` 单次仅回收一部分，需循环至返回 < 1.0；实现里循环上限防死循环。

## 出范围跟踪

- 自动定期备份 + 保留清理（`Retention` 字段已预留）。
- 在线恢复 / 增量备份 / 跨引擎恢复 / 备份加密打包。
- bleve 索引备份（可从 KV 重建，本轮不单独备份）。
