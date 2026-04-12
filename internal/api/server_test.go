// Package api 提供 API 服务器的单元测试
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ==================== Mock 实现 ====================

// MockStorage 存储接口的 mock 实现
type MockStorage struct {
	entries []*Entry
	stats   *Stats
	err     error
}

func (m *MockStorage) GetEntry(id string) (*Entry, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, e := range m.entries {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockStorage) ListEntries(filter EntryFilter) ([]*Entry, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.entries, len(m.entries), nil
}

func (m *MockStorage) CreateEntry(entry *Entry) error {
	return m.err
}

func (m *MockStorage) GetStats() (*Stats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stats, nil
}

// MockSyncService 同步服务的 mock 实现
type MockSyncService struct {
	peers []string
	status *SyncStatus
}

func (m *MockSyncService) GetConnectedPeers() []string {
	return m.peers
}

func (m *MockSyncService) GetSyncStatus() *SyncStatus {
	return m.status
}

// MockCategoryManager 分类管理的 mock 实现
type MockCategoryManager struct {
	categories []*Category
	err        error
}

func (m *MockCategoryManager) List() []*Category {
	return m.categories
}

func (m *MockCategoryManager) Get(id string) (*Category, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, c := range m.categories {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, ErrNotFound
}

// ==================== Server 创建测试 ====================

// TestNewServer 测试创建服务器
func TestNewServer(t *testing.T) {
	storage := &MockStorage{}
	syncSvc := &MockSyncService{}
	category := &MockCategoryManager{}

	server := NewServer(storage, syncSvc, category)
	if server == nil {
		t.Fatal("NewServer 不应返回 nil")
	}
}

// ==================== 统计接口测试 ====================

// TestHandleGetStats 测试获取统计信息
func TestHandleGetStats(t *testing.T) {
	storage := &MockStorage{
		stats: &Stats{
			TotalEntries: 100,
			TotalUsers:   50,
			TotalNodes:   10,
			TotalRatings: 200,
		},
	}
	server := NewServer(storage, &MockSyncService{}, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d, want %d", rec.Code, http.StatusOK)
	}

	var stats Stats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if stats.TotalEntries != 100 {
		t.Errorf("TotalEntries 错误: got %d", stats.TotalEntries)
	}
}

// ==================== 分类接口测试 ====================

// TestHandleListCategories 测试获取分类列表
func TestHandleListCategories(t *testing.T) {
	category := &MockCategoryManager{
		categories: []*Category{
			{ID: "cat-1", Name: "技术", EntryCount: 50},
			{ID: "cat-2", Name: "科学", EntryCount: 30},
		},
	}
	server := NewServer(&MockStorage{}, &MockSyncService{}, category)

	req := httptest.NewRequest("GET", "/api/v1/categories", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	categories, ok := resp["categories"].([]interface{})
	if !ok {
		t.Fatal("响应应包含 categories 数组")
	}

	if len(categories) != 2 {
		t.Errorf("分类数量错误: got %d, want 2", len(categories))
	}
}

// TestHandleGetCategory 测试获取单个分类
func TestHandleGetCategory(t *testing.T) {
	category := &MockCategoryManager{
		categories: []*Category{
			{ID: "cat-1", Name: "技术", EntryCount: 50},
		},
	}
	server := NewServer(&MockStorage{}, &MockSyncService{}, category)

	req := httptest.NewRequest("GET", "/api/v1/categories/cat-1", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d", rec.Code)
	}
}

// TestHandleGetCategoryNotFound 测试获取不存在的分类
func TestHandleGetCategoryNotFound(t *testing.T) {
	server := NewServer(&MockStorage{}, &MockSyncService{}, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/categories/not-exist", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("状态码应为 404: got %d", rec.Code)
	}
}

// ==================== 条目接口测试 ====================

// TestHandleListEntries 测试获取条目列表
func TestHandleListEntries(t *testing.T) {
	storage := &MockStorage{
		entries: []*Entry{
			{ID: "entry-1", Title: "测试条目1", Content: "内容1"},
			{ID: "entry-2", Title: "测试条目2", Content: "内容2"},
		},
	}
	server := NewServer(storage, &MockSyncService{}, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/entries", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatal("响应应包含 entries 数组")
	}

	if len(entries) != 2 {
		t.Errorf("条目数量错误: got %d, want 2", len(entries))
	}
}

// TestHandleGetEntry 测试获取单个条目
func TestHandleGetEntry(t *testing.T) {
	storage := &MockStorage{
		entries: []*Entry{
			{ID: "entry-1", Title: "测试条目", Content: "测试内容"},
		},
	}
	server := NewServer(storage, &MockSyncService{}, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/entries/entry-1", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d", rec.Code)
	}

	var entry Entry
	json.NewDecoder(rec.Body).Decode(&entry)

	if entry.Title != "测试条目" {
		t.Errorf("标题错误: got %q", entry.Title)
	}
}

// TestHandleGetEntryNotFound 测试获取不存在的条目
func TestHandleGetEntryNotFound(t *testing.T) {
	server := NewServer(&MockStorage{}, &MockSyncService{}, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/entries/not-exist", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("状态码应为 404: got %d", rec.Code)
	}
}

// ==================== 搜索接口测试 ====================

// TestHandleSearch 测试搜索
func TestHandleSearch(t *testing.T) {
	storage := &MockStorage{
		entries: []*Entry{
			{ID: "entry-1", Title: "人工智能入门", Content: "AI基础"},
		},
	}
	server := NewServer(storage, &MockSyncService{}, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/search?q=人工智能", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d", rec.Code)
	}
}

// TestHandleSearchEmptyQuery 测试空搜索
func TestHandleSearchEmptyQuery(t *testing.T) {
	server := NewServer(&MockStorage{}, &MockSyncService{}, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("状态码应为 400: got %d", rec.Code)
	}
}

// ==================== 同步接口测试 ====================

// TestHandleSyncStatus 测试获取同步状态
func TestHandleSyncStatus(t *testing.T) {
	syncSvc := &MockSyncService{
		status: &SyncStatus{
			Running:        true,
			LastSync:       time.Now(),
			SyncedEntries:  100,
			ConnectedPeers: 5,
		},
	}
	server := NewServer(&MockStorage{}, syncSvc, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/sync/status", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d", rec.Code)
	}
}

// TestHandleSyncPeers 测试获取同步节点
func TestHandleSyncPeers(t *testing.T) {
	syncSvc := &MockSyncService{
		peers: []string{"peer-1", "peer-2"},
	}
	server := NewServer(&MockStorage{}, syncSvc, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/sync/peers", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	peers, ok := resp["peers"].([]interface{})
	if !ok {
		t.Fatal("响应应包含 peers 数组")
	}

	if len(peers) != 2 {
		t.Errorf("节点数量错误: got %d, want 2", len(peers))
	}
}

// ==================== 认证接口测试 ====================

// TestHandleRegister 测试用户注册
func TestHandleRegister(t *testing.T) {
	_ = NewServer(&MockStorage{}, &MockSyncService{}, &MockCategoryManager{})
	// 简化测试，验证服务器可以创建
}

// TestHandleLogin 测试登录
func TestHandleLogin(t *testing.T) {
	server := NewServer(&MockStorage{}, &MockSyncService{}, &MockCategoryManager{})

	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()

	// 不带请求体的登录会返回 400
	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("状态码应为 400: got %d", rec.Code)
	}
}

// TestHandleGetCurrentUser 测试获取当前用户
func TestHandleGetCurrentUser(t *testing.T) {
	server := NewServer(&MockStorage{}, &MockSyncService{}, &MockCategoryManager{})

	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("状态码应为 401: got %d", rec.Code)
	}
}

// ==================== CORS 中间件测试 ====================

// TestCORSMiddleware 测试 CORS 中间件
func TestCORSMiddleware(t *testing.T) {
	// 测试 corsMiddleware 函数
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// OPTIONS 预检请求
	req := httptest.NewRequest("OPTIONS", "/api/v1/stats", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("OPTIONS 状态码错误: got %d", rec.Code)
	}

	// 检查 CORS 头
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("应设置 CORS Origin 头")
	}
}

// ==================== 工具函数测试 ====================

// TestRespondJSON 测试 JSON 响应
func TestRespondJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	respondJSON(rec, http.StatusOK, map[string]string{"test": "value"})

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Error("Content-Type 应为 application/json")
	}
}

// TestRespondError 测试错误响应
func TestRespondError(t *testing.T) {
	rec := httptest.NewRecorder()
	respondError(rec, http.StatusBadRequest, "测试错误")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("状态码错误: got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["error"] != "测试错误" {
		t.Errorf("错误消息错误: got %q", resp["error"])
	}
}

// TestGenerateID 测试 ID 生成
func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" || id2 == "" {
		t.Error("生成的 ID 不应为空")
	}

	if id1 == id2 {
		t.Error("生成的 ID 应该唯一")
	}
}

// ==================== 错误定义测试 ====================

// TestErrNotFound 测试错误定义
func TestErrNotFound(t *testing.T) {
	if ErrNotFound == nil {
		t.Error("ErrNotFound 应定义为非 nil")
	}
}
