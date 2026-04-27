package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newTestEnricher(t *testing.T) (*ResultEnricher, *storage.Store, *index.TitleIndex) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	ti := index.NewTitleIndex()
	enricher := NewResultEnricher(ti, store.Entry)
	return enricher, store, ti
}

// ========== TestEnricher_InsertLinks_Basic ==========

func TestEnricher_InsertLinks_Basic(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	// Setup TitleIndex: "神经网络" (e1)
	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})

	// Register e1 in store (for reference node title lookup)
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{
		ID:    "e1",
		Title: "神经网络",
	})

	// Search result entry
	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Title:   "深度学习入门",
		Content: "深度学习使用神经网络来处理数据。",
		Status:  model.EntryStatusPublished,
	}

	graph, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Content should contain the link
	expectedLink := "[神经网络](entry://e1)"
	if !strings.Contains(entry.Content, expectedLink) {
		t.Errorf("Expected content to contain %q, got: %q", expectedLink, entry.Content)
	}

	// Verify graph nodes
	if len(graph.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(graph.Nodes))
	}

	// e99 should be a "result" node
	foundResult := false
	foundRef := false
	for _, node := range graph.Nodes {
		if node.ID == "e99" && node.Type == "result" {
			foundResult = true
		}
		if node.ID == "e1" && node.Type == "reference" {
			foundRef = true
		}
	}
	if !foundResult {
		t.Error("Expected node e99 as type 'result'")
	}
	if !foundRef {
		t.Error("Expected node e1 as type 'reference'")
	}

	// Verify edges
	if len(graph.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(graph.Edges))
	}
	if len(graph.Edges) > 0 {
		e := graph.Edges[0]
		if e.From != "e99" || e.To != "e1" || e.Relation != "mentions" {
			t.Errorf("Edge mismatch: got {from:%s, to:%s, rel:%s}, expected {e99, e1, mentions}",
				e.From, e.To, e.Relation)
		}
	}
}

// ========== TestEnricher_InsertLinks_Multiple ==========

func TestEnricher_InsertLinks_Multiple(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	ti.Build([]index.TitleEntry{
		{ID: "e1", Title: "神经网络"},
		{ID: "e2", Title: "深度学习"},
	})

	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "神经网络"})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e2", Title: "深度学习"})

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Title:   "AI概述",
		Content: "深度学习推动神经网络发展。",
		Status:  model.EntryStatusPublished,
	}

	_, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if !strings.Contains(entry.Content, "[深度学习](entry://e2)") {
		t.Errorf("Expected link for 深度学习, got: %s", entry.Content)
	}
	if !strings.Contains(entry.Content, "[神经网络](entry://e1)") {
		t.Errorf("Expected link for 神经网络, got: %s", entry.Content)
	}
}

// ========== TestEnricher_InsertLinks_SkipCodeBlock ==========

func TestEnricher_InsertLinks_SkipCodeBlock(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "神经网络"})

	// Content with "神经网络" inside a code block AND outside
	entry := &model.KnowledgeEntry{
		ID:    "e99",
		Title: "测试",
		Content: "```\n神经网络在被保护的代码块中\n```\n\n但这里的神经网络应该被链接。",
		Status: model.EntryStatusPublished,
	}

	_, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Count occurrences of "[神经网络](entry://e1)"
	count := strings.Count(entry.Content, "[神经网络](entry://e1)")
	if count != 1 {
		t.Errorf("Expected exactly 1 link for 神经网络, got %d. Content: %s", count, entry.Content)
	}

	// The code block content should still have the raw "神经网络"
	// Verify the first "神经网络" (inside code block) is NOT linked
	if !strings.Contains(entry.Content, "```\n神经网络在被保护") {
		t.Error("Code block content should have raw 神经网络, but it seems modified")
	}
}

// ========== TestEnricher_InsertLinks_SkipInlineCode ==========

func TestEnricher_InsertLinks_SkipInlineCode(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "神经网络"})

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Title:   "测试",
		Content: "`神经网络` 是一种模型。",
		Status:  model.EntryStatusPublished,
	}

	_, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// The inline-coded "神经网络" should NOT be linked
	if strings.Contains(entry.Content, "[神经网络](entry://e1)") {
		t.Errorf("Inline code 神经网络 should not be linked. Content: %s", entry.Content)
	}

	// Should still contain the raw backtick-wrapped text
	if !strings.Contains(entry.Content, "`神经网络`") {
		t.Errorf("Content should still contain raw `神经网络`. Content: %s", entry.Content)
	}
}

// ========== TestEnricher_InsertLinks_SkipExistingLink ==========

func TestEnricher_InsertLinks_SkipExistingLink(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "神经网络"})

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Title:   "测试",
		Content: "参考 [神经网络](https://example.com) 了解更多。",
		Status:  model.EntryStatusPublished,
	}

	_, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// The "神经网络" inside existing link text should NOT be re-wrapped
	// Result should still contain only one "[神经网络]" occurrence (the existing link)
	count := strings.Count(entry.Content, "[神经网络]")
	if count != 1 {
		t.Errorf("Expected exactly 1 occurrence of '[神经网络]', got %d. Content: %s", count, entry.Content)
	}

	// Should NOT contain a nested link like `[[神经网络](https://example.com)](entry://e1)`
	if strings.Contains(entry.Content, "[[神经网络]") {
		t.Errorf("Content should not have double-wrapped link. Content: %s", entry.Content)
	}
}

// ========== TestEnricher_InsertLinks_SkipURL ==========

func TestEnricher_InsertLinks_SkipURL(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	ti.Build([]index.TitleEntry{{ID: "e1", Title: "model"}})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "model"})

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Title:   "测试",
		Content: "See https://model.ai for details.",
		Status:  model.EntryStatusPublished,
	}

	_, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// "model" inside URL should NOT be linked
	if strings.Contains(entry.Content, "[model](entry://e1)") {
		t.Errorf("model inside URL should not be linked. Content: %s", entry.Content)
	}

	// URL should remain intact
	if !strings.Contains(entry.Content, "https://model.ai") {
		t.Errorf("URL should remain intact. Content: %s", entry.Content)
	}
}

// ========== TestEnricher_InsertLinks_SelfRef ==========

func TestEnricher_InsertLinks_SelfRef(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "神经网络"})

	// Entry with ID "e1" should NOT link to itself
	entry := &model.KnowledgeEntry{
		ID:      "e1",
		Title:   "神经网络",
		Content: "神经网络是一种模型。",
		Status:  model.EntryStatusPublished,
	}

	_, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Should NOT link to itself
	if strings.Contains(entry.Content, "[神经网络](entry://e1)") {
		t.Errorf("Entry should not link to itself. Content: %s", entry.Content)
	}
}

// ========== TestEnricher_BuildGraph_MultipleResults ==========

func TestEnricher_BuildGraph_MultipleResults(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	ti.Build([]index.TitleEntry{
		{ID: "e1", Title: "神经网络"},
		{ID: "e2", Title: "反向传播"},
	})

	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "神经网络"})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e2", Title: "反向传播"})

	// Two result entries, both mentioning e1 and e2
	entries := []*model.KnowledgeEntry{
		{
			ID:      "a1",
			Title:   "结果1",
			Content: "神经网络使用反向传播算法。",
			Status:  model.EntryStatusPublished,
		},
		{
			ID:      "a2",
			Title:   "结果2",
			Content: "反向传播是神经网络的基础算法。",
			Status:  model.EntryStatusPublished,
		},
	}

	graph, err := enricher.Enrich(entries)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Graph should have 4 nodes: a1, a2, e1, e2
	if len(graph.Nodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(graph.Nodes))
	}

	// Check node types
	nodeTypes := make(map[string]string)
	for _, node := range graph.Nodes {
		nodeTypes[node.ID] = node.Type
	}
	if nodeTypes["a1"] != "result" {
		t.Errorf("Expected a1 type 'result', got '%s'", nodeTypes["a1"])
	}
	if nodeTypes["a2"] != "result" {
		t.Errorf("Expected a2 type 'result', got '%s'", nodeTypes["a2"])
	}
	if nodeTypes["e1"] != "reference" {
		t.Errorf("Expected e1 type 'reference', got '%s'", nodeTypes["e1"])
	}
	if nodeTypes["e2"] != "reference" {
		t.Errorf("Expected e2 type 'reference', got '%s'", nodeTypes["e2"])
	}

	// Graph should have 4 edges
	if len(graph.Edges) != 4 {
		t.Errorf("Expected 4 edges, got %d", len(graph.Edges))
	}

	// Verify edge relations
	for _, edge := range graph.Edges {
		if edge.Relation != "mentions" {
			t.Errorf("All edges should have relation 'mentions', got '%s'", edge.Relation)
		}
		// From should be a result, To should be a reference
		if nodeTypes[edge.From] != "result" {
			t.Errorf("Edge from '%s' should be a result node", edge.From)
		}
		if nodeTypes[edge.To] != "reference" {
			t.Errorf("Edge to '%s' should be a reference node", edge.To)
		}
	}
}

// ========== TestEnricher_BuildGraph_NoReferences ==========

func TestEnricher_BuildGraph_NoReferences(t *testing.T) {
	enricher, _, ti := newTestEnricher(t)

	ti.Build([]index.TitleEntry{{ID: "e1", Title: "量子计算"}})

	entry := &model.KnowledgeEntry{
		ID:      "a1",
		Title:   "不相关的条目",
		Content: "这是一段没有匹配的文本。",
		Status:  model.EntryStatusPublished,
	}

	graph, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Graph should have 1 node (the result itself) and 0 edges
	if len(graph.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 0 {
		t.Errorf("Expected 0 edges, got %d", len(graph.Edges))
	}

	if len(graph.Nodes) > 0 {
		node := graph.Nodes[0]
		if node.ID != "a1" || node.Type != "result" {
			t.Errorf("Expected node {a1, result}, got {%s, %s}", node.ID, node.Type)
		}
	}
}

// ========== Additional Tests for Edge Cases ==========

// TestEnricher_EmptyContentFetchesFromStore verifies that empty content
// triggers a fetch from the entry store.
func TestEnricher_EmptyContentFetchesFromStore(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	ti.Build([]index.TitleEntry{{ID: "e1", Title: "神经网络"}})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "神经网络"})

	// Store the search result entry with full content
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{
		ID:      "e99",
		Title:   "深度学习入门",
		Content: "深度学习使用神经网络来处理数据。",
		Status:  model.EntryStatusPublished,
	})

	// Pass entry with empty content — should fetch from store
	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Title:   "深度学习入门",
		Content: "", // empty, will be fetched
		Status:  model.EntryStatusPublished,
	}

	_, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Content should have been fetched and enriched
	if !strings.Contains(entry.Content, "[神经网络](entry://e1)") {
		t.Errorf("Expected link for 神经网络 in fetched content, got: %s", entry.Content)
	}
}

// TestEnricher_InsertLinks_OverlappingMatches verifies longest-match-wins semantics.
func TestEnricher_InsertLinks_OverlappingMatches(t *testing.T) {
	enricher, store, ti := newTestEnricher(t)

	// "深度" and "深度学习" — overlapping, "深度学习" is longer and should win
	ti.Build([]index.TitleEntry{
		{ID: "e1", Title: "深度"},
		{ID: "e2", Title: "深度学习"},
	})

	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "深度"})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e2", Title: "深度学习"})

	entry := &model.KnowledgeEntry{
		ID:      "e99",
		Title:   "测试",
		Content: "深度学习是AI的基础。",
		Status:  model.EntryStatusPublished,
	}

	_, err := enricher.Enrich([]*model.KnowledgeEntry{entry})
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// "深度学习" should be linked as a whole, not "深度"
	if strings.Contains(entry.Content, "[深度](entry://e1)") {
		t.Errorf("Shorter match '深度' should not win over '深度学习'. Content: %s", entry.Content)
	}
	if !strings.Contains(entry.Content, "[深度学习](entry://e2)") {
		t.Errorf("Expected link for 深度学习. Content: %s", entry.Content)
	}
}

// TestEnricher_ScanProtectionZones_CodeBlock verifies code block zone scanning.
func TestEnricher_ScanProtectionZones_CodeBlock(t *testing.T) {
	content := "before\n```\ncode inside\n```\nafter"
	zones := scanProtectionZones(content)

	if len(zones) != 1 {
		t.Fatalf("Expected 1 zone for code block, got %d", len(zones))
	}

	// Zone should cover the entire ```...``` including delimiters
	start := zones[0].start
	end := zones[0].end
	zoneText := content[start:end]

	if !strings.HasPrefix(zoneText, "```") {
		t.Errorf("Zone should start with ```, got: %q", zoneText[:3])
	}
	if !strings.HasSuffix(zoneText, "```") {
		t.Errorf("Zone should end with ```, got: ...%q", zoneText[len(zoneText)-3:])
	}
	if !strings.Contains(zoneText, "code inside") {
		t.Errorf("Zone should contain the code: %q", zoneText)
	}
}

// TestEnricher_ScanProtectionZones_MarkdownLink verifies markdown link zone scanning.
func TestEnricher_ScanProtectionZones_MarkdownLink(t *testing.T) {
	content := "text [神经网络](https://example.com) more text"
	zones := scanProtectionZones(content)

	if len(zones) < 1 {
		t.Fatalf("Expected at least 1 zone, got %d", len(zones))
	}

	// Find the zone that covers the markdown link
	found := false
	for _, z := range zones {
		zoneText := content[z.start:z.end]
		if zoneText == "[神经网络](https://example.com)" {
			found = true
			break
		}
	}
	if !found {
		var zoneTexts []string
		for _, z := range zones {
			zoneTexts = append(zoneTexts, content[z.start:z.end])
		}
		t.Errorf("Expected protection zone covering '[神经网络](https://example.com)', zones: %v", zoneTexts)
	}
}

// TestEnricher_ScanProtectionZones_Image verifies image syntax zone scanning.
func TestEnricher_ScanProtectionZones_Image(t *testing.T) {
	content := "look at this ![image](https://example.com/img.png) nice"
	zones := scanProtectionZones(content)

	if len(zones) < 1 {
		t.Fatalf("Expected at least 1 zone, got %d", len(zones))
	}

	found := false
	for _, z := range zones {
		zoneText := content[z.start:z.end]
		if zoneText == "![image](https://example.com/img.png)" {
			found = true
			break
		}
	}
	if !found {
		var zoneTexts []string
		for _, z := range zones {
			zoneTexts = append(zoneTexts, content[z.start:z.end])
		}
		t.Errorf("Expected protection zone covering '![image](https://example.com/img.png)', zones: %v", zoneTexts)
	}
}

// TestEnricher_ScanProtectionZones_URL verifies URL zone scanning.
func TestEnricher_ScanProtectionZones_URL(t *testing.T) {
	content := "See https://example.com/page for more"
	zones := scanProtectionZones(content)

	if len(zones) < 1 {
		t.Fatalf("Expected at least 1 zone, got %d", len(zones))
	}

	found := false
	for _, z := range zones {
		zoneText := content[z.start:z.end]
		if zoneText == "https://example.com/page" {
			found = true
			break
		}
	}
	if !found {
		var zoneTexts []string
		for _, z := range zones {
			zoneTexts = append(zoneTexts, content[z.start:z.end])
		}
		t.Errorf("Expected protection zone covering 'https://example.com/page', zones: %v", zoneTexts)
	}
}

// TestEnricher_ScanProtectionZones_InlineCode verifies inline code zone scanning.
func TestEnricher_ScanProtectionZones_InlineCode(t *testing.T) {
	content := "use `fmt.Println` to output"
	zones := scanProtectionZones(content)

	if len(zones) < 1 {
		t.Fatalf("Expected at least 1 zone, got %d", len(zones))
	}

	found := false
	for _, z := range zones {
		zoneText := content[z.start:z.end]
		if zoneText == "`fmt.Println`" {
			found = true
			break
		}
	}
	if !found {
		var zoneTexts []string
		for _, z := range zones {
			zoneTexts = append(zoneTexts, content[z.start:z.end])
		}
		t.Errorf("Expected protection zone covering '`fmt.Println`', zones: %v", zoneTexts)
	}
}
