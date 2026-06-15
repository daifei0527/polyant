package integrity

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// fakeAudit 捕获守护进程写入的审计日志，用于断言。
type fakeAudit struct {
	mu   sync.Mutex
	logs []*model.AuditLog
}

func (f *fakeAudit) Log(_ context.Context, log *model.AuditLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, log)
	return nil
}

func (f *fakeAudit) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.logs)
}

func (f *fakeAudit) targetIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, len(f.logs))
	for i, l := range f.logs {
		ids[i] = l.TargetID
	}
	return ids
}

// newTestStore 构造一个内存存储并写入给定条目（MemoryEntryStore.Create 不重算哈希，
// 因此测试可直接注入篡改的 ContentHash 来模拟落盘后篡改）。
func newTestStore(t *testing.T, entries ...*model.KnowledgeEntry) *storage.Store {
	t.Helper()
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	ctx := context.Background()
	for _, e := range entries {
		if _, err := store.Entry.Create(ctx, e); err != nil {
			t.Fatalf("Create %s: %v", e.ID, err)
		}
	}
	return store
}

func mkEntry(id, title, content string, hash string) *model.KnowledgeEntry {
	return &model.KnowledgeEntry{
		ID:          id,
		Title:       title,
		Content:     content,
		Category:    "test",
		Status:      model.EntryStatusPublished,
		ContentHash: hash,
	}
}

// TestIntegrityChecker_DetectsTamper 验证守护进程检出 ContentHash 不一致的条目并记审计。
func TestIntegrityChecker_DetectsTamper(t *testing.T) {
	good := mkEntry("good-1", "Good", "content-a", "")
	good.ContentHash = good.ComputeContentHash() // 正确哈希

	// 篡改条目：存储的 ContentHash 与内容不匹配（模拟落盘后被直接改库）
	bad := mkEntry("bad-1", "Bad", "content-b", "deadbeef")

	// 空 ContentHash 条目：未受完整性保护，应跳过（无误报）
	empty := mkEntry("empty-1", "Empty", "content-c", "")

	store := newTestStore(t, good, bad, empty)
	fa := &fakeAudit{}

	c := NewIntegrityChecker(store, fa, time.Minute, 10)
	c.checkOnce(context.Background())

	if got := fa.count(); got != 1 {
		t.Fatalf("expected exactly 1 tamper audit (only bad-1), got %d: %v", got, fa.targetIDs())
	}
	if fa.logs[0].TargetID != "bad-1" {
		t.Errorf("expected tamper audit for bad-1, got %s", fa.logs[0].TargetID)
	}
	if fa.logs[0].ActionType != "integrity.tamper" {
		t.Errorf("expected ActionType integrity.tamper, got %s", fa.logs[0].ActionType)
	}
	if fa.logs[0].Success {
		t.Error("tamper audit should record Success=false")
	}
}

// TestIntegrityChecker_CleanStoreNoFalsePositive 验证全合法条目无误报。
func TestIntegrityChecker_CleanStoreNoFalsePositive(t *testing.T) {
	e1 := mkEntry("e1", "T1", "c1", "")
	e1.ContentHash = e1.ComputeContentHash()
	e2 := mkEntry("e2", "T2", "c2", "")
	e2.ContentHash = e2.ComputeContentHash()

	store := newTestStore(t, e1, e2)
	fa := &fakeAudit{}

	c := NewIntegrityChecker(store, fa, time.Minute, 10)
	c.checkOnce(context.Background())

	if got := fa.count(); got != 0 {
		t.Errorf("expected 0 tamper audits for clean store, got %d: %v", got, fa.targetIDs())
	}
}

// TestIntegrityChecker_NilAuditNoPanic 验证未配置审计后端时仅记日志不崩溃。
func TestIntegrityChecker_NilAuditNoPanic(t *testing.T) {
	bad := mkEntry("bad-1", "Bad", "content-b", "deadbeef")
	store := newTestStore(t, bad)

	c := NewIntegrityChecker(store, nil, time.Minute, 10)
	// 不应 panic
	c.checkOnce(context.Background())
}

// TestIntegrityChecker_Defaults 验证 interval/batchSize 默认值。
func TestIntegrityChecker_Defaults(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	c := NewIntegrityChecker(store, nil, 0, 0)
	if c.interval != 15*time.Minute {
		t.Errorf("default interval should be 15m, got %v", c.interval)
	}
	if c.batchSize != 500 {
		t.Errorf("default batch size should be 500, got %d", c.batchSize)
	}
}
