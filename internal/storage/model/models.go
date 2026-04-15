// Package model 定义了Polyant系统的核心数据模型
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ==================== 用户等级常量 ====================

const (
	UserLevelLv0 int32 = 0 // 未认证用户
	UserLevelLv1 int32 = 1 // 普通用户
	UserLevelLv2 int32 = 2 // 活跃用户
	UserLevelLv3 int32 = 3 // 贡献者
	UserLevelLv4 int32 = 4 // 专家
	UserLevelLv5 int32 = 5 // 管理员
)

// ==================== 条目状态常量 ====================

const (
	EntryStatusDraft     = "draft"     // 草稿
	EntryStatusPublished = "published" // 已发布
	EntryStatusArchived  = "archived"  // 已归档
	EntryStatusDeleted   = "deleted"   // 已删除
	EntryStatusReview    = "review"    // 审核中
)

// ==================== 节点类型常量 ====================

const (
	NodeTypeFull    = "full"    // 全节点
	NodeTypeLight   = "light"   // 轻节点
	NodeTypeArchive = "archive" // 归档节点
	NodeTypeEdge    = "edge"    // 边缘节点
)

// ==================== 知识条目 ====================

// KnowledgeEntry 表示一条知识条目
type KnowledgeEntry struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`     // Markdown格式内容
	JSONData    []map[string]interface{} `json:"jsonData"`  // 结构化JSON数据
	Category    string                 `json:"category"`    // 所属分类路径
	Tags        []string               `json:"tags"`        // 标签列表
	Version     int64                  `json:"version"`     // 版本号
	CreatedAt   int64                  `json:"createdAt"`   // 创建时间(Unix时间戳)
	UpdatedAt   int64                  `json:"updatedAt"`   // 更新时间
	CreatedBy   string                 `json:"createdBy"`   // 创建者公钥
	Score       float64                `json:"score"`       // 加权平均评分
	ScoreCount  int32                  `json:"scoreCount"`  // 评分数量
	ContentHash string                 `json:"contentHash"` // 内容哈希
	Status      string                 `json:"status"`      // 条目状态
	License     string                 `json:"license"`     // 许可证
	SourceRef   string                 `json:"sourceRef"`   // 来源引用
	// 多语言支持
	Lang        string            `json:"lang,omitempty"`        // 条目主语言
	TitleI18n   map[string]string `json:"titleI18n,omitempty"`   // 多语言标题 {"zh-CN": "标题", "en-US": "Title"}
	ContentI18n map[string]string `json:"contentI18n,omitempty"` // 多语言内容
}

// NewKnowledgeEntry 创建一个新的知识条目实例，自动生成ID和时间戳
func NewKnowledgeEntry(title, content, category, createdBy string) *KnowledgeEntry {
	now := time.Now().Unix()
	entry := &KnowledgeEntry{
		ID:        generateID(),
		Title:     title,
		Content:   content,
		Category:  category,
		Tags:      []string{},
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: createdBy,
		Score:     0,
		ScoreCount: 0,
		Status:    EntryStatusDraft,
		License:   "CC-BY-SA-4.0",
	}
	entry.ContentHash = entry.ComputeContentHash()
	return entry
}

// ComputeContentHash 计算条目内容的SHA256哈希值
func (e *KnowledgeEntry) ComputeContentHash() string {
	data := fmt.Sprintf("%s:%s:%d:%v", e.Title, e.Content, e.Version, e.JSONData)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// ToJSON 将知识条目序列化为JSON字节数组
func (e *KnowledgeEntry) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON 从JSON字节数组反序列化为知识条目
func (e *KnowledgeEntry) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

// GetTitleByLang 根据语言获取标题
func (e *KnowledgeEntry) GetTitleByLang(lang string) string {
	if e.TitleI18n != nil {
		if title, ok := e.TitleI18n[lang]; ok {
			return title
		}
	}
	return e.Title
}

// GetContentByLang 根据语言获取内容
func (e *KnowledgeEntry) GetContentByLang(lang string) string {
	if e.ContentI18n != nil {
		if content, ok := e.ContentI18n[lang]; ok {
			return content
		}
	}
	return e.Content
}

// ==================== 用户 ====================

// User 表示系统中的一个用户
type User struct {
	PublicKey      string `json:"publicKey"`      // 用户公钥(唯一标识)
	AgentName      string `json:"agentName"`      // 代理名称
	UserLevel      int32  `json:"userLevel"`      // 用户等级
	Email          string `json:"email"`          // 电子邮箱
	EmailVerified  bool   `json:"emailVerified"`  // 邮箱是否已验证
	Phone          string `json:"phone"`          // 手机号码
	RegisteredAt   int64  `json:"registeredAt"`   // 注册时间
	LastActive     int64  `json:"lastActive"`     // 最后活跃时间
	ContributionCnt int32 `json:"contributionCnt"` // 贡献数量
	RatingCnt      int32  `json:"ratingCnt"`      // 评分数量
	NodeId         string `json:"nodeId"`         // 所属节点ID
	Status         string `json:"status"`         // 用户状态
	// 管理员相关字段
	BanType           BanType `json:"banType,omitempty"`           // 封禁类型
	BanReason         string  `json:"banReason,omitempty"`         // 封禁原因
	BannedAt          int64   `json:"bannedAt,omitempty"`          // 封禁时间
	BannedBy          string  `json:"bannedBy,omitempty"`          // 封禁操作者公钥
	UnbannedAt        int64   `json:"unbannedAt,omitempty"`        // 解封时间
	UnbannedBy        string  `json:"unbannedBy,omitempty"`        // 解封操作者公钥
	LevelChangeReason string  `json:"levelChangeReason,omitempty"` // 等级变更原因
	LevelChangedAt    int64   `json:"levelChangedAt,omitempty"`    // 等级变更时间
	LevelChangedBy    string  `json:"levelChangedBy,omitempty"`    // 等级变更操作者公钥
}

// ToJSON 将用户序列化为JSON字节数组
func (u *User) ToJSON() ([]byte, error) {
	return json.Marshal(u)
}

// FromJSON 从JSON字节数组反序列化为用户
func (u *User) FromJSON(data []byte) error {
	return json.Unmarshal(data, u)
}

// IsBanned 检查用户是否被封禁（完全禁止或只读）
func (u *User) IsBanned() bool {
	return u.Status == UserStatusBanned
}

// IsReadOnly 检查用户是否处于只读模式
func (u *User) IsReadOnly() bool {
	return u.Status == UserStatusBanned && u.BanType == BanTypeReadonly
}

// IsFullBanned 检查用户是否完全被封禁
func (u *User) IsFullBanned() bool {
	return u.Status == UserStatusBanned && (u.BanType == "" || u.BanType == BanTypeFull)
}

// UserStats 用户统计信息
type UserStats struct {
	TotalUsers    int64 `json:"totalUsers"`    // 总用户数
	Lv0Count      int64 `json:"lv0Count"`      // Lv0 用户数
	Lv1Count      int64 `json:"lv1Count"`      // Lv1 用户数
	Lv2Count      int64 `json:"lv2Count"`      // Lv2 用户数
	Lv3Count      int64 `json:"lv3Count"`      // Lv3 用户数
	Lv4Count      int64 `json:"lv4Count"`      // Lv4 用户数
	Lv5Count      int64 `json:"lv5Count"`      // Lv5 用户数
	ActiveUsers   int64 `json:"activeUsers"`   // 活跃用户数（30天内）
	BannedCount   int64 `json:"bannedCount"`   // 被封禁用户数
	TotalContribs int64 `json:"totalContribs"` // 总贡献数
	TotalRatings  int64 `json:"totalRatings"`  // 总评分数
}

// ==================== 评分 ====================

// Rating 表示对知识条目的评分
type Rating struct {
	ID           string  `json:"id"`           // 评分唯一ID
	EntryId      string  `json:"entryId"`      // 被评分条目ID
	RaterPubkey  string  `json:"raterPubkey"`  // 评分者公钥
	Score        float64 `json:"score"`        // 原始评分
	Weight       float64 `json:"weight"`       // 评分权重(基于用户等级)
	WeightedScore float64 `json:"weightedScore"` // 加权评分
	RatedAt      int64   `json:"ratedAt"`      // 评分时间
	Comment      string  `json:"comment"`      // 评分评论
}

// ToJSON 将评分序列化为JSON字节数组
func (r *Rating) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON 从JSON字节数组反序列化为评分
func (r *Rating) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

// ==================== 分类 ====================

// Category 表示知识分类
type Category struct {
	ID          string `json:"id"`          // 分类唯一ID
	Path        string `json:"path"`        // 分类路径(如 "tech/programming")
	Name        string `json:"name"`        // 分类名称
	ParentId    string `json:"parentId"`    // 父分类ID
	Level       int32  `json:"level"`       // 层级深度
	SortOrder   int32  `json:"sortOrder"`   // 排序顺序
	IsBuiltin   bool   `json:"isBuiltin"`   // 是否为内置分类
	MaintainedBy string `json:"maintainedBy"` // 维护者公钥
	CreatedAt   int64  `json:"createdAt"`   // 创建时间
	// 多语言支持
	NameI18n map[string]string `json:"nameI18n,omitempty"` // 多语言名称
	DescI18n map[string]string `json:"descI18n,omitempty"` // 多语言描述
}

// ToJSON 将分类序列化为JSON字节数组
func (c *Category) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON 从JSON字节数组反序列化为分类
func (c *Category) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

// GetNameByLang 根据语言获取分类名称
func (c *Category) GetNameByLang(lang string) string {
	if c.NameI18n != nil {
		if name, ok := c.NameI18n[lang]; ok {
			return name
		}
	}
	return c.Name
}

// ==================== 节点信息 ====================

// NodeInfo 表示网络中的一个节点
type NodeInfo struct {
	NodeId         string   `json:"nodeId"`         // 节点唯一ID
	NodeType       string   `json:"nodeType"`       // 节点类型
	PeerId         string   `json:"peerId"`         // P2P网络Peer ID
	PublicKey      string   `json:"publicKey"`      // 节点公钥
	Addresses      []string `json:"addresses"`      // 节点地址列表
	Version        string   `json:"version"`        // 节点软件版本
	EntryCount     int64    `json:"entryCount"`     // 条目数量
	CategoryMirror []string `json:"categoryMirror"` // 镜像的分类列表
	LastSync       int64    `json:"lastSync"`       // 最后同步时间
	Uptime         int64    `json:"uptime"`         // 运行时长(秒)
	AllowMirror    bool     `json:"allowMirror"`    // 是否允许镜像
	BandwidthLimit int64    `json:"bandwidthLimit"` // 带宽限制(bytes/s)
}

// ToJSON 将节点信息序列化为JSON字节数组
func (n *NodeInfo) ToJSON() ([]byte, error) {
	return json.Marshal(n)
}

// FromJSON 从JSON字节数组反序列化为节点信息
func (n *NodeInfo) FromJSON(data []byte) error {
	return json.Unmarshal(data, n)
}

// ==================== 搜索结果 ====================

// SearchResult 表示全文搜索的结果
type SearchResult struct {
	EntryID       string   `json:"entryId"`       // 匹配条目ID
	Title         string   `json:"title"`         // 条目标题
	Score         float64  `json:"score"`         // 匹配得分
	MatchedFields []string `json:"matchedFields"` // 匹配的字段列表
}

// ==================== 辅助函数 ====================

// generateID 生成一个简单的唯一ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// IsCJK 判断一个字符是否为中日韩文字
func IsCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||   // CJK统一汉字
		(r >= 0x3400 && r <= 0x4DBF) ||   // CJK扩展A
		(r >= 0x3000 && r <= 0x303F) ||   // CJK标点符号
		(r >= 0xFF00 && r <= 0xFFEF)      // 全角字符
}

// ContainsCJK 判断字符串是否包含中日韩文字
func ContainsCJK(s string) bool {
	for _, r := range s {
		if IsCJK(r) {
			return true
		}
	}
	return false
}

// NormalizeKey 将字符串转换为小写并去除首尾空格
func NormalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
