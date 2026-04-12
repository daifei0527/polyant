package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
)

// CategoryHandler 分类 HTTP 处理器
// 负责分类列表查询和分类下条目查询
type CategoryHandler struct {
	categoryStore storage.CategoryStore
	entryStore    storage.EntryStore
}

// NewCategoryHandler 创建新的 CategoryHandler 实例
func NewCategoryHandler(categoryStore storage.CategoryStore, entryStore storage.EntryStore) *CategoryHandler {
	return &CategoryHandler{
		categoryStore: categoryStore,
		entryStore:    entryStore,
	}
}

// ListCategoriesHandler 获取分类列表
// GET /api/v1/categories
// 返回所有内置和自定义分类的树形结构
func (h *CategoryHandler) ListCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	categories, err := h.categoryStore.ListAll(r.Context())
	if err != nil {
		writeError(w, awerrors.Wrap(302, awerrors.CategoryStorage, "failed to list categories", 500, err))
		return
	}

	// 构建树形结构
	tree := buildCategoryTree(categories)

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    tree,
	})
}

// GetCategoryEntriesHandler 获取分类下的知识条目
// GET /api/v1/categories/{path}/entries
// 支持分页查询指定分类下的所有条目
func (h *CategoryHandler) GetCategoryEntriesHandler(w http.ResponseWriter, r *http.Request) {
	path := extractPathVar(r, "path")
	if path == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 验证分类是否存在
	_, err := h.categoryStore.Get(r.Context(), path)
	if err != nil {
		writeError(w, awerrors.ErrCategoryNotFound)
		return
	}

	// 解析分页参数
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := parseInt(v); err == nil && n >= 0 {
			offset = n
		}
	}

	// 查询分类下的条目
	filter := storage.EntryFilter{
		Category: path,
		Status:   model.EntryStatusPublished,
		Limit:    limit,
		Offset:   offset,
		OrderBy:  "score",
		OrderDir: "desc",
	}

	entries, total, err := h.entryStore.List(r.Context(), filter)
	if err != nil {
		writeError(w, awerrors.Wrap(300, awerrors.CategoryStorage, "failed to list entries", 500, err))
		return
	}

	hasMore := total > (offset + len(entries))

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data: &PagedData{
			TotalCount: total,
			HasMore:    hasMore,
			Items:      entries,
		},
	})
}

// CategoryTreeNode 分类树节点，用于构建前端展示的树形结构
type CategoryTreeNode struct {
	ID        string              `json:"id"`
	Path      string              `json:"path"`
	Name      string              `json:"name"`
	Level     int32               `json:"level"`
	SortOrder int32               `json:"sort_order"`
	IsBuiltin bool                `json:"is_builtin"`
	Children  []*CategoryTreeNode `json:"children,omitempty"`
}

// buildCategoryTree 将扁平的分类列表构建为树形结构
func buildCategoryTree(categories []*model.Category) []*CategoryTreeNode {
	// 创建路径到节点的映射
	nodeMap := make(map[string]*CategoryTreeNode)
	for _, cat := range categories {
		nodeMap[cat.Path] = &CategoryTreeNode{
			ID:        cat.ID,
			Path:      cat.Path,
			Name:      cat.Name,
			Level:     cat.Level,
			SortOrder: cat.SortOrder,
			IsBuiltin: cat.IsBuiltin,
			Children:  nil,
		}
	}

	// 构建树形关系
	var roots []*CategoryTreeNode
	for _, cat := range categories {
		node := nodeMap[cat.Path]
		if cat.ParentId == "" || cat.Level == 0 {
			roots = append(roots, node)
		} else {
			if parent, ok := nodeMap[cat.ParentId]; ok {
				parent.Children = append(parent.Children, node)
			} else {
				roots = append(roots, node)
			}
		}
	}

	return roots
}

// CreateCategoryRequest 创建分类请求体
type CreateCategoryRequest struct {
	Path     string `json:"path"`                // 分类路径，如 "tech/programming/rust"
	Name     string `json:"name"`                // 分类显示名
	ParentID string `json:"parent_id,omitempty"` // 父分类ID（可选）
}

// CreateCategoryHandler 创建新分类
// POST /api/v1/categories/create
// 需要认证（Lv2及以上权限）
func (h *CategoryHandler) CreateCategoryHandler(w http.ResponseWriter, r *http.Request) {
	// 获取当前用户
	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	// 检查权限（Lv2及以上可创建分类）
	if user.UserLevel < model.UserLevelLv2 {
		writeError(w, awerrors.ErrBasicUserDenied)
		return
	}

	// 解析请求
	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 验证必填字段
	req.Path = strings.TrimSpace(req.Path)
	req.Name = strings.TrimSpace(req.Name)
	if req.Path == "" || req.Name == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 检查分类是否已存在
	existing, _ := h.categoryStore.Get(r.Context(), req.Path)
	if existing != nil {
		writeError(w, awerrors.New(304, awerrors.CategoryStorage, "category already exists", 409))
		return
	}

	// 计算层级
	level := int32(0)
	if req.ParentID != "" {
		parent, err := h.categoryStore.Get(r.Context(), req.ParentID)
		if err == nil {
			level = parent.Level + 1
		}
	} else {
		// 从路径计算层级
		level = int32(strings.Count(req.Path, "/"))
	}

	// 创建分类
	category := &model.Category{
		ID:          generateUUID(),
		Path:        req.Path,
		Name:        req.Name,
		ParentId:    req.ParentID,
		Level:       level,
		SortOrder:   0,
		IsBuiltin:   false,
		CreatedAt:   time.Now().Unix(),
	}

	created, err := h.categoryStore.Create(r.Context(), category)
	if err != nil {
		writeError(w, awerrors.Wrap(302, awerrors.CategoryStorage, "failed to create category", 500, err))
		return
	}

	writeJSON(w, http.StatusCreated, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    created,
	})
}
