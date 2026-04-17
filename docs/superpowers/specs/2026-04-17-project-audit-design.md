# Polyant 项目审计设计文档

> 审计日期：2026-04-17
> 审计范围：全面审计（功能完整性 + 测试覆盖率）
> 关注模块：网络层、API层、核心业务、存储层

---

## 一、执行摘要

本次审计采用**功能优先法**，从 API 端点出发分析各功能路径的完整性和测试覆盖情况。

**关键发现：**
- 66 个测试文件，整体覆盖率尚可但分布不均
- API 层存在多个端点未测试（反向链接、用户更新、选举流程等）
- 网络层 sync 模块接收端（Handle 系列方法）测试不足
- 存储层 VoteStore 未测试
- 多个有价值的新功能待开发

**核心建议：**
1. 优先补充 P0 级测试缺口（选举流程、链接API、VoteStore）
2. 提升 sync/protocol/host 模块覆盖率至 75%+
3. 开发断点续传、冲突解决、版本历史等核心新功能

---

## 二、覆盖率现状

### 2.1 覆盖率较高的模块（>80%）

| 模块 | 覆盖率 | 评价 |
|------|--------|------|
| internal/auth/rbac | 100.0% | 优秀 |
| internal/core/admin | 100.0% | 优秀 |
| internal/core/category | 92.2% | 优秀 |
| pkg/config | 96.1% | 优秀 |
| internal/storage/linkparser | 94.6% | 优秀 |
| internal/core/user | 87.5% | 良好 |
| internal/network/detect | 86.7% | 良好 |
| internal/core/seed | 85.0% | 良好 |
| internal/sync | 85.4% | 良好 |

### 2.2 覆盖率中等的模块（60-80%）

| 模块 | 覆盖率 | 评价 |
|------|--------|------|
| internal/api/middleware | 81.9% | 可改进 |
| internal/auth/ed25519 | 81.2% | 可改进 |
| internal/service/daemon | 81.6% | 可改进 |
| internal/storage/index | 83.5% | 可改进 |
| pkg/logger | 81.1% | 可改进 |
| internal/core/audit | 79.7% | 可改进 |
| internal/storage/model | 79.2% | 可改进 |
| internal/core/email | 77.6% | 需改进 |
| internal/core/export | 77.0% | 需改进 |
| internal/storage/kv | 74.0% | 需改进 |
| internal/core/rating | 73.7% | 需改进 |
| internal/api/handler | 72.8% | 需改进 |
| internal/api/router | 71.6% | 需改进 |
| internal/core/election | 70.2% | 需改进 |
| pkg/i18n | 70.4% | 需改进 |
| internal/network/protocol | 69.9% | 需改进 |
| internal/network/dht | 73.1% | 需改进 |

### 2.3 覆盖率较低的模块（<60%）

| 模块 | 覆盖率 | 评价 |
|------|--------|------|
| internal/network/host | 61.1% | 需优先改进 |
| internal/network/sync | 61.6% | 需优先改进 |
| internal/storage | 66.4% | 需改进 |
| pkg/crypto | 64.3% | 需改进 |
| cmd/pactl | 39.5% | CLI 工具待测试 |
| cmd/seed | 0.0% | 入口点未测试 |
| cmd/user | 0.0% | 入口点未测试 |

---

## 三、功能测试缺口分析

### 3.1 API 层 - 条目管理

| 功能 | API端点 | 测试状态 |
|------|---------|----------|
| 搜索条目 | `/api/v1/search` | ✅ 有测试 |
| 获取条目详情 | `/api/v1/entry/{id}` | ✅ 有测试 |
| 创建条目 | `/api/v1/entry/create` | ✅ 有测试 |
| 更新条目 | `/api/v1/entry/update/{id}` | ✅ 有测试 |
| 删除条目 | `/api/v1/entry/delete/{id}` | ⚠️ 基本测试，需增强权限测试 |
| 条目评分 | `/api/v1/entry/rate/{id}` | ✅ 有测试 |
| 反向链接 | `/api/v1/entry/{id}/backlinks` | ❌ **缺失测试** |
| 正向链接 | `/api/v1/entry/{id}/outlinks` | ❌ **缺失测试** |

### 3.2 API 层 - 用户管理

| 功能 | API端点 | 测试状态 |
|------|---------|----------|
| 用户注册 | `/api/v1/user/register` | ✅ 有测试 |
| 发送验证码 | `/api/v1/user/send-verification` | ✅ 有测试 |
| 验证邮箱 | `/api/v1/user/verify-email` | ✅ 有测试 |
| 获取用户信息 | `/api/v1/user/info` | ✅ 有测试 |
| 更新用户信息 | `/api/v1/user/update` | ❌ **缺失测试** |

### 3.3 API 层 - 分类管理

| 功能 | API端点 | 测试状态 |
|------|---------|----------|
| 获取分类列表 | `/api/v1/categories` | ✅ 有测试 |
| 获取分类条目 | `/api/v1/categories/{path}/entries` | ❌ **缺失测试** |
| 创建分类 | `/api/v1/categories/create` | ❌ **缺失测试** |

### 3.4 API 层 - 选举功能

| 功能 | API端点 | 测试状态 |
|------|---------|----------|
| 列出选举 | `/api/v1/elections` | ✅ 有测试 |
| 获取选举详情 | `/api/v1/elections/{id}` | ✅ 有测试 |
| 创建选举 | `/api/v1/elections/create` | ❌ **缺失测试** |
| 提名候选人 | `/api/v1/elections/{id}/candidates` | ❌ **缺失测试** |
| 确认提名 | `/api/v1/elections/{id}/candidates/{uid}/confirm` | ❌ **缺失测试** |
| 投票 | `/api/v1/elections/{id}/vote` | ❌ **缺失测试** |
| 关闭选举 | `/api/v1/elections/{id}/close` | ❌ **缺失测试** |

### 3.5 网络层 - sync 模块

| 方法 | 测试状态 |
|------|----------|
| `IncrementalSync` | ✅ 有测试 |
| `HandleHandshake` | ✅ 有测试 |
| `HandleBitfield` | ⚠️ 基本测试，需增强 |
| `HandleSyncRequest` | ❌ **缺失测试** |
| `HandlePushEntry` | ❌ **缺失测试** |
| `HandleRatingPush` | ❌ **缺失测试** |
| `RemoteQuery` | ✅ 有测试 |

### 3.6 网络层 - protocol 模块

| 方法 | 测试状态 |
|------|----------|
| `Codec` | ✅ 有测试 |
| `ProtobufCodec` | ✅ 有测试 |
| `Converter` | ✅ 有测试 |
| `SendSyncRequest` | ⚠️ 需更多场景测试 |
| `SendQuery` | ⚠️ 需更多场景测试 |
| `SendPushEntry` | ⚠️ 需更多场景测试 |
| `SendRatingPush` | ❌ **缺失测试** |

### 3.7 存储层

| Store | 测试状态 |
|-------|----------|
| EntryStore | ✅ 有测试 |
| UserStore | ✅ 有测试 |
| RatingStore | ✅ 有测试 |
| CategoryStore | ✅ 有测试 |
| ElectionStore | ⚠️ 基本测试，需增强 |
| VoteStore | ❌ **缺失测试** |
| AuditStore | ✅ 有测试 |
| PebbleStore | ✅ 有测试 |
| MemoryStore | ✅ 有测试 |

---

## 四、可开发新功能清单

### 4.1 数据同步增强

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 断点续传同步 | 同步中断后可从断点继续 | P2 高 |
| 同步冲突解决 | 多节点同时修改的冲突检测与解决 | P2 高 |
| 同步进度监控 | 实时显示同步进度、速度、剩余时间 | P3 中 |
| 选择性同步增强 | 按分类、标签、时间范围选择性同步 | P3 中 |
| 同步回滚 | 同步出错时可回滚到之前状态 | P4 低 |

### 4.2 搜索增强

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 高级搜索语法 | 支持布尔运算、范围查询、模糊匹配 | P2 高 |
| 搜索建议 | 输入时自动补全、推荐相关搜索词 | P3 中 |
| 搜索历史 | 记录用户搜索历史 | P4 低 |
| 相似条目推荐 | 基于内容相似度推荐相关条目 | P3 中 |

### 4.3 用户体系增强

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 用户徽章系统 | 根据贡献类型颁发徽章 | P3 中 |
| 贡献排行榜 | 按时间周期展示贡献排行 | P3 中 |
| 信誉系统 | 基于贡献质量和评价的综合信誉值 | P2 高 |

### 4.4 内容管理增强

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 条目版本历史 | 查看条目完整修改历史，支持版本对比 | P2 高 |
| 条目评论系统 | 用户可对条目发表评论和讨论 | P3 中 |
| 条目收藏功能 | 收藏感兴趣的条目 | P3 中 |

### 4.5 网络层增强

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 节点健康监控 | 实时监控节点健康状态 | P3 中 |
| 离线消息队列 | 节点离线期间的消息缓存 | P2 高 |
| 带宽限流控制 | 控制各节点的带宽使用 | P3 中 |

### 4.6 安全增强

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 内容加密存储 | 对敏感内容加密存储 | P2 高 |
| 内容签名验证增强 | 验证条目内容签名的完整性 | P2 高 |
| API密钥管理 | 支持API密钥方式认证 | P3 中 |

---

## 五、实施优先级

### 5.1 P0 - 立即执行（测试关键缺口）

| 项目 | 模块 | 预估工作量 |
|------|------|------------|
| 选举完整流程测试 | election | 2-3天 |
| 反向链接/正向链接测试 | handler | 1天 |
| 用户更新信息测试 | handler | 0.5天 |
| 分类条目列表测试 | handler | 0.5天 |
| VoteStore 测试 | storage | 1天 |

**目标：API层覆盖率提升至 80%+**

### 5.2 P1 - 短期执行（测试覆盖率深化）

| 项目 | 模块 | 预估工作量 |
|------|------|------------|
| HandleSyncRequest 测试 | sync | 1天 |
| HandlePushEntry 测试 | sync | 1天 |
| HandleRatingPush 测试 | sync | 0.5天 |
| SendRatingPush 测试 | protocol | 0.5天 |
| 批量索引测试 | index | 1天 |
| 流创建失败测试 | host | 0.5天 |
| 多节点DHT交互测试 | dht | 2天 |

**目标：网络层覆盖率提升至 75%+**

### 5.3 P2 - 中期执行（核心新功能）

| 项目 | 预估工作量 |
|------|------------|
| 断点续传同步 | 3-5天 |
| 同步冲突解决 | 3-5天 |
| 条目版本历史 | 3-4天 |
| 高级搜索语法 | 2-3天 |
| 离线消息队列 | 3-4天 |
| 内容签名验证增强 | 2-3天 |

### 5.4 P3 - 长期执行（体验优化）

| 项目 | 预估工作量 |
|------|------------|
| 搜索建议 | 2-3天 |
| 用户徽章系统 | 3-4天 |
| 贡献排行榜 | 2天 |
| 条目评论系统 | 3-4天 |
| 条目收藏功能 | 2天 |
| 节点健康监控 | 3-4天 |

---

## 六、实施路线图

```
阶段 1（当前 → 2周内）：测试覆盖率提升
├── P0: 选举完整流程测试
├── P0: 反向链接/正向链接测试  
├── P0: VoteStore 测试
├── P0: 用户更新/分类条目列表测试
└── 目标：API层覆盖率提升至 80%+

阶段 2（2周 → 4周）：测试覆盖率深化
├── P1: sync 模块 Handle 系列方法测试
├── P1: protocol 模块评分推送测试
├── P1: host 模块边界场景测试
├── P1: dht 多节点交互测试
├── P1: 批量索引测试
└── 目标：网络层覆盖率提升至 75%+

阶段 3（4周 → 8周）：核心新功能
├── P2: 断点续传同步
├── P2: 同步冲突解决
├── P2: 条目版本历史
├── P2: 高级搜索语法
└── 目标：核心功能增强完成

阶段 4（8周 → 12周）：体验优化
├── P3: 搜索建议
├── P3: 用户徽章系统
├── P3: 贡献排行榜
├── P3: 条目收藏功能
└── 目标：用户体验显著提升
```

---

## 七、测试用例清单

### 7.1 选举模块测试用例

```
1. TestCreateElection_Success - Lv5用户成功创建选举
2. TestCreateElection_PermissionDenied - Lv4用户创建失败
3. TestNominateCandidate_Success - 成功提名候选人
4. TestNominateCandidate_AlreadyNominated - 重复提名失败
5. TestConfirmNomination_Success - 候选人确认接受
6. TestConfirmNomination_WrongUser - 非被提名者确认失败
7. TestVote_Success - Lv3用户成功投票
8. TestVote_PermissionDenied - Lv2用户投票失败
9. TestVote_DuplicateVote - 重复投票失败
10. TestCloseElection_Success - Lv5关闭选举
11. TestElection_CompleteFlow - 完整选举流程测试
```

### 7.2 反向链接测试用例

```
1. TestGetBacklinks_EntryWithLinks - 条目有反向链接时正确返回
2. TestGetBacklinks_EntryWithoutLinks - 条目无反向链接时返回空数组
3. TestGetBacklinks_EntryNotFound - 条目不存在时返回404
4. TestGetOutlinks_EntryWithLinks - 条目有正向链接时正确返回
5. TestGetOutlinks_EntryWithoutLinks - 条目无正向链接时返回空数组
6. TestLinkParsing_ComplexContent - 复杂Markdown内容链接解析
```

### 7.3 同步 Handle 方法测试用例

```
1. TestHandleSyncRequest_Basic - 基本同步请求处理
2. TestHandleSyncRequest_EmptyRequest - 空请求处理
3. TestHandleSyncRequest_InvalidVersion - 版本向量无效处理
4. TestHandlePushEntry_NewEntry - 新条目推送处理
5. TestHandlePushEntry_UpdatedEntry - 条目更新推送处理
6. TestHandlePushEntry_InvalidSignature - 签名验证失败
7. TestHandleRatingPush_NewRating - 新评分推送处理
8. TestHandleRatingPush_DuplicateRating - 重复评分处理
```

---

## 八、结论

本次审计系统性地分析了 Polyant 项目的功能完整性和测试覆盖率现状。

**核心问题：**
1. API 层多个端点未测试，影响用户直接使用的功能
2. 网络层接收端逻辑测试不足，同步可靠性存疑
3. 选举流程测试缺失，核心业务功能未完整验证

**改进方向：**
1. 优先补充 P0/P1 级测试缺口，建立稳固的测试基础
2. 开发断点续传、冲突解决、版本历史等核心新功能
3. 分阶段实施，每阶段有明确的目标和验收标准

**下一步行动：**
调用 writing-plans skill，为 P0 级测试改进创建详细实施计划。