// Package handler 定义了 AgentWiki API 的 HTTP 请求处理器。
// 包含知识条目、用户、分类、节点等各模块的 handler 实现。
package handler

// APIResponse 统一API响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PagedData 分页数据结构
type PagedData struct {
	TotalCount int         `json:"total_count"`
	HasMore    bool        `json:"has_more"`
	Items      interface{} `json:"items"`
}

// SearchParams 搜索请求参数
type SearchParams struct {
	Keyword  string   `json:"keyword"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Limit    int      `json:"limit"`
	Offset   int      `json:"offset"`
	MinScore float64  `json:"min_score"`
}

// CreateEntryRequest 创建条目请求体
type CreateEntryRequest struct {
	Title    string                   `json:"title"`
	Content  string                   `json:"content"`
	JsonData []map[string]interface{} `json:"json_data,omitempty"`
	Category string                   `json:"category"`
	Tags     []string                 `json:"tags,omitempty"`
	License  string                   `json:"license,omitempty"`
	SourceRef string                  `json:"source_ref,omitempty"`
}

// UpdateEntryRequest 更新条目请求体
type UpdateEntryRequest struct {
	Title    *string                  `json:"title,omitempty"`
	Content  *string                  `json:"content,omitempty"`
	JsonData []map[string]interface{} `json:"json_data,omitempty"`
	Category *string                  `json:"category,omitempty"`
	Tags     *[]string                `json:"tags,omitempty"`
}

// RegisterRequest 用户注册请求体
type RegisterRequest struct {
	AgentName string `json:"agent_name"`
	Email     string `json:"email,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
}

// VerifyEmailRequest 邮箱验证请求体
type VerifyEmailRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

// RateEntryRequest 评分请求体
type RateEntryRequest struct {
	Score   float64 `json:"score"`
	Comment string  `json:"comment,omitempty"`
}

// CreateEntryResponse 创建条目响应
type CreateEntryResponse struct {
	ID          string `json:"id"`
	Version     int64  `json:"version"`
	CreatedAt   int64  `json:"created_at"`
	ContentHash string `json:"content_hash"`
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	PublicKey     string `json:"public_key"`
	PublicKeyHash string `json:"public_key_hash"`
	AgentName     string `json:"agent_name"`
	UserLevel     int    `json:"user_level"`
}

// NodeStatusResponse 节点状态响应
type NodeStatusResponse struct {
	NodeID     string `json:"node_id"`
	NodeType   string `json:"node_type"`
	Version    string `json:"version"`
	EntryCount int64  `json:"entry_count"`
	Uptime     int64  `json:"uptime"`
	LastSync   int64  `json:"last_sync"`
}
