# Sync & Integration

SpecGraph can push specs to external trackers for visibility and coordination.
Sync is **push-only** — SpecGraph is the source of truth. Changes to issues in
Beads or GitHub are not imported back into SpecGraph.

---

## Beads Sync

`specgraph sync beads` pushes approved specs to Beads as issues. It syncs all
specs that match the given filters — there is no slug argument.

```bash
specgraph sync beads [--dry-run] [--stage <stage>] [--priority <priority>]
```

**Preview before pushing**

```bash
specgraph sync beads --dry-run
```

Prints the specs that would be synced without creating or updating any Beads
issues.

**Filter by stage**

```bash
specgraph sync beads --stage specify
specgraph sync beads --stage decompose
```

Only syncs specs currently at the given authoring stage.

**Filter by priority**

```bash
specgraph sync beads --priority high
specgraph sync beads --priority critical
```

Only syncs specs with the given priority.

**What gets pushed**

Each Beads issue receives:

- **Title** — the spec slug
- **Description** — the spec's intent
- **Stage** — current authoring stage
- **Priority** — spec priority

**Check sync state**

```bash
specgraph sync status --adapter beads
```

---

## GitHub Sync

`specgraph sync github` pushes specs as GitHub Issues. Like Beads sync, it
operates on all matching specs — no slug argument.

```bash
specgraph sync github [--dry-run] [--stage <stage>] [--priority <priority>]
```

**Preview before pushing**

```bash
specgraph sync github --dry-run
```

**Filter by stage or priority**

```bash
specgraph sync github --stage specify --priority high
```

**Issue format**

Each GitHub Issue receives:

- **Title** — the spec slug
- **Body** — the spec's intent
- **Labels** — current stage and priority

**Check sync state**

```bash
specgraph sync status --adapter github
```

---

## Checking Sync Status

`specgraph sync status` shows the sync state for all adapters and all synced
specs.

```bash
specgraph sync status [--adapter beads|github] [--spec <slug>]
```

**Filter by adapter**

```bash
specgraph sync status --adapter beads
specgraph sync status --adapter github
```

**Filter by spec**

```bash
specgraph sync status --spec auth-service-v2
```

Shows the sync state for a single spec across all adapters.

**Combine filters**

```bash
specgraph sync status --adapter github --spec auth-service-v2
