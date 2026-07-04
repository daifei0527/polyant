package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// RateLimiter 速率限制器接口
type RateLimiter interface {
	// Allow 检查是否允许请求
	Allow(key string) bool
	// Remaining 返回剩余配额
	Remaining(key string) int
	// ResetAfter 返回配额重置时间
	ResetAfter(key string) time.Duration
}

// TokenBucketLimiter 令牌桶速率限制器
type TokenBucketLimiter struct {
	mu          sync.RWMutex
	buckets     map[string]*tokenBucket
	rate        int           // 每秒添加的令牌数
	burst       int           // 桶容量
	cleanupTick time.Duration // 清理周期
}

type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
}

// NewTokenBucketLimiter 创建令牌桶限制器
func NewTokenBucketLimiter(rate, burst int) *TokenBucketLimiter {
	l := &TokenBucketLimiter{
		buckets:     make(map[string]*tokenBucket),
		rate:        rate,
		burst:       burst,
		cleanupTick: time.Minute,
	}
	go l.cleanupLoop()
	return l
}

// Allow 检查是否允许请求
func (l *TokenBucketLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	bucket, exists := l.buckets[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:     float64(l.burst - 1),
			lastUpdate: now,
		}
		l.buckets[key] = bucket
		return true
	}

	// 计算自上次更新以来添加的令牌
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	newTokens := float64(l.rate) * elapsed
	bucket.tokens = min(float64(l.burst), bucket.tokens+newTokens)
	bucket.lastUpdate = now

	if bucket.tokens >= 1 {
		bucket.tokens -= 1
		return true
	}

	return false
}

// Remaining 返回剩余配额
func (l *TokenBucketLimiter) Remaining(key string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	bucket, exists := l.buckets[key]
	if !exists {
		return l.burst
	}

	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	newTokens := float64(l.rate) * elapsed
	tokens := min(float64(l.burst), bucket.tokens+newTokens)

	return int(tokens)
}

// ResetAfter 返回配额重置时间
func (l *TokenBucketLimiter) ResetAfter(key string) time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()

	bucket, exists := l.buckets[key]
	if !exists {
		return 0
	}

	if bucket.tokens >= float64(l.burst) {
		return 0
	}

	// 计算填满桶需要的时间
	tokensNeeded := float64(l.burst) - bucket.tokens
	secondsNeeded := tokensNeeded / float64(l.rate)
	return time.Duration(secondsNeeded * float64(time.Second))
}

// cleanupLoop 定期清理不活跃的桶
func (l *TokenBucketLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cleanupTick)
	defer ticker.Stop()

	for range ticker.C {
		l.cleanup()
	}
}

// cleanup 清理过期的桶
func (l *TokenBucketLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	expiry := 5 * time.Minute

	for key, bucket := range l.buckets {
		if now.Sub(bucket.lastUpdate) > expiry {
			delete(l.buckets, key)
		}
	}
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	// Enabled 是否启用速率限制
	Enabled bool
	// DefaultRate 默认每秒请求数
	DefaultRate int
	// DefaultBurst 默认突发容量
	DefaultBurst int
	// SearchRate 搜索接口速率
	SearchRate int
	// SearchBurst 搜索接口突发容量
	SearchBurst int
	// WriteRate 写入接口速率
	WriteRate int
	// WriteBurst 写入接口突发容量
	WriteBurst int
	// TrustedProxies R1-D1: 仅当 RemoteAddr 属于这些 IP/CIDR 时才采信 X-Forwarded-For。
	// 为空表示不信任任何反代，限流键一律用连接级 RemoteAddr（最安全）。
	TrustedProxies []string
}

// DefaultRateLimitConfig 默认速率限制配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:      true,
		DefaultRate:  60, // 60 请求/分钟 = 1 请求/秒
		DefaultBurst: 10,
		SearchRate:   30, // 30 请求/分钟
		SearchBurst:  10,
		WriteRate:    10, // 10 请求/分钟
		WriteBurst:   5,
	}
}

// RateLimitMiddleware 速率限制中间件
type RateLimitMiddleware struct {
	config       *RateLimitConfig
	defaultLimit *TokenBucketLimiter
	searchLimit  *TokenBucketLimiter
	writeLimit   *TokenBucketLimiter
}

// NewRateLimitMiddleware 创建速率限制中间件
func NewRateLimitMiddleware(cfg *RateLimitConfig) *RateLimitMiddleware {
	if cfg == nil {
		cfg = DefaultRateLimitConfig()
	}

	return &RateLimitMiddleware{
		config:       cfg,
		defaultLimit: NewTokenBucketLimiter(cfg.DefaultRate, cfg.DefaultBurst),
		searchLimit:  NewTokenBucketLimiter(cfg.SearchRate, cfg.SearchBurst),
		writeLimit:   NewTokenBucketLimiter(cfg.WriteRate, cfg.WriteBurst),
	}
}

// Middleware 返回 HTTP 中间件
func (m *RateLimitMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// 获取限制键（用户公钥或 IP）
		key := m.getLimitKey(r)

		// 选择适当的限制器
		limiter := m.selectLimiter(r.URL.Path)

		// 检查是否允许
		if !limiter.Allow(key) {
			m.writeRateLimitError(w, limiter, key)
			return
		}

		// 设置速率限制头
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limiter.Remaining(key)))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limiter.Remaining(key)))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(limiter.ResetAfter(key)).Unix()))

		next.ServeHTTP(w, r)
	})
}

// getLimitKey 获取限制键。R1-D1：仅当 RemoteAddr 属于受信代理时才采信 XFF 首跳，
// 否则一律用连接级 RemoteAddr 的 host（去端口），防止 XFF 伪造绕过限流。
func (m *RateLimitMiddleware) getLimitKey(r *http.Request) string {
	// 优先使用用户公钥
	user := GetUserFromContextFromRateLimit(r.Context())
	if user != nil && len(user.PublicKey) >= 16 {
		return fmt.Sprintf("user:%s", user.PublicKey[:16])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	// 仅受信代理来源的 XFF 才采信
	if m.isTrustedProxy(host) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if first != "" {
				return "ip:" + first
			}
		}
	}

	return "ip:" + host
}

// isTrustedProxy 判断 host 是否属于受信代理（精确 IP 或 CIDR 匹配）。
func (m *RateLimitMiddleware) isTrustedProxy(host string) bool {
	if host == "" || len(m.config.TrustedProxies) == 0 {
		return false
	}
	ip := net.ParseIP(host)
	for _, tp := range m.config.TrustedProxies {
		if tp == host {
			return true
		}
		if _, cidr, err := net.ParseCIDR(tp); err == nil && ip != nil && cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// selectLimiter 根据路径选择限制器
func (m *RateLimitMiddleware) selectLimiter(path string) *TokenBucketLimiter {
	// 搜索接口
	if path == "/api/v1/search" {
		return m.searchLimit
	}

	// 写入接口
	if isWritePath(path) {
		return m.writeLimit
	}

	return m.defaultLimit
}

// isWritePath 判断是否为写入路径
func isWritePath(path string) bool {
	writePaths := []string{
		"/api/v1/entry/create",
		"/api/v1/entry/update",
		"/api/v1/entry/delete",
		"/api/v1/entry/rate",
		"/api/v1/categories/create",
	}

	for _, p := range writePaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// writeRateLimitError 写入速率限制错误响应
func (m *RateLimitMiddleware) writeRateLimitError(w http.ResponseWriter, limiter *TokenBucketLimiter, key string) {
	retryAfter := limiter.ResetAfter(key)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(retryAfter).Unix()))
	w.WriteHeader(http.StatusTooManyRequests)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    42901,
		"message": "rate limit exceeded, please retry later",
		"data": map[string]interface{}{
			"retry_after": int(retryAfter.Seconds()),
		},
	})
}

// GetUserFromContextFromRateLimit 从上下文获取用户（避免循环导入）
func GetUserFromContextFromRateLimit(ctx context.Context) *model.User {
	if ctx == nil {
		return nil
	}
	val := ctx.Value(UserKey)
	if val == nil {
		return nil
	}
	user, ok := val.(*model.User)
	if !ok {
		return nil
	}
	return user
}
