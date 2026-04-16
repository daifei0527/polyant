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
	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestTwoNodeRemoteQuery 测试两个节点的远程查询
func TestTwoNodeRemoteQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建节点 A (服务端，有数据)
	hostA, err := host.NewHost(ctx, &host.HostConfig{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatalf("创建节点 A 失败: %v", err)
	}
	defer hostA.Close()

	// 创建节点 B (客户端，发起查询)
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

	// 在节点 A 创建可搜索的条目
	now := time.Now().UnixMilli()
	entry := &model.KnowledgeEntry{
		ID:        "searchable-entry",
		Title:     "Golang Programming Guide",
		Content:   "A comprehensive guide to Go programming language",
		Category:  "tech/programming/go",
		Version:   1,
		UpdatedAt: now,
		Status:    model.EntryStatusPublished,
		Score:     4.5,
	}
	entry.ContentHash = entry.ComputeContentHash()
	storeA.Entry.Create(ctx, entry)

	// 将条目加入搜索索引
	if err := storeA.Search.IndexEntry(entry); err != nil {
		t.Fatalf("索引条目失败: %v", err)
	}
	t.Logf("节点 A 已创建并索引条目: %s", entry.Title)

	// 创建同步引擎 (先传入 nil protocol，后面再设置)
	syncA := sync.NewSyncEngine(hostA, nil, storeA, &sync.SyncConfig{AutoSync: false})
	syncB := sync.NewSyncEngine(hostB, nil, storeB, &sync.SyncConfig{AutoSync: false})

	// 创建协议，传入同步引擎作为 Handler
	protoA := protocol.NewProtocol(hostA, syncA)
	protoB := protocol.NewProtocol(hostB, syncB)

	// 设置协议处理器 (解决循环依赖)
	syncA.SetProtocol(protoA)
	syncB.SetProtocol(protoB)

	// 创建远程查询服务 (在节点 B 上)
	queryConfig := &sync.RemoteQueryConfig{
		EnableRemoteQuery: true,
		MinLocalResults:   10, // 设置较高的阈值，确保会触发远程查询
		QueryTimeout:      5 * time.Second,
		MaxRemotePeers:    3,
		CacheResults:      false,
	}
	remoteQuery := sync.NewRemoteQueryService(hostB, protoB, storeB, queryConfig)

	t.Log("远程查询服务已创建")

	// 验证连接状态
	peers := hostB.GetConnectedPeers()
	t.Logf("节点 B 已连接的节点数: %d", len(peers))
	for i, p := range peers {
		t.Logf("  节点 %d: %s", i+1, p)
	}

	// 执行远程查询
	query := index.SearchQuery{
		Keyword: "Golang",
		Limit:   10,
	}

	result, err := remoteQuery.SearchWithRemote(ctx, query)
	if err != nil {
		t.Logf("远程查询失败: %v", err)
	} else {
		t.Logf("查询结果: 共 %d 条", result.TotalCount)
		for i, e := range result.Entries {
			t.Logf("  结果 %d: %s (分类: %s, 评分: %.1f)", i+1, e.Title, e.Category, e.Score)
		}

		// 验证结果
		if result.TotalCount > 0 {
			found := false
			for _, e := range result.Entries {
				if e.ID == "searchable-entry" {
					found = true
					if e.Title != "Golang Programming Guide" {
						t.Errorf("条目标题错误: got %s, want Golang Programming Guide", e.Title)
					}
					break
				}
			}
			if found {
				t.Log("远程查询成功找到目标条目!")
			} else {
				t.Log("远程查询未找到目标条目")
			}
		}
	}

	t.Log("远程查询测试完成")
}

// TestRemoteQueryWithNoPeers 测试无连接节点时的远程查询
func TestRemoteQueryWithNoPeers(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建单个节点
	hostA, err := host.NewHost(ctx, &host.HostConfig{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatalf("创建节点失败: %v", err)
	}
	defer hostA.Close()

	// 创建存储
	storeA, _ := storage.NewMemoryStore()

	// 创建同步引擎和协议
	syncA := sync.NewSyncEngine(hostA, nil, storeA, &sync.SyncConfig{AutoSync: false})
	protoA := protocol.NewProtocol(hostA, syncA)
	syncA.SetProtocol(protoA)

	// 创建远程查询服务
	queryConfig := &sync.RemoteQueryConfig{
		EnableRemoteQuery: true,
		MinLocalResults:   0, // 设为 0，总是会尝试远程查询
		QueryTimeout:      1 * time.Second,
		MaxRemotePeers:    3,
		CacheResults:      false,
	}
	remoteQuery := sync.NewRemoteQueryService(hostA, protoA, storeA, queryConfig)

	// 在本地添加一个条目
	entry := &model.KnowledgeEntry{
		ID:       "local-entry",
		Title:    "Local Test",
		Content:  "Local content",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	entry.ContentHash = entry.ComputeContentHash()
	storeA.Entry.Create(ctx, entry)
	storeA.Search.IndexEntry(entry)

	// 查询 (无远程节点)
	query := index.SearchQuery{
		Keyword: "Local",
		Limit:   10,
	}

	result, err := remoteQuery.SearchWithRemote(ctx, query)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}

	// 应该返回本地结果
	if result.TotalCount == 0 {
		t.Error("应该返回本地结果")
	}

	t.Logf("本地查询成功: 共 %d 条结果", result.TotalCount)
}
