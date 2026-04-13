package protocol

import (
	"bufio"
	"fmt"
	"io"

	awsp "github.com/daifei0527/agentwiki/internal/network/protocol/proto"
	"google.golang.org/protobuf/proto"
)

// ProtobufCodec implements message encoding/decoding using Protocol Buffers.
// Wire format: [4-byte length prefix][protobuf message]
type ProtobufCodec struct{}

// NewProtobufCodec creates a new ProtobufCodec instance.
func NewProtobufCodec() *ProtobufCodec {
	return &ProtobufCodec{}
}

// Encode serializes a protobuf message with a 4-byte length prefix.
func (c *ProtobufCodec) Encode(msg *awsp.Message) ([]byte, error) {
	// Marshal the protobuf message
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal protobuf message: %w", err)
	}

	// Create buffer with 4-byte length prefix + message data
	buf := make([]byte, HeaderSize+len(data))

	// Write 4-byte big-endian length prefix
	buf[0] = byte(len(data) >> 24)
	buf[1] = byte(len(data) >> 16)
	buf[2] = byte(len(data) >> 8)
	buf[3] = byte(len(data))

	// Copy message data
	copy(buf[HeaderSize:], data)

	return buf, nil
}

// Decode reads a protobuf message from the reader.
// It expects a 4-byte length prefix followed by the protobuf message.
func (c *ProtobufCodec) Decode(r io.Reader) (*awsp.Message, error) {
	// Read 4-byte length prefix
	lenBuf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("read length prefix: %w", err)
	}

	// Parse length (big-endian)
	msgLen := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])

	// Validate message size
	if msgLen > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes (max: %d)", msgLen, MaxMessageSize)
	}
	if msgLen < 0 {
		return nil, fmt.Errorf("invalid message length: %d", msgLen)
	}

	// Read message data
	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(r, msgData); err != nil {
		return nil, fmt.Errorf("read message data: %w", err)
	}

	// Unmarshal protobuf message
	msg := &awsp.Message{}
	if err := proto.Unmarshal(msgData, msg); err != nil {
		return nil, fmt.Errorf("unmarshal protobuf message: %w", err)
	}

	return msg, nil
}

// ProtobufStreamReader provides buffered reading of protobuf messages from a stream.
type ProtobufStreamReader struct {
	reader *bufio.Reader
	codec  *ProtobufCodec
}

// NewProtobufStreamReader creates a new ProtobufStreamReader.
func NewProtobufStreamReader(r io.Reader) *ProtobufStreamReader {
	return &ProtobufStreamReader{
		reader: bufio.NewReader(r),
		codec:  NewProtobufCodec(),
	}
}

// ReadMessage reads and decodes the next protobuf message from the stream.
func (sr *ProtobufStreamReader) ReadMessage() (*awsp.Message, error) {
	msg, err := sr.codec.Decode(sr.reader)
	if err != nil {
		// Convert io.ErrUnexpectedEOF to io.EOF for stream end detection
		if err.Error() == "read length prefix: EOF" || err.Error() == "read length prefix: unexpected EOF" {
			return nil, io.EOF
		}
		return nil, err
	}
	return msg, nil
}

// ProtobufStreamWriter provides buffered writing of protobuf messages to a stream.
type ProtobufStreamWriter struct {
	writer *bufio.Writer
	codec  *ProtobufCodec
}

// NewProtobufStreamWriter creates a new ProtobufStreamWriter.
func NewProtobufStreamWriter(w io.Writer) *ProtobufStreamWriter {
	return &ProtobufStreamWriter{
		writer: bufio.NewWriter(w),
		codec:  NewProtobufCodec(),
	}
}

// WriteMessage encodes and writes a protobuf message to the stream.
func (sw *ProtobufStreamWriter) WriteMessage(msg *awsp.Message) error {
	data, err := sw.codec.Encode(msg)
	if err != nil {
		return err
	}

	if _, err := sw.writer.Write(data); err != nil {
		return err
	}

	return sw.writer.Flush()
}
