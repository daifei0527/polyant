# Phase 7a: 批量操作 API 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 AgentWiki 添加批量操作 API，支持一次请求创建、更新、删除最多 100 个知识条目

**Architecture:** 新增 BatchHandler 处理批量请求，实现预验证模式（先验证全部条目，通过后批量执行）

**Tech Stack:** Go 1.22, net/http

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/api/handler/types.go` | 修改 | 添加批量请求/响应类型 |
| `internal/api/handler/batch_handler.go` | 创建 | 批量操作 HTTP 处理器 |
| `internal/api/router/router.go` | 修改 | 注册批量操作路由 |
| `internal/api/handler/batch_handler_test.go` | 创建 | 批量操作测试 |

---

## Task 1: 添加批量操作数据模型

**Files:**
- Modify: `internal/api/handler/types.go`

- [ ] **Step 1: 添加批量操作请求和响应类型**

在 `internal/api/handler/types.go` 文件末尾添加:

```go
// ==================== 批量操作类型 ====================

const MaxBatchSize = 100

// BatchCreateRequest 批量创建请求
type BatchCreateRequest struct {
	Entries []BatchEntry `json:"entries"`
	Options BatchOptions `json:"options,omitempty"`
}

// BatchUpdateRequest 批量更新请求
type BatchUpdateRequest struct {
	Entries []BatchUpdateEntry `json:"entries"`
}

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []string `json:"ids"`
}

// BatchEntry 批量创建条目
type BatchEntry struct {
	Title     string                   `json:"title"`
	Content   string                   `json:"content"`
	JsonData  []map[string]interface{} `json:"json_data,omitempty"`
	Category  string                   `json:"category"`
	Tags      []string                 `json:"tags,omitempty"`
	License   string                   `json:"license,omitempty"`
	SourceRef string                   `json:"source_ref,omitempty"`
}

// BatchUpdateEntry 批量更新条目
type BatchUpdateEntry struct {
	ID       string                   `json:"id"`
	Title    *string                  `json:"title,omitempty"`
	Content  *string                  `json:"content,omitempty"`
	JsonData []map[string]interface{} `json:"json_data,omitempty"`
	Category *string                  `json:"category,omitempty"`
	Tags     *[]string                `json:"tags,omitempty"`
}

// BatchOptions 批量操作选项
type BatchOptions struct {
	SkipDuplicates bool `json:"skip_duplicates"`
	UpdateExisting bool `json:"update_existing"`
}

// BatchResponse 批量操作响应
type BatchResponse struct {
	Success bool          `json:"success"`
	Summary BatchSummary  `json:"summary"`
	Results []BatchResult `json:"results"`
	Errors  []BatchError  `json:"errors,omitempty"`
}

// BatchSummary 批量操作汇总
type BatchSummary struct {
	Total    int `json:"total"`
	Created  int `json:"created,omitempty"`
	Updated  int `json:"updated,omitempty"`
	Deleted  int `json:"deleted,omitempty"`
	Skipped  int `json:"skipped,omitempty"`
	Failed   int `json:"failed,omitempty"`
	NotFound int `json:"not_found,omitempty"`
}

// BatchResult 单个条目操作结果
type BatchResult struct {
	Index   int    `json:"index"`
	ID      string `json:"id"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Version int64  `json:"version,omitempty"`
}

// BatchError 批量操作错误
type BatchError struct {
	Index   int    `json:"index"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/api/handler/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/types.go
git commit -m "feat(api): 添加批量操作数据模型

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: 实现批量操作 Handler

**Files:**
- Create: `internal/api/handler/batch_handler.go`

- [ ] **Step 1: 创建批量操作 Handler 文件**

创建 `internal/api/handler/batch_handler.go`:

```go
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	awerrors "github.com/daifei0527/agentwiki/internal/api/handler"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/index"
	"github.com/daifei0527/agentwiki/internal/storage/linkparser"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// BatchHandler 批量操作处理器
type BatchHandler struct {
	entryStore   storage.EntryStore
	searchEngine index.SearchEngine
	backlink     storage.BacklinkIndex
	userStore    storage.UserStore
}

// NewBatchHandler 创建批量操作处理器
func NewBatchHandler(entryStore storage.EntryStore, searchEngine index.SearchEngine, backlink storage.BacklinkIndex, userStore storage.UserStore) *BatchHandler {
	return &BatchHandler{
		entryStore:   entryStore,
		searchEngine: searchEngine,
		backlink:     backlink,
		userStore:    userStore,
	}
}

// BatchCreateHandler 批量创建条目
// POST /api/v1/entries/batch
func (h *BatchHandler) BatchCreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 解析请求
	var req BatchCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 验证数量限制
	if len(req.Entries) == 0 {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}
	if len(req.Entries) > MaxBatchSize {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    10100,
			Message: "条目数量超过限制，最多 100 条",
		})
		return
	}

	// 获取用户信息
	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	// 检查权限
	if user.UserLevel < model.UserLevelLv1 {
		writeError(w, awerrors.ErrBasicUserDenied)
		return
	}

	// 预验证所有条目
	errors := h.validateCreateEntries(req.Entries)
	if len(errors) > 0 {
		writeJSON(w, http.StatusBadRequest, &BatchResponse{
			Success: false,
			Summary: BatchSummary{Total: len(req.Entries), Failed: len(errors)},
			Errors:  errors,
		})
		return
	}

	// 执行批量创建
	response := h.executeBatchCreate(r.Context(), req.Entries, user, req.Options)

	writeJSON(w, http.StatusOK, response)
}

// BatchUpdateHandler 批量更新条目
// PUT /api/v1/entries/batch
func (h *BatchHandler) BatchUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	var req BatchUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	if len(req.Entries) == 0 || len(req.Entries) > MaxBatchSize {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	if user.UserLevel < model.UserLevelLv1 {
		writeError(w, awerrors.ErrBasicUserDenied)
		return
	}

	// 预验证
	errors := h.validateUpdateEntries(r.Context(), req.Entries, user)
	if len(errors) > 0 {
		writeJSON(w, http.StatusBadRequest, &BatchResponse{
			Success: false,
			Summary: BatchSummary{Total: len(req.Entries), Failed: len(errors)},
			Errors:  errors,
		})
		return
	}

	// 执行批量更新
	response := h.executeBatchUpdate(r.Context(), req.Entries, user)

	writeJSON(w, http.StatusOK, response)
}

// BatchDeleteHandler 批量删除条目
// DELETE /api/v1/entries/batch
func (h *BatchHandler) BatchDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	var req BatchDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	if len(req.IDs) == 0 || len(req.IDs) > MaxBatchSize {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	if user.UserLevel < model.UserLevelLv1 {
		writeError(w, awerrors.ErrBasicUserDenied)
		return
	}

	// 预验证
	errors := h.validateDeleteEntries(r.Context(), req.IDs, user)
	if len(errors) > 0 {
		writeJSON(w, http.StatusBadRequest, &BatchResponse{
			Success: false,
			Summary: BatchSummary{Total: len(req.IDs), Failed: len(errors)},
			Errors:  errors,
		})
		return
	}

	// 执行批量删除
	response := h.executeBatchDelete(r.Context(), req.IDs)

	writeJSON(w, http.StatusOK, response)
}

// validateCreateEntries 验证创建条目
func (h *BatchHandler) validateCreateEntries(entries []BatchEntry) []BatchError {
	var errors []BatchError
	for i, entry := range entries {
		if entry.Title == "" {
			errors = append(errors, BatchError{Index: i, Field: "title", Message: "标题不能为空"})
		}
		if entry.Content == "" {
			errors = append(errors, BatchError{Index: i, Field: "content", Message: "内容不能为空"})
		}
		if entry.Category == "" {
			errors = append(errors, BatchError{Index: i, Field: "category", Message: "分类不能为空"})
		}
	}
	return errors
}

// validateUpdateEntries 验证更新条目
func (h *BatchHandler) validateUpdateEntries(ctx interface{}, entries []BatchUpdateEntry, user *model.User) []BatchError {
	var errors []BatchError
	for i, entry := range entries {
		if entry.ID == "" {
			errors = append(errors, BatchError{Index: i, Field: "id", Message: "条目ID不能为空"})
			continue
		}
		// 检查条目是否存在和权限
		existing, err := h.entryStore.Get(nil, entry.ID)
		if err != nil {
			errors = append(errors, BatchError{Index: i, Field: "id", Message: "条目不存在"})
			continue
		}
		if existing.CreatedBy != user.PublicKey && user.UserLevel < model.UserLevelLv3 {
			errors = append(errors, BatchError{Index: i, Field: "id", Message: "无权限更新此条目"})
		}
	}
	return errors
}

// validateDeleteEntries 验证删除条目
func (h *BatchHandler) validateDeleteEntries(ctx interface{}, ids []string, user *model.User) []BatchError {
	var errors []BatchError
	for i, id := range ids {
		if id == "" {
			errors = append(errors, BatchError{Index: i, Field: "id", Message: "条目ID不能为空"})
			continue
		}
		existing, err := h.entryStore.Get(nil, id)
		if err != nil {
			errors = append(errors, BatchError{Index: i, Field: "id", Message: "条目不存在"})
			continue
		}
		if existing.CreatedBy != user.PublicKey && user.UserLevel < model.UserLevelLv4 {
			errors = append(errors, BatchError{Index: i, Field: "id", Message: "无权限删除此条目"})
		}
	}
	return errors
}

// executeBatchCreate 执行批量创建
func (h *BatchHandler) executeBatchCreate(ctx interface{}, entries []BatchEntry, user *model.User, opts BatchOptions) *BatchResponse {
	response := &BatchResponse{
		Success: true,
		Summary: BatchSummary{Total: len(entries)},
		Results: make([]BatchResult, len(entries)),
	}

	for i, entry := range entries {
		contentHash := computeBatchContentHash(entry.Title, entry.Content, entry.Category)
		entryID := generateUUID()
		now := model.NowMillis()

		newEntry := &model.KnowledgeEntry{
			ID:          entryID,
			Title:       entry.Title,
			Content:     entry.Content,
			JSONData:    entry.JsonData,
			Category:    entry.Category,
			Tags:        entry.Tags,
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			CreatedBy:   user.PublicKey,
			Score:       0,
			ScoreCount:  0,
			ContentHash: contentHash,
			Status:      model.EntryStatusPublished,
			License:     entry.License,
			SourceRef:   entry.SourceRef,
		}
		if newEntry.License == "" {
			newEntry.License = "CC-BY-SA-4.0"
		}

		created, err := h.entryStore.Create(nil, newEntry)
		if err != nil {
			response.Results[i] = BatchResult{Index: i, Status: "failed", Reason: err.Error()}
			response.Summary.Failed++
			continue
		}

		// 建立索引
		if h.searchEngine != nil {
			_ = h.searchEngine.IndexEntry(created)
		}
		if h.backlink != nil {
			linkedEntryIDs := linkparser.ParseLinks(created.Content)
			_ = h.backlink.UpdateIndex(created.ID, linkedEntryIDs)
		}

		response.Results[i] = BatchResult{Index: i, ID: created.ID, Status: "created", Version: 1}
		response.Summary.Created++
	}

	return response
}

// executeBatchUpdate 执行批量更新
func (h *BatchHandler) executeBatchUpdate(ctx interface{}, entries []BatchUpdateEntry, user *model.User) *BatchResponse {
	response := &BatchResponse{
		Success: true,
		Summary: BatchSummary{Total: len(entries)},
		Results: make([]BatchResult, len(entries)),
	}

	for i, req := range entries {
		existing, _ := h.entryStore.Get(nil, req.ID)

		if req.Title != nil {
			existing.Title = *req.Title
		}
		if req.Content != nil {
			existing.Content = *req.Content
		}
		if req.JsonData != nil {
			existing.JSONData = req.JsonData
		}
		if req.Category != nil {
			existing.Category = *req.Category
		}
		if req.Tags != nil {
			existing.Tags = *req.Tags
		}

		existing.Version++
		existing.UpdatedAt = model.NowMillis()
		existing.ContentHash = computeBatchContentHash(existing.Title, existing.Content, existing.Category)

		updated, err := h.entryStore.Update(nil, existing)
		if err != nil {
			response.Results[i] = BatchResult{Index: i, ID: req.ID, Status: "failed", Reason: err.Error()}
			response.Summary.Failed++
			continue
		}

		if h.searchEngine != nil {
			_ = h.searchEngine.UpdateIndex(updated)
		}
		if h.backlink != nil {
			linkedEntryIDs := linkparser.ParseLinks(updated.Content)
			_ = h.backlink.UpdateIndex(updated.ID, linkedEntryIDs)
		}

		response.Results[i] = BatchResult{Index: i, ID: updated.ID, Status: "updated", Version: updated.Version}
		response.Summary.Updated++
	}

	return response
}

// executeBatchDelete 执行批量删除
func (h *BatchHandler) executeBatchDelete(ctx interface{}, ids []string) *BatchResponse {
	response := &BatchResponse{
		Success: true,
		Summary: BatchSummary{Total: len(ids)},
		Results: make([]BatchResult, len(ids)),
	}

	for i, id := range ids {
		if err := h.entryStore.Delete(nil, id); err != nil {
			response.Results[i] = BatchResult{Index: i, ID: id, Status: "failed", Reason: err.Error()}
			response.Summary.Failed++
			continue
		}

		if h.searchEngine != nil {
			_ = h.searchEngine.DeleteIndex(id)
		}
		if h.backlink != nil {
			_ = h.backlink.DeleteIndex(id)
		}

		response.Results[i] = BatchResult{Index: i, ID: id, Status: "deleted"}
		response.Summary.Deleted++
	}

	return response
}

// computeBatchContentHash 计算内容哈希
func computeBatchContentHash(title, content, category string) string {
	h := sha256.New()
	h.Write([]byte(title))
	h.Write([]byte(content))
	h.Write([]byte(category))
	return hex.EncodeToString(h.Sum(nil))
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/api/handler/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/batch_handler.go
git commit -m "feat(api): 实现批量操作 Handler

- 批量创建、更新、删除条目
- 预验证模式，最多 100 条/批次

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: 注册批量操作路由

**Files:**
- Modify: `internal/api/router/router.go`

- [ ] **Step 1: 在 NewRouterWithDeps 函数中创建 BatchHandler**

在 `internal/api/router/router.go` 的 `NewRouterWithDeps` 函数中，在创建其他 handler 后添加:

```go
	// 创建批量操作 handler
	batchHandler := handler.NewBatchHandler(deps.EntryStore, deps.SearchEngine, deps.Backlink, deps.UserStore)
```

- [ ] **Step 2: 在 registerAuthRoutes 函数签名中添加 batchHandler 参数**

修改 `registerAuthRoutes` 函数签名:

```go
func registerAuthRoutes(mux *http.ServeMux, authMW *middleware.AuthMiddleware, eh *handler.EntryHandler, uh *handler.UserHandler, ch *handler.CategoryHandler, nh *handler.NodeHandler, ah *handler.AdminHandler, elh *handler.ElectionHandler, bh *handler.BatchHandler) {
```

- [ ] **Step 3: 在 registerAuthRoutes 函数末尾添加批量操作路由**

在 `registerAuthRoutes` 函数末尾添加:

```go
	// ==================== 批量操作路由 ====================
	if bh != nil {
		// 批量创建条目 POST /api/v1/entries/batch
		mux.Handle("/api/v1/entries/batch", authMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				bh.BatchCreateHandler(w, r)
			case http.MethodPut:
				bh.BatchUpdateHandler(w, r)
			case http.MethodDelete:
				bh.BatchDeleteHandler(w, r)
			default:
				http.NotFound(w, r)
			}
		})))
	}
```

- [ ] **Step 4: 更新 registerAuthRoutes 调用**

修改调用 `registerAuthRoutes` 的地方，添加 batchHandler 参数:

```go
	registerAuthRoutes(mux, authMW, entryHandler, userHandler, categoryHandler, nodeHandler, adminHandler, electionHandler, batchHandler)
```

- [ ] **Step 5: 验证编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 6: 提交**

```bash
git add internal/api/router/router.go
git commit -m "feat(router): 添加批量操作路由

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: 编写批量操作测试

**Files:**
- Create: `internal/api/handler/batch_handler_test.go`

- [ ] **Step 1: 创建测试文件**

创建 `internal/api/handler/batch_handler_test.go`:

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchCreateHandler(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewBatchHandler(store.Entry, store.Search, store.Backlink, store.User)

	// 创建测试用户
	user := &model.User{
		PublicKey: "test-pk",
		UserLevel: model.UserLevelLv1,
	}
	store.User.Create(nil, user)

	// 创建请求
	entries := []BatchEntry{
		{Title: "条目1", Content: "内容1", Category: "test"},
		{Title: "条目2", Content: "内容2", Category: "test"},
	}
	reqBody := BatchCreateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchCreateHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp BatchResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.True(t, resp.Success)
	assert.Equal(t, 2, resp.Summary.Created)
}

func TestBatchCreateHandler_TooManyEntries(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewBatchHandler(store.Entry, store.Search, store.Backlink, store.User)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1}
	store.User.Create(nil, user)

	// 创建超过限制的条目
	entries := make([]BatchEntry, 101)
	for i := range entries {
		entries[i] = BatchEntry{Title: "条目", Content: "内容", Category: "test"}
	}
	reqBody := BatchCreateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchCreateHandler(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestBatchCreateHandler_ValidationFailure(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewBatchHandler(store.Entry, store.Search, store.Backlink, store.User)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1}
	store.User.Create(nil, user)

	// 创建无效条目（缺少必填字段）
	entries := []BatchEntry{
		{Title: "条目1", Content: "内容1", Category: "test"}, // 有效
		{Title: "", Content: "内容2", Category: "test"},      // 无效：标题为空
		{Title: "条目3", Content: "", Category: "test"},      // 无效：内容为空
	}
	reqBody := BatchCreateRequest{Entries: entries}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchCreateHandler(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp BatchResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.False(t, resp.Success)
	assert.Equal(t, 2, len(resp.Errors)) // 应该有2个错误
}

func TestBatchDeleteHandler(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewBatchHandler(store.Entry, store.Search, store.Backlink, store.User)

	user := &model.User{PublicKey: "test-pk", UserLevel: model.UserLevelLv1}
	store.User.Create(nil, user)

	// 先创建一些条目
	entry1 := &model.KnowledgeEntry{ID: "id1", Title: "条目1", Content: "内容", Category: "test", CreatedBy: "test-pk"}
	entry2 := &model.KnowledgeEntry{ID: "id2", Title: "条目2", Content: "内容", Category: "test", CreatedBy: "test-pk"}
	store.Entry.Create(nil, entry1)
	store.Entry.Create(nil, entry2)

	// 批量删除
	reqBody := BatchDeleteRequest{IDs: []string{"id1", "id2"}}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/entries/batch", bytes.NewReader(body))
	req = req.WithContext(setUserInContext(req.Context(), user))

	rr := httptest.NewRecorder()
	handler.BatchDeleteHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp BatchResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.True(t, resp.Success)
	assert.Equal(t, 2, resp.Summary.Deleted)
}
```

- [ ] **Step 2: 运行测试**

Run: `go test ./internal/api/handler/... -v -run Batch`
Expected: 所有测试通过

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/batch_handler_test.go
git commit -m "test(api): 添加批量操作测试

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: 运行完整测试套件

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
git commit -m "feat: Phase 7a 批量操作 API 完成

功能:
- POST /api/v1/entries/batch - 批量创建条目
- PUT /api/v1/entries/batch - 批量更新条目
- DELETE /api/v1/entries/batch - 批量删除条目
- 预验证模式，最多 100 条/批次

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## 验收清单

- [ ] 批量创建 API 可用
- [ ] 批量更新 API 可用
- [ ] 批量删除 API 可用
- [ ] 数量限制 (100条) 生效
- [ ] 预验证模式正常工作
- [ ] 所有测试通过
- [ ] 测试覆盖率 > 55%
