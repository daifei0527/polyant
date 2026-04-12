// Package kv 提供基于JSON文件的键值存储实现
package kv

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
)

// ==================== 键前缀常量 ====================

const (
	PrefixEntry    = "entry:"    // 知识条目键前缀
	PrefixUser     = "user:"     // 用户键前缀
	PrefixRating   = "rating:"   // 评分键前缀
	PrefixCategory = "category:" // 分类键前缀
	PrefixNode     = "node:"     // 节点键前缀
	PrefixMeta     = "meta:"     // 元数据键前缀
)

// ==================== Store接口 ====================

// Store 定义了键值存储的基本接口
type Store interface {
	// Put 存储一个键值对
	Put(key, value []byte) error
	// Get 根据键获取值
	Get(key []byte) ([]byte, error)
	// Delete 根据键删除键值对
	Delete(key []byte) error
	// Scan 扫描指定前缀的所有键值对
	Scan(prefix []byte) (map[string][]byte, error)
	// Close 关闭存储
	Close() error
}

// ==================== JSONFileStore实现 ====================

// JSONFileStore 是基于JSON文件的键值存储实现
// 数据以JSON格式保存在单个文件中，适合开发和小规模使用
type JSONFileStore struct {
	mu       sync.RWMutex
	filePath string
	data     map[string][]byte
	dirty    bool
}

// NewJSONFileStore 创建一个新的JSON文件存储实例
// 如果文件已存在，会加载已有数据
func NewJSONFileStore(filePath string) (*JSONFileStore, error) {
	store := &JSONFileStore{
		filePath: filePath,
		data:     make(map[string][]byte),
	}

	// 尝试加载已有数据
	if _, err := os.Stat(filePath); err == nil {
		if err := store.load(); err != nil {
			return nil, err
		}
	}

	return store, nil
}

// Put 存储一个键值对
func (s *JSONFileStore) Put(key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	s.data[keyStr] = make([]byte, len(value))
	copy(s.data[keyStr], value)
	s.dirty = true

	return s.save()
}

// Get 根据键获取值
func (s *JSONFileStore) Get(key []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyStr := string(key)
	value, exists := s.data[keyStr]
	if !exists {
		return nil, ErrKeyNotFound
	}

	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Delete 根据键删除键值对
func (s *JSONFileStore) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	if _, exists := s.data[keyStr]; !exists {
		return ErrKeyNotFound
	}

	delete(s.data, keyStr)
	s.dirty = true

	return s.save()
}

// Scan 扫描指定前缀的所有键值对
func (s *JSONFileStore) Scan(prefix []byte) (map[string][]byte, error) {
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

// Close 关闭存储，将数据持久化到文件
func (s *JSONFileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dirty {
		return s.save()
	}
	return nil
}

// load 从文件加载数据
func (s *JSONFileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	// JSON存储格式: 将[]byte值编码为base64字符串
	strData := make(map[string]string)
	if err := json.Unmarshal(data, &strData); err != nil {
		return err
	}

	s.data = make(map[string][]byte)
	for k, v := range strData {
		s.data[k] = []byte(v)
	}

	s.dirty = false
	return nil
}

// save 将数据保存到文件
func (s *JSONFileStore) save() error {
	// 将[]byte值编码为字符串以便JSON序列化
	strData := make(map[string]string)
	for k, v := range s.data {
		strData[k] = string(v)
	}

	data, err := json.MarshalIndent(strData, "", "  ")
	if err != nil {
		return err
	}

	// 原子写入：先写临时文件，再重命名
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return err
	}

	s.dirty = false
	return nil
}

// ==================== Scan辅助函数 ====================

// ScanAndParse 扫描指定前缀的键值对，并将值解析为目标类型
func ScanAndParse[T any](store Store, prefix string, parseFunc func([]byte) (T, error)) ([]T, error) {
	results, err := store.Scan([]byte(prefix))
	if err != nil {
		return nil, err
	}

	// 对键排序以保证结果顺序一致
	keys := make([]string, 0, len(results))
	for k := range results {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	items := make([]T, 0, len(results))
	for _, k := range keys {
		item, err := parseFunc(results[k])
		if err != nil {
			continue // 跳过解析失败的条目
		}
		items = append(items, item)
	}

	return items, nil
}

// ==================== 错误定义 ====================

// ErrKeyNotFound 表示键不存在
var ErrKeyNotFound = &storeError{"key not found"}

// storeError 存储层错误类型
type storeError struct {
	msg string
}

func (e *storeError) Error() string {
	return e.msg
}

// ==================== 工厂函数 ====================

// StoreType 存储类型
type StoreType string

const (
	// StoreTypeJSONFile JSON文件存储（适合开发和小规模使用）
	StoreTypeJSONFile StoreType = "jsonfile"
	// StoreTypeBadger BadgerDB持久化存储（生产环境推荐）
	StoreTypeBadger StoreType = "badger"
	// StoreTypePebble Pebble持久化存储（高性能生产环境推荐）
	StoreTypePebble StoreType = "pebble"
)

// NewStore 根据类型创建存储实例
func NewStore(storeType StoreType, path string) (Store, error) {
	switch storeType {
	case StoreTypeJSONFile:
		return NewJSONFileStore(path)
	case StoreTypeBadger:
		return NewBadgerStore(path)
	case StoreTypePebble:
		return NewPebbleStore(path)
	default:
		return nil, &storeError{"unknown store type: " + string(storeType)}
	}
}
