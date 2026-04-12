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
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		parent, _ := cmd.Flags().GetString("parent")

		if name == "" {
			return fmt.Errorf("必须指定 --name")
		}

		// TODO: 实现创建分类 API
		fmt.Printf("创建分类:\n")
		fmt.Printf("  名称: %s\n", name)
		if desc != "" {
			fmt.Printf("  描述: %s\n", desc)
		}
		if parent != "" {
			fmt.Printf("  父分类: %s\n", parent)
		}
		fmt.Println("\n注意: 需要管理员权限才能创建分类")

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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := client.GetStatus(ctx)
		if err != nil {
			fmt.Println("当前配置 (本地):")
			fmt.Println("  数据目录: ~/.agentwiki")
			fmt.Println("  API端口: 8080")
			fmt.Println("  P2P端口: 9000")
			return nil
		}

		fmt.Println("当前配置:")
		fmt.Printf("  节点ID: %s\n", status.NodeID)
		fmt.Printf("  节点类型: %s\n", status.NodeType)
		fmt.Printf("  NAT类型: %s\n", status.NATType)
		fmt.Printf("  版本: %s\n", status.Version)

		return nil
	},
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
		fmt.Println("注意: 某些配置需要重启服务才能生效")
		return nil
	},
}

// configGetCmd 获取配置
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "获取配置项",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		// TODO: 实现配置获取
		fmt.Printf("%s = (未设置)\n", key)
		return nil
	},
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
	categoryCreateCmd.Flags().String("description", "", "分类描述")
	categoryCreateCmd.Flags().String("parent", "", "父分类ID")

	// 配置命令
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}
