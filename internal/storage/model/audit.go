package model

import (
	"encoding/json"
	"time"
)

// AuditLog 审计日志
type AuditLog struct {
	ID        string `json:"id"`        // 日志唯一 ID
	Timestamp int64  `json:"timestamp"` // 操作时间戳（毫秒）

	// 操作者信息
	OperatorPubkey string `json:"operator_pubkey"` // 操作者公钥
	OperatorLevel  int32  `json:"operator_level"`  // 操作者等级
	OperatorIP     string `json:"operator_ip"`     // 操作者 IP
	UserAgent      string `json:"user_agent"`      // User-Agent

	// 操作信息
	Method     string `json:"method"`      // HTTP 方法（GET/POST/PUT/DELETE）
	Path       string `json:"path"`        // 请求路径
	ActionType string `json:"action_type"` // 操作类型（如 entry.create, user.ban）
	TargetID   string `json:"target_id"`   // 目标对象 ID
	TargetType string `json:"target_type"` // 目标类型（entry/user/category等）

	// 请求/响应
	RequestBody  string `json:"request_body"`  // 请求体（脱敏后）
	ResponseCode int    `json:"response_code"` // HTTP 响应码
	ResponseBody string `json:"response_body"` // 响应体（截断）

	// 结果
	Success      bool   `json:"success"`       // 操作是否成功
	ErrorMessage string `json:"error_message"` // 错误信息（失败时）
}

// AuditFilter 查询过滤器
type AuditFilter struct {
	OperatorPubkey string   // 按操作者筛选
	ActionTypes    []string // 按操作类型筛选（多个）
	TargetID       string   // 按目标 ID 筛选
	Success        *bool    // 按成功/失败筛选
	StartTime      int64    // 开始时间戳（毫秒）
	EndTime        int64    // 结束时间戳（毫秒）
	Limit          int      // 返回数量
	Offset         int      // 偏移量
}

// AuditStats 审计统计
type AuditStats struct {
	TotalLogs    int64            `json:"total_logs"`    // 总日志数
	TodayLogs    int64            `json:"today_logs"`    // 今日日志数
	ActionCounts map[string]int64 `json:"action_counts"` // 各操作类型数量
	FailedCount  int64            `json:"failed_count"`  // 失败操作数
}

// ToJSON 将审计日志序列化为 JSON 字节数组
func (l *AuditLog) ToJSON() ([]byte, error) {
	return json.Marshal(l)
}

// FromJSON 从 JSON 字节数组反序列化为审计日志
func (l *AuditLog) FromJSON(data []byte) error {
	return json.Unmarshal(data, l)
}

// NewAuditLog 创建审计日志
func NewAuditLog() *AuditLog {
	return &AuditLog{
		ID:        "audit_" + generateID(),
		Timestamp: time.Now().UnixMilli(),
	}
}
