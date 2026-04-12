package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/daifei0527/agentwiki/internal/api/middleware"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
)

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// writeError 写入错误响应
// 自动从 AWError 中提取错误码和 HTTP 状态码
func writeError(w http.ResponseWriter, err *awerrors.AWError) {
	status := err.HTTPStatus
	if status == 0 {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, &APIResponse{
		Code:    err.Code,
		Message: err.Message,
		Data:    nil,
	})
}

// extractPathVar 从 URL 路径中提取路径参数
// 支持标准库 net/http 的路由模式
// 对于 /api/v1/entry/{id} 模式，使用 name="id" 提取
// 对于 /api/v1/entry/update/{id} 模式，使用 name="id" 提取最后一个路径段
func extractPathVar(r *http.Request, name string) string {
	path := strings.TrimSuffix(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	// 策略1: 查找路径中与name匹配的段，返回下一段
	for i, part := range parts {
		if part == name && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	// 策略2: 对于 /api/v1/entry/{id} 模式，提取最后一个非空段
	// 常见模式: /api/v1/entry/uuid -> 返回uuid
	if name == "id" && len(parts) >= 5 {
		// /api/v1/entry/{id} -> parts = ["", "api", "v1", "entry", "{id}"]
		lastPart := parts[len(parts)-1]
		if lastPart != "" && lastPart != "entries" && lastPart != "rate" && lastPart != "create" && lastPart != "update" && lastPart != "delete" {
			return lastPart
		}
	}

	return ""
}

// getUserFromContext 从请求上下文中获取用户信息
func getUserFromContext(ctx context.Context) *model.User {
	return middleware.GetUserFromContext(ctx)
}

// setUserInContext 将用户信息设置到请求上下文中
func setUserInContext(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, middleware.UserKey, user)
}

// generateUUID 生成 UUID v4 格式的唯一标识符
// 使用 crypto/rand 保证随机性
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// parseInt 安全地将字符串解析为整数
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}
