// Package storage 定义了 AgentWiki 存储层的接口抽象和内存实现。
// 各 handler 通过这些接口与存储层交互，便于测试和替换底层实现。
package storage

import (
	"context"
	"fmt"

	"github.com/daifei0527/agentwiki/internal/storage/index"
	"github.com/daifei0527/agentwiki/internal/storage/kv"
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
	Entry    EntryStore
	User     UserStore
	Rating   RatingStore
	Category CategoryStore
	Search   index.SearchEngine
	Backlink BacklinkIndex
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

// StoreConfig 存储配置
type StoreConfig struct {
	// KV 存储类型: memory, badger, pebble
	KVType string
	// KV 存储路径
	KVPath string
	// 搜索引擎类型: memory, bleve
	SearchType string
	// 搜索引擎索引路径
	SearchPath string
}

// NewPersistentStore 创建持久化存储实例
func NewPersistentStore(cfg *StoreConfig) (*Store, error) {
	var kvStore kv.Store
	var err error

	// 创建 KV 存储
	switch cfg.KVType {
	case "pebble":
		kvStore, err = kv.NewPebbleStore(cfg.KVPath)
	case "badger":
		kvStore, err = kv.NewBadgerStore(cfg.KVPath)
	default:
		kvStore, err = kv.NewPebbleStore(cfg.KVPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create kv store: %w", err)
	}

	// 创建搜索引擎
	var searchEngine index.SearchEngine
	switch cfg.SearchType {
	case "bleve":
		searchEngine, err = index.NewBleveEngine(cfg.SearchPath)
	default:
		searchEngine, err = index.NewBleveEngine(cfg.SearchPath)
	}
	if err != nil {
		kvStore.Close()
		return nil, fmt.Errorf("failed to create search engine: %w", err)
	}

	// 使用适配器组装存储
	return &Store{
		Entry:    NewBadgerEntryStore(kvStore),
		User:     NewBadgerUserStore(kvStore),
		Rating:   NewBadgerRatingStore(kvStore),
		Category: NewBadgerCategoryStore(kvStore),
		Search:   searchEngine,
		Backlink: NewMemoryBacklinkIndex(), // 反向链接仍使用内存实现
	}, nil
}

// Close 关闭存储
func (s *Store) Close() error {
	if s.Search != nil {
		s.Search.Close()
	}
	return nil
}
