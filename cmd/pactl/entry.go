package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/daifei0527/polyant/pkg/i18n"
	"github.com/spf13/cobra"
)

// getLang 获取当前语言设置
func getLang() i18n.Lang {
	if langFlag == "" {
		return i18n.LangZhCN
	}
	return i18n.Lang(langFlag)
}

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
			fmt.Println(i18n.Tc(getLang(), "cli.entry.no_result"))
			return nil
		}

		fmt.Printf("%s\n\n", i18n.Tc(getLang(), "cli.entry.list_title", map[string]interface{}{"count": total}))

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
	Long: `创建新的知识条目。

需要 Lv1 或更高等级的用户认证。
请确保已运行 'pactl key generate' 生成密钥并注册用户。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		category, _ := cmd.Flags().GetString("category")
		tags, _ := cmd.Flags().GetStringSlice("tags")

		if title == "" {
			return fmt.Errorf("标题不能为空，请使用 --title 指定")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := &CreateEntryRequest{
			Title:    title,
			Content:  content,
			Category: category,
			Tags:     tags,
		}

		entry, err := client.CreateEntry(ctx, req)
		if err != nil {
			return fmt.Errorf("创建条目失败: %w", err)
		}

		fmt.Println("条目创建成功!")
		fmt.Printf("  ID: %s\n", entry.ID)
		fmt.Printf("  标题: %s\n", entry.Title)
		fmt.Printf("  分类: %s\n", entry.Category)
		fmt.Printf("  创建时间: %s\n", time.UnixMilli(entry.CreatedAt).Format("2006-01-02 15:04:05"))

		return nil
	},
}

// entryUpdateCmd 更新条目
var entryUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "更新条目",
	Args:  cobra.ExactArgs(1),
	Long: `更新现有知识条目。

只有条目的创建者才能更新条目。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		category, _ := cmd.Flags().GetString("category")
		tags, _ := cmd.Flags().GetStringSlice("tags")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := &UpdateEntryRequest{
			Title:    title,
			Content:  content,
			Category: category,
			Tags:     tags,
		}

		entry, err := client.UpdateEntry(ctx, id, req)
		if err != nil {
			return fmt.Errorf("更新条目失败: %w", err)
		}

		fmt.Println("条目更新成功!")
		fmt.Printf("  ID: %s\n", entry.ID)
		fmt.Printf("  标题: %s\n", entry.Title)
		fmt.Printf("  更新时间: %s\n", time.UnixMilli(entry.UpdatedAt).Format("2006-01-02 15:04:05"))

		return nil
	},
}

// entryDeleteCmd 删除条目
var entryDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "删除条目",
	Args:  cobra.ExactArgs(1),
	Long: `删除知识条目。

只有条目的创建者才能删除条目。
删除操作不可恢复，请谨慎操作。`,
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

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := client.DeleteEntry(ctx, id); err != nil {
			return fmt.Errorf("删除条目失败: %w", err)
		}

		fmt.Printf("条目 %s 已删除\n", id)
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

// entryRateCmd 为条目评分
var entryRateCmd = &cobra.Command{
	Use:   "rate <id>",
	Short: "为条目评分",
	Args:  cobra.ExactArgs(1),
	Long: `为知识条目评分。

需要 Lv1 或更高等级的用户认证。
评分范围 0-5 分，可以为条目添加评论。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		score, _ := cmd.Flags().GetFloat64("score")
		comment, _ := cmd.Flags().GetString("comment")

		if score < 0 || score > 5 {
			return fmt.Errorf("评分必须在 0-5 之间")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := client.RateEntry(ctx, id, score, comment); err != nil {
			return fmt.Errorf("评分失败: %w", err)
		}

		fmt.Printf("已为条目 %s 评分: %.1f 分\n", id, score)
		if comment != "" {
			fmt.Printf("评论: %s\n", comment)
		}
		return nil
	},
}

// entryBacklinksCmd 获取反向链接
var entryBacklinksCmd = &cobra.Command{
	Use:   "backlinks <id>",
	Short: "获取条目的反向链接",
	Args:  cobra.ExactArgs(1),
	Long:  `获取引用了当前条目的其他条目列表。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		backlinks, err := client.GetBacklinks(ctx, id)
		if err != nil {
			return fmt.Errorf("获取反向链接失败: %w", err)
		}

		if len(backlinks) == 0 {
			fmt.Println("暂无反向链接")
			return nil
		}

		fmt.Printf("反向链接 (%d 条):\n", len(backlinks))
		for _, link := range backlinks {
			fmt.Printf("  - %s\n", link)
		}
		return nil
	},
}

// entryOutlinksCmd 获取正向链接
var entryOutlinksCmd = &cobra.Command{
	Use:   "outlinks <id>",
	Short: "获取条目的正向链接",
	Args:  cobra.ExactArgs(1),
	Long:  `获取当前条目引用的其他条目列表。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outlinks, err := client.GetOutlinks(ctx, id)
		if err != nil {
			return fmt.Errorf("获取正向链接失败: %w", err)
		}

		if len(outlinks) == 0 {
			fmt.Println("暂无正向链接")
			return nil
		}

		fmt.Printf("正向链接 (%d 条):\n", len(outlinks))
		for _, link := range outlinks {
			fmt.Printf("  - %s\n", link)
		}
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
	entryUpdateCmd.Flags().String("cat", "", "新分类")
	entryUpdateCmd.Flags().StringSlice("tags", nil, "新标签")

	// delete 子命令
	entryCmd.AddCommand(entryDeleteCmd)
	entryDeleteCmd.Flags().BoolP("force", "f", false, "强制删除不确认")

	// search 子命令
	entryCmd.AddCommand(entrySearchCmd)
	entrySearchCmd.Flags().StringP("category", "c", "", "按分类过滤")
	entrySearchCmd.Flags().IntP("limit", "l", 20, "结果数量限制")

	// rate 子命令
	entryCmd.AddCommand(entryRateCmd)
	entryRateCmd.Flags().Float64P("score", "s", 0, "评分 (0-5)")
	entryRateCmd.Flags().StringP("comment", "m", "", "评论内容")

	// backlinks 子命令
	entryCmd.AddCommand(entryBacklinksCmd)

	// outlinks 子命令
	entryCmd.AddCommand(entryOutlinksCmd)
}
