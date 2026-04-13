# Phase 6c: 用户体系完善设计

**版本**: v2.1.0
**日期**: 2026-04-13
**状态**: 已批准

## 概述

完善 AgentWiki 用户体系，添加投票选举系统和用户管理功能，实现 Lv4 → Lv5 的民主升级机制，以及管理员对用户的管理能力。

## 目标

| 功能 | 描述 |
|------|------|
| 投票选举系统 | Lv4 用户可被提名，Lv3+ 用户投票，票数达到阈值当选 Lv5 |
| 用户列表 API | 管理员可查看、搜索、筛选用户 |
| 封禁/解封用户 | 管理员可封禁违规用户账号 |
| 手动升级/降级 | 管理员可直接调整用户等级 |
| 用户统计 API | 用户贡献统计、活跃度分析 |

## 架构设计

### 层次结构

```
┌─────────────────────────────────────────────────────┐
│                    API Layer                         │
│  ┌────────────┐ ┌────────────┐ ┌────────────────┐   │
│  │ 用户列表    │ │ 投票选举    │ │ 管理 API       │   │
│  │ /users     │ │ /election  │ │ /admin/users   │   │
│  └────────────┘ └────────────┘ └────────────────┘   │
├─────────────────────────────────────────────────────┤
│                   Service Layer                      │
│  ┌────────────┐ ┌────────────┐ ┌────────────────┐   │
│  │UserManager │ │ElectionMgr │ │ AdminService   │   │
│  │ (现有)     │ │ (新增)     │ │ (新增)         │   │
│  └────────────┘ └────────────┘ └────────────────┘   │
├─────────────────────────────────────────────────────┤
│                   Storage Layer                      │
│  ┌────────────┐ ┌────────────┐ ┌────────────────┐   │
│  │ UserStore  │ │ElectionStore│ │ VoteStore     │   │
│  │ (现有)     │ │ (新增)      │ │ (新增)        │   │
│  └────────────┘ └────────────┘ └────────────────┘   │
└─────────────────────────────────────────────────────┘
```

### 文件结构

```
agentwiki/
├── internal/
│   ├── core/
│   │   ├── user/
│   │   │   ├── manager.go          # 现有: 用户管理
│   │   │   ├── level_checker.go    # 现有: 自动升级
│   │   │   └── admin_service.go    # 新增: 管理员服务
│   │   └── election/
│   │       ├── election.go         # 新增: 选举服务
│   │       └── election_test.go    # 新增: 测试
│   ├── storage/
│   │   ├── kv/
│   │   │   ├── election_store.go   # 新增: 选举存储
│   │   │   └── vote_store.go       # 新增: 投票存储
│   │   └── model/
│   │       └── election.go         # 新增: 选举模型
│   └── api/
│       └── handler/
│           ├── user_handler.go     # 修改: 添加用户列表
│           ├── admin_handler.go    # 新增: 管理员 API
│           └── election_handler.go # 新增: 选举 API
```

## 数据模型

### Election 选举

```go
package model

// Election 选举
type Election struct {
    ID            string `json:"id"`
    Title         string `json:"title"`
    Description   string `json:"description"`
    Status        string `json:"status"` // active, closed
    StartTime     int64  `json:"start_time"`
    EndTime       int64  `json:"end_time"`
    VoteThreshold int32  `json:"vote_threshold"` // 当选所需票数
    CreatedAt     int64  `json:"created_at"`
    CreatedBy     string `json:"created_by"` // 创建者用户 ID
}

// Candidate 候选人
type Candidate struct {
    ElectionID  string `json:"election_id"`
    UserID      string `json:"user_id"`
    UserName    string `json:"user_name"`
    NominatedBy string `json:"nominated_by"` // 提名人 ID
    VoteCount   int32  `json:"vote_count"`
    Status      string `json:"status"` // nominated, elected, rejected
    NominatedAt int64  `json:"nominated_at"`
}

// Vote 投票记录
type Vote struct {
    ID          string `json:"id"`
    ElectionID  string `json:"election_id"`
    VoterID     string `json:"voter_id"`
    CandidateID string `json:"candidate_id"` // 候选人用户 ID
    VotedAt     int64  `json:"voted_at"`
}

// UserStats 用户统计
type UserStats struct {
    TotalUsers      int64 `json:"total_users"`
    ActiveUsers     int64 `json:"active_users"`      // 30天内活跃
    Lv0Count        int64 `json:"lv0_count"`
    Lv1Count        int64 `json:"lv1_count"`
    Lv2Count        int64 `json:"lv2_count"`
    Lv3Count        int64 `json:"lv3_count"`
    Lv4Count        int64 `json:"lv4_count"`
    Lv5Count        int64 `json:"lv5_count"`
    TotalContribs   int64 `json:"total_contribs"`
    TotalRatings    int64 `json:"total_ratings"`
    BannedCount     int64 `json:"banned_count"`
}
```

## API 设计

### 用户列表 API

```
GET /api/v1/users
Query: page, limit, level, status, search
Response: { users: [], total, page, limit }

权限: Lv4+ (Admin)
```

### 管理 API

```
POST /api/v1/admin/users/{id}/ban
Body: { reason: string }
Response: { success: true }
权限: Lv4+ (Admin)

POST /api/v1/admin/users/{id}/unban
Response: { success: true }
权限: Lv4+ (Admin)

PUT /api/v1/admin/users/{id}/level
Body: { level: int32, reason: string }
Response: { success: true, new_level: int32 }
权限: Lv5 (SuperAdmin)

GET /api/v1/admin/stats/users
Response: UserStats
权限: Lv4+ (Admin)
```

### 选举 API

```
POST /api/v1/elections
Body: { title, description, vote_threshold, end_time }
Response: { election_id: string }
权限: Lv5 (SuperAdmin)

GET /api/v1/elections
Response: { elections: [] }
权限: 所有用户

GET /api/v1/elections/{id}
Response: Election 详情 + 候选人列表
权限: 所有用户

POST /api/v1/elections/{id}/candidates
Body: { user_id: string }
Response: { success: true }
权限: Lv4 用户可被提名

POST /api/v1/elections/{id}/vote
Body: { candidate_id: string }
Response: { success: true }
权限: Lv3+ 用户可投票

POST /api/v1/elections/{id}/close
Response: { elected: [] }
权限: Lv5 (SuperAdmin)
```

## 业务逻辑

### 投票选举流程

```
1. 创建选举 (Lv5)
   ↓
2. 开放提名 (Lv4 自动成为候选人，或被提名)
   ↓
3. 开放投票 (Lv3+ 用户投票)
   ↓
4. 关闭选举 (Lv5)
   ↓
5. 计票并宣布当选者
   ↓
6. 当选者自动升级为 Lv5
```

### 封禁用户

```
1. 管理员发送封禁请求
   ↓
2. 检查权限 (Lv4+)
   ↓
3. 检查目标用户等级 (不能封禁同级或更高等级)
   ↓
4. 更新用户状态为 "banned"
   ↓
5. 记录封禁原因和操作人
```

### 手动升级/降级

```
1. 超级管理员发送等级调整请求
   ↓
2. 检查权限 (Lv5 only)
   ↓
3. 更新用户等级
   ↓
4. 记录操作原因和操作人
   ↓
5. 发送通知给被调整用户
```

## 存储设计

### 键前缀

```
election:     "election:"     + election_id
elections:    "elections:list" (有序集合)
candidate:    "candidate:"     + election_id + ":" + user_id
candidates:   "candidates:"    + election_id (有序集合)
vote:         "vote:"          + election_id + ":" + vote_id
votes_by_voter: "votes:"       + election_id + ":" + voter_id
```

### 索引

- 选举列表按创建时间排序
- 候选人按票数排序
- 用户列表按注册时间排序

## 权限矩阵更新

| 操作 | Lv0 | Lv1 | Lv2 | Lv3 | Lv4 | Lv5 |
|------|-----|-----|-----|-----|-----|-----|
| 查看用户列表 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| 封禁/解封用户 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| 手动调整等级 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 创建选举 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 被提名为候选人 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| 投票 | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
| 关闭选举 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |

## 实现任务

### Task 1: 添加选举数据模型
- 创建 `internal/storage/model/election.go`
- 定义 Election, Candidate, Vote, UserStats 结构体

### Task 2: 实现选举存储
- 创建 `internal/storage/kv/election_store.go`
- 实现 ElectionStore 接口
- 创建 `internal/storage/kv/vote_store.go`
- 实现 VoteStore 接口

### Task 3: 实现选举服务
- 创建 `internal/core/election/election.go`
- 实现选举 CRUD、提名、投票、计票逻辑

### Task 4: 实现管理员服务
- 创建 `internal/core/user/admin_service.go`
- 实现用户列表、封禁、等级调整功能

### Task 5: 添加用户列表 API
- 修改 `internal/api/handler/user_handler.go`
- 添加用户列表端点

### Task 6: 添加管理员 API
- 创建 `internal/api/handler/admin_handler.go`
- 实现管理端点

### Task 7: 添加选举 API
- 创建 `internal/api/handler/election_handler.go`
- 实现选举端点

### Task 8: 更新路由和权限
- 修改 `internal/api/router/router.go`
- 添加新路由
- 更新权限检查中间件

### Task 9: 编写测试
- 选举服务测试
- 管理员服务测试
- API 集成测试

### Task 10: 运行完整测试套件
- 验证所有测试通过
- 确保覆盖率 > 60%

## 验收标准

- [ ] 投票选举系统完整实现
- [ ] 用户列表 API 可用
- [ ] 封禁/解封功能正常
- [ ] 手动升级/降级功能正常
- [ ] 用户统计 API 返回正确数据
- [ ] 所有测试通过
- [ ] 测试覆盖率 > 60%

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 选举作弊 | 破坏公平性 | 记录投票日志，限制投票频率 |
| 管理员滥用权限 | 用户投诉 | 记录所有管理操作，支持审计 |
| 数据一致性 | 统计不准 | 使用事务更新相关数据 |
