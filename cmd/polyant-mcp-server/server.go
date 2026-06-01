package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

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
	result, err := s.client.Search(ctx, params.Query, params.Category, nil, params.Limit)
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
