package protocol

import (
	"bytes"
	"io"
	"testing"

	awsp "github.com/daifei0527/agentwiki/internal/network/protocol/proto"
	"google.golang.org/protobuf/proto"
)

func TestProtobufCodec_EncodeDecodeHandshake(t *testing.T) {
	codec := NewProtobufCodec()

	// Create a handshake message
	msg := &awsp.Message{
		Header: &awsp.MessageHeader{
			Type:      awsp.MessageType_MESSAGE_TYPE_HANDSHAKE,
			MessageId: "test-msg-123",
			Timestamp: 1712995200000,
		},
		Payload: &awsp.Message_Handshake{
			Handshake: &awsp.Handshake{
				NodeId:     "node-001",
				PeerId:     "peer-001",
				NodeType:   awsp.NodeType_NODE_TYPE_SEED,
				Version:    "1.0.0",
				Categories: []string{"tech", "science"},
				EntryCount: 100,
			},
		},
	}

	// Encode
	data, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoded, err := codec.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify header
	if decoded.Header.Type != awsp.MessageType_MESSAGE_TYPE_HANDSHAKE {
		t.Errorf("Expected type HANDSHAKE, got %v", decoded.Header.Type)
	}
	if decoded.Header.MessageId != "test-msg-123" {
		t.Errorf("Expected message ID 'test-msg-123', got %v", decoded.Header.MessageId)
	}

	// Verify payload
	handshake := decoded.GetHandshake()
	if handshake == nil {
		t.Fatal("Expected handshake payload, got nil")
	}
	if handshake.NodeId != "node-001" {
		t.Errorf("Expected node ID 'node-001', got %v", handshake.NodeId)
	}
	if handshake.PeerId != "peer-001" {
		t.Errorf("Expected peer ID 'peer-001', got %v", handshake.PeerId)
	}
	if handshake.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %v", handshake.Version)
	}
	if len(handshake.Categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(handshake.Categories))
	}
}

func TestProtobufCodec_EncodeDecodeQuery(t *testing.T) {
	codec := NewProtobufCodec()

	// Create a query message
	msg := &awsp.Message{
		Header: &awsp.MessageHeader{
			Type:      awsp.MessageType_MESSAGE_TYPE_QUERY,
			MessageId: "query-001",
			Timestamp: 1712995200000,
		},
		Payload: &awsp.Message_Query{
			Query: &awsp.Query{
				QueryId:    "query-001",
				Keyword:    "golang protobuf",
				Categories: []string{"programming"},
				Limit:      10,
				Offset:     0,
				QueryType:  awsp.QueryType_QUERY_TYPE_GLOBAL,
			},
		},
	}

	// Encode
	data, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoded, err := codec.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify header
	if decoded.Header.Type != awsp.MessageType_MESSAGE_TYPE_QUERY {
		t.Errorf("Expected type QUERY, got %v", decoded.Header.Type)
	}

	// Verify payload
	query := decoded.GetQuery()
	if query == nil {
		t.Fatal("Expected query payload, got nil")
	}
	if query.Keyword != "golang protobuf" {
		t.Errorf("Expected keyword 'golang protobuf', got %v", query.Keyword)
	}
	if query.QueryType != awsp.QueryType_QUERY_TYPE_GLOBAL {
		t.Errorf("Expected query type GLOBAL, got %v", query.QueryType)
	}
}

func TestProtobufCodec_EncodeDecodeSyncRequest(t *testing.T) {
	codec := NewProtobufCodec()

	// Create a sync request message
	msg := &awsp.Message{
		Header: &awsp.MessageHeader{
			Type:      awsp.MessageType_MESSAGE_TYPE_SYNC_REQUEST,
			MessageId: "sync-001",
			Timestamp: 1712995200000,
		},
		Payload: &awsp.Message_SyncRequest{
			SyncRequest: &awsp.SyncRequest{
				RequestId:         "sync-001",
				LastSyncTimestamp: 1712800000000,
				VersionVector: map[string]int64{
					"node-001": 100,
					"node-002": 50,
				},
				RequestedCategories: []string{"tech", "news"},
			},
		},
	}

	// Encode
	data, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoded, err := codec.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify payload
	syncReq := decoded.GetSyncRequest()
	if syncReq == nil {
		t.Fatal("Expected sync request payload, got nil")
	}
	if syncReq.RequestId != "sync-001" {
		t.Errorf("Expected request ID 'sync-001', got %v", syncReq.RequestId)
	}
	if syncReq.VersionVector["node-001"] != 100 {
		t.Errorf("Expected version vector['node-001'] = 100, got %v", syncReq.VersionVector["node-001"])
	}
	if len(syncReq.RequestedCategories) != 2 {
		t.Errorf("Expected 2 requested categories, got %d", len(syncReq.RequestedCategories))
	}
}

func TestProtobufCodec_SizeReduction(t *testing.T) {
	// Create a message with substantial content
	awspMsg := &awsp.Message{
		Header: &awsp.MessageHeader{
			Type:      awsp.MessageType_MESSAGE_TYPE_HANDSHAKE,
			MessageId: "test-msg-for-size-comparison",
			Timestamp: 1712995200000,
		},
		Payload: &awsp.Message_Handshake{
			Handshake: &awsp.Handshake{
				NodeId:     "node-with-longer-name-001",
				PeerId:     "peer-with-longer-name-001",
				NodeType:   awsp.NodeType_NODE_TYPE_SEED,
				Version:    "1.0.0-beta.1",
				Categories: []string{"technology", "science", "programming", "golang", "distributed-systems"},
				EntryCount: 10000,
			},
		},
	}

	// Encode with protobuf codec
	protoCodec := NewProtobufCodec()
	protoData, err := protoCodec.Encode(awspMsg)
	if err != nil {
		t.Fatalf("Protobuf encode failed: %v", err)
	}

	// Encode with JSON codec for comparison
	jsonCodec := NewCodec()
	jsonMsg := &Message{
		Header: &MessageHeader{
			Type:      MessageTypeHandshake,
			MessageID: "test-msg-for-size-comparison",
			Timestamp: 1712995200000,
		},
		Payload: &Handshake{
			NodeID:     "node-with-longer-name-001",
			PeerID:     "peer-with-longer-name-001",
			NodeType:   NodeTypeSeed,
			Version:    "1.0.0-beta.1",
			Categories: []string{"technology", "science", "programming", "golang", "distributed-systems"},
			EntryCount: 10000,
		},
	}
	jsonData, err := jsonCodec.Encode(jsonMsg)
	if err != nil {
		t.Fatalf("JSON encode failed: %v", err)
	}

	// Protobuf should be smaller
	reduction := float64(len(jsonData)-len(protoData)) / float64(len(jsonData)) * 100
	t.Logf("JSON size: %d bytes, Protobuf size: %d bytes, Reduction: %.1f%%",
		len(jsonData), len(protoData), reduction)

	// Expect at least 30% size reduction for protobuf
	if len(protoData) >= len(jsonData) {
		t.Errorf("Expected protobuf (%d bytes) to be smaller than JSON (%d bytes)",
			len(protoData), len(jsonData))
	}
}

func TestProtobufCodec_StreamReaderWriter(t *testing.T) {
	// Create multiple messages
	messages := []*awsp.Message{
		{
			Header: &awsp.MessageHeader{
				Type:      awsp.MessageType_MESSAGE_TYPE_HANDSHAKE,
				MessageId: "msg-001",
				Timestamp: 1712995200000,
			},
			Payload: &awsp.Message_Handshake{
				Handshake: &awsp.Handshake{
					NodeId:   "node-001",
					PeerId:   "peer-001",
					NodeType: awsp.NodeType_NODE_TYPE_LOCAL,
					Version:  "1.0.0",
				},
			},
		},
		{
			Header: &awsp.MessageHeader{
				Type:      awsp.MessageType_MESSAGE_TYPE_QUERY,
				MessageId: "msg-002",
				Timestamp: 1712995201000,
			},
			Payload: &awsp.Message_Query{
				Query: &awsp.Query{
					QueryId: "query-001",
					Keyword: "test search",
				},
			},
		},
		{
			Header: &awsp.MessageHeader{
				Type:      awsp.MessageType_MESSAGE_TYPE_HEARTBEAT,
				MessageId: "msg-003",
				Timestamp: 1712995202000,
			},
			Payload: &awsp.Message_Heartbeat{
				Heartbeat: &awsp.Heartbeat{
					NodeId:        "node-001",
					UptimeSeconds: 3600,
					EntryCount:    100,
				},
			},
		},
	}

	// Write messages to buffer
	var buf bytes.Buffer
	writer := NewProtobufStreamWriter(&buf)
	for _, msg := range messages {
		if err := writer.WriteMessage(msg); err != nil {
			t.Fatalf("WriteMessage failed: %v", err)
		}
	}

	// Read messages back
	reader := NewProtobufStreamReader(&buf)
	for i, expected := range messages {
		decoded, err := reader.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage[%d] failed: %v", i, err)
		}

		if decoded.Header.Type != expected.Header.Type {
			t.Errorf("Message[%d]: expected type %v, got %v",
				i, expected.Header.Type, decoded.Header.Type)
		}
		if decoded.Header.MessageId != expected.Header.MessageId {
			t.Errorf("Message[%d]: expected message ID %v, got %v",
				i, expected.Header.MessageId, decoded.Header.MessageId)
		}
	}

	// Reading beyond available messages should return EOF
	_, err := reader.ReadMessage()
	if err != io.EOF {
		t.Errorf("Expected EOF after reading all messages, got: %v", err)
	}
}

func TestProtobufCodec_MaxMessageSize(t *testing.T) {
	codec := NewProtobufCodec()

	// Create a message that exceeds max size
	// MaxMessageSize is 64MB, so we create a payload that would exceed it
	largeData := make([]byte, MaxMessageSize+1)
	msg := &awsp.Message{
		Header: &awsp.MessageHeader{
			Type:      awsp.MessageType_MESSAGE_TYPE_QUERY_RESULT,
			MessageId: "large-msg",
			Timestamp: 1712995200000,
		},
		Payload: &awsp.Message_QueryResult{
			QueryResult: &awsp.QueryResult{
				QueryId:    "large-query",
				Entries:    [][]byte{largeData},
				TotalCount: 1,
			},
		},
	}

	// Encode should work (no limit on encode)
	_, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode of large message failed: %v", err)
	}

	// Create a manually crafted oversized message for decode test
	// First, encode a normal message
	normalMsg := &awsp.Message{
		Header: &awsp.MessageHeader{
			Type:      awsp.MessageType_MESSAGE_TYPE_HEARTBEAT,
			MessageId: "normal",
			Timestamp: 1712995200000,
		},
		Payload: &awsp.Message_Heartbeat{
			Heartbeat: &awsp.Heartbeat{
				NodeId: "test",
			},
		},
	}
	_, err = proto.Marshal(normalMsg)
	if err != nil {
		t.Fatalf("Failed to marshal normal message: %v", err)
	}

	// Create a fake length prefix that exceeds max
	fakeLen := MaxMessageSize + 1
	lenBuf := make([]byte, 4)
	lenBuf[0] = byte(fakeLen >> 24)
	lenBuf[1] = byte(fakeLen >> 16)
	lenBuf[2] = byte(fakeLen >> 8)
	lenBuf[3] = byte(fakeLen)

	// Try to decode
	_, err = codec.Decode(bytes.NewReader(lenBuf))
	if err == nil {
		t.Error("Expected error for message exceeding max size, got nil")
	}
}

func TestProtobufCodec_EmptyPayload(t *testing.T) {
	codec := NewProtobufCodec()

	// Create a message with header only
	msg := &awsp.Message{
		Header: &awsp.MessageHeader{
			Type:      awsp.MessageType_MESSAGE_TYPE_HEARTBEAT,
			MessageId: "heartbeat-001",
			Timestamp: 1712995200000,
		},
		Payload: &awsp.Message_Heartbeat{
			Heartbeat: &awsp.Heartbeat{},
		},
	}

	// Encode
	data, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoded, err := codec.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify
	if decoded.Header.Type != awsp.MessageType_MESSAGE_TYPE_HEARTBEAT {
		t.Errorf("Expected type HEARTBEAT, got %v", decoded.Header.Type)
	}
	heartbeat := decoded.GetHeartbeat()
	if heartbeat == nil {
		t.Error("Expected heartbeat payload, got nil")
	}
}
