package host

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"
	"github.com/multiformats/go-multiaddr"
)

const AWSPProtocolID = "/agentwiki/sync/2.0.0"

// NATType represents the type of NAT detected
type NATType int

const (
	NATTypeUnknown NATType = iota
	NATTypeNone
	NATTypeFullCone
	NATTypeRestrictedCone
	NATTypePortRestricted
	NATTypeSymmetric
)

func (n NATType) String() string {
	switch n {
	case NATTypeNone:
		return "None"
	case NATTypeFullCone:
		return "Full Cone"
	case NATTypeRestrictedCone:
		return "Restricted Cone"
	case NATTypePortRestricted:
		return "Port Restricted"
	case NATTypeSymmetric:
		return "Symmetric"
	default:
		return "Unknown"
	}
}

// HostConfig P2P主机配置
type HostConfig struct {
	// ListenAddrs 监听地址列表
	ListenAddrs []string
	// SeedPeers 种子节点地址列表
	SeedPeers []string
	// RelayPeers 中继节点地址列表（用于NAT穿透）
	RelayPeers []string
	// EnableDHT 是否启用DHT
	EnableDHT bool
	// EnableMDNS 是否启用mDNS本地发现
	EnableMDNS bool
	// EnableNAT 是否启用NAT端口映射
	EnableNAT bool
	// EnableRelay 是否启用中继功能
	EnableRelay bool
	// EnableAutoRelay 是否启用自动中继（当无法直连时自动使用中继）
	EnableAutoRelay bool
	// EnableWebSocket 是否启用WebSocket传输（有助于穿透防火墙）
	EnableWebSocket bool
	// EnableHolePunching 是否启用打洞功能
	EnableHolePunching bool
	// PrivateKey 节点私钥
	PrivateKey crypto.PrivKey
	// RelayService 是否作为中继服务节点
	RelayService bool
	// ConnectionTimeout 连接超时
	ConnectionTimeout time.Duration
}

// DefaultHostConfig 返回默认主机配置
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
		ConnectionTimeout:  30 * time.Second,
	}
}

// P2PHost P2P主机封装
type P2PHost struct {
	host.Host
	nodeID      string
	nodeType    string
	version     string
	natType     NATType
	relayPeers  []peer.AddrInfo
	relayServer *relay.Relay
}

// NewHost 创建新的P2P主机
func NewHost(ctx context.Context, cfg *HostConfig) (*P2PHost, error) {
	privKey := cfg.PrivateKey
	if privKey == nil {
		var err error
		privKey, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate key: %w", err)
		}
	}

	// 基础选项
	opts := []libp2p.Option{
		libp2p.Identity(privKey),
		libp2p.UserAgent("agentwiki/1.0.0"),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.ListenAddrStrings(cfg.ListenAddrs...),
	}

	// 传输层配置
	transports := []libp2p.Option{
		libp2p.Transport(tcp.NewTCPTransport),
	}

	// 添加WebSocket传输（有助于穿透防火墙）
	if cfg.EnableWebSocket {
		transports = append(transports, libp2p.Transport(websocket.New))
	}

	opts = append(opts, transports...)

	// NAT端口映射
	if cfg.EnableNAT {
		opts = append(opts, libp2p.NATPortMap())
	}

	// 中继功能配置
	if cfg.EnableRelay {
		// 启用中继客户端（允许通过中继连接）
		opts = append(opts, libp2p.EnableRelay())

		// 启用自动中继（当无法直连时自动使用中继）
		if cfg.EnableAutoRelay {
			opts = append(opts, libp2p.EnableAutoRelay())
		}
	}

	// 打洞功能（用于NAT穿透）
	if cfg.EnableHolePunching {
		opts = append(opts, libp2p.EnableHolePunching())
	}

	// 连接管理
	opts = append(opts,
		libp2p.ConnectionGater(&permissiveConnectionGatter{}),
	)

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create libp2p host: %w", err)
	}

	p2pHost := &P2PHost{
		Host:     h,
		nodeID:   h.ID().String(),
		nodeType: "local",
		version:  "1.0.0",
		natType:  NATTypeUnknown,
	}

	// 如果配置为中继服务节点，启动中继服务
	if cfg.RelayService {
		relayOpts := []relay.Option{
			relay.WithResources(relay.DefaultResources()),
			relay.WithACL(&permissiveACL{}),
		}

		p2pHost.relayServer, err = relay.New(h, relayOpts...)
		if err != nil {
			log.Printf("[P2PHost] Failed to start relay service: %v", err)
		} else {
			log.Printf("[P2PHost] Relay service started")
		}
	}

	// 解析中继节点地址
	if len(cfg.RelayPeers) > 0 {
		for _, addr := range cfg.RelayPeers {
			maddr, err := multiaddr.NewMultiaddr(addr)
			if err != nil {
				log.Printf("[P2PHost] Invalid relay peer address %s: %v", addr, err)
				continue
			}

			info, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				log.Printf("[P2PHost] Failed to parse relay peer info %s: %v", addr, err)
				continue
			}

			p2pHost.relayPeers = append(p2pHost.relayPeers, *info)

			// 添加到peerstore以便后续连接
			h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		}
		log.Printf("[P2PHost] Configured %d relay peers", len(p2pHost.relayPeers))
	}

	// 检测NAT类型
	go p2pHost.detectNATType(ctx)

	return p2pHost, nil
}

// detectNATType 检测NAT类型
func (h *P2PHost) detectNATType(ctx context.Context) {
	// 等待一段时间让网络稳定
	time.Sleep(5 * time.Second)

	// 检查是否有公网地址
	hasPublic := false
	for _, addr := range h.Addrs() {
		// 检查是否为公网地址（非私有IP）
		if isPublicAddr(addr) {
			hasPublic = true
			break
		}
	}

	if hasPublic {
		h.natType = NATTypeNone
		log.Printf("[P2PHost] NAT detection: No NAT detected (public IP)")
	} else {
		// 假设为对称NAT（最常见于家庭和企业网络）
		// 实际检测需要STUN服务器
		h.natType = NATTypeSymmetric
		log.Printf("[P2PHost] NAT detection: Behind NAT (assuming Symmetric)")
	}

	// 如果在NAT后面且启用了中继，尝试连接中继节点
	if h.natType != NATTypeNone && len(h.relayPeers) > 0 {
		h.connectToRelays(ctx)
	}
}

// connectToRelays 连接到中继节点
func (h *P2PHost) connectToRelays(ctx context.Context) {
	for _, info := range h.relayPeers {
		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := h.Connect(connectCtx, info); err != nil {
			log.Printf("[P2PHost] Failed to connect to relay %s: %v", info.ID.String()[:8], err)
		} else {
			log.Printf("[P2PHost] Connected to relay %s", info.ID.String()[:8])
		}
		cancel()
	}
}

// NodeID 返回节点ID
func (h *P2PHost) NodeID() string {
	return h.nodeID
}

// NodeType 返回节点类型
func (h *P2PHost) NodeType() string {
	return h.nodeType
}

// SetNodeType 设置节点类型
func (h *P2PHost) SetNodeType(nodeType string) {
	h.nodeType = nodeType
}

// NATType 返回NAT类型
func (h *P2PHost) NATType() NATType {
	return h.natType
}

// ConnectToPeer 连接到指定节点
func (h *P2PHost) ConnectToPeer(ctx context.Context, addr string) error {
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return fmt.Errorf("parse multiaddr: %w", err)
	}

	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("get peer info: %w", err)
	}

	if err := h.Connect(ctx, *info); err != nil {
		return fmt.Errorf("connect to peer: %w", err)
	}

	return nil
}

// ConnectToPeerInfo 通过AddrInfo连接到节点
func (h *P2PHost) ConnectToPeerInfo(ctx context.Context, info peer.AddrInfo) error {
	if err := h.Connect(ctx, info); err != nil {
		return fmt.Errorf("connect to peer: %w", err)
	}
	return nil
}

// GetConnectedPeers 获取已连接的节点列表
func (h *P2PHost) GetConnectedPeers() []peer.ID {
	return h.Network().Peers()
}

// GetRelayPeers 获取中继节点列表
func (h *P2PHost) GetRelayPeers() []peer.ID {
	var relayPeerIDs []peer.ID
	for _, info := range h.relayPeers {
		relayPeerIDs = append(relayPeerIDs, info.ID)
	}
	return relayPeerIDs
}

// IsRelayServer 返回是否为中继服务节点
func (h *P2PHost) IsRelayServer() bool {
	return h.relayServer != nil
}

// GetObservableAddrs 获取可被外部观察到的地址
func (h *P2PHost) GetObservableAddrs() []multiaddr.Multiaddr {
	var publicAddrs []multiaddr.Multiaddr
	for _, addr := range h.Addrs() {
		if isPublicAddr(addr) {
			publicAddrs = append(publicAddrs, addr)
		}
	}
	return publicAddrs
}

// AddRelayPeer 添加中继节点
func (h *P2PHost) AddRelayPeer(addr string) error {
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return fmt.Errorf("parse multiaddr: %w", err)
	}

	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("get peer info: %w", err)
	}

	h.relayPeers = append(h.relayPeers, *info)
	h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)

	return nil
}

// Close 关闭主机
func (h *P2PHost) Close() error {
	if h.relayServer != nil {
		h.relayServer.Close()
	}
	return h.Host.Close()
}

// isPublicAddr 检查地址是否为公网地址
func isPublicAddr(addr multiaddr.Multiaddr) bool {
	// 提取IP地址
	for _, proto := range []int{
		multiaddr.P_IP4,
		multiaddr.P_IP6,
	} {
		ip, err := addr.ValueForProtocol(proto)
		if err == nil {
			// 检查是否为私有地址
			if isPrivateIP(ip) {
				return false
			}
			return true
		}
	}
	return false
}

// isPrivateIP 检查IP是否为私有地址
func isPrivateIP(ip string) bool {
	// 常见私有IP段
	privateRanges := []string{
		"10.",       // 10.0.0.0/8
		"172.16.",   // 172.16.0.0/12 (部分)
		"172.17.",   // 172.16.0.0/12 (部分)
		"172.18.",   // 172.16.0.0/12 (部分)
		"172.19.",   // 172.16.0.0/12 (部分)
		"172.20.",   // 172.16.0.0/12 (部分)
		"172.21.",   // 172.16.0.0/12 (部分)
		"172.22.",   // 172.16.0.0/12 (部分)
		"172.23.",   // 172.16.0.0/12 (部分)
		"172.24.",   // 172.16.0.0/12 (部分)
		"172.25.",   // 172.16.0.0/12 (部分)
		"172.26.",   // 172.16.0.0/12 (部分)
		"172.27.",   // 172.16.0.0/12 (部分)
		"172.28.",   // 172.16.0.0/12 (部分)
		"172.29.",   // 172.16.0.0/12 (部分)
		"172.30.",   // 172.16.0.0/12 (部分)
		"172.31.",   // 172.16.0.0/12 (部分)
		"192.168.",  // 192.168.0.0/16
		"127.",      // 127.0.0.0/8 (回环)
		"169.254.",  // 169.254.0.0/16 (链路本地)
		"::1",       // IPv6 回环
		"fc",        // IPv6 ULA (fc00::/7)
		"fd",        // IPv6 ULA (fc00::/7)
		"fe80",      // IPv6 链路本地
	}

	for _, prefix := range privateRanges {
		if len(ip) >= len(prefix) && ip[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// permissiveConnectionGatter 允许所有连接
type permissiveConnectionGatter struct{}

func (g *permissiveConnectionGatter) InterceptPeerDial(_ peer.ID) bool {
	return true
}

func (g *permissiveConnectionGatter) InterceptAddrDial(_ peer.ID, _ multiaddr.Multiaddr) bool {
	return true
}

func (g *permissiveConnectionGatter) InterceptAccept(_ network.ConnMultiaddrs) bool {
	return true
}

func (g *permissiveConnectionGatter) InterceptSecured(_ network.Direction, _ peer.ID, _ network.ConnMultiaddrs) bool {
	return true
}

func (g *permissiveConnectionGatter) InterceptUpgraded(_ network.Conn) (bool, control.DisconnectReason) {
	return true, 0
}

// 确保 permissiveConnectionGatter 实现 connmgr.ConnectionGater 接口
var _ connmgr.ConnectionGater = (*permissiveConnectionGatter)(nil)

// permissiveACL 允许所有中继请求
type permissiveACL struct{}

func (a *permissiveACL) AllowReserve(_ peer.ID, _ multiaddr.Multiaddr) bool {
	return true
}

func (a *permissiveACL) AllowConnect(_ peer.ID, _ multiaddr.Multiaddr, _ peer.ID) bool {
	return true
}

// 确保 permissiveACL 实现 relay.ACLFilter 接口
var _ relay.ACLFilter = (*permissiveACL)(nil)
