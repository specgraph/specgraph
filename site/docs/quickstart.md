# Quick Start

Author your first spec in five minutes. This guide walks through installing
SpecGraph, starting the server, and running a "health check endpoint" spec
through the full authoring funnel.

---

## Prerequisites

- **Go 1.23+** (build from source) or **Homebrew** (when published)
- **Docker** (Memgraph container)
- **Claude Code** (recommended for AI-assisted authoring)

---

## Install

> **Action:** Install the `specgraph` binary.

<!-- x-release-please-start-version -->
```bash
# Homebrew (after first release)
brew install specgraph/tap/specgraph

# Or build from source
go install github.com/specgraph/specgraph/cmd/specgraph@v0.1.0
```
<!-- x-release-please-end -->

<details><summary>Build from source (development)</summary>

```bash
git clone https://github.com/specgraph/specgraph.git
cd specgraph
task build
# binary is at ./specgraph
```

</details>

---

## Start the Server

> **Action:** Start Memgraph and the SpecGraph server.

```bash
specgraph up
```

This starts the Memgraph Docker container and installs a background service
(launchd on macOS, systemd on Linux). The server listens at
`http://localhost:7600` by default.

<details><summary>Manual mode</summary>

If you prefer to manage the process yourself, set `server.mode = "manual"` in
`~/.config/specgraph/config.yaml` and run `specgraph serve` in a separate
terminal.

</details>

---

## Initialize a Project

> **Action:** Create a `.specgraph.yaml` in your project root.

```bash
cd /path/to/your/project
specgraph init
```

The slug is derived from your git remote or directory name. Pass an explicit
slug with `specgraph init my-project`.

---

## Install the Claude Code Plugin

> **Action:** Link the plugin so Claude Code discovers SpecGraph skills.

```bash
mkdir -p .claude/plugins
ln -s /path/to/specgraph/plugin/specgraph .claude/plugins/specgraph
```

If you cloned the SpecGraph repo, the plugin is auto-discovered inside that
repo. For other projects, use the symlink above.

---

## Author Your First Spec

Walk a "health check endpoint" idea through all five authoring stages.
Each section shows the **Claude Code skill trigger** (primary path) with the
CLI equivalent in a collapsible block.

### 1. Spark

> **Trigger:** *"I have an idea for a health check endpoint"*

The Spark skill captures the raw idea:

| Field | Value |
|-------|-------|
| seed | Health check endpoint for the API |
| signal | Operational need — load balancers and monitors need a reliable liveness probe |
| scope_sniff | tiny |
| kill_test | If we drop HTTP entirely |

<details><summary>CLI equivalent</summary>

```bash
specgraph spark healthz \
  --seed "Health check endpoint for the API" \
  --signal "Operational need" \
  --scope-sniff tiny \
  --kill-test "If we drop HTTP entirely"
```

</details>

### 2. Shape

> **Trigger:** *"Let's design the healthz spec"* or *"scope this out"*

Shape bounds the scope and explores approaches:

| Field | Value |
|-------|-------|
| scope_in | `GET /healthz` returns 200 with JSON body |
| scope_out | Deep dependency checks, Prometheus metrics |
| approaches | (1) Static 200 response (2) Ping database, report status |
| risks | None significant at this scope |

<details><summary>CLI equivalent</summary>

```bash
specgraph shape healthz \
  --scope-in "GET /healthz returns 200 JSON" \
  --scope-out "Deep checks, metrics" \
  --approaches 2
```

</details>

### 3. Specify

> **Trigger:** *"Define the interface for healthz"* or *"specify"*

Specify locks down the contract:

| Field | Value |
|-------|-------|
| interface | `GET /healthz` -> `200 {"status":"ok"}` |
| acceptance criteria | Returns 200 within 50ms; body is valid JSON with `status` key |
| invariants | Must never return 5xx unless the process is shutting down |

<details><summary>CLI equivalent</summary>

```bash
specgraph specify healthz \
  --interface 'GET /healthz -> 200 {"status":"ok"}' \
  --criteria "Returns 200 within 50ms" \
  --invariant "Never returns 5xx unless shutting down"
```

</details>

### 4. Decompose

> **Trigger:** *"Break this down"* or *"decompose"*

For a tiny spec like this, decomposition produces a single slice:

| Field | Value |
|-------|-------|
| strategy | Single slice — spec is small enough to implement atomically |
| slices | `healthz-v1`: implement handler, register route, add test |

<details><summary>CLI equivalent</summary>

```bash
specgraph decompose healthz \
  --strategy "single slice" \
  --slice "healthz-v1: implement handler, register route, add test"
```

</details>

### 5. Approve

> **Trigger:** *"Approve healthz"* or *"looks good, ready to implement"*

The Approve skill runs a checklist and freezes the spec:

| Check | Result |
|-------|--------|
| All fields populated | Pass |
| Acceptance criteria testable | Pass |
| No unresolved risks | Pass |
| **Verdict** | **Approved — frozen for execution** |

<details><summary>CLI equivalent</summary>

```bash
specgraph approve healthz
```

</details>

---

## Check for Drift

After implementation, verify the spec hasn't drifted from its dependencies:

```bash
specgraph drift healthz
# No drift detected.

# Check all specs
specgraph drift
```

If drift is detected, acknowledge it:

```bash
specgraph drift acknowledge healthz --all --note "Updated after API refactor"
```

---

## Next Steps

- **[How It Works](how-it-works.md)** — architecture and data flow
- **[Concepts](concepts/index.md)** — specs, constitution, authoring funnel, decisions
- **[Example Spec](concepts/example-spec.md)** — full annotated spec with all fields
- **[Architecture](architecture.md)** — system design and storage layer
