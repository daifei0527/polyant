package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/kv"
)

func newStore(t *testing.T) (*storage.Store, kv.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewPersistentStore(&storage.StoreConfig{
		KVType: "pebble", KVPath: filepath.Join(dir, "kv"),
		SearchType: "memory", SearchPath: filepath.Join(dir, "search"),
	})
	if err != nil {
		t.Fatalf("NewPersistentStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store, store.KVStore()
}

func TestCreateBackup_WritesManifest(t *testing.T) {
	_, kvStore := newStore(t)
	kvStore.Put([]byte("entry:a"), []byte("1"))

	svc := NewService(kvStore, t.TempDir(), "pebble")
	res, err := svc.CreateBackup(context.Background())
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if res.SizeBytes <= 0 {
		t.Error("SizeBytes should be > 0")
	}
	if res.KeyCount < 1 {
		t.Error("KeyCount should include entry:a")
	}
	if _, err := os.Stat(filepath.Join(res.Dir, "manifest.json")); err != nil {
		t.Errorf("manifest.json missing: %v", err)
	}
}

func TestListBackups_SortedDesc(t *testing.T) {
	_, kvStore := newStore(t)
	svc := NewService(kvStore, t.TempDir(), "pebble")
	r1, _ := svc.CreateBackup(context.Background())
	r2, _ := svc.CreateBackup(context.Background())
	list, err := svc.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 backups, got %d", len(list))
	}
	if list[0].CreatedAt < list[1].CreatedAt {
		t.Error("ListBackups should be newest-first")
	}
	_ = r1
	_ = r2
}
