package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ==================== TokenBucketLimiter 测试 ====================

// TestNewTokenBucketLimiter 测试创建令牌桶限制器
func TestNewTokenBucketLimiter(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)
	if limiter == nil {
		t.Fatal("NewTokenBucketLimiter 不应返回 nil")
	}

	if limiter.rate != 10 {
		t.Errorf("rate 应为 10: got %d", limiter.rate)
	}

	if limiter.burst != 5 {
		t.Errorf("burst 应为 5: got %d", limiter.burst)
	}
}

// TestTokenBucketLimiter_Allow 测试允许请求
func TestTokenBucketLimiter_Allow(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)

	// 首次请求应允许
	if !limiter.Allow("test-key") {
		t.Error("首次请求应被允许")
	}

	// 连续请求直到耗尽
	for i := 0; i < 10; i++ {
		limiter.Allow("test-key")
	}

	// 再次请求应被拒绝（桶已空）
	if limiter.Allow("test-key") {
		t.Error("令牌耗尽后请求应被拒绝")
	}
}

// TestTokenBucketLimiter_Remaining 测试剩余配额
func TestTokenBucketLimiter_Remaining(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)

	// 新键的剩余配额应为 burst
	if limiter.Remaining("new-key") != 5 {
		t.Errorf("新键剩余配额应为 5: got %d", limiter.Remaining("new-key"))
	}

	// 使用一些令牌
	limiter.Allow("test-key")
	remaining := limiter.Remaining("test-key")

	if remaining >= 5 {
		t.Errorf("使用令牌后剩余应减少: got %d", remaining)
	}
}

// TestTokenBucketLimiter_ResetAfter 测试重置时间
func TestTokenBucketLimiter_ResetAfter(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)

	// 新键的重置时间应为 0
	if limiter.ResetAfter("new-key") != 0 {
		t.Error("新键的重置时间应为 0")
	}

	// 使用一些令牌
	limiter.Allow("test-key")

	// 应有重置时间
	resetAfter := limiter.ResetAfter("test-key")
	if resetAfter == 0 {
		t.Error("使用令牌后应有重置时间")
	}
}

// TestTokenBucketLimiter_DifferentKeys 测试不同键独立计数
func TestTokenBucketLimiter_DifferentKeys(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)

	// 使用 key1 的所有令牌
	for i := 0; i < 10; i++ {
		limiter.Allow("key1")
	}

	// key1 应被限制
	if limiter.Allow("key1") {
		t.Error("key1 应被限制")
	}

	// key2 应仍可用
	if !limiter.Allow("key2") {
		t.Error("key2 应可用")
	}
}

// ==================== RateLimitConfig 测试 ====================

// TestDefaultRateLimitConfig 测试默认配置
func TestDefaultRateLimitConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	if !cfg.Enabled {
		t.Error("默认应启用速率限制")
	}

	if cfg.DefaultRate != 60 {
		t.Errorf("DefaultRate 应为 60: got %d", cfg.DefaultRate)
	}

	if cfg.DefaultBurst != 10 {
		t.Errorf("DefaultBurst 应为 10: got %d", cfg.DefaultBurst)
	}
}

// ==================== RateLimitMiddleware 测试 ====================

// TestNewRateLimitMiddleware 测试创建中间件
func TestNewRateLimitMiddleware(t *testing.T) {
	m := NewRateLimitMiddleware(nil)
	if m == nil {
		t.Fatal("NewRateLimitMiddleware 不应返回 nil")
	}

	if !m.config.Enabled {
		t.Error("默认配置应启用")
	}
}

// TestNewRateLimitMiddlewareWithConfig 测试使用自定义配置
func TestNewRateLimitMiddlewareWithConfig(t *testing.T) {
	cfg := &RateLimitConfig{
		Enabled:      true,
		DefaultRate:  100,
		DefaultBurst: 20,
	}

	m := NewRateLimitMiddleware(cfg)
	if m.config.DefaultRate != 100 {
		t.Error("应使用自定义配置")
	}
}

// TestRateLimitMiddleware_Allow 测试允许请求
func TestRateLimitMiddleware_Allow(t *testing.T) {
	cfg := &RateLimitConfig{
		Enabled:      true,
		DefaultRate:  100,
		DefaultBurst: 10,
	}
	m := NewRateLimitMiddleware(cfg)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("请求应被允许: got %d", rec.Code)
	}

	// 检查速率限制头
	if rec.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("应设置 X-RateLimit-Limit 头")
	}
}

// TestRateLimitMiddleware_Disabled 测试禁用速率限制
func TestRateLimitMiddleware_Disabled(t *testing.T) {
	cfg := &RateLimitConfig{
		Enabled: false,
	}
	m := NewRateLimitMiddleware(cfg)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("禁用时请求应被允许: got %d", rec.Code)
	}
}

// ==================== isWritePath 测试 ====================

// TestIsWritePath 测试写入路径判断
func TestIsWritePath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/api/v1/entry/create", true},
		{"/api/v1/entry/update/123", true},
		{"/api/v1/entry/delete/123", true},
		{"/api/v1/entry/rate/123", true},
		{"/api/v1/categories/create", true},
		{"/api/v1/search", false},
		{"/api/v1/entry/123", false},
		{"/api/v1/user/info", false},
	}

	for _, tt := range tests {
		result := isWritePath(tt.path)
		if result != tt.expected {
			t.Errorf("isWritePath(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

// ==================== selectLimiter 测试 ====================

// TestSelectLimiter 测试选择限制器
func TestSelectLimiter(t *testing.T) {
	m := NewRateLimitMiddleware(DefaultRateLimitConfig())

	// 搜索路径
	if m.selectLimiter("/api/v1/search") != m.searchLimit {
		t.Error("搜索路径应使用 searchLimit")
	}

	// 写入路径
	if m.selectLimiter("/api/v1/entry/create") != m.writeLimit {
		t.Error("写入路径应使用 writeLimit")
	}

	// 普通路径
	if m.selectLimiter("/api/v1/entry/123") != m.defaultLimit {
		t.Error("普通路径应使用 defaultLimit")
	}
}

// ==================== getLimitKey 测试 ====================

// TestGetLimitKey_IP 测试使用 IP 作为键
func TestGetLimitKey_IP(t *testing.T) {
	m := NewRateLimitMiddleware(DefaultRateLimitConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.RemoteAddr = "192.0.2.1:1234"

	key := m.getLimitKey(req)

	if key == "" {
		t.Error("键不应为空")
	}
}

// TestGetLimitKey_XForwardedFor 测试使用 X-Forwarded-For
func TestGetLimitKey_XForwardedFor(t *testing.T) {
	m := NewRateLimitMiddleware(DefaultRateLimitConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	key := m.getLimitKey(req)

	if key == "" {
		t.Error("键不应为空")
	}
}
