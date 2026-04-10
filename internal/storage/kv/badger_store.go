// Package kv 提供基于JSON文件的键值存储实现
package kv

import (
	"github.com/dgraph-io/badger/v4"
)

// BadgerStore 是基于 BadgerDB 的持久化键值存储实现
// BadgerDB 是一个高性能的嵌入式键值数据库，适合持久化存储
type BadgerStore struct {
	db *badger.DB
}

// NewBadgerStore 创建一个新的 BadgerDB 存储实例
// 如果目录不存在，会自动创建
func NewBadgerStore(dir string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(dir)
	// 只在需要时打印日志
	opts.Logger = nil

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &BadgerStore{
		db: db,
	}, nil
}

// Put 存储一个键值对
func (s *BadgerStore) Put(key, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// Get 根据键获取值
func (s *BadgerStore) Get(key []byte) ([]byte, error) {
	var valCopy []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		valCopy, err = item.ValueCopy(nil)
		return err
	})
	if err == badger.ErrKeyNotFound {
		return nil, ErrKeyNotFound
	}
	return valCopy, err
}

// Delete 根据键删除键值对
func (s *BadgerStore) Delete(key []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// Scan 扫描指定前缀的所有键值对
func (s *BadgerStore) Scan(prefix []byte) (map[string][]byte, error) {
	result := make(map[string][]byte)
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			result[string(key)] = val
		}
		return nil
	})
	return result, err
}

// Close 关闭存储
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// RunGC 运行 BadgerDB 垃圾回收
// 应该定期调用以回收空间
func (s *BadgerStore) RunGC() error {
	return s.db.RunValueLogGC(0.7)
}
