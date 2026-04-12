package category

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// CategoryInitializer 分类初始化器
type CategoryInitializer struct {
	store       storage.CategoryStore
	seedDataDir string
	mu          sync.Mutex
	initDone    bool
}

// NewCategoryInitializer 创建分类初始化器
func NewCategoryInitializer(store storage.CategoryStore, seedDataDir string) *CategoryInitializer {
	if seedDataDir == "" {
		seedDataDir = "./configs"
	}
	return &CategoryInitializer{
		store:       store,
		seedDataDir: seedDataDir,
	}
}

// SeedData 种子分类数据结构
type SeedData struct {
	Categories []SeedCategory `json:"categories"`
}

// SeedCategory 种子分类结构
type SeedCategory struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	ParentID  string `json:"parent_id"`
	Level     int32  `json:"level"`
	SortOrder int32  `json:"sort_order"`
	IsBuiltin bool   `json:"is_builtin"`
}

// Initialize 初始化分类（如果尚未初始化）
func (ci *CategoryInitializer) Initialize(ctx context.Context) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if ci.initDone {
		return nil
	}

	// 检查是否已有分类
	existing, err := ci.store.ListAll(ctx)
	if err == nil && len(existing) > 0 {
		ci.initDone = true
		log.Printf("[CategoryInitializer] Categories already exist: %d", len(existing))
		return nil
	}

	// 加载种子分类
	seedData, err := ci.loadSeedData()
	if err != nil {
		log.Printf("[CategoryInitializer] Failed to load seed data: %v, using defaults", err)
		seedData = ci.getDefaultSeedData()
	}

	// 创建分类
	created := 0
	for _, seed := range seedData.Categories {
		cat := &model.Category{
			ID:          seed.ID,
			Path:        seed.Path,
			Name:        seed.Name,
			ParentId:    seed.ParentID,
			Level:       seed.Level,
			SortOrder:   seed.SortOrder,
			IsBuiltin:   seed.IsBuiltin,
			MaintainedBy: "",
			CreatedAt:   0,
		}

		_, err := ci.store.Create(ctx, cat)
		if err != nil {
			log.Printf("[CategoryInitializer] Failed to create category %s: %v", seed.Path, err)
			continue
		}
		created++
	}

	ci.initDone = true
	log.Printf("[CategoryInitializer] Initialized %d categories", created)
	return nil
}

// loadSeedData 从文件加载种子数据
func (ci *CategoryInitializer) loadSeedData() (*SeedData, error) {
	filePath := ci.seedDataDir + "/seed-categories.json"
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read seed file: %w", err)
	}

	var seedData SeedData
	if err := json.Unmarshal(data, &seedData); err != nil {
		return nil, fmt.Errorf("parse seed data: %w", err)
	}

	return &seedData, nil
}

// getDefaultSeedData 获取默认种子数据
func (ci *CategoryInitializer) getDefaultSeedData() *SeedData {
	categories := []SeedCategory{
		// 计算机科学
		{ID: "cat-001", Path: "computer-science", Name: "计算机科学", ParentID: "", Level: 0, SortOrder: 1, IsBuiltin: true},
		{ID: "cat-002", Path: "computer-science/programming-languages", Name: "编程语言", ParentID: "cat-001", Level: 1, SortOrder: 1, IsBuiltin: true},
		{ID: "cat-003", Path: "computer-science/programming-languages/go", Name: "Go", ParentID: "cat-002", Level: 2, SortOrder: 1, IsBuiltin: true},
		{ID: "cat-004", Path: "computer-science/programming-languages/python", Name: "Python", ParentID: "cat-002", Level: 2, SortOrder: 2, IsBuiltin: true},
		{ID: "cat-005", Path: "computer-science/programming-languages/rust", Name: "Rust", ParentID: "cat-002", Level: 2, SortOrder: 3, IsBuiltin: true},
		{ID: "cat-006", Path: "computer-science/programming-languages/javascript", Name: "JavaScript", ParentID: "cat-002", Level: 2, SortOrder: 4, IsBuiltin: true},
		{ID: "cat-007", Path: "computer-science/algorithms", Name: "算法与数据结构", ParentID: "cat-001", Level: 1, SortOrder: 2, IsBuiltin: true},
		{ID: "cat-008", Path: "computer-science/operating-systems", Name: "操作系统", ParentID: "cat-001", Level: 1, SortOrder: 3, IsBuiltin: true},
		{ID: "cat-009", Path: "computer-science/network-protocols", Name: "网络协议", ParentID: "cat-001", Level: 1, SortOrder: 4, IsBuiltin: true},
		{ID: "cat-010", Path: "computer-science/databases", Name: "数据库", ParentID: "cat-001", Level: 1, SortOrder: 5, IsBuiltin: true},

		// 人工智能
		{ID: "cat-011", Path: "artificial-intelligence", Name: "人工智能", ParentID: "", Level: 0, SortOrder: 2, IsBuiltin: true},
		{ID: "cat-012", Path: "artificial-intelligence/machine-learning", Name: "机器学习", ParentID: "cat-011", Level: 1, SortOrder: 1, IsBuiltin: true},
		{ID: "cat-013", Path: "artificial-intelligence/nlp", Name: "自然语言处理", ParentID: "cat-011", Level: 1, SortOrder: 2, IsBuiltin: true},
		{ID: "cat-014", Path: "artificial-intelligence/computer-vision", Name: "计算机视觉", ParentID: "cat-011", Level: 1, SortOrder: 3, IsBuiltin: true},
		{ID: "cat-015", Path: "artificial-intelligence/prompt-engineering", Name: "提示工程", ParentID: "cat-011", Level: 1, SortOrder: 4, IsBuiltin: true},
		{ID: "cat-016", Path: "artificial-intelligence/llm", Name: "大语言模型", ParentID: "cat-011", Level: 1, SortOrder: 5, IsBuiltin: true},

		// 通用知识
		{ID: "cat-017", Path: "general-knowledge", Name: "通用知识", ParentID: "", Level: 0, SortOrder: 3, IsBuiltin: true},
		{ID: "cat-018", Path: "general-knowledge/mathematics", Name: "数学", ParentID: "cat-017", Level: 1, SortOrder: 1, IsBuiltin: true},
		{ID: "cat-019", Path: "general-knowledge/physics", Name: "物理", ParentID: "cat-017", Level: 1, SortOrder: 2, IsBuiltin: true},
		{ID: "cat-020", Path: "general-knowledge/history", Name: "历史", ParentID: "cat-017", Level: 1, SortOrder: 3, IsBuiltin: true},
		{ID: "cat-021", Path: "general-knowledge/languages", Name: "语言", ParentID: "cat-017", Level: 1, SortOrder: 4, IsBuiltin: true},

		// 生活技能
		{ID: "cat-022", Path: "life-skills", Name: "生活技能", ParentID: "", Level: 0, SortOrder: 4, IsBuiltin: true},
		{ID: "cat-023", Path: "life-skills/cooking", Name: "烹饪", ParentID: "cat-022", Level: 1, SortOrder: 1, IsBuiltin: true},
		{ID: "cat-024", Path: "life-skills/health", Name: "健康", ParentID: "cat-022", Level: 1, SortOrder: 2, IsBuiltin: true},
		{ID: "cat-025", Path: "life-skills/finance", Name: "财务", ParentID: "cat-022", Level: 1, SortOrder: 3, IsBuiltin: true},

		// 工具使用
		{ID: "cat-026", Path: "tools", Name: "工具使用", ParentID: "", Level: 0, SortOrder: 5, IsBuiltin: true},
		{ID: "cat-027", Path: "tools/dev-tools", Name: "开发工具", ParentID: "cat-026", Level: 1, SortOrder: 1, IsBuiltin: true},
		{ID: "cat-028", Path: "tools/office-software", Name: "办公软件", ParentID: "cat-026", Level: 1, SortOrder: 2, IsBuiltin: true},
		{ID: "cat-029", Path: "tools/system-administration", Name: "系统管理", ParentID: "cat-026", Level: 1, SortOrder: 3, IsBuiltin: true},
	}

	return &SeedData{Categories: categories}
}

// InitializeFromJSON 从外部 JSON 数据初始化分类
func (ci *CategoryInitializer) InitializeFromJSON(ctx context.Context, jsonData []byte) error {
	var seedData SeedData
	if err := json.Unmarshal(jsonData, &seedData); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	for _, seed := range seedData.Categories {
		// 检查是否已存在
		existing, _ := ci.store.Get(ctx, seed.Path)
		if existing != nil {
			continue
		}

		cat := &model.Category{
			ID:          seed.ID,
			Path:        seed.Path,
			Name:        seed.Name,
			ParentId:    seed.ParentID,
			Level:       seed.Level,
			SortOrder:   seed.SortOrder,
			IsBuiltin:   seed.IsBuiltin,
			CreatedAt:   0,
		}

		_, err := ci.store.Create(ctx, cat)
		if err != nil {
			log.Printf("[CategoryInitializer] Failed to create category %s: %v", seed.Path, err)
		}
	}

	return nil
}
