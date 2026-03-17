---
name: specgraph-decompose
description: >
  Break a spec into implementable work units. Use when ready to split
  work into tasks. Triggered by "break this down", "decompose",
  "split into tasks", "work units".
---

# SpecGraph Decompose

Break the spec into implementable work units.

## Prerequisites

```bash
specgraph health
specgraph show <slug> --format=json
```

## Workflow

### 1. Identify Work Units

- Each unit should be independently implementable and testable
- 1-4 hours of work per unit
- Clear acceptance criteria per unit

### 2. Create Child Specs

```bash
specgraph decompose <slug>
```

### 3. Add Dependencies Between Units

```bash
specgraph edge <child-slug> depends-on <other-child-slug>
```

### 4. Next Steps

- Continue to **Approve** → `/specgraph-approve <slug>`
