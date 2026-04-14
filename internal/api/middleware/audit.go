package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/daifei0527/agentwiki/internal/core/audit"
	"github.com/daifei0527/agentwiki/internal/storage/kv"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// AuditMiddleware 审计中间件
type AuditMiddleware struct {
	auditSvc *audit.Service
}

// NewAuditMiddleware 创建审计中间件
func NewAuditMiddleware(auditStore kv.AuditStore) *AuditMiddleware {
	return &AuditMiddleware{
		auditSvc: audit.NewService(auditStore),
	}
}

// responseWriter 包装 ResponseWriter 以捕获响应
type responseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	mu     sync.Mutex
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// Middleware 返回审计中间件处理函数
func (m *AuditMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. 检查是否为敏感操作
		actionType := audit.GetActionType(r.Method, r.URL.Path)
		if actionType == "" {
			next.ServeHTTP(w, r)
			return
		}

		// 2. 缓冲请求体
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 限制 1MB
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// 3. 提取操作者信息
		operatorPubkey, _ := r.Context().Value(PublicKeyKey).(string)
		operatorLevel, _ := r.Context().Value(UserLevelKey).(int32)

		// 4. 包装 ResponseWriter 捕获响应
		rw := &responseWriter{
			ResponseWriter: w,
			status:         200,
		}

		// 5. 调用下一个 Handler
		next.ServeHTTP(rw, r)

		// 6. 异步写入审计日志
		go m.writeAuditLog(r.Context(), &model.AuditLog{
			Timestamp:      time.Now().UnixMilli(),
			OperatorPubkey: operatorPubkey,
			OperatorLevel:  operatorLevel,
			OperatorIP:     getClientIP(r),
			UserAgent:      r.UserAgent(),
			Method:         r.Method,
			Path:           r.URL.Path,
			ActionType:     actionType,
			TargetID:       audit.ExtractTargetID(r.URL.Path),
			TargetType:     audit.GetTargetType(actionType),
			RequestBody:    string(bodyBytes),
			ResponseCode:   rw.status,
			ResponseBody:   rw.body.String(),
			Success:        rw.status < 400,
			ErrorMessage:   getErrorMessage(rw.body.String()),
		})
	})
}

// writeAuditLog 写入审计日志
func (m *AuditMiddleware) writeAuditLog(ctx context.Context, log *model.AuditLog) {
	log.ID = model.NewAuditLog().ID
	if err := m.auditSvc.Log(ctx, log); err != nil {
		// 记录失败不影响主流程，只打印日志
	}
}

// getClientIP 获取客户端 IP
func getClientIP(r *http.Request) string {
	// 尝试从 X-Forwarded-For 获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	// 尝试从 X-Real-IP 获取
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// 使用 RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// getErrorMessage 从响应体提取错误信息
func getErrorMessage(responseBody string) string {
	if responseBody == "" {
		return ""
	}

	var resp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(responseBody), &resp); err != nil {
		return ""
	}
	return resp.Message
}
