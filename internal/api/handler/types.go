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
	Title            string                   `json:"title"`
	Content          string                   `json:"content"`
	JsonData         []map[string]interface{} `json:"json_data,omitempty"`
	Category         string                   `json:"category"`
	Tags             []string                 `json:"tags,omitempty"`
	License          string                   `json:"license,omitempty"`
	SourceRef        string                   `json:"source_ref,omitempty"`
	CreatorSignature string                   `json:"creator_signature"` // 条目内容签名
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
	Code  string `json:"code"` // 验证码
}

// SendVerificationCodeRequest 发送验证码请求体
type SendVerificationCodeRequest struct {
	Email string `json:"email"`
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

// ==================== 批量操作类型 ====================

const MaxBatchSize = 100

// BatchCreateRequest 批量创建请求
type BatchCreateRequest struct {
	Entries []BatchEntry `json:"entries"`
	Options BatchOptions `json:"options,omitempty"`
}

// BatchUpdateRequest 批量更新请求
type BatchUpdateRequest struct {
	Entries []BatchUpdateEntry `json:"entries"`
}

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []string `json:"ids"`
}

// BatchEntry 批量创建条目
type BatchEntry struct {
	Title     string                   `json:"title"`
	Content   string                   `json:"content"`
	JsonData  []map[string]interface{} `json:"json_data,omitempty"`
	Category  string                   `json:"category"`
	Tags      []string                 `json:"tags,omitempty"`
	License   string                   `json:"license,omitempty"`
	SourceRef string                   `json:"source_ref,omitempty"`
}

// BatchUpdateEntry 批量更新条目
type BatchUpdateEntry struct {
	ID       string                   `json:"id"`
	Title    *string                  `json:"title,omitempty"`
	Content  *string                  `json:"content,omitempty"`
	JsonData []map[string]interface{} `json:"json_data,omitempty"`
	Category *string                  `json:"category,omitempty"`
	Tags     *[]string                `json:"tags,omitempty"`
}

// BatchOptions 批量操作选项
type BatchOptions struct {
	SkipDuplicates bool `json:"skip_duplicates"`
	UpdateExisting bool `json:"update_existing"`
}

// BatchResponse 批量操作响应
type BatchResponse struct {
	Success bool          `json:"success"`
	Summary BatchSummary  `json:"summary"`
	Results []BatchResult `json:"results"`
	Errors  []BatchError  `json:"errors,omitempty"`
}

// BatchSummary 批量操作汇总
type BatchSummary struct {
	Total    int `json:"total"`
	Created  int `json:"created,omitempty"`
	Updated  int `json:"updated,omitempty"`
	Deleted  int `json:"deleted,omitempty"`
	Skipped  int `json:"skipped,omitempty"`
	Failed   int `json:"failed,omitempty"`
	NotFound int `json:"not_found,omitempty"`
}

// BatchResult 单个条目操作结果
type BatchResult struct {
	Index   int    `json:"index"`
	ID      string `json:"id"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Version int64  `json:"version,omitempty"`
}

// BatchError 批量操作错误
type BatchError struct {
	Index   int    `json:"index"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}
