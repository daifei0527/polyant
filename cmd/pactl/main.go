// Package main 是 pactl CLI 工具的入口
package main

import (
	"fmt"
	"os"

	"github.com/daifei0527/polyant/pkg/i18n"
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
	// 密钥目录
	keyDir string
	// 输出语言
	langFlag string
	// 全局客户端
	client *Client
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "pactl",
	Short: "Polyant 命令行管理工具",
	Long: `pactl 是 Polyant 的命令行管理工具。

用于管理知识库条目、用户、同步、镜像等功能。

示例:
  pactl key generate              生成密钥对
  pactl user register --name "my-agent"  注册用户
  pactl status                    查看服务器状态
  pactl search "人工智能"          搜索条目
  pactl entry get <id>            获取条目详情`,
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 初始化 i18n
		localesDir := "pkg/i18n/locales"
		if err := i18n.Init(localesDir, i18n.Lang(langFlag)); err != nil {
			// 静默失败，使用默认语言
		}

		// 初始化客户端
		client = NewClient(serverAddr)

		// 尝试加载密钥（静默失败，某些命令不需要密钥）
		keyPath := keyDir
		if keyPath == "" {
			keyPath = GetDefaultKeyDir()
		}
		_ = client.LoadOrGenerateKeys(keyPath) // 静默忽略错误
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&dataDir, "data", "d", "", "数据目录")
	rootCmd.PersistentFlags().StringVarP(&serverAddr, "server", "s", "http://localhost:8080", "API 服务器地址")
	rootCmd.PersistentFlags().StringVar(&keyDir, "key-dir", "", "密钥目录 (默认 ~/.polyant/keys)")
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "zh-CN", "Output language (zh-CN, en-US)")
}
