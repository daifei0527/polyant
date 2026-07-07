// internal/api/admin/handler.go
package admin

import (
	"net/http"

	"github.com/daifei0527/polyant/internal/api/handler"
	"github.com/daifei0527/polyant/internal/core/review"
	"github.com/daifei0527/polyant/internal/storage"
)

// Handler Admin API 处理器
type Handler struct {
	adminHandler  *handler.AdminHandler
	statsHandler  *handler.StatsHandler
	reviewHandler *handler.ReviewHandler
}

// NewHandler 创建 Admin 处理器
func NewHandler(store *storage.Store, entryPusher handler.EntryPusher) *Handler {
	reviewSvc := review.NewService(store, entryPusher)
	return &Handler{
		adminHandler:  handler.NewAdminHandler(store),
		statsHandler:  handler.NewStatsHandler(store),
		reviewHandler: handler.NewReviewHandler(reviewSvc),
	}
}

// ListUsersHandler 用户列表处理器
func (h *Handler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	h.adminHandler.ListUsersHandler(w, r)
}

// BanUserHandler 封禁用户处理器
func (h *Handler) BanUserHandler(w http.ResponseWriter, r *http.Request) {
	h.adminHandler.BanUserHandler(w, r)
}

// UnbanUserHandler 解封用户处理器
func (h *Handler) UnbanUserHandler(w http.ResponseWriter, r *http.Request) {
	h.adminHandler.UnbanUserHandler(w, r)
}

// SetUserLevelHandler 设置用户等级处理器
func (h *Handler) SetUserLevelHandler(w http.ResponseWriter, r *http.Request) {
	h.adminHandler.SetUserLevelHandler(w, r)
}

// GetUserStatsHandler 获取用户统计处理器
func (h *Handler) GetUserStatsHandler(w http.ResponseWriter, r *http.Request) {
	h.adminHandler.GetUserStatsHandler(w, r)
}

// GetContributionStatsHandler 获取贡献统计处理器
func (h *Handler) GetContributionStatsHandler(w http.ResponseWriter, r *http.Request) {
	h.statsHandler.GetContributionStatsHandler(w, r)
}

// GetActivityTrendHandler 获取活跃度趋势处理器
func (h *Handler) GetActivityTrendHandler(w http.ResponseWriter, r *http.Request) {
	h.statsHandler.GetActivityTrendHandler(w, r)
}

// GetRegistrationTrendHandler 获取注册趋势处理器
func (h *Handler) GetRegistrationTrendHandler(w http.ResponseWriter, r *http.Request) {
	h.statsHandler.GetRegistrationTrendHandler(w, r)
}

// GetEntryStatsHandler 获取条目统计处理器
func (h *Handler) GetEntryStatsHandler(w http.ResponseWriter, r *http.Request) {
	h.statsHandler.GetEntryStatsHandler(w, r)
}

// ListReviewQueueHandler 列出审核队列处理器
func (h *Handler) ListReviewQueueHandler(w http.ResponseWriter, r *http.Request) {
	h.reviewHandler.ListReviewQueueHandler(w, r)
}

// ApproveEntryHandler 审核通过处理器
func (h *Handler) ApproveEntryHandler(w http.ResponseWriter, r *http.Request) {
	h.reviewHandler.ApproveEntryHandler(w, r)
}

// RejectEntryHandler 审核拒绝处理器
func (h *Handler) RejectEntryHandler(w http.ResponseWriter, r *http.Request) {
	h.reviewHandler.RejectEntryHandler(w, r)
}

// TakedownEntryHandler 下架处理器
func (h *Handler) TakedownEntryHandler(w http.ResponseWriter, r *http.Request) {
	h.reviewHandler.TakedownEntryHandler(w, r)
}
