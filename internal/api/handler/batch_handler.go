package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/index"
	"github.com/daifei0527/agentwiki/internal/storage/linkparser"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
)

// BatchHandler 批量操作 HTTP 处理器
// 负责处理知识条目的批量创建、更新、删除操作
type BatchHandler struct {
	entryStore   storage.EntryStore
	searchEngine index.SearchEngine
	backlink     storage.BacklinkIndex
	userStore    storage.UserStore
}

// NewBatchHandler 创建新的 BatchHandler 实例
func NewBatchHandler(entryStore storage.EntryStore, searchEngine index.SearchEngine, backlinkIndex storage.BacklinkIndex, userStore storage.UserStore) *BatchHandler {
	return &BatchHandler{
		entryStore:   entryStore,
		searchEngine: searchEngine,
		backlink:     backlinkIndex,
		userStore:    userStore,
	}
}

// BatchCreateHandler 批量创建知识条目
// POST /api/v1/entries/batch
// 需要认证（Lv1及以上权限）
// 最多100条/批次
func (h *BatchHandler) BatchCreateHandler(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req BatchCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
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

	// 验证批量大小
	if len(req.Entries) == 0 {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}
	if len(req.Entries) > MaxBatchSize {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "batch size exceeds maximum limit of 100",
			Data:    nil,
		})
		return
	}

	// 预验证所有条目
	validationErrors := h.validateCreateEntries(req.Entries)
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, &BatchResponse{
			Success: false,
			Summary: BatchSummary{
				Total:  len(req.Entries),
				Failed: len(validationErrors),
			},
			Results: nil,
			Errors:  validationErrors,
		})
		return
	}

	// 执行批量创建
	response := h.executeBatchCreate(r, req.Entries, user, req.Options)

	status := http.StatusCreated
	if response.Summary.Failed > 0 {
		status = http.StatusOK // 部分成功
	}

	writeJSON(w, status, response)
}

// BatchUpdateHandler 批量更新知识条目
// PUT /api/v1/entries/batch
// 需要认证，只有创建者或Lv3+用户可更新
// 最多100条/批次
func (h *BatchHandler) BatchUpdateHandler(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req BatchUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 获取当前用户信息
	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	// 检查用户权限（Lv1及以上）
	if user.UserLevel < model.UserLevelLv1 {
		writeError(w, awerrors.ErrBasicUserDenied)
		return
	}

	// 验证批量大小
	if len(req.Entries) == 0 {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}
	if len(req.Entries) > MaxBatchSize {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "batch size exceeds maximum limit of 100",
			Data:    nil,
		})
		return
	}

	// 预验证所有条目
	validationErrors := h.validateUpdateEntries(r.Context(), req.Entries, user)
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, &BatchResponse{
			Success: false,
			Summary: BatchSummary{
				Total:  len(req.Entries),
				Failed: len(validationErrors),
			},
			Results: nil,
			Errors:  validationErrors,
		})
		return
	}

	// 执行批量更新
	response := h.executeBatchUpdate(r, req.Entries, user)

	status := http.StatusOK
	if response.Summary.Failed > 0 {
		status = http.StatusOK // 部分成功
	}

	writeJSON(w, status, response)
}

// BatchDeleteHandler 批量删除知识条目
// DELETE /api/v1/entries/batch
// 需要认证，只有创建者或Lv4+用户可删除
// 最多100条/批次
func (h *BatchHandler) BatchDeleteHandler(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req BatchDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 获取当前用户信息
	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	// 检查用户权限（Lv1及以上）
	if user.UserLevel < model.UserLevelLv1 {
		writeError(w, awerrors.ErrBasicUserDenied)
		return
	}

	// 验证批量大小
	if len(req.IDs) == 0 {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}
	if len(req.IDs) > MaxBatchSize {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "batch size exceeds maximum limit of 100",
			Data:    nil,
		})
		return
	}

	// 预验证所有条目
	validationErrors := h.validateDeleteEntries(r.Context(), req.IDs, user)
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, &BatchResponse{
			Success: false,
			Summary: BatchSummary{
				Total:  len(req.IDs),
				Failed: len(validationErrors),
			},
			Results: nil,
			Errors:  validationErrors,
		})
		return
	}

	// 执行批量删除
	response := h.executeBatchDelete(r, req.IDs, user)

	status := http.StatusOK
	if response.Summary.Failed > 0 {
		status = http.StatusOK // 部分成功
	}

	writeJSON(w, status, response)
}

// validateCreateEntries 验证批量创建条目
// 返回验证错误列表，如果为空则验证通过
func (h *BatchHandler) validateCreateEntries(entries []BatchEntry) []BatchError {
	var errors []BatchError

	for i, entry := range entries {
		// 验证必填字段
		if entry.Title == "" {
			errors = append(errors, BatchError{
				Index:   i,
				Field:   "title",
				Message: "title is required",
			})
		}
		if entry.Content == "" {
			errors = append(errors, BatchError{
				Index:   i,
				Field:   "content",
				Message: "content is required",
			})
		}
		if entry.Category == "" {
			errors = append(errors, BatchError{
				Index:   i,
				Field:   "category",
				Message: "category is required",
			})
		}
	}

	return errors
}

// validateUpdateEntries 验证批量更新条目
// 检查条目是否存在和权限
func (h *BatchHandler) validateUpdateEntries(ctx context.Context, entries []BatchUpdateEntry, user *model.User) []BatchError {
	var errors []BatchError

	for i, entry := range entries {
		// 验证ID必填
		if entry.ID == "" {
			errors = append(errors, BatchError{
				Index:   i,
				Field:   "id",
				Message: "id is required",
			})
			continue
		}

		// 检查条目是否存在
		existing, err := h.entryStore.Get(ctx, entry.ID)
		if err != nil {
			errors = append(errors, BatchError{
				Index:   i,
				Field:   "id",
				Message: "entry not found",
			})
			continue
		}

		// 权限检查：创建者或Lv3+可更新
		if existing.CreatedBy != user.PublicKey && user.UserLevel < model.UserLevelLv3 {
			errors = append(errors, BatchError{
				Index:   i,
				Field:   "id",
				Message: "permission denied",
			})
		}
	}

	return errors
}

// validateDeleteEntries 验证批量删除条目
// 检查条目是否存在和权限
func (h *BatchHandler) validateDeleteEntries(ctx context.Context, ids []string, user *model.User) []BatchError {
	var errors []BatchError

	for i, id := range ids {
		// 验证ID必填
		if id == "" {
			errors = append(errors, BatchError{
				Index:   i,
				Field:   "id",
				Message: "id is required",
			})
			continue
		}

		// 检查条目是否存在
		existing, err := h.entryStore.Get(ctx, id)
		if err != nil {
			errors = append(errors, BatchError{
				Index:   i,
				Field:   "id",
				Message: "entry not found",
			})
			continue
		}

		// 权限检查：创建者或Lv4+可删除
		if existing.CreatedBy != user.PublicKey && user.UserLevel < model.UserLevelLv4 {
			errors = append(errors, BatchError{
				Index:   i,
				Field:   "id",
				Message: "permission denied",
			})
		}
	}

	return errors
}

// executeBatchCreate 执行批量创建
func (h *BatchHandler) executeBatchCreate(r *http.Request, entries []BatchEntry, user *model.User, options BatchOptions) *BatchResponse {
	response := &BatchResponse{
		Success: true,
		Summary: BatchSummary{
			Total: len(entries),
		},
		Results: make([]BatchResult, 0, len(entries)),
		Errors:  nil,
	}

	for i, entry := range entries {
		// 计算内容哈希
		contentHash := computeContentHash(entry.Title, entry.Content, entry.Category)

		// 生成UUID作为条目ID
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

		// 存储条目
		created, err := h.entryStore.Create(r.Context(), newEntry)
		if err != nil {
			response.Success = false
			response.Summary.Failed++
			response.Results = append(response.Results, BatchResult{
				Index:  i,
				ID:     "",
				Status: "failed",
				Reason: err.Error(),
			})
			continue
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

		response.Summary.Created++
		response.Results = append(response.Results, BatchResult{
			Index:   i,
			ID:      created.ID,
			Status:  "created",
			Version: created.Version,
		})
	}

	return response
}

// executeBatchUpdate 执行批量更新
func (h *BatchHandler) executeBatchUpdate(r *http.Request, entries []BatchUpdateEntry, user *model.User) *BatchResponse {
	response := &BatchResponse{
		Success: true,
		Summary: BatchSummary{
			Total: len(entries),
		},
		Results: make([]BatchResult, 0, len(entries)),
		Errors:  nil,
	}

	for i, entry := range entries {
		// 获取现有条目
		existing, err := h.entryStore.Get(r.Context(), entry.ID)
		if err != nil {
			response.Success = false
			response.Summary.Failed++
			response.Summary.NotFound++
			response.Results = append(response.Results, BatchResult{
				Index:  i,
				ID:     entry.ID,
				Status: "not_found",
				Reason: "entry not found",
			})
			continue
		}

		// 应用更新
		if entry.Title != nil {
			existing.Title = *entry.Title
		}
		if entry.Content != nil {
			existing.Content = *entry.Content
		}
		if entry.JsonData != nil {
			existing.JSONData = entry.JsonData
		}
		if entry.Category != nil {
			existing.Category = *entry.Category
		}
		if entry.Tags != nil {
			existing.Tags = *entry.Tags
		}

		// 递增版本号
		existing.Version++
		existing.UpdatedAt = model.NowMillis()

		// 重新计算内容哈希
		existing.ContentHash = computeContentHash(existing.Title, existing.Content, existing.Category)

		// 执行更新
		updated, err := h.entryStore.Update(r.Context(), existing)
		if err != nil {
			response.Success = false
			response.Summary.Failed++
			response.Results = append(response.Results, BatchResult{
				Index:  i,
				ID:     entry.ID,
				Status: "failed",
				Reason: err.Error(),
			})
			continue
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

		response.Summary.Updated++
		response.Results = append(response.Results, BatchResult{
			Index:   i,
			ID:      updated.ID,
			Status:  "updated",
			Version: updated.Version,
		})
	}

	return response
}

// executeBatchDelete 执行批量删除
func (h *BatchHandler) executeBatchDelete(r *http.Request, ids []string, user *model.User) *BatchResponse {
	response := &BatchResponse{
		Success: true,
		Summary: BatchSummary{
			Total: len(ids),
		},
		Results: make([]BatchResult, 0, len(ids)),
		Errors:  nil,
	}

	for i, id := range ids {
		// 检查条目是否存在
		_, err := h.entryStore.Get(r.Context(), id)
		if err != nil {
			response.Success = false
			response.Summary.Failed++
			response.Summary.NotFound++
			response.Results = append(response.Results, BatchResult{
				Index:  i,
				ID:     id,
				Status: "not_found",
				Reason: "entry not found",
			})
			continue
		}

		// 执行软删除
		if err := h.entryStore.Delete(r.Context(), id); err != nil {
			response.Success = false
			response.Summary.Failed++
			response.Results = append(response.Results, BatchResult{
				Index:  i,
				ID:     id,
				Status: "failed",
				Reason: err.Error(),
			})
			continue
		}

		// 从全文索引中删除
		if h.searchEngine != nil {
			_ = h.searchEngine.DeleteIndex(id)
		}

		// 从反向链接索引中删除
		if h.backlink != nil {
			_ = h.backlink.DeleteIndex(id)
		}

		response.Summary.Deleted++
		response.Results = append(response.Results, BatchResult{
			Index: i,
			ID:    id,
			Status: "deleted",
		})
	}

	return response
}
