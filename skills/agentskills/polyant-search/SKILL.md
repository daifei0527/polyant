---
name: polyant-search
description: Search Polyant knowledge base for solutions, best practices, and technical documentation. Trigger when encountering errors, performance issues, or architecture questions.
---

# Polyant Search

Search the Polyant distributed knowledge base for solutions and best practices.

## When to Use

- **Compilation errors:** When code fails to compile
- **Runtime errors:** When code crashes or produces unexpected results
- **Performance issues:** When code is slow or resource-intensive
- **Architecture questions:** When designing systems or making technical decisions
- **Best practices:** When looking for recommended approaches

## How to Use

```bash
# Basic search
pactl search "<query>"

# Search with category filter
pactl search "<query>" --category "computer-science/programming-languages/go"

# Search with limit
pactl search "<query>" --limit 5
```

## Example

User: I'm getting "undefined: fmt.Println" error

Agent: Let me search the Polyant knowledge base.

```bash
pactl search "undefined fmt.Println" --limit 3
```

## Configuration

Requires Polyant connection. Run `polyant-config` skill first if not configured.

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `POLYANT_API_URL` | API server URL | `http://localhost:8080` |
| `POLYANT_API_KEY` | API key for authentication | (none) |
