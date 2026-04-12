// Package dht DHT 路由发现实现
// 基于 go-libp2p 提供的 DHT 服务，用于节点发现和路由
package dht

import (
	"context"
	"fmt"

	"github.com/daifei0527/agentwiki/pkg/config"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// DHTNode DHT 路由节点
type DHTNode struct {
	dht  *dht.IpfsDHT
	host host.Host
	cfg  *config.Config
	log  *zap.SugaredLogger
}

// NewDHTNode 创建新的 DHT 节点
func NewDHTNode(h host.Host, cfg *config.Config) (*DHTNode, error) {
	log := zap.S().With("module", "dht")

	// 创建 DHT 实例
	kadDHT, err := dht.New(context.Background(), h, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, fmt.Errorf("create DHT failed: %w", err)
	}

	log.Infof("DHT node created, mode: server")

	return &DHTNode{
		dht:  kadDHT,
		host: h,
		cfg:  cfg,
		log:  log,
	}, nil
}

// Bootstrap 引导 DHT 网络连接到 bootstrap 节点
func (d *DHTNode) Bootstrap(ctx context.Context) error {
	d.log.Info("Bootstrapping DHT...")

	// 如果配置了种子节点，使用配置的，否则使用默认
	peers := dht.GetDefaultBootstrapPeerAddrInfos()
	// 将配置的添加进去
	if d.cfg.Network.SeedNodes != nil && len(d.cfg.Network.SeedNodes) > 0 {
		for _, addrInfoStr := range d.cfg.Network.SeedNodes {
			// 解析 multiaddr 字符串
			maddr, err := multiaddr.NewMultiaddr(addrInfoStr)
			if err != nil {
				d.log.Warnf("Failed to parse seed node address %s: %v", addrInfoStr, err)
				continue
			}
			pi, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				d.log.Warnf("Failed to get peer info from address %s: %v", addrInfoStr, err)
				continue
			}
			peers = append(peers, *pi)
		}
	}

	if err := d.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("DHT bootstrap failed: %w", err)
	}

	d.log.Infof("DHT bootstrap completed with %d bootstrap peers", len(peers))
	return nil
}

// FindPeer 通过 DHT 查找对端节点信息
func (d *DHTNode) FindPeer(ctx context.Context, pid peer.ID) (peer.AddrInfo, error) {
	return d.dht.FindPeer(ctx, pid)
}

// GetRouting 获取路由接口
func (d *DHTNode) GetRouting() routing.Routing {
	return d.dht
}

// Close 关闭 DHT
func (d *DHTNode) Close() error {
	return d.dht.Close()
}
