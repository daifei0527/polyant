// Package storage 提供基于内存的存储实现。
// 适用于开发和测试环境。
//
// Deprecated: 生产环境应使用 NewPersistentStore 创建持久化存储。
// 内存存储不会持久化数据，重启后数据丢失。
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/daifei0527/agentwiki/internal/storage/index"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// MemoryEntryStore 基于内存的知识条目存储实现
type MemoryEntryStore struct {
	mu      sync.RWMutex
	entries map[string]*model.KnowledgeEntry
}

// NewMemoryEntryStore 创建内存条目存储实例
func NewMemoryEntryStore() *MemoryEntryStore {
	return &MemoryEntryStore{
		entries: make(map[string]*model.KnowledgeEntry),
	}
}

// Create 创建新的知识条目
func (s *MemoryEntryStore) Create(ctx context.Context, entry *model.KnowledgeEntry) (*model.KnowledgeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[entry.ID]; exists {
		return nil, fmt.Errorf("entry already exists")
	}

	created := *entry
	if created.Tags == nil {
		created.Tags = []string{}
	}
	if created.JSONData == nil {
		created.JSONData = []map[string]interface{}{}
	}
	s.entries[entry.ID] = &created
	return &created, nil
}

// Get 根据ID获取知识条目
func (s *MemoryEntryStore) Get(ctx context.Context, id string) (*model.KnowledgeEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[id]
	if !exists {
		return nil, fmt.Errorf("entry not found")
	}
	cp := *entry
	return &cp, nil
}

// Update 更新知识条目
func (s *MemoryEntryStore) Update(ctx context.Context, entry *model.KnowledgeEntry) (*model.KnowledgeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[entry.ID]; !exists {
		return nil, fmt.Errorf("entry not found")
	}

	updated := *entry
	s.entries[entry.ID] = &updated
	return &updated, nil
}

// Delete 软删除知识条目（标记为archived）
func (s *MemoryEntryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[id]
	if !exists {
		return fmt.Errorf("entry not found")
	}

	entry.Status = model.EntryStatusArchived
	return nil
}

// List 列出条目，支持过滤和分页
func (s *MemoryEntryStore) List(ctx context.Context, filter EntryFilter) ([]*model.KnowledgeEntry, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*model.KnowledgeEntry
	for _, entry := range s.entries {
		// 状态过滤
		if filter.Status != "" && entry.Status != filter.Status {
			continue
		}
		// 分类过滤
		if filter.Category != "" && entry.Category != filter.Category {
			continue
		}
		// 创建者过滤
		if filter.CreatedBy != "" && entry.CreatedBy != filter.CreatedBy {
			continue
		}
		// 标签过滤
		if len(filter.Tags) > 0 {
			matched := false
			for _, tag := range filter.Tags {
				for _, entryTag := range entry.Tags {
					if tag == entryTag {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		cp := *entry
		results = append(results, &cp)
	}

	// 排序
	sortEntries(results, filter.OrderBy, filter.OrderDir)

	total := len(results)

	// 分页
	if filter.Offset > 0 {
		if filter.Offset >= len(results) {
			return nil, total, nil
		}
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, total, nil
}

// Count 获取条目总数
func (s *MemoryEntryStore) Count(ctx context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, entry := range s.entries {
		if entry.Status == model.EntryStatusPublished {
			count++
		}
	}
	return count, nil
}

// sortEntries 对条目列表进行排序
func sortEntries(entries []*model.KnowledgeEntry, orderBy, orderDir string) {
	if orderBy == "" {
		orderBy = "updated_at"
	}
	if orderDir == "" {
		orderDir = "desc"
	}

	sort.SliceStable(entries, func(i, j int) bool {
		var less bool
		switch orderBy {
		case "score":
			less = entries[i].Score < entries[j].Score
		case "created_at":
			less = entries[i].CreatedAt < entries[j].CreatedAt
		default:
			less = entries[i].UpdatedAt < entries[j].UpdatedAt
		}
		if orderDir == "desc" {
			return !less
		}
		return less
	})
}

// MemoryUserStore 基于内存的用户存储实现
type MemoryUserStore struct {
	mu    sync.RWMutex
	users map[string]*model.User // key: public_key
}

// NewMemoryUserStore 创建内存用户存储实例
func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		users: make(map[string]*model.User),
	}
}

// Create 创建新用户
func (s *MemoryUserStore) Create(ctx context.Context, user *model.User) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 计算公钥哈希作为存储key
	pubKeyBytes, err := base64.StdEncoding.DecodeString(user.PublicKey)
	var pubKeyHash string
	if err == nil {
		hash := sha256.Sum256(pubKeyBytes)
		pubKeyHash = hex.EncodeToString(hash[:])
	} else {
		pubKeyHash = user.PublicKey // fallback
	}

	if _, exists := s.users[pubKeyHash]; exists {
		return nil, fmt.Errorf("user already exists")
	}

	created := *user
	s.users[pubKeyHash] = &created
	return &created, nil
}

// Get 根据公钥获取用户
func (s *MemoryUserStore) Get(ctx context.Context, pubkeyHash string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[pubkeyHash]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	cp := *user
	return &cp, nil
}

// GetByEmail 根据邮箱获取用户
func (s *MemoryUserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.Email == email {
			cp := *user
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

// Update 更新用户信息
func (s *MemoryUserStore) Update(ctx context.Context, user *model.User) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 计算公钥哈希作为存储key
	pubKeyBytes, err := base64.StdEncoding.DecodeString(user.PublicKey)
	var pubKeyHash string
	if err == nil {
		hash := sha256.Sum256(pubKeyBytes)
		pubKeyHash = hex.EncodeToString(hash[:])
	} else {
		pubKeyHash = user.PublicKey
	}

	if _, exists := s.users[pubKeyHash]; !exists {
		return nil, fmt.Errorf("user not found")
	}

	updated := *user
	s.users[pubKeyHash] = &updated
	return &updated, nil
}

// List 列出用户
func (s *MemoryUserStore) List(ctx context.Context, filter UserFilter) ([]*model.User, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*model.User
	for _, user := range s.users {
		if filter.Status != "" && user.Status != filter.Status {
			continue
		}
		if filter.Level != 0 && user.UserLevel != filter.Level {
			continue
		}
		cp := *user
		results = append(results, &cp)
	}

	total := len(results)
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, total, nil
}

// MemoryRatingStore 基于内存的评分存储实现
type MemoryRatingStore struct {
	mu      sync.RWMutex
	ratings map[string]*model.Rating // key: rating id
}

// NewMemoryRatingStore 创建内存评分存储实例
func NewMemoryRatingStore() *MemoryRatingStore {
	return &MemoryRatingStore{
		ratings: make(map[string]*model.Rating),
	}
}

// Create 创建评分记录
func (s *MemoryRatingStore) Create(ctx context.Context, rating *model.Rating) (*model.Rating, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created := *rating
	s.ratings[rating.ID] = &created
	return &created, nil
}

// Get 根据ID获取评分
func (s *MemoryRatingStore) Get(ctx context.Context, id string) (*model.Rating, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rating, exists := s.ratings[id]
	if !exists {
		return nil, fmt.Errorf("rating not found")
	}
	cp := *rating
	return &cp, nil
}

// ListByEntry 获取条目的所有评分
func (s *MemoryRatingStore) ListByEntry(ctx context.Context, entryID string) ([]*model.Rating, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*model.Rating
	for _, rating := range s.ratings {
		if rating.EntryId == entryID {
			cp := *rating
			results = append(results, &cp)
		}
	}
	return results, nil
}

// GetByRater 获取评分者对某条目的评分（检查重复评分）
func (s *MemoryRatingStore) GetByRater(ctx context.Context, entryID, raterPubkeyHash string) (*model.Rating, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, rating := range s.ratings {
		if rating.EntryId == entryID && rating.RaterPubkey == raterPubkeyHash {
			cp := *rating
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("rating not found")
}

// MemoryCategoryStore 基于内存的分类存储实现
type MemoryCategoryStore struct {
	mu         sync.RWMutex
	categories map[string]*model.Category // key: path
}

// NewMemoryCategoryStore 创建内存分类存储实例
func NewMemoryCategoryStore() *MemoryCategoryStore {
	return &MemoryCategoryStore{
		categories: make(map[string]*model.Category),
	}
}

// Create 创建分类
func (s *MemoryCategoryStore) Create(ctx context.Context, cat *model.Category) (*model.Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.categories[cat.Path]; exists {
		return nil, fmt.Errorf("category already exists")
	}

	created := *cat
	s.categories[cat.Path] = &created
	return &created, nil
}

// Get 根据路径获取分类
func (s *MemoryCategoryStore) Get(ctx context.Context, path string) (*model.Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cat, exists := s.categories[path]
	if !exists {
		return nil, fmt.Errorf("category not found")
	}
	cp := *cat
	return &cp, nil
}

// List 列出分类
func (s *MemoryCategoryStore) List(ctx context.Context, parentPath string) ([]*model.Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*model.Category
	for _, cat := range s.categories {
		if parentPath == "" || cat.ParentId == parentPath {
			cp := *cat
			results = append(results, &cp)
		}
	}
	return results, nil
}

// ListAll 列出所有分类
func (s *MemoryCategoryStore) ListAll(ctx context.Context) ([]*model.Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*model.Category, 0, len(s.categories))
	for _, cat := range s.categories {
		cp := *cat
		results = append(results, &cp)
	}
	return results, nil
}

// MemorySearchEngine 基于内存的搜索引擎实现
// 使用简单的关键词匹配，适用于开发和测试
type MemorySearchEngine struct {
	mu      sync.RWMutex
	entries map[string]*model.KnowledgeEntry
}

// NewMemorySearchEngine 创建内存搜索引擎实例
func NewMemorySearchEngine() *MemorySearchEngine {
	return &MemorySearchEngine{
		entries: make(map[string]*model.KnowledgeEntry),
	}
}

// IndexEntry 将条目加入搜索索引
func (e *MemorySearchEngine) IndexEntry(entry *model.KnowledgeEntry) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cp := *entry
	e.entries[entry.ID] = &cp
	return nil
}

// UpdateIndex 更新条目索引
func (e *MemorySearchEngine) UpdateIndex(entry *model.KnowledgeEntry) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cp := *entry
	e.entries[entry.ID] = &cp
	return nil
}

// DeleteIndex 从索引中删除条目
func (e *MemorySearchEngine) DeleteIndex(entryID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.entries, entryID)
	return nil
}

// Search 执行全文搜索
// 简单实现：在标题和内容中搜索关键词，支持分类过滤
func (e *MemorySearchEngine) Search(ctx context.Context, query index.SearchQuery) (*index.SearchResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	keyword := strings.ToLower(query.Keyword)
	var results []*model.KnowledgeEntry

	for _, entry := range e.entries {
		// 只搜索活跃状态的条目
		if entry.Status != model.EntryStatusPublished {
			continue
		}

		// 分类过滤
		if len(query.Categories) > 0 {
			matched := false
			for _, cat := range query.Categories {
				if entry.Category == cat || strings.HasPrefix(entry.Category, cat+"/") {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 标签过滤
		if len(query.Tags) > 0 {
			matched := false
			for _, tag := range query.Tags {
				for _, entryTag := range entry.Tags {
					if strings.EqualFold(tag, entryTag) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 最低评分过滤
		if query.MinScore > 0 && entry.Score < query.MinScore {
			continue
		}

		// 关键词匹配（在标题和内容中搜索）
		titleLower := strings.ToLower(entry.Title)
		contentLower := strings.ToLower(entry.Content)
		if strings.Contains(titleLower, keyword) || strings.Contains(contentLower, keyword) {
			cp := *entry
			results = append(results, &cp)
		}
	}

	// 按评分降序排序
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	total := len(results)

	// 分页
	if query.Offset > 0 {
		if query.Offset >= len(results) {
			return &index.SearchResult{
				TotalCount: total,
				HasMore:    false,
				Entries:    nil,
			}, nil
		}
		results = results[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	hasMore := total > (query.Offset + len(results))

	return &index.SearchResult{
		TotalCount: total,
		HasMore:    hasMore,
		Entries:    results,
	}, nil
}

// Close 关闭搜索引擎
func (e *MemorySearchEngine) Close() error {
	return nil
}

// computeContentHash 计算内容哈希
func computeContentHash(title, content, category string) string {
	h := sha256.New()
	h.Write([]byte(title))
	h.Write([]byte(content))
	h.Write([]byte(category))
	return hex.EncodeToString(h.Sum(nil))
}

// MemoryBacklinkIndex 基于内存的反向链接索引实现
type MemoryBacklinkIndex struct {
	mu        sync.RWMutex
	outlinks  map[string]map[string]bool // 正向链接: source entry ID -> set of linked entry IDs
	backlinks map[string]map[string]bool // 反向链接: target entry ID -> set of source entry IDs
}

// NewMemoryBacklinkIndex 创建内存反向链接索引实例
func NewMemoryBacklinkIndex() *MemoryBacklinkIndex {
	return &MemoryBacklinkIndex{
		outlinks:  make(map[string]map[string]bool),
		backlinks: make(map[string]map[string]bool),
	}
}

// UpdateIndex 更新条目链接索引：先删除旧索引，再添加新链接
func (idx *MemoryBacklinkIndex) UpdateIndex(entryID string, linkedEntryIDs []string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 第一步：移除旧的索引关系
	oldLinkedIDs, exists := idx.outlinks[entryID]
	if exists {
		for linkedID := range oldLinkedIDs {
			// 从反向链接中移除当前entryID
			if backlinks, ok := idx.backlinks[linkedID]; ok {
				delete(backlinks, entryID)
				if len(backlinks) == 0 {
					delete(idx.backlinks, linkedID)
				}
			}
		}
	}

	// 第二步：添加新的链接关系
	newLinkedSet := make(map[string]bool)
	for _, linkedID := range linkedEntryIDs {
		if linkedID == entryID {
			continue // 避免自链接
		}
		newLinkedSet[linkedID] = true

		// 在反向链接中添加当前entryID
		if _, ok := idx.backlinks[linkedID]; !ok {
			idx.backlinks[linkedID] = make(map[string]bool)
		}
		idx.backlinks[linkedID][entryID] = true
	}

	if len(newLinkedSet) > 0 {
		idx.outlinks[entryID] = newLinkedSet
	} else {
		delete(idx.outlinks, entryID)
	}

	return nil
}

// DeleteIndex 删除条目索引，同时从被链接条目的反向链接中移除自身
func (idx *MemoryBacklinkIndex) DeleteIndex(entryID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 从被链接条目的反向链接中移除自身
	if oldLinkedIDs, exists := idx.outlinks[entryID]; exists {
		for linkedID := range oldLinkedIDs {
			if backlinks, ok := idx.backlinks[linkedID]; ok {
				delete(backlinks, entryID)
				if len(backlinks) == 0 {
					delete(idx.backlinks, linkedID)
				}
			}
		}
	}

	// 删除正向链接
	delete(idx.outlinks, entryID)

	// 如果其他页面链接到这个被删除的页面，不需要删除它们的正向链接
	// 因为删除源条目已经在上游处理了，这里只需要清理自身数据
	return nil
}

// GetBacklinks 获取指向目标条目的所有反向链接条目ID
func (idx *MemoryBacklinkIndex) GetBacklinks(targetEntryID string) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	backlinks, exists := idx.backlinks[targetEntryID]
	if !exists || len(backlinks) == 0 {
		return []string{}, nil
	}

	result := make([]string, 0, len(backlinks))
	for id := range backlinks {
		result = append(result, id)
	}
	return result, nil
}

// GetOutlinks 获取当前条目链接出去的所有正向链接条目ID
func (idx *MemoryBacklinkIndex) GetOutlinks(sourceEntryID string) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	outlinks, exists := idx.outlinks[sourceEntryID]
	if !exists || len(outlinks) == 0 {
		return []string{}, nil
	}

	result := make([]string, 0, len(outlinks))
	for id := range outlinks {
		result = append(result, id)
	}
	return result, nil
}
