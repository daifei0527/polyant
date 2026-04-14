// Package audit 提供审计日志服务
package audit

import (
	"context"
	"regexp"
	"strings"

	"github.com/daifei0527/agentwiki/internal/storage/kv"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// 敏感字段脱敏规则
var sensitiveFields = []string{
	"password", "passwd", "pwd",
	"private_key", "privateKey", "private-key",
	"secret", "token", "api_key", "apiKey",
	"code", "verification_code",
}

// 脱敏正则
var sensitivePatterns []*regexp.Regexp

// 敏感操作路径映射
var sensitiveOps = map[string]map[string]string{
	"POST": {
		"/api/v1/user/register":     "user.register",
		"/api/v1/user/verify-email": "user.verify_email",
		"/api/v1/user/update":       "user.update",
		"/api/v1/entry/create":      "entry.create",
		"/api/v1/categories/create": "category.create",
		"/api/v1/elections":         "election.create",
		"/api/v1/batch/create":      "batch.create",
		"/api/v1/batch/update":      "batch.update",
		"/api/v1/batch/delete":      "batch.delete",
		"/api/v1/admin/import":      "admin.import",
	},
	"GET": {
		"/api/v1/admin/export": "admin.export",
	},
}

func init() {
	for _, field := range sensitiveFields {
		// 匹配 "field": "value" 或 "field":"value"
		pattern := regexp.MustCompile(`(?i)"` + field + `"\s*:\s*"[^"]*"`)
		sensitivePatterns = append(sensitivePatterns, pattern)
	}
}

// Service 审计服务
type Service struct {
	store kv.AuditStore
}

// NewService 创建审计服务
func NewService(store kv.AuditStore) *Service {
	return &Service{store: store}
}

// Log 记录审计日志
func (s *Service) Log(ctx context.Context, log *model.AuditLog) error {
	// 脱敏请求体
	log.RequestBody = MaskSensitiveFields(log.RequestBody)
	log.ResponseBody = TruncateString(log.ResponseBody, 4096) // 4KB
	log.RequestBody = TruncateString(log.RequestBody, 16384)  // 16KB

	return s.store.Create(ctx, log)
}

// List 查询审计日志
func (s *Service) List(ctx context.Context, filter model.AuditFilter) ([]*model.AuditLog, int64, error) {
	return s.store.List(ctx, filter)
}

// GetStats 获取审计统计
func (s *Service) GetStats(ctx context.Context) (*model.AuditStats, error) {
	return s.store.GetStats(ctx)
}

// DeleteBefore 删除指定时间之前的日志
func (s *Service) DeleteBefore(ctx context.Context, timestamp int64) (int64, error) {
	return s.store.DeleteBefore(ctx, timestamp)
}

// MaskSensitiveFields 脱敏敏感字段
func MaskSensitiveFields(jsonStr string) string {
	if jsonStr == "" {
		return ""
	}

	result := jsonStr
	for _, pattern := range sensitivePatterns {
		// 替换为 "***"
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// 保留字段名，替换值为 ***
			parts := strings.SplitN(match, ":", 2)
			if len(parts) == 2 {
				return parts[0] + `: "***"`
			}
			return match
		})
	}
	return result
}

// TruncateString 截断字符串
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[TRUNCATED]"
}

// GetActionType 获取操作类型
func GetActionType(method, path string) string {
	// 精确匹配
	if methodOps, ok := sensitiveOps[method]; ok {
		for pattern, action := range methodOps {
			if path == pattern {
				return action
			}
		}
	}

	// 前缀匹配
	if method == "POST" {
		if strings.HasPrefix(path, "/api/v1/entry/update/") {
			return "entry.update"
		}
		if strings.HasPrefix(path, "/api/v1/entry/delete/") {
			return "entry.delete"
		}
		if strings.HasPrefix(path, "/api/v1/entry/rate/") {
			return "entry.rate"
		}
		if strings.HasPrefix(path, "/api/v1/admin/users/") && strings.HasSuffix(path, "/ban") {
			return "admin.user_ban"
		}
		if strings.HasPrefix(path, "/api/v1/admin/users/") && strings.HasSuffix(path, "/unban") {
			return "admin.user_unban"
		}
		if strings.HasPrefix(path, "/api/v1/elections/") && strings.HasSuffix(path, "/vote") {
			return "election.vote"
		}
		if strings.HasPrefix(path, "/api/v1/elections/") && strings.HasSuffix(path, "/close") {
			return "election.close"
		}
	}
	if method == "PUT" {
		if strings.HasPrefix(path, "/api/v1/admin/users/") && strings.HasSuffix(path, "/level") {
			return "admin.user_level"
		}
	}

	return ""
}

// IsSensitiveOperation 检查是否为敏感操作
func IsSensitiveOperation(method, path string) bool {
	return GetActionType(method, path) != ""
}

// ExtractTargetID 从路径提取目标 ID
func ExtractTargetID(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 5 {
		// /api/v1/entry/update/entry-id
		// /api/v1/admin/users/user-pk/ban
		switch {
		case strings.HasPrefix(path, "/api/v1/entry/"):
			return parts[4] // entry-id
		case strings.HasPrefix(path, "/api/v1/admin/users/"):
			if len(parts) >= 6 {
				return parts[5] // user-pk
			}
		case strings.HasPrefix(path, "/api/v1/elections/"):
			return parts[4] // election-id
		}
	}
	return ""
}

// GetTargetType 从操作类型获取目标类型
func GetTargetType(actionType string) string {
	if strings.HasPrefix(actionType, "entry.") {
		return "entry"
	}
	if strings.HasPrefix(actionType, "user.") {
		return "user"
	}
	if strings.HasPrefix(actionType, "category.") {
		return "category"
	}
	if strings.HasPrefix(actionType, "admin.") {
		return "admin"
	}
	if strings.HasPrefix(actionType, "election.") {
		return "election"
	}
	if strings.HasPrefix(actionType, "batch.") {
		return "batch"
	}
	return ""
}
