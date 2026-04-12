package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ==================== CORS 配置测试 ====================

// TestDefaultCORSConfig 测试默认 CORS 配置
func TestDefaultCORSConfig(t *testing.T) {
	cfg := DefaultCORSConfig()

	if len(cfg.AllowedOrigins) == 0 || cfg.AllowedOrigins[0] != "*" {
		t.Error("默认应允许所有来源")
	}

	if len(cfg.AllowedMethods) == 0 {
		t.Error("应配置允许的方法")
	}

	if !cfg.AllowCredentials {
		t.Error("默认应允许凭证")
	}

	if cfg.MaxAge != 86400 {
		t.Errorf("MaxAge 应为 86400: got %d", cfg.MaxAge)
	}
}

// ==================== CORS 中间件测试 ====================

// TestNewCORSMiddleware 测试创建 CORS 中间件
func TestNewCORSMiddleware(t *testing.T) {
	// 使用空配置，应使用默认值
	m := NewCORSMiddleware(CORSConfig{})
	if m == nil {
		t.Fatal("NewCORSMiddleware 不应返回 nil")
	}

	if len(m.config.AllowedOrigins) == 0 {
		t.Error("空配置应使用默认值")
	}
}

// TestNewCORSMiddlewareWithConfig 测试使用自定义配置
func TestNewCORSMiddlewareWithConfig(t *testing.T) {
	cfg := CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowCredentials: false,
		MaxAge:           3600,
	}

	m := NewCORSMiddleware(cfg)
	if m == nil {
		t.Fatal("NewCORSMiddleware 不应返回 nil")
	}

	if m.config.AllowedOrigins[0] != "https://example.com" {
		t.Error("应使用自定义配置")
	}
}

// TestCORSMiddleware_OptionsRequest 测试 OPTIONS 预检请求
func TestCORSMiddleware_OptionsRequest(t *testing.T) {
	m := NewCORSMiddleware(DefaultCORSConfig())

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("OPTIONS 请求不应调用下一个处理器")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS 应返回 204: got %d", rec.Code)
	}

	// 检查 CORS 头
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("应设置 Access-Control-Allow-Origin")
	}
}

// TestCORSMiddleware_GetRequest 测试 GET 请求
func TestCORSMiddleware_GetRequest(t *testing.T) {
	called := false
	m := NewCORSMiddleware(DefaultCORSConfig())

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("GET 请求应调用下一个处理器")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("GET 应返回 200: got %d", rec.Code)
	}
}

// TestCORSMiddleware_NoOriginHeader 测试无 Origin 头
func TestCORSMiddleware_NoOriginHeader(t *testing.T) {
	m := NewCORSMiddleware(DefaultCORSConfig())

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	// 不设置 Origin 头
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 无 Origin 头时，Access-Control-Allow-Origin 不应设置
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("无 Origin 时不应设置 Access-Control-Allow-Origin")
	}
}

// TestCORSMiddleware_AllowedOrigin 测试特定来源
func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	cfg := CORSConfig{
		AllowedOrigins: []string{"https://allowed.com", "https://trusted.com"},
	}
	m := NewCORSMiddleware(cfg)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Origin", "https://allowed.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://allowed.com" {
		t.Error("应回显允许的来源")
	}
}

// TestCORSMiddleware_DisallowedOrigin 测试不允许的来源
func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	cfg := CORSConfig{
		AllowedOrigins: []string{"https://allowed.com"},
	}
	m := NewCORSMiddleware(cfg)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Origin", "https://disallowed.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 不允许的来源不应设置 CORS 头
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("不允许的来源不应设置 Access-Control-Allow-Origin")
	}
}

// ==================== isOriginAllowed 测试 ====================

// TestIsOriginAllowed 测试来源检查
func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		origins   []string
		request   string
		expected  bool
	}{
		{[]string{"*"}, "https://any.com", true},
		{[]string{"https://example.com"}, "https://example.com", true},
		{[]string{"https://example.com"}, "https://other.com", false},
		{[]string{}, "", false},
		{[]string{"https://a.com", "https://b.com"}, "https://b.com", true},
	}

	for _, tt := range tests {
		m := &CORSMiddleware{config: CORSConfig{AllowedOrigins: tt.origins}}
		result := m.isOriginAllowed(tt.request)

		if result != tt.expected {
			t.Errorf("isOriginAllowed(%v, %q) = %v, want %v", tt.origins, tt.request, result, tt.expected)
		}
	}
}
