// internal/api/admin/session.go
package admin

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/daifei0527/polyant/internal/core/admin"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/daifei0527/polyant/pkg/crypto"
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

	// 等级门控：仅 Lv4+ 可签发 admin 会话（即便来自 localhost）
	if user.UserLevel < model.UserLevelLv4 {
		writeAdminError(w, awerrors.ErrPermissionDenied)
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

// LoginHandler 密码登录（Web admin 远程登录入口）。
// POST /api/v1/admin/session/login  {"identifier":"<email 或公钥>","password":"..."}
// 成功返回 session token；任何失败统一返回 401"凭证无效"（不区分用户不存在与密码错，防枚举），
// 等级不足返回 403。
func (h *SessionHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAdminError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}
	var req struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	// body 限制 64KB（登录载荷很小；全局 body 限制中间件落地前先在此兜底）
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeAdminError(w, awerrors.ErrJSONParse)
		return
	}
	if req.Identifier == "" || req.Password == "" {
		writeAdminError(w, awerrors.ErrInvalidParams)
		return
	}

	// 查找用户：先按邮箱（Web 登录主路径），再按公钥（Get 内部会哈希原始公钥）
	user, err := h.userStore.GetByEmail(r.Context(), req.Identifier)
	if err != nil || user == nil {
		user, err = h.userStore.Get(r.Context(), req.Identifier)
		if err != nil || user == nil {
			writeAdminError(w, awerrors.New(401, awerrors.CategoryAPI, "凭证无效", http.StatusUnauthorized))
			return
		}
	}

	// 密码校验（bcrypt 恒定时间）；未设密码（空 hash）直接拒
	if !crypto.CheckPassword(user.PasswordHash, req.Password) {
		writeAdminError(w, awerrors.New(401, awerrors.CategoryAPI, "凭证无效", http.StatusUnauthorized))
		return
	}

	// 等级门控：仅 Lv4+ 可登录 admin
	if user.UserLevel < model.UserLevelLv4 {
		writeAdminError(w, awerrors.ErrPermissionDenied)
		return
	}

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

// GetSessionHandler 校验 Bearer token 并返回当前用户（供 SPA 刷新恢复会话）。
// GET /api/v1/admin/session
func (h *SessionHandler) GetSessionHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeAdminError(w, awerrors.New(401, awerrors.CategoryAPI, "未认证", http.StatusUnauthorized))
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	pubKey, ok := h.sessionMgr.ValidateSession(token)
	if !ok {
		writeAdminError(w, awerrors.New(401, awerrors.CategoryAPI, "会话已过期", http.StatusUnauthorized))
		return
	}
	// Get 内部对原始公钥自动哈希，故用 session 中存的 pubKey 直接查
	user, err := h.userStore.Get(r.Context(), pubKey)
	if err != nil || user == nil {
		writeAdminError(w, awerrors.ErrUserNotFound)
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"public_key": user.PublicKey,
			"agent_name": user.AgentName,
			"user_level": user.UserLevel,
		},
	})
}

// isLocalRequest 检查是否为本地请求，仅信任连接级 RemoteAddr（loopback）。
// 永不信任客户端可控的 Host 头（可被远程攻击者伪造以绕过本地判定），
// 也不信任 X-Forwarded-For（不支持反向代理）。
func isLocalRequest(r *http.Request, localHost string) bool {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if host == "127.0.0.1" || host == "::1" {
			return true
		}
	} else if r.RemoteAddr == "127.0.0.1" || r.RemoteAddr == "::1" {
		return true // 无端口的回退
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
