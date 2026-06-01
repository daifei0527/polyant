---
name: polyant-rate
description: Rate and review Polyant knowledge entries. Trigger after using knowledge to provide feedback.
---

# Polyant Rate

Rate and review knowledge entries in the Polyant knowledge base.

## Rating Scale

| Score | Meaning |
|-------|---------|
| 5 | Excellent |
| 4 | Good |
| 3 | Average |
| 2 | Poor |
| 1 | Bad |

## How to Use

```bash
pactl entry rate <entry-id> --score 4 --comment "Worked well"
```

## Configuration

Requires Polyant connection with authentication (Lv1+).
