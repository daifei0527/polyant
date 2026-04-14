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
	"net/url"
	"os"
	"strconv"
	"time"
)

type KeyPair struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type TestConfig struct {
	BaseURL string
	Client  *http.Client
	UserKey *KeyPair
}

func signRequest(method, path, timestamp, body, privateKey string) string {
	privBytes, _ := base64.StdEncoding.DecodeString(privateKey)
	privKey := ed25519.PrivateKey(privBytes)

	bodyHash := sha256.Sum256([]byte(body))
	signContent := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, timestamp, hex.EncodeToString(bodyHash[:]))
	signature := ed25519.Sign(privKey, []byte(signContent))
	return base64.StdEncoding.EncodeToString(signature)
}

func doRequest(tc *TestConfig, method, path string, body interface{}) (*APIResponse, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature := signRequest(method, path, timestamp, string(bodyBytes), tc.UserKey.PrivateKey)

	reqURL := tc.BaseURL + path
	var req *http.Request
	if method == "GET" || method == "DELETE" {
		req, err = http.NewRequest(method, reqURL, nil)
	} else {
		req, err = http.NewRequest(method, reqURL, bytes.NewReader(bodyBytes))
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentWiki-PublicKey", tc.UserKey.PublicKey)
	req.Header.Set("X-AgentWiki-Timestamp", timestamp)
	req.Header.Set("X-AgentWiki-Signature", signature)

	resp, err := tc.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(respBody))
	}

	return &apiResp, nil
}

func doPublicRequest(tc *TestConfig, method, path string, body interface{}) (*APIResponse, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	reqURL := tc.BaseURL + path
	var req *http.Request
	if method == "GET" {
		req, err = http.NewRequest(method, reqURL, nil)
	} else {
		req, err = http.NewRequest(method, reqURL, bytes.NewReader(bodyBytes))
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(respBody))
	}

	return &apiResp, nil
}

func printResult(name string, resp *APIResponse, err error) bool {
	if err != nil {
		fmt.Printf("❌ %s 失败: %v\n", name, err)
		return false
	}
	if resp.Code == 0 {
		fmt.Printf("✅ %s 成功\n", name)
	} else {
		fmt.Printf("⚠️  %s 返回: code=%d, message=%s\n", name, resp.Code, resp.Message)
		return false
	}
	fmt.Printf("   响应: %s\n\n", string(resp.Data))
	return true
}

func main() {
	// 读取测试用户密钥
	keyFile, err := os.ReadFile("/tmp/test_keys.json")
	if err != nil {
		fmt.Println("请先运行前面的步骤生成测试用户密钥")
		os.Exit(1)
	}

	var keys struct {
		TestUserPublicKey  string `json:"test_user_public_key"`
		TestUserPrivateKey string `json:"test_user_private_key"`
		PublicKeyHash      string `json:"public_key_hash"`
	}
	if err := json.Unmarshal(keyFile, &keys); err != nil {
		fmt.Printf("解析密钥文件失败: %v\n", err)
		os.Exit(1)
	}

	tc := &TestConfig{
		BaseURL: "http://localhost:8080",
		Client:  &http.Client{Timeout: 30 * time.Second},
		UserKey: &KeyPair{
			PublicKey:  keys.TestUserPublicKey,
			PrivateKey: keys.TestUserPrivateKey,
		},
	}

	fmt.Println("========================================")
	fmt.Println("AgentWiki 全面功能测试")
	fmt.Println("========================================")
	fmt.Printf("测试目标: %s\n", tc.BaseURL)
	fmt.Printf("测试用户: %s...\n\n", tc.UserKey.PublicKey[:20])

	var results []string
	passCount := 0
	failCount := 0

	// ============ 测试 1: 节点状态 ============
	fmt.Println("【测试 1】获取节点状态 (公开 API)")
	resp, err := doPublicRequest(tc, "GET", "/api/v1/node/status", nil)
	if printResult("节点状态", resp, err) {
		passCount++
		results = append(results, "✅ 节点状态 API")
	} else {
		failCount++
		results = append(results, "❌ 节点状态 API")
	}

	// ============ 测试 2: 分类列表 ============
	fmt.Println("【测试 2】获取分类列表 (公开 API)")
	resp, err = doPublicRequest(tc, "GET", "/api/v1/categories", nil)
	if err == nil && resp.Code == 0 {
		var cats []map[string]interface{}
		json.Unmarshal(resp.Data, &cats)
		fmt.Printf("✅ 分类列表成功，数量: %d\n\n", len(cats))
		passCount++
		results = append(results, "✅ 分类列表 API")
	} else {
		printResult("分类列表", resp, err)
		failCount++
		results = append(results, "❌ 分类列表 API")
	}

	// ============ 测试 3: 用户信息 ============
	fmt.Println("【测试 3】获取用户信息 (认证 API)")
	resp, err = doRequest(tc, "GET", "/api/v1/user/info", nil)
	if printResult("用户信息", resp, err) {
		passCount++
		results = append(results, "✅ 用户信息 API (认证)")
	} else {
		failCount++
		results = append(results, "❌ 用户信息 API (认证)")
	}

	// ============ 测试 4: 创建条目 ============
	fmt.Println("【测试 4】创建知识条目")
	entryBody := map[string]interface{}{
		"title":    "测试条目 - Go 语言简介",
		"content":  "# Go 语言\n\nGo 是一门开源的编程语言，由 Google 开发。\n\n## 特点\n- 简洁\n- 高效\n- 并发支持",
		"category": "computer-science/programming-languages/go",
		"tags":     []string{"go", "编程", "google"},
	}
	resp, err = doRequest(tc, "POST", "/api/v1/entry/create", entryBody)
	var entryID string
	if err == nil && resp.Code == 0 {
		var entryData map[string]interface{}
		json.Unmarshal(resp.Data, &entryData)
		if id, ok := entryData["id"]; ok {
			entryID = fmt.Sprintf("%v", id)
			fmt.Printf("✅ 创建条目成功，ID: %s\n\n", entryID)
		}
		passCount++
		results = append(results, "✅ 创建条目 API")
	} else {
		printResult("创建条目", resp, err)
		failCount++
		results = append(results, "❌ 创建条目 API")
	}

	// ============ 测试 5: 搜索条目 ============
	fmt.Println("【测试 5】搜索条目")
	searchURL := "/api/v1/search?q=" + url.QueryEscape("Go")
	resp, err = doPublicRequest(tc, "GET", searchURL, nil)
	if printResult("搜索条目", resp, err) {
		passCount++
		results = append(results, "✅ 搜索条目 API")
	} else {
		failCount++
		results = append(results, "❌ 搜索条目 API")
	}

	// ============ 测试 6: 获取条目详情 ============
	if entryID != "" {
		fmt.Println("【测试 6】获取条目详情")
		resp, err = doPublicRequest(tc, "GET", "/api/v1/entry/"+entryID, nil)
		if printResult("获取条目", resp, err) {
			passCount++
			results = append(results, "✅ 获取条目详情 API")
		} else {
			failCount++
			results = append(results, "❌ 获取条目详情 API")
		}
	}

	// ============ 测试 7: 更新条目 ============
	if entryID != "" {
		fmt.Println("【测试 7】更新条目")
		updateEntryBody := map[string]interface{}{
			"title":   "测试条目 - Go 语言简介 (已更新)",
			"content": "# Go 语言\n\nGo 是一门开源的编程语言。\n\n## 更新内容\n- 新增内容",
		}
		resp, err = doRequest(tc, "POST", "/api/v1/entry/update/"+entryID, updateEntryBody)
		if printResult("更新条目", resp, err) {
			passCount++
			results = append(results, "✅ 更新条目 API")
		} else {
			failCount++
			results = append(results, "❌ 更新条目 API")
		}
	}

	// ============ 测试 8: 条目评分 ============
	if entryID != "" {
		fmt.Println("【测试 8】条目评分")
		rateBody := map[string]interface{}{
			"score":   5,
			"comment": "非常好的条目！",
		}
		resp, err = doRequest(tc, "POST", "/api/v1/entry/rate/"+entryID, rateBody)
		if printResult("条目评分", resp, err) {
			passCount++
			results = append(results, "✅ 条目评分 API")
		} else {
			failCount++
			results = append(results, "❌ 条目评分 API")
		}
	}

	// ============ 测试 9: 创建分类 ============
	fmt.Println("【测试 9】创建分类 (需要 Lv2)")
	catBody := map[string]interface{}{
		"path": "test-category-" + strconv.FormatInt(time.Now().Unix(), 10),
		"name": "测试分类",
	}
	resp, err = doRequest(tc, "POST", "/api/v1/categories/create", catBody)
	if err == nil && resp.Code == 0 {
		fmt.Printf("✅ 创建分类成功\n\n")
		passCount++
		results = append(results, "✅ 创建分类 API")
	} else {
		fmt.Printf("⚠️ 创建分类 (需要 Lv2): code=%d, message=%s\n\n", resp.Code, resp.Message)
		results = append(results, fmt.Sprintf("⚠️ 创建分类 API (需要 Lv2权限, code=%d)", resp.Code))
	}

	// ============ 测试 10: 批量创建条目 ============
	fmt.Println("【测试 10】批量创建条目")
	batchBody := map[string]interface{}{
		"entries": []map[string]interface{}{
			{
				"title":    "批量条目 1",
				"content":  "这是批量创建的第一个条目",
				"category": "computer-science",
			},
			{
				"title":    "批量条目 2",
				"content":  "这是批量创建的第二个条目",
				"category": "computer-science",
			},
		},
	}
	resp, err = doRequest(tc, "POST", "/api/v1/entries/batch", batchBody)
	if printResult("批量创建", resp, err) {
		passCount++
		results = append(results, "✅ 批量创建条目 API")
	} else {
		failCount++
		results = append(results, "❌ 批量创建条目 API")
	}

	// ============ 测试 11: 获取反向链接 ============
	if entryID != "" {
		fmt.Println("【测试 11】获取反向链接")
		resp, err = doPublicRequest(tc, "GET", "/api/v1/entry/"+entryID+"/backlinks", nil)
		if printResult("反向链接", resp, err) {
			passCount++
			results = append(results, "✅ 反向链接 API")
		} else {
			failCount++
			results = append(results, "❌ 反向链接 API")
		}
	}

	// ============ 测试 12: 删除条目 ============
	if entryID != "" {
		fmt.Println("【测试 12】删除条目")
		resp, err = doRequest(tc, "POST", "/api/v1/entry/delete/"+entryID, nil)
		if printResult("删除条目", resp, err) {
			passCount++
			results = append(results, "✅ 删除条目 API")
		} else {
			failCount++
			results = append(results, "❌ 删除条目 API")
		}
	}

	// ============ 测试总结 ============
	fmt.Println("\n========================================")
	fmt.Println("测试总结")
	fmt.Println("========================================")
	for _, r := range results {
		fmt.Println(r)
	}
	fmt.Printf("\n通过: %d, 失败: %d, 总计: %d\n", passCount, failCount, passCount+failCount)

	if failCount > 0 {
		os.Exit(1)
	}
}
