package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// entryCmd 条目管理命令
var entryCmd = &cobra.Command{
	Use:   "entry",
	Short: "知识条目管理",
	Long:  "管理知识库中的条目，包括创建、查询、更新、删除等操作",
}

// entryListCmd 列出条目
var entryListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出知识条目",
	RunE: func(cmd *cobra.Command, args []string) error {
		category, _ := cmd.Flags().GetString("category")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		jsonOut, _ := cmd.Flags().GetBool("json")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		entries, total, err := client.ListEntries(ctx, category, limit, offset)
		if err != nil {
			return fmt.Errorf("获取列表失败: %w", err)
		}

		if jsonOut {
			data := map[string]interface{}{
				"entries": entries,
				"total":   total,
				"limit":   limit,
				"offset":  offset,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(data)
		}

		if len(entries) == 0 {
			fmt.Println("暂无条目")
			return nil
		}

		fmt.Printf("条目列表 (共 %d 条):\n\n", total)

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

// entryGetCmd 获取条目详情
var entryGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "获取条目详情",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		jsonOut, _ := cmd.Flags().GetBool("json")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entry, err := client.GetEntry(ctx, id)
		if err != nil {
			return fmt.Errorf("获取条目失败: %w", err)
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(entry)
		}

		fmt.Println("条目详情:")
		fmt.Printf("  ID: %s\n", entry.ID)
		fmt.Printf("  标题: %s\n", entry.Title)
		fmt.Printf("  分类: %s\n", entry.Category)
		if len(entry.Tags) > 0 {
			fmt.Printf("  标签: %v\n", entry.Tags)
		}
		fmt.Printf("  评分: %.2f (%d 人评分)\n", entry.Score, entry.ScoreCount)
		fmt.Printf("  创建时间: %s\n", time.UnixMilli(entry.CreatedAt).Format("2006-01-02 15:04:05"))
		fmt.Printf("  更新时间: %s\n", time.UnixMilli(entry.UpdatedAt).Format("2006-01-02 15:04:05"))
		fmt.Printf("  创建者: %s\n", entry.CreatedBy)

		return nil
	},
}

// entryCreateCmd 创建条目
var entryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建新条目",
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		category, _ := cmd.Flags().GetString("category")
		tags, _ := cmd.Flags().GetStringSlice("tags")

		if title == "" {
			return fmt.Errorf("标题不能为空")
		}

		// TODO: 实现实际的创建 API 调用
		fmt.Printf("创建条目:\n")
		fmt.Printf("  标题: %s\n", title)
		fmt.Printf("  分类: %s\n", category)
		fmt.Printf("  标签: %v\n", tags)
		fmt.Printf("  内容长度: %d 字符\n", len(content))
		fmt.Println("\n注意: 需要认证才能创建条目")

		return nil
	},
}

// entryUpdateCmd 更新条目
var entryUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "更新条目",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")

		// TODO: 实现实际的更新 API 调用
		fmt.Printf("更新条目: %s\n", id)
		if title != "" {
			fmt.Printf("  新标题: %s\n", title)
		}
		if content != "" {
			fmt.Printf("  内容已更新\n")
		}
		fmt.Println("\n注意: 需要认证才能更新条目")

		return nil
	},
}

// entryDeleteCmd 删除条目
var entryDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "删除条目",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("确定要删除条目 %s 吗? (y/n): ", id)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("已取消")
				return nil
			}
		}

		// TODO: 实现实际的删除 API 调用
		fmt.Printf("已删除条目: %s\n", id)
		fmt.Println("注意: 需要认证才能删除条目")
		return nil
	},
}

// entrySearchCmd 搜索条目
var entrySearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "搜索知识条目",
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
			fmt.Println("未找到匹配的条目")
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
	// entry 命令
	rootCmd.AddCommand(entryCmd)

	// list 子命令
	entryCmd.AddCommand(entryListCmd)
	entryListCmd.Flags().StringP("category", "c", "", "按分类过滤")
	entryListCmd.Flags().IntP("limit", "l", 20, "结果数量限制")
	entryListCmd.Flags().Int("offset", 0, "结果偏移量")
	entryListCmd.Flags().Bool("json", false, "JSON格式输出")

	// get 子命令
	entryCmd.AddCommand(entryGetCmd)
	entryGetCmd.Flags().Bool("json", false, "JSON格式输出")

	// create 子命令
	entryCmd.AddCommand(entryCreateCmd)
	entryCreateCmd.Flags().StringP("title", "t", "", "条目标题")
	entryCreateCmd.Flags().StringP("content", "C", "", "条目内容")
	entryCreateCmd.Flags().StringP("category", "c", "other", "分类ID")
	entryCreateCmd.Flags().StringSlice("tags", nil, "标签（逗号分隔）")

	// update 子命令
	entryCmd.AddCommand(entryUpdateCmd)
	entryUpdateCmd.Flags().StringP("title", "t", "", "新标题")
	entryUpdateCmd.Flags().StringP("content", "C", "", "新内容")

	// delete 子命令
	entryCmd.AddCommand(entryDeleteCmd)
	entryDeleteCmd.Flags().BoolP("force", "f", false, "强制删除不确认")

	// search 子命令
	entryCmd.AddCommand(entrySearchCmd)
	entrySearchCmd.Flags().StringP("category", "c", "", "按分类过滤")
	entrySearchCmd.Flags().IntP("limit", "l", 20, "结果数量限制")
}
