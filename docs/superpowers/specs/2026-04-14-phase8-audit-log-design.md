# Phase 8: 审计日志系统设计

**日期**: 2026-04-14
**优先级**: P1
**状态**: 待实现

---

## 一、概述

为 AgentWiki 添加审计日志系统，记录所有敏感操作（登录、条目增删改、评分、管理员操作等），支持安全审计和问题追溯。

### 需求确认

| 项目 | 决策 |
|------|------|
| 记录范围 | 所有敏感操作 |
| 保留策略 | 永久保留 |
| 查询权限 | 仅 Lv5 超级管理员 |
| 日志内容 | 完整请求记录（操作者、操作类型、目标对象、时间戳、IP、请求体、响应体） |
| 实现方案 | 中间件拦截 |

---

## 二、架构设计

### 2.1 组件关系图

```
请求 → AuthMiddleware → AuditMiddleware → Handler → 响应
                                 ↓
                           写入审计日志
                                 ↓
                          AuditStore (Pebble KV)
```

### 2.2 数据流

1. HTTP 请求进入 AuditMiddleware
2. 检查请求路径是否匹配敏感操作规则
3. 如果匹配，缓冲请求体，提取操作者信息
4. 调用下一个 Handler 处理请求
5. 捕获响应，异步写入审计日志
6. 返回响应给客户端

---

## 三、数据模型

### 3.1 AuditLog 结构

```go
// AuditLog 审计日志
type AuditLog struct {
    ID           string `json:"id"`            // 日志唯一 ID
    Timestamp    int64  `json:"timestamp"`     // 操作时间戳（毫秒）

    // 操作者信息
    OperatorPubkey string `json:"operator_pubkey"` // 操作者公钥
    OperatorLevel  int32  `json:"operator_level"`  // 操作者等级
    OperatorIP     string `json:"operator_ip"`     // 操作者 IP
    UserAgent      string `json:"user_agent"`      // User-Agent

    // 操作信息
    Method        string `json:"method"`        // HTTP 方法（GET/POST/PUT/DELETE）
    Path          string `json:"path"`          // 请求路径
    ActionType    string `json:"action_type"`   // 操作类型（如 entry.create, user.ban）
    TargetID      string `json:"target_id"`     // 目标对象 ID
    TargetType    string `json:"target_type"`   // 目标类型（entry/user/category等）

    // 请求/响应
    RequestBody   string `json:"request_body"`  // 请求体（脱敏后）
    ResponseCode  int    `json:"response_code"` // HTTP 响应码
    ResponseBody  string `json:"response_body"` // 响应体（截断）

    // 结果
    Success       bool   `json:"success"`       // 操作是否成功
    ErrorMessage  string `json:"error_message"` // 错误信息（失败时）
}
```

### 3.2 AuditFilter 查询过滤器

```go
type AuditFilter struct {
    OperatorPubkey string   // 按操作者筛选
    ActionTypes    []string // 按操作类型筛选（多个）
    TargetID       string   // 按目标 ID 筛选
    Success        *bool    // 按成功/失败筛选
    StartTime      int64    // 开始时间戳（毫秒）
    EndTime        int64    // 结束时间戳（毫秒）
    Limit          int      // 返回数量
    Offset         int      // 偏移量
}
```

### 3.3 AuditStats 审计统计

```go
type AuditStats struct {
    TotalLogs     int64            `json:"total_logs"`     // 总日志数
    TodayLogs     int64            `json:"today_logs"`     // 今日日志数
    ActionCounts  map[string]int64 `json:"action_counts"`  // 各操作类型数量
    FailedCount   int64            `json:"failed_count"`   // 失败操作数
}
```

---

## 四、存储层设计

### 4.1 存储接口

```go
// AuditStore 审计日志存储接口
type AuditStore interface {
    // Create 创建审计日志
    Create(ctx context.Context, log *model.AuditLog) error

    // Get 根据ID获取日志
    Get(ctx context.Context, id string) (*model.AuditLog, error)

    // List 查询日志列表，支持过滤和分页
    List(ctx context.Context, filter AuditFilter) ([]*model.AuditLog, int64, error)

    // DeleteBefore 删除指定时间之前的日志（用于手动清理）
    DeleteBefore(ctx context.Context, timestamp int64) (int64, error)

    // GetStats 获取审计统计
    GetStats(ctx context.Context) (*AuditStats, error)
}
```

### 4.2 存储实现

基于 Pebble KV 存储：

- **键格式**: `audit:{timestamp}:{random_id}`
- **时间戳倒序存储**: 便于按时间倒序查询
- **索引**: 无额外索引，通过 Scan + 内存过滤实现

---

## 五、中间件设计

### 5.1 AuditMiddleware 结构

```go
// AuditMiddleware 审计中间件
type AuditMiddleware struct {
    auditStore    AuditStore
    sensitivePath *path.Matcher // 敏感路径匹配器
    sensitiveOps  map[string]string // 路径模式 -> 操作类型
}

// 敏感操作路径映射
var sensitiveOps = map[string]string{
    // 用户相关
    "POST /api/v1/user/register":       "user.register",
    "POST /api/v1/user/verify-email":   "user.verify_email",
    "POST /api/v1/user/update":         "user.update",

    // 条目相关
    "POST /api/v1/entry/create":        "entry.create",
    "POST /api/v1/entry/update/*":      "entry.update",
    "POST /api/v1/entry/delete/*":      "entry.delete",
    "POST /api/v1/entry/rate/*":        "entry.rate",

    // 分类相关
    "POST /api/v1/categories/create":   "category.create",

    // 管理员相关
    "POST /api/v1/admin/users/*/ban":   "admin.user_ban",
    "POST /api/v1/admin/users/*/unban": "admin.user_unban",
    "PUT /api/v1/admin/users/*/level":  "admin.user_level",

    // 选举相关
    "POST /api/v1/elections":           "election.create",
    "POST /api/v1/elections/*/vote":    "election.vote",
    "POST /api/v1/elections/*/close":   "election.close",

    // 批量操作
    "POST /api/v1/batch/create":        "batch.create",
    "POST /api/v1/batch/update":        "batch.update",
    "POST /api/v1/batch/delete":        "batch.delete",

    // 导出/导入
    "GET /api/v1/admin/export":         "admin.export",
    "POST /api/v1/admin/import":        "admin.import",
}
```

### 5.2 中间件处理流程

```go
func (m *AuditMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. 检查是否为敏感操作
        actionType := m.matchSensitivePath(r.Method, r.URL.Path)
        if actionType == "" {
            next.ServeHTTP(w, r)
            return
        }

        // 2. 缓冲请求体
        bodyBytes, _ := io.ReadAll(r.Body)
        r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

        // 3. 提取操作者信息
        operatorPubkey, _ := r.Context().Value("public_key").(string)
        operatorLevel, _ := r.Context().Value("user_level").(int32)

        // 4. 包装 ResponseWriter 捕获响应
        rw := &responseWriter{ResponseWriter: w}

        // 5. 调用下一个 Handler
        next.ServeHTTP(rw, r)

        // 6. 异步写入审计日志
        go m.writeAuditLog(AuditLog{
            Timestamp:      time.Now().UnixMilli(),
            OperatorPubkey: operatorPubkey,
            OperatorLevel:  operatorLevel,
            OperatorIP:     getClientIP(r),
            UserAgent:      r.UserAgent(),
            Method:         r.Method,
            Path:           r.URL.Path,
            ActionType:     actionType,
            TargetID:       extractTargetID(r.URL.Path),
            TargetType:     getTargetType(actionType),
            RequestBody:    maskSensitiveFields(string(bodyBytes)),
            ResponseCode:   rw.status,
            ResponseBody:   truncateResponse(rw.body.String()),
            Success:        rw.status < 400,
            ErrorMessage:   getErrorMessage(rw.body.String()),
        })
    })
}
```

---

## 六、API 设计

### 6.1 查询审计日志

```
GET /api/v1/admin/audit/logs
```

**权限**: Lv5 超级管理员

**请求参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| operator | string | 否 | 操作者公钥 |
| action | string | 否 | 操作类型（多个用逗号分隔） |
| target_id | string | 否 | 目标 ID |
| success | bool | 否 | 成功/失败筛选 |
| start_time | int64 | 否 | 开始时间戳（毫秒） |
| end_time | int64 | 否 | 结束时间戳（毫秒） |
| limit | int | 否 | 返回数量（默认50，最大200） |
| offset | int | 否 | 偏移量（默认0） |

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_count": 1234,
    "has_more": true,
    "items": [
      {
        "id": "audit_1712345678901_abc123",
        "timestamp": 1712345678901,
        "operator_pubkey": "Base64公钥",
        "operator_level": 5,
        "operator_ip": "192.168.1.100",
        "user_agent": "Mozilla/5.0...",
        "method": "POST",
        "path": "/api/v1/admin/users/xxx/ban",
        "action_type": "admin.user_ban",
        "target_id": "user-public-key",
        "target_type": "user",
        "request_body": "{\"reason\":\"违规操作\"}",
        "response_code": 200,
        "response_body": "{\"code\":0,\"message\":\"success\"}",
        "success": true,
        "error_message": ""
      }
    ]
  }
}
```

### 6.2 获取审计统计

```
GET /api/v1/admin/audit/stats
```

**权限**: Lv5 超级管理员

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_logs": 12345,
    "today_logs": 234,
    "action_counts": {
      "entry.create": 5000,
      "entry.update": 3000,
      "entry.delete": 200,
      "admin.user_ban": 50,
      "admin.user_level": 30
    },
    "failed_count": 156
  }
}
```

### 6.3 清理审计日志

```
DELETE /api/v1/admin/audit/logs?before={timestamp}
```

**权限**: Lv5 超级管理员

**请求参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| before | int64 | 是 | 删除此时间戳之前的日志 |

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "deleted_count": 1000
  }
}
```

---

## 七、脱敏规则

为保护隐私和安全，以下字段需要脱敏处理：

| 字段类型 | 脱敏规则 | 示例 |
|----------|----------|------|
| 密码字段 | 替换为 `***` | `"password": "***"` |
| 私钥字段 | 替换为 `***` | `"private_key": "***"` |
| 验证码字段 | 替换为 `***` | `"code": "***"` |
| 响应体过长 | 截断至 4KB | `"...[TRUNCATED]"` |
| 请求体过长 | 截断至 16KB | `"...[TRUNCATED]"` |

**需要脱敏的字段名**:
- `password`, `passwd`, `pwd`
- `private_key`, `privateKey`, `private-key`
- `secret`, `token`, `api_key`, `apiKey`
- `code`, `verification_code`

---

## 八、敏感操作清单

| 操作类型 | HTTP 方法 | 路径模式 | 描述 |
|----------|-----------|----------|------|
| `user.register` | POST | /api/v1/user/register | 用户注册 |
| `user.verify_email` | POST | /api/v1/user/verify-email | 邮箱验证 |
| `user.update` | POST | /api/v1/user/update | 用户信息更新 |
| `entry.create` | POST | /api/v1/entry/create | 创建条目 |
| `entry.update` | POST | /api/v1/entry/update/* | 更新条目 |
| `entry.delete` | POST | /api/v1/entry/delete/* | 删除条目 |
| `entry.rate` | POST | /api/v1/entry/rate/* | 评分 |
| `category.create` | POST | /api/v1/categories/create | 创建分类 |
| `admin.user_ban` | POST | /api/v1/admin/users/*/ban | 封禁用户 |
| `admin.user_unban` | POST | /api/v1/admin/users/*/unban | 解封用户 |
| `admin.user_level` | PUT | /api/v1/admin/users/*/level | 调整等级 |
| `admin.export` | GET | /api/v1/admin/export | 数据导出 |
| `admin.import` | POST | /api/v1/admin/import | 数据导入 |
| `election.create` | POST | /api/v1/elections | 创建选举 |
| `election.vote` | POST | /api/v1/elections/*/vote | 投票 |
| `election.close` | POST | /api/v1/elections/*/close | 关闭选举 |
| `batch.create` | POST | /api/v1/batch/create | 批量创建 |
| `batch.update` | POST | /api/v1/batch/update | 批量更新 |
| `batch.delete` | POST | /api/v1/batch/delete | 批量删除 |

---

## 九、文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/storage/model/audit.go` | 创建 | 审计日志数据模型 |
| `internal/storage/kv/audit_store.go` | 创建 | 审计日志存储实现 |
| `internal/core/audit/audit.go` | 创建 | 审计服务逻辑 |
| `internal/api/middleware/audit.go` | 创建 | 审计中间件 |
| `internal/api/handler/audit_handler.go` | 创建 | 审计查询 API |
| `internal/storage/store.go` | 修改 | 添加 AuditStore 接口 |
| `internal/api/router/router.go` | 修改 | 注册中间件和路由 |

---

## 十、性能考虑

1. **异步写入**: 审计日志异步写入，不阻塞主请求
2. **缓冲区**: 响应体使用缓冲区，避免内存泄漏
3. **截断策略**: 超长请求/响应体截断，防止存储膨胀
4. **批量查询**: 列表查询支持分页，避免一次加载过多数据

---

## 十一、测试要点

1. **中间件测试**
   - 敏感路径匹配正确
   - 非敏感路径不记录
   - 请求体正确缓冲和恢复

2. **存储测试**
   - 日志创建和查询
   - 过滤器正确工作
   - 分页正确

3. **API 测试**
   - 权限检查（仅 Lv5 可访问）
   - 查询参数正确解析
   - 统计数据正确

4. **脱敏测试**
   - 敏感字段正确脱敏
   - 长内容正确截断
