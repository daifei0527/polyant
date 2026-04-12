package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/linkparser"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// RemoteQuerier 远程查询接口
type RemoteQuerier interface {
	SearchWithRemote(ctx context.Context, query storage.SearchQuery) (*storage.SearchResult, error)
}

// EntryHandler 知识条目 HTTP 处理器
// 负责处理知识条目的 CRUD 操作和搜索
type EntryHandler struct {
	entryStore    storage.EntryStore
	searchEngine  storage.SearchEngine
	backlink      storage.BacklinkIndex
	userStore     storage.UserStore
	remoteQuerier RemoteQuerier
}

// NewEntryHandler 创建新的 EntryHandler 实例
func NewEntryHandler(entryStore storage.EntryStore, searchEngine storage.SearchEngine, backlinkIndex storage.BacklinkIndex, userStore storage.UserStore) *EntryHandler {
	return &EntryHandler{
		entryStore:   entryStore,
		searchEngine: searchEngine,
		backlink:     backlinkIndex,
		userStore:    userStore,
	}
}

// SetRemoteQuerier 设置远程查询服务
func (h *EntryHandler) SetRemoteQuerier(rq RemoteQuerier) {
	h.remoteQuerier = rq
}

// SearchHandler 搜索知识条目
// GET /api/v1/search?q=keyword&cat=category&limit=10&offset=0
// 支持按关键词、分类、标签搜索，返回分页结果
// 支持远程查询：本地结果不足时查询种子节点
func (h *EntryHandler) SearchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	if len(q) < 2 {
		writeError(w, awerrors.ErrKeywordTooShort)
		return
	}

	// 解析分页参数
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	// 解析分类过滤
	category := r.URL.Query().Get("cat")

	// 解析标签过滤
	var tags []string
	if v := r.URL.Query().Get("tag"); v != "" {
		tags = strings.Split(v, ",")
	}

	// 解析最低评分
	minScore := 0.0
	if v := r.URL.Query().Get("min_score"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			minScore = f
		}
	}

	// 解析查询类型参数
	// local=只查本地, remote=包含远程查询
	queryType := r.URL.Query().Get("type")
	if queryType == "" {
		queryType = "remote" // 默认启用远程查询
	}

	// 构建搜索查询
	var categories []string
	if category != "" {
		categories = []string{category}
	}

	query := storage.SearchQuery{
		Keyword:    q,
		Categories: categories,
		Tags:       tags,
		Limit:      limit,
		Offset:     offset,
		MinScore:   minScore,
	}

	// 执行搜索
	var result *storage.SearchResult
	var err error

	if queryType == "remote" && h.remoteQuerier != nil {
		// 启用远程查询
		result, err = h.remoteQuerier.SearchWithRemote(r.Context(), query)
	} else {
		// 仅本地查询
		result, err = h.searchEngine.Search(r.Context(), query)
	}

	if err != nil {
		writeError(w, awerrors.Wrap(600, awerrors.CategorySearch, "search failed", 500, err))
		return
	}

	hasMore := result.TotalCount > (offset + len(result.Entries))

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data: &PagedData{
			TotalCount: result.TotalCount,
			HasMore:    hasMore,
			Items:      result.Entries,
		},
	})
}

// GetEntryHandler 获取单个知识条目详情
// GET /api/v1/entry/{id}
func (h *EntryHandler) GetEntryHandler(w http.ResponseWriter, r *http.Request) {
	id := extractPathVar(r, "id")
	if id == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	entry, err := h.entryStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, awerrors.ErrEntryNotFound)
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    entry,
	})
}

// CreateEntryHandler 创建新的知识条目
// POST /api/v1/entry
// 需要认证（Lv1及以上权限）
func (h *EntryHandler) CreateEntryHandler(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req CreateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 验证必填字段
	if req.Title == "" || req.Content == "" || req.Category == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 获取当前用户信息
	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	// 检查用户权限（Lv1及以上可创建条目）
	if user.UserLevel < model.UserLevelLv1 {
		writeError(w, awerrors.ErrBasicUserDenied)
		return
	}

	// 计算内容哈希
	contentHash := computeContentHash(req.Title, req.Content, req.Category)

	// 生成UUID作为条目ID
	entryID := generateUUID()

	now := model.NowMillis()

	entry := &model.KnowledgeEntry{
		ID:          entryID,
		Title:       req.Title,
		Content:     req.Content,
		JSONData:    req.JsonData,
		Category:    req.Category,
		Tags:        req.Tags,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   user.PublicKey,
		Score:       0,
		ScoreCount:  0,
		ContentHash: contentHash,
		Status:      model.EntryStatusPublished,
		License:     req.License,
		SourceRef:   req.SourceRef,
	}
	if entry.License == "" {
		entry.License = "CC-BY-SA-4.0"
	}

	// 存储条目
	created, err := h.entryStore.Create(r.Context(), entry)
	if err != nil {
		writeError(w, awerrors.Wrap(305, awerrors.CategoryStorage, "failed to create entry", 500, err))
		return
	}

	// 建立全文索引
	if h.searchEngine != nil {
		_ = h.searchEngine.IndexEntry(created)
	}

	// 建立反向链接索引
	if h.backlink != nil {
		linkedEntryIDs := linkparser.ParseLinks(created.Content)
		_ = h.backlink.UpdateIndex(created.ID, linkedEntryIDs)
	}

	writeJSON(w, http.StatusCreated, &APIResponse{
		Code:    0,
		Message: "success",
		Data: &CreateEntryResponse{
			ID:          created.ID,
			Version:     created.Version,
			CreatedAt:   created.CreatedAt,
			ContentHash: created.ContentHash,
		},
	})
}

// UpdateEntryHandler 更新知识条目
// PUT /api/v1/entry/{id}
// 需要认证，只有创建者或Lv3+用户可更新
func (h *EntryHandler) UpdateEntryHandler(w http.ResponseWriter, r *http.Request) {
	id := extractPathVar(r, "id")
	if id == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 获取当前用户
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

	// 获取现有条目
	existing, err := h.entryStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, awerrors.ErrEntryNotFound)
		return
	}

	// 权限检查：创建者或Lv3+可更新
	if existing.CreatedBy != user.PublicKey && user.UserLevel < model.UserLevelLv3 {
		writeError(w, awerrors.ErrPermissionDenied)
		return
	}

	// 解析更新请求
	var req UpdateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 应用更新
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

	// 递增版本号
	existing.Version++
	existing.UpdatedAt = model.NowMillis()

	// 重新计算内容哈希
	existing.ContentHash = computeContentHash(existing.Title, existing.Content, existing.Category)

	// 执行更新
	updated, err := h.entryStore.Update(r.Context(), existing)
	if err != nil {
		writeError(w, awerrors.Wrap(305, awerrors.CategoryStorage, "failed to update entry", 500, err))
		return
	}

	// 更新全文索引
	if h.searchEngine != nil {
		_ = h.searchEngine.UpdateIndex(updated)
	}

	// 更新反向链接索引
	if h.backlink != nil {
		linkedEntryIDs := linkparser.ParseLinks(updated.Content)
		_ = h.backlink.UpdateIndex(updated.ID, linkedEntryIDs)
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    updated,
	})
}

// DeleteEntryHandler 删除知识条目（软删除）
// DELETE /api/v1/entry/{id}
// 需要认证，只有创建者或Lv4+用户可删除
func (h *EntryHandler) DeleteEntryHandler(w http.ResponseWriter, r *http.Request) {
	id := extractPathVar(r, "id")
	if id == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 获取当前用户
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

	// 获取现有条目
	existing, err := h.entryStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, awerrors.ErrEntryNotFound)
		return
	}

	// 权限检查：创建者或Lv4+可删除
	if existing.CreatedBy != user.PublicKey && user.UserLevel < model.UserLevelLv4 {
		writeError(w, awerrors.ErrPermissionDenied)
		return
	}

	// 执行软删除
	if err := h.entryStore.Delete(r.Context(), id); err != nil {
		writeError(w, awerrors.Wrap(305, awerrors.CategoryStorage, "failed to delete entry", 500, err))
		return
	}

	// 从全文索引中删除
	if h.searchEngine != nil {
		_ = h.searchEngine.DeleteIndex(id)
	}

	// 从反向链接索引中删除
	if h.backlink != nil {
		_ = h.backlink.DeleteIndex(id)
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    nil,
	})
}

// computeContentHash 计算条目内容哈希（SHA-256）
// 用于数据完整性校验
func computeContentHash(title, content, category string) string {
	h := sha256.New()
	h.Write([]byte(title))
	h.Write([]byte(content))
	h.Write([]byte(category))
	return hex.EncodeToString(h.Sum(nil))
}

// GetBacklinksHandler 获取条目的反向链接列表
// GET /api/v1/entry/{id}/backlinks
// 返回所有链接到该条目的条目ID列表
func (h *EntryHandler) GetBacklinksHandler(w http.ResponseWriter, r *http.Request) {
	id := extractPathVar(r, "id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 先检查条目是否存在
	_, err := h.entryStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, awerrors.ErrEntryNotFound)
		return
	}

	if h.backlink == nil {
		// 反向链接索引未启用，返回空列表
		writeJSON(w, http.StatusOK, &APIResponse{
			Code:    0,
			Message: "success",
			Data:    []string{},
		})
		return
	}

	backlinks, err := h.backlink.GetBacklinks(id)
	if err != nil {
		writeError(w, awerrors.Wrap(306, awerrors.CategoryStorage, "failed to get backlinks", 500, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    backlinks,
	})
}

// GetOutlinksHandler 获取条目的正向链接列表
// GET /api/v1/entry/{id}/outlinks
// 返回该条目链接出去的所有条目ID列表
func (h *EntryHandler) GetOutlinksHandler(w http.ResponseWriter, r *http.Request) {
	id := extractPathVar(r, "id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 先检查条目是否存在
	_, err := h.entryStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, awerrors.ErrEntryNotFound)
		return
	}

	if h.backlink == nil {
		// 反向链接索引未启用，返回空列表
		writeJSON(w, http.StatusOK, &APIResponse{
			Code:    0,
			Message: "success",
			Data:    []string{},
		})
		return
	}

	outlinks, err := h.backlink.GetOutlinks(id)
	if err != nil {
		writeError(w, awerrors.Wrap(306, awerrors.CategoryStorage, "failed to get outlinks", 500, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    outlinks,
	})
}
