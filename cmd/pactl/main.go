// Package main 是 pactl CLI 工具的入口
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

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

// searchCmd 顶层搜索命令（快捷方式）
var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "搜索知识条目",
	Long:  "搜索知识库中的条目，是 'pactl entry search' 的快捷方式",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		category, _ := cmd.Flags().GetString("category")
		limit, _ := cmd.Flags().GetInt("limit")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		entries, total, err := client.SearchEntries(ctx, query, limit)
		if err != nil {
			return fmt.Errorf("搜索失败: %w", err)
		}

		// 分类过滤（客户端）
		if category != "" {
			var filtered []EntryInfo
			for _, e := range entries {
				if e.Category == category {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}

		if len(entries) == 0 {
			fmt.Println(i18n.Tc(getLang(), "cli.entry.no_result"))
			return nil
		}

		fmt.Printf("搜索结果 (共 %d 条):\n\n", total)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\t标题\t分类\t评分\t创建时间")
		fmt.Fprintln(w, "--\t----\t----\t----\t--------")

		for _, e := range entries {
			id := e.ID
			if len(id) > 8 {
				id = id[:8]
			}
			title := e.Title
			if len(title) > 30 {
				title = title[:27] + "..."
			}
			createdAt := time.UnixMilli(e.CreatedAt).Format("2006-01-02")
			fmt.Fprintf(w, "%s\t%s\t%s\t%.1f\t%s\n", id, title, e.Category, e.Score, createdAt)
		}
		w.Flush()

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&dataDir, "data", "d", "", "数据目录")
	rootCmd.PersistentFlags().StringVarP(&serverAddr, "server", "s", "http://localhost:8080", "API 服务器地址")
	rootCmd.PersistentFlags().StringVar(&keyDir, "key-dir", "", "密钥目录 (默认 ~/.polyant/keys)")
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "zh-CN", "Output language (zh-CN, en-US)")

	// 顶层 search 命令
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().String("category", "", "按分类过滤")
	searchCmd.Flags().IntP("limit", "l", 20, "结果数量限制")
}
