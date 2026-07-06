package protocol

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	awsp "github.com/daifei0527/polyant/internal/network/protocol/proto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Handler interface {
	HandleHandshake(ctx context.Context, h *Handshake) (*HandshakeAck, error)
	HandleQuery(ctx context.Context, q *Query) (*QueryResult, error)
	HandleSyncRequest(ctx context.Context, r *SyncRequest) (*SyncResponse, error)
	HandleMirrorRequest(ctx context.Context, r *MirrorRequest) (<-chan *MirrorData, error)
	HandlePushEntry(ctx context.Context, e *PushEntry) (*PushAck, error)
	HandleRatingPush(ctx context.Context, r *RatingPush) (*RatingAck, error)
	HandleMirrorData(ctx context.Context, d *MirrorData) error
	HandleHeartbeat(ctx context.Context, h *Heartbeat) error
	HandleBitfield(ctx context.Context, b *Bitfield) error
}

type Protocol struct {
	host    host.Host
	handler Handler
	codec   *ProtobufCodec
	wg      sync.WaitGroup // 跟踪异步 goroutine（如镜像消费者）
}

func NewProtocol(h host.Host, handler Handler) *Protocol {
	p := &Protocol{
		host:    h,
		handler: handler,
		codec:   NewProtobufCodec(),
	}
	h.SetStreamHandler(AWSPProtocolID, p.handleStream)
	return p
}

// Close 等待所有通过 wg.Add 注册的异步 goroutine（如镜像消费者）退出。
// 上层（node/cmd）需在关闭流程中调用此方法以确保 goroutine 不泄漏。
// TODO(cmd/seed, cmd/user): 在 shutdown 流程中调用 proto.Close()。
func (p *Protocol) Close() {
	p.wg.Wait()
}

func (p *Protocol) handleStream(s network.Stream) {
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reader := NewProtobufStreamReader(s)
	writer := NewProtobufStreamWriter(s)

	for {
		protoMsg, err := reader.ReadMessage()
		if err != nil {
			return
		}

		remotePeer := s.Conn().RemotePeer()
		response, err := p.processMessage(ctx, remotePeer, protoMsg)
		if err != nil {
			return
		}

		if response != nil {
			if err := writer.WriteMessage(response); err != nil {
				return
			}
		}
	}
}

// mirrorDialTarget 返回镜像数据流应拨号的目标 peer。
// 恒为请求方的真实 peer（从 stream.Conn().RemotePeer() 传入），绝不是
// MirrorRequest.RequestID（后者仅是同步关联 id，不是 peer id——旧代码用
// peer.ID(r.RequestID) 拨号必败，导致全量镜像同步失效）。
func mirrorDialTarget(requesterPeer peer.ID, _ *MirrorRequest) peer.ID {
	return requesterPeer
}

func (p *Protocol) processMessage(ctx context.Context, remotePeer peer.ID, protoMsg *awsp.Message) (*awsp.Message, error) {
	// Convert proto message to domain message for handler
	msg := fromProtoMessage(protoMsg)

	switch msg.Header.Type {
	case MessageTypeHandshake:
		h := msg.Payload.(*Handshake)
		ack, err := p.handler.HandleHandshake(ctx, h)
		if err != nil {
			return nil, err
		}
		return toProtoMessage(&Message{
			Header:  NewMessageHeader(MessageTypeHandshakeAck),
			Payload: ack,
		}), nil

	case MessageTypeQuery:
		q := msg.Payload.(*Query)
		result, err := p.handler.HandleQuery(ctx, q)
		if err != nil {
			return nil, err
		}
		return toProtoMessage(&Message{
			Header:  NewMessageHeader(MessageTypeQueryResult),
			Payload: result,
		}), nil

	case MessageTypeSyncRequest:
		r := msg.Payload.(*SyncRequest)
		resp, err := p.handler.HandleSyncRequest(ctx, r)
		if err != nil {
			return nil, err
		}
		return toProtoMessage(&Message{
			Header:  NewMessageHeader(MessageTypeSyncResponse),
			Payload: resp,
		}), nil

	case MessageTypeMirrorRequest:
		r := msg.Payload.(*MirrorRequest)
		dataCh, err := p.handler.HandleMirrorRequest(ctx, r)
		if err != nil {
			return nil, err
		}
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Protocol] mirror consumer panic: %v", r)
				}
			}()
			for {
				select {
				case data, ok := <-dataCh:
					if !ok {
						return
					}
					protoMsg := toProtoMessage(&Message{
						Header:  NewMessageHeader(MessageTypeMirrorData),
						Payload: data,
					})
					// NewStream 接受 ctx，当 ctx 取消时返回错误，不需要额外超时控制；
					// WriteMessage 依赖 stream 的传输层超时，属 best-effort 写入。
					// 两者均依赖 libp2p ctx 中断语义，goroutine 通过 ctx.Done() 退出。
					s, err := p.host.NewStream(ctx, mirrorDialTarget(remotePeer, r), AWSPProtocolID)
					if err != nil {
						log.Printf("[Protocol] mirror NewStream failed: %v", err)
						continue
					}
					writer := NewProtobufStreamWriter(s)
					if err := writer.WriteMessage(protoMsg); err != nil {
						log.Printf("[Protocol] mirror write failed: %v", err)
					}
					s.Close()
				case <-ctx.Done():
					return
				}
			}
		}()
		return nil, nil

	case MessageTypePushEntry:
		e := msg.Payload.(*PushEntry)
		ack, err := p.handler.HandlePushEntry(ctx, e)
		if err != nil {
			return nil, err
		}
		return toProtoMessage(&Message{
			Header:  NewMessageHeader(MessageTypePushAck),
			Payload: ack,
		}), nil

	case MessageTypeRatingPush:
		r := msg.Payload.(*RatingPush)
		ack, err := p.handler.HandleRatingPush(ctx, r)
		if err != nil {
			return nil, err
		}
		return toProtoMessage(&Message{
			Header:  NewMessageHeader(MessageTypeRatingAck),
			Payload: ack,
		}), nil

	case MessageTypeHeartbeat:
		h := msg.Payload.(*Heartbeat)
		return nil, p.handler.HandleHeartbeat(ctx, h)

	case MessageTypeBitfield:
		b := msg.Payload.(*Bitfield)
		return nil, p.handler.HandleBitfield(ctx, b)

	case MessageTypeMirrorData:
		d, ok := msg.Payload.(*MirrorData)
		if !ok {
			return nil, fmt.Errorf("invalid MirrorData payload")
		}
		if err := p.handler.HandleMirrorData(ctx, d); err != nil {
			return nil, fmt.Errorf("handle mirror data: %w", err)
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown message type: %v", msg.Header.Type)
	}
}

func (p *Protocol) SendHandshake(ctx context.Context, peerID peer.ID, h *Handshake) (*HandshakeAck, error) {
	s, err := p.host.NewStream(ctx, peerID, AWSPProtocolID)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}
	defer s.Close()

	writer := NewProtobufStreamWriter(s)
	reader := NewProtobufStreamReader(s)

	protoMsg := toProtoMessage(&Message{
		Header:  NewMessageHeader(MessageTypeHandshake),
		Payload: h,
	})
	if err := writer.WriteMessage(protoMsg); err != nil {
		return nil, err
	}
	if err := s.CloseWrite(); err != nil {
		return nil, err
	}

	resp, err := reader.ReadMessage()
	if err != nil {
		return nil, err
	}

	domainResp := fromProtoMessage(resp)
	return domainResp.Payload.(*HandshakeAck), nil
}

func (p *Protocol) SendQuery(ctx context.Context, peerID peer.ID, q *Query) (*QueryResult, error) {
	s, err := p.host.NewStream(ctx, peerID, AWSPProtocolID)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}
	defer s.Close()

	writer := NewProtobufStreamWriter(s)
	reader := NewProtobufStreamReader(s)

	protoMsg := toProtoMessage(&Message{
		Header:  NewMessageHeader(MessageTypeQuery),
		Payload: q,
	})
	if err := writer.WriteMessage(protoMsg); err != nil {
		return nil, err
	}
	if err := s.CloseWrite(); err != nil {
		return nil, err
	}

	resp, err := reader.ReadMessage()
	if err != nil {
		return nil, err
	}

	domainResp := fromProtoMessage(resp)
	return domainResp.Payload.(*QueryResult), nil
}

func (p *Protocol) SendSyncRequest(ctx context.Context, peerID peer.ID, r *SyncRequest) (*SyncResponse, error) {
	s, err := p.host.NewStream(ctx, peerID, AWSPProtocolID)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}
	defer s.Close()

	writer := NewProtobufStreamWriter(s)
	reader := NewProtobufStreamReader(s)

	protoMsg := toProtoMessage(&Message{
		Header:  NewMessageHeader(MessageTypeSyncRequest),
		Payload: r,
	})
	if err := writer.WriteMessage(protoMsg); err != nil {
		return nil, err
	}
	if err := s.CloseWrite(); err != nil {
		return nil, err
	}

	resp, err := reader.ReadMessage()
	if err != nil {
		return nil, err
	}

	domainResp := fromProtoMessage(resp)
	return domainResp.Payload.(*SyncResponse), nil
}

func (p *Protocol) SendPushEntry(ctx context.Context, peerID peer.ID, e *PushEntry) (*PushAck, error) {
	s, err := p.host.NewStream(ctx, peerID, AWSPProtocolID)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}
	defer s.Close()

	writer := NewProtobufStreamWriter(s)
	reader := NewProtobufStreamReader(s)

	protoMsg := toProtoMessage(&Message{
		Header:  NewMessageHeader(MessageTypePushEntry),
		Payload: e,
	})
	if err := writer.WriteMessage(protoMsg); err != nil {
		return nil, err
	}
	if err := s.CloseWrite(); err != nil {
		return nil, err
	}

	resp, err := reader.ReadMessage()
	if err != nil {
		return nil, err
	}

	domainResp := fromProtoMessage(resp)
	return domainResp.Payload.(*PushAck), nil
}

func (p *Protocol) SendRatingPush(ctx context.Context, peerID peer.ID, r *RatingPush) (*RatingAck, error) {
	s, err := p.host.NewStream(ctx, peerID, AWSPProtocolID)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}
	defer s.Close()

	writer := NewProtobufStreamWriter(s)
	reader := NewProtobufStreamReader(s)

	protoMsg := toProtoMessage(&Message{
		Header:  NewMessageHeader(MessageTypeRatingPush),
		Payload: r,
	})
	if err := writer.WriteMessage(protoMsg); err != nil {
		return nil, err
	}
	if err := s.CloseWrite(); err != nil {
		return nil, err
	}

	resp, err := reader.ReadMessage()
	if err != nil {
		return nil, err
	}

	domainResp := fromProtoMessage(resp)
	return domainResp.Payload.(*RatingAck), nil
}

// SendMirrorRequest 向 peer 发送镜像请求。镜像数据通过 HandleMirrorData 异步回流。
// 注意：不关闭 stream。服务端 handleStream 的 ctx 与此 stream 的 ReadMessage 循环绑定；
// 关闭 stream 会导致 ReadMessage 返回 EOF → ctx 取消 → 镜像消费者 goroutine 的
// NewStream(ctx) 也因 ctx 已取消而失败。保持 stream 打开直到 ctx 超时后由 libp2p 回收。
func (p *Protocol) SendMirrorRequest(ctx context.Context, target peer.ID, req *MirrorRequest) error {
	s, err := p.host.NewStream(ctx, target, AWSPProtocolID)
	if err != nil {
		return fmt.Errorf("new stream: %w", err)
	}
	writer := NewProtobufStreamWriter(s)
	msg := &Message{Header: NewMessageHeader(MessageTypeMirrorRequest), Payload: req}
	return writer.WriteMessage(toProtoMessage(msg))
}
