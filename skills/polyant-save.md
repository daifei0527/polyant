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
