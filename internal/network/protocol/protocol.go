package protocol

import (
	"context"
	"fmt"
	"sync"
	"time"

	awsp "github.com/daifei0527/agentwiki/internal/network/protocol/proto"
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
	HandleHeartbeat(ctx context.Context, h *Heartbeat) error
	HandleBitfield(ctx context.Context, b *Bitfield) error
}

type Protocol struct {
	host    host.Host
	handler Handler
	codec   *ProtobufCodec
	mu      sync.RWMutex
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

		response, err := p.processMessage(ctx, protoMsg)
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

func (p *Protocol) processMessage(ctx context.Context, protoMsg *awsp.Message) (*awsp.Message, error) {
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
		go func() {
			for data := range dataCh {
				protoMsg := toProtoMessage(&Message{
					Header:  NewMessageHeader(MessageTypeMirrorData),
					Payload: data,
				})
				s, _ := p.host.NewStream(ctx, peer.ID(r.RequestID), AWSPProtocolID)
				if s != nil {
					writer := NewProtobufStreamWriter(s)
					writer.WriteMessage(protoMsg)
					s.Close()
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
