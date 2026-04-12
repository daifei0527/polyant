// Package category 提供知识分类管理功能
package category

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/daifei0527/agentwiki/internal/storage"
)

// Category 分类信息
type Category struct {
	// 分类ID（路径形式，如 "tech/programming"）
	ID string `json:"id"`
	// 分类名称
	Name string `json:"name"`
	// 分类描述
	Description string `json:"description"`
	// 父分类ID
	ParentID string `json:"parent_id,omitempty"`
	// 图标
	Icon string `json:"icon,omitempty"`
	// 条目数量
	EntryCount int `json:"entry_count"`
	// 子分类
	Children []*Category `json:"children,omitempty"`
}

// Manager 分类管理器
type Manager struct {
	mu      sync.RWMutex
	store   *storage.Store
	cached  map[string]*Category
	loaded  bool
}

// NewManager 创建分类管理器
func NewManager(store *storage.Store) *Manager {
	return &Manager{
		store:  store,
		cached: make(map[string]*Category),
	}
}

// GetInitialCategories 获取初始分类体系
func GetInitialCategories() []*Category {
	return []*Category{
		// 技术类
		{
			ID:          "tech",
			Name:        "技术",
			Description: "技术开发相关",
			Icon:        "💻",
			Children: []*Category{
				{ID: "tech/programming", Name: "编程开发", Description: "软件开发、编程语言", Icon: "👨‍💻"},
				{ID: "tech/ai", Name: "人工智能", Description: "AI、机器学习、深度学习", Icon: "🤖"},
				{ID: "tech/database", Name: "数据库", Description: "数据库技术、SQL、NoSQL", Icon: "🗄️"},
				{ID: "tech/web", Name: "Web开发", Description: "前端、后端、全栈", Icon: "🌐"},
				{ID: "tech/devops", Name: "DevOps", Description: "运维、容器、CI/CD", Icon: "⚙️"},
				{ID: "tech/security", Name: "网络安全", Description: "安全、加密、渗透测试", Icon: "🔐"},
				{ID: "tech/mobile", Name: "移动开发", Description: "iOS、Android、跨平台", Icon: "📱"},
			},
		},
		// 科学类
		{
			ID:          "science",
			Name:        "科学",
			Description: "科学研究与探索",
			Icon:        "🔬",
			Children: []*Category{
				{ID: "science/math", Name: "数学", Description: "数学理论、算法", Icon: "📐"},
				{ID: "science/physics", Name: "物理学", Description: "物理定律、实验", Icon: "⚛️"},
				{ID: "science/chemistry", Name: "化学", Description: "化学反应、材料", Icon: "🧪"},
				{ID: "science/biology", Name: "生物学", Description: "生命科学、基因", Icon: "🧬"},
				{ID: "science/astronomy", Name: "天文学", Description: "宇宙、星系、航天", Icon: "🌌"},
			},
		},
		// 商业类
		{
			ID:          "business",
			Name:        "商业",
			Description: "商业与经济",
			Icon:        "💼",
			Children: []*Category{
				{ID: "business/management", Name: "管理", Description: "企业管理、领导力", Icon: "📊"},
				{ID: "business/marketing", Name: "市场营销", Description: "营销策略、品牌", Icon: "📢"},
				{ID: "business/finance", Name: "金融", Description: "投资、财务、经济学", Icon: "💰"},
				{ID: "business/startup", Name: "创业", Description: "创业经验、商业模式", Icon: "🚀"},
				{ID: "business/ecommerce", Name: "电子商务", Description: "电商运营、跨境", Icon: "🛒"},
			},
		},
		// 生活类
		{
			ID:          "life",
			Name:        "生活",
			Description: "日常生活与技能",
			Icon:        "🏠",
			Children: []*Category{
				{ID: "life/health", Name: "健康", Description: "养生、运动、医疗", Icon: "🏃"},
				{ID: "life/food", Name: "美食", Description: "烹饪、食谱、饮食", Icon: "🍳"},
				{ID: "life/travel", Name: "旅行", Description: "旅游攻略、目的地", Icon: "✈️"},
				{ID: "life/home", Name: "家居", Description: "装修、收纳、家电", Icon: "🏡"},
				{ID: "life/pet", Name: "宠物", Description: "养宠知识、训练", Icon: "🐕"},
			},
		},
		// 教育类
		{
			ID:          "education",
			Name:        "教育",
			Description: "教育与学习",
			Icon:        "📚",
			Children: []*Category{
				{ID: "education/language", Name: "语言学习", Description: "外语、方言", Icon: "🗣️"},
				{ID: "education/exam", Name: "考试", Description: "各类考试、证书", Icon: "📝"},
				{ID: "education/skill", Name: "技能培训", Description: "职业技能、认证", Icon: "🎓"},
				{ID: "education/academic", Name: "学术研究", Description: "论文、研究方法", Icon: "📖"},
			},
		},
		// 艺术类
		{
			ID:          "art",
			Name:        "艺术",
			Description: "艺术与创意",
			Icon:        "🎨",
			Children: []*Category{
				{ID: "art/design", Name: "设计", Description: "UI/UX、平面设计", Icon: "🖌️"},
				{ID: "art/music", Name: "音乐", Description: "乐理、乐器、制作", Icon: "🎵"},
				{ID: "art/video", Name: "影视", Description: "视频制作、剪辑", Icon: "🎬"},
				{ID: "art/photography", Name: "摄影", Description: "摄影技巧、后期", Icon: "📷"},
				{ID: "art/writing", Name: "写作", Description: "创意写作、文案", Icon: "✍️"},
			},
		},
		// 工具类
		{
			ID:          "tools",
			Name:        "工具",
			Description: "软件工具与资源",
			Icon:        "🔧",
			Children: []*Category{
				{ID: "tools/software", Name: "软件", Description: "软件推荐、使用教程", Icon: "💿"},
				{ID: "tools/library", Name: "库与框架", Description: "开源库、开发框架", Icon: "📦"},
				{ID: "tools/api", Name: "API服务", Description: "API文档、集成", Icon: "🔌"},
				{ID: "tools/resource", Name: "资源", Description: "学习资源、素材", Icon: "📁"},
			},
		},
		// 其他
		{
			ID:          "other",
			Name:        "其他",
			Description: "其他未分类内容",
			Icon:        "📁",
		},
	}
}

// Initialize 初始化分类（如果不存在则创建）
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.loaded {
		return nil
	}
	
	categories := GetInitialCategories()
	m.flattenAndCache(categories, "")
	m.loaded = true
	
	return nil
}

// flattenAndCache 扁平化并缓存分类
func (m *Manager) flattenAndCache(categories []*Category, parentID string) {
	for _, cat := range categories {
		cat.ParentID = parentID
		m.cached[cat.ID] = cat
		
		if len(cat.Children) > 0 {
			m.flattenAndCache(cat.Children, cat.ID)
			cat.Children = nil // 清空子分类引用，避免循环
		}
	}
}

// Get 获取分类信息
func (m *Manager) Get(id string) (*Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	cat, exists := m.cached[id]
	if !exists {
		return nil, fmt.Errorf("分类不存在: %s", id)
	}
	return cat, nil
}

// List 列出所有分类
func (m *Manager) List() []*Category {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*Category, 0, len(m.cached))
	for _, cat := range m.cached {
		result = append(result, cat)
	}
	
	// 按ID排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	
	return result
}

// ListTopLevel 列出顶级分类
func (m *Manager) ListTopLevel() []*Category {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var result []*Category
	for _, cat := range m.cached {
		if cat.ParentID == "" {
			// 复制一份，添加子分类引用
			catCopy := *cat
			catCopy.Children = m.getChildren(cat.ID)
			result = append(result, &catCopy)
		}
	}
	
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	
	return result
}

// getChildren 获取子分类
func (m *Manager) getChildren(parentID string) []*Category {
	var children []*Category
	for _, cat := range m.cached {
		if cat.ParentID == parentID {
			children = append(children, cat)
		}
	}
	
	sort.Slice(children, func(i, j int) bool {
		return children[i].ID < children[j].ID
	})
	
	return children
}

// GetTree 获取分类树
func (m *Manager) GetTree() []*Category {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.buildTree("")
}

// buildTree 构建分类树
func (m *Manager) buildTree(parentID string) []*Category {
	var result []*Category
	
	for _, cat := range m.cached {
		if cat.ParentID == parentID {
			catCopy := *cat
			catCopy.Children = m.buildTree(cat.ID)
			result = append(result, &catCopy)
		}
	}
	
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	
	return result
}

// Validate 验证分类ID是否有效
func (m *Manager) Validate(categoryID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	_, exists := m.cached[categoryID]
	return exists
}

// GetBreadcrumb 获取分类面包屑
func (m *Manager) GetBreadcrumb(categoryID string) []*Category {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var breadcrumb []*Category
	currentID := categoryID
	
	for currentID != "" {
		cat, exists := m.cached[currentID]
		if !exists {
			break
		}
		breadcrumb = append([]*Category{cat}, breadcrumb...)
		currentID = cat.ParentID
	}
	
	return breadcrumb
}

// Search 搜索分类
func (m *Manager) Search(query string) []*Category {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	query = strings.ToLower(query)
	var result []*Category
	
	for _, cat := range m.cached {
		if strings.Contains(strings.ToLower(cat.Name), query) ||
			strings.Contains(strings.ToLower(cat.Description), query) {
			result = append(result, cat)
		}
	}
	
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	
	return result
}

// UpdateEntryCount 更新分类条目数量
func (m *Manager) UpdateEntryCount(categoryID string, delta int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	cat, exists := m.cached[categoryID]
	if exists {
		cat.EntryCount += delta
		if cat.EntryCount < 0 {
			cat.EntryCount = 0
		}
	}
}
