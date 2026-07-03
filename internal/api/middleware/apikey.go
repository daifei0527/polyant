package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

const (
	// headerApiKey API Key 请求头
	headerApiKey = "X-Polyant-Api-Key"
)

// ApiKeyMiddleware 验证 API Key
// 如果 validKey 为空，则跳过验证
func ApiKeyMiddleware(validKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果未配置 API Key，跳过验证
			if validKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			apiKey := r.Header.Get(headerApiKey)
			// 恒定时间比较，抗时序侧信道。空请求 key 直接拒（避免与空 validKey 混淆）。
			if apiKey == "" || subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) != 1 {
				writeJSONError(w, http.StatusUnauthorized, "Missing or invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeJSONError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    code,
		"message": message,
	})
}
