// Package dht_test 提供分布式哈希表的单元测试
package dht_test

import (
	"context"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/network/dht"
	"github.com/daifei0527/polyant/internal/network/host"
	"github.com/daifei0527/polyant/pkg/config"
)

// testHostConfig 创建测试用主机配置
func testHostConfig() *host.HostConfig {
	return &host.HostConfig{
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

// createTestHost 创建测试用 P2P 主机
func createTestHost(t *testing.T) *host.P2PHost {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h, err := host.NewHost(ctx, testHostConfig())
	if err != nil {
		t.Fatalf("创建测试主机失败: %v", err)
	}
	return h
}

// ==================== DHTNode 测试 ====================

// TestNewDHTNode 测试创建 DHT 节点
func TestNewDHTNode(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	cfg := &config.Config{}
	dhtNode, err := dht.NewDHTNode(h, cfg)
	if err != nil {
		t.Fatalf("NewDHTNode 失败: %v", err)
	}
	defer dhtNode.Close()

	if dhtNode == nil {
		t.Error("DHTNode 不应为 nil")
	}
}

// TestDHTNodeGetRouting 测试获取路由
func TestDHTNodeGetRouting(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	cfg := &config.Config{}
	dhtNode, err := dht.NewDHTNode(h, cfg)
	if err != nil {
		t.Fatalf("NewDHTNode 失败: %v", err)
	}
	defer dhtNode.Close()

	routing := dhtNode.GetRouting()
	if routing == nil {
		t.Error("GetRouting 不应返回 nil")
	}
}

// TestDHTNodeClose 测试关闭 DHT 节点
func TestDHTNodeClose(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	cfg := &config.Config{}
	dhtNode, err := dht.NewDHTNode(h, cfg)
	if err != nil {
		t.Fatalf("NewDHTNode 失败: %v", err)
	}

	// 关闭应该成功
	err = dhtNode.Close()
	if err != nil {
		t.Errorf("Close 失败: %v", err)
	}
}

// TestDHTNodeBootstrap 测试 Bootstrap
func TestDHTNodeBootstrap(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	cfg := &config.Config{}
	dhtNode, err := dht.NewDHTNode(h, cfg)
	if err != nil {
		t.Fatalf("NewDHTNode 失败: %v", err)
	}
	defer dhtNode.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Bootstrap 使用默认节点，可能因网络原因失败
	// 我们只验证方法可以调用
	_ = dhtNode.Bootstrap(ctx)
}

// TestDHTNodeBootstrapWithSeedNodes 测试带种子节点的 Bootstrap
func TestDHTNodeBootstrapWithSeedNodes(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	cfg := &config.Config{
		Network: config.NetworkConfig{
			SeedNodes: []string{
				// 无效的种子节点地址，用于测试解析逻辑
				"/ip4/127.0.0.1/tcp/12345/p2p/12D3KooWGtest",
			},
		},
	}

	dhtNode, err := dht.NewDHTNode(h, cfg)
	if err != nil {
		t.Fatalf("NewDHTNode 失败: %v", err)
	}
	defer dhtNode.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Bootstrap 应该能处理无效的种子节点
	_ = dhtNode.Bootstrap(ctx)
}
