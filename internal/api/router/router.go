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

// EntryPusher 条目推送接口
type EntryPusher interface {
	// PushEntry 推送条目到种子节点
	PushEntry(entry *model.KnowledgeEntry, signature []byte) error
}

// Dependencies 路由依赖注入容器
// 包含所有 handler 需要的存储和引擎实例
type Dependencies struct {
	Store           *storage.Store
	EntryStore      storage.EntryStore
	UserStore       storage.UserStore
	RatingStore     storage.RatingStore
	CategoryStore   storage.CategoryStore
	SearchEngine    index.SearchEngine
	Backlink        storage.BacklinkIndex
	EmailService    *email.Service
	VerificationMgr *email.VerificationManager
	RemoteQuerier   RemoteQuerier // 远程查询服务
	EntryPusher     EntryPusher   // 条目推送服务
	KVStore         kv.Store      // KV 存储（选举等功能需要）
	SessionManager  *coreadmin.SessionManager
	NodeID          string
	NodeType        string
	Version         string
}

// NewRouter 创建并配置 HTTP 路由
// 注册所有 API 端点，配置中间件链
// 中间件执行顺序: RequestID -> Logging -> Recovery -> CORS -> [Auth] -> Handler
func NewRouter(store *storage.Store, cfg *config.Config) (http.Handler, error) {
	return NewRouterWithDeps(&Dependencies{
		Store:         store,
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		KVStore:       store.KVStore(),
		NodeID:        "local-node-1",
		NodeType:      cfg.Node.Type,
		Version:       "v0.1.0-dev",
	})
}

// NewRouterWithDeps 使用依赖容器创建路由
func NewRouterWithDeps(deps *Dependencies) (http.Handler, error) {
	mux := http.NewServeMux()

	// 创建验证码管理器
	if deps.VerificationMgr == nil {
		deps.VerificationMgr = email.NewVerificationManager()
	}

	// 创建各 handler
	entryHandler := handler.NewEntryHandler(deps.EntryStore, deps.SearchEngine, deps.Backlink, deps.UserStore)

	// 设置远程查询服务
	if deps.RemoteQuerier != nil {
		entryHandler.SetRemoteQuerier(&remoteQuerierAdapter{deps.RemoteQuerier})
	}

	userHandler := handler.NewUserHandler(
		deps.Store,
		deps.UserStore,
		deps.EntryStore,
		deps.RatingStore,
		deps.EmailService,
		deps.VerificationMgr,
	)
	categoryHandler := handler.NewCategoryHandler(deps.CategoryStore, deps.EntryStore)
	nodeHandler := handler.NewNodeHandler(deps.NodeID, deps.NodeType, deps.Version, deps.EntryStore)

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
	batchHandler := handler.NewBatchHandler(deps.EntryStore, deps.SearchEngine, deps.Backlink, deps.UserStore)

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

	// 创建 CORS 中间件（开发环境配置）
	corsMW := middleware.NewCORSMiddleware(middleware.DefaultCORSConfig())

	// 创建速率限制中间件
	rateLimitMW := middleware.NewRateLimitMiddleware(middleware.DefaultRateLimitConfig())

	// 创建 Session Manager
	var sessionMgr *coreadmin.SessionManager
	if deps.SessionManager != nil {
		sessionMgr = deps.SessionManager
	} else {
		sessionMgr = coreadmin.NewSessionManager(24 * time.Hour)
	}

	// 注册公开路由（无需认证）
	registerPublicRoutes(mux, entryHandler, userHandler, categoryHandler, nodeHandler, electionHandler)

	// 注册认证路由（需要 Ed25519 签名认证）
	registerAuthRoutes(mux, authMW, entryHandler, userHandler, categoryHandler, nodeHandler, adminHandler, electionHandler, batchHandler, exportHandler, auditHandler, statsHandler)

	// 注册 Admin 路由（Session Token 认证）
	registerAdminRoutes(mux, deps, sessionMgr)

	// 应用中间件链
	var httpHandler http.Handler = mux
	if auditMW != nil {
		httpHandler = auditMW.Middleware(httpHandler)
	}
	httpHandler = corsMW.Middleware(httpHandler)              // CORS
	httpHandler = rateLimitMW.Middleware(httpHandler)         // 速率限制
	httpHandler = middleware.RecoveryMiddleware(httpHandler)  // 异常恢复
	httpHandler = middleware.LoggingMiddleware(httpHandler)   // 请求日志
	httpHandler = middleware.RequestIDMiddleware(httpHandler) // 请求ID

	return httpHandler, nil
}

// remoteQuerierAdapter 适配 RemoteQuerier 到 handler.RemoteQuerier 接口
type remoteQuerierAdapter struct {
	querier RemoteQuerier
}

func (a *remoteQuerierAdapter) SearchWithRemote(ctx context.Context, query index.SearchQuery) (*index.SearchResult, error) {
	return a.querier.SearchWithRemote(ctx, query)
}

// registerPublicRoutes 注册公开路由（无需认证）
func registerPublicRoutes(mux *http.ServeMux, eh *handler.EntryHandler, uh *handler.UserHandler, ch *handler.CategoryHandler, nh *handler.NodeHandler, elh *handler.ElectionHandler) {
	// 搜索知识条目
	mux.HandleFunc("/api/v1/search", eh.SearchHandler)

	// 获取条目详情
	mux.HandleFunc("/api/v1/entry/", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// 获取分类列表
	mux.HandleFunc("/api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		// 仅处理 GET 请求为公开路由
		if r.Method == http.MethodGet {
			ch.ListCategoriesHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	// 获取分类下的条目
	mux.HandleFunc("/api/v1/categories/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/entries") {
			ch.GetCategoryEntriesHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	// 获取节点状态
	mux.HandleFunc("/api/v1/node/status", nh.GetNodeStatusHandler)

	// 用户注册
	mux.HandleFunc("/api/v1/user/register", uh.RegisterHandler)

	// 选举公开路由（列出选举、获取选举详情）
	if elh != nil {
		// 列出选举 GET /api/v1/elections?status=active
		mux.HandleFunc("/api/v1/elections", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				elh.ListElectionsHandler(w, r)
			} else {
				http.NotFound(w, r)
			}
		})

		// 获取选举详情 GET /api/v1/elections/{id}
		mux.HandleFunc("/api/v1/elections/", func(w http.ResponseWriter, r *http.Request) {
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
		})
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
		mux.Handle("/api/v1/admin/audit/logs", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv5, http.HandlerFunc(auh.ListAuditLogsHandler))))

		// 获取审计统计 GET /api/v1/admin/audit/stats - Lv5 (SuperAdmin)
		mux.Handle("/api/v1/admin/audit/stats", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv5, http.HandlerFunc(auh.GetAuditStatsHandler))))

		// 删除审计日志 DELETE /api/v1/admin/audit/logs - Lv5 (SuperAdmin)
		mux.Handle("/api/v1/admin/audit/logs/delete", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv5, http.HandlerFunc(auh.DeleteAuditLogsHandler))))
	}

	// ==================== 数据导出/导入路由 ====================
	if exh != nil {
		// 数据导出 GET /api/v1/admin/export - Lv4+ (Admin)
		mux.Handle("/api/v1/admin/export", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv4, http.HandlerFunc(exh.ExportHandler))))

		// 数据导入 POST /api/v1/admin/import - Lv4+ (Admin)
		mux.Handle("/api/v1/admin/import", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv4, http.HandlerFunc(exh.ImportHandler))))
	}

	// ==================== 选举路由 ====================
	if elh != nil {
		// 创建选举 POST /api/v1/elections - Lv5 (SuperAdmin)
		mux.Handle("/api/v1/elections/create", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv5, http.HandlerFunc(elh.CreateElectionHandler))))

		// 提名候选人 POST /api/v1/elections/{id}/candidates - 已认证用户
		mux.Handle("/api/v1/elections/candidates/", authMW.Middleware(http.HandlerFunc(elh.NominateCandidateHandler)))

		// 确认接受提名 POST /api/v1/elections/{id}/candidates/{user_id}/confirm - 被提名人自己
		mux.Handle("/api/v1/elections/candidates/confirm/", authMW.Middleware(http.HandlerFunc(elh.ConfirmNominationHandler)))

		// 投票 POST /api/v1/elections/{id}/vote - Lv3+
		mux.Handle("/api/v1/elections/vote/", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv3, http.HandlerFunc(elh.VoteHandler))))

		// 关闭选举 POST /api/v1/elections/{id}/close - Lv5 (SuperAdmin)
		mux.Handle("/api/v1/elections/close/", authMW.Middleware(authMW.RequireLevel(model.UserLevelLv5, http.HandlerFunc(elh.CloseElectionHandler))))
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
	sessionHandler := admin.NewSessionHandler(sessionMgr, deps.UserStore)
	adminHandler := admin.NewHandler(deps.Store)
	adminAuthMW := admin.NewAuthMiddleware(sessionMgr)

	// Session API (仅本地访问)
	mux.Handle("/api/v1/admin/session/create",
		admin.LocalOnlyMiddleware(http.HandlerFunc(sessionHandler.CreateSessionHandler)))

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

	// 静态文件服务 (管理页面)
	staticHandler := admin.NewStaticHandler()
	mux.Handle("/admin/", staticHandler)
	mux.Handle("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently))
}
