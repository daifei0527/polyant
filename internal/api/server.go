// Package api 提供 HTTP API 服务
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// ErrNotFound 资源未找到错误
var ErrNotFound = errors.New("resource not found")

// Server API 服务器
type Server struct {
	router   *mux.Router
	server   *http.Server
	storage  Storage
	sync     SyncService
	category CategoryManager
}

// Storage 存储接口
type Storage interface {
	GetEntry(id string) (*Entry, error)
	ListEntries(filter EntryFilter) ([]*Entry, int, error)
	CreateEntry(entry *Entry) error
	GetStats() (*Stats, error)
}

// SyncService 同步服务接口
type SyncService interface {
	GetConnectedPeers() []string
	GetSyncStatus() *SyncStatus
}

// CategoryManager 分类管理接口
type CategoryManager interface {
	List() []*Category
	Get(id string) (*Category, error)
}

// Entry 知识条目
type Entry struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Category    string    `json:"category"`
	Tags        []string  `json:"tags"`
	AuthorID    string    `json:"author_id"`
	AuthorName  string    `json:"author_name"`
	AvgScore    float64   `json:"avg_score"`
	RatingCount int       `json:"rating_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EntryFilter 条目过滤条件
type EntryFilter struct {
	Category string `json:"category"`
	Tag      string `json:"tag"`
	Query    string `json:"query"`
	Sort     string `json:"sort"`
	Order    string `json:"order"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

// Category 分类
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    string `json:"parent_id,omitempty"`
	Icon        string `json:"icon,omitempty"`
	EntryCount  int    `json:"entry_count"`
}

// Stats 统计信息
type Stats struct {
	TotalEntries  int `json:"total_entries"`
	TotalUsers    int `json:"total_users"`
	TotalNodes    int `json:"total_nodes"`
	TotalRatings  int `json:"total_ratings"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	Running       bool      `json:"running"`
	LastSync      time.Time `json:"last_sync"`
	SyncedEntries int       `json:"synced_entries"`
	ConnectedPeers int      `json:"connected_peers"`
}

// NewServer 创建 API 服务器
func NewServer(storage Storage, sync SyncService, category CategoryManager) *Server {
	s := &Server{
		router:   mux.NewRouter(),
		storage:  storage,
		sync:     sync,
		category: category,
	}
	
	s.setupRoutes()
	return s
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	api := s.router.PathPrefix("/api/v1").Subrouter()
	
	// 统计
	api.HandleFunc("/stats", s.handleGetStats).Methods("GET")
	
	// 分类
	api.HandleFunc("/categories", s.handleListCategories).Methods("GET")
	api.HandleFunc("/categories/{id}", s.handleGetCategory).Methods("GET")
	
	// 条目
	api.HandleFunc("/entries", s.handleListEntries).Methods("GET")
	api.HandleFunc("/entries", s.handleCreateEntry).Methods("POST")
	api.HandleFunc("/entries/{id}", s.handleGetEntry).Methods("GET")
	api.HandleFunc("/entries/{id}", s.handleUpdateEntry).Methods("PUT")
	api.HandleFunc("/entries/{id}", s.handleDeleteEntry).Methods("DELETE")
	
	// 搜索
	api.HandleFunc("/search", s.handleSearch).Methods("GET")
	
	// 同步
	api.HandleFunc("/sync/status", s.handleSyncStatus).Methods("GET")
	api.HandleFunc("/sync/peers", s.handleSyncPeers).Methods("GET")
	
	// 认证
	api.HandleFunc("/auth/register", s.handleRegister).Methods("POST")
	api.HandleFunc("/auth/login", s.handleLogin).Methods("POST")
	api.HandleFunc("/auth/me", s.handleGetCurrentUser).Methods("GET")
	
	// 静态文件
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	s.router.HandleFunc("/", s.handleIndex).Methods("GET")
	s.router.HandleFunc("/search", s.handleSearchPage).Methods("GET")
	s.router.HandleFunc("/categories/{id}", s.handleCategoryPage).Methods("GET")
	s.router.HandleFunc("/entries/{id}", s.handleEntryPage).Methods("GET")
	s.router.HandleFunc("/contribute", s.handleContributePage).Methods("GET")
}

// Run 启动服务器
func (s *Server) Run(addr string) error {
	s.server = &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(s.router),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s.server.ListenAndServe()
}

// Shutdown 关闭服务器
func (s *Server) Shutdown() error {
	if s.server != nil {
		return s.server.Shutdown(nil)
	}
	return nil
}

// ==================== 处理函数 ====================

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.storage.GetStats()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	categories := s.category.List()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"categories": categories,
	})
}

func (s *Server) handleGetCategory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	cat, err := s.category.Get(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "分类不存在")
		return
	}
	
	respondJSON(w, http.StatusOK, cat)
}

func (s *Server) handleListEntries(w http.ResponseWriter, r *http.Request) {
	filter := EntryFilter{
		Category: r.URL.Query().Get("category"),
		Tag:      r.URL.Query().Get("tag"),
		Query:    r.URL.Query().Get("q"),
		Sort:     r.URL.Query().Get("sort"),
		Order:    r.URL.Query().Get("order"),
	}
	
	filter.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	
	filter.Offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	
	entries, total, err := s.storage.ListEntries(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}

func (s *Server) handleGetEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	entry, err := s.storage.GetEntry(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "条目不存在")
		return
	}
	
	respondJSON(w, http.StatusOK, entry)
}

func (s *Server) handleCreateEntry(w http.ResponseWriter, r *http.Request) {
	var entry Entry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据")
		return
	}
	
	// TODO: 验证用户身份
	// TODO: 验证数据有效性
	
	entry.ID = generateID()
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	
	if err := s.storage.CreateEntry(&entry); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	respondJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleUpdateEntry(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现更新逻辑
	respondError(w, http.StatusNotImplemented, "功能开发中")
}

func (s *Server) handleDeleteEntry(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现删除逻辑
	respondError(w, http.StatusNotImplemented, "功能开发中")
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "搜索关键词不能为空")
		return
	}
	
	filter := EntryFilter{
		Query: query,
		Limit: 20,
	}
	
	entries, total, err := s.storage.ListEntries(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
		"query":   query,
	})
}

func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	status := s.sync.GetSyncStatus()
	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleSyncPeers(w http.ResponseWriter, r *http.Request) {
	peers := s.sync.GetConnectedPeers()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peers,
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey string  `json:"public_key"`
		AgentName string  `json:"agent_name"`
		Email     *string `json:"email"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据")
		return
	}
	
	if req.PublicKey == "" || req.AgentName == "" {
		respondError(w, http.StatusBadRequest, "公钥和 Agent 名称不能为空")
		return
	}
	
	// TODO: 实际的用户注册逻辑
	
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "注册成功",
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey string `json:"public_key"`
		Signature string `json:"signature"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据")
		return
	}
	
	// TODO: 实际的登录验证逻辑
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token": "sample-token",
		"user": map[string]interface{}{
			"public_key":  req.PublicKey,
			"agent_name": "示例用户",
		},
	})
}

func (s *Server) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// TODO: 从 token 获取当前用户
	respondError(w, http.StatusUnauthorized, "未登录")
}

// ==================== 页面处理 ====================

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/index.html")
}

func (s *Server) handleSearchPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/search.html")
}

func (s *Server) handleCategoryPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/category.html")
}

func (s *Server) handleEntryPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/entry.html")
}

func (s *Server) handleContributePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/contribute.html")
}

// ==================== 工具函数 ====================

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
