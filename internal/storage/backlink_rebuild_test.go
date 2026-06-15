package storage

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestBacklinkIndex_RebuiltOnStartup 验证反向链接索引在节点重启后从持久化条目重建。
// 创建一个 source 条目（内容用 [[target-1]] 链接 target）后关闭存储，重新打开时
// 启动路径应重新解析链接并重建 backlink，使 GetBacklinks("target-1") 命中 source。
func TestBacklinkIndex_RebuiltOnStartup(t *testing.T) {
	dir := t.TempDir()

	// 第一次打开：写入两个 published 条目（storage 层 Create 不更新 backlink，
	// 那是 handler 层职责——所以关闭前 backlink 索引为空）。
	w1, err := NewBadgerStoreWithCloser(dir)
	if err != nil {
		t.Fatalf("open store (1st): %v", err)
	}
	target := &model.KnowledgeEntry{
		ID:       "target-1",
		Title:    "Target",
		Content:  "Target content.",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	source := &model.KnowledgeEntry{
		ID:       "source-1",
		Title:    "Source",
		Content:  "See [[target-1]] for details.",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	if _, err := w1.Entry.Create(context.Background(), target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	if _, err := w1.Entry.Create(context.Background(), source); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("close store (1st): %v", err)
	}

	// 第二次打开：启动时应从持久化条目重建 backlink。
	w2, err := NewBadgerStoreWithCloser(dir)
	if err != nil {
		t.Fatalf("reopen store (2nd): %v", err)
	}
	defer w2.Close()

	backlinks, err := w2.Backlink.GetBacklinks("target-1")
	if err != nil {
		t.Fatalf("GetBacklinks after rebuild: %v", err)
	}
	if len(backlinks) != 1 || backlinks[0] != "source-1" {
		t.Errorf("expected backlinks [source-1] after rebuild, got %v", backlinks)
	}
}
