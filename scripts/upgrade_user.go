package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/daifei0527/agentwiki/internal/storage/kv"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

const PrefixUser = "user:"

func main() {
	if len(os.Args) < 3 {
		fmt.Println("用法: go run scripts/upgrade_user.go <data_dir> <public_key_base64> [level]")
		fmt.Println("示例: go run scripts/upgrade_user.go ./data/user SjLWhoPmpwC7ghP8MyY1TnqLA3wNVzJr0TqKw+4bf5c= 1")
		os.Exit(1)
	}

	dataDir := os.Args[1]
	publicKey := os.Args[2]
	level := int32(1)
	if len(os.Args) > 3 {
		fmt.Sscanf(os.Args[3], "%d", &level)
	}

	// 打开数据库
	store, err := kv.NewBadgerStore(dataDir + "/kv")
	if err != nil {
		fmt.Printf("打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// 获取用户
	key := []byte(PrefixUser + publicKey)
	data, err := store.Get(key)
	if err != nil {
		fmt.Printf("获取用户失败: %v\n", err)
		fmt.Printf("尝试扫描所有用户...\n")

		// 扫描所有用户
		users, scanErr := scanUsers(store)
		if scanErr != nil {
			fmt.Printf("扫描用户失败: %v\n", scanErr)
			os.Exit(1)
		}

		fmt.Printf("找到 %d 个用户:\n", len(users))
		for _, u := range users {
			fmt.Printf("  - 公钥: %s, 名称: %s, 等级: %d\n", u.PublicKey, u.AgentName, u.UserLevel)
		}
		os.Exit(1)
	}

	var user model.User
	if err := json.Unmarshal(data, &user); err != nil {
		fmt.Printf("解析用户数据失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("当前用户信息:\n")
	fmt.Printf("  公钥: %s\n", user.PublicKey)
	fmt.Printf("  名称: %s\n", user.AgentName)
	fmt.Printf("  当前等级: %d\n", user.UserLevel)

	// 更新等级
	user.UserLevel = level

	// 保存
	newData, _ := json.Marshal(user)
	if err := store.Put(key, newData); err != nil {
		fmt.Printf("保存用户失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ 用户等级已更新为: %d\n", level)
}

func scanUsers(store kv.Store) ([]*model.User, error) {
	prefix := []byte(PrefixUser)
	items, err := store.Scan(prefix)
	if err != nil {
		return nil, err
	}

	var users []*model.User
	for _, data := range items {
		var user model.User
		if err := json.Unmarshal(data, &user); err != nil {
			continue
		}
		users = append(users, &user)
	}
	return users, nil
}
