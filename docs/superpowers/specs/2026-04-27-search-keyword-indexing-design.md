# Search Keyword Indexing Design

## Overview

给搜索结果的 Markdown 内容中增加词条标题的内联链接，同时在 API 响应中返回知识图谱数据（nodes + edges），让智能体能够识别内容中引用了哪些已有词条并进行进一步检索。

## Motivation

当前搜索 API 返回的结果是纯文本 Markdown，智能体无法知道内容中哪些术语对应系统中已有的词条。通过在 content 中插入 `[术语](entry://<id>)` 格式的链接，并附带结构化的图谱 JSON，智能体可以：
- 直接解析 Markdown 链接获取可进一步检索的词条
- 利用 graph 字段构建知识关联网络

## Architecture

新增两个模块，改动两个已有模块：

```
EntryHandler (改)
  SearchHandler → 拿到搜索结果 → ResultEnricher → 返回增强响应
                                 │
                  ┌──────────────┘
                  ▼
          ResultEnricher (新)
            - InsertLinks(content, matches) → 插入了链接的 Markdown
            - BuildGraph(resultEntries, matches) → {nodes, edges}
                  │
                  ▼
          TitleIndex (新)
            - AC 自动机维护所有 published 词条标题
            - MatchAll(content) → []Match
```

## Data Flow

1. `SearchHandler` 调用 `SearchEngine.Search()` 得到 `[]*KnowledgeEntry`
2. 通过 `EntryStore` 补全 entry 的 Content（Bleve 不存 content）
3. 调用 `ResultEnricher.Enrich(entries)`
4. `Enricher` 调 `TitleIndex.MatchAll(content)` 对每个 entry 找到所有标题匹配
5. 过滤保护区（代码块、已有链接、URL 等），按最长匹配优先去重叠
6. 从后往前插入 Markdown 链接，跳过自引用
7. 构建 `SearchGraph`（nodes + edges）
8. 返回扩展后的 `PagedData`

## New Types

### TitleIndex (`internal/storage/index/title_index.go`)

```go
type TitleEntry struct {
    ID    string
    Title string
}

type Match struct {
    Title   string `json:"title"`
    EntryID string `json:"entryId"`
    Offset  int    `json:"offset"`
}

type TitleIndex struct {
    automaton *ahocorasick.Matcher
    entries   map[string]TitleEntry
    mu        sync.RWMutex
}

func (t *TitleIndex) Build(entries []TitleEntry) error
func (t *TitleIndex) Add(entry TitleEntry) error
func (t *TitleIndex) Remove(title string) error
func (t *TitleIndex) Update(old, new TitleEntry) error
func (t *TitleIndex) MatchAll(content string) []Match
```

### ResultEnricher (`internal/api/handler/search_enricher.go`)

```go
type GraphNode struct {
    ID    string `json:"id"`
    Title string `json:"title"`
    Type  string `json:"type"` // "result" | "reference"
}

type GraphEdge struct {
    From     string `json:"from"`
    To       string `json:"to"`
    Relation string `json:"relation"` // "mentions"
}

type SearchGraph struct {
    Nodes []GraphNode `json:"nodes"`
    Edges []GraphEdge `json:"edges"`
}

type ResultEnricher struct {
    titleIndex *index.TitleIndex
    entryStore storage.EntryStore
}

func (e *ResultEnricher) Enrich(entries []*model.KnowledgeEntry) (*SearchGraph, error)
```

## API Response Changes

`PagedData` 新增 `graph` 字段：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "totalCount": 10,
    "hasMore": true,
    "graph": {
      "nodes": [
        {"id": "abc123", "title": "深度学习", "type": "result"},
        {"id": "def456", "title": "神经网络", "type": "reference"}
      ],
      "edges": [
        {"from": "abc123", "to": "def456", "relation": "mentions"}
      ]
    },
    "items": [...]
  }
}
```

每个 entry 的 `content` 中匹配到的词条标题被替换为 `[术语](entry://<entry-id>)`。

## Matching Rules

- **匹配依据**：仅词条标题（Title），精确匹配
- **匹配范围**：所有 `published` 状态的词条
- **重叠处理**：优先最长匹配（「机器学习系统」>「机器学习」）
- **自引用过滤**：条目不链接到自身

## Markdown Protection

插入链接时跳过以下位置：
- 代码块内（`` ``` `` 包裹）
- 行内代码（`` ` `` 包裹）
- 已有 Markdown 链接的文本部分
- 图片 alt 文本
- URL 内

实现：先扫描标记保护区区间，匹配结果落入保护区则丢弃。

## Lifecycle Sync

| 事件 | 操作 | 复杂度 |
|------|------|--------|
| 启动 | `Build(allTitles)` 全量加载 | O(N × L)，N 为词条数，L 为平均标题长度 |
| 创建 published 词条 | `Add()` 增量插入 | O(len(title)) |
| 更新词条（标题变或状态变） | `Update()` 触全量重建 | O(N × L) |
| 删除词条 | `Remove()` 触发全量重建 | O(N × L) |

AC 自动机不支持高效删除（fail links 交叉指向），所以 Remove/Update 触发全量重建。考虑到 published 词条数在几千到几万量级，重建时间为毫秒级，可接受。

## Wiring

`TitleIndex` 放在 `Store` 中，与 `SearchEngine` 平级，在 `NewPersistentStore()` 初始化时构建，注入给 `EntryHandler`。`EntryHandler` 的 CRUD 方法在操作完成后同步更新 `TitleIndex`。

## Testing

### TitleIndex 单元测试

| 用例 | 覆盖 |
|------|------|
| `TestBuild_Success` | 多个模式匹配全部找到 |
| `TestAdd_Incremental` | Add 后新模式可匹配 |
| `TestRemove_Rebuild` | Remove 后不再匹配，其余不受影响 |
| `TestUpdate_Rebuild` | Update 旧→新生效 |
| `TestMatchAll_Overlapping` | 最长匹配优先 |
| `TestMatchAll_NoMatch` | 无匹配返回空 |
| `TestMatchAll_EmptyInput` | 空输入边界 |
| `TestMatchAll_Chinese` | 中文精确匹配 |
| `TestMatchAll_SpecialChars` | 正则特殊字符不误触发 |

### ResultEnricher 单元测试

| 用例 | 覆盖 |
|------|------|
| `TestInsertLinks_Basic` | 单匹配正确插入 |
| `TestInsertLinks_Multiple` | 多匹配按 offset 正确插入 |
| `TestInsertLinks_SkipCodeBlock` | 代码块内跳过 |
| `TestInsertLinks_SkipInlineCode` | 行内代码内跳过 |
| `TestInsertLinks_SkipExistingLink` | 已有链接不破坏 |
| `TestInsertLinks_SkipURL` | URL 内跳过 |
| `TestInsertLinks_SelfRef` | 自引用不生成链接 |
| `TestBuildGraph_SingleResult` | 单结果多引用 graph 正确 |
| `TestBuildGraph_MultipleResults` | 多结果互引去重正确 |
| `TestBuildGraph_NoReferences` | 无匹配时 graph 为空 |

### 集成测试

| 用例 | 覆盖 |
|------|------|
| `TestSearchHandler_WithGraph` | 端到端：搜索返回带 graph 和 Markdown 链接的完整响应 |
| `TestSearchHandler_NoGraphWhenEmpty` | 无可匹配词条时 graph 为空 |

## Approved Design Decisions

- 方案：Aho-Corasick 自动机索引（方案 2）
- 两者都要：Markdown 内联链接 + JSON 图谱
- 匹配依据：仅词条标题
- 匹配方式：精确匹配
- 匹配范围：全库 published 词条
- 图谱格式：nodes + edges
