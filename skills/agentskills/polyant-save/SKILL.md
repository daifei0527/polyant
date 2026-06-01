---
name: polyant-save
description: Save knowledge and experience to Polyant knowledge base. Trigger after completing tasks, solving errors, or discovering best practices.
---

# Polyant Save

Save knowledge, solutions, and best practices to the Polyant distributed knowledge base.

## When to Use

- **Task completion:** After successfully completing a task
- **Error resolution:** After solving a technical problem
- **Best practices:** When discovering recommended approaches

## Entry Format

```markdown
## Problem
[Describe the problem]

## Solution
[Provide the solution]

## Example
[Include code examples]
```

## How to Use

```bash
pactl entry create \
  --title "Title" \
  --content "Content" \
  --category "category/path" \
  --tags "tag1,tag2"
```

## Configuration

Requires Polyant connection with authentication (Lv1+).
