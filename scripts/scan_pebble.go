//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run scripts/scan_pebble.go <data_dir>")
		os.Exit(1)
	}

	dataDir := os.Args[1]

	// 打开 PebbleDB
	db, err := pebble.Open(dataDir+"/kv", &pebble.Options{})
	if err != nil {
		fmt.Printf("打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 扫描所有键
	count := 0
	iter, err := db.NewIter(nil)
	if err != nil {
		fmt.Printf("创建迭代器失败: %v\n", err)
		os.Exit(1)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value, err := iter.ValueAndErr()
		if err != nil {
			fmt.Printf("读取值失败: %v\n", err)
			continue
		}

		count++
		fmt.Printf("键 %d: %s (长度: %d)\n", count, string(key), len(value))

		if len(value) > 0 && len(value) < 500 {
			fmt.Printf("  值: %s\n", string(value))
		} else if len(value) > 0 {
			fmt.Printf("  值预览: %s...\n", string(value[:min(200, len(value))]))
		}

		if count > 100 {
			break
		}
	}

	if err := iter.Error(); err != nil {
		fmt.Printf("迭代错误: %v\n", err)
	}

	fmt.Printf("\n总共找到 %d 个键\n", count)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
