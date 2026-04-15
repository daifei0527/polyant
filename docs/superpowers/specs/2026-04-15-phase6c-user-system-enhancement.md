# Phase 6c: 用户体系完善设计

**版本**: v2.1.0
**日期**: 2026-04-15
**状态**: 已批准

---

## 一、概述

完善 Polyant 用户体系，添加投票选举系统和用户管理功能，实现 Lv4 → Lv5 的民主升级机制，以及管理员对用户的管理能力。

### 需求确认

| 项目 | 决策 |
|------|------|
| 选举机制 | 自动 + 手动（达到阈值自动当选，也支持 Lv5 手动关闭） |
| 封禁类型 | 可配置（完全禁止访问 / 只读模式） |
| 提名方式 | 自荐 + 他荐（Lv4 可自荐，Lv3+ 可提名其他 Lv4） |
| 统计维度 | 等级分布 + 贡献明细 + 活跃度趋势 + 注册趋势 |

---

## 二、架构设计

### 2.1 层次结构

```
┌─────────────────────────────────────────────────────────────┐
│                         API Layer                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ /users       │  │ /elections   │  │ /admin/users     │   │
│  │ 用户列表/统计 │  │ 选举系统     │  │ 封禁/等级调整    │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│                       Service Layer                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ UserManager  │  │ElectionService│ │ AdminService     │   │
│  │ (现有+扩展)   │  │ (新增)       │  │ (新增)           │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│                       Storage Layer                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ UserStore    │  │ElectionStore │  │ VoteStore        │   │
│  │ (现有+扩展)   │  │ (新增)       │  │ (新增)           │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 文件结构

```
polyant/
├── internal/
│   ├── core/
│   │   ├── user/
│   │   │   ├── manager.go          # 现有: 用户管理
│   │   │   ├── level_checker.go    # 现有: 自动升级
│   │   │   ├── admin_service.go    # 新增: 管理员服务
│   │   │   └── stats_service.go    # 新增: 统计服务
│   │   └── election/
│   │       ├── election.go         # 新增: 选举服务
│   │       ├── nomination.go       # 新增: 提名服务
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
│           ├── election_handler.go # 新增: 选举 API
│           └── stats_handler.go    # 新增: 统计 API
```

---

## 三、数据模型

### 3.1 用户模型扩展

```go
package model

// UserStatus 用户状态
type UserStatus string

const (
    UserStatusActive   UserStatus = "active"   // 正常
    UserStatusBanned   UserStatus = "banned"   // 完全封禁
    UserStatusReadonly UserStatus = "readonly" // 只读模式
)

// BanType 封禁类型
type BanType string

const (
    BanTypeFull     BanType = "full"     // 完全禁止访问
    BanTypeReadonly BanType = "readonly" // 只读模式
)

// User 用户模型扩展字段
type User struct {
    // ... 现有字段 ...

    // 封禁相关
    Status    UserStatus `json:"status"`     // 用户状态
    BanType   BanType    `json:"ban_type"`   // 封禁类型
    BanReason string     `json:"ban_reason"` // 封禁原因
    BannedAt  int64      `json:"banned_at"`  // 封禁时间
    BannedBy  string     `json:"banned_by"`  // 操作人 ID
}
```

### 3.2 选举模型

```go
package model

// Election 选举
type Election struct {
    ID            string `json:"id"`
    Title         string `json:"title"`
    Description   string `json:"description"`
    Status        string `json:"status"`        // active, closed
    StartTime     int64  `json:"start_time"`
    EndTime       int64  `json:"end_time"`      // 计划结束时间（可选）
    VoteThreshold int32  `json:"vote_threshold"` // 当选所需票数
    AutoElect     bool   `json:"auto_elect"`    // 是否自动当选
    CreatedAt     int64  `json:"created_at"`
    CreatedBy     string `json:"created_by"`    // 创建者用户 ID
}

// Candidate 候选人
type Candidate struct {
    ElectionID  string `json:"election_id"`
    UserID      string `json:"user_id"`
    UserName    string `json:"user_name"`
    NominatedBy string `json:"nominated_by"`  // 提名人 ID（自荐时为自己）
    SelfNominated bool `json:"self_nominated"` // 是否自荐
    VoteCount   int32  `json:"vote_count"`
    Status      string `json:"status"`        // nominated, elected, rejected
    Confirmed   bool   `json:"confirmed"`     // 是否确认接受提名
    NominatedAt int64  `json:"nominated_at"`
    ConfirmedAt int64  `json:"confirmed_at,omitempty"`
}

// Vote 投票记录
type Vote struct {
    ID          string `json:"id"`
    ElectionID  string `json:"election_id"`
    VoterID     string `json:"voter_id"`
    CandidateID string `json:"candidate_id"` // 候选人用户 ID
    VotedAt     int64  `json:"voted_at"`
}
```

### 3.3 统计模型

```go
package model

// UserStats 用户统计
type UserStats struct {
    TotalUsers  int64 `json:"total_users"`
    ActiveUsers int64 `json:"active_users"` // 30天内活跃
    Lv0Count    int64 `json:"lv0_count"`
    Lv1Count    int64 `json:"lv1_count"`
    Lv2Count    int64 `json:"lv2_count"`
    Lv3Count    int64 `json:"lv3_count"`
    Lv4Count    int64 `json:"lv4_count"`
    Lv5Count    int64 `json:"lv5_count"`
    BannedCount int64 `json:"banned_count"`
}

// UserContribution 用户贡献明细
type UserContribution struct {
    UserID           string `json:"user_id"`
    UserName         string `json:"user_name"`
    EntryCount       int64  `json:"entry_count"`        // 创建条目数
    EditCount        int64  `json:"edit_count"`         // 编辑次数
    RatingGivenCount int64  `json:"rating_given_count"` // 给出评分数
    RatingRecvCount  int64  `json:"rating_recv_count"`  // 收到评分数
    AvgRatingRecv    float64 `json:"avg_rating_recv"`   // 收到评分平均分
}

// ActivityTrend 活跃度趋势
type ActivityTrend struct {
    Date       string `json:"date"`        // 日期 YYYY-MM-DD
    DAU        int64  `json:"dau"`         // 日活跃用户
    WAU        int64  `json:"wau"`         // 周活跃用户（周报时使用）
    MAU        int64  `json:"mau"`         // 月活跃用户（月报时使用）
    NewUsers   int64  `json:"new_users"`   // 新注册用户
    ActionCount int64 `json:"action_count"` // 操作总数
}

// RegistrationTrend 注册趋势
type RegistrationTrend struct {
    Date      string `json:"date"`       // 日期 YYYY-MM-DD
    Count     int64  `json:"count"`      // 当日注册数
    Total     int64  `json:"total"`      // 累计用户数
}
```

---

## 四、API 设计

### 4.1 用户列表 API

```
GET /api/v1/users
权限: Lv4+ (Admin)

Query 参数:
- page: 页码（默认 1）
- limit: 每页数量（默认 20，最大 100）
- level: 等级筛选（0-5）
- status: 状态筛选（active, banned, readonly）
- search: 搜索（用户名、公钥）
- sort: 排序（created_at, last_active, contribution）

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "users": [
      {
        "public_key": "...",
        "agent_name": "Agent1",
        "user_level": 1,
        "status": "active",
        "contrib_count": 5,
        "created_at": 1712800000000,
        "last_active_at": 1712900000000
      }
    ],
    "total": 100,
    "page": 1,
    "limit": 20
  }
}
```

### 4.2 管理 API

#### 封禁用户

```
POST /api/v1/admin/users/{id}/ban
权限: Lv4+ (Admin)

Request:
{
  "ban_type": "full",      // full 或 readonly
  "reason": "违规原因"
}

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": "...",
    "status": "banned",
    "ban_type": "full"
  }
}
```

#### 解封用户

```
POST /api/v1/admin/users/{id}/unban
权限: Lv4+ (Admin)

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": "...",
    "status": "active"
  }
}
```

#### 调整用户等级

```
PUT /api/v1/admin/users/{id}/level
权限: Lv5 (SuperAdmin)

Request:
{
  "level": 4,
  "reason": "特殊贡献"
}

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": "...",
    "old_level": 3,
    "new_level": 4
  }
}
```

### 4.3 选举 API

#### 创建选举

```
POST /api/v1/elections
权限: Lv5 (SuperAdmin)

Request:
{
  "title": "第1届超级管理员选举",
  "description": "选举说明...",
  "vote_threshold": 10,
  "auto_elect": true,
  "end_time": 1713500000000  // 可选
}

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "election_id": "ele-xxx"
  }
}
```

#### 获取选举列表

```
GET /api/v1/elections
权限: 所有用户

Query 参数:
- status: 状态筛选（active, closed）
- page, limit

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "elections": [
      {
        "id": "ele-xxx",
        "title": "...",
        "status": "active",
        "vote_threshold": 10,
        "candidate_count": 5,
        "total_votes": 42,
        "created_at": 1712800000000
      }
    ],
    "total": 5
  }
}
```

#### 获取选举详情

```
GET /api/v1/elections/{id}
权限: 所有用户

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "election": {
      "id": "ele-xxx",
      "title": "...",
      "description": "...",
      "status": "active",
      "vote_threshold": 10,
      "auto_elect": true
    },
    "candidates": [
      {
        "user_id": "...",
        "user_name": "Candidate1",
        "vote_count": 8,
        "status": "nominated",
        "confirmed": true
      }
    ]
  }
}
```

#### 自荐/提名候选人

```
POST /api/v1/elections/{id}/candidates
权限: Lv3+ (Lv4 可自荐，Lv3+ 可提名他人)

Request:
{
  "user_id": "..."  // 自荐时传自己的 ID，提名他人时传被提名人 ID
}

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "candidate_id": "...",
    "self_nominated": true,
    "confirmed": true  // 自荐时自动确认
  }
}
```

#### 确认接受提名

```
POST /api/v1/elections/{id}/candidates/{user_id}/confirm
权限: 被提名人自己

Response:
{
  "code": 0,
  "message": "success"
}
```

#### 投票

```
POST /api/v1/elections/{id}/vote
权限: Lv3+

Request:
{
  "candidate_id": "..."
}

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "vote_count": 9  // 候选人当前票数
  }
}
```

#### 关闭选举

```
POST /api/v1/elections/{id}/close
权限: Lv5 (SuperAdmin)

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "elected": [
      {
        "user_id": "...",
        "user_name": "...",
        "vote_count": 12
      }
    ]
  }
}
```

### 4.4 统计 API

#### 用户统计概览

```
GET /api/v1/admin/stats/users
权限: Lv4+ (Admin)

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "total_users": 1000,
    "active_users": 350,
    "lv0_count": 600,
    "lv1_count": 200,
    "lv2_count": 100,
    "lv3_count": 50,
    "lv4_count": 40,
    "lv5_count": 10,
    "banned_count": 5
  }
}
```

#### 贡献明细

```
GET /api/v1/admin/stats/contributions
权限: Lv4+ (Admin)

Query 参数:
- page, limit
- sort: entry_count, rating_given_count, rating_recv_count

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "contributions": [
      {
        "user_id": "...",
        "user_name": "TopContributor",
        "entry_count": 50,
        "edit_count": 20,
        "rating_given_count": 100,
        "rating_recv_count": 80,
        "avg_rating_recv": 4.5
      }
    ],
    "total": 100
  }
}
```

#### 活跃度趋势

```
GET /api/v1/admin/stats/activity
权限: Lv4+ (Admin)

Query 参数:
- range: day, week, month（默认 day）
- days: 天数（默认 30）

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "trend": [
      {
        "date": "2026-04-01",
        "dau": 120,
        "new_users": 15,
        "action_count": 450
      }
    ]
  }
}
```

#### 注册趋势

```
GET /api/v1/admin/stats/registrations
权限: Lv4+ (Admin)

Query 参数:
- days: 天数（默认 30）

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "trend": [
      {
        "date": "2026-04-01",
        "count": 15,
        "total": 850
      }
    ]
  }
}
```

---

## 五、业务逻辑

### 5.1 投票选举流程

```
1. Lv5 创建选举
   ├─ 设置票数阈值
   ├─ 设置是否自动当选
   └─ 设置计划结束时间（可选）
   ↓
2. 开放提名
   ├─ Lv4 用户可自荐（自动确认）
   └─ Lv3+ 用户可提名其他 Lv4（需被提名人确认）
   ↓
3. 开放投票
   └─ Lv3+ 用户投票（每人每候选人限投一票）
   ↓
4. 自动当选检查（如果启用 auto_elect）
   └─ 候选人票数 ≥ 阈值时自动标记为 elected
   ↓
5. Lv5 手动关闭选举（可选）
   └─ 计票并宣布所有当选者
   ↓
6. 当选者自动升级为 Lv5
```

### 5.2 封禁用户逻辑

```
1. 管理员发送封禁请求
   ├─ 选择封禁类型（full/readonly）
   └─ 填写封禁原因
   ↓
2. 权限检查
   ├─ Lv4 可封禁 Lv0-Lv3
   └─ Lv5 可封禁 Lv0-Lv4
   ↓
3. 更新用户状态
   ├─ status = banned/readonly
   ├─ ban_type = full/readonly
   ├─ ban_reason = 原因
   └─ banned_by = 操作人 ID
   ↓
4. 记录操作日志
```

### 5.3 封禁状态访问控制

| 封禁类型 | 读取条目 | 创建/编辑 | 评分 | 用户注册 |
|---------|---------|----------|------|---------|
| full | ❌ | ❌ | ❌ | ❌ |
| readonly | ✅ | ❌ | ❌ | ❌ |
| active | ✅ | ✅ | ✅ | ✅ |

### 5.4 手动升级/降级逻辑

```
1. Lv5 发送等级调整请求
   ↓
2. 验证目标用户存在
   ↓
3. 更新用户等级
   ↓
4. 记录操作原因和操作人
   ↓
5. （可选）发送通知给被调整用户
```

---

## 六、存储设计

### 6.1 键前缀

```
election:        "election:" + election_id
elections_list:  "elections:list" (按创建时间排序)
candidate:       "candidate:" + election_id + ":" + user_id
candidates_list: "candidates:" + election_id (按票数排序)
vote:            "vote:" + election_id + ":" + vote_id
votes_by_voter:  "votes:" + election_id + ":" + voter_id
vote_check:      "votecheck:" + election_id + ":" + voter_id + ":" + candidate_id

activity_daily:  "activity:daily:" + date
registration:    "registration:daily:" + date
```

### 6.2 索引

- 选举列表按创建时间倒序排序
- 候选人按票数倒序排序
- 用户列表支持多种排序（注册时间、活跃时间、贡献数）

---

## 七、权限矩阵

| 操作 | Lv0 | Lv1 | Lv2 | Lv3 | Lv4 | Lv5 |
|------|-----|-----|-----|-----|-----|-----|
| 查看用户列表 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| 查看选举列表 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 自荐候选人 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| 提名他人 | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
| 投票 | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
| 封禁/解封用户 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| 手动调整等级 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 创建选举 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 关闭选举 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 查看统计 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |

---

## 八、实现阶段

### 阶段 1：用户模型扩展（2 个任务）

| 任务 | 内容 |
|-----|------|
| Task 1 | 扩展 User 模型，添加封禁相关字段 |
| Task 2 | 更新 UserStore，支持状态筛选和更新 |

### 阶段 2：管理功能（3 个任务）

| 任务 | 内容 |
|-----|------|
| Task 3 | 创建 AdminService，实现封禁/解封逻辑 |
| Task 4 | 创建用户列表 API |
| Task 5 | 创建管理 API（封禁、解封、等级调整） |

### 阶段 3：选举系统（4 个任务）

| 任务 | 内容 |
|-----|------|
| Task 6 | 创建选举数据模型和存储 |
| Task 7 | 实现选举服务（CRUD、提名、投票） |
| Task 8 | 实现自动当选逻辑 |
| Task 9 | 创建选举 API |

### 阶段 4：统计 API（2 个任务）

| 任务 | 内容 |
|-----|------|
| Task 10 | 创建 StatsService，实现四个维度统计 |
| Task 11 | 创建统计 API 端点 |

---

## 九、验收标准

- [ ] 投票选举系统完整实现
- [ ] 自动当选功能正常
- [ ] 自荐 + 他荐提名功能正常
- [ ] 用户列表 API 可用
- [ ] 封禁/解封功能正常（支持两种封禁类型）
- [ ] 手动升级/降级功能正常
- [ ] 四个维度统计 API 返回正确数据
- [ ] 所有测试通过
- [ ] 测试覆盖率 > 60%

---

## 十、风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 选举作弊 | 破坏公平性 | 记录投票日志，限制投票频率 |
| 管理员滥用权限 | 用户投诉 | 记录所有管理操作，支持审计 |
| 数据一致性 | 统计不准 | 使用事务更新相关数据 |
| 自动当选时机 | 延迟或过早 | 在投票时检查阈值，及时触发 |
