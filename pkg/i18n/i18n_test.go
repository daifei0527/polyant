// pkg/i18n/i18n_test.go
package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTranslate(t *testing.T) {
	// 获取测试文件所在目录，然后定位 locales 目录
	_, thisFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(thisFile)
	localesDir := filepath.Join(testDir, "locales")

	// 初始化翻译器
	err := Init(localesDir, LangZhCN)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	tests := []struct {
		name     string
		code     string
		lang     Lang
		args     map[string]interface{}
		expected string
	}{
		{
			name:     "中文简单消息",
			code:     "common.success",
			lang:     LangZhCN,
			expected: "操作成功",
		},
		{
			name:     "英文简单消息",
			code:     "common.success",
			lang:     LangEnUS,
			expected: "Success",
		},
		{
			name:     "中文带参数消息",
			code:     "cli.entry.list_title",
			lang:     LangZhCN,
			args:     map[string]interface{}{"count": 10},
			expected: "条目列表 (共 10 条):",
		},
		{
			name:     "英文带参数消息",
			code:     "cli.entry.list_title",
			lang:     LangEnUS,
			args:     map[string]interface{}{"count": 10},
			expected: "Entry List (10 total):",
		},
		{
			name:     "不存在的消息码返回原始码",
			code:     "nonexistent.code",
			lang:     LangZhCN,
			expected: "nonexistent.code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Tc(tt.lang, tt.code, tt.args)
			if result != tt.expected {
				t.Errorf("Tc() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetLangFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		urlParam string
		header   string
		expected Lang
	}{
		{
			name:     "URL参数优先",
			urlParam: "en-US",
			header:   "zh-CN",
			expected: LangEnUS,
		},
		{
			name:     "Header解析英文",
			header:   "en-US,en;q=0.9",
			expected: LangEnUS,
		},
		{
			name:     "Header解析中文",
			header:   "zh-CN,zh;q=0.9,en;q=0.8",
			expected: LangZhCN,
		},
		{
			name:     "默认语言",
			expected: LangZhCN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?lang="+tt.urlParam, nil)
			if tt.header != "" {
				req.Header.Set("Accept-Language", tt.header)
			}

			result := GetLangFromRequest(req)
			if result != tt.expected {
				t.Errorf("GetLangFromRequest() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestContextLang(t *testing.T) {
	ctx := context.Background()
	ctx = WithLang(ctx, LangEnUS)

	result := GetLangFromContext(ctx)
	if result != LangEnUS {
		t.Errorf("GetLangFromContext() = %v, want %v", result, LangEnUS)
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected Lang
	}{
		{
			name:     "中文简体",
			header:   "zh-CN,zh;q=0.9,en;q=0.8",
			expected: LangZhCN,
		},
		{
			name:     "英文美国",
			header:   "en-US,en;q=0.9",
			expected: LangEnUS,
		},
		{
			name:     "英文通用",
			header:   "en",
			expected: LangEnUS,
		},
		{
			name:     "中文通用",
			header:   "zh",
			expected: LangZhCN,
		},
		{
			name:     "空值默认中文",
			header:   "",
			expected: LangZhCN,
		},
		{
			name:     "未知语言默认中文",
			header:   "fr-FR",
			expected: LangZhCN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAcceptLanguage(tt.header)
			if result != tt.expected {
				t.Errorf("ParseAcceptLanguage() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsValidLang(t *testing.T) {
	tests := []struct {
		lang     string
		expected bool
	}{
		{"zh-CN", true},
		{"en-US", true},
		{"fr-FR", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			result := IsValidLang(tt.lang)
			if result != tt.expected {
				t.Errorf("IsValidLang(%s) = %v, want %v", tt.lang, result, tt.expected)
			}
		})
	}
}
