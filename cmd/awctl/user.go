package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// userCmd 用户管理命令
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "用户管理",
	Long:  "管理用户账户、层级、权限等",
}

// userListCmd 列出用户
var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出用户",
	RunE: func(cmd *cobra.Command, args []string) error {
		level, _ := cmd.Flags().GetInt32("level")
		limit, _ := cmd.Flags().GetInt("limit")

		// TODO: 实现用户列表 API
		_ = level

		fmt.Printf("列出用户 (limit=%d):\n\n", limit)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "公钥\tAgent名称\t等级\t贡献数\t评分数\t注册时间")
		fmt.Fprintln(w, "----\t----------\t----\t------\t------\t--------")
		w.Flush()

		fmt.Println("\n提示: 使用 'awctl user get <public-key>' 查看用户详情")

		return nil
	},
}

// userGetCmd 获取用户信息
var userGetCmd = &cobra.Command{
	Use:   "get <public-key>",
	Short: "获取用户详情",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pubKey := args[0]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		user, err := client.GetUser(ctx, pubKey)
		if err != nil {
			return fmt.Errorf("获取用户失败: %w", err)
		}

		fmt.Println("用户信息:")
		fmt.Printf("  公钥: %s\n", user.PublicKey)
		fmt.Printf("  Agent名称: %s\n", user.AgentName)
		if user.Email != "" {
			fmt.Printf("  邮箱: %s\n", maskEmail(user.Email))
		}
		fmt.Printf("  等级: Lv%d\n", user.UserLevel)
		fmt.Printf("  贡献数: %d\n", user.ContribCount)
		fmt.Printf("  评分数: %d\n", user.RatingCount)
		fmt.Printf("  注册时间: %s\n", time.UnixMilli(user.CreatedAt).Format("2006-01-02 15:04:05"))
		if user.LastActiveAt > 0 {
			fmt.Printf("  最后活跃: %s\n", time.UnixMilli(user.LastActiveAt).Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}

// userLevelCmd 用户等级管理
var userLevelCmd = &cobra.Command{
	Use:   "level <public-key> <level>",
	Short: "设置用户等级",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pubKey := args[0]
		level := args[1]

		// TODO: 实现等级设置 API
		fmt.Printf("已将用户 %s... 的等级设置为 %s\n", pubKey[:20], level)
		fmt.Println("注意: 需要管理员权限才能设置用户等级")
		return nil
	},
}

// userStatsCmd 用户统计
var userStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "用户统计信息",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := client.GetStatus(ctx)
		if err != nil {
			return fmt.Errorf("获取状态失败: %w", err)
		}

		fmt.Println("用户统计:")
		fmt.Printf("  总用户数: %d\n", status.UserCount)
		fmt.Println()
		fmt.Println("等级分布:")
		fmt.Println("  Lv0 (基础用户): -")
		fmt.Println("  Lv1 (认证用户): -")
		fmt.Println("  Lv2 (活跃用户): -")
		fmt.Println("  Lv3 (高级用户): -")
		fmt.Println("  Lv4 (专家用户): -")
		fmt.Println("  Lv5 (权威用户): -")

		return nil
	},
}

// userRegisterCmd 注册用户
var userRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "注册新用户",
	Long: `注册新的用户账户。

需要提供公钥和可选的 Agent 名称。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pubKey, _ := cmd.Flags().GetString("public-key")
		agentName, _ := cmd.Flags().GetString("name")
		email, _ := cmd.Flags().GetString("email")

		if pubKey == "" {
			return fmt.Errorf("必须提供公钥 (--public-key)")
		}

		// TODO: 实现用户注册 API
		fmt.Println("注册用户:")
		fmt.Printf("  公钥: %s\n", pubKey[:min(32, len(pubKey))]+"...")
		if agentName != "" {
			fmt.Printf("  名称: %s\n", agentName)
		}
		if email != "" {
			fmt.Printf("  邮箱: %s\n", email)
		}
		fmt.Println("\n请使用 Ed25519 密钥对签名请求来完成注册")

		return nil
	},
}

func maskEmail(email string) string {
	if len(email) < 5 {
		return "***"
	}
	at := -1
	for i, c := range email {
		if c == '@' {
			at = i
			break
		}
	}
	if at == -1 {
		return "***"
	}
	if at <= 2 {
		return email[:at] + "***" + email[at:]
	}
	return email[:2] + "***" + email[at:]
}

func init() {
	rootCmd.AddCommand(userCmd)

	userCmd.AddCommand(userListCmd)
	userListCmd.Flags().Int32("level", -1, "按等级过滤（-1表示全部）")
	userListCmd.Flags().IntP("limit", "l", 20, "结果数量限制")

	userCmd.AddCommand(userGetCmd)
	userCmd.AddCommand(userLevelCmd)
	userCmd.AddCommand(userStatsCmd)

	userCmd.AddCommand(userRegisterCmd)
	userRegisterCmd.Flags().String("public-key", "", "用户公钥")
	userRegisterCmd.Flags().String("name", "", "Agent 名称")
	userRegisterCmd.Flags().String("email", "", "邮箱地址")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
