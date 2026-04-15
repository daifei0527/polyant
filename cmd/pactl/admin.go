package main

import (
	"github.com/spf13/cobra"
)

// adminCmd 管理员命令组
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "管理员操作",
	Long: `管理员操作命令。

需要管理员 (Lv4+) 或超级管理员 (Lv5) 权限。

子命令:
  users    用户管理
  entries  内容审核
  stats    数据统计
  status   系统状态`,
}

func init() {
	rootCmd.AddCommand(adminCmd)
}
