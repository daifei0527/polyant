// Package main AgentWiki 对等知识网络主入口
// AgentWiki 是一个基于 P2P 网络的分布式协同知识维基百科系统
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

	"github.com/daifei0527/agentwiki/internal/api/router"
	"github.com/daifei0527/agentwiki/internal/core/category"
	"github.com/daifei0527/agentwiki/internal/core/seed"
	"github.com/daifei0527/agentwiki/internal/core/user"
	"github.com/daifei0527/agentwiki/internal/network/dht"
	"github.com/daifei0527/agentwiki/internal/network/host"
	"github.com/daifei0527/agentwiki/internal/network/protocol"
	"github.com/daifei0527/agentwiki/internal/network/sync"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/pkg/config"
	"go.uber.org/zap"
)

var (
	configFile   = flag.String("config", "", "配置文件路径 (JSON)")
	showVersion  = flag.Bool("version", false, "显示版本信息")
	initSeed     = flag.Bool("init-seed", false, "初始化种子数据并退出")
	useMemoryDB  = flag.Bool("memory", false, "使用内存存储（仅用于测试）")
)

const (
	Version = "0.1.0-dev"
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("AgentWiki version %s\n", Version)
		os.Exit(0)
	}

	// 初始化日志
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logger.Sync()

	// 加载配置
	var cfg *config.Config
	if *configFile != "" {
		cfg, err = config.Load(*configFile)
		if err != nil {
			log.Fatalf("加载配置文件失败: %v", err)
		}
	} else {
		cfg = config.DefaultConfig()
		logger.Info("使用默认配置")
	}

	// 应用环境变量覆盖
	cfg = config.LoadWithEnv(cfg)

	// 验证配置
	if err := config.Validate(cfg); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	logger.Info("启动 AgentWiki 节点",
		zap.String("name", cfg.Node.Name),
		zap.String("type", cfg.Node.Type),
	)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化存储层
	var store *storage.Store
	if *useMemoryDB {
		// 使用内存存储（用于测试）
		store, err = storage.NewMemoryStore()
		if err != nil {
			logger.Fatal("初始化内存存储失败", zap.Error(err))
		}
		logger.Info("使用内存存储")
	} else {
		// 使用 BadgerDB 持久化存储
		dataDir := cfg.Node.DataDir
		if dataDir == "" {
			dataDir = "./data"
		}
		storeWrapper, err := storage.NewBadgerStoreWithCloser(dataDir + "/db")
		if err != nil {
			logger.Fatal("初始化存储失败", zap.Error(err))
		}
		defer func() {
			if err := storeWrapper.Close(); err != nil {
				logger.Warn("关闭存储失败", zap.Error(err))
			}
		}()
		store = &storeWrapper.Store
		logger.Info("存储层初始化完成", zap.String("path", dataDir+"/db"))
	}

	// 初始化分类
	categoryInit := category.NewCategoryInitializer(store.Category, cfg.Node.DataDir)
	if err := categoryInit.Initialize(ctx); err != nil {
		logger.Warn("分类初始化警告", zap.Error(err))
	}

	// 初始化种子数据（种子节点或明确要求）
	if cfg.Node.Type == "seed" || *initSeed {
		seedInit := seed.NewSeedDataInitializer(store, cfg.Node.DataDir+"/seed-data")
		if err := seedInit.Initialize(ctx); err != nil {
			logger.Warn("种子数据初始化警告", zap.Error(err))
		}
	}

	// 启动用户层级升级检查器
	levelChecker := user.NewLevelUpgradeChecker(store, time.Hour)
	if err := levelChecker.Start(ctx); err != nil {
		logger.Warn("用户层级检查器启动失败", zap.Error(err))
	}
	defer levelChecker.Stop()

	// 构建 P2P 监听地址
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.Network.ListenPort)

	// 创建 P2P Host 配置
	hostCfg := &host.HostConfig{
		ListenAddrs: []string{listenAddr},
		SeedPeers:   cfg.Network.SeedNodes,
		EnableDHT:   cfg.Network.DHTEnabled,
		EnableMDNS:  cfg.Network.MDNSEnabled,
		EnableNAT:   true,
		EnableRelay: true,
		PrivateKey:  nil, // 将自动生成
	}

	// 创建 P2P Host
	p2pHost, err := host.NewHost(ctx, hostCfg)
	if err != nil {
		logger.Fatal("创建 P2P Host 失败", zap.Error(err))
	}
	defer p2pHost.Close()

	// 输出节点信息
	logger.Info("P2P 节点启动成功",
		zap.String("node_id", p2pHost.NodeID()),
		zap.String("node_type", p2pHost.NodeType()),
	)
	for _, addr := range p2pHost.Addrs() {
		logger.Info("监听地址", zap.String("addr", fmt.Sprintf("%s/p2p/%s", addr, p2pHost.ID().String())))
	}

	// 初始化 DHT 路由发现
	var dhtNode *dht.DHTNode
	if cfg.Network.DHTEnabled {
		dhtNode, err = dht.NewDHTNode(p2pHost.Host, cfg)
		if err != nil {
			logger.Fatal("初始化 DHT 失败", zap.Error(err))
		}
		defer dhtNode.Close()

		if err := dhtNode.Bootstrap(ctx); err != nil {
			logger.Warn("DHT bootstrap 警告", zap.Error(err))
		}
		logger.Info("DHT 路由初始化完成")
	}

	// 创建同步配置
	syncCfg := &sync.SyncConfig{
		AutoSync:         cfg.Sync.AutoSync,
		IntervalSeconds:  cfg.Sync.IntervalSeconds,
		MirrorCategories: cfg.Sync.MirrorCategories,
		MaxLocalSizeMB:   cfg.Sync.MaxLocalSizeMB,
		BatchSize:        100,
	}

	// 创建同步引擎
	syncEngine := sync.NewSyncEngine(p2pHost, nil, store, syncCfg)

	// 创建协议处理器
	proto := protocol.NewProtocol(p2pHost.Host, syncEngine)
	syncEngine.SetProtocol(proto) // 解决循环依赖
	logger.Info("AWSP 协议层初始化完成")

	// 启动同步引擎
	if err := syncEngine.Start(ctx); err != nil {
		logger.Fatal("启动同步引擎失败", zap.Error(err))
	}
	defer syncEngine.Stop()
	logger.Info("同步引擎启动完成")

	// 连接到种子节点
	if len(cfg.Network.SeedNodes) > 0 {
		for _, seedAddr := range cfg.Network.SeedNodes {
			if err := p2pHost.ConnectToPeer(ctx, seedAddr); err != nil {
				logger.Warn("连接种子节点失败", zap.String("addr", seedAddr), zap.Error(err))
			} else {
				logger.Info("已连接种子节点", zap.String("addr", seedAddr))
			}
		}
	}

	// 创建 API 路由
	apiHandler, err := router.NewRouter(store, cfg)
	if err != nil {
		logger.Fatal("创建 API 路由失败", zap.Error(err))
	}

	// 构建 HTTP 监听地址
	httpAddr := fmt.Sprintf(":%d", cfg.Network.APIPort)

	// 启动 HTTP 服务器
	server := &http.Server{
		Addr:         httpAddr,
		Handler:      apiHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 优雅关机处理
	go func() {
		logger.Info("HTTP API 服务器启动", zap.String("addr", httpAddr))
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal("HTTP 服务器启动失败", zap.Error(err))
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("收到关闭信号，开始优雅关机...")

	// 关闭 HTTP 服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("HTTP 服务器关闭超时", zap.Error(err))
	}

	logger.Info("AgentWiki 节点已关闭")
}
