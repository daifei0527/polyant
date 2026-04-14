package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/daifei0527/agentwiki/internal/core/audit"
	"github.com/daifei0527/agentwiki/internal/storage/kv"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
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
