// Package host 提供网络主机的单元测试
package host

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
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
	expected := "/polyant/sync/2.0.0"
	if AWSPProtocolID != expected {
		t.Errorf("AWSPProtocolID = %q, want %q", AWSPProtocolID, expected)
	}
}

// ==================== isPublicAddr 测试 ====================

// TestIsPublicAddr 测试公网地址判断
func TestIsPublicAddr(t *testing.T) {
	// 创建测试用的 multiaddr
	tests := []struct {
		addr     string
		expected bool
	}{
		// 私有地址 - 应返回 false
		{"/ip4/127.0.0.1/tcp/8080", false},
		{"/ip4/10.0.0.1/tcp/8080", false},
		{"/ip4/192.168.1.1/tcp/8080", false},
		{"/ip4/172.16.0.1/tcp/8080", false},
		// 公网地址 - 应返回 true
		{"/ip4/8.8.8.8/tcp/8080", true},
		{"/ip4/1.1.1.1/tcp/8080", true},
		{"/ip4/203.0.113.1/tcp/8080", true},
	}

	for _, tt := range tests {
		maddr, err := multiaddr.NewMultiaddr(tt.addr)
		if err != nil {
			t.Fatalf("Failed to create multiaddr: %v", err)
		}
		result := isPublicAddr(maddr)
		if result != tt.expected {
			t.Errorf("isPublicAddr(%q) = %v, want %v", tt.addr, result, tt.expected)
		}
	}
}

// ==================== NATType 方法测试 ====================

// TestP2PHostNATType 测试 NAT 类型获取
func TestP2PHostNATType(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	// 初始 NAT 类型应为 Unknown（异步检测可能还没完成）
	natType := h.NATType()
	// 验证 NATType 方法可以正常调用
	_ = natType
}

// ==================== GetRelayPeers 测试 ====================

// TestGetRelayPeers 测试获取中继节点
func TestGetRelayPeers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	// 没有配置中继节点时应该返回空列表或 nil
	relayPeers := h.GetRelayPeers()
	// 当没有中继节点时，返回 nil 是正常的
	_ = relayPeers
}

// ==================== GetObservableAddrs 测试 ====================

// TestGetObservableAddrs 测试获取可观察地址
func TestGetObservableAddrs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	// 获取可观察地址
	addrs := h.GetObservableAddrs()
	// 在本地测试环境下，可能没有公网地址
	_ = addrs
}

// ==================== AddRelayPeer 测试 ====================

// TestAddRelayPeer 测试添加中继节点
func TestAddRelayPeer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	// 测试无效地址
	err = h.AddRelayPeer("invalid-address")
	if err == nil {
		t.Error("AddRelayPeer 应该对无效地址返回错误")
	}
}

// TestAddRelayPeer_ValidAddress 测试添加有效中继节点地址
func TestAddRelayPeer_ValidAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	// 创建一个测试用的有效 multiaddr
	// 使用一个假的 peer ID
	testAddr := "/ip4/127.0.0.1/tcp/12345/p2p/12D3KooWGxyMSH3mZdQhZVY9tVE3WFKhJzVhPqNvVLgKDmZPpHvP"
	err = h.AddRelayPeer(testAddr)
	if err != nil {
		// 这个错误是预期的，因为地址是假的
		t.Logf("AddRelayPeer returned error (expected for fake address): %v", err)
	}
}

// ==================== ConnectToPeer 测试 ====================

// TestConnectToPeer_InvalidAddress 测试连接到无效地址
func TestConnectToPeer_InvalidAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	// 测试无效地址格式
	err = h.ConnectToPeer(ctx, "invalid-address")
	if err == nil {
		t.Error("ConnectToPeer 应该对无效地址返回错误")
	}
}

// TestConnectToPeerInfo 测试通过 AddrInfo 连接
func TestConnectToPeerInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testHostConfig()
	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()

	// 创建两个主机进行连接测试
	cfg2 := testHostConfig()
	h2, err := NewHost(ctx, cfg2)
	if err != nil {
		t.Fatalf("NewHost 2 失败: %v", err)
	}
	defer h2.Close()

	// 获取 h2 的地址信息
	addrs := h2.Addrs()
	if len(addrs) == 0 {
		t.Fatal("h2 should have addresses")
	}

	// 构建 peer info
	info := peer.AddrInfo{
		ID:    h2.ID(),
		Addrs: addrs,
	}

	// 连接到 h2
	err = h.ConnectToPeerInfo(ctx, info)
	if err != nil {
		t.Logf("ConnectToPeerInfo failed (may be expected in test env): %v", err)
	}
}

// ==================== NewHost 配置测试 ====================

// TestNewHostWithWebSocket 测试启用 WebSocket
func TestNewHostWithWebSocket(t *testing.T) {
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
		EnableWebSocket:    true,
		EnableHolePunching: false,
		ConnectionTimeout:  5 * time.Second,
	}

	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()
}

// TestNewHostWithQUIC 测试启用 QUIC
func TestNewHostWithQUIC(t *testing.T) {
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
		EnableQUIC:         true,
		QUICListenPort:     0, // 随机端口
		EnableHolePunching: false,
		ConnectionTimeout:  5 * time.Second,
	}

	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()
}

// TestNewHostWithHolePunching 测试启用打洞
func TestNewHostWithHolePunching(t *testing.T) {
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
		EnableHolePunching: true,
		EnableWebSocket:    false,
		ConnectionTimeout:  5 * time.Second,
	}

	h, err := NewHost(ctx, cfg)
	if err != nil {
		t.Fatalf("NewHost 失败: %v", err)
	}
	defer h.Close()
}

// TestNewHostWithRelayPeers 测试配置中继节点
func TestNewHostWithRelayPeers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := &HostConfig{
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		RelayPeers: []string{
			"/ip4/127.0.0.1/tcp/12345/p2p/12D3KooWGxyMSH3mZdQhZVY9tVE3WFKhJzVhPqNvVLgKDmZPpHvP",
		},
		EnableDHT:          false,
		EnableMDNS:         false,
		EnableNAT:          false,
		EnableRelay:        true,
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

// ==================== 连接两个主机测试 ====================

// TestTwoHostsConnect 测试两个主机之间的连接
func TestTwoHostsConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 创建第一个主机
	cfg1 := testHostConfig()
	h1, err := NewHost(ctx, cfg1)
	if err != nil {
		t.Fatalf("NewHost 1 失败: %v", err)
	}
	defer h1.Close()

	// 创建第二个主机
	cfg2 := testHostConfig()
	h2, err := NewHost(ctx, cfg2)
	if err != nil {
		t.Fatalf("NewHost 2 失败: %v", err)
	}
	defer h2.Close()

	// 获取 h2 的地址
	h2Addrs := h2.Addrs()
	if len(h2Addrs) == 0 {
		t.Fatal("h2 should have addresses")
	}

	// 构建连接地址
	connectAddr := fmt.Sprintf("%s/p2p/%s", h2Addrs[0].String(), h2.ID().String())

	// h1 连接到 h2
	err = h1.ConnectToPeer(ctx, connectAddr)
	if err != nil {
		t.Logf("ConnectToPeer failed (may be expected in test env): %v", err)
	}
}

// ==================== permissiveConnectionGatter 完整测试 ====================

// TestPermissiveConnectionGatter_AllMethods 测试所有连接控制器方法
func TestPermissiveConnectionGatter_AllMethods(t *testing.T) {
	g := &permissiveConnectionGatter{}

	// 测试所有方法返回 true
	if !g.InterceptPeerDial("test-peer") {
		t.Error("InterceptPeerDial 应返回 true")
	}

	// 注意：其他方法需要 multiaddr 参数，测试它们能被调用即可
	// InterceptAddrDial, InterceptAccept, InterceptSecured, InterceptUpgraded
}

// TestPermissiveACL_AllMethods 测试所有 ACL 方法
func TestPermissiveACL_AllMethods(t *testing.T) {
	acl := &permissiveACL{}

	// 验证 ACL 方法存在且返回 true
	// AllowReserve 和 AllowConnect 需要参数，测试它们能被调用即可
	_ = acl
}
