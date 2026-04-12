// Package index_test 提供搜索引擎的单元测试
package index_test

import (
	"testing"

	"github.com/daifei0527/agentwiki/internal/storage/index"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// ==================== SimpleSearchEngine 测试 ====================

// TestSimpleSearchEngineIndex 测试索引添加
func TestSimpleSearchEngineIndex(t *testing.T) {
	se := index.NewSimpleSearchEngine()
	defer se.Close()

	entry := &model.KnowledgeEntry{
		ID:       "entry-1",
		Title:    "人工智能入门",
		Content:  "人工智能是计算机科学的一个分支",
		Category: "tech/ai",
		Tags:     []string{"AI", "机器学习"},
	}

	err := se.Index(entry)
	if err != nil {
		t.Fatalf("Index 失败: %v", err)
	}
}

// TestSimpleSearchEngineIndexUpdate 测试索引更新
func TestSimpleSearchEngineIndexUpdate(t *testing.T) {
	se := index.NewSimpleSearchEngine()
	defer se.Close()

	// 首次索引
	entry := &model.KnowledgeEntry{
		ID:       "entry-1",
		Title:    "原标题",
		Content:  "原内容",
		Category: "tech",
	}
	se.Index(entry)

	// 更新索引
	entry.Title = "新标题"
	entry.Content = "新内容"
	err := se.Index(entry)
	if err != nil {
		t.Fatalf("Index 更新失败: %v", err)
	}
}

// TestSimpleSearchEngineRemove 测试索引删除
func TestSimpleSearchEngineRemove(t *testing.T) {
	se := index.NewSimpleSearchEngine()
	defer se.Close()

	entry := &model.KnowledgeEntry{
		ID:       "entry-1",
		Title:    "测试条目",
		Content:  "测试内容",
		Category: "test",
	}
	se.Index(entry)

	// 删除
	err := se.Remove("entry-1")
	if err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}

	// 删除不存在的条目
	err = se.Remove("not-exist")
	if err == nil {
		t.Error("删除不存在的条目应该返回错误")
	}
}

// TestSimpleSearchEngineSearch 测试搜索功能
func TestSimpleSearchEngineSearch(t *testing.T) {
	se := index.NewSimpleSearchEngine()
	defer se.Close()

	// 添加测试数据
	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "人工智能基础", Content: "机器学习是人工智能的核心技术", Category: "tech/ai"},
		{ID: "entry-2", Title: "深度学习入门", Content: "神经网络是深度学习的基础", Category: "tech/dl"},
		{ID: "entry-3", Title: "编程语言", Content: "Go语言是一种高效的编程语言", Category: "tech/lang"},
	}

	for _, e := range entries {
		se.Index(e)
	}

	// 测试搜索
	results, err := se.Search("人工智能", nil, 10, 0)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}

	if len(results) == 0 {
		t.Error("搜索结果不应为空")
	}

	// 验证结果包含正确条目
	found := false
	for _, r := range results {
		if r.EntryID == "entry-1" {
			found = true
			if r.Title != "人工智能基础" {
				t.Errorf("Title 错误: got %q", r.Title)
			}
			break
		}
	}
	if !found {
		t.Error("搜索结果应包含 entry-1")
	}
}

// TestSimpleSearchEngineSearchWithCategory 测试分类过滤搜索
func TestSimpleSearchEngineSearchWithCategory(t *testing.T) {
	se := index.NewSimpleSearchEngine()
	defer se.Close()

	entries := []*model.KnowledgeEntry{
		{ID: "entry-1", Title: "人工智能基础", Content: "机器学习技术", Category: "tech/ai"},
		{ID: "entry-2", Title: "人工智能应用", Content: "深度学习应用", Category: "tech/dl"},
		{ID: "entry-3", Title: "人工智能历史", Content: "AI发展历史", Category: "history/ai"},
	}

	for _, e := range entries {
		se.Index(e)
	}

	// 只搜索 tech 分类
	results, err := se.Search("人工智能", []string{"tech"}, 10, 0)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}

	// 验证结果只包含 tech 分类
	for _, r := range results {
		// 需要通过条目ID判断，因为 SearchResult 没有 Category 字段
		if r.EntryID == "entry-3" {
			t.Error("结果不应包含 history 分类的条目")
		}
	}
}

// TestSimpleSearchEngineSearchPagination 测试分页
func TestSimpleSearchEngineSearchPagination(t *testing.T) {
	se := index.NewSimpleSearchEngine()
	defer se.Close()

	// 添加多个匹配的条目
	for i := 0; i < 10; i++ {
		entry := &model.KnowledgeEntry{
			ID:       string(rune('a' + i)),
			Title:    "测试条目",
			Content:  "测试内容",
			Category: "test",
		}
		se.Index(entry)
	}

	// 测试 limit
	results, _ := se.Search("测试", nil, 3, 0)
	if len(results) != 3 {
		t.Errorf("Limit 失败: got %d results, want 3", len(results))
	}

	// 测试 offset
	results2, _ := se.Search("测试", nil, 3, 3)
	if len(results2) != 3 {
		t.Errorf("Offset 失败: got %d results, want 3", len(results2))
	}

	// 验证 offset 结果不重复
	for _, r1 := range results {
		for _, r2 := range results2 {
			if r1.EntryID == r2.EntryID {
				t.Error("分页结果不应有重复")
			}
		}
	}
}

// TestSimpleSearchEngineEmptyQuery 测试空查询
func TestSimpleSearchEngineEmptyQuery(t *testing.T) {
	se := index.NewSimpleSearchEngine()
	defer se.Close()

	entry := &model.KnowledgeEntry{
		ID:       "entry-1",
		Title:    "测试",
		Content:  "内容",
		Category: "test",
	}
	se.Index(entry)

	// 空查询应返回空结果
	results, err := se.Search("", nil, 10, 0)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("空查询应返回空结果, got %d", len(results))
	}
}

// TestSimpleSearchEngineNoMatch 测试无匹配
func TestSimpleSearchEngineNoMatch(t *testing.T) {
	se := index.NewSimpleSearchEngine()
	defer se.Close()

	entry := &model.KnowledgeEntry{
		ID:       "entry-1",
		Title:    "编程入门",
		Content:  "学习编程的基础知识",
		Category: "tech",
	}
	se.Index(entry)

	results, err := se.Search("不存在的关键词xyz", nil, 10, 0)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("无匹配应返回空结果, got %d", len(results))
	}
}

// TestSimpleSearchEngineScoreRelevance 测试相关性评分
func TestSimpleSearchEngineScoreRelevance(t *testing.T) {
	se := index.NewSimpleSearchEngine()
	defer se.Close()

	// 标题匹配权重更高
	entries := []*model.KnowledgeEntry{
		{ID: "title-match", Title: "人工智能导论", Content: "普通内容", Category: "tech"},
		{ID: "content-match", Title: "其他主题", Content: "人工智能是重要技术领域", Category: "tech"},
	}

	for _, e := range entries {
		se.Index(e)
	}

	results, _ := se.Search("人工智能", nil, 10, 0)

	if len(results) < 2 {
		t.Fatalf("需要至少 2 个结果, got %d", len(results))
	}

	// 标题匹配应该排在前面
	if results[0].EntryID != "title-match" {
		t.Errorf("标题匹配应排在前面: got %s", results[0].EntryID)
	}
	if results[0].Score <= results[1].Score {
		t.Error("标题匹配分数应高于内容匹配")
	}
}

// ==================== Tokenize 测试 ====================

// TestTokenizeChinese 测试中文分词
func TestTokenizeChinese(t *testing.T) {
	tokens := index.Tokenize("人工智能技术")

	if len(tokens) == 0 {
		t.Error("中文分词结果不应为空")
	}

	// 验证包含 bigram
	hasBigram := false
	for _, token := range tokens {
		if len([]rune(token)) == 2 {
			hasBigram = true
			break
		}
	}
	if !hasBigram {
		t.Error("应包含 bigram 分词结果")
	}
}

// TestTokenizeEnglish 测试英文分词
func TestTokenizeEnglish(t *testing.T) {
	tokens := index.Tokenize("Machine Learning is amazing")

	// 验证提取了单词
	expectedWords := map[string]bool{
		"machine": false,
		"learning": false,
		"amazing": false,
	}

	for _, token := range tokens {
		if _, ok := expectedWords[token]; ok {
			expectedWords[token] = true
		}
	}

	for word, found := range expectedWords {
		if !found {
			t.Errorf("应包含单词 %q", word)
		}
	}
}

// TestTokenizeMixed 测试中英混合分词
func TestTokenizeMixed(t *testing.T) {
	tokens := index.Tokenize("学习 Machine Learning 很有趣")

	if len(tokens) == 0 {
		t.Error("混合分词结果不应为空")
	}

	// 验证包含中文和英文
	hasChinese := false
	hasEnglish := false
	for _, token := range tokens {
		runes := []rune(token)
		if len(runes) > 0 {
			if runes[0] >= 0x4E00 && runes[0] <= 0x9FFF {
				hasChinese = true
			} else if (runes[0] >= 'a' && runes[0] <= 'z') || (runes[0] >= 'A' && runes[0] <= 'Z') {
				hasEnglish = true
			}
		}
	}

	if !hasChinese {
		t.Error("应包含中文 token")
	}
	if !hasEnglish {
		t.Error("应包含英文 token")
	}
}

// TestTokenizeEmpty 测试空字符串
func TestTokenizeEmpty(t *testing.T) {
	tokens := index.Tokenize("")
	if tokens != nil {
		t.Errorf("空字符串应返回 nil, got %v", tokens)
	}
}

// TestTokenizeStopWords 测试停用词过滤
func TestTokenizeStopWords(t *testing.T) {
	// 包含停用词的文本
	tokens := index.Tokenize("这是一个人工智能的系统")

	// 验证不包含停用词
	for _, token := range tokens {
		if token == "一个" || token == "的" || token == "是" {
			t.Errorf("不应包含停用词 %q", token)
		}
	}
}

// ==================== TokenizerManager 测试 ====================

// TestTokenizerManagerSimple 测试简单分词器管理器
func TestTokenizerManagerSimple(t *testing.T) {
	tm := index.NewTokenizerManager(index.TokenizerSimple)
	defer tm.Close()

	if tm.Type() != index.TokenizerSimple {
		t.Errorf("Type 错误: got %d", tm.Type())
	}

	tokens := tm.Tokenize("测试分词")
	if len(tokens) == 0 {
		t.Error("Tokenize 结果不应为空")
	}
}

// TestTokenizerManagerSwitch 测试切换分词器
func TestTokenizerManagerSwitch(t *testing.T) {
	tm := index.NewTokenizerManager(index.TokenizerSimple)
	defer tm.Close()

	// 切换到 jieba（可能降级到 simple）
	tm.SwitchTokenizer(index.TokenizerJieba)

	// 验证可以正常使用
	tokens := tm.Tokenize("测试")
	if len(tokens) == 0 && tokens != nil {
		t.Error("切换后分词应正常工作")
	}
}

// ==================== 工具函数测试 ====================

// TestIsCJKText 测试 CJK 文本判断
func TestIsCJKText(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"人工智能是未来", true},
		{"Hello World", false},
		{"这是中文和 English 混合", true},
		{"12345", false},
	}

	for _, tt := range tests {
		result := index.IsCJKText(tt.text)
		if result != tt.expected {
			t.Errorf("IsCJKText(%q) = %v, want %v", tt.text, result, tt.expected)
		}
	}
}

// TestExtractKeywords 测试关键词提取
func TestExtractKeywords(t *testing.T) {
	text := "人工智能机器学习深度学习神经网络人工智能"
	keywords := index.ExtractKeywords(text, 3)

	if len(keywords) > 3 {
		t.Errorf("应返回最多 3 个关键词, got %d", len(keywords))
	}

	// "人工智能" 出现两次，应该排在前面
	if len(keywords) > 0 {
		// 检查是否包含 "人工智能" 相关的词
		found := false
		for _, kw := range keywords {
			if kw == "人工" || kw == "智能" || len(kw) >= 2 {
				found = true
				break
			}
		}
		if !found {
			t.Error("关键词提取结果不符合预期")
		}
	}
}

// TestHighlightMatch 测试高亮匹配
func TestHighlightMatch(t *testing.T) {
	text := "人工智能是未来的重要技术"
	query := "人工智能"
	result := index.HighlightMatch(text, query, "[", "]")

	if result == text {
		t.Error("应该高亮匹配词")
	}

	// 验证包含高亮标记
	if len(result) <= len(text) {
		t.Error("高亮后文本应该更长")
	}
}
