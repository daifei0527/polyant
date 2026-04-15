package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// adminStatsCmd 统计命令组
var adminStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "数据统计",
}

// adminStatsUsersCmd 用户统计
var adminStatsUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "用户统计",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		stats, err := client.GetUserStats(ctx)
		if err != nil {
			return fmt.Errorf("获取用户统计失败: %w", err)
		}

		fmt.Println("用户统计:")
		fmt.Printf("  总用户数: %d\n", stats.Total)
		fmt.Println("  等级分布:")
		for _, l := range stats.LevelDistribution {
			fmt.Printf("    Lv%d: %d\n", l.Level, l.Count)
		}
		return nil
	},
}

// adminStatsActivityCmd 活跃趋势
var adminStatsActivityCmd = &cobra.Command{
	Use:   "activity",
	Short: "活跃趋势",
	RunE: func(cmd *cobra.Command, args []string) error {
		days, _ := cmd.Flags().GetInt("days")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		trend, err := client.GetActivityTrend(ctx, days)
		if err != nil {
			return fmt.Errorf("获取活跃趋势失败: %w", err)
		}

		fmt.Printf("活跃趋势 (近 %d 天):\n", days)
		for _, d := range trend {
			fmt.Printf("  %s: %d 活跃用户\n", d.Date, d.ActiveUsers)
		}
		return nil
	},
}

func init() {
	adminCmd.AddCommand(adminStatsCmd)
	adminStatsCmd.AddCommand(adminStatsUsersCmd)
	adminStatsCmd.AddCommand(adminStatsActivityCmd)
	adminStatsActivityCmd.Flags().Int("days", 7, "统计天数")
}
