package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client API 客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
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

// SetAuthToken 设置认证令牌
func (c *Client) SetAuthToken(token string) {
	c.authToken = token
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags,omitempty"`
	Score       float64  `json:"score"`
	ScoreCount  int      `json:"score_count"`
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
	CreatedBy   string   `json:"created_by"`
}

// UserInfo 用户信息
type UserInfo struct {
	PublicKey     string `json:"public_key"`
	AgentName     string `json:"agent_name"`
	Email         string `json:"email,omitempty"`
	UserLevel     int32  `json:"user_level"`
	ContribCount  int    `json:"contrib_count"`
	RatingCount   int    `json:"rating_count"`
	CreatedAt     int64  `json:"created_at"`
	LastActiveAt  int64  `json:"last_active_at"`
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
	Running       bool     `json:"running"`
	LastSync      int64    `json:"last_sync"`
	SyncedEntries int      `json:"synced_entries"`
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
	Version      string `json:"version"`
	Uptime       int64  `json:"uptime_seconds"`
	NodeID       string `json:"node_id"`
	NodeType     string `json:"node_type"`
	NATType      string `json:"nat_type"`
	PeerCount    int    `json:"peer_count"`
	EntryCount   int    `json:"entry_count"`
	UserCount    int    `json:"user_count"`
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
