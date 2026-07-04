package storage

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/stretchr/testify/require"
)

func TestMigrateTimestampsToMillis_SecondsToMillis(t *testing.T) {
	dir := t.TempDir()
	w1, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)

	// 旧数据：秒级时间戳
	sec := int64(1_700_000_000) // 2023-11，秒
	old := &model.KnowledgeEntry{
		ID: "old1", Title: "t", Content: "c", Category: "cat",
		Status: model.EntryStatusPublished, Version: 1,
		CreatedAt: sec, UpdatedAt: sec, CreatedBy: "x",
	}
	old.ContentHash = old.ComputeContentHash()
	_, err = w1.Entry.Create(context.Background(), old)
	require.NoError(t, err)
	w1.Close()

	// 重启触发迁移
	w2, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)
	defer w2.Close()
	require.NoError(t, migrateTimestampsToMillis(w2.Entry))

	got, err := w2.Entry.Get(context.Background(), "old1")
	require.NoError(t, err)
	require.Equal(t, sec*1000, got.CreatedAt, "秒级 CreatedAt 应迁移为毫秒")
	require.Equal(t, sec*1000, got.UpdatedAt, "秒级 UpdatedAt 应迁移为毫秒")
}

func TestMigrateTimestampsToMillis_Idempotent(t *testing.T) {
	dir := t.TempDir()
	w1, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)
	ms := model.NowMillis()
	alreadyMs := &model.KnowledgeEntry{
		ID: "ms1", Title: "t", Content: "c", Category: "cat",
		Status: model.EntryStatusPublished, Version: 1,
		CreatedAt: ms, UpdatedAt: ms, CreatedBy: "x",
	}
	alreadyMs.ContentHash = alreadyMs.ComputeContentHash()
	_, _ = w1.Entry.Create(context.Background(), alreadyMs)
	w1.Close()

	w2, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)
	defer w2.Close()
	require.NoError(t, migrateTimestampsToMillis(w2.Entry))
	require.NoError(t, migrateTimestampsToMillis(w2.Entry)) // 再跑一次

	got, _ := w2.Entry.Get(context.Background(), "ms1")
	require.Equal(t, ms, got.CreatedAt, "已毫秒的不应被改动（幂等）")
}

func TestMigrateTimestampsToMillis_ZeroSkipped(t *testing.T) {
	dir := t.TempDir()
	w, err := NewBadgerStoreWithCloser(dir)
	require.NoError(t, err)
	defer w.Close()
	zero := &model.KnowledgeEntry{
		ID: "z1", Title: "t", Content: "c", Category: "cat",
		Status: model.EntryStatusPublished, Version: 1,
		CreatedAt: 0, UpdatedAt: 0, CreatedBy: "x",
	}
	zero.ContentHash = zero.ComputeContentHash()
	_, _ = w.Entry.Create(context.Background(), zero)
	require.NoError(t, migrateTimestampsToMillis(w.Entry))
	got, _ := w.Entry.Get(context.Background(), "z1")
	require.Equal(t, int64(0), got.CreatedAt, "零值应跳过，不被 ×1000")
}
