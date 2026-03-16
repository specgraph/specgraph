---
name: specgraph-approve
description: >
  Review and approve a spec for execution. Use when ready to sign off.
  Triggered by "approve", "looks good", "ready to implement", "sign off".
---

# SpecGraph Approve

Review the spec and mark it ready for execution.

## Prerequisites

````bash
specgraph health
specgraph show <slug> --format=json
specgraph deps <slug>
````

## Workflow

### 1. Review Checklist

- Scope bounded? (from Shape)
- Interface defined? (from Specify)
- Work units identified? (from Decompose)
- Dependencies declared?
- Risks acknowledged?

### 2. Approve

````bash
specgraph approve <slug>
````

### 3. Next Steps

- Generate execution bundle → `/specgraph-bundle <slug>`
- Or claim and start working → `specgraph claim <slug>`
