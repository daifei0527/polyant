package storage

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestBadgerEntryStore_PublishedCount 验证 published 条目计数器：Create 增量、
// Delete 递减、状态变更调整，且节点重启时从全量数据重建对账。
func TestBadgerEntryStore_PublishedCount(t *testing.T) {
	dir := t.TempDir()
	w, err := NewBadgerStoreWithCloser(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	ctx := context.Background()

	mk := func(id string, status string) *model.KnowledgeEntry {
		return &model.KnowledgeEntry{
			ID: id, Title: "t", Content: "c", Category: "cat", Status: status,
		}
	}

	// 初始为 0（启动重建在空库上设 0）
	if n, _ := w.Entry.Count(ctx); n != 0 {
		t.Fatalf("initial count = %d, want 0", n)
	}

	// 2 published + 1 draft
	if _, err := w.Entry.Create(ctx, mk("p1", model.EntryStatusPublished)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Entry.Create(ctx, mk("p2", model.EntryStatusPublished)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Entry.Create(ctx, mk("d1", model.EntryStatusDraft)); err != nil {
		t.Fatal(err)
	}
	if n, _ := w.Entry.Count(ctx); n != 2 {
		t.Errorf("after create 2 published + 1 draft, count = %d, want 2", n)
	}

	// 删除一个 published → 1
	if err := w.Entry.Delete(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if n, _ := w.Entry.Count(ctx); n != 1 {
		t.Errorf("after delete p1, count = %d, want 1", n)
	}

	// draft → published（状态变更）→ 2
	upd := mk("d1", model.EntryStatusPublished)
	if _, err := w.Entry.Update(ctx, upd); err != nil {
		t.Fatal(err)
	}
	if n, _ := w.Entry.Count(ctx); n != 2 {
		t.Errorf("after draft->published, count = %d, want 2", n)
	}

	// 重启：重新打开应从全量数据重建计数器为 2（对账任何增量漂移）
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	w2, err := NewBadgerStoreWithCloser(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer w2.Close()
	if n, _ := w2.Entry.Count(ctx); n != 2 {
		t.Errorf("after reopen rebuild, count = %d, want 2", n)
	}
}
