# Phase 7a: 批量操作 API 设计

**版本**: v1.0.0
**日期**: 2026-04-13
**状态**: 已批准

## 概述

为 AgentWiki 添加批量操作 API，支持一次请求创建、更新、删除多个知识条目，提升大规模数据操作效率。

## 目标

| 功能 | 描述 |
|------|------|
| 批量创建条目 | 一次请求创建多个条目 |
| 批量更新条目 | 一次请求更新多个条目 |
| 批量删除条目 | 一次请求删除多个条目 |
| 批量导入条目 | 从 JSON 文件导入多个条目 |

## 限制与约束

- **最大条目数**: 100 条/批次
- **失败处理**: 预验证模式（先验证全部条目，全部通过后执行操作）
- **权限要求**: Lv1+（与单条操作相同）
- **超时时间**: 60 秒

## API 设计

### 批量创建条目

```
POST /api/v1/entries/batch
权限: Lv1+

请求体:
{
  "entries": [
    {
      "title": "条目标题1",
      "content": "条目内容1",
      "category": "tech/programming",
      "tags": ["go", "api"]
    },
    {
      "title": "条目标题2",
      "content": "条目内容2",
      "category": "tech/database"
    }
  ],
  "options": {
    "skip_duplicates": true,
    "update_existing": false
  }
}

响应:
{
  "success": true,
  "summary": {
    "total": 50,
    "created": 48,
    "skipped": 2,
    "failed": 0
  },
  "results": [
    {"index": 0, "id": "entry-id-1", "status": "created"},
    {"index": 1, "id": "entry-id-2", "status": "skipped", "reason": "duplicate"}
  ],
  "errors": []
}
```

### 批量更新条目

```
PUT /api/v1/entries/batch
权限: Lv1+

请求体:
{
  "entries": [
    {
      "id": "entry-id-1",
      "title": "更新后的标题1",
      "content": "更新后的内容1"
    },
    {
      "id": "entry-id-2",
      "content": "仅更新内容"
    }
  ]
}

响应:
{
  "success": true,
  "summary": {
    "total": 10,
    "updated": 9,
    "not_found": 1,
    "failed": 0
  },
  "results": [
    {"index": 0, "id": "entry-id-1", "status": "updated", "version": 2},
    {"index": 1, "id": "entry-id-2", "status": "not_found"}
  ]
}
```

### 批量删除条目

```
DELETE /api/v1/entries/batch
权限: Lv1+ (创建者) 或 Lv4+ (管理员)

请求体:
{
  "ids": ["entry-id-1", "entry-id-2", "entry-id-3"]
}

响应:
{
  "success": true,
  "summary": {
    "total": 3,
    "deleted": 2,
    "not_found": 1
  },
  "results": [
    {"index": 0, "id": "entry-id-1", "status": "deleted"},
    {"index": 1, "id": "entry-id-2", "status": "deleted"},
    {"index": 2, "id": "entry-id-3", "status": "not_found"}
  ]
}
```

### 批量导入条目

```
POST /api/v1/entries/import
权限: Lv1+
Content-Type: multipart/form-data

表单字段:
- file: JSON 文件

文件格式:
{
  "entries": [...],
  "options": {
    "skip_duplicates": true,
    "update_existing": false
  }
}

响应: 与批量创建相同
```

## 处理流程

```
┌─────────────────────────────────────────────────────────────┐
│                     批量操作处理流程                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. 接收请求                                                 │
│     ├── 验证条目数量 (≤100)                                  │
│     ├── 验证用户权限 (Lv1+)                                  │
│     └── 解析请求体                                           │
│                    ↓                                        │
│  2. 预验证阶段                                               │
│     ├── 遍历所有条目                                         │
│     ├── 检查字段完整性 (title, content, category)            │
│     ├── 检查分类是否存在                                      │
│     ├── 检查权限 (更新/删除时检查所有权)                       │
│     └── 收集所有验证错误                                      │
│                    ↓                                        │
│  3. 验证结果判断                                             │
│     ├── 有错误? → 返回错误详情，不执行任何操作                 │
│     └── 无错误? → 继续执行                                   │
│                    ↓                                        │
│  4. 执行阶段                                                 │
│     ├── 开启事务                                             │
│     ├── 遍历执行每个操作                                      │
│     ├── 记录每个操作结果                                      │
│     └── 提交事务                                             │
│                    ↓                                        │
│  5. 返回结果                                                 │
│     ├── 汇总统计 (total, created, failed, etc.)              │
│     └── 详细结果列表                                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 数据模型

### BatchRequest 批量请求

```go
// BatchCreateRequest 批量创建请求
type BatchCreateRequest struct {
    Entries []BatchEntry `json:"entries"`
    Options BatchOptions `json:"options"`
}

// BatchEntry 批量操作条目
type BatchEntry struct {
    ID       string   `json:"id,omitempty"`       // 更新/删除时需要
    Title    string   `json:"title"`
    Content  string   `json:"content"`
    Category string   `json:"category"`
    Tags     []string `json:"tags,omitempty"`
}

// BatchOptions 批量操作选项
type BatchOptions struct {
    SkipDuplicates  bool `json:"skip_duplicates"`   // 跳过重复条目
    UpdateExisting  bool `json:"update_existing"`   // 更新已存在的条目
}

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
    IDs []string `json:"ids"`
}

// BatchResponse 批量操作响应
type BatchResponse struct {
    Success bool              `json:"success"`
    Summary BatchSummary      `json:"summary"`
    Results []BatchResult     `json:"results"`
    Errors  []BatchError      `json:"errors,omitempty"`
}

// BatchSummary 批量操作汇总
type BatchSummary struct {
    Total     int `json:"total"`
    Created   int `json:"created,omitempty"`
    Updated   int `json:"updated,omitempty"`
    Deleted   int `json:"deleted,omitempty"`
    Skipped   int `json:"skipped,omitempty"`
    Failed    int `json:"failed,omitempty"`
    NotFound  int `json:"not_found,omitempty"`
}

// BatchResult 单个条目操作结果
type BatchResult struct {
    Index   int    `json:"index"`
    ID      string `json:"id"`
    Status  string `json:"status"` // created, updated, deleted, skipped, failed, not_found
    Reason  string `json:"reason,omitempty"`
    Version int64  `json:"version,omitempty"`
}

// BatchError 批量操作错误
type BatchError struct {
    Index   int    `json:"index"`
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

## 文件结构

```
internal/
├── api/
│   └── handler/
│       └── batch_handler.go    # 新增: 批量操作处理器
├── core/
│   └── entry/
│       └── batch_service.go    # 新增: 批量操作服务
└── storage/
    └── kv/
        └── entry_store.go      # 修改: 添加批量方法
```

## 实现任务

### Task 1: 添加批量操作数据模型
- 创建 `internal/api/handler/types.go` 中的批量请求/响应类型

### Task 2: 实现批量操作服务
- 创建 `internal/core/entry/batch_service.go`
- 实现预验证逻辑
- 实现批量 CRUD 操作

### Task 3: 实现批量操作 Handler
- 创建 `internal/api/handler/batch_handler.go`
- 实现 POST/PUT/DELETE /entries/batch
- 实现 POST /entries/import

### Task 4: 更新路由
- 修改 `internal/api/router/router.go`
- 添加批量操作路由

### Task 5: 编写测试
- 批量创建测试
- 批量更新测试
- 批量删除测试
- 预验证失败测试

### Task 6: 运行测试套件
- 验证所有测试通过
- 确保覆盖率 > 60%

## 错误处理

| 错误 | HTTP | 说明 |
|------|------|------|
| 条目数超过限制 | 400 | 最多 100 条/批次 |
| 预验证失败 | 400 | 返回详细错误列表 |
| 权限不足 | 403 | 需要 Lv1+ |
| 分类不存在 | 400 | 指定的分类路径无效 |
| 条目不存在 | 404 | 更新/删除时条目 ID 无效 |

## 验收标准

- [ ] 批量创建 API 可用
- [ ] 批量更新 API 可用
- [ ] 批量删除 API 可用
- [ ] 批量导入 API 可用
- [ ] 预验证模式正常工作
- [ ] 所有测试通过
- [ ] 测试覆盖率 > 60%
