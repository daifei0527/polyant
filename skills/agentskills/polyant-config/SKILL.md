---
name: polyant-config
description: Configure Polyant knowledge base connection settings. Trigger on first use or when configuration changes are needed.
---

# Polyant Config

Configure the connection to your Polyant knowledge base node.

## Configuration Methods

### Environment Variables

```bash
export POLYANT_API_URL="http://your-node:8080"
export POLYANT_API_KEY="your-api-key"
```

### Configuration File

Create `~/.polyant/config.json`:

```json
{
  "base_url": "http://your-node:8080",
  "api_key": "your-api-key"
}
```

## Quick Setup

```bash
pactl key generate
pactl user register --name "Your Name"
pactl status
```
