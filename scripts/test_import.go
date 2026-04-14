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
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"time"
)

func signRequest(method, path, timestamp, body, privateKey string) string {
	privBytes, _ := base64.StdEncoding.DecodeString(privateKey)
	privKey := ed25519.PrivateKey(privBytes)
	bodyHash := sha256.Sum256([]byte(body))
	signContent := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, timestamp, hex.EncodeToString(bodyHash[:]))
	signature := ed25519.Sign(privKey, []byte(signContent))
	return base64.StdEncoding.EncodeToString(signature)
}

func main() {
	// 读取管理员密钥
	keyFile := []byte(`{"test_user_public_key":"4Zok/irK6lbnki+MmQPn60lQ/tta5ruBkd9XnqqcBbo=","test_user_private_key":"Tq+iogxoc39iDZFM5p2or/LnBDq0ALIkoSBt1QyPpInhmiT+KsrqVueSL4yZA+frSVD+21rmu4GR31eeqpwFug=="}`)
	var keys struct {
		TestUserPublicKey  string `json:"test_user_public_key"`
		TestUserPrivateKey string `json:"test_user_private_key"`
	}
	json.Unmarshal(keyFile, &keys)

	baseURL := "http://localhost:8080"
	client := &http.Client{Timeout: 60 * time.Second}

	fmt.Println("========================================")
	fmt.Println("数据导入测试")
	fmt.Println("========================================\n")

	// ==================== 步骤 1: 导出数据 ====================
	fmt.Println("【步骤 1】导出数据")
	exportTimestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	exportSignature := signRequest("GET", "/api/v1/admin/export", exportTimestamp, "", keys.TestUserPrivateKey)
	exportReq, _ := http.NewRequest("GET", baseURL+"/api/v1/admin/export?include=entries,categories,users,ratings", nil)
	exportReq.Header.Set("X-AgentWiki-PublicKey", keys.TestUserPublicKey)
	exportReq.Header.Set("X-AgentWiki-Timestamp", exportTimestamp)
	exportReq.Header.Set("X-AgentWiki-Signature", exportSignature)

	exportResp, err := client.Do(exportReq)
	if err != nil {
		fmt.Printf("❌ 导出失败: %v\n", err)
		os.Exit(1)
	}

	if exportResp.StatusCode != 200 {
		fmt.Printf("❌ 导出失败: HTTP %d\n", exportResp.StatusCode)
		exportResp.Body.Close()
		os.Exit(1)
	}

	exportData, _ := io.ReadAll(exportResp.Body)
	exportResp.Body.Close()
	fmt.Printf("✅ 导出成功 (%d 字节)\n\n", len(exportData))

	// ==================== 步骤 2: 创建 multipart 表单 ====================
	fmt.Println("【步骤 2】创建 multipart 表单")

	// 创建 multipart buffer
	var multipartBuffer bytes.Buffer
	writer := multipart.NewWriter(&multipartBuffer)

	// 添加文件字段
	fileWriter, _ := writer.CreateFormFile("file", "import.zip")
	fileWriter.Write(exportData)

	// 添加 conflict 字段
	_ = writer.WriteField("conflict", "skip")

	// 关闭 writer
	writer.Close()

	multipartData := multipartBuffer.Bytes()
	contentType := writer.FormDataContentType()

	fmt.Printf("   Content-Type: %s\n", contentType)
	fmt.Printf("   Multipart 大小: %d 字节\n\n", len(multipartData))

	// ==================== 步骤 3: 计算签名 ====================
	fmt.Println("【步骤 3】计算签名")

	// 对于 multipart 请求，签名使用 multipart body
	importTimestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	importSignature := signRequest("POST", "/api/v1/admin/import", importTimestamp, string(multipartData), keys.TestUserPrivateKey)

	fmt.Printf("   时间戳: %s\n", importTimestamp)
	fmt.Printf("   签名: %s...\n\n", importSignature[:30])

	// ==================== 步骤 4: 发送导入请求 ====================
	fmt.Println("【步骤 4】发送导入请求")

	importReq, _ := http.NewRequest("POST", baseURL+"/api/v1/admin/import", &multipartBuffer)
	importReq.Header.Set("Content-Type", contentType)
	importReq.Header.Set("X-AgentWiki-PublicKey", keys.TestUserPublicKey)
	importReq.Header.Set("X-AgentWiki-Timestamp", importTimestamp)
	importReq.Header.Set("X-AgentWiki-Signature", importSignature)

	importResp, err := client.Do(importReq)
	if err != nil {
		fmt.Printf("❌ 导入请求失败: %v\n", err)
		os.Exit(1)
	}
	defer importResp.Body.Close()

	importRespBody, _ := io.ReadAll(importResp.Body)

	fmt.Printf("   HTTP 状态: %d\n", importResp.StatusCode)
	fmt.Printf("   响应: %s\n\n", string(importRespBody))

	// 解析响应
	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Success bool `json:"success"`
			Summary struct {
				EntriesImported   int `json:"entries_imported"`
				CategoriesImported int `json:"categories_imported"`
				UsersImported     int `json:"users_imported"`
				RatingsImported   int `json:"ratings_imported"`
			} `json:"summary"`
		} `json:"data"`
	}
	json.Unmarshal(importRespBody, &apiResp)

	if apiResp.Code == 0 && apiResp.Data.Success {
		fmt.Println("========================================")
		fmt.Println("测试结果: ✅ 数据导入成功")
		fmt.Println("========================================")
		fmt.Printf("   条目导入: %d\n", apiResp.Data.Summary.EntriesImported)
		fmt.Printf("   分类导入: %d\n", apiResp.Data.Summary.CategoriesImported)
		fmt.Printf("   用户导入: %d\n", apiResp.Data.Summary.UsersImported)
		fmt.Printf("   评分导入: %d\n", apiResp.Data.Summary.RatingsImported)
	} else {
		fmt.Println("========================================")
		fmt.Println("测试结果: ❌ 数据导入失败")
		fmt.Println("========================================")
		fmt.Printf("   错误码: %d\n", apiResp.Code)
		fmt.Printf("   错误信息: %s\n", apiResp.Message)
		os.Exit(1)
	}
}
