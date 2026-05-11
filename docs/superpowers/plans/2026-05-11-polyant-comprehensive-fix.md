# Polyant 项目全面修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 Polyant 项目中的现有问题，补全缺失功能（ListUpdatedAfter 接口），补充缺失测试，提升代码覆盖率。

**Architecture:** 采用顺序实施策略：先实现 RatingStore 的 `ListRatedAfter` 接口用于增量评级同步，然后为缺失测试的处理器添加测试，最后提升整体覆盖率。所有测试使用内存存储，遵循项目现有的 testify + httptest 模式。

**Tech Stack:** Go 1.22+, testify, httptest, BadgerDB/PebbleKV

---

## 文件结构

### 阶段1：ListRatedAfter 接口

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 修改 | `internal/storage/store.go:66-75` | 在 RatingStore 接口添加 ListRatedAfter 方法 |
| 修改 | `internal/storage/memory.go:325-390` | MemoryRatingStore 实现 ListRatedAfter |
| 修改 | `internal/storage/kv/badger_store.go` | BadgerRatingStore 实现 ListRatedAfter |
| 修改 | `internal/network/sync/sync.go:449-452` | 使用新接口同步评级 |
| 创建 | `internal/storage/memory_rating_test.go` | MemoryRatingStore.ListRatedAfter 测试 |

### 阶段2：处理器测试补全

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 创建 | `internal/api/handler/category_handler_test.go` | CategoryHandler 测试 |
| 创建 | `internal/api/handler/user_handler_test.go` | UserHandler 测试 |
| 创建 | `internal/api/handler/node_handler_test.go` | NodeHandler 测试 |
| 创建 | `internal/api/handler/helpers_test.go` | 辅助函数测试 |

### 阶段3：admin/static 测试

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 创建 | `internal/api/admin/static_test.go` | StaticHandler 测试 |

---

## Task 1: 在 RatingStore 接口添加 ListRatedAfter 方法

**Files:**
- Modify: `internal/storage/store.go:66-75`

- [ ] **Step 1: 在 RatingStore 接口中添加 ListRatedAfter 方法**

在 `internal/storage/store.go` 的 RatingStore 接口中添加新方法：

```go
// RatingStore 评分存储接口
type RatingStore interface {
	// Create 创建评分记录
	Create(ctx context.Context, rating *model.Rating) (*model.Rating, error)
	// Get 根据ID获取评分
	Get(ctx context.Context, id string) (*model.Rating, error)
	// ListByEntry 获取条目的所有评分
	ListByEntry(ctx context.Context, entryID string) ([]*model.Rating, error)
	// GetByRater 获取评分者对某条目的评分（检查重复评分）
	GetByRater(ctx context.Context, entryID, raterPubkeyHash string) (*model.Rating, error)
	// ListRatedAfter 获取指定时间戳之后创建的所有评分（用于增量同步）
	ListRatedAfter(ctx context.Context, after int64) ([]*model.Rating, error)
}
```

- [ ] **Step 2: 验证编译通过**

Run: `go build ./...`
Expected: 编译失败，因为 MemoryRatingStore 和 BadgerRatingStore 未实现新方法

- [ ] **Step 3: 提交接口变更**

```bash
git add internal/storage/store.go
git commit -m "feat(storage): add ListRatedAfter to RatingStore interface"
```

---

## Task 2: MemoryRatingStore 实现 ListRatedAfter

**Files:**
- Modify: `internal/storage/memory.go` (MemoryRatingStore 部分)
- Create: `internal/storage/memory_rating_test.go`

- [ ] **Step 1: 编写 MemoryRatingStore.ListRatedAfter 的失败测试**

创建 `internal/storage/memory_rating_test.go`：

```go
package storage

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestMemoryRatingStore_ListRatedAfter(t *testing.T) {
	store := NewMemoryRatingStore()
	ctx := context.Background()

	// 创建两个评分，时间戳不同
	rating1 := &model.Rating{
		ID:          "r1",
		EntryId:     "e1",
		RaterPubkey: "pub1",
		Score:       4.0,
		RatedAt:     1000,
	}
	rating2 := &model.Rating{
		ID:          "r2",
		EntryId:     "e1",
		RaterPubkey: "pub2",
		Score:       5.0,
		RatedAt:     2000,
	}

	store.Create(ctx, rating1)
	store.Create(ctx, rating2)

	// 查询 1500 之后的评分，应该只有 rating2
	ratings, err := store.ListRatedAfter(ctx, 1500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ratings) != 1 {
		t.Fatalf("expected 1 rating, got %d", len(ratings))
	}
	if ratings[0].ID != "r2" {
		t.Errorf("expected rating r2, got %s", ratings[0].ID)
	}
}

func TestMemoryRatingStore_ListRatedAfter_Empty(t *testing.T) {
	store := NewMemoryRatingStore()
	ctx := context.Background()

	ratings, err := store.ListRatedAfter(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ratings) != 0 {
		t.Fatalf("expected 0 ratings, got %d", len(ratings))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/storage/ -run TestMemoryRatingStore_ListRatedAfter -v`
Expected: 编译失败，ListRatedAfter 方法未定义

- [ ] **Step 3: 在 MemoryRatingStore 中实现 ListRatedAfter**

在 `internal/storage/memory.go` 的 MemoryRatingStore 部分添加：

```go
// ListRatedAfter 获取指定时间戳之后创建的所有评分
func (s *MemoryRatingStore) ListRatedAfter(ctx context.Context, after int64) ([]*model.Rating, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.Rating
	for _, r := range s.ratings {
		if r.RatedAt > after {
			cp := *r
			result = append(result, &cp)
		}
	}
	return result, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/storage/ -run TestMemoryRatingStore_ListRatedAfter -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/storage/memory.go internal/storage/memory_rating_test.go
git commit -m "feat(storage): implement ListRatedAfter for MemoryRatingStore"
```

---

## Task 3: BadgerRatingStore 实现 ListRatedAfter

**Files:**
- Modify: `internal/storage/kv/badger_store.go`

- [ ] **Step 1: 查看 BadgerRatingStore 现有实现**

在 `internal/storage/kv/badger_store.go` 中找到 BadgerRatingStore 的 Create 和 ListByEntry 方法，了解 KV 存储模式。

- [ ] **Step 2: 在 BadgerRatingStore 中实现 ListRatedAfter**

```go
// ListRatedAfter 获取指定时间戳之后创建的所有评分
func (s *BadgerRatingStore) ListRatedAfter(ctx context.Context, after int64) ([]*model.Rating, error) {
	// 使用 prefix scan 遍历所有评分
	ratings, err := s.ListByEntry(ctx, "")
	if err != nil {
		return nil, err
	}

	var result []*model.Rating
	for _, r := range ratings {
		if r.RatedAt > after {
			result = append(result, r)
		}
	}
	return result, nil
}
```

注意：如果 BadgerRatingStore 的 ListByEntry 不支持空字符串前缀扫描，需要实现独立的全量扫描。查看现有实现后决定具体方案。

- [ ] **Step 3: 验证编译通过**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: 运行现有测试确保无回归**

Run: `go test ./internal/storage/... -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/storage/kv/badger_store.go
git commit -m "feat(storage): implement ListRatedAfter for BadgerRatingStore"
```

---

## Task 4: 更新同步逻辑使用 ListRatedAfter

**Files:**
- Modify: `internal/network/sync/sync.go:449-452`

- [ ] **Step 1: 查看同步上下文**

阅读 `internal/network/sync/sync.go` 第 400-460 行，理解同步流程和 TODO 位置。

- [ ] **Step 2: 替换 TODO 为实际实现**

将 `sync.go:449-452` 的 TODO 替换为：

```go
	// 获取指定时间戳之后新增/更新的评分
	if se.store.Rating != nil {
		ratings, err := se.store.Rating.ListRatedAfter(ctx, req.LastSyncTimestamp)
		if err != nil {
			return nil, fmt.Errorf("list ratings: %w", err)
		}
		for _, rating := range ratings {
			data, err := rating.ToJSON()
			if err != nil {
				continue
			}
			resp.NewRatings = append(resp.NewRatings, data)
		}
	}
```

- [ ] **Step 3: 验证编译通过**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: 运行同步相关测试**

Run: `go test ./internal/network/sync/... -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/network/sync/sync.go
git commit -m "feat(sync): use ListRatedAfter for incremental rating sync"
```

---

## Task 5: CategoryHandler 测试

**Files:**
- Create: `internal/api/handler/category_handler_test.go`

- [ ] **Step 1: 编写 ListCategoriesHandler 测试**

创建 `internal/api/handler/category_handler_test.go`：

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestCategoryHandler_ListCategoriesHandler(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewCategoryHandler(memStore.Category, memStore.Entry)

	// 创建测试分类
	ctx := context.Background()
	cat := &model.Category{
		ID:   "cat1",
		Path: "tech",
		Name: "Technology",
	}
	memStore.Category.Create(ctx, cat)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	w := httptest.NewRecorder()
	handler.ListCategoriesHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var apiResp APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)
	assert.Equal(t, 0, apiResp.Code)
	assert.NotNil(t, apiResp.Data)
}

func TestCategoryHandler_GetCategoryEntriesHandler(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewCategoryHandler(memStore.Category, memStore.Entry)

	ctx := context.Background()

	// 创建分类
	cat := &model.Category{
		ID:   "cat1",
		Path: "tech",
		Name: "Technology",
	}
	memStore.Category.Create(ctx, cat)

	// 创建条目
	entry := &model.KnowledgeEntry{
		ID:       "e1",
		Title:    "Test Entry",
		Content:  "Test Content",
		Category: "tech",
		Status:   model.EntryStatusPublished,
	}
	memStore.Entry.Create(ctx, entry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories/tech/entries", nil)
	w := httptest.NewRecorder()
	handler.GetCategoryEntriesHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var apiResp APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)
	assert.Equal(t, 0, apiResp.Code)
}

func TestCategoryHandler_GetCategoryEntriesHandler_NotFound(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewCategoryHandler(memStore.Category, memStore.Entry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories/nonexistent/entries", nil)
	w := httptest.NewRecorder()
	handler.GetCategoryEntriesHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCategoryHandler_CreateCategoryHandler_NoAuth(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewCategoryHandler(memStore.Category, memStore.Entry)

	body := `{"path":"tech/new","name":"New Tech"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/categories/create", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateCategoryHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCategoryHandler_CreateCategoryHandler_Success(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewCategoryHandler(memStore.Category, memStore.Entry)

	// 创建具有 Lv2 权限的用户
	user := &model.User{
		PublicKey: "testkey",
		UserLevel: model.UserLevelLv2,
		Status:    model.UserStatusActive,
	}

	body := `{"path":"tech/new","name":"New Tech"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/categories/create", strings.NewReader(body))
	// 将用户设置到上下文中
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.CreateCategoryHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var apiResp APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)
	assert.Equal(t, 0, apiResp.Code)
}
```

- [ ] **Step 2: 运行测试确认通过**

Run: `go test ./internal/api/handler/ -run TestCategoryHandler -v`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/category_handler_test.go
git commit -m "test(handler): add CategoryHandler tests"
```

---

## Task 6: UserHandler 测试

**Files:**
- Create: `internal/api/handler/user_handler_test.go`

- [ ] **Step 1: 编写 UserHandler 测试**

创建 `internal/api/handler/user_handler_test.go`：

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestUserHandler_RegisterHandler(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewUserHandler(memStore, memStore.User, memStore.Entry, memStore.Rating, nil, nil)

	body := `{"agent_name":"test-agent","node_id":"node1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/register", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.RegisterHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var apiResp APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)
	assert.Equal(t, 0, apiResp.Code)

	data, ok := apiResp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, data["public_key"])
	assert.NotEmpty(t, data["private_key"])
	assert.Equal(t, "test-agent", data["agent_name"])
}

func TestUserHandler_RegisterHandler_MissingAgentName(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewUserHandler(memStore, memStore.User, memStore.Entry, memStore.Rating, nil, nil)

	body := `{"node_id":"node1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/register", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.RegisterHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUserHandler_GetUserInfoHandler_NoAuth(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewUserHandler(memStore, memStore.User, memStore.Entry, memStore.Rating, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/info", nil)
	w := httptest.NewRecorder()
	handler.GetUserInfoHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestUserHandler_UpdateUserInfoHandler_Success(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewUserHandler(memStore, memStore.User, memStore.Entry, memStore.Rating, nil, nil)

	// 创建用户
	user := &model.User{
		PublicKey: "testkey",
		AgentName: "old-name",
		UserLevel: model.UserLevelLv0,
		Status:    model.UserStatusActive,
	}
	memStore.User.Create(context.Background(), user)

	body := `{"agent_name":"new-name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/info", strings.NewReader(body))
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.UpdateUserInfoHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUserHandler_RateEntryHandler_Success(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewUserHandler(memStore, memStore.User, memStore.Entry, memStore.Rating, nil, nil)

	ctx := context.Background()

	// 创建用户 (Lv1 可以评分)
	user := &model.User{
		PublicKey: "testkey",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusActive,
	}
	memStore.User.Create(ctx, user)

	// 创建条目
	entry := &model.KnowledgeEntry{
		ID:     "e1",
		Title:  "Test",
		Status: model.EntryStatusPublished,
	}
	memStore.Entry.Create(ctx, entry)

	body := `{"score":4.5,"comment":"great"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/e1/rate", strings.NewReader(body))
	ctx = setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.RateEntryHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestUserHandler_RateEntryHandler_Lv0Denied(t *testing.T) {
	memStore, err := storage.NewMemoryStore()
	require.NoError(t, err)

	handler := NewUserHandler(memStore, memStore.User, memStore.Entry, memStore.Rating, nil, nil)

	ctx := context.Background()

	// Lv0 用户不能评分
	user := &model.User{
		PublicKey: "testkey",
		UserLevel: model.UserLevelLv0,
		Status:    model.UserStatusActive,
	}
	memStore.User.Create(ctx, user)

	entry := &model.KnowledgeEntry{
		ID:     "e1",
		Title:  "Test",
		Status: model.EntryStatusPublished,
	}
	memStore.Entry.Create(ctx, entry)

	body := `{"score":4.5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entry/e1/rate", strings.NewReader(body))
	ctx = setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.RateEntryHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
```

- [ ] **Step 2: 运行测试确认通过**

Run: `go test ./internal/api/handler/ -run TestUserHandler -v`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/user_handler_test.go
git commit -m "test(handler): add UserHandler tests"
```

---

## Task 7: NodeHandler 测试

**Files:**
- Create: `internal/api/handler/node_handler_test.go`

- [ ] **Step 1: 编写 NodeHandler 测试**

创建 `internal/api/handler/node_handler_test.go`：

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeHandler_GetNodeStatusHandler(t *testing.T) {
	handler := NewNodeHandler("node1", "seed", "1.0.0", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/status", nil)
	w := httptest.NewRecorder()
	handler.GetNodeStatusHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var apiResp APIResponse
	err := json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)
	assert.Equal(t, 0, apiResp.Code)

	data, ok := apiResp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "node1", data["node_id"])
	assert.Equal(t, "seed", data["node_type"])
	assert.Equal(t, "1.0.0", data["version"])
}

func TestNodeHandler_TriggerSyncHandler(t *testing.T) {
	handler := NewNodeHandler("node1", "seed", "1.0.0", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/sync", nil)
	w := httptest.NewRecorder()
	handler.TriggerSyncHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var apiResp APIResponse
	err := json.NewDecoder(resp.Body).Decode(&apiResp)
	require.NoError(t, err)
	assert.Equal(t, 0, apiResp.Code)

	data, ok := apiResp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "syncing", data["status"])
	assert.NotZero(t, data["triggered_at"])
}

func TestNodeHandler_SetLastSync(t *testing.T) {
	handler := NewNodeHandler("node1", "seed", "1.0.0", nil)

	handler.SetLastSync(12345)
	assert.Equal(t, int64(12345), handler.lastSync)
}
```

- [ ] **Step 2: 运行测试确认通过**

Run: `go test ./internal/api/handler/ -run TestNodeHandler -v`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/node_handler_test.go
git commit -m "test(handler): add NodeHandler tests"
```

---

## Task 8: helpers.go 测试

**Files:**
- Create: `internal/api/handler/helpers_test.go`

- [ ] **Step 1: 编写辅助函数测试**

创建 `internal/api/handler/helpers_test.go`：

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractPathVar_ID(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple", "/api/v1/entry/abc123", "abc123"},
		{"with trailing slash", "/api/v1/entry/abc123/", "abc123"},
		{"nested", "/api/v1/entry/abc123/backlinks", "abc123"},
		{"empty", "/api/v1/entry/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			result := extractPathVar(req, "id")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPathVar_Path(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple", "/api/v1/categories/tech/entries", "tech"},
		{"nested", "/api/v1/categories/tech/programming/go/entries", "tech/programming/go"},
		{"empty", "/api/v1/categories//entries", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			result := extractPathVar(req, "path")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateUUID(t *testing.T) {
	uuid1 := generateUUID()
	uuid2 := generateUUID()

	assert.NotEmpty(t, uuid1)
	assert.NotEmpty(t, uuid2)
	assert.NotEqual(t, uuid1, uuid2)
	assert.Len(t, uuid1, 36) // UUID v4 format: 8-4-4-4-12
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasErr   bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"-1", -1, false},
		{"abc", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseInt(tt.input)
			if tt.hasErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	err := awerrors.New(100, "test", "test error", http.StatusBadRequest, nil)
	writeError(w, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected bool
	}{
		{"test@example.com", true},
		{"user@domain.org", true},
		{"invalid", false},
		{"no-at-sign", false},
		{"no-dot@com", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := isValidEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}
```

- [ ] **Step 2: 运行测试确认通过**

Run: `go test ./internal/api/handler/ -run "TestExtractPathVar|TestGenerateUUID|TestParseInt|TestWriteJSON|TestWriteError|TestIsValidEmail" -v`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/helpers_test.go
git commit -m "test(handler): add helpers.go tests"
```

---

## Task 9: admin/static.go 测试

**Files:**
- Create: `internal/api/admin/static_test.go`

- [ ] **Step 1: 查看 embedded dist 目录**

确认 `internal/api/admin/dist/` 目录下有 `index.html` 文件（用于 SPA 路由测试）。

- [ ] **Step 2: 编写 StaticHandler 测试**

创建 `internal/api/admin/static_test.go`：

```go
package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStaticHandler_ServeHTTP_Index(t *testing.T) {
	handler := NewStaticHandler()

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
}

func TestStaticHandler_ServeHTTP_Root(t *testing.T) {
	handler := NewStaticHandler()

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestStaticHandler_ServeHTTP_SPARoute(t *testing.T) {
	handler := NewStaticHandler()

	// 请求一个不存在的路径，应该返回 index.html（SPA 路由）
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/settings", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
}
```

- [ ] **Step 3: 运行测试确认通过**

Run: `go test ./internal/api/admin/ -run TestStaticHandler -v`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/api/admin/static_test.go
git commit -m "test(admin): add StaticHandler tests"
```

---

## Task 10: 覆盖率验证和最终检查

**Files:**
- 无新文件

- [ ] **Step 1: 运行全部测试**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 2: 生成覆盖率报告**

Run: `go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -1`
Expected: 整体覆盖率提升

- [ ] **Step 3: 检查关键模块覆盖率**

Run: `go test ./internal/storage/... -coverprofile=storage.out && go tool cover -func=storage.out`
Run: `go test ./internal/api/handler/... -coverprofile=handler.out && go tool cover -func=handler.out`
Run: `go test ./internal/api/admin/... -coverprofile=admin.out && go tool cover -func=admin.out`

- [ ] **Step 4: 运行 lint 检查**

Run: `make lint`
Expected: PASS

- [ ] **Step 5: 最终提交**

```bash
git add -A
git commit -m "chore: comprehensive fix - ListRatedAfter, handler tests, coverage improvement"
```

---

## 验证清单

- [ ] ListRatedAfter 接口已添加到 RatingStore
- [ ] MemoryRatingStore 实现 ListRatedAfter
- [ ] BadgerRatingStore 实现 ListRatedAfter
- [ ] 同步逻辑使用 ListRatedAfter
- [ ] CategoryHandler 测试覆盖
- [ ] UserHandler 测试覆盖
- [ ] NodeHandler 测试覆盖
- [ ] helpers.go 测试覆盖
- [ ] admin/static.go 测试覆盖
- [ ] 所有测试通过
- [ ] 覆盖率提升
- [ ] lint 检查通过
