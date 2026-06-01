# 统一智能体技能架构设计文档

## 1. 概述

### 1.1 背景

Polyant 已完成 Claude Code 技能开发（阶段1），现需要扩展到其他流行智能体工具，包括：
- **Codex CLI** (OpenAI) — 轻量级终端编码智能体
- **Hermes Agent** (Nous Research) — 自我改进的 AI 智能体
- **OpenClaw** — 个人 AI 助手

### 1.2 设计目标

1. 采用统一智能体技能标准（agentskills.io）
2. 为 OpenClaw 单独适配
3. 重新设计技能体系
4. 同时构建 MCP 服务器

### 1.3 核心洞察

**统一标准的价值：**
- Codex 和 Hermes 都支持 agentskills.io 开放标准
- 一套技能可同时服务于多个智能体
- 降低维护成本，提高可移植性

**混合架构的必要性：**
- OpenClaw 有独立的技能系统，需要单独适配
- Claude Code 保持现有格式不变
- MCP 服务器作为通用集成层

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          智能体生态                                      │
├─────────────────────────────────────────────────────────────────────────┤
│   Claude Code   │    Codex     │    Hermes    │   OpenClaw   │   其他   │
└────────┬────────┴──────┬───────┴──────┬───────┴──────┬───────┴─────┬────┘
         │               │              │              │             │
         ▼               ▼              ▼              ▼             ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         集成层                                          │
├─────────────────────────────────────────────────────────────────────────┤
│ Claude Code │ agentskills.io │  Hermes   │  OpenClaw │   MCP    │ REST │
│   Skills    │    Skills      │   Skills  │   Skills  │  Server  │ API  │
└─────────────┴────────────────┴───────────┴───────────┴──────────┴──────┘
         │               │              │              │         │
         ▼               ▼              ▼              ▼         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                     共享 SDK (pkg/polyant-sdk)                           │
├─────────────────────────────────────────────────────────────────────────┤
│   客户端    │    认证     │    类型    │    工具    │    配置             │
└─────────────┴─────────────┴────────────┴────────────┴───────────────────┘
         │               │              │              │
         ▼               ▼              ▼              ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      Polyant 知识库                                      │
├─────────────────────────────────────────────────────────────────────────┤
│   搜索引擎   │   存储引擎   │   同步引擎   │   认证引擎   │   分类引擎   │
└──────────────┴──────────────┴──────────────┴──────────────┴─────────────┘
```

### 2.2 核心组件

| 组件 | 职责 | 技术栈 |
|------|------|--------|
| `pkg/polyant-sdk` | 共享 Go SDK，封装 API 调用 | Go |
| `skills/agentskills/` | agentskills.io 标准技能 | Markdown + Shell |
| `skills/openclaw/` | OpenClaw 专用技能 | Markdown |
| `skills/claude-code/` | 现有 Claude Code 技能 | Markdown |
| `mcp-server/` | MCP 服务器 | Go |
| `scripts/install-unified.sh` | 统一安装脚本 | Bash |

### 2.3 数据流

```
智能体调用技能
    │
    ├──→ agentskills.io 技能 ──→ scripts ──→ pactl CLI ──→ Polyant API
    │
    ├──→ OpenClaw 技能 ──→ 直接调用 pactl CLI ──→ Polyant API
    │
    ├──→ Claude Code 技能 ──→ 直接调用 pactl CLI ──→ Polyant API
    │
    └──→ MCP 服务器 ──→ SDK ──→ Polyant API
```

## 3. 技能体系设计

### 3.1 技能分类

基于现有功能和新需求，重新设计为以下技能：

| 技能名称 | 功能 | 触发场景 |
|----------|------|----------|
| `polyant-config` | 配置连接参数 | 首次使用、配置变更 |
| `polyant-search` | 搜索知识库 | 遇到问题、需要参考 |
| `polyant-save` | 保存知识经验 | 完成任务、解决问题 |
| `polyant-learn` | 学习新知识 | 遇到新技术、深入理解 |
| `polyant-rate` | 评价知识条目 | 使用知识后、提供反馈 |
| `polyant-sync` | 同步知识库 | 离线工作、数据同步 |

### 3.2 技能格式对比

| 格式 | 适用智能体 | 文件结构 | 特点 |
|------|------------|----------|------|
| agentskills.io | Codex, Hermes | `SKILL.md` + `scripts/` | 标准化、可移植、支持脚本 |
| OpenClaw | OpenClaw | `.md` 文件 | 自有格式、简单、中文友好 |
| Claude Code | Claude Code | `.md` 文件 | 现有格式、保持不变 |

**重要说明：** Codex 和 Hermes 都支持 agentskills.io 开放标准，因此一套技能可以同时在这两个智能体上运行。Hermes 额外支持 Skills Hub 用于技能分享，以及自学习机制来改进技能。

### 3.3 agentskills.io 标准格式

```
skills/agentskills/
├── polyant-search/
│   ├── SKILL.md              # 技能定义（必需）
│   ├── scripts/
│   │   └── search.sh         # 辅助脚本（可选）
│   └── references/
│       └── api-docs.md       # 参考文档（可选）
├── polyant-save/
│   ├── SKILL.md
│   └── scripts/
│       └── save.sh
├── polyant-learn/
│   ├── SKILL.md
│   └── scripts/
│       └── learn.sh
├── polyant-rate/
│   ├── SKILL.md
│   └── scripts/
│       └── rate.sh
└── polyant-sync/
    ├── SKILL.md
    └── scripts/
        └── sync.sh
```

### 3.4 SKILL.md 示例（polyant-search）

```markdown
---
name: polyant-search
description: 搜索 Polyant 知识库，查找解决方案、最佳实践和技术文档
---

# Polyant 搜索技能

## 功能说明

当遇到技术问题时，自动搜索 Polyant 知识库获取解决方案。

## 触发条件

- 遇到编译错误或运行时错误
- 需要查找最佳实践
- 寻找代码示例
- 了解新技术

## 使用方法

1. 提取问题关键词
2. 调用搜索命令
3. 解析结果并展示

## 命令参考

```bash
# 基础搜索
pactl search -q "关键词"

# 分类搜索
pactl search -q "关键词" -c "computer-science/programming-languages/go"

# 标签搜索
pactl search -q "关键词" -t "error,compilation"
```

## 输出格式

搜索结果包含：
- 标题
- 摘要
- 分类
- 相关度评分
- 链接
```

### 3.5 OpenClaw 技能格式

OpenClaw 使用 `.md` 文件格式，每个技能是一个独立的 Markdown 文件。

**技能发现机制：**
- OpenClaw 扫描 `~/.openclaw/skills/` 目录下的 `.md` 文件
- 文件名即为技能名称（不含扩展名）
- 支持中文文件名和内容

**技能文件结构：**
```
skills/openclaw/
├── polyant-search.md
├── polyant-save.md
├── polyant-learn.md
├── polyant-rate.md
└── polyant-sync.md
```

**与 agentskills.io 的区别：**
| 方面 | agentskills.io | OpenClaw |
|------|----------------|----------|
| 文件结构 | 目录 + SKILL.md | 单个 .md 文件 |
| 脚本支持 | 支持 scripts/ 目录 | 不支持独立脚本 |
| 元数据格式 | YAML frontmatter | Markdown 标题 |
| 安装位置 | ~/.agents/skills/ 或 ~/.hermes/skills/ | ~/.openclaw/skills/ |

### 3.6 OpenClaw 技能示例

```markdown
# Polyant 搜索技能

## 触发条件

当遇到以下情况时，自动触发搜索：
- 编译错误
- 运行时错误
- 性能问题
- 架构问题
- 需要查找最佳实践

## 使用方法

1. 提取错误关键词或问题描述
2. 调用搜索命令：
   ```bash
   pactl search -q "关键词"
   ```
3. 解析搜索结果
4. 展示最相关的解决方案

## 示例

用户：我遇到了一个编译错误：undefined: fmt.Println

智能体：让我搜索知识库...
```bash
pactl search -q "undefined fmt.Println"
```

找到 3 个相关条目：
1. Go 语言常见错误：fmt 包未导入
2. Go 语言编译错误解决方案
3. Go 语言最佳实践：导入管理

## 配置要求

确保已配置 Polyant 连接：
```bash
export POLYANT_API_URL=http://localhost:8080
export POLYANT_API_KEY=your-api-key
```
```

## 4. MCP 服务器设计

### 4.1 MCP 工具定义

| 工具名称 | 功能 | 输入参数 | 输出 |
|----------|------|----------|------|
| `polyant_search` | 搜索知识库 | query, category?, tags? | 条目列表 |
| `polyant_create` | 创建知识条目 | title, content, category, tags | 创建的条目 |
| `polyant_update` | 更新知识条目 | id, title?, content?, category?, tags? | 更新后的条目 |
| `polyant_rate` | 评价知识条目 | id, score, comment? | 评价结果 |
| `polyant_get` | 获取单个条目 | id | 条目详情 |
| `polyant_list` | 列出条目 | category?, limit?, offset? | 条目列表 |

### 4.2 MCP 服务器架构

```
mcp-server/
├── main.go                 # 入口文件
├── server.go               # MCP 服务器实现
├── tools/
│   ├── search.go           # 搜索工具
│   ├── create.go           # 创建工具
│   ├── update.go           # 更新工具
│   ├── rate.go             # 评价工具
│   ├── get.go              # 获取工具
│   └── list.go             # 列表工具
├── types/
│   └── types.go            # 数据类型定义
└── config/
    └── config.go           # 配置管理
```

### 4.3 MCP 工具示例

```go
// tools/search.go
package tools

import (
    "context"
    "github.com/modelcontextprotocol/go-sdk/mcp"
    polyant "github.com/daifei/agentwiki/pkg/polyant-sdk"
)

type SearchTool struct {
    client *polyant.Client
}

func (t *SearchTool) Definition() mcp.Tool {
    return mcp.Tool{
        Name: "polyant_search",
        Description: "搜索 Polyant 知识库",
        InputSchema: mcp.InputSchema{
            Type: "object",
            Properties: map[string]mcp.Property{
                "query": {
                    Type: "string",
                    Description: "搜索关键词",
                },
                "category": {
                    Type: "string",
                    Description: "分类过滤（可选）",
                },
                "tags": {
                    Type: "array",
                    Items: mcp.Items{Type: "string"},
                    Description: "标签过滤（可选）",
                },
            },
            Required: []string{"query"},
        },
    }
}

func (t *SearchTool) Execute(ctx context.Context, params map[string]interface{}) (*mcp.Result, error) {
    query := params["query"].(string)
    category, _ := params["category"].(string)
    tags, _ := params["tags"].([]interface{})

    // 转换 tags 类型
    tagStrings := make([]string, len(tags))
    for i, tag := range tags {
        tagStrings[i] = tag.(string)
    }

    results, err := t.client.Search(ctx, query, category, tagStrings)
    if err != nil {
        return nil, err
    }
    
    return &mcp.Result{
        Content: []mcp.Content{
            {Type: "text", Text: formatResults(results)},
        },
    }, nil
}
```

### 4.4 MCP 配置文件

```json
{
  "mcpServers": {
    "polyant": {
      "command": "polyant-mcp-server",
      "args": ["--config", "~/.polyant/config.json"],
      "env": {
        "POLYANT_API_URL": "http://localhost:8080",
        "POLYANT_API_KEY": "your-api-key"
      }
    }
  }
}
```

## 5. 共享 SDK 设计

### 5.1 SDK 架构

```
pkg/polyant-sdk/
├── client.go               # API 客户端
├── types.go                # 数据类型定义
├── auth.go                 # 认证逻辑
├── search.go               # 搜索功能
├── entry.go                # 条目操作
├── rating.go               # 评价功能
├── config.go               # 配置管理
└── errors.go               # 错误处理
```

### 5.2 核心类型定义

```go
// types.go
package polyant_sdk

type Entry struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Content   string    `json:"content"`
    Category  string    `json:"category"`
    Tags      []string  `json:"tags"`
    Score     float64   `json:"score"`
    ScoreCount int      `json:"score_count"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    CreatedBy string    `json:"created_by"`
}

type SearchResult struct {
    Entries  []Entry `json:"entries"`
    Total    int     `json:"total"`
    Page     int     `json:"page"`
    PageSize int     `json:"page_size"`
}

type Rating struct {
    Score   int    `json:"score"`
    Comment string `json:"comment"`
}
```

### 5.3 客户端接口

```go
// client.go
package polyant_sdk

type Client interface {
    // 搜索知识库
    Search(ctx context.Context, query string, category string, tags []string) (*SearchResult, error)
    
    // 获取单个条目
    GetEntry(ctx context.Context, id string) (*Entry, error)
    
    // 创建条目
    CreateEntry(ctx context.Context, entry *Entry) (*Entry, error)
    
    // 更新条目
    UpdateEntry(ctx context.Context, id string, entry *Entry) (*Entry, error)
    
    // 删除条目
    DeleteEntry(ctx context.Context, id string) error
    
    // 评价条目
    RateEntry(ctx context.Context, id string, rating *Rating) error
    
    // 列出条目
    ListEntries(ctx context.Context, category string, limit, offset int) (*SearchResult, error)
}
```

### 5.4 HTTP 客户端实现

```go
// http_client.go
package polyant_sdk

type HTTPClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func NewHTTPClient(baseURL, apiKey string) *HTTPClient {
    return &HTTPClient{
        baseURL:    baseURL,
        apiKey:     apiKey,
        httpClient: &http.Client{},
    }
}

func (c *HTTPClient) Search(ctx context.Context, query string, category string, tags []string) (*SearchResult, error) {
    // 构建请求
    req, err := c.buildRequest(ctx, "GET", "/api/v1/search", nil)
    if err != nil {
        return nil, err
    }
    
    // 添加查询参数
    q := req.URL.Query()
    q.Set("q", query)
    if category != "" {
        q.Set("category", category)
    }
    if len(tags) > 0 {
        q.Set("tags", strings.Join(tags, ","))
    }
    req.URL.RawQuery = q.Encode()
    
    // 发送请求
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // 解析响应
    var result SearchResult
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    return &result, nil
}
```

## 6. 安装与部署

### 6.1 统一安装脚本

```bash
#!/bin/bash
# scripts/install-unified.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Polyant 智能体技能安装器 ==="

# 检测已安装的智能体
detect_agents() {
    local agents=()
    
    # 检测 Claude Code
    if [ -d ~/.claude ]; then
        agents+=("claude-code")
    fi
    
    # 检测 Codex
    if [ -d ~/.agents ]; then
        agents+=("codex")
    fi
    
    # 检测 Hermes
    if [ -d ~/.hermes ]; then
        agents+=("hermes")
    fi
    
    # 检测 OpenClaw
    if [ -d ~/.openclaw ]; then
        agents+=("openclaw")
    fi
    
    echo "${agents[@]}"
}

# 安装 agentskills.io 标准技能
install_agentskills() {
    echo "安装 agentskills.io 标准技能..."
    
    # 安装到 Codex
    if [ -d ~/.agents ]; then
        mkdir -p ~/.agents/skills
        cp -r "$PROJECT_ROOT/skills/agentskills/"* ~/.agents/skills/
        echo "✓ 已安装到 Codex"
    fi
    
    # 安装到 Hermes
    if [ -d ~/.hermes ]; then
        mkdir -p ~/.hermes/skills
        cp -r "$PROJECT_ROOT/skills/agentskills/"* ~/.hermes/skills/
        echo "✓ 已安装到 Hermes"
    fi
}

# 安装 OpenClaw 技能
install_openclaw() {
    echo "安装 OpenClaw 技能..."
    
    if [ -d ~/.openclaw ]; then
        mkdir -p ~/.openclaw/skills
        cp -r "$PROJECT_ROOT/skills/openclaw/"* ~/.openclaw/skills/
        echo "✓ 已安装到 OpenClaw"
    fi
}

# 安装 Claude Code 技能
install_claude_code() {
    echo "安装 Claude Code 技能..."
    
    if [ -d ~/.claude ]; then
        mkdir -p ~/.claude/skills
        cp -r "$PROJECT_ROOT/skills/claude-code/"* ~/.claude/skills/
        echo "✓ 已安装到 Claude Code"
    fi
}

# 主流程
main() {
    local agents=$(detect_agents)
    
    if [ -z "$agents" ]; then
        echo "未检测到已安装的智能体"
        echo "请先安装以下智能体之一："
        echo "  - Claude Code"
        echo "  - Codex"
        echo "  - Hermes Agent"
        echo "  - OpenClaw"
        exit 1
    fi
    
    echo "检测到以下智能体：$agents"
    echo ""
    
    # 安装技能
    install_agentskills
    install_openclaw
    install_claude_code
    
    echo ""
    echo "=== 安装完成 ==="
    echo ""
    echo "请配置 Polyant 连接："
    echo "  export POLYANT_API_URL=http://localhost:8080"
    echo "  export POLYANT_API_KEY=your-api-key"
    echo ""
    echo "或运行配置技能："
    echo "  /polyant-config"
}

main "$@"
```

### 6.2 MCP 服务器安装

```bash
# 安装 MCP 服务器
go install github.com/daifei/agentwiki/cmd/polyant-mcp-server@latest

# 配置 MCP 服务器
cat > ~/.config/claude/mcp.json << EOF
{
  "mcpServers": {
    "polyant": {
      "command": "polyant-mcp-server",
      "args": ["--config", "~/.polyant/config.json"]
    }
  }
}
EOF
```

### 6.3 配置文件结构

```
~/.polyant/
├── config.json           # 主配置文件
├── credentials.json      # 认证信息（可选）
└── cache/                # 本地缓存
    └── entries/
```

## 7. 开发计划

### 7.1 阶段划分

| 阶段 | 内容 | 时间 | 依赖 |
|------|------|------|------|
| 阶段1 | 共享 SDK 开发 | 1 周 | 无 |
| 阶段2 | agentskills.io 标准技能 | 1 周 | 阶段1 |
| 阶段3 | MCP 服务器 | 1 周 | 阶段1 |
| 阶段4 | OpenClaw 适配 | 3 天 | 阶段2 |
| 阶段5 | 测试与文档 | 3 天 | 阶段2-4 |

### 7.2 详细任务

#### 阶段1：共享 SDK 开发

- [ ] 创建 `pkg/polyant-sdk` 目录结构
- [ ] 实现数据类型定义
- [ ] 实现 HTTP 客户端
- [ ] 实现认证逻辑
- [ ] 编写单元测试
- [ ] 编写使用文档

#### 阶段2：agentskills.io 标准技能

- [ ] 创建 `skills/agentskills/` 目录结构
- [ ] 实现 polyant-search 技能
- [ ] 实现 polyant-save 技能
- [ ] 实现 polyant-learn 技能
- [ ] 实现 polyant-rate 技能
- [ ] 实现 polyant-sync 技能
- [ ] 编写辅助脚本
- [ ] 编写安装脚本

#### 阶段3：MCP 服务器

- [ ] 创建 `cmd/polyant-mcp-server/` 目录结构
- [ ] 实现 MCP 服务器框架
- [ ] 实现搜索工具
- [ ] 实现创建工具
- [ ] 实现更新工具
- [ ] 实现评价工具
- [ ] 实现获取工具
- [ ] 实现列表工具
- [ ] 编写配置管理
- [ ] 编写使用文档

#### 阶段4：OpenClaw 适配

- [ ] 创建 `skills/openclaw/` 目录结构
- [ ] 实现 polyant-search 技能
- [ ] 实现 polyant-save 技能
- [ ] 实现 polyant-learn 技能
- [ ] 实现 polyant-rate 技能
- [ ] 实现 polyant-sync 技能
- [ ] 编写安装脚本

#### 阶段5：测试与文档

- [ ] 集成测试
- [ ] 性能测试
- [ ] 编写用户文档
- [ ] 编写开发者文档
- [ ] 更新 README

## 8. 风险与缓解

### 8.1 技术风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| agentskills.io 标准变更 | 需要重新适配 | 关注标准更新，保持代码灵活 |
| OpenClaw 技能系统变更 | 需要重新适配 | 监控 OpenClaw 更新，及时调整 |
| MCP 协议变更 | 需要更新服务器 | 使用官方 SDK，关注协议演进 |

### 8.2 进度风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| SDK 开发延迟 | 影响后续阶段 | 优先实现核心功能，迭代完善 |
| 技能开发工作量大 | 延期交付 | 分批交付，先完成核心技能 |
| 测试覆盖不足 | 质量问题 | 制定测试计划，持续集成 |

## 9. 成功标准

### 9.1 功能标准

- [ ] 共享 SDK 支持所有核心 API
- [ ] agentskills.io 技能在 Codex 和 Hermes 上正常运行
- [ ] OpenClaw 技能在 OpenClaw 上正常运行
- [ ] MCP 服务器支持所有定义的工具
- [ ] 统一安装脚本正常工作

### 9.2 质量标准

- [ ] 单元测试覆盖率 > 80%
- [ ] 集成测试通过
- [ ] 文档完整清晰
- [ ] 无已知关键缺陷

### 9.3 性能标准

- [ ] API 响应时间 < 500ms
- [ ] 搜索结果返回时间 < 1s
- [ ] MCP 服务器启动时间 < 2s

## 10. 附录

### 10.1 参考资料

- [agentskills.io 开放标准](https://agentskills.io)
- [Codex CLI 技能文档](https://developers.openai.com/codex/skills)
- [Hermes Agent 技能系统](https://hermes-agent.nousresearch.com/docs/skills)
- [OpenClaw GitHub](https://github.com/openclaw/openclaw)
- [MCP 协议规范](https://modelcontextprotocol.io)
- [Polyant API 文档](docs/skill.md)

### 10.2 术语表

| 术语 | 说明 |
|------|------|
| agentskills.io | 智能体技能开放标准 |
| MCP | Model Context Protocol，模型上下文协议 |
| SDK | Software Development Kit，软件开发工具包 |
| 技能 | 智能体可执行的特定功能单元 |
| 工具 | MCP 服务器提供的可调用功能 |

---

**文档版本：** v1.0  
**创建日期：** 2026-06-01  
**作者：** Polyant Team  
**状态：** 待审核
