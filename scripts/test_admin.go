//go:build ignore

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
	req.Header.Set("X-Polyant-PublicKey", tc.UserKey.PublicKey)
	req.Header.Set("X-Polyant-Timestamp", timestamp)
	req.Header.Set("X-Polyant-Signature", signature)

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

func main() {
	keyFile, err := os.ReadFile("/tmp/test_keys.json")
	if err != nil {
		fmt.Println("请先运行前面的步骤生成测试用户密钥")
		os.Exit(1)
	}

	var keys struct {
		TestUserPublicKey  string `json:"test_user_public_key"`
		TestUserPrivateKey string `json:"test_user_private_key"`
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
	fmt.Println("管理员功能测试 (Lv5)")
	fmt.Println("========================================\n")

	var results []string
	passCount := 0
	failCount := 0

	// ============ 测试 1: 列出用户 ============
	fmt.Println("【测试 1】列出所有用户 (Lv4+)")
	resp, err := doRequest(tc, "GET", "/api/v1/admin/users", nil)
	if err != nil {
		fmt.Printf("❌ 列出用户失败: %v\n\n", err)
		failCount++
		results = append(results, "❌ 列出用户 API")
	} else if resp.Code == 0 {
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		fmt.Printf("✅ 列出用户成功\n")
		if users, ok := data["users"].([]interface{}); ok {
			fmt.Printf("   用户数量: %d\n\n", len(users))
		}
		passCount++
		results = append(results, "✅ 列出用户 API")
	} else {
		fmt.Printf("❌ 列出用户失败: code=%d, message=%s\n\n", resp.Code, resp.Message)
		failCount++
		results = append(results, "❌ 列出用户 API")
	}

	// ============ 测试 2: 获取用户统计 ============
	fmt.Println("【测试 2】获取用户统计 (Lv4+)")
	resp, err = doRequest(tc, "GET", "/api/v1/admin/stats/users", nil)
	if err == nil && resp.Code == 0 {
		fmt.Printf("✅ 用户统计成功\n")
		fmt.Printf("   响应: %s\n\n", string(resp.Data))
		passCount++
		results = append(results, "✅ 用户统计 API")
	} else {
		fmt.Printf("❌ 用户统计失败\n\n")
		failCount++
		results = append(results, "❌ 用户统计 API")
	}

	// ============ 测试 3: 导出数据 ============
	fmt.Println("【测试 3】导出数据 (Lv4+)")
	// 导出 API 直接返回 ZIP 文件，不返回 JSON
	// 注意：签名只使用路径，不包含查询参数
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature := signRequest("GET", "/api/v1/admin/export", timestamp, "", tc.UserKey.PrivateKey)
	req, _ := http.NewRequest("GET", tc.BaseURL+"/api/v1/admin/export?include=entries,categories", nil)
	req.Header.Set("X-Polyant-PublicKey", tc.UserKey.PublicKey)
	req.Header.Set("X-Polyant-Timestamp", timestamp)
	req.Header.Set("X-Polyant-Signature", signature)
	exportResp, err := tc.Client.Do(req)
	if err == nil && exportResp.StatusCode == 200 {
		exportData := make([]byte, 0)
		buf := make([]byte, 1024)
		for {
			n, err := exportResp.Body.Read(buf)
			if n > 0 {
				exportData = append(exportData, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		exportResp.Body.Close()
		fmt.Printf("✅ 数据导出成功 (ZIP 文件，大小: %d 字节)\n\n", len(exportData))
		passCount++
		results = append(results, "✅ 数据导出 API")
	} else {
		if err != nil {
			fmt.Printf("❌ 数据导出失败: %v\n\n", err)
		} else {
			fmt.Printf("❌ 数据导出失败: HTTP %d\n\n", exportResp.StatusCode)
		}
		failCount++
		results = append(results, "❌ 数据导出 API")
	}

	// ============ 测试 4: 审计日志统计 ============
	fmt.Println("【测试 4】获取审计日志统计 (Lv5)")
	resp, err = doRequest(tc, "GET", "/api/v1/admin/audit/stats", nil)
	if err == nil && resp.Code == 0 {
		fmt.Printf("✅ 审计统计成功\n")
		fmt.Printf("   响应: %s\n\n", string(resp.Data))
		passCount++
		results = append(results, "✅ 审计统计 API")
	} else {
		fmt.Printf("❌ 审计统计失败\n\n")
		failCount++
		results = append(results, "❌ 审计统计 API")
	}

	// ============ 测试 5: 查询审计日志 ============
	fmt.Println("【测试 5】查询审计日志 (Lv5)")
	// 使用不带查询参数的路径进行签名
	timestamp2 := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature2 := signRequest("GET", "/api/v1/admin/audit/logs", timestamp2, "", tc.UserKey.PrivateKey)
	req2, _ := http.NewRequest("GET", tc.BaseURL+"/api/v1/admin/audit/logs?limit=10", nil)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Polyant-PublicKey", tc.UserKey.PublicKey)
	req2.Header.Set("X-Polyant-Timestamp", timestamp2)
	req2.Header.Set("X-Polyant-Signature", signature2)
	auditResp, err := tc.Client.Do(req2)
	if err == nil && auditResp.StatusCode == 200 {
		auditData, _ := io.ReadAll(auditResp.Body)
		auditResp.Body.Close()
		var apiResp APIResponse
		if err := json.Unmarshal(auditData, &apiResp); err == nil && apiResp.Code == 0 {
			var data map[string]interface{}
			json.Unmarshal(apiResp.Data, &data)
			fmt.Printf("✅ 审计日志查询成功\n")
			if total, ok := data["total_count"].(float64); ok {
				fmt.Printf("   日志总数: %.0f\n\n", total)
			}
			passCount++
			results = append(results, "✅ 审计日志查询 API")
		} else {
			fmt.Printf("❌ 审计日志查询失败: 解析响应失败\n\n")
			failCount++
			results = append(results, "❌ 审计日志查询 API")
		}
	} else {
		if err != nil {
			fmt.Printf("❌ 审计日志查询失败: %v\n\n", err)
		} else {
			fmt.Printf("❌ 审计日志查询失败: HTTP %d\n\n", auditResp.StatusCode)
		}
		failCount++
		results = append(results, "❌ 审计日志查询 API")
	}

	// ============ 测试 6: 创建分类 (Lv2+) ============
	fmt.Println("【测试 6】创建分类 (Lv2+)")
	catPath := "test-category-" + strconv.FormatInt(time.Now().Unix(), 10)
	catBody := map[string]interface{}{
		"path": catPath,
		"name": "测试分类",
	}
	resp, err = doRequest(tc, "POST", "/api/v1/categories/create", catBody)
	if err == nil && resp.Code == 0 {
		fmt.Printf("✅ 创建分类成功: %s\n\n", catPath)
		passCount++
		results = append(results, "✅ 创建分类 API")
	} else {
		fmt.Printf("❌ 创建分类失败: code=%d, message=%s\n\n", resp.Code, resp.Message)
		failCount++
		results = append(results, "❌ 创建分类 API")
	}

	// ============ 测试总结 ============
	fmt.Println("========================================")
	fmt.Println("管理员功能测试总结")
	fmt.Println("========================================")
	for _, r := range results {
		fmt.Println(r)
	}
	fmt.Printf("\n通过: %d, 失败: %d\n", passCount, failCount)
}
