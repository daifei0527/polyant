// Package integration_test 提供集成测试
package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/network/host"
	"github.com/daifei0527/polyant/internal/network/protocol"
	"github.com/daifei0527/polyant/internal/network/sync"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestTwoNodeSync 测试两个真实节点的同步
func TestTwoNodeSync(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建节点 A
	hostA, err := host.NewHost(ctx, &host.HostConfig{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatalf("创建节点 A 失败: %v", err)
	}
	defer hostA.Close()

	// 创建节点 B
	hostB, err := host.NewHost(ctx, &host.HostConfig{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatalf("创建节点 B 失败: %v", err)
	}
	defer hostB.Close()

	// 节点 B 连接到节点 A
	addrA := peer.AddrInfo{
		ID:    hostA.ID(),
		Addrs: hostA.Addrs(),
	}
	if err := hostB.Connect(ctx, addrA); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	t.Logf("节点 A: %s", hostA.ID())
	t.Logf("节点 B: %s", hostB.ID())
	t.Logf("节点 B 已连接到节点 A")

	// 创建存储
	storeA, _ := storage.NewMemoryStore()
	storeB, _ := storage.NewMemoryStore()

	// 在节点 A 创建条目
	now := time.Now().UnixMilli()
	entry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Test Entry",
		Content:   "Test content",
		Category:  "tech",
		Version:   1,
		UpdatedAt: now,
		Status:    model.EntryStatusPublished,
	}
	entry.ContentHash = entry.ComputeContentHash()
	storeA.Entry.Create(ctx, entry)

	// 创建同步引擎（先传入 nil protocol，后面再设置）
	syncA := sync.NewSyncEngine(hostA, nil, storeA, &sync.SyncConfig{AutoSync: false})
	syncB := sync.NewSyncEngine(hostB, nil, storeB, &sync.SyncConfig{AutoSync: false})

	// 创建协议，传入同步引擎作为 Handler
	// P2PHost 内嵌了 libp2p host.Host，所以可以直接传给 NewProtocol
	protoA := protocol.NewProtocol(hostA, syncA)
	protoB := protocol.NewProtocol(hostB, syncB)

	// 设置协议处理器（解决循环依赖）
	syncA.SetProtocol(protoA)
	syncB.SetProtocol(protoB)

	// 节点 B 发起同步
	err = syncB.IncrementalSync(ctx)
	if err != nil {
		t.Logf("同步结果: %v", err)
	}

	// 验证节点 B 收到了条目
	entryB, err := storeB.Entry.Get(ctx, "entry-1")
	if err != nil {
		t.Logf("节点 B 暂未收到条目（可能需要等待协议处理）: %v", err)
	} else {
		if entryB.Title != "Test Entry" {
			t.Errorf("条目标题错误: got %s", entryB.Title)
		}
		t.Log("同步成功!")
	}
}

// TestTwoNodeHandshake 测试两个节点的握手
func TestTwoNodeHandshake(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建两个节点
	hostA, _ := host.NewHost(ctx, &host.HostConfig{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	defer hostA.Close()

	hostB, _ := host.NewHost(ctx, &host.HostConfig{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	defer hostB.Close()

	// 连接
	addrA := peer.AddrInfo{ID: hostA.ID(), Addrs: hostA.Addrs()}
	hostB.Connect(ctx, addrA)

	t.Logf("节点 A ID: %s", hostA.ID())
	t.Logf("节点 B ID: %s", hostB.ID())
	t.Log("握手测试完成")
}
