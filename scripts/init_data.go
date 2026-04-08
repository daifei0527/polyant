// init_data.go - AgentWiki 初始种子数据生成器
// 独立运行的 Go 程序，生成默认分类和示例知识条目
// 输出为 JSON 文件，可导入到 AgentWiki 节点中
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// 版本信息
var version = "v0.1.0-dev"

// KnowledgeEntry 知识条目数据结构
type KnowledgeEntry struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	JsonData    []map[string]interface{} `json:"json_data,omitempty"`
	Category    string                 `json:"category"`
	Tags        []string               `json:"tags,omitempty"`
	Version     int64                  `json:"version"`
	CreatedAt   int64                  `json:"created_at"`
	UpdatedAt   int64                  `json:"updated_at"`
	CreatedBy   string                 `json:"created_by"`
	Score       float64                `json:"score"`
	ScoreCount  int32                  `json:"score_count"`
	ContentHash string                 `json:"content_hash"`
	Status      int                    `json:"status"`
	License     string                 `json:"license"`
	SourceRef   string                 `json:"source_ref,omitempty"`
}

// Category 分类数据结构
type Category struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Name        string `json:"name"`
	ParentID    string `json:"parent_id,omitempty"`
	Level       int32  `json:"level"`
	SortOrder   int32  `json:"sort_order"`
	IsBuiltin   bool   `json:"is_builtin"`
	MaintainedBy string `json:"maintained_by,omitempty"`
	CreatedAt   int64  `json:"created_at"`
}

// SeedData 种子数据集合
type SeedData struct {
	Version    string            `json:"version"`
	GeneratedAt string           `json:"generated_at"`
	Categories []Category        `json:"categories"`
	Entries    []KnowledgeEntry  `json:"entries"`
	Checksum   string            `json:"checksum"`
}

func main() {
	// 解析命令行参数
	outputDir := "."
	if len(os.Args) > 1 {
		outputDir = os.Args[1]
	}

	fmt.Printf("AgentWiki 种子数据生成器 %s\n", version)
	fmt.Printf("输出目录: %s\n", outputDir)

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 创建输出目录失败: %v\n", err)
		os.Exit(1)
	}

	// 生成数据
	seedData := generateSeedData()

	// 计算校验和
	checksum := computeSeedChecksum(seedData)
	seedData.Checksum = checksum

	// 序列化为 JSON
	data, err := json.MarshalIndent(seedData, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 序列化数据失败: %v\n", err)
		os.Exit(1)
	}

	// 写入文件
	outputPath := filepath.Join(outputDir, "seed_data.json")
	if err := ioutil.WriteFile(outputPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 写入文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("种子数据已生成: %s\n", outputPath)
	fmt.Printf("  分类数量: %d\n", len(seedData.Categories))
	fmt.Printf("  条目数量: %d\n", len(seedData.Entries))
	fmt.Printf("  校验和:   %s\n", checksum)

	// 同时分别输出分类和条目文件
	categoriesData, _ := json.MarshalIndent(seedData.Categories, "", "  ")
	categoriesPath := filepath.Join(outputDir, "seed_categories.json")
	ioutil.WriteFile(categoriesPath, categoriesData, 0644)
	fmt.Printf("  分类文件: %s\n", categoriesPath)

	entriesData, _ := json.MarshalIndent(seedData.Entries, "", "  ")
	entriesPath := filepath.Join(outputDir, "seed_entries.json")
	ioutil.WriteFile(entriesPath, entriesData, 0644)
	fmt.Printf("  条目文件: %s\n", entriesPath)
}

// generateSeedData 生成完整的种子数据
// 包含默认分类体系和示例知识条目
func generateSeedData() *SeedData {
	now := time.Now().UnixMilli()

	data := &SeedData{
		Version:     version,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Categories:  generateCategories(now),
		Entries:     generateEntries(now),
	}

	return data
}

// generateCategories 生成默认分类体系
// 按照需求文档中定义的分类树创建
func generateCategories(now int64) []Category {
	return []Category{
		// 计算机科学
		{ID: "cs", Path: "computer-science", Name: "计算机科学", ParentID: "", Level: 0, SortOrder: 1, IsBuiltin: true, CreatedAt: now},
		{ID: "cs-pl", Path: "computer-science/programming-languages", Name: "编程语言", ParentID: "cs", Level: 1, SortOrder: 1, IsBuiltin: true, CreatedAt: now},
		{ID: "cs-pl-go", Path: "computer-science/programming-languages/go", Name: "Go", ParentID: "cs-pl", Level: 2, SortOrder: 1, IsBuiltin: true, CreatedAt: now},
		{ID: "cs-pl-py", Path: "computer-science/programming-languages/python", Name: "Python", ParentID: "cs-pl", Level: 2, SortOrder: 2, IsBuiltin: true, CreatedAt: now},
		{ID: "cs-pl-rust", Path: "computer-science/programming-languages/rust", Name: "Rust", ParentID: "cs-pl", Level: 2, SortOrder: 3, IsBuiltin: true, CreatedAt: now},
		{ID: "cs-pl-js", Path: "computer-science/programming-languages/javascript", Name: "JavaScript", ParentID: "cs-pl", Level: 2, SortOrder: 4, IsBuiltin: true, CreatedAt: now},
		{ID: "cs-algo", Path: "computer-science/algorithms", Name: "算法与数据结构", ParentID: "cs", Level: 1, SortOrder: 2, IsBuiltin: true, CreatedAt: now},
		{ID: "cs-os", Path: "computer-science/operating-systems", Name: "操作系统", ParentID: "cs", Level: 1, SortOrder: 3, IsBuiltin: true, CreatedAt: now},
		{ID: "cs-net", Path: "computer-science/network-protocols", Name: "网络协议", ParentID: "cs", Level: 1, SortOrder: 4, IsBuiltin: true, CreatedAt: now},
		{ID: "cs-db", Path: "computer-science/databases", Name: "数据库", ParentID: "cs", Level: 1, SortOrder: 5, IsBuiltin: true, CreatedAt: now},

		// 人工智能
		{ID: "ai", Path: "artificial-intelligence", Name: "人工智能", ParentID: "", Level: 0, SortOrder: 2, IsBuiltin: true, CreatedAt: now},
		{ID: "ai-ml", Path: "artificial-intelligence/machine-learning", Name: "机器学习", ParentID: "ai", Level: 1, SortOrder: 1, IsBuiltin: true, CreatedAt: now},
		{ID: "ai-nlp", Path: "artificial-intelligence/nlp", Name: "自然语言处理", ParentID: "ai", Level: 1, SortOrder: 2, IsBuiltin: true, CreatedAt: now},
		{ID: "ai-cv", Path: "artificial-intelligence/computer-vision", Name: "计算机视觉", ParentID: "ai", Level: 1, SortOrder: 3, IsBuiltin: true, CreatedAt: now},
		{ID: "ai-pe", Path: "artificial-intelligence/prompt-engineering", Name: "提示工程", ParentID: "ai", Level: 1, SortOrder: 4, IsBuiltin: true, CreatedAt: now},

		// 通用知识
		{ID: "gk", Path: "general-knowledge", Name: "通用知识", ParentID: "", Level: 0, SortOrder: 3, IsBuiltin: true, CreatedAt: now},
		{ID: "gk-math", Path: "general-knowledge/mathematics", Name: "数学", ParentID: "gk", Level: 1, SortOrder: 1, IsBuiltin: true, CreatedAt: now},
		{ID: "gk-phys", Path: "general-knowledge/physics", Name: "物理", ParentID: "gk", Level: 1, SortOrder: 2, IsBuiltin: true, CreatedAt: now},
		{ID: "gk-hist", Path: "general-knowledge/history", Name: "历史", ParentID: "gk", Level: 1, SortOrder: 3, IsBuiltin: true, CreatedAt: now},
		{ID: "gk-lang", Path: "general-knowledge/languages", Name: "语言", ParentID: "gk", Level: 1, SortOrder: 4, IsBuiltin: true, CreatedAt: now},

		// 生活技能
		{ID: "ls", Path: "life-skills", Name: "生活技能", ParentID: "", Level: 0, SortOrder: 4, IsBuiltin: true, CreatedAt: now},
		{ID: "ls-cook", Path: "life-skills/cooking", Name: "烹饪", ParentID: "ls", Level: 1, SortOrder: 1, IsBuiltin: true, CreatedAt: now},
		{ID: "ls-health", Path: "life-skills/health", Name: "健康", ParentID: "ls", Level: 1, SortOrder: 2, IsBuiltin: true, CreatedAt: now},
		{ID: "ls-fin", Path: "life-skills/finance", Name: "财务", ParentID: "ls", Level: 1, SortOrder: 3, IsBuiltin: true, CreatedAt: now},

		// 工具使用
		{ID: "tools", Path: "tools", Name: "工具使用", ParentID: "", Level: 0, SortOrder: 5, IsBuiltin: true, CreatedAt: now},
		{ID: "tools-dev", Path: "tools/dev-tools", Name: "开发工具", ParentID: "tools", Level: 1, SortOrder: 1, IsBuiltin: true, CreatedAt: now},
		{ID: "tools-office", Path: "tools/office-software", Name: "办公软件", ParentID: "tools", Level: 1, SortOrder: 2, IsBuiltin: true, CreatedAt: now},
		{ID: "tools-sysadmin", Path: "tools/system-administration", Name: "系统管理", ParentID: "tools", Level: 1, SortOrder: 3, IsBuiltin: true, CreatedAt: now},
	}
}

// generateEntries 生成示例知识条目
// 包含系统说明、核心知识条目和示例数据
func generateEntries(now int64) []KnowledgeEntry {
	entries := []KnowledgeEntry{
		// 系统说明条目
		{
			ID:        "sys-001",
			Title:     "AgentWiki 使用指南",
			Content:   "# AgentWiki 使用指南\n\nAgentWiki 是一个分布式百科知识库系统，为 AI 智能体提供知识和技能。\n\n## 快速开始\n\n1. 启动 AgentWiki 节点\n2. 通过 REST API 访问知识库\n3. 搜索、创建和管理知识条目\n\n## API 端点\n\n- `GET /api/v1/search` - 搜索知识\n- `GET /api/v1/entry/{id}` - 获取条目\n- `POST /api/v1/entry` - 创建条目\n- `GET /api/v1/categories` - 获取分类\n\n## 认证\n\n所有写操作需要 Ed25519 签名认证。\n",
			JsonData: []map[string]interface{}{
				{
					"type":        "skill_definition",
					"name":        "agentwiki_usage",
					"description": "AgentWiki 知识库使用指南",
				},
			},
			Category:    "tools/dev-tools",
			Tags:        []string{"agentwiki", "guide", "api", "getting-started"},
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			CreatedBy:   "system",
			Score:       5.0,
			ScoreCount:  1,
			ContentHash: "",
			Status:      0,
			License:     "CC BY-SA 4.0",
		},

		// Go 并发编程
		{
			ID:        "entry-go-001",
			Title:     "Go 并发编程指南",
			Content:   "# Go 并发编程指南\n\nGo 语言通过 goroutine 和 channel 提供了强大的并发编程能力。\n\n## Goroutine\n\nGoroutine 是 Go 的轻量级线程，由 Go 运行时管理。\n\n```go\nfunc sayHello() {\n    fmt.Println(\"Hello\")\n}\n\nfunc main() {\n    go sayHello()\n    time.Sleep(time.Second)\n}\n```\n\n## Channel\n\nChannel 是 goroutine 之间的通信机制。\n\n```go\nch := make(chan string)\ngo func() { ch <- \"hello\" }()\nmsg := <-ch\n```\n\n## Select\n\nSelect 允许等待多个 channel 操作。\n\n```go\nselect {\ncase msg := <-ch1:\n    fmt.Println(msg)\ncase msg := <-ch2:\n    fmt.Println(msg)\ncase <-time.After(time.Second):\n    fmt.Println(\"timeout\")\n}\n```\n",
			JsonData: []map[string]interface{}{
				{
					"type":        "skill_definition",
					"name":        "go_concurrent",
					"description": "Go 并发编程技能",
					"parameters": map[string]interface{}{
						"pattern": map[string]interface{}{
							"type":        "string",
							"required":    true,
							"description": "并发模式: goroutine, channel, select, mutex, waitgroup",
						},
					},
				},
			},
			Category:    "computer-science/programming-languages/go",
			Tags:        []string{"go", "concurrency", "goroutine", "channel", "programming"},
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			CreatedBy:   "system",
			Score:       4.5,
			ScoreCount:  2,
			ContentHash: "",
			Status:      0,
			License:     "CC BY-SA 4.0",
		},

		// 提示工程
		{
			ID:        "entry-pe-001",
			Title:     "提示工程基础",
			Content:   "# 提示工程基础\n\n提示工程（Prompt Engineering）是与大语言模型有效交互的关键技能。\n\n## 基本原则\n\n1. **清晰明确**: 避免模糊的指令\n2. **提供上下文**: 给出足够的背景信息\n3. **分步思考**: 复杂问题拆解为步骤\n4. **示例引导**: 通过示例说明期望的输出格式\n\n## 常用技巧\n\n### Few-Shot Prompting\n\n提供少量示例帮助模型理解任务。\n\n### Chain-of-Thought\n\n引导模型逐步推理。\n\n### Role Playing\n\n让模型扮演特定角色来获得更专业的回答。\n",
			JsonData: []map[string]interface{}{
				{
					"type":        "skill_definition",
					"name":        "prompt_engineering",
					"description": "提示工程基础技能",
				},
			},
			Category:    "artificial-intelligence/prompt-engineering",
			Tags:        []string{"prompt", "engineering", "llm", "ai", "chatgpt"},
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			CreatedBy:   "system",
			Score:       4.8,
			ScoreCount:  3,
			ContentHash: "",
			Status:      0,
			License:     "CC BY-SA 4.0",
		},

		// 数据结构
		{
			ID:        "entry-algo-001",
			Title:     "常用数据结构概览",
			Content:   "# 常用数据结构概览\n\n## 数组 (Array)\n\n连续内存存储，O(1) 随机访问。\n\n## 链表 (Linked List)\n\n非连续存储，O(1) 插入删除。\n\n## 栈 (Stack)\n\n后进先出 (LIFO) 结构。\n\n## 队列 (Queue)\n\n先进先出 (FIFO) 结构。\n\n## 哈希表 (Hash Table)\n\nO(1) 平均查找时间。\n\n## 二叉树 (Binary Tree)\n\n层次化数据结构，支持 O(log n) 查找。\n\n## 图 (Graph)\n\n节点和边的集合，用于表示复杂关系。\n",
			JsonData: []map[string]interface{}{
				{
					"type":        "skill_definition",
					"name":        "data_structures",
					"description": "常用数据结构参考",
				},
			},
			Category:    "computer-science/algorithms",
			Tags:        []string{"data-structure", "algorithm", "array", "linked-list", "hash-table", "tree", "graph"},
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			CreatedBy:   "system",
			Score:       4.2,
			ScoreCount:  1,
			ContentHash: "",
			Status:      0,
			License:     "CC BY-SA 4.0",
		},

		// Git 使用指南
		{
			ID:        "entry-tools-001",
			Title:     "Git 版本控制常用命令",
			Content:   "# Git 版本控制常用命令\n\n## 基础操作\n\n```bash\ngit init                  # 初始化仓库\ngit clone <url>           # 克隆仓库\ngit add .                 # 添加所有更改\ngit commit -m \"message\"   # 提交更改\ngit push                  # 推送到远程\ngit pull                  # 拉取远程更新\n```\n\n## 分支操作\n\n```bash\ngit branch <name>         # 创建分支\ngit checkout <name>       # 切换分支\ngit merge <name>          # 合并分支\ngit branch -d <name>      # 删除分支\n```\n\n## 查看历史\n\n```bash\ngit log --oneline         # 查看提交历史\ngit diff                  # 查看更改\ngit status                # 查看状态\n```\n",
			JsonData: []map[string]interface{}{
				{
					"type":        "skill_definition",
					"name":        "git_commands",
					"description": "Git 常用命令参考",
				},
			},
			Category:    "tools/dev-tools",
			Tags:        []string{"git", "version-control", "dev-tools", "commands"},
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			CreatedBy:   "system",
			Score:       4.0,
			ScoreCount:  1,
			ContentHash: "",
			Status:      0,
			License:     "CC BY-SA 4.0",
		},
	}

	// 计算每个条目的内容哈希
	for i := range entries {
		entries[i].ContentHash = computeHash(entries[i].Title, entries[i].Content, entries[i].Category)
	}

	return entries
}

// computeHash 计算内容哈希
func computeHash(title, content, category string) string {
	h := sha256.New()
	h.Write([]byte(title))
	h.Write([]byte(content))
	h.Write([]byte(category))
	return hex.EncodeToString(h.Sum(nil))
}

// computeSeedChecksum 计算种子数据的校验和
func computeSeedChecksum(data *SeedData) string {
	// 使用分类和条目的数量作为简单校验
	h := sha256.New()
	h.Write([]byte(data.Version))
	h.Write([]byte(data.GeneratedAt))
	h.Write([]byte(fmt.Sprintf("%d", len(data.Categories))))
	h.Write([]byte(fmt.Sprintf("%d", len(data.Entries))))
	return hex.EncodeToString(h.Sum(nil))
}
