// Package protocol_test 提供协议层的单元测试
package protocol_test

import (
	"bytes"
	"testing"

	"github.com/daifei0527/agentwiki/internal/network/protocol"
)

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
	expected := "/agentwiki/sync/1.0.0"
	if protocol.AWSPProtocolID != expected {
		t.Errorf("AWSPProtocolID 错误: got %q, want %q", protocol.AWSPProtocolID, expected)
	}
}
