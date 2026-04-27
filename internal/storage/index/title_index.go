package index

import (
	"fmt"
	"sort"
	"sync"
	"unicode/utf8"
)

// TitleEntry represents a title entry to be indexed for matching.
type TitleEntry struct {
	ID    string
	Title string
}

// Match represents a matched title in content.
type Match struct {
	Title   string `json:"title"`
	EntryID string `json:"entryId"`
	Offset  int    `json:"offset"` // rune offset in content (not byte offset)
}

// matchedPattern is stored in the output list of AC automaton nodes.
type matchedPattern struct {
	title string
	id    string
}

// acNode is a node in the Aho-Corasick trie.
type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	output   []matchedPattern
}

// TitleIndex implements an Aho-Corasick automaton for matching entry titles against content.
type TitleIndex struct {
	root    *acNode
	entries map[string]TitleEntry
	mu      sync.RWMutex
}

// NewTitleIndex creates a new empty TitleIndex.
func NewTitleIndex() *TitleIndex {
	return &TitleIndex{
		root:    &acNode{children: make(map[rune]*acNode)},
		entries: make(map[string]TitleEntry),
	}
}

// Build performs a full rebuild of the automaton from the given entries.
func (ti *TitleIndex) Build(entries []TitleEntry) error {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	ti.root = &acNode{children: make(map[rune]*acNode)}
	ti.entries = make(map[string]TitleEntry, len(entries))

	for _, e := range entries {
		ti.entries[e.Title] = e
		ti.insert(e.Title, e.ID)
	}

	ti.buildFailLinks()
	return nil
}

// Add adds a single entry and triggers a full rebuild.
func (ti *TitleIndex) Add(entry TitleEntry) error {
	if entry.Title == "" {
		return nil
	}
	ti.mu.Lock()
	defer ti.mu.Unlock()

	ti.entries[entry.Title] = entry
	ti.rebuildLocked()
	return nil
}

// Remove removes an entry by title and triggers a full rebuild.
func (ti *TitleIndex) Remove(title string) error {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if _, ok := ti.entries[title]; !ok {
		return fmt.Errorf("title %q not found in index", title)
	}

	delete(ti.entries, title)
	ti.rebuildLocked()
	return nil
}

// Update replaces an old entry with a new one and triggers a full rebuild.
func (ti *TitleIndex) Update(old, new TitleEntry) error {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if _, ok := ti.entries[old.Title]; !ok {
		return fmt.Errorf("title %q not found in index", old.Title)
	}

	delete(ti.entries, old.Title)
	ti.entries[new.Title] = new
	ti.rebuildLocked()
	return nil
}

// MatchAll finds all title matches in content.
// Matches are sorted by offset. Overlapping matches are resolved by keeping the longest match.
func (ti *TitleIndex) MatchAll(content string) []Match {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	if len(content) == 0 {
		return nil
	}

	runes := []rune(content)
	var matches []Match

	node := ti.root
	for i, r := range runes {
		// Follow fail links until a match is found or root is reached
		for node != ti.root && node.children[r] == nil {
			node = node.fail
		}

		if child, ok := node.children[r]; ok {
			node = child
		} else {
			node = ti.root
		}

		// Collect all matches ending at this position
		for _, pat := range node.output {
			patRuneCount := utf8.RuneCountInString(pat.title)
			startOffset := i - patRuneCount + 1
			matches = append(matches, Match{
				Title:   pat.title,
				EntryID: pat.id,
				Offset:  startOffset,
			})
		}
	}

	return filterOverlapping(matches)
}

// filterOverlapping sorts matches by offset (ascending), then by length (descending for same offset),
// and removes shorter matches that overlap with a longer one.
func filterOverlapping(matches []Match) []Match {
	if len(matches) <= 1 {
		return matches
	}

	// Sort: offset ascending, then longer match first for same offset
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Offset != matches[j].Offset {
			return matches[i].Offset < matches[j].Offset
		}
		return utf8.RuneCountInString(matches[i].Title) > utf8.RuneCountInString(matches[j].Title)
	})

	result := make([]Match, 0, len(matches))
	for _, m := range matches {
		if len(result) > 0 {
			last := &result[len(result)-1]
			lastEnd := last.Offset + utf8.RuneCountInString(last.Title)

			if m.Offset < lastEnd {
				// Overlaps with the last kept match
				if utf8.RuneCountInString(m.Title) > utf8.RuneCountInString(last.Title) {
					// Current is longer, replace the last
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

// insert inserts a pattern into the trie.
func (ti *TitleIndex) insert(pattern, id string) {
	node := ti.root
	for _, r := range pattern {
		if node.children[r] == nil {
			node.children[r] = &acNode{children: make(map[rune]*acNode)}
		}
		node = node.children[r]
	}
	node.output = append(node.output, matchedPattern{title: pattern, id: id})
}

// buildFailLinks constructs fail links using BFS and merges output from fail nodes.
func (ti *TitleIndex) buildFailLinks() {
	// BFS queue
	var queue []*acNode

	// Initialize: root's children fail to root
	for _, child := range ti.root.children {
		child.fail = ti.root
		queue = append(queue, child)
	}

	// BFS
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for r, child := range current.children {
			// Follow fail links to find the longest proper suffix
			failNode := current.fail
			for failNode != nil && failNode.children[r] == nil {
				failNode = failNode.fail
			}

			if failNode == nil {
				child.fail = ti.root
			} else {
				child.fail = failNode.children[r]
			}

			// Merge output from fail node
			child.output = append(child.output, child.fail.output...)

			queue = append(queue, child)
		}
	}
}

// rebuildLocked rebuilds the entire automaton from the current entries map.
// Must be called with ti.mu held (write lock).
func (ti *TitleIndex) rebuildLocked() {
	ti.root = &acNode{children: make(map[rune]*acNode)}
	for _, e := range ti.entries {
		ti.insert(e.Title, e.ID)
	}
	ti.buildFailLinks()
}
