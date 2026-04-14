# Polyant 分布式百科知识库系统 -- 技术设计文档

> **项目代号**: Polyant
> **文档版本**: v1.0
> **日期**: 2026-04-08
> **技术栈**: Go (Golang) 1.22+
> **目标平台**: Windows / Linux / macOS
> **文档性质**: 技术设计文档 (TDD)，面向开发人员

---

## 目录

1. [系统总体设计](#1-系统总体设计)
2. [存储层设计](#2-存储层设计)
3. [网络层设计](#3-网络层设计)
4. [API层设计](#4-api层设计)
5. [核心算法设计](#5-核心算法设计)
6. [配置管理设计](#6-配置管理设计)
7. [错误处理设计](#7-错误处理设计)
8. [安全性设计](#8-安全性设计)
9. [部署设计](#9-部署设计)

---

## 1. 系统总体设计

### 1.1 整体架构图

```
+================================================================+
|                    智能体 (Agent)                                |
|        (OpenClaw / Claude Code / 其他支持Skill的智能体)           |
+===========================+====================================+
                            | Skill 协议调用 (HTTP REST)
                            v
+================================================================+
|               Polyant 本地节点服务 (单进程多goroutine)           |
|                                                                |
|  +------------------+  +------------------+  +---------------+ |
|  |   Skill API 层    |  |   查询引擎       |  |  同步引擎      | |
|  |  (net/http)      |  |  (Bleve)        |  | (P2P Sync)    | |
|  |  端口: 18531     |  |                 |  |               | |
|  +--------+---------+  +--------+---------+  +-------+-------+ |
|           |                     |                   |         |
|  +--------+---------------------+-------------------+-------+ |
|  |                  核心业务逻辑层 (Core)                     | |
|  |  +----------+ +----------+ +----------+ +----------+     | |
|  |  | EntryMgr | | Search   | | Rating   | | Category |     | |
|  |  | 条目管理  | | 搜索引擎  | | 评分系统  | | 分类管理  |     | |
|  |  +----------+ +----------+ +----------+ +----------+     | |
|  +--------+---------------------+-------------------+-------+ |
|           |                     |                   |         |
|  +--------+---------------------+-------------------+-------+ |
|  |                    存储层 (Storage)                       | |
|  |  +------------+  +-------------+  +------------------+   | |
|  |  | Pebble KV  |  | Bleve Index |  | 用户/元数据存储    |   | |
|  |  | 本地持久化  |  | 全文搜索引擎  |  | (Pebble KV)      |   | |
|  |  +------------+  +-------------+  +------------------+   | |
|  +----------------------------------------------------------+ |
|                                                                |
|  +----------------------------------------------------------+ |
|  |                  P2P 网络层 (go-libp2p)                    | |
|  |  +------+ +------+ +------+ +------------------------+   | |
|  |  | DHT  | | mDNS | | NAT  | | AWSP 自定义同步协议     |   | |
|  |  | 发现  | | 局域网| | 穿透  | | /polyant/sync/1.0.0|   | |
|  |  +------+ +------+ +------+ +------------------------+   | |
|  +----------------------------------------------------------+ |
|                                                                |
|  +----------------------------------------------------------+ |
|  |                  系统服务层 (Service)                       | |
|  |  +------------------+  +------------------+               | |
|  |  | kardianos/service |  | 信号处理/优雅关闭  |               | |
|  |  | 跨平台守护进程    |  | Graceful Shutdown |               | |
|  |  +------------------+  +------------------+               | |
|  +----------------------------------------------------------+ |
+================================================================+
                            |
           +----------------+----------------+
           v                v                v
    +-----------+    +-----------+    +-----------+
    | 种子节点A  |    | 种子节点B  |    | 种子节点C  |
    | (全量数据) |<-->| (全量数据) |<-->| (全量数据) |
    +-----------+    +-----------+    +-----------+
           |                |                |
           +----------------+----------------+
                            |
                   种子节点间定时双向同步 (每5分钟)
```

### 1.2 模块分解与职责

| 模块 | 包路径 | 职责 | 关键依赖 |
|------|--------|------|----------|
| **Skill API** | `internal/api` | 对外暴露REST接口，供智能体调用 | `net/http`, `encoding/json` |
| **查询引擎** | `internal/core/search` | 全文搜索、关键词匹配、结果排序 | `bleve`, `gojieba` |
| **同步引擎** | `internal/network/sync` | P2P数据同步、镜像管理、增量更新 | `go-libp2p`, 自定义协议 |
| **存储层** | `internal/storage` | 本地KV存储、全文索引、元数据管理 | `pebble`, `bleve` |
| **用户管理** | `internal/core/user` | 公钥注册、身份验证、层级管理 | `crypto/ed25519` |
| **评分系统** | `internal/core/rating` | 条目评分、权重计算、用户层级升降 | 自定义算法 |
| **分类管理** | `internal/core/category` | 知识分类体系、分类维护 | 树形结构 |
| **P2P网络层** | `internal/network` | 节点发现、连接管理、NAT穿透 | `go-libp2p`, `go-libp2p-kad-dht` |
| **认证授权** | `internal/auth` | Ed25519签名验证、RBAC权限控制 | `crypto/ed25519` |
| **系统服务** | `internal/service` | 跨平台守护进程化 | `kardianos/service` |
| **配置管理** | `pkg/config` | YAML配置加载、环境变量覆盖 | `gopkg.in/yaml.v3` |

### 1.3 组件关系图

```
+------------------------------------------------------------------+
|                        组件依赖关系                                |
+------------------------------------------------------------------+

  Agent (外部)                          Seed Nodes (外部)
      |                                      |
      v                                      v
 +----------+                          +-----------+
 | Skill API|                          | AWSP Proto|
 +----+-----+                          +-----+-----+
      |                                      |
      v                                      v
 +----------+    +----------+    +------+  +----------+
 | Auth MW  |--->| EntryMgr |<-->| Sync |<->| Protocol |
 +----------+    +----+-----+    | Eng  |  | Handler  |
                      |          +------+  +----------+
                      v              |
                 +----------+        |
                 | Storage  |<-------+
                 | (Pebble) |
                 +----+-----+
                      |
                      v
                 +----------+
                 | Bleve    |
                 | Index    |
                 +----------+

图例:
  ---> 调用/依赖方向
  <--> 双向通信
  |   数据流
```

### 1.4 核心接口定义

```go
// internal/core/entry/manager.go

// EntryManager 知识条目管理器接口
type EntryManager interface {
    // CRUD 操作
    CreateEntry(ctx context.Context, entry *model.KnowledgeEntry) (*model.KnowledgeEntry, error)
    GetEntry(ctx context.Context, id string) (*model.KnowledgeEntry, error)
    UpdateEntry(ctx context.Context, id string, entry *model.KnowledgeEntry) (*model.KnowledgeEntry, error)
    DeleteEntry(ctx context.Context, id string) error // 软删除

    // 批量操作
    ListEntries(ctx context.Context, filter EntryFilter) ([]*model.KnowledgeEntry, int, error)
    BatchCreate(ctx context.Context, entries []*model.KnowledgeEntry) error

    // 同步相关
    GetEntriesSince(ctx context.Context, timestamp int64) ([]*model.KnowledgeEntry, error)
    GetVersionVector(ctx context.Context) (map[string]int64, error)
    MergeEntries(ctx context.Context, entries []*model.KnowledgeEntry) error
}

// EntryFilter 条目查询过滤器
type EntryFilter struct {
    Category   string
    Tags       []string
    Status     model.EntryStatus
    CreatedBy  string
    Limit      int
    Offset     int
    OrderBy    string // "score", "updated_at", "created_at"
    OrderDir   string // "asc", "desc"
}
```

```go
// internal/core/search/engine.go

// SearchEngine 搜索引擎接口
type SearchEngine interface {
    // 索引操作
    IndexEntry(entry *model.KnowledgeEntry) error
    UpdateIndex(entry *model.KnowledgeEntry) error
    DeleteIndex(entryID string) error
    BatchIndex(entries []*model.KnowledgeEntry) error

    // 查询操作
    Search(ctx context.Context, query SearchQuery) (*SearchResult, error)
    Suggest(ctx context.Context, prefix string, limit int) ([]string, error)
}

// SearchQuery 搜索查询参数
type SearchQuery struct {
    Keyword    string
    Categories []string
    Tags       []string
    Limit      int
    Offset     int
    MinScore   float64
}
```

```go
// internal/network/sync/engine.go

// SyncEngine 同步引擎接口
type SyncEngine interface {
    // 连接管理
    ConnectToSeed(ctx context.Context, peerAddr string) error
    DisconnectFromSeed(peerID string) error
    GetConnectedPeers() []PeerInfo

    // 同步操作
    FullSync(ctx context.Context) error
    IncrementalSync(ctx context.Context) error
    PushEntry(ctx context.Context, entry *model.KnowledgeEntry) error
    PushRating(ctx context.Context, rating *model.Rating) error

    // 镜像操作
    MirrorCategories(ctx context.Context, categories []string) error
    ServeMirror(ctx context.Context) error

    // 生命周期
    Start(ctx context.Context) error
    Stop() error
}
```

### 1.5 进程与 Goroutine 架构

```
Polyant Node (单进程, 多goroutine)
|
+-- Main Goroutine
|   |-- 系统服务管理 (kardianos/service)
|   |-- 信号处理 (SIGINT, SIGTERM)
|   +-- 优雅关闭协调 (context cancellation)
|
+-- API Server Goroutine
|   |-- HTTP 监听 (:18531)
|   |-- 请求路由分发
|   |-- 中间件链 (Auth -> RateLimit -> Handler)
|   +-- 请求处理 (每个请求一个goroutine)
|
+-- Query Engine Goroutine Pool (worker pool, size=CPU*2)
|   |-- 全文搜索 (Bleve)
|   |-- 结果排序与评分计算
|   +-- 远程查询聚合
|
+-- Sync Engine Goroutines
|   |-- Seed 连接管理器 (1 goroutine)
|   |-- 数据拉取 Worker Pool (3 goroutines)
|   |-- 数据推送 Worker (1 goroutine)
|   |-- 镜像服务 Worker (N goroutines, 按配置)
|   +-- 增量同步定时器 (1 goroutine, ticker)
|
+-- P2P Network Goroutines (go-libp2p 内部管理)
|   |-- DHT 节点发现
|   |-- mDNS 局域网发现
|   |-- 连接管理 (自动重连)
|   |-- AWSP 协议处理 (每连接1 goroutine)
|   +-- NAT 穿透 (AutoNAT)
|
+-- Background Tasks
|   |-- 评分权重重算 (定时, 每10分钟)
|   |-- 数据完整性校验 (定时, 每15分钟)
|   |-- 用户层级检查与升级 (定时, 每小时)
|   +-- 过期数据清理 (定时, 每天)
```

---

## 2. 存储层设计

### 2.1 存储架构总览

```
+================================================================+
|                      存储层架构                                  |
+================================================================+
|                                                                |
|  +----------------------------------------------------------+ |
|  |                    Pebble KV Store                         | |
|  |                                                           | |
|  |  +------------------+  +------------------+               | |
|  |  |  数据分区 (CF)     |  |  系统分区 (CF)     |               | |
|  |  |  "entries"        |  |  "meta"           |               | |
|  |  |  "users"          |  |  "version_vector" |               | |
|  |  |  "ratings"        |  |  "sync_state"     |               | |
|  |  |  "categories"     |  |  "node_info"      |               | |
|  |  +------------------+  +------------------+               | |
|  +----------------------------------------------------------+ |
|                                                                |
|  +----------------------------------------------------------+ |
|  |                    Bleve 全文索引                           | |
|  |                                                           | |
|  |  +------------------+  +------------------+               | |
|  |  |  条目索引          |  |  分类索引          |               | |
|  |  |  "entries.bleve"  |  |  "categories.bleve"|              | |
|  |  +------------------+  +------------------+               | |
|  +----------------------------------------------------------+ |
|                                                                |
+================================================================+
```

### 2.2 Pebble KV Schema 设计

#### 2.2.1 Key 编码规范

所有 Key 采用分层前缀编码，使用 `[]byte` 存储，前缀之间以固定分隔符 `\x00` 分隔。

```
Key 编码格式:
  <namespace>\x00<id>                       -- 单条记录
  <namespace>\x00<field>\x00<value>\x00<id> -- 索引记录
```

#### 2.2.2 详细 Schema

**知识条目 (entries)**

| Key 格式 | Value | 说明 |
|----------|-------|------|
| `entry\x00{id}` | KnowledgeEntry (Protobuf) | 条目完整数据 |
| `entry_cat\x00{category}\x00{id}` | 空 (Tombstone) | 按分类索引 |
| `entry_tag\x00{tag}\x00{id}` | 空 | 按标签索引 |
| `entry_author\x00{pubkey_hash}\x00{id}` | 空 | 按作者索引 |
| `entry_updated\x00{updated_at_ms}\x00{id}` | 空 | 按更新时间索引 |
| `entry_hash\x00{content_hash}` | `{id}` | 内容哈希去重索引 |

**用户 (users)**

| Key 格式 | Value | 说明 |
|----------|-------|------|
| `user\x00{pubkey_hash}` | User (Protobuf) | 用户完整数据 |
| `user_email\x00{email}` | `{pubkey_hash}` | 邮箱索引 |
| `user_level\x00{level}\x00{pubkey_hash}` | 空 | 按层级索引 |
| `user_node\x00{node_id}` | `{pubkey_hash}` | 节点关联索引 |

**评分 (ratings)**

| Key 格式 | Value | 说明 |
|----------|-------|------|
| `rating\x00{id}` | Rating (Protobuf) | 评分完整数据 |
| `rating_entry\x00{entry_id}\x00{rater_pubkey_hash}` | `{rating_id}` | 条目评分索引 (防止重复评分) |
| `rating_rater\x00{rater_pubkey_hash}\x00{rated_at}\x00{id}` | 空 | 按评分者索引 |
| `rating_entry_time\x00{entry_id}\x00{rated_at}\x00{id}` | 空 | 按条目+时间索引 |

**分类 (categories)**

| Key 格式 | Value | 说明 |
|----------|-------|------|
| `cat\x00{path}` | Category (Protobuf) | 分类完整数据 |
| `cat_parent\x00{parent_id}\x00{path}` | 空 | 父分类索引 |
| `cat_builtin\x00{path}` | 空 | 内置分类标记 |

**系统元数据 (meta)**

| Key 格式 | Value | 说明 |
|----------|-------|------|
| `meta\x00node_id` | string (UTF-8) | 本节点ID |
| `meta\x00node_type` | string (UTF-8) | 节点类型 (local/seed) |
| `meta\x00schema_version` | int64 | Schema版本号 |
| `meta\x00last_sync\x00{peer_id}` | int64 | 与某peer的最后同步时间 |
| `meta\x00total_entries` | int64 | 本地条目总数 |

**版本向量 (version_vector)**

| Key 格式 | Value | 说明 |
|----------|-------|------|
| `vv\x00{entry_id}` | int64 (BigEndian) | 条目最新版本号 |

**同步状态 (sync_state)**

| Key 格式 | Value | 说明 |
|----------|-------|------|
| `sync\x00{peer_id}\x00cursor` | int64 | 同步游标 (时间戳) |
| `sync\x00{peer_id}\x00status` | string | 同步状态 (idle/running/error) |

#### 2.2.3 Pebble Column Family 配置

```go
// internal/storage/kv/store.go

// Column Family 定义
var ColumnFamilies = map[string]*pebble.Options{
    "default": defaultOptions(),  // 默认 CF，存储条目数据
    "meta":    metaOptions(),     // 系统元数据，低写入量
}

func defaultOptions() *pebble.Options {
    opts := &pebble.Options{
        // 缓存大小: 128MB
        Cache:        pebble.NewCache(128 * 1024 * 1024),
        // MemTable 大小: 64MB
        MemTableSize: 64 * 1024 * 1024,
        // L0/L1 压缩阈值
        L0CompactionThreshold: 4,
        L0StopWritesThreshold:  8,
        // 压缩算法
        Compression: pebble.ZstdCompression,
    }
    return opts
}

func metaOptions() *pebble.Options {
    opts := defaultOptions()
    opts.MemTableSize = 8 * 1024 * 1024  // 元数据较小，减少内存
    return opts
}
```

### 2.3 Bleve 索引 Schema 设计

#### 2.3.1 条目索引 Mapping

```go
// internal/storage/index/mapping.go

// CreateEntryMapping 创建知识条目的 Bleve 索引 Mapping
func CreateEntryMapping() *mapping.IndexMapping {
    entryMapping := bleve.NewDocumentMapping()

    // title: 标题字段，中文分词 + 英文分词
    titleMapping := bleve.NewTextFieldMapping()
    titleMapping.Analyzer = "zh_en_mix"  // 自定义中英混合分词器
    titleMapping.Store = true
    titleMapping.Index = true
    titleMapping.IncludeTermVectors = true
    entryMapping.AddFieldMappingsAt("title", titleMapping)

    // content: 内容字段，中文分词
    contentMapping := bleve.NewTextFieldMapping()
    contentMapping.Analyzer = "zh_en_mix"
    contentMapping.Store = true
    contentMapping.Index = true
    contentMapping.IncludeTermVectors = true
    entryMapping.AddFieldMappingsAt("content", contentMapping)

    // category: 分类字段，精确匹配
    categoryMapping := bleve.NewKeywordFieldMapping()
    categoryMapping.Store = true
    categoryMapping.Index = true
    entryMapping.AddFieldMappingsAt("category", categoryMapping)

    // tags: 标签字段，精确匹配 (多值)
    tagsMapping := bleve.NewKeywordFieldMapping()
    tagsMapping.Store = true
    tagsMapping.Index = true
    entryMapping.AddFieldMappingsAt("tags", tagsMapping)

    // score: 评分字段，数值范围
    scoreMapping := bleve.NewNumericFieldMapping()
    scoreMapping.Store = true
    scoreMapping.Index = true
    entryMapping.AddFieldMappingsAt("score", scoreMapping)

    // created_at / updated_at: 时间字段，数值范围
    createdAtMapping := bleve.NewNumericFieldMapping()
    createdAtMapping.Store = true
    createdAtMapping.Index = true
    entryMapping.AddFieldMappingsAt("created_at", createdAtMapping)

    updatedAtMapping := bleve.NewNumericFieldMapping()
    updatedAtMapping.Store = true
    updatedAtMapping.Index = true
    entryMapping.AddFieldMappingsAt("updated_at", updatedAtMapping)

    // status: 状态字段，精确匹配
    statusMapping := bleve.NewKeywordFieldMapping()
    statusMapping.Store = true
    statusMapping.Index = true
    entryMapping.AddFieldMappingsAt("status", statusMapping)

    // created_by: 创建者，精确匹配
    createdByMapping := bleve.NewKeywordFieldMapping()
    createdByMapping.Store = true
    createdByMapping.Index = true
    entryMapping.AddFieldMappingsAt("created_by", createdByMapping)

    indexMapping := bleve.NewIndexMapping()
    indexMapping.AddDocumentMapping("entry", entryMapping)
    indexMapping.DefaultAnalyzer = "zh_en_mix"

    return indexMapping
}
```

#### 2.3.2 中文分词器配置

```go
// internal/storage/index/analyzer.go

import "github.com/yanyiwu/gojieba"

// ZhEnMixAnalyzer 中英混合分词器，实现 bleve.Analyzer 接口
type ZhEnMixAnalyzer struct {
    jieba *gojieba.Jieba
}

func NewZhEnMixAnalyzer() *ZhEnMixAnalyzer {
    return &ZhEnMixAnalyzer{
        jieba: gojieba.NewJieba(),
    }
}

func (a *ZhEnMixAnalyzer) Analyze(input []byte) analysis.TokenStream {
    text := string(input)
    tokens := make(analysis.TokenStream, 0)
    words := a.jieba.CutForSearch(text, true)
    pos := 0
    for _, word := range words {
        idx := strings.Index(text[pos:], word)
        if idx >= 0 {
            token := analysis.Token{
                Start:    pos + idx,
                End:      pos + idx + len(word),
                Term:     []byte(strings.ToLower(word)),
                Position: len(tokens),
                Type:     analysis.IdeographicType,
            }
            tokens = append(tokens, token)
            pos = pos + idx + len(word)
        }
    }
    return tokens
}
```

### 2.4 数据序列化格式 (Protobuf)

#### 2.4.1 数据模型 Protobuf 定义

```protobuf
// pkg/proto/model.proto

syntax = "proto3";
package polyant.model;
option go_package = "github.com/polyant/pkg/proto/model";

import "google/protobuf/struct.proto";

enum EntryStatus {
  ENTRY_STATUS_ACTIVE = 0;
  ENTRY_STATUS_ARCHIVED = 1;
}

message KnowledgeEntry {
  string id = 1;                    // UUID v4
  string title = 2;
  string content = 3;               // Markdown 内容
  repeated google.protobuf.Struct json_data = 4;
  string category = 5;              // "一级/二级/三级"
  repeated string tags = 6;
  int64 version = 7;
  int64 created_at = 8;             // Unix 毫秒时间戳
  int64 updated_at = 9;
  string created_by = 10;           // 创建者公钥哈希
  double score = 11;
  int32 score_count = 12;
  string content_hash = 13;         // SHA-256
  EntryStatus status = 14;
  string license = 15;              // 默认 "CC BY-SA 4.0"
  string source_ref = 16;
  bytes creator_signature = 17;
}

enum UserLevel {
  USER_LEVEL_BASIC = 0;       // Lv0
  USER_LEVEL_VERIFIED = 1;    // Lv1
  USER_LEVEL_ACTIVE = 2;      // Lv2
  USER_LEVEL_SENIOR = 3;      // Lv3
  USER_LEVEL_EXPERT = 4;      // Lv4
  USER_LEVEL_CORE = 5;        // Lv5
}

enum UserStatus {
  USER_STATUS_ACTIVE = 0;
  USER_STATUS_SUSPENDED = 1;
}

message User {
  string public_key = 1;
  string public_key_hash = 2;
  string agent_name = 3;
  UserLevel user_level = 4;
  string email = 5;
  bool email_verified = 6;
  int64 registered_at = 7;
  int64 last_active = 8;
  int32 contribution_cnt = 9;
  int32 rating_cnt = 10;
  string node_id = 11;
  UserStatus status = 12;
  bytes registration_signature = 13;
}

message Rating {
  string id = 1;
  string entry_id = 2;
  string rater_pubkey_hash = 3;
  double score = 4;                 // 1.0 - 5.0
  double weight = 5;
  double weighted_score = 6;
  int64 rated_at = 7;
  string comment = 8;
  bytes rater_signature = 9;
}

message Category {
  string id = 1;
  string path = 2;
  string name = 3;
  string parent_id = 4;
  int32 level = 5;
  int32 sort_order = 6;
  bool is_builtin = 7;
  string maintained_by = 8;
  int64 created_at = 9;
}
```

#### 2.4.2 AWSP 协议 Protobuf 定义

```protobuf
// pkg/proto/protocol.proto

syntax = "proto3";
package polyant.protocol;
option go_package = "github.com/polyant/pkg/proto/protocol";

import "model.proto";

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

enum NodeType {
  NODE_TYPE_LOCAL = 0;
  NODE_TYPE_SEED = 1;
}

message MessageHeader {
  MessageType type = 1;
  string message_id = 2;
  int64 timestamp = 3;
  bytes signature = 4;
}

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

enum QueryType {
  QUERY_TYPE_LOCAL = 0;
  QUERY_TYPE_GLOBAL = 1;
}

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
  repeated KnowledgeEntry entries = 2;
  int32 total_count = 3;
  bool has_more = 4;
}

message SyncRequest {
  string request_id = 1;
  int64 last_sync_timestamp = 2;
  map<string, int64> version_vector = 3;
  repeated string requested_categories = 4;
}

message SyncResponse {
  string request_id = 1;
  repeated KnowledgeEntry new_entries = 2;
  repeated KnowledgeEntry updated_entries = 3;
  repeated string deleted_entry_ids = 4;
  repeated Rating new_ratings = 5;
  repeated User updated_users = 6;
  map<string, int64> server_version_vector = 7;
  int64 server_timestamp = 8;
}

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
  repeated KnowledgeEntry entries = 4;
  repeated Category categories = 5;
}

message MirrorAck {
  string request_id = 1;
  bool success = 2;
  string error_message = 3;
  int64 received_entries = 4;
}

message PushEntry {
  string entry_id = 1;
  KnowledgeEntry entry = 2;
  bytes creator_signature = 3;
}

message PushAck {
  string entry_id = 1;
  bool accepted = 2;
  string reject_reason = 3;
  int64 new_version = 4;
}

message RatingPush {
  Rating rating = 1;
  bytes rater_signature = 2;
}

message RatingAck {
  string rating_id = 1;
  bool accepted = 2;
  string reject_reason = 3;
}

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

### 2.5 存储目录结构

```
~/.polyant/                          # 默认数据根目录
|
+-- data/                              # 数据目录
|   +-- kv/                            # Pebble KV 数据库
|   |   +-- entries/                   # 条目数据 (默认 CF)
|   |   |   +-- 000001.log
|   |   |   +-- MANIFEST-000003
|   |   |   +-- OPTIONS-000004
|   |   +-- meta/                      # 元数据 CF
|   |
|   +-- index/                         # Bleve 全文索引
|   |   +-- entries.bleve/             # 条目全文索引
|   |   |   +-- index_meta.json
|   |   |   +-- store/
|   |   +-- categories.bleve/          # 分类索引
|   |
|   +-- seed-data/                     # 种子节点初始数据
|       +-- default_entries.jsonl
|       +-- default_categories.jsonl
|       +-- checksum.sha256
|
+-- keys/                              # 密钥目录
|   +-- ed25519_private.key            # Ed25519 私钥 (权限 0600)
|   +-- ed25519_public.key             # Ed25519 公钥
|   +-- node.key                       # libp2p 节点密钥
|
+-- logs/                              # 日志目录
|   +-- polyant.log
|   +-- polyant-2026-04-08.log.gz    # 归档日志
|
+-- cache/                             # 缓存目录
|   +-- query_cache/
|   +-- mirror_temp/
|
+-- polyant.yaml                     # 配置文件
+-- polyant.yaml.bak                 # 配置备份
```

### 2.6 存储层接口定义

```go
// internal/storage/store.go

// Store 统一存储接口
type Store interface {
    // 生命周期
    Open(ctx context.Context) error
    Close() error

    // 知识条目
    PutEntry(ctx context.Context, entry *model.KnowledgeEntry) error
    GetEntry(ctx context.Context, id string) (*model.KnowledgeEntry, error)
    DeleteEntry(ctx context.Context, id string) error
    ScanEntries(ctx context.Context, prefix []byte, fn func(*model.KnowledgeEntry) bool) error

    // 用户
    PutUser(ctx context.Context, user *model.User) error
    GetUser(ctx context.Context, pubkeyHash string) (*model.User, error)
    DeleteUser(ctx context.Context, pubkeyHash string) error

    // 评分
    PutRating(ctx context.Context, rating *model.Rating) error
    GetRating(ctx context.Context, id string) (*model.Rating, error)
    ListRatingsByEntry(ctx context.Context, entryID string) ([]*model.Rating, error)

    // 分类
    PutCategory(ctx context.Context, cat *model.Category) error
    GetCategory(ctx context.Context, path string) (*model.Category, error)
    ListCategories(ctx context.Context, parentPath string) ([]*model.Category, error)

    // 元数据
    GetMeta(ctx context.Context, key string) ([]byte, error)
    PutMeta(ctx context.Context, key string, value []byte) error

    // 版本向量
    GetVersion(ctx context.Context, entryID string) (int64, error)
    PutVersion(ctx context.Context, entryID string, version int64) error
    GetAllVersions(ctx context.Context) (map[string]int64, error)

    // 批量操作 (事务)
    Batch(ctx context.Context, fn func(BatchTx) error) error
}

// BatchTx 批量操作事务接口
type BatchTx interface {
    PutEntry(entry *model.KnowledgeEntry)
    DeleteEntry(id string)
    PutUser(user *model.User)
    PutRating(rating *model.Rating)
    PutCategory(cat *model.Category)
    PutMeta(key string, value []byte)
    PutVersion(entryID string, version int64)
}
```

---

## 3. 网络层设计

### 3.1 go-libp2p 集成架构

```
+================================================================+
|                    go-libp2p 集成架构                            |
+================================================================+
|                                                                |
|  +----------------------------------------------------------+ |
|  |                    Host (libp2p.Host)                      | |
|  |                                                           | |
|  |  +-----------------------------------------------------+ | |
|  |  |              Transport Layer                          | | |
|  |  |  +----------+ +----------+ +----------+               | | |
|  |  |  |   TCP    | |   QUIC   | |WebSocket |               | | |
|  |  |  +----------+ +----------+ +----------+               | | |
|  |  +-----------------------------------------------------+ | |
|  |                           |                               | |
|  |  +-----------------------------------------------------+ | |
|  |  |              Security Layer                           | | |
|  |  |  Noise Protocol (XX pattern)                          | | |
|  |  |  - 端到端加密, 前向保密                                | | |
|  |  +-----------------------------------------------------+ | |
|  |                           |                               | |
|  |  +-----------------------------------------------------+ | |
|  |  |              Muxer Layer (yamux)                      | | |
|  |  |  - 单TCP连接上多逻辑流                                 | | |
|  |  +-----------------------------------------------------+ | |
|  |                           |                               | |
|  |  +-----------------------------------------------------+ | |
|  |  |              Discovery Layer                          | | |
|  |  |  +----------------+ +-------------------------------+ | | |
|  |  |  | mDNS (局域网)   | | Kademlia DHT (全网)          | | | |
|  |  |  +----------------+ +-------------------------------+ | | |
|  |  +-----------------------------------------------------+ | |
|  |                           |                               | |
|  |  +-----------------------------------------------------+ | |
|  |  |              NAT Traversal                           | | |
|  |  |  +----------------+ +-------------------------------+ | | |
|  |  |  | AutoNAT         | | Relay (v2)                   | | | |
|  |  |  +----------------+ +-------------------------------+ | | |
|  |  +-----------------------------------------------------+ | |
|  |                           |                               | |
|  |  +-----------------------------------------------------+ | |
|  |  |              Protocol Layer                           | | |
|  |  |  AWSP: /polyant/sync/1.0.0                          | | |
|  |  +-----------------------------------------------------+ | |
|  +----------------------------------------------------------+ |
+================================================================+
```

### 3.2 Host 初始化代码

```go
// internal/network/host/host.go

const AWSPProtocolID = "/polyant/sync/1.0.0"

type HostConfig struct {
    ListenAddrs []string
    SeedPeers   []string
    EnableDHT   bool
    EnableMDNS  bool
    EnableNAT   bool
    EnableRelay bool
    PrivateKey  crypto.PrivKey
}

func NewHost(ctx context.Context, cfg *HostConfig) (host.Host, error) {
    opts := []libp2p.Option{
        libp2p.ListenAddrStrings(cfg.ListenAddrs...),
        libp2p.Identity(cfg.PrivateKey),
        libp2p.UserAgent("polyant/1.0.0"),
        libp2p.Ping(true),
        libp2p.ConnectionManager(&swarm.ConnManager{
            LowWater:    50,
            HighWater:   200,
            GracePeriod: time.Minute,
        }),
        libp2p.Transport(tcp.NewTCPTransport),
        libp2p.Transport(quic.NewTransport),
        libp2p.Security(noise.ID, noise.New),
        libp2p.Muxer("/yamux/1.0.0", libp2pyamux.DefaultTransport),
    }

    h, err := libp2p.New(opts...)
    if err != nil {
        return nil, fmt.Errorf("create libp2p host: %w", err)
    }

    h.SetStreamHandler(AWSPProtocolID, handleAWSPStream)
    return h, nil
}
```

### 3.3 AWSP 协议消息流图

#### 3.3.1 新节点首次全量同步

```
本地节点 (Local)                     种子节点 (Seed)
    |                                     |
    |  1. TCP Connect                     |
    |  ================================>  |
    |  2. Noise Handshake (加密协商)        |
    |  <===============================>  |
    |  3. yamux Stream Open               |
    |     (/polyant/sync/1.0.0)         |
    |  ================================>  |
    |  4. HANDSHAKE                       |
    |     {node_id, type=LOCAL,           |
    |      entry_count=0, signature}       |
    |  ================================>  |
    |  5. HANDSHAKE_ACK                   |
    |     {accepted=true, type=SEED,       |
    |      entry_count=50000}             |
    |  <================================  |
    |  6. MIRROR_REQUEST                  |
    |     {categories=["*"],              |
    |      full_mirror=true, batch=100}   |
    |  ================================>  |
    |  7. MIRROR_DATA (batch 0/N)         |
    |  <================================  |
    |  8. MIRROR_DATA (batch 1..N-1)      |
    |  <================================  |
    |  9. MIRROR_ACK                      |
    |     {success=true, received=50000}  |
    |  ================================>  |
    | 10. BITFIELD (本地版本向量)          |
    |  ================================>  |
    | 11. HEARTBEAT (每30秒)               |
    |  <===============================>  |
    +-- 同步完成, 开始提供服务 ------------+
```

#### 3.3.2 增量同步

```
本地节点 (Local)                     种子节点 (Seed)
    |                                     |
    |  [定时器: 每5分钟]                    |
    |  1. SYNC_REQUEST                    |
    |     {last_sync_timestamp,           |
    |      version_vector={...}}          |
    |  ================================>  |
    |  [种子节点: 对比版本向量, 收集差异]    |
    |  2. SYNC_RESPONSE                   |
    |     {new_entries, updated_entries,   |
    |      deleted_ids, new_ratings,       |
    |      server_version_vector}         |
    |  <================================  |
    |  [本地: 合并数据, 重建索引]           |
    |  3. 更新同步游标                      |
    +-- 增量同步完成 ----------------------+
```

#### 3.3.3 条目推送

```
本地节点 (Local)                     种子节点 (Seed)
    |                                     |
    |  1. PUSH_ENTRY                      |
    |     {entry, creator_signature}      |
    |  ================================>  |
    |  [种子节点: 验签, 验权限, 验哈希]     |
    |  2a. PUSH_ACK {accepted=true}       |
    |  <================================  |
    |  --- 或 ---                          |
    |  2b. PUSH_ACK {accepted=false,      |
    |       reason="permission denied"}   |
    |  <================================  |
    +--------------------------------------+
```

#### 3.3.4 种子节点间双向同步

```
种子节点A                           种子节点B
    |                                     |
    |  Phase 1: EXCHANGE                  |
    |  1. SYNC_REQUEST {version_vector}   |
    |  ================================>  |
    |  2. SYNC_RESPONSE {diffs}           |
    |  <================================  |
    |                                     |
    |  Phase 2: MERGE (双向)               |
    |  Phase 3: RESOLVE (LWW, updated_at) |
    |  Phase 4: VERIFY (BITFIELD交换)      |
    |  3. BITFIELD (A -> B)               |
    |  ================================>  |
    |  4. BITFIELD (B -> A)               |
    |  <================================  |
    +-- 同步周期完成 ----------------------+
```

### 3.4 连接管理

```go
// internal/network/connmgr/manager.go

type ConnectionManager struct {
    host        host.Host
    mu          sync.RWMutex
    peers       map[peer.ID]*PeerConnection
    seedAddrs   []multiaddr.Multiaddr
    maxRetries  int
}

type PeerConnection struct {
    PeerID      peer.ID
    NodeID      string
    NodeType    NodeType
    ConnectedAt time.Time
    LastActive  time.Time
    EntryCount  int64
    Categories  []string
}

func (cm *ConnectionManager) Start(ctx context.Context) error {
    for _, addr := range cm.seedAddrs {
        go cm.connectWithRetry(ctx, addr)
    }
    go cm.keepAlive(ctx)
    go cm.reconnectLoop(ctx)
    cm.host.Network().Notify(&network.NotifyBundle{
        ConnectedF:    cm.onConnected,
        DisconnectedF: cm.onDisconnected,
    })
    return nil
}

func (cm *ConnectionManager) connectWithRetry(ctx context.Context,
    addr multiaddr.Multiaddr) {
    backoff := time.Second * 5
    maxBackoff := time.Minute * 5
    for attempt := 0; attempt < cm.maxRetries; attempt++ {
        select {
        case <-ctx.Done():
            return
        default:
        }
        info, err := peer.AddrInfoFromP2pAddr(addr)
        if err != nil { continue }
        if err = cm.host.Connect(ctx, *info); err == nil {
            cm.sendHandshake(ctx, info.ID)
            return
        }
        time.Sleep(backoff)
        backoff = time.Duration(float64(backoff) * 1.5)
        if backoff > maxBackoff { backoff = maxBackoff }
    }
}
```

### 3.5 NAT 穿透策略

```
场景 1: 本地节点在 NAT 后面 (最常见)
  本地节点 (内网) --出站连接--> 种子节点 (公网)
  无需 NAT 穿透, 所有数据通过出站连接传输

场景 2: 本地节点需要被其他节点连接
  1. AutoNAT 检测 NAT 类型
  2. 尝试 UPnP/NAT-PMP 端口映射
  3. 失败则使用 Relay 中继 (通过种子节点)

场景 3: 种子节点 (公网IP)
  - 直接监听公网端口
  - 作为 DHT 引导节点
  - 可选作为 Relay 中继节点
  - 作为 AutoNAT 服务节点
```

### 3.6 消息编解码器

```go
// internal/network/protocol/codec.go

// 编码格式: [Header Length 4B][Header bytes][Payload bytes]

type Codec struct{}

type Message struct {
    Header  *protocol.MessageHeader
    Payload proto.Message
}

func (c *Codec) Encode(msg *Message) ([]byte, error) {
    headerBytes, _ := proto.Marshal(msg.Header)
    payloadBytes, _ := proto.Marshal(msg.Payload)
    buf := make([]byte, 4+len(headerBytes)+len(payloadBytes))
    binary.BigEndian.PutUint32(buf[:4], uint32(len(headerBytes)))
    copy(buf[4:4+len(headerBytes)], headerBytes)
    copy(buf[4+len(headerBytes):], payloadBytes)
    return buf, nil
}

func (c *Codec) Decode(r io.Reader) (*Message, error) {
    lenBuf := make([]byte, 4)
    io.ReadFull(r, lenBuf)
    headerLen := binary.BigEndian.Uint32(lenBuf)
    headerBytes := make([]byte, headerLen)
    io.ReadFull(r, headerBytes)
    header := &protocol.MessageHeader{}
    proto.Unmarshal(headerBytes, header)
    payloadBytes, _ := io.ReadAll(io.LimitReader(r, 64*1024*1024))
    payload, _ := c.unmarshalPayload(header.Type, payloadBytes)
    return &Message{Header: header, Payload: payload}, nil
}
```

---

## 4. API层设计

### 4.1 REST API 端点规范

| 项目 | 值 |
|------|-----|
| 基础路径 | `http://localhost:{api_port}/api/v1` |
| 默认端口 | 18531 |
| 数据格式 | JSON |
| 字符编码 | UTF-8 |
| 认证方式 | Ed25519 签名 (Header) |

| 方法 | 路径 | 说明 | 认证 | 最低权限 |
|------|------|------|------|----------|
| `GET` | `/search` | 搜索知识条目 | 可选 | Lv0 |
| `GET` | `/entry/{id}` | 获取条目详情 | 可选 | Lv0 |
| `POST` | `/entry` | 创建新条目 | 必须 | Lv1 |
| `PUT` | `/entry/{id}` | 更新条目 | 必须 | Lv1 |
| `DELETE` | `/entry/{id}` | 删除条目 | 必须 | Lv1(创建者)/Lv4 |
| `POST` | `/entry/{id}/rate` | 评分 | 必须 | Lv1 |
| `GET` | `/categories` | 分类列表 | 可选 | Lv0 |
| `GET` | `/categories/{path}/entries` | 分类下条目 | 可选 | Lv0 |
| `POST` | `/categories` | 创建新分类 | 必须 | Lv2 |
| `GET` | `/node/status` | 节点状态 | 可选 | Lv0 |
| `POST` | `/node/sync` | 手动同步 | 必须 | Lv0 |
| `POST` | `/user/register` | 注册 | 可选 | 无 |
| `POST` | `/user/verify-email` | 验证邮箱 | 可选 | Lv0 |
| `GET` | `/user/info` | 用户信息 | 必须 | Lv0 |
| `PUT` | `/user/info` | 更新用户信息 | 必须 | Lv0 |

### 4.2 认证机制

请求签名 Header:

```
X-Polyant-PublicKey: {base64_ed25519_public_key}
X-Polyant-Timestamp: {unix_milliseconds}
X-Polyant-Signature: {base64_ed25519_signature}
```

签名内容: `METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + SHA256(BODY)`

```go
// internal/auth/signature.go

func SignRequest(privateKey ed25519.PrivateKey, method, path string,
    timestamp int64, body []byte) ([]byte, error) {
    bodyHash := sha256.Sum256(body)
    signContent := fmt.Sprintf("%s\n%s\n%d\n%s",
        method, path, timestamp, hex.EncodeToString(bodyHash[:]))
    return ed25519.Sign(privateKey, []byte(signContent)), nil
}

func VerifyRequest(publicKey ed25519.PublicKey, method, path string,
    timestamp int64, body []byte, signature []byte) bool {
    now := time.Now().UnixMilli()
    if abs(now-timestamp) > 5*60*1000 { return false }
    bodyHash := sha256.Sum256(body)
    signContent := fmt.Sprintf("%s\n%s\n%d\n%s",
        method, path, timestamp, hex.EncodeToString(bodyHash[:]))
    return ed25519.Verify(publicKey, []byte(signContent), signature)
}
```

### 4.3 请求/响应格式

#### 搜索条目

```
GET /api/v1/search?q=go并发编程&cat=computer-science/programming-languages/go&limit=10&offset=0
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `q` | string | 是 | 搜索关键词 |
| `cat` | string | 否 | 分类路径 |
| `tag` | string | 否 | 标签 (逗号分隔) |
| `limit` | int | 否 | 默认20, 最大100 |
| `offset` | int | 否 | 默认0 |
| `min_score` | float | 否 | 最低评分 |

响应:

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "total_count": 42,
    "has_more": true,
    "entries": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "title": "Go 并发编程指南",
        "content": "# Go 并发编程\n\n...",
        "json_data": [{"type": "skill_definition", "name": "go_concurrent"}],
        "category": "computer-science/programming-languages/go",
        "tags": ["go", "concurrency"],
        "version": 3,
        "created_at": 1712548800000,
        "updated_at": 1712635200000,
        "created_by": "a1b2c3d4...",
        "score": 4.7,
        "score_count": 23,
        "content_hash": "sha256:abcdef...",
        "status": "active",
        "license": "CC BY-SA 4.0"
      }
    ]
  }
}
```

#### 创建条目

```
POST /api/v1/entry
X-Polyant-PublicKey: {key}
X-Polyant-Timestamp: {ts}
X-Polyant-Signature: {sig}
```

```json
{
  "title": "Rust 所有权系统详解",
  "content": "# Rust 所有权系统\n\n...",
  "json_data": [{"type": "skill_definition", "name": "rust_ownership"}],
  "category": "computer-science/programming-languages/rust",
  "tags": ["rust", "ownership"]
}
```

成功响应:

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "version": 1,
    "created_at": 1712635200000,
    "content_hash": "sha256:fedcba..."
  }
}
```

### 4.4 统一响应格式

```go
// internal/api/types/response.go

type APIResponse struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

type PagedData struct {
    TotalCount int         `json:"total_count"`
    HasMore    bool        `json:"has_more"`
    Items      interface{} `json:"items"`
}
```

### 4.5 错误码定义

| 错误码 | HTTP | 说明 |
|--------|------|------|
| `0` | 200 | 成功 |
| `10000` | 500 | 内部服务器错误 |
| `10001` | 503 | 服务不可用 |
| `10002` | 429 | 请求过于频繁 |
| `10100` | 400 | 参数缺失 |
| `10102` | 400 | JSON 解析失败 |
| `10103` | 400 | 评分值超出 1.0-5.0 |
| `10200` | 401 | 缺少认证信息 |
| `10201` | 401 | 签名验证失败 |
| `10202` | 401 | 时间戳过期 |
| `10300` | 403 | 权限不足 |
| `10301` | 403 | 基础用户无法执行此操作 |
| `10303` | 403 | 用户已被暂停 |
| `10400` | 404 | 条目不存在 |
| `10403` | 409 | 条目已存在 |
| `10404` | 409 | 重复评分 |
| `10500` | 502 | 无法连接种子节点 |
| `10502` | 500 | 同步数据校验失败 |
| `10600` | 500 | 存储层写入失败 |

### 4.6 路由与中间件

```go
// internal/api/router/router.go

func NewRouter(deps *api.Dependencies) *chi.Mux {
    r := chi.NewRouter()
    r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
    r.Use(middleware.Timeout(30 * time.Second))
    r.Use(middlewareLogger, middlewareRateLimit, middlewareCORS)

    // 公开路由
    r.Group(func(r chi.Router) {
        r.Get("/api/v1/search", deps.SearchHandler.Search)
        r.Get("/api/v1/entry/{id}", deps.EntryHandler.Get)
        r.Get("/api/v1/categories", deps.CategoryHandler.List)
        r.Get("/api/v1/categories/{path}/entries", deps.CategoryHandler.Entries)
        r.Get("/api/v1/node/status", deps.NodeHandler.Status)
        r.Post("/api/v1/user/register", deps.UserHandler.Register)
        r.Post("/api/v1/user/verify-email", deps.UserHandler.VerifyEmail)
    })

    // 认证路由
    r.Group(func(r chi.Router) {
        r.Use(middlewareAuth(deps.AuthService))
        r.Post("/api/v1/entry", deps.EntryHandler.Create)
        r.Put("/api/v1/entry/{id}", deps.EntryHandler.Update)
        r.Delete("/api/v1/entry/{id}", deps.EntryHandler.Delete)
        r.Post("/api/v1/entry/{id}/rate", deps.RatingHandler.Rate)
        r.Post("/api/v1/categories", deps.CategoryHandler.Create)
        r.Post("/api/v1/node/sync", deps.NodeHandler.Sync)
        r.Get("/api/v1/user/info", deps.UserHandler.Info)
        r.Put("/api/v1/user/info", deps.UserHandler.Update)
    })
    return r
}
```

---

## 5. 核心算法设计

### 5.1 搜索排序算法

综合排序公式:

```
final_score = text_relevance * 0.4 + entry_score * 0.4 + recency_score * 0.2
```

| 因子 | 权重 | 说明 |
|------|------|------|
| `text_relevance` | 0.4 | Bleve 相关度 (归一化到 0-1) |
| `entry_score` | 0.4 | 条目评分 (除以5.0归一化) |
| `recency_score` | 0.2 | 时效性 (半衰期180天衰减) |

```go
// internal/core/search/ranking.go

func RecencyScore(updatedAt int64) float64 {
    now := time.Now().UnixMilli()
    ageDays := float64(now-updatedAt) / (24 * 60 * 60 * 1000)
    return math.Pow(0.5, ageDays/180.0)
}

func FinalScore(textRelevance float64, entryScore float64, updatedAt int64) float64 {
    normRelevance := math.Min(textRelevance/100.0, 1.0)
    normScore := entryScore / 5.0
    recency := RecencyScore(updatedAt)
    return normRelevance*0.4 + normScore*0.4 + recency*0.2
}

// MergeResults 合并本地和远程结果, 去重后按 final_score 降序
func MergeResults(local, remote []*SearchHit, limit int) []*SearchHit {
    seen := make(map[string]struct{})
    all := append(local, remote...)
    sort.Slice(all, func(i, j int) bool {
        return all[i].FinalScore > all[j].FinalScore
    })
    var merged []*SearchHit
    for _, hit := range all {
        if _, exists := seen[hit.Entry.ID]; !exists {
            seen[hit.Entry.ID] = struct{}{}
            merged = append(merged, hit)
            if len(merged) >= limit { break }
        }
    }
    return merged
}
```

### 5.2 用户层级晋升算法

| 层级 | 条件 | 审核方式 |
|------|------|----------|
| Lv0 -> Lv1 | 邮箱验证 | 自动 |
| Lv1 -> Lv2 | `contribution_cnt >= 10` 且 `rating_cnt >= 20` | 自动 |
| Lv2 -> Lv3 | `contribution_cnt >= 50` 且 `rating_cnt >= 100` | 自动 |
| Lv3 -> Lv4 | `contribution_cnt >= 200` 且 `rating_cnt >= 500` | 自动 |
| Lv4 -> Lv5 | Lv4 用户投票选举 | 人工审核 |

```go
// internal/core/user/level.go

func (m *UserManager) CheckLevelUpgrade(ctx context.Context,
    user *model.User) (model.UserLevel, bool) {
    newLevel := user.UserLevel
    switch user.UserLevel {
    case model.UserLevelVerified:
        if user.ContributionCnt >= 10 && user.RatingCnt >= 20 {
            newLevel = model.UserLevelActive
        }
    case model.UserLevelActive:
        if user.ContributionCnt >= 50 && user.RatingCnt >= 100 {
            newLevel = model.UserLevelSenior
        }
    case model.UserLevelSenior:
        if user.ContributionCnt >= 200 && user.RatingCnt >= 500 {
            newLevel = model.UserLevelExpert
        }
    }
    if newLevel > user.UserLevel {
        user.UserLevel = newLevel
        m.store.PutUser(ctx, user)
        return newLevel, true
    }
    return user.UserLevel, false
}

func GetLevelWeight(level model.UserLevel) float64 {
    switch level {
    case model.UserLevelBasic:    return 0.0
    case model.UserLevelVerified: return 1.0
    case model.UserLevelActive:   return 1.2
    case model.UserLevelSenior:   return 1.5
    case model.UserLevelExpert:   return 2.0
    case model.UserLevelCore:     return 3.0
    default: return 0.0
    }
}
```

### 5.3 加权评分计算

公式: `entry_score = sum(weighted_score) / sum(weight)`

```go
// internal/core/rating/calculator.go

func (rc *RatingCalculator) SubmitRating(ctx context.Context,
    entryID string, rater *model.User, score float64,
    comment string) (*model.Rating, error) {

    if score < 1.0 || score > 5.0 { return nil, ErrScoreOutOfRange }
    if rater.UserLevel < model.UserLevelVerified { return nil, ErrPermissionDenied }

    // 检查重复评分
    existing, _ := rc.store.ListRatingsByEntry(ctx, entryID)
    for _, r := range existing {
        if r.RaterPubkeyHash == rater.PublicKeyHash { return nil, ErrDuplicateRating }
    }

    weight := GetLevelWeight(rater.UserLevel)
    rating := &model.Rating{
        ID: uuid.New().String(), EntryID: entryID,
        RaterPubkeyHash: rater.PublicKeyHash,
        Score: score, Weight: weight, WeightedScore: score * weight,
        RatedAt: time.Now().UnixMilli(), Comment: comment,
    }
    rating.RaterSignature = rc.signRating(rating, rater)
    rc.store.PutRating(ctx, rating)

    // 重算条目综合评分
    newScore := rc.RecalculateEntryScore(ctx, entryID)
    entry, _ := rc.store.GetEntry(ctx, entryID)
    entry.Score = newScore
    entry.ScoreCount = int32(len(existing) + 1)
    rc.store.PutEntry(ctx, entry)

    // 更新评分者计数, 检查晋升
    rater.RatingCnt++
    rc.store.PutUser(ctx, rater)
    rc.userMgr.CheckLevelUpgrade(ctx, rater)

    return rating, nil
}

func (rc *RatingCalculator) RecalculateEntryScore(
    ctx context.Context, entryID string) float64 {
    ratings, _ := rc.store.ListRatingsByEntry(ctx, entryID)
    var totalWS, totalW float64
    for _, r := range ratings {
        totalWS += r.WeightedScore
        totalW += r.Weight
    }
    if totalW == 0 { return 0.0 }
    return totalWS / totalW
}
```

### 5.4 增量同步算法 (版本向量)

```go
// internal/network/sync/version_vector.go

type VersionVector map[string]int64

func (vv VersionVector) Increment(entryID string) int64 {
    vv[entryID]++
    return vv[entryID]
}

func (vv VersionVector) Merge(other VersionVector) VersionVector {
    result := make(VersionVector)
    for k, v := range vv { result[k] = v }
    for k, v := range other {
        if v > result[k] { result[k] = v }
    }
    return result
}

func (vv VersionVector) Diff(other VersionVector) []string {
    var needed []string
    for id, theirV := range other {
        if theirV > vv[id] { needed = append(needed, id) }
    }
    return needed
}
```

```go
// internal/network/sync/incremental.go

func (se *SyncEngine) IncrementalSync(ctx context.Context,
    peerID peer.ID) (*SyncResult, error) {

    localVV, _ := se.store.GetAllVersions(ctx)
    request := &protocol.SyncRequest{
        RequestId: uuid.New().String(),
        VersionVector: localVV.Serialize(),
    }
    response, _ := se.protocol.SendSyncRequest(ctx, peerID, request)

    // 处理新条目
    for _, entry := range response.NewEntries {
        if entry.Version > localVV.Get(entry.Id) && verifyContentHash(entry) {
            se.store.PutEntry(ctx, entry)
            se.index.IndexEntry(entry)
            localVV.Increment(entry.Id)
        }
    }

    // 处理更新 (Last Write Wins)
    for _, entry := range response.UpdatedEntries {
        local, _ := se.store.GetEntry(ctx, entry.Id)
        if local != nil && entry.UpdatedAt > local.UpdatedAt {
            if verifyContentHash(entry) {
                se.store.PutEntry(ctx, entry)
                se.index.UpdateIndex(entry)
            }
        }
    }

    // 处理删除
    for _, id := range response.DeletedEntryIds {
        se.store.DeleteEntry(ctx, id)
        se.index.DeleteIndex(id)
        delete(localVV, id)
    }

    // 合并版本向量
    serverVV := Deserialize(response.ServerVersionVector)
    for id, v := range localVV.Merge(serverVV) {
        se.store.PutVersion(ctx, id, v)
    }
    return result, nil
}
```

同步状态机:

```
IDLE -> PREPARE -> EXCHANGE -> MERGE -> RESOLVE -> VERIFY -> COMPLETE -> IDLE
```

---

## 6. 配置管理设计

### 6.1 配置文件格式 (YAML)

```yaml
# polyant.yaml
# 优先级: 命令行参数 > 环境变量 > 配置文件 > 默认值

node:
  type: local                    # local | seed
  name: "my-agent-node"
  data_dir: "~/.polyant/data"
  log_dir: "~/.polyant/logs"
  log_level: "info"              # debug | info | warn | error

network:
  listen_port: 18530
  api_port: 18531
  api_listen: "127.0.0.1"
  seed_nodes:
    - "/dns4/seed1.polyant.org/tcp/18530/p2p/QmSeedNode1..."
  dht_enabled: true
  mdns_enabled: true
  nat_enabled: true
  relay_enabled: true
  connect_timeout: 30
  message_timeout: 60

sync:
  auto_sync: true
  interval_seconds: 300
  mirror_categories: []
  max_local_size_mb: 500
  compression: "zstd"
  batch_size: 100
  parallel_downloads: 3

sharing:
  allow_mirror: false
  bandwidth_limit_mb: 10
  max_concurrent: 3
  allowed_categories: ["*"]

user:
  private_key_path: "~/.polyant/keys/ed25519_private.key"
  email: ""
  auto_register: true
  agent_name: ""

seed:
  enabled: false
  smtp:
    host: ""
    port: 587
    username: ""
    password: ""
    from_address: "noreply@polyant.org"
    tls: true
  init_data_dir: "./configs/seed-data"
  seed_sync_interval: 300
  max_connections: 1000

search:
  default_limit: 20
  max_limit: 100
  cache_ttl: 300
  min_keyword_length: 2

rate_limit:
  enabled: true
  requests_per_minute: 60
  search_per_minute: 30
  write_per_minute: 10
  burst: 10
```

### 6.2 默认值与环境变量覆盖

| 环境变量 | 配置项 | 默认值 |
|----------|--------|--------|
| `AW_NODE_TYPE` | `node.type` | `local` |
| `AW_NODE_NAME` | `node.name` | `polyant-node` |
| `AW_NODE_DATA_DIR` | `node.data_dir` | `~/.polyant/data` |
| `AW_NODE_LOG_LEVEL` | `node.log_level` | `info` |
| `AW_NETWORK_LISTEN_PORT` | `network.listen_port` | `18530` |
| `AW_NETWORK_API_PORT` | `network.api_port` | `18531` |
| `AW_NETWORK_SEED_NODES` | `network.seed_nodes` | `[]` |
| `AW_SYNC_AUTO_SYNC` | `sync.auto_sync` | `true` |
| `AW_SYNC_INTERVAL_SECONDS` | `sync.interval_seconds` | `300` |
| `AW_SEED_ENABLED` | `seed.enabled` | `false` |
| `AW_SEED_SMTP_HOST` | `seed.smtp.host` | `""` |
| `AW_SEED_SMTP_PORT` | `seed.smtp.port` | `587` |

```go
// pkg/config/loader.go

func Load(configPath string) (*Config, error) {
    cfg := DefaultConfig()
    if configPath != "" {
        data, _ := os.ReadFile(configPath)
        yaml.Unmarshal(data, cfg)
    }
    cfg.applyEnvOverrides()
    return cfg, cfg.Validate()
}
```

---

## 7. 错误处理设计

### 7.1 错误码体系

```
错误码结构: AABBB (AA=模块, BBB=序号)

00 = 系统错误    01 = API错误     02 = 认证错误
03 = 存储错误    04 = 网络错误    05 = 同步错误
06 = 搜索错误    07 = 评分错误    08 = 用户错误
09 = 配置错误
```

```go
// pkg/errors/errors.go

type AWError struct {
    Code       int
    Category   ErrorCategory
    Message    string
    HTTPStatus int
    Cause      error
    Retryable  bool
}

func (e *AWError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AWError) Unwrap() error { return e.Cause }

// 预定义错误
var (
    ErrInternal          = New(0, CategorySystem, "internal error", 500)
    ErrRateLimited       = New(2, CategorySystem, "rate limited", 429)
    ErrInvalidParams     = New(100, CategoryAPI, "invalid params", 400)
    ErrMissingAuth       = New(200, CategoryAuth, "missing auth", 401)
    ErrInvalidSignature  = New(201, CategoryAuth, "invalid signature", 401)
    ErrPermissionDenied  = New(203, CategoryAuth, "permission denied", 403)
    ErrEntryNotFound     = New(300, CategoryStorage, "entry not found", 404)
    ErrDuplicateRating   = New(303, CategoryStorage, "duplicate rating", 409)
    ErrPeerConnectFailed = New(400, CategoryNetwork, "peer connect failed", 502)
    ErrSyncFailed        = New(500, CategorySync, "sync failed", 500)
    ErrHashMismatch      = New(502, CategorySync, "hash mismatch", 500)
)
```

### 7.2 日志策略

- **格式**: JSON (便于 ELK/Loki 等工具处理)
- **字段**: `time`, `level`, `msg`, `caller`
- **轮转**: 按大小轮转 (100MB/文件), 保留30天, gzip压缩
- **敏感信息**: 私钥、签名不出现在日志中
- **请求日志**: method, path, status_code, latency, user_id
- **同步日志**: peer_id, new_entries, duration

```go
// pkg/logger/logger.go
// 使用 zap + lumberjack 实现
func New(cfg Config) (Logger, error) {
    encoderConfig := zapcore.EncoderConfig{
        TimeKey: "time", LevelKey: "level", MessageKey: "msg",
        EncodeTime: zapcore.ISO8601TimeEncoder,
        EncodeLevel: zapcore.LowercaseLevelEncoder,
    }
    writer := &lumberjack.Logger{
        Filename: filepath.Join(cfg.LogDir, cfg.Filename),
        MaxSize: 100, MaxBackups: 30, MaxAge: 30, Compress: true,
    }
    core := zapcore.NewCore(
        zapcore.NewJSONEncoder(encoderConfig),
        zapcore.NewMultiWriteSyncer(
            zapcore.AddSync(writer), zapcore.AddSync(os.Stdout)),
        parseLevel(cfg.Level))
    return zap.New(core, zap.AddCaller(),
        zap.AddStacktrace(zapcore.ErrorLevel)).Sugar(), nil
}
```

### 7.3 优雅关闭

```go
// internal/service/shutdown.go

func registerShutdownComponents(sm *ShutdownManager, app *App) {
    // 按优先级逆序关闭
    sm.Register("api_server", func(ctx context.Context) error {
        return app.apiServer.Shutdown(ctx)
    }, 100)
    sm.Register("sync_engine", func(ctx context.Context) error {
        return app.syncEngine.Stop()
    }, 90)
    sm.Register("background_tasks", func(ctx context.Context) error {
        app.scheduler.Stop(); return nil
    }, 80)
    sm.Register("p2p_network", func(ctx context.Context) error {
        return app.host.Close()
    }, 70)
    sm.Register("search_engine", func(ctx context.Context) error {
        return app.searchEngine.Close()
    }, 60)
    sm.Register("storage", func(ctx context.Context) error {
        return app.store.Close()
    }, 50)
    sm.Register("logger", func(ctx context.Context) error {
        return app.logger.Sync()
    }, 10)
}
```

---

## 8. 安全性设计

### 8.1 Ed25519 密钥管理

```go
// internal/auth/ed25519/keymanager.go

type KeyManager struct {
    privateKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
    keyPath    string
}

func NewKeyManager(keyPath string) (*KeyManager, error) {
    km := &KeyManager{keyPath: keyPath}
    os.MkdirAll(filepath.Dir(keyPath), 0700)
    if _, err := os.Stat(keyPath); err == nil {
        return km, km.load()
    }
    return km, km.generate()
}

func (km *KeyManager) generate() error {
    pub, priv, _ := ed25519.GenerateKey(rand.Reader)
    km.privateKey, km.publicKey = priv, pub
    os.WriteFile(km.keyPath, []byte(base64.StdEncoding.EncodeToString(priv)), 0600)
    os.WriteFile(km.keyPath+".pub", []byte(base64.StdEncoding.EncodeToString(pub)), 0644)
    return nil
}
```

安全要求:

| 项目 | 要求 |
|------|------|
| 私钥权限 | `0600` |
| 目录权限 | `0700` |
| 存储格式 | Base64 |
| 传输 | 仅传公钥 |

### 8.2 Noise 加密

由 go-libp2p security/noise 自动处理:
- 算法: Noise_XX_25519_ChaChaPoly_SHA256
- 前向保密: 每次连接使用临时密钥对
- 身份验证: 静态密钥签名确认

### 8.3 速率限制

```go
// internal/api/middleware/ratelimit.go
// 使用 golang.org/x/time/rate 令牌桶算法
// 按用户公钥哈希 + IP 双维度限流
// 全局: 60 req/min, 搜索: 30 req/min, 写入: 10 req/min
// 定期清理过期限流器防止内存泄漏
```

### 8.4 数据完整性验证

```
创建时: SHA256(title+content+category) -> content_hash
        Ed25519_Sign(privkey, content_hash) -> creator_signature
        存储: entry + content_hash + creator_signature

验证时: 重新计算 content_hash 并比对
        用创建者公钥验证 creator_signature
        双重验证通过才写入
```

### 8.5 RBAC 权限控制

```go
// internal/auth/rbac/rbac.go

var LevelPermissions = map[model.UserLevel][]Permission{
    model.UserLevelBasic:    {PermRead, PermSearch, PermMirror},
    model.UserLevelVerified: {PermRead, PermSearch, PermMirror, PermWrite, PermRate},
    model.UserLevelActive:   {PermRead, PermSearch, PermMirror, PermWrite, PermRate, PermSuggestCategory},
    model.UserLevelSenior:   {PermRead, PermSearch, PermMirror, PermWrite, PermRate, PermSuggestCategory, PermManageCategory},
    model.UserLevelExpert:   {PermRead, PermSearch, PermMirror, PermWrite, PermRate, PermSuggestCategory, PermManageCategory, PermAuditEntry, PermManageUser},
    model.UserLevelCore:     {PermRead, PermSearch, PermMirror, PermWrite, PermRate, PermSuggestCategory, PermManageCategory, PermAuditEntry, PermManageUser, PermSystemAdmin},
}
```

---

## 9. 部署设计

### 9.1 跨平台构建 (Makefile)

```makefile
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
BUILD_DIR := ./bin
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all build cross-compile test lint proto clean

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/polyant ./cmd/polyant/
	go build $(LDFLAGS) -o $(BUILD_DIR)/awctl ./cmd/awctl/

cross-compile:
	@mkdir -p $(BUILD_DIR)
	@for p in $(PLATFORMS); do \
		GOOS=$${p%/*}; GOARCH=$${p#*/}; \
		out=$(BUILD_DIR)/polyant-$${GOOS}-$${GOARCH}; \
		[ "$${GOOS}" = "windows" ] && out=$${out}.exe; \
		echo "Building $$out..."; \
		GOOS=$${GOOS} GOARCH=$${GOARCH} go build $(LDFLAGS) -o $$out ./cmd/polyant/; \
	done

test:
	go test -v -race -coverprofile=coverage.out ./...

lint: vet fmt-check
vet: go vet ./...
fmt: gofmt -s -w .
fmt-check: @test -z "$$(gofmt -s -l . | tee /dev/stderr)"

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--proto_path=pkg/proto pkg/proto/model.proto pkg/proto/protocol.proto

clean: rm -rf $(BUILD_DIR)

docker-build:
	docker build -t polyant:$(VERSION) -f Dockerfile.seed .
```

### 9.2 systemd 配置 (Linux)

```ini
# configs/systemd/polyant.service
[Unit]
Description=Polyant Distributed Knowledge Base
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=polyant
Group=polyant
WorkingDirectory=/var/lib/polyant
ExecStart=/usr/local/bin/polyant --config /etc/polyant/polyant.yaml
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/polyant /var/log/polyant
LimitNOFILE=65536
MemoryMax=512M

[Install]
WantedBy=multi-user.target
```

种子节点 systemd (更多资源):

```ini
# configs/systemd/polyant-seed.service
[Service]
ExecStart=/usr/local/bin/polyant --config /etc/polyant/polyant-seed.yaml
Restart=always
RestartSec=3
LimitNOFILE=131072
MemoryMax=2G
CPUQuota=400%
```

### 9.3 launchd 配置 (macOS)

```xml
<!-- configs/launchd/com.polyant.polyant.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.polyant.polyant</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/polyant</string>
        <string>--config</string>
        <string>/etc/polyant/polyant.yaml</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/var/lib/polyant</string>
    <key>UserName</key>
    <string>_polyant</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/polyant/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/polyant/stderr.log</string>
    <key>SoftResourceLimits</key>
    <dict>
        <key>NumberOfFiles</key>
        <integer>65536</integer>
    </dict>
</dict>
</plist>
```

### 9.4 Windows Service 配置

```go
// internal/service/daemon/service_windows.go
// 使用 kardianos/service 库

func newSystemService(cfg *config.Config) (service.Service, error) {
    svcConfig := &service.Config{
        Name:        "Polyant",
        DisplayName: "Polyant Distributed Knowledge Base",
        Description: "P2P distributed knowledge base for AI agents",
        Arguments:   []string{"--config", cfg.ConfigPath},
    }

    prg := &program{cfg: cfg}
    return service.New(prg, svcConfig)
}

type program struct {
    cfg    *config.Config
    server *App
}

func (p *program) Start(s service.Service) error {
    go p.run()
    return nil
}

func (p *program) run() {
    app, _ := NewApp(p.cfg)
    app.Run()
}

func (p *program) Stop(s service.Service) error {
    // 优雅关闭
    return nil
}
```

安装命令:

```powershell
# 安装服务
polyant.exe install --config C:\Polyant\polyant.yaml
# 启动服务
polyant.exe start
# 停止服务
polyant.exe stop
# 卸载服务
polyant.exe uninstall
```

### 9.5 Docker 支持 (种子节点)

```dockerfile
# Dockerfile.seed
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git protobuf

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make proto
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w" -o /polyant ./cmd/polyant/

# --- 运行镜像 ---
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -g 1000 polyant && \
    adduser -u 1000 -G polyant -s /bin/sh -D polyant

WORKDIR /var/lib/polyant
COPY --from=builder /polyant /usr/local/bin/polyant
COPY configs/seed-data/ /var/lib/polyant/seed-data/

RUN mkdir -p /var/lib/polyant/data \
             /var/lib/polyant/keys \
             /var/lib/polyant/logs \
             /etc/polyant

COPY configs/polyant-seed.yaml /etc/polyant/polyant.yaml

EXPOSE 18530 18531

USER polyant
ENTRYPOINT ["polyant"]
CMD ["--config", "/etc/polyant/polyant.yaml"]
```

```yaml
# docker-compose.yml (种子节点集群)
version: '3.8'

services:
  seed-node-1:
    build:
      context: .
      docker_file: Dockerfile.seed
    container_name: polyant-seed-1
    ports:
      - "18530:18530"
      - "18531:18531"
    volumes:
      - seed1-data:/var/lib/polyant/data
      - seed1-keys:/var/lib/polyant/keys
    environment:
      - AW_NODE_TYPE=seed
      - AW_SEED_ENABLED=true
      - AW_NETWORK_SEED_NODES=
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2.0'

  seed-node-2:
    build:
      context: .
      docker_file: Dockerfile.seed
    container_name: polyant-seed-2
    ports:
      - "18532:18530"
      - "18533:18531"
    volumes:
      - seed2-data:/var/lib/polyant/data
      - seed2-keys:/var/lib/polyant/keys
    environment:
      - AW_NODE_TYPE=seed
      - AW_SEED_ENABLED=true
      - AW_NETWORK_SEED_NODES=/dns4/seed-node-1/tcp/18530/p2p/QmSeedNode1...
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2.0'

volumes:
  seed1-data:
  seed1-keys:
  seed2-data:
  seed2-keys:
```

### 9.6 部署检查清单

| 检查项 | 本地节点 | 种子节点 |
|--------|---------|---------|
| 创建用户 `polyant` | 推荐 | 必须 |
| 创建数据目录 | 自动 | 手动 |
| 配置文件部署 | 自动生成 | 手动配置 |
| Ed25519 密钥生成 | 自动 | 手动/首次启动 |
| libp2p 节点密钥 | 自动 | 手动/首次启动 |
| 初始数据导入 | 内置最小知识库 | 手动导入 |
| 防火墙开放 18530 | 不需要 | 必须 |
| 防火墙开放 18531 | 不需要 | 可选 |
| systemd/launchd 安装 | 推荐 | 必须 |
| SMTP 配置 | 不需要 | 必须 |
| SSL/TLS 证书 | 不需要 | 推荐 |

---

> **文档结束**
>
> 本文档为 Polyant 分布式百科知识库系统的完整技术设计文档，涵盖系统架构、存储层、网络层、API层、核心算法、配置管理、错误处理、安全性和部署设计。Go 开发人员可依据本文档直接开始编码实现。
