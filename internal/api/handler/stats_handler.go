package handler

import (
	"net/http"
	"strconv"

	"github.com/daifei0527/polyant/internal/core/user"
	"github.com/daifei0527/polyant/internal/storage"
	awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// StatsHandler 统计 API 处理器
type StatsHandler struct {
	statsSvc *user.StatsService
}

// NewStatsHandler 创建统计处理器
func NewStatsHandler(store *storage.Store) *StatsHandler {
	return &StatsHandler{
		statsSvc: user.NewStatsService(store),
	}
}

// GetUserStatsHandler 获取用户统计概览
// GET /api/v1/admin/stats/users
func (h *StatsHandler) GetUserStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	stats, err := h.statsSvc.GetUserStats(r.Context())
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusInternalServerError, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// GetContributionStatsHandler 获取贡献明细
// GET /api/v1/admin/stats/contributions?page=1&limit=20&sort=entry_count
func (h *StatsHandler) GetContributionStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "entry_count"
	}

	contribs, total, err := h.statsSvc.GetContributionStats(r.Context(), (page-1)*limit, limit, sortBy)
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusInternalServerError, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"contributions": contribs,
			"total":         total,
			"page":          page,
			"limit":         limit,
		},
	})
}

// GetActivityTrendHandler 获取活跃度趋势
// GET /api/v1/admin/stats/activity?days=30
func (h *StatsHandler) GetActivityTrendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days < 1 || days > 365 {
		days = 30
	}

	trend, err := h.statsSvc.GetActivityTrend(r.Context(), days)
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusInternalServerError, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]interface{}{"trend": trend},
	})
}

// GetRegistrationTrendHandler 获取注册趋势
// GET /api/v1/admin/stats/registrations?days=30
func (h *StatsHandler) GetRegistrationTrendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days < 1 || days > 365 {
		days = 30
	}

	trend, err := h.statsSvc.GetRegistrationTrend(r.Context(), days)
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusInternalServerError, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]interface{}{"trend": trend},
	})
}
