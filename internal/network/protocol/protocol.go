package protocol

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	codec   *Codec
	mu      sync.RWMutex
}

func NewProtocol(h host.Host, handler Handler) *Protocol {
	p := &Protocol{
		host:    h,
		handler: handler,
		codec:   NewCodec(),
	}
	h.SetStreamHandler(AWSPProtocolID, p.handleStream)
	return p
}

func (p *Protocol) handleStream(s network.Stream) {
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reader := NewStreamReader(s)
	writer := NewStreamWriter(s)

	for {
		msg, err := reader.ReadMessage()
		if err != nil {
			return
		}

		response, err := p.processMessage(ctx, msg)
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

func (p *Protocol) processMessage(ctx context.Context, msg *Message) (*Message, error) {
	switch msg.Header.Type {
	case MessageTypeHandshake:
		h := msg.Payload.(*Handshake)
		ack, err := p.handler.HandleHandshake(ctx, h)
		if err != nil {
			return nil, err
		}
		return &Message{
			Header:  NewMessageHeader(MessageTypeHandshakeAck),
			Payload: ack,
		}, nil

	case MessageTypeQuery:
		q := msg.Payload.(*Query)
		result, err := p.handler.HandleQuery(ctx, q)
		if err != nil {
			return nil, err
		}
		return &Message{
			Header:  NewMessageHeader(MessageTypeQueryResult),
			Payload: result,
		}, nil

	case MessageTypeSyncRequest:
		r := msg.Payload.(*SyncRequest)
		resp, err := p.handler.HandleSyncRequest(ctx, r)
		if err != nil {
			return nil, err
		}
		return &Message{
			Header:  NewMessageHeader(MessageTypeSyncResponse),
			Payload: resp,
		}, nil

	case MessageTypeMirrorRequest:
		r := msg.Payload.(*MirrorRequest)
		dataCh, err := p.handler.HandleMirrorRequest(ctx, r)
		if err != nil {
			return nil, err
		}
		go func() {
			for data := range dataCh {
				msg := &Message{
					Header:  NewMessageHeader(MessageTypeMirrorData),
					Payload: data,
				}
				s, _ := p.host.NewStream(ctx, peer.ID(r.RequestID), AWSPProtocolID)
				if s != nil {
					writer := NewStreamWriter(s)
					writer.WriteMessage(msg)
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
		return &Message{
			Header:  NewMessageHeader(MessageTypePushAck),
			Payload: ack,
		}, nil

	case MessageTypeRatingPush:
		r := msg.Payload.(*RatingPush)
		ack, err := p.handler.HandleRatingPush(ctx, r)
		if err != nil {
			return nil, err
		}
		return &Message{
			Header:  NewMessageHeader(MessageTypeRatingAck),
			Payload: ack,
		}, nil

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

	writer := NewStreamWriter(s)
	reader := NewStreamReader(s)

	msg := &Message{
		Header:  NewMessageHeader(MessageTypeHandshake),
		Payload: h,
	}
	if err := writer.WriteMessage(msg); err != nil {
		return nil, err
	}

	resp, err := reader.ReadMessage()
	if err != nil {
		return nil, err
	}

	return resp.Payload.(*HandshakeAck), nil
}

func (p *Protocol) SendQuery(ctx context.Context, peerID peer.ID, q *Query) (*QueryResult, error) {
	s, err := p.host.NewStream(ctx, peerID, AWSPProtocolID)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}
	defer s.Close()

	writer := NewStreamWriter(s)
	reader := NewStreamReader(s)

	msg := &Message{
		Header:  NewMessageHeader(MessageTypeQuery),
		Payload: q,
	}
	if err := writer.WriteMessage(msg); err != nil {
		return nil, err
	}

	resp, err := reader.ReadMessage()
	if err != nil {
		return nil, err
	}

	return resp.Payload.(*QueryResult), nil
}

func (p *Protocol) SendSyncRequest(ctx context.Context, peerID peer.ID, r *SyncRequest) (*SyncResponse, error) {
	s, err := p.host.NewStream(ctx, peerID, AWSPProtocolID)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}
	defer s.Close()

	writer := NewStreamWriter(s)
	reader := NewStreamReader(s)

	msg := &Message{
		Header:  NewMessageHeader(MessageTypeSyncRequest),
		Payload: r,
	}
	if err := writer.WriteMessage(msg); err != nil {
		return nil, err
	}

	resp, err := reader.ReadMessage()
	if err != nil {
		return nil, err
	}

	return resp.Payload.(*SyncResponse), nil
}
