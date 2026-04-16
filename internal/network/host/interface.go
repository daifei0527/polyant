package host

import (
	"context"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
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
	NewStream(ctx context.Context, pid peer.ID, pids ...protocol.ID) (network.Stream, error)

	// NodeID 返回节点名称标识
	NodeID() string

	// Close 关闭主机
	Close() error
}

// 确保 P2PHost 实现 P2PHostInterface 接口
var _ P2PHostInterface = (*P2PHost)(nil)
