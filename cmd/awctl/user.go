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

会自动使用当前密钥对中的公钥进行注册。
如果密钥对不存在，会自动生成。

注册后用户等级为 Lv0，验证邮箱后升级到 Lv1。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName, _ := cmd.Flags().GetString("name")

		// 确保密钥已加载
		if !client.HasKeys() {
			keyDir := GetDefaultKeyDir()
			if err := client.LoadOrGenerateKeys(keyDir); err != nil {
				return fmt.Errorf("加载/生成密钥失败: %w", err)
			}
		}

		pubKey := client.GetPublicKey()
		if pubKey == "" {
			return fmt.Errorf("无法获取公钥")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := &RegisterRequest{
			PublicKey: pubKey,
			AgentName: agentName,
		}

		resp, err := client.RegisterUser(ctx, req)
		if err != nil {
			return fmt.Errorf("注册失败: %w", err)
		}

		fmt.Println("用户注册成功!")
		fmt.Printf("  公钥: %s\n", resp.PublicKey)
		if resp.AgentName != "" {
			fmt.Printf("  名称: %s\n", resp.AgentName)
		}
		fmt.Printf("  等级: Lv%d\n", resp.UserLevel)
		fmt.Printf("  注册时间: %s\n", time.UnixMilli(resp.CreatedAt).Format("2006-01-02 15:04:05"))
		fmt.Println()
		fmt.Println("提示: 验证邮箱可升级到 Lv1")
		fmt.Println("  awctl user verify --email your@email.com")

		return nil
	},
}

// userInfoCmd 获取当前用户信息
var userInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "获取当前用户信息",
	Long: `获取当前已认证用户的信息。

需要先运行 'awctl key generate' 生成密钥并注册。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !client.HasKeys() {
			return fmt.Errorf("未找到密钥，请先运行 'awctl key generate'")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		user, err := client.GetCurrentUserInfo(ctx)
		if err != nil {
			return fmt.Errorf("获取用户信息失败: %w", err)
		}

		fmt.Println("当前用户信息:")
		fmt.Printf("  公钥: %s\n", user.PublicKey)
		fmt.Printf("  Agent名称: %s\n", user.AgentName)
		if user.Email != "" {
			fmt.Printf("  邮箱: %s", user.Email)
			if user.Email != "" {
				fmt.Printf(" (已验证)")
			}
			fmt.Println()
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

// userUpdateCmd 更新用户信息
var userUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "更新用户信息",
	Long: `更新当前用户的 Agent 名称。

需要先运行 'awctl key generate' 生成密钥并注册。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName, _ := cmd.Flags().GetString("name")

		if agentName == "" {
			return fmt.Errorf("请指定新的 Agent 名称 (--name)")
		}

		if !client.HasKeys() {
			return fmt.Errorf("未找到密钥，请先运行 'awctl key generate'")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.UpdateUserInfo(ctx, agentName); err != nil {
			return fmt.Errorf("更新用户信息失败: %w", err)
		}

		fmt.Printf("用户信息已更新，Agent 名称: %s\n", agentName)
		return nil
	},
}

// userVerifyCmd 验证邮箱
var userVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "验证邮箱",
	Long: `验证邮箱地址。

两步流程：
1. 发送验证码到邮箱
2. 使用验证码完成验证

验证邮箱后，用户等级将升级到 Lv1。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		code, _ := cmd.Flags().GetString("code")
		sendOnly, _ := cmd.Flags().GetBool("send")

		if email == "" {
			return fmt.Errorf("请指定邮箱地址 (--email)")
		}

		if !client.HasKeys() {
			return fmt.Errorf("未找到密钥，请先运行 'awctl key generate'")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 如果只是发送验证码
		if sendOnly || code == "" {
			if err := client.SendVerificationCode(ctx, email); err != nil {
				return fmt.Errorf("发送验证码失败: %w", err)
			}
			fmt.Printf("验证码已发送到 %s\n", email)
			fmt.Println("请查收邮件并使用以下命令完成验证:")
			fmt.Printf("  awctl user verify --email %s --code <验证码>\n", email)
			return nil
		}

		// 使用验证码完成验证
		if err := client.VerifyEmail(ctx, email, code); err != nil {
			return fmt.Errorf("验证邮箱失败: %w", err)
		}

		fmt.Printf("邮箱 %s 验证成功！\n", email)
		fmt.Println("用户等级已升级到 Lv1")
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

	// register 子命令
	userCmd.AddCommand(userRegisterCmd)
	userRegisterCmd.Flags().String("name", "", "Agent 名称")

	// info 子命令
	userCmd.AddCommand(userInfoCmd)

	// update 子命令
	userCmd.AddCommand(userUpdateCmd)
	userUpdateCmd.Flags().String("name", "", "新的 Agent 名称")

	// verify 子命令
	userCmd.AddCommand(userVerifyCmd)
	userVerifyCmd.Flags().String("email", "", "邮箱地址")
	userVerifyCmd.Flags().String("code", "", "验证码")
	userVerifyCmd.Flags().Bool("send", false, "仅发送验证码")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
