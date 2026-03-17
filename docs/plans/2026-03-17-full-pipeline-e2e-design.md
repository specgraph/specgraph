# Full Pipeline E2E Test Suite — Design

**Goal:** Prove the entire SpecGraph pipeline works end-to-end with real Memgraph, from spec creation through agent execution and completion, across three test tiers of increasing realism.

**Architecture:** Three tiers — protocol-level (ConnectRPC against in-process server), CLI-level (binary commands against real server), and agent-level (Claude Code plugin driving the funnel). Each tier has focused test suites: happy-path pipeline, multi-project isolation, constitution violations, and lifecycle transitions.

---

## Test Tiers

| Tier | Build Tag | Infrastructure | CI Gate | Purpose |
|------|-----------|---------------|---------|---------|
| Protocol | `e2e` | testcontainer Memgraph + in-process Go server | Every PR (`task pr-prep`) | Verify ConnectRPC API contracts with real graph DB |
| CLI | `e2e_cli` | testcontainer Memgraph + `specgraph serve` binary | Every PR | Verify CLI commands + server + Memgraph integration |
| Agent | `e2e_agent` | testcontainer Memgraph + `specgraph serve` + `claude` CLI | Nightly / manual | Verify Claude Code plugin skills drive the full funnel |

Tier 1 and 2 require Docker only. Tier 3 additionally requires `ANTHROPIC_API_KEY`.

---

## Tier 1: Protocol Tests (`e2e/api/`)

### Suite 1: `pipeline_test.go` — Full Happy-Path Pipeline

Single `Ordered` Describe block walking the entire lifecycle.

**Slug convention:** Each suite uses a unique prefix to avoid collisions (e.g., `pipeline-`, `iso-`, `const-`, `lc-`). `ClearAll` is called once in `BeforeSuite` only — suites rely on prefix isolation, not clean state.

```text
1. Set constitution (project name, principles, constraints, tech stack)
2. Create spec with intent
3. Spark — seed, signal, scope sniff, unknowns, kill test
4. Shape — scope in/out, approaches, chosen approach, decisions, edges, success criteria
5. Verify: decisions promoted to Decision nodes, DECIDED_IN edges created
6. Specify — interface contract, verify criteria, invariants, touches
7. Decompose — strategy, slices with inter-slice dependencies
8. Verify: child specs created with DEPENDS_ON edges
9. Approve — stage transitions to approved
10. Claim — agent claims spec with lease duration
11. Generate bundle — verify bundle contains constitution subset, spec details, decisions
12. GetPrime — verify prime data includes constitution + spec + claimed state
13. Report progress ("implementing slice-1")
14. Report progress ("implementing slice-2")
15. Report blocker ("blocked on dependency X")
16. Report completion
17. Verify final state:
    - Spec stage = done
    - Claim released (no active claim)
    - Execution events recorded in order (2 progress, 1 blocker, 1 completion)
    - GetExecutionEvents returns all 4 events
```

### Suite 2: `isolation_test.go` — Multi-Project Isolation

```text
1. Create server with two scoped projects: "project-alpha" and "project-beta"
2. In project-alpha: create spec "shared-name" with intent "alpha intent"
3. In project-beta: create spec "shared-name" with intent "beta intent"
4. Verify: GetSpec("shared-name") in project-alpha returns "alpha intent"
5. Verify: GetSpec("shared-name") in project-beta returns "beta intent"
6. Verify: ListSpecs in project-alpha shows only alpha specs
7. Verify: constitution set in alpha is not visible in beta
8. Verify: decisions in alpha are not visible in beta
9. Verify: edges between specs in alpha don't leak into beta's graph queries
```

Implementation: **new code required** — add `projectClientFor(slug string) *http.Client` helper to `helpers_test.go` (the existing `projectClient()` hardcodes `e2eProject = "e2e-test"`). Isolation tests use `projectClientFor("project-alpha")` and `projectClientFor("project-beta")` to create ConnectRPC clients with different project headers. The e2e server already has `ProjectMiddleware` wired.

### Suite 3: `constitution_test.go` — Constitution Integration

**Note:** Analytical passes (safety net, violation detection) currently return placeholder data — real LLM-driven passes are deferred to a future slice. This suite tests the constitution storage and retrieval pipeline, not semantic violation detection.

```text
1. Set constitution with name, principles, constraints, tech stack via UpdateConstitution RPC
2. Verify: GetConstitution returns the stored constitution
3. Create spec → spark → shape (with constitution loaded)
4. Verify: shape response includes constitution context
5. Generate bundle for an approved spec
6. Verify: bundle contains the constitution subset (principles, constraints)
7. Verify: constitution is per-project (set in alpha, not visible in beta)
```

### Suite 4: `lifecycle_transitions_test.go` — Post-Approval Lifecycle

```text
1. Create spec → full funnel to approved
2. Amend (back to shape, re_entry_stage: "shape", reason: "scope changed")
   - Verify: stage = shape, version incremented, history records amend
3. Re-shape → re-specify → re-decompose → re-approve
4. Supersede with new spec
   - Verify: old spec stage = superseded, superseded_by set
   - Verify: new spec supersedes set, linked to old
5. Create another spec → approve → abandon (reason: "deprioritized")
   - Verify: stage = abandoned
6. Lint the abandoned spec — verify lint report acknowledges terminal state
7. Drift check on the superseded spec — verify drift report
```

---

## Tier 2: CLI Tests (`e2e/cli/`)

### Suite: `pipeline_test.go` — CLI-Driven Happy Path

Uses `testutil.CLIRunner` to drive the `specgraph` binary:

```text
1. Build binary (shared BeforeSuite)
2. Start Memgraph testcontainer + in-process server (reuses testutil.StartServer)
3. specgraph create pipeline-cli-test --title="CLI pipeline test" --priority=p2
4. specgraph spark pipeline-cli-test --json-file=testdata/spark-output.json
5. specgraph shape pipeline-cli-test --json-file=testdata/shape-output.json
6. specgraph specify pipeline-cli-test --json-file=testdata/specify-output.json
7. specgraph decompose pipeline-cli-test --json-file=testdata/decompose-output.json
8. specgraph approve pipeline-cli-test
9. specgraph claim pipeline-cli-test --agent=cli-test-agent --duration=1m
10. specgraph bundle pipeline-cli-test → verify YAML output
11. specgraph progress pipeline-cli-test --agent=cli-test-agent --message="working"
12. specgraph show pipeline-cli-test → verify stage=done
13. specgraph progress pipeline-cli-test → list execution events (read mode)
14. Verify exit codes, stdout/stderr for each command
```

**Test fixtures**: `e2e/cli/testdata/` contains pre-built JSON files for each stage output. These are static — they encode the structured output that an agent would produce conversationally.

### Infrastructure

The CLI tier needs its own suite setup:

```go
// e2e/cli/cli_suite_test.go
// BeforeSuite:
//   1. BuildBinary()
//   2. StartMemgraph testcontainer
//   3. Write temp global config pointing at testcontainer
//   4. Start `specgraph serve` as subprocess using the temp config
//   5. Wait for health check
// AfterSuite:
//   1. Kill serve subprocess
//   2. Cleanup testcontainer
```

The `specgraph` binary reads `~/.config/specgraph/config.yaml` by default. For tests, set `SPECGRAPH_CONFIG` env var or use `--config` flag pointing at a temp config.

---

## Tier 3: Agent Tests (`e2e/agent/`)

### Suite: `pipeline_test.go` — Claude-Driven Full Funnel

```text
1. Start Memgraph + specgraph serve (same as CLI tier)
2. Install specgraph plugin to a temp Claude Code config
3. For each stage, invoke claude CLI:
   claude --print --allowedTools "Bash,Read,Write" \
     -p "You have the specgraph plugin installed. \
         Use /specgraph-spark to create a spec called agent-test-feature \
         with the idea: automated testing framework for specgraph"
4. After each claude invocation, verify state via specgraph CLI:
   specgraph show agent-test-feature → assert correct stage
5. Continue through shape, specify, decompose, approve
6. Claim + bundle via CLI (agent doesn't do this directly)
7. Verify full state
```

**Constraints:**

- Requires `ANTHROPIC_API_KEY` env var — skip if not set
- 5-minute timeout per agent invocation
- Agent output is non-deterministic — verify state via CLI, not agent stdout
- `//go:build e2e_agent` tag — never runs in normal CI

---

## Infrastructure Changes

### New Helpers

1. **`testutil.StartServer` already supports multi-project** — No change needed. The server uses `ProjectMiddleware` to extract the project from each request's `X-Specgraph-Project` header. The `WithProject("e2e-test")` in `memgraph.New` only sets the bootstrap project for index creation — it doesn't restrict which projects the server can handle. For isolation tests, callers use `projectClientFor("project-alpha")` and `projectClientFor("project-beta")` — the server handles both via `Store.Scoped()`.

2. **`newExecutionClient()`** — **New code required.** Add to `e2e/api/helpers_test.go`. Follows same `projectClient()` pattern as other client helpers.

3. **`newLifecycleClient()`** — Already exists.

**Note:** No `StartServeProcess` or `WriteTestConfig` helpers needed — the existing `testutil.StartServer` starts an in-process HTTP server and writes a temp config (`ServerInfo.ConfigPath`). The CLI runner uses `--config` to connect to it. No subprocess needed for CLI tier.

**Note on CLI tier config:** The `specgraph` binary's default config path is `.specgraph/config.yaml` (relative to CWD), NOT `~/.config/specgraph/config.yaml`. The CLI tier must always use `--config` (via `CLIRunner.ConfigPath`) to point at the test server.

**Note on `specgraph init` in CLI tier:** `init` calls `runUp()` which attempts Docker compose. In test, Memgraph is already running via testcontainer. The `runUp` failure is non-fatal (prints warning to stderr). The CLI tier should skip `specgraph init` and instead use `specgraph create` directly with `--config` pointing at the test server. The `.specgraph.yaml` project file is not needed when `--config` provides a working server connection.

### Test Data Files

```text
e2e/cli/testdata/
  spark-output.json      # SparkOutput proto JSON
  shape-output.json      # ShapeOutput proto JSON
  specify-output.json    # SpecifyOutput proto JSON
  decompose-output.json  # DecomposeOutput proto JSON
```

### CI Integration

```yaml
# .github/workflows/ci.yaml additions:
jobs:
  e2e:
    steps:
      - run: task test:e2e:api     # existing — protocol tests
      - run: task test:e2e:cli     # new — CLI tests

  e2e-agent:   # new job, manual trigger or nightly
    if: github.event_name == 'schedule' || contains(github.event.head_commit.message, '[e2e-agent]')
    steps:
      - run: task test:e2e:agent
    env:
      ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

### Taskfile Additions

```yaml
test:e2e:cli:
  cmd: go test -tags e2e_cli ./e2e/cli/ -v -count=1 -timeout=300s

test:e2e:agent:
  cmd: go test -tags e2e_agent ./e2e/agent/ -v -count=1 -timeout=600s
```

---

## Priority Order

1. **`pipeline_test.go` (protocol)** — Most value, fastest, proves the full API pipeline
2. **`isolation_test.go` (protocol)** — Validates the Slice 7 BELONGS_TO architecture
3. **`pipeline_test.go` (CLI)** — Proves the binary works end-to-end
4. **`constitution_test.go` (protocol)** — Constitution integration
5. **`lifecycle_transitions_test.go` (protocol)** — Lifecycle flows
6. **`pipeline_test.go` (agent)** — Real agent test (depends on claude CLI)

---

## Open Questions (Resolved)

| Question | Decision |
|----------|----------|
| Test tiers? | Three: protocol, CLI, agent |
| Suite organization? | Focused suites per concern |
| Agent invocation? | `claude --print` with plugin, verify via CLI |
| CI gating? | Protocol + CLI on every PR; agent nightly/manual |
| Multi-project in tests? | Use `projectClient()` with different slugs + `ProjectMiddleware` |
