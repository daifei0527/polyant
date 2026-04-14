// Package i18n 提供国际化支持，包括消息翻译、语言检测和上下文管理。
// 支持模板变量替换和 HTTP 中间件集成。
package i18n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

// Lang 语言类型
type Lang string

const (
	// LangZhCN 中文简体
	LangZhCN Lang = "zh-CN"
	// LangEnUS 英文
	LangEnUS Lang = "en-US"
)

// SupportedLangs 支持的语言列表
var SupportedLangs = []Lang{LangZhCN, LangEnUS}

// Translator 翻译器
type Translator struct {
	mu       sync.RWMutex
	locales  map[Lang]map[string]string
	lang     Lang
	loaddir  string
}

// globalTranslator 全局翻译器实例
var (
	globalTranslator *Translator
	globalOnce       sync.Once
	globalInitErr    error
	globalMu         sync.RWMutex
)

// Init 初始化全局翻译器（线程安全，只会初始化一次）
func Init(localesDir string, defaultLang Lang) error {
	globalOnce.Do(func() {
		globalTranslator, globalInitErr = NewTranslator(localesDir, defaultLang)
	})
	return globalInitErr
}

// NewTranslator 创建新的翻译器
func NewTranslator(localesDir string, defaultLang Lang) (*Translator, error) {
	t := &Translator{
		locales: make(map[Lang]map[string]string),
		lang:    defaultLang,
		loaddir: localesDir,
	}

	// 加载所有支持的语言文件
	for _, lang := range SupportedLangs {
		if err := t.loadLocale(lang); err != nil {
			return nil, fmt.Errorf("load locale %s: %w", lang, err)
		}
	}

	return t, nil
}

// loadLocale 加载语言文件
func (t *Translator) loadLocale(lang Lang) error {
	filename := fmt.Sprintf("%s.json", lang)
	path := filepath.Join(t.loaddir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}

	t.mu.Lock()
	t.locales[lang] = messages
	t.mu.Unlock()

	return nil
}

// T 翻译消息（使用默认语言）
func T(code string, args ...map[string]interface{}) string {
	globalMu.RLock()
	t := globalTranslator
	globalMu.RUnlock()

	if t == nil {
		return code
	}
	return t.Translate(code, args...)
}

// Tc 带上下文的翻译
func Tc(lang Lang, code string, args ...map[string]interface{}) string {
	globalMu.RLock()
	t := globalTranslator
	globalMu.RUnlock()

	if t == nil {
		return code
	}
	return t.TranslateWithLang(lang, code, args...)
}

// Translate 翻译消息
func (t *Translator) Translate(code string, args ...map[string]interface{}) string {
	return t.TranslateWithLang(t.lang, code, args...)
}

// TranslateWithLang 指定语言翻译
func (t *Translator) TranslateWithLang(lang Lang, code string, args ...map[string]interface{}) string {
	t.mu.RLock()
	locale, ok := t.locales[lang]
	t.mu.RUnlock()

	if !ok {
		return code
	}

	msg, ok := locale[code]
	if !ok {
		return code
	}

	// 如果有参数，进行模板替换
	if len(args) > 0 && len(args[0]) > 0 {
		return renderTemplate(msg, args[0])
	}

	return msg
}

// SetLang 设置默认语言
func (t *Translator) SetLang(lang Lang) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lang = lang
}

// GetLang 获取当前默认语言
func (t *Translator) GetLang() Lang {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lang
}

// renderTemplate 渲染模板字符串
// 支持 {{.key}} 格式的变量替换
func renderTemplate(tmpl string, args map[string]interface{}) string {
	t, err := template.New("msg").Parse(tmpl)
	if err != nil {
		return tmpl
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, args); err != nil {
		return tmpl
	}

	return buf.String()
}

// IsValidLang 检查语言是否有效
func IsValidLang(lang string) bool {
	for _, l := range SupportedLangs {
		if string(l) == lang {
			return true
		}
	}
	return false
}

// ParseAcceptLanguage 解析 Accept-Language 头
// 返回优先级最高的支持语言
func ParseAcceptLanguage(header string) Lang {
	if header == "" {
		return LangZhCN
	}

	// 简单解析，取第一个语言
	parts := strings.Split(header, ",")
	if len(parts) == 0 {
		return LangZhCN
	}

	// 提取语言代码
	langPart := strings.TrimSpace(parts[0])
	langPart = strings.Split(langPart, ";")[0]
	langPart = strings.TrimSpace(langPart)

	// 标准化
	switch {
	case strings.HasPrefix(langPart, "zh"):
		return LangZhCN
	case strings.HasPrefix(langPart, "en"):
		return LangEnUS
	default:
		return LangZhCN
	}
}

// GetGlobalLang 获取全局默认语言
func GetGlobalLang() Lang {
	globalMu.RLock()
	t := globalTranslator
	globalMu.RUnlock()

	if t == nil {
		return LangZhCN
	}
	return t.GetLang()
}

// SetGlobalLang 设置全局默认语言
func SetGlobalLang(lang Lang) {
	globalMu.RLock()
	t := globalTranslator
	globalMu.RUnlock()

	if t != nil {
		t.SetLang(lang)
	}
}
