package host

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
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
func (m *MockP2PHost) NewStream(ctx context.Context, pid peer.ID, pids ...protocol.ID) (network.Stream, error) {
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

// 确保 MockP2PHost 实现 P2PHostInterface 接口
var _ P2PHostInterface = (*MockP2PHost)(nil)
