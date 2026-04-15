//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/dgraph-io/badger/v4"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run scripts/scan_badger.go <data_dir>")
		os.Exit(1)
	}

	dataDir := os.Args[1]

	// 打开 BadgerDB
	opts := badger.DefaultOptions(dataDir + "/kv")
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		fmt.Printf("打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 扫描所有键
	count := 0
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			count++

			err := item.Value(func(v []byte) error {
				fmt.Printf("键 %d: %s (长度: %d)\n", count, string(k), len(v))
				if len(v) > 0 && len(v) < 500 {
					fmt.Printf("  值: %s\n", string(v))
				} else if len(v) > 0 {
					fmt.Printf("  值预览: %s...\n", string(v[:200]))
				}
				return nil
			})
			if err != nil {
				fmt.Printf("  读取值失败: %v\n", err)
			}

			if count > 100 {
				return nil
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("扫描失败: %v\n", err)
	}

	fmt.Printf("\n总共找到 %d 个键\n", count)
}
