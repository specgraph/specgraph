# SliceService Handler + Decompose Handler Update Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the SliceService ConnectRPC handler so clients can list, get, claim, and complete decomposition slices.

**Architecture:** New `slice_handler.go` implements `SliceServiceHandler` (4 RPCs) using the existing scoper+handler pattern. `ClaimSlice`/`CompleteSlice` storage methods change from `error` to `(*Slice, error)` so the handler returns the updated slice without a re-fetch. Proto conversions added to `convert.go`. Registration in `serve.go`.

**Tech Stack:** Go, ConnectRPC, Memgraph (Cypher), testify

**Spec:** `docs/superpowers/specs/2026-03-26-slice-first-class-vertex-design.md` §6

**Bead:** spgr-6sw.4

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/storage/slice.go` | Change `ClaimSlice`/`CompleteSlice` signatures to return `(*Slice, error)` |
| Modify | `internal/storage/memgraph/slice.go` | Update Cypher RETURN clauses + return `*Slice` |
| Modify | `internal/storage/memgraph/slice_test.go` | Update integration test assertions for new return types |
| Modify | `internal/storage/memgraph/slice_unit_test.go` | Update unit test assertions for new return types |
| Modify | `internal/server/convert.go` | Add `sliceToProto`, `slicesToProto`, status maps |
| Create | `internal/server/slice_handler.go` | SliceHandler struct, 4 RPCs, RegisterSliceService |
| Create | `internal/server/slice_handler_test.go` | Mock backend, test server setup, happy path + error tests |
| Modify | `internal/server/test_scoper_test.go` | Update `stubBackend` ClaimSlice/CompleteSlice signatures |
| Modify | `cmd/specgraph/serve.go` | Add `server.RegisterSliceService(mux, store, opts)` |

---

## Task 1: Change SliceBackend return types

The storage interface and Memgraph implementation need to return `*Slice` from `ClaimSlice`/`CompleteSlice` so handlers avoid a re-fetch.

**Files:**

- Modify: `internal/storage/slice.go:57-59` (interface)
- Modify: `internal/storage/memgraph/slice.go:119-174` (implementation)
- Modify: `internal/storage/memgraph/slice_test.go` (integration tests)
- Modify: `internal/storage/memgraph/slice_unit_test.go` (unit tests)
- Modify: `internal/server/test_scoper_test.go:299-305` (stub signatures)
- Modify: `internal/storage/memgraph/authoring_decompose_test.go` (faultingSliceOps stub)

- [ ] **Step 1: Update the SliceBackend interface**

In `internal/storage/slice.go`, change:

```go
// ClaimSlice transitions a slice to claimed status and records the assignee.
// Returns the updated slice.
ClaimSlice(ctx context.Context, slug, assignee string) (*Slice, error)
// CompleteSlice transitions a slice to done status.
// Returns the updated slice.
CompleteSlice(ctx context.Context, slug string) (*Slice, error)
```

- [ ] **Step 2: Update the Memgraph ClaimSlice implementation**

In `internal/storage/memgraph/slice.go`, change `ClaimSlice` to return all fields from the updated node and call `recordToSlice`:

```go
func (s *Store) ClaimSlice(ctx context.Context, slug, assignee string) (*storage.Slice, error) {
	if strings.TrimSpace(assignee) == "" {
		return nil, fmt.Errorf("memgraph: claim slice %q: assignee must not be blank", slug)
	}
	nowStr := s.now()
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(sl:Slice {slug: $slug})
		WHERE sl.status = $expected_status
		SET sl.status = $new_status, sl.assigned_to = $assignee, sl.updated_at = $now
		RETURN sl.id, sl.slug, sl.parent_slug, sl.slice_id, sl.intent,
		       sl.verify_json, sl.touches_json, sl.depends_on_json, sl.status,
		       sl.assigned_to, sl.created_at, sl.updated_at
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"slug":            slug,
		"expected_status": string(storage.SliceStatusOpen),
		"new_status":      string(storage.SliceStatusClaimed),
		"assignee":        assignee,
		"now":             nowStr,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: claim slice %q: %w", slug, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: claim slice %q: %w", slug, storage.ErrSliceWrongStatus)
	}
	return recordToSlice(records[0])
}
```

- [ ] **Step 3: Update the Memgraph CompleteSlice implementation**

Same pattern — return all fields, call `recordToSlice`:

```go
func (s *Store) CompleteSlice(ctx context.Context, slug string) (*storage.Slice, error) {
	nowStr := s.now()
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(sl:Slice {slug: $slug})
		WHERE sl.status = $expected_status
		SET sl.status = $new_status, sl.updated_at = $now
		RETURN sl.id, sl.slug, sl.parent_slug, sl.slice_id, sl.intent,
		       sl.verify_json, sl.touches_json, sl.depends_on_json, sl.status,
		       sl.assigned_to, sl.created_at, sl.updated_at
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"slug":            slug,
		"expected_status": string(storage.SliceStatusClaimed),
		"new_status":      string(storage.SliceStatusDone),
		"now":             nowStr,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: complete slice %q: %w", slug, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: complete slice %q: %w", slug, storage.ErrSliceWrongStatus)
	}
	return recordToSlice(records[0])
}
```

- [ ] **Step 4: Update stubBackend in test_scoper_test.go**

```go
func (stubBackend) ClaimSlice(context.Context, string, string) (*storage.Slice, error) {
	return nil, errNotImplemented
}

func (stubBackend) CompleteSlice(context.Context, string) (*storage.Slice, error) {
	return nil, errNotImplemented
}
```

- [ ] **Step 5: Update faultingSliceOps in authoring_decompose_test.go**

```go
func (f *faultingSliceOps) ClaimSlice(context.Context, string, string) (*storage.Slice, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *faultingSliceOps) CompleteSlice(context.Context, string) (*storage.Slice, error) {
	return nil, fmt.Errorf("not implemented")
}
```

- [ ] **Step 6: Update integration tests**

In `internal/storage/memgraph/slice_test.go`, update `ClaimSlice`/`CompleteSlice` assertions to capture and verify the returned `*Slice`:

```go
// ClaimSlice returns updated slice
claimed, err := store.ClaimSlice(ctx, slug, "alice")
require.NoError(t, err)
require.Equal(t, storage.SliceStatusClaimed, claimed.Status)
require.Equal(t, "alice", claimed.AssignedTo)

// CompleteSlice returns updated slice
completed, err := store.CompleteSlice(ctx, slug)
require.NoError(t, err)
require.Equal(t, storage.SliceStatusDone, completed.Status)
```

For error cases (wrong status), capture `*Slice`:

```go
sl, err := store.ClaimSlice(ctx, slug, "bob")
require.Nil(t, sl)
require.ErrorIs(t, err, storage.ErrSliceWrongStatus)
```

- [ ] **Step 7: Update unit tests**

In `internal/storage/memgraph/slice_unit_test.go`, update any mock return assertions if they reference ClaimSlice/CompleteSlice return types.

- [ ] **Step 8: Build and test**

Run: `go build ./... && go test ./internal/storage/... && go test ./internal/server/...`

Expected: All pass. The interface change cascades cleanly.

- [ ] **Step 9: Commit**

```text
jj --no-pager describe -m "refactor(storage): ClaimSlice/CompleteSlice return (*Slice, error)

Return updated Slice from mutating methods instead of requiring a
re-fetch. Eliminates wasted DB round trip and TOCTOU window between
write and read.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Proto conversion functions

Add slice domain↔proto conversions to `convert.go`.

**Files:**

- Modify: `internal/server/convert.go` (add conversions at end of file)

- [ ] **Step 1: Write conversion test**

Create tests in a new section of an existing convert test file, or inline in the handler test (Task 3). For now, the handler tests (Task 3) will exercise these conversions. Skip standalone unit tests — the conversions are trivial maps and the handler tests provide integration coverage.

- [ ] **Step 2: Add status maps and conversion functions**

Append to `internal/server/convert.go`:

```go
// --- Slice conversions ---

var sliceStatusToProtoMap = map[storage.SliceStatus]specv1.SliceStatus{
	storage.SliceStatusOpen:    specv1.SliceStatus_SLICE_STATUS_OPEN,
	storage.SliceStatusClaimed: specv1.SliceStatus_SLICE_STATUS_CLAIMED,
	storage.SliceStatusDone:    specv1.SliceStatus_SLICE_STATUS_DONE,
}

func sliceStatusToProto(s storage.SliceStatus) (specv1.SliceStatus, error) {
	if v, ok := sliceStatusToProtoMap[s]; ok {
		return v, nil
	}
	return specv1.SliceStatus_SLICE_STATUS_UNSPECIFIED, fmt.Errorf("unknown slice status: %q", s)
}

func sliceToProto(s *storage.Slice) (*specv1.Slice, error) {
	status, err := sliceStatusToProto(s.Status)
	if err != nil {
		return nil, err
	}
	return &specv1.Slice{
		Id:         s.ID,
		Slug:       s.Slug,
		ParentSlug: s.ParentSlug,
		SliceId:    s.SliceID,
		Intent:     s.Intent,
		Verify:     s.Verify,
		Touches:    s.Touches,
		DependsOn:  s.DependsOn,
		Status:     status,
		AssignedTo: s.AssignedTo,
		CreatedAt:  timeToProto(s.CreatedAt),
		UpdatedAt:  timeToProto(s.UpdatedAt),
	}, nil
}

func slicesToProto(slices []*storage.Slice) ([]*specv1.Slice, error) {
	result := make([]*specv1.Slice, len(slices))
	for i, s := range slices {
		pb, err := sliceToProto(s)
		if err != nil {
			return nil, err
		}
		result[i] = pb
	}
	return result, nil
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/server/...`

Expected: Clean build.

- [ ] **Step 4: Commit**

```text
jj --no-pager describe -m "feat(server): add slice proto conversion functions

sliceToProto, slicesToProto, sliceStatusToProto with bidirectional
status maps following the existing DecisionStatus pattern.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: SliceHandler implementation + tests

The handler and its tests. Tests first (TDD), then implementation.

**Files:**

- Create: `internal/server/slice_handler.go`
- Create: `internal/server/slice_handler_test.go`

- [ ] **Step 1: Write slice_handler_test.go with mock backend and test server**

Create `internal/server/slice_handler_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockSliceBackend struct {
	stubBackend
	mu     sync.Mutex
	slices map[string]*storage.Slice
	seq    int
}

func newMockSliceBackend() *mockSliceBackend {
	return &mockSliceBackend{slices: make(map[string]*storage.Slice)}
}

func (m *mockSliceBackend) CreateSlice(_ context.Context, sl *storage.Slice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	sl.ID = fmt.Sprintf("slc-%05d", m.seq)
	sl.Status = storage.SliceStatusOpen
	sl.CreatedAt = time.Now().UTC()
	sl.UpdatedAt = sl.CreatedAt
	m.slices[sl.Slug] = sl
	return nil
}

func (m *mockSliceBackend) ListSlices(_ context.Context, parentSlug string) ([]*storage.Slice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*storage.Slice
	for _, s := range m.slices {
		if s.ParentSlug == parentSlug {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSliceBackend) GetSlice(_ context.Context, slug string) (*storage.Slice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.slices[slug]
	if !ok {
		return nil, storage.ErrSliceNotFound
	}
	return s, nil
}

func (m *mockSliceBackend) ClaimSlice(_ context.Context, slug, assignee string) (*storage.Slice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.slices[slug]
	if !ok {
		return nil, storage.ErrSliceNotFound
	}
	if s.Status != storage.SliceStatusOpen {
		return nil, storage.ErrSliceWrongStatus
	}
	s.Status = storage.SliceStatusClaimed
	s.AssignedTo = assignee
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

func (m *mockSliceBackend) CompleteSlice(_ context.Context, slug string) (*storage.Slice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.slices[slug]
	if !ok {
		return nil, storage.ErrSliceNotFound
	}
	if s.Status != storage.SliceStatusClaimed {
		return nil, storage.ErrSliceWrongStatus
	}
	s.Status = storage.SliceStatusDone
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

func setupSliceServer(t *testing.T) (specgraphv1connect.SliceServiceClient, *mockSliceBackend) {
	t.Helper()
	mb := newMockSliceBackend()
	scoper := &testScoper{backend: mb}
	mux := http.NewServeMux()
	server.RegisterSliceService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewSliceServiceClient(http.DefaultClient, srv.URL), mb
}

// seed creates a slice in the mock backend for testing.
func seed(t *testing.T, mb *mockSliceBackend, parentSlug, sliceID, intent string) string {
	t.Helper()
	slug := parentSlug + "/" + sliceID
	err := mb.CreateSlice(context.Background(), &storage.Slice{
		Slug:       slug,
		ParentSlug: parentSlug,
		SliceID:    sliceID,
		Intent:     intent,
	})
	require.NoError(t, err)
	return slug
}

func TestSliceHandler_ListSlices(t *testing.T) {
	client, mb := setupSliceServer(t)
	ctx := context.Background()

	seed(t, mb, "parent", "a", "First")
	seed(t, mb, "parent", "b", "Second")
	seed(t, mb, "other", "c", "Other parent")

	resp, err := client.ListSlices(ctx, connect.NewRequest(&specv1.ListSlicesRequest{
		ParentSlug: "parent",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Slices, 2)
}

func TestSliceHandler_ListSlices_MissingParentSlug(t *testing.T) {
	client, _ := setupSliceServer(t)
	_, err := client.ListSlices(context.Background(), connect.NewRequest(&specv1.ListSlicesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSliceHandler_GetSlice(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seed(t, mb, "parent", "a", "Test slice")

	resp, err := client.GetSlice(context.Background(), connect.NewRequest(&specv1.GetSliceRequest{
		Slug: slug,
	}))
	require.NoError(t, err)
	require.Equal(t, "Test slice", resp.Msg.Slice.Intent)
	require.Equal(t, specv1.SliceStatus_SLICE_STATUS_OPEN, resp.Msg.Slice.Status)
}

func TestSliceHandler_GetSlice_NotFound(t *testing.T) {
	client, _ := setupSliceServer(t)
	_, err := client.GetSlice(context.Background(), connect.NewRequest(&specv1.GetSliceRequest{
		Slug: "nonexistent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSliceHandler_ClaimSlice(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seed(t, mb, "parent", "a", "Claimable")

	resp, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug:     slug,
		Assignee: "alice",
	}))
	require.NoError(t, err)
	require.Equal(t, specv1.SliceStatus_SLICE_STATUS_CLAIMED, resp.Msg.Slice.Status)
	require.Equal(t, "alice", resp.Msg.Slice.AssignedTo)
}

func TestSliceHandler_ClaimSlice_WrongStatus(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seed(t, mb, "parent", "a", "Already claimed")
	_, _ = mb.ClaimSlice(context.Background(), slug, "bob")

	_, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug:     slug,
		Assignee: "alice",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestSliceHandler_ClaimSlice_MissingAssignee(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seed(t, mb, "parent", "a", "Missing assignee")

	_, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug: slug,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSliceHandler_CompleteSlice(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seed(t, mb, "parent", "a", "Completable")
	_, _ = mb.ClaimSlice(context.Background(), slug, "alice")

	resp, err := client.CompleteSlice(context.Background(), connect.NewRequest(&specv1.CompleteSliceRequest{
		Slug: slug,
	}))
	require.NoError(t, err)
	require.Equal(t, specv1.SliceStatus_SLICE_STATUS_DONE, resp.Msg.Slice.Status)
}

func TestSliceHandler_CompleteSlice_WrongStatus(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seed(t, mb, "parent", "a", "Not yet claimed")

	_, err := client.CompleteSlice(context.Background(), connect.NewRequest(&specv1.CompleteSliceRequest{
		Slug: slug,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -run TestSliceHandler -v`

Expected: Compilation failure — `RegisterSliceService` doesn't exist yet.

- [ ] **Step 3: Write slice_handler.go**

Create `internal/server/slice_handler.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/storage"
)

// SliceHandler implements the ConnectRPC SliceService.
type SliceHandler struct {
	scoper storage.Scoper
	logger *slog.Logger
}

var _ specgraphv1connect.SliceServiceHandler = (*SliceHandler)(nil)

// ListSlices handles the ListSlices RPC.
func (h *SliceHandler) ListSlices(ctx context.Context, req *connect.Request[specv1.ListSlicesRequest]) (*connect.Response[specv1.ListSlicesResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.GetParentSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	slices, err := store.ListSlices(ctx, req.Msg.ParentSlug)
	if err != nil {
		h.logger.ErrorContext(ctx, "list slices failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := slicesToProto(slices)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ListSlicesResponse{Slices: pb}), nil
}

// GetSlice handles the GetSlice RPC.
func (h *SliceHandler) GetSlice(ctx context.Context, req *connect.Request[specv1.GetSliceRequest]) (*connect.Response[specv1.GetSliceResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	s, err := store.GetSlice(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSliceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("slice not found"))
		}
		h.logger.ErrorContext(ctx, "get slice failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := sliceToProto(s)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetSliceResponse{Slice: pb}), nil
}

// ClaimSlice handles the ClaimSlice RPC.
func (h *SliceHandler) ClaimSlice(ctx context.Context, req *connect.Request[specv1.ClaimSliceRequest]) (*connect.Response[specv1.ClaimSliceResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := validateRequiredField("assignee", msg.GetAssignee()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	s, err := store.ClaimSlice(ctx, msg.Slug, msg.Assignee)
	if err != nil {
		if errors.Is(err, storage.ErrSliceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("slice not found"))
		}
		if errors.Is(err, storage.ErrSliceWrongStatus) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("slice is not in open status"))
		}
		h.logger.ErrorContext(ctx, "claim slice failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := sliceToProto(s)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ClaimSliceResponse{Slice: pb}), nil
}

// CompleteSlice handles the CompleteSlice RPC.
func (h *SliceHandler) CompleteSlice(ctx context.Context, req *connect.Request[specv1.CompleteSliceRequest]) (*connect.Response[specv1.CompleteSliceResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	s, err := store.CompleteSlice(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSliceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("slice not found"))
		}
		if errors.Is(err, storage.ErrSliceWrongStatus) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("slice is not in claimed status"))
		}
		h.logger.ErrorContext(ctx, "complete slice failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := sliceToProto(s)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.CompleteSliceResponse{Slice: pb}), nil
}

// RegisterSliceService registers the SliceService handler on the mux.
func RegisterSliceService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	handler := &SliceHandler{scoper: scoper, logger: slog.Default()}
	path, h := specgraphv1connect.NewSliceServiceHandler(handler, opts...)
	mux.Handle(path, h)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestSliceHandler -v`

Expected: All 8 tests pass.

- [ ] **Step 5: Run full server test suite**

Run: `go test ./internal/server/`

Expected: All pass (including existing tests unaffected by changes).

- [ ] **Step 6: Commit**

```text
jj --no-pager describe -m "feat(server): implement SliceService handler with 4 RPCs (spgr-6sw.4)

ListSlices, GetSlice, ClaimSlice, CompleteSlice following the existing
DecisionHandler pattern. Proto conversions in convert.go. Mock backend
tests cover happy path, not-found, wrong-status, and validation errors.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Register in serve.go + final verification

**Files:**

- Modify: `cmd/specgraph/serve.go:101` (add registration line)

- [ ] **Step 1: Add registration**

In `cmd/specgraph/serve.go`, after the `RegisterExecutionService` line (around line 101), add:

```go
server.RegisterSliceService(mux, store, opts)
```

- [ ] **Step 2: Full build and lint**

Run: `task check`

Expected: All pass (fmt, license, lint, build, tests).

- [ ] **Step 3: Run integration tests**

Run: `task test:integration`

Expected: All pass. The storage interface change cascades through the Memgraph integration tests.

- [ ] **Step 4: Commit**

```text
jj --no-pager describe -m "feat(server): register SliceService in serve.go (spgr-6sw.4)

Wire SliceService into the HTTP mux alongside existing services.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 5: Close bead**

```text
bd close spgr-6sw.4 --reason="SliceService handler implemented, registered, tested"
```
