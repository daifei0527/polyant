package middleware

import (
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
			if apiKey == "" || apiKey != validKey {
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
