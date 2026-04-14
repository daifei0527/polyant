package seed

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func TestNewSeedDataInitializer(t *testing.T) {
	store, _ := storage.NewMemoryStore()

	initializer := NewSeedDataInitializer(store, "/custom/path")
	if initializer == nil {
		t.Fatal("NewSeedDataInitializer returned nil")
	}

	if initializer.seedDataDir != "/custom/path" {
		t.Errorf("Expected seedDataDir '/custom/path', got %s", initializer.seedDataDir)
	}

	// Default path
	initializer2 := NewSeedDataInitializer(store, "")
	if initializer2.seedDataDir != "./configs/seed-data" {
		t.Errorf("Default seedDataDir should be './configs/seed-data', got %s", initializer2.seedDataDir)
	}
}

func TestSeedDataInitializer_Initialize_Empty(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	initializer := NewSeedDataInitializer(store, "/nonexistent/path")

	err := initializer.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize with no data should not fail: %v", err)
	}

	// Should be marked as initialized
	if !initializer.initDone {
		t.Error("Should be marked as initialized")
	}
}

func TestSeedDataInitializer_Initialize_Idempotent(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	initializer := NewSeedDataInitializer(store, "/nonexistent/path")

	// First initialization
	err := initializer.Initialize(context.Background())
	if err != nil {
		t.Fatalf("First Initialize failed: %v", err)
	}

	// Second initialization (should be no-op)
	err = initializer.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Second Initialize failed: %v", err)
	}
}

func TestSeedDataInitializer_Initialize_WithExistingData(t *testing.T) {
	store, _ := storage.NewMemoryStore()

	// Add an existing entry
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{
		ID:      "existing-entry",
		Title:   "Existing",
		Content: "Content",
	})

	initializer := NewSeedDataInitializer(store, "/nonexistent/path")

	err := initializer.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize with existing data should not fail: %v", err)
	}
}

func TestSeedDataInitializer_ImportFromFile(t *testing.T) {
	// Create temp directory and file
	tmpDir, err := os.MkdirTemp("", "seed-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test JSONL file
	testFile := filepath.Join(tmpDir, "test_entries.jsonl")
	testData := `{"id":"entry-1","title":"Test Entry 1","content":"Content 1","category":"test","status":"published"}
{"id":"entry-2","title":"Test Entry 2","content":"Content 2","category":"test","status":"published"}`
	err = os.WriteFile(testFile, []byte(testData), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	store, _ := storage.NewMemoryStore()
	initializer := NewSeedDataInitializer(store, tmpDir)

	// Import entries
	count, err := initializer.ImportFromFile(context.Background(), testFile)
	if err != nil {
		t.Fatalf("ImportFromFile failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 imported entries, got %d", count)
	}

	// Verify entries were imported
	entry1, err := store.Entry.Get(context.Background(), "entry-1")
	if err != nil {
		t.Fatalf("Entry 1 should exist: %v", err)
	}
	if entry1.Title != "Test Entry 1" {
		t.Errorf("Expected title 'Test Entry 1', got %s", entry1.Title)
	}
}

func TestSeedDataInitializer_ImportFromFile_Nonexistent(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	initializer := NewSeedDataInitializer(store, "/nonexistent/path")

	_, err := initializer.ImportFromFile(context.Background(), "/nonexistent/file.jsonl")
	if err == nil {
		t.Error("ImportFromFile with nonexistent file should return error")
	}
}

func TestSeedDataInitializer_ImportFromFile_InvalidJSON(t *testing.T) {
	// Create temp file with invalid JSON
	tmpDir, err := os.MkdirTemp("", "seed-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "invalid.jsonl")
	err = os.WriteFile(testFile, []byte("invalid json\n{not valid}"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	store, _ := storage.NewMemoryStore()
	initializer := NewSeedDataInitializer(store, tmpDir)

	// Should skip invalid lines but not fail
	count, err := initializer.ImportFromFile(context.Background(), testFile)
	if err != nil {
		t.Fatalf("ImportFromFile should not fail: %v", err)
	}

	// Invalid lines should be skipped
	if count != 0 {
		t.Errorf("Expected 0 valid entries, got %d", count)
	}
}

func TestSeedDataImporter_GetSeedEntriesCount(t *testing.T) {
	store, _ := storage.NewMemoryStore()

	// Nonexistent file
	initializer := NewSeedDataInitializer(store, "/nonexistent/path")
	count := initializer.GetSeedEntriesCount()
	if count != 0 {
		t.Errorf("Nonexistent file should return 0, got %d", count)
	}
}

func TestSeedDataInitializer_GetSeedEntriesCount_WithFile(t *testing.T) {
	// Create temp directory and file
	tmpDir, err := os.MkdirTemp("", "seed-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "default_entries.jsonl")
	testData := `{"id":"entry-1","title":"Test"}
{"id":"entry-2","title":"Test"}
{"id":"entry-3","title":"Test"}`
	err = os.WriteFile(testFile, []byte(testData), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	store, _ := storage.NewMemoryStore()
	initializer := NewSeedDataInitializer(store, tmpDir)

	count := initializer.GetSeedEntriesCount()
	if count != 3 {
		t.Errorf("Expected 3 entries, got %d", count)
	}
}

func TestSeedDataInitializer_IsInitialized(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	initializer := NewSeedDataInitializer(store, "/nonexistent/path")

	if initializer.IsInitialized() {
		t.Error("Should not be initialized initially")
	}

	initializer.Initialize(context.Background())

	if !initializer.IsInitialized() {
		t.Error("Should be initialized after Initialize")
	}
}
