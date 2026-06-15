package sync

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestResolveConflictAndMerge_ForgedHashDoesNotBypassConflict 锁定 sync.go 中
// "内容相同"比较的安全属性：必须比对【重算】的内容哈希，而非远端提供的 ContentHash 字段。
//
// 攻击场景：远端条目内容被篡改为 "TAMPERED"，但把 ContentHash 字段伪造成本地条目的
// 真实哈希，并设置更旧的 UpdatedAt。若直接比字段值（旧实现），伪造哈希会触发"内容相同"
// 快捷路径，使更旧的篡改条目静默覆盖较新的本地条目。修复后比对重算哈希会识别出内容不同，
// 走冲突路径并由 LWW 保留更新的本地条目。
func TestResolveConflictAndMerge_ForgedHashDoesNotBypassConflict(t *testing.T) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	ctx := context.Background()

	// 本地条目（authentic，UpdatedAt 较新）
	local := &model.KnowledgeEntry{
		ID:        "e1",
		Title:     "T",
		Content:   "original",
		Category:  "c",
		Version:   1,
		UpdatedAt: 2000,
		Status:    model.EntryStatusPublished,
	}
	local.ContentHash = local.ComputeContentHash()
	if _, err := store.Entry.Create(ctx, local); err != nil {
		t.Fatalf("create local: %v", err)
	}

	engine := NewSyncEngine(nil, nil, store, &SyncConfig{AutoSync: false})

	// 伪造远端：内容为 "TAMPERED"，但 ContentHash 字段伪造为本地的真实哈希；
	// 版本相同，UpdatedAt 更旧。
	remote := &model.KnowledgeEntry{
		ID:        "e1",
		Title:     "T",
		Content:   "TAMPERED",
		Category:  "c",
		Version:   1,
		UpdatedAt: 1000,
		Status:    model.EntryStatusPublished,
	}
	remote.ContentHash = local.ContentHash // 伪造的哈希字段

	// localVersion=1 表示本地已存在
	result, err := engine.resolveConflictAndMerge(ctx, remote, 1)
	if err != nil {
		t.Fatalf("resolveConflictAndMerge: %v", err)
	}

	// 修复后：重算比对识别内容不同 → 冲突 → LWW 保留更新的本地条目。
	// 旧实现（比字段值）会让伪造哈希触发"内容相同"，用更旧的篡改内容覆盖本地。
	if result.Content != "original" {
		t.Errorf("forged-hash older remote must not overwrite newer local: got content=%q, want %q",
			result.Content, "original")
	}
}

// TestResolveConflictAndMerge_SameContentStillFastPath 验证修复后真正相同的内容
// 仍走"内容相同"快捷路径（版本号更新），即修复没有破坏合法场景。
func TestResolveConflictAndMerge_SameContentStillFastPath(t *testing.T) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	ctx := context.Background()

	local := &model.KnowledgeEntry{
		ID:        "e2",
		Title:     "T",
		Content:   "same",
		Category:  "c",
		Version:   1,
		UpdatedAt: 1000,
		Status:    model.EntryStatusPublished,
	}
	local.ContentHash = local.ComputeContentHash()
	store.Entry.Create(ctx, local)

	engine := NewSyncEngine(nil, nil, store, &SyncConfig{AutoSync: false})

	// 远端内容真正相同（仅版本更高），ContentHash 由重算得出
	remote := &model.KnowledgeEntry{
		ID:        "e2",
		Title:     "T",
		Content:   "same",
		Category:  "c",
		Version:   2,
		UpdatedAt: 2000,
		Status:    model.EntryStatusPublished,
	}
	remote.ContentHash = remote.ComputeContentHash()

	result, err := engine.resolveConflictAndMerge(ctx, remote, 1)
	if err != nil {
		t.Fatalf("resolveConflictAndMerge: %v", err)
	}
	if result.Version != 2 {
		t.Errorf("same-content fast path should adopt remote version 2, got %d", result.Version)
	}
	if result.Content != "same" {
		t.Errorf("content should be unchanged, got %q", result.Content)
	}
}
