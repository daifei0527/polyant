// Package index 提供全文搜索功能
package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/index/upsidedown"
	"github.com/blevesearch/bleve/v2/index/upsidedown/store/boltdb"
	"github.com/blevesearch/bleve/v2/mapping"

	"github.com/daifei0527/polyant/pkg/safeconv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

const (
	// indexMappingName 索引映射名称
	indexMappingName = "entry"
)

// BleveEngine 是基于 Bleve 的全文搜索引擎实现
// 支持持久化索引和中文分词
type BleveEngine struct {
	index     bleve.Index
	jieba     *JiebaWrapper
	indexPath string // 索引磁盘路径，供 Rebuild 删目录重建使用
}

// entryDocument 是用于索引的文档结构
type entryDocument struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	AllText   string   `json:"all_text"` // 合并主文本 + 所有语言本地化文本，供跨语言搜索命中
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	Status    string   `json:"status"`
	Score     float64  `json:"score"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// NewBleveEngine 创建一个新的 Bleve 搜索引擎实例
// indexPath: 索引文件存储路径
func NewBleveEngine(indexPath string) (*BleveEngine, error) {
	// 创建中文分词器
	jieba := NewJiebaWrapper()

	indexPath = filepath.Clean(indexPath)

	idx, err := openOrCreate(indexPath, buildMapping())
	if err != nil {
		return nil, err
	}

	return &BleveEngine{
		index:     idx,
		jieba:     jieba,
		indexPath: indexPath,
	}, nil
}

// buildMapping 构造 bleve 索引映射（字段分析器、权重等）。
// 抽出为独立 helper，供 NewBleveEngine 与 Rebuild 复用，保证两次创建的 mapping 完全一致。
func buildMapping() mapping.IndexMapping {
	mapping := bleve.NewIndexMapping()
	mapping.TypeField = "type"
	mapping.DefaultAnalyzer = "standard"

	// 为标题和内容字段使用自定义分析器
	entryMapping := bleve.NewDocumentMapping()

	// 标题字段 - 更高权重
	titleField := bleve.NewTextFieldMapping()
	titleField.Analyzer = "standard"
	titleField.Store = true
	entryMapping.AddFieldMappingsAt("title", titleField)

	// 内容字段
	contentField := bleve.NewTextFieldMapping()
	contentField.Analyzer = "standard"
	contentField.Store = false // 内容不需要存储，只索引
	entryMapping.AddFieldMappingsAt("content", contentField)

	// 全文本字段（合并主文本 + 所有语言本地化文本），供跨语言搜索命中
	allTextField := bleve.NewTextFieldMapping()
	allTextField.Analyzer = "standard"
	allTextField.Store = false
	entryMapping.AddFieldMappingsAt("all_text", allTextField)

	// 分类字段 - 使用 keyword 分析器支持精确匹配
	categoryField := bleve.NewTextFieldMapping()
	categoryField.Analyzer = keyword.Name
	categoryField.Store = true
	entryMapping.AddFieldMappingsAt("category", categoryField)

	// 标签字段
	tagsField := bleve.NewTextFieldMapping()
	tagsField.Analyzer = keyword.Name
	tagsField.Store = true
	entryMapping.AddFieldMappingsAt("tags", tagsField)

	// 其他字段
	entryMapping.AddFieldMappingsAt("id", bleve.NewKeywordFieldMapping())
	entryMapping.AddFieldMappingsAt("status", bleve.NewKeywordFieldMapping())
	entryMapping.AddFieldMappingsAt("score", bleve.NewNumericFieldMapping())
	entryMapping.AddFieldMappingsAt("created_at", bleve.NewNumericFieldMapping())
	entryMapping.AddFieldMappingsAt("updated_at", bleve.NewNumericFieldMapping())

	mapping.AddDocumentMapping(indexMappingName, entryMapping)
	return mapping
}

// openOrCreate 打开已存在的索引，或用 mapping 新建。
func openOrCreate(indexPath string, idxMapping mapping.IndexMapping) (bleve.Index, error) {
	indexPath = filepath.Clean(indexPath)

	// 检查索引是否已存在
	if bleveIndexExists(indexPath) {
		idx, err := bleve.OpenUsing(indexPath, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create/open bleve index: %w", err)
		}
		return idx, nil
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		// gojieba 有 finalizer，会自动释放
		return nil, fmt.Errorf("failed to create index directory: %w", err)
	}
	// 创建新索引，使用 upsidedown 索引类型和 boltdb 存储
	idx, err := bleve.NewUsing(indexPath, idxMapping, upsidedown.Name, boltdb.Name, nil)
	if err != nil {
		// 注意: gojieba 有 finalizer，会在 GC 时自动释放，不需要手动 Free
		return nil, fmt.Errorf("failed to create/open bleve index: %w", err)
	}
	return idx, nil
}

// Rebuild 清空索引并按给定 entries 全量重建（bleve 无 ClearIndex，故 Close+删目录+重建）。
// 用于启动时强制索引↔store 一致，自愈历史漂移或损坏。
func (e *BleveEngine) Rebuild(entries []*model.KnowledgeEntry) error {
	if e.index == nil {
		return fmt.Errorf("bleve engine not initialized")
	}
	if e.indexPath == "" {
		return fmt.Errorf("bleve engine missing indexPath, cannot rebuild")
	}

	// 关闭并删除旧索引目录，重建空索引
	if err := e.index.Close(); err != nil {
		return fmt.Errorf("rebuild: close old index: %w", err)
	}
	if err := os.RemoveAll(e.indexPath); err != nil {
		return fmt.Errorf("rebuild: remove old index dir: %w", err)
	}
	idx, err := openOrCreate(e.indexPath, buildMapping())
	if err != nil {
		// 标记 index 已关闭，避免后续 Close/操作二次报错；返回错误
		e.index = nil
		return fmt.Errorf("rebuild: reopen index: %w", err)
	}
	e.index = idx

	// 全量重灌
	for _, entry := range entries {
		if err := e.IndexEntry(entry); err != nil {
			return fmt.Errorf("rebuild index entry %s: %w", entry.ID, err)
		}
	}
	return nil
}

// bleveIndexExists 检查索引是否存在
func bleveIndexExists(path string) bool {
	// 检查目录是否存在
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	// 尝试打开确认是有效索引
	idx, err := bleve.Open(path)
	if err != nil {
		return false
	}
	// 立即关闭（我们只是检查是否存在）
	idx.Close()
	return true
}

// IndexEntry 将条目加入全文索引
func (e *BleveEngine) IndexEntry(entry *model.KnowledgeEntry) error {
	doc := &entryDocument{
		ID:        entry.ID,
		Title:     entry.Title,
		Content:   entry.Content,
		AllText:   buildAllText(entry),
		Category:  entry.Category,
		Tags:      entry.Tags,
		Status:    string(entry.Status),
		Score:     entry.Score,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
	}

	return e.index.Index(entry.ID, doc)
}

// buildAllText 合并条目的主文本与所有语言的本地化文本，用于跨语言全文搜索。
func buildAllText(entry *model.KnowledgeEntry) string {
	parts := make([]string, 0, 2+len(entry.TitleI18n)+len(entry.ContentI18n))
	parts = append(parts, entry.Title, entry.Content)
	for _, v := range entry.TitleI18n {
		parts = append(parts, v)
	}
	for _, v := range entry.ContentI18n {
		parts = append(parts, v)
	}
	return strings.Join(parts, " ")
}

// UpdateIndex 更新条目索引
func (e *BleveEngine) UpdateIndex(entry *model.KnowledgeEntry) error {
	// Bleve 的 Index 方法会自动覆盖已存在的文档
	return e.IndexEntry(entry)
}

// DeleteIndex 从索引中删除条目
func (e *BleveEngine) DeleteIndex(entryID string) error {
	return e.index.Delete(entryID)
}

// Search 执行全文搜索
func (e *BleveEngine) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	// 空关键词返回空结果
	if query.Keyword == "" {
		return &SearchResult{
			TotalCount: 0,
			HasMore:    false,
			Entries:    []*model.KnowledgeEntry{},
		}, nil
	}

	// 构建布尔查询
	boolQuery := bleve.NewBooleanQuery()

	// 关键词查询
	// 对中文关键词进行分词
	keywords := e.segmentChinese(query.Keyword)

	// 每个分词结果都必须匹配（使用 conjunction 查询）
	// 这样可以确保 "不存在的关键词" 不会因为单个字匹配而返回结果
	for _, kw := range keywords {
		if kw == "" {
			continue
		}

		// 每个关键词必须在标题或内容中匹配
		fieldDisjunction := bleve.NewDisjunctionQuery()

		// 标题匹配
		titleQuery := bleve.NewMatchQuery(kw)
		titleQuery.SetField("title")

		// 内容匹配
		contentQuery := bleve.NewMatchQuery(kw)
		contentQuery.SetField("content")

		// 全文本（含本地化）匹配
		allTextQuery := bleve.NewMatchQuery(kw)
		allTextQuery.SetField("all_text")

		fieldDisjunction.AddQuery(titleQuery)
		fieldDisjunction.AddQuery(contentQuery)
		fieldDisjunction.AddQuery(allTextQuery)

		// 添加到 must 子句 - 每个关键词都必须匹配
		boolQuery.AddMust(fieldDisjunction)
	}

	// 分类过滤
	if len(query.Categories) > 0 {
		catDisjunction := bleve.NewDisjunctionQuery()
		for _, cat := range query.Categories {
			// 使用前缀匹配支持层级分类
			catQuery := bleve.NewPrefixQuery(cat)
			catQuery.SetField("category")
			catDisjunction.AddQuery(catQuery)
		}
		boolQuery.AddMust(catDisjunction)
	}

	// 最低评分过滤
	if query.MinScore > 0 {
		rangeQuery := bleve.NewNumericRangeQuery(&query.MinScore, nil)
		rangeQuery.SetField("score")
		boolQuery.AddMust(rangeQuery)
	}

	// C3：只搜 published 条目（与 MemorySearchEngine/BadgerSearchEngine 行为一致）
	statusQuery := bleve.NewTermQuery(model.EntryStatusPublished)
	statusQuery.SetField("status")
	boolQuery.AddMust(statusQuery)

	// 构建搜索请求
	searchRequest := bleve.NewSearchRequest(boolQuery)
	searchRequest.Size = query.Limit
	searchRequest.From = query.Offset

	// 按相关度排序
	searchRequest.SortBy([]string{"-_score"})

	// 设置需要存储的字段
	searchRequest.Fields = []string{"title", "category", "tags", "score"}

	// 执行搜索
	searchResult, err := e.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 获取命中的文档
	entries := make([]*model.KnowledgeEntry, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		entry := &model.KnowledgeEntry{
			ID: hit.ID,
		}

		// 从搜索结果中获取字段
		if title, ok := hit.Fields["title"].(string); ok {
			entry.Title = title
		}
		if category, ok := hit.Fields["category"].(string); ok {
			entry.Category = category
		}
		if score, ok := hit.Fields["score"].(float64); ok {
			entry.Score = score
		}
		// 处理 tags 字段（可能是切片）
		if tags, ok := hit.Fields["tags"].([]interface{}); ok {
			entry.Tags = make([]string, 0, len(tags))
			for _, t := range tags {
				if ts, ok := t.(string); ok {
					entry.Tags = append(entry.Tags, ts)
				}
			}
		}

		entries = append(entries, entry)
	}

	return &SearchResult{
		TotalCount: safeconv.IntFromUint64(searchResult.Total),
		HasMore:    safeconv.IntFromUint64(searchResult.Total) > query.Offset+query.Limit,
		Entries:    entries,
	}, nil
}

// segmentChinese 对中文文本进行分词
func (e *BleveEngine) segmentChinese(text string) []string {
	if e.jieba == nil {
		// 降级到简单分词
		return simpleTokenize(text)
	}

	// 使用 jieba 进行精确模式分词
	words := e.jieba.Cut(text, true)

	// 过滤停用词和单字
	result := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) > 1 && !isStopWord(word) {
			result = append(result, word)
		}
	}

	// 如果分词后没有有效词，使用原始关键词
	if len(result) == 0 {
		return []string{text}
	}

	return result
}

// simpleTokenize 简单分词（当 jieba 不可用时使用）
func simpleTokenize(text string) []string {
	// 简单按空格和标点分词
	sep := []string{" ", "\t", "\n", ",", ".", "，", "。", "、", "；", "：", "！", "？"}
	result := []string{text}
	for _, s := range sep {
		var newResult []string
		for _, r := range result {
			parts := strings.Split(r, s)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if len(p) > 1 {
					newResult = append(newResult, p)
				}
			}
		}
		result = newResult
	}
	return result
}

// isStopWord 检查是否为停用词
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"的": true, "是": true, "在": true, "了": true,
		"和": true, "与": true, "或": true, "等": true,
		"这": true, "那": true, "有": true, "为": true,
		"以": true, "及": true, "其": true, "于": true,
		"也": true, "都": true, "就": true, "着": true,
		"还": true, "会": true, "能": true, "要": true,
		"可": true, "但": true, "而": true, "被": true,
		"把": true, "从": true, "到": true, "对": true,
	}
	return stopWords[word]
}

// Close 关闭搜索引擎，释放资源
func (e *BleveEngine) Close() error {
	// 注意: gojieba 有 runtime.SetFinalizer 设置，会在 GC 时自动调用 Free()
	// 所以我们不应该手动调用 Free()，否则会导致 double-free 崩溃
	// 只需要将引用置空，让 GC 处理
	e.jieba = nil
	if e.index != nil {
		return e.index.Close()
	}
	return nil
}

// IndexCount 返回索引中的文档数量
func (e *BleveEngine) IndexCount() (uint64, error) {
	return e.index.DocCount()
}
