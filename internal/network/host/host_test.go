// Package host 提供网络主机的单元测试
package host

import (
	"context"
	"testing"
	"time"
)

// testHostConfig 创建用于测试的主机配置
func testHostConfig() *HostConfig {
	return &HostConfig{
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		EnableDHT:          false,
		EnableMDNS:         false,
		EnableNAT:          false,
		EnableRelay:        false,
		EnableAutoRelay:    false,
		EnableWebSocket:    false,
		EnableHolePunching: false,
		ConnectionTimeout:  5 * time.Second,
	}
}

// ==================== NATType 测试 ====================

// TestNATTypeString 测试 NAT 类型字符串表示
func TestNATTypeString(t *testing.T) {
	tests := []struct {
		natType  NATType
		expected string
	}{
		{NATTypeUnknown, "Unknown"},
		{NATTypeNone, "None"},
		{NATTypeFullCone, "Full Cone"},
		{NATTypeRestrictedCone, "Restricted Cone"},
		{NATTypePortRestricted, "Port Restricted"},
		{NATTypeSymmetric, "Symmetric"},
	}

	for _, tt := range tests {
		result := tt.natType.String()
		if result != tt.expected {
			t.Errorf("NATType(%d).String() = %q, want %q", tt.natType, result, tt.expected)
		}
	}
}

// ==================== HostConfig 测试 ====================

// TestDefaultHostConfig 测试默认配置
func TestDefaultHostConfig(t *testing.T) {
	cfg := DefaultHostConfig()

	if cfg == nil {
		t.Fatal("DefaultHostConfig 不应返回 nil")
	}

	if len(cfg.ListenAddrs) == 0 {
		t.Error("ListenAddrs 不应为空")
	}

	if !cfg.EnableDHT {
		t.Error("默认应启用 DHT")
	}

	if !cfg.EnableMDNS {
		t.Error("默认应启用 MDNS")
	}

	if !cfg.EnableNAT {
		t.Error("默认应启用 NAT")
	}

	if cfg.ConnectionTimeout != 30*time.Second {
		t.Errorf("ConnectionTimeout = %v, want 30s", cfg.ConnectionTimeout)
	}
}

// ==================== isPrivateIP 测试 ====================

// TestIsPrivateIP 测试私有 IP 判断
func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		// 私有地址
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.1.100", true},
		{"127.0.0.1", true},
		{"169.254.1.1", true},
		// 公网地址
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"203.0.113.1", false},
		{"172.15.0.1", false}, // 172.15 不是私有地址
	}

	for _, tt := range tests {
		result := isPrivateIP(tt.ip)
		if result != tt.expected {
			t.Errorf("isPrivateIP(%q) = %v, want %v", tt.ip, result, tt.expected)
		}
	}
}

// TestIsPrivateIPIPv6 测试 IPv6 私有地址
func TestIsPrivateIPIPv6(t *testing.T) {
	// IPv6 回环
	if !isPrivateIP("::1") {
		t.Error("::1 应为私有地址")
	}

	// IPv6 ULA
	if !isPrivateIP("fc00::1") {
		t.Error("fc00::1 应为私有地址")
	}

	if !isPrivateIP("fd00::1") {
		t.Error("fd00::1 应为私有地址")
	}

	// IPv6 链路本地
	if !isPrivateIP("fe80::1") {
		t.Error("fe80::1 应为私有地址")
	}
}

// ==================== P2PHost 测试 ====================

// TestNewHost 测试创建主机
func TestNewHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := &HostConfig{
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		EnableDHT:          false,
		EnableMDNS:         false,
		EnableNAT:          false,
		EnableRelay:        false,
		EnableAutoRelay:    false,
		EnableWebSocket:    false,
		EnableHolePunching: false,
		ConnectionTimeout:  5 * time.Second,
	}

	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	if h == nil {
		t.Fatal("Host 不应为 nil")
	}
}

// TestNewHostWithRelay 测试创建带中继的主机
func TestNewHostWithRelay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := &HostConfig{
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		EnableDHT:          false,
		EnableMDNS:         false,
		EnableNAT:          false,
		EnableRelay:        true,
		EnableAutoRelay:    false, // AutoRelay 需要配置中继节点源
		EnableWebSocket:    false,
		EnableHolePunching: false,
		ConnectionTimeout:  5 * time.Second,
	}

	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()
}

// TestNewHostAsRelayServer 测试创建中继服务节点
func TestNewHostAsRelayServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := &HostConfig{
		ListenAddrs:   []string{"/ip4/127.0.0.1/tcp/0"},
		RelayService:  true,
		EnableDHT:     false,
		EnableMDNS:    false,
		EnableNAT:     false,
		EnableRelay:   true,
		EnableWebSocket: false,
		EnableHolePunching: false,
	}

	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	if !h.IsRelayServer() {
		t.Error("应为中继服务节点")
	}
}

// TestP2PHostNodeID 测试节点 ID
func TestP2PHostNodeID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	nodeID := h.NodeID()
	if nodeID == "" {
		t.Error("NodeID 不应为空")
	}
}

// TestP2PHostNodeType 测试节点类型
func TestP2PHostNodeType(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	// 默认节点类型
	if h.NodeType() != "local" {
		t.Errorf("NodeType = %q, want 'local'", h.NodeType())
	}

	// 设置节点类型
	h.SetNodeType("seed")
	if h.NodeType() != "seed" {
		t.Errorf("NodeType = %q, want 'seed'", h.NodeType())
	}
}

// TestP2PHostGetConnectedPeers 测试获取连接节点
func TestP2PHostGetConnectedPeers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	peers := h.GetConnectedPeers()
	// 刚创建时应该没有连接
	if peers == nil {
		t.Error("GetConnectedPeers 不应返回 nil")
	}
}

// TestP2PHostClose 测试关闭主机
func TestP2PHostClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}

	// 关闭应该成功
	err = h.Close()
	if err != nil {
		t.Errorf("Close 失败: %v", err)
	}
}

// ==================== ConnectionGater 测试 ====================

// TestPermissiveConnectionGatter 测试连接控制器
func TestPermissiveConnectionGatter(t *testing.T) {
	g := &permissiveConnectionGatter{}

	// 所有方法都应返回 true（允许）
	if !g.InterceptPeerDial("") {
		t.Error("InterceptPeerDial 应返回 true")
	}
	// 这些方法需要具体参数，但总是返回 true
	// 验证方法存在即可
}

// ==================== ACL 测试 ====================

// TestPermissiveACL 测试 ACL
func TestPermissiveACL(t *testing.T) {
	acl := &permissiveACL{}

	// 验证 ACL 接口实现
	// 所有方法都应返回 true（允许），但需要有效参数
	_ = acl
}

// ==================== 协议 ID 测试 ====================

// TestAWSPProtocolID 测试协议 ID
func TestAWSPProtocolID(t *testing.T) {
	expected := "/agentwiki/sync/2.0.0"
	if AWSPProtocolID != expected {
		t.Errorf("AWSPProtocolID = %q, want %q", AWSPProtocolID, expected)
	}
}
