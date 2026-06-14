// internal/api/admin/session.go
package admin

import (
	"encoding/json"
	"net"
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
	localHost  string // 期望的本地监听地址（如 "127.0.0.1:18531"），用于 isLocalRequest
}

// NewSessionHandler 创建会话处理器。localHost 为期望的本地监听地址（来自配置）。
func NewSessionHandler(sessionMgr *admin.SessionManager, userStore storage.UserStore, localHost string) *SessionHandler {
	return &SessionHandler{
		sessionMgr: sessionMgr,
		userStore:  userStore,
		localHost:  localHost,
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
	if !isLocalRequest(r, h.localHost) {
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

// isLocalRequest 检查是否为本地请求。
// localHost 为配置的本地监听地址（如 "127.0.0.1:18531"）。
// 主判断用连接级 RemoteAddr（难以伪造）；Host 头作为辅助，按 localHost 的端口比对。
// 注意：不支持反向代理（X-Forwarded-For 等不被信任）。
func isLocalRequest(r *http.Request, localHost string) bool {
	// 连接级判断（主）：RemoteAddr 形如 "127.0.0.1:54321"
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if host == "127.0.0.1" || host == "::1" {
			return true
		}
	} else if r.RemoteAddr == "127.0.0.1" || r.RemoteAddr == "::1" {
		return true // 无端口的回退
	}

	// Host 头辅助判断（与配置的本地监听地址比对，端口可配置）
	if r.Host == localHost {
		return true
	}
	if _, port, err := net.SplitHostPort(localHost); err == nil && port != "" {
		if r.Host == "localhost:"+port || r.Host == "127.0.0.1:"+port || r.Host == "[::1]:"+port {
			return true
		}
	}
	return false
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
