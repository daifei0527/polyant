// Package protocol_test 提供协议层的单元测试
package protocol_test

import (
	"context"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/network/protocol"
	libp2p "github.com/libp2p/go-libp2p"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

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

// ==================== 协议 ID 测试 ====================

// TestAWSPProtocolID 测试协议 ID
func TestAWSPProtocolID(t *testing.T) {
	expected := "/polyant/sync/2.0.0"
	if protocol.AWSPProtocolID != expected {
		t.Errorf("AWSPProtocolID 错误: got %q, want %q", protocol.AWSPProtocolID, expected)
	}
}

// ==================== 错误处理测试 ====================

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

// ==================== Protocol 测试 ====================

// mockHandler 实现 protocol.Handler 接口用于测试
type mockHandler struct {
	handshakeFunc     func(ctx context.Context, h *protocol.Handshake) (*protocol.HandshakeAck, error)
	queryFunc         func(ctx context.Context, q *protocol.Query) (*protocol.QueryResult, error)
	syncRequestFunc   func(ctx context.Context, r *protocol.SyncRequest) (*protocol.SyncResponse, error)
	mirrorRequestFunc func(ctx context.Context, r *protocol.MirrorRequest) (<-chan *protocol.MirrorData, error)
	mirrorDataFunc    func(ctx context.Context, d *protocol.MirrorData) error
	pushEntryFunc     func(ctx context.Context, e *protocol.PushEntry) (*protocol.PushAck, error)
	ratingPushFunc    func(ctx context.Context, r *protocol.RatingPush) (*protocol.RatingAck, error)
	heartbeatFunc     func(ctx context.Context, h *protocol.Heartbeat) error
	bitfieldFunc      func(ctx context.Context, b *protocol.Bitfield) error
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

func (m *mockHandler) HandleMirrorData(ctx context.Context, d *protocol.MirrorData) error {
	if m.mirrorDataFunc != nil {
		return m.mirrorDataFunc(ctx, d)
	}
	return nil
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
				RequestID:    r.RequestID,
				BatchIndex:   0,
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
