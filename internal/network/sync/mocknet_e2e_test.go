package sync

import (
	"context"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/network/protocol"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

// testNode 一个 mocknet 测试节点：libp2p host + 内存存储 + SyncEngine + Protocol。
// SyncEngine 直接满足 protocol.Handler（8 个 Handle 方法齐全），无需 adapter。
type testNode struct {
	h      host.Host
	store  *storage.Store
	engine *SyncEngine
	proto  *protocol.Protocol
}

// setupMocknetNodes 起 n 个全互联 mocknet 节点，每个装配真实 store + SyncEngine + Protocol。
// 关键：SyncEngine(nil, nil, store, cfg) —— Handle* 接收侧只用 store，不需要 p2pHost/proto；
// 发送侧用 Protocol.Send* 经 mocknet 流驱动，从而绕开 P2PHost 依赖验证传输 + 接收全链路。
func setupMocknetNodes(t *testing.T, n int) []*testNode {
	t.Helper()
	mn, err := mocknet.FullMeshConnected(n)
	if err != nil {
		t.Fatalf("mocknet.FullMeshConnected(%d): %v", n, err)
	}
	hosts := mn.Hosts()
	if len(hosts) < n {
		t.Fatalf("expected %d hosts, got %d", n, len(hosts))
	}
	nodes := make([]*testNode, n)
	for i := 0; i < n; i++ {
		store, err := storage.NewMemoryStore()
		if err != nil {
			t.Fatalf("NewMemoryStore: %v", err)
		}
		engine := NewSyncEngine(nil, nil, store, &SyncConfig{AutoSync: false})
		proto := protocol.NewProtocol(hosts[i], engine)
		nodes[i] = &testNode{h: hosts[i], store: store, engine: engine, proto: proto}
	}
	return nodes
}

func mkEntry(id, content string, version int64, updatedAt int64) *model.KnowledgeEntry {
	return &model.KnowledgeEntry{
		ID: id, Title: "T-" + id, Content: content, Category: "cat",
		Status: model.EntryStatusPublished, Version: version, UpdatedAt: updatedAt,
		CreatedBy: "creator",
	}
}

// TestMocknetE2E_PushReplicates 验证 push-entry 经真实流复制到接收方存储（P1.5 e2e）。
func TestMocknetE2E_PushReplicates(t *testing.T) {
	nodes := setupMocknetNodes(t, 2)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	entry := mkEntry("e1", "content-1", 1, now)
	data, err := entry.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	ack, err := nodes[0].proto.SendPushEntry(ctx, nodes[1].h.ID(), &protocol.PushEntry{
		EntryID: entry.ID, Entry: data,
	})
	if err != nil {
		t.Fatalf("SendPushEntry: %v", err)
	}
	if !ack.Accepted {
		t.Fatalf("push should be accepted: %s", ack.RejectReason)
	}
	got, err := nodes[1].store.Entry.Get(ctx, "e1")
	if err != nil {
		t.Fatalf("receiver should have entry: %v", err)
	}
	if got.Content != "content-1" {
		t.Errorf("content = %q, want content-1", got.Content)
	}
}

// TestMocknetE2E_PushCrossReplicates 验证 A push 到 B 和 C，两者都收到（三节点扇出）。
func TestMocknetE2E_PushCrossReplicates(t *testing.T) {
	nodes := setupMocknetNodes(t, 3)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	entry := mkEntry("e-cross", "x", 1, now)
	data, _ := entry.ToJSON()
	pe := &protocol.PushEntry{EntryID: entry.ID, Entry: data}

	for _, target := range []int{1, 2} {
		ack, err := nodes[0].proto.SendPushEntry(ctx, nodes[target].h.ID(), pe)
		if err != nil {
			t.Fatalf("push to node %d: %v", target, err)
		}
		if !ack.Accepted {
			t.Fatalf("push to node %d rejected: %s", target, ack.RejectReason)
		}
		if _, err := nodes[target].store.Entry.Get(ctx, "e-cross"); err != nil {
			t.Errorf("node %d should have entry: %v", target, err)
		}
	}
}

// TestMocknetE2E_PushVersionLWW 验证 push 的版本检查：高版本覆盖，低版本被拒。
func TestMocknetE2E_PushVersionLWW(t *testing.T) {
	nodes := setupMocknetNodes(t, 2)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	// B 先有 v1
	v1 := mkEntry("e-lww", "old", 1, now)
	if _, err := nodes[1].store.Entry.Create(ctx, v1); err != nil {
		t.Fatal(err)
	}

	// A push v2（Version=2 > 1）→ B 应更新
	v2 := mkEntry("e-lww", "new", 2, now+1000)
	data, _ := v2.ToJSON()
	ack, err := nodes[0].proto.SendPushEntry(ctx, nodes[1].h.ID(), &protocol.PushEntry{EntryID: "e-lww", Entry: data})
	if err != nil {
		t.Fatalf("push v2: %v", err)
	}
	if !ack.Accepted {
		t.Fatalf("push v2 should be accepted: %s", ack.RejectReason)
	}
	got, _ := nodes[1].store.Entry.Get(ctx, "e-lww")
	if got.Version != 2 || got.Content != "new" {
		t.Errorf("after v2 push, want Version=2/new, got Version=%d/%q", got.Version, got.Content)
	}

	// A push 旧版本（Version=0 < 2）→ B 应拒绝，保留 v2
	v0 := mkEntry("e-lww", "stale", 0, now)
	data0, _ := v0.ToJSON()
	ack0, err := nodes[0].proto.SendPushEntry(ctx, nodes[1].h.ID(), &protocol.PushEntry{EntryID: "e-lww", Entry: data0})
	if err != nil {
		t.Fatalf("push v0: %v", err)
	}
	if ack0.Accepted {
		t.Error("push of older version should be rejected")
	}
	got2, _ := nodes[1].store.Entry.Get(ctx, "e-lww")
	if got2.Version != 2 || got2.Content != "new" {
		t.Errorf("B should still have v2/new after stale push, got Version=%d/%q", got2.Version, got2.Content)
	}
}

// TestMocknetE2E_SyncPullReplicates 验证增量同步拉取：A 向 B 发 SyncRequest，
// B 经 HandleSyncRequest 返回其条目，A 经 resolveConflictAndMerge 合并存入。
// 覆盖 sync 传输 + 三向合并（含 P3.2 的 forge-proof 哈希比对路径）。
func TestMocknetE2E_SyncPullReplicates(t *testing.T) {
	nodes := setupMocknetNodes(t, 2)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	// B 拥有 e-sync（Version=1, UpdatedAt>0）
	bEntry := mkEntry("e-sync", "from-b", 1, now)
	if _, err := nodes[1].store.Entry.Create(ctx, bEntry); err != nil {
		t.Fatalf("create on B: %v", err)
	}

	// A 发 SyncRequest（LastSyncTimestamp=0 + 空 VV → 请求全部）
	resp, err := nodes[0].proto.SendSyncRequest(ctx, nodes[1].h.ID(), &protocol.SyncRequest{
		RequestID:         "req-1",
		LastSyncTimestamp: 0,
	})
	if err != nil {
		t.Fatalf("SendSyncRequest: %v", err)
	}
	if len(resp.NewEntries) == 0 {
		t.Fatal("SyncResponse should include B's entry in NewEntries")
	}

	// A 合并每个新条目（resolveConflictAndMerge 对 localVersion=0 走 Create 分支）
	for _, data := range resp.NewEntries {
		var e model.KnowledgeEntry
		if err := e.FromJSON(data); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, err := nodes[0].engine.resolveConflictAndMerge(ctx, &e, 0); err != nil {
			t.Fatalf("merge: %v", err)
		}
	}

	got, err := nodes[0].store.Entry.Get(ctx, "e-sync")
	if err != nil {
		t.Fatalf("A should have entry after sync: %v", err)
	}
	if got.Content != "from-b" {
		t.Errorf("content = %q, want from-b", got.Content)
	}
}

// TestSync_MirrorDataReceivedAndStored 验证镜像请求 → 生产端推 MirrorData → 接收端 HandleMirrorData 落库。
func TestSync_MirrorDataReceivedAndStored(t *testing.T) {
	nodes := setupMocknetNodes(t, 2)
	src := nodes[0] // 镜像源
	dst := nodes[1] // 镜像方

	// 在 src 放一个 published 条目（带合法签名供 R1 验签通过；默认 RequireEntrySignatures=false 可不带）
	srcEntry := &model.KnowledgeEntry{
		ID: "mirror-e1", Title: "镜像条目", Content: "内容", Category: "cat",
		Status: model.EntryStatusPublished, Version: 1,
		CreatedAt: model.NowMillis(), UpdatedAt: model.NowMillis(),
		CreatedBy: "src-creator",
	}
	_, err := src.store.Entry.Create(context.Background(), srcEntry)
	if err != nil {
		t.Fatalf("create on src: %v", err)
	}

	// 需要先 Start engine 以使 HandleMirrorRequest 生产者能使用服务级 ctx
	ctx := context.Background()
	if err := src.engine.Start(ctx); err != nil {
		t.Fatalf("start src engine: %v", err)
	}
	defer src.engine.Stop()

	// dst 向 src 发 MirrorRequest，src 推 MirrorData 回 dst，dst 的 HandleMirrorData 落库
	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = dst.proto.SendMirrorRequest(reqCtx, src.h.ID(), &protocol.MirrorRequest{
		RequestID: "req1", Categories: []string{"cat"},
	})
	if err != nil {
		t.Fatalf("SendMirrorRequest: %v", err)
	}

	// 轮询 dst store，等待镜像条目出现
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, gerr := dst.store.Entry.Get(context.Background(), "mirror-e1")
		if gerr == nil && got != nil {
			if got.Title != "镜像条目" {
				t.Errorf("镜像条目 title = %q, want 镜像条目", got.Title)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("镜像条目未在接收端落库（HandleMirrorData 未实现或未路由）")
}
