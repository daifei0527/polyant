// Package index 提供全文搜索功能
package index

import (
	"strings"
	"sync"
	"unicode"
)

// TokenizerType 分词器类型
type TokenizerType int

const (
	// TokenizerSimple 简单分词器（bigram）
	TokenizerSimple TokenizerType = iota
	// TokenizerJieba 结巴分词器（gojieba）
	TokenizerJieba
)

// Tokenizer 分词器接口
type Tokenizer interface {
	Tokenize(text string) []string
	Close()
}

// ==================== 简单分词器 ====================

// SimpleTokenizer 简单分词器实现
type SimpleTokenizer struct{}

// NewSimpleTokenizer 创建简单分词器
func NewSimpleTokenizer() *SimpleTokenizer {
	return &SimpleTokenizer{}
}

// Tokenize 实现简单分词
func (t *SimpleTokenizer) Tokenize(text string) []string {
	return Tokenize(text)
}

// Close 关闭分词器
func (t *SimpleTokenizer) Close() {}

// ==================== 分词器管理器 ====================

// TokenizerManager 分词器管理器
type TokenizerManager struct {
	mu        sync.RWMutex
	tokenizer Tokenizer
	ttype     TokenizerType
}

// GlobalTokenizer 全局分词器实例
var GlobalTokenizer = NewTokenizerManager(TokenizerSimple)

// NewTokenizerManager 创建分词器管理器
func NewTokenizerManager(ttype TokenizerType) *TokenizerManager {
	tm := &TokenizerManager{
		ttype: ttype,
	}
	
	switch ttype {
	case TokenizerJieba:
		// 尝试初始化jieba分词器
		jieba := NewJiebaTokenizer()
		if jieba != nil {
			tm.tokenizer = jieba
		} else {
			// 降级到简单分词器
			tm.tokenizer = NewSimpleTokenizer()
			tm.ttype = TokenizerSimple
		}
	default:
		tm.tokenizer = NewSimpleTokenizer()
	}
	
	return tm
}

// Tokenize 使用当前分词器进行分词
func (tm *TokenizerManager) Tokenize(text string) []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	if tm.tokenizer == nil {
		return Tokenize(text)
	}
	return tm.tokenizer.Tokenize(text)
}

// SwitchTokenizer 切换分词器类型
func (tm *TokenizerManager) SwitchTokenizer(ttype TokenizerType) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	// 已经是目标类型
	if tm.ttype == ttype && tm.tokenizer != nil {
		return true
	}
	
	// 关闭旧分词器
	if tm.tokenizer != nil {
		tm.tokenizer.Close()
	}
	
	switch ttype {
	case TokenizerJieba:
		jieba := NewJiebaTokenizer()
		if jieba != nil {
			tm.tokenizer = jieba
			tm.ttype = TokenizerJieba
			return true
		}
		// 降级
		tm.tokenizer = NewSimpleTokenizer()
		tm.ttype = TokenizerSimple
		return false
	default:
		tm.tokenizer = NewSimpleTokenizer()
		tm.ttype = TokenizerSimple
		return true
	}
}

// Type 返回当前分词器类型
func (tm *TokenizerManager) Type() TokenizerType {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.ttype
}

// Close 关闭分词器管理器
func (tm *TokenizerManager) Close() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.tokenizer != nil {
		tm.tokenizer.Close()
		tm.tokenizer = nil
	}
}

// ==================== 工具函数 ====================

// IsCJKText 判断文本是否主要包含CJK字符
func IsCJKText(text string) bool {
	cjkCount := 0
	totalCount := 0
	
	for _, r := range text {
		if unicode.IsLetter(r) {
			totalCount++
			if isCJKRune(r) {
				cjkCount++
			}
		}
	}
	
	// 如果CJK字符占比超过30%，认为是中文文本
	if totalCount == 0 {
		return false
	}
	return float64(cjkCount)/float64(totalCount) > 0.3
}

// ExtractKeywords 从文本中提取关键词
func ExtractKeywords(text string, topN int) []string {
	tokens := GlobalTokenizer.Tokenize(text)
	
	// 统计词频
	freq := make(map[string]int)
	for _, token := range tokens {
		freq[token]++
	}
	
	// 转换为切片并排序
	type kw struct {
		word  string
		count int
	}
	
	var kws []kw
	for w, c := range freq {
		kws = append(kws, kw{word: w, count: c})
	}
	
	// 简单排序（实际应用中可使用TF-IDF）
	for i := 0; i < len(kws); i++ {
		for j := i + 1; j < len(kws); j++ {
			if kws[j].count > kws[i].count {
				kws[i], kws[j] = kws[j], kws[i]
			}
		}
	}
	
	// 返回前N个
	result := make([]string, 0, topN)
	for i := 0; i < len(kws) && i < topN; i++ {
		result = append(result, kws[i].word)
	}
	
	return result
}

// HighlightMatch 在原文中高亮匹配的关键词
func HighlightMatch(text string, query string, prefix, suffix string) string {
	queryTokens := GlobalTokenizer.Tokenize(query)
	if len(queryTokens) == 0 {
		return text
	}
	
	result := text
	for _, token := range queryTokens {
		// 简单替换（实际应用中可能需要更复杂的高亮逻辑）
		result = strings.ReplaceAll(result, token, prefix+token+suffix)
	}
	
	return result
}
