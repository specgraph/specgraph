---
name: specgraph-init
description: >
  Initialize a SpecGraph project and start the server. Use when "initialize",
  "bootstrap", "set up specgraph", "start the server", "init", "get started",
  or starting fresh with SpecGraph in a new project.
---

# SpecGraph Init

Bootstrap a new SpecGraph project: initialize config, start the server and
database, verify health, and guide to constitution setup.

---

## Step 1: Prerequisites Check

```bash
docker info >/dev/null 2>&1 && echo "Docker: ready" || echo "Docker: NOT running — start Docker first"
specgraph --version 2>/dev/null || ./specgraph --version 2>/dev/null || echo "specgraph binary not found — run: task build"
```

If Docker isn't running, stop and tell the user. SpecGraph needs Docker for
Memgraph.

If `specgraph` isn't on PATH, check for `./specgraph` in the current
directory. Use whichever works.

## Step 2: Initialize

```bash
specgraph init
```

This does three things:

1. Writes `.specgraph.yaml` with the project slug (derived from git remote or directory name)
2. Starts Docker Compose for Memgraph
3. Starts the SpecGraph server (attempts launchd/systemd service, falls back to `specgraph serve`)

If `init` reports the server is already running, that's fine — skip to Step 3.

If the service install fails (common in sandboxed environments), start manually:

```bash
specgraph serve &
```

## Step 3: Verify

```bash
specgraph health
specgraph list
```

Health should return `Status: ok`. List should return `No specs found.`
(clean slate).

## Step 4: Report and Next Steps

Summarize what's running:

- **Config**: `.specgraph.yaml` with project slug
- **Server**: Running at the configured address
- **Database**: Memgraph via Docker
- **Specs**: Empty — ready for authoring

Then check for a constitution:

```bash
specgraph constitution show
```

If no constitution exists, route to `/specgraph-constitution`:
"No constitution configured yet. The constitution is your project's ground
truth — analytical passes check every spec against it. Let's set one up."

If a constitution exists, offer next steps:
"Ready to go. You can start a new spec with `/specgraph-spark` or see
what's available with `specgraph list`."
