// Package index 提供全文搜索功能
package index

import (
	"context"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// SearchQuery 搜索查询参数
type SearchQuery struct {
	Keyword    string
	Categories []string
	Tags       []string
	Limit      int
	Offset     int
	MinScore   float64
}

// SearchResult 搜索结果
type SearchResult struct {
	TotalCount int                     `json:"total_count"`
	HasMore    bool                    `json:"has_more"`
	Entries    []*model.KnowledgeEntry `json:"entries"`
}

// SearchEngine 搜索引擎接口
// 定义了搜索引擎需要实现的方法
type SearchEngine interface {
	// IndexEntry 将条目加入全文索引
	IndexEntry(entry *model.KnowledgeEntry) error
	// UpdateIndex 更新条目索引
	UpdateIndex(entry *model.KnowledgeEntry) error
	// DeleteIndex 从索引中删除条目
	DeleteIndex(entryID string) error
	// Search 执行全文搜索
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)
	// Close 关闭搜索引擎
	Close() error
}
