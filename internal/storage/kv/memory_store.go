package kv

import (
	"strings"
	"sync"
)

// MemoryStore 是简单的内存键值存储实现
// 用于测试和内存存储场景
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMemoryStore 创建内存存储实例
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string][]byte),
	}
}

// Put 存储一个键值对
func (s *MemoryStore) Put(key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[string(key)] = value
	return nil
}

// Get 根据键获取值
func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.data[string(key)]
	if !exists {
		return nil, ErrKeyNotFound
	}

	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Delete 根据键删除键值对
func (s *MemoryStore) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	if _, exists := s.data[keyStr]; !exists {
		return ErrKeyNotFound
	}

	delete(s.data, keyStr)
	return nil
}

// Scan 扫描指定前缀的所有键值对
func (s *MemoryStore) Scan(prefix []byte) (map[string][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefixStr := string(prefix)
	result := make(map[string][]byte)

	for k, v := range s.data {
		if strings.HasPrefix(k, prefixStr) {
			result[k] = v
		}
	}

	return result, nil
}

// Close 关闭存储（内存存储无需操作）
func (s *MemoryStore) Close() error {
	return nil
}

var _ Store = (*MemoryStore)(nil)
