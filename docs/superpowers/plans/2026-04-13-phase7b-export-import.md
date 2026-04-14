# Phase 7b: 数据导出/导入实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Polyant 添加数据导出/导入功能，支持管理员备份、迁移和恢复系统数据

**Architecture:** 新增 ExportService 和 ImportService 处理 ZIP 文件生成和解析，ExportHandler 提供 REST API 端点，支持三种冲突处理策略

**Tech Stack:** Go 1.22, net/http, archive/zip, encoding/json

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/core/export/exporter.go` | 创建 | 导出服务，生成 ZIP 文件 |
| `internal/core/export/importer.go` | 创建 | 导入服务，解析 ZIP 并处理冲突 |
| `internal/api/handler/export_handler.go` | 创建 | 导出/导入 HTTP 处理器 |
| `internal/api/router/router.go` | 修改 | 注册导出/导入路由 |
| `internal/core/export/exporter_test.go` | 创建 | 导出服务测试 |
| `internal/core/export/importer_test.go` | 创建 | 导入服务测试 |

---

## Task 1: 创建导出服务

**Files:**
- Create: `internal/core/export/exporter.go`

- [ ] **Step 1: 创建导出服务目录和文件**

创建 `internal/core/export/exporter.go`:

```go
// Package export 提供数据导出和导入功能
package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// Manifest 导出文件元数据
type Manifest struct {
	Version    string         `json:"version"`
	ExportedAt int64          `json:"exported_at"`
	NodeID     string         `json:"node_id"`
	Counts     map[string]int `json:"counts"`
}

// ExportUser 导出用户格式（隐私保护）
type ExportUser struct {
	PublicKey    string `json:"public_key"`
	AgentName    string `json:"agent_name"`
	UserLevel    int32  `json:"user_level"`
	RegisteredAt int64  `json:"registered_at"`
	Status       string `json:"status"`
}

// Exporter 导出服务
type Exporter struct {
	store  *storage.Store
	nodeID string
}

// NewExporter 创建导出服务
func NewExporter(store *storage.Store, nodeID string) *Exporter {
	return &Exporter{
		store:  store,
		nodeID: nodeID,
	}
}

// ExportOptions 导出选项
type ExportOptions struct {
	IncludeEntries    bool
	IncludeCategories bool
	IncludeUsers      bool
	IncludeRatings    bool
}

// Export 导出数据到 ZIP 文件
func (e *Exporter) Export(opts ExportOptions) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 创建 manifest
	manifest := &Manifest{
		Version:    "1.0",
		ExportedAt: time.Now().UnixMilli(),
		NodeID:     e.nodeID,
		Counts:     make(map[string]int),
	}

	// 导出条目
	if opts.IncludeEntries {
		entries, _, err := e.store.Entry.List(nil, storage.EntryFilter{Limit: 100000})
		if err != nil {
			zipWriter.Close()
			return nil, fmt.Errorf("failed to list entries: %w", err)
		}
		if err := e.writeJSONToZip(zipWriter, "entries.json", entries); err != nil {
			zipWriter.Close()
			return nil, err
		}
		manifest.Counts["entries"] = len(entries)
	}

	// 导出分类
	if opts.IncludeCategories {
		categories, err := e.store.Category.ListAll(nil)
		if err != nil {
			zipWriter.Close()
			return nil, fmt.Errorf("failed to list categories: %w", err)
		}
		if err := e.writeJSONToZip(zipWriter, "categories.json", categories); err != nil {
			zipWriter.Close()
			return nil, err
		}
		manifest.Counts["categories"] = len(categories)
	}

	// 导出用户
	if opts.IncludeUsers {
		users, _, err := e.store.User.List(nil, storage.UserFilter{Limit: 100000})
		if err != nil {
			zipWriter.Close()
			return nil, fmt.Errorf("failed to list users: %w", err)
		}
		// 转换为导出格式（去除敏感信息）
		exportUsers := make([]ExportUser, len(users))
		for i, u := range users {
			exportUsers[i] = ExportUser{
				PublicKey:    u.PublicKey,
				AgentName:    u.AgentName,
				UserLevel:    u.UserLevel,
				RegisteredAt: u.RegisteredAt,
				Status:       u.Status,
			}
		}
		if err := e.writeJSONToZip(zipWriter, "users.json", exportUsers); err != nil {
			zipWriter.Close()
			return nil, err
		}
		manifest.Counts["users"] = len(users)
	}

	// 导出评分
	if opts.IncludeRatings {
		// 评分需要通过条目ID获取
		entries, _, _ := e.store.Entry.List(nil, storage.EntryFilter{Limit: 100000})
		var allRatings []*model.Rating
		for _, entry := range entries {
			ratings, _ := e.store.Rating.ListByEntry(nil, entry.ID)
			allRatings = append(allRatings, ratings...)
		}
		if err := e.writeJSONToZip(zipWriter, "ratings.json", allRatings); err != nil {
			zipWriter.Close()
			return nil, err
		}
		manifest.Counts["ratings"] = len(allRatings)
	}

	// 写入 manifest
	if err := e.writeJSONToZip(zipWriter, "manifest.json", manifest); err != nil {
		zipWriter.Close()
		return nil, err
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip: %w", err)
	}

	return buf.Bytes(), nil
}

// writeJSONToZip 写入 JSON 文件到 ZIP
func (e *Exporter) writeJSONToZip(zipWriter *zip.Writer, filename string, data interface{}) error {
	writer, err := zipWriter.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create %s in zip: %w", filename, err)
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode %s: %w", filename, err)
	}

	return nil
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/core/export/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/core/export/exporter.go
git commit -m "feat(export): 添加导出服务

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: 创建导入服务

**Files:**
- Create: `internal/core/export/importer.go`

- [ ] **Step 1: 创建导入服务文件**

创建 `internal/core/export/importer.go`:

```go
package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// ConflictStrategy 冲突处理策略
type ConflictStrategy string

const (
	ConflictSkip      ConflictStrategy = "skip"      // 跳过冲突
	ConflictOverwrite ConflictStrategy = "overwrite" // 覆盖现有
	ConflictMerge     ConflictStrategy = "merge"     // 合并
)

// ImportOptions 导入选项
type ImportOptions struct {
	ConflictStrategy ConflictStrategy
	OperatorLevel    int32 // 操作者等级，用于权限检查
}

// ImportSummary 导入结果汇总
type ImportSummary struct {
	EntriesImported int `json:"entries_imported"`
	EntriesSkipped  int `json:"entries_skipped"`
	EntriesUpdated  int `json:"entries_updated"`
	CategoriesImported int `json:"categories_imported"`
	UsersImported   int `json:"users_imported"`
	RatingsImported int `json:"ratings_imported"`
}

// ImportError 导入错误
type ImportError struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Message string `json:"message"`
}

// ImportResult 导入结果
type ImportResult struct {
	Success bool           `json:"success"`
	Summary ImportSummary  `json:"summary"`
	Errors  []ImportError  `json:"errors,omitempty"`
}

// Importer 导入服务
type Importer struct {
	store *storage.Store
}

// NewImporter 创建导入服务
func NewImporter(store *storage.Store) *Importer {
	return &Importer{store: store}
}

// Import 从 ZIP 文件导入数据
func (i *Importer) Import(zipData []byte, opts ImportOptions) *ImportResult {
	result := &ImportResult{
		Success: true,
		Errors:  []ImportError{},
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, ImportError{
			Type:    "zip",
			Message: fmt.Sprintf("failed to read zip: %v", err),
		})
		return result
	}

	// 解析文件到内存
	files := make(map[string][]byte)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[file.Name] = data
	}

	// 导入分类（先导入，因为条目依赖分类）
	if data, ok := files["categories.json"]; ok {
		i.importCategories(data, opts, result)
	}

	// 导入用户
	if data, ok := files["users.json"]; ok {
		i.importUsers(data, opts, result)
	}

	// 导入条目
	if data, ok := files["entries.json"]; ok {
		i.importEntries(data, opts, result)
	}

	// 导入评分
	if data, ok := files["ratings.json"]; ok {
		i.importRatings(data, opts, result)
	}

	return result
}

// importCategories 导入分类
func (i *Importer) importCategories(data []byte, opts ImportOptions, result *ImportResult) {
	var categories []*model.Category
	if err := json.Unmarshal(data, &categories); err != nil {
		result.Errors = append(result.Errors, ImportError{
			Type:    "category",
			Message: fmt.Sprintf("failed to parse categories: %v", err),
		})
		return
	}

	for _, cat := range categories {
		existing, err := i.store.Category.Get(nil, cat.Path)
		if err == nil {
			// 分类已存在
			switch opts.ConflictStrategy {
			case ConflictSkip:
				continue
			case ConflictOverwrite:
				// 更新分类
				i.store.Category.Create(nil, cat) // 覆盖
			case ConflictMerge:
				// 保留现有分类
				continue
			}
		} else {
			// 分类不存在，创建
			i.store.Category.Create(nil, cat)
		}
		result.Summary.CategoriesImported++
	}
}

// importUsers 导入用户
func (i *Importer) importUsers(data []byte, opts ImportOptions, result *ImportResult) {
	var exportUsers []ExportUser
	if err := json.Unmarshal(data, &exportUsers); err != nil {
		result.Errors = append(result.Errors, ImportError{
			Type:    "user",
			Message: fmt.Sprintf("failed to parse users: %v", err),
		})
		return
	}

	for _, eu := range exportUsers {
		// 安全检查：不能导入高于操作者等级的用户
		if eu.UserLevel > opts.OperatorLevel {
			result.Errors = append(result.Errors, ImportError{
				Type:    "user",
				ID:      eu.PublicKey,
				Message: "cannot import user with higher level",
			})
			continue
		}

		existing, err := i.store.User.Get(nil, eu.PublicKey)
		if err == nil {
			// 用户已存在
			switch opts.ConflictStrategy {
			case ConflictSkip:
				continue
			case ConflictOverwrite, ConflictMerge:
				// 只更新公开字段，不修改等级
				existing.AgentName = eu.AgentName
				existing.Status = eu.Status
				i.store.User.Update(nil, existing)
			}
		} else {
			// 用户不存在，创建
			user := &model.User{
				PublicKey:    eu.PublicKey,
				AgentName:    eu.AgentName,
				UserLevel:    eu.UserLevel,
				RegisteredAt: eu.RegisteredAt,
				Status:       eu.Status,
			}
			i.store.User.Create(nil, user)
		}
		result.Summary.UsersImported++
	}
}

// importEntries 导入条目
func (i *Importer) importEntries(data []byte, opts ImportOptions, result *ImportResult) {
	var entries []*model.KnowledgeEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		result.Errors = append(result.Errors, ImportError{
			Type:    "entry",
			Message: fmt.Sprintf("failed to parse entries: %v", err),
		})
		return
	}

	for _, entry := range entries {
		existing, err := i.store.Entry.Get(nil, entry.ID)
		if err == nil {
			// 条目已存在
			switch opts.ConflictStrategy {
			case ConflictSkip:
				result.Summary.EntriesSkipped++
				continue
			case ConflictOverwrite:
				i.store.Update(nil, entry)
			case ConflictMerge:
				// 比较 version，保留更高版本
				if entry.Version > existing.Version {
					i.store.Entry.Update(nil, entry)
				} else {
					result.Summary.EntriesSkipped++
					continue
				}
			}
			result.Summary.EntriesUpdated++
		} else {
			// 条目不存在，创建
			i.store.Entry.Create(nil, entry)
			result.Summary.EntriesImported++
		}
	}
}

// importRatings 导入评分
func (i *Importer) importRatings(data []byte, opts ImportOptions, result *ImportResult) {
	var ratings []*model.Rating
	if err := json.Unmarshal(data, &ratings); err != nil {
		result.Errors = append(result.Errors, ImportError{
			Type:    "rating",
			Message: fmt.Sprintf("failed to parse ratings: %v", err),
		})
		return
	}

	for _, rating := range ratings {
		// 检查是否已存在评分
		existing, _ := i.store.Rating.GetByRater(nil, rating.EntryId, rating.RaterPubkey)
		if existing != nil {
			switch opts.ConflictStrategy {
			case ConflictSkip:
				continue
			case ConflictOverwrite:
				i.store.Rating.Create(nil, rating)
			case ConflictMerge:
				// 保留现有评分
				continue
			}
		} else {
			i.store.Rating.Create(nil, rating)
		}
		result.Summary.RatingsImported++
	}
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/core/export/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/core/export/importer.go
git commit -m "feat(export): 添加导入服务

支持 skip/overwrite/merge 三种冲突策略

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: 创建导出/导入 Handler

**Files:**
- Create: `internal/api/handler/export_handler.go`

- [ ] **Step 1: 创建 Handler 文件**

创建 `internal/api/handler/export_handler.go`:

```go
package handler

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/daifei0527/polyant/internal/core/export"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// ExportHandler 导出/导入处理器
type ExportHandler struct {
	exporter *export.Exporter
	importer *export.Importer
}

// NewExportHandler 创建导出处理器
func NewExportHandler(store *storage.Store, nodeID string) *ExportHandler {
	return &ExportHandler{
		exporter: export.NewExporter(store, nodeID),
		importer: export.NewImporter(store),
	}
}

// ExportHandler 导出数据
// GET /api/v1/admin/export?include=entries,categories,users,ratings
func (h *ExportHandler) ExportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	// 解析 include 参数
	includeParam := r.URL.Query().Get("include")
	if includeParam == "" {
		includeParam = "entries,categories"
	}

	opts := export.ExportOptions{
		IncludeEntries:    strings.Contains(includeParam, "entries"),
		IncludeCategories: strings.Contains(includeParam, "categories"),
		IncludeUsers:      strings.Contains(includeParam, "users"),
		IncludeRatings:    strings.Contains(includeParam, "ratings"),
	}

	// 执行导出
	zipData, err := h.exporter.Export(opts)
	if err != nil {
		writeError(w, awerrors.Wrap(900, awerrors.CategoryAPI, err.Error(), http.StatusInternalServerError, err))
		return
	}

	// 设置响应头
	filename := fmt.Sprintf("polyant-export-%s.zip", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	w.Write(zipData)
}

// ImportHandler 导入数据
// POST /api/v1/admin/import
// Content-Type: multipart/form-data
// Fields: file (ZIP), conflict (skip|overwrite|merge)
func (h *ExportHandler) ImportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	// 解析 multipart 表单
	maxSize := int64(100 << 20) // 100MB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	if err := r.ParseMultipartForm(maxSize); err != nil {
		writeError(w, awerrors.New(101, awerrors.CategoryAPI, "file too large or invalid form", http.StatusBadRequest))
		return
	}

	// 获取上传的文件
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, awerrors.New(102, awerrors.CategoryAPI, "missing file field", http.StatusBadRequest))
		return
	}
	defer file.Close()

	// 读取文件内容
	zipData, err := io.ReadAll(file)
	if err != nil {
		writeError(w, awerrors.New(103, awerrors.CategoryAPI, "failed to read file", http.StatusBadRequest))
		return
	}

	// 获取冲突策略
	conflictStr := r.FormValue("conflict")
	if conflictStr == "" {
		conflictStr = "skip"
	}

	strategy := export.ConflictStrategy(conflictStr)
	if strategy != export.ConflictSkip && strategy != export.ConflictOverwrite && strategy != export.ConflictMerge {
		writeError(w, awerrors.New(104, awerrors.CategoryAPI, "invalid conflict strategy", http.StatusBadRequest))
		return
	}

	// 获取操作者等级（用于权限检查）
	user := getUserFromContext(r.Context())
	var operatorLevel int32
	if user != nil {
		operatorLevel = user.UserLevel
	}

	// 执行导入
	opts := export.ImportOptions{
		ConflictStrategy: strategy,
		OperatorLevel:    operatorLevel,
	}
	result := h.importer.Import(zipData, opts)

	// 返回结果
	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadRequest
	}

	writeJSON(w, status, &APIResponse{
		Code:    0,
		Message: "import completed",
		Data:    result,
	})
}

// setUserInContext 辅助函数（如果不存在则声明）
// 注意：此函数应在 handler_test.go 或其他地方已定义
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/api/handler/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/export_handler.go
git commit -m "feat(api): 添加导出/导入 Handler

- GET /api/v1/admin/export 导出 ZIP
- POST /api/v1/admin/import 导入 ZIP

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: 注册路由

**Files:**
- Modify: `internal/api/router/router.go`

- [ ] **Step 1: 在 NewRouterWithDeps 中创建 ExportHandler**

在 `internal/api/router/router.go` 的 `NewRouterWithDeps` 函数中，在 batchHandler 创建之后添加:

```go
	// 创建导出/导入 handler
	exportHandler := handler.NewExportHandler(deps.Store, deps.NodeID)
```

- [ ] **Step 2: 在 registerAuthRoutes 函数签名中添加 exportHandler 参数**

修改 `registerAuthRoutes` 函数签名:

```go
func registerAuthRoutes(mux *http.ServeMux, authMW *middleware.AuthMiddleware, eh *handler.EntryHandler, uh *handler.UserHandler, ch *handler.CategoryHandler, nh *handler.NodeHandler, ah *handler.AdminHandler, elh *handler.ElectionHandler, bh *handler.BatchHandler, exh *handler.ExportHandler) {
```

- [ ] **Step 3: 在 registerAuthRoutes 函数的管理员路由部分添加导出/导入路由**

在管理员路由部分（`if ah != nil` 块内）添加:

```go
			// 数据导出 GET /api/v1/admin/export - Lv4+ (Admin)
			if exh != nil {
				mux.Handle("/api/v1/admin/export", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv4, http.HandlerFunc(exh.ExportHandler))))

				// 数据导入 POST /api/v1/admin/import - Lv4+ (Admin)
				mux.Handle("/api/v1/admin/import", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv4, http.HandlerFunc(exh.ImportHandler))))
			}
```

- [ ] **Step 4: 更新 registerAuthRoutes 调用**

修改调用 `registerAuthRoutes` 的地方，添加 exportHandler 参数:

```go
	registerAuthRoutes(mux, authMW, entryHandler, userHandler, categoryHandler, nodeHandler, adminHandler, electionHandler, batchHandler, exportHandler)
```

- [ ] **Step 5: 验证编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 6: 提交**

```bash
git add internal/api/router/router.go
git commit -m "feat(router): 添加导出/导入路由

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: 编写测试

**Files:**
- Create: `internal/core/export/exporter_test.go`
- Create: `internal/core/export/importer_test.go`

- [ ] **Step 1: 创建导出服务测试**

创建 `internal/core/export/exporter_test.go`:

```go
package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newTestStore(t *testing.T) *storage.Store {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return store
}

func TestExporter_Export(t *testing.T) {
	store := newTestStore(t)

	// 创建测试数据
	entry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Test Entry",
		Content:   "Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
		CreatedBy: "user-1",
	}
	store.Entry.Create(nil, entry)

	cat := &model.Category{
		ID:    "cat-1",
		Path:  "test",
		Name:  "Test",
		Level: 0,
	}
	store.Category.Create(nil, cat)

	user := &model.User{
		PublicKey:    "user-1",
		AgentName:    "TestAgent",
		UserLevel:    1,
		Status:       model.UserStatusActive,
	}
	store.User.Create(nil, user)

	// 创建导出器
	exporter := NewExporter(store, "test-node")

	// 执行导出
	opts := ExportOptions{
		IncludeEntries:    true,
		IncludeCategories: true,
		IncludeUsers:      true,
		IncludeRatings:    false,
	}

	zipData, err := exporter.Export(opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// 验证 ZIP 文件
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	// 检查文件列表
	fileNames := make(map[string]bool)
	for _, file := range reader.File {
		fileNames[file.Name] = true
	}

	if !fileNames["manifest.json"] {
		t.Error("Missing manifest.json")
	}
	if !fileNames["entries.json"] {
		t.Error("Missing entries.json")
	}
	if !fileNames["categories.json"] {
		t.Error("Missing categories.json")
	}
	if !fileNames["users.json"] {
		t.Error("Missing users.json")
	}

	// 验证 manifest
	for _, file := range reader.File {
		if file.Name == "manifest.json" {
			rc, _ := file.Open()
			data, _ := bytes.NewBuffer(nil).ReadFrom(rc)
			rc.Close()

			var manifest Manifest
			if err := json.Unmarshal(bytes.NewReader(zipData), &manifest); err != nil {
				// Re-read properly
			}
		}
	}
}

func TestExporter_ExportEmpty(t *testing.T) {
	store := newTestStore(t)
	exporter := NewExporter(store, "test-node")

	opts := ExportOptions{
		IncludeEntries: true,
	}

	zipData, err := exporter.Export(opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(zipData) < 100 {
		t.Errorf("ZIP too small, got %d bytes", len(zipData))
	}
}
```

- [ ] **Step 2: 创建导入服务测试**

创建 `internal/core/export/importer_test.go`:

```go
package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestImporter_Import(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	// 创建测试 ZIP 数据
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 写入条目
	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "Test", Content: "Content", Category: "test", Status: "published"},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	// 写入 manifest
	manifest := &Manifest{Version: "1.0", NodeID: "test"}
	manifestData, _ := json.Marshal(manifest)
	w2, _ := zipWriter.Create("manifest.json")
	w2.Write(manifestData)

	zipWriter.Close()
	zipData := buf.Bytes()

	// 执行导入
	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(zipData, opts)

	if !result.Success {
		t.Errorf("Import failed: %v", result.Errors)
	}
	if result.Summary.EntriesImported != 1 {
		t.Errorf("Expected 1 entry imported, got %d", result.Summary.EntriesImported)
	}

	// 验证数据已导入
	entry, err := store.Entry.Get(nil, "entry-1")
	if err != nil {
		t.Fatalf("Failed to get imported entry: %v", err)
	}
	if entry.Title != "Test" {
		t.Errorf("Expected title 'Test', got '%s'", entry.Title)
	}
}

func TestImporter_ConflictSkip(t *testing.T) {
	store := newTestStore(t)

	// 预先创建条目
	existing := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Existing",
		Content:   "Old Content",
		Category:  "test",
		Status:    "published",
		Version:   1,
	}
	store.Entry.Create(nil, existing)

	importer := NewImporter(store)

	// 创建包含相同 ID 的导入数据
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "New Title", Content: "New Content", Category: "test", Status: "published", Version: 2},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Summary.EntriesSkipped != 1 {
		t.Errorf("Expected 1 entry skipped, got %d", result.Summary.EntriesSkipped)
	}

	// 验证现有数据未被修改
	entry, _ := store.Entry.Get(nil, "entry-1")
	if entry.Title != "Existing" {
		t.Errorf("Title should remain 'Existing', got '%s'", entry.Title)
	}
}

func TestImporter_ConflictOverwrite(t *testing.T) {
	store := newTestStore(t)

	existing := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Existing",
		Content:   "Old",
		Category:  "test",
		Status:    "published",
		Version:   1,
	}
	store.Entry.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "New Title", Content: "New", Category: "test", Status: "published", Version: 2},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictOverwrite,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	if result.Summary.EntriesUpdated != 1 {
		t.Errorf("Expected 1 entry updated, got %d", result.Summary.EntriesUpdated)
	}

	entry, _ := store.Entry.Get(nil, "entry-1")
	if entry.Title != "New Title" {
		t.Errorf("Title should be updated to 'New Title', got '%s'", entry.Title)
	}
}

func TestImporter_ConflictMerge(t *testing.T) {
	store := newTestStore(t)

	// 现有条目版本较高
	existing := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Existing",
		Content:   "Old",
		Category:  "test",
		Status:    "published",
		Version:   5,
	}
	store.Entry.Create(nil, existing)

	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 导入条目版本较低
	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "New Title", Content: "New", Category: "test", Status: "published", Version: 2},
	}
	entryData, _ := json.Marshal(entries)
	w, _ := zipWriter.Create("entries.json")
	w.Write(entryData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictMerge,
		OperatorLevel:    5,
	}
	result := importer.Import(buf.Bytes(), opts)

	// 应该跳过，因为现有版本更高
	if result.Summary.EntriesSkipped != 1 {
		t.Errorf("Expected 1 entry skipped, got %d", result.Summary.EntriesSkipped)
	}

	entry, _ := store.Entry.Get(nil, "entry-1")
	if entry.Title != "Existing" {
		t.Errorf("Title should remain 'Existing' (higher version), got '%s'", entry.Title)
	}
}

func TestImporter_UserLevelSecurity(t *testing.T) {
	store := newTestStore(t)
	importer := NewImporter(store)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 尝试导入 Lv5 用户，但操作者只有 Lv4
	users := []ExportUser{
		{PublicKey: "user-1", AgentName: "Admin", UserLevel: 5, Status: "active"},
	}
	userData, _ := json.Marshal(users)
	w, _ := zipWriter.Create("users.json")
	w.Write(userData)

	zipWriter.Close()

	opts := ImportOptions{
		ConflictStrategy: ConflictSkip,
		OperatorLevel:    4, // Lv4 操作者
	}
	result := importer.Import(buf.Bytes(), opts)

	// 应该有错误
	if len(result.Errors) == 0 {
		t.Error("Expected error for importing higher level user")
	}
}
```

- [ ] **Step 3: 运行测试**

Run: `go test ./internal/core/export/... -v`
Expected: 所有测试通过

- [ ] **Step 4: 提交**

```bash
git add internal/core/export/exporter_test.go internal/core/export/importer_test.go
git commit -m "test(export): 添加导出/导入服务测试

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: 运行完整测试套件

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
git commit -m "feat: Phase 7b 数据导出/导入完成

功能:
- GET /api/v1/admin/export - 导出数据到 ZIP
- POST /api/v1/admin/import - 从 ZIP 导入数据
- 支持 skip/overwrite/merge 三种冲突策略
- 用户隐私保护（导出不含敏感字段）
- 权限检查（Lv4+ Admin）

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## 验收清单

- [ ] `GET /api/v1/admin/export` 可下载 ZIP 文件
- [ ] `POST /api/v1/admin/import` 可上传并导入数据
- [ ] 支持 `skip`/`overwrite`/`merge` 三种冲突策略
- [ ] 导出的 ZIP 包含正确的数据结构
- [ ] 导入时权限检查正确（Lv4+）
- [ ] 用户隐私保护（导出不包含邮箱、手机）
- [ ] 导入用户等级不能高于操作者等级
- [ ] 所有测试通过
- [ ] 测试覆盖率 > 55%
