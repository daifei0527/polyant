package handler

import (
	"context"
	"sort"
	"unicode/utf8"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// GraphNode represents a node in the search result graph.
type GraphNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"` // "result" | "reference"
}

// GraphEdge represents an edge in the search result graph.
type GraphEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"` // "mentions"
}

// SearchGraph represents the graph of search results and their references.
type SearchGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// protZone is an unexported type representing a protected byte range in content.
type protZone struct {
	start int // byte offset
	end   int // byte offset (exclusive)
}

// ResultEnricher enriches search results by inserting links and building
// a relationship graph between search results and referenced entries.
type ResultEnricher struct {
	titleIndex *index.TitleIndex
	entryStore storage.EntryStore
}

// NewResultEnricher creates a new ResultEnricher.
func NewResultEnricher(ti *index.TitleIndex, es storage.EntryStore) *ResultEnricher {
	return &ResultEnricher{
		titleIndex: ti,
		entryStore: es,
	}
}

// Enrich processes search result entries: inserts markdown links for matched
// entry titles and builds a graph of result-to-reference relationships.
func (e *ResultEnricher) Enrich(entries []*model.KnowledgeEntry) (*SearchGraph, error) {
	graph := &SearchGraph{}

	// Track reference nodes to avoid duplicates
	seenRefs := make(map[string]bool)
	seenResultNodes := make(map[string]bool)

	for _, entry := range entries {
		// Register as result node
		if !seenResultNodes[entry.ID] {
			graph.Nodes = append(graph.Nodes, GraphNode{
				ID:    entry.ID,
				Title: entry.Title,
				Type:  "result",
			})
			seenResultNodes[entry.ID] = true
		}

		content := entry.Content

		// If content is empty, try to fetch from entryStore
		if content == "" {
			stored, err := e.entryStore.Get(context.Background(), entry.ID)
			if err != nil || stored == nil {
				continue
			}
			content = stored.Content
			if content == "" {
				continue
			}
			// Update the entry's content for the caller
			entry.Content = content
		}

		// Scan for protection zones
		zones := scanProtectionZones(content)

		// Find all title matches
		matches := e.titleIndex.MatchAll(content)

		// Filter matches: remove self-refs, protection zone matches, and overlaps
		matches = filterMatches(matches, zones, entry.ID, content)

		if len(matches) == 0 {
			continue
		}

		// Insert markdown links (end-to-start to preserve offsets)
		entry.Content = insertLinks(content, matches)

		// Add reference nodes and edges
		for _, m := range matches {
			// Add reference node (deduplicate)
			if !seenRefs[m.EntryID] {
				refNode := GraphNode{
					ID:   m.EntryID,
					Type: "reference",
				}
				// Lookup title from the stored entry
				refEntry, err := e.entryStore.Get(context.Background(), m.EntryID)
				if err == nil && refEntry != nil {
					refNode.Title = refEntry.Title
				} else {
					refNode.Title = m.Title
				}
				graph.Nodes = append(graph.Nodes, refNode)
				seenRefs[m.EntryID] = true
			}

			// Add edge: this result mentions the reference
			graph.Edges = append(graph.Edges, GraphEdge{
				From:     entry.ID,
				To:       m.EntryID,
				Relation: "mentions",
			})
		}
	}

	return graph, nil
}

// scanProtectionZones scans content for protected zones that should not be
// modified by link insertion. Protected zones include:
//  1. Code blocks (``` ... ```)
//  2. Inline code (` ... `)
//  3. Existing Markdown links ([text](url))
//  4. Image syntax (![alt](url))
//  5. URLs (http://... or https://...)
func scanProtectionZones(content string) []protZone {
	b := []byte(content)
	n := len(b)
	var zones []protZone

	i := 0
	for i < n {
		if end, ok := findCodeBlock(b, i); ok {
			zones = append(zones, protZone{start: i, end: end})
			i = end
		} else if end, ok := findInlineCode(b, i); ok {
			zones = append(zones, protZone{start: i, end: end})
			i = end
		} else if end, ok := findLinkLike(b, i); ok {
			zones = append(zones, protZone{start: i, end: end})
			i = end
		} else if end, ok := findURL(b, i); ok {
			zones = append(zones, protZone{start: i, end: end})
			i = end
		} else {
			i++
		}
	}

	return mergeZones(zones)
}

// findCodeBlock detects ``` ``` code blocks. Returns the byte index after the
// closing ``` and true if found.
func findCodeBlock(b []byte, start int) (int, bool) {
	if start+2 >= len(b) {
		return 0, false
	}
	if b[start] != '`' || b[start+1] != '`' || b[start+2] != '`' {
		return 0, false
	}
	// Find closing ```
	for j := start + 3; j+2 < len(b); j++ {
		if b[j] == '`' && b[j+1] == '`' && b[j+2] == '`' {
			return j + 3, true
		}
	}
	// No closing delimiter found — treat the opening as consumed
	return len(b), true
}

// findInlineCode detects ` ` inline code spans. Returns the byte index after
// the closing backtick and true if found. The backtick must not be adjacent to
// another backtick (to distinguish from ``` code blocks).
func findInlineCode(b []byte, start int) (int, bool) {
	if !isSingleBacktick(b, start) {
		return 0, false
	}
	for j := start + 1; j < len(b); j++ {
		if isSingleBacktick(b, j) {
			return j + 1, true
		}
	}
	return 0, false
}

// isSingleBacktick returns true if the byte at position i is a backtick that is
// not adjacent to another backtick (i.e., it's not part of `` or ```).
func isSingleBacktick(b []byte, i int) bool {
	if i >= len(b) || b[i] != '`' {
		return false
	}
	if i > 0 && b[i-1] == '`' {
		return false
	}
	if i+1 < len(b) && b[i+1] == '`' {
		return false
	}
	return true
}

// findLinkLike detects [text](url) links and ![alt](url) images. Returns the
// byte index after the closing ')' and true if found.
func findLinkLike(b []byte, start int) (int, bool) {
	i := start

	// Optional '!' for image syntax
	if b[i] == '!' {
		i++
		if i >= len(b) || b[i] != '[' {
			return 0, false
		}
	}

	if b[i] != '[' {
		return 0, false
	}

	// Find closing ']' — must not cross newlines
	j := i + 1
	for j < len(b) && b[j] != ']' {
		if b[j] == '\n' {
			return 0, false
		}
		j++
	}
	if j >= len(b) {
		return 0, false
	}
	// j is at ']'

	// After ']' must be '(' for link/image syntax
	if j+1 >= len(b) || b[j+1] != '(' {
		return 0, false
	}

	// Find closing ')' — must not cross newlines
	k := j + 2
	for k < len(b) && b[k] != ')' {
		if b[k] == '\n' {
			return 0, false
		}
		k++
	}
	if k >= len(b) {
		return 0, false
	}
	// k is at ')'

	return k + 1, true
}

// findURL detects raw URLs starting with http:// or https://. Returns the
// byte index after the last URL character and true if found.
func findURL(b []byte, start int) (int, bool) {
	if start+7 >= len(b) {
		return 0, false
	}
	s := string(b[start : start+8])
	isHTTP := false
	if len(s) >= 7 && s[:7] == "http://" {
		isHTTP = true
	} else if len(s) >= 8 && s[:8] == "https://" {
		isHTTP = true
	}
	if !isHTTP {
		return 0, false
	}

	// Find end of URL: whitespace, ')', or end of content
	j := start
	for j < len(b) && b[j] != ' ' && b[j] != '\t' && b[j] != '\n' && b[j] != ')' {
		j++
	}
	return j, true
}

// mergeZones sorts zones by start offset and merges overlapping or adjacent zones.
func mergeZones(zones []protZone) []protZone {
	if len(zones) <= 1 {
		return zones
	}

	sort.Slice(zones, func(i, j int) bool { return zones[i].start < zones[j].start })

	result := []protZone{zones[0]}
	for i := 1; i < len(zones); i++ {
		last := &result[len(result)-1]
		if zones[i].start <= last.end {
			// Overlapping — merge by extending if current zone is wider
			if zones[i].end > last.end {
				last.end = zones[i].end
			}
		} else {
			result = append(result, zones[i])
		}
	}
	return result
}

// filterMatches filters out self-references, matches inside protection zones,
// and removes shorter overlapping matches (longest match wins).
func filterMatches(matches []index.Match, zones []protZone, selfID string, content string) []index.Match {
	if len(matches) == 0 {
		return nil
	}

	contentRunes := []rune(content)

	// Step 1: Filter out self-references and protection-zone matches
	var filtered []index.Match
	for _, m := range matches {
		// Remove self-references
		if m.EntryID == selfID {
			continue
		}

		// Convert rune offset to byte offset
		matchByteOffset := len([]byte(string(contentRunes[:m.Offset])))

		// Check if match falls inside any protection zone
		if isInZone(matchByteOffset, zones) {
			continue
		}

		filtered = append(filtered, m)
	}

	if len(filtered) == 0 {
		return nil
	}

	// Step 2: Sort by offset ascending, then by length descending for ties
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Offset != filtered[j].Offset {
			return filtered[i].Offset < filtered[j].Offset
		}
		return utf8.RuneCountInString(filtered[i].Title) > utf8.RuneCountInString(filtered[j].Title)
	})

	// Step 3: Remove shorter overlapping matches (longest match wins)
	result := make([]index.Match, 0, len(filtered))
	for _, m := range filtered {
		if len(result) > 0 {
			last := &result[len(result)-1]
			lastEnd := last.Offset + utf8.RuneCountInString(last.Title)

			if m.Offset < lastEnd {
				// Overlaps with last kept match
				if utf8.RuneCountInString(m.Title) > utf8.RuneCountInString(last.Title) {
					// Current is longer, replace last
					*last = m
				}
				// Otherwise discard current
				continue
			}
		}
		result = append(result, m)
	}

	return result
}

// isInZone checks if a byte offset falls within any protection zone.
func isInZone(offset int, zones []protZone) bool {
	for _, z := range zones {
		if offset >= z.start && offset < z.end {
			return true
		}
	}
	return false
}

// insertLinks inserts markdown links into content for the given matches.
// Matches are processed end-to-start to preserve byte offsets.
func insertLinks(content string, matches []index.Match) string {
	if len(matches) == 0 {
		return content
	}

	// Sort by offset ascending so we can iterate in reverse (end-to-start)
	sort.Slice(matches, func(i, j int) bool { return matches[i].Offset < matches[j].Offset })

	b := []byte(content)
	contentRunes := []rune(content)

	// Process from end to start to preserve byte offsets
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]

		// Convert rune offset to byte offset
		matchByteOffset := len([]byte(string(contentRunes[:m.Offset])))
		matchByteLen := len([]byte(m.Title))

		// Build the link: [title](entry://id)
		link := "[" + m.Title + "](entry://" + m.EntryID + ")"

		// Replace the matched text with the link
		b = append(b[:matchByteOffset], append([]byte(link), b[matchByteOffset+matchByteLen:]...)...)
	}

	return string(b)
}
