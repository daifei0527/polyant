// Package kv 提供键值存储实现
package kv

import (
	"github.com/cockroachdb/pebble"
)

// PebbleStore 是基于 Pebble 的持久化键值存储实现
// Pebble 是 CockroachDB 团队开发的高性能嵌入式 KV 存储
type PebbleStore struct {
	db *pebble.DB
}

// NewPebbleStore 创建一个新的 Pebble 存储实例
// 如果目录不存在，会自动创建
func NewPebbleStore(dir string) (*PebbleStore, error) {
	opts := &pebble.Options{
		// 使用默认配置，生产环境可调整
		// Pebble 默认使用 Snappy 压缩
	}

	db, err := pebble.Open(dir, opts)
	if err != nil {
		return nil, err
	}

	return &PebbleStore{
		db: db,
	}, nil
}

// Put 存储一个键值对
func (s *PebbleStore) Put(key, value []byte) error {
	// Sync=true 确保数据持久化
	return s.db.Set(key, value, pebble.Sync)
}

// Get 根据键获取值
func (s *PebbleStore) Get(key []byte) ([]byte, error) {
	value, closer, err := s.db.Get(key)
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	defer closer.Close()

	// 复制值，因为 Pebble 的值在 closer.Close() 后失效
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Delete 根据键删除键值对
func (s *PebbleStore) Delete(key []byte) error {
	err := s.db.Delete(key, pebble.Sync)
	if err == pebble.ErrNotFound {
		return ErrKeyNotFound
	}
	return err
}

// Scan 扫描指定前缀的所有键值对
func (s *PebbleStore) Scan(prefix []byte) (map[string][]byte, error) {
	result := make(map[string][]byte)

	iter, err := s.db.NewIter(nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	// 设置起始位置
	iter.SeekGE(prefix)

	for iter.Valid() {
		key := iter.Key()

		// 检查是否还在前缀范围内
		if !hasPrefix(key, prefix) {
			break
		}

		value, err := iter.ValueAndErr()
		if err != nil {
			iter.Next()
			continue
		}

		// 复制键值
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)

		result[string(keyCopy)] = valueCopy
		iter.Next()
	}

	return result, nil
}

// hasPrefix 检查字节切片是否有指定前缀
func hasPrefix(data, prefix []byte) bool {
	if len(prefix) > len(data) {
		return false
	}
	for i, b := range prefix {
		if data[i] != b {
			return false
		}
	}
	return true
}

// Close 关闭存储
func (s *PebbleStore) Close() error {
	return s.db.Close()
}

// Flush 刷新内存表到磁盘
func (s *PebbleStore) Flush() error {
	return s.db.Flush()
}

// Compact 触发压缩，回收空间
func (s *PebbleStore) Compact() error {
	return s.db.Compact(nil, nil, false)
}
