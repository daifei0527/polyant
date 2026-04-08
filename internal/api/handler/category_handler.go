package handler

import (
	"net/http"

	awerrors "github.com/agentwiki/agentwiki/pkg/errors"
	"github.com/agentwiki/agentwiki/internal/storage/model"
	"github.com/agentwiki/agentwiki/internal/storage"
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
