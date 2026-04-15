// internal/api/admin/middleware.go
package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	coreadmin "github.com/daifei0527/polyant/internal/core/admin"
	awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// AuthMiddleware Admin 认证中间件
type AuthMiddleware struct {
	sessionMgr *coreadmin.SessionManager
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(sessionMgr *coreadmin.SessionManager) *AuthMiddleware {
	return &AuthMiddleware{sessionMgr: sessionMgr}
}

// Middleware 验证 Session Token
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从 Header 获取 Token
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeAdminError(w, awerrors.ErrMissingAuth)
			return
		}

		// 解析 Bearer Token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeAdminError(w, awerrors.ErrInvalidSignature)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// 验证 Token
		publicKey, valid := m.sessionMgr.ValidateSession(token)
		if !valid {
			writeAdminError(w, awerrors.New(401, awerrors.CategoryAPI, "会话已过期", http.StatusUnauthorized))
			return
		}

		// 将公钥注入上下文
		ctx := context.WithValue(r.Context(), "public_key", publicKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LocalOnlyMiddleware 限制仅本地访问
func LocalOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLocalRequest(r) {
			writeAdminError(w, awerrors.New(403, awerrors.CategoryAPI, "仅限本地访问", http.StatusForbidden))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeAdminJSONWithEncoder writes JSON response using the package-level writeAdminJSON
// This is an alias for consistency with middleware package patterns
func writeAdminJSONWithEncoder(w http.ResponseWriter, status int, data interface{}) {
	writeAdminJSON(w, status, data)
}

// writeAdminErrorWithEncoder writes error response
func writeAdminErrorWithEncoder(w http.ResponseWriter, err *awerrors.AWError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(err.HTTPStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    err.Code,
		"message": err.Message,
	})
}
