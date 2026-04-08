package host

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
)

const AWSPProtocolID = "/agentwiki/sync/1.0.0"

type HostConfig struct {
	ListenAddrs []string
	SeedPeers   []string
	EnableDHT   bool
	EnableMDNS  bool
	EnableNAT   bool
	EnableRelay bool
	PrivateKey  crypto.PrivKey
}

type P2PHost struct {
	host.Host
	nodeID   string
	nodeType string
	version  string
}

func NewHost(ctx context.Context, cfg *HostConfig) (*P2PHost, error) {
	privKey := cfg.PrivateKey
	if privKey == nil {
		var err error
		privKey, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate key: %w", err)
		}
	}

	opts := []libp2p.Option{
		libp2p.Identity(privKey),
		libp2p.UserAgent("agentwiki/1.0.0"),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.ListenAddrStrings(cfg.ListenAddrs...),
	}

	if cfg.EnableNAT {
		opts = append(opts, libp2p.NATPortMap())
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create libp2p host: %w", err)
	}

	return &P2PHost{
		Host:     h,
		nodeID:   h.ID().String(),
		nodeType: "local",
		version:  "1.0.0",
	}, nil
}

func (h *P2PHost) NodeID() string {
	return h.nodeID
}

func (h *P2PHost) NodeType() string {
	return h.nodeType
}

func (h *P2PHost) SetNodeType(nodeType string) {
	h.nodeType = nodeType
}

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

func (h *P2PHost) GetConnectedPeers() []peer.ID {
	return h.Network().Peers()
}

func (h *P2PHost) Close() error {
	return h.Host.Close()
}
