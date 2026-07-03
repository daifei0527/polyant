package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// adminSetPasswordCmd 为指定用户设置/重置 Web admin 登录密码。
// 需调用者持有 ManageUser 权限（服务端 RequirePermission 守卫）。
var adminSetPasswordCmd = &cobra.Command{
	Use:   "set-password",
	Short: "为指定用户设置 Web admin 登录密码",
	Long: `为指定用户设置 Web admin 登录密码（bcrypt 存储）。

设置后该用户可用标识（email 或公钥）+ 密码通过 Web admin 登录。
需调用者自身持有 ManageUser 权限（Lv4+）。

示例:
  pactl admin set-password --pubkey <pubkey> --password <password>
  pactl admin set-password --pubkey <pubkey>   # 交互式输入密码`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		pw := adminSetPasswordPassword
		if pw == "" {
			fmt.Fprint(os.Stderr, "输入新密码（至少 8 位）: ")
			b, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("读取密码失败: %w", err)
			}
			pw = string(b)
			fmt.Fprintln(os.Stderr)
		}
		if len(pw) < 8 {
			return fmt.Errorf("密码至少 8 位")
		}

		if err := client.AdminSetPassword(ctx, adminSetPasswordPubkey, pw); err != nil {
			return fmt.Errorf("设置密码失败: %w", err)
		}
		fmt.Println("密码设置成功")
		return nil
	},
}

var (
	adminSetPasswordPubkey   string
	adminSetPasswordPassword string
)

func init() {
	adminSetPasswordCmd.Flags().StringVar(&adminSetPasswordPubkey, "pubkey", "", "目标用户公钥（必填）")
	adminSetPasswordCmd.Flags().StringVar(&adminSetPasswordPassword, "password", "", "新密码（不填则交互读取，至少 8 位）")
	_ = adminSetPasswordCmd.MarkFlagRequired("pubkey")
	adminCmd.AddCommand(adminSetPasswordCmd)
}
