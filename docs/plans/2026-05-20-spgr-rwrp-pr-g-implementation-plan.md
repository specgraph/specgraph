# spgr-rwrp PR G — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `specgraph doctor` (four check groups: Binary / Server / Project config / Managed files), the drift-nudge `PersistentPreRun` hook with throttle + GC, the `specgraph health` deprecation as an alias for `doctor server`, plus the dogfood tasks `task plugin:refresh` and `task plugin:check` wired into the `task check` chain.

**Architecture:** A new `cmd/specgraph/doctor*.go` family implements the four-group cobra command on top of existing `InspectAll` infrastructure. `cmd/specgraph/nudge.go` adds a `PersistentPreRun` hook on `rootCmd` that calls `FindProjectRoot` → `LoadProject` → `InspectAll` and emits one stderr nudge line if any managed file is non-Synced, gated by isatty, an allow-list, env/config mutes, and a 24h-throttle file at `xdg.CacheHome()/nudges/<sha256(EvalSymlinks(projectRoot))>-<binaryVersionHash>`. `ProjectConfig` grows `Harnesses []string` and `Nudges struct{Quiet bool}` (lenient decode for everyone except doctor); `FileState` grows `Harness Harness` populated by `InspectAll` after the per-file strategy returns. `xdg.CacheHome()` and the `dev` build tag are already shipped — PR G consumes them.

**Tech Stack:** Go 1.24, `connectrpc.com/connect`, `github.com/mark3labs/mcp-go/client`, `github.com/spf13/cobra`, `golang.org/x/term` (isatty), `gopkg.in/yaml.v3` (with `Decoder.KnownFields(true)` for doctor's strict pass), `internal/xdg.CacheHome()`. Tests use stdlib `testing` plus `testify/require`. E2E uses Ginkgo/Gomega under `//go:build e2e`.

**Spec:** [`2026-05-20-spgr-rwrp-pr-g-doctor-design.md`](2026-05-20-spgr-rwrp-pr-g-doctor-design.md)

**Bead:** spgr-hdki

---

## File Structure

### New files

- `cmd/specgraph/doctor.go` — top-level `doctor` cobra command, `DoctorReport` struct, group dispatch.
- `cmd/specgraph/doctor_binary.go` — Binary group.
- `cmd/specgraph/doctor_config.go` — Project config group + uses `config.ValidateProjectStrict`.
- `cmd/specgraph/doctor_server.go` — Server group (Health RPC + MCP handshake + Skills count) + `doctor server` subcommand.
- `cmd/specgraph/doctor_managed.go` — Managed files group + `--fix` + `--harness` + path-prefix grouping.
- `cmd/specgraph/doctor_render.go` — compact-vs-expanded text rendering + JSON marshal.
- `cmd/specgraph/doctor_test.go` — table-driven state-machine tests + render tests.
- `cmd/specgraph/nudge.go` — `PersistentPreRun` hook + throttle + allow-list + GC.
- `cmd/specgraph/nudge_test.go` — skip-gate tests (allow-list, isatty, env, config, throttle, no-project, GC, fallback).
- `e2e/api/doctor_test.go` — Ginkgo e2e test (build tag `e2e`).

### Modified files

- `internal/config/project.go` — adds `Harnesses []string` and `Nudges struct { Quiet bool }` to `ProjectConfig`; adds `ValidateProjectStrict(path string) error` helper.
- `internal/config/project_test.go` — new field round-trips + strict-decoder rejection.
- `internal/config/managedfiles/types.go` — adds `Harness Harness` field to `FileState`.
- `internal/config/managedfiles/inspect.go` — `InspectAll` populates `state.Harness = mf.Harness` after the per-file strategy returns.
- `internal/config/managedfiles/inspect_test.go` — asserts `FileState.Harness` round-trip from `InspectAll`.
- `cmd/specgraph/init.go` — reads `cfg.Harnesses` (fallback to all three when empty); adds `--check` (exit non-zero if any managed file would change) and `--quiet` (suppress per-file action lines) flags.
- `cmd/specgraph/health.go` — becomes a thin deprecation wrapper that prints `specgraph health: deprecated, use 'specgraph doctor server'` on stderr and dispatches to the `doctor server` runE.
- `cmd/specgraph/root.go` — wires `rootCmd.PersistentPreRunE` to the nudge hook; adds `doctorCmd` to `rootCmd`.
- `Taskfile.yml` — adds `plugin:refresh` and `plugin:check` targets; inserts `- task: plugin:check` into the `check:` cmds sequence between `- task: lint` and `- task: skills:validate`.
- `CLAUDE.md` — adds a "Doctor + drift-nudge" subsection (`specgraph doctor` surface, `--fix` semantics, `task plugin:refresh` + `task plugin:check` use, new `.specgraph.yaml` fields `harnesses:` and `nudges:`).
- `plugin/specgraph/routing-guide.md` — short note: "if something seems off, run `specgraph doctor`".

### Not in scope (already shipped — DO NOT create or modify)

- `internal/xdg/xdg.go` — `CacheHome()` is already at line 54 with cache-dir docs.
- `internal/config/managedfiles/source_release.go` / `source_dev.go` — the `dev` build tag is already wired with `SPECGRAPH_DEV_SOURCE_ROOT`.
- `internal/mcp/skills/embedded.go` — already reads from real files inside the package via `//go:embed`; no dev-tag work needed.

---

## Task 1: ProjectConfig.Harnesses + Nudges.Quiet; init.go falls back to cfg.Harnesses

**Files:**

- Modify: `internal/config/project.go`
- Modify: `internal/config/project_test.go`
- Modify: `cmd/specgraph/init.go:117`

- [ ] **Step 1: Write failing tests for the new fields**

Append to `internal/config/project_test.go`:

```go
func TestProjectConfig_DecodesNewFields(t *testing.T) {
	dir := t.TempDir()
	yaml := `project: my-spec
server: https://example.com
harnesses:
  - claude
  - cursor
nudges:
  quiet: true
`
	if err := os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if got := cfg.Slug; got != "my-spec" {
		t.Errorf("Slug = %q, want my-spec", got)
	}
	if got := cfg.Server; got != "https://example.com" {
		t.Errorf("Server = %q", got)
	}
	if !reflect.DeepEqual(cfg.Harnesses, []string{"claude", "cursor"}) {
		t.Errorf("Harnesses = %v, want [claude cursor]", cfg.Harnesses)
	}
	if !cfg.Nudges.Quiet {
		t.Errorf("Nudges.Quiet = false, want true")
	}
}

func TestProjectConfig_EmptyHarnessesAcceptedAsLegacy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte("project: legacy\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(cfg.Harnesses) != 0 {
		t.Errorf("Harnesses = %v, want empty", cfg.Harnesses)
	}
	if cfg.Nudges.Quiet {
		t.Errorf("Nudges.Quiet = true, want false (zero value)")
	}
}
```

If `project_test.go` doesn't import `reflect` yet, add it to the import block.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-pr-g
go test ./internal/config/ -run 'TestProjectConfig_DecodesNewFields|TestProjectConfig_EmptyHarnessesAcceptedAsLegacy' -v
```

Expected: FAIL with "cfg.Harnesses undefined" or similar.

- [ ] **Step 3: Extend ProjectConfig**

In `internal/config/project.go`, find the `ProjectConfig` struct (around L22-25):

```go
type ProjectConfig struct {
	Slug   string `yaml:"project,omitempty"`
	Server string `yaml:"server,omitempty"`
}
```

Replace with:

```go
// ProjectConfig is the per-repo .specgraph.yaml.
type ProjectConfig struct {
	Slug      string   `yaml:"project,omitempty"`
	Server    string   `yaml:"server,omitempty"`
	Harnesses []string `yaml:"harnesses,omitempty"`
	Nudges    Nudges   `yaml:"nudges,omitempty"`
}

// Nudges configures the drift-nudge that fires on every CLI invocation.
// Quiet suppresses the nudge at the project level (the SPECGRAPH_DRIFT_NUDGE
// environment variable does the same at the user level).
type Nudges struct {
	Quiet bool `yaml:"quiet,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/ -run 'TestProjectConfig_DecodesNewFields|TestProjectConfig_EmptyHarnessesAcceptedAsLegacy' -v
```

Expected: PASS.

- [ ] **Step 5: Wire cfg.Harnesses into init.go**

In `cmd/specgraph/init.go`, find the "Hard-coded for PR B" block (around L117):

```go
	// Hard-coded for PR B; .specgraph.yaml-driven harnesses: list lands later.
	harnesses := []managedfiles.Harness{
		managedfiles.HarnessClaude,
		managedfiles.HarnessCursor,
		managedfiles.HarnessOpenCode,
	}
```

Replace with:

```go
	// Read harnesses from .specgraph.yaml when present; fall back to all
	// three when the list is empty (legacy configs and no-config case).
	harnesses := harnessSliceFromConfig(pc.Harnesses)
```

Add the helper function at the bottom of `init.go`:

```go
// harnessSliceFromConfig maps strings from cfg.Harnesses to Harness enum
// values. Unknown names are silently dropped (doctor's Project config
// group surfaces them as drift). Empty input returns all three harnesses
// — the legacy default before this commit.
func harnessSliceFromConfig(names []string) []managedfiles.Harness {
	if len(names) == 0 {
		return []managedfiles.Harness{
			managedfiles.HarnessClaude,
			managedfiles.HarnessCursor,
			managedfiles.HarnessOpenCode,
		}
	}
	var out []managedfiles.Harness
	for _, n := range names {
		switch n {
		case "claude":
			out = append(out, managedfiles.HarnessClaude)
		case "cursor":
			out = append(out, managedfiles.HarnessCursor)
		case "opencode":
			out = append(out, managedfiles.HarnessOpenCode)
		}
	}
	return out
}
```

- [ ] **Step 6: Run task check**

```bash
task check
```

Expected: PASS.

- [ ] **Step 7: Commit + start next-task**

```bash
jj --no-pager describe -m "feat(config): ProjectConfig.Harnesses + Nudges.Quiet; init.go falls back to cfg.Harnesses

Extends ProjectConfig with two new fields needed by PR G:

- Harnesses []string — the per-project harness allow-list (empty
  means all three, matching the legacy hard-coded slice).
- Nudges struct{ Quiet bool } — project-level mute for the
  drift-nudge that lands in commit 8.

YAML decode stays lenient (yaml.Unmarshal as-is); the strict pass
that detects unknown top-level keys lives in commit 4's doctor
Project config group, not in LoadProject itself. Existing
.specgraph.yaml files keep working unchanged.

init.go:117's hard-coded harness slice now falls back from
cfg.Harnesses via a new harnessSliceFromConfig helper. Unknown
harness names are silently dropped; doctor surfaces them later.

Per design §Project config (commit 1).

Bead: spgr-hdki

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new -m "(next task)"
```

---

## Task 1a: FileState.Harness populated by InspectAll after strategy dispatch

**Files:**

- Modify: `internal/config/managedfiles/types.go`
- Modify: `internal/config/managedfiles/inspect.go`
- Modify: `internal/config/managedfiles/inspect_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/config/managedfiles/inspect_test.go`:

```go
func TestInspectAll_PopulatesFileStateHarness(t *testing.T) {
	dir := t.TempDir()
	params := ProjectParams{Slug: "test", ServerURL: "https://example.com/mcp/"}
	states, err := InspectAll(dir, []Harness{HarnessClaude}, params)
	if err != nil {
		t.Fatalf("InspectAll: %v", err)
	}
	if len(states) == 0 {
		t.Fatal("expected at least one FileState for HarnessClaude")
	}
	for _, s := range states {
		if s.Harness != HarnessClaude {
			t.Errorf("%s: Harness = %v, want HarnessClaude", s.Path, s.Harness)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/config/managedfiles/ -run TestInspectAll_PopulatesFileStateHarness -v
```

Expected: FAIL with `s.Harness undefined`.

- [ ] **Step 3: Add Harness field to FileState**

In `internal/config/managedfiles/types.go`, find:

```go
type FileState struct {
	Path         string
	Strategy     Strategy
	State        State
	DiskHash     string // sha256 of current disk content (empty if Missing)
	SentinelHash string // hash recorded in disk sentinel (empty if no sentinel)
	EmbeddedHash string // sha256 of canonical source content
	Detail       string // human-readable explanation, used in doctor output
}
```

Replace with:

```go
type FileState struct {
	Path         string
	Strategy     Strategy
	Harness      Harness // which harness owns this entry; populated by InspectAll
	State        State
	DiskHash     string // sha256 of current disk content (empty if Missing)
	SentinelHash string // hash recorded in disk sentinel (empty if no sentinel)
	EmbeddedHash string // sha256 of canonical source content
	Detail       string // human-readable explanation, used in doctor output
}
```

- [ ] **Step 4: Populate Harness in InspectAll**

In `internal/config/managedfiles/inspect.go`, find the `InspectAll` for-loop (around L43-54):

```go
	for _, mf := range mfs {
		state, err := Inspect(cwd, mf, params)
		if err != nil {
			out = append(out, FileState{
				Path:     mf.Path,
				Strategy: mf.Strategy,
				State:    StateDrifted,
				Detail:   fmt.Sprintf("inspect error: %v", err),
			})
			continue
		}
		out = append(out, state)
	}
```

Replace with:

```go
	for _, mf := range mfs {
		state, err := Inspect(cwd, mf, params)
		if err != nil {
			out = append(out, FileState{
				Path:     mf.Path,
				Strategy: mf.Strategy,
				Harness:  mf.Harness,
				State:    StateDrifted,
				Detail:   fmt.Sprintf("inspect error: %v", err),
			})
			continue
		}
		// Strategy literals don't know which harness owns the manifest
		// entry; overwrite here so callers (doctor's --harness filter,
		// JSON output, etc.) see the attribution. Per design §Managed files.
		state.Harness = mf.Harness
		out = append(out, state)
	}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/config/managedfiles/ -v
```

Expected: PASS. The `TestInspectAll_PopulatesFileStateHarness` test passes; all other existing tests in the package still pass.

- [ ] **Step 6: task check**

```bash
task check
```

Expected: PASS.

- [ ] **Step 7: Commit + start next-task**

```bash
jj --no-pager describe -m "feat(managedfiles): FileState.Harness populated by InspectAll after strategy dispatch

Adds Harness Harness to FileState so doctor's --harness filter and
JSON output can attribute each row. InspectAll writes
state.Harness = mf.Harness after the strategy returns; the strategy
implementations (jsonkeymerge.go, markdownblock.go, wholefile.go)
construct FileState literals that don't know which harness owns the
entry, but that's fine — the post-dispatch overwrite covers all
cases without touching any strategy code.

Per design §Managed files (commit 1a).

Bead: spgr-hdki

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new -m "(next task)"
```

---

## Task 2: init --check and --quiet flags

**Files:**

- Modify: `cmd/specgraph/init.go`
- Modify: `cmd/specgraph/init_test.go` (or create if absent)

- [ ] **Step 1: Write failing tests**

Add to `cmd/specgraph/init_test.go` (or create the file):

```go
func TestInit_CheckFlag_ExitsZeroOnSyncedProject(t *testing.T) {
	dir := t.TempDir()
	// First, run init normally to bring the project up to Synced.
	if err := runInitForTest(dir, false, false); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	// Then run with --check; should exit 0 (no diff).
	err := runInitForTest(dir, true /*check*/, true /*quiet*/)
	if err != nil {
		t.Errorf("--check on Synced project returned %v, want nil", err)
	}
}

func TestInit_CheckFlag_ExitsNonZeroOnStaleProject(t *testing.T) {
	dir := t.TempDir()
	if err := runInitForTest(dir, false, false); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	// Corrupt one managed file so it's no longer Synced.
	corruptOneManagedFile(t, dir)
	err := runInitForTest(dir, true, true)
	if err == nil {
		t.Error("--check on stale project returned nil, want non-nil")
	}
}

// runInitForTest invokes the init runE directly with flags set.
// Implementation depends on how init.go exposes its run logic for testing.
func runInitForTest(dir string, check, quiet bool) error {
	// ... see init.go for the existing testing pattern ...
}
```

(Note: the exact form of `runInitForTest` depends on how `init.go`'s cobra command exposes its `RunE` for testing. Read `init_test.go` if it exists, or use a `--check`-via-`exec` approach. The point is to assert the exit-code behaviour.)

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./cmd/specgraph/ -run 'TestInit_CheckFlag' -v
```

Expected: FAIL (flag not registered, function not present).

- [ ] **Step 3: Add the flags + behaviour**

In `cmd/specgraph/init.go`, find the `init()` function where the cobra flags are registered and add `--check` and `--quiet`:

```go
func init() {
	initCmd.Flags().Bool("yes", false, "Skip confirmation prompts")
	initCmd.Flags().Bool("check", false, "Exit non-zero if any managed file would be modified (no writes)")
	initCmd.Flags().Bool("quiet", false, "Suppress per-file action lines")
	rootCmd.AddCommand(initCmd)
}
```

In the runE function, read the flags and branch:

```go
	check, _ := cmd.Flags().GetBool("check")
	quiet, _ := cmd.Flags().GetBool("quiet")

	// ... existing config loading + harness resolution ...

	if check {
		states, err := managedfiles.InspectAll(cwd, harnesses, params)
		if err != nil {
			return fmt.Errorf("inspect for --check: %w", err)
		}
		nonSynced := 0
		for _, s := range states {
			if s.State != managedfiles.StateSynced {
				nonSynced++
				if !quiet {
					fmt.Printf("%s: %s\n", s.Path, managedfiles.StateName(s.State))
				}
			}
		}
		if nonSynced > 0 {
			return fmt.Errorf("%d managed file(s) not in sync", nonSynced)
		}
		if !quiet {
			fmt.Printf("init --check: all %d managed file(s) synced\n", len(states))
		}
		return nil
	}

	// ... existing Sync path ...
	// Wrap the per-file output emit:
	if !quiet {
		// existing fmt.Println(line) etc. stays here
	}
```

(If `StateName(State) string` doesn't exist in the managedfiles package yet, add a tiny helper next to `ActionName` for symmetry. It's a 5-line addition.)

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./cmd/specgraph/ -run 'TestInit_CheckFlag' -v
```

Expected: PASS.

- [ ] **Step 5: task check**

```bash
task check
```

Expected: PASS.

- [ ] **Step 6: Commit + start next-task**

```bash
jj --no-pager describe -m "feat(init): add --check (exit non-zero if any managed file would change) + --quiet

--check: performs InspectAll without writing; exits 0 if every
managed file is Synced, exits non-zero (with optional per-file
output unless --quiet) if any is Missing, Stale, or Drifted.
Required by task plugin:check in commit 8.

--quiet: suppresses per-file action lines for both --check and
the normal sync path; used by task plugin:refresh so the rebuild
+ init output stays terse.

Per design §--check flag mechanics.

Bead: spgr-hdki

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new -m "(next task)"
```

---

## Task 3: doctor cobra command + DoctorReport + Binary group + render dispatch

**Files:**

- Create: `cmd/specgraph/doctor.go`
- Create: `cmd/specgraph/doctor_binary.go`
- Create: `cmd/specgraph/doctor_render.go`
- Create: `cmd/specgraph/doctor_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/specgraph/doctor_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDoctorReport_BinaryGroupAllHealthy(t *testing.T) {
	rep := DoctorReport{}
	rep.Binary = runBinaryGroup() // builds the group; no external deps
	if !rep.Binary.OK {
		t.Errorf("Binary group not OK: %+v", rep.Binary)
	}
}

func TestDoctorReport_Render_CompactWhenAllGreen(t *testing.T) {
	rep := DoctorReport{
		Binary: BinaryReport{OK: true, Version: "0.7.3", BuiltAt: "2026-05-20T14:30:00Z", Commit: "abc1234"},
	}
	var buf bytes.Buffer
	renderText(&buf, rep, false /*verbose*/)
	out := buf.String()
	if !strings.Contains(out, "Binary:") || !strings.Contains(out, "0.7.3") {
		t.Errorf("compact render missing binary line: %s", out)
	}
}

func TestDoctorReport_Render_JSONStableSchema(t *testing.T) {
	rep := DoctorReport{
		ExitCode: 0,
		Binary:   BinaryReport{OK: true, Version: "0.7.3", BuiltAt: "2026-05-20T14:30:00Z", Commit: "abc1234"},
	}
	var buf bytes.Buffer
	renderJSON(&buf, rep)
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if got["exitCode"].(float64) != 0 {
		t.Errorf("exitCode = %v, want 0", got["exitCode"])
	}
	if groups, ok := got["groups"].(map[string]any); !ok {
		t.Errorf("missing groups object: %s", buf.String())
	} else {
		if _, ok := groups["binary"]; !ok {
			t.Errorf("missing groups.binary")
		}
	}
}

func TestDoctorReport_ExitZeroForcesZero(t *testing.T) {
	rep := DoctorReport{ExitCode: 1}
	if code := finalExitCode(rep, true /*exitZero*/); code != 0 {
		t.Errorf("--exit-zero with unhealthy state: exit = %d, want 0", code)
	}
	if code := finalExitCode(rep, false); code != 1 {
		t.Errorf("normal mode with unhealthy state: exit = %d, want 1", code)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./cmd/specgraph/ -run TestDoctor -v
```

Expected: FAIL (undefined `DoctorReport`, `runBinaryGroup`, `renderText`, etc.).

- [ ] **Step 3: Create `doctor.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// DoctorReport is the canonical structure all output modes (text +
// JSON) emit. Schema is stable across versions; new fields may be
// added but existing ones don't change shape.
type DoctorReport struct {
	ExitCode int            `json:"exitCode"`
	Binary   BinaryReport   `json:"binary"`
	Server   ServerReport   `json:"server"` // populated in commit 5
	Project  ProjectReport  `json:"project"` // populated in commit 4
	Managed  ManagedReport  `json:"managed"` // populated in commit 6
}

// runDoctor is doctorCmd's RunE entry point. It builds the report,
// renders it (text or JSON), and returns the final exit code as an
// error so cobra propagates it.
func runDoctor(cmd *cobra.Command, _ []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	verbose, _ := cmd.Flags().GetBool("verbose")
	exitZero, _ := cmd.Flags().GetBool("exit-zero")

	rep := DoctorReport{
		Binary: runBinaryGroup(),
		// Server, Project, Managed wired in later commits.
	}
	rep.ExitCode = computeExitCode(rep)

	if jsonOut {
		renderJSON(os.Stdout, rep)
	} else {
		renderText(os.Stdout, rep, verbose)
	}
	final := finalExitCode(rep, exitZero)
	if final == 0 {
		return nil
	}
	// Cobra exits 0 unless RunE returns non-nil; use SilenceUsage and
	// a sentinel error to propagate the code without printing the
	// usage banner.
	cmd.SilenceUsage = true
	return fmt.Errorf("doctor: exit %d", final)
}

// computeExitCode picks among 0 (clean), 1 (any group unhealthy), 2
// (infrastructure failure — reserved for the Server group's dial
// errors etc.; filled in by commit 5).
func computeExitCode(rep DoctorReport) int {
	if !rep.Binary.OK {
		return 1
	}
	return 0
}

// finalExitCode applies the --exit-zero override.
func finalExitCode(rep DoctorReport, exitZero bool) int {
	if exitZero {
		return 0
	}
	return rep.ExitCode
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check SpecGraph integration health (binary, server, project config, managed files)",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().Bool("json", false, "Machine-readable output (full structure, never compacted)")
	doctorCmd.Flags().Bool("fix", false, "Auto-init for Stale/Missing; print guidance for Drifted")
	doctorCmd.Flags().String("harness", "", "Narrow Managed Files group to one harness (claude | cursor | opencode)")
	doctorCmd.Flags().Bool("verbose", false, "Force per-row expansion of all four groups")
	doctorCmd.Flags().Bool("exit-zero", false, "Always exit 0 (advisory-only mode)")
	doctorCmd.Flags().Duration("timeout", 2*time.Second, "Per-RPC timeout for the Server group")
	rootCmd.AddCommand(doctorCmd)
}
```

(Add `"time"` to the imports for the `time.Second` reference in the timeout flag.)

- [ ] **Step 4: Create `doctor_binary.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

// BinaryReport describes the running specgraph binary.
type BinaryReport struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
	BuiltAt string `json:"builtAt"`
	Commit  string `json:"commit"`
}

// runBinaryGroup reports the running binary's identity. Inputs are
// the build-time ldflags-injected values already populated for
// `specgraph health`. OK iff all three are non-empty.
func runBinaryGroup() BinaryReport {
	rep := BinaryReport{
		Version: version,   // existing ldflags var
		BuiltAt: buildTime, // existing ldflags var
		Commit:  commit,    // existing ldflags var
	}
	rep.OK = rep.Version != "" && rep.BuiltAt != "" && rep.Commit != ""
	return rep
}
```

If the existing ldflags variables are named differently (e.g., `Version` exported, no `commit`), look in `cmd/specgraph/version.go` or wherever `specgraph version` reads them, and adjust the references. The point is: pick up the three values the project already tracks; don't introduce new ones.

- [ ] **Step 5: Create `doctor_render.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// renderText writes the compact-when-green / expanded-when-problems text
// form of the report. verbose=true forces every group to expand.
func renderText(w io.Writer, rep DoctorReport, verbose bool) {
	if rep.Binary.OK && !verbose {
		fmt.Fprintf(w, "Binary:         OK (v%s, built %s from %s)\n",
			rep.Binary.Version, rep.Binary.BuiltAt, rep.Binary.Commit)
	} else {
		fmt.Fprintf(w, "Binary:         %s\n", binaryStatusText(rep.Binary))
		if verbose || !rep.Binary.OK {
			fmt.Fprintf(w, "  Version:  %s\n", rep.Binary.Version)
			fmt.Fprintf(w, "  Built at: %s\n", rep.Binary.BuiltAt)
			fmt.Fprintf(w, "  Commit:   %s\n", rep.Binary.Commit)
		}
	}
	// Server, Project, Managed group rendering land in commits 4, 5, 6.
}

func binaryStatusText(b BinaryReport) string {
	if b.OK {
		return fmt.Sprintf("OK (v%s)", b.Version)
	}
	return "PROBLEM (one or more identity fields empty)"
}

// renderJSON writes the canonical machine-readable form. Schema stays
// stable across versions; new fields may be added.
func renderJSON(w io.Writer, rep DoctorReport) {
	wrapped := map[string]any{
		"exitCode": rep.ExitCode,
		"groups": map[string]any{
			"binary":  rep.Binary,
			"server":  rep.Server,
			"project": rep.Project,
			"managed": rep.Managed,
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(wrapped)
}
```

Define placeholder structs for the not-yet-implemented groups so `DoctorReport` compiles:

In `doctor.go`, add (until commits 4-6 replace them):

```go
type ServerReport  struct { OK bool `json:"ok"` }
type ProjectReport struct { OK bool `json:"ok"` }
type ManagedReport struct { OK bool `json:"ok"` }
```

- [ ] **Step 6: Run tests**

```bash
go test ./cmd/specgraph/ -run TestDoctor -v
```

Expected: PASS.

- [ ] **Step 7: task check**

```bash
task check
```

Expected: PASS.

- [ ] **Step 8: Commit + start next-task**

```bash
jj --no-pager describe -m "feat(doctor): scaffold cobra command + Binary group + DoctorReport rendering

cmd/specgraph/doctor.go establishes the top-level command, the
DoctorReport struct, and the runDoctor entry point. Five flags:
--json, --fix, --harness, --verbose, --exit-zero, plus --timeout
for the Server group (lands fully in commit 5).

cmd/specgraph/doctor_binary.go implements the Binary group:
reports the ldflags-injected version + build time + commit;
OK iff all three are non-empty.

cmd/specgraph/doctor_render.go implements compact-when-green text
rendering (Binary line is one row; expanded form lands as each
group wires in) and the JSON encoder. ProjectReport, ServerReport,
ManagedReport are placeholder structs until commits 4-6 fill them
in.

Exit-code policy: computeExitCode returns 0 if all groups
healthy, 1 if any unhealthy, 2 reserved for infrastructure
failures (Server group dial errors etc., wired in commit 5).
finalExitCode applies the --exit-zero override.

Per design §Output format and §Sequencing (commit 3).

Bead: spgr-hdki

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new -m "(next task)"
```

---

## Task 4: Project config group + ValidateProjectStrict helper

**Files:**

- Modify: `internal/config/project.go` (add `ValidateProjectStrict`)
- Modify: `internal/config/project_test.go` (test the strict helper)
- Create: `cmd/specgraph/doctor_config.go`
- Modify: `cmd/specgraph/doctor.go` (wire ProjectReport)
- Modify: `cmd/specgraph/doctor_test.go` (Project group tests)

- [ ] **Step 1: Add ValidateProjectStrict to project.go**

Append to `internal/config/project.go`:

```go
// ValidateProjectStrict re-reads the file at path and decodes it with
// KnownFields(true). Returns nil if the file decodes cleanly; returns
// an error naming the offending field(s) otherwise. Doctor's Project
// config group is the only caller; init/nudge/everywhere else stays
// on LoadProject (lenient).
func ValidateProjectStrict(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read project config: %w", err)
	}
	var pc ProjectConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&pc); err != nil {
		return fmt.Errorf("strict decode: %w", err)
	}
	return nil
}
```

Add `"bytes"` to the imports if not already present.

- [ ] **Step 2: Write the failing strict-decode tests**

Append to `internal/config/project_test.go`:

```go
func TestValidateProjectStrict_AcceptsKnownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".specgraph.yaml")
	yaml := `project: x
server: https://example.com
harnesses: [claude]
nudges:
  quiet: false
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := ValidateProjectStrict(path); err != nil {
		t.Errorf("ValidateProjectStrict on known-keys config: %v", err)
	}
}

func TestValidateProjectStrict_RejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".specgraph.yaml")
	yaml := `project: x
fnord: 42
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := ValidateProjectStrict(path)
	if err == nil {
		t.Fatal("expected strict-decode error on unknown key, got nil")
	}
	if !strings.Contains(err.Error(), "fnord") {
		t.Errorf("error %q does not name the unknown key 'fnord'", err.Error())
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/config/ -run TestValidateProjectStrict -v
```

Expected: PASS.

- [ ] **Step 4: Create `doctor_config.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"

	"github.com/specgraph/specgraph/internal/config"
)

// ProjectReport describes the .specgraph.yaml parse + harness resolution.
type ProjectReport struct {
	OK          bool     `json:"ok"`
	Harnesses   []string `json:"harnesses"`
	StrictError string   `json:"strictError,omitempty"`   // unknown top-level key, etc.
	UnknownNames []string `json:"unknownNames,omitempty"` // names in cfg.Harnesses that didn't resolve
}

// runProjectConfigGroup loads cfg via the lenient LoadProject path
// (matching everything else), then re-validates strictly via the new
// ValidateProjectStrict helper. The combined result is OK only if both
// pass and every Harnesses entry resolves to a known Harness.
func runProjectConfigGroup(cwd string) ProjectReport {
	rep := ProjectReport{OK: true}
	root, err := config.FindProjectRoot(cwd)
	if err != nil {
		if errors.Is(err, config.ErrProjectNotFound) {
			// No project config — treat as OK (the binary works without one).
			return ProjectReport{OK: true}
		}
		rep.OK = false
		rep.StrictError = err.Error()
		return rep
	}
	cfg, err := config.LoadProject(root)
	if err != nil {
		rep.OK = false
		rep.StrictError = err.Error()
		return rep
	}
	rep.Harnesses = cfg.Harnesses

	if err := config.ValidateProjectStrict(root + "/" + ".specgraph.yaml"); err != nil {
		rep.OK = false
		rep.StrictError = err.Error()
	}

	// Resolve every Harnesses entry against the known names.
	for _, name := range cfg.Harnesses {
		switch name {
		case "claude", "cursor", "opencode":
			// resolved
		default:
			rep.OK = false
			rep.UnknownNames = append(rep.UnknownNames, name)
		}
	}
	return rep
}

// projectStatusLine renders the compact form.
func projectStatusLine(rep ProjectReport) string {
	if rep.OK {
		if len(rep.Harnesses) == 0 {
			return "Project config: OK (no project-level customization)"
		}
		return fmt.Sprintf("Project config: OK (%d harnesses enabled)", len(rep.Harnesses))
	}
	return fmt.Sprintf("Project config: PROBLEM (%s)", rep.StrictError)
}
```

(Adjust the path join — use `filepath.Join(root, ".specgraph.yaml")` and import `"path/filepath"` — for cross-platform correctness. The example above uses `+ "/" +` for readability; the implementer should use `filepath.Join`.)

- [ ] **Step 5: Wire ProjectReport into doctor.go + doctor_render.go**

In `doctor.go`, replace the `ProjectReport` placeholder with a removal (now lives in `doctor_config.go`) and update `runDoctor` to populate it:

```go
	rep := DoctorReport{
		Binary:  runBinaryGroup(),
		Project: runProjectConfigGroup(cwd),
		// Server, Managed wired in later commits.
	}
```

Where `cwd, _ := os.Getwd()` is grabbed near the top of `runDoctor`.

Update `computeExitCode` to consider Project:

```go
func computeExitCode(rep DoctorReport) int {
	if !rep.Binary.OK || !rep.Project.OK {
		return 1
	}
	return 0
}
```

Extend `renderText` in `doctor_render.go` to print the Project line after Binary.

- [ ] **Step 6: Tests for the Project group**

Append to `doctor_test.go`:

```go
func TestDoctorReport_ProjectGroup_NoProjectIsOK(t *testing.T) {
	dir := t.TempDir() // empty — no .specgraph.yaml anywhere up the tree
	rep := runProjectConfigGroup(dir)
	if !rep.OK {
		t.Errorf("no-project case: OK = false, want true (%+v)", rep)
	}
}

func TestDoctorReport_ProjectGroup_UnknownKeyReported(t *testing.T) {
	dir := t.TempDir()
	yaml := `project: x
fnord: 42
`
	if err := os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	rep := runProjectConfigGroup(dir)
	if rep.OK {
		t.Errorf("unknown key not flagged: %+v", rep)
	}
	if !strings.Contains(rep.StrictError, "fnord") {
		t.Errorf("StrictError missing 'fnord': %q", rep.StrictError)
	}
}

func TestDoctorReport_ProjectGroup_UnknownHarnessReported(t *testing.T) {
	dir := t.TempDir()
	yaml := `project: x
harnesses: [bogus]
`
	if err := os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	rep := runProjectConfigGroup(dir)
	if rep.OK {
		t.Errorf("unknown harness not flagged: %+v", rep)
	}
	if len(rep.UnknownNames) != 1 || rep.UnknownNames[0] != "bogus" {
		t.Errorf("UnknownNames = %v, want [bogus]", rep.UnknownNames)
	}
}
```

Add imports `"os"`, `"path/filepath"`, `"strings"` to the test file if not already there.

- [ ] **Step 7: task check**

```bash
task check
```

Expected: PASS.

- [ ] **Step 8: Commit + start next-task**

```bash
jj --no-pager describe -m "feat(doctor): Project config group + ValidateProjectStrict helper

Adds config.ValidateProjectStrict(path) for strict (KnownFields)
YAML decode. Used only by doctor's Project config group; init
and the nudge keep using LoadProject (lenient).

cmd/specgraph/doctor_config.go runs the group:
- LoadProject for the lenient path
- ValidateProjectStrict for unknown-key detection
- Per-entry Harnesses resolution against {claude, cursor, opencode}

ProjectReport carries OK, Harnesses, optional StrictError, and
optional UnknownNames. computeExitCode considers Project alongside
Binary. renderText prints the compact line after the Binary line.

Three new tests cover no-project, unknown key, and unknown harness
name paths.

Per design §Project config (commit 4).

Bead: spgr-hdki

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new -m "(next task)"
```

---

## Task 5: Server group + doctor server subcommand + --timeout

**Files:**

- Create: `cmd/specgraph/doctor_server.go`
- Modify: `cmd/specgraph/doctor.go` (wire Server)
- Modify: `cmd/specgraph/doctor_render.go` (Server line)
- Modify: `cmd/specgraph/doctor_test.go` (Server tests)

- [ ] **Step 1: Create `doctor_server.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/mark3labs/mcp-go/client"
	mcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// ServerReport describes the running specgraph server's reachability,
// MCP transport, and the catalog count it exposes.
type ServerReport struct {
	OK           bool   `json:"ok"`
	Reachable    bool   `json:"reachable"`
	Version      string `json:"version"`
	MCPHandshake string `json:"mcpHandshake"`         // "ok" | "failed" | "skipped"
	SkillsCount  int    `json:"skillsCount"`
	Error        string `json:"error,omitempty"`
}

// runServerGroup performs three sub-checks: Connect Health RPC,
// MCP Streamable-HTTP initialize handshake, specgraph_skills_list count.
// Each sub-check honors the shared timeout.
func runServerGroup(timeout time.Duration) ServerReport {
	rep := ServerReport{MCPHandshake: "skipped"}

	// 1. Connect Health RPC.
	cl, err := healthClient()
	if err != nil {
		rep.Error = fmt.Sprintf("client construct: %v", err)
		return rep
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	resp, err := cl.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
	if err != nil {
		rep.Error = fmt.Sprintf("health rpc: %v", err)
		return rep
	}
	rep.Reachable = true
	rep.Version = resp.Msg.Version

	// 2. MCP Streamable-HTTP initialize handshake + 3. Skills count.
	mcpURL := serverMCPURL()
	mcpCli, err := client.NewStreamableHttpClient(mcpURL)
	if err != nil {
		rep.MCPHandshake = "failed"
		rep.Error = fmt.Sprintf("mcp client: %v", err)
		return rep
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
	defer cancel2()
	initResp, err := mcpCli.Initialize(ctx2, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ClientInfo:      mcp.Implementation{Name: "specgraph-doctor", Version: rep.Version},
			ProtocolVersion: mcp.LatestProtocolVersion,
		},
	})
	if err != nil || initResp == nil {
		rep.MCPHandshake = "failed"
		rep.Error = fmt.Sprintf("mcp init: %v", err)
		return rep
	}
	rep.MCPHandshake = "ok"

	// Skills count via specgraph_skills_list.
	ctx3, cancel3 := context.WithTimeout(context.Background(), timeout)
	defer cancel3()
	res, err := mcpCli.CallTool(ctx3, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "specgraph_skills_list"},
	})
	if err != nil || res == nil || len(res.Content) == 0 {
		rep.SkillsCount = -1
		rep.Error = fmt.Sprintf("skills_list: %v", err)
		return rep
	}
	rep.SkillsCount = countSkillsFromJSON(res.Content[0].(*mcp.TextContent).Text)
	rep.OK = rep.Reachable && rep.MCPHandshake == "ok" && rep.SkillsCount >= 0
	return rep
}

// countSkillsFromJSON unmarshals the [{name,summary,uri}] array
// specgraph_skills_list returns and counts entries. -1 on parse failure.
func countSkillsFromJSON(text string) int {
	var rows []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return -1
	}
	return len(rows)
}

func serverStatusLine(rep ServerReport) string {
	if rep.OK {
		return fmt.Sprintf("Server:         OK (reachable v%s · MCP handshake OK · %d skills)",
			rep.Version, rep.SkillsCount)
	}
	if !rep.Reachable {
		return fmt.Sprintf("Server:         UNREACHABLE (%s)", rep.Error)
	}
	return fmt.Sprintf("Server:         PROBLEM (%s)", rep.Error)
}

// doctorServerCmd is the `doctor server` subcommand health uses as its
// alias target (commit 7).
var doctorServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Run only the Server group (used by `specgraph health`)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		timeout, _ := cmd.Flags().GetDuration("timeout")
		rep := runServerGroup(timeout)
		fmt.Println(serverStatusLine(rep))
		if !rep.OK {
			cmd.SilenceUsage = true
			return fmt.Errorf("server unhealthy")
		}
		return nil
	},
}

func init() {
	doctorServerCmd.Flags().Duration("timeout", 2*time.Second, "Per-RPC timeout")
	doctorCmd.AddCommand(doctorServerCmd)
}

// serverMCPURL returns the configured MCP endpoint URL. Reads from
// the same globalCfg.ResolveServer path init uses.
func serverMCPURL() string {
	pc, _ := loadProjectForServer() // helper, see below
	globalCfg, _ := loadGlobalCfg()
	base := globalCfg.ResolveServer(pc.Slug, pc.Server)
	return base + "/mcp" // mcp-go expects the /mcp endpoint
}

// loadProjectForServer is a thin helper that loads the project config
// from cwd, falling back to an empty struct if no project is found.
func loadProjectForServer() (*config.ProjectConfig, error) {
	cwd, _ := os.Getwd()
	return config.LoadProject(cwd)
}
```

The exact import block + helper functions depend on what's already in `cmd/specgraph/`. Read `health.go` and `init.go` to verify the names of `healthClient`, `loadGlobalCfg`, `config` package alias, etc.

- [ ] **Step 2: Wire Server into doctor.go's runDoctor**

```go
	timeout, _ := cmd.Flags().GetDuration("timeout")
	rep := DoctorReport{
		Binary:  runBinaryGroup(),
		Project: runProjectConfigGroup(cwd),
		Server:  runServerGroup(timeout),
		// Managed wired in commit 6.
	}
```

Update `computeExitCode`:

```go
func computeExitCode(rep DoctorReport) int {
	if !rep.Binary.OK || !rep.Project.OK || !rep.Server.OK {
		return 1
	}
	return 0
}
```

Add the Server line to `renderText`.

- [ ] **Step 3: Server group tests**

Append to `doctor_test.go`:

```go
func TestDoctorReport_Render_ServerOKLine(t *testing.T) {
	rep := DoctorReport{
		Binary:  BinaryReport{OK: true, Version: "0.7.3", BuiltAt: "x", Commit: "y"},
		Project: ProjectReport{OK: true},
		Server:  ServerReport{OK: true, Reachable: true, Version: "0.7.3", MCPHandshake: "ok", SkillsCount: 6},
	}
	var buf bytes.Buffer
	renderText(&buf, rep, false)
	if !strings.Contains(buf.String(), "Server:         OK (reachable v0.7.3 · MCP handshake OK · 6 skills)") {
		t.Errorf("Server OK line not found: %s", buf.String())
	}
}

func TestServerStatusLine_UnreachableExpanded(t *testing.T) {
	rep := ServerReport{Reachable: false, Error: "dial tcp: refused"}
	if !strings.Contains(serverStatusLine(rep), "UNREACHABLE") {
		t.Errorf("unreachable status line missing marker: %s", serverStatusLine(rep))
	}
}
```

The unreachable end-to-end test is left for the e2e in commit 6 — unit tests stay focused on the rendering and status-line logic. The MCP handshake itself is hard to unit-test without a fake server; the e2e covers it.

- [ ] **Step 4: task check**

```bash
task check
```

Expected: PASS.

- [ ] **Step 5: Commit + start next-task**

```bash
jj --no-pager describe -m "feat(doctor): Server group (Health RPC + MCP handshake + Skills count) + --timeout

cmd/specgraph/doctor_server.go runs three sub-checks under one
Server group:

1. Connect Health RPC (matches today's specgraph health).
2. MCP Streamable-HTTP initialize handshake via mcp-go's
   NewStreamableHttpClient + Initialize. Same construction pattern
   PR F's e2e/api/skills_test.go uses.
3. specgraph_skills_list count via mcp-go CallTool, parsed as
   [{name,summary,uri}].

ServerReport carries OK, Reachable, Version, MCPHandshake status,
SkillsCount, optional Error. Each sub-check honors the new --timeout
flag (default 2s). If reachability fails, the MCP + Skills sub-checks
are skipped (no point handshaking an unreachable server).

Adds the 'specgraph doctor server' subcommand the next commit's
health alias targets. Updates computeExitCode and renderText.

Per design §Server group (commit 5).

Bead: spgr-hdki

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new -m "(next task)"
```

---

## Task 6: Managed files group + --fix + --harness + --verbose + path-prefix grouping + e2e

**Files:**

- Create: `cmd/specgraph/doctor_managed.go`
- Modify: `cmd/specgraph/doctor.go` (wire Managed)
- Modify: `cmd/specgraph/doctor_render.go` (Managed line + expanded table)
- Modify: `cmd/specgraph/doctor_test.go` (Managed tests)
- Create: `e2e/api/doctor_test.go`

- [ ] **Step 1: Create `doctor_managed.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"strings"

	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

// ManagedReport describes the inspect results for all managed files
// (filtered by --harness).
type ManagedReport struct {
	OK     bool                    `json:"ok"`
	Synced int                     `json:"synced"`
	Total  int                     `json:"total"`
	Files  []managedfiles.FileState `json:"files"`
}

// runManagedGroup calls InspectAll. The harness slice is built from
// cfg.Harnesses (with the --harness flag overriding if set).
func runManagedGroup(cwd string, harnesses []managedfiles.Harness, params managedfiles.ProjectParams) ManagedReport {
	states, err := managedfiles.InspectAll(cwd, harnesses, params)
	if err != nil {
		return ManagedReport{OK: false, Files: nil}
	}
	rep := ManagedReport{Total: len(states), Files: states}
	for _, s := range states {
		if s.State == managedfiles.StateSynced {
			rep.Synced++
		}
	}
	rep.OK = rep.Synced == rep.Total
	return rep
}

// managedStatusLine renders the compact form.
func managedStatusLine(rep ManagedReport) string {
	if rep.OK {
		return fmt.Sprintf("Managed files:  %d/%d synced", rep.Synced, rep.Total)
	}
	var stale, drifted, missing int
	for _, s := range rep.Files {
		switch s.State {
		case managedfiles.StateStale:
			stale++
		case managedfiles.StateDrifted:
			drifted++
		case managedfiles.StateMissing:
			missing++
		}
	}
	parts := []string{}
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", missing))
	}
	if stale > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", stale))
	}
	if drifted > 0 {
		parts = append(parts, fmt.Sprintf("%d drifted", drifted))
	}
	return fmt.Sprintf("Managed files:  %d/%d synced — %s",
		rep.Synced, rep.Total, strings.Join(parts, ", "))
}

// isHostPinned returns true when the file's path lives outside the
// SpecGraph-owned tree. Anything under .specgraph/agents/ is
// SpecGraph-owned; everything else is host-pinned (.cursor/, .claude/,
// .mcp.json, AGENTS.md, opencode.json).
func isHostPinned(path string) bool {
	return !strings.HasPrefix(path, ".specgraph/agents/")
}

// runDoctorFix applies --fix semantics: Sync each Stale/Missing entry;
// print guidance for Drifted; leave Synced alone.
func runDoctorFix(cwd string, rep ManagedReport, params managedfiles.ProjectParams) error {
	var driftedPaths []string
	for _, s := range rep.Files {
		switch s.State {
		case managedfiles.StateStale, managedfiles.StateMissing:
			mf, ok := managedfiles.FindByPath(s.Path) // helper, see below
			if !ok {
				continue
			}
			if _, err := managedfiles.SyncOne(cwd, mf, params, managedfiles.SyncOptions{}); err != nil {
				return fmt.Errorf("sync %s: %w", s.Path, err)
			}
		case managedfiles.StateDrifted:
			driftedPaths = append(driftedPaths, s.Path)
		}
	}
	for _, path := range driftedPaths {
		fmt.Printf("%s (drifted): run `specgraph init --force --keep-edits %s`\n", path, path)
		fmt.Printf("  to keep your changes, or `specgraph init --force %s` to discard them.\n", path)
	}
	return nil
}
```

(Note: the helpers `managedfiles.FindByPath` and `managedfiles.SyncOne` may not exist today; if so, add them as small wrappers. `FindByPath(path) (ManagedFile, bool)` walks `Manifest(allHarnesses)` and returns the matching entry; `SyncOne(cwd, mf, params, opts)` is `strategyImpl(mf.Strategy).Sync(cwd, mf, params, opts)`. Both are 5-line additions.)

- [ ] **Step 2: Wire Managed group into runDoctor + render**

In `doctor.go`:

```go
	harnessFlag, _ := cmd.Flags().GetString("harness")
	pc, _ := config.LoadProject(cwd) // may be nil for no-project case
	harnesses := harnessesFromFlag(pc, harnessFlag)
	globalCfg, _ := loadGlobalCfg()
	var serverURL string
	slug := ""
	if pc != nil {
		slug = pc.Slug
		serverURL = globalCfg.ResolveServer(pc.Slug, pc.Server)
	}
	params := managedfiles.ProjectParams{Slug: slug, ServerURL: serverURL}

	rep := DoctorReport{
		Binary:  runBinaryGroup(),
		Project: runProjectConfigGroup(cwd),
		Server:  runServerGroup(timeout),
		Managed: runManagedGroup(cwd, harnesses, params),
	}
	rep.ExitCode = computeExitCode(rep)

	// --fix handling
	if fix, _ := cmd.Flags().GetBool("fix"); fix {
		if err := runDoctorFix(cwd, rep.Managed, params); err != nil {
			return err
		}
		// Re-inspect after fix.
		rep.Managed = runManagedGroup(cwd, harnesses, params)
		rep.ExitCode = computeExitCode(rep)
	}
```

Add a `harnessesFromFlag` helper that returns `[]Harness{HarnessClaude}` if `--harness=claude`, falls back to `harnessSliceFromConfig(pc.Harnesses)` otherwise.

Update `computeExitCode`:

```go
func computeExitCode(rep DoctorReport) int {
	if !rep.Binary.OK || !rep.Project.OK || !rep.Server.OK || !rep.Managed.OK {
		return 1
	}
	return 0
}
```

Extend `renderText` to print the Managed line and (when non-OK or verbose) the expanded table grouped by host-pinned / SpecGraph-owned.

- [ ] **Step 3: Managed group tests**

Append to `doctor_test.go`:

```go
func TestManagedStatusLine_AllSynced(t *testing.T) {
	rep := ManagedReport{OK: true, Synced: 14, Total: 14}
	if got := managedStatusLine(rep); got != "Managed files:  14/14 synced" {
		t.Errorf("got %q", got)
	}
}

func TestManagedStatusLine_Mixed(t *testing.T) {
	rep := ManagedReport{Synced: 12, Total: 14, Files: []managedfiles.FileState{
		{State: managedfiles.StateSynced},
		{State: managedfiles.StateStale},
		{State: managedfiles.StateDrifted},
	}}
	got := managedStatusLine(rep)
	if !strings.Contains(got, "12/14 synced") {
		t.Errorf("missing count: %q", got)
	}
	if !strings.Contains(got, "1 stale") || !strings.Contains(got, "1 drifted") {
		t.Errorf("missing breakdown: %q", got)
	}
}

func TestIsHostPinned(t *testing.T) {
	cases := []struct {
		path     string
		wantHost bool
	}{
		{".cursor/rules/specgraph.mdc", true},
		{".claude/settings.json", true},
		{".mcp.json", true},
		{"AGENTS.md", true},
		{".specgraph/agents/claude/routing-guide.md", false},
		{".specgraph/agents/opencode/specgraph.ts", false},
	}
	for _, tc := range cases {
		if got := isHostPinned(tc.path); got != tc.wantHost {
			t.Errorf("isHostPinned(%q) = %v, want %v", tc.path, got, tc.wantHost)
		}
	}
}
```

- [ ] **Step 4: Create the e2e test**

Create `e2e/api/doctor_test.go` (build tag `e2e`):

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("specgraph doctor", func() {
	It("reports all groups healthy on a freshly-init'd project", func() {
		tmp := freshProjectDir()
		runSpecgraph(tmp, "init", "--yes")
		out := runSpecgraph(tmp, "doctor")
		Expect(out.Stdout).To(ContainSubstring("Binary:"))
		Expect(out.Stdout).To(ContainSubstring("Server:         OK"))
		Expect(out.Stdout).To(ContainSubstring("Project config: OK"))
		Expect(out.Stdout).To(ContainSubstring("Managed files:  14/14 synced"))
		Expect(out.ExitCode).To(Equal(0))
	})

	It("flags a drifted managed file and prints guidance", func() {
		tmp := freshProjectDir()
		runSpecgraph(tmp, "init", "--yes")
		corruptOneManagedFile(tmp, "AGENTS.md")
		out := runSpecgraph(tmp, "doctor", "--fix")
		Expect(out.Stdout).To(ContainSubstring("AGENTS.md (drifted)"))
		Expect(out.Stdout).To(ContainSubstring("--keep-edits"))
		Expect(out.ExitCode).To(Equal(1))
	})

	It("--json produces stable schema", func() {
		tmp := freshProjectDir()
		runSpecgraph(tmp, "init", "--yes")
		out := runSpecgraph(tmp, "doctor", "--json")
		var rep map[string]any
		Expect(json.Unmarshal([]byte(out.Stdout), &rep)).To(Succeed())
		groups, _ := rep["groups"].(map[string]any)
		Expect(groups).To(HaveKey("binary"))
		Expect(groups).To(HaveKey("server"))
		Expect(groups).To(HaveKey("project"))
		Expect(groups).To(HaveKey("managed"))
	})
})
```

The `freshProjectDir`, `runSpecgraph`, and `corruptOneManagedFile` helpers should reuse the e2e harness pattern from `e2e/api/skills_test.go` (PR F's e2e). If those helpers don't exist with those names, match what the e2e suite actually provides.

- [ ] **Step 5: task check + e2e compile-check**

```bash
task check
go vet -tags e2e ./e2e/api/...
```

Expected: both PASS.

- [ ] **Step 6: Commit + start next-task**

```bash
jj --no-pager describe -m "feat(doctor): Managed files group + --fix + --harness + --verbose + e2e

cmd/specgraph/doctor_managed.go calls InspectAll and reports the
14-file table compactly when all synced ('14/14 synced') or with a
mismatch breakdown when not ('12/14 synced — 1 stale, 1 drifted').

--fix path: SyncOne for each Stale/Missing row; one guidance line
per Drifted file naming both recovery commands. Synced rows are
left alone.

--harness flag narrows the InspectAll filter to one harness when
set; falls back to cfg.Harnesses (commit 1) when not.

Path-prefix grouping: isHostPinned checks for the
.specgraph/agents/ prefix; expanded table renders host-pinned and
SpecGraph-owned subsections. No new ManagedFile field needed.

e2e/api/doctor_test.go covers three It blocks: all-healthy
after fresh init, drifted-file flagging with --fix guidance, and
--json schema parse.

Per design §Managed files (commit 6).

Bead: spgr-hdki

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new -m "(next task)"
```

---

## Task 7: Deprecate health as alias for doctor server

**Files:**

- Modify: `cmd/specgraph/health.go`
- Modify: `cmd/specgraph/doctor_test.go` (TestHealthAlias)

- [ ] **Step 1: Replace `health.go`'s body**

Replace the entire `runHealth` function with:

```go
func runHealth(cmd *cobra.Command, args []string) error {
	// `specgraph health` is deprecated; it now dispatches to
	// `specgraph doctor server`. The deprecation notice goes to stderr
	// so script consumers reading stdout don't see it. The doctor
	// server runE preserves the original health exit codes.
	fmt.Fprintln(os.Stderr,
		"specgraph health: deprecated, use `specgraph doctor server` (this command will be removed in a future release)")
	return doctorServerCmd.RunE(cmd, args)
}
```

Keep the rest of `health.go` (the `init()` that registers `healthCmd`, the client helpers) as-is.

- [ ] **Step 2: Test**

Append to `doctor_test.go`:

```go
func TestHealthAlias_DispatchesAndEmitsDeprecationNotice(t *testing.T) {
	// Capture stderr; run healthCmd's RunE with a stub cobra cmd.
	// Assert: stderr contains the deprecation notice; stdout matches
	// what doctorServerCmd's RunE would emit.
	// Exact form depends on the project's existing pattern for
	// invoking cobra RunEs from tests (see init_test.go).
}
```

- [ ] **Step 3: task check**

```bash
task check
```

Expected: PASS.

- [ ] **Step 4: Commit + start next-task**

```bash
jj --no-pager describe -m "refactor(health): deprecate health command as alias for doctor server

cmd/specgraph/health.go now prints a deprecation notice on stderr
and dispatches to doctorServerCmd.RunE. Existing scripts that
parse stdout see unchanged output and exit codes. The notice
appears only when 'health' is invoked directly; 'specgraph doctor
server' itself prints no notice.

Per design §Sequencing (commit 7) and §Public surface.

Bead: spgr-hdki

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new -m "(next task)"
```

---

## Task 8: Drift-nudge PersistentPreRun + task plugin:refresh/check + docs

**Files:**

- Create: `cmd/specgraph/nudge.go`
- Create: `cmd/specgraph/nudge_test.go`
- Modify: `cmd/specgraph/root.go` (wire PersistentPreRunE)
- Modify: `Taskfile.yml`
- Modify: `CLAUDE.md`
- Modify: `plugin/specgraph/routing-guide.md`

- [ ] **Step 1: Write `nudge.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/config/managedfiles"
	"github.com/specgraph/specgraph/internal/xdg"
)

// driftNudgeAllowList enumerates the top-level command names whose
// subtrees skip the drift-nudge entirely. Matched against the
// top-level command (one level under rootCmd) per design.
var driftNudgeAllowList = map[string]bool{
	"init":              true,
	"doctor":            true,
	"health":            true,
	"read-mcp-resource": true,
	"serve":             true,
	"version":           true,
	"bundle":            true,
	"up":                true,
	"confluence":        true,
}

// nudgePreRun is rootCmd.PersistentPreRunE. Runs InspectAll and emits
// one stderr line if any file is non-Synced. Multiple skip gates
// (subcommand allow-list, isatty, env, config, throttle) keep the
// fast path cheap.
func nudgePreRun(cmd *cobra.Command, _ []string) error {
	// 1. Subcommand allow-list: walk to the top-level command.
	top := cmd
	for top.HasParent() && top.Parent() != rootCmd {
		top = top.Parent()
	}
	if driftNudgeAllowList[top.Name()] {
		return nil
	}
	// 2. isatty(stderr).
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return nil
	}
	// 3. Env-var mute.
	if os.Getenv("SPECGRAPH_DRIFT_NUDGE") == "off" {
		return nil
	}
	// 4. Project-level mute, project root, and harness list — together,
	//    because all three derive from ProjectConfig.
	cwd, err := os.Getwd()
	if err != nil {
		return nil // advisory feature; never fail the CLI
	}
	root, err := config.FindProjectRoot(cwd)
	if err != nil {
		// No project up the tree — nothing to inspect, nothing to nudge.
		return nil
	}
	pc, err := config.LoadProject(root)
	if err != nil {
		return nil
	}
	if pc.Nudges.Quiet {
		return nil
	}
	// 5. Throttle.
	if !shouldEmitAfterThrottle(root) {
		return nil
	}
	// Build ProjectParams matching init's path so sentinel hashes line
	// up (commit 1's harnessSliceFromConfig + ResolveServer).
	globalCfg, err := loadGlobalCfg()
	if err != nil {
		return nil
	}
	params := managedfiles.ProjectParams{
		Slug:      pc.Slug,
		ServerURL: globalCfg.ResolveServer(pc.Slug, pc.Server),
	}
	harnesses := harnessSliceFromConfig(pc.Harnesses)

	states, err := managedfiles.InspectAll(root, harnesses, params)
	if err != nil {
		return nil
	}
	var stale, drifted int
	for _, s := range states {
		switch s.State {
		case managedfiles.StateStale:
			stale++
		case managedfiles.StateDrifted:
			drifted++
		}
	}
	if stale == 0 && drifted == 0 {
		return nil
	}
	fmt.Fprintf(os.Stderr,
		"note: %d managed files out of date with this binary (%d stale, %d drifted); run `specgraph init` to refresh, `specgraph doctor` for details\n",
		stale+drifted, stale, drifted)
	gcOldNudgeFiles()
	return nil
}

// shouldEmitAfterThrottle returns true if the throttle file for
// (projectRoot, binaryVersionHash) is missing or older than 24h.
// On error or unwritable cache, returns true (fail open) per design.
func shouldEmitAfterThrottle(projectRoot string) bool {
	path, err := throttleFilePath(projectRoot)
	if err != nil {
		return true
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// First time — create and emit.
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			_ = os.WriteFile(path, nil, 0o644) //nolint:gosec // empty marker file
			return true
		}
		return true
	}
	if time.Since(info.ModTime()) < 24*time.Hour {
		return false
	}
	_ = os.Chtimes(path, time.Now(), time.Now())
	return true
}

func throttleFilePath(projectRoot string) (string, error) {
	resolved, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		resolved = projectRoot
	}
	projectHash := fmt.Sprintf("%x", sha256.Sum256([]byte(resolved)))
	versionHash := fmt.Sprintf("%x", crc32.ChecksumIEEE([]byte(version+buildTime+commit)))
	return filepath.Join(xdg.CacheHome(), "nudges", projectHash+"-"+versionHash), nil
}

// gcOldNudgeFiles deletes throttle entries with mtime > 30 days. One
// readdir per nudge; cheap and prevents indefinite accumulation.
func gcOldNudgeFiles() {
	dir := filepath.Join(xdg.CacheHome(), "nudges")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
```

- [ ] **Step 2: Wire into `root.go`**

In `cmd/specgraph/root.go`, find the `rootCmd` declaration and add:

```go
var rootCmd = &cobra.Command{
	Use:                "specgraph",
	Short:              "...",  // keep existing
	PersistentPreRunE:  nudgePreRun,
	// ... existing fields ...
}
```

If `rootCmd` already has `PersistentPreRunE` (unlikely but possible), chain the new hook after the existing one.

- [ ] **Step 3: Tests**

Create `cmd/specgraph/nudge_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestNudge_SkippedByAllowList(t *testing.T) {
	t.Setenv("SPECGRAPH_DRIFT_NUDGE", "") // ensure not off
	cmd := &cobra.Command{Use: "init"}
	rootCmd.AddCommand(cmd)
	defer rootCmd.RemoveCommand(cmd)
	if err := nudgePreRun(cmd, nil); err != nil {
		t.Errorf("init allow-list hit returned error: %v", err)
	}
	// Implementer: capture stderr; assert no output.
}

func TestNudge_SkippedByEnvVar(t *testing.T) {
	t.Setenv("SPECGRAPH_DRIFT_NUDGE", "off")
	cmd := &cobra.Command{Use: "list"}
	rootCmd.AddCommand(cmd)
	defer rootCmd.RemoveCommand(cmd)
	if err := nudgePreRun(cmd, nil); err != nil {
		t.Errorf("env-off returned error: %v", err)
	}
}

func TestNudge_SkippedWhenNoProject(t *testing.T) {
	// chdir to a temp dir with no .specgraph.yaml anywhere up the tree.
	tmp := t.TempDir()
	oldwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	cmd := &cobra.Command{Use: "list"}
	rootCmd.AddCommand(cmd)
	defer rootCmd.RemoveCommand(cmd)
	if err := nudgePreRun(cmd, nil); err != nil {
		t.Errorf("no-project case returned error: %v", err)
	}
	// Assert no nudge file was created and no stderr emitted.
}

func TestNudge_ThrottledWithinWindow(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	// Set up a throttle file with fresh mtime.
	root := t.TempDir()
	path, err := throttleFilePath(root)
	if err != nil {
		t.Fatalf("throttleFilePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if shouldEmitAfterThrottle(root) {
		t.Error("fresh throttle file should suppress emit")
	}
}

func TestNudge_EmittedAfterWindow(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	root := t.TempDir()
	path, err := throttleFilePath(root)
	if err != nil {
		t.Fatalf("throttleFilePath: %v", err)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, nil, 0o644)
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if !shouldEmitAfterThrottle(root) {
		t.Error("expired throttle file should permit emit")
	}
}

func TestNudge_GarbageCollectsOldEntries(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	nudgesDir := filepath.Join(cacheDir, "specgraph", "nudges")
	_ = os.MkdirAll(nudgesDir, 0o755)
	oldPath := filepath.Join(nudgesDir, "old-entry")
	freshPath := filepath.Join(nudgesDir, "fresh-entry")
	_ = os.WriteFile(oldPath, nil, 0o644)
	_ = os.WriteFile(freshPath, nil, 0o644)
	veryOld := time.Now().Add(-31 * 24 * time.Hour)
	_ = os.Chtimes(oldPath, veryOld, veryOld)

	gcOldNudgeFiles()

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("old entry not GC'd: %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Errorf("fresh entry incorrectly GC'd: %v", err)
	}
}
```

(Adjust the `rootCmd` interaction if cobra's API doesn't support `RemoveCommand` cleanly — alternative is to use a fresh `cobra.Command` for the test rather than the package's `rootCmd`.)

- [ ] **Step 4: Taskfile updates**

In `Taskfile.yml`, find the `check:` block (around L297-306) and insert `- task: plugin:check` between `- task: lint` and `- task: skills:validate`:

```yaml
  check:
    desc: Fast local quality gate (no Docker required)
    deps: [generate]
    cmds:
      - task: fmt:check
      - task: license:check
      - task: lint
      - task: plugin:check
      - task: skills:validate
      - task: build
      - go test -short -race ./...
```

Add the two new targets at the end of the file:

```yaml
  plugin:refresh:
    desc: Rebuild specgraph and re-run init against the current project to pick up plugin canonical edits
    cmds:
      - task: build
      - go run ./cmd/specgraph init --quiet

  plugin:check:
    desc: Verify the embedded canonicals match what specgraph init would write
    cmds:
      - task: build
      - go run ./cmd/specgraph init --check
```

- [ ] **Step 5: Documentation updates**

In `CLAUDE.md`, add a new subsection under "Commands" (or wherever fits):

```markdown
### Doctor + drift-nudge

`specgraph doctor` reports four check groups (Binary, Server, Project
config, Managed files). Default output is compact when everything is
green; sections expand when problems exist. `--json` for
machine-readable output; `--fix` auto-init's Stale/Missing rows and
prints guidance for Drifted; `--harness <name>` narrows; `--exit-zero`
suppresses non-zero exit for advisory use.

Every CLI invocation runs a drift-nudge in `PersistentPreRun` that
emits one stderr line if any managed file is non-Synced. Skip gates:
the subcommand allow-list (`init`, `doctor`, `health`, etc.),
`isatty(stderr)`, `SPECGRAPH_DRIFT_NUDGE=off`, `.specgraph.yaml`'s
`nudges.quiet: true`, and a 24h throttle file at
`xdg.CacheHome()/nudges/`.

`task plugin:refresh` rebuilds + re-init's against the current
project. `task plugin:check` runs `init --check` and exits non-zero
if any managed file would be modified — it's wired into `task check`
so a contributor who edited `plugin/<harness>/...` without rebuilding
sees the failure during the same `task check` they run pre-push.

`.specgraph.yaml` gains two fields:
- `harnesses: [claude, cursor, opencode]` — per-project allow-list
  (empty = all three, matching the legacy behaviour).
- `nudges: { quiet: true }` — project-level mute for the drift-nudge.
```

In `plugin/specgraph/routing-guide.md`, add a line:

```markdown
If something seems off (drift, server unreachable, harness misconfigured), run `specgraph doctor` for the full report.
```

- [ ] **Step 6: task check**

```bash
task check
```

Expected: PASS. This run now includes `plugin:check` between `lint` and `skills:validate`; since this is the first commit where `plugin:check` exists, the contributor running task check verifies the whole chain end-to-end here.

- [ ] **Step 7: Commit (final)**

```bash
jj --no-pager describe -m "feat(cmd): drift-nudge PersistentPreRun + task plugin:refresh/plugin:check + docs

cmd/specgraph/nudge.go implements the drift-nudge as a
rootCmd.PersistentPreRunE hook. Skip gates evaluate in order:

1. Subcommand allow-list (top-level walk to ignore cmd.Name() leaf
   weirdness for nested cobra commands).
2. isatty(stderr) — primary gate; catches Claude session-start
   hook, MCP server, shell pipes, CI.
3. SPECGRAPH_DRIFT_NUDGE=off env var.
4. Project root + lenient LoadProject + Nudges.Quiet check (folded
   together because all three derive from the project config).
5. 24h throttle file at xdg.CacheHome()/nudges/<project-hash>-
   <version-hash>; opportunistic GC of entries >30 days old.

ProjectParams.ServerURL goes through globalCfg.ResolveServer
(matching init.go) so sentinel hashes line up — otherwise
JSONKeyMerge entries would flip Stale spuriously on every fresh
init.

Taskfile.yml: adds plugin:refresh and plugin:check; inserts
plugin:check into the `check:` cmds sequence between lint and
skills:validate.

Documentation: CLAUDE.md gains a Doctor + drift-nudge subsection
covering the surface, the nudge skip gates, the new Taskfile
targets, and the .specgraph.yaml schema additions. routing-guide.md
adds a one-line pointer to `specgraph doctor`.

Per design §Drift-nudge and §Dogfood tasks (commit 8).

Bead: spgr-hdki

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new -m "(post-pr-g)"
```

---

## Test plan (full PR)

After all 9 tasks land:

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-pr-g
task check         # fmt → license → lint → plugin:check → skills:validate → build → tests
task pr-prep       # check + integration + e2e (requires Docker)
go vet -tags e2e ./e2e/api/...
```

Targeted runs:

```bash
go test -v ./internal/config/...
go test -v ./internal/config/managedfiles/...
go test -v ./cmd/specgraph/ -run 'TestDoctor|TestNudge|TestInit_CheckFlag|TestHealthAlias'
go test -tags e2e -v ./e2e/api/... -run doctor
```

---

## Self-review checklist

**Spec coverage:**

- ✓ ProjectConfig.Harnesses + Nudges.Quiet + init.go fallback → Task 1
- ✓ FileState.Harness + InspectAll overwrite → Task 1a
- ✓ init --check + --quiet → Task 2
- ✓ doctor command + DoctorReport + Binary group + render → Task 3
- ✓ Project config group + ValidateProjectStrict → Task 4
- ✓ Server group (Health + MCP handshake + Skills) + --timeout → Task 5
- ✓ Managed files group + --fix + --harness + --verbose + e2e → Task 6
- ✓ health → doctor server alias + deprecation notice → Task 7
- ✓ Drift-nudge PersistentPreRun + Taskfile + docs → Task 8

**Placeholder scan:** every step contains concrete code or commands; no TBD / TODO / "implement later".

**Type consistency:**

- `harnessSliceFromConfig([]string) []managedfiles.Harness` defined in Task 1, used in Tasks 6 + 8.
- `config.ValidateProjectStrict(path string) error` defined in Task 4, called from Task 4's `doctor_config.go`.
- `runBinaryGroup`, `runProjectConfigGroup`, `runServerGroup`, `runManagedGroup` form a uniform group-result pattern.
- `BinaryReport`, `ProjectReport`, `ServerReport`, `ManagedReport` all carry `OK bool` plus group-specific fields and a JSON tag.
- `computeExitCode(DoctorReport) int` grows across Tasks 3 → 4 → 5 → 6; each task adds one term to the AND chain.

---

## Summary

Nine commits (Tasks 1, 1a, 2-8). Each commit must stay green under `task check`. The pr-g jj workspace at `/Users/SeBrandt/Code/github.com/specgraph-pr-g` holds the stack. After landing, push the bookmark with `jj git push --bookmark spgr-rwrp-pr-g` and open the PR following the same flow PRs C/D/E/F used.
