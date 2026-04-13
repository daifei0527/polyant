package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/daifei0527/agentwiki/internal/core/user"
	"github.com/daifei0527/agentwiki/internal/storage"
	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
)

// AdminHandler 管理员 API 处理器
type AdminHandler struct {
	adminSvc *user.AdminService
}

// NewAdminHandler 创建管理员处理器
func NewAdminHandler(store *storage.Store) *AdminHandler {
	return &AdminHandler{
		adminSvc: user.NewAdminService(store),
	}
}

// BanUserRequest 封禁用户请求
type BanUserRequest struct {
	Reason string `json:"reason"`
}

// SetLevelRequest 设置等级请求
type SetLevelRequest struct {
	Level  int32  `json:"level"`
	Reason string `json:"reason"`
}

// BanUserHandler 封禁用户
// POST /api/v1/admin/users/{public_key}/ban
func (h *AdminHandler) BanUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	// 从 URL 获取目标用户公钥
	publicKey := extractAdminPathParam(r.URL.Path, "/api/v1/admin/users/", "/ban")
	if publicKey == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 解析请求体
	var req BanUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 从上下文获取管理员公钥
	adminPublicKey, _ := r.Context().Value("public_key").(string)

	// 执行封禁
	ctx := r.Context()
	if err := h.adminSvc.BanUser(ctx, publicKey, adminPublicKey, req.Reason); err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusBadRequest, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]bool{"success": true},
	})
}

// UnbanUserHandler 解封用户
// POST /api/v1/admin/users/{public_key}/unban
func (h *AdminHandler) UnbanUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	publicKey := extractAdminPathParam(r.URL.Path, "/api/v1/admin/users/", "/unban")
	if publicKey == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	adminPublicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	if err := h.adminSvc.UnbanUser(ctx, publicKey, adminPublicKey); err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusBadRequest, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]bool{"success": true},
	})
}

// SetUserLevelHandler 设置用户等级
// PUT /api/v1/admin/users/{public_key}/level
func (h *AdminHandler) SetUserLevelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	publicKey := extractAdminPathParam(r.URL.Path, "/api/v1/admin/users/", "/level")
	if publicKey == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	var req SetLevelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	adminPublicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	if err := h.adminSvc.SetUserLevel(ctx, publicKey, req.Level, adminPublicKey, req.Reason); err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusBadRequest, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"success":   true,
			"new_level": req.Level,
		},
	})
}

// GetUserStatsHandler 获取用户统计
// GET /api/v1/admin/stats/users
func (h *AdminHandler) GetUserStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	ctx := r.Context()
	stats, err := h.adminSvc.GetUserStats(ctx)
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

// ListUsersHandler 用户列表处理器
// GET /api/v1/admin/users?page=1&limit=20&level=1&search=keyword
func (h *AdminHandler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	// 解析查询参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	level, _ := strconv.Atoi(r.URL.Query().Get("level"))
	search := r.URL.Query().Get("search")

	// 获取用户列表
	ctx := r.Context()
	users, total, err := h.adminSvc.ListUsers(ctx, (page-1)*limit, limit, int32(level), search)
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, err.Error(), http.StatusInternalServerError, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"users": users,
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

// extractAdminPathParam 从 URL 中提取路径参数
// 用于管理员 API 路径如 /api/v1/admin/users/{public_key}/ban
func extractAdminPathParam(path, prefix, suffix string) string {
	if len(path) <= len(prefix)+len(suffix) {
		return ""
	}
	return path[len(prefix) : len(path)-len(suffix)]
}
