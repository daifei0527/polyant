package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/daifei0527/polyant/internal/auth/ed25519"
	"github.com/daifei0527/polyant/pkg/polysdk"
)

// JSON-RPC 2.0 请求
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSON-RPC 2.0 响应
type jsonrpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

// JSON-RPC 错误
type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP 工具定义
type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Server MCP 服务器
type Server struct {
	client *polysdk.Client
	reader *bufio.Reader
	writer io.Writer
}

// NewServer 创建新的 MCP 服务器
func NewServer(config *Config) (*Server, error) {
	client := polysdk.NewClient(config.BaseURL)
	if config.APIKey != "" {
		client.SetAPIKey(config.APIKey)
	}
	if config.KeyDir != "" {
		priv, pub, err := ed25519.LoadKeyPair(config.KeyDir)
		if err == nil {
			client.SetKeys(pub, priv)
		}
		// 加载失败不 fatal——read-only 工具仍可用（API key）；write 工具会因无签名失败并报错
	}

	return &Server{
		client: client,
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
	}, nil
}

// Run 启动服务器主循环，从 stdin 读取 JSON-RPC 请求，写入 stdout
func (s *Server) Run() error {
	decoder := json.NewDecoder(s.reader)
	encoder := json.NewEncoder(s.writer)

	for {
		var req jsonrpcRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode request: %w", err)
		}

		resp := s.handleRequest(&req)
		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
	}
}

// handleRequest 分发请求到对应的处理方法
func (s *Server) handleRequest(req *jsonrpcRequest) *jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(req)
	default:
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &rpcError{
				Code:    -32601,
				Message: fmt.Sprintf("unknown method: %s", req.Method),
			},
		}
	}
}

// handleInitialize 处理初始化请求，返回协议版本和能力
func (s *Server) handleInitialize(req *jsonrpcRequest) *jsonrpcResponse {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "polyant-mcp-server",
			"version": "1.0.0",
		},
	}
	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleListTools 返回可用工具列表
func (s *Server) handleListTools(req *jsonrpcRequest) *jsonrpcResponse {
	tools := []tool{
		{
			Name:        "polyant_search",
			Description: "搜索 Polyant 知识库中的条目",
			InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {
							"type": "string",
							"description": "搜索关键词"
						},
						"category": {
							"type": "string",
							"description": "分类过滤（可选）"
						},
						"limit": {
							"type": "integer",
							"description": "返回结果数量限制",
							"default": 10
						},
						"lang": {
							"type": "string",
							"description": "返回结果的本地化语言，如 zh-CN、en-US（可选，默认条目主语言）"
						}
					},
					"required": ["query"]
				}`),
		},
		{
			Name:        "polyant_create",
			Description: "在 Polyant 知识库中创建新条目",
			InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"title": {
							"type": "string",
							"description": "条目标题"
						},
						"content": {
							"type": "string",
							"description": "条目内容"
						},
						"category": {
							"type": "string",
							"description": "条目分类"
						},
						"tags": {
							"type": "array",
							"items": {"type": "string"},
							"description": "标签列表（可选）"
						}
					},
					"required": ["title", "content", "category"]
				}`),
		},
		{
			Name:        "polyant_rate",
			Description: "为 Polyant 知识库中的条目评分",
			InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"id": {
							"type": "string",
							"description": "条目 ID"
						},
						"score": {
							"type": "number",
							"description": "评分值（1.0-5.0）",
							"minimum": 1.0,
							"maximum": 5.0
						},
						"comment": {
							"type": "string",
							"description": "评语（可选）"
						}
					},
					"required": ["id", "score"]
				}`),
		},
		{
			Name:        "polyant_get",
			Description: "获取 Polyant 知识库中的单个条目",
			InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"id":   {"type": "string", "description": "条目 ID"},
						"lang": {"type": "string", "description": "返回结果的本地化语言（可选）"}
					},
					"required": ["id"]
				}`),
		},
		{
			Name:        "polyant_update",
			Description: "更新 Polyant 知识库中的条目（只传需改的字段）",
			InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"id":       {"type": "string", "description": "条目 ID"},
						"title":    {"type": "string", "description": "新标题（可选，不传则不改）"},
						"content":  {"type": "string", "description": "新内容（可选）"},
						"category": {"type": "string", "description": "新分类（可选）"},
						"tags":     {"type": "array", "items": {"type": "string"}, "description": "新标签（可选）"}
					},
					"required": ["id"]
				}`),
		},
		{
			Name:        "polyant_delete",
			Description: "删除 Polyant 知识库中的条目（需确认）",
			InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"id":      {"type": "string", "description": "条目 ID"},
						"confirm": {"type": "boolean", "description": "必须为 true 才执行删除"}
					},
					"required": ["id", "confirm"]
				}`),
		},
		{
			Name:        "polyant_list_categories",
			Description: "列出 Polyant 知识库的所有分类",
			InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {}
				}`),
		},
	}

	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// handleCallTool 分发工具调用到具体处理函数
func (s *Server) handleCallTool(req *jsonrpcRequest) *jsonrpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &rpcError{
				Code:    -32602,
				Message: fmt.Sprintf("invalid params: %v", err),
			},
		}
	}

	switch params.Name {
	case "polyant_search":
		return s.handleSearch(req.ID, params.Arguments)
	case "polyant_create":
		return s.handleCreate(req.ID, params.Arguments)
	case "polyant_rate":
		return s.handleRate(req.ID, params.Arguments)
	case "polyant_get":
		return s.handleGet(req.ID, params.Arguments)
	case "polyant_update":
		return s.handleUpdate(req.ID, params.Arguments)
	case "polyant_delete":
		return s.handleDelete(req.ID, params.Arguments)
	case "polyant_list_categories":
		return s.handleListCategories(req.ID, params.Arguments)
	default:
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &rpcError{
				Code:    -32601,
				Message: fmt.Sprintf("unknown tool: %s", params.Name),
			},
		}
	}
}

// formatEntries 格式化条目列表为文本
func formatEntries(entries []polysdk.Entry) string {
	if len(entries) == 0 {
		return "未找到匹配的条目。"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 条结果:\n\n", len(entries)))
	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("## %s\n", entry.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", entry.ID))
		sb.WriteString(fmt.Sprintf("分类: %s\n", entry.Category))
		if len(entry.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("标签: %s\n", strings.Join(entry.Tags, ", ")))
		}
		sb.WriteString(fmt.Sprintf("评分: %.1f (%d 次)\n", entry.Score, entry.ScoreCount))
		sb.WriteString(fmt.Sprintf("内容: %s\n\n", entry.Content))
	}
	return sb.String()
}

// formatEntry 格式化单个条目
func formatEntry(e *polysdk.Entry) string {
	if e == nil {
		return "条目不存在"
	}
	return formatEntries([]polysdk.Entry{*e})
}

// formatCategories 格式化分类列表
func formatCategories(cats []polysdk.Category) string {
	if len(cats) == 0 {
		return "暂无分类"
	}
	var sb strings.Builder
	for _, c := range cats {
		sb.WriteString(fmt.Sprintf("## %s\n", c.Name))
		sb.WriteString(fmt.Sprintf("ID: %s\n", c.ID))
		if c.Description != "" {
			sb.WriteString(fmt.Sprintf("描述: %s\n", c.Description))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatResult 格式化工具调用结果为 MCP content 格式
func formatResult(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	}
}

// handleSearch 处理搜索工具调用
func (s *Server) handleSearch(id interface{}, args json.RawMessage) *jsonrpcResponse {
	var params struct {
		Query    string `json:"query"`
		Category string `json:"category,omitempty"`
		Limit    int    `json:"limit,omitempty"`
		Lang     string `json:"lang,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error: &rpcError{
				Code:    -32602,
				Message: fmt.Sprintf("invalid arguments: %v", err),
			},
		}
	}

	if params.Limit <= 0 {
		params.Limit = 10
	}

	ctx := context.Background()
	result, err := s.client.Search(ctx, params.Query, params.Category, nil, params.Limit, params.Lang)
	if err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  formatResult(fmt.Sprintf("搜索失败: %v", err)),
		}
	}

	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  formatResult(formatEntries(result.Entries)),
	}
}

// handleCreate 处理创建工具调用
func (s *Server) handleCreate(id interface{}, args json.RawMessage) *jsonrpcResponse {
	var params struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Category string   `json:"category"`
		Tags     []string `json:"tags,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error: &rpcError{
				Code:    -32602,
				Message: fmt.Sprintf("invalid arguments: %v", err),
			},
		}
	}

	ctx := context.Background()
	entry, err := s.client.CreateEntry(ctx, &polysdk.CreateEntryRequest{
		Title:    params.Title,
		Content:  params.Content,
		Category: params.Category,
		Tags:     params.Tags,
	})
	if err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  formatResult(fmt.Sprintf("创建条目失败: %v", err)),
		}
	}

	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  formatResult(fmt.Sprintf("条目创建成功!\nID: %s\n标题: %s\n分类: %s", entry.ID, entry.Title, entry.Category)),
	}
}

// handleRate 处理评分工具调用
func (s *Server) handleRate(id interface{}, args json.RawMessage) *jsonrpcResponse {
	var params struct {
		ID      string  `json:"id"`
		Score   float64 `json:"score"`
		Comment string  `json:"comment,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error: &rpcError{
				Code:    -32602,
				Message: fmt.Sprintf("invalid arguments: %v", err),
			},
		}
	}

	if params.Score < 1.0 || params.Score > 5.0 {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  formatResult("评分必须在 1.0 到 5.0 之间。"),
		}
	}

	ctx := context.Background()
	err := s.client.RateEntry(ctx, params.ID, params.Score, params.Comment)
	if err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  formatResult(fmt.Sprintf("评分失败: %v", err)),
		}
	}

	comment := ""
	if params.Comment != "" {
		comment = fmt.Sprintf("\n评语: %s", params.Comment)
	}
	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  formatResult(fmt.Sprintf("评分成功!\n条目: %s\n评分: %.1f%s", params.ID, params.Score, comment)),
	}
}

// handleGet 处理获取条目工具调用
func (s *Server) handleGet(id interface{}, args json.RawMessage) *jsonrpcResponse {
	var params struct {
		ID   string `json:"id"`
		Lang string `json:"lang"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &jsonrpcResponse{JSONRPC: "2.0", ID: id,
			Error: &rpcError{Code: -32602, Message: fmt.Sprintf("invalid arguments: %v", err)}}
	}
	ctx := context.Background()
	entry, err := s.client.GetEntry(ctx, params.ID, params.Lang)
	if err != nil {
		return &jsonrpcResponse{JSONRPC: "2.0", ID: id,
			Result: formatResult(fmt.Sprintf("获取条目失败: %v", err))}
	}
	return &jsonrpcResponse{JSONRPC: "2.0", ID: id, Result: formatResult(formatEntry(entry))}
}

// handleUpdate 处理更新条目工具调用
func (s *Server) handleUpdate(id interface{}, args json.RawMessage) *jsonrpcResponse {
	var params struct {
		ID       string   `json:"id"`
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Category string   `json:"category"`
		Tags     []string `json:"tags"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &jsonrpcResponse{JSONRPC: "2.0", ID: id,
			Error: &rpcError{Code: -32602, Message: fmt.Sprintf("invalid arguments: %v", err)}}
	}
	ctx := context.Background()
	entry, err := s.client.UpdateEntry(ctx, params.ID, &polysdk.UpdateEntryRequest{
		Title: params.Title, Content: params.Content, Category: params.Category, Tags: params.Tags,
	})
	if err != nil {
		return &jsonrpcResponse{JSONRPC: "2.0", ID: id,
			Result: formatResult(fmt.Sprintf("更新条目失败: %v", err))}
	}
	return &jsonrpcResponse{JSONRPC: "2.0", ID: id, Result: formatResult(formatEntry(entry))}
}

// handleDelete 处理删除条目工具调用
func (s *Server) handleDelete(id interface{}, args json.RawMessage) *jsonrpcResponse {
	var params struct {
		ID      string `json:"id"`
		Confirm bool   `json:"confirm"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &jsonrpcResponse{JSONRPC: "2.0", ID: id,
			Error: &rpcError{Code: -32602, Message: fmt.Sprintf("invalid arguments: %v", err)}}
	}
	if !params.Confirm {
		return &jsonrpcResponse{JSONRPC: "2.0", ID: id,
			Result: formatResult("请设置 confirm=true 以确认删除条目: " + params.ID)}
	}
	ctx := context.Background()
	if err := s.client.DeleteEntry(ctx, params.ID); err != nil {
		return &jsonrpcResponse{JSONRPC: "2.0", ID: id,
			Result: formatResult(fmt.Sprintf("删除条目失败: %v", err))}
	}
	return &jsonrpcResponse{JSONRPC: "2.0", ID: id,
		Result: formatResult(fmt.Sprintf("条目已删除: %s", params.ID))}
}

// handleListCategories 处理列出分类工具调用
func (s *Server) handleListCategories(id interface{}, args json.RawMessage) *jsonrpcResponse {
	ctx := context.Background()
	cats, err := s.client.ListCategories(ctx)
	if err != nil {
		return &jsonrpcResponse{JSONRPC: "2.0", ID: id,
			Result: formatResult(fmt.Sprintf("获取分类失败: %v", err))}
	}
	return &jsonrpcResponse{JSONRPC: "2.0", ID: id, Result: formatResult(formatCategories(cats))}
}
