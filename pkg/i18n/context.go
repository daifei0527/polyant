// pkg/i18n/context.go
package i18n

import (
	"context"
	"net/http"
)

// contextKey 上下文键类型
type contextKey string

const (
	// LangKey 上下文中的语言键
	LangKey contextKey = "lang"
)

// WithLang 将语言设置到上下文
func WithLang(ctx context.Context, lang Lang) context.Context {
	return context.WithValue(ctx, LangKey, lang)
}

// GetLangFromContext 从上下文获取语言
func GetLangFromContext(ctx context.Context) Lang {
	if lang, ok := ctx.Value(LangKey).(Lang); ok {
		return lang
	}
	return GetGlobalLang()
}

// GetLangFromRequest 从 HTTP 请求获取语言
// 优先级: URL参数 > Header > 全局默认
func GetLangFromRequest(r *http.Request) Lang {
	// 1. URL 参数 ?lang=zh-CN
	if lang := r.URL.Query().Get("lang"); lang != "" {
		if IsValidLang(lang) {
			return Lang(lang)
		}
	}

	// 2. Accept-Language Header
	if acceptLang := r.Header.Get("Accept-Language"); acceptLang != "" {
		return ParseAcceptLanguage(acceptLang)
	}

	// 3. 全局默认
	return GetGlobalLang()
}

// LangMiddleware 语言检测中间件
func LangMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := GetLangFromRequest(r)
		ctx := WithLang(r.Context(), lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
