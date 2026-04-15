// internal/api/admin/handler.go
package admin

import (
	"net/http"

	"github.com/daifei0527/polyant/internal/api/handler"
	"github.com/daifei0527/polyant/internal/storage"
)

// Handler Admin API 处理器
type Handler struct {
	adminHandler *handler.AdminHandler
	statsHandler *handler.StatsHandler
}

// NewHandler 创建 Admin 处理器
func NewHandler(store *storage.Store) *Handler {
	return &Handler{
		adminHandler: handler.NewAdminHandler(store),
		statsHandler: handler.NewStatsHandler(store),
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
