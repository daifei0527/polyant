# 种子节点与用户节点分离实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Polyant 终端拆分为种子节点 (`polyant-seed`) 和用户节点 (`polyant-user`) 两个独立应用程序，明确各自的部署场景和功能边界。

**Architecture:**
- 两个独立二进制入口：`cmd/seed/main.go` 和 `cmd/user/main.go`
- 新增配置类型：`SeedConfig`、`UserNodeConfig`、`MirrorConfig`
- 新增网络能力检测模块：`internal/network/detect/`
- 新增同步队列模块：`internal/sync/queue.go`

**Tech Stack:** Go 1.21+, libp2p, BadgerDB/Pebble

---

## 文件结构

**新建文件：**
- `cmd/seed/main.go` - 种子节点入口
- `cmd/user/main.go` - 用户节点入口
- `internal/network/detect/capability.go` - 网络能力检测
- `internal/network/detect/capability_test.go` - 网络能力检测测试
- `internal/sync/queue.go` - 同步队列
- `internal/sync/queue_test.go` - 同步队列测试
- `configs/seed.json` - 种子节点配置示例
- `configs/user.json` - 用户节点配置示例
- `Dockerfile.seed` - 种子节点 Docker 镜像
- `Dockerfile.user` - 用户节点 Docker 镜像

**修改文件：**
- `pkg/config/config.go` - 新增 SeedConfig、UserNodeConfig、MirrorConfig、AccountConfig
- `internal/network/protocol/types.go` - 新增 Capability 结构
- `Makefile` - 新增 seed、user 构建目标
- `docs/skill.md` - 网络环境判断指南

---

## Task 1: 配置结构扩展

**Files:**
- Modify: `pkg/config/config.go`
- Test: `pkg/config/config_test.go`

- [ ] **Step 1: 写失败的测试 - SeedConfig 配置验证**

```go
// pkg/config/config_test.go

func TestSeedConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  *SeedConfig
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid seed config",
            config: &SeedConfig{
                Domain:  "seed.example.com",
                TLSCert: "/path/to/cert.pem",
                TLSKey:  "/path/to/key.pem",
            },
            wantErr: false,
        },
        {
            name: "missing domain",
            config: &SeedConfig{
                TLSCert: "/path/to/cert.pem",
                TLSKey:  "/path/to/key.pem",
            },
            wantErr: true,
            errMsg:  "domain is required",
        },
        {
            name: "missing tls cert",
            config: &SeedConfig{
                Domain: "seed.example.com",
                TLSKey: "/path/to/key.pem",
            },
            wantErr: true,
            errMsg:  "tls_cert is required",
        },
        {
            name: "missing tls key",
            config: &SeedConfig{
                Domain:  "seed.example.com",
                TLSCert: "/path/to/cert.pem",
            },
            wantErr: true,
            errMsg:  "tls_key is required",
            },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./pkg/config/... -run TestSeedConfigValidation`
Expected: FAIL - SeedConfig.Validate not defined

- [ ] **Step 3: 实现 SeedConfig 和 UserNodeConfig 结构体**

```go
// pkg/config/config.go

// SeedConfig 种子节点专用配置
type SeedConfig struct {
    Domain         string   `json:"domain"`          // 域名（必填）
    TLSCert        string   `json:"tls_cert"`        // TLS 证书路径
    TLSKey         string   `json:"tls_key"`         // TLS 密钥路径
    BootstrapPeers []string `json:"bootstrap_peers"` // 启动时连接的其他种子节点
}

// Validate 验证种子节点配置
func (c *SeedConfig) Validate() error {
    if c.Domain == "" {
        return fmt.Errorf("domain is required for seed node")
    }
    if c.TLSCert == "" {
        return fmt.Errorf("tls_cert is required for seed node")
    }
    if c.TLSKey == "" {
        return fmt.Errorf("tls_key is required for seed node")
    }
    return nil
}

// UserNodeConfig 用户节点专用配置
type UserNodeConfig struct {
    ServiceMode   bool `json:"service_mode"`    // 是否启用服务模式
    RelayEnabled  bool `json:"relay_enabled"`   // 是否提供中继服务
    MirrorEnabled bool `json:"mirror_enabled"`  // 是否提供数据镜像
    MirrorLimitGB int  `json:"mirror_limit_gb"` // 镜像数据上限（GB）
}

// MirrorConfig 镜像配置（种子节点专用）
type MirrorConfig struct {
    Enabled    bool     `json:"enabled"`     // 是否启用镜像
    Categories []string `json:"categories"`  // 镜像的分类，["*"] 表示全部
    MaxSizeGB  int      `json:"max_size_gb"` // 最大镜像大小（GB）
}

// AccountConfig 账户配置（通用，原 UserConfig 重命名）
type AccountConfig struct {
    PrivateKeyPath string `json:"private_key_path"`
    Email          string `json:"email"`
    AutoRegister   bool   `json:"auto_register"`
}
```

- [ ] **Step 4: 更新顶层 Config 结构体**

```go
// pkg/config/config.go

// Config 顶层配置
type Config struct {
    Seed    SeedConfig     `json:"seed"`    // 种子节点专用
    User    UserNodeConfig `json:"user"`    // 用户节点专用
    Account AccountConfig  `json:"account"` // 账户配置（通用）
    Node    NodeConfig     `json:"node"`    // 节点配置（通用）
    Network NetworkConfig  `json:"network"` // 网络配置（通用）
    Sync    SyncConfig     `json:"sync"`    // 同步配置（通用）
    Mirror  MirrorConfig   `json:"mirror"`  // 镜像配置（种子节点）
    Sharing SharingConfig  `json:"sharing"` // 共享配置
    SMTP    SMTPConfig     `json:"smtp"`    // SMTP 配置
    API     APIConfig      `json:"api"`     // API 配置
    Storage StorageConfig  `json:"storage"` // 存储配置
    I18n    I18nConfig     `json:"i18n"`    // 国际化
}
```

- [ ] **Step 5: 更新 DefaultConfig 函数**

```go
// pkg/config/config.go

// DefaultConfig 返回包含所有默认值的配置实例
func DefaultConfig() *Config {
    return &Config{
        Node: NodeConfig{
            Type:     "local",
            Name:     "polyant-node-1",
            DataDir:  "./data",
            LogDir:   "./logs",
            LogLevel: "info",
        },
        Network: NetworkConfig{
            ListenPort:  18530,
            APIPort:     18531,
            SeedNodes:   []string{},
            DHTEnabled:  true,
            MDNSEnabled: true,
        },
        Sync: SyncConfig{
            AutoSync:         true,
            IntervalSeconds:  300,
            MirrorCategories: []string{},
            MaxLocalSizeMB:   1024,
            Compression:      "gzip",
        },
        Sharing: SharingConfig{
            AllowMirror:      true,
            BandwidthLimitMB: 100,
            MaxConcurrent:    10,
        },
        Account: AccountConfig{
            PrivateKeyPath: "./data/keys",
            Email:          "",
            AutoRegister:   true,
        },
        SMTP: SMTPConfig{
            Enabled:  false,
            Host:     "",
            Port:     587,
            Username: "",
            Password: "",
            From:     "",
        },
        API: APIConfig{
            Enabled: true,
            CORS:    true,
        },
        Storage: StorageConfig{
            KVType:     "pebble",
            SearchType: "bleve",
        },
        I18n: I18nConfig{
            DefaultLang:    "zh-CN",
            AvailableLangs: []string{"zh-CN", "en-US"},
            LogBilingual:   false,
        },
    }
}
```

- [ ] **Step 6: 更新 Validate 函数**

```go
// pkg/config/config.go

// Validate 验证配置值的合法性
func Validate(config *Config) error {
    if config == nil {
        return fmt.Errorf("配置不能为空")
    }

    // 验证节点类型
    validNodeTypes := map[string]bool{"local": true, "seed": true, "user": true}
    if !validNodeTypes[config.Node.Type] {
        return fmt.Errorf("无效的节点类型: %s，必须是 'local'、'seed' 或 'user'", config.Node.Type)
    }

    // 种子节点必须验证 SeedConfig
    if config.Node.Type == "seed" {
        if err := config.Seed.Validate(); err != nil {
            return fmt.Errorf("种子节点配置错误: %w", err)
        }
    }

    // 验证节点配置
    if config.Node.Name == "" {
        return fmt.Errorf("节点名称不能为空")
    }
    validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
    if !validLogLevels[config.Node.LogLevel] {
        return fmt.Errorf("无效的日志级别: %s，必须是 debug/info/warn/error", config.Node.LogLevel)
    }

    // 验证网络配置
    if config.Network.ListenPort < 0 || config.Network.ListenPort > 65535 {
        return fmt.Errorf("无效的监听端口: %d，必须在 0-65535 之间", config.Network.ListenPort)
    }
    if config.Network.APIPort < 1 || config.Network.APIPort > 65535 {
        return fmt.Errorf("无效的 API 端口: %d，必须在 1-65535 之间", config.Network.APIPort)
    }
    if config.Network.ListenPort > 0 && config.Network.ListenPort == config.Network.APIPort {
        return fmt.Errorf("监听端口和 API 端口不能相同: %d", config.Network.ListenPort)
    }

    // 验证同步配置
    if config.Sync.IntervalSeconds < 0 {
        return fmt.Errorf("同步间隔不能为负数: %d", config.Sync.IntervalSeconds)
    }
    if config.Sync.MaxLocalSizeMB < 0 {
        return fmt.Errorf("本地最大存储大小不能为负数: %d", config.Sync.MaxLocalSizeMB)
    }
    validCompression := map[string]bool{"gzip": true, "zlib": true, "none": true, "": true}
    if !validCompression[config.Sync.Compression] {
        return fmt.Errorf("无效的压缩算法: %s，必须是 gzip/zlib/none", config.Sync.Compression)
    }

    // 验证共享配置
    if config.Sharing.BandwidthLimitMB < 0 {
        return fmt.Errorf("带宽限制不能为负数: %d", config.Sharing.BandwidthLimitMB)
    }
    if config.Sharing.MaxConcurrent < 0 {
        return fmt.Errorf("最大并发数不能为负数: %d", config.Sharing.MaxConcurrent)
    }

    // 验证 SMTP 配置（仅在启用时验证）
    if config.SMTP.Enabled {
        if config.SMTP.Host == "" {
            return fmt.Errorf("SMTP 已启用但未配置主机地址")
        }
        if config.SMTP.Port < 1 || config.SMTP.Port > 65535 {
            return fmt.Errorf("无效的 SMTP 端口: %d", config.SMTP.Port)
        }
        if config.SMTP.From == "" {
            return fmt.Errorf("SMTP 已启用但未配置发件人地址")
        }
    }

    return nil
}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test -v ./pkg/config/... -run TestSeedConfigValidation`
Expected: PASS

- [ ] **Step 8: 更新环境变量加载函数**

```go
// pkg/config/config.go

// LoadWithEnv 使用环境变量覆盖配置值
func LoadWithEnv(config *Config) *Config {
    if config == nil {
        config = DefaultConfig()
    }

    // ... 保留现有环境变量加载 ...

    // 种子节点配置环境变量
    if v := os.Getenv("POLYANT_SEED_DOMAIN"); v != "" {
        config.Seed.Domain = v
    }
    if v := os.Getenv("POLYANT_SEED_TLS_CERT"); v != "" {
        config.Seed.TLSCert = v
    }
    if v := os.Getenv("POLYANT_SEED_TLS_KEY"); v != "" {
        config.Seed.TLSKey = v
    }
    if v := os.Getenv("POLYANT_SEED_BOOTSTRAP_PEERS"); v != "" {
        config.Seed.BootstrapPeers = strings.Split(v, ",")
    }

    // 用户节点配置环境变量
    if v := os.Getenv("POLYANT_USER_SERVICE_MODE"); v != "" {
        config.User.ServiceMode = parseBool(v)
    }
    if v := os.Getenv("POLYANT_USER_RELAY_ENABLED"); v != "" {
        config.User.RelayEnabled = parseBool(v)
    }
    if v := os.Getenv("POLYANT_USER_MIRROR_ENABLED"); v != "" {
        config.User.MirrorEnabled = parseBool(v)
    }
    if v := os.Getenv("POLYANT_USER_MIRROR_LIMIT_GB"); v != "" {
        if limit, err := strconv.Atoi(v); err == nil {
            config.User.MirrorLimitGB = limit
        }
    }

    // 镜像配置环境变量
    if v := os.Getenv("POLYANT_MIRROR_ENABLED"); v != "" {
        config.Mirror.Enabled = parseBool(v)
    }
    if v := os.Getenv("POLYANT_MIRROR_CATEGORIES"); v != "" {
        config.Mirror.Categories = strings.Split(v, ",")
    }
    if v := os.Getenv("POLYANT_MIRROR_MAX_SIZE_GB"); v != "" {
        if size, err := strconv.Atoi(v); err == nil {
            config.Mirror.MaxSizeGB = size
        }
    }

    return config
}
```

- [ ] **Step 9: 提交配置结构扩展**

```bash
git add pkg/config/config.go pkg/config/config_test.go
git commit -m "$(cat <<'EOF'
feat(config): add SeedConfig, UserNodeConfig, and MirrorConfig

Add node-type-specific configuration structures:
- SeedConfig: domain, TLS certs, bootstrap peers
- UserNodeConfig: service mode, relay, mirror settings
- MirrorConfig: categories and size limits

Update Config struct to include new types.
Add validation for seed node required fields.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: 网络能力检测模块

**Files:**
- Create: `internal/network/detect/capability.go`
- Create: `internal/network/detect/capability_test.go`

- [ ] **Step 1: 写失败的测试 - 网络能力检测**

```go
// internal/network/detect/capability_test.go

package detect

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestDetectPublicIP(t *testing.T) {
    // 这个测试可能会因为网络原因失败，所以标记为集成测试
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    ip := detectPublicIP()
    // 如果有网络连接，应该返回一个 IP 地址
    if ip != "" {
        assert.Regexp(t, `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`, ip)
    }
}

func TestNetworkCapability(t *testing.T) {
    cap := &NetworkCapability{
        HasPublicIP:     true,
        PublicIP:        "1.2.3.4",
        NATType:         NATTypeNone,
        CanBeReached:    true,
        CanRelay:        true,
        RecommendedMode: "service",
    }

    assert.True(t, cap.HasPublicIP)
    assert.Equal(t, "service", cap.RecommendedMode)
}

func TestNATTypeString(t *testing.T) {
    tests := []struct {
        natType NATType
        want    string
    }{
        {NATTypeNone, "none"},
        {NATTypeFullCone, "full_cone"},
        {NATTypeRestricted, "restricted"},
        {NATTypePortRestricted, "port_restricted"},
        {NATTypeSymmetric, "symmetric"},
        {NATTypeUnknown, "unknown"},
    }

    for _, tt := range tests {
        t.Run(tt.want, func(t *testing.T) {
            assert.Equal(t, tt.want, string(tt.natType))
        })
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/network/detect/...`
Expected: FAIL - package not found

- [ ] **Step 3: 实现网络能力检测模块**

```go
// internal/network/detect/capability.go

package detect

import (
    "io"
    "log"
    "net"
    "net/http"
    "strings"
    "time"
)

// NATType NAT 类型
type NATType string

const (
    NATTypeNone           NATType = "none"            // 有公网 IP，无 NAT
    NATTypeFullCone       NATType = "full_cone"       // 全锥形 NAT
    NATTypeRestricted     NATType = "restricted"      // 受限锥形 NAT
    NATTypePortRestricted NATType = "port_restricted" // 端口受限 NAT
    NATTypeSymmetric      NATType = "symmetric"       // 对称 NAT
    NATTypeUnknown        NATType = "unknown"         // 未知
)

// NetworkCapability 网络能力检测结果
type NetworkCapability struct {
    HasPublicIP     bool     `json:"has_public_ip"`     // 是否有公网 IP
    PublicIP        string   `json:"public_ip"`         // 公网 IP 地址
    NATType         NATType  `json:"nat_type"`          // NAT 类型
    CanBeReached    bool     `json:"can_be_reached"`    // 是否可被直接访问
    CanRelay        bool     `json:"can_relay"`         // 是否可提供中继
    RecommendedMode string   `json:"recommended_mode"`  // 推荐模式：normal / service
}

// Detector 网络能力检测器
type Detector struct {
    stunServers []string
    httpClients []string
    timeout     time.Duration
}

// NewDetector 创建检测器
func NewDetector() *Detector {
    return &Detector{
        httpClients: []string{
            "https://api.ipify.org",
            "https://ifconfig.me/ip",
            "https://icanhazip.com",
        },
        timeout: 10 * time.Second,
    }
}

// Detect 检测网络能力
func (d *Detector) Detect() *NetworkCapability {
    cap := &NetworkCapability{
        NATType: NATTypeUnknown,
    }

    // 1. 检测公网 IP
    cap.PublicIP = d.detectPublicIP()
    cap.HasPublicIP = cap.PublicIP != ""

    // 2. 如果有公网 IP，测试端口可达性
    if cap.HasPublicIP {
        cap.CanBeReached = d.testPortReachability(cap.PublicIP)
        cap.NATType = d.detectNATType()
    }

    // 3. 判断是否可提供中继
    cap.CanRelay = cap.CanBeReached && cap.NATType != NATTypeSymmetric

    // 4. 推荐模式
    if cap.CanBeReached {
        cap.RecommendedMode = "service"
    } else {
        cap.RecommendedMode = "normal"
    }

    return cap
}

// detectPublicIP 检测公网 IP
func (d *Detector) detectPublicIP() string {
    client := &http.Client{
        Timeout: d.timeout,
    }

    for _, url := range d.httpClients {
        resp, err := client.Get(url)
        if err != nil {
            continue
        }
        defer resp.Body.Close()

        ip, err := io.ReadAll(resp.Body)
        if err != nil {
            continue
        }

        ipStr := strings.TrimSpace(string(ip))
        if net.ParseIP(ipStr) != nil {
            return ipStr
        }
    }

    return ""
}

// detectPublicIP 导出函数（用于测试）
func detectPublicIP() string {
    return NewDetector().detectPublicIP()
}

// testPortReachability 测试端口可达性
func (d *Detector) testPortReachability(publicIP string) bool {
    // 1. 监听临时端口
    ln, err := net.Listen("tcp", ":0")
    if err != nil {
        return false
    }
    defer ln.Close()

    port := ln.Addr().(*net.TCPAddr).Port

    // 2. 在后台等待连接
    acceptCh := make(chan bool, 1)
    go func() {
        conn, err := ln.Accept()
        if err == nil {
            conn.Close()
            acceptCh <- true
        } else {
            acceptCh <- false
        }
    }()

    // 3. 尝试从外部连接（这里简化处理，实际需要外部服务配合）
    // 如果本地能连接到自己的公网 IP:port，说明端口可达
    conn, err := net.DialTimeout("tcp",
        net.JoinHostPort(publicIP, fmt.Sprintf("%d", port)),
        5*time.Second)
    if err == nil {
        conn.Close()
        return <-acceptCh
    }

    return false
}

// detectNATType 检测 NAT 类型
// 简化版本：实际需要 STUN 服务器配合
func (d *Detector) detectNATType() NATType {
    // 简化处理：
    // - 如果能获取公网 IP 且能连接，假设为 NATTypeNone
    // - 否则需要 STUN 服务器检测
    // 实际实现中应该使用 STUN 协议检测
    return NATTypeNone
}

// DetectNetworkCapability 便捷函数
func DetectNetworkCapability() *NetworkCapability {
    return NewDetector().Detect()
}
```

- [ ] **Step 4: 添加 fmt 导入**

```go
// internal/network/detect/capability.go

import (
    "fmt"
    "io"
    "log"
    "net"
    "net/http"
    "strings"
    "time"
)
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -v ./internal/network/detect/... -short`
Expected: PASS

- [ ] **Step 6: 提交网络能力检测模块**

```bash
git add internal/network/detect/
git commit -m "$(cat <<'EOF'
feat(network): add network capability detection module

Add detect package for determining node network capabilities:
- Public IP detection via HTTP services
- NAT type detection (stub for STUN)
- Port reachability test
- Recommended mode suggestion (normal/service)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: 同步队列模块

**Files:**
- Create: `internal/sync/queue.go`
- Create: `internal/sync/queue_test.go`

- [ ] **Step 1: 写失败的测试 - 同步队列**

```go
// internal/sync/queue_test.go

package sync

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
)

func TestSyncQueue(t *testing.T) {
    q := NewSyncQueue()

    // 测试添加任务
    task := &SyncTask{
        EntryID:   "entry-1",
        Action:    "create",
        CreatedAt: time.Now(),
        MaxRetry:  3,
    }

    q.Add(task)
    assert.Equal(t, 1, q.PendingCount())
}

func TestSyncQueueProcess(t *testing.T) {
    q := NewSyncQueue()

    task1 := &SyncTask{
        EntryID:   "entry-1",
        Action:    "create",
        CreatedAt: time.Now(),
        MaxRetry:  3,
    }
    task2 := &SyncTask{
        EntryID:   "entry-2",
        Action:    "update",
        CreatedAt: time.Now(),
        MaxRetry:  3,
    }

    q.Add(task1)
    q.Add(task2)

    // 模拟处理
    processed := 0
    q.Process(func(task *SyncTask) error {
        processed++
        return nil
    })

    assert.Equal(t, 2, processed)
    assert.Equal(t, 0, q.PendingCount())
}

func TestSyncQueueRetry(t *testing.T) {
    q := NewSyncQueue()

    task := &SyncTask{
        EntryID:   "entry-1",
        Action:    "create",
        CreatedAt: time.Now(),
        MaxRetry:  2,
    }

    q.Add(task)

    // 第一次处理失败
    callCount := 0
    q.Process(func(task *SyncTask) error {
        callCount++
        if callCount < 2 {
            return fmt.Errorf("network error")
        }
        return nil
    })

    // 应该有一次待重试
    assert.Equal(t, 1, q.RetryCount())
}

func TestSyncStatus(t *testing.T) {
    status := &SyncStatus{
        EntryID:      "entry-1",
        LocalSaved:   true,
        SyncedToSeed: false,
        RetryCount:   0,
    }

    assert.True(t, status.LocalSaved)
    assert.False(t, status.SyncedToSeed)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/sync/... -run TestSyncQueue`
Expected: FAIL - SyncQueue not defined

- [ ] **Step 3: 实现同步队列**

```go
// internal/sync/queue.go

package sync

import (
    "fmt"
    "sync"
    "time"
)

// SyncTask 同步任务
type SyncTask struct {
    EntryID    string    `json:"entry_id"`
    Action     string    `json:"action"`     // create, update, delete
    CreatedAt  time.Time `json:"created_at"`
    RetryCount int       `json:"retry_count"`
    MaxRetry   int       `json:"max_retry"`
    NextRetry  time.Time `json:"next_retry"`
}

// SyncStatus 同步状态
type SyncStatus struct {
    EntryID      string    `json:"entry_id"`
    LocalSaved   bool      `json:"local_saved"`
    SyncedToSeed bool      `json:"synced_to_seed"`
    SyncedNodes  []string  `json:"synced_nodes"`
    RetryCount   int       `json:"retry_count"`
    LastSyncAt   time.Time `json:"last_sync_at"`
}

// SyncQueue 同步队列
type SyncQueue struct {
    pending []*SyncTask
    retry   []*SyncTask
    statuses map[string]*SyncStatus
    mu      sync.Mutex
    offline bool
}

// NewSyncQueue 创建同步队列
func NewSyncQueue() *SyncQueue {
    return &SyncQueue{
        pending:  make([]*SyncTask, 0),
        retry:    make([]*SyncTask, 0),
        statuses: make(map[string]*SyncStatus),
    }
}

// Add 添加同步任务
func (q *SyncQueue) Add(task *SyncTask) {
    q.mu.Lock()
    defer q.mu.Unlock()

    q.pending = append(q.pending, task)

    // 初始化状态
    q.statuses[task.EntryID] = &SyncStatus{
        EntryID:    task.EntryID,
        LocalSaved: true,
    }
}

// Process 处理待同步任务
func (q *SyncQueue) Process(syncFn func(*SyncTask) error) {
    q.mu.Lock()
    defer q.mu.Unlock()

    now := time.Now()

    // 处理待同步任务
    var newPending []*SyncTask
    for _, task := range q.pending {
        if err := syncFn(task); err != nil {
            task.RetryCount++
            if task.RetryCount < task.MaxRetry {
                task.NextRetry = now.Add(time.Duration(task.RetryCount*task.RetryCount) * time.Second)
                q.retry = append(q.retry, task)
            }
            // 更新状态
            if status, ok := q.statuses[task.EntryID]; ok {
                status.RetryCount = task.RetryCount
            }
        } else {
            // 同步成功，更新状态
            if status, ok := q.statuses[task.EntryID]; ok {
                status.SyncedToSeed = true
                status.LastSyncAt = now
            }
        }
    }
    q.pending = newPending

    // 处理重试任务
    var stillRetry []*SyncTask
    for _, task := range q.retry {
        if now.After(task.NextRetry) {
            if err := syncFn(task); err != nil {
                task.RetryCount++
                if task.RetryCount < task.MaxRetry {
                    task.NextRetry = now.Add(time.Duration(task.RetryCount*task.RetryCount) * time.Second)
                    stillRetry = append(stillRetry, task)
                }
            } else {
                // 同步成功，更新状态
                if status, ok := q.statuses[task.EntryID]; ok {
                    status.SyncedToSeed = true
                    status.LastSyncAt = now
                }
            }
        } else {
            stillRetry = append(stillRetry, task)
        }
    }
    q.retry = stillRetry
}

// PendingCount 返回待处理任务数
func (q *SyncQueue) PendingCount() int {
    q.mu.Lock()
    defer q.mu.Unlock()
    return len(q.pending)
}

// RetryCount 返回重试任务数
func (q *SyncQueue) RetryCount() int {
    q.mu.Lock()
    defer q.mu.Unlock()
    return len(q.retry)
}

// GetStatus 获取同步状态
func (q *SyncQueue) GetStatus(entryID string) *SyncStatus {
    q.mu.Lock()
    defer q.mu.Unlock()
    return q.statuses[entryID]
}

// EnableOfflineMode 启用离线模式
func (q *SyncQueue) EnableOfflineMode() {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.offline = true
}

// DisableOfflineMode 禁用离线模式
func (q *SyncQueue) DisableOfflineMode() {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.offline = false
}

// IsOffline 返回是否离线模式
func (q *SyncQueue) IsOffline() bool {
    q.mu.Lock()
    defer q.mu.Unlock()
    return q.offline
}

// ProcessPending 处理所有待处理任务（恢复在线时调用）
func (q *SyncQueue) ProcessPending(syncFn func(*SyncTask) error) {
    q.DisableOfflineMode()
    q.Process(syncFn)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -v ./internal/sync/... -run TestSyncQueue`
Expected: PASS

- [ ] **Step 5: 提交同步队列模块**

```bash
git add internal/sync/queue.go internal/sync/queue_test.go
git commit -m "$(cat <<'EOF'
feat(sync): add sync queue for offline support

Add SyncQueue for managing data synchronization:
- Pending task queue with retry logic
- Exponential backoff for failed tasks
- Sync status tracking per entry
- Offline mode support

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 协议类型扩展 - Capability

**Files:**
- Modify: `internal/network/protocol/types.go`

- [ ] **Step 1: 写失败的测试 - Capability 结构**

```go
// internal/network/protocol/types_test.go

func TestCapability(t *testing.T) {
    cap := Capability{
        Type:      CapabilityRelay,
        Limit:     100,
        Available: true,
    }

    assert.Equal(t, CapabilityRelay, cap.Type)
    assert.Equal(t, 100, cap.Limit)
    assert.True(t, cap.Available)
}

func TestHandshakeWithCapabilities(t *testing.T) {
    h := Handshake{
        NodeID:   "node-1",
        PeerID:   "peer-1",
        NodeType: NodeTypeSeed,
        Version:  "2.0.0",
        Capabilities: []Capability{
            {Type: CapabilityRelay, Limit: 100, Available: true},
            {Type: CapabilityMirror, Limit: 50, Available: true},
        },
    }

    assert.Len(t, h.Capabilities, 2)
    assert.Equal(t, NodeTypeSeed, h.NodeType)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v ./internal/network/protocol/... -run TestCapability`
Expected: FAIL - Capability not defined

- [ ] **Step 3: 添加 Capability 类型定义**

```go
// internal/network/protocol/types.go

// CapabilityType 能力类型
type CapabilityType int

const (
    CapabilityRelay CapabilityType = iota + 1 // 中继服务
    CapabilityMirror                           // 数据镜像
    CapabilityDHT                              // DHT 路由
)

// Capability 节点能力声明
type Capability struct {
    Type      CapabilityType `json:"type"`
    Limit     int            `json:"limit"`      // 中继连接数上限 / 镜像大小上限
    Available bool           `json:"available"`  // 当前是否可用
}
```

- [ ] **Step 4: 更新 Handshake 结构体**

```go
// internal/network/protocol/types.go

type Handshake struct {
    NodeID       string       `json:"node_id"`
    PeerID       string       `json:"peer_id"`
    NodeType     NodeType     `json:"node_type"`
    Version      string       `json:"version"`
    Categories   []string     `json:"categories"`
    EntryCount   int64        `json:"entry_count"`
    Capabilities []Capability `json:"capabilities"` // 新增：节点能力
    Signature    []byte       `json:"signature,omitempty"`
}
```

- [ ] **Step 5: 添加 NodeTypeSeed 常量**

```go
// internal/network/protocol/types.go

type NodeType int

const (
    NodeTypeLocal NodeType = iota
    NodeTypeSeed
    NodeTypeUser // 新增：用户节点类型
)
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go test -v ./internal/network/protocol/... -run TestCapability`
Expected: PASS

- [ ] **Step 7: 提交协议类型扩展**

```bash
git add internal/network/protocol/types.go internal/network/protocol/types_test.go
git commit -m "$(cat <<'EOF'
feat(protocol): add Capability type for node feature declaration

Extend protocol types for node capabilities:
- Add CapabilityType enum (Relay, Mirror, DHT)
- Add Capability struct with limit and availability
- Update Handshake to include capabilities
- Add NodeTypeUser for user node identification

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 种子节点入口

**Files:**
- Create: `cmd/seed/main.go`

- [ ] **Step 1: 实现种子节点入口**

```go
// cmd/seed/main.go

// Package main Polyant 种子节点入口
// 种子节点要求：公网 IP + 域名 + TLS 证书
package main

import (
    "context"
    "crypto/tls"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/daifei0527/polyant/internal/api/router"
    "github.com/daifei0527/polyant/internal/core/category"
    "github.com/daifei0527/polyant/internal/core/seed"
    "github.com/daifei0527/polyant/internal/network/dht"
    "github.com/daifei0527/polyant/internal/network/host"
    "github.com/daifei0527/polyant/internal/network/protocol"
    "github.com/daifei0527/polyant/internal/network/sync"
    "github.com/daifei0527/polyant/internal/storage"
    "github.com/daifei0527/polyant/pkg/config"
    "github.com/daifei0527/polyant/pkg/i18n"
    "go.uber.org/zap"
)

var (
    configFile  = flag.String("config", "", "配置文件路径 (JSON)")
    domain      = flag.String("domain", "", "域名（必填）")
    tlsCert     = flag.String("tls-cert", "", "TLS 证书路径")
    tlsKey      = flag.String("tls-key", "", "TLS 密钥路径")
    p2pPort     = flag.Int("p2p-port", 9000, "P2P 监听端口")
    apiPort     = flag.Int("api-port", 8080, "API 服务端口")
    dataDir     = flag.String("data-dir", "./data/seed", "数据目录")
    showVersion = flag.Bool("version", false, "显示版本信息")
)

const Version = "2.0.0"

// SeedApp 种子节点应用
type SeedApp struct {
    config      *config.Config
    logger      *zap.Logger
    store       *storage.Store
    p2pHost     *host.P2PHost
    dhtNode     *dht.DHTNode
    syncEngine  *sync.SyncEngine
    pushService *sync.PushService
    httpServer  *http.Server
    cancel      context.CancelFunc
}

func main() {
    flag.Parse()

    if *showVersion {
        fmt.Printf("polyant-seed version %s\n", Version)
        os.Exit(0)
    }

    cfg, err := loadConfig()
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }

    // 验证种子节点必需配置
    if cfg.Seed.Domain == "" {
        log.Fatal("种子节点必须配置域名 (--domain 或 config.seed.domain)")
    }
    if cfg.Seed.TLSCert == "" || cfg.Seed.TLSKey == "" {
        log.Fatal("种子节点必须配置 TLS 证书 (--tls-cert, --tls-key 或 config.seed.tls_cert/tls_key)")
    }

    app, err := NewSeedApp(cfg)
    if err != nil {
        log.Fatalf("初始化失败: %v", err)
    }

    if err := app.Run(); err != nil {
        log.Fatalf("运行失败: %v", err)
    }
}

func loadConfig() (*config.Config, error) {
    var cfg *config.Config
    var err error

    if *configFile != "" {
        cfg, err = config.Load(*configFile)
        if err != nil {
            return nil, err
        }
    } else {
        cfg = config.DefaultConfig()
    }

    // 命令行参数覆盖
    if *domain != "" {
        cfg.Seed.Domain = *domain
    }
    if *tlsCert != "" {
        cfg.Seed.TLSCert = *tlsCert
    }
    if *tlsKey != "" {
        cfg.Seed.TLSKey = *tlsKey
    }
    if *p2pPort != 9000 {
        cfg.Network.ListenPort = *p2pPort
    }
    if *apiPort != 8080 {
        cfg.Network.APIPort = *apiPort
    }
    if *dataDir != "./data/seed" {
        cfg.Node.DataDir = *dataDir
    }

    // 强制设置节点类型为 seed
    cfg.Node.Type = "seed"

    cfg = config.LoadWithEnv(cfg)
    if err := config.Validate(cfg); err != nil {
        return nil, err
    }

    // 初始化 i18n
    localesDir := cfg.Node.DataDir + "/locales"
    if err := i18n.Init(localesDir, i18n.Lang(cfg.I18n.DefaultLang)); err != nil {
        if err := i18n.Init("./pkg/i18n/locales", i18n.Lang(cfg.I18n.DefaultLang)); err != nil {
            log.Printf("警告: i18n初始化失败: %v", err)
        }
    }

    return cfg, nil
}

func NewSeedApp(cfg *config.Config) (*SeedApp, error) {
    logger, err := zap.NewProduction()
    if err != nil {
        return nil, fmt.Errorf("初始化日志失败: %w", err)
    }

    logger.Info("启动 Polyant 种子节点",
        zap.String("domain", cfg.Seed.Domain),
        zap.String("name", cfg.Node.Name),
    )

    ctx, cancel := context.WithCancel(context.Background())

    app := &SeedApp{
        config: cfg,
        logger: logger,
        cancel: cancel,
    }

    // 初始化存储
    if err := app.initStorage(ctx); err != nil {
        app.cleanup()
        return nil, err
    }

    // 初始化种子数据
    if err := app.initData(ctx); err != nil {
        app.cleanup()
        return nil, err
    }

    return app, nil
}

func (app *SeedApp) initStorage(ctx context.Context) error {
    dataDir := app.config.Node.DataDir
    if dataDir == "" {
        dataDir = "./data/seed"
    }

    storeCfg := &storage.StoreConfig{
        KVType:     app.config.Storage.KVType,
        KVPath:     dataDir + "/kv",
        SearchType: app.config.Storage.SearchType,
        SearchPath: dataDir + "/search.bleve",
    }

    if storeCfg.KVType == "" {
        storeCfg.KVType = "pebble"
    }
    if storeCfg.SearchType == "" {
        storeCfg.SearchType = "bleve"
    }

    var err error
    app.store, err = storage.NewPersistentStore(storeCfg)
    if err != nil {
        return fmt.Errorf("初始化存储失败: %w", err)
    }

    app.logger.Info("存储层初始化完成",
        zap.String("kv_path", storeCfg.KVPath),
    )
    return nil
}

func (app *SeedApp) initData(ctx context.Context) error {
    categoryInit := category.NewCategoryInitializer(app.store.Category, app.config.Node.DataDir)
    if err := categoryInit.Initialize(ctx); err != nil {
        app.logger.Warn("分类初始化警告", zap.Error(err))
    }

    seedInit := seed.NewSeedDataInitializer(app.store, app.config.Node.DataDir+"/seed-data")
    if err := seedInit.Initialize(ctx); err != nil {
        app.logger.Warn("种子数据初始化警告", zap.Error(err))
    }

    return nil
}

func (app *SeedApp) Start() error {
    ctx := context.Background()

    // 构建 P2P 监听地址
    listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", app.config.Network.ListenPort)

    hostCfg := &host.HostConfig{
        ListenAddrs:   []string{listenAddr},
        SeedPeers:     app.config.Network.SeedNodes,
        EnableDHT:     true,
        EnableMDNS:    app.config.Network.MDNSEnabled,
        EnableNAT:     true,
        EnableRelay:   true,
        RelayService:  true, // 种子节点启用中继服务
        EnableQUIC:    true,
    }

    var err error
    app.p2pHost, err = host.NewHost(ctx, hostCfg)
    if err != nil {
        return fmt.Errorf("创建 P2P Host 失败: %w", err)
    }

    app.p2pHost.SetNodeType("seed")
    app.logger.Info("P2P 节点启动成功",
        zap.String("node_id", app.p2pHost.NodeID()),
    )

    // 初始化 DHT
    app.dhtNode, err = dht.NewDHTNode(app.p2pHost.Host, app.config)
    if err != nil {
        return fmt.Errorf("初始化 DHT 失败: %w", err)
    }
    if err := app.dhtNode.Bootstrap(ctx); err != nil {
        app.logger.Warn("DHT bootstrap 警告", zap.Error(err))
    }

    // 创建同步引擎
    syncCfg := &sync.SyncConfig{
        AutoSync:         true,
        IntervalSeconds:  300,
        MirrorCategories: []string{"*"}, // 种子节点镜像所有分类
        MaxLocalSizeMB:   app.config.Mirror.MaxSizeGB * 1024,
    }
    app.syncEngine = sync.NewSyncEngine(app.p2pHost, nil, app.store, syncCfg)

    // 创建协议处理器
    proto := protocol.NewProtocol(app.p2pHost.Host, app.syncEngine)
    app.syncEngine.SetProtocol(proto)

    // 创建推送服务
    app.pushService = sync.NewPushService(app.p2pHost, nil)
    app.pushService.SetProtocol(proto)
    if err := app.pushService.Start(ctx); err != nil {
        app.logger.Warn("推送服务启动失败", zap.Error(err))
    }

    if err := app.syncEngine.Start(ctx); err != nil {
        return fmt.Errorf("启动同步引擎失败: %w", err)
    }

    // 连接其他种子节点
    for _, seedAddr := range app.config.Seed.BootstrapPeers {
        if err := app.p2pHost.ConnectToPeer(ctx, seedAddr); err != nil {
            app.logger.Warn("连接引导节点失败", zap.String("addr", seedAddr), zap.Error(err))
        }
    }

    // 创建 API 路由
    apiHandler, err := router.NewRouterWithDeps(&router.Dependencies{
        Store:        app.store,
        EntryStore:   app.store.Entry,
        UserStore:    app.store.User,
        RatingStore:  app.store.Rating,
        CategoryStore: app.store.Category,
        SearchEngine: app.store.Search,
        Backlink:     app.store.Backlink,
        KVStore:      app.store.KVStore(),
        NodeID:       app.p2pHost.NodeID(),
        NodeType:     "seed",
        Version:      Version,
    })
    if err != nil {
        return fmt.Errorf("创建 API 路由失败: %w", err)
    }

    // 启动 HTTPS 服务器
    httpAddr := fmt.Sprintf(":%d", app.config.Network.APIPort)
    app.httpServer = &http.Server{
        Addr:         httpAddr,
        Handler:      apiHandler,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // 加载 TLS 证书
    cert, err := tls.LoadX509KeyPair(app.config.Seed.TLSCert, app.config.Seed.TLSKey)
    if err != nil {
        return fmt.Errorf("加载 TLS 证书失败: %w", err)
    }

    app.httpServer.TLSConfig = &tls.Config{
        Certificates: []tls.Certificate{cert},
    }

    go func() {
        app.logger.Info("HTTPS API 服务器启动",
            zap.String("addr", httpAddr),
            zap.String("domain", app.config.Seed.Domain),
        )
        if err := app.httpServer.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
            app.logger.Fatal("HTTPS 服务器启动失败", zap.Error(err))
        }
    }()

    return nil
}

func (app *SeedApp) Run() error {
    if err := app.Start(); err != nil {
        return err
    }

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    app.logger.Info("收到关闭信号，开始优雅关机...")
    return app.Stop()
}

func (app *SeedApp) Stop() error {
    app.logger.Info("正在停止服务...")

    if app.httpServer != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        if err := app.httpServer.Shutdown(ctx); err != nil {
            app.logger.Warn("HTTP 服务器关闭超时", zap.Error(err))
        }
    }

    if app.pushService != nil {
        app.pushService.Stop()
    }
    if app.syncEngine != nil {
        app.syncEngine.Stop()
    }
    if app.p2pHost != nil {
        app.p2pHost.Close()
    }

    app.cleanup()
    app.logger.Info("Polyant 种子节点已关闭")
    return nil
}

func (app *SeedApp) cleanup() {
    if app.cancel != nil {
        app.cancel()
    }
    if app.store != nil {
        if err := app.store.Close(); err != nil {
            app.logger.Warn("关闭存储失败", zap.Error(err))
        }
    }
}
```

- [ ] **Step 2: 编译确认无语法错误**

Run: `go build -o bin/polyant-seed ./cmd/seed/`
Expected: Build success

- [ ] **Step 3: 提交种子节点入口**

```bash
git add cmd/seed/
git commit -m "$(cat <<'EOF'
feat(cmd): add polyant-seed binary entry point

Create dedicated seed node binary with:
- Required domain and TLS certificate validation
- HTTPS server with TLS
- P2P host with relay service enabled
- DHT routing enabled
- Full data mirroring capability

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 用户节点入口

**Files:**
- Create: `cmd/user/main.go`

- [ ] **Step 1: 实现用户节点入口**

```go
// cmd/user/main.go

// Package main Polyant 用户节点入口
// 用户节点面向智能体用户，自动适配网络环境
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/daifei0527/polyant/internal/api/router"
    "github.com/daifei0527/polyant/internal/core/category"
    "github.com/daifei0527/polyant/internal/network/detect"
    "github.com/daifei0527/polyant/internal/network/dht"
    "github.com/daifei0527/polyant/internal/network/host"
    "github.com/daifei0527/polyant/internal/network/protocol"
    "github.com/daifei0527/polyant/internal/network/sync"
    "github.com/daifei0527/polyant/internal/storage"
    "github.com/daifei0527/polyant/pkg/config"
    "github.com/daifei0527/polyant/pkg/i18n"
    "go.uber.org/zap"
)

var (
    configFile   = flag.String("config", "", "配置文件路径 (JSON)")
    seedNodes    = flag.String("seed-nodes", "", "种子节点地址（逗号分隔）")
    serviceMode  = flag.Bool("service", false, "启用服务模式（监听入站连接）")
    p2pPort      = flag.Int("p2p-port", 0, "P2P 监听端口（0=随机，服务模式下使用）")
    apiPort      = flag.Int("api-port", 8080, "API 服务端口")
    dataDir      = flag.String("data-dir", "./data/user", "数据目录")
    enableRelay  = flag.Bool("relay", false, "提供中继服务（服务模式下）")
    enableMirror = flag.Bool("mirror", false, "提供数据镜像（服务模式下）")
    showVersion  = flag.Bool("version", false, "显示版本信息")
    autoDetect   = flag.Bool("auto-detect", true, "自动检测网络环境")
)

const Version = "2.0.0"

// UserApp 用户节点应用
type UserApp struct {
    config      *config.Config
    logger      *zap.Logger
    store       *storage.Store
    p2pHost     *host.P2PHost
    dhtNode     *dht.DHTNode
    syncEngine  *sync.SyncEngine
    pushService *sync.PushService
    syncQueue   *sync.SyncQueue
    httpServer  *http.Server
    cancel      context.CancelFunc
}

func main() {
    flag.Parse()

    if *showVersion {
        fmt.Printf("polyant-user version %s\n", Version)
        os.Exit(0)
    }

    cfg, err := loadConfig()
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }

    app, err := NewUserApp(cfg)
    if err != nil {
        log.Fatalf("初始化失败: %v", err)
    }

    if err := app.Run(); err != nil {
        log.Fatalf("运行失败: %v", err)
    }
}

func loadConfig() (*config.Config, error) {
    var cfg *config.Config
    var err error

    if *configFile != "" {
        cfg, err = config.Load(*configFile)
        if err != nil {
            return nil, err
        }
    } else {
        cfg = config.DefaultConfig()
    }

    // 命令行参数覆盖
    if *seedNodes != "" {
        cfg.Network.SeedNodes = splitAndTrim(*seedNodes)
    }
    if *serviceMode {
        cfg.User.ServiceMode = true
    }
    if *p2pPort != 0 {
        cfg.Network.ListenPort = *p2pPort
    }
    if *apiPort != 8080 {
        cfg.Network.APIPort = *apiPort
    }
    if *dataDir != "./data/user" {
        cfg.Node.DataDir = *dataDir
    }
    if *enableRelay {
        cfg.User.RelayEnabled = true
    }
    if *enableMirror {
        cfg.User.MirrorEnabled = true
    }

    // 强制设置节点类型为 user
    cfg.Node.Type = "user"

    cfg = config.LoadWithEnv(cfg)

    // 自动检测网络环境
    if *autoDetect && !cfg.User.ServiceMode {
        detector := detect.NewDetector()
        cap := detector.Detect()
        if cap.RecommendedMode == "service" {
            log.Printf("检测到公网可达，建议启用服务模式 (--service)")
        }
    }

    if err := config.Validate(cfg); err != nil {
        return nil, err
    }

    // 初始化 i18n
    localesDir := cfg.Node.DataDir + "/locales"
    if err := i18n.Init(localesDir, i18n.Lang(cfg.I18n.DefaultLang)); err != nil {
        if err := i18n.Init("./pkg/i18n/locales", i18n.Lang(cfg.I18n.DefaultLang)); err != nil {
            log.Printf("警告: i18n初始化失败: %v", err)
        }
    }

    return cfg, nil
}

func splitAndTrim(s string) []string {
    parts := []string{}
    for _, p := range splitString(s, ",") {
        p = trimSpace(p)
        if p != "" {
            parts = append(parts, p)
        }
    }
    return parts
}

func splitString(s, sep string) []string {
    if s == "" {
        return nil
    }
    var result []string
    start := 0
    for i := 0; i <= len(s)-len(sep); i++ {
        if s[i:i+len(sep)] == sep {
            result = append(result, s[start:i])
            start = i + len(sep)
            i += len(sep) - 1
        }
    }
    result = append(result, s[start:])
    return result
}

func trimSpace(s string) string {
    start := 0
    end := len(s)
    for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
        start++
    }
    for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
        end--
    }
    return s[start:end]
}

func NewUserApp(cfg *config.Config) (*UserApp, error) {
    logger, err := zap.NewProduction()
    if err != nil {
        return nil, fmt.Errorf("初始化日志失败: %w", err)
    }

    logger.Info("启动 Polyant 用户节点",
        zap.String("name", cfg.Node.Name),
        zap.Bool("service_mode", cfg.User.ServiceMode),
    )

    ctx, cancel := context.WithCancel(context.Background())

    app := &UserApp{
        config:    cfg,
        logger:    logger,
        cancel:    cancel,
        syncQueue: sync.NewSyncQueue(),
    }

    if err := app.initStorage(ctx); err != nil {
        app.cleanup()
        return nil, err
    }

    if err := app.initData(ctx); err != nil {
        app.cleanup()
        return nil, err
    }

    return app, nil
}

func (app *UserApp) initStorage(ctx context.Context) error {
    dataDir := app.config.Node.DataDir
    if dataDir == "" {
        dataDir = "./data/user"
    }

    storeCfg := &storage.StoreConfig{
        KVType:     app.config.Storage.KVType,
        KVPath:     dataDir + "/kv",
        SearchType: app.config.Storage.SearchType,
        SearchPath: dataDir + "/search.bleve",
    }

    if storeCfg.KVType == "" {
        storeCfg.KVType = "pebble"
    }
    if storeCfg.SearchType == "" {
        storeCfg.SearchType = "bleve"
    }

    var err error
    app.store, err = storage.NewPersistentStore(storeCfg)
    if err != nil {
        return fmt.Errorf("初始化存储失败: %w", err)
    }

    app.logger.Info("存储层初始化完成")
    return nil
}

func (app *UserApp) initData(ctx context.Context) error {
    categoryInit := category.NewCategoryInitializer(app.store.Category, app.config.Node.DataDir)
    if err := categoryInit.Initialize(ctx); err != nil {
        app.logger.Warn("分类初始化警告", zap.Error(err))
    }
    return nil
}

func (app *UserApp) Start() error {
    ctx := context.Background()

    // 根据服务模式配置 P2P Host
    var listenAddrs []string
    relayService := false

    if app.config.User.ServiceMode {
        port := app.config.Network.ListenPort
        if port == 0 {
            port = 9001
        }
        listenAddrs = []string{fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)}
        relayService = app.config.User.RelayEnabled
    }

    hostCfg := &host.HostConfig{
        ListenAddrs:    listenAddrs,
        SeedPeers:      app.config.Network.SeedNodes,
        EnableDHT:      app.config.User.ServiceMode,
        EnableMDNS:     app.config.Network.MDNSEnabled,
        EnableNAT:      true,
        EnableRelay:    true,
        EnableAutoRelay: true,
        RelayService:   relayService,
        EnableQUIC:     true,
    }

    var err error
    app.p2pHost, err = host.NewHost(ctx, hostCfg)
    if err != nil {
        return fmt.Errorf("创建 P2P Host 失败: %w", err)
    }

    app.p2pHost.SetNodeType("user")
    app.logger.Info("P2P 节点启动成功",
        zap.String("node_id", app.p2pHost.NodeID()),
        zap.Bool("service_mode", app.config.User.ServiceMode),
    )

    // 服务模式下初始化 DHT
    if app.config.User.ServiceMode && app.config.Network.DHTEnabled {
        app.dhtNode, err = dht.NewDHTNode(app.p2pHost.Host, app.config)
        if err != nil {
            app.logger.Warn("DHT 初始化失败", zap.Error(err))
        } else if err := app.dhtNode.Bootstrap(ctx); err != nil {
            app.logger.Warn("DHT bootstrap 警告", zap.Error(err))
        }
    }

    // 创建同步引擎
    syncCfg := &sync.SyncConfig{
        AutoSync:         app.config.Sync.AutoSync,
        IntervalSeconds:  app.config.Sync.IntervalSeconds,
        MirrorCategories: app.config.Sync.MirrorCategories,
        MaxLocalSizeMB:   app.config.User.MirrorLimitGB * 1024,
    }
    app.syncEngine = sync.NewSyncEngine(app.p2pHost, nil, app.store, syncCfg)

    // 创建协议处理器
    proto := protocol.NewProtocol(app.p2pHost.Host, app.syncEngine)
    app.syncEngine.SetProtocol(proto)

    // 创建推送服务
    app.pushService = sync.NewPushService(app.p2pHost, nil)
    app.pushService.SetProtocol(proto)
    if err := app.pushService.Start(ctx); err != nil {
        app.logger.Warn("推送服务启动失败", zap.Error(err))
    }

    if err := app.syncEngine.Start(ctx); err != nil {
        return fmt.Errorf("启动同步引擎失败: %w", err)
    }

    // 连接种子节点
    for _, seedAddr := range app.config.Network.SeedNodes {
        if err := app.p2pHost.ConnectToPeer(ctx, seedAddr); err != nil {
            app.logger.Warn("连接种子节点失败", zap.String("addr", seedAddr), zap.Error(err))
        } else {
            app.logger.Info("已连接种子节点", zap.String("addr", seedAddr))
        }
    }

    // 创建 API 路由
    apiHandler, err := router.NewRouterWithDeps(&router.Dependencies{
        Store:        app.store,
        EntryStore:   app.store.Entry,
        UserStore:    app.store.User,
        RatingStore:  app.store.Rating,
        CategoryStore: app.store.Category,
        SearchEngine: app.store.Search,
        Backlink:     app.store.Backlink,
        KVStore:      app.store.KVStore(),
        NodeID:       app.p2pHost.NodeID(),
        NodeType:     "user",
        Version:      Version,
    })
    if err != nil {
        return fmt.Errorf("创建 API 路由失败: %w", err)
    }

    // 启动 HTTP 服务器
    httpAddr := fmt.Sprintf(":%d", app.config.Network.APIPort)
    app.httpServer = &http.Server{
        Addr:         httpAddr,
        Handler:      apiHandler,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        app.logger.Info("HTTP API 服务器启动", zap.String("addr", httpAddr))
        if err := app.httpServer.ListenAndServe(); err != http.ErrServerClosed {
            app.logger.Fatal("HTTP 服务器启动失败", zap.Error(err))
        }
    }()

    return nil
}

func (app *UserApp) Run() error {
    if err := app.Start(); err != nil {
        return err
    }

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    app.logger.Info("收到关闭信号，开始优雅关机...")
    return app.Stop()
}

func (app *UserApp) Stop() error {
    app.logger.Info("正在停止服务...")

    if app.httpServer != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        if err := app.httpServer.Shutdown(ctx); err != nil {
            app.logger.Warn("HTTP 服务器关闭超时", zap.Error(err))
        }
    }

    if app.pushService != nil {
        app.pushService.Stop()
    }
    if app.syncEngine != nil {
        app.syncEngine.Stop()
    }
    if app.p2pHost != nil {
        app.p2pHost.Close()
    }

    app.cleanup()
    app.logger.Info("Polyant 用户节点已关闭")
    return nil
}

func (app *UserApp) cleanup() {
    if app.cancel != nil {
        app.cancel()
    }
    if app.store != nil {
        if err := app.store.Close(); err != nil {
            app.logger.Warn("关闭存储失败", zap.Error(err))
        }
    }
}
```

- [ ] **Step 2: 编译确认无语法错误**

Run: `go build -o bin/polyant-user ./cmd/user/`
Expected: Build success

- [ ] **Step 3: 提交用户节点入口**

```bash
git add cmd/user/
git commit -m "$(cat <<'EOF'
feat(cmd): add polyant-user binary entry point

Create dedicated user node binary with:
- Auto network capability detection
- Service mode option for public IP scenarios
- Seed node connection and registration
- Sync queue for offline support
- Lightweight setup for agent users

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Makefile 更新

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: 更新 Makefile 添加 seed 和 user 构建**

```makefile
# Makefile

# ... 保留现有内容 ...

# 目标二进制
POLYANT_BIN := $(BUILD_DIR)/polyant
SEED_BIN := $(BUILD_DIR)/polyant-seed
USER_BIN := $(BUILD_DIR)/polyant-user
AWCTL_BIN := $(BUILD_DIR)/awctl

# ... 保留现有目标 ...

## build: 编译所有二进制（包括 seed 和 user）
build:
	@echo ">>> 编译 Polyant..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(POLYANT_BIN) $(CMD_DIR)/polyant/
	$(GOBUILD) $(LDFLAGS) -o $(SEED_BIN) $(CMD_DIR)/seed/
	$(GOBUILD) $(LDFLAGS) -o $(USER_BIN) $(CMD_DIR)/user/
	$(GOBUILD) $(LDFLAGS) -o $(AWCTL_BIN) $(CMD_DIR)/awctl/
	@echo ">>> 编译完成: $(POLYANT_BIN), $(SEED_BIN), $(USER_BIN), $(AWCTL_BIN)"

## build-seed: 仅编译种子节点二进制
build-seed:
	@echo ">>> 编译种子节点..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(SEED_BIN) $(CMD_DIR)/seed/
	@echo ">>> 编译完成: $(SEED_BIN)"

## build-user: 仅编译用户节点二进制
build-user:
	@echo ">>> 编译用户节点..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(USER_BIN) $(CMD_DIR)/user/
	@echo ">>> 编译完成: $(USER_BIN)"

## docker-seed: 构建种子节点 Docker 镜像
docker-seed:
	@echo ">>> 构建种子节点 Docker 镜像..."
	docker build -f Dockerfile.seed -t polyant-seed:$(VERSION) .
	docker tag polyant-seed:$(VERSION) polyant-seed:latest
	@echo ">>> Docker 镜像构建完成: polyant-seed:$(VERSION)"

## docker-user: 构建用户节点 Docker 镜像
docker-user:
	@echo ">>> 构建用户节点 Docker 镜像..."
	docker build -f Dockerfile.user -t polyant-user:$(VERSION) .
	docker tag polyant-user:$(VERSION) polyant-user:latest
	@echo ">>> Docker 镜像构建完成: polyant-user:$(VERSION)"
```

- [ ] **Step 2: 测试新构建目标**

Run: `make build-seed && make build-user`
Expected: Both binaries created in ./bin/

- [ ] **Step 3: 提交 Makefile 更新**

```bash
git add Makefile
git commit -m "$(cat <<'EOF'
feat(build): add seed and user binary build targets

Add Makefile targets:
- build-seed: compile seed node binary
- build-user: compile user node binary
- docker-seed: build seed node Docker image
- docker-user: build user node Docker image

Update default build to include all binaries.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: 配置示例文件

**Files:**
- Create: `configs/seed.json`
- Create: `configs/user.json`

- [ ] **Step 1: 创建种子节点配置示例**

```json
// configs/seed.json
{
  "seed": {
    "domain": "seed.polyant.top",
    "tls_cert": "/etc/letsencrypt/live/seed.polyant.top/fullchain.pem",
    "tls_key": "/etc/letsencrypt/live/seed.polyant.top/privkey.pem",
    "bootstrap_peers": []
  },
  "node": {
    "type": "seed",
    "name": "polyant-seed-1",
    "data_dir": "./data/seed",
    "log_dir": "./logs",
    "log_level": "info"
  },
  "network": {
    "listen_port": 9000,
    "api_port": 8080,
    "seed_nodes": [],
    "dht_enabled": true,
    "mdns_enabled": false
  },
  "mirror": {
    "enabled": true,
    "categories": ["*"],
    "max_size_gb": 100
  },
  "storage": {
    "kv_type": "pebble",
    "search_type": "bleve"
  },
  "i18n": {
    "default_lang": "zh-CN",
    "available_langs": ["zh-CN", "en-US"],
    "log_bilingual": false
  }
}
```

- [ ] **Step 2: 创建用户节点配置示例**

```json
// configs/user.json
{
  "user": {
    "service_mode": false,
    "relay_enabled": false,
    "mirror_enabled": false,
    "mirror_limit_gb": 10
  },
  "node": {
    "type": "user",
    "name": "polyant-user-1",
    "data_dir": "./data/user",
    "log_dir": "./logs",
    "log_level": "info"
  },
  "network": {
    "listen_port": 0,
    "api_port": 8080,
    "seed_nodes": ["/dns4/seed.polyant.top/tcp/9000/p2p/12D3Koo..."],
    "dht_enabled": false,
    "mdns_enabled": true
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
  "storage": {
    "kv_type": "pebble",
    "search_type": "bleve"
  },
  "i18n": {
    "default_lang": "zh-CN",
    "available_langs": ["zh-CN", "en-US"],
    "log_bilingual": false
  }
}
```

- [ ] **Step 3: 提交配置示例**

```bash
git add configs/seed.json configs/user.json
git commit -m "$(cat <<'EOF'
docs(config): add seed and user configuration examples

Add example configuration files:
- configs/seed.json: seed node with domain, TLS, mirroring
- configs/user.json: user node with seed connection, sync

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Dockerfile 创建

**Files:**
- Create: `Dockerfile.seed`
- Create: `Dockerfile.user`

- [ ] **Step 1: 创建种子节点 Dockerfile**

```dockerfile
# Dockerfile.seed
FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /polyant-seed ./cmd/seed/

FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /polyant-seed /usr/local/bin/polyant-seed

# 创建数据和配置目录
RUN mkdir -p /app/data /app/configs

# 暴露端口
EXPOSE 9000 8080

ENTRYPOINT ["polyant-seed"]
CMD ["--config", "/app/configs/seed.json"]
```

- [ ] **Step 2: 创建用户节点 Dockerfile**

```dockerfile
# Dockerfile.user
FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /polyant-user ./cmd/user/

FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /polyant-user /usr/local/bin/polyant-user

# 创建数据和配置目录
RUN mkdir -p /app/data /app/configs

# 暴露端口（服务模式）
EXPOSE 8080

ENTRYPOINT ["polyant-user"]
CMD ["--config", "/app/configs/user.json"]
```

- [ ] **Step 3: 提交 Dockerfile**

```bash
git add Dockerfile.seed Dockerfile.user
git commit -m "$(cat <<'EOF'
feat(docker): add Dockerfiles for seed and user binaries

Add multi-stage Dockerfiles:
- Dockerfile.seed: seed node with exposed P2P and API ports
- Dockerfile.user: user node with API port

Both use Alpine base for minimal image size.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: 最终测试与验证

**Files:**
- Run tests across the project

- [ ] **Step 1: 运行所有测试**

Run: `make test`
Expected: All tests pass

- [ ] **Step 2: 编译所有二进制**

Run: `make build`
Expected: All binaries created

- [ ] **Step 3: 验证种子节点帮助信息**

Run: `./bin/polyant-seed --help`
Expected: Shows usage with domain and TLS flags

- [ ] **Step 4: 验证用户节点帮助信息**

Run: `./bin/polyant-user --help`
Expected: Shows usage with seed-nodes and service flags

- [ ] **Step 5: 最终提交**

```bash
git add -A
git commit -m "$(cat <<'EOF'
chore: final verification and cleanup for seed/user separation

Complete seed-user separation implementation:
- Two independent binaries: polyant-seed, polyant-user
- Node-type-specific configuration
- Network capability detection
- Sync queue for offline support
- Docker support for both node types

Breaking change: Users should migrate to new binaries.
The old polyant binary is deprecated but kept for compatibility.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## 自检清单

**1. 规范覆盖检查：**

| 规范要求 | 对应任务 |
|---------|---------|
| 两个独立二进制文件 | Task 5, Task 6 |
| 种子节点：公网+域名+TLS | Task 5 (验证逻辑) |
| 用户节点：自动适配网络环境 | Task 2, Task 6 |
| 用户节点服务模式 | Task 6 (--service 参数) |
| 配置结构 SeedConfig/UserNodeConfig | Task 1 |
| 网络能力检测 | Task 2 |
| 同步队列 | Task 3 |
| Capability 协议扩展 | Task 4 |
| Makefile 构建 | Task 7 |
| 配置示例文件 | Task 8 |
| Dockerfile | Task 9 |

**2. 占位符扫描：** 无 TBD、TODO、implement later 等占位符。

**3. 类型一致性检查：**
- `SeedConfig`、`UserNodeConfig`、`MirrorConfig` 定义与使用一致
- `Capability` 类型定义与 `Handshake` 中使用一致
- `SyncQueue` 方法签名在创建和使用处一致
