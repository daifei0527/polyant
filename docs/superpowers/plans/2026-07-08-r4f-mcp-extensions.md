# R4f MCP Extensions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 4 MCP tools (get/update/delete/list-categories) + wire Ed25519 key loading so write tools actually sign requests.

**Architecture:** 1 task — add key loading to `NewServer`, 4 new tool handlers (mirroring `handleCreate`), 2 new formatters, register in list + dispatch, update tests. All in `cmd/polyant-mcp-server/{server.go,server_test.go}`.

**Tech Stack:** Go 1.25.x / MCP JSON-RPC 2.0 over stdio / polysdk / Ed25519.

## Global Constraints

- **Go 1.25.x**; module `github.com/daifei0527/polyant`.
- **MCP server** at `cmd/polyant-mcp-server/`: `server.go` (414 lines), `server_test.go` (214 lines). Tool = 3 manual edits (list entry at `:129`, switch case at `:236`, handler func). Error convention: arg-parse → JSON-RPC `-32602`; SDK error → `Result` with `formatResult` text.
- **polysdk methods** (`pkg/polysdk/client.go`): `GetEntry(ctx, id, lang string) (*Entry, error)` `:89`; `UpdateEntry(ctx, id string, req *UpdateEntryRequest) (*Entry, error)` `:119`; `DeleteEntry(ctx, id string) error` `:153`; `ListCategories(ctx) ([]Category, error)` `:170`. `UpdateEntryRequest{Title, Content, Category string; Tags []string}` (all `omitempty`).
- **Key loading**: `ed25519.LoadKeyPair(dir string) (privateKey, publicKey []byte, err error)` (`internal/auth/ed25519/keys.go:139`). `client.SetKeys(publicKey, privateKey []byte)` (`polysdk/client.go:51`). MCP server (a `cmd/`) CAN import `internal/auth/ed25519`.
- **Existing helpers**: `formatResult(text string) map[string]interface{}` `:255`; `formatEntries(entries []polysdk.Entry) string` `:256` (renders Title/ID/Category/Tags/Score/ScoreCount/Content). `Category` struct (`types.go:51`): `ID, Name, Description string; ParentID string` (verify exact tags).
- **Test count** hard-coded at `server_test.go:103` (`len(tools) != 3`); `expectedNames` map at `:107-111`.
- **Canonical verification**:
  ```
  gofmt -l $(find . -name '*.go' -not -path './vendor/*')
  go build ./cmd/... ./internal/... ./pkg/...
  go vet ./...
  go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
  golangci-lint run ./...
  ```
- **Commit prefix**: `feat(mcp)`. End with blank line + `Co-Authored-By: Claude <noreply@anthropic.com>`.
- **Spec**: `docs/superpowers/specs/2026-07-08-polyant-r4f-mcp-extensions-design.md`.

---

## Task 1: Key wiring + 4 new tools + formatters + tests

**Files:**
- Modify: `cmd/polyant-mcp-server/server.go` (NewServer key loading + 4 list entries + 4 switch cases + 4 handlers + 2 formatters)
- Modify: `cmd/polyant-mcp-server/server_test.go` (count 3→7 + 4 new names + formatter tests + delete confirm test)

**Interfaces:**
- Consumes: `polysdk.GetEntry/UpdateEntry/DeleteEntry/ListCategories`, `ed25519.LoadKeyPair`, `client.SetKeys`.
- Produces: 7 registered MCP tools (was 3), Ed25519 key loading wired.

- [ ] **Step 1: Wire key loading in NewServer**

In `cmd/polyant-mcp-server/server.go`, add the import:
```go
	"github.com/daifei0527/polyant/internal/auth/ed25519"
```
In `NewServer` (currently `:53-64`), after the `SetAPIKey` block, add key loading:
```go
	if config.KeyDir != "" {
		priv, pub, err := ed25519.LoadKeyPair(config.KeyDir)
		if err == nil {
			client.SetKeys(pub, priv)
		}
		// 加载失败不 fatal——read-only 工具仍可用（API key）；write 工具会因无签名失败并报错
	}
```

- [ ] **Step 2: Add 4 tool list entries**

In `handleListTools` (the `[]tool{...}` literal at `:129-208`), add 4 entries after `polyant_rate`:
```go
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
```

- [ ] **Step 3: Add 4 switch cases in handleCallTool**

In the `switch params.Name` block (`:236-252`), add before `default:`:
```go
	case "polyant_get":
		return s.handleGet(req.ID, params.Arguments)
	case "polyant_update":
		return s.handleUpdate(req.ID, params.Arguments)
	case "polyant_delete":
		return s.handleDelete(req.ID, params.Arguments)
	case "polyant_list_categories":
		return s.handleListCategories(req.ID, params.Arguments)
```

- [ ] **Step 4: Add 2 formatters**

After `formatEntries` (`:256-274`), add:
```go
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
```
Ensure `"strings"` is imported (it likely is — used elsewhere in server.go).

- [ ] **Step 5: Add 4 handlers**

After `handleRate` (`:370-414`), add:
```go
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
```

- [ ] **Step 6: Update tests**

In `server_test.go`:
(a) `TestHandleListTools` line ~103: change `if len(tools) != 3` → `if len(tools) != 7`.
(b) Add 4 new names to the `expectedNames` map (~`:107-111`):
```go
	expectedNames := map[string]bool{
		"polyant_search": true, "polyant_create": true, "polyant_rate": true,
		"polyant_get": true, "polyant_update": true, "polyant_delete": true, "polyant_list_categories": true,
	}
```
(c) Add formatter + delete-confirm tests at the end of the file:
```go
func TestFormatEntry(t *testing.T) {
	e := &polysdk.Entry{ID: "e1", Title: "T", Content: "C", Category: "x", Score: 4.5, ScoreCount: 10}
	result := formatEntry(e)
	if !strings.Contains(result, "T") || !strings.Contains(result, "e1") {
		t.Errorf("formatEntry missing fields: %s", result)
	}
	if formatEntry(nil) != "条目不存在" {
		t.Error("formatEntry(nil) should return 不存在")
	}
}

func TestFormatCategories(t *testing.T) {
	cats := []polysdk.Category{{ID: "x", Name: "Cat1", Description: "D"}}
	result := formatCategories(cats)
	if !strings.Contains(result, "Cat1") || !strings.Contains(result, "x") {
		t.Errorf("formatCategories missing fields: %s", result)
	}
	if formatCategories(nil) != "暂无分类" {
		t.Error("formatCategories(nil) should return 暂无分类")
	}
}

func TestHandleDeleteConfirmRequired(t *testing.T) {
	server := setupTestServer(t)
	// confirm=false → should NOT call DeleteEntry (no real API), returns prompt
	resp := sendRequest(t, server, "tools/call", map[string]interface{}{
		"name": "polyant_delete",
		"arguments": map[string]interface{}{"id": "e1", "confirm": false},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	resultStr := ""
	if m, ok := resp.Result.(map[string]interface{}); ok {
		if content, ok := m["content"].([]interface{}); ok && len(content) > 0 {
			if tm, ok := content[0].(map[string]interface{}); ok {
				resultStr, _ = tm["text"].(string)
			}
		}
	}
	if !strings.Contains(resultStr, "confirm=true") {
		t.Errorf("confirm=false should prompt, got: %s", resultStr)
	}
}
```
Ensure `"strings"` is imported in the test file.

- [ ] **Step 7: Verify + commit**

Run the canonical verification block. Then:
```bash
git add cmd/polyant-mcp-server/server.go cmd/polyant-mcp-server/server_test.go
git commit -m "feat(mcp): add get/update/delete/list-categories tools + Ed25519 key wiring

4 new MCP tools wrapping polysdk GetEntry/UpdateEntry/DeleteEntry/ListCategories.
NewServer now loads Ed25519 keys from KeyDir (LoadKeyPair -> SetKeys), enabling
write tools to sign requests. delete requires confirm=true. formatEntry +
formatCategories helpers. Tests: count 3->7, formatter tests, delete-confirm test.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Final verification (after Task 1)

- [ ] `gofmt -l` repo-wide empty; `go build ./cmd/... ./internal/... ./pkg/...` OK; `go vet ./...` OK.
- [ ] `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` all PASS.
- [ ] `golangci-lint run ./...` exit 0.
- [ ] `./bin/polyant-mcp-server` builds.
- [ ] Manual smoke (optional): send `tools/list` → 7 tools; `tools/call polyant_get {id:...}` → entry; `tools/call polyant_list_categories` → categories.
- [ ] Branch `r4f-mcp-extensions` has 1 task commit + spec/plan, ready for review/merge.
