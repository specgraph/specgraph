# Slice CLI Commands Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `specgraph slice list|get|claim|complete` CLI commands so users can manage decomposition slices from the terminal.

**Architecture:** New `cmd/specgraph/slice.go` with a `slice` parent command and 4 subcommands following the `decision.go` pattern. New `internal/render/slice.go` for markdown output. Update `decompose.go` to show `slice_slugs` from the response.

**Tech Stack:** Go, cobra, ConnectRPC, testify

**Bead:** spgr-6sw.6

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `cmd/specgraph/slice.go` | `slice` parent + `list`, `get`, `claim`, `complete` subcommands |
| Create | `internal/render/slice.go` | `SliceDetail` and `SliceList` markdown renderers |
| Create | `internal/render/slice_test.go` | Renderer unit tests |
| Modify | `cmd/specgraph/decompose.go:47-53` | Show `slice_slugs` instead of inline slice details |

---

## Task 1: Slice renderers

Render functions that format proto Slice messages as Markdown for terminal output.

**Files:**

- Create: `internal/render/slice.go`
- Create: `internal/render/slice_test.go`

- [ ] **Step 1: Write renderer tests**

Create `internal/render/slice_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render_test

import (
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/stretchr/testify/require"
)

func TestSliceDetail(t *testing.T) {
	out := render.SliceDetail(&specv1.Slice{
		Slug:       "my-spec/backend-api",
		ParentSlug: "my-spec",
		SliceId:    "backend-api",
		Intent:     "Implement REST endpoints",
		Status:     specv1.SliceStatus_SLICE_STATUS_CLAIMED,
		AssignedTo: "alice",
		Verify:     []string{"all tests pass", "no lint errors"},
		Touches:    []string{"internal/server/", "cmd/specgraph/"},
		DependsOn:  []string{"my-spec/data-model"},
	})
	require.Contains(t, out, "my-spec/backend-api")
	require.Contains(t, out, "Implement REST endpoints")
	require.Contains(t, out, "claimed")
	require.Contains(t, out, "alice")
	require.Contains(t, out, "all tests pass")
	require.Contains(t, out, "my-spec/data-model")
}

func TestSliceDetail_Nil(t *testing.T) {
	require.Empty(t, render.SliceDetail(nil))
}

func TestSliceList(t *testing.T) {
	out := render.SliceList([]*specv1.Slice{
		{Slug: "p/a", SliceId: "a", Intent: "First", Status: specv1.SliceStatus_SLICE_STATUS_OPEN},
		{Slug: "p/b", SliceId: "b", Intent: "Second", Status: specv1.SliceStatus_SLICE_STATUS_DONE},
	})
	require.Contains(t, out, "a")
	require.Contains(t, out, "First")
	require.Contains(t, out, "open")
	require.Contains(t, out, "done")
}

func TestSliceList_Empty(t *testing.T) {
	require.Contains(t, render.SliceList(nil), "No slices")
}
```

- [ ] **Step 2: Run tests — expect compilation failure**

Run: `go test ./internal/render/ -run TestSlice -v`

Expected: Fails — `render.SliceDetail` and `render.SliceList` don't exist.

- [ ] **Step 3: Write renderers**

Create `internal/render/slice.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// SliceDetail renders a single slice as markdown.
func SliceDetail(s *specv1.Slice) string {
	if s == nil {
		return ""
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", s.Slug)
	if s.Intent != "" {
		fmt.Fprintf(&b, "> %s\n\n", s.Intent)
	}

	pairs := [][2]string{
		{"Status", sliceStatusString(s.Status)},
		{"Parent", s.ParentSlug},
	}
	if s.AssignedTo != "" {
		pairs = append(pairs, [2]string{"Assigned To", s.AssignedTo})
	}
	b.WriteString(metadataTable(pairs))

	if len(s.Verify) > 0 {
		b.WriteString("\n## Verify\n\n")
		for _, v := range s.Verify {
			fmt.Fprintf(&b, "- %s\n", v)
		}
	}
	if len(s.Touches) > 0 {
		b.WriteString("\n## Touches\n\n")
		for _, t := range s.Touches {
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	if len(s.DependsOn) > 0 {
		b.WriteString("\n## Depends On\n\n")
		for _, d := range s.DependsOn {
			fmt.Fprintf(&b, "- %s\n", d)
		}
	}
	return b.String()
}

// SliceList renders a list of slices as a table.
func SliceList(slices []*specv1.Slice) string {
	if len(slices) == 0 {
		return "No slices found.\n"
	}
	var b strings.Builder
	b.WriteString("| ID | Intent | Status | Assigned To |\n")
	b.WriteString("|------|--------|--------|-------------|\n")
	for _, s := range slices {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			s.SliceId, truncate(s.Intent, 40),
			sliceStatusString(s.Status), s.AssignedTo)
	}
	return b.String()
}

func sliceStatusString(s specv1.SliceStatus) string {
	switch s {
	case specv1.SliceStatus_SLICE_STATUS_OPEN:
		return "open"
	case specv1.SliceStatus_SLICE_STATUS_CLAIMED:
		return "claimed"
	case specv1.SliceStatus_SLICE_STATUS_DONE:
		return "done"
	default:
		return "unknown"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
```

- [ ] **Step 4: Run tests — all pass**

Run: `go test ./internal/render/ -run TestSlice -v`

Expected: 4 tests pass.

- [ ] **Step 5: Commit**

```text
jj --no-pager describe -m "feat(render): add Slice markdown renderers

SliceDetail and SliceList for CLI output, following the Decision
renderer pattern. Status string mapping and table formatting.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Slice CLI commands

The `specgraph slice` parent command and 4 subcommands.

**Files:**

- Create: `cmd/specgraph/slice.go`

- [ ] **Step 1: Create slice.go with all commands**

Create `cmd/specgraph/slice.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

func sliceClient() (specgraphv1connect.SliceServiceClient, error) {
	return newClient(specgraphv1connect.NewSliceServiceClient)
}

// --- slice parent command ---

var sliceCmd = &cobra.Command{
	Use:   "slice",
	Short: "Manage decomposition slices",
}

// --- slice list ---

var sliceListCmd = &cobra.Command{
	Use:   "list <parent-slug>",
	Short: "List slices for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runSliceList,
}

var sliceListJSON bool

func runSliceList(cmd *cobra.Command, args []string) error {
	client, err := sliceClient()
	if err != nil {
		return err
	}
	resp, err := client.ListSlices(context.Background(), connect.NewRequest(&specv1.ListSlicesRequest{
		ParentSlug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("list slices: %w", err)
	}
	if sliceListJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.SliceList(resp.Msg.Slices))
	return nil
}

// --- slice get ---

var sliceGetCmd = &cobra.Command{
	Use:   "get <slug>",
	Short: "Show slice details",
	Args:  cobra.ExactArgs(1),
	RunE:  runSliceGet,
}

var sliceGetJSON bool

func runSliceGet(cmd *cobra.Command, args []string) error {
	client, err := sliceClient()
	if err != nil {
		return err
	}
	resp, err := client.GetSlice(context.Background(), connect.NewRequest(&specv1.GetSliceRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("get slice: %w", err)
	}
	if sliceGetJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.SliceDetail(resp.Msg.Slice))
	return nil
}

// --- slice claim ---

var sliceClaimCmd = &cobra.Command{
	Use:   "claim <slug>",
	Short: "Claim a slice for work",
	Args:  cobra.ExactArgs(1),
	RunE:  runSliceClaim,
}

var sliceClaimAssignee string

func runSliceClaim(_ *cobra.Command, args []string) error {
	client, err := sliceClient()
	if err != nil {
		return err
	}
	resp, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug:     args[0],
		Assignee: sliceClaimAssignee,
	}))
	if err != nil {
		return fmt.Errorf("claim slice: %w", err)
	}
	fmt.Printf("Claimed: %s by %s\n", resp.Msg.GetSlice().GetSlug(), resp.Msg.GetSlice().GetAssignedTo())
	return nil
}

// --- slice complete ---

var sliceCompleteCmd = &cobra.Command{
	Use:   "complete <slug>",
	Short: "Mark a slice as done",
	Args:  cobra.ExactArgs(1),
	RunE:  runSliceComplete,
}

func runSliceComplete(_ *cobra.Command, args []string) error {
	client, err := sliceClient()
	if err != nil {
		return err
	}
	resp, err := client.CompleteSlice(context.Background(), connect.NewRequest(&specv1.CompleteSliceRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("complete slice: %w", err)
	}
	fmt.Printf("Completed: %s\n", resp.Msg.GetSlice().GetSlug())
	return nil
}

// --- registration ---

func init() {
	rootCmd.AddCommand(sliceCmd)

	sliceListCmd.Flags().BoolVar(&sliceListJSON, "json", false, "output as JSON")
	sliceCmd.AddCommand(sliceListCmd)

	sliceGetCmd.Flags().BoolVar(&sliceGetJSON, "json", false, "output as JSON")
	sliceCmd.AddCommand(sliceGetCmd)

	sliceClaimCmd.Flags().StringVar(&sliceClaimAssignee, "assignee", "", "who is claiming (required)")
	cobra.CheckErr(sliceClaimCmd.MarkFlagRequired("assignee"))
	sliceCmd.AddCommand(sliceClaimCmd)

	sliceCmd.AddCommand(sliceCompleteCmd)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./cmd/specgraph/`

Expected: Clean build.

- [ ] **Step 3: Verify help output**

Run: `./specgraph slice --help`

Expected: Shows `list`, `get`, `claim`, `complete` subcommands.

- [ ] **Step 4: Commit**

```text
jj --no-pager describe -m "feat(cli): add slice list/get/claim/complete commands (spgr-6sw.6)

Four subcommands under 'specgraph slice' following the decision.go
pattern. --json flag on list/get, --assignee on claim.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Update decompose.go output

Show `slice_slugs` from the response instead of inline slice details.

**Files:**

- Modify: `cmd/specgraph/decompose.go:47-53`

- [ ] **Step 1: Update decompose output**

In `cmd/specgraph/decompose.go`, replace the output block (lines 47-53):

```go
	fmt.Printf("Decomposed: %s\n", args[0])
	if resp.Msg.Output != nil {
		fmt.Printf("Strategy: %s\n", resp.Msg.Output.Strategy)
	}
	if len(resp.Msg.SliceSlugs) > 0 {
		fmt.Printf("Slices (%d):\n", len(resp.Msg.SliceSlugs))
		for _, slug := range resp.Msg.SliceSlugs {
			fmt.Printf("  - %s\n", slug)
		}
	}
```

- [ ] **Step 2: Build**

Run: `go build ./cmd/specgraph/`

Expected: Clean build.

- [ ] **Step 3: Full quality check**

Run: `task check`

Expected: All pass (fmt, license, lint, build, tests).

- [ ] **Step 4: Commit**

```text
jj --no-pager describe -m "refactor(cli): decompose output shows slice_slugs instead of inline details

DecomposeResponse.slice_slugs replaces inline slice rendering.
Users can pipe slugs to 'specgraph slice get' for full details.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 5: Close bead**

```text
bd close spgr-6sw.6 --reason="Slice CLI commands implemented: list, get, claim, complete + decompose output updated"
```
