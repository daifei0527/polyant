// Package index 提供全文搜索功能
package index

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// SimpleSearcher 定义了简单搜索引擎的接口
// 用于 SimpleSearchEngine 的内部接口定义
type SimpleSearcher interface {
	// Index 将知识条目添加到搜索索引
	Index(entry *model.KnowledgeEntry) error
	// Remove 从搜索索引中移除指定条目
	Remove(id string) error
	// Search 执行全文搜索，支持分类过滤和分页
	Search(query string, categories []string, limit, offset int) ([]*model.SearchResult, error)
	// Close 关闭搜索引擎，释放资源
	Close() error
}

// indexedEntry 存储已索引条目的信息
type indexedEntry struct {
	ID       string
	Title    string
	Content  string
	Category string
	Tags     []string
	// 各字段的词频统计
	titleTokens    []string
	contentTokens  []string
	tagTokens      []string
	titleTF        map[string]float64
	contentTF      map[string]float64
}

// SimpleSearchEngine 是一个简单的全文搜索引擎实现
// 使用TF-IDF类似的评分算法，支持中英文混合搜索
type SimpleSearchEngine struct {
	mu       sync.RWMutex
	entries  map[string]*indexedEntry
	// 文档频率：每个词出现在多少个文档中
	docFreq map[string]int
	// 总文档数
	totalDocs int
}

// NewSimpleSearchEngine 创建一个新的简单搜索引擎实例
func NewSimpleSearchEngine() *SimpleSearchEngine {
	return &SimpleSearchEngine{
		entries:  make(map[string]*indexedEntry),
		docFreq:  make(map[string]int),
		totalDocs: 0,
	}
}

// Index 将知识条目添加到搜索索引
func (se *SimpleSearchEngine) Index(entry *model.KnowledgeEntry) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	// 如果条目已存在，先移除旧索引
	if old, exists := se.entries[entry.ID]; exists {
		se.removeDocFreq(old)
		se.totalDocs--
	}

	// 对各字段进行分词
	titleTokens := Tokenize(entry.Title)
	contentTokens := Tokenize(entry.Content)
	tagTokens := make([]string, 0)
	for _, tag := range entry.Tags {
		tagTokens = append(tagTokens, Tokenize(tag)...)
	}

	// 计算词频
	titleTF := computeTF(titleTokens)
	contentTF := computeTF(contentTokens)

	// 合并所有唯一词用于文档频率统计
	allTokens := make(map[string]bool)
	for _, t := range titleTokens {
		allTokens[t] = true
	}
	for _, t := range contentTokens {
		allTokens[t] = true
	}
	for _, t := range tagTokens {
		allTokens[t] = true
	}

	// 更新文档频率
	for token := range allTokens {
		se.docFreq[token]++
	}

	ie := &indexedEntry{
		ID:            entry.ID,
		Title:         entry.Title,
		Content:       entry.Content,
		Category:      entry.Category,
		Tags:          entry.Tags,
		titleTokens:   titleTokens,
		contentTokens: contentTokens,
		tagTokens:     tagTokens,
		titleTF:       titleTF,
		contentTF:     contentTF,
	}

	se.entries[entry.ID] = ie
	se.totalDocs++

	return nil
}

// Remove 从搜索索引中移除指定条目
func (se *SimpleSearchEngine) Remove(id string) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	entry, exists := se.entries[id]
	if !exists {
		return fmt.Errorf("entry %s not found in index", id)
	}

	se.removeDocFreq(entry)
	delete(se.entries, id)
	se.totalDocs--

	return nil
}

// Search 执行全文搜索
// query: 搜索关键词
// categories: 可选的分类过滤列表，为空则搜索所有分类
// limit: 返回结果数量限制
// offset: 结果偏移量
func (se *SimpleSearchEngine) Search(query string, categories []string, limit, offset int) ([]*model.SearchResult, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	if query == "" {
		return []*model.SearchResult{}, nil
	}

	// 对查询进行分词
	queryTokens := Tokenize(query)
	if len(queryTokens) == 0 {
		return []*model.SearchResult{}, nil
	}

	// 计算查询词的IDF
	queryIDF := make(map[string]float64)
	for _, token := range queryTokens {
		queryIDF[token] = se.computeIDF(token)
	}

	// 对每个文档计算相关度得分
	type scoredEntry struct {
		entry *indexedEntry
		score float64
		matchedFields []string
	}

	var results []scoredEntry

	for _, ie := range se.entries {
		// 分类过滤
		if len(categories) > 0 {
			matched := false
			for _, cat := range categories {
				if ie.Category == cat || strings.HasPrefix(ie.Category, cat+"/") {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		score, matchedFields := se.scoreDocument(ie, queryTokens, queryIDF)
		if score > 0 {
			results = append(results, scoredEntry{
				entry:         ie,
				score:         score,
				matchedFields: matchedFields,
			})
		}
	}

	// 按得分降序排列，相同得分按ID升序确保稳定排序
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].entry.ID < results[j].entry.ID
	})

	// 应用分页
	if offset >= len(results) {
		return []*model.SearchResult{}, nil
	}

	end := offset + limit
	if end > len(results) {
		end = len(results)
	}

	searchResults := make([]*model.SearchResult, 0, end-offset)
	for i := offset; i < end; i++ {
		r := results[i]
		searchResults = append(searchResults, &model.SearchResult{
			EntryID:       r.entry.ID,
			Title:         r.entry.Title,
			Score:         r.score,
			MatchedFields: r.matchedFields,
		})
	}

	return searchResults, nil
}

// Close 关闭搜索引擎
func (se *SimpleSearchEngine) Close() error {
	se.mu.Lock()
	defer se.mu.Unlock()

	se.entries = nil
	se.docFreq = nil
	se.totalDocs = 0

	return nil
}

// scoreDocument 计算文档与查询的相关度得分
func (se *SimpleSearchEngine) scoreDocument(ie *indexedEntry, queryTokens []string, queryIDF map[string]float64) (float64, []string) {
	var score float64
	matchedFields := make(map[string]bool)

	// 标题匹配得分（权重更高）
	titleScore := se.scoreField(ie.titleTF, queryTokens, queryIDF)
	if titleScore > 0 {
		score += titleScore * 3.0 // 标题权重为3倍
		matchedFields["title"] = true
	}

	// 内容匹配得分
	contentScore := se.scoreField(ie.contentTF, queryTokens, queryIDF)
	if contentScore > 0 {
		score += contentScore * 1.0
		matchedFields["content"] = true
	}

	// 标签匹配得分
	tagTF := computeTF(ie.tagTokens)
	tagScore := se.scoreField(tagTF, queryTokens, queryIDF)
	if tagScore > 0 {
		score += tagScore * 2.0 // 标签权重为2倍
		matchedFields["tags"] = true
	}

	// 转换matchedFields为切片
	fields := make([]string, 0, len(matchedFields))
	for f := range matchedFields {
		fields = append(fields, f)
	}

	return score, fields
}

// scoreField 计算单个字段的匹配得分
func (se *SimpleSearchEngine) scoreField(tf map[string]float64, queryTokens []string, queryIDF map[string]float64) float64 {
	var score float64

	for _, token := range queryTokens {
		if freq, exists := tf[token]; exists {
			idf := queryIDF[token]
			// TF-IDF得分
			score += freq * idf
		}
	}

	return score
}

// computeIDF 计算逆文档频率
func (se *SimpleSearchEngine) computeIDF(token string) float64 {
	df := float64(se.docFreq[token])
	if df == 0 {
		return 0
	}
	// 使用平滑IDF避免除零
	return math.Log(float64(se.totalDocs+1) / (df + 1)) + 1
}

// removeDocFreq 移除文档的词频贡献
func (se *SimpleSearchEngine) removeDocFreq(ie *indexedEntry) {
	allTokens := make(map[string]bool)
	for _, t := range ie.titleTokens {
		allTokens[t] = true
	}
	for _, t := range ie.contentTokens {
		allTokens[t] = true
	}
	for _, t := range ie.tagTokens {
		allTokens[t] = true
	}

	for token := range allTokens {
		if se.docFreq[token] > 0 {
			se.docFreq[token]--
		}
	}
}

// computeTF 计算词频（归一化）
func computeTF(tokens []string) map[string]float64 {
	if len(tokens) == 0 {
		return make(map[string]float64)
	}

	tf := make(map[string]float64)
	for _, token := range tokens {
		tf[token]++
	}

	// 归一化
	total := float64(len(tokens))
	for token := range tf {
		tf[token] /= total
	}

	return tf
}
