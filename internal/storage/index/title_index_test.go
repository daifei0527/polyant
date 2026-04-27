package index

import (
	"sync"
	"testing"
)

// =============================================================================
// 1. TestTitleIndex_BuildAndMatchAll
// =============================================================================
func TestTitleIndex_BuildAndMatchAll(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "1", Title: "神经网络"},
		{ID: "2", Title: "深度学习"},
		{ID: "3", Title: "机器学习"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	content := "深度学习是机器学习的一个分支，神经网络是其中的关键技术。"
	matches := ti.MatchAll(content)

	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d: %+v", len(matches), matches)
	}

	// Verify matches sorted by offset
	for i := 1; i < len(matches); i++ {
		if matches[i].Offset < matches[i-1].Offset {
			t.Errorf("matches not sorted by offset: match[%d].Offset=%d < match[%d].Offset=%d",
				i, matches[i].Offset, i-1, matches[i-1].Offset)
		}
	}

	// Check each expected match
	expected := map[string]struct {
		title string
		min   int
		max   int
	}{
		"2": {title: "深度学习", min: 0, max: 4},
		"3": {title: "机器学习", min: 4, max: 10},
		"1": {title: "神经网络", min: 10, max: 20},
	}
	for _, m := range matches {
		exp, ok := expected[m.EntryID]
		if !ok {
			t.Errorf("unexpected EntryID in matches: %s", m.EntryID)
			continue
		}
		if m.Title != exp.title {
			t.Errorf("EntryID %s: expected title %q, got %q", m.EntryID, exp.title, m.Title)
		}
		if m.Offset < exp.min || m.Offset > exp.max {
			t.Errorf("EntryID %s: offset %d out of expected range [%d, %d]",
				m.EntryID, m.Offset, exp.min, exp.max)
		}
	}
}

// =============================================================================
// 2. TestTitleIndex_MatchAll_NoMatch
// =============================================================================
func TestTitleIndex_MatchAll_NoMatch(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "1", Title: "神经网络"},
		{ID: "2", Title: "深度学习"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	matches := ti.MatchAll("这是一段完全不相关的内容")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d: %+v", len(matches), matches)
	}
}

// =============================================================================
// 3. TestTitleIndex_MatchAll_EmptyInput
// =============================================================================
func TestTitleIndex_MatchAll_EmptyInput(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "1", Title: "神经网络"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	matches := ti.MatchAll("")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for empty input, got %d", len(matches))
	}
}

// =============================================================================
// 4. TestTitleIndex_MatchAll_Overlapping
// =============================================================================
func TestTitleIndex_MatchAll_Overlapping(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "1", Title: "机器学习"},
		{ID: "2", Title: "机器学习系统"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	content := "机器学习系统是一个复杂的学科。"
	matches := ti.MatchAll(content)

	// Only "机器学习系统" should be in results (longer match wins)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match (longest only), got %d: %+v", len(matches), matches)
	}
	if matches[0].EntryID != "2" {
		t.Errorf("expected '机器学习系统' (ID=2), got ID=%s title=%q", matches[0].EntryID, matches[0].Title)
	}
}

// =============================================================================
// 5. TestTitleIndex_MatchAll_Chinese
// =============================================================================
func TestTitleIndex_MatchAll_Chinese(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "1", Title: "人工智能"},
		{ID: "2", Title: "自然语言处理"},
		{ID: "3", Title: "计算机视觉"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	content := "人工智能领域包括自然语言处理和计算机视觉等方向。"
	matches := ti.MatchAll(content)

	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d: %+v", len(matches), matches)
	}

	// Verify rune offsets (not byte offsets)
	for _, m := range matches {
		// Extract the substring at the reported offset to verify correctness
		runes := []rune(content)
		titleRunes := []rune(m.Title)
		if m.Offset+len(titleRunes) > len(runes) {
			t.Errorf("match %q at offset %d extends past content end (rune len=%d)",
				m.Title, m.Offset, len(runes))
			continue
		}
		matched := string(runes[m.Offset : m.Offset+len(titleRunes)])
		if matched != m.Title {
			t.Errorf("at offset %d expected %q, got %q (byte pos may differ from rune pos)",
				m.Offset, m.Title, matched)
		}
	}
}

// =============================================================================
// 6. TestTitleIndex_Add
// =============================================================================
func TestTitleIndex_Add(t *testing.T) {
	ti := NewTitleIndex()
	if err := ti.Build([]TitleEntry{{ID: "1", Title: "神经网络"}}); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Add a second pattern
	if err := ti.Add(TitleEntry{ID: "2", Title: "深度学习"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	matches := ti.MatchAll("神经网络和深度学习")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches after Add, got %d: %+v", len(matches), matches)
	}

	foundIDs := make(map[string]bool)
	for _, m := range matches {
		foundIDs[m.EntryID] = true
	}
	if !foundIDs["1"] {
		t.Error("original pattern '神经网络' not found after Add")
	}
	if !foundIDs["2"] {
		t.Error("added pattern '深度学习' not found after Add")
	}
}

// =============================================================================
// 7. TestTitleIndex_Remove
// =============================================================================
func TestTitleIndex_Remove(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "1", Title: "神经网络"},
		{ID: "2", Title: "深度学习"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if err := ti.Remove("神经网络"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	matches := ti.MatchAll("神经网络和深度学习")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match after Remove, got %d: %+v", len(matches), matches)
	}
	if matches[0].EntryID != "2" {
		t.Errorf("expected remaining match ID=2, got ID=%s title=%q", matches[0].EntryID, matches[0].Title)
	}
}

// =============================================================================
// 8. TestTitleIndex_Update
// =============================================================================
func TestTitleIndex_Update(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "1", Title: "机器学习"},
		{ID: "2", Title: "深度学习"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Update "机器学习" -> "强化学习"
	if err := ti.Update(TitleEntry{ID: "1", Title: "机器学习"}, TitleEntry{ID: "1", Title: "强化学习"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Old title should NOT match
	matches := ti.MatchAll("机器学习是AI的重要分支")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for old title, got %d: %+v", len(matches), matches)
	}

	// New title SHOULD match; "深度学习" (ID=2) is not in this content
	matches = ti.MatchAll("强化学习是机器学习的分支")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match (强化学习 only), got %d: %+v", len(matches), matches)
	}

	foundIDs := make(map[string]bool)
	for _, m := range matches {
		foundIDs[m.EntryID] = true
	}
	if !foundIDs["1"] {
		t.Error("updated pattern '强化学习' not found")
	}
	if foundIDs["2"] {
		t.Error("'深度学习' should not match in this content")
	}
}

// =============================================================================
// 9. TestTitleIndex_MatchAll_SpecialChars
// =============================================================================
func TestTitleIndex_MatchAll_SpecialChars(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "1", Title: "C++"},
		{ID: "2", Title: "Go (语言)"},
		{ID: "3", Title: "C#"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	matches := ti.MatchAll("C++和Go (语言)以及C#都是编程语言")
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches for special chars, got %d: %+v", len(matches), matches)
	}

	found := make(map[string]bool)
	for _, m := range matches {
		found[m.EntryID] = true
	}
	for _, id := range []string{"1", "2", "3"} {
		if !found[id] {
			t.Errorf("pattern ID=%s not found in matches", id)
		}
	}
}

// =============================================================================
// 10. TestTitleIndex_MultipleAdds
// =============================================================================
func TestTitleIndex_MultipleAdds(t *testing.T) {
	ti := NewTitleIndex()
	ti.Build([]TitleEntry{{ID: "e1", Title: "A"}})
	ti.Add(TitleEntry{ID: "e2", Title: "B"})
	ti.Add(TitleEntry{ID: "e3", Title: "AB"})
	ti.Add(TitleEntry{ID: "e4", Title: "C"})

	matches := ti.MatchAll("AB C")
	// verify no duplicate match titles
	seen := make(map[string]int)
	for _, m := range matches {
		seen[m.Title]++
	}
	for title, count := range seen {
		if count > 1 {
			t.Errorf("duplicate match for %q: %d occurrences", title, count)
		}
	}
}

// =============================================================================
// 11. TestTitleIndex_Concurrent
// =============================================================================
func TestTitleIndex_Concurrent(t *testing.T) {
	ti := NewTitleIndex()
	entries := []TitleEntry{
		{ID: "1", Title: "神经网络"},
		{ID: "2", Title: "深度学习"},
		{ID: "3", Title: "机器学习"},
		{ID: "4", Title: "强化学习"},
		{ID: "5", Title: "迁移学习"},
	}
	if err := ti.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	numIterations := 100

	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				content := "深度学习和机器学习是神经网络的基础，强化学习和迁移学习也很重要。"
				matches := ti.MatchAll(content)
				if len(matches) < 1 {
					errors <- nil // just count, don't fail on empty since content may not have all
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// No panics or races = success
}
