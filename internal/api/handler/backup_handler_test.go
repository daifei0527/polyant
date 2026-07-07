package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/core/backup"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/kv"
)

func newBackupHandler(t *testing.T) (*BackupHandler, kv.Store) {
	t.Helper()
	dir := t.TempDir()
	store, _ := storage.NewPersistentStore(&storage.StoreConfig{
		KVType: "pebble", KVPath: dir + "/kv", SearchType: "memory", SearchPath: dir + "/s",
	})
	t.Cleanup(func() { store.Close() })
	svc := backup.NewService(store.KVStore(), t.TempDir(), "pebble")
	return NewBackupHandler(svc), store.KVStore()
}

func TestBackupHandler_Create(t *testing.T) {
	h, kvStore := newBackupHandler(t)
	kvStore.Put([]byte("entry:a"), []byte("1"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/backup", nil)
	req = req.WithContext(context.Background())
	rec := httptest.NewRecorder()
	h.CreateBackupHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBackupHandler_List(t *testing.T) {
	h, kvStore := newBackupHandler(t)
	kvStore.Put([]byte("entry:a"), []byte("1"))
	h.svc.CreateBackup(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups", nil)
	rec := httptest.NewRecorder()
	h.ListBackupsHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
