package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/daifei0527/polyant/pkg/config"
	"github.com/spf13/cobra"
)

// categoryCmd 分类管理命令
var categoryCmd = &cobra.Command{
	Use:     "category",
	Aliases: []string{"cat"},
	Short:   "分类管理",
	Long:    "管理知识分类体系",
}

// categoryListCmd 列出分类
var categoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有分类",
	RunE: func(cmd *cobra.Command, args []string) error {
		tree, _ := cmd.Flags().GetBool("tree")
		jsonOut, _ := cmd.Flags().GetBool("json")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		categories, err := client.ListCategories(ctx)
		if err != nil {
			// 如果 API 不可用，显示默认分类
			return showDefaultCategories(tree, jsonOut)
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(categories)
		}

		if tree {
			return showCategoriesTree(categories)
		}

		fmt.Printf("分类列表 (%d 个):\n\n", len(categories))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\t名称\t描述")
		fmt.Fprintln(w, "--\t----\t----")

		for _, c := range categories {
			desc := c.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", c.ID, c.Name, desc)
		}
		w.Flush()

		return nil
	},
}

// categoryGetCmd 获取分类详情
var categoryGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "获取分类详情",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		categories, err := client.ListCategories(ctx)
		if err != nil {
			return fmt.Errorf("获取分类失败: %w", err)
		}

		for _, c := range categories {
			if c.ID == id {
				fmt.Println("分类详情:")
				fmt.Printf("  ID: %s\n", c.ID)
				fmt.Printf("  名称: %s\n", c.Name)
				if c.Description != "" {
					fmt.Printf("  描述: %s\n", c.Description)
				}
				if c.ParentID != "" {
					fmt.Printf("  父分类: %s\n", c.ParentID)
				}
				fmt.Printf("  创建时间: %s\n", time.UnixMilli(c.CreatedAt).Format("2006-01-02"))
				return nil
			}
		}

		return fmt.Errorf("分类不存在: %s", id)
	},
}

// categoryCreateCmd 创建分类
var categoryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建新分类",
	Long: `创建新的知识分类。

需要 Lv2 或更高等级的用户认证。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		path, _ := cmd.Flags().GetString("path")
		parent, _ := cmd.Flags().GetString("parent")

		if name == "" {
			return fmt.Errorf("必须指定 --name")
		}
		if path == "" {
			return fmt.Errorf("必须指定 --path")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cat, err := client.CreateCategory(ctx, path, name, parent)
		if err != nil {
			return fmt.Errorf("创建分类失败: %w", err)
		}

		fmt.Println("分类创建成功!")
		fmt.Printf("  ID: %s\n", cat.ID)
		fmt.Printf("  名称: %s\n", cat.Name)
		if cat.ParentID != "" {
			fmt.Printf("  父分类: %s\n", cat.ParentID)
		}
		fmt.Printf("  创建时间: %s\n", time.UnixMilli(cat.CreatedAt).Format("2006-01-02 15:04:05"))

		return nil
	},
}

// categoryEntriesCmd 列出分类下的条目
var categoryEntriesCmd = &cobra.Command{
	Use:   "entries <path>",
	Short: "列出分类下的条目",
	Args:  cobra.ExactArgs(1),
	Long:  `获取指定分类路径下的所有条目。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		entries, total, err := client.GetCategoryEntries(ctx, path, limit, offset)
		if err != nil {
			return fmt.Errorf("获取条目失败: %w", err)
		}

		if len(entries) == 0 {
			fmt.Printf("分类 %s 下暂无条目\n", path)
			return nil
		}

		fmt.Printf("分类 %s 下的条目 (共 %d 条):\n\n", path, total)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\t标题\t评分\t创建时间")
		fmt.Fprintln(w, "--\t----\t----\t--------")

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
			fmt.Fprintf(w, "%s\t%s\t%.1f\t%s\n", id, title, e.Score, createdAt)
		}
		w.Flush()

		return nil
	},
}

func showDefaultCategories(tree, jsonOut bool) error {
	if jsonOut {
		data := map[string]interface{}{
			"categories": []string{
				"tech",
				"tech/programming",
				"tech/ai",
				"science",
				"business",
				"life",
				"education",
				"art",
				"tools",
				"other",
			},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	if tree {
		fmt.Println("📁 技术领域 (tech)")
		fmt.Println("  💻 编程开发 (tech/programming)")
		fmt.Println("  🤖 人工智能 (tech/ai)")
		fmt.Println("  🗄️ 数据库 (tech/database)")
		fmt.Println("📁 科学研究 (science)")
		fmt.Println("  📐 数学 (science/math)")
		fmt.Println("  ⚛️ 物理学 (science/physics)")
		fmt.Println("📁 商业经济 (business)")
		fmt.Println("📁 日常生活 (life)")
		fmt.Println("📁 教育学习 (education)")
		fmt.Println("📁 艺术创意 (art)")
		fmt.Println("📁 工具资源 (tools)")
		fmt.Println("📁 其他 (other)")
	} else {
		fmt.Println("分类列表:")
		fmt.Println("  tech         - 技术领域")
		fmt.Println("  science      - 科学研究")
		fmt.Println("  business     - 商业经济")
		fmt.Println("  life         - 日常生活")
		fmt.Println("  education    - 教育学习")
		fmt.Println("  art          - 艺术创意")
		fmt.Println("  tools        - 工具资源")
		fmt.Println("  other        - 其他")
		fmt.Println("\n提示: 使用 --tree 查看树形结构")
	}

	return nil
}

func showCategoriesTree(categories []CategoryInfo) error {
	// 构建树形结构
	rootCategories := make(map[string][]CategoryInfo)
	for _, c := range categories {
		parent := c.ParentID
		if parent == "" {
			parent = "root"
		}
		rootCategories[parent] = append(rootCategories[parent], c)
	}

	// 打印根分类
	roots := rootCategories["root"]
	for _, r := range roots {
		fmt.Printf("📁 %s (%s)\n", r.Name, r.ID)
		children := rootCategories[r.ID]
		for _, c := range children {
			fmt.Printf("  📄 %s (%s)\n", c.Name, c.ID)
		}
	}

	return nil
}

// configCmd 配置管理命令
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
	Long:  "查看和修改配置",
}

// configShowCmd 显示配置
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "显示当前配置",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig("")
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}

		fmt.Println("当前配置:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		printConfigField(w, "node.type", cfg.Node.Type)
		printConfigField(w, "node.name", cfg.Node.Name)
		printConfigField(w, "node.data_dir", cfg.Node.DataDir)
		printConfigField(w, "node.log_level", cfg.Node.LogLevel)
		printConfigField(w, "network.api_port", fmt.Sprintf("%d", cfg.Network.APIPort))
		printConfigField(w, "network.listen_port", fmt.Sprintf("%d", cfg.Network.ListenPort))
		printConfigField(w, "network.dht_enabled", fmt.Sprintf("%v", cfg.Network.DHTEnabled))
		printConfigField(w, "network.mdns_enabled", fmt.Sprintf("%v", cfg.Network.MDNSEnabled))
		printConfigField(w, "sync.auto_sync", fmt.Sprintf("%v", cfg.Sync.AutoSync))
		printConfigField(w, "sync.interval_seconds", fmt.Sprintf("%d", cfg.Sync.IntervalSeconds))
		w.Flush()

		return nil
	},
}

func printConfigField(w *tabwriter.Writer, key, value string) {
	fmt.Fprintf(w, "  %s\t%s\n", key, value)
}

// configSetCmd 设置配置
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "设置配置项",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]
		fmt.Printf("设置 %s = %s\n", key, value)
		fmt.Println("注意: 配置修改功能尚未实现，请直接编辑配置文件")
		return nil
	},
}

// configGetCmd 获取配置
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "获取配置项",
	Long: `获取指定配置项的值。

支持的配置键:
  node.type          - 节点类型 (local/seed)
  node.name          - 节点名称
  node.data_dir      - 数据目录
  node.log_level     - 日志级别
  network.api_port   - API端口
  network.listen_port - P2P监听端口
  network.dht_enabled - 是否启用DHT
  network.mdns_enabled - 是否启用mDNS
  sync.auto_sync     - 是否自动同步
  sync.interval_seconds - 同步间隔`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		cfg, err := loadConfig("")
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}

		value, err := getConfigValue(cfg, key)
		if err != nil {
			return err
		}

		fmt.Printf("%s = %v\n", key, value)
		return nil
	},
}

// getConfigValue 从配置对象获取指定键的值
func getConfigValue(cfg *config.Config, key string) (interface{}, error) {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("无效的配置键格式，应为 'section.field'")
	}

	section := parts[0]
	field := parts[1]

	var sectionValue reflect.Value
	switch section {
	case "node":
		sectionValue = reflect.ValueOf(cfg.Node)
	case "network":
		sectionValue = reflect.ValueOf(cfg.Network)
	case "sync":
		sectionValue = reflect.ValueOf(cfg.Sync)
	case "sharing":
		sectionValue = reflect.ValueOf(cfg.Sharing)
	case "account":
		sectionValue = reflect.ValueOf(cfg.Account)
	case "seed":
		sectionValue = reflect.ValueOf(cfg.Seed)
	case "user_node", "user":
		sectionValue = reflect.ValueOf(cfg.User)
	case "mirror":
		sectionValue = reflect.ValueOf(cfg.Mirror)
	case "smtp":
		sectionValue = reflect.ValueOf(cfg.SMTP)
	case "api":
		sectionValue = reflect.ValueOf(cfg.API)
	case "storage":
		sectionValue = reflect.ValueOf(cfg.Storage)
	case "i18n":
		sectionValue = reflect.ValueOf(cfg.I18n)
	default:
		return nil, fmt.Errorf("未知的配置节: %s", section)
	}

	// 获取字段值
	fieldValue := sectionValue.FieldByNameFunc(func(name string) bool {
		return strings.EqualFold(name, field) || strings.EqualFold(toSnakeCase(name), field)
	})

	if !fieldValue.IsValid() {
		return nil, fmt.Errorf("未知的配置字段: %s.%s", section, field)
	}

	return fieldValue.Interface(), nil
}

func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

func init() {
	// 分类命令
	rootCmd.AddCommand(categoryCmd)
	categoryCmd.AddCommand(categoryListCmd)
	categoryListCmd.Flags().Bool("tree", false, "树形显示")
	categoryListCmd.Flags().Bool("json", false, "JSON格式输出")
	categoryCmd.AddCommand(categoryGetCmd)
	categoryCmd.AddCommand(categoryCreateCmd)
	categoryCreateCmd.Flags().String("name", "", "分类名称")
	categoryCreateCmd.Flags().String("path", "", "分类路径 (如: tech/ai)")
	categoryCreateCmd.Flags().String("parent", "", "父分类ID")

	// entries 子命令
	categoryCmd.AddCommand(categoryEntriesCmd)
	categoryEntriesCmd.Flags().IntP("limit", "l", 20, "结果数量限制")
	categoryEntriesCmd.Flags().Int("offset", 0, "结果偏移量")

	// 配置命令
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}
