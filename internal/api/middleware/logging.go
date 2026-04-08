package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"time"
)

// responseRecorder 用于记录响应状态码和大小
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    int
}

// WriteHeader 记录状态码
func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Write 记录写入字节数
func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += n
	return n, err
}

// LoggingMiddleware 请求日志中间件
// 记录每个 HTTP 请求的方法、路径、状态码、耗时等信息
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装 ResponseWriter 以记录状态码
		rec := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// 处理请求
		next.ServeHTTP(rec, r)

		// 计算请求耗时
		duration := time.Since(start)

		// 记录请求日志
		log.Printf("[%s] %s %s %d %d %v",
			r.RemoteAddr,
			r.Method,
			r.URL.Path,
			rec.statusCode,
			rec.written,
			duration,
		)
	})
}

// RecoveryMiddleware 异常恢复中间件
// 捕获 handler 中的 panic，防止服务崩溃
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC] %s %s: %v", r.Method, r.URL.Path, err)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"code":0,"message":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestIDMiddleware 请求ID中间件
// 为每个请求生成唯一ID，便于日志追踪
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查是否已有请求ID
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = generateRequestID()
		}
		// 设置响应头
		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, r)
	})
}

// generateRequestID 生成简单的请求ID
func generateRequestID() string {
	return time.Now().Format("20060102-150405.000") + "-" + randomHex(4)
}

// randomHex 生成指定字节长度的随机十六进制字符串
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
