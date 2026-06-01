# 统一智能体技能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建统一的智能体技能系统，支持 Codex、Hermes、OpenClaw 和 MCP 服务器，使 Polyant 知识库能够被多种 AI 智能体访问。

**Architecture:** 混合架构 — 共享 Go SDK (`pkg/polysdk`) 封装 API 调用，agentskills.io 标准技能供 Codex/Hermes 使用，OpenClaw 专用技能单独适配，MCP 服务器作为通用集成层。

**Tech Stack:** Go 1.25+, agentskills.io 标准, MCP Protocol, Ed25519 认证

---

## 文件结构总览

```
pkg/polysdk/                      # 共享 SDK
├── client.go                     # HTTP 客户端（复用 pactl/client.go 模式）
├── client_test.go                # 客户端测试
├── types.go                      # 数据类型定义
├── errors.go                     # 错误类型
└── config.go                     # 配置加载

skills/agentskills/               # agentskills.io 标准技能
├── polyant-search/
│   ├── SKILL.md
│   └── scripts/search.sh
├── polyant-save/
│   ├── SKILL.md
│   └── scripts/save.sh
├── polyant-learn/
│   ├── SKILL.md
│   └── scripts/learn.sh
├── polyant-rate/
│   ├── SKILL.md
│   └── scripts/rate.sh
└── polyant-config/
    ├── SKILL.md
    └── scripts/config.sh

skills/openclaw/                  # OpenClaw 专用技能
├── polyant-search.md
├── polyant-save.md
├── polyant-learn.md
├── polyant-rate.md
└── polyant-config.md

cmd/polyant-mcp-server/           # MCP 服务器
├── main.go
├── server.go
├── server_test.go
├── tools/
│   ├── search.go
│   ├── create.go
│   ├── update.go
│   ├── rate.go
│   ├── get.go
│   └── list.go
└── config.go

scripts/
└── install-unified.sh            # 统一安装脚本
```

---

## 阶段 1：共享 SDK (`pkg/polysdk`)

### Task 1: 创建 SDK 目录和类型定义

**Files:**
- Create: `pkg/polysdk/types.go`
- Create: `pkg/polysdk/errors.go`

- [ ] **Step 1: 创建 types.go**

```go
// pkg/polysdk/types.go
package polysdk

import "time"

// Entry 知识条目
type Entry struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Category   string    `json:"category"`
	Tags       []string  `json:"tags,omitempty"`
	Score      float64   `json:"score"`
	ScoreCount int       `json:"score_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	CreatedBy  string    `json:"created_by"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Entries    []Entry `json:"items"`
	TotalCount int     `json:"total_count"`
}

// CreateEntryRequest 创建条目请求
type CreateEntryRequest struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Category string   `json:"category"`
	Tags     []string `json:"tags,omitempty"`
}

// UpdateEntryRequest 更新条目请求
type UpdateEntryRequest struct {
	Title    string   `json:"title,omitempty"`
	Content  string   `json:"content,omitempty"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// RatingRequest 评分请求
type RatingRequest struct {
	Score   float64 `json:"score"`
	Comment string  `json:"comment,omitempty"`
}

// Category 分类信息
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
}

// APIResponse 标准 API 响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// APIError API 错误响应
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
```

- [ ] **Step 2: 创建 errors.go**

```go
// pkg/polysdk/errors.go
package polysdk

import "fmt"

// Error Polyant SDK 错误
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("polyant error %d: %s", e.Code, e.Message)
}

// NewError 创建错误
func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// IsNotFoundError 判断是否为未找到错误
func IsNotFoundError(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == 404
	}
	return false
}

// IsAuthError 判断是否为认证错误
func IsAuthError(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == 401 || e.Code == 403
	}
	return false
}
```

- [ ] **Step 3: 提交**

```bash
git add pkg/polysdk/types.go pkg/polysdk/errors.go
git commit -m "feat(polysdk): add types and errors definitions"
```

---

### Task 2: 实现 HTTP 客户端

**Files:**
- Create: `pkg/polysdk/client.go`
- Create: `pkg/polysdk/client_test.go`

- [ ] **Step 1: 创建 client.go**

```go
// pkg/polysdk/client.go
package polysdk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client Polyant API 客户端
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	publicKey  []byte
	privateKey []byte
}

// NewClient 创建新的 API 客户端
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetAPIKey 设置 API Key（用于公开路由）
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// SetKeys 设置 Ed25519 密钥对（用于认证路由）
func (c *Client) SetKeys(publicKey, privateKey []byte) {
	c.publicKey = publicKey
	c.privateKey = privateKey
}

// HasKeys 检查是否已设置密钥
func (c *Client) HasKeys() bool {
	return len(c.publicKey) > 0 && len(c.privateKey) > 0
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody []byte
	var bodyReader io.Reader

	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(reqBody)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 设置 API Key
	if c.apiKey != "" {
		req.Header.Set("X-Polyant-Api-Key", c.apiKey)
	}

	// 设置 Ed25519 认证头
	if c.HasKeys() {
		if err := c.setAuthHeaders(req, reqBody); err != nil {
			return fmt.Errorf("set auth headers: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp APIError
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Code > 0 {
			return &Error{Code: errResp.Code, Message: errResp.Message}
		}
		return &Error{Code: resp.StatusCode, Message: string(respBody)}
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// setAuthHeaders 设置 Ed25519 认证头
func (c *Client) setAuthHeaders(req *http.Request, body []byte) error {
	// 使用内部 auth 包进行签名
	// 这里简化实现，实际应调用 internal/auth/ed25519
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	bodyHash := sha256.Sum256(body)
	signContent := fmt.Sprintf("%s\n%s\n%s\n%s",
		req.Method, req.URL.Path, timestamp, hex.EncodeToString(bodyHash[:]))

	// TODO: 使用 Ed25519 签名
	_ = signContent

	req.Header.Set("X-Polyant-PublicKey", base64.StdEncoding.EncodeToString(c.publicKey))
	req.Header.Set("X-Polyant-Timestamp", timestamp)
	// req.Header.Set("X-Polyant-Signature", signature)

	return nil
}

// ========== 搜索 API ==========

// Search 搜索知识库
func (c *Client) Search(ctx context.Context, query string, category string, tags []string, limit int) (*SearchResult, error) {
	path := fmt.Sprintf("/api/v1/search?q=%s&limit=%d", query, limit)
	if category != "" {
		path += "&category=" + category
	}
	if len(tags) > 0 {
		path += "&tags=" + strings.Join(tags, ",")
	}

	var resp APIResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	// 解析响应数据
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal response data: %w", err)
	}

	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal search result: %w", err)
	}

	return &result, nil
}

// ========== 条目 API ==========

// GetEntry 获取条目详情
func (c *Client) GetEntry(ctx context.Context, id string) (*Entry, error) {
	var resp APIResponse
	if err := c.doRequest(ctx, http.MethodGet, "/api/v1/entry/"+id, nil, &resp); err != nil {
		return nil, err
	}

	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal response data: %w", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("unmarshal entry: %w", err)
	}

	return &entry, nil
}

// CreateEntry 创建条目
func (c *Client) CreateEntry(ctx context.Context, req *CreateEntryRequest) (*Entry, error) {
	var resp APIResponse
	if err := c.doRequest(ctx, http.MethodPost, "/api/v1/entry/create", req, &resp); err != nil {
		return nil, err
	}

	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal response data: %w", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("unmarshal entry: %w", err)
	}

	return &entry, nil
}

// UpdateEntry 更新条目
func (c *Client) UpdateEntry(ctx context.Context, id string, req *UpdateEntryRequest) (*Entry, error) {
	var resp APIResponse
	path := fmt.Sprintf("/api/v1/entry/update/%s", id)
	if err := c.doRequest(ctx, http.MethodPut, path, req, &resp); err != nil {
		return nil, err
	}

	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal response data: %w", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("unmarshal entry: %w", err)
	}

	return &entry, nil
}

// DeleteEntry 删除条目
func (c *Client) DeleteEntry(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/entry/delete/%s", id)
	var resp APIResponse
	return c.doRequest(ctx, http.MethodDelete, path, nil, &resp)
}

// RateEntry 为条目评分
func (c *Client) RateEntry(ctx context.Context, id string, score float64, comment string) error {
	req := &RatingRequest{Score: score, Comment: comment}
	path := fmt.Sprintf("/api/v1/entry/rate/%s", id)
	var resp APIResponse
	return c.doRequest(ctx, http.MethodPost, path, req, &resp)
}

// ========== 分类 API ==========

// ListCategories 列出分类
func (c *Client) ListCategories(ctx context.Context) ([]Category, error) {
	var resp APIResponse
	if err := c.doRequest(ctx, http.MethodGet, "/api/v1/categories", nil, &resp); err != nil {
		return nil, err
	}

	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal response data: %w", err)
	}

	var result struct {
		Items []Category `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal categories: %w", err)
	}

	return result.Items, nil
}
```

- [ ] **Step 2: 创建 client_test.go**

```go
// pkg/polysdk/client_test.go
package polysdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8080")
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8080", client.baseURL)
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	client := NewClient("http://localhost:8080/")
	assert.Equal(t, "http://localhost:8080", client.baseURL)
}

func TestSetAPIKey(t *testing.T) {
	client := NewClient("http://localhost:8080")
	client.SetAPIKey("test-key")
	assert.Equal(t, "test-key", client.apiKey)
}

func TestSearch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/search")
		assert.Equal(t, "test query", r.URL.Query().Get("q"))

		resp := APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"total_count": 1,
				"items": []map[string]interface{}{
					{
						"id":       "entry-1",
						"title":    "Test Entry",
						"category": "test",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Search(context.Background(), "test query", "", nil, 10)

	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, "entry-1", result.Entries[0].ID)
}

func TestSearch_WithCategoryAndTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "go", r.URL.Query().Get("category"))
		assert.Equal(t, "error,compile", r.URL.Query().Get("tags"))

		resp := APIResponse{Code: 0, Data: map[string]interface{}{"total_count": 0, "items": []interface{}{}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Search(context.Background(), "test", "go", []string{"error", "compile"}, 10)
	require.NoError(t, err)
}

func TestGetEntry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/entry/entry-1")

		resp := APIResponse{
			Code: 0,
			Data: map[string]interface{}{
				"id":       "entry-1",
				"title":    "Test Entry",
				"content":  "Test content",
				"category": "test",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	entry, err := client.GetEntry(context.Background(), "entry-1")

	require.NoError(t, err)
	assert.Equal(t, "entry-1", entry.ID)
	assert.Equal(t, "Test Entry", entry.Title)
}

func TestCreateEntry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/entry/create")

		var req CreateEntryRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "New Entry", req.Title)

		resp := APIResponse{
			Code: 0,
			Data: map[string]interface{}{
				"id":       "new-entry",
				"title":    req.Title,
				"content":  req.Content,
				"category": req.Category,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	entry, err := client.CreateEntry(context.Background(), &CreateEntryRequest{
		Title:    "New Entry",
		Content:  "Content",
		Category: "test",
	})

	require.NoError(t, err)
	assert.Equal(t, "new-entry", entry.ID)
}

func TestRateEntry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/entry/rate/entry-1")

		var req RatingRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, 4.5, req.Score)

		resp := APIResponse{Code: 0, Message: "success"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.RateEntry(context.Background(), "entry-1", 4.5, "Good entry")
	require.NoError(t, err)
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		resp := APIError{Code: 404, Message: "entry not found"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetEntry(context.Background(), "nonexistent")

	require.Error(t, err)
	assert.True(t, IsNotFoundError(err))
}
```

- [ ] **Step 3: 运行测试**

```bash
go test -v ./pkg/polysdk/...
```

Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add pkg/polysdk/client.go pkg/polysdk/client_test.go
git commit -m "feat(polysdk): implement HTTP client with search, entry, and rating APIs"
```

---

### Task 3: 实现配置加载

**Files:**
- Create: `pkg/polysdk/config.go`

- [ ] **Step 1: 创建 config.go**

```go
// pkg/polysdk/config.go
package polysdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config Polyant SDK 配置
type Config struct {
	// API 服务器地址
	BaseURL string `json:"base_url"`
	// API Key（用于公开路由）
	APIKey string `json:"api_key,omitempty"`
	// 密钥目录
	KeyDir string `json:"key_dir,omitempty"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		BaseURL: "http://localhost:8080",
		KeyDir:  filepath.Join(homeDir, ".polyant", "keys"),
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(config *Config, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// LoadConfigOrDefault 加载配置或返回默认配置
func LoadConfigOrDefault(path string) *Config {
	config, err := LoadConfig(path)
	if err != nil {
		return DefaultConfig()
	}
	return config
}

// NewClientFromConfig 从配置创建客户端
func NewClientFromConfig(config *Config) *Client {
	client := NewClient(config.BaseURL)
	if config.APIKey != "" {
		client.SetAPIKey(config.APIKey)
	}
	return client
}

// DefaultConfigPath 返回默认配置文件路径
func DefaultConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".polyant", "config.json")
}
```

- [ ] **Step 2: 提交**

```bash
git add pkg/polysdk/config.go
git commit -m "feat(polysdk): add config loading and client factory"
```

---

## 阶段 2：agentskills.io 标准技能

### Task 4: 创建 polyant-search 技能

**Files:**
- Create: `skills/agentskills/polyant-search/SKILL.md`
- Create: `skills/agentskills/polyant-search/scripts/search.sh`

- [ ] **Step 1: 创建 SKILL.md**

```markdown
---
name: polyant-search
description: Search Polyant knowledge base for solutions, best practices, and technical documentation. Trigger when encountering errors, performance issues, or architecture questions.
---

# Polyant Search

Search the Polyant distributed knowledge base for solutions and best practices.

## When to Use

- **Compilation errors:** When code fails to compile
- **Runtime errors:** When code crashes or produces unexpected results
- **Performance issues:** When code is slow or resource-intensive
- **Architecture questions:** When designing systems or making technical decisions
- **Best practices:** When looking for recommended approaches

## How to Use

### Automatic Detection

When you encounter an error or problem, automatically search for solutions:

1. Extract keywords from the error message or problem description
2. Run the search script: `bash scripts/search.sh "<query>"`
3. Parse and present the results to the user

### Manual Search

```bash
# Basic search
pactl search "<query>"

# Search with category filter
pactl search "<query>" --category "computer-science/programming-languages/go"

# Search with limit
pactl search "<query>" --limit 5
```

## Output Format

The search results contain:
- **Title:** Entry title
- **Category:** Knowledge category path
- **Score:** Community rating (1-5)
- **Tags:** Related tags
- **Content:** Entry content (in full output mode)

## Example

User: I'm getting "undefined: fmt.Println" error

Agent: Let me search the Polyant knowledge base for solutions.

```bash
pactl search "undefined fmt.Println" --limit 3
```

Found 3 relevant entries:

1. **Go Common Errors: Missing fmt Import** (4.8/5)
   - Category: computer-science/programming-languages/go
   - Solution: Add `import "fmt"` to your file

2. **Go Import Management Best Practices** (4.6/5)
   - Category: computer-science/programming-languages/go
   - Solution: Use goimports to auto-manage imports

3. **Go Compilation Error Troubleshooting** (4.5/5)
   - Category: computer-science/programming-languages/go
   - Solution: Check import paths and package names

## Configuration

Requires Polyant connection configuration. Run `polyant-config` skill first if not configured.

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `POLYANT_API_URL` | API server URL | `http://localhost:8080` |
| `POLYANT_API_KEY` | API key for authentication | (none) |
```

- [ ] **Step 2: 创建 search.sh**

```bash
#!/bin/bash
# skills/agentskills/polyant-search/scripts/search.sh
# Polyant 搜索技能辅助脚本

set -e

# 默认配置
POLYANT_API_URL="${POLYANT_API_URL:-http://localhost:8080}"
POLYANT_API_KEY="${POLYANT_API_KEY:-}"

# 检查 pactl 是否可用
if ! command -v pactl &> /dev/null; then
    echo "错误: pactl 未安装。请先安装 Polyant CLI。"
    echo "安装方法: go install github.com/daifei0527/polyant/cmd/pactl@latest"
    exit 1
fi

# 获取查询参数
QUERY="${1:-}"
if [ -z "$QUERY" ]; then
    echo "用法: search.sh <query> [category] [limit]"
    echo "示例: search.sh \"Go error handling\" computer-science/programming-languages/go 5"
    exit 1
fi

CATEGORY="${2:-}"
LIMIT="${3:-5}"

# 构建命令
CMD="pactl search \"$QUERY\" --limit $LIMIT"
if [ -n "$CATEGORY" ]; then
    CMD="$CMD --category \"$CATEGORY\""
fi

# 执行搜索
echo "搜索 Polyant 知识库: $QUERY"
echo "---"
eval $CMD
```

- [ ] **Step 3: 设置脚本可执行权限**

```bash
chmod +x skills/agentskills/polyant-search/scripts/search.sh
```

- [ ] **Step 4: 提交**

```bash
git add skills/agentskills/polyant-search/
git commit -m "feat(skills): add polyant-search agentskills.io skill"
```

---

### Task 5: 创建 polyant-save 技能

**Files:**
- Create: `skills/agentskills/polyant-save/SKILL.md`
- Create: `skills/agentskills/polyant-save/scripts/save.sh`

- [ ] **Step 1: 创建 SKILL.md**

```markdown
---
name: polyant-save
description: Save knowledge and experience to Polyant knowledge base. Trigger after completing tasks, solving errors, or discovering best practices.
---

# Polyant Save

Save knowledge, solutions, and best practices to the Polyant distributed knowledge base.

## When to Use

- **Task completion:** After successfully completing a task
- **Error resolution:** After solving a technical problem
- **Best practices:** When discovering recommended approaches
- **New knowledge:** When learning something valuable

## Entry Format

Use the following structure for knowledge entries:

```markdown
## Problem
[Describe the problem or question]

## Solution
[Provide the solution or answer]

## Example
[Include code examples if applicable]

## Why
[Explain why this solution works]

## When
[Describe when to use this approach]

## References
[List any related resources]
```

## How to Use

### Automatic Detection

After solving a problem or completing a task, automatically save the experience:

1. Extract the key knowledge from the conversation
2. Format it using the entry format above
3. Run the save script: `bash scripts/save.sh "<title>" "<content>" "<category>" "<tags>"`

### Manual Save

```bash
# Create a new entry
pactl entry create \
  --title "Go Error Handling Best Practices" \
  --content "## Problem\nHow to handle errors in Go effectively..." \
  --category "computer-science/programming-languages/go" \
  --tags "go,error-handling,best-practices"
```

## Category Taxonomy

Use the following category structure:

```
computer-science/
├── programming-languages/
│   ├── go/
│   ├── python/
│   ├── javascript/
│   └── rust/
├── algorithms/
├── databases/
└── network-protocols/

artificial-intelligence/
├── machine-learning/
├── nlp/
└── llm/

tools/
├── dev-tools/
└── system-administration/
```

## Quality Guidelines

- **Be specific:** Include concrete examples and code
- **Be complete:** Cover the problem, solution, and context
- **Be accurate:** Verify the solution works before saving
- **Use tags:** Add relevant tags for discoverability

## Example

After solving a Go import error:

```bash
bash scripts/save.sh \
  "Go Common Errors: Missing fmt Import" \
  "## Problem\nGetting 'undefined: fmt.Println' error.\n\n## Solution\nAdd 'import \"fmt\"' to your file.\n\n## Example\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}" \
  "computer-science/programming-languages/go" \
  "go,compilation,import,fmt"
```

## Configuration

Requires Polyant connection with authentication (Lv1+). Run `polyant-config` skill first if not configured.
```

- [ ] **Step 2: 创建 save.sh**

```bash
#!/bin/bash
# skills/agentskills/polyant-save/scripts/save.sh
# Polyant 保存技能辅助脚本

set -e

# 检查 pactl 是否可用
if ! command -v pactl &> /dev/null; then
    echo "错误: pactl 未安装。请先安装 Polyant CLI。"
    exit 1
fi

# 获取参数
TITLE="${1:-}"
CONTENT="${2:-}"
CATEGORY="${3:-}"
TAGS="${4:-}"

if [ -z "$TITLE" ] || [ -z "$CONTENT" ]; then
    echo "用法: save.sh <title> <content> [category] [tags]"
    echo "示例: save.sh \"Go Error Handling\" \"## Problem\n...\" \"computer-science/go\" \"go,error\""
    exit 1
fi

# 构建命令
CMD="pactl entry create --title \"$TITLE\" --content \"$CONTENT\""
if [ -n "$CATEGORY" ]; then
    CMD="$CMD --category \"$CATEGORY\""
fi
if [ -n "$TAGS" ]; then
    CMD="$CMD --tags \"$TAGS\""
fi

# 执行保存
echo "保存知识到 Polyant: $TITLE"
echo "---"
eval $CMD
```

- [ ] **Step 3: 设置脚本可执行权限并提交**

```bash
chmod +x skills/agentskills/polyant-save/scripts/save.sh
git add skills/agentskills/polyant-save/
git commit -m "feat(skills): add polyant-save agentskills.io skill"
```

---

### Task 6: 创建 polyant-learn 技能

**Files:**
- Create: `skills/agentskills/polyant-learn/SKILL.md`
- Create: `skills/agentskills/polyant-learn/scripts/learn.sh`

- [ ] **Step 1: 创建 SKILL.md**

```markdown
---
name: polyant-learn
description: Learn from Polyant knowledge base to improve skills. Trigger when encountering new technologies or needing deep understanding.
---

# Polyant Learn

Learn from the Polyant knowledge base to improve your skills and understanding.

## When to Use

- **New technology:** When encountering unfamiliar technologies
- **Deep understanding:** When needing comprehensive knowledge
- **Best practices:** When looking for recommended approaches
- **Skill building:** When wanting to improve specific skills

## How to Use

### Learning Path Generation

1. Identify the topic you want to learn
2. Search for related entries: `bash scripts/learn.sh "<topic>"`
3. Follow the suggested learning path
4. Save your learning notes after completing each step

### Manual Learning

```bash
# Search for learning materials
pactl search "<topic>" --limit 10

# Get specific entry details
pactl entry get "<entry-id>"

# List categories to explore
pactl category list --tree
```

## Learning Strategies

### 1. Spaced Repetition

- Review learned material at increasing intervals
- Save review notes to track progress
- Use tags like "reviewed-1", "reviewed-2" for tracking

### 2. Knowledge Graph

- Connect related entries using wiki-style links
- Build a personal knowledge graph
- Use backlinks to discover related knowledge

### 3. Active Learning

- Practice with real examples
- Save your implementations
- Compare with best practices

## Example

Learning Go concurrency:

```bash
# Search for Go concurrency materials
bash scripts/learn.sh "Go concurrency goroutine channel"

# Results will include:
# 1. Go Concurrency Basics
# 2. Goroutine Best Practices
# 3. Channel Patterns
# 4. Common Concurrency Mistakes
```

After learning, save your notes:

```bash
pactl entry create \
  --title "My Go Concurrency Learning Notes" \
  --content "## What I Learned\n..." \
  --category "personal/learning/go" \
  --tags "go,concurrency,learning"
```

## Configuration

Requires Polyant connection configuration. Run `polyant-config` skill first if not configured.
```

- [ ] **Step 2: 创建 learn.sh**

```bash
#!/bin/bash
# skills/agentskills/polyant-learn/scripts/learn.sh
# Polyant 学习技能辅助脚本

set -e

# 检查 pactl 是否可用
if ! command -v pactl &> /dev/null; then
    echo "错误: pactl 未安装。请先安装 Polyant CLI。"
    exit 1
fi

# 获取参数
TOPIC="${1:-}"
LIMIT="${2:-10}"

if [ -z "$TOPIC" ]; then
    echo "用法: learn.sh <topic> [limit]"
    echo "示例: learn.sh \"Go concurrency\" 10"
    exit 1
fi

# 搜索学习材料
echo "搜索学习材料: $TOPIC"
echo "---"
pactl search "$TOPIC" --limit $LIMIT

echo ""
echo "---"
echo "提示: 使用 'pactl entry get <id>' 查看详细内容"
echo "学习完成后，使用 polyant-save 技能保存学习笔记"
```

- [ ] **Step 3: 设置脚本可执行权限并提交**

```bash
chmod +x skills/agentskills/polyant-learn/scripts/learn.sh
git add skills/agentskills/polyant-learn/
git commit -m "feat(skills): add polyant-learn agentskills.io skill"
```

---

### Task 7: 创建 polyant-rate 技能

**Files:**
- Create: `skills/agentskills/polyant-rate/SKILL.md`
- Create: `skills/agentskills/polyant-rate/scripts/rate.sh`

- [ ] **Step 1: 创建 SKILL.md**

```markdown
---
name: polyant-rate
description: Rate and review Polyant knowledge entries. Trigger after using knowledge to provide feedback and improve quality.
---

# Polyant Rate

Rate and review knowledge entries in the Polyant knowledge base.

## When to Use

- **After using knowledge:** When you successfully apply a solution
- **Quality feedback:** When you want to improve entry quality
- **Correction:** When you find errors or outdated information

## Rating Scale

| Score | Meaning |
|-------|---------|
| 5 | Excellent - Works perfectly, clear explanation |
| 4 | Good - Works well, minor improvements possible |
| 3 | Average - Works but could be better |
| 2 | Poor - Partially works or unclear |
| 1 | Bad - Doesn't work or misleading |

## How to Use

### Rate an Entry

```bash
# Rate with score and optional comment
pactl entry rate <entry-id> --score 4 --comment "Worked perfectly for my use case"
```

### Using the Script

```bash
bash scripts/rate.sh <entry-id> <score> [comment]
```

## Example

After using a solution from the knowledge base:

```bash
bash scripts/rate.sh "entry-123" 5 "This solved my Go import error immediately. Clear and concise."
```

## Best Practices

- **Be honest:** Rate based on actual usefulness
- **Be constructive:** Add comments explaining your rating
- **Be specific:** Mention what worked or didn't work
- **Update ratings:** Re-rate if you find issues later

## Configuration

Requires Polyant connection with authentication (Lv1+). Run `polyant-config` skill first if not configured.
```

- [ ] **Step 2: 创建 rate.sh**

```bash
#!/bin/bash
# skills/agentskills/polyant-rate/scripts/rate.sh
# Polyant 评价技能辅助脚本

set -e

# 检查 pactl 是否可用
if ! command -v pactl &> /dev/null; then
    echo "错误: pactl 未安装。请先安装 Polyant CLI。"
    exit 1
fi

# 获取参数
ENTRY_ID="${1:-}"
SCORE="${2:-}"
COMMENT="${3:-}"

if [ -z "$ENTRY_ID" ] || [ -z "$SCORE" ]; then
    echo "用法: rate.sh <entry-id> <score> [comment]"
    echo "示例: rate.sh \"entry-123\" 5 \"Excellent solution\""
    exit 1
fi

# 验证评分范围
if [ "$SCORE" -lt 1 ] || [ "$SCORE" -gt 5 ]; then
    echo "错误: 评分必须在 1-5 之间"
    exit 1
fi

# 构建命令
CMD="pactl entry rate \"$ENTRY_ID\" --score $SCORE"
if [ -n "$COMMENT" ]; then
    CMD="$CMD --comment \"$COMMENT\""
fi

# 执行评价
echo "评价条目: $ENTRY_ID (评分: $SCORE/5)"
echo "---"
eval $CMD
```

- [ ] **Step 3: 设置脚本可执行权限并提交**

```bash
chmod +x skills/agentskills/polyant-rate/scripts/rate.sh
git add skills/agentskills/polyant-rate/
git commit -m "feat(skills): add polyant-rate agentskills.io skill"
```

---

### Task 8: 创建 polyant-config 技能

**Files:**
- Create: `skills/agentskills/polyant-config/SKILL.md`
- Create: `skills/agentskills/polyant-config/scripts/config.sh`

- [ ] **Step 1: 创建 SKILL.md**

```markdown
---
name: polyant-config
description: Configure Polyant knowledge base connection settings. Trigger on first use or when configuration changes are needed.
---

# Polyant Config

Configure the connection to your Polyant knowledge base node.

## When to Use

- **First time setup:** When using Polyant skills for the first time
- **Configuration changes:** When switching to a different node
- **Troubleshooting:** When connection issues occur

## Configuration Methods

### 1. Environment Variables

```bash
export POLYANT_API_URL="http://your-node:8080"
export POLYANT_API_KEY="your-api-key"
```

### 2. Configuration File

Create `~/.polyant/config.json`:

```json
{
  "base_url": "http://your-node:8080",
  "api_key": "your-api-key",
  "key_dir": "~/.polyant/keys"
}
```

### 3. Interactive Setup

```bash
bash scripts/config.sh
```

## How to Use

### Quick Setup

```bash
# Set API URL
bash scripts/config.sh set-url "http://your-node:8080"

# Set API Key
bash scripts/config.sh set-key "your-api-key"

# Show current config
bash scripts/config.sh show
```

### Manual Setup

```bash
# Generate keys for authentication
pactl key generate

# Register as a user
pactl user register --name "Your Agent Name"

# Verify connection
pactl status
```

## Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `POLYANT_API_URL` | API server URL | `http://localhost:8080` |
| `POLYANT_API_KEY` | API key for public routes | (none) |
| `POLYANT_KEY_DIR` | Directory for Ed25519 keys | `~/.polyant/keys` |

## Troubleshooting

### Connection Refused

```bash
# Check if node is running
curl http://localhost:8080/api/v1/node/status

# Check firewall
telnet your-node 8080
```

### Authentication Failed

```bash
# Regenerate keys
pactl key generate --force

# Re-register
pactl user register --name "Your Agent Name"
```

### Permission Denied

```bash
# Check user level
pactl user info

# Request level upgrade from admin
```

## Security Notes

- **API Key:** Used for public routes only, not for authenticated operations
- **Ed25519 Keys:** Used for signing authenticated requests
- **Key Storage:** Keys are stored in `~/.polyant/keys/` with 0600 permissions
- **Never share:** Never share your private key or API key
```

- [ ] **Step 2: 创建 config.sh**

```bash
#!/bin/bash
# skills/agentskills/polyant-config/scripts/config.sh
# Polyant 配置技能辅助脚本

set -e

CONFIG_DIR="$HOME/.polyant"
CONFIG_FILE="$CONFIG_DIR/config.json"

# 创建配置目录
mkdir -p "$CONFIG_DIR"

# 显示当前配置
show_config() {
    echo "当前 Polyant 配置:"
    echo "---"
    if [ -f "$CONFIG_FILE" ]; then
        cat "$CONFIG_FILE"
    else
        echo "配置文件不存在: $CONFIG_FILE"
    fi
    echo ""
    echo "环境变量:"
    echo "  POLYANT_API_URL: ${POLYANT_API_URL:-未设置}"
    echo "  POLYANT_API_KEY: ${POLYANT_API_KEY:+已设置}"
}

# 设置 API URL
set_url() {
    local url="$1"
    if [ -z "$url" ]; then
        echo "用法: config.sh set-url <url>"
        exit 1
    fi

    # 读取现有配置或创建新的
    if [ -f "$CONFIG_FILE" ]; then
        local config=$(cat "$CONFIG_FILE")
    else
        local config='{}'
    fi

    # 更新配置
    echo "$config" | jq --arg url "$url" '.base_url = $url' > "$CONFIG_FILE"
    echo "已设置 API URL: $url"
}

# 设置 API Key
set_key() {
    local key="$1"
    if [ -z "$key" ]; then
        echo "用法: config.sh set-key <key>"
        exit 1
    fi

    # 读取现有配置或创建新的
    if [ -f "$CONFIG_FILE" ]; then
        local config=$(cat "$CONFIG_FILE")
    else
        local config='{}'
    fi

    # 更新配置
    echo "$config" | jq --arg key "$key" '.api_key = $key' > "$CONFIG_FILE"
    echo "已设置 API Key"
}

# 主逻辑
case "${1:-show}" in
    show)
        show_config
        ;;
    set-url)
        set_url "$2"
        ;;
    set-key)
        set_key "$2"
        ;;
    *)
        echo "用法: config.sh [show|set-url|set-key] [value]"
        exit 1
        ;;
esac
```

- [ ] **Step 3: 设置脚本可执行权限并提交**

```bash
chmod +x skills/agentskills/polyant-config/scripts/config.sh
git add skills/agentskills/polyant-config/
git commit -m "feat(skills): add polyant-config agentskills.io skill"
```

---

## 阶段 3：OpenClaw 专用技能

### Task 9: 创建 OpenClaw 技能

**Files:**
- Create: `skills/openclaw/polyant-search.md`
- Create: `skills/openclaw/polyant-save.md`
- Create: `skills/openclaw/polyant-learn.md`
- Create: `skills/openclaw/polyant-rate.md`
- Create: `skills/openclaw/polyant-config.md`

- [ ] **Step 1: 创建 polyant-search.md**

```markdown
# Polyant 搜索技能

搜索 Polyant 分布式知识库，查找解决方案、最佳实践和技术文档。

## 触发条件

当遇到以下情况时，自动触发搜索：
- 编译错误
- 运行时错误
- 性能问题
- 架构问题
- 需要查找最佳实践

## 使用方法

1. 提取错误关键词或问题描述
2. 调用搜索命令：
   ```bash
   pactl search "关键词"
   ```
3. 解析搜索结果
4. 展示最相关的解决方案

## 示例

用户：我遇到了一个编译错误：undefined: fmt.Println

智能体：让我搜索知识库...
```bash
pactl search "undefined fmt.Println" --limit 3
```

找到 3 个相关条目：
1. Go 语言常见错误：fmt 包未导入
2. Go 语言编译错误解决方案
3. Go 语言最佳实践：导入管理

## 配置要求

确保已配置 Polyant 连接：
```bash
export POLYANT_API_URL=http://localhost:8080
export POLYANT_API_KEY=your-api-key
```
```

- [ ] **Step 2: 创建 polyant-save.md**

```markdown
# Polyant 保存技能

保存知识和经验到 Polyant 分布式知识库。

## 触发条件

当完成以下操作时，自动触发保存：
- 完成任务
- 解决错误
- 发现最佳实践
- 学习新知识

## 使用方法

1. 提取关键知识
2. 格式化为标准条目格式
3. 调用保存命令：
   ```bash
   pactl entry create --title "标题" --content "内容" --category "分类" --tags "标签"
   ```

## 条目格式

```markdown
## 问题
[描述问题]

## 解决方案
[提供解决方案]

## 示例
[包含代码示例]

## 原因
[解释为什么有效]

## 适用场景
[描述何时使用此方法]
```

## 示例

用户：我已经解决了这个编译错误

智能体：让我保存这个经验...
```bash
pactl entry create \
  --title "Go 语言常见错误：fmt 包未导入" \
  --content "## 问题\n编译错误：undefined: fmt.Println\n\n## 解决方案\n添加 import \"fmt\"" \
  --category "computer-science/programming-languages/go" \
  --tags "go,compilation,import"
```

已保存到知识库。

## 配置要求

需要认证（Lv1+），确保已生成密钥并注册：
```bash
pactl key generate
pactl user register --name "Your Name"
```
```

- [ ] **Step 3: 创建 polyant-learn.md**

```markdown
# Polyant 学习技能

从 Polyant 知识库学习新知识，提升技能水平。

## 触发条件

当遇到以下情况时，自动触发学习：
- 遇到新技术
- 需要深入理解
- 寻找最佳实践
- 提升特定技能

## 使用方法

1. 确定学习主题
2. 搜索相关知识：
   ```bash
   pactl search "主题" --limit 10
   ```
3. 按照学习路径阅读
4. 保存学习笔记

## 学习策略

### 间隔重复
- 在递增的间隔复习已学材料
- 使用标签跟踪复习进度

### 知识图谱
- 使用 wiki 链接连接相关条目
- 构建个人知识图谱

### 主动学习
- 用真实示例练习
- 保存你的实现
- 与最佳实践对比

## 示例

用户：我想学习 Go 语言的并发编程

智能体：让我搜索相关学习材料...
```bash
pactl search "Go concurrency goroutine channel" --limit 5
```

找到 5 个相关条目：
1. Go 并发编程基础
2. Goroutine 最佳实践
3. Channel 模式
4. 常见并发错误
5. 并发性能优化

学习完成后，保存学习笔记：
```bash
pactl entry create \
  --title "Go 并发编程学习笔记" \
  --content "## 学到的内容\n..." \
  --category "personal/learning/go" \
  --tags "go,concurrency,learning"
```

## 配置要求

确保已配置 Polyant 连接。
```

- [ ] **Step 4: 创建 polyant-rate.md**

```markdown
# Polyant 评价技能

评价 Polyant 知识库中的条目，提供反馈以提高质量。

## 触发条件

当使用知识后，提供评价反馈：
- 成功应用解决方案后
- 发现条目有误时
- 想要改进条目质量时

## 评分标准

| 评分 | 含义 |
|------|------|
| 5 | 优秀 - 完美工作，解释清晰 |
| 4 | 良好 - 效果好，可小幅改进 |
| 3 | 一般 - 可用但可以更好 |
| 2 | 较差 - 部分可用或不清晰 |
| 1 | 很差 - 不可用或有误导性 |

## 使用方法

```bash
pactl entry rate <条目ID> --score <评分> --comment "评论"
```

## 示例

用户：这个解决方案很有效

智能体：让我为这个条目评分...
```bash
pactl entry rate "entry-123" --score 5 --comment "完美解决了我的 Go import 错误"
```

已评价。

## 配置要求

需要认证（Lv1+）。
```

- [ ] **Step 5: 创建 polyant-config.md**

```markdown
# Polyant 配置技能

配置 Polyant 知识库连接设置。

## 触发条件

首次使用或需要更改配置时：
- 首次使用 Polyant 技能
- 切换到不同的节点
- 连接出现问题

## 配置方法

### 环境变量

```bash
export POLYANT_API_URL="http://your-node:8080"
export POLYANT_API_KEY="your-api-key"
```

### 配置文件

创建 `~/.polyant/config.json`：
```json
{
  "base_url": "http://your-node:8080",
  "api_key": "your-api-key"
}
```

## 快速设置

```bash
# 生成密钥
pactl key generate

# 注册用户
pactl user register --name "Your Name"

# 验证连接
pactl status
```

## 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `POLYANT_API_URL` | API 服务器地址 | `http://localhost:8080` |
| `POLYANT_API_KEY` | API 密钥 | 无 |
| `POLYANT_KEY_DIR` | 密钥目录 | `~/.polyant/keys` |

## 故障排除

### 连接被拒绝
```bash
curl http://localhost:8080/api/v1/node/status
```

### 认证失败
```bash
pactl key generate --force
pactl user register --name "Your Name"
```
```

- [ ] **Step 6: 提交**

```bash
git add skills/openclaw/
git commit -m "feat(skills): add OpenClaw-specific Polyant skills"
```

---

## 阶段 4：MCP 服务器

### Task 10: 创建 MCP 服务器框架

**Files:**
- Create: `cmd/polyant-mcp-server/main.go`
- Create: `cmd/polyant-mcp-server/server.go`
- Create: `cmd/polyant-mcp-server/config.go`

- [ ] **Step 1: 创建 main.go**

```go
// cmd/polyant-mcp-server/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	if *configPath == "" {
		homeDir, _ := os.UserHomeDir()
		*configPath = homeDir + "/.polyant/config.json"
	}

	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	server, err := NewServer(config)
	if err != nil {
		log.Fatalf("创建服务器失败: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Polyant MCP 服务器启动中...\n")
	fmt.Fprintf(os.Stderr, "API 地址: %s\n", config.BaseURL)

	if err := server.Run(); err != nil {
		log.Fatalf("服务器运行失败: %v", err)
	}
}
```

- [ ] **Step 2: 创建 config.go**

```go
// cmd/polyant-mcp-server/config.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config MCP 服务器配置
type Config struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
	KeyDir  string `json:"key_dir,omitempty"`
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8080"
	}

	return &config, nil
}
```

- [ ] **Step 3: 创建 server.go 框架**

```go
// cmd/polyant-mcp-server/server.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	polysdk "github.com/daifei0527/polyant/pkg/polysdk"
)

// Server Polyant MCP 服务器
type Server struct {
	client *polysdk.Client
	config *Config
}

// NewServer 创建新的 MCP 服务器
func NewServer(config *Config) (*Server, error) {
	client := polysdk.NewClient(config.BaseURL)
	if config.APIKey != "" {
		client.SetAPIKey(config.APIKey)
	}

	return &Server{
		client: client,
		config: config,
	}, nil
}

// Run 运行服务器
func (s *Server) Run() error {
	// MCP 服务器通过 stdin/stdout 通信
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stderr) // 错误输出到 stderr

	for {
		var request map[string]interface{}
		if err := decoder.Decode(&request); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("decode request: %w", err)
		}

		response := s.handleRequest(context.Background(), request)
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
	}

	return nil
}

// handleRequest 处理 MCP 请求
func (s *Server) handleRequest(ctx context.Context, request map[string]interface{}) map[string]interface{} {
	method, _ := request["method"].(string)
	id, _ := request["id"]

	switch method {
	case "initialize":
		return s.handleInitialize(id)
	case "tools/list":
		return s.handleListTools(id)
	case "tools/call":
		return s.handleCallTool(ctx, id, request)
	default:
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]interface{}{
				"code":    -32601,
				"message": "Method not found",
			},
		}
	}
}

// handleInitialize 处理初始化请求
func (s *Server) handleInitialize(id interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "polyant-mcp-server",
				"version": "1.0.0",
			},
		},
	}
}

// handleListTools 处理工具列表请求
func (s *Server) handleListTools(id interface{}) map[string]interface{} {
	tools := []map[string]interface{}{
		{
			"name":        "polyant_search",
			"description": "搜索 Polyant 知识库",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "搜索关键词",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "分类过滤（可选）",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "标签过滤（可选）",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "结果数量限制",
						"default":     10,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "polyant_create",
			"description": "创建知识条目",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "条目标题",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "条目内容",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "分类路径",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "标签列表",
					},
				},
				"required": []string{"title", "content", "category"},
			},
		},
		{
			"name":        "polyant_rate",
			"description": "评价知识条目",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "条目 ID",
					},
					"score": map[string]interface{}{
						"type":        "number",
						"description": "评分 (1-5)",
					},
					"comment": map[string]interface{}{
						"type":        "string",
						"description": "评价评论（可选）",
					},
				},
				"required": []string{"id", "score"},
			},
		},
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"tools": tools,
		},
	}
}

// handleCallTool 处理工具调用请求
func (s *Server) handleCallTool(ctx context.Context, id interface{}, request map[string]interface{}) map[string]interface{} {
	params, _ := request["params"].(map[string]interface{})
	toolName, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]interface{})

	var result interface{}
	var err error

	switch toolName {
	case "polyant_search":
		result, err = s.handleSearch(ctx, arguments)
	case "polyant_create":
		result, err = s.handleCreate(ctx, arguments)
	case "polyant_rate":
		result, err = s.handleRate(ctx, arguments)
	default:
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]interface{}{
				"code":    -32601,
				"message": "Tool not found",
			},
		}
	}

	if err != nil {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]interface{}{
				"code":    -32000,
				"message": err.Error(),
			},
		}
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": formatResult(result),
				},
			},
		},
	}
}

// handleSearch 处理搜索请求
func (s *Server) handleSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	category, _ := args["category"].(string)
	limit := 10
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}

	var tags []string
	if t, ok := args["tags"].([]interface{}); ok {
		for _, v := range t {
			if s, ok := v.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	return s.client.Search(ctx, query, category, tags, limit)
}

// handleCreate 处理创建请求
func (s *Server) handleCreate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	category, _ := args["category"].(string)

	var tags []string
	if t, ok := args["tags"].([]interface{}); ok {
		for _, v := range t {
			if s, ok := v.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	return s.client.CreateEntry(ctx, &polysdk.CreateEntryRequest{
		Title:    title,
		Content:  content,
		Category: category,
		Tags:     tags,
	})
}

// handleRate 处理评价请求
func (s *Server) handleRate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, _ := args["id"].(string)
	score, _ := args["score"].(float64)
	comment, _ := args["comment"].(string)

	err := s.client.RateEntry(ctx, id, score, comment)
	if err != nil {
		return nil, err
	}

	return map[string]string{"status": "success"}, nil
}

// formatResult 格式化结果为文本
func formatResult(result interface{}) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return string(data)
}
```

- [ ] **Step 4: 添加 go.mod 依赖并测试编译**

```bash
cd cmd/polyant-mcp-server && go mod tidy && go build -o /dev/null .
```

- [ ] **Step 5: 提交**

```bash
git add cmd/polyant-mcp-server/
git commit -m "feat(mcp): implement Polyant MCP server with search, create, and rate tools"
```

---

### Task 11: 添加 MCP 服务器测试

**Files:**
- Create: `cmd/polyant-mcp-server/server_test.go`

- [ ] **Step 1: 创建 server_test.go**

```go
// cmd/polyant-mcp-server/server_test.go
package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleInitialize(t *testing.T) {
	server := &Server{}
	response := server.handleInitialize("test-id")

	assert.Equal(t, "2.0", response["jsonrpc"])
	assert.Equal(t, "test-id", response["id"])

	result, ok := response["result"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2024-11-05", result["protocolVersion"])
}

func TestHandleListTools(t *testing.T) {
	server := &Server{}
	response := server.handleListTools("test-id")

	assert.Equal(t, "2.0", response["jsonrpc"])

	result, ok := response["result"].(map[string]interface{})
	require.True(t, ok)

	tools, ok := result["tools"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, tools, 3) // search, create, rate

	// 验证搜索工具
	searchTool := tools[0]
	assert.Equal(t, "polyant_search", searchTool["name"])
}

func TestHandleUnknownMethod(t *testing.T) {
	server := &Server{}
	request := map[string]interface{}{
		"method": "unknown",
		"id":     "test-id",
	}
	response := server.handleRequest(context.Background(), request)

	assert.Equal(t, "2.0", response["jsonrpc"])
	_, hasError := response["error"]
	assert.True(t, hasError)
}

func TestFormatResult(t *testing.T) {
	result := map[string]string{"key": "value"}
	formatted := formatResult(result)

	var parsed map[string]string
	err := json.Unmarshal([]byte(formatted), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "value", parsed["key"])
}
```

- [ ] **Step 2: 运行测试**

```bash
go test -v ./cmd/polyant-mcp-server/...
```

- [ ] **Step 3: 提交**

```bash
git add cmd/polyant-mcp-server/server_test.go
git commit -m "test(mcp): add MCP server unit tests"
```

---

## 阶段 5：安装脚本和文档

### Task 12: 创建统一安装脚本

**Files:**
- Create: `scripts/install-unified.sh`

- [ ] **Step 1: 创建 install-unified.sh**

```bash
#!/bin/bash
# scripts/install-unified.sh
# Polyant 智能体技能统一安装脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Polyant 智能体技能安装器 ==="
echo ""

# 检测已安装的智能体
detect_agents() {
    local agents=()

    # 检测 Claude Code
    if [ -d ~/.claude ]; then
        agents+=("claude-code")
    fi

    # 检测 Codex
    if [ -d ~/.agents ]; then
        agents+=("codex")
    fi

    # 检测 Hermes
    if [ -d ~/.hermes ]; then
        agents+=("hermes")
    fi

    # 检测 OpenClaw
    if [ -d ~/.openclaw ]; then
        agents+=("openclaw")
    fi

    echo "${agents[@]}"
}

# 安装 agentskills.io 标准技能
install_agentskills() {
    echo "安装 agentskills.io 标准技能..."

    # 安装到 Codex
    if [ -d ~/.agents ]; then
        mkdir -p ~/.agents/skills
        cp -r "$PROJECT_ROOT/skills/agentskills/"* ~/.agents/skills/
        echo "✓ 已安装到 Codex (~/.agents/skills/)"
    fi

    # 安装到 Hermes
    if [ -d ~/.hermes ]; then
        mkdir -p ~/.hermes/skills
        cp -r "$PROJECT_ROOT/skills/agentskills/"* ~/.hermes/skills/
        echo "✓ 已安装到 Hermes (~/.hermes/skills/)"
    fi
}

# 安装 OpenClaw 技能
install_openclaw() {
    echo "安装 OpenClaw 技能..."

    if [ -d ~/.openclaw ]; then
        mkdir -p ~/.openclaw/skills
        cp -r "$PROJECT_ROOT/skills/openclaw/"* ~/.openclaw/skills/
        echo "✓ 已安装到 OpenClaw (~/.openclaw/skills/)"
    fi
}

# 安装 Claude Code 技能
install_claude_code() {
    echo "安装 Claude Code 技能..."

    if [ -d ~/.claude ]; then
        mkdir -p ~/.claude/skills
        cp -r "$PROJECT_ROOT/skills/polyant-*.md" ~/.claude/skills/ 2>/dev/null || true
        echo "✓ 已安装到 Claude Code (~/.claude/skills/)"
    fi
}

# 安装 MCP 服务器
install_mcp_server() {
    echo "安装 MCP 服务器..."

    if command -v go &> /dev/null; then
        go install github.com/daifei0527/polyant/cmd/polyant-mcp-server@latest
        echo "✓ 已安装 MCP 服务器"
    else
        echo "⚠ Go 未安装，跳过 MCP 服务器安装"
    fi
}

# 主流程
main() {
    local agents=$(detect_agents)

    if [ -z "$agents" ]; then
        echo "未检测到已安装的智能体"
        echo ""
        echo "请先安装以下智能体之一："
        echo "  - Claude Code (https://claude.ai/code)"
        echo "  - Codex (https://github.com/openai/codex)"
        echo "  - Hermes Agent (https://hermes-agent.nousresearch.com)"
        echo "  - OpenClaw (https://openclaw.ai)"
        echo ""
        echo "然后重新运行此脚本。"
        exit 1
    fi

    echo "检测到以下智能体: $agents"
    echo ""

    # 安装技能
    install_agentskills
    install_openclaw
    install_claude_code

    echo ""
    echo "=== 安装完成 ==="
    echo ""
    echo "下一步："
    echo "1. 配置 Polyant 连接："
    echo "   export POLYANT_API_URL=http://your-node:8080"
    echo "   export POLYANT_API_KEY=your-api-key"
    echo ""
    echo "2. 或运行配置技能："
    echo "   /polyant-config (Claude Code)"
    echo "   \$polyant-config (Codex)"
    echo "   /polyant-config (Hermes)"
    echo ""
    echo "3. 生成密钥并注册（如需认证操作）："
    echo "   pactl key generate"
    echo "   pactl user register --name 'Your Name'"
}

main "$@"
```

- [ ] **Step 2: 设置脚本可执行权限并提交**

```bash
chmod +x scripts/install-unified.sh
git add scripts/install-unified.sh
git commit -m "feat(scripts): add unified agent skills installer"
```

---

### Task 13: 更新项目文档

**Files:**
- Modify: `README.md` (添加智能体集成部分)
- Modify: `SKILL.md` (更新技能列表)

- [ ] **Step 1: 在 README.md 中添加智能体集成部分**

在 README.md 的适当位置添加：

```markdown
## 智能体集成

Polyant 支持多种 AI 智能体访问知识库：

| 智能体 | 集成方式 | 状态 |
|--------|----------|------|
| Claude Code | Skills | ✅ 已支持 |
| Codex CLI | agentskills.io | ✅ 已支持 |
| Hermes Agent | agentskills.io | ✅ 已支持 |
| OpenClaw | 专用技能 | ✅ 已支持 |
| 其他 MCP 智能体 | MCP 服务器 | ✅ 已支持 |

### 快速安装

```bash
# 一键安装所有技能
./scripts/install-unified.sh
```

### MCP 服务器

对于支持 MCP 协议的智能体，可以使用 MCP 服务器：

```bash
# 安装 MCP 服务器
go install github.com/daifei0527/polyant/cmd/polyant-mcp-server@latest

# 配置 (添加到 MCP 客户端配置)
{
  "mcpServers": {
    "polyant": {
      "command": "polyant-mcp-server",
      "args": ["--config", "~/.polyant/config.json"]
    }
  }
}
```

更多详情请参阅 [智能体集成文档](docs/agent-integration.md)。
```

- [ ] **Step 2: 提交**

```bash
git add README.md SKILL.md
git commit -m "docs: update README with agent integration instructions"
```

---

## 自检清单

完成所有任务后，运行以下检查：

- [ ] SDK 测试通过: `go test ./pkg/polysdk/...`
- [ ] MCP 服务器测试通过: `go test ./cmd/polyant-mcp-server/...`
- [ ] 所有技能文件存在且格式正确
- [ ] 安装脚本可执行: `./scripts/install-unified.sh --help`
- [ ] 文档已更新
- [ ] 代码已提交

---

**计划完成。** 两种执行方式：

**1. 子代理驱动（推荐）** - 每个任务分派一个新子代理，任务间进行审查，快速迭代

**2. 内联执行** - 在当前会话中使用 executing-plans 执行任务，批量执行并设置检查点

您选择哪种方式？
