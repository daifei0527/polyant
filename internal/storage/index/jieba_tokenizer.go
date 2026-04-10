// Package index 提供全文搜索功能
// +build cgo

package index

import (
	"strings"
	"sync"
	"unicode"

	"github.com/yanyiwu/gojieba"
)

// JiebaTokenizer 结巴分词器实现
type JiebaTokenizer struct {
	jieba    *gojieba.Jieba
	mu       sync.RWMutex
	initialized bool
}

// jiebaInstance 全局jieba实例（单例模式）
var (
	jiebaInstance *JiebaTokenizer
	jiebaOnce     sync.Once
)

// NewJiebaTokenizer 创建结巴分词器
func NewJiebaTokenizer() *JiebaTokenizer {
	var j *gojieba.Jieba
	
	jiebaOnce.Do(func() {
		// 使用默认词典路径
		j = gojieba.NewJieba()
		if j != nil {
			jiebaInstance = &JiebaTokenizer{
				jieba:       j,
				initialized: true,
			}
		}
	})
	
	return jiebaInstance
}

// Tokenize 使用jieba进行中文分词
func (t *JiebaTokenizer) Tokenize(text string) []string {
	if text == "" || !t.initialized {
		return nil
	}
	
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	var tokens []string
	
	// 将文本按字符类型分段处理
	segments := segmentText(text)
	
	for _, segment := range segments {
		if isCJKSegment(segment) {
			// 中文段：使用jieba分词
			cjkTokens := t.tokenizeCJKWithJieba(segment)
			tokens = append(tokens, cjkTokens...)
		} else {
			// 英文/数字段：按空格分割
			engTokens := tokenizeEnglish(segment)
			tokens = append(tokens, engTokens...)
		}
	}
	
	// 过滤停用词
	tokens = filterStopWords(tokens)
	
	return tokens
}

// tokenizeCJKWithJieba 使用jieba对中文文本分词
func (t *JiebaTokenizer) tokenizeCJKWithJieba(text string) []string {
	if t.jieba == nil {
		// 降级到bigram
		return tokenizeCJK(text)
	}
	
	// 使用搜索引擎模式分词，更适合搜索场景
	words := t.jieba.CutForSearch(text, true)
	
	var tokens []string
	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) == 0 {
			continue
		}
		
		// 过滤纯标点
		allPunct := true
		for _, r := range word {
			if !unicode.IsPunct(r) {
				allPunct = false
				break
			}
		}
		if allPunct {
			continue
		}
		
		tokens = append(tokens, word)
	}
	
	return tokens
}

// TokenizeForSearch 搜索专用分词
func (t *JiebaTokenizer) TokenizeForSearch(text string) []string {
	if text == "" || !t.initialized || t.jieba == nil {
		return nil
	}
	
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	// 直接使用搜索引擎模式
	return t.jieba.CutForSearch(text, true)
}

// TokenizePrecise 精确模式分词
func (t *JiebaTokenizer) TokenizePrecise(text string) []string {
	if text == "" || !t.initialized || t.jieba == nil {
		return nil
	}
	
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	return t.jieba.Cut(text, false)
}

// TokenizeAll 全模式分词（返回所有可能的分词结果）
func (t *JiebaTokenizer) TokenizeAll(text string) []string {
	if text == "" || !t.initialized || t.jieba == nil {
		return nil
	}
	
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	return t.jieba.CutAll(text)
}

// Tag 词性标注
func (t *JiebaTokenizer) Tag(text string) []string {
	if text == "" || !t.initialized || t.jieba == nil {
		return nil
	}
	
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	return t.jieba.Tag(text)
}

// ExtractTags 提取关键词
func (t *JiebaTokenizer) ExtractTags(text string, topN int) []string {
	if text == "" || !t.initialized || t.jieba == nil {
		return nil
	}
	
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	return t.jieba.Extract(text, topN)
}

// Close 关闭分词器
func (t *JiebaTokenizer) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.jieba != nil {
		t.jieba.Free()
		t.jieba = nil
		t.initialized = false
	}
}

// IsInitialized 检查是否已初始化
func (t *JiebaTokenizer) IsInitialized() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.initialized
}
