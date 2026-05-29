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
