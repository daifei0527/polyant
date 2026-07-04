package middleware

import "net/http"

// BodyLimitMiddleware 限制请求体大小（R1-C2）。
// maxBytes<=0 表示不限制。已知 ContentLength 超限直接 413；否则用 http.MaxBytesReader
// 包装 r.Body，使下游读取超限时 Read 报错（防御流式/chunked 大 body 拖垮服务）。
func BodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maxBytes > 0 {
				if r.ContentLength > maxBytes {
					writeJSONError(w, http.StatusRequestEntityTooLarge, "Request body too large")
					return
				}
				if r.Body != nil {
					r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
