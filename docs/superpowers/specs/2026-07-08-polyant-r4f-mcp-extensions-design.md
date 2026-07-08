# R4f MCP 扩展设计

**范围**：R4 第六个（最后一个）迷你轮——MCP server 工具扩展。R4a-R4e 已合入 master（`7246d1d`）。MCP server（`cmd/polyant-mcp-server/`）当前 3 个工具（search/create/rate），polysdk 还有 4 个方法未暴露（get/update/delete/list-categories）。本轮加这 4 个 + 接线 Ed25519 密钥加载（修复 write 工具的认证）。

**轮次定位**：R4f 只做 MCP 工具扩展 + 密钥接线，不改 MCP 协议版本/能力、不改 polysdk 业务方法。

## 目标

- **4 个新工具**：`polyant_get`、`polyant_update`、`polyant_delete`、`polyant_list_categories`。
- **接线密钥加载**：MCP server `NewServer` 读 `KeyDir` → `ed25519.LoadKeyPair` → `client.SetKeys`，让 write 工具（create/rate/update/delete）真正可用（Ed25519 签名）。
- `polyant_delete` 加 `confirm: bool` 必填标志防误删。

## 非目标

- handler 级 HTTP mock 测试（现有测试协议级，保持一致）。
- MCP server 嵌入节点进程（保持独立 stdio 二进制）。
- 新 MCP 协议版本/能力（保持 `"2024-11-05"`、`{tools:{}}`）。
- polysdk 业务方法改动（复用现成 GetEntry/UpdateEntry/DeleteEntry/ListCategories）。
- update 的显式清空（omitempty plain string → 空字符串=不改；显式清空留后续）。

## 现状核实（代码 grounded）

| 能力 | 现状 | 位置 |
|---|---|---|
| MCP 工具数 | 3（search/create/rate） | `server.go:129-208`、test `:103` count=3 |
| 工具添加方式 | 3 手动编辑（list + switch case + handler） | `server.go:129,236` |
| polysdk 未暴露方法 | GetEntry/UpdateEntry/DeleteEntry/ListCategories | `pkg/polysdk/client.go:89,119,153,170` |
| **密钥加载** | **缺失**——KeyDir 在 config 但 NewServer 从不加载 | `config.go:10`、`server.go:53-64` |
| Ed25519 密钥加载器 | `ed25519.LoadKeyPair(dir)→(priv,pub,err)` | `internal/auth/ed25519/keys.go:139` |
| client.SetKeys | `SetKeys(publicKey, privateKey []byte)` | `pkg/polysdk/client.go:51` |
| create/rate 签名 | 内部 `HasKeys()` 门控——无密钥跳过签名 | `client.go:102-116` |
| error 约定 | arg-parse 失败→JSON-RPC `-32602` Error；SDK 错误→Result 文本 | `server.go:329-367` |
| 格式化器 | `formatResult`(string)、`formatEntries`([]Entry) | `server.go:255-286` |

## 架构：密钥接线 + 4 工具

**密钥接线**：`pkg/polysdk` 不能 import `internal/auth/ed25519`（pkg↛internal 边界），故 MCP server（`cmd/`，可 import internal）直接在 `NewServer` 调 `ed25519.LoadKeyPair(KeyDir)` → `client.SetKeys(pub, priv)`。

**4 工具**：照搬 `handleCreate` 模式（params struct → unmarshal → polysdk 调用 → formatResult）。每工具 3 处编辑（list + switch + handler）。

## 组件

### 1. 密钥接线（`cmd/polyant-mcp-server/server.go` NewServer）

```go
func NewServer(config *Config) (*Server, error) {
	client := polysdk.NewClient(config.BaseURL)
	if config.APIKey != "" {
		client.SetAPIKey(config.APIKey)
	}
	// R4f：加载 Ed25519 密钥（启用 write 工具签名）
	if config.KeyDir != "" {
		priv, pub, err := ed25519.LoadKeyPair(config.KeyDir)
		if err == nil {
			client.SetKeys(pub, priv)
		}
		// 加载失败不 fatal——read-only 工具仍可用（API key）；write 工具会因无签名失败并报错
	}
	return &Server{client: client, reader: bufio.NewReader(os.Stdin), writer: os.Stdout}, nil
}
```
加 import `"github.com/daifei0527/polyant/internal/auth/ed25519"`。加载失败仅日志（不 fatal）——read-only 工具（search/get/list-categories）仍可用；write 工具会因签名缺失在 SDK 调用时返回错误（handler 显示给用户）。

### 2. 4 新工具（`server.go`）

每个工具 = handleListTools 加条目 + handleCallTool switch 加 case + handler 函数。

- **`polyant_get`**：`handleGet(id, args)`——args `{id: string, lang?: string}` → `client.GetEntry(ctx, id, lang)` → `formatEntry`。
- **`polyant_update`**：`handleUpdate(id, args)`——args `{id, title?, content?, category?, tags?}` → `client.UpdateEntry(ctx, id, &UpdateEntryRequest{...})` → `formatEntry`。空字段=不改（omitempty）。
- **`polyant_delete`**：`handleDelete(id, args)`——args `{id: string, confirm: bool}`。confirm=false → `formatResult("请设 confirm=true 确认删除")` 返回（不调 SDK）；confirm=true → `client.DeleteEntry(ctx, id)` → 成功字符串。
- **`polyant_list_categories`**：`handleListCategories(id, args)`——无 args → `client.ListCategories(ctx)` → `formatCategories`。

### 3. 格式化器（`server.go`）

- `formatEntry(e *polysdk.Entry) string`：单条目（标题/ID/分类/标签/评分/内容摘要），复用 formatEntries 的字段集。
- `formatCategories(cats []polysdk.Category) string`：列表（ID/Name/Description）。

### 4. 测试（`server_test.go`）

- `TestHandleListTools`：count `3`→`7`；`expectedNames` 加 4 个新名字。
- `TestFormatEntry`：单条目格式化（含可选字段）。
- `TestFormatCategories`：空 + 非空。
- dispatch：4 新 case 路由（可选——现有 unknown-tool 测试已覆盖 switch；新 case 的参数解析测试可选）。
- delete confirm：confirm=false → 错误响应（不调 SDK）；confirm=true → 参数正确（协议级，不调真 API）。

## 数据流

- **get/list-categories**：只读，API key（公共路由），密钥未加载也可用。
- **create/rate/update/delete**：write，Ed25519 authMW——密钥接线后 client 自动签名（`HasKeys()` 门控）。
- **delete**：confirm=false → 提示（不删）；confirm=true → 删除 + 回显 ID。

## 接口变化

- MCP tools/list 从 3 个工具变 7 个（新增 4 个）。
- MCP server config：`KeyDir` 字段从"声明但未用"变为"实际加载密钥"。
- 无 polysdk / 节点端点 / 协议版本变化。

## 风险与回退

- **密钥格式**：`ed25519.LoadKeyPair` 读 `keypair.json`（或回退分离文件）。与 pactl 一致（同包）。加载失败不 fatal（read-only 仍可用）。
- **update omitempty**：空字符串=不改（无法显式清空）。MVP 可接受。
- **delete 破坏性**：confirm 标志防误删；响应回显 ID。
- 每个 task 独立 commit + 验证。

## 出范围跟踪

- handler 级 HTTP mock 测试。
- update 显式清空（指针字段）。
- MCP server 嵌入节点 / 新协议能力。
