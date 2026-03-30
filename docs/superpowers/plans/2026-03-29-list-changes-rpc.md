# ListChanges RPC Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the existing `ChangeLogBackend.ListChanges` via a ConnectRPC endpoint in SpecService, with a CLI command and markdown renderer.

**Architecture:** Add proto messages + RPC to SpecService, implement handler following existing scoper pattern, new converter + renderer files, new CLI command mirroring `findings list` pattern.

**Tech Stack:** ConnectRPC, protobuf, Go, Cobra CLI

**Spec:** `docs/superpowers/specs/2026-03-29-list-changes-rpc-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `proto/specgraph/v1/spec.proto` | Add `ChangeLogEntry` message, `ListChangesRequest/Response`, `ListChanges` RPC |
| `internal/server/convert_changelog.go` | New. Domain → proto converter for changelog entries |
| `internal/server/convert_changelog_test.go` | New. Converter tests |
| `internal/server/spec_handler.go` | Add `ListChanges` method |
| `internal/auth/permissions.go` | Add RPC permission entry |
| `internal/render/changelog.go` | New. Markdown timeline renderer |
| `internal/render/changelog_test.go` | New. Renderer tests |
| `cmd/specgraph/changes.go` | New. `specgraph changes <slug>` CLI command |

---

## Chunk 1: Proto + Converter + Handler + Permissions + Renderer + CLI

### Task 1: Add proto messages and RPC

**Files:**

- Modify: `proto/specgraph/v1/spec.proto:48-95`

- [ ] **Step 1: Add ChangeLogEntry message and ListChanges RPC**

In `proto/specgraph/v1/spec.proto`, add after the existing `FieldChange` message (line 52) and before `CreateSpecRequest` (line 54):

```protobuf
message ChangeLogEntry {
  string id = 1;
  int32 version = 2;
  string stage = 3;
  string content_hash = 4;
  bool checkpoint = 5;
  string summary = 6;
  string reason = 7;
  repeated FieldChange changes = 8;
  google.protobuf.Timestamp date = 9;
}

message ListChangesRequest {
  string slug = 1;
  bool checkpoints_only = 2;
  int32 since_version = 3;
  int32 limit = 4;
}

message ListChangesResponse {
  repeated ChangeLogEntry entries = 1;
}
```

Add `google.protobuf.Timestamp` import at the top if not already present:
```protobuf
import "google/protobuf/timestamp.proto";
```

Add the RPC to the `SpecService` block (after `UpdateSpec`):
```protobuf
  rpc ListChanges(ListChangesRequest) returns (ListChangesResponse);
```

- [ ] **Step 2: Generate Go code**

Run: `task proto`

- [ ] **Step 3: Verify generated code compiles**

Run: `go build ./gen/...`
Expected: Clean

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe -m "proto(spec): add ChangeLogEntry message and ListChanges RPC (spgr-fn5)"
jj --no-pager new -m ""
```

---

### Task 2: Add converter (domain → proto)

**Files:**

- Create: `internal/server/convert_changelog.go`
- Create: `internal/server/convert_changelog_test.go`

- [ ] **Step 1: Write failing converter test**

Create `internal/server/convert_changelog_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
)

func TestChangeLogEntryToProto(t *testing.T) {
	entry := &storage.ChangeLogEntry{
		ID:          "cl-1",
		Version:     3,
		Stage:       storage.StageShape,
		ContentHash: "a1b2c3d4",
		Checkpoint:  true,
		Summary:     "Refined scope",
		Reason:      "Feedback from review",
		Changes: []storage.FieldChange{
			{Field: "intent", OldValue: "Build X", NewValue: "Build X (v2)"},
			{Field: "stage", OldValue: "spark", NewValue: "shape"},
		},
		Date: time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC),
	}

	pb := changeLogEntryToProto(entry)

	assert.Equal(t, "cl-1", pb.Id)
	assert.Equal(t, int32(3), pb.Version)
	assert.Equal(t, "shape", pb.Stage)
	assert.Equal(t, "a1b2c3d4", pb.ContentHash)
	assert.True(t, pb.Checkpoint)
	assert.Equal(t, "Refined scope", pb.Summary)
	assert.Equal(t, "Feedback from review", pb.Reason)
	require.Len(t, pb.Changes, 2)
	assert.Equal(t, "intent", pb.Changes[0].Field)
	assert.Equal(t, "Build X", pb.Changes[0].OldValue)
	assert.Equal(t, "Build X (v2)", pb.Changes[0].NewValue)
	assert.Equal(t, int64(2026), int64(pb.Date.AsTime().Year()))
}

func TestChangeLogEntryToProto_NoChanges(t *testing.T) {
	entry := &storage.ChangeLogEntry{
		ID:      "cl-2",
		Version: 1,
		Stage:   storage.StageSpark,
		Date:    time.Now(),
	}

	pb := changeLogEntryToProto(entry)

	assert.Equal(t, "cl-2", pb.Id)
	assert.Empty(t, pb.Changes)
}

func TestChangeLogEntriesToProto_Empty(t *testing.T) {
	pbs := changeLogEntriesToProto(nil)
	assert.Empty(t, pbs)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestChangeLogEntry -v -count=1`
Expected: FAIL — `changeLogEntryToProto` not defined

- [ ] **Step 3: Implement converter**

Create `internal/server/convert_changelog.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func changeLogEntryToProto(e *storage.ChangeLogEntry) *specv1.ChangeLogEntry {
	changes := make([]*specv1.FieldChange, len(e.Changes))
	for i, c := range e.Changes {
		changes[i] = &specv1.FieldChange{
			Field:    c.Field,
			OldValue: c.OldValue,
			NewValue: c.NewValue,
		}
	}
	return &specv1.ChangeLogEntry{
		Id:          e.ID,
		Version:     e.Version,
		Stage:       string(e.Stage),
		ContentHash: e.ContentHash,
		Checkpoint:  e.Checkpoint,
		Summary:     e.Summary,
		Reason:      e.Reason,
		Changes:     changes,
		Date:        timestamppb.New(e.Date),
	}
}

func changeLogEntriesToProto(entries []*storage.ChangeLogEntry) []*specv1.ChangeLogEntry {
	pbs := make([]*specv1.ChangeLogEntry, len(entries))
	for i, e := range entries {
		pbs[i] = changeLogEntryToProto(e)
	}
	return pbs
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestChangeLogEntry -v -count=1`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
jj --no-pager describe -m "feat(server): add changelog domain-to-proto converter (spgr-fn5)"
jj --no-pager new -m ""
```

---

### Task 3: Add handler method + permissions

**Files:**

- Modify: `internal/server/spec_handler.go`
- Modify: `internal/auth/permissions.go`

- [ ] **Step 1: Add ListChanges handler**

Add to `internal/server/spec_handler.go` after the `UpdateSpec` method:

```go
// ListChanges handles the ListChanges RPC.
func (h *SpecHandler) ListChanges(ctx context.Context, req *connect.Request[specv1.ListChangesRequest]) (*connect.Response[specv1.ListChangesResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	filter := storage.ChangeLogFilter{
		CheckpointsOnly: msg.CheckpointsOnly,
		SinceVersion:    msg.SinceVersion,
		Limit:           int(msg.Limit),
	}
	entries, err := store.ListChanges(ctx, msg.Slug, filter)
	if err != nil {
		return nil, specError(err)
	}
	return connect.NewResponse(&specv1.ListChangesResponse{
		Entries: changeLogEntriesToProto(entries),
	}), nil
}
```

- [ ] **Step 2: Add permission entry**

In `internal/auth/permissions.go`, add to the `rpcPermissions` map in the `// SpecService` section:

```go
specgraphv1connect.SpecServiceListChangesProcedure: "spec:read",
```

In `internal/auth/permissions_test.go`, add to the `allProcedures` slice in the `// SpecService` section (after `SpecServiceUpdateSpecProcedure`):

```go
specgraphv1connect.SpecServiceListChangesProcedure,
```

- [ ] **Step 3: Verify build + existing tests pass**

Run: `go build ./... && go test ./internal/server/ ./internal/auth/ -count=1`
Expected: Clean build, all tests pass. `TestPermissionTable_Completeness` confirms the new RPC is registered.

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe -m "feat(server): add ListChanges handler to SpecService (spgr-fn5)"
jj --no-pager new -m ""
```

---

### Task 4: Add markdown renderer

**Files:**

- Create: `internal/render/changelog.go`
- Create: `internal/render/changelog_test.go`

- [ ] **Step 1: Write failing renderer test**

Create `internal/render/changelog_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render_test

import (
	"strings"
	"testing"
	"time"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestChanges_WithEntries(t *testing.T) {
	entries := []*specv1.ChangeLogEntry{
		{
			Version:     3,
			Stage:       "shape",
			ContentHash: "a1b2c3d4",
			Checkpoint:  true,
			Summary:     "Refined scope",
			Changes: []*specv1.FieldChange{
				{Field: "intent", OldValue: "Build X", NewValue: "Build X (v2)"},
			},
			Date: timestamppb.New(time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)),
		},
		{
			Version:     2,
			Stage:       "spark",
			ContentHash: "e5f6g7h8",
			Date:        timestamppb.New(time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)),
		},
	}

	out := render.Changes(entries)

	if !strings.Contains(out, "v3") {
		t.Error("missing version 3")
	}
	if !strings.Contains(out, "shape") {
		t.Error("missing stage")
	}
	if !strings.Contains(out, "checkpoint") {
		t.Error("missing checkpoint marker")
	}
	if !strings.Contains(out, "Refined scope") {
		t.Error("missing summary")
	}
	if !strings.Contains(out, "intent") {
		t.Error("missing field change")
	}
	if !strings.Contains(out, "v2") {
		t.Error("missing version 2")
	}
}

func TestChanges_Empty(t *testing.T) {
	out := render.Changes(nil)
	if !strings.Contains(out, "No changelog entries") {
		t.Errorf("expected empty message, got %q", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestChanges -v -count=1`
Expected: FAIL — `render.Changes` not defined

- [ ] **Step 3: Implement renderer**

Create `internal/render/changelog.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Changes renders changelog entries as a markdown timeline.
func Changes(entries []*specv1.ChangeLogEntry) string {
	if len(entries) == 0 {
		return "No changelog entries found.\n"
	}
	var b strings.Builder
	for _, e := range entries {
		header := fmt.Sprintf("## v%d — %s", e.Version, e.Stage)
		if e.Checkpoint {
			header += " (checkpoint)"
		}
		fmt.Fprintln(&b, header)
		date := ""
		if e.Date != nil {
			date = e.Date.AsTime().Format("2006-01-02")
		}
		fmt.Fprintf(&b, "**%s** | Hash: %s\n", date, e.ContentHash)
		if e.Summary != "" {
			fmt.Fprintf(&b, "\n%s\n", e.Summary)
		}
		if len(e.Changes) > 0 {
			fmt.Fprintln(&b)
			headers := []string{"Field", "Old", "New"}
			rows := make([][]string, len(e.Changes))
			for i, c := range e.Changes {
				rows[i] = []string{c.Field, c.OldValue, c.NewValue}
			}
			fmt.Fprint(&b, itemTable(headers, rows))
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/render/ -run TestChanges -v -count=1`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
jj --no-pager describe -m "feat(render): add changelog markdown renderer (spgr-fn5)"
jj --no-pager new -m ""
```

---

### Task 5: Add CLI command

**Files:**

- Create: `cmd/specgraph/changes.go`

- [ ] **Step 1: Create CLI command**

Create `cmd/specgraph/changes.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

var changesCmd = &cobra.Command{
	Use:   "changes <slug>",
	Short: "List changelog entries for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runChanges,
}

var (
	changesCheckpoints  bool
	changesSinceVersion int32
	changesLimit        int32
	changesJSON         bool
)

func init() {
	changesCmd.Flags().BoolVar(&changesCheckpoints, "checkpoints", false, "show only checkpoint entries")
	changesCmd.Flags().Int32Var(&changesSinceVersion, "since-version", 0, "show entries after this version")
	changesCmd.Flags().Int32Var(&changesLimit, "limit", 0, "maximum number of entries (0 = all)")
	changesCmd.Flags().BoolVar(&changesJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(changesCmd)
}

func runChanges(cmd *cobra.Command, args []string) error {
	client, err := newClient(specgraphv1connect.NewSpecServiceClient)
	if err != nil {
		return err
	}

	resp, err := client.ListChanges(cmd.Context(), connect.NewRequest(&specv1.ListChangesRequest{
		Slug:            args[0],
		CheckpointsOnly: changesCheckpoints,
		SinceVersion:    changesSinceVersion,
		Limit:           changesLimit,
	}))
	if err != nil {
		return fmt.Errorf("list changes: %w", err)
	}
	if changesJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.Changes(resp.Msg.Entries))
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: Clean

- [ ] **Step 3: Run task check**

Run: `task check`
Expected: All checks pass

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe -m "feat(cli): add specgraph changes command (spgr-fn5)"
jj --no-pager new -m ""
```

---

### Task 6: Run full verification

- [ ] **Step 1: Run task check**

Run: `task check`
Expected: All pass

- [ ] **Step 2: Run task pr-prep**

Run: `task pr-prep`
Expected: All pass (build, lint, unit tests, integration, e2e)

- [ ] **Step 3: Close bead**

```bash
bd close spgr-fn5 --reason="ListChanges RPC, CLI command, and renderer implemented"
```
