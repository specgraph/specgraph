# Impact Notification Service Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add post-commit change notifications so subscribers (starting with an impact logger) are notified when a spec materially changes.

**Architecture:** Events stashed via context during transactions, fired to registered subscribers after commit. Subscribers registered on a shared struct visible to both root and scoped stores. First subscriber logs impacted specs via `GetImpact`.

**Tech Stack:** Go, context-based event stashing, `slog` for impact logging

**Spec:** `docs/superpowers/specs/2026-03-30-impact-notification-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/storage/change_event.go` | New. `ChangeEvent`, `ChangeSubscriber`, `Subscribable` interfaces, context stash helpers |
| `internal/storage/change_event_test.go` | New. Stash/drain round-trip tests |
| `internal/storage/scoper.go` | Modify. Compose `Subscribable` into `Scoper` |
| `internal/storage/memgraph/memgraph.go` | Modify. Add `shared` struct with subscribers, update `New()` and `Scoped()` |
| `internal/storage/memgraph/changelog.go` | Modify. `createChangeLog` stashes `ChangeEvent` |
| `internal/storage/memgraph/tx.go` | Modify. `RunInTransaction` inits event context and dispatches after commit |
| `internal/notify/impact.go` | New. `ImpactLogger` subscriber |
| `internal/notify/impact_test.go` | New. Unit test with mock |
| `cmd/specgraph/serve.go` | Modify. Wire subscriber at startup |

---

## Chunk 1: Event Types + Stashing + Store Plumbing + Dispatch + Subscriber + Wiring

### Task 1: Add ChangeEvent types, Subscribable interface, and context stash helpers

**Files:**

- Create: `internal/storage/change_event.go`
- Create: `internal/storage/change_event_test.go`
- Modify: `internal/storage/scoper.go`

- [ ] **Step 1: Write failing stash/drain tests**

Create `internal/storage/change_event_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
)

func TestStashAndDrainChangeEvents(t *testing.T) {
	ctx := storage.InitChangeEvents(context.Background())
	storage.StashChangeEvent(ctx, storage.ChangeEvent{Slug: "spec-a", Version: 1})
	storage.StashChangeEvent(ctx, storage.ChangeEvent{Slug: "spec-b", Version: 2})

	events := storage.DrainChangeEvents(ctx)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Slug != "spec-a" {
		t.Errorf("events[0].Slug = %q, want spec-a", events[0].Slug)
	}
	if events[1].Slug != "spec-b" {
		t.Errorf("events[1].Slug = %q, want spec-b", events[1].Slug)
	}
}

func TestDrainChangeEvents_NoInit(t *testing.T) {
	events := storage.DrainChangeEvents(context.Background())
	if len(events) != 0 {
		t.Fatalf("got %d events from un-initialized context, want 0", len(events))
	}
}

func TestStashChangeEvent_NoInit(t *testing.T) {
	// Should not panic on un-initialized context.
	storage.StashChangeEvent(context.Background(), storage.ChangeEvent{Slug: "x"})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/storage/ -run TestStash -v -count=1`
Expected: FAIL — functions don't exist

- [ ] **Step 3: Implement change_event.go**

Create `internal/storage/change_event.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "context"

// ChangeEvent is emitted after a changelog entry is persisted.
type ChangeEvent struct {
	Slug        string
	Version     int32
	Stage       SpecStage
	ContentHash string
	Checkpoint  bool
	Summary     string
	Reason      string
}

// ChangeSubscriber receives notifications after spec changes are committed.
type ChangeSubscriber interface {
	OnSpecChanged(ctx context.Context, event *ChangeEvent)
}

// Subscribable is implemented by storage backends that support change notifications.
type Subscribable interface {
	Subscribe(ChangeSubscriber)
}

// changeEventsKey is the context key for stashed change events.
type changeEventsKey struct{}

// InitChangeEvents returns a new context with an empty event slice for stashing.
// Called once at the start of RunInTransaction.
func InitChangeEvents(ctx context.Context) context.Context {
	events := make([]ChangeEvent, 0, 4)
	return context.WithValue(ctx, changeEventsKey{}, &events)
}

// StashChangeEvent appends an event to the context's event slice.
// No-op if the context has no event slice (non-transactional path).
func StashChangeEvent(ctx context.Context, event ChangeEvent) {
	ptr, ok := ctx.Value(changeEventsKey{}).(*[]ChangeEvent)
	if !ok || ptr == nil {
		return
	}
	*ptr = append(*ptr, event)
}

// DrainChangeEvents returns all stashed events from the context.
// Returns nil if no events were stashed.
func DrainChangeEvents(ctx context.Context) []ChangeEvent {
	ptr, ok := ctx.Value(changeEventsKey{}).(*[]ChangeEvent)
	if !ok || ptr == nil {
		return nil
	}
	return *ptr
}
```

- [ ] **Step 4: Add Subscribable to Scoper**

In `internal/storage/scoper.go`, add `Subscribable` to the `Scoper` interface:

```go
// Scoper creates project-scoped storage instances.
type Scoper interface {
	Scoped(ctx context.Context, project string) (ScopedBackend, error)
	Subscribable
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/storage/ -run TestStash -v -count=1`
Expected: All PASS

- [ ] **Step 6: Verify build (will fail until Store implements Subscribe)**

Run: `go build ./...`
Expected: Compile error — `Store` doesn't implement `Subscribable`. This is expected and fixed in Task 2.

- [ ] **Step 7: Commit**

```bash
jj --no-pager describe -m "feat(storage): add ChangeEvent types, Subscribable interface, context stash helpers (spgr-w6o)"
jj --no-pager new -m ""
```

---

### Task 2: Add shared subscribers to Store, implement Subscribe, update Scoped

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go`

- [ ] **Step 1: Add shared struct and Subscribe method**

In `internal/storage/memgraph/memgraph.go`, add a `sharedState` struct and modify `Store`:

After the `Store` struct definition (line 46), add:

```go
// sharedState holds state shared between root and scoped Store instances.
type sharedState struct {
	subscribers []storage.ChangeSubscriber
}
```

Add a `shared` field to the `Store` struct:

```go
type Store struct {
	driver     neo4j.DriverWithContext
	nowFunc    func() time.Time
	sliceOps   storage.SliceBackend
	project    string
	ownsDriver bool
	username   string
	password   string
	useTLS     bool
	shared     *sharedState
}
```

In `New()` (the constructor), initialize `shared`:

Find the line where the Store is constructed and add `shared: &sharedState{}` to the struct literal.

In `Scoped()` (line 176), add `shared` to the scoped store:

```go
scoped := &Store{driver: s.driver, nowFunc: s.nowFunc, project: project, shared: s.shared}
```

Add `Subscribe` method:

```go
// Subscribe registers a subscriber for change notifications.
// Must be called before any writes (at startup). Not goroutine-safe.
func (s *Store) Subscribe(sub storage.ChangeSubscriber) {
	s.shared.subscribers = append(s.shared.subscribers, sub)
}
```

Add the compile-time assertion:

```go
var _ storage.Subscribable = (*Store)(nil)
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: Clean (Store now implements Subscribable)

- [ ] **Step 3: Run existing tests**

Run: `go test ./internal/storage/memgraph/ -count=1 -short`
Expected: All PASS (shared field is nil-safe — no subscribers means no dispatch)

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe -m "feat(memgraph): add shared subscribers to Store, implement Subscribe (spgr-w6o)"
jj --no-pager new -m ""
```

---

### Task 3: Stash events in createChangeLog, dispatch after commit in RunInTransaction

**Files:**

- Modify: `internal/storage/memgraph/changelog.go`
- Modify: `internal/storage/memgraph/tx.go`

- [ ] **Step 1: Stash ChangeEvent in createChangeLog**

In `internal/storage/memgraph/changelog.go`, at the end of `createChangeLog` (after the successful query execution and before the `return nil`), add:

```go
	storage.StashChangeEvent(ctx, storage.ChangeEvent{
		Slug:        slug,
		Version:     entry.Version,
		Stage:       entry.Stage,
		ContentHash: entry.ContentHash,
		Checkpoint:  entry.Checkpoint,
		Summary:     entry.Summary,
		Reason:      entry.Reason,
	})
```

Add `"github.com/specgraph/specgraph/internal/storage"` to imports if not already present (it likely is).

- [ ] **Step 2: Add event init and dispatch to RunInTransaction**

In `internal/storage/memgraph/tx.go`, modify `RunInTransaction`:

1. After the nested-tx short circuit (line 74), init the event context:

```go
	ctx = storage.InitChangeEvents(ctx)
```

2. After the successful `session.ExecuteWrite` (after line 93, before `return nil`), add dispatch:

```go
	s.dispatchChangeEvents(ctx)
```

3. Add the dispatch method:

```go
// dispatchChangeEvents fires stashed change events to all registered subscribers.
// Called after successful commit. Each subscriber is isolated with panic recovery.
func (s *Store) dispatchChangeEvents(ctx context.Context) {
	if s.shared == nil {
		return
	}
	events := storage.DrainChangeEvents(ctx)
	if len(events) == 0 {
		return
	}
	for _, sub := range s.shared.subscribers {
		for _, event := range events {
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("change subscriber panicked",
							"subscriber", fmt.Sprintf("%T", sub),
							"slug", event.Slug,
							"panic", r,
						)
					}
				}()
				sub.OnSpecChanged(ctx, event)
			}()
		}
	}
}
```

Add `"log/slog"` to the imports in tx.go.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Clean

- [ ] **Step 4: Run existing tests**

Run: `go test ./internal/storage/memgraph/ -count=1 -short`
Expected: All PASS (no subscribers registered, so dispatch is a no-op)

- [ ] **Step 5: Commit**

```bash
jj --no-pager describe -m "feat(memgraph): stash events in createChangeLog, dispatch after commit (spgr-w6o)"
jj --no-pager new -m ""
```

---

### Task 4: Add GraphBackend to dispatch context, implement ImpactLogger

**Files:**

- Modify: `internal/storage/change_event.go` (add context helpers for GraphBackend)
- Modify: `internal/storage/memgraph/tx.go` (inject scoped store into dispatch context)
- Create: `internal/notify/impact.go`
- Create: `internal/notify/impact_test.go`

The `ImpactLogger` needs a scoped store for `GetImpact`, but subscribers are registered at startup on the root store (no project scope). The dispatch loop runs on the scoped store. Solution: `dispatchChangeEvents` injects the scoped store into context via `WithGraphBackend(ctx, s)`, and `ImpactLogger` extracts it via `GraphBackendFromContext(ctx)`. Stateless subscriber — no constructor dependencies.

- [ ] **Step 1: Add GraphBackend context helpers to change_event.go**

Append to `internal/storage/change_event.go`:

```go
type graphBackendKey struct{}

// WithGraphBackend returns a context carrying the given GraphBackend.
func WithGraphBackend(ctx context.Context, g GraphBackend) context.Context {
	return context.WithValue(ctx, graphBackendKey{}, g)
}

// GraphBackendFromContext extracts a GraphBackend from context, if present.
func GraphBackendFromContext(ctx context.Context) (GraphBackend, bool) {
	g, ok := ctx.Value(graphBackendKey{}).(GraphBackend)
	return g, ok
}
```

- [ ] **Step 2: Update dispatchChangeEvents to inject scoped store**

In `internal/storage/memgraph/tx.go`, update `dispatchChangeEvents` to wrap context:

```go
func (s *Store) dispatchChangeEvents(ctx context.Context) {
	if s.shared == nil {
		return
	}
	events := storage.DrainChangeEvents(ctx)
	if len(events) == 0 {
		return
	}
	notifyCtx := storage.WithGraphBackend(ctx, s)
	for _, sub := range s.shared.subscribers {
		for _, event := range events {
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("change subscriber panicked",
							"subscriber", fmt.Sprintf("%T", sub),
							"slug", event.Slug,
							"panic", r,
						)
					}
				}()
				sub.OnSpecChanged(notifyCtx, event)
			}()
		}
	}
}
```

- [ ] **Step 3: Write failing ImpactLogger test**

Create `internal/notify/impact_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package notify_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/notify"
	"github.com/specgraph/specgraph/internal/storage"
)

type mockGraphBackend struct {
	refs []storage.NodeRef
	err  error
}

func (m *mockGraphBackend) GetImpact(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return m.refs, m.err
}

func TestImpactLogger_LogsImpactedSpecs(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	mock := &mockGraphBackend{
		refs: []storage.NodeRef{
			{Slug: "downstream-a"},
			{Slug: "downstream-b"},
		},
	}

	ctx := storage.WithGraphBackend(context.Background(), mock)
	sub := notify.NewImpactLogger()
	sub.OnSpecChanged(ctx, storage.ChangeEvent{Slug: "upstream-spec", Version: 3})

	out := buf.String()
	if !strings.Contains(out, "upstream-spec") {
		t.Errorf("log missing slug, got: %s", out)
	}
	if !strings.Contains(out, "downstream-a") {
		t.Errorf("log missing impacted spec, got: %s", out)
	}
}

func TestImpactLogger_NoImpact(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	mock := &mockGraphBackend{refs: nil}
	ctx := storage.WithGraphBackend(context.Background(), mock)
	sub := notify.NewImpactLogger()
	sub.OnSpecChanged(ctx, storage.ChangeEvent{Slug: "isolated-spec", Version: 1})

	out := buf.String()
	if !strings.Contains(out, "isolated-spec") {
		t.Errorf("log missing slug, got: %s", out)
	}
	if !strings.Contains(out, "impacted_count=0") {
		t.Errorf("expected impacted_count=0, got: %s", out)
	}
}

func TestImpactLogger_NoGraphBackend(t *testing.T) {
	sub := notify.NewImpactLogger()
	// Should not panic when no GraphBackend in context.
	sub.OnSpecChanged(context.Background(), storage.ChangeEvent{Slug: "x"})
}
```

- [ ] **Step 4: Implement ImpactLogger**

Create `internal/notify/impact.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package notify provides subscribers for storage change events.
package notify

import (
	"context"
	"log/slog"

	"github.com/specgraph/specgraph/internal/storage"
)

// ImpactLogger logs which specs are impacted when an upstream spec changes.
// Stateless — extracts the scoped GraphBackend from context at dispatch time.
type ImpactLogger struct{}

// NewImpactLogger creates an ImpactLogger.
func NewImpactLogger() *ImpactLogger {
	return &ImpactLogger{}
}

// OnSpecChanged implements storage.ChangeSubscriber.
func (l *ImpactLogger) OnSpecChanged(ctx context.Context, event *storage.ChangeEvent) {
	graph, ok := storage.GraphBackendFromContext(ctx)
	if !ok {
		return
	}

	refs, err := graph.GetImpact(ctx, event.Slug)
	if err != nil {
		slog.Warn("impact analysis failed",
			"slug", event.Slug,
			"error", err.Error(),
		)
		return
	}

	slugs := make([]string, len(refs))
	for i, r := range refs {
		slugs[i] = r.Slug
	}

	slog.Info("spec change impact",
		"slug", event.Slug,
		"version", event.Version,
		"impacted_count", len(refs),
		"impacted", slugs,
	)
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/notify/ -v -count=1`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(notify): add ImpactLogger change subscriber with context-based graph access (spgr-w6o)"
jj --no-pager new -m ""
```

---

### Task 5: Wire subscriber in serve.go and verify

**Files:**

- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Wire ImpactLogger in serve.go**

In `cmd/specgraph/serve.go`, after the `store.Close` defer block (around line 92), add:

```go
		// Register change notification subscribers.
		store.Subscribe(notify.NewImpactLogger())
```

Add import: `"github.com/specgraph/specgraph/internal/notify"`

- [ ] **Step 2: Run task check**

Run: `task check`
Expected: All pass

- [ ] **Step 3: Run task pr-prep**

Run: `task pr-prep`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe -m "feat(serve): wire ImpactLogger change subscriber (spgr-w6o)"
jj --no-pager new -m ""
```

- [ ] **Step 5: Close bead**

```bash
bd close spgr-w6o --reason="Impact notification service: post-commit event dispatch, context-based stashing, ImpactLogger subscriber"
```
