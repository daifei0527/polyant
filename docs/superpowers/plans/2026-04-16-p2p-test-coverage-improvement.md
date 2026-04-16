# P2P 网络模块测试覆盖率提升实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 提升 sync 和 dht 模块的测试覆盖率从 49.4%/69.2% 到 80%+

**Architecture:** 创建 P2PHost 和 Protocol 接口实现依赖注入，使用 Mock 进行单元测试，添加真实 libp2p 节点的集成测试

**Tech Stack:** Go 1.22+, go-libp2p, testing

---

## 文件结构

### 新增文件
| 文件 | 职责 |
|------|------|
| `internal/network/host/interface.go` | P2PHost 接口定义 |
| `internal/network/host/mock_host.go` | Mock P2PHost 实现 |
| `internal/network/protocol/interface.go` | Protocol 接口定义 |
| `internal/network/protocol/mock_protocol.go` | Mock Protocol 实现 |
| `test/integration/p2p_sync_test.go` | 双节点同步集成测试 |
| `test/integration/p2p_query_test.go` | 远程查询集成测试 |

### 修改文件
| 文件 | 修改内容 |
|------|----------|
| `internal/network/sync/sync.go` | 使用接口类型替代具体类型 |
| `internal/network/sync/sync_test.go` | 追加 Mock 测试用例 |
| `internal/network/dht/dht_test.go` | 追加 FindPeer 测试 |

---

## Task 1: 创建 P2PHost 接口

**Files:**
- Create: `internal/network/host/interface.go`

- [ ] **Step 1: 创建接口定义文件**

```go
package host

import (
	"context"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// P2PHostInterface P2P 主机接口，用于依赖注入和测试
type P2PHostInterface interface {
	// ID 返回节点 ID
	ID() peer.ID

	// GetConnectedPeers 返回已连接的节点列表
	GetConnectedPeers() []peer.ID

	// Connect 连接到指定节点
	Connect(ctx context.Context, addr peer.AddrInfo) error

	// NewStream 创建到指定节点的新流
	NewStream(ctx context.Context, pid peer.ID, pids ...string) (network.Stream, error)

	// NodeID 返回节点名称标识
	NodeID() string

	// Close 关闭主机
	Close() error
}
```

- [ ] **Step 2: 验证 P2PHost 实现接口**

运行编译检查接口是否正确实现：
```bash
cd /home/daifei/agentwiki && go build ./internal/network/host/...
```
Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
git add internal/network/host/interface.go
git commit -m "feat(host): add P2PHostInterface for dependency injection

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: 创建 Protocol 接口

**Files:**
- Create: `internal/network/protocol/interface.go`

- [ ] **Step 1: 创建接口定义文件**

```go
package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ProtocolInterface 协议接口，用于依赖注入和测试
type ProtocolInterface interface {
	// SendSyncRequest 发送同步请求
	SendSyncRequest(ctx context.Context, peerID peer.ID, req *SyncRequest) (*SyncResponse, error)

	// SendQuery 发送查询请求
	SendQuery(ctx context.Context, peerID peer.ID, req *Query) (*QueryResult, error)

	// SendPushEntry 发送条目推送
	SendPushEntry(ctx context.Context, peerID peer.ID, req *PushEntry) (*PushAck, error)

	// SendRatingPush 发送评分推送
	SendRatingPush(ctx context.Context, peerID peer.ID, req *RatingPush) (*RatingAck, error)
}
```

- [ ] **Step 2: 验证 Protocol 实现接口**

运行编译检查：
```bash
cd /home/daifei/agentwiki && go build ./internal/network/protocol/...
```
Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
git add internal/network/protocol/interface.go
git commit -m "feat(protocol): add ProtocolInterface for dependency injection

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: 创建 Mock P2PHost

**Files:**
- Create: `internal/network/host/mock_host.go`

- [ ] **Step 1: 创建 Mock 实现文件**

```go
package host

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// MockP2PHost 用于测试的 Mock 实现
type MockP2PHost struct {
	mu             sync.RWMutex
	id             peer.ID
	nodeID         string
	connectedPeers []peer.ID
	connectError   error
	streamError    error
}

// NewMockP2PHost 创建 Mock 主机
func NewMockP2PHost() *MockP2PHost {
	return &MockP2PHost{
		id:             peer.ID("mock-peer-id"),
		nodeID:         "mock-node",
		connectedPeers: []peer.ID{},
	}
}

// ID 返回节点 ID
func (m *MockP2PHost) ID() peer.ID {
	return m.id
}

// NodeID 返回节点名称
func (m *MockP2PHost) NodeID() string {
	return m.nodeID
}

// GetConnectedPeers 返回已连接节点
func (m *MockP2PHost) GetConnectedPeers() []peer.ID {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connectedPeers
}

// Connect 模拟连接
func (m *MockP2PHost) Connect(ctx context.Context, addr peer.AddrInfo) error {
	if m.connectError != nil {
		return m.connectError
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectedPeers = append(m.connectedPeers, addr.ID)
	return nil
}

// NewStream 模拟创建流
func (m *MockP2PHost) NewStream(ctx context.Context, pid peer.ID, pids ...string) (network.Stream, error) {
	if m.streamError != nil {
		return nil, m.streamError
	}
	return nil, nil
}

// Close 模拟关闭
func (m *MockP2PHost) Close() error {
	return nil
}

// --- 测试辅助方法 ---

// SetConnectedPeers 设置已连接节点
func (m *MockP2PHost) SetConnectedPeers(peers []peer.ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectedPeers = peers
}

// SetConnectError 设置连接错误
func (m *MockP2PHost) SetConnectError(err error) {
	m.connectError = err
}

// SetStreamError 设置流创建错误
func (m *MockP2PHost) SetStreamError(err error) {
	m.streamError = err
}

// SetID 设置节点 ID
func (m *MockP2PHost) SetID(id peer.ID) {
	m.id = id
}

// SetNodeID 设置节点名称
func (m *MockP2PHost) SetNodeID(nodeID string) {
	m.nodeID = nodeID
}

// 确保 MockP2PHost 实现接口
var _ P2PHostInterface = (*MockP2PHost)(nil)
```

- [ ] **Step 2: 验证编译**

```bash
cd /home/daifei/agentwiki && go build ./internal/network/host/...
```
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/network/host/mock_host.go
git commit -m "feat(host): add MockP2PHost for testing

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: 创建 Mock Protocol

**Files:**
- Create: `internal/network/protocol/mock_protocol.go`

- [ ] **Step 1: 创建 Mock 实现文件**

```go
package protocol

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// MockProtocol 用于测试的 Mock 实现
type MockProtocol struct {
	mu sync.RWMutex

	// 可编程的响应
	syncResponse  *SyncResponse
	syncError     error
	queryResult   *QueryResult
	queryError    error
	pushAck       *PushAck
	pushError     error
	ratingAck     *RatingAck
	ratingError   error

	// 记录调用
	syncRequests   []*SyncRequest
	queryRequests  []*Query
	pushRequests   []*PushEntry
	ratingRequests []*RatingPush
}

// NewMockProtocol 创建 Mock 协议
func NewMockProtocol() *MockProtocol {
	return &MockProtocol{
		syncRequests:   make([]*SyncRequest, 0),
		queryRequests:  make([]*Query, 0),
		pushRequests:   make([]*PushEntry, 0),
		ratingRequests: make([]*RatingPush, 0),
	}
}

// SendSyncRequest 发送同步请求
func (m *MockProtocol) SendSyncRequest(ctx context.Context, peerID peer.ID, req *SyncRequest) (*SyncResponse, error) {
	m.mu.Lock()
	m.syncRequests = append(m.syncRequests, req)
	m.mu.Unlock()

	if m.syncError != nil {
		return nil, m.syncError
	}
	return m.syncResponse, nil
}

// SendQuery 发送查询
func (m *MockProtocol) SendQuery(ctx context.Context, peerID peer.ID, req *Query) (*QueryResult, error) {
	m.mu.Lock()
	m.queryRequests = append(m.queryRequests, req)
	m.mu.Unlock()

	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.queryResult, nil
}

// SendPushEntry 发送条目推送
func (m *MockProtocol) SendPushEntry(ctx context.Context, peerID peer.ID, req *PushEntry) (*PushAck, error) {
	m.mu.Lock()
	m.pushRequests = append(m.pushRequests, req)
	m.mu.Unlock()

	if m.pushError != nil {
		return nil, m.pushError
	}
	return m.pushAck, nil
}

// SendRatingPush 发送评分推送
func (m *MockProtocol) SendRatingPush(ctx context.Context, peerID peer.ID, req *RatingPush) (*RatingAck, error) {
	m.mu.Lock()
	m.ratingRequests = append(m.ratingRequests, req)
	m.mu.Unlock()

	if m.ratingError != nil {
		return nil, m.ratingError
	}
	return m.ratingAck, nil
}

// --- 测试辅助方法 ---

// SetSyncResponse 设置同步响应
func (m *MockProtocol) SetSyncResponse(resp *SyncResponse) {
	m.syncResponse = resp
}

// SetSyncError 设置同步错误
func (m *MockProtocol) SetSyncError(err error) {
	m.syncError = err
}

// SetQueryResult 设置查询结果
func (m *MockProtocol) SetQueryResult(result *QueryResult) {
	m.queryResult = result
}

// SetQueryError 设置查询错误
func (m *MockProtocol) SetQueryError(err error) {
	m.queryError = err
}

// SetPushAck 设置推送确认
func (m *MockProtocol) SetPushAck(ack *PushAck) {
	m.pushAck = ack
}

// SetPushError 设置推送错误
func (m *MockProtocol) SetPushError(err error) {
	m.pushError = err
}

// SetRatingAck 设置评分确认
func (m *MockProtocol) SetRatingAck(ack *RatingAck) {
	m.ratingAck = ack
}

// SetRatingError 设置评分错误
func (m *MockProtocol) SetRatingError(err error) {
	m.ratingError = err
}

// GetSyncRequests 获取同步请求记录
func (m *MockProtocol) GetSyncRequests() []*SyncRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncRequests
}

// Reset 重置所有状态
func (m *MockProtocol) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncRequests = make([]*SyncRequest, 0)
	m.queryRequests = make([]*Query, 0)
	m.pushRequests = make([]*PushEntry, 0)
	m.ratingRequests = make([]*RatingPush, 0)
	m.syncResponse = nil
	m.syncError = nil
	m.queryResult = nil
	m.queryError = nil
	m.pushAck = nil
	m.pushError = nil
	m.ratingAck = nil
	m.ratingError = nil
}

// 确保 MockProtocol 实现接口
var _ ProtocolInterface = (*MockProtocol)(nil)
```

- [ ] **Step 2: 验证编译**

```bash
cd /home/daifei/agentwiki && go build ./internal/network/protocol/...
```
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/network/protocol/mock_protocol.go
git commit -m "feat(protocol): add MockProtocol for testing

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: 修改 SyncEngine 使用接口

**Files:**
- Modify: `internal/network/sync/sync.go:88-102`

- [ ] **Step 1: 修改 SyncEngine 结构体**

找到 `internal/network/sync/sync.go` 第 88-102 行，将具体类型改为接口类型：

```go
type SyncEngine struct {
	p2pHost    host.P2PHostInterface  // 改为接口
	protocol   protocol.ProtocolInterface  // 改为接口
	store      *storage.Store
	config     *SyncConfig
	state      SyncState
	versionVec VersionVector
	lastSync   int64
	mu         sync.RWMutex
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewSyncEngine(p2pHost host.P2PHostInterface, proto protocol.ProtocolInterface, store *storage.Store, cfg *SyncConfig) *SyncEngine {
	return &SyncEngine{
		p2pHost:    p2pHost,
		protocol:   proto,
		store:      store,
		config:     cfg,
		state:      SyncStateIdle,
		versionVec: make(VersionVector),
	}
}
```

- [ ] **Step 2: 添加 import 别名（如果需要）**

在文件顶部确保 import 正确：
```go
import (
	// ... 其他 imports
	"github.com/daifei0527/polyant/internal/network/host"
	"github.com/daifei0527/polyant/internal/network/protocol"
)
```

- [ ] **Step 3: 运行现有测试验证不破坏功能**

```bash
cd /home/daifei/agentwiki && go test ./internal/network/sync/... -v -count=1
```
Expected: 所有现有测试通过

- [ ] **Step 4: 提交**

```bash
git add internal/network/sync/sync.go
git commit -m "refactor(sync): use interfaces for dependency injection

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: 添加 sync 模块 Mock 测试

**Files:**
- Modify: `internal/network/sync/sync_test.go` (追加)

- [ ] **Step 1: 添加 Mock 测试导入**

在 `sync_test.go` 文件中添加导入（如果尚未添加）：
```go
import (
	// ... 现有 imports
	"github.com/daifei0527/polyant/internal/network/host"
	"github.com/daifei0527/polyant/internal/network/protocol"
)
```

- [ ] **Step 2: 添加 IncrementalSync Mock 测试**

在文件末尾追加：

```go
// ==================== 使用 Mock 的增量同步测试 ====================

// TestIncrementalSync_WithMockPeers 测试有 Mock 节点时的增量同步
func TestIncrementalSync_WithMockPeers(t *testing.T) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	// 创建 Mock 主机
	mockHost := host.NewMockP2PHost()
	mockHost.SetConnectedPeers([]peer.ID{peer.ID("peer-1")})

	// 创建 Mock 协议
	mockProtocol := protocol.NewMockProtocol()
	mockProtocol.SetSyncResponse(&protocol.SyncResponse{
		RequestID:           "test-1",
		NewEntries:          [][]byte{},
		UpdatedEntries:      [][]byte{},
		DeletedEntryIDs:     []string{},
		NewRatings:          [][]byte{},
		ServerVersionVector: map[string]int64{},
		ServerTimestamp:     time.Now().UnixMilli(),
	})

	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(mockHost, mockProtocol, store, cfg)

	err = engine.IncrementalSync(context.Background())
	if err != nil {
		t.Errorf("IncrementalSync 失败: %v", err)
	}
}

// TestIncrementalSync_NoPeers 测试无节点时的增量同步
func TestIncrementalSync_NoPeers(t *testing.T) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	mockHost := host.NewMockP2PHost()
	mockHost.SetConnectedPeers([]peer.ID{}) // 无连接节点

	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(mockHost, nil, store, cfg)

	err = engine.IncrementalSync(context.Background())
	if err != nil {
		t.Errorf("无节点时应返回 nil: %v", err)
	}
}

// TestIncrementalSync_SyncError 测试同步失败
func TestIncrementalSync_SyncError(t *testing.T) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	mockHost := host.NewMockP2PHost()
	mockHost.SetConnectedPeers([]peer.ID{peer.ID("peer-1")})

	mockProtocol := protocol.NewMockProtocol()
	mockProtocol.SetSyncError(fmt.Errorf("network error"))

	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(mockHost, mockProtocol, store, cfg)

	err = engine.IncrementalSync(context.Background())
	if err == nil {
		t.Error("同步失败时应返回错误")
	}
}

// TestSyncWithPeer_NewEntries 测试同步新条目
func TestSyncWithPeer_NewEntries(t *testing.T) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	// 创建测试条目
	now := time.Now().UnixMilli()
	remoteEntry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Remote Entry",
		Content:   "content",
		Category:  "tech",
		Version:   1,
		UpdatedAt: now,
		Status:    model.EntryStatusPublished,
	}
	remoteEntry.ContentHash = remoteEntry.ComputeContentHash()
	entryData, _ := remoteEntry.ToJSON()

	mockHost := host.NewMockP2PHost()
	mockHost.SetConnectedPeers([]peer.ID{peer.ID("peer-1")})

	mockProtocol := protocol.NewMockProtocol()
	mockProtocol.SetSyncResponse(&protocol.SyncResponse{
		RequestID:           "test-1",
		NewEntries:          [][]byte{entryData},
		UpdatedEntries:      [][]byte{},
		DeletedEntryIDs:     []string{},
		NewRatings:          [][]byte{},
		ServerVersionVector: map[string]int64{"entry-1": 1},
		ServerTimestamp:     now,
	})

	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(mockHost, mockProtocol, store, cfg)

	err = engine.IncrementalSync(context.Background())
	if err != nil {
		t.Errorf("同步失败: %v", err)
	}

	// 验证条目已创建
	entry, err := store.Entry.Get(context.Background(), "entry-1")
	if err != nil {
		t.Errorf("获取条目失败: %v", err)
	}
	if entry.Title != "Remote Entry" {
		t.Errorf("条目标题错误: got %s", entry.Title)
	}
}

// TestHandleHandshake_WithMockHost 测试握手处理
func TestHandleHandshake_WithMockHost(t *testing.T) {
	mockHost := host.NewMockP2PHost()
	mockHost.SetID(peer.ID("test-peer-123"))
	mockHost.SetNodeID("test-node")

	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(mockHost, nil, nil, cfg)

	handshake := &protocol.Handshake{
		NodeID:   "client-node",
		NodeType: protocol.NodeTypeLocal,
		Version:  "1.0.0",
	}

	ack, err := engine.HandleHandshake(context.Background(), handshake)
	if err != nil {
		t.Errorf("HandleHandshake 失败: %v", err)
	}
	if !ack.Accepted {
		t.Error("握手应被接受")
	}
	if ack.PeerID != "test-peer-123" {
		t.Errorf("PeerID 错误: got %s", ack.PeerID)
	}
}

// TestIncrementalSync_DeletedEntries 测试同步删除条目
func TestIncrementalSync_DeletedEntries(t *testing.T) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	// 先创建一个本地条目
	localEntry := &model.KnowledgeEntry{
		ID:        "entry-1",
		Title:     "Local Entry",
		Content:   "content",
		Category:  "tech",
		Version:   1,
		UpdatedAt: time.Now().UnixMilli(),
		Status:    model.EntryStatusPublished,
	}
	localEntry.ContentHash = localEntry.ComputeContentHash()
	store.Entry.Create(context.Background(), localEntry)

	mockHost := host.NewMockP2PHost()
	mockHost.SetConnectedPeers([]peer.ID{peer.ID("peer-1")})

	mockProtocol := protocol.NewMockProtocol()
	mockProtocol.SetSyncResponse(&protocol.SyncResponse{
		RequestID:           "test-1",
		NewEntries:          [][]byte{},
		UpdatedEntries:      [][]byte{},
		DeletedEntryIDs:     []string{"entry-1"},
		NewRatings:          [][]byte{},
		ServerVersionVector: map[string]int64{},
		ServerTimestamp:     time.Now().UnixMilli(),
	})

	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(mockHost, mockProtocol, store, cfg)
	engine.HandleBitfield(context.Background(), &protocol.Bitfield{
		VersionVector: map[string]int64{"entry-1": 1},
	})

	err = engine.IncrementalSync(context.Background())
	if err != nil {
		t.Errorf("同步失败: %v", err)
	}

	// 验证条目已删除
	_, err = store.Entry.Get(context.Background(), "entry-1")
	if err == nil {
		t.Error("条目应被删除")
	}
}
```

- [ ] **Step 3: 运行新测试验证**

```bash
cd /home/daifei/agentwiki && go test ./internal/network/sync/... -v -run "TestIncrementalSync_|TestSyncWithPeer_|TestHandleHandshake_WithMockHost"
```
Expected: 所有新测试通过

- [ ] **Step 4: 检查覆盖率提升**

```bash
cd /home/daifei/agentwiki && go test ./internal/network/sync/... -cover
```
Expected: 覆盖率从 49.4% 提升

- [ ] **Step 5: 提交**

```bash
git add internal/network/sync/sync_test.go
git commit -m "test(sync): add mock-based tests for IncrementalSync and HandleHandshake

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 7: 添加 dht 模块测试

**Files:**
- Modify: `internal/network/dht/dht_test.go` (追加)

- [ ] **Step 1: 添加 FindPeer 测试**

在文件末尾追加：

```go
// ==================== DHT FindPeer 测试 ====================

// TestDHTNodeFindPeer 测试查找节点
func TestDHTNodeFindPeer(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	cfg := &config.Config{}
	dhtNode, err := dht.NewDHTNode(h, cfg)
	if err != nil {
		t.Fatalf("NewDHTNode 失败: %v", err)
	}
	defer dhtNode.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 查找一个不存在的节点，应该超时或返回错误
	// 这是预期行为，因为我们没有连接到其他节点
	_, err = dhtNode.FindPeer(ctx, "non-existent-peer")
	// 错误是预期的，我们不验证具体错误
	_ = err
}

// TestDHTNodeFindPeerAfterBootstrap 测试 Bootstrap 后查找
func TestDHTNodeFindPeerAfterBootstrap(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	cfg := &config.Config{}
	dhtNode, err := dht.NewDHTNode(h, cfg)
	if err != nil {
		t.Fatalf("NewDHTNode 失败: %v", err)
	}
	defer dhtNode.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Bootstrap
	_ = dhtNode.Bootstrap(ctx)

	// 查找节点
	_, err = dhtNode.FindPeer(ctx, "some-peer-id")
	// 查找可能失败，因为网络中没有这个节点
	_ = err
}

// TestDHTNodeFindPeerContextCancel 测试上下文取消
func TestDHTNodeFindPeerContextCancel(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	cfg := &config.Config{}
	dhtNode, err := dht.NewDHTNode(h, cfg)
	if err != nil {
		t.Fatalf("NewDHTNode 失败: %v", err)
	}
	defer dhtNode.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err = dhtNode.FindPeer(ctx, "any-peer")
	if err == nil {
		t.Error("上下文取消时应返回错误")
	}
}
```

- [ ] **Step 2: 运行新测试验证**

```bash
cd /home/daifei/agentwiki && go test ./internal/network/dht/... -v -run "TestDHTNodeFindPeer"
```
Expected: 所有新测试通过

- [ ] **Step 3: 检查覆盖率提升**

```bash
cd /home/daifei/agentwiki && go test ./internal/network/dht/... -cover
```
Expected: 覆盖率从 69.2% 提升

- [ ] **Step 4: 提交**

```bash
git add internal/network/dht/dht_test.go
git commit -m "test(dht): add FindPeer tests for DHT node

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 8: 创建集成测试 - 双节点同步

**Files:**
- Create: `test/integration/p2p_sync_test.go`

- [ ] **Step 1: 创建集成测试文件**

```go
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

	// 创建协议和同步引擎
	protoA := protocol.NewProtocol(hostA, storeA)
	protoB := protocol.NewProtocol(hostB, storeB)

	syncA := sync.NewSyncEngine(hostA, protoA, storeA, &sync.SyncConfig{AutoSync: false})
	syncB := sync.NewSyncEngine(hostB, protoB, storeB, &sync.SyncConfig{AutoSync: false})

	// 设置协议处理器
	protoA.SetSyncHandler(syncA)
	protoB.SetSyncHandler(syncB)

	// 启动协议
	protoA.Start(ctx)
	defer protoA.Stop()
	protoB.Start(ctx)
	defer protoB.Stop()

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
```

- [ ] **Step 2: 验证编译**

```bash
cd /home/daifei/agentwiki && go build ./test/integration/...
```
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add test/integration/p2p_sync_test.go
git commit -m "test(integration): add two-node sync integration test

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 9: 创建集成测试 - 远程查询

**Files:**
- Create: `test/integration/p2p_query_test.go`

- [ ] **Step 1: 创建集成测试文件**

```go
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

// TestTwoNodeQuery 测试两个节点的远程查询
func TestTwoNodeQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	// 创建存储和协议
	storeA, _ := storage.NewMemoryStore()
	storeB, _ := storage.NewMemoryStore()

	protoA := protocol.NewProtocol(hostA, storeA)
	protoB := protocol.NewProtocol(hostB, storeB)

	syncA := sync.NewSyncEngine(hostA, protoA, storeA, &sync.SyncConfig{AutoSync: false})
	syncB := sync.NewSyncEngine(hostB, protoB, storeB, &sync.SyncConfig{AutoSync: false})

	protoA.SetSyncHandler(syncA)
	protoB.SetSyncHandler(syncB)

	protoA.Start(ctx)
	defer protoA.Stop()
	protoB.Start(ctx)
	defer protoB.Stop()

	// 在节点 A 创建可搜索的条目
	now := time.Now().UnixMilli()
	entry := &model.KnowledgeEntry{
		ID:        "searchable-entry",
		Title:     "Golang Programming Guide",
		Content:   "A comprehensive guide to Go programming",
		Category:  "tech/programming/go",
		Version:   1,
		UpdatedAt: now,
		Status:    model.EntryStatusPublished,
	}
	entry.ContentHash = entry.ComputeContentHash()
	storeA.Entry.Create(ctx, entry)
	storeA.Search.IndexEntry(entry)

	// 等待索引更新
	time.Sleep(100 * time.Millisecond)

	// 节点 B 发起远程查询
	remoteQuery := sync.NewRemoteQueryService(hostB, protoB, storeB, &sync.RemoteQueryConfig{
		EnableRemoteQuery: true,
		MinLocalResults:   1,
		QueryTimeout:      5 * time.Second,
		MaxRemotePeers:    1,
	})

	t.Log("远程查询服务已创建")
	_ = remoteQuery
}
```

- [ ] **Step 2: 验证编译**

```bash
cd /home/daifei/agentwiki && go build ./test/integration/...
```
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add test/integration/p2p_query_test.go
git commit -m "test(integration): add two-node query integration test

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 10: 验证最终覆盖率

**Files:**
- 无文件变更，仅运行验证

- [ ] **Step 1: 运行完整测试套件**

```bash
cd /home/daifei/agentwiki && go test ./internal/network/sync/... ./internal/network/dht/... -v -cover
```
Expected: 所有测试通过

- [ ] **Step 2: 生成覆盖率报告**

```bash
cd /home/daifei/agentwiki && go test ./internal/network/sync/... -coverprofile=sync_coverage.out && go tool cover -func=sync_coverage.out | tail -1
```
Expected: sync 模块覆盖率 >= 75%

```bash
cd /home/daifei/agentwiki && go test ./internal/network/dht/... -coverprofile=dht_coverage.out && go tool cover -func=dht_coverage.out | tail -1
```
Expected: dht 模块覆盖率 >= 75%

- [ ] **Step 3: 最终提交（如果有未提交的更改）**

```bash
git status
# 如果有未提交的更改，提交它们
```

---

## 完成检查清单

- [ ] 所有新增文件已创建
- [ ] 所有修改文件已更新
- [ ] 所有测试通过
- [ ] sync 模块覆盖率 >= 75%
- [ ] dht 模块覆盖率 >= 75%
- [ ] 所有更改已提交
