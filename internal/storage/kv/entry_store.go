package kv

import (
	"fmt"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

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

	return es.store.Put(key, data)
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

	// 检查条目是否存在
	_, err := es.store.Get(key)
	if err != nil {
		if err == ErrKeyNotFound {
			return fmt.Errorf("entry %s not found", entry.ID)
		}
		return fmt.Errorf("failed to check entry existence: %w", err)
	}

	// 更新时间戳和版本号
	entry.UpdatedAt = time.Now().Unix()
	entry.Version++
	entry.ContentHash = entry.ComputeContentHash()

	data, err := entry.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize entry: %w", err)
	}

	return es.store.Put(key, data)
}

// DeleteEntry 根据ID删除知识条目
func (es *EntryStore) DeleteEntry(id string) error {
	key := []byte(PrefixEntry + id)

	err := es.store.Delete(key)
	if err != nil {
		if err == ErrKeyNotFound {
			return fmt.Errorf("entry %s not found", id)
		}
		return fmt.Errorf("failed to delete entry: %w", err)
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
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].UpdatedAt > entries[i].UpdatedAt {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
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
