// internal/api/admin/session.go
package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/daifei0527/polyant/internal/core/admin"
	"github.com/daifei0527/polyant/internal/storage"
	awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// SessionHandler 会话处理器
type SessionHandler struct {
	sessionMgr *admin.SessionManager
	userStore  storage.UserStore
}

// NewSessionHandler 创建会话处理器
func NewSessionHandler(sessionMgr *admin.SessionManager, userStore storage.UserStore) *SessionHandler {
	return &SessionHandler{
		sessionMgr: sessionMgr,
		userStore:  userStore,
	}
}

// CreateSessionHandler 创建会话 (仅限本地访问)
// POST /api/v1/admin/session/create
func (h *SessionHandler) CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAdminError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	// 检查是否为本地访问
	if !isLocalRequest(r) {
		writeAdminError(w, awerrors.New(403, awerrors.CategoryAPI, "仅限本地访问", http.StatusForbidden))
		return
	}

	// 解析请求
	var req struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, awerrors.ErrJSONParse)
		return
	}

	if req.PublicKey == "" {
		writeAdminError(w, awerrors.ErrInvalidParams)
		return
	}

	// 验证用户是否存在
	user, err := h.userStore.Get(r.Context(), req.PublicKey)
	if err != nil {
		writeAdminError(w, awerrors.ErrUserNotFound)
		return
	}

	// 创建 Session Token
	token, err := h.sessionMgr.CreateSession(user.PublicKey)
	if err != nil {
		writeAdminError(w, awerrors.New(500, awerrors.CategoryAPI, "创建会话失败", http.StatusInternalServerError))
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"token":      token,
			"expires_at": time.Now().Add(24 * time.Hour).UnixMilli(),
			"user": map[string]interface{}{
				"public_key": user.PublicKey,
				"agent_name": user.AgentName,
				"user_level": user.UserLevel,
			},
		},
	})
}

// isLocalRequest 检查是否为本地请求
func isLocalRequest(r *http.Request) bool {
	host := r.Host
	// 检查 Host 是否为 127.0.0.1 或 localhost
	return host == "127.0.0.1:18531" ||
		host == "localhost:18531" ||
		r.RemoteAddr == "127.0.0.1" ||
		r.RemoteAddr == "[::1]"
}

func writeAdminJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeAdminError(w http.ResponseWriter, err *awerrors.AWError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(err.HTTPStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    err.Code,
		"message": err.Message,
	})
}
