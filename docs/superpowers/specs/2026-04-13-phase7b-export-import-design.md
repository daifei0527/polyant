# Phase 7b: 数据导出/导入设计

**版本**: v1.0.0
**日期**: 2026-04-13
**状态**: 已批准

## 概述

为 AgentWiki 添加数据导出/导入功能，支持管理员备份、迁移和恢复系统数据。

## 目标

| 功能 | 描述 |
|------|------|
| 数据导出 | 导出条目、分类、用户、评分到 ZIP 文件 |
| 数据导入 | 从 ZIP 文件导入数据，支持冲突处理策略 |
| 完整数据备份 | 支持灾难恢复和数据迁移 |

## 限制与约束

- **权限要求**: Lv4+ (Admin)
- **导出格式**: ZIP 压缩包（内含 JSON 文件）
- **用户隐私**: 导出用户时不含邮箱、手机等敏感字段
- **导入安全**: 导入用户等级不能高于操作者等级

## API 设计

### 导出 API

```
GET /api/v1/admin/export
权限: Lv4+ (Admin)

Query 参数:
- include: 导出范围，逗号分隔 (entries,categories,users,ratings)
- format: 文件格式，默认 "zip"

示例:
GET /api/v1/admin/export?include=entries,categories,users,ratings

响应:
Content-Type: application/zip
Content-Disposition: attachment; filename="agentwiki-export-20260413.zip"

ZIP 文件结构:
├── manifest.json        # 元数据
├── entries.json         # 知识条目
├── categories.json      # 分类
├── users.json           # 用户（不含敏感字段）
└── ratings.json         # 评分
```

### 导入 API

```
POST /api/v1/admin/import
权限: Lv4+ (Admin)
Content-Type: multipart/form-data

表单字段:
- file: ZIP 文件
- conflict: 冲突策略 (skip|overwrite|merge)

响应:
{
  "success": true,
  "summary": {
    "entries_imported": 150,
    "entries_skipped": 10,
    "entries_updated": 5,
    "categories_imported": 8,
    "users_imported": 25,
    "ratings_imported": 320
  },
  "errors": []
}
```

## 数据格式

### manifest.json 元数据

```json
{
  "version": "1.0",
  "exported_at": 1744567890123,
  "node_id": "node-abc123",
  "counts": {
    "entries": 150,
    "categories": 8,
    "users": 25,
    "ratings": 320
  }
}
```

### entries.json 条目格式

```json
[
  {
    "id": "entry-001",
    "title": "Go 并发编程",
    "content": "# Go 并发编程\n\n...",
    "category": "tech/programming",
    "tags": ["go", "concurrency"],
    "version": 3,
    "created_at": 1744567890123,
    "updated_at": 1744589123456,
    "created_by": "public-key-base64...",
    "score": 4.5,
    "score_count": 12,
    "status": "published",
    "license": "CC-BY-SA-4.0"
  }
]
```

### users.json 用户格式（隐私保护）

```json
[
  {
    "public_key": "base64...",
    "agent_name": "AI-Agent-001",
    "user_level": 2,
    "registered_at": 1744500000000,
    "status": "active"
  }
]
```

**注意：** 用户导出不含邮箱、手机等敏感信息，只保留公开字段。

## 冲突处理策略

### 冲突判断

- **条目冲突**: 相同 `id` 的条目已存在
- **分类冲突**: 相同 `path` 的分类已存在
- **用户冲突**: 相同 `public_key` 的用户已存在

### skip（跳过）

```
遇到冲突 → 记录跳过 → 继续下一条
```

### overwrite（覆盖）

```
遇到冲突 → 删除现有数据 → 插入导入数据
```

### merge（合并）

```
条目冲突 → 比较 version，保留更高版本
分类冲突 → 保留现有（分类结构不宜随意覆盖）
用户冲突 → 保留现有用户等级（防止降级攻击）
```

### 安全考虑

- 导入的用户等级**不能高于**导入者的等级
- 导入用户时，如果用户已存在，只更新公开字段，不修改等级

## 文件结构

```
internal/
├── api/handler/
│   └── export_handler.go      # 导出/导入 Handler
├── core/export/
│   ├── exporter.go            # 导出服务
│   └── importer.go            # 导入服务
```

## 实现任务

### Task 1: 创建导出服务
- 创建 `internal/core/export/exporter.go`
- 实现 ZIP 文件生成
- 实现各数据类型的导出方法

### Task 2: 创建导入服务
- 创建 `internal/core/export/importer.go`
- 实现 ZIP 文件解析
- 实现冲突处理逻辑

### Task 3: 创建导出/导入 Handler
- 创建 `internal/api/handler/export_handler.go`
- 实现导出端点
- 实现导入端点

### Task 4: 注册路由
- 修改 `internal/api/router/router.go`
- 添加导出/导入路由

### Task 5: 编写测试
- 导出功能测试
- 导入功能测试
- 冲突策略测试

## 验收标准

- [ ] `GET /api/v1/admin/export` 可下载 ZIP 文件
- [ ] `POST /api/v1/admin/import` 可上传并导入数据
- [ ] 支持 `skip`/`overwrite`/`merge` 三种冲突策略
- [ ] 导出的 ZIP 包含正确的数据结构
- [ ] 导入时权限检查正确（Lv4+）
- [ ] 测试覆盖率 > 55%
