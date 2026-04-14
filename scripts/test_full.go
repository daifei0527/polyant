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

type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type TestContext struct {
	BaseURL       string
	Client        *http.Client
	AdminPubKey   string
	AdminPrivKey  string
	TestUserKey   string // 普通测试用户公钥
	TestUserPriv  string // 普通测试用户私钥
}

func signRequest(method, path, timestamp, body, privateKey string) string {
	privBytes, _ := base64.StdEncoding.DecodeString(privateKey)
	privKey := ed25519.PrivateKey(privBytes)
	bodyHash := sha256.Sum256([]byte(body))
	signContent := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, timestamp, hex.EncodeToString(bodyHash[:]))
	signature := ed25519.Sign(privKey, []byte(signContent))
	return base64.StdEncoding.EncodeToString(signature)
}

func doRequest(tc *TestContext, method, path string, body interface{}) (*APIResponse, error) {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	return doRequestWithKey(tc, method, path, string(bodyBytes), tc.AdminPubKey, tc.AdminPrivKey)
}

func doRequestWithKey(tc *TestContext, method, path, body, pubKey, privKey string) (*APIResponse, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature := signRequest(method, path, timestamp, body, privKey)

	reqURL := tc.BaseURL + path
	var req *http.Request
	var err error
	if method == "GET" || method == "DELETE" {
		req, err = http.NewRequest(method, reqURL, nil)
	} else {
		req, err = http.NewRequest(method, reqURL, bytes.NewReader([]byte(body)))
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentWiki-PublicKey", pubKey)
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

func doPublicRequest(tc *TestContext, method, path string, body interface{}) (*APIResponse, error) {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}

	reqURL := tc.BaseURL + path
	var req *http.Request
	var err error
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

func main() {
	// 读取管理员密钥
	keyFile, err := os.ReadFile("/tmp/test_keys.json")
	if err != nil {
		fmt.Println("请先运行前面的步骤生成测试用户密钥")
		os.Exit(1)
	}

	var keys struct {
		TestUserPublicKey  string `json:"test_user_public_key"`
		TestUserPrivateKey string `json:"test_user_private_key"`
	}
	json.Unmarshal(keyFile, &keys)

	tc := &TestContext{
		BaseURL:      "http://localhost:8080",
		Client:       &http.Client{Timeout: 30 * time.Second},
		AdminPubKey:  keys.TestUserPublicKey,
		AdminPrivKey: keys.TestUserPrivateKey,
	}

	fmt.Println("========================================")
	fmt.Println("AgentWiki 完整功能测试 - Part 2")
	fmt.Println("========================================\n")

	// ==================== 创建普通测试用户 ====================
	fmt.Println("【准备】创建普通测试用户")
	registerBody := map[string]interface{}{
		"agent_name": "NormalUser_" + strconv.FormatInt(time.Now().Unix(), 10),
	}
	resp, err := doPublicRequest(tc, "POST", "/api/v1/user/register", registerBody)
	if err == nil && resp.Code == 0 {
		var userData map[string]interface{}
		json.Unmarshal(resp.Data, &userData)
		tc.TestUserKey = userData["public_key"].(string)
		tc.TestUserPriv = userData["private_key"].(string)
		fmt.Printf("✅ 测试用户创建成功\n")
		fmt.Printf("   公钥: %s...\n\n", tc.TestUserKey[:30])
	} else {
		fmt.Printf("❌ 创建测试用户失败\n\n")
		os.Exit(1)
	}

	var results []string
	passCount := 0
	failCount := 0

	// ==================== 测试 1: 封禁用户 ====================
	fmt.Println("【测试 1】封禁用户 (Lv4+)")
	// 签名使用未编码的路径（服务端会解码 URL）
	banPath := "/api/v1/admin/users/" + tc.TestUserKey + "/ban"
	banBody := map[string]interface{}{"reason": "测试封禁"}
	banBodyBytes, _ := json.Marshal(banBody)

	// 发送请求时需要编码公钥
	encodedKey := url.PathEscape(tc.TestUserKey)
	reqPath := tc.BaseURL + "/api/v1/admin/users/" + encodedKey + "/ban"

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature := signRequest("POST", banPath, timestamp, string(banBodyBytes), tc.AdminPrivKey)

	req, _ := http.NewRequest("POST", reqPath, bytes.NewReader(banBodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentWiki-PublicKey", tc.AdminPubKey)
	req.Header.Set("X-AgentWiki-Timestamp", timestamp)
	req.Header.Set("X-AgentWiki-Signature", signature)

	httpResp, err := tc.Client.Do(req)
	if err == nil && httpResp.StatusCode == 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		var resp APIResponse
		json.Unmarshal(respBody, &resp)
		if resp.Code == 0 {
			fmt.Printf("✅ 封禁用户成功\n\n")
			passCount++
			results = append(results, "✅ 封禁用户 API")
		} else {
			fmt.Printf("❌ 封禁用户失败: code=%d, message=%s\n\n", resp.Code, resp.Message)
			failCount++
			results = append(results, "❌ 封禁用户 API")
		}
	} else {
		fmt.Printf("❌ 封禁用户失败: %v\n\n", err)
		failCount++
		results = append(results, "❌ 封禁用户 API")
	}

	// ==================== 测试 2: 解封用户 ====================
	fmt.Println("【测试 2】解封用户 (Lv4+)")
	unbanPath := "/api/v1/admin/users/" + tc.TestUserKey + "/unban"
	reqPath2 := tc.BaseURL + "/api/v1/admin/users/" + encodedKey + "/unban"

	timestamp2 := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature2 := signRequest("POST", unbanPath, timestamp2, "", tc.AdminPrivKey)

	req2, _ := http.NewRequest("POST", reqPath2, nil)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-AgentWiki-PublicKey", tc.AdminPubKey)
	req2.Header.Set("X-AgentWiki-Timestamp", timestamp2)
	req2.Header.Set("X-AgentWiki-Signature", signature2)

	httpResp2, err := tc.Client.Do(req2)
	if err == nil && httpResp2.StatusCode == 200 {
		respBody2, _ := io.ReadAll(httpResp2.Body)
		httpResp2.Body.Close()
		var resp2 APIResponse
		json.Unmarshal(respBody2, &resp2)
		if resp2.Code == 0 {
			fmt.Printf("✅ 解封用户成功\n\n")
			passCount++
			results = append(results, "✅ 解封用户 API")
		} else {
			fmt.Printf("❌ 解封用户失败: code=%d\n\n", resp2.Code)
			failCount++
			results = append(results, "❌ 解封用户 API")
		}
	} else {
		fmt.Printf("❌ 解封用户失败\n\n")
		failCount++
		results = append(results, "❌ 解封用户 API")
	}

	// ==================== 测试 3: 设置用户等级 ====================
	fmt.Println("【测试 3】设置用户等级 (Lv5)")
	levelPath := "/api/v1/admin/users/" + tc.TestUserKey + "/level"
	levelBody := map[string]interface{}{"level": 2, "reason": "测试升级"}
	levelBodyBytes, _ := json.Marshal(levelBody)
	reqPath3 := tc.BaseURL + "/api/v1/admin/users/" + encodedKey + "/level"

	timestamp3 := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature3 := signRequest("PUT", levelPath, timestamp3, string(levelBodyBytes), tc.AdminPrivKey)

	req3, _ := http.NewRequest("PUT", reqPath3, bytes.NewReader(levelBodyBytes))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-AgentWiki-PublicKey", tc.AdminPubKey)
	req3.Header.Set("X-AgentWiki-Timestamp", timestamp3)
	req3.Header.Set("X-AgentWiki-Signature", signature3)

	httpResp3, err := tc.Client.Do(req3)
	if err == nil && httpResp3.StatusCode == 200 {
		respBody3, _ := io.ReadAll(httpResp3.Body)
		httpResp3.Body.Close()
		var resp3 APIResponse
		json.Unmarshal(respBody3, &resp3)
		if resp3.Code == 0 {
			fmt.Printf("✅ 设置用户等级成功\n\n")
			passCount++
			results = append(results, "✅ 设置用户等级 API")
		} else {
			fmt.Printf("❌ 设置用户等级失败: code=%d, message=%s\n\n", resp3.Code, resp3.Message)
			failCount++
			results = append(results, "❌ 设置用户等级 API")
		}
	} else {
		fmt.Printf("❌ 设置用户等级失败\n\n")
		failCount++
		results = append(results, "❌ 设置用户等级 API")
	}

	// ==================== 测试 4: 批量创建条目 ====================
	fmt.Println("【测试 4】批量创建条目")
	batchCreateBody := map[string]interface{}{
		"entries": []map[string]interface{}{
			{"title": "批量测试1", "content": "内容1", "category": "computer-science"},
			{"title": "批量测试2", "content": "内容2", "category": "computer-science"},
			{"title": "批量测试3", "content": "内容3", "category": "computer-science"},
		},
	}
	batchCreateBodyBytes, _ := json.Marshal(batchCreateBody)
	batchTimestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	batchSignature := signRequest("POST", "/api/v1/entries/batch", batchTimestamp, string(batchCreateBodyBytes), tc.AdminPrivKey)
	batchReq, _ := http.NewRequest("POST", tc.BaseURL+"/api/v1/entries/batch", bytes.NewReader(batchCreateBodyBytes))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.Header.Set("X-AgentWiki-PublicKey", tc.AdminPubKey)
	batchReq.Header.Set("X-AgentWiki-Timestamp", batchTimestamp)
	batchReq.Header.Set("X-AgentWiki-Signature", batchSignature)
	batchResp, err := tc.Client.Do(batchReq)

	var entryIDs []string
	if err == nil && batchResp.StatusCode >= 200 && batchResp.StatusCode < 300 {
		batchRespBody, _ := io.ReadAll(batchResp.Body)
		batchResp.Body.Close()
		var batchResult struct {
			Success bool `json:"success"`
			Results []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"results"`
		}
		json.Unmarshal(batchRespBody, &batchResult)
		fmt.Printf("✅ 批量创建成功\n")
		for _, r := range batchResult.Results {
			if r.ID != "" && r.Status == "created" {
				entryIDs = append(entryIDs, r.ID)
			}
		}
		fmt.Printf("   创建了 %d 个条目\n\n", len(entryIDs))
		passCount++
		results = append(results, "✅ 批量创建条目 API")
	} else {
		fmt.Printf("❌ 批量创建失败\n\n")
		failCount++
		results = append(results, "❌ 批量创建条目 API")
	}

	// ==================== 测试 5: 批量更新条目 ====================
	fmt.Println("【测试 5】批量更新条目")
	if len(entryIDs) >= 2 {
		batchUpdateBody := map[string]interface{}{
			"entries": []map[string]interface{}{
				{"id": entryIDs[0], "title": "更新后的标题1", "content": "更新内容1"},
				{"id": entryIDs[1], "title": "更新后的标题2", "content": "更新内容2"},
			},
		}
		batchUpdateBodyBytes, _ := json.Marshal(batchUpdateBody)
		updateTimestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		updateSignature := signRequest("PUT", "/api/v1/entries/batch", updateTimestamp, string(batchUpdateBodyBytes), tc.AdminPrivKey)
		updateReq, _ := http.NewRequest("PUT", tc.BaseURL+"/api/v1/entries/batch", bytes.NewReader(batchUpdateBodyBytes))
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set("X-AgentWiki-PublicKey", tc.AdminPubKey)
		updateReq.Header.Set("X-AgentWiki-Timestamp", updateTimestamp)
		updateReq.Header.Set("X-AgentWiki-Signature", updateSignature)
		updateResp, err := tc.Client.Do(updateReq)
		
		if err == nil && updateResp.StatusCode >= 200 && updateResp.StatusCode < 300 {
			updateResp.Body.Close()
			fmt.Printf("✅ 批量更新成功\n\n")
			passCount++
			results = append(results, "✅ 批量更新条目 API")
		} else {
			fmt.Printf("❌ 批量更新失败\n\n")
			failCount++
			results = append(results, "❌ 批量更新条目 API")
		}
	} else {
		fmt.Printf("⚠️ 跳过批量更新测试（没有足够的条目）\n\n")
	}

	// ==================== 测试 6: 批量删除条目 ====================
	fmt.Println("【测试 6】批量删除条目")
	if len(entryIDs) >= 1 {
		batchDeleteBody := map[string]interface{}{
			"ids": entryIDs[:1], // 删除第一个
		}
		batchDeleteBodyBytes, _ := json.Marshal(batchDeleteBody)
		deleteTimestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		deleteSignature := signRequest("DELETE", "/api/v1/entries/batch", deleteTimestamp, string(batchDeleteBodyBytes), tc.AdminPrivKey)
		deleteReq, _ := http.NewRequest("DELETE", tc.BaseURL+"/api/v1/entries/batch", bytes.NewReader(batchDeleteBodyBytes))
		deleteReq.Header.Set("Content-Type", "application/json")
		deleteReq.Header.Set("X-AgentWiki-PublicKey", tc.AdminPubKey)
		deleteReq.Header.Set("X-AgentWiki-Timestamp", deleteTimestamp)
		deleteReq.Header.Set("X-AgentWiki-Signature", deleteSignature)
		deleteResp, err := tc.Client.Do(deleteReq)
		
		if err == nil && deleteResp.StatusCode >= 200 && deleteResp.StatusCode < 300 {
			deleteResp.Body.Close()
			fmt.Printf("✅ 批量删除成功\n\n")
			passCount++
			results = append(results, "✅ 批量删除条目 API")
		} else {
			fmt.Printf("❌ 批量删除失败\n\n")
			failCount++
			results = append(results, "❌ 批量删除条目 API")
		}
	}

	// ==================== 测试 7: 创建选举 ====================
	fmt.Println("【测试 7】创建选举 (Lv5)")
	electionBody := map[string]interface{}{
		"title":       "测试选举",
		"description": "这是一个测试选举",
		"duration":    3600, // 1小时
	}
	resp, err = doRequest(tc, "POST", "/api/v1/elections/create", electionBody)
	var electionID string
	if err == nil && resp.Code == 0 {
		var electionData map[string]interface{}
		json.Unmarshal(resp.Data, &electionData)
		if id, ok := electionData["id"]; ok {
			electionID = fmt.Sprintf("%v", id)
			fmt.Printf("✅ 创建选举成功，ID: %s\n\n", electionID)
		}
		passCount++
		results = append(results, "✅ 创建选举 API")
	} else {
		fmt.Printf("❌ 创建选举失败: code=%d, message=%s\n\n", resp.Code, resp.Message)
		failCount++
		results = append(results, "❌ 创建选举 API")
	}

	// ==================== 测试 8: 获取选举列表 ====================
	fmt.Println("【测试 8】获取选举列表")
	resp, err = doPublicRequest(tc, "GET", "/api/v1/elections", nil)
	if err == nil && resp.Code == 0 {
		fmt.Printf("✅ 获取选举列表成功\n\n")
		passCount++
		results = append(results, "✅ 选举列表 API")
	} else {
		fmt.Printf("❌ 获取选举列表失败\n\n")
		failCount++
		results = append(results, "❌ 选举列表 API")
	}

	// ==================== 测试 9: 投票 ====================
	if electionID != "" {
		fmt.Println("【测试 9】投票 (Lv3+)")
		// 先升级测试用户到 Lv3
		levelPath := "/api/v1/admin/users/" + url.PathEscape(tc.TestUserKey) + "/level"
		levelBody := map[string]interface{}{"level": 3, "reason": "允许投票"}
		doRequest(tc, "PUT", levelPath, levelBody)

		votePath := "/api/v1/elections/" + electionID + "/vote"
		// 使用测试用户投票
		resp, err = doRequestWithKey(tc, "POST", votePath, `{"candidate_id":"`+tc.AdminPubKey+`"}`, tc.TestUserKey, tc.TestUserPriv)
		if err == nil && resp.Code == 0 {
			fmt.Printf("✅ 投票成功\n\n")
			passCount++
			results = append(results, "✅ 投票 API")
		} else {
			fmt.Printf("❌ 投票失败: code=%d, message=%s\n\n", resp.Code, resp.Message)
			failCount++
			results = append(results, "❌ 投票 API")
		}
	}

	// ==================== 测试 10: 关闭选举 ====================
	if electionID != "" {
		fmt.Println("【测试 10】关闭选举 (Lv5)")
		closePath := "/api/v1/elections/" + electionID + "/close"
		resp, err = doRequest(tc, "POST", closePath, nil)
		if err == nil && resp.Code == 0 {
			fmt.Printf("✅ 关闭选举成功\n\n")
			passCount++
			results = append(results, "✅ 关闭选举 API")
		} else {
			fmt.Printf("❌ 关闭选举失败\n\n")
			failCount++
			results = append(results, "❌ 关闭选举 API")
		}
	}

	// ==================== 测试 11: 数据导出和导入 ====================
	fmt.Println("【测试 11】数据导出 (用于导入测试)")
	// 导出数据
	exportTimestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	exportSignature := signRequest("GET", "/api/v1/admin/export", exportTimestamp, "", tc.AdminPrivKey)
	exportReq, _ := http.NewRequest("GET", tc.BaseURL+"/api/v1/admin/export?include=entries,categories,users", nil)
	exportReq.Header.Set("X-AgentWiki-PublicKey", tc.AdminPubKey)
	exportReq.Header.Set("X-AgentWiki-Timestamp", exportTimestamp)
	exportReq.Header.Set("X-AgentWiki-Signature", exportSignature)
	exportResp, err := tc.Client.Do(exportReq)

	var exportData []byte
	if err == nil && exportResp.StatusCode == 200 {
		exportData, _ = io.ReadAll(exportResp.Body)
		exportResp.Body.Close()
		fmt.Printf("✅ 数据导出成功 (%d 字节)\n\n", len(exportData))
		passCount++
		results = append(results, "✅ 数据导出 API")
	} else {
		fmt.Printf("❌ 数据导出失败\n\n")
		failCount++
		results = append(results, "❌ 数据导出 API")
	}

	// ==================== 测试 12: 数据导入 ====================
	if len(exportData) > 0 {
		fmt.Println("【测试 12】数据导入 (Lv4+)")
		fmt.Printf("⚠️ 数据导入需要 multipart/form-data，跳过实际导入测试\n\n")
		results = append(results, "⚠️ 数据导入 API (需要 multipart)")
	}

	// ==================== 测试总结 ====================
	fmt.Println("========================================")
	fmt.Println("测试总结")
	fmt.Println("========================================")
	for _, r := range results {
		fmt.Println(r)
	}
	fmt.Printf("\n通过: %d, 失败: %d\n", passCount, failCount)

	// 保存新用户密钥
	newKeys := map[string]string{
		"test_user_public_key":  keys.TestUserPublicKey,
		"test_user_private_key": keys.TestUserPrivateKey,
		"normal_user_public":    tc.TestUserKey,
		"normal_user_private":   tc.TestUserPriv,
	}
	keyJSON, _ := json.MarshalIndent(newKeys, "", "  ")
	os.WriteFile("/tmp/test_keys.json", keyJSON, 0600)
}
