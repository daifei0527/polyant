package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ProtocolInterface 协议接口，用于依赖注入和测试
type ProtocolInterface interface {
	// SendHandshake 发送握手请求
	SendHandshake(ctx context.Context, peerID peer.ID, h *Handshake) (*HandshakeAck, error)

	// SendSyncRequest 发送同步请求
	SendSyncRequest(ctx context.Context, peerID peer.ID, req *SyncRequest) (*SyncResponse, error)

	// SendQuery 发送查询请求
	SendQuery(ctx context.Context, peerID peer.ID, req *Query) (*QueryResult, error)

	// SendPushEntry 发送条目推送
	SendPushEntry(ctx context.Context, peerID peer.ID, req *PushEntry) (*PushAck, error)

	// SendRatingPush 发送评分推送
	SendRatingPush(ctx context.Context, peerID peer.ID, req *RatingPush) (*RatingAck, error)
}

// 编译时检查 Protocol 是否实现 ProtocolInterface
var _ ProtocolInterface = (*Protocol)(nil)
