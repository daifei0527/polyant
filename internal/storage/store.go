// Package storage 定义了 AgentWiki 存储层的接口抽象和内存实现。
// 各 handler 通过这些接口与存储层交互，便于测试和替换底层实现。
package storage

import (
	"context"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// EntryStore 知识条目存储接口
type EntryStore interface {
	// Create 创建新的知识条目
	Create(ctx context.Context, entry *model.KnowledgeEntry) (*model.KnowledgeEntry, error)
	// Get 根据ID获取知识条目
	Get(ctx context.Context, id string) (*model.KnowledgeEntry, error)
	// Update 更新知识条目
	Update(ctx context.Context, entry *model.KnowledgeEntry) (*model.KnowledgeEntry, error)
	// Delete 软删除知识条目（标记为archived）
	Delete(ctx context.Context, id string) error
	// List 列出条目，支持过滤和分页
	List(ctx context.Context, filter EntryFilter) ([]*model.KnowledgeEntry, int, error)
	// Count 获取条目总数
	Count(ctx context.Context) (int64, error)
}

// EntryFilter 条目查询过滤器
type EntryFilter struct {
	Category  string
	Tags      []string
	Status    string
	CreatedBy string
	Limit     int
	Offset    int
	OrderBy   string // "score", "updated_at", "created_at"
	OrderDir  string // "asc", "desc"
}

// UserStore 用户存储接口
type UserStore interface {
	// Create 创建新用户
	Create(ctx context.Context, user *model.User) (*model.User, error)
	// Get 根据公钥哈希获取用户
	Get(ctx context.Context, pubkeyHash string) (*model.User, error)
	// GetByEmail 根据邮箱获取用户
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	// Update 更新用户信息
	Update(ctx context.Context, user *model.User) (*model.User, error)
	// List 列出用户
	List(ctx context.Context, filter UserFilter) ([]*model.User, int, error)
}

// UserFilter 用户查询过滤器
type UserFilter struct {
	Level  int32
	Status string
	Limit  int
	Offset int
}

// RatingStore 评分存储接口
type RatingStore interface {
	// Create 创建评分记录
	Create(ctx context.Context, rating *model.Rating) (*model.Rating, error)
	// Get 根据ID获取评分
	Get(ctx context.Context, id string) (*model.Rating, error)
	// ListByEntry 获取条目的所有评分
	ListByEntry(ctx context.Context, entryID string) ([]*model.Rating, error)
	// GetByRater 获取评分者对某条目的评分（检查重复评分）
	GetByRater(ctx context.Context, entryID, raterPubkeyHash string) (*model.Rating, error)
}

// CategoryStore 分类存储接口
type CategoryStore interface {
	// Create 创建分类
	Create(ctx context.Context, cat *model.Category) (*model.Category, error)
	// Get 根据路径获取分类
	Get(ctx context.Context, path string) (*model.Category, error)
	// List 列出分类
	List(ctx context.Context, parentPath string) ([]*model.Category, error)
	// ListAll 列出所有分类
	ListAll(ctx context.Context) ([]*model.Category, error)
}

// SearchEngine 搜索引擎接口
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

// BacklinkIndex 反向链接索引接口
// 维护条目之间的链接关系，支持查询哪些页面链接到了当前页面
type BacklinkIndex interface {
	// UpdateIndex 更新条目链接索引：先删除旧索引，再添加新链接
	UpdateIndex(entryID string, linkedEntryIDs []string) error
	// DeleteIndex 删除条目索引，同时从被链接条目的反向链接中移除自身
	DeleteIndex(entryID string) error
	// GetBacklinks 获取指向目标条目的所有反向链接条目ID
	GetBacklinks(targetEntryID string) ([]string, error)
	// GetOutlinks 获取当前条目链接出去的所有正向链接条目ID
	GetOutlinks(sourceEntryID string) ([]string, error)
}

// Store 存储接口集合
type Store struct {
	Entry        EntryStore
	User         UserStore
	Rating       RatingStore
	Category     CategoryStore
	Search       SearchEngine
	Backlink     BacklinkIndex
}

// NewMemoryStore 创建内存存储实例
func NewMemoryStore() (*Store, error) {
	entryStore := NewMemoryEntryStore()
	userStore := NewMemoryUserStore()
	ratingStore := NewMemoryRatingStore()
	categoryStore := NewMemoryCategoryStore()
	searchEngine := NewMemorySearchEngine()
	backlinkIndex := NewMemoryBacklinkIndex()

	return &Store{
		Entry:    entryStore,
		User:     userStore,
		Rating:   ratingStore,
		Category: categoryStore,
		Search:   searchEngine,
		Backlink: backlinkIndex,
	}, nil
}
