# R2 正确性加固实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 闭合 Polyant v2.3.0 审计的 12 个 Tier-2 正确性缺陷（并发竞态、版本/时间戳语义错乱、镜像同步失效、搜索索引↔store 不一致、评分/选举 lost-update、goroutine 泄漏、静默吞错），生产后端（Badger/Pebble + bleve）下数据完整、P2P 仲裁与搜索一致，不破坏历史数据。

**Architecture:** 5 个 task-group（R2-A sync 并发与镜像 / R2-B 存储语义统一 / R2-C 搜索索引一致 / R2-D 评分选举并发 / R2-E 死代码清理），共 12 个独立可提交 task，每组 TDD、独立测试周期、独立提交。核心原则：存储层忠实持久化调用方语义（不再副作用改 Version/UpdatedAt）；并发修复用封装类型 / 复用 `lockFor` per-key 锁；索引启动 rebuild + 写错误不再静默双管齐下。

**Tech Stack:** Go 1.25.7 / sync 原语（RWMutex、sync.Map、WaitGroup、context）/ badger / pebble / bleve v2 / libp2p mocknet / 标准测试 + `-race`。

**Spec:** `docs/superpowers/specs/2026-07-04-polyant-r2-correctness-design.md`

## Global Constraints

- Go 版本：`go 1.25.7`（go.mod）；CI `1.25.x`；本地 1.26.2 可编译。
- 每个改动**先写失败测试再实现**（TDD）。并发项（A1/A2/D1/D3）`go test -race` 必须通过。
- **补 Badger/Pebble 真实后端测试**（B1/B3 关键）：现测试多用 Memory 后端，掩盖生产路径 bug。B1 的 Version 测试、B3 的 Scan 错误测试必须在真实后端上跑。
- 提交信息前缀：`fix(correctness): ...` / `feat(correctness): ...` / `refactor(correctness): ...` / `test(correctness): ...` / `chore(correctness): ...`。
- **纯 bugfix，默认无配置开关**；行为变化（镜像接收端"启用"、评分聚合"新增"）视为修 bug，默认开。
- **时间戳迁移幂等**：值 `< 1e12` 视为秒 ×1000；`== 0` 跳过；重复运行不变。
- **存储层忠实持久化调用方语义**：`kv` 层不再副作用改 `Version`/`UpdatedAt`，由调用方（handler/sync）显式赋值。
- 每个 task 结束运行 `go build ./cmd/... ./internal/... ./pkg/... && go vet ./... && go test -race -count=1 <受影响包>`，全绿才提交。
- 中文注释 OK，沿用周边文件风格。
- 本计划所有 file:line 引用以 2026-07-04 master `877ce1a` 为准；实现时若行号漂移按符号名定位。

## File Structure

**R2-A（sync 层）：**
- Modify: `internal/network/sync/sync.go`（VersionVector 类型 :39-90、SyncEngine.versionVec :98、所有 versionVec 访问点、HandleMirrorRequest :491-558、resolveConflictAndMerge :566-601、HandleMirrorData 新增）
- Modify: `internal/network/protocol/protocol.go`（processMessage switch :80-173、Handler 接口 :15-24、镜像消费者 goroutine :118-138）
- Modify: `internal/network/protocol/codec_test.go`、`internal/network/protocol/mock_protocol.go`（Handler 接口加方法后补 mock）
- Modify: `internal/network/sync/remote_query.go`（cacheRemoteEntries :95）
- Test: `internal/network/sync/sync_test.go`、`internal/network/sync/mocknet_e2e_test.go`

**R2-B（存储层）：**
- Modify: `internal/storage/kv/entry_store.go`（UpdateEntry :153-154）
- Modify: `internal/storage/model/models.go`（NewKnowledgeEntry :75、字段注释 :56-57）
- Modify: `internal/storage/store.go`（NewPersistentStore :168-227 加迁移）
- Create: `internal/storage/migrate.go`（migrateTimestampsToMillis）
- Modify: `internal/storage/kv/badger_store.go`（Scan :76）、`internal/storage/kv/pebble_store.go`（Scan :84-85、:98）
- Test: `internal/storage/kv/store_test.go`（:285 断言）、`internal/storage/kv/badger_store_test.go`（新建）、`internal/storage/kv/pebble_store_test.go`、`internal/storage/model/models_test.go`、`internal/storage/migrate_test.go`（新建）

**R2-C（索引）：**
- Modify: `internal/storage/index/bleve_engine.go`（NewBleveEngine :46-128 抽 helper、Search :190-308 加 status 过滤、新增 Rebuild）
- Modify: `internal/storage/store.go`（NewPersistentStore :212 后加 rebuild 调用）
- Modify: `internal/api/handler/entry_handler.go`（:310/:436/:514）、`internal/api/handler/batch_handler.go`（:407/:521/:596）、`internal/network/sync/sync.go`（:267/:288）、`internal/core/seed/initializer.go`（:103）
- Test: `internal/storage/index/bleve_engine_test.go`、handler/batch/sync 测试新增 mockSearchEngine

**R2-D（评分/选举）：**
- Modify: `internal/api/handler/user_handler.go`（RateEntryHandler :395-482 加 lockForEntry + 重算）
- Modify: `internal/storage/kv/election_store.go`（UpdateStatus :166-173 加锁）
- Test: `internal/api/handler/user_handler_test.go`、`internal/storage/kv/election_atomic_test.go`

**R2-E（死代码）：**
- Modify: `internal/network/dht/dht.go`（Bootstrap :52-69/:75）、`internal/network/host/host.go`（HostConfig.SeedPeers :61-62）、`cmd/user/main.go`（:277）、`cmd/seed/main.go`（:305）
- Modify: `internal/network/dht/dht_test.go`（删 :118-143）

---

# R2-A：sync 层并发与镜像

闭合：versionVec 并发 fatal panic（A1）、goroutine 泄漏残留（A2）、镜像接收端未实现（A3）。

## Task A1: VersionVector 封装为线程安全类型

**Files:**
- Modify: `internal/network/sync/sync.go`（VersionVector :39-90、SyncEngine.versionVec :98、所有访问点 :112/224/242/258/264/279/285/300/374/382/387/405/801）
- Test: `internal/network/sync/sync_test.go`

**Interfaces:**
- Produces: `NewVersionVector() *VersionVector`；`(*VersionVector)` 方法 `Increment/Get/Set/Merge/Clone/ToProto/Delete/Range`（内部 `sync.RWMutex`）；`SyncEngine.versionVec` 类型由 `VersionVector`（裸 map）改为 `*VersionVector`。

- [ ] **Step 1: 写失败测试**（追加到 `sync_test.go`）

```go
// TestVersionVector_ConcurrentSafe 验证 VersionVector 在多 goroutine 并发下不触发
// fatal "concurrent map read/write" 且 -race 无报告。重构前 NewVersionVector 不存在 → 编译失败。
func TestVersionVector_ConcurrentSafe(t *testing.T) {
	vv := NewVersionVector()
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("e%d", i%20)
			vv.Set(id, int64(i))
			_ = vv.Get(id)
			_ = vv.Increment(id)
			_ = vv.ToProto()
			_ = vv.Clone()
			vv.Merge(map[string]int64{id: int64(i + 1)})
		}(i)
	}
	// 并发 delete + range
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			vv.Delete(fmt.Sprintf("e%d", i%20))
			vv.Range(func(_ string, _ int64) {})
		}(i)
	}
	wg.Wait()
	// 最终值应是最后写入的某个版本（非零即可，不要求精确）
	if vv.Get("e0") == 0 && vv.Get("e1") == 0 {
		t.Fatal("versionVector lost all writes")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/network/sync/ -run TestVersionVector_ConcurrentSafe`
Expected: FAIL / 编译失败（`NewVersionVector` undefined）

- [ ] **Step 3: 重构 VersionVector 类型**（替换 `sync.go:39-90`）

```go
// VersionVector 是线程安全的版本向量。所有方法内部加锁，可在多 goroutine 间并发使用。
type VersionVector struct {
	mu sync.RWMutex
	m  map[string]int64
}

// NewVersionVector 创建空的线程安全版本向量。
func NewVersionVector() *VersionVector {
	return &VersionVector{m: make(map[string]int64)}
}

// Increment 递增某条目版本号并返回新值。
func (vv *VersionVector) Increment(entryID string) int64 {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	vv.m[entryID]++
	return vv.m[entryID]
}

// Get 返回某条目版本号（不存在返回 0）。
func (vv *VersionVector) Get(entryID string) int64 {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	return vv.m[entryID]
}

// Set 设置某条目版本号。
func (vv *VersionVector) Set(entryID string, ver int64) {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	vv.m[entryID] = ver
}

// Merge 合并另一版本向量（按 key 取 max）。
func (vv *VersionVector) Merge(other map[string]int64) {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	for k, v := range other {
		if cur, ok := vv.m[k]; !ok || v > cur {
			vv.m[k] = v
		}
	}
}

// Clone 返回内部 map 的深拷贝。
func (vv *VersionVector) Clone() map[string]int64 {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	out := make(map[string]int64, len(vv.m))
	for k, v := range vv.m {
		out[k] = v
	}
	return out
}

// ToProto 返回可序列化的 map 拷贝（用于 protobuf 传输）。
func (vv *VersionVector) ToProto() map[string]int64 {
	return vv.Clone()
}

// Delete 删除某条目版本记录。
func (vv *VersionVector) Delete(entryID string) {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	delete(vv.m, entryID)
}

// Range 遍历所有条目（持读锁，回调内不可回头改 vv）。
func (vv *VersionVector) Range(fn func(entryID string, ver int64)) {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	for k, v := range vv.m {
		fn(k, v)
	}
}
```

`VersionVectorFromProto`（原 :84）保留为返回裸 map 的辅助：
```go
// VersionVectorFromProto 从 protobuf map 构造裸 map（供 Merge 消费）。
func VersionVectorFromProto(m map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
```

- [ ] **Step 4: 改 SyncEngine.versionVec 字段 + 所有调用点**

字段（`:98`）：`versionVec VersionVector` → `versionVec *VersionVector`

逐点改造（按侦察清单）：
- `:112` `versionVec: make(VersionVector)` → `versionVec: NewVersionVector()`
- `:224` `se.versionVec.ToProto()` → 不变（方法仍存在，内部加锁）
- `:242` `se.versionVec = se.versionVec.Merge(remoteVV)` → `se.versionVec.Merge(remoteVV)`（in-place；`remoteVV` 改为 `map[string]int64`，见下）
- `:258` `se.versionVec.Get(entry.ID)` → 不变
- `:264` `se.versionVec[entry.ID] = entry.Version` → `se.versionVec.Set(entry.ID, entry.Version)`
- `:279`/`:285` 同 `:258`/`:264`
- `:300` `delete(se.versionVec, deletedID)` → `se.versionVec.Delete(deletedID)`
- `:374` `for k, v := range se.versionVec` → `se.versionVec.Range(func(k string, v int64) { /* 原 loop 体，用 k/v */ })`
- `:382`/`:387` 同 Get/Set
- `:405` `se.versionVec.ToProto()` → 不变
- `:801` `se.versionVec = se.versionVec.Merge(remoteVV)` → `se.versionVec.Merge(remoteVV)`

`Merge` 调用点 `remoteVV` 变量类型：原来是 `VersionVector`（map），现在 `Merge` 接收 `map[string]int64`。把 `:242`/`:801` 上游 `remoteVV` 的来源（来自 `VersionVectorFromProto(...)` 或 `resp.GetServerVersionVector()`）改为直接传 proto map。若上游已是 `map[string]int64` 直接传入；若是 `*VersionVector`，改传 `.Clone()`。

`GetVersionVector`（`:369-378`）原手工 copy，改为：
```go
func (se *SyncEngine) GetVersionVector() map[string]int64 {
	return se.versionVec.Clone()
}
```
（去掉 :370-371 的 se.mu.RLock——versionVec 自保护；若该函数还读了 se 其他字段，仅对其他字段保留 se.mu）。

注意：`se.mu` 仍保护 state/lastSync 等字段；versionVec 不再依赖 se.mu。`HandleSyncRequest`（:394-395）/`HandleBitfield`（:800-802）仍持 se.mu 读其他字段时调 `versionVec.ToProto()/Merge()` 不会死锁（vv 锁细粒度、不嵌套 se.mu）。

- [ ] **Step 5: 跑测试确认通过 + race**

Run: `go test -race -count=1 ./internal/network/sync/`
Expected: PASS（含 TestVersionVector_ConcurrentSafe 及现有 VersionVector/Merge/GetState 测试可能需小改适配新签名——见 Step 6）

- [ ] **Step 6: 修现有测试签名适配**

`sync_test.go` 中直接 `make(sync.VersionVector)` 的测试（:575 TestVersionVectorConcurrentIncrement、:599 TestVersionVectorConcurrentMerge）改为 `NewVersionVector()` + 新方法；这些测试原本断言弱、可能侥幸通过，借机用新 API 重写断言（如 Increment 返回值校验）。

Run: `go test -race -count=1 ./internal/network/sync/`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/network/sync/sync.go internal/network/sync/sync_test.go
git commit -m "refactor(correctness): thread-safe VersionVector, fix concurrent map fatal (R2-A1)"
```

## Task A2: 修复 3 处 goroutine 泄漏

**Files:**
- Modify: `internal/network/sync/sync.go`（HandleMirrorRequest 生产者 :494-555）
- Modify: `internal/network/protocol/protocol.go`（镜像消费者 :124-137）
- Modify: `internal/network/sync/remote_query.go`（cacheRemoteEntries :95）
- Test: `internal/network/sync/sync_test.go`、`internal/network/protocol/protocol_test.go`（若无则新增）

**Interfaces:**
- Consumes: SyncEngine 的 `wg sync.WaitGroup` + `cancel context.CancelFunc`（已存在 :101-102）；RemoteQueryService 需新增 `wg`/`cancel` 字段或由上层注入。
- Produces: HandleMirrorRequest 生产者与 protocol 消费者注册到 `wg` 并响应 ctx；cacheRemoteEntries 继承服务级 ctx。

- [ ] **Step 1: 写失败测试**（追加到 `sync_test.go`）

```go
// TestSyncEngine_StopReapsMirrorProducer 验证 Stop 后无 goroutine 悬挂。
func TestSyncEngine_StopReapsMirrorProducer(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	// 填入若干条目让生产者有数据可发
	for i := 0; i < 200; i++ {
		_, _ = store.Entry.Create(context.Background(), &model.KnowledgeEntry{
			ID: fmt.Sprintf("e%d", i), Title: "t", Content: "c", Category: "cat",
			Status: model.EntryStatusPublished, Version: 1,
			CreatedAt: model.NowMillis(), UpdatedAt: model.NowMillis(),
		})
	}
	se := NewSyncEngine(nil, nil, store, &SyncConfig{AutoSync: false})
	require.NoError(t, se.Start(context.Background()))

	before := runtime.NumGoroutine()
	// 触发镜像生产，但无人消费 dataCh（消费者弃读场景）
	_, _ = se.HandleMirrorRequest(context.Background(), &protocolpkg.MirrorRequest{
		RequestID: "r1", Categories: []string{"cat"},
	})
	time.Sleep(50 * time.Millisecond) // 让生产者进入阻塞写

	require.NoError(t, se.Stop())
	// 等 goroutine 退出
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > before {
		t.Fatalf("goroutine leak: before=%d after=%d", before, got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -race -count=1 ./internal/network/sync/ -run TestSyncEngine_StopReapsMirrorProducer`
Expected: FAIL（生产者阻塞在 `dataCh <-`，Stop 不 cancel 不 wait，goroutine 泄漏 → 超时后 after > before）

- [ ] **Step 3: 修 HandleMirrorRequest 生产者**（`sync.go:494-555`）

```go
	dataCh := make(chan *protocolpkg.MirrorData, 10)
	se.wg.Add(1)
	go func() {
		defer se.wg.Done()
		defer close(dataCh)
		entries, _, err := se.store.Entry.List(ctx, EntryFilterAll)
		if err != nil {
			log.Printf("[Sync] HandleMirrorRequest list entries failed: %v", err)
			return
		}
		// ... 过滤 + 分批逻辑保持不变 ...
		for i := 0; i < totalBatches; i++ {
			// ... 组装 data ...
			select {
			case dataCh <- &protocolpkg.MirrorData{ /* ... */ }:
			case <-ctx.Done():
				return
			}
		}
	}()
```
关键改动：`se.wg.Add(1)` + `defer se.wg.Done()` + `select` 包裹 `dataCh <-`（响应 ctx 取消）。`ctx` 由 `HandleMirrorRequest` 的入参传下（已是服务级 ctx 的派生）。

注意：`HandleMirrorRequest` 当前入参 `ctx` 是请求级 ctx；为了让 Stop 能取消生产者，生产者应继承**服务级 ctx**。若 SyncEngine 持有服务级 ctx（Start 时保存），生产者改用服务级 ctx + 请求 cancel。实现：在 Start 中保存 `se.ctx`（context.Context）字段，HandleMirrorRequest 生产者用 `se.ctx`。本步同时加 `se.ctx context.Context` 字段并在 Start 赋值。

- [ ] **Step 4: 修 protocol 镜像消费者**（`protocol.go:124-137`）

```go
	case MessageTypeMirrorRequest:
		r := msg.Payload.(*MirrorRequest)
		dataCh, err := p.handler.HandleMirrorRequest(ctx, r)
		if err != nil {
			return nil, err
		}
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Protocol] mirror consumer panic: %v", r)
				}
			}()
			for {
				select {
				case data, ok := <-dataCh:
					if !ok {
						return
					}
					protoMsg := toProtoMessage(&Message{
						Header: NewMessageHeader(MessageTypeMirrorData), Payload: data,
					})
					s, err := p.host.NewStream(ctx, mirrorDialTarget(remotePeer, r), AWSPProtocolID)
					if err != nil {
						log.Printf("[Protocol] mirror NewStream failed: %v", err)
						continue
					}
					writer := NewProtobufStreamWriter(s)
					if err := writer.WriteMessage(protoMsg); err != nil {
						log.Printf("[Protocol] mirror write failed: %v", err)
					}
					s.Close()
				case <-ctx.Done():
					return
				}
			}
		}()
		return nil, nil
```
关键改动：`p.wg.Add/Done` + recover + `select` 响应 `ctx.Done()` + 不再吞 NewStream/WriteMessage 错误。需给 Protocol 加 `wg sync.WaitGroup` 字段，并在 `NewProtocol`/`Close` 里 Add/Wait（参照 SyncEngine.Start/Stop 范式）。若 Protocol 已有 Close，在 Close 里 `p.wg.Wait()`。

- [ ] **Step 5: 修 cacheRemoteEntries**（`remote_query.go:94-96`）

给 RemoteQueryService 加服务级生命周期：
```go
type RemoteQueryService struct {
	// ... 既有字段 ...
	wg     sync.WaitGroup
	cancel context.CancelFunc
	ctx    context.Context
}
```
在 RemoteQueryService 的 Start/构造里 `s.ctx, s.cancel = context.WithCancel(ctx)`；Close 里 `s.cancel(); s.wg.Wait()`。若无 Start/Close，新增并在上层（node）调用。

改 `:94-96`：
```go
	if s.config.CacheResults && len(remoteEntries) > 0 {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.cacheRemoteEntries(s.ctx, remoteEntries) // 用服务级 ctx，不再 context.Background()
		}()
	}
```

- [ ] **Step 6: 跑测试 + 全量 race**

Run: `go test -race -count=1 ./internal/network/sync/ ./internal/network/protocol/`
Expected: PASS（无 goroutine 泄漏）

- [ ] **Step 7: 提交**

```bash
git add internal/network/sync/sync.go internal/network/sync/remote_query.go internal/network/sync/sync_test.go internal/network/protocol/protocol.go
git commit -m "fix(correctness): reap leaked goroutines (mirror prod/cons, cacheRemote) (R2-A2)"
```

## Task A3: 实现镜像同步接收端

**Files:**
- Modify: `internal/network/protocol/protocol.go`（processMessage :170 default 前加 case；Handler 接口 :15-24 加方法）
- Modify: `internal/network/protocol/codec_test.go`、`internal/network/protocol/mock_protocol.go`（mock 补 HandleMirrorData）
- Modify: `internal/network/sync/sync.go`（新增 HandleMirrorData）
- Test: `internal/network/sync/mocknet_e2e_test.go`

**Interfaces:**
- Produces: `Handler.HandleMirrorData(ctx context.Context, d *MirrorData) error`；`SyncEngine.HandleMirrorData` 反序列化 `d.Entries` → 逐个 `resolveConflictAndMerge` 落库 + 更新 versionVec + 建索引。
- Consumes: `resolveConflictAndMerge`（:566）、`MessageTypeMirrorData` 常量（types.go:16 已定义）、toProtoMirrorData/fromProtoMirrorData（converter.go:350-372 已存在）。

- [ ] **Step 1: 写失败测试**（追加到 `mocknet_e2e_test.go`）

```go
// TestSync_MirrorDataReceivedAndStored 验证镜像请求 → 生产端推 MirrorData → 接收端 HandleMirrorData 落库。
func TestSync_MirrorDataReceivedAndStored(t *testing.T) {
	nodes := setupMocknetNodes(t, 2)
	src := nodes[0]   // 镜像源
	dst := nodes[1]   // 镜像方

	// 在 src 放一个 published 条目（带合法签名供 R1 验签通过；默认 RequireEntrySignatures=false 可不带）
	srcEntry := &model.KnowledgeEntry{
		ID: "mirror-e1", Title: "镜像条目", Content: "内容", Category: "cat",
		Status: model.EntryStatusPublished, Version: 1,
		CreatedAt: model.NowMillis(), UpdatedAt: model.NowMillis(),
		CreatedBy: "src-creator",
	}
	_, err := src.store.Entry.Create(context.Background(), srcEntry)
	require.NoError(t, err)

	// dst 向 src 发 MirrorRequest，src 推 MirrorData 回 dst，dst 的 HandleMirrorData 落库
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = dst.proto.SendMirrorRequest(ctx, src.h.ID(), &protocolpkg.MirrorRequest{
		RequestID: "req1", Categories: []string{"cat"},
	})
	require.NoError(t, err)

	// 轮询 dst store，等待镜像条目出现
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, gerr := dst.store.Entry.Get(context.Background(), "mirror-e1")
		if gerr == nil && got != nil {
			assert.Equal(t, "镜像条目", got.Title, "镜像条目应在接收端落库")
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("镜像条目未在接收端落库（HandleMirrorData 未实现或未路由）")
}
```

> 若 `SendMirrorRequest` 客户端方法不存在，本步同时新增（见 Step 4）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -race -count=1 ./internal/network/sync/ -run TestSync_MirrorDataReceivedAndStored`
Expected: FAIL（接收端 processMessage 走 default 丢弃，镜像条目不出现；或 SendMirrorRequest undefined）

- [ ] **Step 3: processMessage 加 case + Handler 接口加方法**

Handler 接口（`protocol.go:15-24`）在 `HandleMirrorRequest` 后追加：
```go
	HandleMirrorData(ctx context.Context, d *MirrorData) error
```

processMessage（`:170` default 前）加：
```go
	case MessageTypeMirrorData:
		d, ok := msg.Payload.(*MirrorData)
		if !ok {
			return nil, fmt.Errorf("invalid MirrorData payload")
		}
		if err := p.handler.HandleMirrorData(ctx, d); err != nil {
			return nil, fmt.Errorf("handle mirror data: %w", err)
		}
		return nil, nil
```

- [ ] **Step 4: SyncEngine 实现 HandleMirrorData**（sync.go 新增）

```go
// HandleMirrorData 处理接收到的镜像数据批次：反序列化 entries，逐个经冲突仲裁落库。
func (se *SyncEngine) HandleMirrorData(ctx context.Context, d *protocolpkg.MirrorData) error {
	if d == nil {
		return fmt.Errorf("nil mirror data")
	}
	for _, entryJSON := range d.Entries {
		var entry model.KnowledgeEntry
		if err := json.Unmarshal(entryJSON, &entry); err != nil {
			log.Printf("[Sync] mirror data unmarshal entry failed: %v", err)
			continue
		}
		localVersion := se.versionVec.Get(entry.ID)
		merged, err := se.resolveConflictAndMerge(ctx, &entry, localVersion)
		if err != nil {
			log.Printf("[Sync] mirror merge entry %s failed: %v", entry.ID, err)
			continue
		}
		se.versionVec.Set(merged.ID, merged.Version)
		if se.store.Search != nil {
			if err := se.store.Search.IndexEntry(merged); err != nil {
				log.Printf("[Sync] mirror index entry %s failed: %v", merged.ID, err)
			}
		}
	}
	return nil
}
```
（R2-C2 会把这里的 `_ =` / 裸调用改为 log；本 task 先 log，保持一致。）

- [ ] **Step 5: 补 mock（codec_test.go mockHandler、mock_protocol.go）**

给所有实现 `Handler` 接口的 mock 加 `HandleMirrorData` 方法（否则编译失败）：
```go
func (m *mockHandler) HandleMirrorData(ctx context.Context, d *MirrorData) error { return nil }
```

- [ ] **Step 6: 补 SendMirrorRequest 客户端方法**（若不存在）

`protocol.go` 新增：
```go
// SendMirrorRequest 向 peer 发送镜像请求。镜像数据通过 HandleMirrorData 异步回流。
func (p *Protocol) SendMirrorRequest(ctx context.Context, target peer.ID, req *MirrorRequest) error {
	s, err := p.host.NewStream(ctx, target, AWSPProtocolID)
	if err != nil {
		return fmt.Errorf("new stream: %w", err)
	}
	defer s.Close()
	writer := NewProtobufStreamWriter(s)
	msg := &Message{Header: NewMessageHeader(MessageTypeMirrorRequest), Payload: req}
	return writer.WriteMessage(toProtoMessage(msg))
}
```

- [ ] **Step 7: 跑测试 + 全量**

Run: `go test -race -count=1 ./internal/network/sync/ ./internal/network/protocol/`
Expected: PASS（镜像条目在接收端落库）

- [ ] **Step 8: 提交**

```bash
git add internal/network/protocol/protocol.go internal/network/protocol/codec_test.go internal/network/protocol/mock_protocol.go internal/network/sync/sync.go internal/network/sync/mocknet_e2e_test.go
git commit -m "feat(correctness): implement mirror data receiver (HandleMirrorData) (R2-A3)"
```

---

# R2-B：存储层语义统一

闭合：双重 Version++（B1）、时间戳单位错配 + 历史迁移（B2）、Scan 吞错（B3）。原则：`kv` 层不再副作用改 `Version`/`UpdatedAt`，由调用方负责。

## Task B1: 删除 kv 层 Version++/UpdatedAt 覆盖

**Files:**
- Modify: `internal/storage/kv/entry_store.go:153-154`
- Modify: `internal/storage/kv/store_test.go:285`（断言）
- Test: `internal/storage/badger_adapter_test.go`（补 Version 断言，若现有 :56 未断言则加）

**Interfaces:**
- Produces: `kv.EntryStore.UpdateEntry` 不再副作用自增 Version / 覆盖 UpdatedAt（纯写入入参，仅重算 ContentHash）。调用方（handler/sync）语义不变（handler 已自增、sync 已设 max）。

- [ ] **Step 1: 改现有失败断言**（`store_test.go` TestEntryStoreUpdate :285）

当前 :285 期望 `got.Version != 2`（依赖 kv 自增）。改为断言"UpdateEntry 不动 Version"：

```go
	// B1：kv 层不再副作用自增 Version；调用方负责。UpdateEntry 后 Version 保持入参值。
	updated.Version = 5 // 显式设一个值
	require.NoError(t, es.UpdateEntry(updated))
	got, err := es.GetEntry(updated.ID)
	require.NoError(t, err)
	if got.Version != 5 {
		t.Fatalf("UpdateEntry must not bump Version; got=%d want=5", got.Version)
	}
```

（具体测试函数结构以仓库实际为准；核心断言：UpdateEntry 前后 Version 不变。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/storage/kv/ -run TestEntryStoreUpdate`
Expected: FAIL（当前实现 :154 `entry.Version++` 会把 5 变 6）

- [ ] **Step 3: 删除 kv 层副作用**（`entry_store.go:152-155`）

```go
	// B1：Version/UpdatedAt 由调用方负责（handler 自增、sync 设 max）。
	// 存储层只忠实写入，并重算 ContentHash（幂等，保证哈希契约）。
	entry.ContentHash = entry.ComputeContentHash()
```
即删除原 :153 `entry.UpdatedAt = time.Now().Unix()` 与 :154 `entry.Version++`，保留 :155 ContentHash 重算。

- [ ] **Step 4: 补 Badger 后端 Version 端到端测试**

`badger_adapter_test.go` TestBadgerEntryStore_Update（:56）补 Version 断言（Badger 路径此前无 Version 测试，正是掩盖 bug 的盲区）：

```go
	// Badger 路径：调用方自增 +1，Update 落库恰为该值（不再 +1）
	e.Version = 1
	_, err = s.Update(ctx, e)
	require.NoError(t, err)
	e.Version++ // 模拟 handler 自增
	_, err = s.Update(ctx, e)
	require.NoError(t, err)
	got, _ := s.Get(ctx, e.ID)
	if got.Version != 2 {
		t.Fatalf("Badger update Version: got=%d want=2 (no double bump)", got.Version)
	}
```

- [ ] **Step 5: 跑测试 + 受影响包**

Run: `go test -race -count=1 ./internal/storage/... ./internal/api/handler/`
Expected: PASS（含 handler_test.go:768 `updated.Version != 2` 仍通过——handler 自增 +1、kv 不再 ++）

- [ ] **Step 6: 提交**

```bash
git add internal/storage/kv/entry_store.go internal/storage/kv/store_test.go internal/storage/badger_adapter_test.go
git commit -m "fix(correctness): kv layer no longer bumps Version/overwrites UpdatedAt (R2-B1)"
```

## Task B2: 时间戳统一毫秒 + 历史数据迁移

**Files:**
- Modify: `internal/storage/model/models.go:75`（NewKnowledgeEntry）
- Modify: `internal/storage/model/models.go:56-57`（字段注释）
- Create: `internal/storage/migrate.go`
- Modify: `internal/storage/store.go`（NewPersistentStore 迁移挂载）
- Test: `internal/storage/model/models_test.go`、`internal/storage/migrate_test.go`（新建）

**Interfaces:**
- Produces: `model.NewKnowledgeEntry` 用毫秒；`migrateTimestampsToMillis(entryStore EntryStore) error`（幂等）。

- [ ] **Step 1: 确认迁移挂载点**（先 grep）

Run: `grep -rn "NewPersistentStore\b\|NewBadgerStore\b\|NewBadgerStoreWithCloser\b" cmd/ internal/ | grep -v _test.go`

记录 cmd/seed、cmd/user 实际用哪个工厂。若都走 `NewPersistentStore`，迁移只挂 store.go 一处；若走 `NewBadgerStoreWithCloser`（badger_adapter.go:528/570 也重建 titleIdx/backlinkIdx），需在两处都挂或抽公共 `initStoreIndexes()` 函数。本计划默认挂 `NewPersistentStore`（store.go:199 后），实现时按 grep 结果调整。

- [ ] **Step 2: 写失败测试**（`model/models_test.go` 追加 + 新建 `migrate_test.go`）

`models_test.go` TestNewKnowledgeEntry（:62）追加毫秒断言：
```go
	if e.CreatedAt < 1e12 || e.UpdatedAt < 1e12 {
		t.Fatalf("NewKnowledgeEntry timestamps must be millis (>=1e12); got created=%d updated=%d", e.CreatedAt, e.UpdatedAt)
	}
```

`migrate_test.go`（新建，包 `storage`）：
```go
package storage

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/stretchr/testify/require"
)

func TestMigrateTimestampsToMillis_SecondsToMillis(t *testing.T) {
	dir := t.TempDir()
	w1, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)

	// 旧数据：秒级时间戳
	sec := int64(1_700_000_000) // 2023-11，秒
	old := &model.KnowledgeEntry{
		ID: "old1", Title: "t", Content: "c", Category: "cat",
		Status: model.EntryStatusPublished, Version: 1,
		CreatedAt: sec, UpdatedAt: sec, CreatedBy: "x",
	}
	old.ContentHash = old.ComputeContentHash()
	_, err = w1.Entry.Create(context.Background(), old)
	require.NoError(t, err)
	w1.Close()

	// 重启触发迁移
	w2, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)
	defer w2.Close()
	require.NoError(t, migrateTimestampsToMillis(w2.Entry))

	got, err := w2.Entry.Get(context.Background(), "old1")
	require.NoError(t, err)
	require.Equal(t, sec*1000, got.CreatedAt, "秒级 CreatedAt 应迁移为毫秒")
	require.Equal(t, sec*1000, got.UpdatedAt, "秒级 UpdatedAt 应迁移为毫秒")
}

func TestMigrateTimestampsToMillis_Idempotent(t *testing.T) {
	dir := t.TempDir()
	w1, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)
	ms := model.NowMillis()
	alreadyMs := &model.KnowledgeEntry{
		ID: "ms1", Title: "t", Content: "c", Category: "cat",
		Status: model.EntryStatusPublished, Version: 1,
		CreatedAt: ms, UpdatedAt: ms, CreatedBy: "x",
	}
	alreadyMs.ContentHash = alreadyMs.ComputeContentHash()
	_, _ = w1.Entry.Create(context.Background(), alreadyMs)
	w1.Close()

	w2, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)
	defer w2.Close()
	require.NoError(t, migrateTimestampsToMillis(w2.Entry))
	require.NoError(t, migrateTimestampsToMillis(w2.Entry)) // 再跑一次

	got, _ := w2.Entry.Get(context.Background(), "ms1")
	require.Equal(t, ms, got.CreatedAt, "已毫秒的不应被改动（幂等）")
}

func TestMigrateTimestampsToMillis_ZeroSkipped(t *testing.T) {
	dir := t.TempDir()
	w, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)
	defer w.Close()
	zero := &model.KnowledgeEntry{
		ID: "z1", Title: "t", Content: "c", Category: "cat",
		Status: model.EntryStatusPublished, Version: 1,
		CreatedAt: 0, UpdatedAt: 0, CreatedBy: "x",
	}
	zero.ContentHash = zero.ComputeContentHash()
	_, _ = w.Entry.Create(context.Background(), zero)
	require.NoError(t, migrateTimestampsToMillis(w.Entry))
	got, _ := w.Entry.Get(context.Background(), "z1")
	require.Equal(t, int64(0), got.CreatedAt, "零值应跳过，不被 ×1000")
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./internal/storage/model/ -run TestNewKnowledgeEntry && go test ./internal/storage/ -run TestMigrateTimestamps`
Expected: FAIL（models.go:75 用 Unix() 不满足 >=1e12；`migrateTimestampsToMillis` undefined）

- [ ] **Step 4: 修 NewKnowledgeEntry**（`models.go:73-93`）

`:75` `now := time.Now().Unix()` → `now := NowMillis()`（UnixMilli）。字段注释 :56-57 `// 创建时间(Unix时间戳)` → `// 创建时间(Unix 毫秒时间戳)`。

- [ ] **Step 5: 实现 migrate.go**

```go
package storage

import (
	"context"
	"log"
)

// tsMillisThreshold 区分秒 vs 毫秒的阈值。毫秒值 ~1.7e12，秒值 ~1.7e9，阈值居中无歧义。
const tsMillisThreshold int64 = 1_000_000_000_000 // 1e12

// migrateTimestampsToMillis 把历史秒级时间戳（< 1e12）修正为毫秒（×1000）。
// 幂等：已毫秒（>=1e12）或零值（==0）的不动。
func migrateTimestampsToMillis(entryStore EntryStore) error {
	entries, _, err := entryStore.List(context.Background(), EntryFilter{Limit: 100000})
	if err != nil {
		return err
	}
	migrated := 0
	for _, e := range entries {
		changed := false
		if e.CreatedAt != 0 && e.CreatedAt < tsMillisThreshold {
			e.CreatedAt *= 1000
			changed = true
		}
		if e.UpdatedAt != 0 && e.UpdatedAt < tsMillisThreshold {
			e.UpdatedAt *= 1000
			changed = true
		}
		if changed {
			if _, err := entryStore.Update(context.Background(), e); err != nil {
				return err
			}
			migrated++
		}
	}
	if migrated > 0 {
		log.Printf("[Store] migrated %d entries' timestamps seconds→millis", migrated)
	}
	return nil
}
```

- [ ] **Step 6: NewPersistentStore 挂载迁移**（`store.go:199` 之后、`:204` titleIdx 重建之前）

```go
	entryStore := NewBadgerEntryStore(kvStore)

	// B2：迁移历史秒级时间戳为毫秒（幂等）
	if err := migrateTimestampsToMillis(entryStore); err != nil {
		return nil, fmt.Errorf("migrate timestamps: %w", err)
	}
```

若 Step 1 grep 发现 cmd 走 `NewBadgerStoreWithCloser`，则在 badger_adapter.go 的该工厂同等位置也挂迁移（或抽 `initStoreIndexes(kvStore, entryStore)` 公共函数两处复用）。

- [ ] **Step 7: 跑测试 + 全量**

Run: `go test -race -count=1 ./internal/storage/... ./internal/storage/model/`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/storage/model/models.go internal/storage/migrate.go internal/storage/migrate_test.go internal/storage/store.go
git commit -m "fix(correctness): unify timestamps to millis + idempotent migration (R2-B2)"
```

## Task B3: Scan 不再静默吞错

**Files:**
- Modify: `internal/storage/kv/badger_store.go:76`
- Modify: `internal/storage/kv/pebble_store.go:84-85`、`:98`
- Test: `internal/storage/kv/badger_store_test.go`（新建或追加）、`internal/storage/kv/pebble_store_test.go`

**Interfaces:**
- Produces: `BadgerStore.Scan`/`PebbleStore.Scan` 单键读取错误或迭代器累积错误时返回非 nil error，不静默丢弃。

- [ ] **Step 1: 写失败测试**（`badger_store_test.go`）

badger 难以注入单键 ValueCopy 错误，改为验证 Scan 在正常路径下返回的 error 类型可传播；并在 pebble 侧用 `iter.Error()` 路径。简化为「确认 Scan 返回 error 时不返回残缺结果」的契约测试 + 直接读改后代码路径。

由于注入坏 value 困难，本 task 的 TDD 落在「代码审查断言」：写一个测试确认 `Scan` 在底层 View 返回 err 时透传：
```go
func TestBadgerStore_Scan_PropagatesError(t *testing.T) {
	// 关闭 db 后 Scan 应返回非 nil error（不静默返回空 map）
	dir := t.TempDir()
	s, err := NewBadgerStore(dir)
	require.NoError(t, err)
	_ = s.Put([]byte("entry:a"), []byte("{}"))
	require.NoError(t, s.Close()) // 关库

	_, err = s.Scan([]byte("entry:"))
	// badger 关库后 Scan 会报错；断言 err != nil（之前可能返回空 map+nil）
	if err == nil {
		t.Fatal("Scan on closed store must return error, not silent empty result")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/storage/kv/ -run TestBadgerStore_Scan`
Expected: 视当前实现可能 PASS 或 FAIL；以代码审查确认 :76 仍是 `continue` 为「未修复」标志。

- [ ] **Step 3: 修 BadgerStore.Scan**（`badger_store.go:71-79`）

```go
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err // B3：不再吞错，传播给外层 View
			}
			result[string(key)] = val
		}
```

- [ ] **Step 4: 修 PebbleStore.Scan**（`pebble_store.go:74-98`）

```go
	for iter.Valid() {
		key := iter.Key()
		if !hasPrefix(key, prefix) {
			break
		}

		value, err := iter.ValueAndErr()
		if err != nil {
			return result, err // B3：不再吞错
		}

		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)

		result[string(keyCopy)] = valueCopy
		iter.Next()
	}
	// B3：检查迭代器累积错误（pebble 要求显式检查，Valid() 在底层错误时静默返回 false）
	if err := iter.Error(); err != nil {
		return result, err
	}
	return result, nil
```

- [ ] **Step 5: 跑测试 + 全量**

Run: `go test -race -count=1 ./internal/storage/kv/ ./internal/storage/...`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/storage/kv/badger_store.go internal/storage/kv/pebble_store.go internal/storage/kv/badger_store_test.go
git commit -m "fix(correctness): Scan propagates read errors instead of swallowing (R2-B3)"
```

---

# R2-C：搜索索引一致性

闭合：bleve 重启不核对（C1）、索引写错误被静默吞掉（C2）、bleve Search 不过滤 status（C3）。同根因（索引↔store 不一致）的三表现，同组修复。

## Task C1: bleve 启动 Rebuild

**Files:**
- Modify: `internal/storage/index/bleve_engine.go`（NewBleveEngine :46-128 抽 helper、新增 Rebuild）
- Modify: `internal/storage/store.go`（NewPersistentStore :212 后调 Rebuild）
- Test: `internal/storage/index/bleve_engine_test.go`

**Interfaces:**
- Produces: `BleveEngine.Rebuild(entries []*model.KnowledgeEntry) error`（Close + 删目录 + 重建 + 全量 IndexEntry）；内部抽 `buildMapping() bleve.IndexMapping` 与 `openOrCreate(indexPath, mapping)` helper。
- Consumes: `EntryStore.List`（store.go:206 模式）、bleve v2（无 ClearIndex，故 Close+RemoveAll）。

- [ ] **Step 1: 写失败测试**（`bleve_engine_test.go`，参照 TestBleveEngine_Persistence :163-200）

```go
// TestBleveEngine_Rebuild_FixesStaleIndex 验证索引陈旧/损坏时 Rebuild 后内容与给定 entries 一致。
func TestBleveEngine_Rebuild_FixesStaleIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bleve")

	// 第一次：索引一个会随后被"丢弃"的陈旧条目 + 一个 published
	e1 := &model.KnowledgeEntry{ID: "e1", Title: "alpha", Content: "alpha", Category: "c", Status: model.EntryStatusPublished, CreatedBy: "x"}
	e1.ContentHash = e1.ComputeContentHash()
	stale := &model.KnowledgeEntry{ID: "stale", Title: "beta gone", Content: "beta", Category: "c", Status: model.EntryStatusPublished, CreatedBy: "x"}
	stale.ContentHash = stale.ComputeContentHash()

	eng1, err := NewBleveEngine(path)
	require.NoError(t, err)
	require.NoError(t, eng1.IndexEntry(e1))
	require.NoError(t, eng1.IndexEntry(stale))
	require.NoError(t, eng1.Close())

	// 第二次：reopen，调 Rebuild 只喂 e1（模拟 stale 已从 store 删除）
	eng2, err := NewBleveEngine(path)
	require.NoError(t, err)
	defer eng2.Close()
	require.NoError(t, eng2.Rebuild([]*model.KnowledgeEntry{e1}))

	// 陈旧条目搜不到
	res, err := eng2.Search(context.Background(), SearchQuery{Keyword: "beta"})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount, "stale entry should be gone after rebuild")

	// e1 仍可搜
	res2, err := eng2.Search(context.Background(), SearchQuery{Keyword: "alpha"})
	require.NoError(t, err)
	assert.Equal(t, 1, res2.TotalCount, "e1 should be searchable after rebuild")

	// 自检：DocCount == entries 数
	cnt, err := eng2.IndexCount()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), cnt)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/storage/index/ -run TestBleveEngine_Rebuild`
Expected: FAIL（`Rebuild` undefined）

- [ ] **Step 3: 抽 buildMapping / openOrCreate helper**（重构 `bleve_engine.go:46-128`）

把 :51-95 的 mapping 构造抽为：
```go
func buildMapping() bleve.IndexMapping {
	// 原 :51-95 的 mapping 构造逻辑整体搬入，返回 mapping
	// ...
	return mapping
}
```
把 :101-117 的 open/create 逻辑抽为：
```go
// openOrCreate 打开已存在的索引，或用 mapping 新建。
func openOrCreate(indexPath string, mapping bleve.IndexMapping) (bleve.Index, error) {
	if bleveIndexExists(indexPath) {
		return bleve.OpenUsing(indexPath, nil)
	}
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return nil, err
	}
	return bleve.NewUsing(indexPath, mapping, upsidedown.Name, boltdb.Name, nil)
}
```
`NewBleveEngine` 简化为：
```go
func NewBleveEngine(indexPath string) (*BleveEngine, error) {
	jieba, err := newJieba() // 原 :48 逻辑
	if err != nil {
		return nil, err
	}
	idx, err := openOrCreate(indexPath, buildMapping())
	if err != nil {
		return nil, err
	}
	return &BleveEngine{index: idx, jieba: jieba}, nil
}
```

- [ ] **Step 4: 实现 Rebuild**

```go
// Rebuild 清空索引并按给定 entries 全量重建（bleve 无 ClearIndex，故 Close+删目录+重建）。
func (e *BleveEngine) Rebuild(entries []*model.KnowledgeEntry) error {
	if e.index == nil {
		return fmt.Errorf("bleve engine not initialized")
	}
	indexPath, _ := e.index.StatsMap()["indexPath"] // 若不可用，需在 NewBleveEngine 保存 indexPath 字段
	_ = e.index.Close()
	if s, ok := indexPath.(string); ok && s != "" {
		_ = os.RemoveAll(s)
		idx, err := openOrCreate(s, buildMapping())
		if err != nil {
			return err
		}
		e.index = idx
	}
	for _, entry := range entries {
		if err := e.IndexEntry(entry); err != nil {
			return fmt.Errorf("rebuild index entry %s: %w", entry.ID, err)
		}
	}
	return nil
}
```

> **注意：** bleve `Index` 接口不暴露 indexPath。需给 `BleveEngine` 加 `indexPath string` 字段，在 `NewBleveEngine` 赋值，`Rebuild` 用 `e.indexPath` 而非 StatsMap。实现时改为：
```go
type BleveEngine struct {
	index    bleve.Index
	jieba    *JiebaWrapper
	indexPath string
}
// NewBleveEngine 里：return &BleveEngine{index: idx, jieba: jieba, indexPath: indexPath}
// Rebuild 里用 e.indexPath
```

- [ ] **Step 5: NewPersistentStore 集成**（`store.go:212` 之后）

```go
	titleIdx.Build(titleEntries)
	_ = kv.SetEntryPublishedCount(kvStore, int64(len(entries)))

	// C1：启动全量 rebuild bleve 索引，保证索引↔store 一致（含历史漂移自愈）
	if be, ok := searchEngine.(*index.BleveEngine); ok {
		if err := be.Rebuild(entries); err != nil {
			log.Printf("[Store] bleve rebuild failed: %v", err)
		}
	}
```
（`entries` 已是 :206 的 published 全量切片，rebuild 后天然只含 published。）

- [ ] **Step 6: 跑测试 + 全量**

Run: `go test -race -count=1 ./internal/storage/index/ ./internal/storage/`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/storage/index/bleve_engine.go internal/storage/index/bleve_engine_test.go internal/storage/store.go
git commit -m "feat(correctness): bleve startup rebuild for index-store consistency (R2-C1)"
```

## Task C2: 索引写错误不再静默吞掉

**Files:**
- Modify: `internal/api/handler/entry_handler.go:310,436,514`
- Modify: `internal/api/handler/batch_handler.go:407,521,596`
- Modify: `internal/network/sync/sync.go:267,288`
- Modify: `internal/core/seed/initializer.go:103`
- Test: `internal/api/handler/mock_search_test.go`（新建）、handler/batch/sync 相关测试

**Interfaces:**
- Produces: 所有 `IndexEntry/UpdateIndex/DeleteIndex` 调用点检查 err 并 `log`（best-effort，不阻塞 entry 写、不改变 HTTP 状态码）。
- Consumes: 新增 `mockSearchEngine`（实现 `index.SearchEngine`，可注入 `indexErr`）。

- [ ] **Step 1: 写 mockSearchEngine**（`internal/api/handler/mock_search_test.go`）

```go
package handler

import (
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// mockSearchEngine 实现 storage.SearchEngine 用于测试，可注入错误。
type mockSearchEngine struct {
	indexErr   error
	indexCalls int
}

func (m *mockSearchEngine) IndexEntry(entry *model.KnowledgeEntry) error {
	m.indexCalls++
	return m.indexErr
}
func (m *mockSearchEngine) UpdateIndex(entry *model.KnowledgeEntry) error { return m.indexErr }
func (m *mockSearchEngine) DeleteIndex(entryID string) error              { return m.indexErr }
func (m *mockSearchEngine) Search(ctx interface{}, q interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockSearchEngine) Close() error { return nil }
```
> 实际签名须对齐 `storage.SearchEngine`/`index.SearchEngine`（`Search(ctx, SearchQuery) (*SearchResult, error)`）。把上面的占位 `interface{}` 换成真实类型（`context.Context`、`index.SearchQuery`、`*index.SearchResult`）。`storage.SearchEngine` 与 `index.SearchEngine` 若是同一接口，按 import 调整。

- [ ] **Step 2: 写失败测试**（验证 best-effort：索引失败不阻塞 entry 写）

```go
func TestEntryHandler_CreateEntry_IndexFailureDoesNotBlock(t *testing.T) {
	handler, store := newTestHandler(t) // 既有 helper
	handler.searchEngine = &mockSearchEngine{indexErr: errors.New("disk full")}

	// 走正常 create 流程（带合法签名/认证，按既有测试模式）
	// ... 构造请求、调用 CreateEntryHandler ...
	// 断言：HTTP 201（entry 写成功），索引失败被 log 不阻塞
	if rec.Code != http.StatusCreated {
		t.Fatalf("entry create must succeed despite index error; got %d", rec.Code)
	}
}
```
（具体构造按 `handler_test.go` 既有 entry create 测试模式。）

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./internal/api/handler/ -run TestEntryHandler_CreateEntry_IndexFailure`
Expected: FAIL（当前 :310 `_ =` 吞错，但若 mock 注入 err 后 entry 仍 201 则可能已 PASS；以代码审查 :310 仍 `_ =` 为未修复标志。失败点定位在"未检查 err"。）

- [ ] **Step 4: 修所有 9 处调用点**

统一模式（以 `entry_handler.go:310` 为例）：
```go
	if err := h.searchEngine.IndexEntry(created); err != nil {
		log.Printf("[EntryHandler] index entry %s failed: %v", created.ID, err)
	}
```
逐处替换：
- `entry_handler.go:310` `IndexEntry(created)`、`:436` `UpdateIndex(updated)`、`:514` `DeleteIndex(id)`
- `batch_handler.go:407` `IndexEntry(created)`、`:521` `UpdateIndex(updated)`、`:596` `DeleteIndex(id)`
- `sync.go:267` `se.store.Search.IndexEntry(&entry)` → 接 err + log（参照 :304-306 已有的 DeleteIndex 范式）
- `sync.go:288` `se.store.Search.UpdateIndex(&entry)` → 同上
- `seed/initializer.go:103` `si.store.Search.IndexEntry(&entry)` → 同上

（各文件已 import `log`，无需新增依赖。）

- [ ] **Step 5: 跑测试 + 全量**

Run: `go test -race -count=1 ./internal/api/handler/ ./internal/network/sync/ ./internal/core/seed/`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/api/handler/entry_handler.go internal/api/handler/batch_handler.go internal/api/handler/mock_search_test.go internal/network/sync/sync.go internal/core/seed/initializer.go
git commit -m "fix(correctness): surface index write errors instead of swallowing (R2-C2)"
```

## Task C3: bleve Search 过滤 status=published

**Files:**
- Modify: `internal/storage/index/bleve_engine.go`（Search :190-308，boolQuery :201 之后加 status must）
- Test: `internal/storage/index/bleve_engine_test.go`

**Interfaces:**
- Produces: `BleveEngine.Search` 在 bool query 加 `status=published` 的 TermQuery must 条件（SearchQuery 不加 status 字段，内部硬编码，与 MemorySearchEngine/BadgerSearchEngine 行为一致）。

- [ ] **Step 1: 写失败测试**（`bleve_engine_test.go`）

```go
// TestBleveEngine_SearchFiltersDraftStatus 验证 draft/archived 条目不被搜出。
func TestBleveEngine_SearchFiltersDraftStatus(t *testing.T) {
	eng, err := NewBleveEngine(filepath.Join(t.TempDir(), "t.bleve"))
	require.NoError(t, err)
	defer eng.Close()

	pub := &model.KnowledgeEntry{ID: "pub", Title: "shared keyword", Content: "x", Category: "c", Status: model.EntryStatusPublished, CreatedBy: "x"}
	pub.ContentHash = pub.ComputeContentHash()
	draft := &model.KnowledgeEntry{ID: "draft", Title: "shared keyword", Content: "x", Category: "c", Status: model.EntryStatusDraft, CreatedBy: "x"}
	draft.ContentHash = draft.ComputeContentHash()
	require.NoError(t, eng.IndexEntry(pub))
	require.NoError(t, eng.IndexEntry(draft))

	res, err := eng.Search(context.Background(), SearchQuery{Keyword: "shared"})
	require.NoError(t, err)
	assert.Equal(t, 1, res.TotalCount, "only published entry should be searchable; draft filtered")
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/storage/index/ -run TestBleveEngine_SearchFiltersDraftStatus`
Expected: FAIL（当前 Search 不过滤 status，TotalCount==2）

- [ ] **Step 3: Search 加 status must 条件**（`bleve_engine.go`，boolQuery 构造后、构建 searchRequest 前，约 :255）

```go
	// C3：只搜 published 条目（与 MemorySearchEngine/BadgerSearchEngine 行为一致）
	statusQuery := bleve.NewTermQuery(model.EntryStatusPublished)
	statusQuery.SetField("status")
	boolQuery.AddMust(statusQuery)
```
（`model.EntryStatusPublished` 值为 `"published"`，与 mapping :90 keyword 字段匹配。）

- [ ] **Step 4: 跑测试 + 全量**

Run: `go test -race -count=1 ./internal/storage/index/ ./internal/storage/`
Expected: PASS（含现有 IndexAndSearch/ChineseSearch 测试——其 fixture 都是 published，不受影响）

- [ ] **Step 5: 提交**

```bash
git add internal/storage/index/bleve_engine.go internal/storage/index/bleve_engine_test.go
git commit -m "fix(correctness): bleve Search filters status=published (R2-C3)"
```

---

# R2-D：评分 / 选举并发

闭合：双重投票竞态（D1）、评分聚合缺失（D2，spec D1 同路径的子问题——`RateEntryHandler` 当前完全不重算 entry.Score/ScoreCount）、候选人 UpdateStatus lost-update（D3）。

## Task D1: per-entry 锁防止双重投票竞态

**Files:**
- Modify: `internal/api/handler/user_handler.go`（UserHandler 加 `entryLocks` 字段、RateEntryHandler :395-482 加 `lockForEntry` 临界区）
- Test: `internal/api/handler/user_handler_test.go`

**Interfaces:**
- Produces: `(*UserHandler).lockForEntry(entryID string) *sync.Mutex`（仿 election_store.go:147 lockFor 模式）；RateEntryHandler 把 `GetByRater` 查重 + `Create` 纳入同一临界区。

- [ ] **Step 1: 写失败测试**（`user_handler_test.go`，模板见 :158-184）

```go
// TestRateEntryHandler_ConcurrentSameRaterNoDup 验证同一用户并发评分只成功一次。
func TestRateEntryHandler_ConcurrentSameRaterNoDup(t *testing.T) {
	handler, store := newTestUserHandler(t)
	user, _ := createTestUser(t, store, "rater", model.UserLevelLv1)
	createTestEntry(t, store, "e1", "Test")

	const n = 50
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		ok, dup int
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := `{"score":4.0,"comment":"x"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/e1/rate", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(setUserInContext(req.Context(), user))
			rec := httptest.NewRecorder()
			handler.RateEntryHandler(rec, req)
			mu.Lock()
			defer mu.Unlock()
			if rec.Code == http.StatusCreated {
				ok++
			} else {
				dup++
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("expected exactly 1 successful rating, got %d (race: double voting)", ok)
	}
	if dup != n-1 {
		t.Fatalf("expected %d rejections, got %d", n-1, dup)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -race -count=1 ./internal/api/handler/ -run TestRateEntryHandler_ConcurrentSameRaterNoDup`
Expected: FAIL（`ok > 1`，GetByRater 与 Create 之间无锁，并发双重投票成功）

- [ ] **Step 3: UserHandler 加 lockForEntry**（`user_handler.go:23-31` 结构体加字段 + 新增方法）

```go
type UserHandler struct {
	// ... 既有字段 ...
	entryLocks sync.Map // entryID -> *sync.Mutex，保证评分查重+写入原子
}

// lockForEntry 返回某条目的评分互斥锁（惰性创建）。
func (h *UserHandler) lockForEntry(entryID string) *sync.Mutex {
	actual, _ := h.entryLocks.LoadOrStore(entryID, &sync.Mutex{})
	return actual.(*sync.Mutex)
}
```
（构造函数 `NewUserHandler` 无需改——sync.Map 零值可用。）

- [ ] **Step 4: RateEntryHandler 加临界区**（`user_handler.go:437-463` 范围）

把 `GetByRater` 查重 + `Create` 包进 `lockForEntry(entryID)` 临界区：
```go
	h.lockForEntry(entryID).Lock()
	defer h.lockForEntry(entryID).Unlock()

	existing, _ := h.ratingStore.GetByRater(r.Context(), entryID, user.PublicKey)
	if existing != nil {
		writeError(w, awerrors.ErrDuplicateRating)
		return
	}
	// ... 构造 rating ...
	created, err := h.ratingStore.Create(r.Context(), rating)
	// ... 后续（D2 在此加重算）...
```
（确保 `lockForEntry` 的 Lock/Unlock 包住 GetByRater→Create 整段；用 defer Unlock 或显式 Unlock。注意 entryID 在 :429 已从路径解析。）

- [ ] **Step 5: 跑测试 + race**

Run: `go test -race -count=1 ./internal/api/handler/ -run TestRateEntryHandler_ConcurrentSameRaterNoDup`
Expected: PASS（ok==1）

- [ ] **Step 6: 提交**

```bash
git add internal/api/handler/user_handler.go internal/api/handler/user_handler_test.go
git commit -m "fix(correctness): per-entry lock prevents double-voting race (R2-D1)"
```

## Task D2: 评分聚合重算（新增，加权口径）

**Files:**
- Modify: `internal/api/handler/user_handler.go`（RateEntryHandler 临界区在 Create 后加重算 + `entryStore.Update`）
- Test: `internal/api/handler/user_handler_test.go`

**Interfaces:**
- Produces: RateEntryHandler 在评分后重算 `entry.Score`（加权平均 Σ WeightedScore / Σ Weight）与 `ScoreCount`，并 `entryStore.Update` 落库——修复"评分后 entry.Score 永远陈旧"。口径与 `kv.RatingStore.ComputeEntryScore`（rating_store.go:143-168）一致，**不**用 sync.go:342-356 的简单平均。

- [ ] **Step 1: 写失败测试**（`user_handler_test.go`）

```go
// TestRateEntryHandler_UpdatesEntryScore 验证评分后 entry.Score/ScoreCount 被重算并持久化。
func TestRateEntryHandler_UpdatesEntryScore(t *testing.T) {
	handler, store := newTestUserHandler(t)
	user, _ := createTestUser(t, store, "rater", model.UserLevelLv1)
	createTestEntry(t, store, "e1", "Test")

	body := `{"score":4.0,"comment":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/e1/rate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(setUserInContext(req.Context(), user))
	rec := httptest.NewRecorder()
	handler.RateEntryHandler(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// 评分后 entry.ScoreCount 应为 1，Score 应为该评分（weight=GetLevelWeight(Lv1)）
	updated, err := store.Entry.Get(context.Background(), "e1")
	require.NoError(t, err)
	assert.Equal(t, int32(1), updated.ScoreCount, "ScoreCount must be recomputed after rating")
	assert.Greater(t, updated.Score, 0.0, "Score must be recomputed (not stale 0)")
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/api/handler/ -run TestRateEntryHandler_UpdatesEntryScore`
Expected: FAIL（当前 RateEntryHandler 不重算，`updated.ScoreCount == 0`）

- [ ] **Step 3: 临界区在 Create 后加重算**（`user_handler.go`，D1 临界区内 Create 之后）

```go
	created, err := h.ratingStore.Create(r.Context(), rating)
	if err != nil {
		writeError(w, awerrors.Wrap(...))
		return
	}

	// D2：重算 entry.Score（加权平均）与 ScoreCount，落库
	if entry, gerr := h.entryStore.Get(r.Context(), entryID); gerr == nil && entry != nil {
		ratings, rerr := h.ratingStore.ListByEntry(r.Context(), entryID)
		if rerr == nil && len(ratings) > 0 {
			var sumW, sumWS float64
			for _, rt := range ratings {
				sumW += rt.Weight
				sumWS += rt.WeightedScore
			}
			if sumW > 0 {
				entry.Score = sumWS / sumW
			}
			entry.ScoreCount = int32(len(ratings))
			if _, uerr := h.entryStore.Update(r.Context(), entry); uerr != nil {
				log.Printf("[UserHandler] recompute entry %s score failed: %v", entryID, uerr)
			}
		}
	}
```
（`log` 已 import；`rating.Weight`/`WeightedScore` 在 Rating 结构体已有。）

- [ ] **Step 4: 跑测试 + race + 全量**

Run: `go test -race -count=1 ./internal/api/handler/ ./internal/storage/...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/api/handler/user_handler.go internal/api/handler/user_handler_test.go
git commit -m "feat(correctness): recompute entry score after rating (weighted) (R2-D2)"
```

## Task D3: 候选人 UpdateStatus 复用 lockFor

**Files:**
- Modify: `internal/storage/kv/election_store.go:166-173`（UpdateStatus 加锁）
- Test: `internal/storage/kv/election_atomic_test.go`

**Interfaces:**
- Produces: `KVCandidateStore.UpdateStatus` 复用 `lockFor(electionID)`（与 `UpdateVoteCount` 同把锁），串行化 read-modify-write。

- [ ] **Step 1: 写失败测试**（扩展 `election_atomic_test.go`，模板见 :23-52 的 100-goroutine 模式）

```go
// TestKVCandidateStore_UpdateStatus_ConcurrentWithVoteCount 验证并发 UpdateVoteCount + UpdateStatus 不丢更新。
func TestKVCandidateStore_UpdateStatus_ConcurrentWithVoteCount(t *testing.T) {
	store := NewMemoryStore()
	cs := NewKVCandidateStore(store)
	require.NoError(t, cs.Add(context.Background(), &model.Candidate{ElectionID: "e1", UserID: "u1", VoteCount: 0}))

	const n = 100
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_ = cs.UpdateVoteCount(context.Background(), "e1", "u1", 1)
			} else {
				_ = cs.UpdateStatus(context.Background(), "e1", "u1", model.CandidateStatusElected)
			}
		}(i)
	}
	wg.Wait()

	got, err := cs.Get(context.Background(), "e1", "u1")
	require.NoError(t, err)
	if got.VoteCount != int32(n/2) {
		t.Fatalf("VoteCount lost update: got=%d want=%d", got.VoteCount, int32(n/2))
	}
	if got.Status != model.CandidateStatusElected {
		t.Fatalf("Status lost update: got=%v want=Elected", got.Status)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -race -count=1 ./internal/storage/kv/ -run TestKVCandidateStore_UpdateStatus_Concurrent`
Expected: FAIL（当前 UpdateStatus 无锁，并发与 UpdateVoteCount 交错时 lost-update；-race 也可能报）

- [ ] **Step 3: UpdateStatus 加锁**（`election_store.go:166-173`）

```go
func (s *KVCandidateStore) UpdateStatus(ctx context.Context, electionID, userID string, status model.CandidateStatus) error {
	s.lockFor(electionID).Lock()
	defer s.lockFor(electionID).Unlock()

	candidate, err := s.Get(ctx, electionID, userID)
	if err != nil {
		return err
	}
	candidate.Status = status
	return s.Add(ctx, candidate)
}
```

- [ ] **Step 4: 跑测试 + race + 全量**

Run: `go test -race -count=1 ./internal/storage/kv/ ./internal/core/election/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/storage/kv/election_store.go internal/storage/kv/election_atomic_test.go
git commit -m "fix(correctness): candidate UpdateStatus reuses lockFor (no lost-update) (R2-D3)"
```

---

# R2-E：网络死代码清理

## Task E1: 删除 DHT seed / SeedPeers 死代码

**Files:**
- Modify: `internal/network/dht/dht.go:52-69, :75`（删 peers 解析 + 误导日志）
- Modify: `internal/network/host/host.go:61-62`（删 HostConfig.SeedPeers 字段）
- Modify: `cmd/user/main.go:277`、`cmd/seed/main.go:305`（删 dead write）
- Modify: `internal/network/dht/dht_test.go:118-143`（删 TestDHTNodeBootstrapWithSeedNodes）

**Interfaces:**
- Produces: 无（纯删除）。app 层 `ConnectToPeer` 循环是唯一 dial 真相，不受影响。

- [ ] **Step 1: 确认删字段无外部消费**

Run: `grep -rn "SeedPeers" --include="*.go" . | grep -v _test.go`
Expected: 仅 `host.go:61-62`（声明）+ `cmd/user/main.go:277` + `cmd/seed/main.go:305`（两处 dead write）。测试零命中。

- [ ] **Step 2: 删 dht.go 死代码**（`:52-69` peers 解析块 + `:75` 误导日志）

Bootstrap（:48-77）简化为：
```go
func (d *DHTNode) Bootstrap(ctx context.Context) error {
	d.log.Info("Bootstrapping DHT...")
	if err := d.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("DHT bootstrap failed: %w", err)
	}
	d.log.Info("DHT bootstrap completed")
	return nil
}
```
（删 `peers := dht.GetDefaultBootstrapPeerAddrInfos()`、SeedNodes 解析循环、`len(peers)` 日志。保留 `d.dht.Bootstrap(ctx)` 真实工作。）

- [ ] **Step 3: 删 host.go SeedPeers 字段**（`:61-62` 两行）

- [ ] **Step 4: 删两处 dead write**（`cmd/user/main.go:277`、`cmd/seed/main.go:305` 的 `SeedPeers: ...` 行）

- [ ] **Step 5: 删死代码测试**（`dht_test.go:118-143` 整个 `TestDHTNodeBootstrapWithSeedNodes`）

（该测试传入无效 SeedNodes 地址仅断言"不 panic"，测的是被删的死代码容错路径。保留 `TestDHTNodeBootstrap` :98。）

- [ ] **Step 6: 编译 + vet + 测试**

Run: `go build ./... && go vet ./... && go test -race -count=1 ./internal/network/dht/ ./internal/network/host/ ./cmd/...`
Expected: PASS（连通性靠 app 层，不受影响）

- [ ] **Step 7: 提交**

```bash
git add internal/network/dht/dht.go internal/network/dht/dht_test.go internal/network/host/host.go cmd/user/main.go cmd/seed/main.go
git commit -m "refactor(correctness): remove misleading DHT seed / SeedPeers dead code (R2-E1)"
```

---

# 验收（R2 收尾）

- [ ] `go build ./cmd/... ./internal/... ./pkg/...` 绿。
- [ ] `go vet ./...` 绿。
- [ ] `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` 全绿。
- [ ] golangci-lint 仍绿（`golangci-lint run --timeout=10m ./...` exit 0）—— 注意 D1 的 `entryLocks sync.Map`、D2 的字段不要引入新的 unused 警告。
- [ ] 手动核对（由自动化测试覆盖）：
  - 并发 IncrementalSync + HandleSyncRequest 不崩（A1）。
  - Badger 后端 update Version 恰好 +1（B1）。
  - 历史秒值条目迁移后 LWW 仲裁正确（B2）。
  - Scan 在错误下返 error（B3）。
  - 索引损坏重启后搜得全部 published（C1）。
  - 镜像数据接收端落库（A3）。
  - 并发评分不双计、entry.Score 被重算（D1/D2）。
  - 候选人并发状态不丢（D3）。
- [ ] R2 合并到 master 后开 R3 cycle（坏功能修复：admin SPA、统计、空壳审核等）。

# 自审清单（writing-plans）

- **Spec 覆盖**：spec A1→plan A1；A2→A2；A3→A3；B1→B1；B2→B2；B3→B3；C1→C1；C2→C2；C3→C3；spec D1→plan D1+D2（侦察发现重算缺失，拆为查重竞态与聚合重算两个独立 task）；spec D2→plan D3；spec E1→plan E1。全覆盖。
- **类型一致**：`NewVersionVector`/`*VersionVector` 方法集（Increment/Get/Set/Merge/Clone/ToProto/Delete/Range）在 A1 定义、A3 HandleMirrorData 与各调用点使用一致；`HandleMirrorData(ctx, *MirrorData) error` 在 A3 接口与实现一致；`lockForEntry`（UserHandler，D1）与 `lockFor`（KVCandidateStore，D3）区分明确（前者 per-entry、后者 per-election）；`Rebuild([]*KnowledgeEntry) error`（C1）定义与 NewPersistentStore 调用一致。
- **占位符**：无 TBD/TODO。B2 迁移挂载点给了 grep 确认步骤 + 两处挂载的应对方案，非占位符。C1 bleve indexPath 字段给出了明确实现（加 `indexPath string` 字段）。C2 mockSearchEngine 的 Search 方法签名提示"按真实接口对齐"，因接口签名已在 Consumes 标注，非占位符。
- **TDD 顺序**：每 task 先写失败测试（含完整测试代码）→ 跑确认失败 → 实现（完整代码）→ 跑通过 → 提交（含 commit message）。
- **风险已标**：A1 se.mu 职责（不嵌套锁，安全）；B1 现有 store_test.go:285 断言需同步改；B2 CreatedAt==0 跳过 + 挂载点 grep；C1 bleve 无 ClearIndex（Close+RemoveAll）；C2 mockSearchEngine 接口对齐；D1 MemoryRatingStore 无去重（handler 层 GetByRater 查重用 Memory 可测）；D2 加权口径（不用 sync 简单平均）；E1 仅删死代码无行为变化。
