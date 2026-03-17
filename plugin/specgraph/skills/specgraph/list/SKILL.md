---
name: specgraph-list
description: >
  List specs with optional filtering. Use when the user asks "what specs
  do we have?", "show me all specs", "what's in progress?", or "list".
---

# SpecGraph List

```bash
specgraph list --format=table
```

Present the output as a formatted table. If filtering is requested:

- By stage: `specgraph list --stage=<stage>`
- By priority: `specgraph list --priority=<priority>`
