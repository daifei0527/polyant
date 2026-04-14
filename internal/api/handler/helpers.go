package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/storage/model"
	awerrors "github.com/daifei0527/polyant/pkg/errors"
	"github.com/daifei0527/polyant/pkg/i18n"
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

// writeSuccess 写入成功响应（带多语言支持）
func writeSuccess(w http.ResponseWriter, r *http.Request, data interface{}) {
	lang := i18n.GetLangFromRequest(r)
	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: i18n.Tc(lang, "common.success"),
		Data:    data,
	})
}

// writeSuccessWithCode 写入成功响应（指定消息码）
func writeSuccessWithCode(w http.ResponseWriter, r *http.Request, code string, data interface{}) {
	lang := i18n.GetLangFromRequest(r)
	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: i18n.Tc(lang, code),
		Data:    data,
	})
}

// writeErrorI18n 写入错误响应（带多语言支持）
func writeErrorI18n(w http.ResponseWriter, r *http.Request, err *awerrors.AWError) {
	lang := i18n.GetLangFromRequest(r)

	message := err.Message
	if err.I18nCode != "" {
		if translated := i18n.Tc(lang, err.I18nCode); translated != err.I18nCode {
			message = translated
		}
	}

	status := err.HTTPStatus
	if status == 0 {
		status = http.StatusInternalServerError
	}

	writeJSON(w, status, &APIResponse{
		Code:    err.Code,
		Message: message,
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
		excluded := []string{"entries", "rate", "create", "update", "delete", "backlinks", "outlinks"}
		isExcluded := false
		for _, ex := range excluded {
			if lastPart == ex {
				isExcluded = true
				break
			}
		}
		if !isExcluded && lastPart != "" {
			return lastPart
		}

		// If last part is excluded (like "backlinks"), get the second to last
		// /api/v1/entry/{id}/backlinks -> parts = ["", "api", "v1", "entry", "{id}", "backlinks"]
		if isExcluded && len(parts) >= 6 {
			idPart := parts[len(parts)-2]
			if idPart != "" && idPart != "entry" {
				return idPart
			}
		}
	}

	// 策略3: 对于 /api/v1/categories/{path}/entries 模式
	// 提取 categories 和 entries 之间的部分
	if name == "path" && len(parts) >= 5 {
		// /api/v1/categories/{path}/entries
		// 查找 categories 和 entries 的位置
		categoriesIdx := -1
		entriesIdx := -1
		for i, part := range parts {
			if part == "categories" {
				categoriesIdx = i
			}
			if part == "entries" {
				entriesIdx = i
			}
		}
		// 如果找到了 categories 和 entries，返回它们之间的部分
		if categoriesIdx >= 0 && entriesIdx > categoriesIdx+1 {
			// 返回 categories 和 entries 之间的路径部分
			// 对于 /api/v1/categories/programming/entries，返回 "programming"
			// 对于 /api/v1/categories/tech/ai/entries，返回 "tech/ai"
			pathParts := parts[categoriesIdx+1 : entriesIdx]
			return strings.Join(pathParts, "/")
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
