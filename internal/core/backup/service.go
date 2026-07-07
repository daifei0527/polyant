package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/daifei0527/polyant/internal/storage/kv"
)

// BackupMeta is the on-disk manifest + listed-backup record.
type BackupMeta struct {
	Dir       string `json:"dir"`
	Engine    string `json:"engine"`
	CreatedAt int64  `json:"created_at"` // unix millis
	SizeBytes int64  `json:"size_bytes"`
	KeyCount  int64  `json:"key_count"`
}

// BackupResult is returned from CreateBackup.
type BackupResult struct {
	BackupMeta
}

// Service creates and lists raw-KV backups.
type Service struct {
	kvStore   kv.Store
	backupDir string
	engine    string
}

// NewService creates a backup service. backupDir is created on first CreateBackup.
func NewService(kvStore kv.Store, backupDir, engine string) *Service {
	return &Service{kvStore: kvStore, backupDir: backupDir, engine: engine}
}

// CreateBackup writes a consistent snapshot to <backupDir>/<unix-ms>/ + a manifest.
func (s *Service) CreateBackup(ctx context.Context) (*BackupResult, error) {
	_ = ctx
	ts := time.Now().UnixMilli()
	dir := filepath.Join(s.backupDir, fmt.Sprintf("%d", ts))
	// Ensure parent backupDir exists; the Backup implementation (e.g. Pebble
	// Checkpoint) creates the destination dir itself.
	if err := os.MkdirAll(s.backupDir, 0o750); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}
	if err := s.kvStore.Backup(dir); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("kv backup: %w", err)
	}
	size := dirSize(dir)
	count := s.estimateKeyCount()
	meta := BackupMeta{Dir: dir, Engine: s.engine, CreatedAt: ts, SizeBytes: size, KeyCount: count}
	mb, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), mb, 0o600); err != nil { //nolint:gosec // backup directory
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("write manifest: %w", err)
	}
	return &BackupResult{BackupMeta: meta}, nil
}

// ListBackups scans <backupDir>/*/manifest.json, newest-first.
func (s *Service) ListBackups() ([]*BackupMeta, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*BackupMeta{}, nil
		}
		return nil, err
	}
	var out []*BackupMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		mpath := filepath.Join(s.backupDir, e.Name(), "manifest.json")
		b, err := os.ReadFile(mpath)
		if err != nil {
			continue
		}
		var m BackupMeta
		if json.Unmarshal(b, &m) == nil {
			out = append(out, &m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// estimateKeyCount sums Scan results across the known key prefixes.
func (s *Service) estimateKeyCount() int64 {
	var total int64
	for _, p := range []string{
		kv.PrefixEntry, kv.PrefixUser, kv.PrefixUserEmail, kv.PrefixUserHash,
		kv.PrefixRating, kv.PrefixRatingByRater, kv.PrefixCategory,
		kv.PrefixNode, kv.PrefixMeta,
	} {
		m, err := s.kvStore.Scan([]byte(p))
		if err == nil {
			total += int64(len(m))
		}
	}
	return total
}
