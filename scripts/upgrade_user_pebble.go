//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("用法: go run scripts/upgrade_user_pebble.go <data_dir> <public_key_base64> <level>")
		fmt.Println("示例: go run scripts/upgrade_user_pebble.go ./data/seed '4Zok/irK6lbnki+MmQPn60lQ/tta5ruBkd9XnqqcBbo=' 1")
		os.Exit(1)
	}

	dataDir := os.Args[1]
	publicKey := os.Args[2]
	level := int32(1)
	fmt.Sscanf(os.Args[3], "%d", &level)

	// 打开 PebbleDB
	db, err := pebble.Open(dataDir+"/kv", &pebble.Options{})
	if err != nil {
		fmt.Printf("打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 用户键
	key := []byte("user:" + publicKey)

	// 获取用户数据
	value, closer, err := db.Get(key)
	if err != nil {
		fmt.Printf("获取用户失败: %v\n", err)
		os.Exit(1)
	}

	// 复制数据
	data := make([]byte, len(value))
	copy(data, value)
	closer.Close()

	// 解析用户
	var user map[string]interface{}
	if err := json.Unmarshal(data, &user); err != nil {
		fmt.Printf("解析用户数据失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("当前用户信息:\n")
	fmt.Printf("  公钥: %v\n", user["publicKey"])
	fmt.Printf("  名称: %v\n", user["agentName"])
	fmt.Printf("  当前等级: %v\n", user["userLevel"])

	// 更新等级
	user["userLevel"] = level

	// 保存
	newData, _ := json.Marshal(user)
	if err := db.Set(key, newData, pebble.Sync); err != nil {
		fmt.Printf("保存用户失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ 用户等级已更新为: %d\n", level)
}
