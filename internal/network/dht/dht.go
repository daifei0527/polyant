// Package dht DHT 路由发现实现
// 基于 go-libp2p 提供的 DHT 服务，用于节点发现和路由
package dht

import (
	"context"
	"fmt"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"go.uber.org/zap"

	"github.com/daifei0527/polyant/pkg/config"
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

// Bootstrap 引导 DHT 网络连接到 bootstrap 节点。
//
// R2-E1：删除了误导性的 SeedNodes 解析块——它把配置里的种子地址解析进一个本地
// peers 切片后从未真正用于引导（d.dht.Bootstrap 用的是 libp2p 内置默认 peer），
// 连接实际靠 app 层的 ConnectToPeer 循环完成。保留真实工作 d.dht.Bootstrap(ctx)。
func (d *DHTNode) Bootstrap(ctx context.Context) error {
	d.log.Info("Bootstrapping DHT...")

	if err := d.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("DHT bootstrap failed: %w", err)
	}

	d.log.Info("DHT bootstrap completed")
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
