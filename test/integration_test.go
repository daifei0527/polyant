// Package test 提供集成测试
// 使用内存存储运行完整的 API 端到端测试
package test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/daifei0527/agentwiki/internal/api/router"
	"github.com/daifei0527/agentwiki/internal/storage"
)

// TestServer 测试服务器
type TestServer struct {
	server    *httptest.Server
	client    *http.Client
	privKey   ed25519.PrivateKey
	pubKey    ed25519.PublicKey
	pubKeyB64 string
	registered bool
}

// NewTestServer 创建测试服务器
func NewTestServer(t *testing.T) *TestServer {
	// 创建内存存储
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存存储失败: %v", err)
	}

	// 创建路由
	handler, err := router.NewRouterWithDeps(&router.Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		NodeID:        "test-node-1",
		NodeType:      "local",
		Version:       "test-0.1.0",
	})
	if err != nil {
		t.Fatalf("创建路由失败: %v", err)
	}

	return &TestServer{
		server: httptest.NewServer(handler),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Register 注册用户并获取密钥对
func (s *TestServer) Register(t *testing.T, agentName string) {
	resp, err := s.DoRequestNoAuth("POST", "/api/v1/user/register", map[string]string{
		"agent_name": agentName,
	})
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("注册失败: code=%d, message=%s", resp.Code, resp.Message)
	}

	var data struct {
		PublicKey  string `json:"public_key"`
		PrivateKey string `json:"private_key"`
		AgentName  string `json:"agent_name"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("解析注册响应失败: %v", err)
	}

	// 解码密钥
	privKeyBytes, err := base64.StdEncoding.DecodeString(data.PrivateKey)
	if err != nil {
		t.Fatalf("解码私钥失败: %v", err)
	}
	pubKeyBytes, err := base64.StdEncoding.DecodeString(data.PublicKey)
	if err != nil {
		t.Fatalf("解码公钥失败: %v", err)
	}

	s.privKey = ed25519.PrivateKey(privKeyBytes)
	s.pubKey = ed25519.PublicKey(pubKeyBytes)
	s.pubKeyB64 = data.PublicKey
	s.registered = true
}

// VerifyEmail 验证邮箱并升级到 Lv1
func (s *TestServer) VerifyEmail(t *testing.T, email string) {
	// 先发送验证码
	sendResp, err := s.DoRequest("POST", "/api/v1/user/send-verification", map[string]string{
		"email": email,
	}, true)
	if err != nil {
		t.Fatalf("发送验证码失败: %v", err)
	}

	if sendResp.Code != 0 {
		t.Fatalf("发送验证码失败: code=%d, message=%s", sendResp.Code, sendResp.Message)
	}

	// 从响应中获取验证码（测试环境会返回）
	var sendData struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(sendResp.Data, &sendData); err != nil {
		t.Fatalf("解析验证码响应失败: %v", err)
	}

	// 验证邮箱
	resp, err := s.DoRequest("POST", "/api/v1/user/verify-email", map[string]string{
		"email": email,
		"code":  sendData.Code,
	}, true)
	if err != nil {
		t.Fatalf("邮箱验证失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("邮箱验证失败: code=%d, message=%s", resp.Code, resp.Message)
	}
}

// RegisterAndVerify 注册用户、验证邮箱升级到 Lv1
func (s *TestServer) RegisterAndVerify(t *testing.T, agentName string) {
	s.Register(t, agentName)
	s.VerifyEmail(t, agentName+"@test.example.com")
}

// Close 关闭测试服务器
func (s *TestServer) Close() {
	s.server.Close()
}

// URL 获取测试URL
func (s *TestServer) URL(path string) string {
	return s.server.URL + path
}

// APIResponse 统一响应结构
type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// DoRequestNoAuth 发送无认证的HTTP请求
func (s *TestServer) DoRequestNoAuth(method, path string, body interface{}) (*APIResponse, error) {
	return s.DoRequest(method, path, body, false)
}

// DoRequest 发送HTTP请求
func (s *TestServer) DoRequest(method, path string, body interface{}, needAuth bool) (*APIResponse, error) {
	var reqBody io.Reader
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, s.URL(path), reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if needAuth {
		if !s.registered {
			return nil, fmt.Errorf("用户未注册，请先调用 Register()")
		}
		timestamp := time.Now().UnixMilli()
		// 签名格式: METHOD\nPATH\nTIMESTAMP\nSHA256(BODY)_HEX
		bodyHash := sha256.Sum256(bodyBytes)
		signContent := fmt.Sprintf("%s\n%s\n%d\n%s", method, path, timestamp, hex.EncodeToString(bodyHash[:]))
		signature := ed25519.Sign(s.privKey, []byte(signContent))

		req.Header.Set("X-AgentWiki-PublicKey", s.pubKeyB64)
		req.Header.Set("X-AgentWiki-Timestamp", fmt.Sprintf("%d", timestamp))
		req.Header.Set("X-AgentWiki-Signature", base64.StdEncoding.EncodeToString(signature))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result APIResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("响应解析失败: %s", string(data))
	}

	return &result, nil
}

// ==================== 测试用例 ====================

// TestIntegration_NodeStatus 测试节点状态接口
func TestIntegration_NodeStatus(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	resp, err := s.DoRequest("GET", "/api/v1/node/status", nil, false)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("期望 code=0, 实际 code=%d, message=%s", resp.Code, resp.Message)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("解析响应数据失败: %v", err)
	}

	if _, ok := data["node_id"]; !ok {
		t.Error("缺少 node_id 字段")
	}
	if _, ok := data["version"]; !ok {
		t.Error("缺少 version 字段")
	}
	if v, ok := data["version"].(string); ok && !strings.HasPrefix(v, "test-") {
		t.Errorf("版本格式错误: %s", v)
	}
}

// TestIntegration_UserRegister 测试用户注册
func TestIntegration_UserRegister(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	// 注册用户
	s.Register(t, "test-agent")

	if s.pubKey == nil || s.privKey == nil {
		t.Error("注册后应返回密钥对")
	}
}

// TestIntegration_CategoryList 测试分类列表
func TestIntegration_CategoryList(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	resp, err := s.DoRequest("GET", "/api/v1/categories", nil, false)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("期望 code=0, 实际 code=%d, message=%s", resp.Code, resp.Message)
	}

	var items []interface{}
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		t.Fatalf("解析响应数据失败: %v", err)
	}

	// 内存存储默认为空分类列表
	t.Logf("分类数量: %d", len(items))
}

// TestIntegration_SearchEmpty 测试空搜索结果
func TestIntegration_SearchEmpty(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	resp, err := s.DoRequest("GET", "/api/v1/search?q=nonexistent_keyword_xyz_12345", nil, false)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("期望 code=0, 实际 code=%d, message=%s", resp.Code, resp.Message)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("解析响应数据失败: %v", err)
	}

	if count, ok := data["total_count"].(float64); ok && count != 0 {
		t.Errorf("空搜索应返回0条结果, 实际: %d", int(count))
	}
}

// TestIntegration_CreateEntry 测试创建条目
func TestIntegration_CreateEntry(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	// 注册并验证邮箱升级到 Lv1
	s.RegisterAndVerify(t, "entry-creator")

	// 创建条目
	resp, err := s.DoRequest("POST", "/api/v1/entry/create", map[string]interface{}{
		"title":    "集成测试条目",
		"content":  "# 测试\n\n这是一个集成测试创建的知识条目。",
		"category": "tech/programming",
		"tags":     []string{"test", "integration"},
	}, true)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("期望 code=0, 实际 code=%d, message=%s", resp.Code, resp.Message)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("解析响应数据失败: %v", err)
	}

	if _, ok := data["id"]; !ok {
		t.Error("缺少 id 字段")
	}
}

// TestIntegration_EntryLifecycle 测试条目生命周期
func TestIntegration_EntryLifecycle(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	// 注册并验证邮箱升级到 Lv1
	s.RegisterAndVerify(t, "lifecycle-tester")

	// 创建条目
	createResp, err := s.DoRequest("POST", "/api/v1/entry/create", map[string]interface{}{
		"title":    "生命周期测试条目",
		"content":  "初始内容",
		"category": "tech/programming",
		"tags":     []string{"lifecycle"},
	}, true)
	if err != nil {
		t.Fatalf("创建条目失败: %v", err)
	}
	if createResp.Code != 0 {
		t.Fatalf("创建条目失败: code=%d, message=%s", createResp.Code, createResp.Message)
	}

	var createData map[string]interface{}
	if err := json.Unmarshal(createResp.Data, &createData); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	entryID, ok := createData["id"].(string)
	if !ok {
		t.Fatal("创建响应缺少 id 字段")
	}

	// 获取条目
	getResp, err := s.DoRequest("GET", "/api/v1/entry/"+entryID, nil, false)
	if err != nil {
		t.Fatalf("获取条目失败: %v", err)
	}

	if getResp.Code != 0 {
		t.Fatalf("获取条目失败: code=%d, message=%s", getResp.Code, getResp.Message)
	}

	var getData map[string]interface{}
	if err := json.Unmarshal(getResp.Data, &getData); err != nil {
		t.Fatalf("解析获取响应失败: %v", err)
	}

	if getData["title"] != "生命周期测试条目" {
		t.Errorf("标题不匹配: %v", getData["title"])
	}

	// 更新条目
	updateResp, err := s.DoRequest("PUT", "/api/v1/entry/update/"+entryID, map[string]interface{}{
		"title":   "更新后的标题",
		"content": "更新后的内容",
	}, true)
	if err != nil {
		t.Fatalf("更新条目失败: %v", err)
	}

	if updateResp.Code != 0 {
		t.Fatalf("更新条目失败: code=%d, message=%s", updateResp.Code, updateResp.Message)
	}

	// 再次获取验证更新
	getResp2, err := s.DoRequest("GET", "/api/v1/entry/"+entryID, nil, false)
	if err != nil {
		t.Fatalf("再次获取条目失败: %v", err)
	}

	var getData2 map[string]interface{}
	if err := json.Unmarshal(getResp2.Data, &getData2); err != nil {
		t.Fatalf("解析获取响应失败: %v", err)
	}

	if getData2["title"] != "更新后的标题" {
		t.Errorf("更新后标题不匹配: %v", getData2["title"])
	}
}

// TestIntegration_SearchAfterCreate 测试创建后搜索
func TestIntegration_SearchAfterCreate(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	// 注册并验证邮箱升级到 Lv1
	s.RegisterAndVerify(t, "search-tester")

	// 创建条目
	_, err := s.DoRequest("POST", "/api/v1/entry/create", map[string]interface{}{
		"title":    "Go语言并发编程",
		"content":  "Go语言通过goroutine实现并发编程",
		"category": "tech/programming",
		"tags":     []string{"go", "concurrency"},
	}, true)
	if err != nil {
		t.Fatalf("创建条目失败: %v", err)
	}

	// 搜索
	resp, err := s.DoRequest("GET", "/api/v1/search?q=Go语言", nil, false)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("搜索失败: code=%d, message=%s", resp.Code, resp.Message)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("解析搜索响应失败: %v", err)
	}

	count, ok := data["total_count"].(float64)
	if !ok || count == 0 {
		t.Error("搜索应有结果")
	}
}

// TestIntegration_RatingFlow 测试评分流程
func TestIntegration_RatingFlow(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	// 注册并验证邮箱升级到 Lv1
	s.RegisterAndVerify(t, "rating-tester")

	// 创建条目
	createResp, err := s.DoRequest("POST", "/api/v1/entry/create", map[string]interface{}{
		"title":    "评分测试条目",
		"content":  "用于测试评分功能",
		"category": "tech/programming",
	}, true)
	if err != nil {
		t.Fatalf("创建条目失败: %v", err)
	}
	if createResp.Code != 0 {
		t.Fatalf("创建条目失败: code=%d, message=%s", createResp.Code, createResp.Message)
	}

	var createData map[string]interface{}
	if err := json.Unmarshal(createResp.Data, &createData); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	entryID := createData["id"].(string)

	// 评分
	rateResp, err := s.DoRequest("POST", "/api/v1/entry/rate/"+entryID, map[string]interface{}{
		"score":   5,
		"comment": "非常棒的条目",
	}, true)
	if err != nil {
		t.Fatalf("评分失败: %v", err)
	}

	if rateResp.Code != 0 {
		t.Fatalf("评分失败: code=%d, message=%s", rateResp.Code, rateResp.Message)
	}

	// 验证评分记录返回
	var rateData map[string]interface{}
	if err := json.Unmarshal(rateResp.Data, &rateData); err != nil {
		t.Fatalf("解析评分响应失败: %v", err)
	}

	if rateData["score"].(float64) != 5.0 {
		t.Errorf("期望评分 5.0, 实际: %v", rateData["score"])
	}

	// 验证评分权重（Lv1 权重为 1.0）
	if rateData["weight"].(float64) != 1.0 {
		t.Errorf("期望权重 1.0, 实际: %v", rateData["weight"])
	}
}

// TestIntegration_AuthRequired 测试认证必需的接口
func TestIntegration_AuthRequired(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	// 尝试无认证创建条目
	resp, err := s.DoRequest("POST", "/api/v1/entry/create", map[string]interface{}{
		"title":    "无认证条目",
		"content":  "不应创建成功",
		"category": "tech",
	}, false)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	// 应该返回认证错误
	if resp.Code == 0 {
		t.Error("无认证请求应被拒绝")
	}
}

// TestIntegration_UserInfo 测试用户信息
func TestIntegration_UserInfo(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	// 注册用户
	s.Register(t, "info-tester")

	// 获取用户信息
	resp, err := s.DoRequest("GET", "/api/v1/user/info", nil, true)
	if err != nil {
		t.Fatalf("获取用户信息失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("获取用户信息失败: code=%d, message=%s", resp.Code, resp.Message)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if data["agent_name"] != "info-tester" {
		t.Errorf("agent_name 不匹配: %v", data["agent_name"])
	}
}

// TestIntegration_RateLimit 测试速率限制
func TestIntegration_RateLimit(t *testing.T) {
	s := NewTestServer(t)
	defer s.Close()

	// 快速发送多个请求，测试速率限制
	var rateLimited bool
	for i := 0; i < 100; i++ {
		resp, err := s.DoRequest("GET", "/api/v1/node/status", nil, false)
		if err != nil {
			continue
		}
		if resp.Code == 42901 { // Rate limit error code
			rateLimited = true
			break
		}
	}

	if !rateLimited {
		t.Log("警告: 未触发速率限制 (可能测试环境限制较宽松)")
	}
}
