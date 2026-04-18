# 测试覆盖率提升设计文档

> 设计日期：2026-04-18
> 目标：将所有 <75% 的模块提升至 75%+ 覆盖率
> 方法：并行开发，每个模块独立任务

---

## 一、执行摘要

本次设计旨在提升 Polyant 项目各模块的测试覆盖率，确保代码质量和可维护性。

**当前状态：**
- 7 个模块覆盖率低于 75%
- 最低覆盖率：host (61.1%), admin (63.0%), storage (66.4%)

**目标状态：**
- 所有模块覆盖率 ≥ 75%
- 新增约 50+ 测试用例
- 覆盖主要功能路径和边界条件

---

## 二、覆盖率现状

### 2.1 需要提升的模块

| 模块 | 当前覆盖率 | 目标覆盖率 | 优先级 |
|------|------------|------------|--------|
| `internal/network/host` | 61.1% | 75%+ | P0 |
| `internal/api/admin` | 63.0% | 75%+ | P0 |
| `internal/storage` | 66.4% | 75%+ | P0 |
| `internal/network/sync` | 69.4% | 75%+ | P1 |
| `internal/network/protocol` | 69.9% | 75%+ | P1 |
| `internal/api/router` | 71.6% | 75%+ | P1 |
| `internal/network/dht` | 73.1% | 75%+ | P1 |

### 2.2 已达标模块

| 模块 | 覆盖率 | 状态 |
|------|--------|------|
| `internal/auth/rbac` | 100.0% | 优秀 |
| `internal/core/admin` | 100.0% | 优秀 |
| `internal/core/category` | 92.2% | 优秀 |
| `pkg/config` | 96.1% | 优秀 |
| `internal/storage/linkparser` | 94.6% | 优秀 |
| `internal/core/user` | 87.5% | 良好 |
| `internal/network/detect` | 86.7% | 良好 |

---

## 三、未覆盖代码分析

### 3.1 internal/network/host (61.1%)

**未覆盖函数：**

| 函数 | 覆盖率 | 说明 |
|------|--------|------|
| `MockHost.*` | 0% | 测试替身，需要基本测试 |
| `connectToRelays` | 0% | 中继连接逻辑 |
| `detectNATType` | 23.1% | NAT 检测逻辑 |
| `AllowReserve` | 0% | 中继资源预留 |
| `AllowConnect` | 0% | 中继连接控制 |

### 3.2 internal/api/admin (63.0%)

**未覆盖函数：**

| 函数 | 覆盖率 | 说明 |
|------|--------|------|
| `NewHandler` | 0% | Handler 构造函数 |
| `ListUsersHandler` | 0% | 用户列表 API |
| `BanUserHandler` | 0% | 封禁用户 API |
| `UnbanUserHandler` | 0% | 解封用户 API |
| `SetUserLevelHandler` | 0% | 设置用户级别 API |
| `GetUserStatsHandler` | 0% | 用户统计 API |
| `GetContributionStatsHandler` | 0% | 贡献统计 API |
| `GetActivityTrendHandler` | 0% | 活动趋势 API |
| `GetRegistrationTrendHandler` | 0% | 注册趋势 API |
| `StaticHandler.*` | 0% | 静态文件服务 |

### 3.3 internal/storage (66.4%)

**未覆盖函数：**

| 函数 | 覆盖率 | 说明 |
|------|--------|------|
| `NewBadgerBacklinkIndex` | 0% | 创建反向链接索引 |
| `UpdateIndex` | 0% | 更新索引 |
| `DeleteIndex` | 0% | 删除索引 |
| `GetBacklinks` | 0% | 获取反向链接 |
| `GetOutlinks` | 0% | 获取正向链接 |
| `BadgerRatingStore.Get` | 0% | 获取评分 |

---

## 四、测试用例设计

### 4.1 internal/network/host

```go
// MockHost 测试
func TestNewMockP2PHost(t *testing.T)
func TestMockP2PHost_ID(t *testing.T)
func TestMockP2PHost_NodeID(t *testing.T)
func TestMockP2PHost_GetConnectedPeers(t *testing.T)
func TestMockP2PHost_Connect(t *testing.T)
func TestMockP2PHost_NewStream(t *testing.T)
func TestMockP2PHost_Close(t *testing.T)
func TestMockP2PHost_Reset(t *testing.T)
func TestMockP2PHost_SetConnectedPeers(t *testing.T)

// Host 边界测试
func TestConnectToRelays_NoRelays(t *testing.T)
func TestConnectToRelays_ConnectionError(t *testing.T)
func TestDetectNATType_InvalidAddress(t *testing.T)
func TestAllowReserve_Success(t *testing.T)
func TestAllowConnect_Success(t *testing.T)
```

### 4.2 internal/api/admin

```go
// Handler 测试
func TestListUsersHandler_Success(t *testing.T)
func TestListUsersHandler_Empty(t *testing.T)
func TestListUsersHandler_WithPagination(t *testing.T)
func TestBanUserHandler_Success(t *testing.T)
func TestBanUserHandler_NotFound(t *testing.T)
func TestBanUserHandler_AlreadyBanned(t *testing.T)
func TestUnbanUserHandler_Success(t *testing.T)
func TestUnbanUserHandler_NotFound(t *testing.T)
func TestSetUserLevelHandler_Success(t *testing.T)
func TestSetUserLevelHandler_InvalidLevel(t *testing.T)
func TestGetUserStatsHandler_Success(t *testing.T)
func TestGetContributionStatsHandler_Success(t *testing.T)
func TestGetActivityTrendHandler_Success(t *testing.T)
func TestGetRegistrationTrendHandler_Success(t *testing.T)

// StaticHandler 测试
func TestStaticHandler_ServeHTTP(t *testing.T)
func TestStaticHandler_FileNotFound(t *testing.T)
```

### 4.3 internal/storage

```go
// BacklinkIndex 测试
func TestNewBadgerBacklinkIndex(t *testing.T)
func TestUpdateIndex_NewEntry(t *testing.T)
func TestUpdateIndex_ExistingEntry(t *testing.T)
func TestUpdateIndex_ComplexContent(t *testing.T)
func TestDeleteIndex(t *testing.T)
func TestDeleteIndex_NotFound(t *testing.T)
func TestGetBacklinks_Success(t *testing.T)
func TestGetBacklinks_Empty(t *testing.T)
func TestGetOutlinks_Success(t *testing.T)
func TestGetOutlinks_Empty(t *testing.T)

// 错误路径测试
func TestBadgerEntryStore_CreateError(t *testing.T)
func TestBadgerUserStore_UpdateError(t *testing.T)
func TestBadgerRatingStore_Get_Success(t *testing.T)
func TestBadgerRatingStore_Get_NotFound(t *testing.T)
```

### 4.4 其他模块

**internal/network/protocol (69.9% → 75%+)**
- 预估增加 5-10 个测试用例
- 主要补充错误路径和边界条件测试

**internal/network/sync (69.4% → 75%+)**
- 预估增加 5-10 个测试用例
- 主要补充并发场景和边界条件测试

**internal/api/router (71.6% → 75%+)**
- 预估增加 3-5 个测试用例
- 主要补充路由注册和中间件链测试

**internal/network/dht (73.1% → 75%+)**
- 预估增加 3-5 个测试用例
- 主要补充多节点 DHT 交互测试

---

## 五、实施计划

### 5.1 任务分配

| 任务 | 模块 | 预估工作量 | 优先级 |
|------|------|------------|--------|
| Task 1 | internal/network/host | 1天 | P0 |
| Task 2 | internal/api/admin | 1天 | P0 |
| Task 3 | internal/storage | 0.5天 | P0 |
| Task 4 | internal/network/protocol | 0.5天 | P1 |
| Task 5 | internal/network/sync | 0.5天 | P1 |
| Task 6 | internal/api/router | 0.5天 | P1 |
| Task 7 | internal/network/dht | 0.5天 | P1 |

**总预估工作量：** 约 4.5 天

### 5.2 实施顺序

```
阶段 1（P0）：基础模块测试
├── Task 1: host 模块测试（1天）
├── Task 2: admin 模块测试（1天）
└── Task 3: storage 模块测试（0.5天）

阶段 2（P1）：覆盖率深化
├── Task 4: protocol 模块测试（0.5天）
├── Task 5: sync 模块测试（0.5天）
├── Task 6: router 模块测试（0.5天）
└── Task 7: dht 模块测试（0.5天）
```

### 5.3 验收标准

- 所有模块覆盖率 ≥ 75%
- 所有新增测试通过
- 无测试失败或跳过
- 代码已提交到 git

---

## 六、风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| MockHost 测试可能需要重构 | 中 | 保持测试简单，只验证基本行为 |
| admin Handler 测试需要数据库依赖 | 中 | 使用 MemoryStore 进行测试 |
| DHT 测试可能需要网络环境 | 中 | 使用 mock 或集成测试环境 |

---

## 七、下一步行动

1. 调用 writing-plans skill 创建详细实施计划
2. 为每个模块创建独立任务
3. 按优先级顺序实施测试改进
