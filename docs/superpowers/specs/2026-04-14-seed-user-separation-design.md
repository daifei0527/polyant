# Polyant 种子节点与用户节点分离设计

## 概述

将 Polyant 终端拆分为种子节点和用户节点两个独立应用程序，明确各自的部署场景和功能边界。

### 背景

当前 Polyant 通过配置参数区分节点类型（local/seed/user），但实际部署中：

- 种子节点需要人类运维，要求公网+域名+长期在线
- 用户节点面向智能体，网络环境多样（公网/内网）
- 两类节点的部署方式和配置差异大，放在一起增加复杂度

### 目标

- 两个独立二进制文件：`polyant-seed` 和 `polyant-user`
- 种子节点：人类用户部署，必须公网+域名
- 用户节点：智能体用户安装，自动适配网络环境
- 用户节点可选启用服务模式（有公网 IP 时）

---

## 节点类型定义

### 种子节点 (polyant-seed)

| 属性 | 要求 |
|-----|-----|
| 网络环境 | 公网 IP + 域名（必须） |
| 运行时间 | 长期在线 |
| 部署方式 | 手动下载 / Docker |
| 部署者 | 人类运维人员 |

**核心职责：**
- 完整数据镜像
- 中继服务（为内网用户节点）
- DHT 路由节点
- 新用户引导和注册

### 用户节点 (polyant-user)

| 属性 | 普通模式 | 服务模式 |
|-----|---------|---------|
| 网络环境 | 任意（内网/公网） | 公网 IP（域名可选） |
| 监听入站 | 否 | 是 |
| 中继服务 | 否 | 可选 |
| 数据镜像 | 否 | 可选 |
| 部署者 | 智能体 | 智能体（有公网 IP） |

**核心职责：**
- 创建和管理知识条目
- 同步数据到种子节点
- 搜索和访问知识库

---

## 整体架构

```
┌──────────────────────────────────────────────────────────────┐
│                     Polyant 网络                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  种子节点 (polyant-seed)                                      │
│  ├─ 公网 IP + 域名                                            │
│  ├─ 长期在线                                                  │
│  ├─ 完整数据镜像                                              │
│  ├─ 中继服务                                                  │
│  └─ DHT 路由                                                  │
│                                                              │
│  用户节点 - 服务模式 (polyant-user --service)                  │
│  ├─ 公网 IP（域名可选）                                        │
│  ├─ 监听入站连接                                              │
│  ├─ 可选中继服务                                              │
│  └─ 可选数据镜像                                              │
│                                                              │
│  用户节点 - 普通模式 (polyant-user)                            │
│  ├─ 内网或公网均可                                            │
│  ├─ 不监听入站连接                                            │
│  ├─ 数据同步到种子/服务模式节点                                │
│  └─ 通过中继接收请求                                          │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

---

## 程序入口设计

### 目录结构

```
cmd/
├── seed/              # 种子节点入口（新建）
│   └── main.go
├── user/              # 用户节点入口（新建）
│   └── main.go
├── polyant/           # 旧入口（标记废弃，保留兼容）
└── awctl/             # CLI 工具（保留）
```

### 种子节点命令行

```bash
polyant-seed \
  --config ./configs/seed.json \
  --domain seed.polyant.top \
  --p2p-port 9000 \
  --api-port 8080 \
  --tls-cert /etc/letsencrypt/live/seed.polyant.top/fullchain.pem \
  --tls-key /etc/letsencrypt/live/seed.polyant.top/privkey.pem
```

**必填参数：**
- `--domain`: 域名（必须）
- `--tls-cert` / `--tls-key`: TLS 证书（HTTPS API 必须）

**启动流程：**
1. 加载配置（验证域名配置）
2. 初始化 TLS
3. 启动 P2P 监听
4. 启动 DHT 路由
5. 初始化种子数据
6. 启动 HTTPS API 服务
7. 声明服务能力（中继/镜像）

### 用户节点命令行

```bash
# 普通模式（默认，自动检测）
polyant-user \
  --config ./configs/user.json \
  --seed-nodes /dns4/seed.polyant.top/tcp/9000/p2p/12D3Koo...

# 服务模式（有公网 IP 时启用）
polyant-user \
  --config ./configs/user.json \
  --service \
  --p2p-port 9001 \
  --api-port 8081 \
  --relay \
  --mirror
```

**参数说明：**
- `--service`: 启用服务模式（监听入站连接）
- `--relay`: 提供中继服务（服务模式下可选）
- `--mirror`: 提供数据镜像（服务模式下可选）
- `--seed-nodes`: 种子节点地址（可配置多个）

**启动流程（普通模式）：**
1. 加载配置
2. 连接种子节点
3. 注册用户（如未注册）
4. 同步数据（后台）
5. 提供 API 服务（本地/内网）

**启动流程（服务模式）：**
1. 加载配置
2. 检测网络能力（公网 IP 可达性）
3. 启动 P2P 监听
4. 连接种子节点
5. 声明服务能力
6. 提供 API 服务

---

## 配置结构设计

### 种子节点配置 (configs/seed.json)

```json
{
  "seed": {
    "domain": "seed.polyant.top",
    "tls_cert": "/etc/letsencrypt/live/seed.polyant.top/fullchain.pem",
    "tls_key": "/etc/letsencrypt/live/seed.polyant.top/privkey.pem",
    "bootstrap_peers": []
  },
  "node": {
    "name": "polyant-seed-1",
    "data_dir": "./data/seed",
    "log_level": "info"
  },
  "network": {
    "p2p_port": 9000,
    "api_port": 8080,
    "dht_enabled": true,
    "relay_enabled": true
  },
  "mirror": {
    "enabled": true,
    "categories": ["*"],
    "max_size_gb": 100
  },
  "i18n": {
    "default_lang": "zh-CN",
    "available_langs": ["zh-CN", "en-US"],
    "log_bilingual": false
  }
}
```

### 用户节点配置 (configs/user.json)

```json
{
  "user": {
    "service_mode": false,
    "relay_enabled": false,
    "mirror_enabled": false,
    "mirror_limit_gb": 10
  },
  "node": {
    "name": "polyant-user-1",
    "data_dir": "./data/user",
    "log_level": "info"
  },
  "network": {
    "p2p_port": 0,
    "api_port": 8080,
    "seed_nodes": ["/dns4/seed.polyant.top/tcp/9000/p2p/12D3Koo..."],
    "dht_enabled": false
  },
  "account": {
    "private_key_path": "./data/keys",
    "email": "",
    "auto_register": true
  },
  "sync": {
    "auto_sync": true,
    "interval_seconds": 300
  },
  "i18n": {
    "default_lang": "zh-CN",
    "available_langs": ["zh-CN", "en-US"],
    "log_bilingual": false
  }
}
```

### 配置结构体定义

```go
// pkg/config/config.go

// SeedConfig 种子节点专用配置
type SeedConfig struct {
    Domain        string   `json:"domain"`         // 域名（必填）
    TLSCert       string   `json:"tls_cert"`       // TLS 证书路径
    TLSKey        string   `json:"tls_key"`        // TLS 密钥路径
    BootstrapPeers []string `json:"bootstrap_peers"` // 启动时连接的其他种子节点
}

// UserNodeConfig 用户节点专用配置
type UserNodeConfig struct {
    ServiceMode   bool `json:"service_mode"`    // 是否启用服务模式
    RelayEnabled  bool `json:"relay_enabled"`   // 是否提供中继服务
    MirrorEnabled bool `json:"mirror_enabled"`  // 是否提供数据镜像
    MirrorLimitGB int  `json:"mirror_limit_gb"` // 镜像数据上限（GB）
}

// AccountConfig 账户配置（通用，原 UserConfig 重命名）
type AccountConfig struct {
    PrivateKeyPath string `json:"private_key_path"`
    Email          string `json:"email"`
    AutoRegister   bool   `json:"auto_register"`
}

// MirrorConfig 镜像配置（种子节点专用）
type MirrorConfig struct {
    Enabled    bool     `json:"enabled"`     // 是否启用镜像
    Categories []string `json:"categories"`  // 镜像的分类，["*"] 表示全部
    MaxSizeGB  int      `json:"max_size_gb"` // 最大镜像大小（GB）
}

// Config 顶层配置
type Config struct {
    Seed    SeedConfig    `json:"seed"`    // 种子节点专用
    User    UserNodeConfig `json:"user"`   // 用户节点专用
    Account AccountConfig `json:"account"` // 账户配置（通用）
    Node    NodeConfig    `json:"node"`    // 节点配置（通用）
    Network NetworkConfig `json:"network"` // 网络配置（通用）
    Sync    SyncConfig    `json:"sync"`    // 同步配置（通用）
    Mirror  MirrorConfig  `json:"mirror"`  // 镜像配置（种子节点）
    Storage StorageConfig `json:"storage"` // 存储配置（通用）
    I18n    I18nConfig    `json:"i18n"`    // 国际化（通用）
}
```

---

## 网络能力检测

### 检测流程

```go
// internal/network/detect/capability.go

type NATType string

const (
    NATTypeNone           NATType = "none"            // 有公网 IP，无 NAT
    NATTypeFullCone       NATType = "full_cone"       // 全锥形 NAT
    NATTypeRestricted     NATType = "restricted"      // 受限锥形 NAT
    NATTypePortRestricted NATType = "port_restricted" // 端口受限 NAT
    NATTypeSymmetric      NATType = "symmetric"       // 对称 NAT
)

type NetworkCapability struct {
    HasPublicIP     bool     `json:"has_public_ip"`     // 是否有公网 IP
    PublicIP        string   `json:"public_ip"`         // 公网 IP 地址
    NATType         NATType  `json:"nat_type"`          // NAT 类型
    CanBeReached    bool     `json:"can_be_reached"`    // 是否可被直接访问
    CanRelay        bool     `json:"can_relay"`         // 是否可提供中继
    RecommendedMode string   `json:"recommended_mode"`  // 推荐模式：normal / service
}

func DetectNetworkCapability() *NetworkCapability {
    // 1. 检测公网 IP
    publicIP := detectPublicIP()

    // 2. 检测 NAT 类型
    natType := detectNATType()

    // 3. 测试端口可达性
    canBeReached := testPortReachability(publicIP)

    // 4. 判断是否可提供中继
    canRelay := canBeReached && natType != NATTypeSymmetric

    // 5. 推荐模式
    recommendedMode := "normal"
    if canBeReached {
        recommendedMode = "service"
    }

    return &NetworkCapability{
        HasPublicIP:     publicIP != "",
        PublicIP:        publicIP,
        NATType:         natType,
        CanBeReached:    canBeReached,
        CanRelay:        canRelay,
        RecommendedMode: recommendedMode,
    }
}
```

### 检测实现

```go
// 检测公网 IP
func detectPublicIP() string {
    // 使用公共 STUN 服务或 HTTP 服务
    services := []string{
        "https://api.ipify.org",
        "https://ifconfig.me/ip",
        "https://icanhazip.com",
    }

    for _, svc := range services {
        resp, err := http.Get(svc)
        if err != nil {
            continue
        }
        defer resp.Body.Close()

        ip, err := io.ReadAll(resp.Body)
        if err != nil {
            continue
        }

        return strings.TrimSpace(string(ip))
    }

    return ""
}

// 测试端口可达性
func testPortReachability(publicIP string) bool {
    // 1. 监听临时端口
    ln, err := net.Listen("tcp", ":0")
    if err != nil {
        return false
    }
    defer ln.Close()

    port := ln.Addr().(*net.TCPAddr).Port

    // 2. 请求外部服务测试连接
    // 调用种子节点的端口检测 API
    testURL := fmt.Sprintf("https://seed.polyant.top/api/v1/test-port?ip=%s&port=%d", publicIP, port)

    resp, err := http.Get(testURL)
    if err != nil {
        return false
    }
    defer resp.Body.Close()

    var result struct {
        Reachable bool `json:"reachable"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    return result.Reachable
}
```

---

## 连接与中继机制

### 连接策略

```
连接目标节点流程：

1. 尝试直连 IP:Port
   ├─ 成功 → 建立直连
   └─ 失败 → 继续

2. 查询种子节点获取中继列表
   ├─ 有中继节点 → 选择延迟最低的中继
   │   └─ 通过中继建立连接
   └─ 无中继节点 → 继续

3. 使用本地缓存数据（只读模式）
```

### 中继协议

**节点能力声明：**

```go
// internal/network/protocol/types.go

type CapabilityType int

const (
    CapabilityRelay  CapabilityType = iota + 1 // 中继服务
    CapabilityMirror                             // 数据镜像
    CapabilityDHT                                // DHT 路由
)

type Capability struct {
    Type      CapabilityType `json:"type"`
    Limit     int            `json:"limit"`      // 中继连接数上限 / 镜像大小上限
    Available bool           `json:"available"`  // 当前是否可用
}

type Handshake struct {
    NodeID       string       `json:"node_id"`
    NodeType     NodeType     `json:"node_type"`
    Capabilities []Capability `json:"capabilities"`
}
```

**中继请求流程：**

```
用户节点 A 无法直连用户节点 B：

1. A → 种子节点: QueryRelay{Target: B.NodeID}
2. 种子节点 → A: RelayList{Nodes: [R1, R2, R3]}
3. A → R1: RelayRequest{Target: B.NodeID}
4. R1 → B: RelayAccept{From: A.NodeID}
5. B → R1: RelayAcceptAck
6. A ↔ R1 ↔ B: 双向数据转发
```

**中继服务实现：**

```go
// internal/network/relay/relay.go

type RelayService struct {
    nodeID      string
    maxRelays   int
    activeRelays map[string]*RelayConnection
    mu          sync.RWMutex
}

type RelayConnection struct {
    ID         string
    FromNodeID string
    ToNodeID   string
    FromConn   net.Conn
    ToConn     net.Conn
    CreatedAt  time.Time
}

func (s *RelayService) HandleRelayRequest(req *RelayRequest) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // 检查中继数量限制
    if len(s.activeRelays) >= s.maxRelays {
        return ErrRelayLimitExceeded
    }

    // 创建中继连接
    relay := &RelayConnection{
        ID:         uuid.New().String(),
        FromNodeID: req.FromNodeID,
        ToNodeID:   req.Target,
        CreatedAt:  time.Now(),
    }

    s.activeRelays[relay.ID] = relay

    // 转发数据
    go s.relayData(relay)

    return nil
}
```

---

## 数据同步机制

### 数据流向

```
用户节点创建条目：
    │
    ▼
本地存储确认
    │
    ▼
同步到种子节点 ──失败──► 排队重试
    │
    成功
    │
    ▼
其他用户可访问
```

### 同步队列

```go
// internal/sync/queue.go

type SyncQueue struct {
    pending []*SyncTask
    retry   []*SyncTask
    mu      sync.Mutex
}

type SyncTask struct {
    EntryID    string    `json:"entry_id"`
    Action     string    `json:"action"`     // create, update, delete
    CreatedAt  time.Time `json:"created_at"`
    RetryCount int       `json:"retry_count"`
    MaxRetry   int       `json:"max_retry"`
    NextRetry  time.Time `json:"next_retry"`
}

type SyncStatus struct {
    EntryID       string    `json:"entry_id"`
    LocalSaved    bool      `json:"local_saved"`
    SyncedToSeed  bool      `json:"synced_to_seed"`
    SyncedNodes   []string  `json:"synced_nodes"`
    RetryCount    int       `json:"retry_count"`
    LastSyncAt    time.Time `json:"last_sync_at"`
}

func (q *SyncQueue) Add(task *SyncTask) {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.pending = append(q.pending, task)
}

func (q *SyncQueue) Process() {
    q.mu.Lock()
    defer q.mu.Unlock()

    now := time.Now()

    // 处理待同步任务
    for _, task := range q.pending {
        if err := q.syncToSeed(task); err != nil {
            task.RetryCount++
            if task.RetryCount < task.MaxRetry {
                task.NextRetry = now.Add(time.Duration(task.RetryCount*task.RetryCount) * time.Second)
                q.retry = append(q.retry, task)
            }
        }
    }
    q.pending = nil

    // 处理重试任务
    var stillRetry []*SyncTask
    for _, task := range q.retry {
        if now.After(task.NextRetry) {
            if err := q.syncToSeed(task); err != nil {
                task.RetryCount++
                if task.RetryCount < task.MaxRetry {
                    task.NextRetry = now.Add(time.Duration(task.RetryCount*task.RetryCount) * time.Second)
                    stillRetry = append(stillRetry, task)
                }
            }
        } else {
            stillRetry = append(stillRetry, task)
        }
    }
    q.retry = stillRetry
}
```

### 离线模式

```go
// 用户节点离线时
// 1. 本地数据正常读写
// 2. 创建/更新操作记录到同步队列
// 3. 恢复在线后自动同步

func (n *UserNode) OnOffline() {
    n.syncQueue.EnableOfflineMode()
}

func (n *UserNode) OnOnline() {
    n.syncQueue.ProcessPending()
}

// 种子节点离线时
// 1. 用户节点尝试连接其他种子节点
// 2. 无可用种子时，数据保留本地队列
// 3. 定期重连

func (n *UserNode) connectToSeed() error {
    for _, seedAddr := range n.config.Network.SeedNodes {
        if err := n.connect(seedAddr); err == nil {
            return nil
        }
    }
    return ErrNoSeedAvailable
}
```

---

## skill.md 文档设计

### 文档结构

```markdown
# Polyant Skill 安装指南

## 快速开始

### 第一步：检测网络环境

智能体应执行以下检测：

1. 检测公网 IP
2. 检测端口可达性
3. 判断推荐模式

### 第二步：选择节点类型

| 检测结果 | 推荐节点类型 | 推荐模式 |
|---------|-------------|---------|
| 有域名 | 种子节点 | - |
| 有公网 IP 无域名 | 用户节点 | 服务模式 |
| 仅内网 | 用户节点 | 普通模式 |

### 第三步：安装和配置

## 网络环境检测指南

### 检测公网 IP

\`\`\`bash
curl -s https://api.ipify.org
\`\`\`

如果返回 IP 地址，说明有公网 IP。

### 检测端口可达性

\`\`\`bash
# 请求种子节点测试端口
curl "https://seed.polyant.top/api/v1/test-port?ip=<YOUR_IP>&port=<PORT>"
\`\`\`

### 检测域名（可选）

如果有域名，可以部署种子节点。

## 安装命令

### 用户节点安装（推荐）

\`\`\`bash
# 普通模式（自动配置）
curl -fsSL https://get.polyant.top/user.sh | sh

# 启动
polyant-user --seed-nodes /dns4/seed.polyant.top/tcp/9000/p2p/12D3Koo...
\`\`\`

### 服务模式（有公网 IP）

\`\`\`bash
polyant-user --service --p2p-port 9001 --api-port 8081
\`\`\`

### 种子节点安装

\`\`\`bash
# 下载
wget https://github.com/daifei0527/polyant/releases/download/v2.0.0/polyant-seed-2.0.0-linux-amd64.tar.gz

# 解压并配置
tar -xzf polyant-seed-2.0.0-linux-amd64.tar.gz

# 启动
polyant-seed --domain seed.example.com --tls-cert /path/to/cert --tls-key /path/to/key
\`\`\`

## 配置参数说明

| 参数 | 适用节点 | 说明 | 默认值 |
|-----|---------|------|-------|
| --domain | 种子节点 | 域名（必填） | - |
| --tls-cert | 种子节点 | TLS 证书路径 | - |
| --tls-key | 种子节点 | TLS 密钥路径 | - |
| --p2p-port | 种子/用户服务 | P2P 监听端口 | 9000 |
| --api-port | 全部 | API 服务端口 | 8080 |
| --service | 用户节点 | 启用服务模式 | false |
| --seed-nodes | 用户节点 | 种子节点地址 | 内置默认 |
| --relay | 服务模式 | 提供中继服务 | false |
| --mirror | 服务模式 | 提供数据镜像 | false |
| --lang | 全部 | 输出语言 | zh-CN |

## API 调用示例

（保留现有 API 文档内容）
```

---

## 实现计划

### 文件变更清单

**新建文件：**

| 文件 | 说明 |
|-----|-----|
| `cmd/seed/main.go` | 种子节点入口 |
| `cmd/user/main.go` | 用户节点入口 |
| `internal/network/relay/relay.go` | 中继服务 |
| `internal/network/relay/discovery.go` | 中继节点发现 |
| `internal/network/detect/capability.go` | 网络能力检测 |
| `internal/sync/queue.go` | 同步队列 |
| `configs/seed.json` | 种子节点配置示例 |
| `configs/user.json` | 用户节点配置示例 |
| `Dockerfile.seed` | 种子节点 Docker 镜像 |
| `Dockerfile.user` | 用户节点 Docker 镜像 |

**修改文件：**

| 文件 | 修改内容 |
|-----|---------|
| `pkg/config/config.go` | 新增 SeedConfig、UserNodeConfig、MirrorConfig |
| `internal/network/host/host.go` | 支持服务模式、中继服务 |
| `internal/network/protocol/types.go` | 新增 Capability 结构 |
| `internal/sync/sync.go` | 集成同步队列 |
| `docs/skill.md` | 网络环境判断指南、新的安装说明 |
| `Makefile` | 新增 seed、user 构建目标 |

**废弃文件：**

| 文件 | 处理方式 |
|-----|---------|
| `cmd/polyant/main.go` | 标记废弃，保留向后兼容一个版本 |
| `configs/default.json` | 拆分为 seed.json 和 user.json |

### 构建目标

```makefile
# Makefile

build: build-seed build-user

build-seed:
	go build -o bin/polyant-seed ./cmd/seed

build-user:
	go build -o bin/polyant-user ./cmd/user

docker-seed:
	docker build -f Dockerfile.seed -t polyant-seed:latest .

docker-user:
	docker build -f Dockerfile.user -t polyant-user:latest .
```

### 发布计划

**版本：2.0.0**

**发布物：**
- `polyant-seed-2.0.0-linux-amd64.tar.gz`
- `polyant-seed-2.0.0-darwin-amd64.tar.gz`
- `polyant-user-2.0.0-linux-amd64.tar.gz`
- `polyant-user-2.0.0-darwin-amd64.tar.gz`
- `polyant-user-2.0.0-windows-amd64.zip`
- Docker 镜像：`polyant-seed:2.0.0`, `polyant-user:2.0.0`

**迁移指南：**
- 现有种子节点用户：迁移到 `polyant-seed`
- 现有用户节点用户：迁移到 `polyant-user`
- 配置文件需更新为新格式

---

## 不在本次范围

- 多语言 i18n 内容迁移（已完成）
- 前端宣传页更新
- 管理后台开发
- 多种子节点选举机制
- 用户权限等级调整
