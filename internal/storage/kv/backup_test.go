package kv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestBackupRestoreRoundtrip_Pebble verifies PebbleStore.Backup produces a
// directory that can be reopened as a valid Pebble DB containing the same keys.
func TestBackupRestoreRoundtrip_Pebble(t *testing.T) {
	srcDir := t.TempDir()
	store, err := NewPebbleStore(srcDir)
	if err != nil {
		t.Fatalf("NewPebbleStore: %v", err)
	}

	seed := []struct{ k, v string }{{"entry:a", "1"}, {"entry:b", "2"}, {"user:x", "9"}}
	for _, e := range seed {
		if err := store.Put([]byte(e.k), []byte(e.v)); err != nil {
			t.Fatalf("Put %s: %v", e.k, err)
		}
	}

	backupDir := filepath.Join(t.TempDir(), "bk")
	if err := store.Backup(backupDir); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	// backup dir must exist and be non-empty
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("backup dir is empty")
	}

	// close source before reopening the checkpoint as a separate DB
	if err := store.Close(); err != nil {
		t.Fatalf("close src: %v", err)
	}
	restored, err := NewPebbleStore(backupDir)
	if err != nil {
		t.Fatalf("reopen backup: %v", err)
	}
	defer restored.Close()
	for _, e := range seed {
		got, err := restored.Get([]byte(e.k))
		if err != nil {
			t.Errorf("Get %s from restored: %v", e.k, err)
			continue
		}
		if string(got) != e.v {
			t.Errorf("Get %s = %q, want %q", e.k, got, e.v)
		}
	}
}

// TestRunGC_Pebble verifies RunGC does not panic on a small Pebble DB.
func TestRunGC_Pebble(t *testing.T) {
	store, err := NewPebbleStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewPebbleStore: %v", err)
	}
	defer store.Close()
	store.Put([]byte("k"), []byte("v"))
	if err := store.RunGC(); err != nil {
		t.Fatalf("RunGC: %v", err)
	}
}

// TestBackup_Memory verifies the generic dump path (used by tests/dev).
func TestBackup_Memory(t *testing.T) {
	m := NewMemoryStore()
	m.Put([]byte("k1"), []byte("v1"))
	backupDir := filepath.Join(t.TempDir(), "bk")
	if err := m.Backup(backupDir); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "dump.json")); err != nil {
		t.Fatalf("dump.json missing: %v", err)
	}
	if err := m.RunGC(); err != nil {
		t.Fatalf("Memory RunGC: %v", err)
	}
}

// Reference model package so the import isn't dropped if unused later.
var _ = model.EntryStatusPublished
