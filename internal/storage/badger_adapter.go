// Package storage 提供持久化存储适配器
// 将 kv 包的存储实现适配到 storage 接口
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/linkparser"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// BadgerEntryStore 适配 kv.EntryStore 到 storage.EntryStore 接口
type BadgerEntryStore struct {
	store *kv.EntryStore
}

// NewBadgerEntryStore 创建 BadgerDB 条目存储
func NewBadgerEntryStore(s kv.Store) *BadgerEntryStore {
	return &BadgerEntryStore{store: kv.NewEntryStore(s)}
}

func (s *BadgerEntryStore) Create(ctx context.Context, entry *model.KnowledgeEntry) (*model.KnowledgeEntry, error) {
	if err := s.store.CreateEntry(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *BadgerEntryStore) Get(ctx context.Context, id string) (*model.KnowledgeEntry, error) {
	return s.store.GetEntry(id)
}

func (s *BadgerEntryStore) Update(ctx context.Context, entry *model.KnowledgeEntry) (*model.KnowledgeEntry, error) {
	if err := s.store.UpdateEntry(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *BadgerEntryStore) Delete(ctx context.Context, id string) error {
	// 软删除：获取条目并标记为 archived
	entry, err := s.store.GetEntry(id)
	if err != nil {
		return err
	}
	entry.Status = model.EntryStatusArchived
	return s.store.UpdateEntry(entry)
}

func (s *BadgerEntryStore) List(ctx context.Context, filter EntryFilter) ([]*model.KnowledgeEntry, int, error) {
	entries, err := s.store.ListEntries(filter.Offset, filter.Limit)
	if err != nil {
		return nil, 0, err
	}

	// 应用过滤器
	var filtered []*model.KnowledgeEntry
	for _, e := range entries {
		if filter.Category != "" && e.Category != filter.Category {
			continue
		}
		if filter.Status != "" && e.Status != filter.Status {
			continue
		}
		if filter.CreatedBy != "" && e.CreatedBy != filter.CreatedBy {
			continue
		}
		filtered = append(filtered, e)
	}

	return filtered, len(filtered), nil
}

func (s *BadgerEntryStore) Count(ctx context.Context) (int64, error) {
	// 经维护的 published 计数器 O(1) 取值（启动重建 + Create/Update/Delete 增量维护；
	// 计数器缺失时 PublishedCount 回退扫描）。取代原先 ListEntries(0,1000000) 全量反序列化。
	return s.store.PublishedCount()
}

// BadgerUserStore 适配 kv.UserStore 到 storage.UserStore 接口
type BadgerUserStore struct {
	store *kv.UserStore
	kv    kv.Store // 维护 hash→pubkey 索引，让按公钥哈希查找成为 O(1)
}

// NewBadgerUserStore 创建 BadgerDB 用户存储
func NewBadgerUserStore(s kv.Store) *BadgerUserStore {
	return &BadgerUserStore{store: kv.NewUserStore(s), kv: s}
}

func (s *BadgerUserStore) Create(ctx context.Context, user *model.User) (*model.User, error) {
	if err := s.store.CreateUser(user); err != nil {
		return nil, err
	}
	// 维护 hash→pubkey 索引：API 主路径（/user/{id}、rating、admin 等）按公钥哈希查找，
	// 建索引后 Get(hash) 为 O(1) 而非全表扫描。索引写失败不阻塞主流程（Get 有扫描兜底）。
	_ = s.kv.Put([]byte(kv.PrefixUserHash+hashPublicKey(user.PublicKey)), []byte(user.PublicKey))
	return user, nil
}

func (s *BadgerUserStore) Get(ctx context.Context, pubkeyHash string) (*model.User, error) {
	// 判断传入的是原始公钥（base64，含 + / = 等）还是公钥哈希（hex，仅 0-9 a-f -）
	isRawPublicKey := false
	for _, c := range pubkeyHash {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && c != '-' {
			isRawPublicKey = true
			break
		}
	}

	// O(1) 路径：原始公钥直查；哈希经 user-hash: 索引取得公钥再直查
	if isRawPublicKey {
		if u, err := s.store.GetUser(pubkeyHash); err == nil {
			return u, nil
		}
	} else {
		if pub, err := s.kv.Get([]byte(kv.PrefixUserHash + pubkeyHash)); err == nil {
			if u, gerr := s.store.GetUser(string(pub)); gerr == nil {
				return u, nil
			}
		}
	}

	// 兜底：索引缺失（升级前写入的旧用户未建 hash 索引）→ 回退全表扫描，保证向后兼容
	users, err := s.store.ListUsers(0, 100000)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if isRawPublicKey {
			if u.PublicKey == pubkeyHash {
				return u, nil
			}
		} else {
			if hashPublicKey(u.PublicKey) == pubkeyHash {
				return u, nil
			}
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (s *BadgerUserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.store.GetByEmail(ctx, email)
}

func (s *BadgerUserStore) Update(ctx context.Context, user *model.User) (*model.User, error) {
	if err := s.store.UpdateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *BadgerUserStore) List(ctx context.Context, filter UserFilter) ([]*model.User, int64, error) {
	users, err := s.store.ListUsers(filter.Offset, filter.Limit)
	if err != nil {
		return nil, 0, err
	}

	// 应用过滤器
	var filtered []*model.User
	for _, u := range users {
		if filter.Status != "" && u.Status != filter.Status {
			continue
		}
		if filter.Level != 0 && u.UserLevel != filter.Level {
			continue
		}
		filtered = append(filtered, u)
	}

	return filtered, int64(len(filtered)), nil
}

// BadgerRatingStore 适配 kv.RatingStore 到 storage.RatingStore 接口
type BadgerRatingStore struct {
	store *kv.RatingStore
}

// NewBadgerRatingStore 创建 BadgerDB 评分存储
func NewBadgerRatingStore(s kv.Store) *BadgerRatingStore {
	return &BadgerRatingStore{store: kv.NewRatingStore(s)}
}

func (s *BadgerRatingStore) Create(ctx context.Context, rating *model.Rating) (*model.Rating, error) {
	if err := s.store.CreateRating(rating); err != nil {
		return nil, err
	}
	return rating, nil
}

func (s *BadgerRatingStore) Get(ctx context.Context, id string) (*model.Rating, error) {
	// Rating ID format is entryId:raterPubkey
	// Parse the ID to get entryId and raterPubkey
	// For simplicity, we scan all ratings
	ratings, err := s.store.GetRatingsByEntry("")
	if err != nil {
		return nil, err
	}
	for _, r := range ratings {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("rating not found")
}

func (s *BadgerRatingStore) ListByEntry(ctx context.Context, entryID string) ([]*model.Rating, error) {
	return s.store.GetRatingsByEntry(entryID)
}

// ListByRater 获取评分者的所有评分（经 by-rater 索引，O(评分者评分数)，修 N+1）。
func (s *BadgerRatingStore) ListByRater(ctx context.Context, raterPubkeyHash string) ([]*model.Rating, error) {
	return s.store.ListByRater(raterPubkeyHash)
}

// ListAll 获取全部评分（导出用；取代原先 entries×ListByEntry 的笛卡尔积 + 100k 截断）。
func (s *BadgerRatingStore) ListAll(ctx context.Context) ([]*model.Rating, error) {
	return s.store.ListAllRatings()
}

func (s *BadgerRatingStore) GetByRater(ctx context.Context, entryID, raterPubkeyHash string) (*model.Rating, error) {
	// We need to find rating by entryId and raterPubkey
	// The kv store uses raterPubkey directly, not hash
	ratings, err := s.store.GetRatingsByEntry(entryID)
	if err != nil {
		return nil, err
	}
	for _, r := range ratings {
		if r.RaterPubkey == raterPubkeyHash {
			return r, nil
		}
	}
	return nil, fmt.Errorf("rating not found")
}

func (s *BadgerRatingStore) ListRatedAfter(ctx context.Context, after int64) ([]*model.Rating, error) {
	allRatings, err := s.store.ListAllRatings()
	if err != nil {
		return nil, err
	}

	var result []*model.Rating
	for _, r := range allRatings {
		if r.RatedAt > after {
			result = append(result, r)
		}
	}
	return result, nil
}

// BadgerCategoryStore 适配 kv.CategoryStore 到 storage.CategoryStore 接口
type BadgerCategoryStore struct {
	store *kv.CategoryStore
}

// NewBadgerCategoryStore 创建 BadgerDB 分类存储
func NewBadgerCategoryStore(s kv.Store) *BadgerCategoryStore {
	return &BadgerCategoryStore{store: kv.NewCategoryStore(s)}
}

func (s *BadgerCategoryStore) Create(ctx context.Context, cat *model.Category) (*model.Category, error) {
	if err := s.store.CreateCategory(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *BadgerCategoryStore) Get(ctx context.Context, path string) (*model.Category, error) {
	return s.store.GetCategory(path)
}

func (s *BadgerCategoryStore) List(ctx context.Context, parentPath string) ([]*model.Category, error) {
	if parentPath == "" {
		return s.store.ListCategories()
	}
	return s.store.GetChildren(parentPath)
}

func (s *BadgerCategoryStore) ListAll(ctx context.Context) ([]*model.Category, error) {
	return s.store.ListCategories()
}

// ==================== 搜索引擎适配器 ====================

// BadgerSearchEngine 基于 BadgerDB 的搜索引擎实现
type BadgerSearchEngine struct {
	entryStore *BadgerEntryStore
	mu         sync.RWMutex
	entries    map[string]*model.KnowledgeEntry
}

// NewBadgerSearchEngine 创建搜索引擎
func NewBadgerSearchEngine(entryStore *BadgerEntryStore) *BadgerSearchEngine {
	return &BadgerSearchEngine{
		entryStore: entryStore,
		entries:    make(map[string]*model.KnowledgeEntry),
	}
}

func (e *BadgerSearchEngine) IndexEntry(entry *model.KnowledgeEntry) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := *entry
	e.entries[entry.ID] = &cp
	return nil
}

func (e *BadgerSearchEngine) UpdateIndex(entry *model.KnowledgeEntry) error {
	return e.IndexEntry(entry)
}

func (e *BadgerSearchEngine) DeleteIndex(entryID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.entries, entryID)
	return nil
}

func (e *BadgerSearchEngine) Search(ctx context.Context, query index.SearchQuery) (*index.SearchResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*model.KnowledgeEntry
	for _, entry := range e.entries {
		if entry.Status != model.EntryStatusPublished {
			continue
		}

		// 分类过滤
		if len(query.Categories) > 0 {
			matched := false
			for _, cat := range query.Categories {
				if entry.Category == cat {
					matched = true
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

		// 关键词匹配
		if query.Keyword != "" {
			keyword := query.Keyword
			if !containsIgnoreCase(entry.Title, keyword) && !containsIgnoreCase(entry.Content, keyword) {
				continue
			}
		}

		cp := *entry
		results = append(results, &cp)
	}

	// 按评分排序
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	total := len(results)

	// 分页
	if query.Offset > 0 {
		if query.Offset >= len(results) {
			return &index.SearchResult{TotalCount: total, HasMore: false, Entries: nil}, nil
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

func (e *BadgerSearchEngine) Close() error {
	return nil
}

// ==================== 反向链接索引适配器 ====================

// BadgerBacklinkIndex 反向链接索引（内存实现，启动时从持久化条目重建）。
//
// 该索引本身是纯内存的（outlinks/backlinks map），不直接读写 BadgerDB；但由
// NewBadgerStore / NewBadgerStoreWithCloser 在启动时遍历已发布条目、用 linkparser
// 解析链接后调用 UpdateIndex 重建。因此条目间的反向链接关系在重启后不丢失。
type BadgerBacklinkIndex struct {
	mu        sync.RWMutex
	outlinks  map[string]map[string]bool
	backlinks map[string]map[string]bool
}

// NewBadgerBacklinkIndex 创建反向链接索引
func NewBadgerBacklinkIndex() *BadgerBacklinkIndex {
	return &BadgerBacklinkIndex{
		outlinks:  make(map[string]map[string]bool),
		backlinks: make(map[string]map[string]bool),
	}
}

func (idx *BadgerBacklinkIndex) UpdateIndex(entryID string, linkedEntryIDs []string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 移除旧索引
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

	// 添加新索引
	newLinkedSet := make(map[string]bool)
	for _, linkedID := range linkedEntryIDs {
		if linkedID == entryID {
			continue
		}
		newLinkedSet[linkedID] = true
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

func (idx *BadgerBacklinkIndex) DeleteIndex(entryID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

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
	delete(idx.outlinks, entryID)

	return nil
}

func (idx *BadgerBacklinkIndex) GetBacklinks(targetEntryID string) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	backlinks, exists := idx.backlinks[targetEntryID]
	if !exists {
		return []string{}, nil
	}

	result := make([]string, 0, len(backlinks))
	for id := range backlinks {
		result = append(result, id)
	}
	return result, nil
}

func (idx *BadgerBacklinkIndex) GetOutlinks(sourceEntryID string) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	outlinks, exists := idx.outlinks[sourceEntryID]
	if !exists {
		return []string{}, nil
	}

	result := make([]string, 0, len(outlinks))
	for id := range outlinks {
		result = append(result, id)
	}
	return result, nil
}

// ==================== 工厂函数 ====================

// BadgerStoreWrapper 包装 Store 和 kv.Store 以支持关闭
type BadgerStoreWrapper struct {
	Store
	kvStore kv.Store
}

// Close 关闭存储
func (w *BadgerStoreWrapper) Close() error {
	if w.kvStore != nil {
		return w.kvStore.Close()
	}
	return nil
}

// NewBadgerStore 创建完整的 BadgerDB 存储实例
func NewBadgerStore(dataDir string) (*Store, error) {
	// 创建底层 BadgerDB 存储
	kvStore, err := kv.NewBadgerStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create badger store: %w", err)
	}

	// 创建各存储适配器
	entryStore := NewBadgerEntryStore(kvStore)
	userStore := NewBadgerUserStore(kvStore)
	ratingStore := NewBadgerRatingStore(kvStore)
	categoryStore := NewBadgerCategoryStore(kvStore)
	searchEngine := NewBadgerSearchEngine(entryStore)
	backlinkIndex := NewBadgerBacklinkIndex()

	// 创建标题索引并从已发布条目构建
	titleIdx := index.NewTitleIndex()
	entries, _, _ := entryStore.List(context.Background(), EntryFilter{Status: model.EntryStatusPublished, Limit: 100000})
	titleEntries := make([]index.TitleEntry, 0, len(entries))
	for _, e := range entries {
		titleEntries = append(titleEntries, index.TitleEntry{ID: e.ID, Title: e.Title})
		_ = backlinkIndex.UpdateIndex(e.ID, linkparser.ParseLinks(e.Content))
	}
	titleIdx.Build(titleEntries)
	// 重建 published 条目计数器（Create/Update/Delete 增量维护，Count 经它 O(1) 取值，
	// 取代原先 ListEntries(0,1000000) 全量反序列化）
	_ = kv.SetEntryPublishedCount(kvStore, int64(len(entries)))

	log.Printf("[Storage] BadgerDB initialized at %s", dataDir)

	return &Store{
		Entry:    entryStore,
		User:     userStore,
		Rating:   ratingStore,
		Category: categoryStore,
		Search:   searchEngine,
		Backlink: backlinkIndex,
		TitleIdx: titleIdx,
	}, nil
}

// NewBadgerStoreWithCloser 创建带关闭方法的存储
func NewBadgerStoreWithCloser(dataDir string) (*BadgerStoreWrapper, error) {
	// 创建底层 BadgerDB 存储
	kvStore, err := kv.NewBadgerStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create badger store: %w", err)
	}

	// 创建各存储适配器
	entryStore := NewBadgerEntryStore(kvStore)
	userStore := NewBadgerUserStore(kvStore)
	ratingStore := NewBadgerRatingStore(kvStore)
	categoryStore := NewBadgerCategoryStore(kvStore)
	searchEngine := NewBadgerSearchEngine(entryStore)
	backlinkIndex := NewBadgerBacklinkIndex()

	// 创建标题索引并从已发布条目构建
	titleIdx := index.NewTitleIndex()
	entries, _, _ := entryStore.List(context.Background(), EntryFilter{Status: model.EntryStatusPublished, Limit: 100000})
	titleEntries := make([]index.TitleEntry, 0, len(entries))
	for _, e := range entries {
		titleEntries = append(titleEntries, index.TitleEntry{ID: e.ID, Title: e.Title})
		_ = backlinkIndex.UpdateIndex(e.ID, linkparser.ParseLinks(e.Content))
	}
	titleIdx.Build(titleEntries)
	// 重建 published 条目计数器（详见 NewBadgerStore）
	_ = kv.SetEntryPublishedCount(kvStore, int64(len(entries)))

	log.Printf("[Storage] BadgerDB initialized at %s", dataDir)

	return &BadgerStoreWrapper{
		Store: Store{
			Entry:    entryStore,
			User:     userStore,
			Rating:   ratingStore,
			Category: categoryStore,
			Search:   searchEngine,
			Backlink: backlinkIndex,
			TitleIdx: titleIdx,
		},
		kvStore: kvStore,
	}, nil
}

// ==================== 辅助函数 ====================

func containsIgnoreCase(s, substr string) bool {
	// 简单实现
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			subc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if subc >= 'A' && subc <= 'Z' {
				subc += 32
			}
			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// hashPublicKey 计算公钥哈希
func hashPublicKey(publicKey string) string {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		hash := sha256.Sum256([]byte(publicKey))
		return hex.EncodeToString(hash[:])
	}
	hash := sha256.Sum256(pubKeyBytes)
	return hex.EncodeToString(hash[:])
}
