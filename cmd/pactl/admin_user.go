package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// adminUsersCmd 用户管理命令组
var adminUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "用户管理",
	Long:  "用户管理操作，包括列出、封禁、设置等级等",
}

// adminUsersListCmd 列出用户
var adminUsersListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出用户",
	RunE: func(cmd *cobra.Command, args []string) error {
		level, _ := cmd.Flags().GetInt32("level")
		limit, _ := cmd.Flags().GetInt("limit")
		search, _ := cmd.Flags().GetString("search")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		users, total, err := client.ListUsers(ctx, 1, limit, level, search)
		if err != nil {
			return fmt.Errorf("获取用户列表失败: %w", err)
		}

		fmt.Printf("用户列表 (共 %d 个):\n\n", total)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "公钥\t名称\t等级\t状态\t贡献数\t评分数")
		fmt.Fprintln(w, "----\t----\t----\t----\t------\t------")

		for _, u := range users {
			pubKey := u.PublicKey
			if len(pubKey) > 20 {
				pubKey = pubKey[:20] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\tLv%d\t%s\t%d\t%d\n",
				pubKey, u.AgentName, u.UserLevel, u.Status, u.ContributionCnt, u.RatingCnt)
		}
		w.Flush()

		return nil
	},
}

// adminUsersBanCmd 封禁用户
var adminUsersBanCmd = &cobra.Command{
	Use:   "ban <public-key>",
	Short: "封禁用户",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pubKey := args[0]
		reason, _ := cmd.Flags().GetString("reason")
		banType, _ := cmd.Flags().GetString("type")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.BanUser(ctx, pubKey, reason, banType); err != nil {
			return fmt.Errorf("封禁用户失败: %w", err)
		}

		fmt.Printf("用户 %s 已被封禁\n", pubKey[:min(20, len(pubKey))])
		return nil
	},
}

// adminUsersUnbanCmd 解封用户
var adminUsersUnbanCmd = &cobra.Command{
	Use:   "unban <public-key>",
	Short: "解封用户",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pubKey := args[0]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.UnbanUser(ctx, pubKey); err != nil {
			return fmt.Errorf("解封用户失败: %w", err)
		}

		fmt.Printf("用户 %s 已解封\n", pubKey[:min(20, len(pubKey))])
		return nil
	},
}

// adminUsersLevelCmd 设置用户等级
var adminUsersLevelCmd = &cobra.Command{
	Use:   "level <public-key> <level>",
	Short: "设置用户等级 (需要 Lv5)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pubKey := args[0]
		level := args[1]
		reason, _ := cmd.Flags().GetString("reason")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.SetUserLevel(ctx, pubKey, parseLevel(level), reason); err != nil {
			return fmt.Errorf("设置等级失败: %w", err)
		}

		fmt.Printf("用户 %s 等级已设置为 %s\n", pubKey[:min(20, len(pubKey))], level)
		return nil
	},
}

func init() {
	adminCmd.AddCommand(adminUsersCmd)

	adminUsersCmd.AddCommand(adminUsersListCmd)
	adminUsersListCmd.Flags().Int32("level", -1, "按等级过滤")
	adminUsersListCmd.Flags().IntP("limit", "l", 20, "结果数量限制")
	adminUsersListCmd.Flags().String("search", "", "搜索关键词")

	adminUsersCmd.AddCommand(adminUsersBanCmd)
	adminUsersBanCmd.Flags().String("reason", "", "封禁原因")
	adminUsersBanCmd.Flags().String("type", "full", "封禁类型 (full/readonly)")

	adminUsersCmd.AddCommand(adminUsersUnbanCmd)

	adminUsersCmd.AddCommand(adminUsersLevelCmd)
	adminUsersLevelCmd.Flags().String("reason", "", "设置原因")
}

func parseLevel(s string) int32 {
	levels := map[string]int32{
		"0": 0, "lv0": 0, "Lv0": 0,
		"1": 1, "lv1": 1, "Lv1": 1,
		"2": 2, "lv2": 2, "Lv2": 2,
		"3": 3, "lv3": 3, "Lv3": 3,
		"4": 4, "lv4": 4, "Lv4": 4,
		"5": 5, "lv5": 5, "Lv5": 5,
	}
	if l, ok := levels[s]; ok {
		return l
	}
	return -1
}
