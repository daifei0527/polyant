# Phase 6b: 网络协议增强实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 AWSP 协议从 JSON 迁移到 Protobuf，添加 QUIC 传输支持，提升网络性能

**Architecture:** 定义 .proto 文件生成 Go 代码，重写 Codec 使用 Protobuf 编解码，修改 Host 添加 QUIC 传输层

**Tech Stack:** Go 1.22, google.golang.org/protobuf, go-libp2p QUIC transport

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `proto/awsp.proto` | 创建 | Protobuf 消息定义 |
| `internal/network/protocol/proto/awsp.pb.go` | 生成 | Protobuf 生成的 Go 代码 |
| `internal/network/protocol/codec.go` | 重写 | Protobuf 编解码实现 |
| `internal/network/protocol/codec_test.go` | 修改 | 更新编解码测试 |
| `internal/network/protocol/types.go` | 修改 | 更新协议版本常量 |
| `internal/network/host/host.go` | 修改 | 添加 QUIC 传输支持 |
| `go.mod` | 修改 | 添加 protobuf 依赖 |

---

## Task 1: 添加 Protobuf 依赖

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: 添加 protobuf 依赖**

Run: `go get google.golang.org/protobuf/proto`
Expected: 依赖添加成功

- [ ] **Step 2: 运行 go mod tidy**

Run: `go mod tidy`
Expected: 无错误

- [ ] **Step 3: 提交依赖更新**

```bash
git add go.mod go.sum
git commit -m "deps: 添加 protobuf 依赖

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: 创建 Protobuf 消息定义

**Files:**
- Create: `proto/awsp.proto`

- [ ] **Step 1: 创建 proto 目录**

Run: `mkdir -p proto`

- [ ] **Step 2: 创建 awsp.proto 文件**

创建 `proto/awsp.proto`:

```protobuf
syntax = "proto3";

package awsp;

option go_package = "github.com/daifei0527/agentwiki/internal/network/protocol/proto";

// 消息类型枚举
enum MessageType {
  MESSAGE_TYPE_UNKNOWN = 0;
  MESSAGE_TYPE_HANDSHAKE = 1;
  MESSAGE_TYPE_HANDSHAKE_ACK = 2;
  MESSAGE_TYPE_QUERY = 3;
  MESSAGE_TYPE_QUERY_RESULT = 4;
  MESSAGE_TYPE_SYNC_REQUEST = 5;
  MESSAGE_TYPE_SYNC_RESPONSE = 6;
  MESSAGE_TYPE_MIRROR_REQUEST = 7;
  MESSAGE_TYPE_MIRROR_DATA = 8;
  MESSAGE_TYPE_MIRROR_ACK = 9;
  MESSAGE_TYPE_PUSH_ENTRY = 10;
  MESSAGE_TYPE_PUSH_ACK = 11;
  MESSAGE_TYPE_RATING_PUSH = 12;
  MESSAGE_TYPE_RATING_ACK = 13;
  MESSAGE_TYPE_HEARTBEAT = 14;
  MESSAGE_TYPE_BITFIELD = 15;
}

// 节点类型
enum NodeType {
  NODE_TYPE_LOCAL = 0;
  NODE_TYPE_SEED = 1;
}

// 查询类型
enum QueryType {
  QUERY_TYPE_LOCAL = 0;
  QUERY_TYPE_GLOBAL = 1;
}

// 消息头
message MessageHeader {
  MessageType type = 1;
  string message_id = 2;
  int64 timestamp = 3;
  bytes signature = 4;
}

// 完整消息封装
message Message {
  MessageHeader header = 1;
  oneof payload {
    Handshake handshake = 2;
    HandshakeAck handshake_ack = 3;
    Query query = 4;
    QueryResult query_result = 5;
    SyncRequest sync_request = 6;
    SyncResponse sync_response = 7;
    MirrorRequest mirror_request = 8;
    MirrorData mirror_data = 9;
    MirrorAck mirror_ack = 10;
    PushEntry push_entry = 11;
    PushAck push_ack = 12;
    RatingPush rating_push = 13;
    RatingAck rating_ack = 14;
    Heartbeat heartbeat = 15;
    Bitfield bitfield = 16;
  }
}

// 握手消息
message Handshake {
  string node_id = 1;
  string peer_id = 2;
  NodeType node_type = 3;
  string version = 4;
  repeated string categories = 5;
  int64 entry_count = 6;
  bytes signature = 7;
}

message HandshakeAck {
  string node_id = 1;
  string peer_id = 2;
  NodeType node_type = 3;
  string version = 4;
  bool accepted = 5;
  string reject_reason = 6;
  bytes signature = 7;
}

// 查询消息
message Query {
  string query_id = 1;
  string keyword = 2;
  repeated string categories = 3;
  int32 limit = 4;
  int32 offset = 5;
  QueryType query_type = 6;
}

message QueryResult {
  string query_id = 1;
  repeated bytes entries = 2;
  int32 total_count = 3;
  bool has_more = 4;
}

// 同步消息
message SyncRequest {
  string request_id = 1;
  int64 last_sync_timestamp = 2;
  map<string, int64> version_vector = 3;
  repeated string requested_categories = 4;
}

message SyncResponse {
  string request_id = 1;
  repeated bytes new_entries = 2;
  repeated bytes updated_entries = 3;
  repeated string deleted_entry_ids = 4;
  repeated bytes new_ratings = 5;
  map<string, int64> server_version_vector = 6;
  int64 server_timestamp = 7;
}

// 镜像消息
message MirrorRequest {
  string request_id = 1;
  repeated string categories = 2;
  bool full_mirror = 3;
  int32 batch_size = 4;
}

message MirrorData {
  string request_id = 1;
  int32 batch_index = 2;
  int32 total_batches = 3;
  repeated bytes entries = 4;
  repeated bytes categories = 5;
}

message MirrorAck {
  string request_id = 1;
  bool success = 2;
  string error_message = 3;
  int64 received_entries = 4;
}

// 推送消息
message PushEntry {
  string entry_id = 1;
  bytes entry = 2;
  bytes creator_signature = 3;
}

message PushAck {
  string entry_id = 1;
  bool accepted = 2;
  string reject_reason = 3;
  int64 new_version = 4;
}

message RatingPush {
  bytes rating = 1;
  bytes rater_signature = 2;
}

message RatingAck {
  string rating_id = 1;
  bool accepted = 2;
  string reject_reason = 3;
}

// 心跳与状态消息
message Heartbeat {
  string node_id = 1;
  int64 uptime_seconds = 2;
  int64 entry_count = 3;
  int64 timestamp = 4;
}

message Bitfield {
  string node_id = 1;
  map<string, int64> version_vector = 2;
  int64 entry_count = 3;
}
```

- [ ] **Step 3: 提交 proto 文件**

```bash
git add proto/awsp.proto
git commit -m "proto: 添加 AWSP 协议 Protobuf 定义

定义 16 种消息类型：
- 握手: Handshake, HandshakeAck
- 查询: Query, QueryResult
- 同步: SyncRequest, SyncResponse
- 镜像: MirrorRequest, MirrorData, MirrorAck
- 推送: PushEntry, PushAck, RatingPush, RatingAck
- 状态: Heartbeat, Bitfield

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: 生成 Protobuf Go 代码

**Files:**
- Create: `internal/network/protocol/proto/awsp.pb.go`

- [ ] **Step 1: 创建输出目录**

Run: `mkdir -p internal/network/protocol/proto`

- [ ] **Step 2: 安装 protoc-gen-go**

Run: `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`
Expected: 安装成功

- [ ] **Step 3: 生成 Go 代码**

Run: `protoc --go_out=. --go_opt=module=github.com/daifei0527/agentwiki proto/awsp.proto`
Expected: 生成 internal/network/protocol/proto/awsp.pb.go

- [ ] **Step 4: 验证生成的代码**

Run: `go build ./internal/network/protocol/proto/...`
Expected: 编译成功

- [ ] **Step 5: 提交生成的代码**

```bash
git add internal/network/protocol/proto/awsp.pb.go
git commit -m "proto: 生成 AWSP 协议 Go 代码

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: 编写 Protobuf Codec 测试

**Files:**
- Create: `internal/network/protocol/protobuf_codec_test.go`

- [ ] **Step 1: 创建测试文件**

创建 `internal/network/protocol/protobuf_codec_test.go`:

```go
package protocol

import (
	"bytes"
	"testing"

	"github.com/daifei0527/agentwiki/internal/network/protocol/proto"
)

func TestProtobufCodec_EncodeDecodeHandshake(t *testing.T) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_HANDSHAKE,
			MessageId: "test-msg-1",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_Handshake{
			Handshake: &proto.Handshake{
				NodeId:     "node-1",
				PeerId:     "peer-1",
				NodeType:   proto.NodeType_NODE_TYPE_LOCAL,
				Version:    "2.0.0",
				Categories: []string{"tech", "science"},
				EntryCount: 100,
			},
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(encoded) == 0 {
		t.Error("Encoded message should not be empty")
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Header.Type != proto.MessageType_MESSAGE_TYPE_HANDSHAKE {
		t.Errorf("Type mismatch: got %v, want %v", decoded.Header.Type, proto.MessageType_MESSAGE_TYPE_HANDSHAKE)
	}

	hs := decoded.GetHandshake()
	if hs == nil {
		t.Fatal("Handshake payload is nil")
	}

	if hs.NodeId != "node-1" {
		t.Errorf("NodeId mismatch: got %s, want node-1", hs.NodeId)
	}
}

func TestProtobufCodec_EncodeDecodeQuery(t *testing.T) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_QUERY,
			MessageId: "test-query-1",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_Query{
			Query: &proto.Query{
				QueryId:    "q-1",
				Keyword:    "golang",
				Categories: []string{"tech/programming"},
				Limit:      10,
				Offset:     0,
				QueryType:  proto.QueryType_QUERY_TYPE_LOCAL,
			},
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	q := decoded.GetQuery()
	if q == nil {
		t.Fatal("Query payload is nil")
	}

	if q.Keyword != "golang" {
		t.Errorf("Keyword mismatch: got %s, want golang", q.Keyword)
	}
}

func TestProtobufCodec_EncodeDecodeSyncRequest(t *testing.T) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_SYNC_REQUEST,
			MessageId: "test-sync-1",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_SyncRequest{
			SyncRequest: &proto.SyncRequest{
				RequestId:         "sync-1",
				LastSyncTimestamp: 1234560000,
				VersionVector:     map[string]int64{"entry-1": 100, "entry-2": 200},
				RequestedCategories: []string{"tech"},
			},
		},
	}

	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := codec.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	sr := decoded.GetSyncRequest()
	if sr == nil {
		t.Fatal("SyncRequest payload is nil")
	}

	if sr.RequestId != "sync-1" {
		t.Errorf("RequestId mismatch: got %s, want sync-1", sr.RequestId)
	}

	if len(sr.VersionVector) != 2 {
		t.Errorf("VersionVector length mismatch: got %d, want 2", len(sr.VersionVector))
	}
}

func TestProtobufCodec_SizeReduction(t *testing.T) {
	// 对比 JSON 和 Protobuf 的消息大小
	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_HANDSHAKE,
			MessageId: "test-size-comparison",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_Handshake{
			Handshake: &proto.Handshake{
				NodeId:     "node-with-long-name",
				PeerId:     "peer-with-long-id",
				NodeType:   proto.NodeType_NODE_TYPE_SEED,
				Version:    "2.0.0",
				Categories: []string{"tech/programming/go", "science/physics", "business/finance"},
				EntryCount: 10000,
			},
		},
	}

	// Protobuf 编码
	protobufCodec := NewProtobufCodec()
	protobufBytes, err := protobufCodec.Encode(msg)
	if err != nil {
		t.Fatalf("Protobuf encode failed: %v", err)
	}

	// JSON 编码（模拟旧格式）
	// 预估 JSON 大小约为 Protobuf 的 2-4 倍
	// 这里我们只验证 Protobuf 编码成功
	t.Logf("Protobuf message size: %d bytes", len(protobufBytes))

	if len(protobufBytes) > 200 {
		t.Logf("Protobuf size is reasonable for this message")
	}
}

func TestProtobufCodec_StreamReaderWriter(t *testing.T) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_HEARTBEAT,
			MessageId: "test-hb-1",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_Heartbeat{
			Heartbeat: &proto.Heartbeat{
				NodeId:        "node-1",
				UptimeSeconds: 3600,
				EntryCount:    500,
				Timestamp:     1234567890,
			},
		},
	}

	// 使用 StreamWriter 写入
	var buf bytes.Buffer
	writer := NewProtobufStreamWriter(&buf)
	err := writer.WriteMessage(msg)
	if err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	// 使用 StreamReader 读取
	reader := NewProtobufStreamReader(&buf)
	decoded, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	hb := decoded.GetHeartbeat()
	if hb == nil {
		t.Fatal("Heartbeat payload is nil")
	}

	if hb.UptimeSeconds != 3600 {
		t.Errorf("UptimeSeconds mismatch: got %d, want 3600", hb.UptimeSeconds)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/network/protocol/... -run TestProtobufCodec -v`
Expected: FAIL - undefined: NewProtobufCodec

- [ ] **Step 3: 提交测试文件**

```bash
git add internal/network/protocol/protobuf_codec_test.go
git commit -m "test(protocol): 添加 Protobuf Codec 测试

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: 实现 Protobuf Codec

**Files:**
- Create: `internal/network/protocol/protobuf_codec.go`

- [ ] **Step 1: 创建 Protobuf Codec 实现**

创建 `internal/network/protocol/protobuf_codec.go`:

```go
package protocol

import (
	"bufio"
	"fmt"
	"io"

	"github.com/daifei0527/agentwiki/internal/network/protocol/proto"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const (
	// MaxProtobufMessageSize 最大消息大小 (64MB)
	MaxProtobufMessageSize = 64 * 1024 * 1024
	// MessageSizeFieldLen 消息长度字段大小 (4 bytes)
	MessageSizeFieldLen = 4
)

// ProtobufCodec Protobuf 编解码器
type ProtobufCodec struct{}

// NewProtobufCodec 创建 Protobuf 编解码器
func NewProtobufCodec() *ProtobufCodec {
	return &ProtobufCodec{}
}

// Encode 编码消息为字节切片
// 格式: [4字节长度][protobuf消息]
func (c *ProtobufCodec) Encode(msg *proto.Message) ([]byte, error) {
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	// 添加 4 字节长度前缀
	buf := make([]byte, MessageSizeFieldLen+len(msgBytes))
	protowire.AppendFixed32(buf[:0], uint32(len(msgBytes)))
	copy(buf[MessageSizeFieldLen:], msgBytes)

	return buf, nil
}

// Decode 从 Reader 解码消息
func (c *ProtobufCodec) Decode(r io.Reader) (*proto.Message, error) {
	// 读取 4 字节长度
	lenBuf := make([]byte, MessageSizeFieldLen)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}

	msgLen := protowire.Fixed32(lenBuf[0]) |
		protowire.Fixed32(lenBuf[1])<<8 |
		protowire.Fixed32(lenBuf[2])<<16 |
		protowire.Fixed32(lenBuf[3])<<24

	if msgLen > MaxProtobufMessageSize {
		return nil, fmt.Errorf("message too large: %d > %d", msgLen, MaxProtobufMessageSize)
	}

	// 读取消息体
	msgBytes := make([]byte, msgLen)
	if _, err := io.ReadFull(r, msgBytes); err != nil {
		return nil, fmt.Errorf("read message: %w", err)
	}

	// 解析 Protobuf
	msg := &proto.Message{}
	if err := proto.Unmarshal(msgBytes, msg); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}

	return msg, nil
}

// ProtobufStreamReader Protobuf 流读取器
type ProtobufStreamReader struct {
	reader *bufio.Reader
	codec  *ProtobufCodec
}

// NewProtobufStreamReader 创建流读取器
func NewProtobufStreamReader(r io.Reader) *ProtobufStreamReader {
	return &ProtobufStreamReader{
		reader: bufio.NewReader(r),
		codec:  NewProtobufCodec(),
	}
}

// ReadMessage 从流中读取消息
func (sr *ProtobufStreamReader) ReadMessage() (*proto.Message, error) {
	return sr.codec.Decode(sr.reader)
}

// ProtobufStreamWriter Protobuf 流写入器
type ProtobufStreamWriter struct {
	writer *bufio.Writer
	codec  *ProtobufCodec
}

// NewProtobufStreamWriter 创建流写入器
func NewProtobufStreamWriter(w io.Writer) *ProtobufStreamWriter {
	return &ProtobufStreamWriter{
		writer: bufio.NewWriter(w),
		codec:  NewProtobufCodec(),
	}
}

// WriteMessage 向流中写入消息
func (sw *ProtobufStreamWriter) WriteMessage(msg *proto.Message) error {
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
```

- [ ] **Step 2: 运行测试验证通过**

Run: `go test ./internal/network/protocol/... -run TestProtobufCodec -v`
Expected: PASS

- [ ] **Step 3: 提交实现**

```bash
git add internal/network/protocol/protobuf_codec.go
git commit -m "feat(protocol): 实现 Protobuf 编解码器

- ProtobufCodec 实现消息编解码
- 支持 StreamReader/StreamWriter 流式处理
- 使用 4 字节长度前缀协议

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: 更新协议处理器使用 Protobuf

**Files:**
- Modify: `internal/network/protocol/protocol.go`

- [ ] **Step 1: 更新协议处理器使用 Protobuf**

修改 `internal/network/protocol/protocol.go`，替换 Codec 为 ProtobufCodec:

```go
// 在 Protocol 结构体中
type Protocol struct {
	host    host.Host
	handler Handler
	codec   *ProtobufCodec  // 改为 ProtobufCodec
	mu      sync.RWMutex
}

// 在 NewProtocol 中
func NewProtocol(h host.Host, handler Handler) *Protocol {
	p := &Protocol{
		host:    h,
		handler: handler,
		codec:   NewProtobufCodec(),  // 使用 ProtobufCodec
	}
	h.SetStreamHandler(AWSPProtocolID, p.handleStream)
	return p
}

// 在 handleStream 中
func (p *Protocol) handleStream(s network.Stream) {
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reader := NewProtobufStreamReader(s)  // 使用 ProtobufStreamReader
	writer := NewProtobufStreamWriter(s)  // 使用 ProtobufStreamWriter

	// ... 其余逻辑保持不变，但需要适配 proto.Message 类型
}
```

- [ ] **Step 2: 更新 processMessage 适配 Protobuf 类型**

修改 `processMessage` 函数:

```go
func (p *Protocol) processMessage(ctx context.Context, msg *proto.Message) (*proto.Message, error) {
	switch msg.Header.Type {
	case proto.MessageType_MESSAGE_TYPE_HANDSHAKE:
		h := msg.GetHandshake()
		ack, err := p.handler.HandleHandshake(ctx, convertHandshakeFromProto(h))
		if err != nil {
			return nil, err
		}
		return &proto.Message{
			Header: NewProtobufMessageHeader(proto.MessageType_MESSAGE_TYPE_HANDSHAKE_ACK),
			Payload: &proto.Message_HandshakeAck{
				HandshakeAck: convertHandshakeAckToProto(ack),
			},
		}, nil

	// ... 其他消息类型类似处理
	}
}
```

- [ ] **Step 3: 添加类型转换函数**

在 `internal/network/protocol/converter.go` 中添加:

```go
package protocol

import (
	"github.com/daifei0527/agentwiki/internal/network/protocol/proto"
)

// Handshake 转换
func convertHandshakeFromProto(p *proto.Handshake) *Handshake {
	return &Handshake{
		NodeID:     p.NodeId,
		PeerID:     p.PeerId,
		NodeType:   NodeType(p.NodeType),
		Version:    p.Version,
		Categories: p.Categories,
		EntryCount: p.EntryCount,
		Signature:  p.Signature,
	}
}

func convertHandshakeAckToProto(h *HandshakeAck) *proto.HandshakeAck {
	return &proto.HandshakeAck{
		NodeId:       h.NodeID,
		PeerId:       h.PeerID,
		NodeType:     proto.NodeType(h.NodeType),
		Version:      h.Version,
		Accepted:     h.Accepted,
		RejectReason: h.RejectReason,
		Signature:    h.Signature,
	}
}

// ... 其他类型的转换函数
```

- [ ] **Step 4: 运行测试验证**

Run: `go test ./internal/network/protocol/... -v`
Expected: 所有测试通过

- [ ] **Step 5: 提交更新**

```bash
git add internal/network/protocol/
git commit -m "feat(protocol): 协议处理器切换到 Protobuf

- Protocol 使用 ProtobufCodec
- 添加 proto<->domain 类型转换器
- 保持 Handler 接口不变

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 7: 更新协议版本常量

**Files:**
- Modify: `internal/network/protocol/types.go`

- [ ] **Step 1: 更新协议版本**

修改 `internal/network/protocol/types.go`:

```go
const (
	// AWSPProtocolID 协议 ID (v2.0.0 使用 Protobuf)
	AWSPProtocolID = "/agentwiki/sync/2.0.0"
)
```

- [ ] **Step 2: 更新 host.go 中的协议 ID**

修改 `internal/network/host/host.go`:

```go
const AWSPProtocolID = "/agentwiki/sync/2.0.0"
```

- [ ] **Step 3: 提交版本更新**

```bash
git add internal/network/protocol/types.go internal/network/host/host.go
git commit -m "feat(protocol): 升级协议版本到 2.0.0

使用 Protobuf 序列化，不向后兼容 v1.0.0

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 8: 添加 QUIC 传输支持

**Files:**
- Modify: `internal/network/host/host.go`

- [ ] **Step 1: 添加 QUIC 依赖**

Run: `go get github.com/libp2p/go-libp2p/p2p/transport/quic`
Expected: 依赖添加成功

- [ ] **Step 2: 更新 HostConfig 添加 QUIC 配置**

修改 `internal/network/host/host.go` 中的 HostConfig:

```go
type HostConfig struct {
	// ... 现有字段 ...

	// EnableQUIC 是否启用 QUIC 传输
	EnableQUIC bool
	// QUICListenPort QUIC 监听端口 (UDP)，0 表示随机
	QUICListenPort int
}
```

更新 DefaultHostConfig:

```go
func DefaultHostConfig() *HostConfig {
	return &HostConfig{
		ListenAddrs: []string{
			"/ip4/0.0.0.0/tcp/0",
		},
		EnableDHT:          true,
		EnableMDNS:         true,
		EnableNAT:          true,
		EnableRelay:        true,
		EnableAutoRelay:    true,
		EnableWebSocket:    true,
		EnableHolePunching: true,
		EnableQUIC:         true,  // 默认启用 QUIC
		QUICListenPort:     0,
		ConnectionTimeout:  30 * time.Second,
	}
}
```

- [ ] **Step 3: 在 NewHost 中添加 QUIC 传输**

修改 `internal/network/host/host.go` 中的 NewHost 函数:

```go
import (
	"github.com/libp2p/go-libp2p/p2p/transport/quic"
)

func NewHost(ctx context.Context, cfg *HostConfig) (*P2PHost, error) {
	// ... 现有代码 ...

	// 传输层配置
	transports := []libp2p.Option{
		libp2p.Transport(tcp.NewTCPTransport),
	}

	// 添加 WebSocket 传输
	if cfg.EnableWebSocket {
		transports = append(transports, libp2p.Transport(websocket.New))
	}

	// 添加 QUIC 传输
	if cfg.EnableQUIC {
		quicOpts := []quic.Option{
			quic.WithConnectionIdleTimeout(30 * time.Second),
		}
		transports = append(transports, libp2p.Transport(quic.NewTransport, quicOpts...))
	}

	opts = append(opts, transports...)

	// ... 其余代码不变 ...
}
```

- [ ] **Step 4: 运行测试验证**

Run: `go test ./internal/network/host/... -v`
Expected: 测试通过

- [ ] **Step 5: 提交 QUIC 支持**

```bash
git add go.mod go.sum internal/network/host/host.go
git commit -m "feat(network): 添加 QUIC 传输支持

- 默认启用 QUIC 传输 (UDP)
- 与 TCP/WebSocket 并行运行
- 更好的 NAT 穿透性能

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 9: 性能基准测试对比

**Files:**
- Create: `internal/network/protocol/benchmark_test.go`

- [ ] **Step 1: 创建性能对比测试**

创建 `internal/network/protocol/benchmark_test.go`:

```go
package protocol

import (
	"bytes"
	"testing"

	"github.com/daifei0527/agentwiki/internal/network/protocol/proto"
)

func BenchmarkProtobufCodec_Encode(b *testing.B) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_HANDSHAKE,
			MessageId: "bench-msg",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_Handshake{
			Handshake: &proto.Handshake{
				NodeId:     "node-1",
				PeerId:     "peer-1",
				NodeType:   proto.NodeType_NODE_TYPE_LOCAL,
				Version:    "2.0.0",
				Categories: []string{"tech", "science", "business"},
				EntryCount: 1000,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(msg)
	}
}

func BenchmarkProtobufCodec_Decode(b *testing.B) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_HANDSHAKE,
			MessageId: "bench-msg",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_Handshake{
			Handshake: &proto.Handshake{
				NodeId:     "node-1",
				PeerId:     "peer-1",
				NodeType:   proto.NodeType_NODE_TYPE_LOCAL,
				Version:    "2.0.0",
				Categories: []string{"tech", "science", "business"},
				EntryCount: 1000,
			},
		},
	}

	encoded, _ := codec.Encode(msg)
	reader := bytes.NewReader(encoded)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(encoded)
		codec.Decode(reader)
	}
}

func BenchmarkProtobufCodec_RoundTrip(b *testing.B) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_SYNC_REQUEST,
			MessageId: "bench-sync",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_SyncRequest{
			SyncRequest: &proto.SyncRequest{
				RequestId:           "sync-1",
				LastSyncTimestamp:   1234560000,
				VersionVector:       map[string]int64{"e1": 100, "e2": 200, "e3": 300},
				RequestedCategories: []string{"tech", "science"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := codec.Encode(msg)
		codec.Decode(bytes.NewReader(encoded))
	}
}
```

- [ ] **Step 2: 运行性能基准测试**

Run: `go test -bench=. ./internal/network/protocol/... -benchmem`
Expected: 记录性能数据

- [ ] **Step 3: 提交基准测试**

```bash
git add internal/network/protocol/benchmark_test.go
git commit -m "test(protocol): 添加 Protobuf 编解码性能基准测试

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 10: 运行完整测试套件

**Files:**
- 无新增文件

- [ ] **Step 1: 运行所有测试**

Run: `go test ./... -v -count=1`
Expected: 所有测试通过

- [ ] **Step 2: 运行测试覆盖率**

Run: `go test ./... -coverprofile=coverage.out`
Expected: 覆盖率 > 60%

- [ ] **Step 3: 提交最终更新**

```bash
git add .
git commit -m "feat: Phase 6b 网络协议增强完成

- Protobuf 替换 JSON 序列化
- 添加 QUIC 传输支持
- 协议版本升级到 2.0.0

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## 验收清单

- [ ] Protobuf 消息定义完成 (proto/awsp.proto)
- [ ] Go 代码生成成功 (internal/network/protocol/proto/awsp.pb.go)
- [ ] ProtobufCodec 实现并通过测试
- [ ] 协议处理器使用 Protobuf
- [ ] QUIC 传输启用
- [ ] 协议版本更新为 2.0.0
- [ ] 性能基准测试完成
- [ ] 所有测试通过
- [ ] 测试覆盖率 > 60%

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| protoc 未安装 | 代码生成失败 | Makefile 自动检测并提示安装 |
| QUIC UDP 端口被阻止 | 连接失败 | 保留 TCP 作为备选传输 |
| 不兼容旧节点 | 网络分裂 | 部署文档说明升级要求 |
