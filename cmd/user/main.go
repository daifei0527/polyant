// Package main Polyant 用户节点入口
// 用户节点是为 AI 代理提供的轻量级客户端，支持灵活的网络环境
// 特性：自动网络检测、离线支持、可选服务模式
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/daifei0527/polyant/internal/api/router"
	"github.com/daifei0527/polyant/internal/core/category"
	"github.com/daifei0527/polyant/internal/network/detect"
	"github.com/daifei0527/polyant/internal/network/dht"
	"github.com/daifei0527/polyant/internal/network/host"
	"github.com/daifei0527/polyant/internal/network/protocol"
	networksync "github.com/daifei0527/polyant/internal/network/sync"
	"github.com/daifei0527/polyant/internal/storage"
	queuesync "github.com/daifei0527/polyant/internal/sync"
	"github.com/daifei0527/polyant/pkg/config"
	"github.com/daifei0527/polyant/pkg/i18n"
	"go.uber.org/zap"
)

var (
	configFile   = flag.String("config", "", "Configuration file path (JSON)")
	seedNodes    = flag.String("seed-nodes", "", "Seed node addresses (comma separated)")
	serviceMode  = flag.Bool("service", false, "Enable service mode (listen for inbound connections)")
	p2pPort      = flag.Int("p2p-port", 0, "P2P listen port (0=random, used in service mode)")
	apiPort      = flag.Int("api-port", 8080, "API service port")
	dataDir      = flag.String("data-dir", "./data/user", "Data directory")
	relayService = flag.Bool("relay", false, "Provide relay service (in service mode)")
	mirrorMode   = flag.Bool("mirror", false, "Provide data mirror (in service mode)")
	showVersion  = flag.Bool("version", false, "Show version info")
	autoDetect   = flag.Bool("auto-detect", true, "Auto detect network environment")
)

const (
	Version = "1.0.0"
)

// UserApp 用户节点应用
type UserApp struct {
	config         *config.Config
	logger         *zap.Logger
	store          *storage.Store
	p2pHost        *host.P2PHost
	dhtNode        *dht.DHTNode
	syncEngine     *networksync.SyncEngine
	pushService    *networksync.PushService
	syncQueue      *queuesync.SyncQueue
	httpServer     *http.Server
	cancel         context.CancelFunc
	serviceMode    bool
	networkCap     *detect.NetworkCapability
	seedNodeAddrs  []string
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("Polyant User Node version %s\n", Version)
		os.Exit(0)
	}

	// 加载配置
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 强制设置节点类型为 user
	cfg.Node.Type = "user"

	// 解析种子节点地址
	seedAddrs := parseSeedNodes(*seedNodes, cfg.Network.SeedNodes)

	// 自动检测网络环境
	var networkCap *detect.NetworkCapability
	if *autoDetect {
		networkCap = detect.DetectNetworkCapability()
		log.Printf("Network detection: PublicIP=%s, HasPublicIP=%v, RecommendedMode=%s",
			networkCap.PublicIP, networkCap.HasPublicIP, networkCap.RecommendedMode)

		// 如果检测到公网 IP 且未指定服务模式，建议启用
		if networkCap.CanBeReached && !*serviceMode {
			log.Printf("Public IP detected, consider using --service mode for better connectivity")
		}
	}

	// 确定是否启用服务模式
	if *autoDetect && networkCap != nil && networkCap.CanBeReached {
		// 自动建议服务模式（但不强制）
		if !*serviceMode {
			log.Printf("Tip: Your network supports service mode. Use --service for full P2P capabilities.")
		}
	}

	// 验证配置
	if err := config.Validate(cfg); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	// 创建并运行应用
	app, err := NewUserApp(cfg, seedAddrs, *serviceMode, networkCap)
	if err != nil {
		log.Fatalf("Failed to initialize user node: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("User node failed: %v", err)
	}
}

// parseSeedNodes 解析种子节点地址
func parseSeedNodes(flagNodes string, configNodes []string) []string {
	if flagNodes != "" {
		return strings.Split(flagNodes, ",")
	}
	return configNodes
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
	if *p2pPort > 0 {
		cfg.Network.ListenPort = *p2pPort
	}
	cfg.Network.APIPort = *apiPort
	cfg.Node.DataDir = *dataDir

	// 加载环境变量
	cfg = config.LoadWithEnv(cfg)

	// 初始化 i18n
	localesDir := cfg.Node.DataDir
	if localesDir == "" {
		localesDir = "./pkg/i18n/locales"
	} else {
		localesDir = localesDir + "/locales"
	}
	if err := i18n.Init(localesDir, i18n.Lang(cfg.I18n.DefaultLang)); err != nil {
		// 使用默认路径重试
		if err := i18n.Init("./pkg/i18n/locales", i18n.Lang(cfg.I18n.DefaultLang)); err != nil {
			log.Printf("Warning: i18n initialization failed: %v", err)
		}
	}

	return cfg, nil
}

// NewUserApp 创建用户节点应用实例
func NewUserApp(cfg *config.Config, seedAddrs []string, serviceMode bool, networkCap *detect.NetworkCapability) (*UserApp, error) {
	// 初始化日志
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Starting Polyant User Node",
		zap.String("data_dir", cfg.Node.DataDir),
		zap.Int("api_port", cfg.Network.APIPort),
		zap.Bool("service_mode", serviceMode),
	)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	app := &UserApp{
		config:        cfg,
		logger:        logger,
		cancel:        cancel,
		serviceMode:   serviceMode,
		networkCap:    networkCap,
		seedNodeAddrs: seedAddrs,
	}

	// 初始化存储层
	if err := app.initStorage(ctx); err != nil {
		app.cleanup()
		return nil, err
	}

	// 初始化分类
	if err := app.initData(ctx); err != nil {
		app.cleanup()
		return nil, err
	}

	// 初始化同步队列（用于离线支持）
	// Note: The sync queue is initialized here for future integration with the sync engine.
	// Currently, the offline mode can still be tracked via IsOffline() for logging/monitoring purposes.
	// Full integration into the sync engine flow requires modifications to the sync package.
	app.syncQueue = queuesync.NewSyncQueue()
	logger.Info("Sync queue initialized for offline support")

	return app, nil
}

// initStorage 初始化存储层
func (app *UserApp) initStorage(ctx context.Context) error {
	dataDir := app.config.Node.DataDir
	if dataDir == "" {
		dataDir = "./data/user"
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
func (app *UserApp) initData(ctx context.Context) error {
	// 初始化分类
	categoryInit := category.NewCategoryInitializer(app.store.Category, app.config.Node.DataDir)
	if err := categoryInit.Initialize(ctx); err != nil {
		app.logger.Warn("Category initialization warning", zap.Error(err))
	}

	return nil
}

// Start 启动服务
func (app *UserApp) Start() error {
	ctx := context.Background()

	// 创建 P2P Host 配置
	hostCfg := &host.HostConfig{
		SeedPeers:       app.seedNodeAddrs,
		EnableDHT:       true,
		EnableMDNS:      true, // 用户节点启用 mDNS 用于本地发现
		EnableNAT:       true,
		EnableRelay:     true,
		EnableAutoRelay: true, // 用户节点需要自动中继
		PrivateKey:      nil,
	}

	// 根据服务模式配置 P2P 监听
	if app.serviceMode {
		// 服务模式：监听 P2P 端口
		p2pListenPort := app.config.Network.ListenPort
		if p2pListenPort == 0 {
			p2pListenPort = 9000
		}
		listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", p2pListenPort)
		hostCfg.ListenAddrs = []string{listenAddr}
		hostCfg.EnableQUIC = true
		hostCfg.EnableWebSocket = true

		// 如果启用中继服务
		if *relayService {
			hostCfg.RelayService = true
		}

		app.logger.Info("Service mode enabled - listening for inbound connections",
			zap.Int("p2p_port", p2pListenPort),
			zap.Bool("relay", *relayService),
			zap.Bool("mirror", *mirrorMode),
		)
	} else {
		// 普通模式：不监听 P2P 端口，只作为客户端
		hostCfg.ListenAddrs = []string{} // 空监听地址
		hostCfg.EnableHolePunching = true

		app.logger.Info("Normal mode - client-only operation (no P2P listening)")
	}

	// 创建 P2P Host
	var err error
	app.p2pHost, err = host.NewHost(ctx, hostCfg)
	if err != nil {
		return fmt.Errorf("failed to create P2P host: %w", err)
	}

	// 设置节点类型为 user
	app.p2pHost.SetNodeType("user")

	app.logger.Info("P2P node started",
		zap.String("node_id", app.p2pHost.NodeID()),
		zap.String("node_type", app.p2pHost.NodeType()),
	)

	// 初始化 DHT（用于节点发现）
	app.dhtNode, err = dht.NewDHTNode(app.p2pHost.Host, app.config)
	if err != nil {
		app.logger.Warn("DHT initialization failed, continuing without DHT", zap.Error(err))
	} else {
		if err := app.dhtNode.Bootstrap(ctx); err != nil {
			app.logger.Warn("DHT bootstrap warning", zap.Error(err))
		}
		app.logger.Info("DHT routing initialized")
	}

	// 创建同步引擎配置
	syncCfg := &networksync.SyncConfig{
		AutoSync:         app.config.Sync.AutoSync,
		IntervalSeconds:  app.config.Sync.IntervalSeconds,
		MirrorCategories: []string{}, // 用户节点默认不镜像
		MaxLocalSizeMB:   100,        // 用户节点限制本地存储大小
		BatchSize:        50,
	}

	// 如果启用镜像模式
	if app.serviceMode && *mirrorMode {
		syncCfg.MirrorCategories = []string{"*"}
		syncCfg.MaxLocalSizeMB = 0 // 镜像模式无大小限制
		app.logger.Info("Mirror mode enabled - will mirror all categories")
	}

	app.syncEngine = networksync.NewSyncEngine(app.p2pHost, nil, app.store, syncCfg)

	// 创建协议处理器
	proto := protocol.NewProtocol(app.p2pHost.Host, app.syncEngine)
	app.syncEngine.SetProtocol(proto)
	app.logger.Info("AWSP protocol layer initialized")

	// 创建远程查询服务
	remoteQueryService := networksync.NewRemoteQueryService(app.p2pHost, proto, app.store, nil)
	remoteQueryService.SetProtocol(proto)
	app.logger.Info("Remote query service initialized")

	// 创建推送服务
	app.pushService = networksync.NewPushService(app.p2pHost, nil)
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

	// 连接到种子节点
	if len(app.seedNodeAddrs) > 0 {
		connected := 0
		for _, seedAddr := range app.seedNodeAddrs {
			if err := app.p2pHost.ConnectToPeer(ctx, seedAddr); err != nil {
				app.logger.Warn("Failed to connect to seed node",
					zap.String("addr", seedAddr),
					zap.Error(err),
				)
			} else {
				app.logger.Info("Connected to seed node", zap.String("addr", seedAddr))
				connected++
			}
		}
		if connected == 0 {
			app.logger.Warn("No seed nodes available - operating in offline mode")
			app.syncQueue.EnableOfflineMode()
		}
	} else {
		app.logger.Warn("No seed nodes configured - operating in standalone mode")
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
		NodeType:      "user",
		Version:       Version,
	})
	if err != nil {
		return fmt.Errorf("failed to create API router: %w", err)
	}

	// 启动 HTTP 服务器（用户节点通常使用 HTTP，无需 TLS）
	httpAddr := fmt.Sprintf(":%d", app.config.Network.APIPort)
	app.httpServer = &http.Server{
		Addr:         httpAddr,
		Handler:      apiHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		app.logger.Info("HTTP API server starting", zap.String("addr", httpAddr))
		if err := app.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			app.logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	return nil
}

// Run 运行应用（阻塞）
func (app *UserApp) Run() error {
	// 启动服务
	if err := app.Start(); err != nil {
		return err
	}

	app.logger.Info("Polyant User Node is running",
		zap.String("node_id", app.p2pHost.NodeID()),
		zap.Bool("service_mode", app.serviceMode),
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
func (app *UserApp) Stop() error {
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

	app.logger.Info("Polyant User Node stopped")
	return nil
}

// cleanup 清理资源
func (app *UserApp) cleanup() {
	if app.cancel != nil {
		app.cancel()
	}
	if app.store != nil {
		if err := app.store.Close(); err != nil {
			app.logger.Warn("Storage close failed", zap.Error(err))
		}
	}
}
