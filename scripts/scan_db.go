package main

import (
	"fmt"
	"os"

	"github.com/daifei0527/agentwiki/internal/storage/kv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run scripts/scan_db.go <data_dir>")
		os.Exit(1)
	}

	dataDir := os.Args[1]

	// 打开数据库
	store, err := kv.NewBadgerStore(dataDir + "/kv")
	if err != nil {
		fmt.Printf("打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// 扫描所有键
	prefix := []byte("")
	items, err := store.Scan(prefix)
	if err != nil {
		fmt.Printf("扫描失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("找到 %d 个键:\n", len(items))
	for key, value := range items {
		fmt.Printf("  键: %s (长度: %d)\n", key, len(value))
		if len(key) > 0 && (key[0] == 'u' || key[0] == 'e' || key[0] == 'c') {
			// 只打印前100个字符
			if len(value) > 100 {
				fmt.Printf("      值预览: %s...\n", string(value[:100]))
			} else {
				fmt.Printf("      值: %s\n", string(value))
			}
		}
	}
}
