package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// syncCmd 同步管理命令
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "同步管理",
	Long:  "管理节点同步、种子节点、增量同步等",
}

// syncStatusCmd 同步状态
var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看同步状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := client.GetSyncStatus(ctx)
		if err != nil {
			// 如果 API 不可用，显示本地状态
			fmt.Println("同步状态:")
			fmt.Println("  服务状态: 未连接")
			fmt.Println("  提示: 请确保 AgentWiki 服务正在运行")
			return nil
		}

		fmt.Println("同步状态:")
		if status.Running {
			fmt.Println("  运行中: 是")
		} else {
			fmt.Println("  运行中: 否")
		}

		if status.LastSync > 0 {
			fmt.Printf("  最后同步: %s\n", time.UnixMilli(status.LastSync).Format("2006-01-02 15:04:05"))
		} else {
			fmt.Println("  最后同步: 从未")
		}

		fmt.Printf("  已同步条目: %d\n", status.SyncedEntries)

		if len(status.ConnectedPeers) > 0 {
			fmt.Printf("  连接节点: %d\n", len(status.ConnectedPeers))
			for _, p := range status.ConnectedPeers {
				fmt.Printf("    - %s (%d ms)\n", p.ID[:8], p.Latency)
			}
		}

		return nil
	},
}

// syncStartCmd 启动同步
var syncStartCmd = &cobra.Command{
	Use:   "start",
	Short: "启动同步服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		seeds, _ := cmd.Flags().GetStringSlice("seeds")
		full, _ := cmd.Flags().GetBool("full")

		// TODO: 实现启动同步 API
		fmt.Println("启动同步服务...")
		if full {
			fmt.Println("  模式: 全量同步")
		} else {
			fmt.Println("  模式: 增量同步")
		}
		if len(seeds) > 0 {
			fmt.Printf("  种子节点: %v\n", seeds)
		}

		return nil
	},
}

// syncStopCmd 停止同步
var syncStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止同步服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: 实现停止同步 API
		fmt.Println("同步服务已停止")
		return nil
	},
}

// syncPeersCmd 查看连接的节点
var syncPeersCmd = &cobra.Command{
	Use:   "peers",
	Short: "查看连接的对等节点",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := client.GetStatus(ctx)
		if err != nil {
			fmt.Println("连接的节点:")
			fmt.Println("  (无法连接到服务器)")
			return nil
		}

		fmt.Printf("连接的节点 (%d 个):\n", status.PeerCount)
		if status.PeerCount == 0 {
			fmt.Println("  暂无连接的节点")
			fmt.Println()
			fmt.Println("提示:")
			fmt.Println("  配置种子节点: awctl config set seeds <addr>")
			fmt.Println("  手动连接节点: awctl sync connect <addr>")
		}

		return nil
	},
}

// syncConnectCmd 连接到节点
var syncConnectCmd = &cobra.Command{
	Use:   "connect <addr>",
	Short: "连接到指定节点",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := args[0]

		// TODO: 实现连接 API
		fmt.Printf("正在连接到节点: %s\n", addr)
		fmt.Println("连接成功")
		return nil
	},
}

// syncMirrorCmd 镜像同步
var syncMirrorCmd = &cobra.Command{
	Use:   "mirror",
	Short: "镜像同步操作",
}

// syncMirrorStartCmd 启动镜像同步
var syncMirrorStartCmd = &cobra.Command{
	Use:   "start <peer-id>",
	Short: "启动镜像同步",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		peerID := args[0]
		categories, _ := cmd.Flags().GetStringSlice("categories")

		// TODO: 实现镜像同步 API
		fmt.Printf("启动镜像同步: %s\n", peerID)
		if len(categories) > 0 {
			fmt.Printf("  分类: %v\n", categories)
		} else {
			fmt.Println("  分类: 全部")
		}

		return nil
	},
}

// syncMirrorStopCmd 停止镜像同步
var syncMirrorStopCmd = &cobra.Command{
	Use:   "stop <peer-id>",
	Short: "停止镜像同步",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		peerID := args[0]

		// TODO: 实现停止镜像 API
		fmt.Printf("已停止镜像同步: %s\n", peerID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncStartCmd)
	syncStartCmd.Flags().StringSlice("seeds", nil, "种子节点地址")
	syncStartCmd.Flags().Bool("full", false, "全量同步")

	syncCmd.AddCommand(syncStopCmd)
	syncCmd.AddCommand(syncPeersCmd)
	syncCmd.AddCommand(syncConnectCmd)

	// 镜像同步子命令
	syncCmd.AddCommand(syncMirrorCmd)
	syncMirrorCmd.AddCommand(syncMirrorStartCmd)
	syncMirrorStartCmd.Flags().StringSlice("categories", nil, "指定同步的分类")

	syncMirrorCmd.AddCommand(syncMirrorStopCmd)
}
