package middleware

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig CORS 跨域配置
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig 返回默认的 CORS 配置（开发环境）
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Polyant-PublicKey", "X-Polyant-Timestamp", "X-Polyant-Signature"},
		ExposedHeaders:   []string{"Content-Length", "X-Request-Id"},
		AllowCredentials: false, // "*" + credentials is invalid per CORS spec
		MaxAge:           86400, // 24小时
	}
}

// CORSMiddleware CORS 跨域中间件
// 用于处理跨域请求，在开发环境下允许所有来源
type CORSMiddleware struct {
	config CORSConfig
}

// NewCORSMiddleware 创建 CORS 中间件实例
func NewCORSMiddleware(config CORSConfig) *CORSMiddleware {
	if len(config.AllowedOrigins) == 0 {
		config = DefaultCORSConfig()
	}
	// R3-D：混合配置（同时含 * 与具体 origin）规范化——剔除 *，白名单优先。
	// 否则 isOriginAllowed 对任意 origin 返 true，而 Middleware 的"单一 *"判断
	// 不成立，会走 else 把任意 Origin 反射回 Access-Control-Allow-Origin。
	if hasOriginWildcard(config.AllowedOrigins) && len(config.AllowedOrigins) > 1 {
		log.Printf("[CORS] 配置同时含 \"*\" 与具体 origin，剔除 \"*\"（白名单优先）: %v", config.AllowedOrigins)
		filtered := make([]string, 0, len(config.AllowedOrigins)-1)
		for _, o := range config.AllowedOrigins {
			if o != "*" {
				filtered = append(filtered, o)
			}
		}
		config.AllowedOrigins = filtered
	}
	// CORS 规范不允许通配符 origin 与 credentials 同时启用——浏览器会拒绝。
	// 作为防线，即便配置错误也强制降级。
	if config.AllowCredentials {
		for _, o := range config.AllowedOrigins {
			if o == "*" {
				config.AllowCredentials = false
				break
			}
		}
	}
	return &CORSMiddleware{
		config: config,
	}
}

// hasOriginWildcard 报告 origins 是否含 "*"。
func hasOriginWildcard(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}

// Middleware 返回 HTTP 中间件处理函数
func (m *CORSMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// 设置 Access-Control-Allow-Origin
		if m.isOriginAllowed(origin) {
			if len(m.config.AllowedOrigins) == 1 && m.config.AllowedOrigins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
		}

		// 设置 Access-Control-Allow-Methods
		if len(m.config.AllowedMethods) > 0 {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.AllowedMethods, ", "))
		}

		// 设置 Access-Control-Allow-Headers
		if len(m.config.AllowedHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.AllowedHeaders, ", "))
		}

		// 设置 Access-Control-Expose-Headers
		if len(m.config.ExposedHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.config.ExposedHeaders, ", "))
		}

		// 设置 Access-Control-Allow-Credentials
		if m.config.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// 设置 Access-Control-Max-Age
		if m.config.MaxAge > 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.config.MaxAge))
		}

		// 处理预检请求（OPTIONS）
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed 检查请求来源是否在允许列表中
func (m *CORSMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, allowed := range m.config.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
