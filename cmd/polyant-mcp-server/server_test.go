package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/daifei0527/polyant/pkg/polysdk"
)

// setupTestServer 创建用于测试的服务器实例（不连接真实 API）
func setupTestServer(t *testing.T) *Server {
	t.Helper()
	config := &Config{BaseURL: "http://localhost:9999"}
	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建测试服务器失败: %v", err)
	}
	return server
}

// sendRequest 发送 JSON-RPC 请求并返回响应
func sendRequest(t *testing.T, server *Server, method string, params interface{}) *jsonrpcResponse {
	t.Helper()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  method,
	}
	if params != nil {
		paramsBytes, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("序列化参数失败: %v", err)
		}
		req.Params = paramsBytes
	}

	return server.handleRequest(&req)
}

func TestHandleInitialize(t *testing.T) {
	server := setupTestServer(t)
	resp := sendRequest(t, server, "initialize", nil)

	if resp.JSONRPC != "2.0" {
		t.Errorf("期望 jsonrpc 为 2.0，实际为 %s", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Fatalf("不期望错误: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("结果类型错误: %T", resp.Result)
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("期望协议版本 2024-11-05，实际为 %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("serverInfo 类型错误: %T", result["serverInfo"])
	}
	if serverInfo["name"] != "polyant-mcp-server" {
		t.Errorf("期望服务器名称 polyant-mcp-server，实际为 %v", serverInfo["name"])
	}

	caps, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("capabilities 类型错误: %T", result["capabilities"])
	}
	if _, hasTools := caps["tools"]; !hasTools {
		t.Error("capabilities 中缺少 tools")
	}
}

func TestHandleListTools(t *testing.T) {
	server := setupTestServer(t)
	resp := sendRequest(t, server, "tools/list", nil)

	if resp.Error != nil {
		t.Fatalf("不期望错误: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("结果类型错误: %T", resp.Result)
	}

	tools, ok := result["tools"].([]tool)
	if !ok {
		// JSON unmarshal 会将结构体切片转为 []interface{}，需要重新序列化
		toolsBytes, _ := json.Marshal(result["tools"])
		var toolsList []tool
		if err := json.Unmarshal(toolsBytes, &toolsList); err != nil {
			t.Fatalf("解析工具列表失败: %v", err)
		}
		tools = toolsList
	}

	if len(tools) != 3 {
		t.Fatalf("期望 3 个工具，实际为 %d", len(tools))
	}

	expectedNames := map[string]bool{
		"polyant_search": false,
		"polyant_create": false,
		"polyant_rate":   false,
	}
	for _, tool := range tools {
		if _, ok := expectedNames[tool.Name]; ok {
			expectedNames[tool.Name] = true
		} else {
			t.Errorf("意外的工具名称: %s", tool.Name)
		}
	}
	for name, found := range expectedNames {
		if !found {
			t.Errorf("缺少工具: %s", name)
		}
	}

	// 验证每个工具都有描述和 inputSchema
	for _, tool := range tools {
		if tool.Description == "" {
			t.Errorf("工具 %s 缺少描述", tool.Name)
		}
		if len(tool.InputSchema) == 0 {
			t.Errorf("工具 %s 缺少 inputSchema", tool.Name)
		}
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	server := setupTestServer(t)
	resp := sendRequest(t, server, "unknown/method", nil)

	if resp.Error == nil {
		t.Fatal("期望返回错误")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("期望错误码 -32601，实际为 %d", resp.Error.Code)
	}
}

func TestHandleCallUnknownTool(t *testing.T) {
	server := setupTestServer(t)
	resp := sendRequest(t, server, "tools/call", map[string]interface{}{
		"name":      "nonexistent_tool",
		"arguments": map[string]interface{}{},
	})

	if resp.Error == nil {
		t.Fatal("期望返回错误")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("期望错误码 -32601，实际为 %d", resp.Error.Code)
	}
}

func TestFormatResult(t *testing.T) {
	text := "测试结果"
	result := formatResult(text)

	content, ok := result["content"].([]map[string]interface{})
	if !ok {
		t.Fatalf("content 类型错误: %T", result["content"])
	}
	if len(content) != 1 {
		t.Fatalf("期望 1 个 content 项，实际为 %d", len(content))
	}
	if content[0]["type"] != "text" {
		t.Errorf("期望类型 text，实际为 %v", content[0]["type"])
	}
	if content[0]["text"] != text {
		t.Errorf("期望文本 %q，实际为 %q", text, content[0]["text"])
	}
}

func TestFormatEntries(t *testing.T) {
	t.Run("空列表", func(t *testing.T) {
		result := formatEntries(nil)
		if result != "未找到匹配的条目。" {
			t.Errorf("期望 '未找到匹配的条目。'，实际为 %q", result)
		}
	})

	t.Run("有结果", func(t *testing.T) {
		entries := []polysdk.Entry{
			{
				ID:         "test-1",
				Title:      "测试条目",
				Content:    "测试内容",
				Category:   "测试分类",
				Tags:       []string{"tag1", "tag2"},
				Score:      4.5,
				ScoreCount: 2,
			},
		}

		result := formatEntries(entries)
		if result == "" {
			t.Error("期望非空结果")
		}
		if !bytes.Contains([]byte(result), []byte("测试条目")) {
			t.Error("结果中应包含标题")
		}
		if !bytes.Contains([]byte(result), []byte("test-1")) {
			t.Error("结果中应包含 ID")
		}
	})
}
