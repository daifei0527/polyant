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
