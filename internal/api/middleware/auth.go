// Package middleware 定义了 AgentWiki API 的 HTTP 中间件。
// 包含认证、CORS、日志等中间件实现。
package middleware

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
)

const (
	// headerPublicKey 公钥请求头
	headerPublicKey = "X-AgentWiki-PublicKey"
	// headerTimestamp 时间戳请求头
	headerTimestamp = "X-AgentWiki-Timestamp"
	// headerSignature 签名请求头
	headerSignature = "X-AgentWiki-Signature"
	// headerAuthorization 传统 Authorization 头（兼容）
	headerAuthorization = "Authorization"
	// maxTimestampDrift 最大时间戳偏差（毫秒），5分钟
	maxTimestampDrift = 5 * 60 * 1000
)

// userContextKey 用户信息上下文键类型
type userContextKey string

const (
	// UserKey 用户信息在 context 中的键
	UserKey userContextKey = "user"
)

// AuthMiddleware Ed25519 签名认证中间件
// 解析请求头中的公钥、时间戳和签名，验证请求合法性
// 验证通过后将用户信息注入到请求上下文中
type AuthMiddleware struct {
	userStore storage.UserStore
}

// NewAuthMiddleware 创建认证中间件实例
func NewAuthMiddleware(userStore storage.UserStore) *AuthMiddleware {
	return &AuthMiddleware{
		userStore: userStore,
	}
}

// Middleware 返回 HTTP 中间件处理函数
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 读取请求体（需要在签名验证中使用，同时保留给后续 handler）
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 限制 1MB
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// 提取认证信息
		publicKeyStr, timestampStr, signatureStr := extractAuthHeaders(r)

		// 验证必填字段
		if publicKeyStr == "" || timestampStr == "" || signatureStr == "" {
			writeAuthError(w, awerrors.ErrMissingAuth)
			return
		}

		// 解码公钥
		publicKey, err := base64.StdEncoding.DecodeString(publicKeyStr)
		if err != nil || len(publicKey) != ed25519.PublicKeySize {
			writeAuthError(w, awerrors.ErrInvalidSignature)
			return
		}

		// 解析时间戳
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			writeAuthError(w, awerrors.ErrTimestampExpired)
			return
		}

		// 验证时间戳是否在允许范围内（防止重放攻击）
		now := time.Now().UnixMilli()
		if absInt64(now-timestamp) > maxTimestampDrift {
			writeAuthError(w, awerrors.ErrTimestampExpired)
			return
		}

		// 解码签名
		signature, err := base64.StdEncoding.DecodeString(signatureStr)
		if err != nil || len(signature) != ed25519.SignatureSize {
			writeAuthError(w, awerrors.ErrInvalidSignature)
			return
		}

		// 构造签名内容: METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + SHA256(BODY)
		bodyHash := sha256.Sum256(bodyBytes)
		signContent := fmt.Sprintf("%s\n%s\n%d\n%s",
			r.Method, r.URL.Path, timestamp, hex.EncodeToString(bodyHash[:]))

		// 验证 Ed25519 签名
		if !ed25519.Verify(publicKey, []byte(signContent), signature) {
			writeAuthError(w, awerrors.ErrInvalidSignature)
			return
		}

		// 计算公钥哈希，查找用户
		pubKeyHashBytes := sha256.Sum256(publicKey)
		pubKeyHash := hex.EncodeToString(pubKeyHashBytes[:])

		user, err := m.userStore.Get(r.Context(), pubKeyHash)
		if err != nil {
			writeAuthError(w, awerrors.ErrUserNotFound)
			return
		}

		// 检查用户状态
		if user.Status == model.UserStatusSuspended {
			writeAuthError(w, awerrors.ErrUserSuspended)
			return
		}

		// 将用户信息注入上下文
		ctx := context.WithValue(r.Context(), UserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractAuthHeaders 从请求头中提取认证信息
// 支持两种格式：
// 1. X-AgentWiki-PublicKey / X-AgentWiki-Timestamp / X-AgentWiki-Signature
// 2. Authorization: Ed25519 base64signature (兼容格式)
func extractAuthHeaders(r *http.Request) (pubKey, timestamp, signature string) {
	pubKey = r.Header.Get(headerPublicKey)
	timestamp = r.Header.Get(headerTimestamp)
	signature = r.Header.Get(headerSignature)

	// 如果使用 Authorization 头格式
	if pubKey == "" && signature == "" {
		authHeader := r.Header.Get(headerAuthorization)
		if strings.HasPrefix(authHeader, "Ed25519 ") {
			signature = strings.TrimPrefix(authHeader, "Ed25519 ")
			// 公钥从查询参数获取
			pubKey = r.URL.Query().Get("pubkey")
		}
	}

	return pubKey, timestamp, signature
}

// GetUserFromContext 从请求上下文中获取用户信息
func GetUserFromContext(ctx context.Context) *model.User {
	if ctx == nil {
		return nil
	}
	val := ctx.Value(UserKey)
	if val == nil {
		return nil
	}
	user, ok := val.(*model.User)
	if !ok {
		return nil
	}
	return user
}

// writeAuthError 写入认证错误响应
func writeAuthError(w http.ResponseWriter, err *awerrors.AWError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(err.HTTPStatus)
	resp := map[string]interface{}{
		"code":    err.Code,
		"message": err.Message,
	}
	json.NewEncoder(w).Encode(resp)
}

// absInt64 返回 int64 的绝对值
func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
