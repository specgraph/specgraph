// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// fakeLifecycleBackend is a minimal fake implementation of storage.LifecycleBackend for testing.
type fakeLifecycleBackend struct {
	amendSpec        func(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error)
	supersedeSpec    func(ctx context.Context, oldSlug, newSlug string) (*storage.Spec, *storage.Spec, error)
	abandonSpec      func(ctx context.Context, slug, reason string) (*storage.Spec, error)
	acknowledgeDrift func(ctx context.Context, slug, note string) (*storage.DriftReport, error)
}

func (f *fakeLifecycleBackend) LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
	return f.amendSpec(ctx, slug, reason, reEntryStage)
}

func (f *fakeLifecycleBackend) LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*storage.Spec, *storage.Spec, error) {
	return f.supersedeSpec(ctx, oldSlug, newSlug)
}

func (f *fakeLifecycleBackend) LifecycleAbandonSpec(ctx context.Context, slug, reason string) (*storage.Spec, error) {
	return f.abandonSpec(ctx, slug, reason)
}

func (f *fakeLifecycleBackend) LifecycleAcknowledgeDrift(ctx context.Context, slug, note string) (*storage.DriftReport, error) {
	return f.acknowledgeDrift(ctx, slug, note)
}

// fakeDriftChecker implements server.DriftChecker for testing.
type fakeDriftChecker struct {
	check func(ctx context.Context, slug, scope string) ([]storage.DriftReport, error)
}

func (f *fakeDriftChecker) Check(ctx context.Context, slug, scope string) ([]storage.DriftReport, error) {
	return f.check(ctx, slug, scope)
}

// fakeLinter implements server.SpecLinter for testing.
type fakeLinter struct {
	lint func(ctx context.Context, slug string) ([]storage.LintResult, error)
}

func (f *fakeLinter) Lint(ctx context.Context, slug string) ([]storage.LintResult, error) {
	return f.lint(ctx, slug)
}

type lifecycleTestDeps struct {
	store  *fakeLifecycleBackend
	drift  *fakeDriftChecker
	linter *fakeLinter
}

func defaultTestDeps() *lifecycleTestDeps {
	return &lifecycleTestDeps{
		store:  &fakeLifecycleBackend{},
		drift:  &fakeDriftChecker{},
		linter: &fakeLinter{},
	}
}

func newLifecycleClient(t *testing.T, deps *lifecycleTestDeps) specgraphv1connect.LifecycleServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, deps.store, deps.drift, deps.linter)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)
}

func TestLifecycleHandler_Amend(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, slug, _, _ string) (*storage.Spec, error) {
		return &storage.Spec{
			Slug:    slug,
			Stage:   storage.SpecStageAmended,
			Version: 2,
			History: []storage.HistoryEntry{
				{Version: 2, Stage: storage.SpecStageAmended, Summary: "amended", Reason: "needs rework", Date: now},
			},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "my-spec",
		Reason:       "needs rework",
		ReEntryStage: "shape",
	}))
	require.NoError(t, err)
	s := resp.Msg.GetSpec()
	require.Equal(t, "my-spec", s.GetSlug())
	require.Equal(t, "amended", s.GetStage())
	require.Equal(t, int32(2), s.GetVersion())
	require.Len(t, s.GetHistory(), 1)
	require.Equal(t, "needs rework", s.GetHistory()[0].GetReason())
}

func TestLifecycleHandler_Amend_NotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecNotFound
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "no-such-spec",
		Reason:       "rework",
		ReEntryStage: "shape",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_Amend_NotDone(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecNotDone
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "in-progress-spec",
		Reason:       "rework",
		ReEntryStage: "shape",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_Amend_InvalidReEntryStage(t *testing.T) {
	deps := defaultTestDeps()
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "my-spec",
		Reason:       "rework",
		ReEntryStage: "invalid-stage",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Supersede(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.supersedeSpec = func(_ context.Context, oldSlug, newSlug string) (*storage.Spec, *storage.Spec, error) {
		return &storage.Spec{
				Slug:         oldSlug,
				Stage:        storage.SpecStageSuperseded,
				SupersededBy: newSlug,
				Version:      3,
			}, &storage.Spec{
				Slug:       newSlug,
				Stage:      storage.SpecStageSpark,
				Supersedes: oldSlug,
				Version:    1,
			}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "old-spec",
		NewSlug: "new-spec",
	}))
	require.NoError(t, err)
	require.Equal(t, "old-spec", resp.Msg.OldSpec.Slug)
	require.Equal(t, "superseded", resp.Msg.OldSpec.Stage)
	require.Equal(t, "new-spec", resp.Msg.OldSpec.SupersededBy)
	require.Equal(t, "new-spec", resp.Msg.NewSpec.Slug)
	require.Equal(t, "old-spec", resp.Msg.NewSpec.Supersedes)
}

func TestLifecycleHandler_Abandon(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.abandonSpec = func(_ context.Context, slug, _ string) (*storage.Spec, error) {
		return &storage.Spec{
			Slug:    slug,
			Stage:   storage.SpecStageAbandoned,
			Version: 2,
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   "dead-spec",
		Reason: "no longer needed",
	}))
	require.NoError(t, err)
	s := resp.Msg.GetSpec()
	require.Equal(t, "dead-spec", s.GetSlug())
	require.Equal(t, "abandoned", s.GetStage())
}

func TestLifecycleHandler_CheckDrift(t *testing.T) {
	deps := defaultTestDeps()
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return []storage.DriftReport{
			{
				SpecSlug: "my-spec",
				Items: []storage.DriftItem{
					{
						Type:            storage.DriftTypeDependency,
						Severity:        storage.DriftSeverityHigh,
						Description:     "upstream changed",
						SpecSlug:        "my-spec",
						UpstreamSlug:    "upstream-spec",
						ExpectedVersion: 2,
						ActualVersion:   3,
					},
				},
			},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Reports, 1)
	require.Equal(t, "my-spec", resp.Msg.Reports[0].SpecSlug)
	require.Len(t, resp.Msg.Reports[0].Items, 1)
	require.Equal(t, specv1.DriftType_DRIFT_TYPE_DEPENDENCY, resp.Msg.Reports[0].Items[0].Type)
	require.Equal(t, specv1.DriftSeverity_DRIFT_SEVERITY_HIGH, resp.Msg.Reports[0].Items[0].Severity)
}

func TestLifecycleHandler_CheckDrift_AllSpecs(t *testing.T) {
	deps := defaultTestDeps()
	deps.drift.check = func(_ context.Context, slug, _ string) ([]storage.DriftReport, error) {
		require.Empty(t, slug, "empty slug means check all specs")
		return []storage.DriftReport{
			{SpecSlug: "spec-a"},
			{SpecSlug: "spec-b"},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Reports, 2)
}

func TestLifecycleHandler_AcknowledgeDrift(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, slug, note string) (*storage.DriftReport, error) {
		return &storage.DriftReport{
			SpecSlug:        slug,
			Acknowledged:    true,
			AcknowledgeNote: note,
		}, nil
	}
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return nil, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "intentional divergence",
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", resp.Msg.SpecSlug)
	require.True(t, resp.Msg.Acknowledged)
	require.Equal(t, "intentional divergence", resp.Msg.AcknowledgeNote)
}

func TestLifecycleHandler_Amend_EmptySlug(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:   "",
		Reason: "rework",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Supersede_SameSlug(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "same-spec",
		NewSlug: "same-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Supersede_NewSpecNotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.supersedeSpec = func(_ context.Context, _, _ string) (*storage.Spec, *storage.Spec, error) {
		return nil, nil, storage.ErrNewSpecNotFound
	}
	client := newLifecycleClient(t, deps)
	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "old-spec",
		NewSlug: "missing-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_Lint(t *testing.T) {
	deps := defaultTestDeps()
	deps.linter.lint = func(_ context.Context, slug string) ([]storage.LintResult, error) {
		return []storage.LintResult{
			{
				SpecSlug: slug,
				Passed:   true,
			},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Results, 1)
	require.Equal(t, "my-spec", resp.Msg.Results[0].SpecSlug)
	require.True(t, resp.Msg.Results[0].Passed)
}

func TestLifecycleHandler_Lint_WithViolations(t *testing.T) {
	deps := defaultTestDeps()
	deps.linter.lint = func(_ context.Context, _ string) ([]storage.LintResult, error) {
		return []storage.LintResult{
			{
				SpecSlug: "bad-spec",
				Passed:   false,
				Violations: []storage.LintViolation{
					{
						Rule:     "schema.enum",
						Severity: storage.LintSeverityError,
						Message:  "invalid stage",
						Location: "stage",
					},
				},
			},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "bad-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Results, 1)
	require.False(t, resp.Msg.Results[0].Passed)
	require.Len(t, resp.Msg.Results[0].Violations, 1)
	require.Equal(t, "schema.enum", resp.Msg.Results[0].Violations[0].Rule)
}

func TestLifecycleHandler_Abandon_Terminal(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.abandonSpec = func(_ context.Context, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecTerminal
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   "already-abandoned",
		Reason: "try again",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_Supersede_Terminal(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.supersedeSpec = func(_ context.Context, _, _ string) (*storage.Spec, *storage.Spec, error) {
		return nil, nil, storage.ErrSpecTerminal
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "terminal-spec",
		NewSlug: "new-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_Amend_ConcurrentModification(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrConcurrentModification
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:   "busy-spec",
		Reason: "rework",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeAborted, connErr.Code())
}

func TestLifecycleHandler_Amend_TerminalReEntryStage(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	for _, stage := range []string{"amended", "superseded", "abandoned"} {
		_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
			Slug:         "my-spec",
			Reason:       "rework",
			ReEntryStage: stage,
		}))
		require.Error(t, err, "stage %q should be rejected", stage)
		var connErr *connect.Error
		require.ErrorAs(t, err, &connErr)
		require.Equal(t, connect.CodeInvalidArgument, connErr.Code(), "stage %q should return InvalidArgument", stage)
	}
}

func TestLifecycleHandler_CheckDrift_Error(t *testing.T) {
	deps := defaultTestDeps()
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return nil, errors.New("backend unavailable")
	}
	client := newLifecycleClient(t, deps)

	_, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestLifecycleHandler_CheckDrift_NilChecker(t *testing.T) {
	deps := defaultTestDeps()
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, deps.store, nil, deps.linter)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)

	_, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeUnimplemented, connErr.Code())
}

func TestLifecycleHandler_Lint_NilLinter(t *testing.T) {
	deps := defaultTestDeps()
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, deps.store, deps.drift, nil)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)

	_, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeUnimplemented, connErr.Code())
}

func TestLifecycleHandler_Amend_TerminalSpec(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecTerminal
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:   "superseded-spec",
		Reason: "rework",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_AcknowledgeDrift_EmptyNote(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Abandon_NotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.abandonSpec = func(_ context.Context, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecNotFound
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   "no-such-spec",
		Reason: "no longer needed",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_Supersede_OldNotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.supersedeSpec = func(_ context.Context, _, _ string) (*storage.Spec, *storage.Spec, error) {
		return nil, nil, storage.ErrSpecNotFound
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "missing-old-spec",
		NewSlug: "new-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_Amend_EmptyReason(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "my-spec",
		Reason:       "",
		ReEntryStage: "shape",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Abandon_EmptyReason(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   "dead-spec",
		Reason: "",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}
