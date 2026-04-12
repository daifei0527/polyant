package linkparser

import (
	"sort"
	"testing"
)

func TestParseLinks_WikiLinks(t *testing.T) {
	content := `This is a test with [[link1]] and [[link2]].
Another line with [[link3|display text]].`

	links := ParseLinks(content)

	if len(links) != 3 {
		t.Errorf("Expected 3 links, got %d", len(links))
	}

	// Sort for consistent testing
	sort.Strings(links)

	expected := []string{"link1", "link2", "link3"}
	for i, exp := range expected {
		if links[i] != exp {
			t.Errorf("Expected link %s, got %s", exp, links[i])
		}
	}
}

func TestParseLinks_EntryScheme(t *testing.T) {
	content := `Check out [this entry](entry://entry-123) for more info.
Also see [another](entry://entry-456).`

	links := ParseLinks(content)

	if len(links) != 2 {
		t.Errorf("Expected 2 links, got %d", len(links))
	}
}

func TestParseLinks_HashRoute(t *testing.T) {
	content := `See [documentation](/#/entry/doc-1) for details.
Navigate to [tutorial](/#/entry/tutorial-2).`

	links := ParseLinks(content)

	if len(links) != 2 {
		t.Errorf("Expected 2 links, got %d", len(links))
	}
}

func TestParseLinks_MixedFormats(t *testing.T) {
	content := `Various link formats:
- Wiki link: [[entry-a]]
- Entry scheme: [text](entry://entry-b)
- Hash route: [text](/#/entry/entry-c)
All three should be detected.`

	links := ParseLinks(content)

	if len(links) != 3 {
		t.Errorf("Expected 3 links, got %d: %v", len(links), links)
	}
}

func TestParseLinks_Deduplication(t *testing.T) {
	content := `This has [[duplicate]] and [[duplicate]] again.
Also [dup](entry://duplicate) again.`

	links := ParseLinks(content)

	if len(links) != 1 {
		t.Errorf("Expected 1 unique link, got %d: %v", len(links), links)
	}
}

func TestParseLinks_EmptyContent(t *testing.T) {
	links := ParseLinks("")

	if len(links) != 0 {
		t.Errorf("Expected 0 links for empty content, got %d", len(links))
	}
}

func TestParseLinks_NoLinks(t *testing.T) {
	content := `This is just plain text without any links.
No wiki links, no entry:// schemes, no hash routes.`

	links := ParseLinks(content)

	if len(links) != 0 {
		t.Errorf("Expected 0 links, got %d", len(links))
	}
}

func TestParseLinks_Whitespace(t *testing.T) {
	content := `Link with spaces: [[ spaced-id ]] and [text](entry:// spaced-entry )`

	links := ParseLinks(content)

	// Links should be trimmed
	for _, link := range links {
		if link != "spaced-id" && link != "spaced-entry" {
			t.Errorf("Link should be trimmed: %q", link)
		}
	}
}

func TestParseLinksWithText_WikiLinks(t *testing.T) {
	content := `[[entry-1]] and [[entry-2|Display Text]]`

	links := ParseLinksWithText(content)

	if len(links) != 2 {
		t.Fatalf("Expected 2 links, got %d", len(links))
	}

	// Find entry-1
	var found1, found2 bool
	for _, link := range links {
		if link.EntryID == "entry-1" {
			found1 = true
			if link.DisplayText != "entry-1" {
				t.Errorf("Display text for entry-1 should be 'entry-1', got %s", link.DisplayText)
			}
		}
		if link.EntryID == "entry-2" {
			found2 = true
			if link.DisplayText != "Display Text" {
				t.Errorf("Display text for entry-2 should be 'Display Text', got %s", link.DisplayText)
			}
		}
	}

	if !found1 {
		t.Error("entry-1 not found")
	}
	if !found2 {
		t.Error("entry-2 not found")
	}
}

func TestParseLinksWithText_EntryScheme(t *testing.T) {
	content := `[Click here](entry://doc-1) for documentation.`

	links := ParseLinksWithText(content)

	if len(links) != 1 {
		t.Fatalf("Expected 1 link, got %d", len(links))
	}

	if links[0].EntryID != "doc-1" {
		t.Errorf("Expected EntryID 'doc-1', got %s", links[0].EntryID)
	}
	if links[0].DisplayText != "Click here" {
		t.Errorf("Expected DisplayText 'Click here', got %s", links[0].DisplayText)
	}
}

func TestParseLinksWithText_HashRoute(t *testing.T) {
	content := `[View Tutorial](/#/entry/tutorial-1)`

	links := ParseLinksWithText(content)

	if len(links) != 1 {
		t.Fatalf("Expected 1 link, got %d", len(links))
	}

	if links[0].EntryID != "tutorial-1" {
		t.Errorf("Expected EntryID 'tutorial-1', got %s", links[0].EntryID)
	}
	if links[0].DisplayText != "View Tutorial" {
		t.Errorf("Expected DisplayText 'View Tutorial', got %s", links[0].DisplayText)
	}
}

func TestParseLinksWithText_Deduplication(t *testing.T) {
	content := `[[same-id]] and [text](entry://same-id)`

	links := ParseLinksWithText(content)

	if len(links) != 1 {
		t.Errorf("Expected 1 unique link, got %d", len(links))
	}
}

func TestParseLinksWithText_EmptyContent(t *testing.T) {
	links := ParseLinksWithText("")

	if len(links) != 0 {
		t.Errorf("Expected 0 links for empty content, got %d", len(links))
	}
}

func TestLinkInfo(t *testing.T) {
	link := LinkInfo{
		EntryID:     "test-entry",
		DisplayText: "Test Display",
	}

	if link.EntryID != "test-entry" {
		t.Errorf("EntryID mismatch")
	}
	if link.DisplayText != "Test Display" {
		t.Errorf("DisplayText mismatch")
	}
}

func TestParseLinks_ComplexMarkdown(t *testing.T) {
	content := `# Title

This is a complex document with multiple link types.

## Section 1
- Wiki link: [[category/sub-entry]]
- With display: [[entry-id|Custom Display]]
- Multiple: [[a]] [[b]] [[c]]

## Section 2
Regular [markdown link](https://example.com) should be ignored.
But [entry link](entry://special-entry) should be found.

## Section 3
Route style: [UI Link](/#/entry/ui-entry)

## Code blocks should be parsed too
` + "```" + `
[[code-block-link]]
` + "```" + `
`

	links := ParseLinks(content)

	// Should find all internal links
	expectedCount := 8 // category/sub-entry, entry-id, a, b, c, special-entry, ui-entry, code-block-link
	if len(links) != expectedCount {
		t.Errorf("Expected %d links, got %d: %v", expectedCount, len(links), links)
	}
}
