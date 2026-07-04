package kv

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// entryPublishedCountKey 是"已发布条目数"计数器的键（落在 meta: 前缀，不被
// ListEntries 的 entry: 前缀扫描命中）。由 Create/Update/Delete 增量维护，
// 并在节点启动时（TitleIndex 构建处）从全量扫描重建，避免漂移。
const entryPublishedCountKey = PrefixMeta + "count:entry:published"

// SetEntryPublishedCount 设置已发布条目计数器（启动重建用；接受裸 Store 以便
// storage 层在装配时直接写入）。
func SetEntryPublishedCount(store Store, n int64) error {
	return store.Put([]byte(entryPublishedCountKey), []byte(strconv.FormatInt(n, 10)))
}

// publishedCountRaw 读取计数器原值；键不存在时返回 (0, ErrKeyNotFound)。
func (es *EntryStore) publishedCountRaw() (int64, error) {
	data, err := es.store.Get([]byte(entryPublishedCountKey))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(data), 10, 64)
}

// adjustPublishedCount 对计数器做 +delta/-delta 的读-改-写。
// 注意：kv.Store 无事务，并发写存在丢失更新的可能；该计数器仅用于节点状态展示
// 等非关键场景，且每次启动从全量数据重建对账，故可接受偶发偏差。
func (es *EntryStore) adjustPublishedCount(delta int64) error {
	cur, err := es.publishedCountRaw()
	if err != nil && err != ErrKeyNotFound {
		return err
	}
	next := cur + delta
	if next < 0 {
		next = 0
	}
	return es.store.Put([]byte(entryPublishedCountKey), []byte(strconv.FormatInt(next, 10)))
}

// PublishedCount 返回已发布条目数。优先读计数器（O(1)）；计数器缺失（旧数据/未
// 跑过启动重建）时回退到扫描，保证向后兼容。
func (es *EntryStore) PublishedCount() (int64, error) {
	if n, err := es.publishedCountRaw(); err == nil {
		return n, nil
	} else if err != ErrKeyNotFound {
		return 0, err
	}

	// 回退扫描
	entries, err := ScanAndParse(es.store, PrefixEntry, func(data []byte) (*model.KnowledgeEntry, error) {
		entry := &model.KnowledgeEntry{}
		if err := entry.FromJSON(data); err != nil {
			return nil, err
		}
		return entry, nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count published entries: %w", err)
	}
	var n int64
	for _, e := range entries {
		if e.Status == model.EntryStatusPublished {
			n++
		}
	}
	return n, nil
}

// EntryStore 提供知识条目的CRUD操作
type EntryStore struct {
	store Store
}

// NewEntryStore 创建一个新的知识条目存储实例
func NewEntryStore(store Store) *EntryStore {
	return &EntryStore{store: store}
}

// CreateEntry 创建一个新的知识条目
func (es *EntryStore) CreateEntry(entry *model.KnowledgeEntry) error {
	if entry.ID == "" {
		return fmt.Errorf("entry id must not be empty")
	}

	key := []byte(PrefixEntry + entry.ID)

	// 检查是否已存在
	_, err := es.store.Get(key)
	if err == nil {
		return fmt.Errorf("entry with id %s already exists", entry.ID)
	}

	// 计算内容哈希
	entry.ContentHash = entry.ComputeContentHash()

	data, err := entry.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize entry: %w", err)
	}

	if err := es.store.Put(key, data); err != nil {
		return err
	}
	// 维护 published 计数器
	if entry.Status == model.EntryStatusPublished {
		_ = es.adjustPublishedCount(1)
	}
	return nil
}

// GetEntry 根据ID获取知识条目
func (es *EntryStore) GetEntry(id string) (*model.KnowledgeEntry, error) {
	key := []byte(PrefixEntry + id)

	data, err := es.store.Get(key)
	if err != nil {
		if err == ErrKeyNotFound {
			return nil, fmt.Errorf("entry %s not found", id)
		}
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}

	entry := &model.KnowledgeEntry{}
	if err := entry.FromJSON(data); err != nil {
		return nil, fmt.Errorf("failed to deserialize entry: %w", err)
	}

	return entry, nil
}

// UpdateEntry 更新知识条目
func (es *EntryStore) UpdateEntry(entry *model.KnowledgeEntry) error {
	key := []byte(PrefixEntry + entry.ID)

	// 读旧条目（用于维护 published 计数器：状态变更时调整）
	oldData, err := es.store.Get(key)
	if err != nil {
		if err == ErrKeyNotFound {
			return fmt.Errorf("entry %s not found", entry.ID)
		}
		return fmt.Errorf("failed to check entry existence: %w", err)
	}

	// B1：Version/UpdatedAt 由调用方负责（handler 自增、sync 设 max）。
	// 存储层只忠实写入，并重算 ContentHash（幂等，保证哈希契约）。
	entry.ContentHash = entry.ComputeContentHash()

	data, err := entry.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize entry: %w", err)
	}

	if err := es.store.Put(key, data); err != nil {
		return err
	}

	// 维护 published 计数器：仅状态变更时调整
	var old model.KnowledgeEntry
	if json.Unmarshal(oldData, &old) == nil && old.Status != entry.Status {
		delta := int64(0)
		if entry.Status == model.EntryStatusPublished {
			delta++
		}
		if old.Status == model.EntryStatusPublished {
			delta--
		}
		if delta != 0 {
			_ = es.adjustPublishedCount(delta)
		}
	}
	return nil
}

// DeleteEntry 根据ID删除知识条目
func (es *EntryStore) DeleteEntry(id string) error {
	key := []byte(PrefixEntry + id)

	// 读旧条目以维护 published 计数器
	oldData, getErr := es.store.Get(key)

	err := es.store.Delete(key)
	if err != nil {
		if err == ErrKeyNotFound {
			return fmt.Errorf("entry %s not found", id)
		}
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	if getErr == nil {
		var old model.KnowledgeEntry
		if json.Unmarshal(oldData, &old) == nil && old.Status == model.EntryStatusPublished {
			_ = es.adjustPublishedCount(-1)
		}
	}
	return nil
}

// ListEntries 列出知识条目，支持分页
func (es *EntryStore) ListEntries(offset, limit int) ([]*model.KnowledgeEntry, error) {
	entries, err := ScanAndParse(es.store, PrefixEntry, func(data []byte) (*model.KnowledgeEntry, error) {
		entry := &model.KnowledgeEntry{}
		if err := entry.FromJSON(data); err != nil {
			return nil, err
		}
		return entry, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}

	// 按更新时间倒序排列
	sortEntriesByUpdated(entries)

	// 应用分页
	return paginateEntries(entries, offset, limit), nil
}

// ListByCategory 根据分类路径列出知识条目
func (es *EntryStore) ListByCategory(category string) ([]*model.KnowledgeEntry, error) {
	entries, err := ScanAndParse(es.store, PrefixEntry, func(data []byte) (*model.KnowledgeEntry, error) {
		entry := &model.KnowledgeEntry{}
		if err := entry.FromJSON(data); err != nil {
			return nil, err
		}
		return entry, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list entries by category: %w", err)
	}

	// 过滤指定分类的条目
	var filtered []*model.KnowledgeEntry
	for _, entry := range entries {
		if entry.Category == category {
			filtered = append(filtered, entry)
		}
	}

	// 按更新时间倒序排列
	sortEntriesByUpdated(filtered)

	return filtered, nil
}

// sortEntriesByUpdated 按更新时间倒序排列条目
func sortEntriesByUpdated(entries []*model.KnowledgeEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UpdatedAt > entries[j].UpdatedAt
	})
}

// paginateEntries 对条目列表进行分页
func paginateEntries(entries []*model.KnowledgeEntry, offset, limit int) []*model.KnowledgeEntry {
	if offset >= len(entries) {
		return []*model.KnowledgeEntry{}
	}

	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}

	return entries[offset:end]
}
