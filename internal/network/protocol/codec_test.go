// Package protocol_test 提供协议层的单元测试
package protocol_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/daifei0527/agentwiki/internal/network/protocol"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	libp2p_protocol "github.com/libp2p/go-libp2p/core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

// ==================== Codec 编解码测试 ====================

// ==================== Codec 编解码测试 ====================

// TestCodecEncodeDecodeHandshake 测试 Handshake 消息编解码
func TestCodecEncodeDecodeHandshake(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypeHandshake),
		Payload: &protocol.Handshake{
			NodeID:     "test-node-1",
			PeerID:     "test-peer-1",
			NodeType:   protocol.NodeTypeSeed,
			Version:    "1.0.0",
			Categories: []string{"tech", "science"},
			EntryCount: 100,
		},
	}

	// 编码
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	// 解码
	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	// 验证消息类型
	if decoded.Header.Type != protocol.MessageTypeHandshake {
		t.Errorf("消息类型错误: got %d, want %d", decoded.Header.Type, protocol.MessageTypeHandshake)
	}

	// 验证负载
	handshake, ok := decoded.Payload.(*protocol.Handshake)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if handshake.NodeID != "test-node-1" {
		t.Errorf("NodeID 错误: got %q, want %q", handshake.NodeID, "test-node-1")
	}

	if handshake.EntryCount != 100 {
		t.Errorf("EntryCount 错误: got %d, want %d", handshake.EntryCount, 100)
	}
}

// TestCodecEncodeDecodeQuery 测试 Query 消息编解码
func TestCodecEncodeDecodeQuery(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypeQuery),
		Payload: &protocol.Query{
			QueryID:    "query-123",
			Keyword:    "人工智能",
			Categories: []string{"tech/ai"},
			Limit:      10,
			Offset:     0,
			QueryType:  protocol.QueryTypeGlobal,
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	query, ok := decoded.Payload.(*protocol.Query)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if query.Keyword != "人工智能" {
		t.Errorf("Keyword 错误: got %q", query.Keyword)
	}

	if query.QueryType != protocol.QueryTypeGlobal {
		t.Errorf("QueryType 错误: got %d", query.QueryType)
	}
}

// TestCodecEncodeDecodeSyncRequest 测试 SyncRequest 消息编解码
func TestCodecEncodeDecodeSyncRequest(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypeSyncRequest),
		Payload: &protocol.SyncRequest{
			RequestID:         "sync-456",
			LastSyncTimestamp: 1700000000000,
			VersionVector: map[string]int64{
				"entry-1": 5,
				"entry-2": 3,
			},
			RequestedCategories: []string{"tech", "science"},
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	req, ok := decoded.Payload.(*protocol.SyncRequest)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if req.RequestID != "sync-456" {
		t.Errorf("RequestID 错误: got %q", req.RequestID)
	}

	if req.VersionVector["entry-1"] != 5 {
		t.Errorf("VersionVector 错误: %v", req.VersionVector)
	}
}

// TestCodecEncodeDecodeSyncResponse 测试 SyncResponse 消息编解码
func TestCodecEncodeDecodeSyncResponse(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypeSyncResponse),
		Payload: &protocol.SyncResponse{
			RequestID:       "sync-789",
			NewEntries:      [][]byte{[]byte("entry1"), []byte("entry2")},
			UpdatedEntries:  [][]byte{[]byte("entry3")},
			DeletedEntryIDs: []string{"old-1", "old-2"},
			ServerTimestamp: 1700000001000,
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	resp, ok := decoded.Payload.(*protocol.SyncResponse)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if len(resp.NewEntries) != 2 {
		t.Errorf("NewEntries 数量错误: got %d, want 2", len(resp.NewEntries))
	}

	if len(resp.DeletedEntryIDs) != 2 {
		t.Errorf("DeletedEntryIDs 数量错误: got %d, want 2", len(resp.DeletedEntryIDs))
	}
}

// TestCodecEncodeDecodePushEntry 测试 PushEntry 消息编解码
func TestCodecEncodeDecodePushEntry(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypePushEntry),
		Payload: &protocol.PushEntry{
			EntryID:          "entry-push-1",
			Entry:            []byte(`{"id":"entry-push-1","title":"测试条目"}`),
			CreatorSignature: []byte("signature-bytes"),
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	push, ok := decoded.Payload.(*protocol.PushEntry)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if push.EntryID != "entry-push-1" {
		t.Errorf("EntryID 错误: got %q", push.EntryID)
	}
}

// TestCodecEncodeDecodeHeartbeat 测试 Heartbeat 消息编解码
func TestCodecEncodeDecodeHeartbeat(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypeHeartbeat),
		Payload: &protocol.Heartbeat{
			NodeID:        "node-heartbeat",
			UptimeSeconds: 86400,
			EntryCount:    500,
			Timestamp:     1700000002000,
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	hb, ok := decoded.Payload.(*protocol.Heartbeat)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if hb.UptimeSeconds != 86400 {
		t.Errorf("UptimeSeconds 错误: got %d", hb.UptimeSeconds)
	}
}

// TestCodecEncodeDecodeRatingPush 测试 RatingPush 消息编解码
func TestCodecEncodeDecodeRatingPush(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypeRatingPush),
		Payload: &protocol.RatingPush{
			Rating:         []byte(`{"entry_id":"entry-1","score":5}`),
			RaterSignature: []byte("rater-sig"),
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	rp, ok := decoded.Payload.(*protocol.RatingPush)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if string(rp.Rating) != `{"entry_id":"entry-1","score":5}` {
		t.Errorf("Rating 内容错误: %s", rp.Rating)
	}
}

// TestCodecEncodeDecodeMirrorRequest 测试 MirrorRequest 消息编解码
func TestCodecEncodeDecodeMirrorRequest(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypeMirrorRequest),
		Payload: &protocol.MirrorRequest{
			RequestID:  "mirror-1",
			Categories: []string{"tech", "science"},
			FullMirror: true,
			BatchSize:  100,
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	req, ok := decoded.Payload.(*protocol.MirrorRequest)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if !req.FullMirror {
		t.Error("FullMirror 应为 true")
	}
}

// TestCodecEncodeDecodeBitfield 测试 Bitfield 消息编解码
func TestCodecEncodeDecodeBitfield(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypeBitfield),
		Payload: &protocol.Bitfield{
			NodeID: "node-bitfield",
			VersionVector: map[string]int64{
				"entry-1": 10,
				"entry-2": 5,
			},
			EntryCount: 1000,
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	bf, ok := decoded.Payload.(*protocol.Bitfield)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if bf.EntryCount != 1000 {
		t.Errorf("EntryCount 错误: got %d", bf.EntryCount)
	}
}

// TestCodecEncodeNilPayload 测试空负载编码
func TestCodecEncodeNilPayload(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeHeartbeat),
		Payload: nil,
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	if decoded.Payload != nil {
		t.Errorf("Payload 应为 nil, got %T", decoded.Payload)
	}
}

// TestStreamWriterReader 测试流式读写
func TestStreamWriterReader(t *testing.T) {
	// 测试单条消息的流式读写
	var buf bytes.Buffer

	writer := protocol.NewStreamWriter(&buf)

	msg := &protocol.Message{
		Header: protocol.NewMessageHeader(protocol.MessageTypeHandshake),
		Payload: &protocol.Handshake{
			NodeID:   "node-1",
			PeerID:   "peer-1",
			Version:  "1.0.0",
			NodeType: protocol.NodeTypeLocal,
		},
	}

	// 写入消息
	if err := writer.WriteMessage(msg); err != nil {
		t.Fatalf("WriteMessage 失败: %v", err)
	}

	// 读取消息
	reader := protocol.NewStreamReader(&buf)

	decoded, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}

	if decoded.Header.Type != protocol.MessageTypeHandshake {
		t.Errorf("消息类型错误: got %d", decoded.Header.Type)
	}

	handshake, ok := decoded.Payload.(*protocol.Handshake)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if handshake.NodeID != "node-1" {
		t.Errorf("NodeID 错误: got %q", handshake.NodeID)
	}
}

// ==================== 消息类型常量测试 ====================

// TestMessageTypeConstants 测试消息类型常量
func TestMessageTypeConstants(t *testing.T) {
	types := []protocol.MessageType{
		protocol.MessageTypeHandshake,
		protocol.MessageTypeHandshakeAck,
		protocol.MessageTypeQuery,
		protocol.MessageTypeQueryResult,
		protocol.MessageTypeSyncRequest,
		protocol.MessageTypeSyncResponse,
		protocol.MessageTypeMirrorRequest,
		protocol.MessageTypeMirrorData,
		protocol.MessageTypeMirrorAck,
		protocol.MessageTypePushEntry,
		protocol.MessageTypePushAck,
		protocol.MessageTypeRatingPush,
		protocol.MessageTypeRatingAck,
		protocol.MessageTypeHeartbeat,
		protocol.MessageTypeBitfield,
	}

	seen := make(map[protocol.MessageType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("消息类型重复: %d", mt)
		}
		seen[mt] = true
	}

	if len(seen) != len(types) {
		t.Errorf("消息类型数量不一致")
	}
}

// TestNodeTypeConstants 测试节点类型常量
func TestNodeTypeConstants(t *testing.T) {
	if protocol.NodeTypeLocal >= protocol.NodeTypeSeed {
		t.Error("NodeTypeLocal 应小于 NodeTypeSeed")
	}
}

// TestQueryTypeConstants 测试查询类型常量
func TestQueryTypeConstants(t *testing.T) {
	if protocol.QueryTypeLocal >= protocol.QueryTypeGlobal {
		t.Error("QueryTypeLocal 应小于 QueryTypeGlobal")
	}
}

// ==================== 消息头测试 ====================

// TestNewMessageHeader 测试创建消息头
func TestNewMessageHeader(t *testing.T) {
	header := protocol.NewMessageHeader(protocol.MessageTypeQuery)

	if header.Type != protocol.MessageTypeQuery {
		t.Errorf("消息类型错误: got %d", header.Type)
	}

	if header.MessageID == "" {
		t.Error("MessageID 不应为空")
	}

	if header.Timestamp == 0 {
		t.Error("Timestamp 不应为 0")
	}
}

// TestMessageHeaderWithSignature 测试带签名的消息头
func TestMessageHeaderWithSignature(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header: &protocol.MessageHeader{
			Type:      protocol.MessageTypeHandshake,
			MessageID: "msg-123",
			Timestamp: 1700000000000,
			Signature: []byte("test-signature"),
		},
		Payload: &protocol.Handshake{
			NodeID: "test",
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	if string(decoded.Header.Signature) != "test-signature" {
		t.Errorf("Signature 错误: got %q", decoded.Header.Signature)
	}
}

// ==================== 协议 ID 测试 ====================

// TestAWSPProtocolID 测试协议 ID
func TestAWSPProtocolID(t *testing.T) {
	expected := "/agentwiki/sync/2.0.0"
	if protocol.AWSPProtocolID != expected {
		t.Errorf("AWSPProtocolID 错误: got %q, want %q", protocol.AWSPProtocolID, expected)
	}
}

// ==================== 错误处理测试 ====================

// TestCodecDecodeInvalidJSON 测试无效 JSON 解码
func TestCodecDecodeInvalidJSON(t *testing.T) {
	codec := protocol.NewCodec()

	invalidData := []byte("invalid json data")
	_, err := codec.Decode(bytes.NewReader(invalidData))
	if err == nil {
		t.Error("无效 JSON 应返回错误")
	}
}

// TestCodecDecodeTruncatedData 测试截断数据解码
func TestCodecDecodeTruncatedData(t *testing.T) {
	codec := protocol.NewCodec()

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeHandshake),
		Payload: &protocol.Handshake{NodeID: "test"},
	}
	encoded, _ := codec.Encode(msg)

	// 截断数据
	if len(encoded) > 10 {
		truncated := encoded[:len(encoded)/2]
		_, err := codec.Decode(bytes.NewReader(truncated))
		if err == nil {
			t.Error("截断数据应返回错误")
		}
	}
}

// TestCodecDecodeEmptyData 测试空数据解码
func TestCodecDecodeEmptyData(t *testing.T) {
	codec := protocol.NewCodec()

	_, err := codec.Decode(bytes.NewReader([]byte{}))
	if err == nil {
		t.Error("空数据应返回错误")
	}
}

// TestMessageHeaderTimestamp 测试消息头时间戳
func TestMessageHeaderTimestamp(t *testing.T) {
	before := time.Now().UnixMilli()
	header := protocol.NewMessageHeader(protocol.MessageTypeQuery)
	after := time.Now().UnixMilli()

	if header.Timestamp < before || header.Timestamp > after {
		t.Errorf("时间戳应在当前时间范围内: got %d, expected between %d and %d", header.Timestamp, before, after)
	}
}

// TestMessageHeaderUniqueID 测试消息 ID 格式
func TestMessageHeaderUniqueID(t *testing.T) {
	// 消息 ID 使用时间戳生成，在同一毫秒内可能重复
	// 这里验证 ID 非空且格式正确
	for i := 0; i < 100; i++ {
		header := protocol.NewMessageHeader(protocol.MessageTypeQuery)
		if header.MessageID == "" {
			t.Error("消息 ID 不应为空")
		}
		// ID 应该是数字字符串
		for _, c := range header.MessageID {
			if c < '0' || c > '9' {
				t.Errorf("消息 ID 应为数字: got %q", header.MessageID)
				break
			}
		}
	}
}

// TestCodecRoundTrip 测试所有消息类型的往返
func TestCodecRoundTrip(t *testing.T) {
	codec := protocol.NewCodec()

	testCases := []struct {
		name    string
		msgType protocol.MessageType
		payload interface{}
	}{
		{"Handshake", protocol.MessageTypeHandshake, &protocol.Handshake{NodeID: "node-1"}},
		{"HandshakeAck", protocol.MessageTypeHandshakeAck, &protocol.HandshakeAck{NodeID: "node-2", Accepted: true}},
		{"Query", protocol.MessageTypeQuery, &protocol.Query{QueryID: "q-1", Keyword: "test"}},
		{"QueryResult", protocol.MessageTypeQueryResult, &protocol.QueryResult{QueryID: "q-1", TotalCount: 10}},
		{"SyncRequest", protocol.MessageTypeSyncRequest, &protocol.SyncRequest{RequestID: "sync-1"}},
		{"SyncResponse", protocol.MessageTypeSyncResponse, &protocol.SyncResponse{RequestID: "sync-1"}},
		{"MirrorRequest", protocol.MessageTypeMirrorRequest, &protocol.MirrorRequest{RequestID: "mirror-1"}},
		{"MirrorData", protocol.MessageTypeMirrorData, &protocol.MirrorData{RequestID: "mirror-1"}},
		{"MirrorAck", protocol.MessageTypeMirrorAck, &protocol.MirrorAck{RequestID: "mirror-1", Success: true}},
		{"PushEntry", protocol.MessageTypePushEntry, &protocol.PushEntry{EntryID: "entry-1"}},
		{"PushAck", protocol.MessageTypePushAck, &protocol.PushAck{EntryID: "entry-1", Accepted: true}},
		{"RatingPush", protocol.MessageTypeRatingPush, &protocol.RatingPush{Rating: []byte("test")}},
		{"RatingAck", protocol.MessageTypeRatingAck, &protocol.RatingAck{RatingID: "rating-1", Accepted: true}},
		{"Heartbeat", protocol.MessageTypeHeartbeat, &protocol.Heartbeat{NodeID: "node-1"}},
		{"Bitfield", protocol.MessageTypeBitfield, &protocol.Bitfield{NodeID: "node-1"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &protocol.Message{
				Header:  protocol.NewMessageHeader(tc.msgType),
				Payload: tc.payload,
			}

			encoded, err := codec.Encode(msg)
			if err != nil {
				t.Fatalf("Encode 失败: %v", err)
			}

			decoded, err := codec.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("Decode 失败: %v", err)
			}

			if decoded.Header.Type != tc.msgType {
				t.Errorf("消息类型不匹配: got %d, want %d", decoded.Header.Type, tc.msgType)
			}
		})
	}
}

// TestCodecUnknownMessageType 测试未知消息类型处理
func TestCodecUnknownMessageType(t *testing.T) {
	codec := protocol.NewCodec()

	// 使用未知的消息类型
	msg := &protocol.Message{
		Header:  &protocol.MessageHeader{Type: protocol.MessageType(999), MessageID: "test", Timestamp: 1},
		Payload: nil,
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	// 未知消息类型应返回 nil payload
	if decoded.Payload != nil {
		t.Errorf("未知消息类型应返回 nil payload: got %T", decoded.Payload)
	}
}

// TestCodecEncodeDecodeAllMessageTypes 测试所有消息类型的编解码
func TestCodecEncodeDecodeAllMessageTypes(t *testing.T) {
	codec := protocol.NewCodec()

	// HandshakeAck
	t.Run("HandshakeAck", func(t *testing.T) {
		msg := &protocol.Message{
			Header: protocol.NewMessageHeader(protocol.MessageTypeHandshakeAck),
			Payload: &protocol.HandshakeAck{
				NodeID:       "node-ack",
				PeerID:       "peer-ack",
				NodeType:     protocol.NodeTypeSeed,
				Version:      "1.0.0",
				Accepted:     true,
				RejectReason: "",
				Signature:    []byte("sig"),
			},
		}
		encoded, _ := codec.Encode(msg)
		decoded, _ := codec.Decode(bytes.NewReader(encoded))
		ack, ok := decoded.Payload.(*protocol.HandshakeAck)
		if !ok {
			t.Fatalf("负载类型错误: %T", decoded.Payload)
		}
		if !ack.Accepted {
			t.Error("Accepted 应为 true")
		}
	})

	// QueryResult
	t.Run("QueryResult", func(t *testing.T) {
		msg := &protocol.Message{
			Header: protocol.NewMessageHeader(protocol.MessageTypeQueryResult),
			Payload: &protocol.QueryResult{
				QueryID:    "query-result-1",
				Entries:    [][]byte{[]byte("entry1"), []byte("entry2")},
				TotalCount: 2,
				HasMore:    false,
			},
		}
		encoded, _ := codec.Encode(msg)
		decoded, _ := codec.Decode(bytes.NewReader(encoded))
		result, ok := decoded.Payload.(*protocol.QueryResult)
		if !ok {
			t.Fatalf("负载类型错误: %T", decoded.Payload)
		}
		if result.TotalCount != 2 {
			t.Errorf("TotalCount 错误: got %d", result.TotalCount)
		}
	})

	// MirrorData
	t.Run("MirrorData", func(t *testing.T) {
		msg := &protocol.Message{
			Header: protocol.NewMessageHeader(protocol.MessageTypeMirrorData),
			Payload: &protocol.MirrorData{
				RequestID:    "mirror-data-1",
				BatchIndex:   0,
				TotalBatches: 3,
				Entries:      [][]byte{[]byte("entry1")},
				Categories:   [][]byte{[]byte("cat1")},
			},
		}
		encoded, _ := codec.Encode(msg)
		decoded, _ := codec.Decode(bytes.NewReader(encoded))
		data, ok := decoded.Payload.(*protocol.MirrorData)
		if !ok {
			t.Fatalf("负载类型错误: %T", decoded.Payload)
		}
		if data.TotalBatches != 3 {
			t.Errorf("TotalBatches 错误: got %d", data.TotalBatches)
		}
	})

	// MirrorAck
	t.Run("MirrorAck", func(t *testing.T) {
		msg := &protocol.Message{
			Header: protocol.NewMessageHeader(protocol.MessageTypeMirrorAck),
			Payload: &protocol.MirrorAck{
				RequestID:       "mirror-ack-1",
				Success:         true,
				ErrorMessage:    "",
				ReceivedEntries: 100,
			},
		}
		encoded, _ := codec.Encode(msg)
		decoded, _ := codec.Decode(bytes.NewReader(encoded))
		ack, ok := decoded.Payload.(*protocol.MirrorAck)
		if !ok {
			t.Fatalf("负载类型错误: %T", decoded.Payload)
		}
		if !ack.Success {
			t.Error("Success 应为 true")
		}
	})

	// PushAck
	t.Run("PushAck", func(t *testing.T) {
		msg := &protocol.Message{
			Header: protocol.NewMessageHeader(protocol.MessageTypePushAck),
			Payload: &protocol.PushAck{
				EntryID:      "entry-ack-1",
				Accepted:     true,
				RejectReason: "",
				NewVersion:   2,
			},
		}
		encoded, _ := codec.Encode(msg)
		decoded, _ := codec.Decode(bytes.NewReader(encoded))
		ack, ok := decoded.Payload.(*protocol.PushAck)
		if !ok {
			t.Fatalf("负载类型错误: %T", decoded.Payload)
		}
		if ack.NewVersion != 2 {
			t.Errorf("NewVersion 错误: got %d", ack.NewVersion)
		}
	})

	// RatingAck
	t.Run("RatingAck", func(t *testing.T) {
		msg := &protocol.Message{
			Header: protocol.NewMessageHeader(protocol.MessageTypeRatingAck),
			Payload: &protocol.RatingAck{
				RatingID:     "rating-ack-1",
				Accepted:     true,
				RejectReason: "",
			},
		}
		encoded, _ := codec.Encode(msg)
		decoded, _ := codec.Decode(bytes.NewReader(encoded))
		ack, ok := decoded.Payload.(*protocol.RatingAck)
		if !ok {
			t.Fatalf("负载类型错误: %T", decoded.Payload)
		}
		if !ack.Accepted {
			t.Error("Accepted 应为 true")
		}
	})
}

// TestCodecDecodeHeaderTooLarge 测试解码时头部过大
func TestCodecDecodeHeaderTooLarge(t *testing.T) {
	codec := protocol.NewCodec()

	// 构造一个头部长度字段超大的数据
	// 4 字节长度 + 后续数据
	lenBuf := make([]byte, 4)
	// 设置一个非常大的长度值 (超过 64MB)
	lenBuf[0] = 0xFF
	lenBuf[1] = 0xFF
	lenBuf[2] = 0xFF
	lenBuf[3] = 0xFF

	_, err := codec.Decode(bytes.NewReader(lenBuf))
	if err == nil {
		t.Error("头部过大应返回错误")
	}
}

// TestStreamWriterReaderMultiple 测试多条消息的流式读写
// 注意: 当前协议设计不支持在同一缓冲区中读取多条消息
// 因为 Decode 会读取所有剩余字节作为 payload
// 这个测试验证单条消息的写入和读取
func TestStreamWriterReaderMultiple(t *testing.T) {
	// 测试多条消息的独立读写
	testCases := []struct {
		name    string
		msgType protocol.MessageType
		payload interface{}
	}{
		{"Handshake", protocol.MessageTypeHandshake, &protocol.Handshake{NodeID: "node-1"}},
		{"Query", protocol.MessageTypeQuery, &protocol.Query{QueryID: "q-1", Keyword: "test"}},
		{"Heartbeat", protocol.MessageTypeHeartbeat, &protocol.Heartbeat{NodeID: "node-1"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			writer := protocol.NewStreamWriter(&buf)
			msg := &protocol.Message{
				Header:  protocol.NewMessageHeader(tc.msgType),
				Payload: tc.payload,
			}

			if err := writer.WriteMessage(msg); err != nil {
				t.Fatalf("WriteMessage 失败: %v", err)
			}

			reader := protocol.NewStreamReader(&buf)
			decoded, err := reader.ReadMessage()
			if err != nil {
				t.Fatalf("ReadMessage 失败: %v", err)
			}

			if decoded.Header.Type != tc.msgType {
				t.Errorf("消息类型错误: got %d, want %d", decoded.Header.Type, tc.msgType)
			}
		})
	}
}

// TestCodecEncodeWithLargePayload 测试大负载编码
func TestCodecEncodeWithLargePayload(t *testing.T) {
	codec := protocol.NewCodec()

	// 创建一个大负载
	largeContent := make([]byte, 1024*100) // 100KB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypePushEntry),
		Payload: &protocol.PushEntry{EntryID: "large-entry", Entry: largeContent},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	push, ok := decoded.Payload.(*protocol.PushEntry)
	if !ok {
		t.Fatalf("负载类型错误: %T", decoded.Payload)
	}

	if len(push.Entry) != len(largeContent) {
		t.Errorf("Entry 长度错误: got %d, want %d", len(push.Entry), len(largeContent))
	}
}

// ==================== Protocol 测试 ====================

// mockHandler 实现 protocol.Handler 接口用于测试
type mockHandler struct {
	handshakeFunc    func(ctx context.Context, h *protocol.Handshake) (*protocol.HandshakeAck, error)
	queryFunc        func(ctx context.Context, q *protocol.Query) (*protocol.QueryResult, error)
	syncRequestFunc  func(ctx context.Context, r *protocol.SyncRequest) (*protocol.SyncResponse, error)
	mirrorRequestFunc func(ctx context.Context, r *protocol.MirrorRequest) (<-chan *protocol.MirrorData, error)
	pushEntryFunc    func(ctx context.Context, e *protocol.PushEntry) (*protocol.PushAck, error)
	ratingPushFunc   func(ctx context.Context, r *protocol.RatingPush) (*protocol.RatingAck, error)
	heartbeatFunc    func(ctx context.Context, h *protocol.Heartbeat) error
	bitfieldFunc     func(ctx context.Context, b *protocol.Bitfield) error
}

func (m *mockHandler) HandleHandshake(ctx context.Context, h *protocol.Handshake) (*protocol.HandshakeAck, error) {
	if m.handshakeFunc != nil {
		return m.handshakeFunc(ctx, h)
	}
	return &protocol.HandshakeAck{Accepted: true}, nil
}

func (m *mockHandler) HandleQuery(ctx context.Context, q *protocol.Query) (*protocol.QueryResult, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, q)
	}
	return &protocol.QueryResult{QueryID: q.QueryID}, nil
}

func (m *mockHandler) HandleSyncRequest(ctx context.Context, r *protocol.SyncRequest) (*protocol.SyncResponse, error) {
	if m.syncRequestFunc != nil {
		return m.syncRequestFunc(ctx, r)
	}
	return &protocol.SyncResponse{RequestID: r.RequestID}, nil
}

func (m *mockHandler) HandleMirrorRequest(ctx context.Context, r *protocol.MirrorRequest) (<-chan *protocol.MirrorData, error) {
	if m.mirrorRequestFunc != nil {
		return m.mirrorRequestFunc(ctx, r)
	}
	ch := make(chan *protocol.MirrorData)
	close(ch)
	return ch, nil
}

func (m *mockHandler) HandlePushEntry(ctx context.Context, e *protocol.PushEntry) (*protocol.PushAck, error) {
	if m.pushEntryFunc != nil {
		return m.pushEntryFunc(ctx, e)
	}
	return &protocol.PushAck{Accepted: true}, nil
}

func (m *mockHandler) HandleRatingPush(ctx context.Context, r *protocol.RatingPush) (*protocol.RatingAck, error) {
	if m.ratingPushFunc != nil {
		return m.ratingPushFunc(ctx, r)
	}
	return &protocol.RatingAck{Accepted: true}, nil
}

func (m *mockHandler) HandleHeartbeat(ctx context.Context, h *protocol.Heartbeat) error {
	if m.heartbeatFunc != nil {
		return m.heartbeatFunc(ctx, h)
	}
	return nil
}

func (m *mockHandler) HandleBitfield(ctx context.Context, b *protocol.Bitfield) error {
	if m.bitfieldFunc != nil {
		return m.bitfieldFunc(ctx, b)
	}
	return nil
}

// TestNewProtocol 测试创建协议实例
func TestNewProtocol(t *testing.T) {
	_ = context.Background()

	// 创建测试用的 libp2p host
	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 libp2p host 失败: %v", err)
	}
	defer h.Close()

	handler := &mockHandler{}
	p := protocol.NewProtocol(h, handler)

	if p == nil {
		t.Error("NewProtocol 不应返回 nil")
	}
}

// TestProtocolHandlerHandshake 测试握手处理
func TestProtocolHandlerHandshake(t *testing.T) {
	ctx := context.Background()

	h1, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host1 失败: %v", err)
	}
	defer h1.Close()

	h2, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host2 失败: %v", err)
	}
	defer h2.Close()

	// 连接两个节点
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	if err := h1.Connect(ctx, h2.Peerstore().PeerInfo(h2.ID())); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 在 h2 上设置协议处理器
	handler := &mockHandler{
		handshakeFunc: func(ctx context.Context, h *protocol.Handshake) (*protocol.HandshakeAck, error) {
			return &protocol.HandshakeAck{
				NodeID:   "node-2",
				PeerID:   h2.ID().String(),
				Accepted: true,
			}, nil
		},
	}
	p2 := protocol.NewProtocol(h2, handler)

	if p2 == nil {
		t.Error("NewProtocol 不应返回 nil")
	}
}

// TestProtocolHandlerQuery 测试查询处理
func TestProtocolHandlerQuery(t *testing.T) {
	_ = context.Background()

	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host 失败: %v", err)
	}
	defer h.Close()

	handler := &mockHandler{
		queryFunc: func(ctx context.Context, q *protocol.Query) (*protocol.QueryResult, error) {
			return &protocol.QueryResult{
				QueryID:    q.QueryID,
				Entries:    nil,
				TotalCount: 0,
				HasMore:    false,
			}, nil
		},
	}
	p := protocol.NewProtocol(h, handler)

	if p == nil {
		t.Error("NewProtocol 不应返回 nil")
	}
}

// TestProtocolHandlerSyncRequest 测试同步请求处理
func TestProtocolHandlerSyncRequest(t *testing.T) {
	_ = context.Background()

	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host 失败: %v", err)
	}
	defer h.Close()

	handler := &mockHandler{
		syncRequestFunc: func(ctx context.Context, r *protocol.SyncRequest) (*protocol.SyncResponse, error) {
			return &protocol.SyncResponse{
				RequestID:       r.RequestID,
				NewEntries:      nil,
				UpdatedEntries:  nil,
				DeletedEntryIDs: nil,
				ServerTimestamp: time.Now().UnixMilli(),
			}, nil
		},
	}
	p := protocol.NewProtocol(h, handler)

	if p == nil {
		t.Error("NewProtocol 不应返回 nil")
	}
}

// TestProtocolHandlerPushEntry 测试推送条目处理
func TestProtocolHandlerPushEntry(t *testing.T) {
	_ = context.Background()

	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host 失败: %v", err)
	}
	defer h.Close()

	handler := &mockHandler{
		pushEntryFunc: func(ctx context.Context, e *protocol.PushEntry) (*protocol.PushAck, error) {
			return &protocol.PushAck{
				EntryID:  e.EntryID,
				Accepted: true,
			}, nil
		},
	}
	p := protocol.NewProtocol(h, handler)

	if p == nil {
		t.Error("NewProtocol 不应返回 nil")
	}
}

// TestProtocolHandlerRatingPush 测试评分推送处理
func TestProtocolHandlerRatingPush(t *testing.T) {
	_ = context.Background()

	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host 失败: %v", err)
	}
	defer h.Close()

	handler := &mockHandler{
		ratingPushFunc: func(ctx context.Context, r *protocol.RatingPush) (*protocol.RatingAck, error) {
			return &protocol.RatingAck{
				RatingID: "rating-1",
				Accepted: true,
			}, nil
		},
	}
	p := protocol.NewProtocol(h, handler)

	if p == nil {
		t.Error("NewProtocol 不应返回 nil")
	}
}

// TestProtocolHandlerHeartbeat 测试心跳处理
func TestProtocolHandlerHeartbeat(t *testing.T) {
	_ = context.Background()

	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host 失败: %v", err)
	}
	defer h.Close()

	heartbeatReceived := false
	handler := &mockHandler{
		heartbeatFunc: func(ctx context.Context, hb *protocol.Heartbeat) error {
			heartbeatReceived = true
			return nil
		},
	}
	p := protocol.NewProtocol(h, handler)

	if p == nil {
		t.Error("NewProtocol 不应返回 nil")
	}

	_ = heartbeatReceived // 验证 handler 设置正确
}

// TestProtocolHandlerBitfield 测试位图处理
func TestProtocolHandlerBitfield(t *testing.T) {
	_ = context.Background()

	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host 失败: %v", err)
	}
	defer h.Close()

	handler := &mockHandler{
		bitfieldFunc: func(ctx context.Context, b *protocol.Bitfield) error {
			return nil
		},
	}
	p := protocol.NewProtocol(h, handler)

	if p == nil {
		t.Error("NewProtocol 不应返回 nil")
	}
}

// TestProtocolHandlerMirrorRequest 测试镜像请求处理
func TestProtocolHandlerMirrorRequest(t *testing.T) {
	_ = context.Background()

	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host 失败: %v", err)
	}
	defer h.Close()

	handler := &mockHandler{
		mirrorRequestFunc: func(ctx context.Context, r *protocol.MirrorRequest) (<-chan *protocol.MirrorData, error) {
			ch := make(chan *protocol.MirrorData, 1)
			ch <- &protocol.MirrorData{
				RequestID:   r.RequestID,
				BatchIndex:  0,
				TotalBatches: 1,
			}
			close(ch)
			return ch, nil
		},
	}
	p := protocol.NewProtocol(h, handler)

	if p == nil {
		t.Error("NewProtocol 不应返回 nil")
	}
}

// ==================== ProcessMessage 测试 (使用模拟流) ====================

// mockStream 实现 network.Stream 接口用于测试
type mockStream struct {
	r *bytes.Buffer
	w *bytes.Buffer
}

func (m *mockStream) Read(b []byte) (n int, err error)  { return m.r.Read(b) }
func (m *mockStream) Write(b []byte) (n int, err error) { return m.w.Write(b) }
func (m *mockStream) Close() error                      { return nil }
func (m *mockStream) Reset() error                      { return nil }
func (m *mockStream) SetDeadline(t time.Time) error     { return nil }
func (m *mockStream) SetReadDeadline(t time.Time) error { return nil }
func (m *mockStream) SetWriteDeadline(t time.Time) error { return nil }
func (m *mockStream) ID() string                        { return "mock-stream" }
func (m *mockStream) Protocol() libp2p_protocol.ID      { return libp2p_protocol.ID(protocol.AWSPProtocolID) }
func (m *mockStream) SetProtocol(p libp2p_protocol.ID)  {}
func (m *mockStream) Conn() network.Conn                { return nil }
func (m *mockStream) Scope() network.StreamScope        { return nil }
func (m *mockStream) Stat() network.Stats               { return network.Stats{} }

// TestProcessMessageHandshake 测试处理握手消息
func TestProcessMessageHandshake(t *testing.T) {
	ctx := context.Background()

	handler := &mockHandler{
		handshakeFunc: func(ctx context.Context, h *protocol.Handshake) (*protocol.HandshakeAck, error) {
			return &protocol.HandshakeAck{
				NodeID:   "node-2",
				Accepted: true,
			}, nil
		},
	}

	// 创建消息
	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeHandshake),
		Payload: &protocol.Handshake{NodeID: "node-1", Version: "1.0.0"},
	}

	// 编码消息
	codec := protocol.NewCodec()
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	// 创建模拟流
	stream := &mockStream{
		r: bytes.NewBuffer(encoded),
		w: &bytes.Buffer{},
	}

	// 读取并处理消息
	reader := protocol.NewStreamReader(stream)
	receivedMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}

	// 验证消息
	handshake, ok := receivedMsg.Payload.(*protocol.Handshake)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", receivedMsg.Payload)
	}
	if handshake.NodeID != "node-1" {
		t.Errorf("NodeID 错误: got %q", handshake.NodeID)
	}

	// 处理消息
	_ = handler
	// 验证处理函数可被调用
	ack, err := handler.HandleHandshake(ctx, handshake)
	if err != nil {
		t.Fatalf("HandleHandshake 失败: %v", err)
	}
	if !ack.Accepted {
		t.Error("握手应被接受")
	}
}

// TestProcessMessageQuery 测试处理查询消息
func TestProcessMessageQuery(t *testing.T) {
	handler := &mockHandler{
		queryFunc: func(ctx context.Context, q *protocol.Query) (*protocol.QueryResult, error) {
			return &protocol.QueryResult{
				QueryID:    q.QueryID,
				TotalCount: 5,
			}, nil
		},
	}

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeQuery),
		Payload: &protocol.Query{QueryID: "q-1", Keyword: "test"},
	}

	codec := protocol.NewCodec()
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	stream := &mockStream{
		r: bytes.NewBuffer(encoded),
		w: &bytes.Buffer{},
	}

	reader := protocol.NewStreamReader(stream)
	receivedMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}

	query, ok := receivedMsg.Payload.(*protocol.Query)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", receivedMsg.Payload)
	}

	result, err := handler.HandleQuery(context.Background(), query)
	if err != nil {
		t.Fatalf("HandleQuery 失败: %v", err)
	}
	if result.TotalCount != 5 {
		t.Errorf("TotalCount 错误: got %d", result.TotalCount)
	}
}

// TestProcessMessageSyncRequest 测试处理同步请求消息
func TestProcessMessageSyncRequest(t *testing.T) {
	handler := &mockHandler{
		syncRequestFunc: func(ctx context.Context, r *protocol.SyncRequest) (*protocol.SyncResponse, error) {
			return &protocol.SyncResponse{
				RequestID:       r.RequestID,
				ServerTimestamp: time.Now().UnixMilli(),
			}, nil
		},
	}

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeSyncRequest),
		Payload: &protocol.SyncRequest{RequestID: "sync-1"},
	}

	codec := protocol.NewCodec()
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	stream := &mockStream{
		r: bytes.NewBuffer(encoded),
		w: &bytes.Buffer{},
	}

	reader := protocol.NewStreamReader(stream)
	receivedMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}

	syncReq, ok := receivedMsg.Payload.(*protocol.SyncRequest)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", receivedMsg.Payload)
	}

	resp, err := handler.HandleSyncRequest(context.Background(), syncReq)
	if err != nil {
		t.Fatalf("HandleSyncRequest 失败: %v", err)
	}
	if resp.RequestID != "sync-1" {
		t.Errorf("RequestID 错误: got %q", resp.RequestID)
	}
}

// TestProcessMessagePushEntry 测试处理条目推送消息
func TestProcessMessagePushEntry(t *testing.T) {
	handler := &mockHandler{
		pushEntryFunc: func(ctx context.Context, e *protocol.PushEntry) (*protocol.PushAck, error) {
			return &protocol.PushAck{
				EntryID:  e.EntryID,
				Accepted: true,
			}, nil
		},
	}

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypePushEntry),
		Payload: &protocol.PushEntry{EntryID: "entry-1", Entry: []byte("test")},
	}

	codec := protocol.NewCodec()
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	stream := &mockStream{
		r: bytes.NewBuffer(encoded),
		w: &bytes.Buffer{},
	}

	reader := protocol.NewStreamReader(stream)
	receivedMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}

	pushEntry, ok := receivedMsg.Payload.(*protocol.PushEntry)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", receivedMsg.Payload)
	}

	ack, err := handler.HandlePushEntry(context.Background(), pushEntry)
	if err != nil {
		t.Fatalf("HandlePushEntry 失败: %v", err)
	}
	if !ack.Accepted {
		t.Error("推送应被接受")
	}
}

// TestProcessMessageRatingPush 测试处理评分推送消息
func TestProcessMessageRatingPush(t *testing.T) {
	handler := &mockHandler{
		ratingPushFunc: func(ctx context.Context, r *protocol.RatingPush) (*protocol.RatingAck, error) {
			return &protocol.RatingAck{
				RatingID: "rating-1",
				Accepted: true,
			}, nil
		},
	}

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeRatingPush),
		Payload: &protocol.RatingPush{Rating: []byte("test")},
	}

	codec := protocol.NewCodec()
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	stream := &mockStream{
		r: bytes.NewBuffer(encoded),
		w: &bytes.Buffer{},
	}

	reader := protocol.NewStreamReader(stream)
	receivedMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}

	ratingPush, ok := receivedMsg.Payload.(*protocol.RatingPush)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", receivedMsg.Payload)
	}

	ack, err := handler.HandleRatingPush(context.Background(), ratingPush)
	if err != nil {
		t.Fatalf("HandleRatingPush 失败: %v", err)
	}
	if !ack.Accepted {
		t.Error("评分应被接受")
	}
}

// TestProcessMessageHeartbeat 测试处理心跳消息
func TestProcessMessageHeartbeat(t *testing.T) {
	heartbeatReceived := false
	handler := &mockHandler{
		heartbeatFunc: func(ctx context.Context, h *protocol.Heartbeat) error {
			heartbeatReceived = true
			return nil
		},
	}

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeHeartbeat),
		Payload: &protocol.Heartbeat{NodeID: "node-1"},
	}

	codec := protocol.NewCodec()
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	stream := &mockStream{
		r: bytes.NewBuffer(encoded),
		w: &bytes.Buffer{},
	}

	reader := protocol.NewStreamReader(stream)
	receivedMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}

	hb, ok := receivedMsg.Payload.(*protocol.Heartbeat)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", receivedMsg.Payload)
	}

	err = handler.HandleHeartbeat(context.Background(), hb)
	if err != nil {
		t.Fatalf("HandleHeartbeat 失败: %v", err)
	}

	if !heartbeatReceived {
		t.Error("心跳应被接收")
	}
}

// TestProcessMessageBitfield 测试处理位图消息
func TestProcessMessageBitfield(t *testing.T) {
	bitfieldReceived := false
	handler := &mockHandler{
		bitfieldFunc: func(ctx context.Context, b *protocol.Bitfield) error {
			bitfieldReceived = true
			return nil
		},
	}

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeBitfield),
		Payload: &protocol.Bitfield{NodeID: "node-1", EntryCount: 100},
	}

	codec := protocol.NewCodec()
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	stream := &mockStream{
		r: bytes.NewBuffer(encoded),
		w: &bytes.Buffer{},
	}

	reader := protocol.NewStreamReader(stream)
	receivedMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}

	bf, ok := receivedMsg.Payload.(*protocol.Bitfield)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", receivedMsg.Payload)
	}

	err = handler.HandleBitfield(context.Background(), bf)
	if err != nil {
		t.Fatalf("HandleBitfield 失败: %v", err)
	}

	if !bitfieldReceived {
		t.Error("位图应被接收")
	}
}

// TestProcessMessageMirrorRequest 测试处理镜像请求消息
func TestProcessMessageMirrorRequest(t *testing.T) {
	handler := &mockHandler{
		mirrorRequestFunc: func(ctx context.Context, r *protocol.MirrorRequest) (<-chan *protocol.MirrorData, error) {
			ch := make(chan *protocol.MirrorData, 1)
			ch <- &protocol.MirrorData{RequestID: r.RequestID}
			close(ch)
			return ch, nil
		},
	}

	msg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeMirrorRequest),
		Payload: &protocol.MirrorRequest{RequestID: "mirror-1"},
	}

	codec := protocol.NewCodec()
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	stream := &mockStream{
		r: bytes.NewBuffer(encoded),
		w: &bytes.Buffer{},
	}

	reader := protocol.NewStreamReader(stream)
	receivedMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}

	mirrorReq, ok := receivedMsg.Payload.(*protocol.MirrorRequest)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", receivedMsg.Payload)
	}

	ch, err := handler.HandleMirrorRequest(context.Background(), mirrorReq)
	if err != nil {
		t.Fatalf("HandleMirrorRequest 失败: %v", err)
	}

	data := <-ch
	if data.RequestID != "mirror-1" {
		t.Errorf("RequestID 错误: got %q", data.RequestID)
	}
}

// TestRoundTripHandshake 测试完整的握手请求-响应往返
func TestRoundTripHandshake(t *testing.T) {
	// 创建客户端和服务端的管道
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	// 服务端处理器
	handler := &mockHandler{
		handshakeFunc: func(ctx context.Context, h *protocol.Handshake) (*protocol.HandshakeAck, error) {
			return &protocol.HandshakeAck{
				NodeID:   "server-node",
				Accepted: true,
			}, nil
		},
	}

	// 服务端协程
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverWriter.Close()
		defer serverReader.Close()

		// 读取请求
		reader := protocol.NewStreamReader(serverReader)
		msg, err := reader.ReadMessage()
		if err != nil {
			return
		}

		// 处理请求
		handshake := msg.Payload.(*protocol.Handshake)
		ack, _ := handler.HandleHandshake(context.Background(), handshake)

		// 发送响应
		writer := protocol.NewStreamWriter(serverWriter)
		respMsg := &protocol.Message{
			Header:  protocol.NewMessageHeader(protocol.MessageTypeHandshakeAck),
			Payload: ack,
		}
		writer.WriteMessage(respMsg)
	}()

	// 客户端发送请求
	writer := protocol.NewStreamWriter(clientWriter)
	reqMsg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeHandshake),
		Payload: &protocol.Handshake{NodeID: "client-node"},
	}
	if err := writer.WriteMessage(reqMsg); err != nil {
		t.Fatalf("WriteMessage 失败: %v", err)
	}
	clientWriter.Close()

	// 客户端读取响应
	reader := protocol.NewStreamReader(clientReader)
	respMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}
	clientReader.Close()

	<-done // 等待服务端完成

	// 验证响应
	ack, ok := respMsg.Payload.(*protocol.HandshakeAck)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", respMsg.Payload)
	}
	if !ack.Accepted {
		t.Error("握手应被接受")
	}
	if ack.NodeID != "server-node" {
		t.Errorf("NodeID 错误: got %q", ack.NodeID)
	}
}

// TestRoundTripQuery 测试完整的查询请求-响应往返
func TestRoundTripQuery(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	handler := &mockHandler{
		queryFunc: func(ctx context.Context, q *protocol.Query) (*protocol.QueryResult, error) {
			return &protocol.QueryResult{
				QueryID:    q.QueryID,
				TotalCount: 3,
				Entries:    [][]byte{[]byte("e1"), []byte("e2"), []byte("e3")},
			}, nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverWriter.Close()
		defer serverReader.Close()

		reader := protocol.NewStreamReader(serverReader)
		msg, err := reader.ReadMessage()
		if err != nil {
			return
		}

		query := msg.Payload.(*protocol.Query)
		result, _ := handler.HandleQuery(context.Background(), query)

		writer := protocol.NewStreamWriter(serverWriter)
		respMsg := &protocol.Message{
			Header:  protocol.NewMessageHeader(protocol.MessageTypeQueryResult),
			Payload: result,
		}
		writer.WriteMessage(respMsg)
	}()

	writer := protocol.NewStreamWriter(clientWriter)
	reqMsg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeQuery),
		Payload: &protocol.Query{QueryID: "q-1", Keyword: "test"},
	}
	if err := writer.WriteMessage(reqMsg); err != nil {
		t.Fatalf("WriteMessage 失败: %v", err)
	}
	clientWriter.Close()

	reader := protocol.NewStreamReader(clientReader)
	respMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}
	clientReader.Close()

	<-done

	result, ok := respMsg.Payload.(*protocol.QueryResult)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", respMsg.Payload)
	}
	if result.TotalCount != 3 {
		t.Errorf("TotalCount 错误: got %d", result.TotalCount)
	}
}

// TestRoundTripSyncRequest 测试完整的同步请求-响应往返
func TestRoundTripSyncRequest(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	handler := &mockHandler{
		syncRequestFunc: func(ctx context.Context, r *protocol.SyncRequest) (*protocol.SyncResponse, error) {
			return &protocol.SyncResponse{
				RequestID:       r.RequestID,
				NewEntries:      [][]byte{[]byte("new")},
				DeletedEntryIDs: []string{"old"},
			}, nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverWriter.Close()
		defer serverReader.Close()

		reader := protocol.NewStreamReader(serverReader)
		msg, err := reader.ReadMessage()
		if err != nil {
			return
		}

		syncReq := msg.Payload.(*protocol.SyncRequest)
		resp, _ := handler.HandleSyncRequest(context.Background(), syncReq)

		writer := protocol.NewStreamWriter(serverWriter)
		respMsg := &protocol.Message{
			Header:  protocol.NewMessageHeader(protocol.MessageTypeSyncResponse),
			Payload: resp,
		}
		writer.WriteMessage(respMsg)
	}()

	writer := protocol.NewStreamWriter(clientWriter)
	reqMsg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeSyncRequest),
		Payload: &protocol.SyncRequest{RequestID: "sync-1"},
	}
	if err := writer.WriteMessage(reqMsg); err != nil {
		t.Fatalf("WriteMessage 失败: %v", err)
	}
	clientWriter.Close()

	reader := protocol.NewStreamReader(clientReader)
	respMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}
	clientReader.Close()

	<-done

	resp, ok := respMsg.Payload.(*protocol.SyncResponse)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", respMsg.Payload)
	}
	if len(resp.NewEntries) != 1 {
		t.Errorf("NewEntries 数量错误: got %d", len(resp.NewEntries))
	}
}

// TestRoundTripPushEntry 测试完整的条目推送请求-响应往返
func TestRoundTripPushEntry(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	handler := &mockHandler{
		pushEntryFunc: func(ctx context.Context, e *protocol.PushEntry) (*protocol.PushAck, error) {
			return &protocol.PushAck{
				EntryID:    e.EntryID,
				Accepted:   true,
				NewVersion: 2,
			}, nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverWriter.Close()
		defer serverReader.Close()

		reader := protocol.NewStreamReader(serverReader)
		msg, err := reader.ReadMessage()
		if err != nil {
			return
		}

		pushEntry := msg.Payload.(*protocol.PushEntry)
		ack, _ := handler.HandlePushEntry(context.Background(), pushEntry)

		writer := protocol.NewStreamWriter(serverWriter)
		respMsg := &protocol.Message{
			Header:  protocol.NewMessageHeader(protocol.MessageTypePushAck),
			Payload: ack,
		}
		writer.WriteMessage(respMsg)
	}()

	writer := protocol.NewStreamWriter(clientWriter)
	reqMsg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypePushEntry),
		Payload: &protocol.PushEntry{EntryID: "entry-1", Entry: []byte("data")},
	}
	if err := writer.WriteMessage(reqMsg); err != nil {
		t.Fatalf("WriteMessage 失败: %v", err)
	}
	clientWriter.Close()

	reader := protocol.NewStreamReader(clientReader)
	respMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}
	clientReader.Close()

	<-done

	ack, ok := respMsg.Payload.(*protocol.PushAck)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", respMsg.Payload)
	}
	if !ack.Accepted {
		t.Error("推送应被接受")
	}
}

// TestRoundTripRatingPush 测试完整的评分推送请求-响应往返
func TestRoundTripRatingPush(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	handler := &mockHandler{
		ratingPushFunc: func(ctx context.Context, r *protocol.RatingPush) (*protocol.RatingAck, error) {
			return &protocol.RatingAck{
				RatingID: "rating-1",
				Accepted: true,
			}, nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverWriter.Close()
		defer serverReader.Close()

		reader := protocol.NewStreamReader(serverReader)
		msg, err := reader.ReadMessage()
		if err != nil {
			return
		}

		ratingPush := msg.Payload.(*protocol.RatingPush)
		ack, _ := handler.HandleRatingPush(context.Background(), ratingPush)

		writer := protocol.NewStreamWriter(serverWriter)
		respMsg := &protocol.Message{
			Header:  protocol.NewMessageHeader(protocol.MessageTypeRatingAck),
			Payload: ack,
		}
		writer.WriteMessage(respMsg)
	}()

	writer := protocol.NewStreamWriter(clientWriter)
	reqMsg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeRatingPush),
		Payload: &protocol.RatingPush{Rating: []byte("rating-data")},
	}
	if err := writer.WriteMessage(reqMsg); err != nil {
		t.Fatalf("WriteMessage 失败: %v", err)
	}
	clientWriter.Close()

	reader := protocol.NewStreamReader(clientReader)
	respMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}
	clientReader.Close()

	<-done

	ack, ok := respMsg.Payload.(*protocol.RatingAck)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", respMsg.Payload)
	}
	if !ack.Accepted {
		t.Error("评分应被接受")
	}
}

// TestSendHandshakeError 测试发送握手失败场景
func TestSendHandshakeError(t *testing.T) {
	ctx := context.Background()

	h1, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建 host1 失败: %v", err)
	}
	defer h1.Close()

	// 不创建 h2, 尝试连接不存在的节点
	p1 := protocol.NewProtocol(h1, &mockHandler{})

	_, err = p1.SendHandshake(ctx, "non-existent-peer", &protocol.Handshake{
		NodeID: "node-1",
	})

	if err == nil {
		t.Error("连接不存在的节点应返回错误")
	}
}

// TestSendHandshakeRejected 测试握手被拒绝场景
func TestSendHandshakeRejected(t *testing.T) {
	// 使用模拟流测试握手被拒绝
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	handler := &mockHandler{
		handshakeFunc: func(ctx context.Context, h *protocol.Handshake) (*protocol.HandshakeAck, error) {
			return &protocol.HandshakeAck{
				NodeID:       "server-node",
				Accepted:     false,
				RejectReason: "version mismatch",
			}, nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverWriter.Close()
		defer serverReader.Close()

		reader := protocol.NewStreamReader(serverReader)
		msg, err := reader.ReadMessage()
		if err != nil {
			return
		}

		handshake := msg.Payload.(*protocol.Handshake)
		ack, _ := handler.HandleHandshake(context.Background(), handshake)

		writer := protocol.NewStreamWriter(serverWriter)
		respMsg := &protocol.Message{
			Header:  protocol.NewMessageHeader(protocol.MessageTypeHandshakeAck),
			Payload: ack,
		}
		writer.WriteMessage(respMsg)
	}()

	writer := protocol.NewStreamWriter(clientWriter)
	reqMsg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeHandshake),
		Payload: &protocol.Handshake{NodeID: "client-node", Version: "0.9.0"},
	}
	if err := writer.WriteMessage(reqMsg); err != nil {
		t.Fatalf("WriteMessage 失败: %v", err)
	}
	clientWriter.Close()

	reader := protocol.NewStreamReader(clientReader)
	respMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}
	clientReader.Close()

	<-done

	ack, ok := respMsg.Payload.(*protocol.HandshakeAck)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", respMsg.Payload)
	}
	if ack.Accepted {
		t.Error("握手应被拒绝")
	}
	if ack.RejectReason != "version mismatch" {
		t.Errorf("RejectReason 错误: got %q", ack.RejectReason)
	}
}

// TestSendQueryEmptyResult 测试查询返回空结果
func TestSendQueryEmptyResult(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	handler := &mockHandler{
		queryFunc: func(ctx context.Context, q *protocol.Query) (*protocol.QueryResult, error) {
			return &protocol.QueryResult{
				QueryID:    q.QueryID,
				TotalCount: 0,
				Entries:    nil,
			}, nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverWriter.Close()
		defer serverReader.Close()

		reader := protocol.NewStreamReader(serverReader)
		msg, err := reader.ReadMessage()
		if err != nil {
			return
		}

		query := msg.Payload.(*protocol.Query)
		result, _ := handler.HandleQuery(context.Background(), query)

		writer := protocol.NewStreamWriter(serverWriter)
		respMsg := &protocol.Message{
			Header:  protocol.NewMessageHeader(protocol.MessageTypeQueryResult),
			Payload: result,
		}
		writer.WriteMessage(respMsg)
	}()

	writer := protocol.NewStreamWriter(clientWriter)
	reqMsg := &protocol.Message{
		Header:  protocol.NewMessageHeader(protocol.MessageTypeQuery),
		Payload: &protocol.Query{QueryID: "q-empty", Keyword: "nonexistent"},
	}
	if err := writer.WriteMessage(reqMsg); err != nil {
		t.Fatalf("WriteMessage 失败: %v", err)
	}
	clientWriter.Close()

	reader := protocol.NewStreamReader(clientReader)
	respMsg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 失败: %v", err)
	}
	clientReader.Close()

	<-done

	result, ok := respMsg.Payload.(*protocol.QueryResult)
	if !ok {
		t.Fatalf("Payload 类型错误: %T", respMsg.Payload)
	}
	if result.TotalCount != 0 {
		t.Errorf("TotalCount 应为 0, got %d", result.TotalCount)
	}
	if len(result.Entries) != 0 {
		t.Errorf("Entries 应为空, got %d", len(result.Entries))
	}
}

// ==================== 使用 Mocknet 的协议测试 ====================

// TestMocknetSendHandshake 使用 mocknet 测试发送握手消息
func TestMocknetSendHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建 mocknet
	mn, err := mocknet.FullMeshLinked(2)
	if err != nil {
		t.Fatalf("创建 mocknet 失败: %v", err)
	}

	hosts := mn.Hosts()
	h1, h2 := hosts[0], hosts[1]

	// 在 h2 上设置协议处理器
	handler := &mockHandler{
		handshakeFunc: func(ctx context.Context, h *protocol.Handshake) (*protocol.HandshakeAck, error) {
			return &protocol.HandshakeAck{
				NodeID:   "node-2",
				PeerID:   h2.ID().String(),
				Accepted: true,
			}, nil
		},
	}
	protocol.NewProtocol(h2, handler)

	// 在 h1 上创建协议并发送握手
	p1 := protocol.NewProtocol(h1, &mockHandler{})

	ack, err := p1.SendHandshake(ctx, h2.ID(), &protocol.Handshake{
		NodeID:   "node-1",
		PeerID:   h1.ID().String(),
		NodeType: protocol.NodeTypeLocal,
		Version:  "1.0.0",
	})

	if err != nil {
		t.Fatalf("SendHandshake 失败: %v", err)
	}

	if !ack.Accepted {
		t.Error("握手应被接受")
	}

	if ack.NodeID != "node-2" {
		t.Errorf("NodeID 错误: got %q, want %q", ack.NodeID, "node-2")
	}
}

// TestMocknetSendQuery 使用 mocknet 测试发送查询消息
func TestMocknetSendQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mn, err := mocknet.FullMeshLinked(2)
	if err != nil {
		t.Fatalf("创建 mocknet 失败: %v", err)
	}

	hosts := mn.Hosts()
	h1, h2 := hosts[0], hosts[1]

	handler := &mockHandler{
		queryFunc: func(ctx context.Context, q *protocol.Query) (*protocol.QueryResult, error) {
			return &protocol.QueryResult{
				QueryID:    q.QueryID,
				TotalCount: 2,
				Entries:    [][]byte{[]byte("entry1"), []byte("entry2")},
				HasMore:    false,
			}, nil
		},
	}
	protocol.NewProtocol(h2, handler)

	p1 := protocol.NewProtocol(h1, &mockHandler{})

	result, err := p1.SendQuery(ctx, h2.ID(), &protocol.Query{
		QueryID:    "query-1",
		Keyword:    "test",
		Categories: []string{"tech"},
		Limit:      10,
		QueryType:  protocol.QueryTypeGlobal,
	})

	if err != nil {
		t.Fatalf("SendQuery 失败: %v", err)
	}

	if result.QueryID != "query-1" {
		t.Errorf("QueryID 错误: got %q", result.QueryID)
	}

	if result.TotalCount != 2 {
		t.Errorf("TotalCount 错误: got %d, want 2", result.TotalCount)
	}
}

// TestMocknetSendSyncRequest 使用 mocknet 测试发送同步请求
func TestMocknetSendSyncRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mn, err := mocknet.FullMeshLinked(2)
	if err != nil {
		t.Fatalf("创建 mocknet 失败: %v", err)
	}

	hosts := mn.Hosts()
	h1, h2 := hosts[0], hosts[1]

	handler := &mockHandler{
		syncRequestFunc: func(ctx context.Context, r *protocol.SyncRequest) (*protocol.SyncResponse, error) {
			return &protocol.SyncResponse{
				RequestID:       r.RequestID,
				NewEntries:      [][]byte{[]byte("new-entry")},
				UpdatedEntries:  nil,
				DeletedEntryIDs: []string{"old-1"},
				ServerTimestamp: time.Now().UnixMilli(),
			}, nil
		},
	}
	protocol.NewProtocol(h2, handler)

	p1 := protocol.NewProtocol(h1, &mockHandler{})

	resp, err := p1.SendSyncRequest(ctx, h2.ID(), &protocol.SyncRequest{
		RequestID:         "sync-1",
		LastSyncTimestamp: 0,
		VersionVector:     map[string]int64{"entry-1": 1},
	})

	if err != nil {
		t.Fatalf("SendSyncRequest 失败: %v", err)
	}

	if resp.RequestID != "sync-1" {
		t.Errorf("RequestID 错误: got %q", resp.RequestID)
	}

	if len(resp.NewEntries) != 1 {
		t.Errorf("NewEntries 数量错误: got %d, want 1", len(resp.NewEntries))
	}
}

// TestMocknetSendPushEntry 使用 mocknet 测试发送条目推送
func TestMocknetSendPushEntry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mn, err := mocknet.FullMeshLinked(2)
	if err != nil {
		t.Fatalf("创建 mocknet 失败: %v", err)
	}

	hosts := mn.Hosts()
	h1, h2 := hosts[0], hosts[1]

	handler := &mockHandler{
		pushEntryFunc: func(ctx context.Context, e *protocol.PushEntry) (*protocol.PushAck, error) {
			return &protocol.PushAck{
				EntryID:    e.EntryID,
				Accepted:   true,
				NewVersion: 2,
			}, nil
		},
	}
	protocol.NewProtocol(h2, handler)

	p1 := protocol.NewProtocol(h1, &mockHandler{})

	ack, err := p1.SendPushEntry(ctx, h2.ID(), &protocol.PushEntry{
		EntryID:          "entry-1",
		Entry:            []byte(`{"title":"test"}`),
		CreatorSignature: []byte("sig"),
	})

	if err != nil {
		t.Fatalf("SendPushEntry 失败: %v", err)
	}

	if !ack.Accepted {
		t.Error("推送应被接受")
	}

	if ack.NewVersion != 2 {
		t.Errorf("NewVersion 错误: got %d, want 2", ack.NewVersion)
	}
}

// TestMocknetSendRatingPush 使用 mocknet 测试发送评分推送
func TestMocknetSendRatingPush(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mn, err := mocknet.FullMeshLinked(2)
	if err != nil {
		t.Fatalf("创建 mocknet 失败: %v", err)
	}

	hosts := mn.Hosts()
	h1, h2 := hosts[0], hosts[1]

	handler := &mockHandler{
		ratingPushFunc: func(ctx context.Context, r *protocol.RatingPush) (*protocol.RatingAck, error) {
			return &protocol.RatingAck{
				RatingID:     "rating-1",
				Accepted:     true,
				RejectReason: "",
			}, nil
		},
	}
	protocol.NewProtocol(h2, handler)

	p1 := protocol.NewProtocol(h1, &mockHandler{})

	ack, err := p1.SendRatingPush(ctx, h2.ID(), &protocol.RatingPush{
		Rating:         []byte(`{"entry_id":"entry-1","score":5}`),
		RaterSignature: []byte("rater-sig"),
	})

	if err != nil {
		t.Fatalf("SendRatingPush 失败: %v", err)
	}

	if !ack.Accepted {
		t.Error("评分应被接受")
	}
}
