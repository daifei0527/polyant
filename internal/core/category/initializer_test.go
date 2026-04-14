package category

import (
	"context"
	"fmt"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// mockCategoryStore 实现 storage.CategoryStore 接口用于测试
type mockCategoryStore struct {
	categories map[string]*model.Category
}

func newMockCategoryStore() *mockCategoryStore {
	return &mockCategoryStore{
		categories: make(map[string]*model.Category),
	}
}

func (m *mockCategoryStore) Create(ctx context.Context, category *model.Category) (*model.Category, error) {
	m.categories[category.Path] = category
	return category, nil
}

func (m *mockCategoryStore) Get(ctx context.Context, path string) (*model.Category, error) {
	cat, ok := m.categories[path]
	if !ok {
		return nil, fmt.Errorf("category not found")
	}
	return cat, nil
}

func (m *mockCategoryStore) Update(ctx context.Context, category *model.Category) (*model.Category, error) {
	m.categories[category.Path] = category
	return category, nil
}

func (m *mockCategoryStore) Delete(ctx context.Context, path string) error {
	delete(m.categories, path)
	return nil
}

func (m *mockCategoryStore) List(ctx context.Context, parentPath string) ([]*model.Category, error) {
	var result []*model.Category
	for _, cat := range m.categories {
		if parentPath == "" || cat.ParentId == parentPath {
			result = append(result, cat)
		}
	}
	return result, nil
}

func (m *mockCategoryStore) ListAll(ctx context.Context) ([]*model.Category, error) {
	result := make([]*model.Category, 0, len(m.categories))
	for _, cat := range m.categories {
		result = append(result, cat)
	}
	return result, nil
}

func TestCategoryInitializer_Initialize(t *testing.T) {
	store := newMockCategoryStore()
	initializer := NewCategoryInitializer(store, "/nonexistent/path")

	err := initializer.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Should have created default categories
	cats, _ := store.ListAll(context.Background())
	if len(cats) == 0 {
		t.Error("Should have created default categories")
	}
}

func TestCategoryInitializer_InitializeIdempotent(t *testing.T) {
	store := newMockCategoryStore()
	initializer := NewCategoryInitializer(store, "/nonexistent/path")

	// First initialization
	err := initializer.Initialize(context.Background())
	if err != nil {
		t.Fatalf("First Initialize failed: %v", err)
	}

	cats1, _ := store.ListAll(context.Background())
	count1 := len(cats1)

	// Second initialization (should not create duplicates)
	err = initializer.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Second Initialize failed: %v", err)
	}

	cats2, _ := store.ListAll(context.Background())
	count2 := len(cats2)

	if count1 != count2 {
		t.Errorf("Second initialization should not create duplicates: %d vs %d", count1, count2)
	}
}

func TestCategoryInitializer_InitializeFromJSON(t *testing.T) {
	store := newMockCategoryStore()
	initializer := NewCategoryInitializer(store, "")

	jsonData := []byte(`{
		"categories": [
			{"id": "cat-001", "path": "test", "name": "Test Category", "parent_id": "", "level": 0, "sort_order": 1, "is_builtin": true},
			{"id": "cat-002", "path": "test/sub", "name": "Sub Category", "parent_id": "cat-001", "level": 1, "sort_order": 1, "is_builtin": false}
		]
	}`)

	err := initializer.InitializeFromJSON(context.Background(), jsonData)
	if err != nil {
		t.Fatalf("InitializeFromJSON failed: %v", err)
	}

	// Verify categories were created
	cat, err := store.Get(context.Background(), "test")
	if err != nil {
		t.Fatalf("Category 'test' should exist: %v", err)
	}
	if cat.Name != "Test Category" {
		t.Errorf("Expected name 'Test Category', got %s", cat.Name)
	}
}

func TestCategoryInitializer_InitializeFromJSON_Invalid(t *testing.T) {
	store := newMockCategoryStore()
	initializer := NewCategoryInitializer(store, "")

	// Invalid JSON
	err := initializer.InitializeFromJSON(context.Background(), []byte("invalid json"))
	if err == nil {
		t.Error("InitializeFromJSON with invalid JSON should return error")
	}
}

func TestGetDefaultSeedData(t *testing.T) {
	store := newMockCategoryStore()
	initializer := NewCategoryInitializer(store, "")

	data := initializer.getDefaultSeedData()

	if len(data.Categories) == 0 {
		t.Error("getDefaultSeedData should return categories")
	}

	// Verify structure of default categories
	for _, cat := range data.Categories {
		if cat.ID == "" {
			t.Error("Category ID should not be empty")
		}
		if cat.Path == "" {
			t.Error("Category Path should not be empty")
		}
		if cat.Name == "" {
			t.Error("Category Name should not be empty")
		}
	}
}
