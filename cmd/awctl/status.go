package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// statusCmd 状态查看命令
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看服务器状态",
	Long:  "查看 AgentWiki 服务器的运行状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := client.GetStatus(ctx)
		if err != nil {
			return fmt.Errorf("获取状态失败: %w", err)
		}

		fmt.Println("AgentWiki 服务器状态:")
		fmt.Printf("  版本: %s\n", status.Version)
		fmt.Printf("  运行时间: %s\n", formatDuration(status.Uptime))
		fmt.Printf("  节点ID: %s\n", status.NodeID)
		fmt.Printf("  节点类型: %s\n", status.NodeType)
		fmt.Printf("  NAT类型: %s\n", status.NATType)
		fmt.Printf("  连接节点: %d\n", status.PeerCount)
		fmt.Printf("  条目数量: %d\n", status.EntryCount)
		fmt.Printf("  用户数量: %d\n", status.UserCount)

		return nil
	},
}

// uptimeCmd 查看运行时间
var uptimeCmd = &cobra.Command{
	Use:   "uptime",
	Short: "查看服务器运行时间",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := client.GetStatus(ctx)
		if err != nil {
			return fmt.Errorf("获取状态失败: %w", err)
		}

		fmt.Printf("运行时间: %s\n", formatDuration(status.Uptime))
		return nil
	},
}

// versionCmd 查看版本
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "查看版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("awctl 版本: %s\n", version)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		status, err := client.GetStatus(ctx)
		if err != nil {
			fmt.Println("服务器: 未连接")
			return
		}

		fmt.Printf("服务器版本: %s\n", status.Version)
	},
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d 秒", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%d 分钟", seconds/60)
	} else if seconds < 86400 {
		return fmt.Sprintf("%d 小时 %d 分钟", seconds/3600, (seconds%3600)/60)
	}
	return fmt.Sprintf("%d 天 %d 小时", seconds/86400, (seconds%86400)/3600)
}

func init() {
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(uptimeCmd)
	rootCmd.AddCommand(versionCmd)
}
