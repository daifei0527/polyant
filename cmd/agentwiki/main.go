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
	"github.com/daifei0527/agentwiki/internal/service/daemon"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/pkg/config"
	"go.uber.org/zap"
)

var (
	configFile  = flag.String("config", "", "配置文件路径 (JSON)")
	showVersion = flag.Bool("version", false, "显示版本信息")
	initSeed    = flag.Bool("init-seed", false, "初始化种子数据并退出")
	useMemoryDB = flag.Bool("memory", false, "使用内存存储（仅用于测试）")
	runAsService = flag.Bool("service", false, "作为系统服务运行")
)

const (
	Version = "1.0.0"
)

// AgentWiki 应用主结构
type AgentWiki struct {
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
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("AgentWiki version %s\n", Version)
		os.Exit(0)
	}

	// 加载配置
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 检查是否作为系统服务运行
	if len(os.Args) > 1 && isServiceCommand(os.Args[1]) {
		runServiceCommand(cfg)
		return
	}

	if *runAsService {
		runAsSystemService(cfg)
		return
	}

	// 直接运行
	app, err := NewAgentWiki(cfg)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("运行失败: %v", err)
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

	cfg = config.LoadWithEnv(cfg)
	if err := config.Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// isServiceCommand 检查是否为服务管理命令
func isServiceCommand(cmd string) bool {
	serviceCmds := []string{"install", "uninstall", "start", "stop", "restart", "status"}
	for _, c := range serviceCmds {
		if cmd == c {
			return true
		}
	}
	return false
}

// runServiceCommand 执行服务管理命令
func runServiceCommand(cfg *config.Config) {
	d, err := daemon.NewDaemon(cfg, nil, nil)
	if err != nil {
		log.Fatalf("创建服务失败: %v", err)
	}

	cmd := os.Args[1]
	switch cmd {
	case "install":
		err = d.Install()
	case "uninstall":
		_ = d.Stop()
		err = d.Uninstall()
	case "start":
		err = d.Start()
	case "stop":
		err = d.Stop()
	case "restart":
		_ = d.Stop()
		err = d.Start()
	case "status":
		status, serr := d.Status()
		if serr != nil {
			log.Fatalf("获取状态失败: %v", serr)
		}
		fmt.Printf("服务状态: %s\n", status)
		return
	}

	if err != nil {
		log.Fatalf("执行命令失败: %v", err)
	}
	fmt.Printf("✓ 命令 %s 执行成功\n", cmd)
}

// runAsSystemService 作为系统服务运行
func runAsSystemService(cfg *config.Config) {
	var app *AgentWiki

	startFn := func() error {
		var err error
		app, err = NewAgentWiki(cfg)
		if err != nil {
			return err
		}
		return app.Start()
	}

	stopFn := func() error {
		if app != nil {
			return app.Stop()
		}
		return nil
	}

	if err := daemon.RunAsService(cfg, startFn, stopFn); err != nil {
		log.Fatalf("服务运行失败: %v", err)
	}
}

// NewAgentWiki 创建应用实例
func NewAgentWiki(cfg *config.Config) (*AgentWiki, error) {
	// 初始化日志
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	logger.Info("启动 AgentWiki 节点",
		zap.String("name", cfg.Node.Name),
		zap.String("type", cfg.Node.Type),
	)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	app := &AgentWiki{
		config: cfg,
		logger: logger,
		cancel: cancel,
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
func (app *AgentWiki) initStorage(ctx context.Context) error {
	var err error

	if *useMemoryDB {
		app.store, err = storage.NewMemoryStore()
		if err != nil {
			return fmt.Errorf("初始化内存存储失败: %w", err)
		}
		app.logger.Info("使用内存存储")
	} else {
		dataDir := app.config.Node.DataDir
		if dataDir == "" {
			dataDir = "./data"
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

		app.store, err = storage.NewPersistentStore(storeCfg)
		if err != nil {
			return fmt.Errorf("初始化存储失败: %w", err)
		}
		app.logger.Info("存储层初始化完成",
			zap.String("kv_type", storeCfg.KVType),
			zap.String("kv_path", storeCfg.KVPath),
			zap.String("search_type", storeCfg.SearchType),
			zap.String("search_path", storeCfg.SearchPath),
		)
	}

	return nil
}

// initData 初始化数据
func (app *AgentWiki) initData(ctx context.Context) error {
	// 初始化分类
	categoryInit := category.NewCategoryInitializer(app.store.Category, app.config.Node.DataDir)
	if err := categoryInit.Initialize(ctx); err != nil {
		app.logger.Warn("分类初始化警告", zap.Error(err))
	}

	// 初始化种子数据
	if app.config.Node.Type == "seed" || *initSeed {
		seedInit := seed.NewSeedDataInitializer(app.store, app.config.Node.DataDir+"/seed-data")
		if err := seedInit.Initialize(ctx); err != nil {
			app.logger.Warn("种子数据初始化警告", zap.Error(err))
		}
	}

	return nil
}

// Start 启动服务
func (app *AgentWiki) Start() error {
	ctx := context.Background()

	// 启动用户层级升级检查器
	app.levelChecker = user.NewLevelUpgradeChecker(app.store, time.Hour)
	if err := app.levelChecker.Start(ctx); err != nil {
		app.logger.Warn("用户层级检查器启动失败", zap.Error(err))
	}

	// 构建 P2P 监听地址
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", app.config.Network.ListenPort)

	// 创建 P2P Host 配置
	hostCfg := &host.HostConfig{
		ListenAddrs: []string{listenAddr},
		SeedPeers:   app.config.Network.SeedNodes,
		EnableDHT:   app.config.Network.DHTEnabled,
		EnableMDNS:  app.config.Network.MDNSEnabled,
		EnableNAT:   true,
		EnableRelay: true,
		PrivateKey:  nil,
	}

	// 创建 P2P Host
	var err error
	app.p2pHost, err = host.NewHost(ctx, hostCfg)
	if err != nil {
		return fmt.Errorf("创建 P2P Host 失败: %w", err)
	}

	app.logger.Info("P2P 节点启动成功",
		zap.String("node_id", app.p2pHost.NodeID()),
		zap.String("node_type", app.p2pHost.NodeType()),
	)

	// 初始化 DHT
	if app.config.Network.DHTEnabled {
		app.dhtNode, err = dht.NewDHTNode(app.p2pHost.Host, app.config)
		if err != nil {
			return fmt.Errorf("初始化 DHT 失败: %w", err)
		}

		if err := app.dhtNode.Bootstrap(ctx); err != nil {
			app.logger.Warn("DHT bootstrap 警告", zap.Error(err))
		}
		app.logger.Info("DHT 路由初始化完成")
	}

	// 创建同步引擎
	syncCfg := &sync.SyncConfig{
		AutoSync:         app.config.Sync.AutoSync,
		IntervalSeconds:  app.config.Sync.IntervalSeconds,
		MirrorCategories: app.config.Sync.MirrorCategories,
		MaxLocalSizeMB:   app.config.Sync.MaxLocalSizeMB,
		BatchSize:        100,
	}
	app.syncEngine = sync.NewSyncEngine(app.p2pHost, nil, app.store, syncCfg)

	// 创建协议处理器
	proto := protocol.NewProtocol(app.p2pHost.Host, app.syncEngine)
	app.syncEngine.SetProtocol(proto)
	app.logger.Info("AWSP 协议层初始化完成")

	// 创建远程查询服务
	remoteQueryService := sync.NewRemoteQueryService(app.p2pHost, proto, app.store, nil)
	remoteQueryService.SetProtocol(proto)
	app.logger.Info("远程查询服务初始化完成")

	// 创建推送服务
	app.pushService = sync.NewPushService(app.p2pHost, nil)
	app.pushService.SetProtocol(proto)
	if err := app.pushService.Start(ctx); err != nil {
		app.logger.Warn("推送服务启动失败", zap.Error(err))
	} else {
		app.logger.Info("推送服务启动完成")
	}

	// 启动同步引擎
	if err := app.syncEngine.Start(ctx); err != nil {
		return fmt.Errorf("启动同步引擎失败: %w", err)
	}
	app.logger.Info("同步引擎启动完成")

	// 连接到种子节点
	if len(app.config.Network.SeedNodes) > 0 {
		for _, seedAddr := range app.config.Network.SeedNodes {
			if err := app.p2pHost.ConnectToPeer(ctx, seedAddr); err != nil {
				app.logger.Warn("连接种子节点失败", zap.String("addr", seedAddr), zap.Error(err))
			} else {
				app.logger.Info("已连接种子节点", zap.String("addr", seedAddr))
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
		NodeType:      app.config.Node.Type,
		Version:       Version,
	})
	if err != nil {
		return fmt.Errorf("创建 API 路由失败: %w", err)
	}

	// 启动 HTTP 服务器
	httpAddr := fmt.Sprintf(":%d", app.config.Network.APIPort)
	app.httpServer = &http.Server{
		Addr:         httpAddr,
		Handler:      apiHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		app.logger.Info("HTTP API 服务器启动", zap.String("addr", httpAddr))
		if err := app.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			app.logger.Fatal("HTTP 服务器启动失败", zap.Error(err))
		}
	}()

	return nil
}

// Run 运行应用（阻塞）
func (app *AgentWiki) Run() error {
	// 启动服务
	if err := app.Start(); err != nil {
		return err
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	app.logger.Info("收到关闭信号，开始优雅关机...")

	// 停止服务
	return app.Stop()
}

// Stop 停止服务
func (app *AgentWiki) Stop() error {
	app.logger.Info("正在停止服务...")

	// 关闭 HTTP 服务器
	if app.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.httpServer.Shutdown(ctx); err != nil {
			app.logger.Warn("HTTP 服务器关闭超时", zap.Error(err))
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
	if app.p2pHost != nil {
		app.p2pHost.Close()
	}

	// 清理资源
	app.cleanup()

	app.logger.Info("AgentWiki 节点已关闭")
	return nil
}

// cleanup 清理资源
func (app *AgentWiki) cleanup() {
	if app.cancel != nil {
		app.cancel()
	}
	if app.store != nil {
		if err := app.store.Close(); err != nil {
			app.logger.Warn("关闭存储失败", zap.Error(err))
		}
	}
}
