// Package polysdk 提供 Polyant API 的 Go SDK 客户端。
// 支持知识条目的搜索、创建、更新、删除和评分等操作。
package polysdk

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/daifei0527/polyant/pkg/crypto"
)

// Client Polyant API 客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	publicKey  []byte
	privateKey []byte
}

// NewClient 创建新的 API 客户端
// baseURL 为 API 基础地址，末尾斜杠会被自动去除
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetAPIKey 设置 API Key，用于公开路由的身份标识
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// SetKeys 设置 Ed25519 密钥对，用于请求签名认证
// publicKey 为 Ed25519 公钥（32 字节），privateKey 为 Ed25519 私钥（64 字节）
func (c *Client) SetKeys(publicKey, privateKey []byte) {
	c.publicKey = publicKey
	c.privateKey = privateKey
}

// HasKeys 检查是否已设置 Ed25519 密钥
func (c *Client) HasKeys() bool {
	return len(c.publicKey) > 0 && len(c.privateKey) > 0
}

// Search 搜索知识条目
// query 为搜索关键词，category 为分类过滤（可选），tags 为标签过滤（可选），
// limit 为返回数量限制，lang 为返回结果的本地化语言（可选，传 "" 则使用条目主语言）。
func (c *Client) Search(ctx context.Context, query, category string, tags []string, limit int, lang string) (*SearchResult, error) {
	// 用 url.Values 构造查询串以正确转义 query/cat/tag/lang——原先 fmt.Sprintf+拼接
	// 不会转义，含空格 / & / = / 中文等字符的查询会破坏 URL（服务端解析出错）。
	v := url.Values{}
	v.Set("q", query)
	v.Set("limit", strconv.Itoa(limit))
	if category != "" {
		v.Set("cat", category)
	}
	if len(tags) > 0 {
		v.Set("tag", strings.Join(tags, ","))
	}
	if lang != "" {
		v.Set("lang", lang)
	}
	path := "/api/v1/search?" + v.Encode()

	var result SearchResult
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEntry 获取知识条目详情。lang 为返回结果的本地化语言（可选）。
func (c *Client) GetEntry(ctx context.Context, id, lang string) (*Entry, error) {
	path := "/api/v1/entry/" + id
	if lang != "" {
		path += "?lang=" + url.QueryEscape(lang)
	}
	var result Entry
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateEntry 创建知识条目（需要 Lv1+ 认证）
func (c *Client) CreateEntry(ctx context.Context, req *CreateEntryRequest) (*Entry, error) {
	// R1-B3：用创建者私钥对内容(title,content,category)签名，服务端强制验签。
	if c.HasKeys() {
		sig, err := crypto.SignContent(c.privateKey, req.Title, req.Content, req.Category)
		if err != nil {
			return nil, fmt.Errorf("sign entry content: %w", err)
		}
		req.CreatorSignature = base64.StdEncoding.EncodeToString(sig)
	}
	var result Entry
	if err := c.doRequest(ctx, http.MethodPost, "/api/v1/entry/create", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateEntry 更新知识条目（需要认证）
func (c *Client) UpdateEntry(ctx context.Context, id string, req *UpdateEntryRequest) (*Entry, error) {
	// R1-B3：服务端按"合并后的最终 (title,content,category)"验签，故客户端需先取当前条目，
	// 合并待改字段，对完整三元组签名后发送全量内容更新（未改字段回填原值，等同 no-op）。
	if c.HasKeys() {
		cur, err := c.GetEntry(ctx, id, "")
		if err != nil {
			return nil, fmt.Errorf("fetch current entry for signing: %w", err)
		}
		title, content, category := cur.Title, cur.Content, cur.Category
		if req.Title != "" {
			title = req.Title
		}
		if req.Content != "" {
			content = req.Content
		}
		if req.Category != "" {
			category = req.Category
		}
		sig, err := crypto.SignContent(c.privateKey, title, content, category)
		if err != nil {
			return nil, fmt.Errorf("sign entry content: %w", err)
		}
		req.Title, req.Content, req.Category = title, content, category
		req.CreatorSignature = base64.StdEncoding.EncodeToString(sig)
	}
	path := "/api/v1/entry/update/" + id
	var result Entry
	if err := c.doRequest(ctx, http.MethodPut, path, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteEntry 删除知识条目（需要认证）
func (c *Client) DeleteEntry(ctx context.Context, id string) error {
	path := "/api/v1/entry/delete/" + id
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil)
}

// RateEntry 为知识条目评分（需要 Lv1+ 认证）
// score 为评分值（1.0-5.0），comment 为评语（可选）
func (c *Client) RateEntry(ctx context.Context, id string, score float64, comment string) error {
	path := "/api/v1/entry/rate/" + id
	req := &RatingRequest{
		Score:   score,
		Comment: comment,
	}
	return c.doRequest(ctx, http.MethodPost, path, req, nil)
}

// ListCategories 获取分类列表
func (c *Client) ListCategories(ctx context.Context) ([]Category, error) {
	var result []Category
	if err := c.doRequest(ctx, http.MethodGet, "/api/v1/categories", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// doRequest 执行 HTTP 请求并解析响应
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

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// API Key 认证
	if c.apiKey != "" {
		req.Header.Set("X-Polyant-Api-Key", c.apiKey)
	}

	// Ed25519 签名认证
	if c.HasKeys() {
		c.setAuthHeaders(req, reqBody)
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

	// 处理 HTTP 错误
	if resp.StatusCode >= 400 {
		var errResp APIError
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Code != 0 {
			return &Error{Code: errResp.Code, Message: errResp.Message}
		}
		return &Error{Code: resp.StatusCode, Message: string(respBody)}
	}

	// 解析响应
	if result != nil && len(respBody) > 0 {
		var apiResp APIResponse
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
		if apiResp.Data != nil {
			dataBytes, err := json.Marshal(apiResp.Data)
			if err != nil {
				return fmt.Errorf("marshal response data: %w", err)
			}
			if err := json.Unmarshal(dataBytes, result); err != nil {
				return fmt.Errorf("unmarshal response data: %w", err)
			}
		}
	}

	return nil
}

// setAuthHeaders 为请求设置 Ed25519 签名认证头
// 签名内容格式: METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + SHA256(BODY)
func (c *Client) setAuthHeaders(req *http.Request, body []byte) {
	timestamp := time.Now().UnixMilli()

	bodyHash := sha256.Sum256(body)
	signContent := fmt.Sprintf("%s\n%s\n%d\n%s",
		req.Method, req.URL.Path, timestamp, hex.EncodeToString(bodyHash[:]))

	signature := ed25519.Sign(c.privateKey, []byte(signContent))

	req.Header.Set("X-Polyant-PublicKey", base64.StdEncoding.EncodeToString(c.publicKey))
	req.Header.Set("X-Polyant-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Polyant-Signature", base64.StdEncoding.EncodeToString(signature))
}
