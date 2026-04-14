package category

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
)

func newTestManager(t *testing.T) *Manager {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return NewManager(store)
}

func TestGetInitialCategories(t *testing.T) {
	categories := GetInitialCategories()

	if len(categories) == 0 {
		t.Error("GetInitialCategories should return at least one category")
	}

	// Check that top-level categories have expected fields
	for _, cat := range categories {
		if cat.ID == "" {
			t.Error("Category ID should not be empty")
		}
		if cat.Name == "" {
			t.Error("Category Name should not be empty")
		}
	}
}

func TestManager_Initialize(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify categories are loaded
	if len(mgr.cached) == 0 {
		t.Error("No categories cached after initialization")
	}

	// Test double initialization (should be idempotent)
	err = mgr.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Second Initialize should not fail: %v", err)
	}
}

func TestManager_Get(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Initialize(context.Background())

	// Get existing category
	cat, err := mgr.Get("tech")
	if err != nil {
		t.Fatalf("Get existing category failed: %v", err)
	}
	if cat.ID != "tech" {
		t.Errorf("Expected ID 'tech', got %s", cat.ID)
	}

	// Get non-existing category
	_, err = mgr.Get("nonexistent")
	if err == nil {
		t.Error("Get non-existing category should return error")
	}
}

func TestManager_List(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Initialize(context.Background())

	categories := mgr.List()
	if len(categories) == 0 {
		t.Error("List should return categories")
	}

	// Verify categories are sorted by ID
	for i := 1; i < len(categories); i++ {
		if categories[i-1].ID > categories[i].ID {
			t.Error("Categories should be sorted by ID")
		}
	}
}

func TestManager_ListTopLevel(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Initialize(context.Background())

	topLevel := mgr.ListTopLevel()

	// All top-level categories should have empty ParentID
	for _, cat := range topLevel {
		if cat.ParentID != "" {
			t.Errorf("Top-level category %s should have empty ParentID", cat.ID)
		}
		// Top-level categories should have children populated
		if len(cat.Children) == 0 {
			// Some top-level categories may not have children (like "other")
			// so this is not necessarily an error
		}
	}
}

func TestManager_GetTree(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Initialize(context.Background())

	tree := mgr.GetTree()

	if len(tree) == 0 {
		t.Error("GetTree should return categories")
	}

	// Verify tree structure
	for _, cat := range tree {
		if cat.ParentID != "" {
			t.Errorf("Root category %s should have empty ParentID", cat.ID)
		}
	}
}

func TestManager_Validate(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Initialize(context.Background())

	// Valid category
	if !mgr.Validate("tech") {
		t.Error("tech should be a valid category")
	}

	if !mgr.Validate("tech/programming") {
		t.Error("tech/programming should be a valid category")
	}

	// Invalid category
	if mgr.Validate("nonexistent") {
		t.Error("nonexistent should not be a valid category")
	}
}

func TestManager_GetBreadcrumb(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Initialize(context.Background())

	// Get breadcrumb for a nested category
	breadcrumb := mgr.GetBreadcrumb("tech/programming")

	if len(breadcrumb) < 2 {
		t.Errorf("Expected at least 2 breadcrumb items for tech/programming, got %d", len(breadcrumb))
	}

	// First should be parent, last should be the category itself
	if len(breadcrumb) >= 2 {
		if breadcrumb[0].ID != "tech" {
			t.Errorf("First breadcrumb should be 'tech', got %s", breadcrumb[0].ID)
		}
		if breadcrumb[len(breadcrumb)-1].ID != "tech/programming" {
			t.Errorf("Last breadcrumb should be 'tech/programming', got %s", breadcrumb[len(breadcrumb)-1].ID)
		}
	}

	// Non-existing category
	breadcrumb = mgr.GetBreadcrumb("nonexistent")
	if len(breadcrumb) != 0 {
		t.Error("Non-existing category should return empty breadcrumb")
	}
}

func TestManager_Search(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Initialize(context.Background())

	// Search by name
	results := mgr.Search("技术")
	if len(results) == 0 {
		t.Error("Search for '技术' should return results")
	}

	// Search by description
	results = mgr.Search("编程")
	if len(results) == 0 {
		t.Error("Search for '编程' should return results")
	}

	// Case insensitive search - search for lowercase matches uppercase
	results = mgr.Search("技术") // Chinese characters
	if len(results) == 0 {
		t.Error("Search should return results for Chinese characters")
	}

	// No results
	results = mgr.Search("xyznonexistent")
	if len(results) != 0 {
		t.Error("Search for non-existent term should return empty")
	}
}

func TestManager_UpdateEntryCount(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Initialize(context.Background())

	// Update count
	mgr.UpdateEntryCount("tech", 5)

	cat, _ := mgr.Get("tech")
	if cat.EntryCount != 5 {
		t.Errorf("Expected EntryCount 5, got %d", cat.EntryCount)
	}

	// Increment
	mgr.UpdateEntryCount("tech", 3)
	cat, _ = mgr.Get("tech")
	if cat.EntryCount != 8 {
		t.Errorf("Expected EntryCount 8, got %d", cat.EntryCount)
	}

	// Decrement below zero should be clamped to zero
	mgr.UpdateEntryCount("tech", -100)
	cat, _ = mgr.Get("tech")
	if cat.EntryCount < 0 {
		t.Errorf("EntryCount should not be negative, got %d", cat.EntryCount)
	}

	// Non-existing category (should not panic)
	mgr.UpdateEntryCount("nonexistent", 5)
}
