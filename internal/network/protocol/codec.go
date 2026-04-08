package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

const (
	MaxMessageSize = 64 * 1024 * 1024
	HeaderSize     = 4
)

type Codec struct{}

func NewCodec() *Codec {
	return &Codec{}
}

type Message struct {
	Header  *MessageHeader
	Payload interface{}
}

func (c *Codec) Encode(msg *Message) ([]byte, error) {
	headerBytes, err := json.Marshal(msg.Header)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}

	var payloadBytes []byte
	if msg.Payload != nil {
		payloadBytes, err = json.Marshal(msg.Payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
	}

	buf := make([]byte, HeaderSize+len(headerBytes)+len(payloadBytes))
	buf[0] = byte(len(headerBytes) >> 24)
	buf[1] = byte(len(headerBytes) >> 16)
	buf[2] = byte(len(headerBytes) >> 8)
	buf[3] = byte(len(headerBytes))
	copy(buf[HeaderSize:HeaderSize+len(headerBytes)], headerBytes)
	copy(buf[HeaderSize+len(headerBytes):], payloadBytes)

	return buf, nil
}

func (c *Codec) Decode(r io.Reader) (*Message, error) {
	lenBuf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("read header length: %w", err)
	}

	headerLen := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])
	if headerLen > MaxMessageSize {
		return nil, fmt.Errorf("header too large: %d", headerLen)
	}

	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	header := &MessageHeader{}
	if err := json.Unmarshal(headerBytes, header); err != nil {
		return nil, fmt.Errorf("unmarshal header: %w", err)
	}

	limitedReader := io.LimitReader(r, MaxMessageSize)
	payloadBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	payload, err := c.unmarshalPayload(header.Type, payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &Message{Header: header, Payload: payload}, nil
}

func (c *Codec) unmarshalPayload(msgType MessageType, data []byte) (interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}

	switch msgType {
	case MessageTypeHandshake:
		p := &Handshake{}
		return p, json.Unmarshal(data, p)
	case MessageTypeHandshakeAck:
		p := &HandshakeAck{}
		return p, json.Unmarshal(data, p)
	case MessageTypeQuery:
		p := &Query{}
		return p, json.Unmarshal(data, p)
	case MessageTypeQueryResult:
		p := &QueryResult{}
		return p, json.Unmarshal(data, p)
	case MessageTypeSyncRequest:
		p := &SyncRequest{}
		return p, json.Unmarshal(data, p)
	case MessageTypeSyncResponse:
		p := &SyncResponse{}
		return p, json.Unmarshal(data, p)
	case MessageTypeMirrorRequest:
		p := &MirrorRequest{}
		return p, json.Unmarshal(data, p)
	case MessageTypeMirrorData:
		p := &MirrorData{}
		return p, json.Unmarshal(data, p)
	case MessageTypeMirrorAck:
		p := &MirrorAck{}
		return p, json.Unmarshal(data, p)
	case MessageTypePushEntry:
		p := &PushEntry{}
		return p, json.Unmarshal(data, p)
	case MessageTypePushAck:
		p := &PushAck{}
		return p, json.Unmarshal(data, p)
	case MessageTypeRatingPush:
		p := &RatingPush{}
		return p, json.Unmarshal(data, p)
	case MessageTypeRatingAck:
		p := &RatingAck{}
		return p, json.Unmarshal(data, p)
	case MessageTypeHeartbeat:
		p := &Heartbeat{}
		return p, json.Unmarshal(data, p)
	case MessageTypeBitfield:
		p := &Bitfield{}
		return p, json.Unmarshal(data, p)
	default:
		return nil, nil
	}
}

func NewMessageHeader(msgType MessageType) *MessageHeader {
	return &MessageHeader{
		Type:      msgType,
		MessageID: generateMessageID(),
		Timestamp: currentTimeMillis(),
	}
}

func generateMessageID() string {
	return fmt.Sprintf("%d", currentTimeMillis())
}

func currentTimeMillis() int64 {
	return time.Now().UnixMilli()
}

type StreamReader struct {
	reader *bufio.Reader
	codec  *Codec
}

func NewStreamReader(r io.Reader) *StreamReader {
	return &StreamReader{
		reader: bufio.NewReader(r),
		codec:  NewCodec(),
	}
}

func (sr *StreamReader) ReadMessage() (*Message, error) {
	return sr.codec.Decode(sr.reader)
}

type StreamWriter struct {
	writer *bufio.Writer
	codec  *Codec
}

func NewStreamWriter(w io.Writer) *StreamWriter {
	return &StreamWriter{
		writer: bufio.NewWriter(w),
		codec:  NewCodec(),
	}
}

func (sw *StreamWriter) WriteMessage(msg *Message) error {
	data, err := sw.codec.Encode(msg)
	if err != nil {
		return err
	}
	_, err = sw.writer.Write(data)
	if err != nil {
		return err
	}
	return sw.writer.Flush()
}
