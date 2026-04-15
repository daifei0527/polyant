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
			fmt.Println("  提示: 请确保 Polyant 服务正在运行")
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
	Short: "触发手动同步",
	Long: `触发一次手动同步操作。

需要认证才能执行此操作。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := client.TriggerSync(ctx); err != nil {
			return fmt.Errorf("触发同步失败: %w", err)
		}

		fmt.Println("同步已触发，正在后台执行...")
		fmt.Println("使用 'pactl sync status' 查看同步状态")
		return nil
	},
}

// syncStopCmd 停止同步
var syncStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止同步服务",
	Long: `停止正在进行的同步服务。

注意: 此功能需要服务端支持，目前服务端尚未实现。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("停止同步功能尚未在服务端实现")
		fmt.Println("当前版本可通过重启服务来停止同步")
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
			fmt.Println("  配置种子节点: pactl config set seeds <addr>")
			fmt.Println("  手动连接节点: pactl sync connect <addr>")
		}

		return nil
	},
}

// syncConnectCmd 连接到节点
var syncConnectCmd = &cobra.Command{
	Use:   "connect <addr>",
	Short: "连接到指定节点",
	Long: `手动连接到指定的 P2P 节点。

地址格式: /ip4/<ip>/tcp/<port>/p2p/<peer-id>
示例: /ip4/192.168.1.100/tcp/9000/p2p/12D3KooW...

注意: 此功能需要服务端支持。`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := args[0]

		fmt.Printf("连接地址: %s\n", addr)
		fmt.Println("注意: 手动连接功能尚未在服务端实现")
		fmt.Println("当前版本请通过配置文件设置种子节点")
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
	Long: `从指定节点启动镜像同步。

镜像同步会复制指定分类的所有条目到本地。

注意: 此功能需要服务端支持。`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		peerID := args[0]
		categories, _ := cmd.Flags().GetStringSlice("categories")

		fmt.Printf("镜像同步目标: %s\n", peerID)
		if len(categories) > 0 {
			fmt.Printf("  分类: %v\n", categories)
		} else {
			fmt.Println("  分类: 全部")
		}
		fmt.Println()
		fmt.Println("注意: 镜像同步功能尚未在服务端实现")

		return nil
	},
}

// syncMirrorStopCmd 停止镜像同步
var syncMirrorStopCmd = &cobra.Command{
	Use:   "stop <peer-id>",
	Short: "停止镜像同步",
	Long: `停止与指定节点的镜像同步。

注意: 此功能需要服务端支持。`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		peerID := args[0]

		fmt.Printf("停止镜像同步: %s\n", peerID)
		fmt.Println("注意: 此功能尚未在服务端实现")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncStartCmd)
	syncCmd.AddCommand(syncStopCmd)
	syncCmd.AddCommand(syncPeersCmd)
	syncCmd.AddCommand(syncConnectCmd)

	// 镜像同步子命令
	syncCmd.AddCommand(syncMirrorCmd)
	syncMirrorCmd.AddCommand(syncMirrorStartCmd)
	syncMirrorStartCmd.Flags().StringSlice("categories", nil, "指定同步的分类")

	syncMirrorCmd.AddCommand(syncMirrorStopCmd)
}
