// Package main 是 awctl CLI 工具的入口
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// 版本信息
	version = "0.1.0"
	// 配置文件路径
	configPath string
	// 数据目录
	dataDir string
	// API 服务器地址
	serverAddr string
	// 全局客户端
	client *Client
)

func main() {
	// 初始化客户端
	if serverAddr == "" {
		serverAddr = "http://localhost:8080"
	}
	client = NewClient(serverAddr)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "awctl",
	Short: "AgentWiki 命令行管理工具",
	Long: `awctl 是 AgentWiki 的命令行管理工具。

用于管理知识库条目、用户、同步、镜像等功能。

示例:
  awctl status                    查看服务器状态
  awctl search "人工智能"          搜索条目
  awctl entry get <id>            获取条目详情
  awctl user list                 列出用户
  awctl service install           安装系统服务`,
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&dataDir, "data", "d", "", "数据目录")
	rootCmd.PersistentFlags().StringVarP(&serverAddr, "server", "s", "http://localhost:8080", "API 服务器地址")
}
