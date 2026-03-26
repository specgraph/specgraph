# Idempotent Push Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `FindOrCreate` to the sync Adapter interface so adapters search for existing external items before creating new ones, preventing orphaned duplicates.

**Architecture:** Each adapter implements `FindOrCreate` with a title-search-then-create strategy. The sync handler calls `FindOrCreate` instead of `Push`. When an existing item is found, the handler still records the sync mapping to heal orphans.

**Tech Stack:** Go, `gh` CLI (GitHub search), `bd` CLI (beads search), ConnectRPC handler

**Spec:** `docs/superpowers/specs/2026-03-26-idempotent-push-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/sync/adapter.go` | Modify | Add `FindOrCreate` to `Adapter` interface |
| `internal/sync/beads.go` | Modify | Implement `BeadsAdapter.FindOrCreate` |
| `internal/sync/beads_test.go` | Modify | Add sequencing mock, FindOrCreate tests |
| `internal/sync/github.go` | Modify | Implement `GitHubAdapter.FindOrCreate` |
| `internal/sync/github_test.go` | Modify | Add FindOrCreate tests |
| `internal/server/sync_handler.go` | Modify | Call `FindOrCreate` instead of `Push` |
| `internal/server/sync_handler_test.go` | Modify | Add `findOrCreateFn` to mock, add handler tests |

---

## Chunk 1: Interface + Sequencing Mock + BeadsAdapter

### Task 1: Add FindOrCreate to Adapter interface

**Files:**

- Modify: `internal/sync/adapter.go:22-35`

- [ ] **Step 1: Add FindOrCreate to the Adapter interface**

In `internal/sync/adapter.go`, add the `FindOrCreate` method to the `Adapter` interface after `Push`:

```go
// FindOrCreate searches for an existing external item matching this spec.
// If found, returns its ID with created=false.
// If not found, creates a new one (like Push) and returns created=true.
FindOrCreate(ctx context.Context, spec *storage.Spec) (externalID string, created bool, err error)
```

- [ ] **Step 2: Verify build fails**

Run: `go build ./internal/sync/ ./internal/server/`

Expected: Compile errors — `GitHubAdapter`, `BeadsAdapter`, and test mocks don't implement `FindOrCreate` yet.

### Task 2: Add sequencing mock runner

**Files:**

- Modify: `internal/sync/beads_test.go`

The existing `mockRunner` returns a single output/error. `FindOrCreate` makes two sequential CLI calls (search, then optionally create). We need a mock that returns different results per call.

- [ ] **Step 1: Add `sequenceRunner` to `beads_test.go`**

Add after the existing `mockRunner` (around line 23):

```go
// sequenceRunner returns different output/error for each sequential call.
type sequenceRunner struct {
	calls []struct {
		output []byte
		err    error
	}
	idx int
}

func (s *sequenceRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	if s.idx >= len(s.calls) {
		return nil, errors.New("sequenceRunner: unexpected call")
	}
	c := s.calls[s.idx]
	s.idx++
	return c.output, c.err
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/sync/`

Expected: Compile error (FindOrCreate not yet on BeadsAdapter). That's expected — we add it in Task 3.

### Task 3: Implement BeadsAdapter.FindOrCreate

**Files:**

- Modify: `internal/sync/beads.go`
- Modify: `internal/sync/beads_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/sync/beads_test.go`:

```go
func TestBeadsAdapter_FindOrCreate_ExistingItem(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: []byte(`[{"id": "bead-existing"}]`), err: nil}, // search finds it
		},
	}
	b := NewBeadsAdapter(runner)
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP2,
	}
	id, created, err := b.FindOrCreate(context.Background(), spec)
	if err != nil {
		t.Fatalf("FindOrCreate() unexpected error: %v", err)
	}
	if created {
		t.Error("FindOrCreate() created = true, want false")
	}
	if id != "bead-existing" {
		t.Errorf("FindOrCreate() id = %q, want %q", id, "bead-existing")
	}
}

func TestBeadsAdapter_FindOrCreate_NewItem(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: []byte(`[]`), err: nil},                                        // search finds nothing
			{output: []byte(`{"id": "bead-new123"}`), err: nil},                     // create succeeds
		},
	}
	b := NewBeadsAdapter(runner)
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP2,
	}
	id, created, err := b.FindOrCreate(context.Background(), spec)
	if err != nil {
		t.Fatalf("FindOrCreate() unexpected error: %v", err)
	}
	if !created {
		t.Error("FindOrCreate() created = false, want true")
	}
	if id != "bead-new123" {
		t.Errorf("FindOrCreate() id = %q, want %q", id, "bead-new123")
	}
}

func TestBeadsAdapter_FindOrCreate_SearchError(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: nil, err: errors.New("bd search failed")}, // search errors
		},
	}
	b := NewBeadsAdapter(runner)
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP2,
	}
	_, _, err := b.FindOrCreate(context.Background(), spec)
	if err == nil {
		t.Fatal("FindOrCreate() expected error, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/sync/ -run TestBeadsAdapter_FindOrCreate -v`

Expected: Compile error — `FindOrCreate` not defined on `BeadsAdapter`.

- [ ] **Step 3: Implement BeadsAdapter.FindOrCreate**

Add to `internal/sync/beads.go` after the `Push` method:

```go
// beadsSearchResponse captures one entry from bd search --json output.
type beadsSearchResponse struct {
	ID string `json:"id"`
}

// FindOrCreate searches for an existing bead matching "[spec] <slug>".
// If found, returns its ID with created=false.
// If not found, creates via Push and returns created=true.
func (b *BeadsAdapter) FindOrCreate(ctx context.Context, spec *storage.Spec) (string, bool, error) {
	if spec.Slug == "" {
		return "", false, fmt.Errorf("%w: spec slug is required", errPushFailed)
	}

	searchTitle := fmt.Sprintf("[spec] %s", spec.Slug)
	out, err := b.runner.Run(ctx, "bd", "search", searchTitle, "--json", "--limit", "1")
	if err != nil {
		return "", false, fmt.Errorf("failed to search for existing bead: %w", err)
	}

	var results []beadsSearchResponse
	if err := json.Unmarshal(out, &results); err != nil {
		return "", false, fmt.Errorf("failed to parse search results: %w", err)
	}

	if len(results) > 0 && results[0].ID != "" {
		return results[0].ID, false, nil
	}

	externalID, pushErr := b.Push(ctx, spec)
	if pushErr != nil {
		return "", false, pushErr
	}
	return externalID, true, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/sync/ -run TestBeadsAdapter_FindOrCreate -v`

Expected: All 3 tests PASS.

- [ ] **Step 5: Run full sync package tests**

Run: `go test ./internal/sync/ -v`

Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(sync): add FindOrCreate to Adapter interface, implement for BeadsAdapter (spgr-ylq)"
```

---

## Chunk 2: GitHubAdapter.FindOrCreate

### Task 4: Implement GitHubAdapter.FindOrCreate

**Files:**

- Modify: `internal/sync/github.go`
- Modify: `internal/sync/github_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/sync/github_test.go`:

```go
func TestGitHubAdapter_FindOrCreate_ExistingItem(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: []byte(`[{"number":42,"url":"https://github.com/owner/repo/issues/42"}]`), err: nil},
		},
	}
	g := NewGitHubAdapter(runner, "owner/repo")
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP1,
	}
	id, created, err := g.FindOrCreate(context.Background(), spec)
	if err != nil {
		t.Fatalf("FindOrCreate() unexpected error: %v", err)
	}
	if created {
		t.Error("FindOrCreate() created = true, want false")
	}
	if id != "https://github.com/owner/repo/issues/42" {
		t.Errorf("FindOrCreate() id = %q, want URL", id)
	}
}

func TestGitHubAdapter_FindOrCreate_NewItem(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: []byte(`[]`), err: nil},                                                  // search empty
			{output: []byte("https://github.com/owner/repo/issues/99\n"), err: nil},           // create
		},
	}
	g := NewGitHubAdapter(runner, "owner/repo")
	spec := &storage.Spec{
		Slug:     "new-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP1,
	}
	id, created, err := g.FindOrCreate(context.Background(), spec)
	if err != nil {
		t.Fatalf("FindOrCreate() unexpected error: %v", err)
	}
	if !created {
		t.Error("FindOrCreate() created = false, want true")
	}
	if id != "https://github.com/owner/repo/issues/99" {
		t.Errorf("FindOrCreate() id = %q, want URL", id)
	}
}

func TestGitHubAdapter_FindOrCreate_SearchError(t *testing.T) {
	runner := &sequenceRunner{
		calls: []struct {
			output []byte
			err    error
		}{
			{output: nil, err: errors.New("gh search failed")},
		},
	}
	g := NewGitHubAdapter(runner, "owner/repo")
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Build a thing",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriorityP1,
	}
	_, _, err := g.FindOrCreate(context.Background(), spec)
	if err == nil {
		t.Fatal("FindOrCreate() expected error, got nil")
	}
}
```

Note: `sequenceRunner` is defined in `beads_test.go` and is accessible since both files are in the `sync` package.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/sync/ -run TestGitHubAdapter_FindOrCreate -v`

Expected: Compile error — `FindOrCreate` not defined on `GitHubAdapter`.

- [ ] **Step 3: Implement GitHubAdapter.FindOrCreate**

Add to `internal/sync/github.go` after the `Push` method:

```go
// ghSearchResult captures one entry from gh issue list --json output.
type ghSearchResult struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// FindOrCreate searches for an existing GitHub issue matching "[spec] <slug>".
// If found, returns its URL with created=false.
// If not found, creates via Push and returns created=true.
func (g *GitHubAdapter) FindOrCreate(ctx context.Context, spec *storage.Spec) (string, bool, error) {
	if spec.Slug == "" {
		return "", false, fmt.Errorf("%w: spec slug is required", errPushFailed)
	}
	if g.repo == "" {
		return "", false, fmt.Errorf("%w: repo is required", errPushFailed)
	}

	searchTitle := fmt.Sprintf("in:title [spec] %s", spec.Slug)
	out, err := g.runner.Run(ctx, "gh", "issue", "list",
		"--search", searchTitle,
		"--repo", g.repo,
		"--json", "number,url",
		"--limit", "1",
	)
	if err != nil {
		return "", false, fmt.Errorf("failed to search for existing issue: %w", err)
	}

	var results []ghSearchResult
	if err := json.Unmarshal(out, &results); err != nil {
		return "", false, fmt.Errorf("failed to parse search results: %w", err)
	}

	if len(results) > 0 && results[0].URL != "" {
		return results[0].URL, false, nil
	}

	externalID, pushErr := g.Push(ctx, spec)
	if pushErr != nil {
		return "", false, pushErr
	}
	return externalID, true, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/sync/ -run TestGitHubAdapter_FindOrCreate -v`

Expected: All 3 tests PASS.

- [ ] **Step 5: Run full sync package tests**

Run: `go test ./internal/sync/ -v`

Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(sync): implement GitHubAdapter.FindOrCreate with title search (spgr-ylq)"
```

---

## Chunk 3: Handler + Mock Update + Integration Tests

### Task 5: Update mock adapter and sync handler

**Files:**

- Modify: `internal/server/sync_handler.go:145`
- Modify: `internal/server/sync_handler_test.go` (mockAdapter around line 382)

- [ ] **Step 1: Add `findOrCreateFn` to mockAdapter**

In `internal/server/sync_handler_test.go`, update the `mockAdapter` struct (around line 382):

```go
type mockAdapter struct {
	name            storage.SyncAdapterType
	pushFn          func(ctx context.Context, spec *storage.Spec) (string, error)
	findOrCreateFn  func(ctx context.Context, spec *storage.Spec) (string, bool, error)
	available       bool
}
```

Add the `FindOrCreate` method:

```go
func (m *mockAdapter) FindOrCreate(ctx context.Context, spec *storage.Spec) (string, bool, error) {
	if m.findOrCreateFn != nil {
		return m.findOrCreateFn(ctx, spec)
	}
	// Fall back to Push for backward compatibility with existing tests.
	id, err := m.pushFn(ctx, spec)
	return id, true, err
}
```

- [ ] **Step 2: Verify existing tests still pass**

Run: `go test ./internal/server/ -run TestSyncHandler -v -count=1`

Expected: All existing sync handler tests PASS (the fallback to `pushFn` preserves compatibility).

- [ ] **Step 3: Update sync handler to call FindOrCreate**

In `internal/server/sync_handler.go`, replace line 145:

```go
externalID, pushErr := adapter.Push(ctx, spec)
```

With:

```go
externalID, created, pushErr := adapter.FindOrCreate(ctx, spec)
```

And update the success log/message at line 202. Replace:

```go
result.Message = "synced"
```

With:

```go
if created {
	result.Message = "synced"
} else {
	result.Message = "synced (recovered existing external item)"
}
```

- [ ] **Step 4: Add handler test for orphan recovery**

Add to `internal/server/sync_handler_test.go`:

```go
func TestSyncHandler_SyncBeads_FindOrCreate_Recovery(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		findOrCreateFn: func(_ context.Context, spec *storage.Spec) (string, bool, error) {
			return "beads-recovered-" + spec.Slug, false, nil // found existing, not created
		},
	}
	client := setupSyncServerWithAdapter(t, adapter)
	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Synced)
	require.Len(t, resp.Msg.Results, 1)
	require.Equal(t, specv1.SyncState_SYNC_STATE_SYNCED, resp.Msg.Results[0].State)
	require.Contains(t, resp.Msg.Results[0].Message, "recovered")
}
```

- [ ] **Step 5: Run all sync handler tests**

Run: `go test ./internal/server/ -run TestSyncHandler -v -count=1`

Expected: All tests PASS including the new recovery test.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(sync): handler calls FindOrCreate instead of Push, orphan recovery (spgr-ylq)"
```

---

## Chunk 4: Final Verification + PR

### Task 6: Verify and create PR

- [ ] **Step 1: Run task check**

```bash
task check
```

Expected: All checks pass (fmt, lint, build, test).

- [ ] **Step 2: Squash changes**

If working on separate jj changes, squash into one:

```bash
jj --no-pager squash --into <first-change-id> -m "feat(sync): idempotent push — FindOrCreate to prevent orphaned external items (spgr-ylq)"
```

- [ ] **Step 3: Create bookmark and push**

```bash
jj --no-pager bookmark set feat/idempotent-push -r @
jj --no-pager git push --bookmark feat/idempotent-push
```

- [ ] **Step 4: Create PR**

```bash
gh pr create \
  --title "feat(sync): idempotent push — FindOrCreate to prevent orphaned external items (spgr-ylq)" \
  --body "$(cat <<'EOF'
## Summary
- Add `FindOrCreate` method to sync `Adapter` interface
- GitHubAdapter searches by `[spec] <slug>` title via `gh issue list --search`
- BeadsAdapter searches by title via `bd search`
- Sync handler calls `FindOrCreate` instead of `Push`, healing orphaned mappings
- Existing retry logic on `CreateSyncMapping` unchanged

Closes #166

## Test plan
- [ ] Unit tests for FindOrCreate on both adapters (found, not-found, search-error)
- [ ] Handler test for orphan recovery path (found existing, mapping created)
- [ ] All existing sync tests pass unchanged

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```
