// Package main 提供集成测试脚本
// 运行方式: go run scripts/integration_check.go
package main

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
	"os"
	"strings"
	"time"
)

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

const baseURL = "http://127.0.0.1:18531/api/v1"

// 统一响应结构
type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// 注册响应
type RegisterData struct {
	AgentName   string `json:"agent_name"`
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
	PublicKeyHash string `json:"public_key_hash"`
	UserLevel   int    `json:"user_level"`
}

var (
	testPrivateKey ed25519.PrivateKey
	testPublicKey  ed25519.PublicKey
)

func main() {
	passed := 0
	failed := 0

	// 先注册用户获取密钥
	if err := setupTestUser(); err != nil {
		fmt.Printf("❌ 用户注册失败: %v\n", err)
		os.Exit(1)
	}

	tests := []struct {
		name string
		fn   func() error
	}{
		{"节点状态", testNodeStatus},
		{"分类列表", testCategoryList},
		{"搜索空结果", testSearchEmpty},
		{"创建知识条目", testCreateEntry},
		{"搜索命中", testSearchHit},
		{"获取条目详情", testGetEntry},
		{"获取分类下条目", testCategoryEntries},
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║       AgentWiki 集成测试                    ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println()

	for _, tc := range tests {
		fmt.Printf("▶ %s ... ", tc.name)
		if err := tc.fn(); err != nil {
			fmt.Printf("❌ 失败: %v\n", err)
			failed++
		} else {
			fmt.Println("✅ 通过")
			passed++
		}
	}

	fmt.Println()
	fmt.Printf("══════════════════════════════════════════════\n")
	fmt.Printf("总计: %d  通过: %d  失败: %d\n", passed+failed, passed, failed)
	fmt.Printf("══════════════════════════════════════════════\n")

	if failed > 0 {
		os.Exit(1)
	}
}

func setupTestUser() error {
	fmt.Print("▶ 用户注册 ... ")
	resp, err := doRequest("POST", "/user/register", map[string]string{
		"agent_name": "integration-test-agent",
	})
	if err != nil {
		fmt.Printf("❌ 失败: %v\n", err)
		return err
	}
	if resp.Code != 0 {
		fmt.Printf("❌ 失败: code=%d msg=%s\n", resp.Code, resp.Message)
		return fmt.Errorf("register failed: %s", resp.Message)
	}

	var data RegisterData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return err
	}

	// 解码密钥
	privKeyBytes, err := base64.StdEncoding.DecodeString(data.PrivateKey)
	if err != nil {
		return fmt.Errorf("私钥解码失败: %v", err)
	}
	pubKeyBytes, err := base64.StdEncoding.DecodeString(data.PublicKey)
	if err != nil {
		return fmt.Errorf("公钥解码失败: %v", err)
	}

	testPrivateKey = ed25519.PrivateKey(privKeyBytes)
	testPublicKey = ed25519.PublicKey(pubKeyBytes)

	fmt.Println("✅ 通过")
	return nil
}

// doRequest 发送HTTP请求（公开接口，无需签名）
func doRequest(method, path string, body interface{}) (*APIResponse, error) {
	return doSignedRequest(method, path, body, false)
}

// doSignedRequest 发送HTTP请求（可选签名）
func doSignedRequest(method, path string, body interface{}, needSign bool) (*APIResponse, error) {
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

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if needSign && testPrivateKey != nil {
		timestamp := time.Now().UnixMilli()
		bodyHash := sha256.Sum256(bodyBytes)
		// 签名路径必须包含完整API路径前缀
		fullPath := "/api/v1" + path
		signContent := fmt.Sprintf("%s\n%s\n%d\n%s",
			method, fullPath, timestamp, hex.EncodeToString(bodyHash[:]))
		signature := ed25519.Sign(testPrivateKey, []byte(signContent))

		req.Header.Set("X-AgentWiki-PublicKey", base64.StdEncoding.EncodeToString(testPublicKey))
		req.Header.Set("X-AgentWiki-Timestamp", fmt.Sprintf("%d", timestamp))
		req.Header.Set("X-AgentWiki-Signature", base64.StdEncoding.EncodeToString(signature))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
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

func testNodeStatus() error {
	resp, err := doRequest("GET", "/node/status", nil)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("code=%d msg=%s", resp.Code, resp.Message)
	}
	var data map[string]interface{}
	json.Unmarshal(resp.Data, &data)
	if _, ok := data["node_id"]; !ok {
		return fmt.Errorf("缺少 node_id 字段")
	}
	if _, ok := data["version"]; !ok {
		return fmt.Errorf("缺少 version 字段")
	}
	return nil
}

func testCategoryList() error {
	resp, err := doRequest("GET", "/categories", nil)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("code=%d msg=%s", resp.Code, resp.Message)
	}
	var data []interface{}
	json.Unmarshal(resp.Data, &data)
	if len(data) < 10 {
		return fmt.Errorf("分类数量不足: %d", len(data))
	}
	return nil
}

func testSearchEmpty() error {
	resp, err := doRequest("GET", "/search?q=nonexistent_keyword_xyz_12345", nil)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("code=%d msg=%s", resp.Code, resp.Message)
	}
	var data map[string]interface{}
	json.Unmarshal(resp.Data, &data)
	if count, ok := data["total_count"].(float64); ok && count != 0 {
		return fmt.Errorf("空搜索应返回0条结果，实际: %d", int(count))
	}
	return nil
}

func testCreateEntry() error {
	// 先验证邮箱升级为Lv1正式用户
	email := "test@example.com"
	token := sha256Hex(email + "agentwiki-email-verification-secret")[:16]
	verifyResp, err := doSignedRequest("POST", "/user/verify-email", map[string]string{
		"email": email,
		"token": token,
	}, true)
	if err != nil {
		return fmt.Errorf("邮箱验证请求失败: %v", err)
	}
	if verifyResp.Code != 0 {
		return fmt.Errorf("邮箱验证失败: code=%d msg=%s", verifyResp.Code, verifyResp.Message)
	}

	// 现在创建条目
	resp, err := doSignedRequest("POST", "/entry/create", map[string]interface{}{
		"title":    "集成测试条目-" + time.Now().Format("150405"),
		"content":  "# 测试\n\n这是一个集成测试创建的知识条目。\n\n## Go语言\n\nGo是一种编译型语言。",
		"category": "computer-science/programming-languages/go",
		"tags":     []string{"test", "integration", "go"},
		"json_data": []map[string]interface{}{
			{"type": "skill_definition", "name": "test_skill", "description": "测试技能"},
		},
	}, true)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("code=%d msg=%s", resp.Code, resp.Message)
	}
	return nil
}

func testSearchHit() error {
	resp, err := doRequest("GET", "/search?q=测试&limit=5", nil)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("code=%d msg=%s", resp.Code, resp.Message)
	}
	var data map[string]interface{}
	json.Unmarshal(resp.Data, &data)
	if count, ok := data["total_count"].(float64); ok && count == 0 {
		return fmt.Errorf("搜索'测试'应有结果")
	}
	return nil
}

func testGetEntry() error {
	// 先搜索获取一个条目ID
	resp, err := doRequest("GET", "/search?q=Go语言&limit=1", nil)
	if err != nil {
		return err
	}
	var data map[string]interface{}
	json.Unmarshal(resp.Data, &data)
	items, ok := data["items"].([]interface{})
	if !ok || len(items) == 0 {
		return fmt.Errorf("没有可获取的条目")
	}
	item := items[0].(map[string]interface{})
	entryID, ok := item["id"].(string)
	if !ok {
		return fmt.Errorf("条目缺少 id 字段")
	}

	// 获取条目详情 - 需要使用完整路径
	resp, err = doRequest("GET", "/entry/"+entryID, nil)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("code=%d msg=%s", resp.Code, resp.Message)
	}

	var entry map[string]interface{}
	json.Unmarshal(resp.Data, &entry)
	if _, ok := entry["title"]; !ok {
		return fmt.Errorf("条目缺少 title 字段")
	}
	title, _ := entry["title"].(string)
	if !strings.Contains(title, "测试") {
		return fmt.Errorf("条目标题不匹配: %s", title)
	}
	return nil
}

func testCategoryEntries() error {
	// 用搜索API按分类过滤来验证
	resp, err := doRequest("GET", "/search?q=Go语言&cat=computer-science/programming-languages/go&limit=10", nil)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("code=%d msg=%s", resp.Code, resp.Message)
	}
	return nil
}
