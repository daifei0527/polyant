// Package router 定义了 AgentWiki API 的路由注册和中间件链。
// 使用标准库 net/http 实现，不依赖第三方路由库。
package router

import (
	"net/http"
	"strings"

	"github.com/daifei0527/agentwiki/internal/api/handler"
	"github.com/daifei0527/agentwiki/internal/api/middleware"
	"github.com/daifei0527/agentwiki/internal/core/email"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/pkg/config"
)

// Dependencies 路由依赖注入容器
// 包含所有 handler 需要的存储和引擎实例
type Dependencies struct {
	EntryStore      storage.EntryStore
	UserStore       storage.UserStore
	RatingStore     storage.RatingStore
	CategoryStore   storage.CategoryStore
	SearchEngine    storage.SearchEngine
	Backlink        storage.BacklinkIndex
	EmailService    *email.Service
	VerificationMgr *email.VerificationManager
	NodeID          string
	NodeType        string
	Version         string
}

// NewRouter 创建并配置 HTTP 路由
// 注册所有 API 端点，配置中间件链
// 中间件执行顺序: RequestID -> Logging -> Recovery -> CORS -> [Auth] -> Handler
func NewRouter(store *storage.Store, cfg *config.Config) (http.Handler, error) {
	return NewRouterWithDeps(&Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
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
	userHandler := handler.NewUserHandler(
		deps.UserStore,
		deps.EntryStore,
		deps.RatingStore,
		deps.EmailService,
		deps.VerificationMgr,
	)
	categoryHandler := handler.NewCategoryHandler(deps.CategoryStore, deps.EntryStore)
	nodeHandler := handler.NewNodeHandler(deps.NodeID, deps.NodeType, deps.Version, deps.EntryStore)

	// 创建认证中间件
	authMW := middleware.NewAuthMiddleware(deps.UserStore)

	// 创建 CORS 中间件（开发环境配置）
	corsMW := middleware.NewCORSMiddleware(middleware.DefaultCORSConfig())

	// 注册公开路由（无需认证）
	registerPublicRoutes(mux, entryHandler, userHandler, categoryHandler, nodeHandler)

	// 注册认证路由（需要 Ed25519 签名认证）
	registerAuthRoutes(mux, authMW, entryHandler, userHandler, categoryHandler, nodeHandler)

	// 应用中间件链
	var httpHandler http.Handler = mux
	httpHandler = corsMW.Middleware(httpHandler)              // CORS
	httpHandler = middleware.RecoveryMiddleware(httpHandler)  // 异常恢复
	httpHandler = middleware.LoggingMiddleware(httpHandler)   // 请求日志
	httpHandler = middleware.RequestIDMiddleware(httpHandler) // 请求ID

	return httpHandler, nil
}

// registerPublicRoutes 注册公开路由（无需认证）
func registerPublicRoutes(mux *http.ServeMux, eh *handler.EntryHandler, uh *handler.UserHandler, ch *handler.CategoryHandler, nh *handler.NodeHandler) {
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
}

// registerAuthRoutes 注册需要认证的路由
func registerAuthRoutes(mux *http.ServeMux, authMW *middleware.AuthMiddleware, eh *handler.EntryHandler, uh *handler.UserHandler, ch *handler.CategoryHandler, nh *handler.NodeHandler) {
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
}
