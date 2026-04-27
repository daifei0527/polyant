# Search Keyword Indexing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在搜索结果的 Markdown content 中插入已有词条标题的链接，并在 API 响应中返回知识图谱数据 (nodes + edges)。

**Architecture:** 新增 TitleIndex（AC 自动机维护所有 published 词条标题）和 ResultEnricher（匹配标题 + 插入 Markdown 链接 + 构建图谱）两个模块。TitleIndex 在 Store 初始化时全量构建，EntryHandler CRUD 时增量同步。ResultEnricher 在 SearchHandler 返回前调用。

**Tech Stack:** Go 标准库，无新增外部依赖（AC 自动机自实现，约 80 行）。

---

### Task 1: TitleIndex — AC 自动机核心实现

**Files:**
- Create: `internal/storage/index/title_index.go`
- Create: `internal/storage/index/title_index_test.go`

- [ ] **Step 1: 写 TitleIndex 测试骨架**

```go
package index

import (
	"testing"
)

func TestTitleIndex_BuildAndMatchAll(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "e1", Title: "神经网络"},
		{ID: "e2", Title: "深度学习"},
		{ID: "e3", Title: "机器学习"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	content := "深度学习是机器学习的一个分支，神经网络是其中的关键技术。"
	matches := ti.MatchAll(content)

	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d: %v", len(matches), matches)
	}

	// 验证匹配按 offset 排序，且 title 和 entryId 正确
	for _, m := range matches {
		if m.Title == "" || m.EntryID == "" {
			t.Errorf("match missing Title or EntryID: %+v", m)
		}
	}
}

func TestTitleIndex_MatchAll_NoMatch(t *testing.T) {
	ti := NewTitleIndex()
	ti.Build([]TitleEntry{{ID: "e1", Title: "神经网络"}})

	matches := ti.MatchAll("这是一段不包含任何匹配的文本。")
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

func TestTitleIndex_MatchAll_EmptyInput(t *testing.T) {
	ti := NewTitleIndex()
	ti.Build([]TitleEntry{{ID: "e1", Title: "测试"}})

	if matches := ti.MatchAll(""); len(matches) != 0 {
		t.Fatalf("expected 0 matches for empty content, got %d", len(matches))
	}
}

func TestTitleIndex_MatchAll_Overlapping(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "e1", Title: "机器学习"},
		{ID: "e2", Title: "机器学习系统"},
	}
	ti.Build(entries)

	content := "机器学习系统是一个复杂的学科。"
	matches := ti.MatchAll(content)

	// 只保留最长匹配「机器学习系统」，排除「机器学习」
	for _, m := range matches {
		if m.Title == "机器学习" {
			t.Errorf("shorter overlapping match should be filtered: %+v", m)
		}
	}
}

func TestTitleIndex_MatchAll_Chinese(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "e1", Title: "深度学习"},
		{ID: "e2", Title: "自然语言处理"},
	}
	ti.Build(entries)

	matches := ti.MatchAll("自然语言处理是深度学习的一个重要方向。")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches for Chinese text, got %d", len(matches))
	}
}

func TestTitleIndex_Add(t *testing.T) {
	ti := NewTitleIndex()
	ti.Build([]TitleEntry{{ID: "e1", Title: "深度学习"}})

	// 增量添加新模式
	if err := ti.Add(TitleEntry{ID: "e2", Title: "Transformer"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	matches := ti.MatchAll("Transformer 是深度学习的关键架构。")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches after Add, got %d", len(matches))
	}
}

func TestTitleIndex_Remove(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "e1", Title: "深度学习"},
		{ID: "e2", Title: "机器学习"},
	}
	ti.Build(entries)

	if err := ti.Remove("深度学习"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	matches := ti.MatchAll("深度学习是机器学习的一个分支。")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match after Remove, got %d", len(matches))
	}
	if matches[0].Title != "机器学习" {
		t.Errorf("expected remaining match '机器学习', got %s", matches[0].Title)
	}
}

func TestTitleIndex_Update(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "e1", Title: "深度学习"},
		{ID: "e2", Title: "机器学习"},
	}
	ti.Build(entries)

	if err := ti.Update(
		TitleEntry{ID: "e1", Title: "深度学习"},
		TitleEntry{ID: "e1", Title: "深度神经网络"},
	); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	matches := ti.MatchAll("深度神经网络是机器学习的一个分支。")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches after Update, got %d", len(matches))
	}
	// 旧标题不应再匹配
	matches2 := ti.MatchAll("深度学习是热门领域。")
	found := false
	for _, m := range matches2 {
		if m.Title == "深度学习" {
			found = true
		}
	}
	if found {
		t.Error("old title should no longer match after Update")
	}
}

func TestTitleIndex_MatchAll_SpecialChars(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "e1", Title: "C++"},
		{ID: "e2", Title: "Go (语言)"},
	}
	ti.Build(entries)

	matches := ti.MatchAll("C++ 和 Go (语言) 都是常用的编程语言。")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches for special chars, got %d", len(matches))
	}
}

func TestTitleIndex_Concurrent(t *testing.T) {
	ti := NewTitleIndex()
	ti.Build([]TitleEntry{
		{ID: "e1", Title: "并发测试"},
	})

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			ti.MatchAll("这是一个并发测试的文本内容。")
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
go test -v -run TestTitleIndex ./internal/storage/index/...
```

Expected: compilation error — `NewTitleIndex`, `TitleIndex`, `TitleEntry`, `Match`, `Build`, `MatchAll`, `Add`, `Remove`, `Update` 未定义。

- [ ] **Step 3: 实现 TitleIndex**

```go
package index

import "sync"

// TitleEntry 词条标题条目
type TitleEntry struct {
	ID    string
	Title string
}

// Match 标题匹配结果
type Match struct {
	Title   string `json:"title"`
	EntryID string `json:"entryId"`
	Offset  int    `json:"offset"`
}

// acNode AC 自动机节点
type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	output   []matchedPattern
}

type matchedPattern struct {
	title string
	id    string
}

// TitleIndex 基于 AC 自动机的标题索引
type TitleIndex struct {
	root    *acNode
	entries map[string]TitleEntry // title → entry
	mu      sync.RWMutex
}

// NewTitleIndex 创建新的 TitleIndex
func NewTitleIndex() *TitleIndex {
	return &TitleIndex{
		root:    &acNode{children: make(map[rune]*acNode)},
		entries: make(map[string]TitleEntry),
	}
}

// Build 全量构建索引
func (t *TitleIndex) Build(entries []TitleEntry) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.root = &acNode{children: make(map[rune]*acNode)}
	t.entries = make(map[string]TitleEntry)

	for _, e := range entries {
		if e.Title == "" {
			continue
		}
		t.entries[e.Title] = e
		t.insert(e.Title, e.ID)
	}
	t.buildFailLinks()
	return nil
}

// Add 增量添加一个标题
func (t *TitleIndex) Add(entry TitleEntry) error {
	if entry.Title == "" {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[entry.Title] = entry
	t.insert(entry.Title, entry.ID)
	t.buildFailLinks()
	return nil
}

// Remove 删除一个标题（触发重建）
func (t *TitleIndex) Remove(title string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.entries, title)
	// 用剩余条目重建自动机
	t.rebuildLocked()
	return nil
}

// Update 更新标题（触发重建）
func (t *TitleIndex) Update(old, new TitleEntry) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.entries, old.Title)
	if new.Title != "" {
		t.entries[new.Title] = new
	}
	t.rebuildLocked()
	return nil
}

// rebuildLocked 在持有写锁时全量重建自动机
func (t *TitleIndex) rebuildLocked() {
	t.root = &acNode{children: make(map[rune]*acNode)}
	for _, e := range t.entries {
		t.insert(e.Title, e.ID)
	}
	t.buildFailLinks()
}

// insert 将单个模式串插入 trie
func (t *TitleIndex) insert(pattern, id string) {
	node := t.root
	for _, ch := range pattern {
		if node.children[ch] == nil {
			node.children[ch] = &acNode{children: make(map[rune]*acNode)}
		}
		node = node.children[ch]
	}
	node.output = append(node.output, matchedPattern{title: pattern, id: id})
}

// buildFailLinks 构建所有节点的失效链接（BFS）
func (t *TitleIndex) buildFailLinks() {
	queue := make([]*acNode, 0)

	// 根的直接子节点 fail 指向根
	for _, child := range t.root.children {
		child.fail = t.root
		queue = append(queue, child)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for ch, child := range current.children {
			// 查找 child 的 fail 节点
			failNode := current.fail
			for failNode != nil {
				if failNode.children[ch] != nil {
					child.fail = failNode.children[ch]
					break
				}
				failNode = failNode.fail
			}
			if child.fail == nil {
				child.fail = t.root
			}
			// 合并 fail 节点的输出
			if child.fail.output != nil {
				child.output = append(child.output, child.fail.output...)
			}
			queue = append(queue, child)
		}
	}
}

// MatchAll 在 content 中找出所有匹配的标题
func (t *TitleIndex) MatchAll(content string) []Match {
	t.mu.RLock()
	defer t.mu.RUnlock()

	runes := []rune(content)
	var matches []Match

	node := t.root
	for i, ch := range runes {
		// 跟随失效链接直到找到匹配
		for node != t.root && node.children[ch] == nil {
			node = node.fail
		}
		if node.children[ch] != nil {
			node = node.children[ch]
		}
		// 收集当前位置的所有输出
		for _, p := range node.output {
			matchLen := len([]rune(p.title))
			start := i - matchLen + 1
			if start >= 0 {
				matches = append(matches, Match{
					Title:   p.title,
					EntryID: p.id,
					Offset:  start,
				})
			}
		}
	}
	return matches
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
go test -v -race -run TestTitleIndex ./internal/storage/index/...
```

Expected: ALL PASS，无 race condition。

- [ ] **Step 5: Commit**

```bash
git add internal/storage/index/title_index.go internal/storage/index/title_index_test.go
git commit -m "feat(index): add TitleIndex with Aho-Corasick automaton for entry title matching"
```

---

### Task 2: ResultEnricher — 链接插入 + 图谱构建

**Files:**
- Create: `internal/api/handler/search_enricher.go`
- Create: `internal/api/handler/search_enricher_test.go`

- [ ] **Step 1: 写 ResultEnricher 测试**

```go
package handler

import (
	"strings"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestEnricher_InsertLinks_Basic(t *testing.T) {
	ti := index.NewTitleIndex()
	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})

	memStore, _ := storage.NewMemoryStore()
	enricher := NewResultEnricher(ti, memStore.Entry)

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Title:   "深度学习入门",
		Content: "深度学习使用神经网络来处理数据。",
	}
	entries := []*model.KnowledgeEntry{entry}

	graph, err := enricher.Enrich(entries)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	expected := "[神经网络](entry://e1)"
	if !strings.Contains(entry.Content, expected) {
		t.Errorf("content missing link, got: %s", entry.Content)
	}

	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}
}

func TestEnricher_InsertLinks_Multiple(t *testing.T) {
	ti := index.NewTitleIndex()
	ti.Build([]index.TitleEntry{
		{ID: "e1", Title: "神经网络"},
		{ID: "e2", Title: "深度学习"},
	})

	memStore, _ := storage.NewMemoryStore()
	enricher := NewResultEnricher(ti, memStore.Entry)

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Content: "深度学习推动神经网络发展。",
	}
	enricher.Enrich([]*model.KnowledgeEntry{entry})

	if !strings.Contains(entry.Content, "[深度学习](entry://e2)") {
		t.Errorf("missing link for '深度学习', got: %s", entry.Content)
	}
	if !strings.Contains(entry.Content, "[神经网络](entry://e1)") {
		t.Errorf("missing link for '神经网络', got: %s", entry.Content)
	}
}

func TestEnricher_InsertLinks_SkipCodeBlock(t *testing.T) {
	ti := index.NewTitleIndex()
	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})

	memStore, _ := storage.NewMemoryStore()
	enricher := NewResultEnricher(ti, memStore.Entry)

	entry := &model.KnowledgeEntry{
		ID: "e99",
		Content: "```go\nimport \"神经网络\"\n```\n\n这里提到神经网络。",
	}
	enricher.Enrich([]*model.KnowledgeEntry{entry})

	// 代码块内的「神经网络」不应被替换
	count := strings.Count(entry.Content, "[神经网络](entry://e1)")
	if count != 1 {
		t.Errorf("expected 1 link outside code block, got %d. Content: %s", count, entry.Content)
	}
}

func TestEnricher_InsertLinks_SkipInlineCode(t *testing.T) {
	ti := index.NewTitleIndex()
	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})

	memStore, _ := storage.NewMemoryStore()
	enricher := NewResultEnricher(ti, memStore.Entry)

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Content: "`神经网络` 是一种模型。",
	}
	enricher.Enrich([]*model.KnowledgeEntry{entry})

	if strings.Contains(entry.Content, "[神经网络](entry://e1)") {
		t.Errorf("inline code should not be linked, got: %s", entry.Content)
	}
}

func TestEnricher_InsertLinks_SkipExistingLink(t *testing.T) {
	ti := index.NewTitleIndex()
	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})

	memStore, _ := storage.NewMemoryStore()
	enricher := NewResultEnricher(ti, memStore.Entry)

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Content: "参考 [神经网络](https://example.com) 了解更多。",
	}
	enricher.Enrich([]*model.KnowledgeEntry{entry})

	// 已有链接中的「神经网络」不应再次被替换
	if strings.Count(entry.Content, "[神经网络]") != 1 {
		t.Errorf("existing link text should not be wrapped again, got: %s", entry.Content)
	}
}

func TestEnricher_InsertLinks_SkipURL(t *testing.T) {
	ti := index.NewTitleIndex()
	ti.Build([]index.TitleEntry{{ID: "e1", Title: "model"}})

	memStore, _ := storage.NewMemoryStore()
	enricher := NewResultEnricher(ti, memStore.Entry)

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Content: "See https://model.ai for details.",
	}
	enricher.Enrich([]*model.KnowledgeEntry{entry})

	if strings.Contains(entry.Content, "[model](entry://e1)") {
		t.Errorf("URL should not be linked, got: %s", entry.Content)
	}
}

func TestEnricher_InsertLinks_SelfRef(t *testing.T) {
	ti := index.NewTitleIndex()
	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})

	memStore, _ := storage.NewMemoryStore()
	enricher := NewResultEnricher(ti, memStore.Entry)

	entry := &model.KnowledgeEntry{
		ID:      "e1", // same ID
		Content: "神经网络是一种模型。",
	}
	enricher.Enrich([]*model.KnowledgeEntry{entry})

	if strings.Contains(entry.Content, "[神经网络](entry://e1)") {
		t.Errorf("self-reference should not be linked, got: %s", entry.Content)
	}
}

func TestEnricher_BuildGraph_MultipleResults(t *testing.T) {
	ti := index.NewTitleIndex()
	ti.Build([]index.TitleEntry{
		{ID: "e1", Title: "神经网络"},
		{ID: "e2", Title: "反向传播"},
	})

	memStore, _ := storage.NewMemoryStore()
	enricher := NewResultEnricher(ti, memStore.Entry)

	entries := []*model.KnowledgeEntry{
		{ID: "a1", Content: "神经网络使用反向传播。"},
		{ID: "a2", Content: "反向传播是神经网络的核心算法。"},
	}
	graph, _ := enricher.Enrich(entries)

	// 应该有 4 个节点: a1, a2 (result) + e1, e2 (reference)
	if len(graph.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(graph.Nodes))
	}
	// 应该有 4 条边: a1→e1, a1→e2, a2→e2, a2→e1
	if len(graph.Edges) != 4 {
		t.Fatalf("expected 4 edges, got %d", len(graph.Edges))
	}

	// 验证节点类型
	for _, n := range graph.Nodes {
		if n.ID == "a1" || n.ID == "a2" {
			if n.Type != "result" {
				t.Errorf("node %s should be 'result', got '%s'", n.ID, n.Type)
			}
		}
	}
	// 验证 relation
	for _, e := range graph.Edges {
		if e.Relation != "mentions" {
			t.Errorf("edge %s→%s should be 'mentions', got '%s'", e.From, e.To, e.Relation)
		}
	}
}

func TestEnricher_BuildGraph_NoReferences(t *testing.T) {
	ti := index.NewTitleIndex()
	ti.Build([]index.TitleEntry{{ID: "e1", Title: "量子计算"}})

	memStore, _ := storage.NewMemoryStore()
	enricher := NewResultEnricher(ti, memStore.Entry)

	entries := []*model.KnowledgeEntry{
		{ID: "a1", Content: "这是一段没有匹配的文本。"},
	}
	graph, _ := enricher.Enrich(entries)

	// 只有 result 节点，没有 reference 节点和边
	if len(graph.Nodes) != 1 {
		t.Fatalf("expected 1 result node, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(graph.Edges))
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
go test -v -race -run TestEnricher ./internal/api/handler/...
```

Expected: compilation error — `NewResultEnricher`、`ResultEnricher`、`SearchGraph`、`GraphNode`、`GraphEdge`、`Enrich` 未定义。

- [ ] **Step 3: 实现 ResultEnricher**

```go
package handler

import (
	"strings"
	"unicode/utf8"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// GraphNode 图谱节点
type GraphNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"` // "result" | "reference"
}

// GraphEdge 图谱边
type GraphEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"` // "mentions"
}

// SearchGraph 搜索图谱
type SearchGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// ResultEnricher 搜索结果增强器
type ResultEnricher struct {
	titleIndex *index.TitleIndex
	entryStore storage.EntryStore
}

// NewResultEnricher 创建 ResultEnricher
func NewResultEnricher(ti *index.TitleIndex, es storage.EntryStore) *ResultEnricher {
	return &ResultEnricher{
		titleIndex: ti,
		entryStore: es,
	}
}

// Enrich 对搜索结果做增强：插入 Markdown 链接 + 构建图谱
func (e *ResultEnricher) Enrich(entries []*model.KnowledgeEntry) (*SearchGraph, error) {
	nodeSet := make(map[string]GraphNode)
	edgeSet := make(map[string]GraphEdge) // "from→to" as key

	for _, entry := range entries {
		// 保证 content 不为空
		if entry.Content == "" {
			full, err := e.entryStore.Get(nil, entry.ID)
			if err == nil && full != nil {
				entry.Content = full.Content
			}
		}

		// 注册 result 节点
		nodeSet[entry.ID] = GraphNode{ID: entry.ID, Title: entry.Title, Type: "result"}

		if entry.Content == "" {
			continue
		}

		// 保护区分扫描
		protectionZones := scanProtectionZones(entry.Content)

		// 查找匹配
		matches := e.titleIndex.MatchAll(entry.Content)

		// 过滤 + 插入链接
		filtered := filterMatches(matches, protectionZones, entry.ID)
		entry.Content = insertLinks(entry.Content, filtered)

		// 构建图谱
		for _, m := range filtered {
			nodeSet[m.EntryID] = GraphNode{ID: m.EntryID, Title: m.Title, Type: "reference"}
			edgeKey := entry.ID + "→" + m.EntryID
			if _, ok := edgeSet[edgeKey]; !ok {
				edgeSet[edgeKey] = GraphEdge{From: entry.ID, To: m.EntryID, Relation: "mentions"}
			}
		}
	}

	nodes := make([]GraphNode, 0, len(nodeSet))
	for _, n := range nodeSet {
		nodes = append(nodes, n)
	}
	edges := make([]GraphEdge, 0, len(edgeSet))
	for _, e := range edgeSet {
		edges = append(edges, e)
	}

	return &SearchGraph{Nodes: nodes, Edges: edges}, nil
}

// protZone 保护区区间（字节偏移）
type protZone struct {
	start int // 字节偏移
	end   int // 字节偏移 (不含)
}

// scanProtectionZones 扫描 Markdown 内容中的保护区
func scanProtectionZones(content string) []protZone {
	var zones []protZone
	b := []byte(content)

	// 1. 代码块: ``` 配对
	zones = scanCodeBlocks(b, zones)

	// 2. 行内代码: ` 配对
	zones = scanInlineCode(b, zones)

	// 3. 已有 Markdown 链接: [text](url)
	zones = scanExistingLinks(b, zones)

	// 4. 图片: ![alt](url)
	zones = scanImageLinks(b, zones)

	// 5. URL: http(s)://...
	zones = scanURLs(b, zones)

	return mergeZones(zones)
}

func scanCodeBlocks(b []byte, zones []protZone) []protZone {
	for i := 0; i <= len(b)-3; i++ {
		if b[i] == '`' && b[i+1] == '`' && b[i+2] == '`' {
			start := i
			// 找到结束的 ```
			for j := i + 3; j <= len(b)-3; j++ {
				if b[j] == '`' && b[j+1] == '`' && b[j+2] == '`' {
					zones = append(zones, protZone{start: start, end: j + 3})
					i = j + 2
					break
				}
			}
		}
	}
	return zones
}

func scanInlineCode(b []byte, zones []protZone) []protZone {
	for i := 0; i < len(b); i++ {
		if b[i] == '`' {
			start := i
			for j := i + 1; j < len(b); j++ {
				if b[j] == '`' {
					if j > start+1 {
						zones = append(zones, protZone{start: start, end: j + 1})
					}
					i = j
					break
				}
			}
		}
	}
	return zones
}

func scanExistingLinks(b []byte, zones []protZone) []protZone {
	for i := 0; i < len(b); i++ {
		if b[i] == '[' {
			closeBracket := -1
			for j := i + 1; j < len(b); j++ {
				if b[j] == ']' {
					closeBracket = j
					break
				}
			}
			if closeBracket > i+1 && closeBracket+1 < len(b) && b[closeBracket+1] == '(' {
				// 找到 )
				for k := closeBracket + 2; k < len(b); k++ {
					if b[k] == ')' {
						zones = append(zones, protZone{start: i, end: k + 1})
						i = k
						break
					}
				}
			}
		}
	}
	return zones
}

func scanImageLinks(b []byte, zones []protZone) []protZone {
	for i := 0; i <= len(b)-2; i++ {
		if b[i] == '!' && b[i+1] == '[' {
			closeBracket := -1
			for j := i + 2; j < len(b); j++ {
				if b[j] == ']' {
					closeBracket = j
					break
				}
			}
			if closeBracket > i+2 && closeBracket+1 < len(b) && b[closeBracket+1] == '(' {
				for k := closeBracket + 2; k < len(b); k++ {
					if b[k] == ')' {
						zones = append(zones, protZone{start: i, end: k + 1})
						i = k
						break
					}
				}
			}
		}
	}
	return zones
}

func scanURLs(b []byte, zones []protZone) []protZone {
	// 简单匹配 http:// 或 https:// 开头的 URL
	for i := 0; i < len(b); i++ {
		if (len(b)-i >= 7 && string(b[i:i+7]) == "http://") ||
			(len(b)-i >= 8 && string(b[i:i+8]) == "https://") {
			start := i
			for j := i; j < len(b); j++ {
				if b[j] == ' ' || b[j] == '\n' || b[j] == '\t' || b[j] == ')' {
					zones = append(zones, protZone{start: start, end: j})
					i = j
					break
				}
				if j == len(b)-1 {
					zones = append(zones, protZone{start: start, end: len(b)})
					i = j
					break
				}
			}
		}
	}
	return zones
}

func mergeZones(zones []protZone) []protZone {
	if len(zones) <= 1 {
		return zones
	}
	// 简单冒泡排序
	for i := 0; i < len(zones)-1; i++ {
		for j := i + 1; j < len(zones); j++ {
			if zones[j].start < zones[i].start {
				zones[i], zones[j] = zones[j], zones[i]
			}
		}
	}
	// 合并重叠
	merged := []protZone{zones[0]}
	for i := 1; i < len(zones); i++ {
		last := &merged[len(merged)-1]
		if zones[i].start <= last.end {
			if zones[i].end > last.end {
				last.end = zones[i].end
			}
		} else {
			merged = append(merged, zones[i])
		}
	}
	return merged
}

func isInProtectionZone(offset int, zones []protZone) bool {
	for _, z := range zones {
		if offset >= z.start && offset < z.end {
			return true
		}
	}
	return false
}

// filterMatches 过滤保护区内的匹配，处理重叠匹配（最长优先）
func filterMatches(matches []index.Match, zones []protZone, selfID string) []index.Match {
	var filtered []index.Match

	// 自引用过滤 + 保护区过滤
	for _, m := range matches {
		if m.EntryID == selfID {
			continue
		}
		if isInProtectionZone(m.Offset, zones) {
			continue
		}
		filtered = append(filtered, m)
	}

	// 处理重叠匹配：按 offset 升序，同 offset 按长度降序
	sortMatches(filtered)

	// 移除被更长匹配覆盖的短匹配
	var result []index.Match
	for _, m := range filtered {
		overlapped := false
		for _, r := range result {
			rEnd := r.Offset + utf8.RuneCountInString(r.Title)
			mEnd := m.Offset + utf8.RuneCountInString(m.Title)
			// 如果 m 被 r 完全覆盖，跳过
			if m.Offset >= r.Offset && mEnd <= rEnd {
				overlapped = true
				break
			}
		}
		if !overlapped {
			result = append(result, m)
		}
	}

	return result
}

func sortMatches(matches []index.Match) {
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Offset < matches[i].Offset {
				matches[i], matches[j] = matches[j], matches[i]
			} else if matches[j].Offset == matches[i].Offset {
				if len(matches[j].Title) > len(matches[i].Title) {
					matches[i], matches[j] = matches[j], matches[i]
				}
			}
		}
	}
}

// insertLinks 从后往前插入 Markdown 链接（不破坏靠前的 offset）
func insertLinks(content string, matches []index.Match) string {
	if len(matches) == 0 {
		return content
	}

	// 按 offset 排序
	sortMatches(matches)

	b := []byte(content)
	var result []byte
	prevEnd := 0

	for _, m := range matches {
		titleLen := len([]byte(m.Title))
		// 从 prevEnd 到 m.Offset 的原始内容
		result = append(result, b[prevEnd:m.Offset]...)
		// 插入 Markdown 链接
		link := "["
		link += m.Title
		link += "](entry://"
		link += m.EntryID
		link += ")"
		result = append(result, []byte(link)...)
		prevEnd = m.Offset + titleLen
	}
	// 剩余内容
	result = append(result, b[prevEnd:]...)

	return string(result)
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
go test -v -race -run TestEnricher ./internal/api/handler/...
```

Expected: ALL PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/search_enricher.go internal/api/handler/search_enricher_test.go
git commit -m "feat(handler): add ResultEnricher for search result link insertion and graph building"
```

---

### Task 3: API 类型变更 + Store 集成 + EntryHandler 改动

**Files:**
- Modify: `internal/api/handler/types.go:13-17`
- Modify: `internal/api/handler/entry_handler.go:26-44,53-145,238-246,334-342,394-400`
- Modify: `internal/storage/store.go:103-112,150-190`

- [ ] **Step 1: PagedData 增加 Graph 字段**

```go
// 修改 internal/api/handler/types.go

type PagedData struct {
	TotalCount int         `json:"total_count"`
	HasMore    bool        `json:"has_more"`
	Items      interface{} `json:"items"`
	Graph      interface{} `json:"graph,omitempty"`
}
```

- [ ] **Step 2: Store 增加 TitleIndex 字段并在工厂方法中初始化**

修改 `internal/storage/store.go`：
- `Store` 结构体增加 `TitleIdx *index.TitleIndex`
- `NewMemoryStore()` 中创建 `TitleIdx` 并从 entry store 加载已发布条目标题
- `NewPersistentStore()` 中创建 `TitleIdx` 并从 entry store 加载已发布条目标题

```go
// Store 结构体新增字段
type Store struct {
	Entry    EntryStore
	User     UserStore
	Rating   RatingStore
	Category CategoryStore
	Search   index.SearchEngine
	Backlink BacklinkIndex
	TitleIdx *index.TitleIndex  // 新增
	Audit    kv.AuditStore
	kvStore  kv.Store
}

// NewMemoryStore 新增 TitleIdx 初始化
func NewMemoryStore() (*Store, error) {
	// ... 现有代码 ...
	titleIdx := index.NewTitleIndex()
	// 加载已有 published 条目标题
	entries, _, _ := entryStore.List(context.Background(), EntryFilter{Status: model.EntryStatusPublished, Limit: 100000})
	titleEntries := make([]index.TitleEntry, 0, len(entries))
	for _, e := range entries {
		titleEntries = append(titleEntries, index.TitleEntry{ID: e.ID, Title: e.Title})
	}
	titleIdx.Build(titleEntries)

	return &Store{
		// ... 现有字段 ...
		TitleIdx: titleIdx,
		// ...
	}, nil
}

// NewPersistentStore 新增 TitleIdx 初始化
func NewPersistentStore(cfg *StoreConfig) (*Store, error) {
	// ... 现有 kv store + search engine 创建代码 ...

	entryStore := NewBadgerEntryStore(kvStore)
	titleIdx := index.NewTitleIndex()
	entries, _, _ := entryStore.List(context.Background(), EntryFilter{Status: model.EntryStatusPublished, Limit: 100000})
	titleEntries := make([]index.TitleEntry, 0, len(entries))
	for _, e := range entries {
		titleEntries = append(titleEntries, index.TitleEntry{ID: e.ID, Title: e.Title})
	}
	titleIdx.Build(titleEntries)

	return &Store{
		Entry:    entryStore,
		// ... 现有字段 ...
		TitleIdx: titleIdx,
	}, nil
}
```

- [ ] **Step 3: EntryHandler 集成 ResultEnricher**

修改 `internal/api/handler/entry_handler.go`：

```go
// 新增字段
type EntryHandler struct {
	entryStore    storage.EntryStore
	searchEngine  index.SearchEngine
	backlink      storage.BacklinkIndex
	userStore     storage.UserStore
	remoteQuerier RemoteQuerier
	enricher      *ResultEnricher  // 新增
}

// NewEntryHandler 增加 titleIndex 参数
func NewEntryHandler(entryStore storage.EntryStore, searchEngine index.SearchEngine, backlinkIndex storage.BacklinkIndex, userStore storage.UserStore, titleIndex *index.TitleIndex) *EntryHandler {
	return &EntryHandler{
		entryStore:   entryStore,
		searchEngine: searchEngine,
		backlink:     backlinkIndex,
		userStore:    userStore,
		enricher:     NewResultEnricher(titleIndex, entryStore),
	}
}
```

**路由层适配** — 修改 `internal/api/router/router.go:85`：

```go
// 修改前
entryHandler := handler.NewEntryHandler(deps.EntryStore, deps.SearchEngine, deps.Backlink, deps.UserStore)

// 修改后
entryHandler := handler.NewEntryHandler(deps.EntryStore, deps.SearchEngine, deps.Backlink, deps.UserStore, deps.Store.TitleIdx)
```

**SearchHandler 集成** — 在返回响应前调用 Enricher：

```go
// 在 SearchHandler 的 result 获取后、writeJSON 前插入：

// 结果增强：插入词条链接 + 构建图谱
var graph *SearchGraph
if h.enricher != nil {
	var enrichErr error
	graph, enrichErr = h.enricher.Enrich(result.Entries)
	if enrichErr != nil {
		// 增强失败不影响搜索，只打日志
		log.Printf("[SearchHandler] enrich failed: %v", enrichErr)
	}
}

// 返回
writeJSON(w, http.StatusOK, &APIResponse{
	Code:    0,
	Message: "success",
	Data: &PagedData{
		TotalCount: result.TotalCount,
		HasMore:    hasMore,
		Items:      result.Entries,
		Graph:      graph,
	},
})
```

需要添加 `"log"` 到 entry_handler.go 的 import。

- [ ] **Step 4: CRUD 处理器同步 TitleIndex**

**CreateEntryHandler** — 创建 published 条目后添加：

```go
// 在 h.searchEngine.IndexEntry(created) 之后、反向链接建立之后插入
if h.enricher != nil {
	_ = h.enricher.titleIndex.Add(index.TitleEntry{ID: created.ID, Title: created.Title})
}
```

**UpdateEntryHandler** — 更新条目后同步标题变更：

```go
// 在 h.searchEngine.UpdateIndex(updated) 之后插入
if h.enricher != nil && req.Title != nil {
	oldEntry := index.TitleEntry{ID: existing.ID, Title: existing.Title}
	newEntry := index.TitleEntry{ID: updated.ID, Title: updated.Title}
	if oldEntry.Title != newEntry.Title {
		_ = h.enricher.titleIndex.Update(oldEntry, newEntry)
	}
}
```

**DeleteEntryHandler** — 删除条目后从 TitleIndex 移除：

```go
// 在 h.searchEngine.DeleteIndex(id) 之后插入
if h.enricher != nil {
	_ = h.enricher.titleIndex.Remove(existing.Title)
}
```

- [ ] **Step 5: 运行所有已有测试确保无回归**

```bash
go test -v -race ./internal/api/handler/...
go test -v -race ./internal/storage/...
```

Expected: ALL PASS（注意 `NewEntryHandler` 签名变了，需要更新所有相关测试文件中的调用）。

- [ ] **Step 6: 批量操作和测试文件适配**

查找所有调用 `NewEntryHandler` 的地方并适配新签名：

```bash
grep -rn "NewEntryHandler" --include="*.go" .
```

需要更新：
- `internal/api/handler/batch_handler.go`
- `internal/api/handler/entry_handler_test.go`
- `internal/api/handler/batch_handler_test.go`
- 等等

将 `nil` 或 `new(index.TitleIndex)` 作为最后一个参数传入。

- [ ] **Step 7: Commit**

```bash
git add internal/api/handler/types.go internal/api/handler/entry_handler.go internal/storage/store.go internal/api/router/router.go
git commit -m "feat: wire TitleIndex and ResultEnricher into search flow and CRUD lifecycle"
```

---

### Task 4: 集成测试

**Files:**
- Modify: `internal/api/handler/entry_handler_test.go`

- [ ] **Step 1: 写集成测试**

```go
func TestSearchHandler_WithGraph(t *testing.T) {
	memStore, _ := storage.NewMemoryStore()

	// 先创建词条 A: 神经网络
	entryA := &model.KnowledgeEntry{
		ID: "a1", Title: "神经网络", Content: "神经网络是一种计算模型。",
		Category: "AI", Status: model.EntryStatusPublished,
	}
	memStore.Entry.Create(context.Background(), entryA)
	memStore.TitleIdx.Add(index.TitleEntry{ID: "a1", Title: "神经网络"})

	// 再创建词条 B: 深度学习，内容引用神经网络
	entryB := &model.KnowledgeEntry{
		ID: "b1", Title: "深度学习", Content: "深度学习基于神经网络技术。",
		Category: "AI", Status: model.EntryStatusPublished,
	}
	memStore.Entry.Create(context.Background(), entryB)
	memStore.Search.IndexEntry(entryB)
	memStore.TitleIdx.Add(index.TitleEntry{ID: "b1", Title: "深度学习"})

	handler := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=深度学习", nil)
	w := httptest.NewRecorder()
	handler.SearchHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var apiResp APIResponse
	json.NewDecoder(resp.Body).Decode(&apiResp)

	pagedData, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data is not a map")
	}

	// 验证 graph 存在
	graph, ok := pagedData["graph"]
	if !ok || graph == nil {
		t.Fatal("graph should not be nil")
	}

	// 验证 content 中的链接
	items := pagedData["items"].([]interface{})
	entryMap := items[0].(map[string]interface{})
	content := entryMap["content"].(string)
	if !strings.Contains(content, "[神经网络](entry://a1)") {
		t.Errorf("content should contain link to 神经网络, got: %s", content)
	}
}

func TestSearchHandler_NoGraphWhenEmpty(t *testing.T) {
	memStore, _ := storage.NewMemoryStore()

	entry := &model.KnowledgeEntry{
		ID: "x1", Title: "量子计算", Content: "量子计算使用量子比特。",
		Category: "Physics", Status: model.EntryStatusPublished,
	}
	memStore.Entry.Create(context.Background(), entry)
	memStore.TitleIdx.Add(index.TitleEntry{ID: "x1", Title: "量子计算"})
	memStore.Search.IndexEntry(entry)

	handler := NewEntryHandler(memStore.Entry, memStore.Search, memStore.Backlink, memStore.User, memStore.TitleIdx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=量子计算", nil)
	w := httptest.NewRecorder()
	handler.SearchHandler(w, req)

	var apiResp APIResponse
	json.NewDecoder(w.Result().Body).Decode(&apiResp)
	pagedData := apiResp.Data.(map[string]interface{})
	graph := pagedData["graph"]
	// graph 存在但 nodes 应只有 1 个 result，edges 为空
	if graph == nil {
		t.Fatal("graph should exist even when no external references")
	}
}
```

- [ ] **Step 2: 运行集成测试**

```bash
go test -v -race -run TestSearchHandler_WithGraph ./internal/api/handler/...
go test -v -race -run TestSearchHandler_NoGraphWhenEmpty ./internal/api/handler/...
```

Expected: ALL PASS。

- [ ] **Step 3: 全量测试**

```bash
go test -v -race ./internal/...
```

Expected: ALL PASS。

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler/entry_handler_test.go
git commit -m "test(handler): add integration tests for search graph and keyword indexing"
```

- [ ] **Step 5: 最终验证 — 编译 + 测试**

```bash
make build
make test
```

Expected: build SUCCESS, all tests PASS。

---

### Summary

| Task | 内容 | 新建文件 | 修改文件 |
|------|------|----------|----------|
| 1 | TitleIndex AC 自动机 | `title_index.go`, `title_index_test.go` | — |
| 2 | ResultEnricher 链接+图谱 | `search_enricher.go`, `search_enricher_test.go` | — |
| 3 | API 类型 + Store + Handler 集成 | — | `types.go`, `store.go`, `entry_handler.go`, `router.go`, 各测试文件 |
| 4 | 集成测试 | — | `entry_handler_test.go` |
