// Package router 定义了 Polyant API 的路由注册和中间件链。
// 使用标准库 net/http 实现，不依赖第三方路由库。
package router

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/daifei0527/polyant/internal/api/admin"
	"github.com/daifei0527/polyant/internal/api/handler"
	"github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/auth/rbac"
	coreadmin "github.com/daifei0527/polyant/internal/core/admin"
	"github.com/daifei0527/polyant/internal/core/email"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/daifei0527/polyant/pkg/config"
)

// RemoteQuerier 远程查询接口
type RemoteQuerier interface {
	// SearchWithRemote 执行搜索，本地结果不足时查询远程节点
	SearchWithRemote(ctx context.Context, query index.SearchQuery) (*index.SearchResult, error)
}

// Dependencies 路由依赖注入容器
// 包含所有 handler 需要的存储和引擎实例
type Dependencies struct {
	Store                     *storage.Store
	EntryStore                storage.EntryStore
	UserStore                 storage.UserStore
	RatingStore               storage.RatingStore
	CategoryStore             storage.CategoryStore
	SearchEngine              index.SearchEngine
	Backlink                  storage.BacklinkIndex
	EmailService              *email.Service
	VerificationMgr           *email.VerificationManager
	RemoteQuerier             RemoteQuerier       // 远程查询服务
	EntryPusher               handler.EntryPusher // 条目推送服务
	SyncTrigger               handler.SyncTrigger // /node/sync 增量同步触发器（可选）
	KVStore                   kv.Store            // KV 存储（选举等功能需要）
	SessionManager            *coreadmin.SessionManager
	NodeID                    string
	NodeType                  string
	Version                   string
	ApiKey                    string                // API 访问密钥
	DevReturnVerificationCode bool                  // dev/测试：发送验证码接口是否回传验证码（默认 false）
	CORSConfig                middleware.CORSConfig // 可选；为零值时使用 DefaultCORSConfig
	AdminListenAddr           string                // admin 本地访问校验用的监听地址（默认 127.0.0.1:18531）
	BodyLimitBytes            int64                 // R1-C2: 请求体大小上限（<=0 不限制）
	TrustedProxies            []string              // R1-D1: 受信反代 IP/CIDR
	BackupDir                 string                // R4c: 备份目录
	KVType                    string                // R4c: KV 引擎类型（pebble/badger）
}

// Router 包装已注册路由与中间件链，并暴露 Close 以便优雅停机时
// 释放后台资源（如认证中间件的重放保护清理 goroutine）。
type Router struct {
	handler http.Handler
	authMW  *middleware.AuthMiddleware
}

// ServeHTTP 让 *Router 满足 http.Handler（可直接作为 http.Server.Handler）。
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.handler.ServeHTTP(w, req)
}

// Close 释放路由持有的后台资源。应在 HTTP 服务器 Shutdown 之后调用。
func (r *Router) Close() {
	if r.authMW != nil {
		r.authMW.Close()
	}
}

// NewRouter 创建并配置 HTTP 路由
// 注册所有 API 端点，配置中间件链
// 中间件执行顺序: RequestID -> Logging -> Recovery -> CORS -> [ApiKey] -> [Auth] -> Handler
func NewRouter(store *storage.Store, cfg *config.Config) (*Router, error) {
	return NewRouterWithDeps(&Dependencies{
		Store:                     store,
		EntryStore:                store.Entry,
		UserStore:                 store.User,
		RatingStore:               store.Rating,
		CategoryStore:             store.Category,
		SearchEngine:              store.Search,
		Backlink:                  store.Backlink,
		KVStore:                   store.KVStore(),
		NodeID:                    "local-node-1",
		NodeType:                  cfg.Node.Type,
		Version:                   "v0.1.0-dev",
		ApiKey:                    cfg.Network.ApiKey,
		DevReturnVerificationCode: cfg.Dev.ReturnVerificationCode,
		CORSConfig:                CORSConfigFromConfig(cfg),
		AdminListenAddr:           cfg.Admin.Listen,
		BodyLimitBytes:            cfg.API.BodyLimitBytes,
		TrustedProxies:            cfg.API.TrustedProxies,
	})
}

// NewRouterWithDeps 使用依赖容器创建路由
func NewRouterWithDeps(deps *Dependencies) (*Router, error) {
	mux := http.NewServeMux()

	// 创建验证码管理器
	if deps.VerificationMgr == nil {
		deps.VerificationMgr = email.NewVerificationManager()
	}

	// 创建各 handler
	var titleIdx *index.TitleIndex
	if deps.Store != nil {
		titleIdx = deps.Store.TitleIdx
	}
	entryHandler := handler.NewEntryHandler(deps.EntryStore, deps.SearchEngine, deps.Backlink, deps.UserStore, titleIdx)

	// 设置远程查询服务
	if deps.RemoteQuerier != nil {
		entryHandler.SetRemoteQuerier(&remoteQuerierAdapter{deps.RemoteQuerier})
	}

	// 设置条目推送服务（新建/更新条目推送到种子节点）
	if deps.EntryPusher != nil {
		entryHandler.SetEntryPusher(deps.EntryPusher)
	}

	userHandler := handler.NewUserHandler(
		deps.Store,
		deps.UserStore,
		deps.EntryStore,
		deps.RatingStore,
		deps.EmailService,
		deps.VerificationMgr,
	)
	userHandler.SetDevReturnVerificationCode(deps.DevReturnVerificationCode)
	categoryHandler := handler.NewCategoryHandler(deps.CategoryStore, deps.EntryStore)
	nodeHandler := handler.NewNodeHandler(deps.NodeID, deps.NodeType, deps.Version, deps.EntryStore)
	if deps.SyncTrigger != nil {
		nodeHandler.SetSyncTrigger(deps.SyncTrigger)
	}

	// 创建管理员和选举 handler
	var adminHandler *handler.AdminHandler
	var electionHandler *handler.ElectionHandler
	if deps.Store != nil {
		adminHandler = handler.NewAdminHandler(deps.Store)
	}
	if deps.KVStore != nil {
		electionHandler = handler.NewElectionHandler(deps.KVStore)
	}

	// 创建批量操作 handler
	batchHandler := handler.NewBatchHandler(deps.EntryStore, deps.SearchEngine, deps.Backlink, deps.UserStore, titleIdx)

	// 创建审计 handler 和中间件
	var auditHandler *handler.AuditHandler
	var auditMW *middleware.AuditMiddleware
	if deps.Store != nil && deps.Store.Audit != nil {
		auditHandler = handler.NewAuditHandler(deps.Store.Audit)
		auditMW = middleware.NewAuditMiddleware(deps.Store.Audit)
	}

	// 创建导出/导入 handler
	var exportHandler *handler.ExportHandler
	if deps.Store != nil {
		exportHandler = handler.NewExportHandler(deps.Store, deps.NodeID)
	}

	// 创建统计 handler
	var statsHandler *handler.StatsHandler
	if deps.Store != nil {
		statsHandler = handler.NewStatsHandler(deps.Store)
	}

	// 创建认证中间件
	authMW := middleware.NewAuthMiddleware(deps.UserStore)

	// 创建 CORS 中间件（使用注入配置，缺省退化为安全的 DefaultCORSConfig）
	corsConf := deps.CORSConfig
	if len(corsConf.AllowedOrigins) == 0 && len(corsConf.AllowedMethods) == 0 {
		corsConf = middleware.DefaultCORSConfig()
	}
	corsMW := middleware.NewCORSMiddleware(corsConf)

	// 创建速率限制中间件
	rateLimitCfg := middleware.DefaultRateLimitConfig()
	rateLimitCfg.TrustedProxies = deps.TrustedProxies // R1-D1: 仅受信反代的 XFF 被采信
	rateLimitMW := middleware.NewRateLimitMiddleware(rateLimitCfg)

	// 创建 Session Manager（有 KV 后端时持久化，重启不丢 admin session）
	var sessionMgr *coreadmin.SessionManager
	if deps.SessionManager != nil {
		sessionMgr = deps.SessionManager
	} else if deps.KVStore != nil {
		sessionMgr = coreadmin.NewSessionManagerWithStore(24*time.Hour, deps.KVStore)
	} else {
		sessionMgr = coreadmin.NewSessionManager(24 * time.Hour)
	}

	// 注册公开路由（通过 ApiKeyMiddleware 保护）
	registerPublicRoutes(mux, deps.ApiKey, entryHandler, userHandler, categoryHandler, nodeHandler, electionHandler)

	// 注册认证路由（需要 Ed25519 签名认证）
	registerAuthRoutes(mux, authMW, entryHandler, userHandler, categoryHandler, nodeHandler, adminHandler, electionHandler, batchHandler, exportHandler, auditHandler, statsHandler)

	// 注册 Admin 路由（Session Token 认证）
	registerAdminRoutes(mux, deps, sessionMgr)

	// 应用中间件链
	var httpHandler http.Handler = mux
	// R1-C2: 请求体大小限制（最内层，包住 mux 使所有路由生效）
	if deps.BodyLimitBytes > 0 {
		httpHandler = middleware.BodyLimitMiddleware(deps.BodyLimitBytes)(httpHandler)
	}
	if auditMW != nil {
		httpHandler = auditMW.Middleware(httpHandler)
	}
	httpHandler = corsMW.Middleware(httpHandler)              // CORS
	httpHandler = rateLimitMW.Middleware(httpHandler)         // 速率限制
	httpHandler = middleware.RecoveryMiddleware(httpHandler)  // 异常恢复
	httpHandler = middleware.LoggingMiddleware(httpHandler)   // 请求日志
	httpHandler = middleware.RequestIDMiddleware(httpHandler) // 请求ID

	return &Router{
		handler: httpHandler,
		authMW:  authMW,
	}, nil
}

// remoteQuerierAdapter 适配 RemoteQuerier 到 handler.RemoteQuerier 接口
type remoteQuerierAdapter struct {
	querier RemoteQuerier
}

func (a *remoteQuerierAdapter) SearchWithRemote(ctx context.Context, query index.SearchQuery) (*index.SearchResult, error) {
	return a.querier.SearchWithRemote(ctx, query)
}

// CORSConfigFromConfig 根据应用配置构建 CORS 中间件配置。
// 未配置 origins 时退化为安全的通配符默认值（credentials=false）。
func CORSConfigFromConfig(cfg *config.Config) middleware.CORSConfig {
	c := middleware.DefaultCORSConfig()
	if cfg != nil && len(cfg.API.CORSAllowOrigins) > 0 {
		c.AllowedOrigins = cfg.API.CORSAllowOrigins
	}
	if cfg != nil && cfg.API.CORSAllowCredentials {
		c.AllowCredentials = true
	}
	return c
}

// registerPublicRoutes 注册公开路由（通过 ApiKeyMiddleware 保护）
// 这些路由无需 Ed25519 签名认证，但需要在请求头中携带 X-Polyant-Api-Key
func registerPublicRoutes(mux *http.ServeMux, apiKey string, eh *handler.EntryHandler, uh *handler.UserHandler, ch *handler.CategoryHandler, nh *handler.NodeHandler, elh *handler.ElectionHandler) {
	// 创建 API Key 中间件
	apiKeyMW := middleware.ApiKeyMiddleware(apiKey)

	// wrap 将 HandlerFunc 包装为带 ApiKey 验证的 Handler
	wrap := func(h http.HandlerFunc) http.Handler {
		return apiKeyMW(h)
	}

	// 搜索知识条目
	mux.Handle("/api/v1/search", wrap(eh.SearchHandler))

	// 获取条目详情
	mux.Handle("/api/v1/entry/", wrap(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// 检查是否是反向链接请求: /api/v1/entry/{id}/backlinks
		if strings.HasSuffix(path, "/backlinks") {
			if r.Method == http.MethodGet {
				eh.GetBacklinksHandler(w, r)
			} else {
				http.NotFound(w, r)
			}
			return
		}
		// 检查是否是正向链接请求: /api/v1/entry/{id}/outlinks
		if strings.HasSuffix(path, "/outlinks") {
			if r.Method == http.MethodGet {
				eh.GetOutlinksHandler(w, r)
			} else {
				http.NotFound(w, r)
			}
			return
		}
		// 默认：获取条目详情
		if r.Method == http.MethodGet {
			eh.GetEntryHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))

	// 获取分类列表
	mux.Handle("/api/v1/categories", wrap(func(w http.ResponseWriter, r *http.Request) {
		// 仅处理 GET 请求为公开路由
		if r.Method == http.MethodGet {
			ch.ListCategoriesHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))

	// 获取分类下的条目
	mux.Handle("/api/v1/categories/", wrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/entries") {
			ch.GetCategoryEntriesHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))

	// 获取节点状态
	mux.Handle("/api/v1/node/status", wrap(nh.GetNodeStatusHandler))

	// 用户注册
	mux.Handle("/api/v1/user/register", wrap(uh.RegisterHandler))

	// 选举公开路由（列出选举、获取选举详情）
	if elh != nil {
		// 列出选举 GET /api/v1/elections?status=active
		mux.Handle("/api/v1/elections", wrap(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				elh.ListElectionsHandler(w, r)
			} else {
				http.NotFound(w, r)
			}
		}))

		// 获取选举详情 GET /api/v1/elections/{id}
		mux.Handle("/api/v1/elections/", wrap(func(w http.ResponseWriter, r *http.Request) {
			// 检查是否是子资源请求
			path := r.URL.Path
			if strings.Contains(path, "/candidates") || strings.Contains(path, "/vote") || strings.Contains(path, "/close") {
				// 这些需要认证，交给认证路由处理
				http.NotFound(w, r)
				return
			}
			if r.Method == http.MethodGet {
				elh.GetElectionHandler(w, r)
			} else {
				http.NotFound(w, r)
			}
		}))
	}
}

// registerAuthRoutes 注册需要认证的路由
func registerAuthRoutes(mux *http.ServeMux, authMW *middleware.AuthMiddleware, eh *handler.EntryHandler, uh *handler.UserHandler, ch *handler.CategoryHandler, nh *handler.NodeHandler, ah *handler.AdminHandler, elh *handler.ElectionHandler, bh *handler.BatchHandler, exh *handler.ExportHandler, auh *handler.AuditHandler, sh *handler.StatsHandler) {
	// 创建条目（POST /api/v1/entry）
	mux.Handle("/api/v1/entry/create", authMW.Middleware(http.HandlerFunc(eh.CreateEntryHandler)))

	// 更新条目（PUT /api/v1/entry/{id}）
	mux.Handle("/api/v1/entry/update/", authMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eh.UpdateEntryHandler(w, r)
	})))

	// 删除条目（DELETE /api/v1/entry/{id}）
	mux.Handle("/api/v1/entry/delete/", authMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eh.DeleteEntryHandler(w, r)
	})))

	// 评分条目（POST /api/v1/entry/{id}/rate）
	mux.Handle("/api/v1/entry/rate/", authMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uh.RateEntryHandler(w, r)
	})))

	// 发送邮箱验证码（POST /api/v1/user/send-verification）
	mux.Handle("/api/v1/user/send-verification", authMW.Middleware(http.HandlerFunc(uh.SendVerificationCodeHandler)))

	// 邮箱验证（POST /api/v1/user/verify-email）- 需要认证
	mux.Handle("/api/v1/user/verify-email", authMW.Middleware(http.HandlerFunc(uh.VerifyEmailHandler)))

	// 获取用户信息（GET /api/v1/user/info）
	mux.Handle("/api/v1/user/info", authMW.Middleware(http.HandlerFunc(uh.GetUserInfoHandler)))

	// 更新用户信息（PUT /api/v1/user/info）
	mux.Handle("/api/v1/user/update", authMW.Middleware(http.HandlerFunc(uh.UpdateUserInfoHandler)))

	// 创建分类（POST /api/v1/categories）- 需要 Lv2+ 权限
	mux.Handle("/api/v1/categories/create", authMW.Middleware(http.HandlerFunc(ch.CreateCategoryHandler)))

	// 触发同步（POST /api/v1/node/sync）
	mux.Handle("/api/v1/node/sync", authMW.Middleware(http.HandlerFunc(nh.TriggerSyncHandler)))

	// ==================== 审计日志路由 ====================
	if auh != nil {
		// 查询审计日志 GET /api/v1/admin/audit/logs - Lv5 (SuperAdmin)
		mux.Handle("/api/v1/admin/audit/logs", authMW.Middleware(authMW.RequirePermission(rbac.PermAdmin, http.HandlerFunc(auh.ListAuditLogsHandler))))

		// 获取审计统计 GET /api/v1/admin/audit/stats - Lv5 (SuperAdmin)
		mux.Handle("/api/v1/admin/audit/stats", authMW.Middleware(authMW.RequirePermission(rbac.PermAdmin, http.HandlerFunc(auh.GetAuditStatsHandler))))

		// 删除审计日志 DELETE /api/v1/admin/audit/logs - Lv5 (SuperAdmin)
		mux.Handle("/api/v1/admin/audit/logs/delete", authMW.Middleware(authMW.RequirePermission(rbac.PermAdmin, http.HandlerFunc(auh.DeleteAuditLogsHandler))))
	}

	// ==================== 数据导出/导入路由 ====================
	if exh != nil {
		// 数据导出 GET /api/v1/admin/export - Lv4+ (Admin)
		mux.Handle("/api/v1/admin/export", authMW.Middleware(authMW.RequirePermission(rbac.PermManageUser, http.HandlerFunc(exh.ExportHandler))))

		// 数据导入 POST /api/v1/admin/import - Lv4+ (Admin)
		mux.Handle("/api/v1/admin/import", authMW.Middleware(authMW.RequirePermission(rbac.PermManageUser, http.HandlerFunc(exh.ImportHandler))))
	}

	// ==================== 管理员账户路由 ====================
	if ah != nil {
		// 设置/重置 Web admin 登录密码 POST /api/v1/admin/user/password - Lv4+ (ManageUser)
		mux.Handle("/api/v1/admin/user/password", authMW.Middleware(authMW.RequirePermission(rbac.PermManageUser, http.HandlerFunc(ah.SetPasswordHandler))))
	}

	// ==================== 选举路由 ====================
	if elh != nil {
		// 创建选举 POST /api/v1/elections - Lv5 (SuperAdmin)
		mux.Handle("/api/v1/elections/create", authMW.Middleware(authMW.RequirePermission(rbac.PermAdmin, http.HandlerFunc(elh.CreateElectionHandler))))

		// 提名候选人 POST /api/v1/elections/{id}/candidates - 已认证用户
		mux.Handle("/api/v1/elections/candidates/", authMW.Middleware(http.HandlerFunc(elh.NominateCandidateHandler)))

		// 确认接受提名 POST /api/v1/elections/{id}/candidates/{user_id}/confirm - 被提名人自己
		mux.Handle("/api/v1/elections/candidates/confirm/", authMW.Middleware(http.HandlerFunc(elh.ConfirmNominationHandler)))

		// 投票 POST /api/v1/elections/{id}/vote - Lv3+
		mux.Handle("/api/v1/elections/vote/", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv3, http.HandlerFunc(elh.VoteHandler))))

		// 关闭选举 POST /api/v1/elections/{id}/close - Lv5 (SuperAdmin)
		mux.Handle("/api/v1/elections/close/", authMW.Middleware(authMW.RequirePermission(rbac.PermAdmin, http.HandlerFunc(elh.CloseElectionHandler))))
	}

	// ==================== 批量操作路由 ====================
	if bh != nil {
		// 批量操作 POST/PUT/DELETE /api/v1/entries/batch
		mux.Handle("/api/v1/entries/batch", authMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				bh.BatchCreateHandler(w, r)
			case http.MethodPut:
				bh.BatchUpdateHandler(w, r)
			case http.MethodDelete:
				bh.BatchDeleteHandler(w, r)
			default:
				http.NotFound(w, r)
			}
		})))
	}
}

// registerAdminRoutes 注册 Admin API 路由
// 使用 Session Token 认证，独立于 Ed25519 签名认证
func registerAdminRoutes(mux *http.ServeMux, deps *Dependencies, sessionMgr *coreadmin.SessionManager) {
	// 创建 handlers
	sessionHandler := admin.NewSessionHandler(sessionMgr, deps.UserStore, deps.AdminListenAddr)
	adminHandler := admin.NewHandler(deps.Store, deps.EntryPusher, deps.BackupDir, deps.KVType)
	adminAuthMW := admin.NewAuthMiddleware(sessionMgr)

	// Session API
	mux.Handle("/api/v1/admin/session/create",
		admin.LocalOnlyMiddleware(http.HandlerFunc(sessionHandler.CreateSessionHandler), deps.AdminListenAddr))
	// 密码登录（Web admin 远程入口；公开，靠密码+等级门控，全局限流/body 限制兜底）
	mux.Handle("/api/v1/admin/session/login",
		http.HandlerFunc(sessionHandler.LoginHandler))
	// token 自检（Bearer 认证，供 SPA 刷新恢复会话）
	mux.Handle("/api/v1/admin/session",
		adminAuthMW.Middleware(http.HandlerFunc(sessionHandler.GetSessionHandler)))

	// Admin API (需要 Session Token 认证)
	// 用户管理
	mux.Handle("/api/v1/admin/users",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.ListUsersHandler)))
	mux.Handle("/api/v1/admin/users/",
		adminAuthMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/ban") {
				adminHandler.BanUserHandler(w, r)
			} else if strings.HasSuffix(path, "/unban") {
				adminHandler.UnbanUserHandler(w, r)
			} else if strings.HasSuffix(path, "/level") {
				adminHandler.SetUserLevelHandler(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))

	// 统计 API
	mux.Handle("/api/v1/admin/stats/users",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetUserStatsHandler)))
	mux.Handle("/api/v1/admin/stats/contributions",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetContributionStatsHandler)))
	mux.Handle("/api/v1/admin/stats/activity",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetActivityTrendHandler)))
	mux.Handle("/api/v1/admin/stats/registrations",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetRegistrationTrendHandler)))
	mux.Handle("/api/v1/admin/stats/entries",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.GetEntryStatsHandler)))

	// 内容审核 API（admin session-token 认证）
	mux.Handle("/api/v1/admin/entries",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.ListReviewQueueHandler)))
	mux.Handle("/api/v1/admin/entries/",
		adminAuthMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			switch {
			case strings.HasSuffix(path, "/approve"):
				adminHandler.ApproveEntryHandler(w, r)
			case strings.HasSuffix(path, "/reject"):
				adminHandler.RejectEntryHandler(w, r)
			case strings.HasSuffix(path, "/takedown"):
				adminHandler.TakedownEntryHandler(w, r)
			default:
				http.NotFound(w, r)
			}
		})))

	// KV 备份 API（admin session-token 认证）
	mux.Handle("/api/v1/admin/backup",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.CreateBackupHandler)))
	mux.Handle("/api/v1/admin/backups",
		adminAuthMW.Middleware(http.HandlerFunc(adminHandler.ListBackupsHandler)))

	// 静态文件服务 (管理页面)
	staticHandler := admin.NewStaticHandler()
	mux.Handle("/admin/", staticHandler)
	mux.Handle("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently))
}
