# Phase 6b: 网络协议增强设计

**版本**: v2.0.0
**日期**: 2026-04-13
**状态**: 已批准

## 概述

将 AWSP (AgentWiki Sync Protocol) 从 JSON 序列化迁移到 Protobuf，并添加 QUIC 传输支持，提升网络性能和效率。

## 目标

| 指标 | 当前 (JSON) | 目标 (Protobuf) |
|------|------------|----------------|
| 消息体积 | 100% | 减少 60-80% |
| 编码速度 | 100% | 提升 10-50x |
| 解码速度 | 100% | 提升 10-100x |
| 网络延迟 | TCP | QUIC (更低) |

## 架构设计

### 协议栈

```
┌─────────────────────────────────────────────────────┐
│                    Protocol Layer                     │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │   Handshake  │  │    Query     │  │   Sync     │ │
│  └──────────────┘  └──────────────┘  └────────────┘ │
├─────────────────────────────────────────────────────┤
│                   Codec Layer                        │
│  ┌──────────────────────────────────────────────┐   │
│  │           Protobuf Encoder/Decoder            │   │
│  └──────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────┤
│                  Transport Layer                     │
│  ┌────────────┐  ┌────────────┐  ┌──────────────┐  │
│  │    TCP     │  │  WebSocket │  │    QUIC      │  │
│  └────────────┘  └────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────┘
```

### 文件结构

```
agentwiki/
├── proto/
│   └── awsp.proto           # Protobuf 消息定义
├── internal/network/
│   ├── protocol/
│   │   ├── codec.go         # 重写: Protobuf 编解码
│   │   ├── codec_test.go    # 更新测试
│   │   ├── types.go         # 保留: Go 类型定义
│   │   ├── protocol.go      # 保留: 协议处理器
│   │   └── proto/           # 生成的 Go 代码
│   │       └── awsp.pb.go
│   └── host/
│       └── host.go          # 修改: 添加 QUIC 传输
```

## Protobuf 消息定义

### 消息头

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
```

### 核心消息

```protobuf
// 握手
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

// 查询
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

// 同步
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
```

### 镜像与推送

```protobuf
// 镜像请求
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

// 推送
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
```

### 心跳与状态

```protobuf
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

## QUIC 传输配置

### Host 配置更新

```go
// HostConfig 新增字段
type HostConfig struct {
    // ... 现有字段 ...
    
    // EnableQUIC 是否启用 QUIC 传输
    EnableQUIC bool
    // QUICListenPort QUIC 监听端口 (UDP)
    QUICListenPort int
}
```

### libp2p QUIC 集成

```go
import (
    "github.com/libp2p/go-libp2p/p2p/transport/quic"
)

// 创建 host 时添加 QUIC
func NewHost(ctx context.Context, cfg *HostConfig) (*P2PHost, error) {
    opts := []libp2p.Option{
        // ... 现有选项 ...
        libp2p.Transport(tcp.NewTCPTransport),
        libp2p.Transport(websocket.New),
    }
    
    // 添加 QUIC 传输
    if cfg.EnableQUIC {
        opts = append(opts, libp2p.Transport(quic.NewTransport))
    }
    
    // ...
}
```

## 协议版本

- **旧版本**: `/agentwiki/sync/1.0.0` (JSON)
- **新版本**: `/agentwiki/sync/2.0.0` (Protobuf + QUIC)

**注意**: v2.0.0 不向后兼容 v1.0.0 节点。

## 实现任务

### Task 1: 添加 Protobuf 依赖

**Files:**
- Modify: `go.mod`
- Create: `proto/awsp.proto`

**Steps:**
1. 运行 `go get google.golang.org/protobuf/proto`
2. 安装 protoc 编译器
3. 创建 proto/awsp.proto 文件

### Task 2: 生成 Go 代码

**Files:**
- Create: `internal/network/protocol/proto/awsp.pb.go`

**Steps:**
1. 运行 `protoc --go_out=. --go_opt=paths=source_relative proto/awsp.proto`
2. 验证生成的代码

### Task 3: 重写 Codec

**Files:**
- Modify: `internal/network/protocol/codec.go`
- Modify: `internal/network/protocol/codec_test.go`

**Steps:**
1. 替换 JSON.Marshal 为 proto.Marshal
2. 替换 JSON.Unmarshal 为 proto.Unmarshal
3. 更新 Message 结构体使用 proto 生成的类型
4. 保持 Encode/Decode 接口不变
5. 更新测试

### Task 4: 添加 QUIC 传输

**Files:**
- Modify: `internal/network/host/host.go`
- Modify: `internal/network/host/host_test.go`

**Steps:**
1. 添加 quic 依赖
2. 在 HostConfig 中添加 EnableQUIC 字段
3. 在 NewHost 中添加 QUIC transport
4. 更新默认配置
5. 添加测试

### Task 5: 更新协议版本

**Files:**
- Modify: `internal/network/protocol/types.go`
- Modify: `internal/network/host/host.go`

**Steps:**
1. 更新 AWSPProtocolID 为 `/agentwiki/sync/2.0.0`
2. 更新 UserAgent 版本号

### Task 6: 测试验证

**Steps:**
1. 单元测试: 编解码正确性
2. 集成测试: 节点间通信
3. 性能基准测试: JSON vs Protobuf 对比

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| protoc 安装复杂 | 构建失败 | 提供 Makefile 自动化安装 |
| QUIC UDP 防火墙 | 连接失败 | 保留 TCP 作为备选 |
| 不兼容旧节点 | 网络分裂 | 部署文档说明 |

## 验收标准

- [ ] Protobuf 消息定义完成
- [ ] Codec 重写并通过测试
- [ ] QUIC 传输启用并测试
- [ ] 性能基准测试显示改进
- [ ] 所有现有测试通过
- [ ] 代码覆盖率 > 70%

## 参考资料

- [Protocol Buffers Language Guide](https://protobuf.dev/programming-guides/)
- [libp2p QUIC Transport](https://github.com/libp2p/go-libp2p/tree/master/p2p/transport/quic)
- [Protobuf Performance](https://protobuf.dev/programming-guides/techniques/)
