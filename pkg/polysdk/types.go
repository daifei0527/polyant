// pkg/polysdk/types.go
package polysdk

import "time"

// Entry 知识条目
type Entry struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Category   string    `json:"category"`
	Tags       []string  `json:"tags,omitempty"`
	Score      float64   `json:"score"`
	ScoreCount int       `json:"score_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	CreatedBy  string    `json:"created_by"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Entries    []Entry `json:"items"`
	TotalCount int     `json:"total_count"`
}

// CreateEntryRequest 创建条目请求
type CreateEntryRequest struct {
	Title            string   `json:"title"`
	Content          string   `json:"content"`
	Category         string   `json:"category"`
	Tags             []string `json:"tags,omitempty"`
	CreatorSignature string   `json:"creator_signature,omitempty"` // 创建者对 (title,content,category) 的 Ed25519 签名(base64)，由 Client.CreateEntry 自动填充
}

// UpdateEntryRequest 更新条目请求
type UpdateEntryRequest struct {
	Title            string   `json:"title,omitempty"`
	Content          string   `json:"content,omitempty"`
	Category         string   `json:"category,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	CreatorSignature string   `json:"creator_signature,omitempty"` // 更新后完整内容的签名(base64)，由 Client.UpdateEntry 自动填充
}

// RatingRequest 评分请求
type RatingRequest struct {
	Score   float64 `json:"score"`
	Comment string  `json:"comment,omitempty"`
}

// Category 分类信息
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
}

// APIResponse 标准 API 响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// APIError API 错误响应
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
