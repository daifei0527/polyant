# Polyant Claude Code Skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Claude Code 开发技能，让智能体能够自动搜索、保存和学习知识库中的内容。

**Architecture:** 创建 3 个独立的技能文件（search.md、save.md、learn.md），每个技能包含触发条件、使用方法和示例。技能通过 pactl CLI 工具与 Polyant 知识库交互。

**Tech Stack:** Markdown, pactl CLI, Polyant REST API

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `skills/polyant-search.md` | 创建 | 搜索技能：遇到问题时自动搜索知识库 |
| `skills/polyant-save.md` | 创建 | 保存技能：完成任务后自动保存经验 |
| `skills/polyant-learn.md` | 创建 | 学习技能：遇到新知识时自动学习 |
| `skills/polyant-config.md` | 创建 | 配置技能：配置 Polyant 连接信息 |

---

### Task 1: 创建配置技能

**Files:**
- Create: `skills/polyant-config.md`

- [ ] **Step 1: 创建配置技能文件**

```markdown
---
name: polyant-config
description: Configure Polyant knowledge base connection settings
---

# Polyant Configuration

Configure the connection to your Polyant knowledge base node.

## Setup

1. **Set the API endpoint:**
   ```bash
   export POLYANT_API_URL="https://your-node.example.com:8080"
   ```

2. **Set the API key:**
   ```bash
   export POLYANT_API_KEY="sk_live_your_api_key_here"
   ```

3. **Set the node ID (optional):**
   ```bash
   export POLYANT_NODE_ID="your-node-id"
   ```

## Configuration File

Create `~/.polyant/config.json`:

```json
{
  "api_url": "https://your-node.example.com:8080",
  "api_key": "sk_live_your_api_key_here",
  "node_id": "your-node-id",
  "default_category": "computer-science/programming-languages/go",
  "auto_save": true,
  "auto_search": true
}
```

## Verify Configuration

```bash
pactl status
```

Expected output:
```
Connected to Polyant node: https://your-node.example.com:8080
Node ID: your-node-id
Status: online
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `POLYANT_API_URL` | API endpoint URL | `http://localhost:8080` |
| `POLYANT_API_KEY` | API authentication key | (required) |
| `POLYANT_NODE_ID` | Node identifier | (auto-generated) |
| `POLYANT_DEFAULT_CATEGORY` | Default category for new entries | `general-knowledge` |
| `POLYANT_AUTO_SAVE` | Auto-save completed tasks | `true` |
| `POLYANT_AUTO_SEARCH` | Auto-search on errors | `true` |

## Troubleshooting

**Connection refused:**
- Check if the Polyant node is running
- Verify the API URL is correct
- Check firewall settings

**Authentication failed:**
- Verify the API key is correct
- Check if the key has expired
- Ensure the key has proper permissions

**Timeout:**
- Check network connectivity
- Increase timeout: `export POLYANT_TIMEOUT=30`
```

- [ ] **Step 2: 提交配置技能**

```bash
git add skills/polyant-config.md
git commit -m "feat(skills): add Polyant configuration skill"
```

---

### Task 2: 创建搜索技能

**Files:**
- Create: `skills/polyant-search.md`

- [ ] **Step 1: 创建搜索技能文件**

```markdown
---
name: polyant-search
description: Search Polyant knowledge base for solutions to problems
triggers:
  - "遇到编译错误"
  - "遇到运行时错误"
  - "遇到性能问题"
  - "遇到架构问题"
  - "搜索最佳实践"
---

# Polyant Search

Automatically search the Polyant knowledge base when encountering problems.

## When to Use

- **Compilation errors:** When code fails to compile
- **Runtime errors:** When code crashes or produces unexpected results
- **Performance issues:** When code is slow or resource-intensive
- **Architecture questions:** When designing systems or making technical decisions
- **Best practices:** When looking for recommended approaches

## How It Works

1. **Detect the problem:** Identify the type of issue (error, performance, architecture)
2. **Extract keywords:** Pull relevant terms from error messages or context
3. **Search knowledge base:** Query Polyant for related entries
4. **Present solutions:** Show relevant knowledge entries with solutions

## Usage

### Automatic Trigger

When you encounter an error, the skill will automatically:

1. Analyze the error message
2. Search for similar issues in the knowledge base
3. Present potential solutions

**Example:**
```
User: I'm getting this error: "undefined: fmt.Println"

Skill: Searching Polyant knowledge base...

Found 3 relevant entries:

1. **Go Common Errors: Missing fmt Import**
   - Category: computer-science/programming-languages/go
   - Solution: Add `import "fmt"` to your file
   - Rating: 4.8/5 (127 reviews)

2. **Go Import Management Best Practices**
   - Category: computer-science/programming-languages/go
   - Solution: Use goimports to auto-manage imports
   - Rating: 4.6/5 (89 reviews)

3. **Go Compilation Error Troubleshooting**
   - Category: computer-science/programming-languages/go
   - Solution: Check import paths and package names
   - Rating: 4.5/5 (65 reviews)
```

### Manual Search

You can also explicitly search:

```bash
pactl search "golang error handling"
```

Or use the skill directly:

```
Search Polyant for: "Go error handling best practices"
```

## Search Tips

- **Be specific:** "Go nil pointer dereference" vs "Go error"
- **Include context:** "React useEffect cleanup function" vs "React hook"
- **Use error messages:** Copy exact error text for best results
- **Try variations:** If first search doesn't help, try different keywords

## Integration with Error Handling

The skill integrates with common error patterns:

### Go Errors
```go
// When you see: "cannot use x (type T) as type U"
// Skill searches for: "Go type conversion error"
```

### Python Errors
```python
# When you see: "NameError: name 'x' is not defined"
# Skill searches for: "Python NameError undefined variable"
```

### JavaScript Errors
```javascript
// When you see: "TypeError: Cannot read property 'x' of undefined"
// Skill searches for: "JavaScript TypeError undefined property"
```

## Configuration

| Setting | Description | Default |
|---------|-------------|---------|
| `POLYANT_AUTO_SEARCH` | Enable automatic search | `true` |
| `POLYANT_SEARCH_LIMIT` | Max results to show | `5` |
| `POLYANT_MIN_RATING` | Minimum rating filter | `3.0` |
| `POLYANT_SEARCH_TIMEOUT` | Search timeout (seconds) | `10` |

## Privacy

- Search queries are sent to your Polyant node
- Queries are not shared with other nodes
- Search history is stored locally only
```

- [ ] **Step 2: 提交搜索技能**

```bash
git add skills/polyant-search.md
git commit -m "feat(skills): add Polyant search skill"
```

---

### Task 3: 创建保存技能

**Files:**
- Create: `skills/polyant-save.md`

- [ ] **Step 1: 创建保存技能文件**

```markdown
---
name: polyant-save
description: Save knowledge and experience to Polyant knowledge base
triggers:
  - "完成任务"
  - "解决错误"
  - "发现最佳实践"
  - "学习新知识"
---

# Polyant Save

Automatically save knowledge and experience to the Polyant knowledge base.

## When to Use

- **Task completion:** After successfully finishing a coding task
- **Error resolution:** After solving a tricky bug or error
- **Best practices:** When discovering good patterns or approaches
- **New knowledge:** When learning something valuable

## How It Works

1. **Detect save-worthy content:** Identify knowledge that should be preserved
2. **Extract key information:** Pull title, description, solution, and context
3. **Generate knowledge entry:** Create a structured knowledge entry
4. **Save to Polyant:** Store the entry in the knowledge base

## Usage

### Automatic Save

After completing a task, the skill will automatically:

1. Analyze what was done
2. Extract the key knowledge
3. Offer to save it to Polyant

**Example:**
```
User: I fixed the nil pointer error by adding a nil check before accessing the struct field.

Skill: I see you solved a nil pointer issue. Would you like me to save this to Polyant?

**Proposed Entry:**
- Title: "Go Nil Pointer Dereference Prevention"
- Category: computer-science/programming-languages/go
- Tags: go, nil-pointer, error-handling, best-practice
- Content: 
  ## Problem
  Nil pointer dereference when accessing struct fields
  
  ## Solution
  Add nil check before accessing: `if obj != nil { obj.Field }`
  
  ## Prevention
  - Use pointer types carefully
  - Initialize structs properly
  - Consider using value types when nil isn't valid

Save to Polyant? [Y/n]
```

### Manual Save

You can also explicitly save:

```bash
pactl entry create \
  --title "Go Error Handling Pattern" \
  --category "computer-science/programming-languages/go" \
  --tags "go,error-handling,pattern" \
  --content "## Problem\nUnhandled errors in Go\n\n## Solution\nAlways check err != nil"
```

Or use the skill directly:

```
Save to Polyant:
Title: "Docker Multi-Stage Build Optimization"
Category: tools/dev-tools
Tags: docker, optimization, build
Content: [your knowledge here]
```

## Save Categories

Choose the most appropriate category:

### Programming Languages
- `computer-science/programming-languages/go`
- `computer-science/programming-languages/python`
- `computer-science/programming-languages/javascript`
- `computer-science/programming-languages/rust`

### Tools
- `tools/dev-tools` (Git, Docker, etc.)
- `tools/system-administration`

### AI/ML
- `artificial-intelligence/machine-learning`
- `artificial-intelligence/llm`
- `artificial-intelligence/prompt-engineering`

### General
- `general-knowledge/mathematics`
- `general-knowledge/physics`

## Entry Format

### Title
- Be specific and descriptive
- Include the technology/language
- Example: "Go Context Cancellation Pattern"

### Content
Structure your knowledge entry:

```markdown
## Problem
[Describe the problem or situation]

## Solution
[Provide the solution or approach]

## Example
[Include code examples if applicable]

## Why It Works
[Explain the reasoning]

## When to Use
[Describe appropriate use cases]

## References
[Link to relevant documentation]
```

### Tags
- Use lowercase
- Separate with commas
- Include technology, concept, and context
- Example: `go, context, cancellation, goroutine`

## Quality Guidelines

### Good Entries
- **Specific:** "Go Context Timeout for HTTP Handlers" vs "Go Context"
- **Complete:** Include problem, solution, and explanation
- **Practical:** Include working code examples
- **Relevant:** Focus on common issues and solutions

### Avoid
- **Too generic:** "Programming is hard"
- **Incomplete:** Problem without solution
- **Outdated:** Information for deprecated versions
- **Duplicate:** Check if similar entry exists first

## Privacy and Sharing

- **Private entries:** Only visible to you
- **Public entries:** Shared with all Polyant nodes
- **Default:** Private (you choose what to share)

## Configuration

| Setting | Description | Default |
|---------|-------------|---------|
| `POLYANT_AUTO_SAVE` | Prompt to save after tasks | `true` |
| `POLYANT_DEFAULT_PRIVACY` | Default privacy level | `private` |
| `POLYANT_DEFAULT_CATEGORY` | Default category | `general-knowledge` |
| `POLYANT_SAVE_CONFIRMATION` | Require confirmation | `true` |
```

- [ ] **Step 2: 提交保存技能**

```bash
git add skills/polyant-save.md
git commit -m "feat(skills): add Polyant save skill"
```

---

### Task 4: 创建学习技能

**Files:**
- Create: `skills/polyant-learn.md`

- [ ] **Step 1: 创建学习技能文件**

```markdown
---
name: polyant-learn
description: Learn from Polyant knowledge base to improve skills
triggers:
  - "遇到新知识"
  - "学习新技术"
  - "遇到最佳实践"
  - "需要深入理解"
---

# Polyant Learn

Learn from the Polyant knowledge base to improve your skills and understanding.

## When to Use

- **New technology:** When encountering unfamiliar tech
- **Deep understanding:** When you need to understand concepts deeply
- **Best practices:** When looking for recommended approaches
- **Skill building:** When wanting to improve in an area

## How It Works

1. **Identify learning goal:** Determine what you want to learn
2. **Search knowledge base:** Find relevant entries
3. **Study the material:** Read and understand the content
4. **Apply learning:** Use the knowledge in your work
5. **Save insights:** Optionally save what you learned

## Usage

### Learn About a Topic

```
Learn from Polyant: "Go concurrency patterns"

Skill: Found 12 entries about Go concurrency:

**Fundamentals:**
1. Go Goroutines Basics (Rating: 4.9/5)
2. Go Channels Explained (Rating: 4.8/5)
3. Go sync.WaitGroup Usage (Rating: 4.7/5)

**Patterns:**
4. Go Worker Pool Pattern (Rating: 4.6/5)
5. Go Fan-Out/Fan-In Pattern (Rating: 4.5/5)
6. Go Context Cancellation (Rating: 4.8/5)

**Advanced:**
7. Go Race Condition Prevention (Rating: 4.4/5)
8. Go Memory Model (Rating: 4.3/5)

Which would you like to learn about?
```

### Deep Dive

```
Deep dive into: "Go Worker Pool Pattern"

Skill: **Go Worker Pool Pattern**

**Overview:**
A concurrency pattern that limits the number of goroutines processing tasks simultaneously.

**When to Use:**
- Processing many tasks with limited resources
- Controlling resource usage (CPU, memory, connections)
- Rate limiting external API calls

**Implementation:**

```go
func workerPool(jobs <-chan Job, results chan<- Result, numWorkers int) {
    var wg sync.WaitGroup
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                results <- process(job)
            }
        }()
    }
    
    wg.Wait()
    close(results)
}
```

**Key Points:**
1. Use buffered channels for jobs and results
2. Number of workers based on resources
3. Always use WaitGroup for synchronization
4. Close results channel when done

**Related Entries:**
- Go Channels Explained
- Go sync.WaitGroup Usage
- Go Context Cancellation

Would you like to:
1. Learn more about related topics?
2. Practice with examples?
3. Save your own insights?
```

### Learning Path

```
Create learning path for: "Becoming a Go expert"

Skill: **Go Expert Learning Path**

**Phase 1: Fundamentals (Week 1-2)**
1. Go Syntax and Types
2. Go Error Handling
3. Go Concurrency Basics

**Phase 2: Intermediate (Week 3-4)**
4. Go Interfaces and Composition
5. Go Testing and Benchmarking
6. Go Standard Library

**Phase 3: Advanced (Week 5-6)**
7. Go Performance Optimization
8. Go System Programming
9. Go Design Patterns

**Phase 4: Expert (Week 7-8)**
10. Go Internals and Runtime
11. Go Toolchain and Build System
12. Go Ecosystem and Community

Start with Phase 1? [Y/n]
```

## Learning Strategies

### 1. Spaced Repetition
- Review material at increasing intervals
- Focus on weak areas
- Test yourself regularly

### 2. Active Learning
- Don't just read—practice
- Build projects using new knowledge
- Teach others what you learned

### 3. Connected Learning
- Link new knowledge to existing knowledge
- Build mental models
- Understand the "why" behind things

## Learning Features

### Knowledge Graph
- See how concepts connect
- Find prerequisites
- Discover related topics

### Progress Tracking
- Track what you've learned
- Identify knowledge gaps
- Set learning goals

### Practice Exercises
- Apply what you learned
- Get feedback
- Build muscle memory

## Integration with Development

### Before Coding
```
Before implementing X, learn about it first.
```

### During Coding
```
Stuck on Y? Search for learning resources.
```

### After Coding
```
Implemented Z? Save your learnings for others.
```

## Configuration

| Setting | Description | Default |
|---------|-------------|---------|
| `POLYANT_LEARNING_MODE` | Enable learning features | `true` |
| `POLYANT_TRACK_PROGRESS` | Track learning progress | `true` |
| `POLYANT_SUGGEST_RELATED` | Suggest related content | `true` |
| `POLYANT_LEARNING_STYLE` | Preferred learning style | `balanced` |

## Privacy

- Learning history is stored locally
- Progress is not shared with other nodes
- You control what you share
```

- [ ] **Step 2: 提交学习技能**

```bash
git add skills/polyant-learn.md
git commit -m "feat(skills): add Polyant learn skill"
```

---

### Task 5: 更新 pactl CLI 支持技能

**Files:**
- Modify: `cmd/pactl/main.go`

- [ ] **Step 1: 添加技能相关命令**

在 pactl CLI 中添加技能相关命令：

```bash
# 搜索知识库
pactl search "query"

# 创建条目
pactl entry create --title "Title" --category "category" --tags "tag1,tag2" --content "Content"

# 查看条目
pactl entry get <id>

# 更新条目
pactl entry update <id> --title "New Title"

# 删除条目
pactl entry delete <id>

# 列出分类
pactl category list

# 查看状态
pactl status
```

- [ ] **Step 2: 测试技能命令**

Run: `pactl search "test"`
Expected: 返回搜索结果或提示配置

Run: `pactl status`
Expected: 显示连接状态

- [ ] **Step 3: 提交 CLI 更新**

```bash
git add cmd/pactl/main.go
git commit -m "feat(pactl): add skill-related commands"
```

---

### Task 6: 创建技能安装脚本

**Files:**
- Create: `scripts/install-skills.sh`

- [ ] **Step 1: 创建安装脚本**

```bash
#!/bin/bash

# Polyant Skills Installer for Claude Code
# This script installs Polyant skills for Claude Code

set -e

echo "Installing Polyant skills for Claude Code..."

# Create skills directory if it doesn't exist
SKILLS_DIR="${HOME}/.claude/skills"
mkdir -p "$SKILLS_DIR"

# Copy skills
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SOURCE_DIR="$(dirname "$SCRIPT_DIR")/skills"

if [ ! -d "$SKILLS_SOURCE_DIR" ]; then
    echo "Error: Skills directory not found at $SKILLS_SOURCE_DIR"
    exit 1
fi

for skill in polyant-config.md polyant-search.md polyant-save.md polyant-learn.md; do
    if [ -f "$SKILLS_SOURCE_DIR/$skill" ]; then
        cp "$SKILLS_SOURCE_DIR/$skill" "$SKILLS_DIR/"
        echo "Installed: $skill"
    else
        echo "Warning: $skill not found"
    fi
done

echo ""
echo "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Configure your Polyant node:"
echo "   export POLYANT_API_URL='https://your-node.example.com:8080'"
echo "   export POLYANT_API_KEY='sk_live_your_api_key'"
echo ""
echo "2. Verify connection:"
echo "   pactl status"
echo ""
echo "3. Start using skills:"
echo "   - Search: 'Search Polyant for...'"
echo "   - Save: 'Save to Polyant...'"
echo "   - Learn: 'Learn from Polyant...'"
```

- [ ] **Step 2: 使脚本可执行**

Run: `chmod +x scripts/install-skills.sh`

- [ ] **Step 3: 测试安装脚本**

Run: `./scripts/install-skills.sh`
Expected: 技能文件安装到 ~/.claude/skills/

- [ ] **Step 4: 提交安装脚本**

```bash
git add scripts/install-skills.sh
git commit -m "feat(skills): add installation script"
```

---

### Task 7: 创建使用文档

**Files:**
- Create: `docs/skills/README.md`

- [ ] **Step 1: 创建使用文档**

```markdown
# Polyant Skills for Claude Code

This directory contains skills for integrating Claude Code with the Polyant knowledge base.

## Available Skills

### polyant-config
Configure the connection to your Polyant node.

**Usage:**
```
Configure Polyant connection
```

### polyant-search
Search the knowledge base when encountering problems.

**Triggers:**
- Compilation errors
- Runtime errors
- Performance issues
- Architecture questions

**Usage:**
```
Search Polyant for: "Go error handling"
```

### polyant-save
Save knowledge and experience to the knowledge base.

**Triggers:**
- Task completion
- Error resolution
- Best practice discovery

**Usage:**
```
Save to Polyant:
Title: "My Knowledge"
Category: "computer-science/programming-languages/go"
Content: "..."
```

### polyant-learn
Learn from the knowledge base to improve skills.

**Usage:**
```
Learn from Polyant: "Go concurrency patterns"
```

## Installation

### Automatic Installation

```bash
./scripts/install-skills.sh
```

### Manual Installation

1. Copy skill files to `~/.claude/skills/`:
   ```bash
   cp skills/*.md ~/.claude/skills/
   ```

2. Configure environment variables:
   ```bash
   export POLYANT_API_URL="https://your-node.example.com:8080"
   export POLYANT_API_KEY="sk_live_your_api_key"
   ```

3. Verify connection:
   ```bash
   pactl status
   ```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `POLYANT_API_URL` | API endpoint URL | `http://localhost:8080` |
| `POLYANT_API_KEY` | API authentication key | (required) |
| `POLYANT_NODE_ID` | Node identifier | (auto-generated) |
| `POLYANT_DEFAULT_CATEGORY` | Default category | `general-knowledge` |
| `POLYANT_AUTO_SAVE` | Auto-save completed tasks | `true` |
| `POLYANT_AUTO_SEARCH` | Auto-search on errors | `true` |

### Configuration File

Create `~/.polyant/config.json`:

```json
{
  "api_url": "https://your-node.example.com:8080",
  "api_key": "sk_live_your_api_key",
  "node_id": "your-node-id",
  "default_category": "computer-science/programming-languages/go",
  "auto_save": true,
  "auto_search": true
}
```

## Usage Examples

### Example 1: Error Resolution

```
User: I'm getting "undefined: fmt.Println"

Skill: Searching Polyant...

Found: Go Common Errors: Missing fmt Import
Solution: Add `import "fmt"` to your file
```

### Example 2: Saving Knowledge

```
User: I solved the goroutine leak by using context cancellation.

Skill: Would you like to save this to Polyant?

Title: "Goroutine Leak Prevention with Context"
Category: computer-science/programming-languages/go
Tags: go, goroutine, context, leak-prevention

Save? [Y/n]
```

### Example 3: Learning

```
User: Learn about Go interfaces.

Skill: Found 8 entries about Go interfaces:

1. Go Interfaces Basics (Rating: 4.9/5)
2. Go Interface Design Patterns (Rating: 4.7/5)
3. Go Empty Interface Usage (Rating: 4.5/5)

Which would you like to learn about?
```

## Troubleshooting

### Connection Issues

**Problem:** Cannot connect to Polyant node

**Solutions:**
1. Check if node is running: `pactl status`
2. Verify API URL: `echo $POLYANT_API_URL`
3. Check firewall settings
4. Increase timeout: `export POLYANT_TIMEOUT=30`

### Authentication Issues

**Problem:** Authentication failed

**Solutions:**
1. Verify API key: `echo $POLYANT_API_KEY`
2. Check key permissions
3. Generate new key if expired

### Search Issues

**Problem:** No search results

**Solutions:**
1. Try different keywords
2. Check spelling
3. Use more specific terms
4. Verify node has data

## Contributing

To add new skills:

1. Create a new `.md` file in `skills/`
2. Add triggers and usage examples
3. Update this README
4. Submit a pull request

## Support

- Documentation: [docs/api.md](../api.md)
- Issues: [GitHub Issues](https://github.com/daifei0527/polyant/issues)
- Discussions: [GitHub Discussions](https://github.com/daifei0527/polyant/discussions)
```

- [ ] **Step 2: 提交使用文档**

```bash
git add docs/skills/README.md
git commit -m "docs(skills): add usage documentation"
```

---

## 安全注意事项

1. **API Key 安全：** 不要在代码中硬编码 API Key
2. **传输安全：** 使用 HTTPS 连接
3. **权限控制：** 使用最小权限原则
4. **数据隐私：** 注意保存内容的隐私性

## 未来扩展

1. **更多触发器：** 支持更多自动触发场景
2. **智能推荐：** 基于上下文推荐相关知识
3. **协作功能：** 多智能体协作编辑知识
4. **知识图谱：** 可视化知识关系
