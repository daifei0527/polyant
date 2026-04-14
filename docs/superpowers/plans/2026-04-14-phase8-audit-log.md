# Phase 8: 审计日志系统实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Polyant 添加审计日志系统，记录所有敏感操作，支持安全审计和问题追溯

**Architecture:** 通过 HTTP 中间件拦截敏感操作请求，记录完整请求/响应信息，存储到 Pebble KV，提供 Lv5 超级管理员查询 API

**Tech Stack:** Go 1.22, net/http, Pebble KV, encoding/json

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/storage/model/audit.go` | 创建 | 审计日志数据模型 |
| `internal/storage/kv/audit_store.go` | 创建 | 审计日志存储实现 |
| `internal/core/audit/audit.go` | 创建 | 审计服务逻辑 |
| `internal/api/middleware/audit.go` | 创建 | 审计中间件 |
| `internal/api/handler/audit_handler.go` | 创建 | 审计查询 API |
| `internal/storage/store.go` | 修改 | 添加 AuditStore 接口和实例 |
| `internal/api/router/router.go` | 修改 | 注册中间件和路由 |

---

## Task 1: 创建审计日志数据模型

**Files:**
- Create: `internal/storage/model/audit.go`

- [ ] **Step 1: 创建审计日志模型文件**

创建 `internal/storage/model/audit.go`:

```go
package model

import (
	"fmt"
	"time"
)

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

// AuditFilter 查询过滤器
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

// AuditStats 审计统计
type AuditStats struct {
	TotalLogs    int64            `json:"total_logs"`    // 总日志数
	TodayLogs    int64            `json:"today_logs"`    // 今日日志数
	ActionCounts map[string]int64 `json:"action_counts"` // 各操作类型数量
	FailedCount  int64            `json:"failed_count"`  // 失败操作数
}

// NewAuditLog 创建审计日志
func NewAuditLog() *AuditLog {
	return &AuditLog{
		ID:        fmt.Sprintf("audit_%d_%s", time.Now().UnixMilli(), generateShortID()),
		Timestamp: time.Now().UnixMilli(),
	}
}

// generateShortID 生成短 ID
func generateShortID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano()%0xFFFFFF)
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/storage/model/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/storage/model/audit.go
git commit -m "feat(model): 添加审计日志数据模型

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: 创建审计日志存储实现

**Files:**
- Create: `internal/storage/kv/audit_store.go`

- [ ] **Step 1: 创建审计日志存储文件**

创建 `internal/storage/kv/audit_store.go`:

```go
package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/storage/model"
)

const auditPrefix = "audit:"

// AuditStore 审计日志存储接口
type AuditStore interface {
	Create(ctx context.Context, log *model.AuditLog) error
	Get(ctx context.Context, id string) (*model.AuditLog, error)
	List(ctx context.Context, filter model.AuditFilter) ([]*model.AuditLog, int64, error)
	DeleteBefore(ctx context.Context, timestamp int64) (int64, error)
	GetStats(ctx context.Context) (*model.AuditStats, error)
}

// KVAuditStore KV 审计日志存储实现
type KVAuditStore struct {
	kv   Store
	mu   sync.RWMutex
}

// NewAuditStore 创建审计日志存储
func NewAuditStore(kv Store) *KVAuditStore {
	return &KVAuditStore{kv: kv}
}

func (s *KVAuditStore) Create(ctx context.Context, log *model.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("marshal audit log: %w", err)
	}

	// 键格式: audit:{timestamp}:{id}
	// 使用时间戳倒序（用一个大数减去时间戳）便于按时间倒序查询
	key := []byte(fmt.Sprintf("%s%019d:%s", auditPrefix, maxTimestamp-log.Timestamp, log.ID))
	return s.kv.Put(key, data)
}

func (s *KVAuditStore) Get(ctx context.Context, id string) (*model.AuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 需要扫描查找，因为 ID 嵌入在键中
	prefix := []byte(auditPrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return nil, fmt.Errorf("scan audit logs: %w", err)
	}

	for key, data := range items {
		var log model.AuditLog
		if err := json.Unmarshal(data, &log); err != nil {
			continue
		}
		if log.ID == id {
			return &log, nil
		}
		// 键中包含 ID，检查键
		if strings.Contains(string(key), id) {
			return &log, nil
		}
	}

	return nil, fmt.Errorf("audit log not found: %s", id)
}

func (s *KVAuditStore) List(ctx context.Context, filter model.AuditFilter) ([]*model.AuditLog, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := []byte(auditPrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return nil, 0, fmt.Errorf("scan audit logs: %w", err)
	}

	// 解析并过滤
	var logs []*model.AuditLog
	for _, data := range items {
		var log model.AuditLog
		if err := json.Unmarshal(data, &log); err != nil {
			continue
		}

		// 应用过滤器
		if !s.matchFilter(&log, filter) {
			continue
		}

		logs = append(logs, &log)
	}

	// 按时间戳倒序排序（最新的在前）
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp > logs[j].Timestamp
	})

	total := int64(len(logs))

	// 应用分页
	if filter.Offset > 0 {
		if filter.Offset >= len(logs) {
			return []*model.AuditLog{}, total, nil
		}
		logs = logs[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(logs) {
		logs = logs[:filter.Limit]
	}

	return logs, total, nil
}

func (s *KVAuditStore) matchFilter(log *model.AuditLog, filter model.AuditFilter) bool {
	// 操作者过滤
	if filter.OperatorPubkey != "" && log.OperatorPubkey != filter.OperatorPubkey {
		return false
	}

	// 操作类型过滤
	if len(filter.ActionTypes) > 0 {
		found := false
		for _, t := range filter.ActionTypes {
			if log.ActionType == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 目标 ID 过滤
	if filter.TargetID != "" && log.TargetID != filter.TargetID {
		return false
	}

	// 成功/失败过滤
	if filter.Success != nil && log.Success != *filter.Success {
		return false
	}

	// 时间范围过滤
	if filter.StartTime > 0 && log.Timestamp < filter.StartTime {
		return false
	}
	if filter.EndTime > 0 && log.Timestamp > filter.EndTime {
		return false
	}

	return true
}

func (s *KVAuditStore) DeleteBefore(ctx context.Context, timestamp int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefix := []byte(auditPrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return 0, fmt.Errorf("scan audit logs: %w", err)
	}

	var deleted int64
	for key, data := range items {
		var log model.AuditLog
		if err := json.Unmarshal(data, &log); err != nil {
			continue
		}
		if log.Timestamp < timestamp {
			if err := s.kv.Delete([]byte(key)); err != nil {
				continue
			}
			deleted++
		}
	}

	return deleted, nil
}

func (s *KVAuditStore) GetStats(ctx context.Context) (*model.AuditStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := []byte(auditPrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return nil, fmt.Errorf("scan audit logs: %w", err)
	}

	stats := &model.AuditStats{
		ActionCounts: make(map[string]int64),
	}

	// 计算今日开始时间戳
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UnixMilli()

	for _, data := range items {
		var log model.AuditLog
		if err := json.Unmarshal(data, &log); err != nil {
			continue
		}

		stats.TotalLogs++
		if log.Timestamp >= todayStart {
			stats.TodayLogs++
		}
		if !log.Success {
			stats.FailedCount++
		}
		stats.ActionCounts[log.ActionType]++
	}

	return stats, nil
}

// 用于时间戳倒序存储
const maxTimestamp = 9999999999999 // 最大的13位时间戳
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/storage/kv/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/storage/kv/audit_store.go
git commit -m "feat(storage): 添加审计日志存储实现

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: 更新 Store 结构添加 AuditStore

**Files:**
- Modify: `internal/storage/store.go`

- [ ] **Step 1: 在 Store 结构体中添加 Audit 字段**

在 `internal/storage/store.go` 的 `Store` 结构体中添加 `Audit` 字段:

找到 `Store` 结构体定义（约第 102-111 行），修改为:

```go
// Store 存储接口集合
type Store struct {
	Entry    EntryStore
	User     UserStore
	Rating   RatingStore
	Category CategoryStore
	Search   index.SearchEngine
	Backlink BacklinkIndex
	Audit    kv.AuditStore // 审计日志存储
	kvStore  kv.Store      // underlying KV store for cleanup
}
```

- [ ] **Step 2: 更新 NewMemoryStore 函数**

在 `NewMemoryStore` 函数中添加 AuditStore 初始化:

```go
// NewMemoryStore 创建内存存储实例
func NewMemoryStore() (*Store, error) {
	entryStore := NewMemoryEntryStore()
	userStore := NewMemoryUserStore()
	ratingStore := NewMemoryRatingStore()
	categoryStore := NewMemoryCategoryStore()
	searchEngine := NewMemorySearchEngine()
	backlinkIndex := NewMemoryBacklinkIndex()

	// 创建内存 KV 存储用于审计日志
	memKV := kv.NewMemoryKVStore()

	return &Store{
		Entry:    entryStore,
		User:     userStore,
		Rating:   ratingStore,
		Category: categoryStore,
		Search:   searchEngine,
		Backlink: backlinkIndex,
		Audit:    kv.NewAuditStore(memKV),
	}, nil
}
```

- [ ] **Step 3: 更新 NewPersistentStore 函数**

在 `NewPersistentStore` 函数中添加 AuditStore 初始化:

找到 `NewPersistentStore` 函数（约第 144-185 行），修改返回部分:

```go
	// 使用适配器组装存储
	return &Store{
		Entry:    NewBadgerEntryStore(kvStore),
		User:     NewBadgerUserStore(kvStore),
		Rating:   NewBadgerRatingStore(kvStore),
		Category: NewBadgerCategoryStore(kvStore),
		Search:   searchEngine,
		Backlink: NewMemoryBacklinkIndex(),
		Audit:    kv.NewAuditStore(kvStore), // 共享 KV 存储
		kvStore:  kvStore,
	}, nil
```

- [ ] **Step 4: 验证编译**

Run: `go build ./internal/storage/...`
Expected: 编译成功

- [ ] **Step 5: 提交**

```bash
git add internal/storage/store.go
git commit -m "feat(storage): 在 Store 中添加 AuditStore

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: 创建审计服务

**Files:**
- Create: `internal/core/audit/audit.go`

- [ ] **Step 1: 创建审计服务目录和文件**

创建 `internal/core/audit/audit.go`:

```go
// Package audit 提供审计日志服务
package audit

import (
	"context"
	"regexp"
	"strings"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// 敏感字段脱敏规则
var sensitiveFields = []string{
	"password", "passwd", "pwd",
	"private_key", "privateKey", "private-key",
	"secret", "token", "api_key", "apiKey",
	"code", "verification_code",
}

// 脱敏正则
var sensitivePatterns []*regexp.Regexp

func init() {
	for _, field := range sensitiveFields {
		// 匹配 "field": "value" 或 "field":"value"
		pattern := regexp.MustCompile(`(?i)"` + field + `"\s*:\s*"[^"]*"`)
		sensitivePatterns = append(sensitivePatterns, pattern)
	}
}

// Service 审计服务
type Service struct {
	store kv.AuditStore
}

// NewService 创建审计服务
func NewService(store kv.AuditStore) *Service {
	return &Service{store: store}
}

// Log 记录审计日志
func (s *Service) Log(ctx context.Context, log *model.AuditLog) error {
	// 脱敏请求体
	log.RequestBody = MaskSensitiveFields(log.RequestBody)
	log.ResponseBody = TruncateString(log.ResponseBody, 4096) // 4KB
	log.RequestBody = TruncateString(log.RequestBody, 16384)  // 16KB

	return s.store.Create(ctx, log)
}

// List 查询审计日志
func (s *Service) List(ctx context.Context, filter model.AuditFilter) ([]*model.AuditLog, int64, error) {
	return s.store.List(ctx, filter)
}

// GetStats 获取审计统计
func (s *Service) GetStats(ctx context.Context) (*model.AuditStats, error) {
	return s.store.GetStats(ctx)
}

// DeleteBefore 删除指定时间之前的日志
func (s *Service) DeleteBefore(ctx context.Context, timestamp int64) (int64, error) {
	return s.store.DeleteBefore(ctx, timestamp)
}

// MaskSensitiveFields 脱敏敏感字段
func MaskSensitiveFields(jsonStr string) string {
	if jsonStr == "" {
		return ""
	}

	result := jsonStr
	for _, pattern := range sensitivePatterns {
		// 替换为 "***"
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// 保留字段名，替换值为 ***
			parts := strings.SplitN(match, ":", 2)
			if len(parts) == 2 {
				return parts[0] + `: "***"`
			}
			return match
		})
	}
	return result
}

// TruncateString 截断字符串
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[TRUNCATED]"
}

// GetActionType 获取操作类型
func GetActionType(method, path string) string {
	// 敏感操作路径映射
	sensitiveOps := map[string]map[string]string{
		"POST": {
			"/api/v1/user/register":     "user.register",
			"/api/v1/user/verify-email": "user.verify_email",
			"/api/v1/user/update":       "user.update",
			"/api/v1/entry/create":      "entry.create",
			"/api/v1/categories/create": "category.create",
			"/api/v1/elections":         "election.create",
			"/api/v1/batch/create":      "batch.create",
			"/api/v1/batch/update":      "batch.update",
			"/api/v1/batch/delete":      "batch.delete",
			"/api/v1/admin/import":      "admin.import",
		},
		"PUT": {
			"/api/v1/admin/users/": "/level": "admin.user_level",
		},
		"GET": {
			"/api/v1/admin/export": "admin.export",
		},
	}

	// 精确匹配
	if methodOps, ok := sensitiveOps[method]; ok {
		for pattern, action := range methodOps {
			if path == pattern {
				return action
			}
		}
	}

	// 前缀匹配
	if method == "POST" {
		if strings.HasPrefix(path, "/api/v1/entry/update/") {
			return "entry.update"
		}
		if strings.HasPrefix(path, "/api/v1/entry/delete/") {
			return "entry.delete"
		}
		if strings.HasPrefix(path, "/api/v1/entry/rate/") {
			return "entry.rate"
		}
		if strings.HasPrefix(path, "/api/v1/admin/users/") && strings.HasSuffix(path, "/ban") {
			return "admin.user_ban"
		}
		if strings.HasPrefix(path, "/api/v1/admin/users/") && strings.HasSuffix(path, "/unban") {
			return "admin.user_unban"
		}
		if strings.HasPrefix(path, "/api/v1/elections/") && strings.HasSuffix(path, "/vote") {
			return "election.vote"
		}
		if strings.HasPrefix(path, "/api/v1/elections/") && strings.HasSuffix(path, "/close") {
			return "election.close"
		}
	}
	if method == "PUT" {
		if strings.HasPrefix(path, "/api/v1/admin/users/") && strings.HasSuffix(path, "/level") {
			return "admin.user_level"
		}
	}

	return ""
}

// IsSensitiveOperation 检查是否为敏感操作
func IsSensitiveOperation(method, path string) bool {
	return GetActionType(method, path) != ""
}

// ExtractTargetID 从路径提取目标 ID
func ExtractTargetID(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 5 {
		// /api/v1/entry/update/entry-id
		// /api/v1/admin/users/user-pk/ban
		switch {
		case strings.HasPrefix(path, "/api/v1/entry/"):
			return parts[4] // entry-id
		case strings.HasPrefix(path, "/api/v1/admin/users/"):
			return parts[5] // user-pk
		case strings.HasPrefix(path, "/api/v1/elections/"):
			if len(parts) >= 5 {
				return parts[4] // election-id
			}
		}
	}
	return ""
}

// GetTargetType 从操作类型获取目标类型
func GetTargetType(actionType string) string {
	if strings.HasPrefix(actionType, "entry.") {
		return "entry"
	}
	if strings.HasPrefix(actionType, "user.") {
		return "user"
	}
	if strings.HasPrefix(actionType, "category.") {
		return "category"
	}
	if strings.HasPrefix(actionType, "admin.") {
		return "admin"
	}
	if strings.HasPrefix(actionType, "election.") {
		return "election"
	}
	if strings.HasPrefix(actionType, "batch.") {
		return "batch"
	}
	return ""
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/core/audit/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/core/audit/audit.go
git commit -m "feat(audit): 添加审计服务

- 日志记录、查询、统计功能
- 敏感字段脱敏
- 路径匹配判断

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: 创建审计中间件

**Files:**
- Create: `internal/api/middleware/audit.go`

- [ ] **Step 1: 创建审计中间件文件**

创建 `internal/api/middleware/audit.go`:

```go
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/core/audit"
	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// AuditMiddleware 审计中间件
type AuditMiddleware struct {
	auditSvc *audit.Service
}

// NewAuditMiddleware 创建审计中间件
func NewAuditMiddleware(auditStore kv.AuditStore) *AuditMiddleware {
	return &AuditMiddleware{
		auditSvc: audit.NewService(auditStore),
	}
}

// responseWriter 包装 ResponseWriter 以捕获响应
type responseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	mu     sync.Mutex
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// Middleware 返回审计中间件处理函数
func (m *AuditMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. 检查是否为敏感操作
		actionType := audit.GetActionType(r.Method, r.URL.Path)
		if actionType == "" {
			next.ServeHTTP(w, r)
			return
		}

		// 2. 缓冲请求体
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 限制 1MB
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// 3. 提取操作者信息
		operatorPubkey, _ := r.Context().Value(PublicKeyKey).(string)
		operatorLevel, _ := r.Context().Value(UserLevelKey).(int32)

		// 4. 包装 ResponseWriter 捕获响应
		rw := &responseWriter{
			ResponseWriter: w,
			status:         200,
		}

		// 5. 调用下一个 Handler
		next.ServeHTTP(rw, r)

		// 6. 异步写入审计日志
		go m.writeAuditLog(r.Context(), &model.AuditLog{
			Timestamp:      time.Now().UnixMilli(),
			OperatorPubkey: operatorPubkey,
			OperatorLevel:  operatorLevel,
			OperatorIP:     getClientIP(r),
			UserAgent:      r.UserAgent(),
			Method:         r.Method,
			Path:           r.URL.Path,
			ActionType:     actionType,
			TargetID:       audit.ExtractTargetID(r.URL.Path),
			TargetType:     audit.GetTargetType(actionType),
			RequestBody:    string(bodyBytes),
			ResponseCode:   rw.status,
			ResponseBody:   rw.body.String(),
			Success:        rw.status < 400,
			ErrorMessage:   getErrorMessage(rw.body.String()),
		})
	})
}

// writeAuditLog 写入审计日志
func (m *AuditMiddleware) writeAuditLog(ctx context.Context, log *model.AuditLog) {
	log.ID = generateAuditID()
	if err := m.auditSvc.Log(ctx, log); err != nil {
		// 记录失败不影响主流程，只打印日志
		// logger.Error("failed to write audit log", "error", err)
	}
}

// generateAuditID 生成审计日志 ID
func generateAuditID() string {
	return model.NewAuditLog().ID
}

// getClientIP 获取客户端 IP
func getClientIP(r *http.Request) string {
	// 尝试从 X-Forwarded-For 获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	// 尝试从 X-Real-IP 获取
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// 使用 RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// getErrorMessage 从响应体提取错误信息
func getErrorMessage(responseBody string) string {
	if responseBody == "" {
		return ""
	}

	var resp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(responseBody), &resp); err != nil {
		return ""
	}
	return resp.Message
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/api/middleware/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/api/middleware/audit.go
git commit -m "feat(middleware): 添加审计中间件

- 拦截敏感操作请求
- 捕获请求/响应
- 异步写入日志

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: 创建审计 API Handler

**Files:**
- Create: `internal/api/handler/audit_handler.go`

- [ ] **Step 1: 创建审计 API Handler 文件**

创建 `internal/api/handler/audit_handler.go`:

```go
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/daifei0527/polyant/internal/core/audit"
	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
	awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// AuditHandler 审计 API 处理器
type AuditHandler struct {
	auditSvc *audit.Service
}

// NewAuditHandler 创建审计处理器
func NewAuditHandler(auditStore kv.AuditStore) *AuditHandler {
	return &AuditHandler{
		auditSvc: audit.NewService(auditStore),
	}
}

// ListAuditLogsHandler 查询审计日志
// GET /api/v1/admin/audit/logs
func (h *AuditHandler) ListAuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	// 解析查询参数
	filter := model.AuditFilter{
		OperatorPubkey: r.URL.Query().Get("operator"),
		TargetID:       r.URL.Query().Get("target_id"),
	}

	// 解析操作类型（逗号分隔）
	if actions := r.URL.Query().Get("action"); actions != "" {
		filter.ActionTypes = strings.Split(actions, ",")
	}

	// 解析成功/失败
	if success := r.URL.Query().Get("success"); success != "" {
		b := success == "true"
		filter.Success = &b
	}

	// 解析时间范围
	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		filter.StartTime, _ = strconv.ParseInt(startTime, 10, 64)
	}
	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		filter.EndTime, _ = strconv.ParseInt(endTime, 10, 64)
	}

	// 解析分页
	filter.Limit = 50
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 200 {
			filter.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		filter.Offset, _ = strconv.Atoi(offset)
	}

	// 查询日志
	logs, total, err := h.auditSvc.List(r.Context(), filter)
	if err != nil {
		writeError(w, awerrors.Wrap(900, awerrors.CategoryAPI, err.Error(), http.StatusInternalServerError, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total_count": total,
			"has_more":    int64(filter.Offset+len(logs)) < total,
			"items":       logs,
		},
	})
}

// GetAuditStatsHandler 获取审计统计
// GET /api/v1/admin/audit/stats
func (h *AuditHandler) GetAuditStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	stats, err := h.auditSvc.GetStats(r.Context())
	if err != nil {
		writeError(w, awerrors.Wrap(901, awerrors.CategoryAPI, err.Error(), http.StatusInternalServerError, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// DeleteAuditLogsHandler 删除审计日志
// DELETE /api/v1/admin/audit/logs?before={timestamp}
func (h *AuditHandler) DeleteAuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	beforeStr := r.URL.Query().Get("before")
	if beforeStr == "" {
		writeError(w, awerrors.New(101, awerrors.CategoryAPI, "missing 'before' parameter", http.StatusBadRequest))
		return
	}

	before, err := strconv.ParseInt(beforeStr, 10, 64)
	if err != nil {
		writeError(w, awerrors.New(102, awerrors.CategoryAPI, "invalid 'before' timestamp", http.StatusBadRequest))
		return
	}

	deleted, err := h.auditSvc.DeleteBefore(r.Context(), before)
	if err != nil {
		writeError(w, awerrors.Wrap(902, awerrors.CategoryAPI, err.Error(), http.StatusInternalServerError, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data: map[string]int64{
			"deleted_count": deleted,
		},
	})
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/api/handler/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/audit_handler.go
git commit -m "feat(api): 添加审计日志查询 API

- GET /api/v1/admin/audit/logs 查询日志
- GET /api/v1/admin/audit/stats 获取统计
- DELETE /api/v1/admin/audit/logs 删除日志

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 7: 注册中间件和路由

**Files:**
- Modify: `internal/api/router/router.go`

- [ ] **Step 1: 在 NewRouterWithDeps 中创建 AuditHandler 和 AuditMiddleware**

在 `internal/api/router/router.go` 的 `NewRouterWithDeps` 函数中，找到创建 handler 的位置（约第 95-110 行），在 batchHandler 创建之后添加:

```go
	// 创建审计 handler 和中间件
	var auditHandler *handler.AuditHandler
	var auditMW *middleware.AuditMiddleware
	if deps.Store != nil && deps.Store.Audit != nil {
		auditHandler = handler.NewAuditHandler(deps.Store.Audit)
		auditMW = middleware.NewAuditMiddleware(deps.Store.Audit)
	}
```

- [ ] **Step 2: 在 registerAuthRoutes 函数签名中添加参数**

修改 `registerAuthRoutes` 函数签名（约第 241 行）:

```go
func registerAuthRoutes(mux *http.ServeMux, authMW *middleware.AuthMiddleware, eh *handler.EntryHandler, uh *handler.UserHandler, ch *handler.CategoryHandler, nh *handler.NodeHandler, ah *handler.AdminHandler, elh *handler.ElectionHandler, bh *handler.BatchHandler, exh *handler.ExportHandler, auh *handler.AuditHandler) {
```

- [ ] **Step 3: 在 registerAuthRoutes 中添加审计路由**

在管理员路由部分（`if ah != nil` 块内），添加审计路由:

```go
		// ==================== 审计日志路由 ====================
		if auh != nil {
			// 查询审计日志 GET /api/v1/admin/audit/logs - Lv5 (SuperAdmin)
			mux.Handle("/api/v1/admin/audit/logs", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv5, http.HandlerFunc(auh.ListAuditLogsHandler))))

			// 获取审计统计 GET /api/v1/admin/audit/stats - Lv5 (SuperAdmin)
			mux.Handle("/api/v1/admin/audit/stats", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv5, http.HandlerFunc(auh.GetAuditStatsHandler))))

			// 删除审计日志 DELETE /api/v1/admin/audit/logs - Lv5 (SuperAdmin)
			mux.Handle("/api/v1/admin/audit/logs/delete", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv5, http.HandlerFunc(auh.DeleteAuditLogsHandler))))
		}
```

- [ ] **Step 4: 更新 registerAuthRoutes 调用**

修改调用 `registerAuthRoutes` 的地方:

```go
	registerAuthRoutes(mux, authMW, entryHandler, userHandler, categoryHandler, nodeHandler, adminHandler, electionHandler, batchHandler, exportHandler, auditHandler)
```

- [ ] **Step 5: 在中间件链中添加 AuditMiddleware**

找到中间件链部分（约第 140-150 行），修改为:

```go
	// 应用中间件链
	var httpHandler http.Handler = mux
	if auditMW != nil {
		httpHandler = auditMW.Middleware(httpHandler)
	}
```

- [ ] **Step 6: 验证编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 7: 提交**

```bash
git add internal/api/router/router.go
git commit -m "feat(router): 注册审计中间件和路由

- 添加审计日志查询 API (Lv5)
- 集成审计中间件到请求处理链

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 8: 编写测试

**Files:**
- Create: `internal/core/audit/audit_test.go`

- [ ] **Step 1: 创建审计服务测试文件**

创建 `internal/core/audit/audit_test.go`:

```go
package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSensitiveFields(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "mask password",
			input:    `{"username":"test","password":"secret123"}`,
			expected: `{"username":"test","password": "***"}`,
		},
		{
			name:     "mask private_key",
			input:    `{"public_key":"abc","private_key":"secret"}`,
			expected: `{"public_key":"abc","private_key": "***"}`,
		},
		{
			name:     "mask verification_code",
			input:    `{"email":"test@example.com","code":"123456"}`,
			expected: `{"email":"test@example.com","code": "***"}`,
		},
		{
			name:     "no sensitive fields",
			input:    `{"name":"test","value":"data"}`,
			expected: `{"name":"test","value":"data"}`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveFields(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string",
			input:    "hello world this is a long string",
			maxLen:   10,
			expected: "hello worl...[TRUNCATED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetActionType(t *testing.T) {
	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{"POST", "/api/v1/entry/create", "entry.create"},
		{"POST", "/api/v1/entry/update/entry-123", "entry.update"},
		{"POST", "/api/v1/entry/delete/entry-123", "entry.delete"},
		{"POST", "/api/v1/entry/rate/entry-123", "entry.rate"},
		{"POST", "/api/v1/user/register", "user.register"},
		{"POST", "/api/v1/admin/users/user-pk/ban", "admin.user_ban"},
		{"POST", "/api/v1/admin/users/user-pk/unban", "admin.user_unban"},
		{"PUT", "/api/v1/admin/users/user-pk/level", "admin.user_level"},
		{"GET", "/api/v1/admin/export", "admin.export"},
		{"GET", "/api/v1/search", ""}, // 非敏感操作
		{"GET", "/api/v1/entry/entry-123", ""}, // 非敏感操作
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			result := GetActionType(tt.method, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractTargetID(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/api/v1/entry/update/entry-123", "entry-123"},
		{"/api/v1/entry/delete/entry-456", "entry-456"},
		{"/api/v1/admin/users/user-pk/ban", "user-pk"},
		{"/api/v1/elections/election-1/vote", "election-1"},
		{"/api/v1/entry/create", ""},
		{"/api/v1/search", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := ExtractTargetID(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTargetType(t *testing.T) {
	tests := []struct {
		actionType string
		expected   string
	}{
		{"entry.create", "entry"},
		{"entry.update", "entry"},
		{"user.register", "user"},
		{"admin.user_ban", "admin"},
		{"election.vote", "election"},
		{"batch.create", "batch"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.actionType, func(t *testing.T) {
			result := GetTargetType(tt.actionType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSensitiveOperation(t *testing.T) {
	assert.True(t, IsSensitiveOperation("POST", "/api/v1/entry/create"))
	assert.True(t, IsSensitiveOperation("POST", "/api/v1/admin/users/user-pk/ban"))
	assert.False(t, IsSensitiveOperation("GET", "/api/v1/search"))
	assert.False(t, IsSensitiveOperation("GET", "/api/v1/entry/entry-123"))
}
```

- [ ] **Step 2: 运行测试**

Run: `go test ./internal/core/audit/... -v`
Expected: 所有测试通过

- [ ] **Step 3: 提交**

```bash
git add internal/core/audit/audit_test.go
git commit -m "test(audit): 添加审计服务测试

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 9: 运行完整测试套件

**Files:**
- 无新增文件

- [ ] **Step 1: 运行所有测试**

Run: `go test ./... -count=1`
Expected: 所有测试通过

- [ ] **Step 2: 运行测试覆盖率**

Run: `go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -1`
Expected: 覆盖率 > 55%

- [ ] **Step 3: 最终提交**

```bash
git add .
git commit -m "feat: Phase 8 审计日志系统完成

功能:
- 中间件拦截所有敏感操作
- 完整请求/响应记录（脱敏处理）
- 查询、统计、删除 API
- 仅 Lv5 超级管理员可访问

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## 验收清单

- [ ] `internal/storage/model/audit.go` 定义审计日志数据模型
- [ ] `internal/storage/kv/audit_store.go` 实现审计日志存储
- [ ] `internal/storage/store.go` 添加 AuditStore 字段
- [ ] `internal/core/audit/audit.go` 实现审计服务和脱敏逻辑
- [ ] `internal/api/middleware/audit.go` 实现审计中间件
- [ ] `internal/api/handler/audit_handler.go` 实现审计 API
- [ ] `internal/api/router/router.go` 注册中间件和路由
- [ ] 敏感操作路径正确匹配
- [ ] 敏感字段正确脱敏
- [ ] 长内容正确截断
- [ ] 所有测试通过
- [ ] 测试覆盖率 > 55%
