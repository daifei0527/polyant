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
	rate        float64       // 每秒添加的令牌数（R1-D3: float64 以支持 <1/s，如 0.5=30/min）
	burst       float64       // 桶容量
	cleanupTick time.Duration // 清理周期
}

type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
}

// NewTokenBucketLimiter 创建令牌桶限制器。rate 为令牌/秒，burst 为桶容量。
func NewTokenBucketLimiter(rate, burst float64) *TokenBucketLimiter {
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
			tokens:     l.burst - 1,
			lastUpdate: now,
		}
		l.buckets[key] = bucket
		return true
	}

	// 计算自上次更新以来添加的令牌
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	newTokens := l.rate * elapsed
	bucket.tokens = min(l.burst, bucket.tokens+newTokens)
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
		return int(l.burst)
	}

	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	newTokens := l.rate * elapsed
	tokens := min(l.burst, bucket.tokens+newTokens)

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

	if bucket.tokens >= l.burst {
		return 0
	}

	// 计算填满桶需要的时间
	tokensNeeded := l.burst - bucket.tokens
	secondsNeeded := tokensNeeded / l.rate
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

// RateLimitConfig 速率限制配置。速率单位均为 令牌/秒（R1-D3: float64 以支持 <1/s）。
type RateLimitConfig struct {
	// Enabled 是否启用速率限制
	Enabled bool
	// DefaultRate 默认令牌/秒（1 = 60/min）
	DefaultRate float64
	// DefaultBurst 默认突发容量
	DefaultBurst float64
	// SearchRate 搜索接口令牌/秒
	SearchRate float64
	// SearchBurst 搜索接口突发容量
	SearchBurst float64
	// WriteRate 写入接口令牌/秒
	WriteRate float64
	// WriteBurst 写入接口突发容量
	WriteBurst float64
	// TrustedProxies R1-D1: 仅当 RemoteAddr 属于这些 IP/CIDR 时才采信 X-Forwarded-For。
	// 为空表示不信任任何反代，限流键一律用连接级 RemoteAddr（最安全）。
	TrustedProxies []string
}

// DefaultRateLimitConfig 默认速率限制配置。
// 旧实现把 rate 当 "请求/分钟" 但令牌桶按 "令牌/秒" 计算，导致实际阈值是注释的 60 倍
// （DefaultRate=60 实为 60/s=3600/min，限流形同虚设）。R1-D3 修正为真实的 令牌/秒。
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:      true,
		DefaultRate:  1, // 1/s = 60/min
		DefaultBurst: 10,
		SearchRate:   1, // 1/s = 60/min
		SearchBurst:  10,
		WriteRate:    0.5, // 0.5/s = 30/min（写入更严）
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
		// R1-D3: OPTIONS 预检不计入限流（CORS 预检不应被限流阻断）
		if !m.config.Enabled || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// 获取限制键（用户公钥或 IP）
		key := m.getLimitKey(r)

		// 选择适当的限制器
		limiter := m.selectLimiter(r)

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

// selectLimiter 按 HTTP 方法选择限制器（R1-D3: 替换原路径白名单，更全面地覆盖写操作）。
// POST/PUT/PATCH/DELETE → writeLimit；GET /search → searchLimit；其余 → defaultLimit。
func (m *RateLimitMiddleware) selectLimiter(r *http.Request) *TokenBucketLimiter {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return m.writeLimit
	}
	if r.URL.Path == "/api/v1/search" {
		return m.searchLimit
	}
	return m.defaultLimit
}

// writeRateLimitError 写入速率限制错误响应
func (m *RateLimitMiddleware) writeRateLimitError(w http.ResponseWriter, limiter *TokenBucketLimiter, key string) {
	retryAfter := limiter.ResetAfter(key)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(retryAfter).Unix()))
	w.WriteHeader(http.StatusTooManyRequests)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    42901,
		"message": "rate limit exceeded, please retry later",
		"data": map[string]interface{}{
			"retry_after": int(retryAfter.Seconds()),
		},
	}) // HTTP 响应写入；headers 已发，无法恢复
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
