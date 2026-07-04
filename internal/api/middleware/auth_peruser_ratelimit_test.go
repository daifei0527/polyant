package middleware

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAuthMiddleware_PerUserRateLimit: 同一已认证用户高频请求应被 per-user 限流（R1-D2）。
// 全局限流在 auth 之前运行、无法按用户区分；per-user 限流必须在认证后生效。
func TestAuthMiddleware_PerUserRateLimit(t *testing.T) {
	store, user, privKey := newTestUserStore(t)
	authMW := NewAuthMiddleware(store)
	defer authMW.Close()

	okCount := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		okCount++
		w.WriteHeader(http.StatusOK)
	})
	handler := authMW.Middleware(testHandler)

	lastCode := 0
	// 发送 25 个不同时间戳+body 的签名请求（避免重放保护误判），同一用户
	for i := 0; i < 25; i++ {
		body := []byte(fmt.Sprintf(`{"i":%d}`, i))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/test", bytes.NewBuffer(body))
		ts := time.Now().UnixMilli() + int64(i) // 不同时间戳 → 不同请求指纹
		bodyHash := sha256.Sum256(body)
		signContent := fmt.Sprintf("POST\n/api/v1/test\n%d\n%s", ts, hex.EncodeToString(bodyHash[:]))
		sig := ed25519.Sign(privKey, []byte(signContent))
		req.Header.Set("X-Polyant-PublicKey", user.PublicKey)
		req.Header.Set("X-Polyant-Timestamp", fmt.Sprintf("%d", ts))
		req.Header.Set("X-Polyant-Signature", base64.StdEncoding.EncodeToString(sig))

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		lastCode = rec.Code
	}

	assert.Less(t, okCount, 25, "per-user rate limit must block some of the 25 requests")
	assert.Greater(t, okCount, 0, "at least the burst quota should pass")
	assert.Equal(t, http.StatusTooManyRequests, lastCode, "later requests must be rate-limited (429)")
}
