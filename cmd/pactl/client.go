package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	authed25519 "github.com/daifei0527/polyant/internal/auth/ed25519"
)

// Client API 客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
	// Ed25519 密钥
	publicKey  []byte
	privateKey []byte
	keyDir     string
}

// NewClient 创建新的 API 客户端
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LoadOrGenerateKeys 加载现有密钥对，如果不存在则生成新的
func (c *Client) LoadOrGenerateKeys(keyDir string) error {
	c.keyDir = keyDir

	// 尝试加载现有密钥
	privKey, pubKey, err := authed25519.LoadKeyPair(keyDir)
	if err == nil {
		c.privateKey = privKey
		c.publicKey = pubKey
		return nil
	}

	// 密钥不存在，生成新的
	pubKey, privKey, err = authed25519.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("生成密钥对失败: %w", err)
	}

	// 保存密钥
	if err := authed25519.SaveKeyPair(privKey, pubKey, keyDir); err != nil {
		return fmt.Errorf("保存密钥对失败: %w", err)
	}

	c.privateKey = privKey
	c.publicKey = pubKey
	return nil
}

// HasKeys 检查是否已加载密钥
func (c *Client) HasKeys() bool {
	return c.privateKey != nil && c.publicKey != nil
}

// GetPublicKey 获取 Base64 编码的公钥
func (c *Client) GetPublicKey() string {
	if c.publicKey == nil {
		return ""
	}
	return authed25519.PublicKeyToString(c.publicKey)
}

// SignRequest 为请求生成签名
// 签名内容格式: METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + SHA256(BODY)
func (c *Client) SignRequest(method, path string, body []byte) (pubKey, timestamp, signature string, err error) {
	if c.privateKey == nil {
		return "", "", "", fmt.Errorf("未加载私钥")
	}

	// 生成时间戳（毫秒）
	timestampInt := time.Now().UnixMilli()
	timestamp = fmt.Sprintf("%d", timestampInt)

	// 计算请求体哈希
	bodyHash := sha256.Sum256(body)

	// 构造签名内容
	signContent := fmt.Sprintf("%s\n%s\n%s\n%s",
		method, path, timestamp, hex.EncodeToString(bodyHash[:]))

	// 使用 Ed25519 签名
	sig, err := authed25519.Sign(c.privateKey, []byte(signContent))
	if err != nil {
		return "", "", "", fmt.Errorf("签名失败: %w", err)
	}

	pubKey = authed25519.PublicKeyToString(c.publicKey)
	signature = hex.EncodeToString(sig)

	return pubKey, timestamp, signature, nil
}

// SetAuthHeaders 为请求设置认证头
func (c *Client) SetAuthHeaders(req *http.Request, body []byte) error {
	pubKey, timestamp, signature, err := c.SignRequest(req.Method, req.URL.Path, body)
	if err != nil {
		return err
	}

	req.Header.Set("X-Polyant-PublicKey", pubKey)
	req.Header.Set("X-Polyant-Timestamp", timestamp)
	req.Header.Set("X-Polyant-Signature", signature)

	return nil
}

// SetAuthToken 设置认证令牌
func (c *Client) SetAuthToken(token string) {
	c.authToken = token
}

// GetDefaultKeyDir 获取默认密钥目录
func GetDefaultKeyDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".polyant", "keys")
}

// EnsureKeyDirExists 确保密钥目录存在
func EnsureKeyDirExists() error {
	keyDir := GetDefaultKeyDir()
	return os.MkdirAll(filepath.Dir(keyDir), 0700)
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	return c.doRequestWithAuth(ctx, method, path, body, result, false)
}

// doRequestWithAuth 执行 HTTP 请求，可选强制认证
func (c *Client) doRequestWithAuth(ctx context.Context, method, path string, body interface{}, result interface{}, requireAuth bool) error {
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

	// 如果有密钥，添加认证头
	if c.HasKeys() {
		if err := c.SetAuthHeaders(req, reqBody); err != nil {
			return fmt.Errorf("设置认证头失败: %w", err)
		}
	} else if requireAuth {
		return fmt.Errorf("此操作需要认证，请先运行 'pactl key generate' 生成密钥")
	}

	// 兼容旧的 authToken
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
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
		if json.Unmarshal(respBody, &errResp) == nil {
			return fmt.Errorf("API error: %s (code: %d)", errResp.Message, errResp.Code)
		}
		return fmt.Errorf("HTTP error: %d - %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// Get 执行 GET 请求
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, result)
}

// Post 执行 POST 请求
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, body, result)
}

// Put 执行 PUT 请求
func (c *Client) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPut, path, body, result)
}

// Delete 执行 DELETE 请求
func (c *Client) Delete(ctx context.Context, path string, result interface{}) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, result)
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

// EntryInfo 条目信息
type EntryInfo struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags,omitempty"`
	Score      float64  `json:"score"`
	ScoreCount int      `json:"score_count"`
	CreatedAt  int64    `json:"created_at"`
	UpdatedAt  int64    `json:"updated_at"`
	CreatedBy  string   `json:"created_by"`
}

// UserInfo 用户信息
type UserInfo struct {
	PublicKey    string `json:"public_key"`
	AgentName    string `json:"agent_name"`
	Email        string `json:"email,omitempty"`
	UserLevel    int32  `json:"user_level"`
	ContribCount int    `json:"contrib_count"`
	RatingCount  int    `json:"rating_count"`
	CreatedAt    int64  `json:"created_at"`
	LastActiveAt int64  `json:"last_active_at"`
}

// CategoryInfo 分类信息
type CategoryInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
	CreatedAt   int64  `json:"created_at"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	Running        bool       `json:"running"`
	LastSync       int64      `json:"last_sync"`
	SyncedEntries  int        `json:"synced_entries"`
	ConnectedPeers []PeerInfo `json:"connected_peers,omitempty"`
}

// PeerInfo 节点信息
type PeerInfo struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Latency int64  `json:"latency_ms"`
}

// ServerStatus 服务器状态
type ServerStatus struct {
	Version    string `json:"version"`
	Uptime     int64  `json:"uptime_seconds"`
	NodeID     string `json:"node_id"`
	NodeType   string `json:"node_type"`
	NATType    string `json:"nat_type"`
	PeerCount  int    `json:"peer_count"`
	EntryCount int    `json:"entry_count"`
	UserCount  int    `json:"user_count"`
}

// GetStatus 获取服务器状态
func (c *Client) GetStatus(ctx context.Context) (*ServerStatus, error) {
	var resp APIResponse
	if err := c.Get(ctx, "/api/v1/status", &resp); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	status := &ServerStatus{}
	if v, ok := data["version"].(string); ok {
		status.Version = v
	}
	if v, ok := data["uptime_seconds"].(float64); ok {
		status.Uptime = int64(v)
	}
	if v, ok := data["node_id"].(string); ok {
		status.NodeID = v
	}
	if v, ok := data["node_type"].(string); ok {
		status.NodeType = v
	}
	if v, ok := data["nat_type"].(string); ok {
		status.NATType = v
	}
	if v, ok := data["peer_count"].(float64); ok {
		status.PeerCount = int(v)
	}
	if v, ok := data["entry_count"].(float64); ok {
		status.EntryCount = int(v)
	}
	if v, ok := data["user_count"].(float64); ok {
		status.UserCount = int(v)
	}

	return status, nil
}

// ListEntries 列出条目
func (c *Client) ListEntries(ctx context.Context, category string, limit, offset int) ([]EntryInfo, int, error) {
	path := fmt.Sprintf("/api/v1/entries?limit=%d&offset=%d", limit, offset)
	if category != "" {
		path += "&cat=" + category
	}

	var resp APIResponse
	if err := c.Get(ctx, path, &resp); err != nil {
		return nil, 0, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid response data")
	}

	var totalCount int
	if v, ok := data["total_count"].(float64); ok {
		totalCount = int(v)
	}

	var entries []EntryInfo
	if items, ok := data["items"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				entry := EntryInfo{}
				if v, ok := m["id"].(string); ok {
					entry.ID = v
				}
				if v, ok := m["title"].(string); ok {
					entry.Title = v
				}
				if v, ok := m["category"].(string); ok {
					entry.Category = v
				}
				if v, ok := m["score"].(float64); ok {
					entry.Score = v
				}
				if v, ok := m["score_count"].(float64); ok {
					entry.ScoreCount = int(v)
				}
				if v, ok := m["created_at"].(float64); ok {
					entry.CreatedAt = int64(v)
				}
				if v, ok := m["updated_at"].(float64); ok {
					entry.UpdatedAt = int64(v)
				}
				if v, ok := m["created_by"].(string); ok {
					entry.CreatedBy = v
				}
				if v, ok := m["tags"].([]interface{}); ok {
					for _, t := range v {
						if s, ok := t.(string); ok {
							entry.Tags = append(entry.Tags, s)
						}
					}
				}
				entries = append(entries, entry)
			}
		}
	}

	return entries, totalCount, nil
}

// GetEntry 获取条目详情
func (c *Client) GetEntry(ctx context.Context, id string) (*EntryInfo, error) {
	var resp APIResponse
	if err := c.Get(ctx, "/api/v1/entry/"+id, &resp); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	entry := &EntryInfo{}
	if v, ok := data["id"].(string); ok {
		entry.ID = v
	}
	if v, ok := data["title"].(string); ok {
		entry.Title = v
	}
	if v, ok := data["category"].(string); ok {
		entry.Category = v
	}
	if v, ok := data["score"].(float64); ok {
		entry.Score = v
	}
	if v, ok := data["score_count"].(float64); ok {
		entry.ScoreCount = int(v)
	}
	if v, ok := data["created_at"].(float64); ok {
		entry.CreatedAt = int64(v)
	}
	if v, ok := data["updated_at"].(float64); ok {
		entry.UpdatedAt = int64(v)
	}
	if v, ok := data["created_by"].(string); ok {
		entry.CreatedBy = v
	}
	if v, ok := data["tags"].([]interface{}); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				entry.Tags = append(entry.Tags, s)
			}
		}
	}

	return entry, nil
}

// SearchEntries 搜索条目
func (c *Client) SearchEntries(ctx context.Context, query string, limit int) ([]EntryInfo, int, error) {
	path := fmt.Sprintf("/api/v1/search?q=%s&limit=%d", query, limit)

	var resp APIResponse
	if err := c.Get(ctx, path, &resp); err != nil {
		return nil, 0, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid response data")
	}

	var totalCount int
	if v, ok := data["total_count"].(float64); ok {
		totalCount = int(v)
	}

	var entries []EntryInfo
	if items, ok := data["items"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				entry := EntryInfo{}
				if v, ok := m["id"].(string); ok {
					entry.ID = v
				}
				if v, ok := m["title"].(string); ok {
					entry.Title = v
				}
				if v, ok := m["category"].(string); ok {
					entry.Category = v
				}
				if v, ok := m["score"].(float64); ok {
					entry.Score = v
				}
				if v, ok := m["score_count"].(float64); ok {
					entry.ScoreCount = int(v)
				}
				if v, ok := m["created_at"].(float64); ok {
					entry.CreatedAt = int64(v)
				}
				if v, ok := m["updated_at"].(float64); ok {
					entry.UpdatedAt = int64(v)
				}
				if v, ok := m["created_by"].(string); ok {
					entry.CreatedBy = v
				}
				entries = append(entries, entry)
			}
		}
	}

	return entries, totalCount, nil
}

// GetUser 获取用户信息
func (c *Client) GetUser(ctx context.Context, pubKey string) (*UserInfo, error) {
	var resp APIResponse
	if err := c.Get(ctx, "/api/v1/user/"+pubKey, &resp); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	user := &UserInfo{}
	if v, ok := data["public_key"].(string); ok {
		user.PublicKey = v
	}
	if v, ok := data["agent_name"].(string); ok {
		user.AgentName = v
	}
	if v, ok := data["email"].(string); ok {
		user.Email = v
	}
	if v, ok := data["user_level"].(float64); ok {
		user.UserLevel = int32(v)
	}
	if v, ok := data["contrib_count"].(float64); ok {
		user.ContribCount = int(v)
	}
	if v, ok := data["rating_count"].(float64); ok {
		user.RatingCount = int(v)
	}
	if v, ok := data["created_at"].(float64); ok {
		user.CreatedAt = int64(v)
	}
	if v, ok := data["last_active_at"].(float64); ok {
		user.LastActiveAt = int64(v)
	}

	return user, nil
}

// ListCategories 列出分类
func (c *Client) ListCategories(ctx context.Context) ([]CategoryInfo, error) {
	var resp APIResponse
	if err := c.Get(ctx, "/api/v1/categories", &resp); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	var categories []CategoryInfo
	if items, ok := data["items"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				cat := CategoryInfo{}
				if v, ok := m["id"].(string); ok {
					cat.ID = v
				}
				if v, ok := m["name"].(string); ok {
					cat.Name = v
				}
				if v, ok := m["description"].(string); ok {
					cat.Description = v
				}
				if v, ok := m["parent_id"].(string); ok {
					cat.ParentID = v
				}
				if v, ok := m["created_at"].(float64); ok {
					cat.CreatedAt = int64(v)
				}
				categories = append(categories, cat)
			}
		}
	}

	return categories, nil
}

// GetSyncStatus 获取同步状态
func (c *Client) GetSyncStatus(ctx context.Context) (*SyncStatus, error) {
	var resp APIResponse
	if err := c.Get(ctx, "/api/v1/sync/status", &resp); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	status := &SyncStatus{}
	if v, ok := data["running"].(bool); ok {
		status.Running = v
	}
	if v, ok := data["last_sync"].(float64); ok {
		status.LastSync = int64(v)
	}
	if v, ok := data["synced_entries"].(float64); ok {
		status.SyncedEntries = int(v)
	}

	return status, nil
}

// ========== 条目操作 API ==========

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

// RateEntryRequest 评分请求
type RateEntryRequest struct {
	Score   float64 `json:"score"`
	Comment string  `json:"comment,omitempty"`
}

// EntryDetail 条目详情（包含内容）
type EntryDetail struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags,omitempty"`
	Score      float64  `json:"score"`
	ScoreCount int      `json:"score_count"`
	CreatedAt  int64    `json:"created_at"`
	UpdatedAt  int64    `json:"updated_at"`
	CreatedBy  string   `json:"created_by"`
}

// CreateEntry 创建条目（需要 Lv1+ 认证）
func (c *Client) CreateEntry(ctx context.Context, req *CreateEntryRequest) (*EntryDetail, error) {
	var resp APIResponse
	if err := c.doRequestWithAuth(ctx, http.MethodPost, "/api/v1/entry/create", req, &resp, true); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	return parseEntryDetail(data), nil
}

// UpdateEntry 更新条目（需要认证）
func (c *Client) UpdateEntry(ctx context.Context, id string, req *UpdateEntryRequest) (*EntryDetail, error) {
	var resp APIResponse
	path := fmt.Sprintf("/api/v1/entry/update/%s", id)
	if err := c.doRequestWithAuth(ctx, http.MethodPut, path, req, &resp, true); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	return parseEntryDetail(data), nil
}

// DeleteEntry 删除条目（需要认证）
func (c *Client) DeleteEntry(ctx context.Context, id string) error {
	var resp APIResponse
	path := fmt.Sprintf("/api/v1/entry/delete/%s", id)
	return c.doRequestWithAuth(ctx, http.MethodDelete, path, nil, &resp, true)
}

// RateEntry 为条目评分（需要 Lv1+ 认证）
func (c *Client) RateEntry(ctx context.Context, entryID string, score float64, comment string) error {
	req := &RateEntryRequest{
		Score:   score,
		Comment: comment,
	}
	var resp APIResponse
	path := fmt.Sprintf("/api/v1/entry/rate/%s", entryID)
	return c.doRequestWithAuth(ctx, http.MethodPost, path, req, &resp, true)
}

// GetBacklinks 获取条目的反向链接
func (c *Client) GetBacklinks(ctx context.Context, entryID string) ([]string, error) {
	var resp APIResponse
	path := fmt.Sprintf("/api/v1/entry/%s/backlinks", entryID)
	if err := c.Get(ctx, path, &resp); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	var backlinks []string
	if items, ok := data["backlinks"].([]interface{}); ok {
		for _, item := range items {
			if s, ok := item.(string); ok {
				backlinks = append(backlinks, s)
			}
		}
	}

	return backlinks, nil
}

// GetOutlinks 获取条目的正向链接
func (c *Client) GetOutlinks(ctx context.Context, entryID string) ([]string, error) {
	var resp APIResponse
	path := fmt.Sprintf("/api/v1/entry/%s/outlinks", entryID)
	if err := c.Get(ctx, path, &resp); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	var outlinks []string
	if items, ok := data["outlinks"].([]interface{}); ok {
		for _, item := range items {
			if s, ok := item.(string); ok {
				outlinks = append(outlinks, s)
			}
		}
	}

	return outlinks, nil
}

func parseEntryDetail(data map[string]interface{}) *EntryDetail {
	entry := &EntryDetail{}
	if v, ok := data["id"].(string); ok {
		entry.ID = v
	}
	if v, ok := data["title"].(string); ok {
		entry.Title = v
	}
	if v, ok := data["content"].(string); ok {
		entry.Content = v
	}
	if v, ok := data["category"].(string); ok {
		entry.Category = v
	}
	if v, ok := data["score"].(float64); ok {
		entry.Score = v
	}
	if v, ok := data["score_count"].(float64); ok {
		entry.ScoreCount = int(v)
	}
	if v, ok := data["created_at"].(float64); ok {
		entry.CreatedAt = int64(v)
	}
	if v, ok := data["updated_at"].(float64); ok {
		entry.UpdatedAt = int64(v)
	}
	if v, ok := data["created_by"].(string); ok {
		entry.CreatedBy = v
	}
	if v, ok := data["tags"].([]interface{}); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				entry.Tags = append(entry.Tags, s)
			}
		}
	}
	return entry
}

// ========== 用户操作 API ==========

// RegisterRequest 注册请求
type RegisterRequest struct {
	PublicKey string `json:"public_key"`
	AgentName string `json:"agent_name,omitempty"`
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	PublicKey string `json:"public_key"`
	AgentName string `json:"agent_name"`
	UserLevel int32  `json:"user_level"`
	CreatedAt int64  `json:"created_at"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	AgentName string `json:"agent_name"`
}

// RegisterUser 注册新用户（无需认证）
func (c *Client) RegisterUser(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	var resp APIResponse
	if err := c.Post(ctx, "/api/v1/user/register", req, &resp); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	result := &RegisterResponse{}
	if v, ok := data["public_key"].(string); ok {
		result.PublicKey = v
	}
	if v, ok := data["agent_name"].(string); ok {
		result.AgentName = v
	}
	if v, ok := data["user_level"].(float64); ok {
		result.UserLevel = int32(v)
	}
	if v, ok := data["created_at"].(float64); ok {
		result.CreatedAt = int64(v)
	}

	return result, nil
}

// GetCurrentUserInfo 获取当前用户信息（需要认证）
func (c *Client) GetCurrentUserInfo(ctx context.Context) (*UserInfo, error) {
	var resp APIResponse
	if err := c.doRequestWithAuth(ctx, http.MethodGet, "/api/v1/user/info", nil, &resp, true); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	return parseUserInfo(data), nil
}

// UpdateUserInfo 更新用户信息（需要认证）
func (c *Client) UpdateUserInfo(ctx context.Context, agentName string) error {
	req := &UpdateUserRequest{
		AgentName: agentName,
	}
	var resp APIResponse
	return c.doRequestWithAuth(ctx, http.MethodPut, "/api/v1/user/update", req, &resp, true)
}

// SendVerificationCode 发送邮箱验证码（需要认证）
func (c *Client) SendVerificationCode(ctx context.Context, email string) error {
	req := map[string]string{"email": email}
	var resp APIResponse
	return c.doRequestWithAuth(ctx, http.MethodPost, "/api/v1/user/send-verification", req, &resp, true)
}

// VerifyEmail 验证邮箱（需要认证）
func (c *Client) VerifyEmail(ctx context.Context, email, code string) error {
	req := map[string]string{
		"email": email,
		"code":  code,
	}
	var resp APIResponse
	return c.doRequestWithAuth(ctx, http.MethodPost, "/api/v1/user/verify-email", req, &resp, true)
}

func parseUserInfo(data map[string]interface{}) *UserInfo {
	user := &UserInfo{}
	if v, ok := data["public_key"].(string); ok {
		user.PublicKey = v
	}
	if v, ok := data["agent_name"].(string); ok {
		user.AgentName = v
	}
	if v, ok := data["email"].(string); ok {
		user.Email = v
	}
	if v, ok := data["user_level"].(float64); ok {
		user.UserLevel = int32(v)
	}
	if v, ok := data["contrib_count"].(float64); ok {
		user.ContribCount = int(v)
	}
	if v, ok := data["rating_count"].(float64); ok {
		user.RatingCount = int(v)
	}
	if v, ok := data["created_at"].(float64); ok {
		user.CreatedAt = int64(v)
	}
	if v, ok := data["last_active_at"].(float64); ok {
		user.LastActiveAt = int64(v)
	}
	return user
}

// ========== 分类操作 API ==========

// CreateCategoryRequest 创建分类请求
type CreateCategoryRequest struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
}

// GetCategoryEntries 获取分类下的条目
func (c *Client) GetCategoryEntries(ctx context.Context, path string, limit, offset int) ([]EntryInfo, int, error) {
	urlPath := fmt.Sprintf("/api/v1/categories/%s/entries?limit=%d&offset=%d", path, limit, offset)

	var resp APIResponse
	if err := c.Get(ctx, urlPath, &resp); err != nil {
		return nil, 0, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid response data")
	}

	var totalCount int
	if v, ok := data["total_count"].(float64); ok {
		totalCount = int(v)
	}

	var entries []EntryInfo
	if items, ok := data["items"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				entries = append(entries, parseEntryInfo(m))
			}
		}
	}

	return entries, totalCount, nil
}

// CreateCategory 创建分类（需要 Lv2+ 认证）
func (c *Client) CreateCategory(ctx context.Context, path, name, parentID string) (*CategoryInfo, error) {
	req := &CreateCategoryRequest{
		Path:     path,
		Name:     name,
		ParentID: parentID,
	}

	var resp APIResponse
	if err := c.doRequestWithAuth(ctx, http.MethodPost, "/api/v1/categories/create", req, &resp, true); err != nil {
		return nil, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	cat := &CategoryInfo{}
	if v, ok := data["id"].(string); ok {
		cat.ID = v
	}
	if v, ok := data["name"].(string); ok {
		cat.Name = v
	}
	if v, ok := data["parent_id"].(string); ok {
		cat.ParentID = v
	}
	if v, ok := data["created_at"].(float64); ok {
		cat.CreatedAt = int64(v)
	}

	return cat, nil
}

func parseEntryInfo(m map[string]interface{}) EntryInfo {
	entry := EntryInfo{}
	if v, ok := m["id"].(string); ok {
		entry.ID = v
	}
	if v, ok := m["title"].(string); ok {
		entry.Title = v
	}
	if v, ok := m["category"].(string); ok {
		entry.Category = v
	}
	if v, ok := m["score"].(float64); ok {
		entry.Score = v
	}
	if v, ok := m["score_count"].(float64); ok {
		entry.ScoreCount = int(v)
	}
	if v, ok := m["created_at"].(float64); ok {
		entry.CreatedAt = int64(v)
	}
	if v, ok := m["updated_at"].(float64); ok {
		entry.UpdatedAt = int64(v)
	}
	if v, ok := m["created_by"].(string); ok {
		entry.CreatedBy = v
	}
	if v, ok := m["tags"].([]interface{}); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				entry.Tags = append(entry.Tags, s)
			}
		}
	}
	return entry
}

// ========== 同步操作 API ==========

// TriggerSync 触发手动同步（需要认证）
func (c *Client) TriggerSync(ctx context.Context) error {
	var resp APIResponse
	return c.doRequestWithAuth(ctx, http.MethodPost, "/api/v1/node/sync", nil, &resp, true)
}

// ========== 管理员操作 API ==========

// UserListItem 用户列表项
type UserListItem struct {
	PublicKey       string `json:"public_key"`
	AgentName       string `json:"agent_name"`
	UserLevel       int32  `json:"user_level"`
	Status          string `json:"status"`
	ContributionCnt int    `json:"contribution_cnt"`
	RatingCnt       int    `json:"rating_cnt"`
	CreatedAt       int64  `json:"created_at"`
	LastActive      int64  `json:"last_active_at"`
}

// ListUsers 列出用户（需要管理员权限）
func (c *Client) ListUsers(ctx context.Context, page, limit int, level int32, search string) ([]UserListItem, int, error) {
	path := fmt.Sprintf("/api/v1/admin/users?page=%d&limit=%d", page, limit)
	if level >= 0 {
		path += fmt.Sprintf("&level=%d", level)
	}
	if search != "" {
		path += "&search=" + search
	}

	var resp APIResponse
	if err := c.doRequestWithAuth(ctx, http.MethodGet, path, nil, &resp, true); err != nil {
		return nil, 0, err
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid response data")
	}

	var total int
	if v, ok := data["total"].(float64); ok {
		total = int(v)
	}

	var users []UserListItem
	if items, ok := data["users"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				user := UserListItem{}
				if v, ok := m["public_key"].(string); ok {
					user.PublicKey = v
				}
				if v, ok := m["agent_name"].(string); ok {
					user.AgentName = v
				}
				if v, ok := m["user_level"].(float64); ok {
					user.UserLevel = int32(v)
				}
				if v, ok := m["status"].(string); ok {
					user.Status = v
				}
				if v, ok := m["contribution_cnt"].(float64); ok {
					user.ContributionCnt = int(v)
				}
				if v, ok := m["rating_cnt"].(float64); ok {
					user.RatingCnt = int(v)
				}
				if v, ok := m["created_at"].(float64); ok {
					user.CreatedAt = int64(v)
				}
				if v, ok := m["last_active_at"].(float64); ok {
					user.LastActive = int64(v)
				}
				users = append(users, user)
			}
		}
	}

	return users, total, nil
}

// SetUserLevel 设置用户等级（需要超级管理员权限）
func (c *Client) SetUserLevel(ctx context.Context, publicKey string, level int32, reason string) error {
	req := map[string]interface{}{
		"level":  level,
		"reason": reason,
	}
	path := fmt.Sprintf("/api/v1/admin/users/%s/level", publicKey)
	var resp APIResponse
	return c.doRequestWithAuth(ctx, http.MethodPut, path, req, &resp, true)
}
