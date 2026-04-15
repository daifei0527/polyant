// Package main Polyant 种子节点入口
// 种子节点是为 AI 代理提供的公共 P2P 知识网络基础设施
// 要求：公网 IP、域名、TLS 证书
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
	"github.com/daifei0527/polyant/internal/core/user"
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
	configFile = flag.String("config", "", "Configuration file path (JSON)")
	domain     = flag.String("domain", "", "Domain name (required)")
	tlsCert    = flag.String("tls-cert", "", "TLS certificate path")
	tlsKey     = flag.String("tls-key", "", "TLS key path")
	p2pPort    = flag.Int("p2p-port", 9000, "P2P listen port")
	apiPort    = flag.Int("api-port", 8080, "API service port")
	dataDir    = flag.String("data-dir", "./data/seed", "Data directory")
	showVersion = flag.Bool("version", false, "Show version info")
)

const (
	Version = "1.0.0"
)

// SeedApp 种子节点应用
type SeedApp struct {
	config       *config.Config
	logger       *zap.Logger
	store        *storage.Store
	p2pHost      *host.P2PHost
	dhtNode      *dht.DHTNode
	syncEngine   *sync.SyncEngine
	pushService  *sync.PushService
	httpServer   *http.Server
	levelChecker *user.LevelUpgradeChecker
	cancel       context.CancelFunc
	tlsCertPath  string
	tlsKeyPath   string
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("Polyant Seed Node version %s\n", Version)
		os.Exit(0)
	}

	// 验证必需参数
	if *domain == "" {
		log.Fatal("--domain is required for seed node")
	}
	if *tlsCert == "" {
		log.Fatal("--tls-cert is required for seed node")
	}
	if *tlsKey == "" {
		log.Fatal("--tls-key is required for seed node")
	}

	// 加载配置
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 强制设置节点类型为 seed
	cfg.Node.Type = "seed"

	// 设置种子节点专用配置
	cfg.Seed.Domain = *domain
	cfg.Seed.TLSCert = *tlsCert
	cfg.Seed.TLSKey = *tlsKey

	// 验证种子节点配置
	if err := cfg.Seed.Validate(); err != nil {
		log.Fatalf("Invalid seed config: %v", err)
	}

	// 创建并运行应用
	app, err := NewSeedApp(cfg, *tlsCert, *tlsKey)
	if err != nil {
		log.Fatalf("Failed to initialize seed node: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Seed node failed: %v", err)
	}
}

// loadConfig 加载配置
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

	// 应用命令行参数覆盖
	cfg.Network.ListenPort = *p2pPort
	cfg.Network.APIPort = *apiPort
	cfg.Node.DataDir = *dataDir

	// 加载环境变量
	cfg = config.LoadWithEnv(cfg)

	// 初始化 i18n
	localesDir := cfg.Node.DataDir + "/locales"
	if localesDir == "/locales" {
		localesDir = "./pkg/i18n/locales"
	}
	if err := i18n.Init(localesDir, i18n.Lang(cfg.I18n.DefaultLang)); err != nil {
		// 使用默认路径重试
		if err := i18n.Init("./pkg/i18n/locales", i18n.Lang(cfg.I18n.DefaultLang)); err != nil {
			log.Printf("Warning: i18n initialization failed: %v", err)
		}
	}

	return cfg, nil
}

// NewSeedApp 创建种子节点应用实例
func NewSeedApp(cfg *config.Config, tlsCertPath, tlsKeyPath string) (*SeedApp, error) {
	// 初始化日志
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Starting Polyant Seed Node",
		zap.String("domain", cfg.Seed.Domain),
		zap.String("data_dir", cfg.Node.DataDir),
		zap.Int("p2p_port", cfg.Network.ListenPort),
		zap.Int("api_port", cfg.Network.APIPort),
	)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	app := &SeedApp{
		config:      cfg,
		logger:      logger,
		cancel:      cancel,
		tlsCertPath: tlsCertPath,
		tlsKeyPath:  tlsKeyPath,
	}

	// 初始化存储层
	if err := app.initStorage(ctx); err != nil {
		app.cleanup()
		return nil, err
	}

	// 初始化分类和种子数据
	if err := app.initData(ctx); err != nil {
		app.cleanup()
		return nil, err
	}

	return app, nil
}

// initStorage 初始化存储层
func (app *SeedApp) initStorage(ctx context.Context) error {
	dataDir := app.config.Node.DataDir
	if dataDir == "" {
		dataDir = "./data/seed"
	}

	// 构建存储配置
	storeCfg := &storage.StoreConfig{
		KVType:     app.config.Storage.KVType,
		KVPath:     dataDir + "/kv",
		SearchType: app.config.Storage.SearchType,
		SearchPath: dataDir + "/search.bleve",
	}

	// 如果未配置存储类型，使用默认值
	if storeCfg.KVType == "" {
		storeCfg.KVType = "pebble"
	}
	if storeCfg.SearchType == "" {
		storeCfg.SearchType = "bleve"
	}

	var err error
	app.store, err = storage.NewPersistentStore(storeCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	app.logger.Info("Storage initialized",
		zap.String("kv_type", storeCfg.KVType),
		zap.String("kv_path", storeCfg.KVPath),
		zap.String("search_type", storeCfg.SearchType),
		zap.String("search_path", storeCfg.SearchPath),
	)

	return nil
}

// initData 初始化数据
func (app *SeedApp) initData(ctx context.Context) error {
	// 初始化分类
	categoryInit := category.NewCategoryInitializer(app.store.Category, app.config.Node.DataDir)
	if err := categoryInit.Initialize(ctx); err != nil {
		app.logger.Warn("Category initialization warning", zap.Error(err))
	}

	// 初始化种子数据
	seedInit := seed.NewSeedDataInitializer(app.store, app.config.Node.DataDir+"/seed-data")
	if err := seedInit.Initialize(ctx); err != nil {
		app.logger.Warn("Seed data initialization warning", zap.Error(err))
	}

	return nil
}

// Start 启动服务
func (app *SeedApp) Start() error {
	ctx := context.Background()

	// 启动用户层级升级检查器
	app.levelChecker = user.NewLevelUpgradeChecker(app.store, time.Hour)
	if err := app.levelChecker.Start(ctx); err != nil {
		app.logger.Warn("User level checker start failed", zap.Error(err))
	}

	// 构建 P2P 监听地址
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", app.config.Network.ListenPort)

	// 创建 P2P Host 配置 - 种子节点启用中继服务
	hostCfg := &host.HostConfig{
		ListenAddrs:    []string{listenAddr},
		SeedPeers:      app.config.Seed.BootstrapPeers,
		EnableDHT:      true, // 种子节点必须启用 DHT
		EnableMDNS:     false, // 种子节点不需要 mDNS
		EnableNAT:      false, // 种子节点有公网 IP，不需要 NAT 映射
		EnableRelay:    true,
		EnableAutoRelay: false, // 种子节点不需要自动中继
		RelayService:   true,  // 种子节点提供中继服务
		EnableWebSocket: true, // 启用 WebSocket 传输
		EnableQUIC:     true,
		EnableHolePunching: false, // 种子节点不需要打洞
		PrivateKey:     nil,
	}

	// 创建 P2P Host
	var err error
	app.p2pHost, err = host.NewHost(ctx, hostCfg)
	if err != nil {
		return fmt.Errorf("failed to create P2P host: %w", err)
	}

	// 设置节点类型为 seed
	app.p2pHost.SetNodeType("seed")

	app.logger.Info("P2P node started successfully",
		zap.String("node_id", app.p2pHost.NodeID()),
		zap.String("node_type", app.p2pHost.NodeType()),
		zap.Bool("relay_service", app.p2pHost.IsRelayServer()),
	)

	// 初始化 DHT - 种子节点作为 DHT 服务器节点
	app.dhtNode, err = dht.NewDHTNode(app.p2pHost.Host, app.config)
	if err != nil {
		return fmt.Errorf("failed to initialize DHT: %w", err)
	}

	if err := app.dhtNode.Bootstrap(ctx); err != nil {
		app.logger.Warn("DHT bootstrap warning", zap.Error(err))
	}
	app.logger.Info("DHT routing initialized")

	// 创建同步引擎 - 种子节点支持全量镜像
	syncCfg := &sync.SyncConfig{
		AutoSync:         app.config.Sync.AutoSync,
		IntervalSeconds:  app.config.Sync.IntervalSeconds,
		MirrorCategories: []string{"*"}, // 种子节点镜像所有分类
		MaxLocalSizeMB:   0,             // 种子节点无大小限制
		BatchSize:        100,
	}
	app.syncEngine = sync.NewSyncEngine(app.p2pHost, nil, app.store, syncCfg)

	// 创建协议处理器
	proto := protocol.NewProtocol(app.p2pHost.Host, app.syncEngine)
	app.syncEngine.SetProtocol(proto)
	app.logger.Info("AWSP protocol layer initialized")

	// 创建远程查询服务
	remoteQueryService := sync.NewRemoteQueryService(app.p2pHost, proto, app.store, nil)
	remoteQueryService.SetProtocol(proto)
	app.logger.Info("Remote query service initialized")

	// 创建推送服务
	app.pushService = sync.NewPushService(app.p2pHost, nil)
	app.pushService.SetProtocol(proto)
	if err := app.pushService.Start(ctx); err != nil {
		app.logger.Warn("Push service start failed", zap.Error(err))
	} else {
		app.logger.Info("Push service started")
	}

	// 启动同步引擎
	if err := app.syncEngine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start sync engine: %w", err)
	}
	app.logger.Info("Sync engine started")

	// 连接到 bootstrap peers
	if len(app.config.Seed.BootstrapPeers) > 0 {
		for _, peerAddr := range app.config.Seed.BootstrapPeers {
			if err := app.p2pHost.ConnectToPeer(ctx, peerAddr); err != nil {
				app.logger.Warn("Failed to connect to bootstrap peer",
					zap.String("addr", peerAddr),
					zap.Error(err),
				)
			} else {
				app.logger.Info("Connected to bootstrap peer", zap.String("addr", peerAddr))
			}
		}
	}

	// 创建 API 路由
	apiHandler, err := router.NewRouterWithDeps(&router.Dependencies{
		Store:         app.store,
		EntryStore:    app.store.Entry,
		UserStore:     app.store.User,
		RatingStore:   app.store.Rating,
		CategoryStore: app.store.Category,
		SearchEngine:  app.store.Search,
		Backlink:      app.store.Backlink,
		RemoteQuerier: remoteQueryService,
		EntryPusher:   app.pushService,
		KVStore:       app.store.KVStore(),
		NodeID:        app.p2pHost.NodeID(),
		NodeType:      "seed",
		Version:       Version,
	})
	if err != nil {
		return fmt.Errorf("failed to create API router: %w", err)
	}

	// 启动 HTTPS 服务器
	httpAddr := fmt.Sprintf(":%d", app.config.Network.APIPort)
	app.httpServer = &http.Server{
		Addr:         httpAddr,
		Handler:      apiHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	go func() {
		app.logger.Info("HTTPS API server starting", zap.String("addr", httpAddr))
		if err := app.httpServer.ListenAndServeTLS(app.tlsCertPath, app.tlsKeyPath); err != http.ErrServerClosed {
			app.logger.Fatal("HTTPS server failed", zap.Error(err))
		}
	}()

	return nil
}

// Run 运行应用（阻塞）
func (app *SeedApp) Run() error {
	// 启动服务
	if err := app.Start(); err != nil {
		return err
	}

	app.logger.Info("Polyant Seed Node is running",
		zap.String("domain", app.config.Seed.Domain),
		zap.String("node_id", app.p2pHost.NodeID()),
	)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	app.logger.Info("Shutdown signal received, starting graceful shutdown...")

	// 停止服务
	return app.Stop()
}

// Stop 停止服务
func (app *SeedApp) Stop() error {
	app.logger.Info("Stopping services...")

	// 关闭 HTTP 服务器
	if app.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.httpServer.Shutdown(ctx); err != nil {
			app.logger.Warn("HTTP server shutdown timeout", zap.Error(err))
		}
	}

	// 停止各组件
	if app.pushService != nil {
		app.pushService.Stop()
	}
	if app.syncEngine != nil {
		app.syncEngine.Stop()
	}
	if app.levelChecker != nil {
		app.levelChecker.Stop()
	}
	if app.dhtNode != nil {
		if err := app.dhtNode.Close(); err != nil {
			app.logger.Warn("DHT close failed", zap.Error(err))
		}
	}
	if app.p2pHost != nil {
		app.p2pHost.Close()
	}

	// 清理资源
	app.cleanup()

	app.logger.Info("Polyant Seed Node stopped")
	return nil
}

// cleanup 清理资源
func (app *SeedApp) cleanup() {
	if app.cancel != nil {
		app.cancel()
	}
	if app.store != nil {
		if err := app.store.Close(); err != nil {
			app.logger.Warn("Storage close failed", zap.Error(err))
		}
	}
}
