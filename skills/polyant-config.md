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
