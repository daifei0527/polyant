# Polyant 多语言支持实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Polyant 添加完整的多语言支持，包括 API、CLI、日志和数据模型

**Architecture:** 创建 `pkg/i18n/` 核心包，使用消息码 + 外部 JSON 翻译文件方案，与现有错误系统整合，扩展数据模型支持多语言字段

**Tech Stack:** Go 1.21+, text/template (模板变量替换), encoding/json

---

## 文件结构

### 新建文件

| 文件 | 职责 |
|------|------|
| `pkg/i18n/i18n.go` | 核心翻译引擎，语言检测，全局翻译器 |
| `pkg/i18n/codes.go` | 消息码常量定义 |
| `pkg/i18n/context.go` | 上下文语言传递，请求语言检测 |
| `pkg/i18n/locales/zh-CN.json` | 中文翻译文件 |
| `pkg/i18n/locales/en-US.json` | 英文翻译文件 |
| `pkg/i18n/i18n_test.go` | 单元测试 |

### 修改文件

| 文件 | 修改内容 |
|------|----------|
| `pkg/config/config.go` | 添加 I18nConfig 结构 |
| `pkg/errors/errors.go` | 添加 I18nCode 字段，新增 NewWithI18n 函数 |
| `pkg/logger/logger.go` | 支持消息码翻译，双语模式 |
| `internal/storage/model/models.go` | Entry/Category 添加多语言字段 |
| `internal/api/handler/helpers.go` | 添加多语言响应函数 |

---

## Task 1: 创建 i18n 核心包结构

**Files:**
- Create: `pkg/i18n/i18n.go`
- Create: `pkg/i18n/codes.go`
- Create: `pkg/i18n/context.go`

- [ ] **Step 1: 创建 pkg/i18n 目录**

```bash
mkdir -p /home/daifei/agentwiki/pkg/i18n/locales
```

Run: `mkdir -p pkg/i18n/locales`
Expected: 目录创建成功

- [ ] **Step 2: 创建消息码定义文件**

```go
// pkg/i18n/codes.go
package i18n

// 消息码命名规范: <模块>.<子模块>.<动作/状态>

// 通用消息码
const (
	CodeSuccess       = "common.success"
	CodeInvalidParams = "common.invalid_params"
	CodeInternalError = "common.internal_error"
	CodeNotFound      = "common.not_found"
)

// API 条目相关消息码
const (
	CodeEntryCreated    = "api.entry.created"
	CodeEntryUpdated    = "api.entry.updated"
	CodeEntryDeleted    = "api.entry.deleted"
	CodeEntryNotFound   = "api.entry.not_found"
	CodeEntryListLoaded = "api.entry.list_loaded"
)

// API 用户相关消息码
const (
	CodeUserRegistered  = "api.user.registered"
	CodeUserNotFound    = "api.user.not_found"
	CodeUserUpdated     = "api.user.updated"
	CodeUserInfoLoaded  = "api.user.info_loaded"
)

// API 认证相关消息码
const (
	CodeAuthMissing       = "api.auth.missing"
	CodeAuthInvalidSig    = "api.auth.invalid_signature"
	CodeAuthExpired       = "api.auth.expired"
	CodeAuthNoPermission  = "api.auth.permission_denied"
	CodeAuthUserSuspended = "api.auth.user_suspended"
)

// API 分类相关消息码
const (
	CodeCategoryCreated  = "api.category.created"
	CodeCategoryNotFound = "api.category.not_found"
	CodeCategoryList     = "api.category.list_loaded"
)

// API 搜索相关消息码
const (
	CodeSearchSuccess    = "api.search.success"
	CodeSearchKeywordShort = "api.search.keyword_too_short"
)

// CLI 消息码
const (
	CodeCLIEntryListTitle = "cli.entry.list_title"
	CodeCLIEntryNoResult  = "cli.entry.no_result"
	CodeCLIConfigSaved    = "cli.config.saved"
	CodeCLIKeyGenerated   = "cli.key.generated"
	CodeCLIUserListTitle  = "cli.user.list_title"
	CodeCLIServerStarted  = "cli.server.started"
	CodeCLIServerStopped  = "cli.server.stopped"
)

// 日志消息码
const (
	CodeLogServerStarted  = "log.server.started"
	CodeLogServerStopped  = "log.server.stopped"
	CodeLogDBConnected    = "log.db.connected"
	CodeLogDBError        = "log.db.error"
	CodeLogP2PConnected   = "log.p2p.connected"
	CodeLogP2PError       = "log.p2p.error"
	CodeLogSyncStarted    = "log.sync.started"
	CodeLogSyncCompleted  = "log.sync.completed"
	CodeLogSyncError      = "log.sync.error"
	CodeLogRequestIn      = "log.request.incoming"
	CodeLogRequestOut     = "log.request.outgoing"
)
```

Run: 创建 `pkg/i18n/codes.go`

- [ ] **Step 3: 创建核心翻译引擎**

```go
// pkg/i18n/i18n.go
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
var globalTranslator *Translator

// Init 初始化全局翻译器
func Init(localesDir string, defaultLang Lang) error {
	t, err := NewTranslator(localesDir, defaultLang)
	if err != nil {
		return err
	}
	globalTranslator = t
	return nil
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
	if globalTranslator == nil {
		return code
	}
	return globalTranslator.Translate(code, args...)
}

// Tc 带上下文的翻译
func Tc(lang Lang, code string, args ...map[string]interface{}) string {
	if globalTranslator == nil {
		return code
	}
	return globalTranslator.TranslateWithLang(lang, code, args...)
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
	if globalTranslator == nil {
		return LangZhCN
	}
	return globalTranslator.GetLang()
}

// SetGlobalLang 设置全局默认语言
func SetGlobalLang(lang Lang) {
	if globalTranslator != nil {
		globalTranslator.SetLang(lang)
	}
}
```

Run: 创建 `pkg/i18n/i18n.go`

- [ ] **Step 4: 创建上下文语言传递文件**

```go
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
```

Run: 创建 `pkg/i18n/context.go`

- [ ] **Step 5: 提交 i18n 核心包**

```bash
cd /home/daifei/agentwiki
git add pkg/i18n/
git commit -m "$(cat <<'EOF'
feat: add i18n core package

- Add message codes definition (codes.go)
- Add translator engine with template support (i18n.go)
- Add context language propagation (context.go)
- Support zh-CN and en-US locales

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: 创建翻译文件

**Files:**
- Create: `pkg/i18n/locales/zh-CN.json`
- Create: `pkg/i18n/locales/en-US.json`

- [ ] **Step 1: 创建中文翻译文件**

```json
{
  "common.success": "操作成功",
  "common.invalid_params": "参数无效",
  "common.internal_error": "内部错误",
  "common.not_found": "资源不存在",

  "api.entry.created": "条目创建成功",
  "api.entry.updated": "条目更新成功",
  "api.entry.deleted": "条目已删除",
  "api.entry.not_found": "条目不存在",
  "api.entry.list_loaded": "条目列表加载成功",

  "api.user.registered": "用户注册成功",
  "api.user.not_found": "用户不存在",
  "api.user.updated": "用户信息更新成功",
  "api.user.info_loaded": "用户信息加载成功",

  "api.auth.missing": "缺少认证信息",
  "api.auth.invalid_signature": "签名验证失败",
  "api.auth.expired": "请求已过期",
  "api.auth.permission_denied": "权限不足",
  "api.auth.user_suspended": "用户已被暂停",

  "api.category.created": "分类创建成功",
  "api.category.not_found": "分类不存在",
  "api.category.list_loaded": "分类列表加载成功",

  "api.search.success": "搜索完成",
  "api.search.keyword_too_short": "搜索关键词过短，至少需要2个字符",

  "cli.entry.list_title": "条目列表 (共 {{.count}} 条):",
  "cli.entry.no_result": "暂无条目",
  "cli.config.saved": "配置已保存",
  "cli.key.generated": "密钥对已生成",
  "cli.user.list_title": "用户列表 (共 {{.count}} 条):",
  "cli.server.started": "服务已启动",
  "cli.server.stopped": "服务已停止",

  "log.server.started": "服务启动完成，监听端口 {{.port}}",
  "log.server.stopped": "服务已停止",
  "log.db.connected": "数据库连接成功",
  "log.db.error": "数据库错误: {{.error}}",
  "log.p2p.connected": "P2P网络已连接，节点数: {{.count}}",
  "log.p2p.error": "P2P网络错误: {{.error}}",
  "log.sync.started": "同步任务开始",
  "log.sync.completed": "同步任务完成，同步条目数: {{.count}}",
  "log.sync.error": "同步错误: {{.error}}",
  "log.request.incoming": "收到请求: {{.method}} {{.path}}",
  "log.request.outgoing": "发送请求: {{.method}} {{.url}}"
}
```

Run: 创建 `pkg/i18n/locales/zh-CN.json`

- [ ] **Step 2: 创建英文翻译文件**

```json
{
  "common.success": "Success",
  "common.invalid_params": "Invalid parameters",
  "common.internal_error": "Internal error",
  "common.not_found": "Resource not found",

  "api.entry.created": "Entry created successfully",
  "api.entry.updated": "Entry updated successfully",
  "api.entry.deleted": "Entry deleted",
  "api.entry.not_found": "Entry not found",
  "api.entry.list_loaded": "Entry list loaded",

  "api.user.registered": "User registered successfully",
  "api.user.not_found": "User not found",
  "api.user.updated": "User updated successfully",
  "api.user.info_loaded": "User info loaded",

  "api.auth.missing": "Missing authentication info",
  "api.auth.invalid_signature": "Invalid signature",
  "api.auth.expired": "Request expired",
  "api.auth.permission_denied": "Permission denied",
  "api.auth.user_suspended": "User is suspended",

  "api.category.created": "Category created successfully",
  "api.category.not_found": "Category not found",
  "api.category.list_loaded": "Category list loaded",

  "api.search.success": "Search completed",
  "api.search.keyword_too_short": "Keyword too short, minimum 2 characters required",

  "cli.entry.list_title": "Entry List ({{.count}} total):",
  "cli.entry.no_result": "No entries found",
  "cli.config.saved": "Configuration saved",
  "cli.key.generated": "Key pair generated",
  "cli.user.list_title": "User List ({{.count}} total):",
  "cli.server.started": "Server started",
  "cli.server.stopped": "Server stopped",

  "log.server.started": "Server started, listening on port {{.port}}",
  "log.server.stopped": "Server stopped",
  "log.db.connected": "Database connected",
  "log.db.error": "Database error: {{.error}}",
  "log.p2p.connected": "P2P network connected, peers: {{.count}}",
  "log.p2p.error": "P2P network error: {{.error}}",
  "log.sync.started": "Sync started",
  "log.sync.completed": "Sync completed, entries: {{.count}}",
  "log.sync.error": "Sync error: {{.error}}",
  "log.request.incoming": "Incoming request: {{.method}} {{.path}}",
  "log.request.outgoing": "Outgoing request: {{.method}} {{.url}}"
}
```

Run: 创建 `pkg/i18n/locales/en-US.json`

- [ ] **Step 3: 提交翻译文件**

```bash
cd /home/daifei/agentwiki
git add pkg/i18n/locales/
git commit -m "$(cat <<'EOF'
feat: add zh-CN and en-US translation files

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: 扩展配置结构

**Files:**
- Modify: `pkg/config/config.go`

- [ ] **Step 1: 添加 I18nConfig 结构**

在 `pkg/config/config.go` 的 `StorageConfig` 结构之后添加:

```go
// I18nConfig 国际化配置
type I18nConfig struct {
	DefaultLang    string   `json:"default_lang"`    // 默认语言
	AvailableLangs []string `json:"available_langs"` // 可用语言列表
	LogBilingual   bool     `json:"log_bilingual"`   // 日志双语模式
}
```

- [ ] **Step 2: 更新 Config 结构体**

将 `Config` 结构体修改为:

```go
// Config 顶层配置结构体
// 包含所有子模块的配置
type Config struct {
	Node    NodeConfig    `json:"node"`
	Network NetworkConfig `json:"network"`
	Sync    SyncConfig    `json:"sync"`
	Sharing SharingConfig `json:"sharing"`
	User    UserConfig    `json:"user"`
	SMTP    SMTPConfig    `json:"smtp"`
	API     APIConfig     `json:"api"`
	Storage StorageConfig `json:"storage"`
	I18n    I18nConfig    `json:"i18n"`    // 新增
}
```

- [ ] **Step 3: 更新 DefaultConfig 函数**

在 `DefaultConfig()` 函数的 return 语句中添加 I18n 默认配置:

```go
// 在 DefaultConfig() 函数的 return &Config{...} 中添加:
		I18n: I18nConfig{
			DefaultLang:    "zh-CN",
			AvailableLangs: []string{"zh-CN", "en-US"},
			LogBilingual:   false,
		},
```

- [ ] **Step 4: 添加环境变量支持**

在 `LoadWithEnv` 函数末尾添加:

```go
	// I18n 配置环境变量
	if v := os.Getenv("POLYANT_I18N_DEFAULT_LANG"); v != "" {
		config.I18n.DefaultLang = v
	}
	if v := os.Getenv("POLYANT_I18N_LOG_BILINGUAL"); v != "" {
		config.I18n.LogBilingual = parseBool(v)
	}
```

- [ ] **Step 5: 提交配置更改**

```bash
cd /home/daifei/agentwiki
git add pkg/config/config.go
git commit -m "$(cat <<'EOF'
feat: add i18n configuration support

- Add I18nConfig struct with default_lang, available_langs, log_bilingual
- Add environment variable support for i18n settings

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 扩展错误系统

**Files:**
- Modify: `pkg/errors/errors.go`

- [ ] **Step 1: 添加 I18nCode 字段到 AWError**

修改 `AWError` 结构体:

```go
// AWError Polyant 统一错误类型
type AWError struct {
	Code       int           `json:"code"`
	Category   ErrorCategory `json:"-"`
	Message    string        `json:"message"`      // 默认英文消息
	I18nCode   string        `json:"i18n_code"`    // i18n 消息码
	HTTPStatus int           `json:"-"`
	Cause      error         `json:"-"`
	Retryable  bool          `json:"-"`
}
```

- [ ] **Step 2: 添加 NewWithI18n 函数**

在 `Wrap` 函数之后添加:

```go
// NewWithI18n 创建带多语言支持的错误
func NewWithI18n(code int, category ErrorCategory, i18nCode, defaultMessage string, httpStatus int) *AWError {
	return &AWError{
		Code:       code,
		Category:   category,
		Message:    defaultMessage,
		I18nCode:   i18nCode,
		HTTPStatus: httpStatus,
	}
}

// WrapWithI18n 创建带多语言支持和底层错误的错误
func WrapWithI18n(code int, category ErrorCategory, i18nCode, defaultMessage string, httpStatus int, cause error) *AWError {
	return &AWError{
		Code:       code,
		Category:   category,
		Message:    defaultMessage,
		I18nCode:   i18nCode,
		HTTPStatus: httpStatus,
		Cause:      cause,
	}
}
```

- [ ] **Step 3: 更新预定义错误**

更新错误定义，添加 I18nCode:

```go
// 预定义错误
var (
	// 系统错误 (0xxxx)
	ErrInternal    = NewWithI18n(0, CategorySystem, "common.internal_error", "internal error", 500)
	ErrUnavailable = NewWithI18n(1, CategorySystem, "common.internal_error", "service unavailable", 503)
	ErrRateLimited = NewWithI18n(2, CategorySystem, "common.internal_error", "rate limited", 429)

	// API错误 (1xxxx)
	ErrInvalidParams    = NewWithI18n(100, CategoryAPI, "common.invalid_params", "invalid params", 400)
	ErrJSONParse        = NewWithI18n(102, CategoryAPI, "common.invalid_params", "json parse failed", 400)
	ErrScoreOutOfRange  = NewWithI18n(103, CategoryAPI, "common.invalid_params", "score must be between 1.0 and 5.0", 400)

	// 认证错误 (2xxxx)
	ErrMissingAuth      = NewWithI18n(200, CategoryAuth, "api.auth.missing", "missing auth info", 401)
	ErrInvalidSignature = NewWithI18n(201, CategoryAuth, "api.auth.invalid_signature", "invalid signature", 401)
	ErrTimestampExpired = NewWithI18n(202, CategoryAuth, "api.auth.expired", "timestamp expired", 401)
	ErrPermissionDenied = NewWithI18n(203, CategoryAuth, "api.auth.permission_denied", "permission denied", 403)
	ErrBasicUserDenied  = NewWithI18n(204, CategoryAuth, "api.auth.permission_denied", "basic user cannot perform this action", 403)
	ErrUserSuspended    = NewWithI18n(205, CategoryAuth, "api.auth.user_suspended", "user is suspended", 403)

	// 存储错误 (3xxxx)
	ErrEntryNotFound    = NewWithI18n(300, CategoryStorage, "api.entry.not_found", "entry not found", 404)
	ErrUserNotFound     = NewWithI18n(301, CategoryStorage, "api.user.not_found", "user not found", 404)
	ErrCategoryNotFound = NewWithI18n(302, CategoryStorage, "api.category.not_found", "category not found", 404)
	ErrDuplicateRating  = NewWithI18n(303, CategoryStorage, "common.internal_error", "duplicate rating", 409)
	ErrEntryExists      = NewWithI18n(304, CategoryStorage, "common.internal_error", "entry already exists", 409)
	ErrWriteFailed      = NewWithI18n(305, CategoryStorage, "common.internal_error", "storage write failed", 500)

	// 网络错误 (4xxxx)
	ErrPeerConnectFailed = NewWithI18n(400, CategoryNetwork, "common.internal_error", "peer connect failed", 502)

	// 同步错误 (5xxxx)
	ErrSyncFailed   = NewWithI18n(500, CategorySync, "common.internal_error", "sync failed", 500)
	ErrHashMismatch = NewWithI18n(502, CategorySync, "common.internal_error", "hash mismatch", 500)

	// 搜索错误 (6xxxx)
	ErrSearchFailed    = NewWithI18n(600, CategorySearch, "common.internal_error", "search failed", 500)
	ErrKeywordTooShort = NewWithI18n(601, CategorySearch, "api.search.keyword_too_short", "keyword too short", 400)

	// 评分错误 (7xxxx)
	ErrRatingNotFound = NewWithI18n(700, CategoryRating, "common.not_found", "rating not found", 404)

	// 用户错误 (8xxxx)
	ErrUserAlreadyExists   = NewWithI18n(800, CategoryUser, "common.internal_error", "user already exists", 409)
	ErrEmailNotVerified    = NewWithI18n(801, CategoryUser, "common.invalid_params", "email not verified", 403)
	ErrInvalidEmailToken   = NewWithI18n(802, CategoryUser, "common.invalid_params", "invalid email token", 400)
	ErrVerificationExpired = NewWithI18n(803, CategoryUser, "common.invalid_params", "verification code expired", 400)
	ErrVerificationSent    = NewWithI18n(804, CategoryUser, "common.internal_error", "verification code already sent, please check your email", 429)
	ErrEmailAlreadyUsed    = NewWithI18n(805, CategoryUser, "common.internal_error", "email already used by another user", 409)
)
```

- [ ] **Step 4: 提交错误系统更改**

```bash
cd /home/daifei/agentwiki
git add pkg/errors/errors.go
git commit -m "$(cat <<'EOF'
feat: add i18n support to error system

- Add I18nCode field to AWError struct
- Add NewWithI18n and WrapWithI18n functions
- Update all predefined errors with i18n codes

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 扩展数据模型

**Files:**
- Modify: `internal/storage/model/models.go`

- [ ] **Step 1: 扩展 KnowledgeEntry 结构体**

修改 `KnowledgeEntry` 结构体，添加多语言字段:

```go
// KnowledgeEntry 表示一条知识条目
type KnowledgeEntry struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`     // Markdown格式内容
	JSONData    []map[string]interface{} `json:"jsonData"`  // 结构化JSON数据
	Category    string                 `json:"category"`    // 所属分类路径
	Tags        []string               `json:"tags"`        // 标签列表
	Version     int64                  `json:"version"`     // 版本号
	CreatedAt   int64                  `json:"createdAt"`   // 创建时间(Unix时间戳)
	UpdatedAt   int64                  `json:"updatedAt"`   // 更新时间
	CreatedBy   string                 `json:"createdBy"`   // 创建者公钥
	Score       float64                `json:"score"`       // 加权平均评分
	ScoreCount  int32                  `json:"scoreCount"`  // 评分数量
	ContentHash string                 `json:"contentHash"` // 内容哈希
	Status      string                 `json:"status"`      // 条目状态
	License     string                 `json:"license"`     // 许可证
	SourceRef   string                 `json:"sourceRef"`   // 来源引用
	// 多语言支持
	Lang        string                 `json:"lang,omitempty"`        // 条目主语言
	TitleI18n   map[string]string      `json:"titleI18n,omitempty"`   // 多语言标题 {"zh-CN": "标题", "en-US": "Title"}
	ContentI18n map[string]string      `json:"contentI18n,omitempty"` // 多语言内容
}
```

- [ ] **Step 2: 扩展 Category 结构体**

修改 `Category` 结构体，添加多语言字段:

```go
// Category 表示知识分类
type Category struct {
	ID          string `json:"id"`          // 分类唯一ID
	Path        string `json:"path"`        // 分类路径(如 "tech/programming")
	Name        string `json:"name"`        // 分类名称
	ParentId    string `json:"parentId"`    // 父分类ID
	Level       int32  `json:"level"`       // 层级深度
	SortOrder   int32  `json:"sortOrder"`   // 排序顺序
	IsBuiltin   bool   `json:"isBuiltin"`   // 是否为内置分类
	MaintainedBy string `json:"maintainedBy"` // 维护者公钥
	CreatedAt   int64  `json:"createdAt"`   // 创建时间
	// 多语言支持
	NameI18n    map[string]string `json:"nameI18n,omitempty"`    // 多语言名称
	DescI18n    map[string]string `json:"descI18n,omitempty"`    // 多语言描述
}
```

- [ ] **Step 3: 添加获取本地化标题方法**

在 `KnowledgeEntry` 结构体后添加方法:

```go
// GetTitleByLang 根据语言获取标题
func (e *KnowledgeEntry) GetTitleByLang(lang string) string {
	if e.TitleI18n != nil {
		if title, ok := e.TitleI18n[lang]; ok {
			return title
		}
	}
	return e.Title
}

// GetContentByLang 根据语言获取内容
func (e *KnowledgeEntry) GetContentByLang(lang string) string {
	if e.ContentI18n != nil {
		if content, ok := e.ContentI18n[lang]; ok {
			return content
		}
	}
	return e.Content
}
```

- [ ] **Step 4: 添加获取本地化名称方法**

在 `Category` 结构体后添加方法:

```go
// GetNameByLang 根据语言获取分类名称
func (c *Category) GetNameByLang(lang string) string {
	if c.NameI18n != nil {
		if name, ok := c.NameI18n[lang]; ok {
			return name
		}
	}
	return c.Name
}
```

- [ ] **Step 5: 提交数据模型更改**

```bash
cd /home/daifei/agentwiki
git add internal/storage/model/models.go
git commit -m "$(cat <<'EOF'
feat: add i18n fields to data models

- Add Lang, TitleI18n, ContentI18n to KnowledgeEntry
- Add NameI18n, DescI18n to Category
- Add GetTitleByLang, GetContentByLang, GetNameByLang methods

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 改造 API Handler

**Files:**
- Modify: `internal/api/handler/helpers.go`

- [ ] **Step 1: 添加多语言响应函数**

在 `writeError` 函数后添加:

```go
import (
	// 在现有 import 中添加
	"github.com/daifei0527/polyant/pkg/i18n"
)

// writeSuccess 写入成功响应（带多语言支持）
func writeSuccess(w http.ResponseWriter, r *http.Request, data interface{}) {
	lang := i18n.GetLangFromRequest(r)
	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: i18n.Tc(lang, "common.success"),
		Data:    data,
	})
}

// writeSuccessWithCode 写入成功响应（指定消息码）
func writeSuccessWithCode(w http.ResponseWriter, r *http.Request, code string, data interface{}) {
	lang := i18n.GetLangFromRequest(r)
	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: i18n.Tc(lang, code),
		Data:    data,
	})
}

// writeErrorI18n 写入错误响应（带多语言支持）
func writeErrorI18n(w http.ResponseWriter, r *http.Request, err *awerrors.AWError) {
	lang := i18n.GetLangFromRequest(r)
	
	message := err.Message
	if err.I18nCode != "" {
		if translated := i18n.Tc(lang, err.I18nCode); translated != err.I18nCode {
			message = translated
		}
	}
	
	status := err.HTTPStatus
	if status == 0 {
		status = http.StatusInternalServerError
	}
	
	writeJSON(w, status, &APIResponse{
		Code:    err.Code,
		Message: message,
		Data:    nil,
	})
}
```

- [ ] **Step 2: 提交 Handler 更改**

```bash
cd /home/daifei/agentwiki
git add internal/api/handler/helpers.go
git commit -m "$(cat <<'EOF'
feat: add i18n support to API response helpers

- Add writeSuccess, writeSuccessWithCode, writeErrorI18n functions
- Auto-detect language from request

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: 改造日志系统

**Files:**
- Modify: `pkg/logger/logger.go`

- [ ] **Step 1: 添加 i18n 相关字段**

在 `Logger` 结构体中添加字段:

```go
import (
	// 在现有 import 中添加
	"github.com/daifei0527/polyant/pkg/i18n"
)

// Logger 自定义日志结构体
type Logger struct {
	level      int
	filePath   string
	maxSize    int64
	maxBackups int
	mu         sync.Mutex
	file       *os.File
	fileSize   int64
	logger     *log.Logger
	// 新增: 多语言支持
	lang      i18n.Lang
	bilingual bool
}
```

- [ ] **Step 2: 更新 LoggerConfig 结构体**

```go
// LoggerConfig 日志配置结构体
type LoggerConfig struct {
	Level      int    // 日志级别：0=Debug, 1=Info, 2=Warn, 3=Error
	FilePath   string // 日志文件路径，为空则仅输出到 stdout
	MaxSizeMB  int    // 单个日志文件最大大小（MB）
	MaxBackups int    // 保留的备份日志文件数量
	// 新增
	Lang      string // 日志语言
	Bilingual bool   // 是否双语模式
}
```

- [ ] **Step 3: 更新 NewLogger 函数**

修改 `NewLogger` 函数以支持新字段:

```go
// NewLogger 根据配置创建新的 Logger 实例
func NewLogger(config *LoggerConfig) *Logger {
	if config == nil {
		config = &LoggerConfig{
			Level: LevelInfo,
		}
	}

	// 解析语言
	lang := i18n.LangZhCN
	if config.Lang != "" {
		lang = i18n.Lang(config.Lang)
	}

	l := &Logger{
		level:      config.Level,
		filePath:   config.FilePath,
		maxSize:    int64(config.MaxSizeMB) * 1024 * 1024,
		maxBackups: config.MaxBackups,
		lang:       lang,
		bilingual:  config.Bilingual,
	}

	// ... 其余代码保持不变
}
```

- [ ] **Step 4: 添加新的日志方法**

在现有 `Error` 方法后添加:

```go
// InfoI18n 输出信息级别的日志（带多语言支持）
func (l *Logger) InfoI18n(code string, args map[string]interface{}) {
	l.logI18n(LevelInfo, code, args)
}

// WarnI18n 输出警告级别的日志（带多语言支持）
func (l *Logger) WarnI18n(code string, args map[string]interface{}) {
	l.logI18n(LevelWarn, code, args)
}

// ErrorI18n 输出错误级别的日志（带多语言支持）
func (l *Logger) ErrorI18n(code string, args map[string]interface{}) {
	l.logI18n(LevelError, code, args)
}

// logI18n 内部日志方法（带多语言支持）
func (l *Logger) logI18n(level int, code string, args map[string]interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotate(); err != nil {
		fmt.Fprintf(os.Stderr, "日志轮转失败: %v\n", err)
	}

	levelName := "UNKNOWN"
	if name, ok := levelNames[level]; ok {
		levelName = name
	}

	if l.bilingual {
		msgZh := i18n.Tc(i18n.LangZhCN, code, args)
		msgEn := i18n.Tc(i18n.LangEnUS, code, args)
		l.logger.Printf("[%s] %s | %s", levelName, msgZh, msgEn)
	} else {
		msg := i18n.Tc(l.lang, code, args)
		l.logger.Printf("[%s] %s", levelName, msg)
	}
}
```

- [ ] **Step 5: 提交日志系统更改**

```bash
cd /home/daifei/agentwiki
git add pkg/logger/logger.go
git commit -m "$(cat <<'EOF'
feat: add i18n support to logger

- Add lang and bilingual fields to Logger
- Add InfoI18n, WarnI18n, ErrorI18n methods
- Support bilingual log output mode

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: 改造 CLI 输出

**Files:**
- Modify: `cmd/awctl/entry.go`
- Modify: `cmd/awctl/root.go`

- [ ] **Step 1: 在 root.go 添加全局 lang 参数**

在 `cmd/awctl/root.go` 中添加:

```go
import (
	"github.com/daifei0527/polyant/pkg/i18n"
)

var (
	// 全局语言参数
	langFlag string
)

func init() {
	// 在 rootCmd 的初始化中添加
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "zh-CN", "Output language (zh-CN, en-US)")
}
```

- [ ] **Step 2: 更新 entry.go 使用多语言**

修改 `entryListCmd`:

```go
import (
	"github.com/daifei0527/polyant/pkg/i18n"
)

var entryListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("cli.entry.list_short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		lang := i18n.Lang(langFlag)
		category, _ := cmd.Flags().GetString("category")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		jsonOut, _ := cmd.Flags().GetBool("json")

		// ... 现有逻辑 ...

		if len(entries) == 0 {
			fmt.Println(i18n.Tc(lang, "cli.entry.no_result"))
			return nil
		}

		fmt.Println(i18n.Tc(lang, "cli.entry.list_title", map[string]interface{}{"count": total}))
		// ... 其余输出 ...
	},
}
```

- [ ] **Step 3: 提交 CLI 更改**

```bash
cd /home/daifei/agentwiki
git add cmd/awctl/entry.go cmd/awctl/root.go
git commit -m "$(cat <<'EOF'
feat: add i18n support to CLI

- Add --lang global flag
- Update entry list command with i18n output

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: 添加单元测试

**Files:**
- Create: `pkg/i18n/i18n_test.go`

- [ ] **Step 1: 创建测试文件**

```go
// pkg/i18n/i18n_test.go
package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranslate(t *testing.T) {
	// 初始化翻译器
	err := Init("locales", LangZhCN)
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
		name       string
		urlParam   string
		header     string
		expected   Lang
	}{
		{
			name:     "URL参数优先",
			urlParam: "en-US",
			header:   "zh-CN",
			expected: LangEnUS,
		},
		{
			name:     "Header解析",
			header:   "en-US,en;q=0.9",
			expected: LangEnUS,
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
			name:     "中文",
			header:   "zh-CN,zh;q=0.9,en;q=0.8",
			expected: LangZhCN,
		},
		{
			name:     "英文",
			header:   "en-US,en;q=0.9",
			expected: LangEnUS,
		},
		{
			name:     "空值默认中文",
			header:   "",
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
```

- [ ] **Step 2: 运行测试**

```bash
cd /home/daifei/agentwiki
go test -v ./pkg/i18n/...
```

Expected: 所有测试通过

- [ ] **Step 3: 提交测试文件**

```bash
cd /home/daifei/agentwiki
git add pkg/i18n/i18n_test.go
git commit -m "$(cat <<'EOF'
test: add i18n package unit tests

- Test translation with different languages
- Test language detection from request
- Test context language propagation

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: 集成测试与文档

**Files:**
- Modify: `cmd/polyant/main.go`
- Modify: `configs/config.json`

- [ ] **Step 1: 在 main.go 初始化 i18n**

在 `cmd/polyant/main.go` 的 main 函数中，配置加载后添加:

```go
import (
	"github.com/daifei0527/polyant/pkg/i18n"
)

func main() {
	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化 i18n
	if err := i18n.Init("pkg/i18n/locales", i18n.Lang(cfg.I18n.DefaultLang)); err != nil {
		log.Printf("警告: i18n初始化失败: %v", err)
	}

	// ... 其余代码
}
```

- [ ] **Step 2: 更新配置文件示例**

在 `configs/config.json` 添加:

```json
{
  "node": { ... },
  "i18n": {
    "default_lang": "zh-CN",
    "available_langs": ["zh-CN", "en-US"],
    "log_bilingual": false
  }
}
```

- [ ] **Step 3: 运行完整测试**

```bash
cd /home/daifei/agentwiki
go test -v ./...
make build
```

Expected: 编译成功，测试通过

- [ ] **Step 4: 最终提交**

```bash
cd /home/daifei/agentwiki
git add .
git commit -m "$(cat <<'EOF'
feat: complete i18n implementation

- Integrate i18n initialization in main
- Update config example
- All components support multi-language

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## 实现摘要

| Task | 描述 | 文件数 |
|------|------|--------|
| 1 | 创建 i18n 核心包 | 3 新建 |
| 2 | 创建翻译文件 | 2 新建 |
| 3 | 扩展配置结构 | 1 修改 |
| 4 | 扩展错误系统 | 1 修改 |
| 5 | 扩展数据模型 | 1 修改 |
| 6 | 改造 API Handler | 1 修改 |
| 7 | 改造日志系统 | 1 修改 |
| 8 | 改造 CLI 输出 | 2 修改 |
| 9 | 添加单元测试 | 1 新建 |
| 10 | 集成测试 | 2 修改 |

**总计**: 6 个新建文件，10 个修改文件
