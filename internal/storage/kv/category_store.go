package kv

import (
	"fmt"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// CategoryStore 提供分类的CRUD操作
type CategoryStore struct {
	store Store
}

// NewCategoryStore 创建一个新的分类存储实例
func NewCategoryStore(store Store) *CategoryStore {
	return &CategoryStore{store: store}
}

// CreateCategory 创建一个新分类
func (cs *CategoryStore) CreateCategory(cat *model.Category) error {
	if cat.Path == "" {
		return fmt.Errorf("category path must not be empty")
	}

	key := []byte(PrefixCategory + cat.Path)

	// 检查是否已存在
	_, err := cs.store.Get(key)
	if err == nil {
		return fmt.Errorf("category with path %s already exists", cat.Path)
	}

	// 设置创建时间
	if cat.CreatedAt == 0 {
		cat.CreatedAt = time.Now().Unix()
	}

	// 生成ID
	if cat.ID == "" {
		cat.ID = cat.Path
	}

	data, err := cat.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize category: %w", err)
	}

	return cs.store.Put(key, data)
}

// GetCategory 根据路径获取分类
func (cs *CategoryStore) GetCategory(path string) (*model.Category, error) {
	key := []byte(PrefixCategory + path)

	data, err := cs.store.Get(key)
	if err != nil {
		if err == ErrKeyNotFound {
			return nil, fmt.Errorf("category %s not found", path)
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	cat := &model.Category{}
	if err := cat.FromJSON(data); err != nil {
		return nil, fmt.Errorf("failed to deserialize category: %w", err)
	}

	return cat, nil
}

// GetChildren 获取指定父路径下的所有子分类
func (cs *CategoryStore) GetChildren(parentPath string) ([]*model.Category, error) {
	categories, err := cs.ListCategories()
	if err != nil {
		return nil, err
	}

	var children []*model.Category
	for _, cat := range categories {
		// 检查是否是直接子分类
		if cat.ParentId == parentPath {
			children = append(children, cat)
		}
	}

	return children, nil
}

// ListCategories 列出所有分类
func (cs *CategoryStore) ListCategories() ([]*model.Category, error) {
	categories, err := ScanAndParse(cs.store, PrefixCategory, func(data []byte) (*model.Category, error) {
		cat := &model.Category{}
		if err := cat.FromJSON(data); err != nil {
			return nil, err
		}
		return cat, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}

	return categories, nil
}

// InitBuiltinCategories 初始化内置分类树
// 创建默认的层级分类结构
func (cs *CategoryStore) InitBuiltinCategories() error {
	// 定义内置分类树
	builtinCategories := []struct {
		path      string
		name      string
		parentId  string
		level     int32
		sortOrder int32
	}{
		// 顶级分类
		{"tech", "技术", "", 0, 1},
		{"science", "科学", "", 0, 2},
		{"humanities", "人文", "", 0, 3},
		{"social", "社会科学", "", 0, 4},
		{"arts", "艺术", "", 0, 5},

		// 技术子分类
		{"tech/programming", "编程", "tech", 1, 1},
		{"tech/network", "网络", "tech", 1, 2},
		{"tech/security", "安全", "tech", 1, 3},
		{"tech/ai", "人工智能", "tech", 1, 4},
		{"tech/database", "数据库", "tech", 1, 5},
		{"tech/devops", "运维", "tech", 1, 6},

		// 科学子分类
		{"science/physics", "物理学", "science", 1, 1},
		{"science/chemistry", "化学", "science", 1, 2},
		{"science/biology", "生物学", "science", 1, 3},
		{"science/math", "数学", "science", 1, 4},
		{"science/astronomy", "天文学", "science", 1, 5},

		// 人文子分类
		{"humanities/history", "历史", "humanities", 1, 1},
		{"humanities/philosophy", "哲学", "humanities", 1, 2},
		{"humanities/literature", "文学", "humanities", 1, 3},
		{"humanities/linguistics", "语言学", "humanities", 1, 4},

		// 社会科学子分类
		{"social/economics", "经济学", "social", 1, 1},
		{"social/psychology", "心理学", "social", 1, 2},
		{"social/sociology", "社会学", "social", 1, 3},
		{"social/politics", "政治学", "social", 1, 4},
		{"social/law", "法学", "social", 1, 5},

		// 艺术子分类
		{"arts/music", "音乐", "arts", 1, 1},
		{"arts/painting", "绘画", "arts", 1, 2},
		{"arts/film", "电影", "arts", 1, 3},
		{"arts/architecture", "建筑", "arts", 1, 4},
	}

	now := time.Now().Unix()

	for _, bc := range builtinCategories {
		cat := &model.Category{
			ID:          bc.path,
			Path:        bc.path,
			Name:        bc.name,
			ParentId:    bc.parentId,
			Level:       bc.level,
			SortOrder:   bc.sortOrder,
			IsBuiltin:   true,
			MaintainedBy: "",
			CreatedAt:   now,
		}

		// 如果分类已存在则跳过
		_, err := cs.GetCategory(bc.path)
		if err == nil {
			continue
		}

		if err := cs.CreateCategory(cat); err != nil {
			return fmt.Errorf("failed to create builtin category %s: %w", bc.path, err)
		}
	}

	return nil
}
