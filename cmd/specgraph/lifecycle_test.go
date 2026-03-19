// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/driftscope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriftScopeToProto(t *testing.T) {
	tests := []struct {
		input string
		want  specv1.DriftScope
	}{
		{"deps", specv1.DriftScope_DRIFT_SCOPE_DEPS},
		{"interfaces", specv1.DriftScope_DRIFT_SCOPE_INTERFACES},
		{"verify", specv1.DriftScope_DRIFT_SCOPE_VERIFY},
		{"", specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := driftScopeToProto(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDriftScopeToProto_UnknownReturnsError(t *testing.T) {
	_, err := driftScopeToProto("bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scope")
	assert.Contains(t, err.Error(), "bogus")
}

func TestDriftScopeToProtoMap_Completeness(t *testing.T) {
	expected := []string{"", "deps", "interfaces", "verify"}
	for _, scope := range expected {
		_, ok := driftScopeToProtoMap[scope]
		assert.True(t, ok, "expected scope %q in driftScopeToProtoMap", scope)
	}
}

func TestDriftScopeToProtoMap_SyncWithDriftscope(t *testing.T) {
	for scope := range driftScopeToProtoMap {
		if scope == "" {
			continue // empty string maps to UNSPECIFIED; it is not a CLI scope
		}
		assert.True(t, driftscope.IsValid(scope),
			"CLI scope %q not recognized by driftscope.IsValid — tables out of sync", scope)
	}
}

// --- lifecycle CLI run function tests ---

func TestRunAmend_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runAmend(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunSupersede_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runSupersede(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunAbandon_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runAbandon(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunDrift_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runDrift(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunDrift_InvalidScope(t *testing.T) {
	old := driftScope
	driftScope = "bogus"
	t.Cleanup(func() { driftScope = old })
	err := runDrift(nil, []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scope")
}

func TestRunDriftAck_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runDriftAck(nil, []string{"my-spec"})
	require.Error(t, err)
}

// fakeAckHappyHandler implements AcknowledgeDrift returning a successful ack.
type fakeAckHappyHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeAckHappyHandler) AcknowledgeDrift(_ context.Context, req *connect.Request[specv1.DriftAcknowledgeRequest]) (*connect.Response[specv1.DriftAcknowledgeResponse], error) {
	return connect.NewResponse(&specv1.DriftAcknowledgeResponse{
		Report: &specv1.DriftReport{
			SpecSlug: req.Msg.GetSlug(),
		},
	}), nil
}

func TestRunDriftAck_HappyPath_All(t *testing.T) {
	startFakeLifecycleServer(t, fakeAckHappyHandler{})

	oldAll := driftAckAll
	driftAckAll = true
	t.Cleanup(func() { driftAckAll = oldAll })

	err := runDriftAck(nil, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunDriftAck_HappyPath_Upstream(t *testing.T) {
	startFakeLifecycleServer(t, fakeAckHappyHandler{})

	oldUpstream := driftAckUpstream
	driftAckUpstream = "dep-spec"
	t.Cleanup(func() { driftAckUpstream = oldUpstream })

	err := runDriftAck(nil, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunDriftAck_RequiresUpstreamOrAll(t *testing.T) {
	oldAll := driftAckAll
	oldUpstream := driftAckUpstream
	driftAckAll = false
	driftAckUpstream = ""
	t.Cleanup(func() {
		driftAckAll = oldAll
		driftAckUpstream = oldUpstream
	})

	err := runDriftAck(nil, []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--upstream")
}

func TestRunDriftAck_CannotSpecifyBoth(t *testing.T) {
	oldAll := driftAckAll
	oldUpstream := driftAckUpstream
	driftAckAll = true
	driftAckUpstream = "dep-spec"
	t.Cleanup(func() {
		driftAckAll = oldAll
		driftAckUpstream = oldUpstream
	})

	err := runDriftAck(nil, []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both")
}

// startFakeLifecycleServer registers handler with a fresh httptest.Server and
// sets cfgFile to point at it.
func startFakeLifecycleServer(t *testing.T, h specgraphv1connect.LifecycleServiceHandler) {
	t.Helper()
	mux := http.NewServeMux()
	path, hnd := specgraphv1connect.NewLifecycleServiceHandler(h)
	mux.Handle(path, hnd)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(fmt.Sprintf("server:\n  remote: %s\n", srv.URL)), 0o600))
	old := cfgFile
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = old })
}

// --- happy-path fake handlers ---

type fakeAmendHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeAmendHandler) TransitionAmend(_ context.Context, _ *connect.Request[specv1.TransitionAmendRequest]) (*connect.Response[specv1.TransitionAmendResponse], error) {
	return connect.NewResponse(&specv1.TransitionAmendResponse{
		Spec: &specv1.Spec{Slug: "my-spec", Stage: "amended", Lifecycle: specv1.SpecLifecycle_SPEC_LIFECYCLE_LIVING, Version: 2},
	}), nil
}

func TestRunAmend_HappyPath(t *testing.T) {
	startFakeLifecycleServer(t, fakeAmendHandler{})
	old := amendReason
	amendReason = "test reason"
	t.Cleanup(func() { amendReason = old })
	err := runAmend(nil, []string{"my-spec"})
	require.NoError(t, err)
}

type fakeSupersedeHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeSupersedeHandler) TransitionSupersede(_ context.Context, _ *connect.Request[specv1.TransitionSupersedeRequest]) (*connect.Response[specv1.TransitionSupersedeResponse], error) {
	return connect.NewResponse(&specv1.TransitionSupersedeResponse{
		OldSpec: &specv1.Spec{Slug: "old-spec", Lifecycle: specv1.SpecLifecycle_SPEC_LIFECYCLE_TASK},
		NewSpec: &specv1.Spec{Slug: "new-spec", Stage: "spark", Lifecycle: specv1.SpecLifecycle_SPEC_LIFECYCLE_LIVING},
	}), nil
}

func TestRunSupersede_HappyPath(t *testing.T) {
	startFakeLifecycleServer(t, fakeSupersedeHandler{})
	old := supersedeWith
	supersedeWith = "new-spec"
	t.Cleanup(func() { supersedeWith = old })
	err := runSupersede(nil, []string{"old-spec"})
	require.NoError(t, err)
}

type fakeAbandonHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeAbandonHandler) TransitionAbandon(_ context.Context, _ *connect.Request[specv1.TransitionAbandonRequest]) (*connect.Response[specv1.TransitionAbandonResponse], error) {
	return connect.NewResponse(&specv1.TransitionAbandonResponse{
		Spec: &specv1.Spec{Slug: "my-spec", Stage: "abandoned", Lifecycle: specv1.SpecLifecycle_SPEC_LIFECYCLE_TASK, Version: 1},
	}), nil
}

func TestRunAbandon_HappyPath(t *testing.T) {
	startFakeLifecycleServer(t, fakeAbandonHandler{})
	old := abandonReason
	abandonReason = "no longer needed"
	t.Cleanup(func() { abandonReason = old })
	err := runAbandon(nil, []string{"my-spec"})
	require.NoError(t, err)
}

type fakeLintHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeLintHandler) Lint(_ context.Context, _ *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	return connect.NewResponse(&specv1.LintResponse{
		Results: []*specv1.LintResult{{SpecSlug: "my-spec", Passed: true}},
	}), nil
}

func TestRunLint_HappyPath(t *testing.T) {
	startFakeLifecycleServer(t, fakeLintHandler{})
	err := runLint(nil, nil)
	require.NoError(t, err)
}

func TestRunLint_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runLint(nil, nil)
	require.Error(t, err)
}

func TestAmendCmd_RequiresSlug(t *testing.T) {
	err := amendCmd.Args(amendCmd, []string{})
	require.Error(t, err)
}

func TestSupersedeCmd_RequiresSlug(t *testing.T) {
	err := supersedeCmd.Args(supersedeCmd, []string{})
	require.Error(t, err)
}

func TestAbandonCmd_RequiresSlug(t *testing.T) {
	err := abandonCmd.Args(abandonCmd, []string{})
	require.Error(t, err)
}

func TestDriftCmd_AcceptsNoArgs(t *testing.T) {
	err := driftCmd.Args(driftCmd, []string{})
	require.NoError(t, err)
}

func TestDriftCmd_AcceptsOneArg(t *testing.T) {
	err := driftCmd.Args(driftCmd, []string{"my-spec"})
	require.NoError(t, err)
}

func TestDriftAckCmd_RequiresSlug(t *testing.T) {
	err := driftAckCmd.Args(driftAckCmd, []string{})
	require.Error(t, err)
}

func TestLintCmd_AcceptsNoArgs(t *testing.T) {
	err := lintCmd.Args(lintCmd, []string{})
	require.NoError(t, err)
}

func TestLintCmd_AcceptsOneArg(t *testing.T) {
	err := lintCmd.Args(lintCmd, []string{"my-spec"})
	require.NoError(t, err)
}

// --- runLint failure path (spgr-79b.29) ---

type fakeLintFailHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeLintFailHandler) Lint(_ context.Context, _ *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	return connect.NewResponse(&specv1.LintResponse{
		Results: []*specv1.LintResult{
			{SpecSlug: "good-spec", Passed: true},
			{
				SpecSlug: "bad-spec",
				Passed:   false,
				Violations: []*specv1.LintViolation{
					{Rule: "missing-intent", Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR, Message: "spec missing intent"},
				},
			},
		},
	}), nil
}

func TestRunLint_HappyPath_WithFailures(t *testing.T) {
	startFakeLifecycleServer(t, fakeLintFailHandler{})
	err := runLint(nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "spec(s) failed")
}

// --- runLint infra-error path (spgr-myz.2) ---

type fakeLintInfraErrHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeLintInfraErrHandler) Lint(_ context.Context, _ *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	return connect.NewResponse(&specv1.LintResponse{
		Results: []*specv1.LintResult{
			{SpecSlug: "good-spec", Passed: true},
			{
				SpecSlug: "infra-err-spec",
				Passed:   false,
				Error:    "storage unavailable",
			},
		},
	}), nil
}

func TestRunLint_InfraError(t *testing.T) {
	startFakeLifecycleServer(t, fakeLintInfraErrHandler{})
	err := runLint(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "infrastructure error")
}

// --- runDrift happy-path tests (spgr-79b.28) ---

type fakeDriftNoneHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeDriftNoneHandler) CheckDrift(_ context.Context, _ *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	return connect.NewResponse(&specv1.DriftCheckResponse{}), nil
}

func TestRunDrift_HappyPath_NoDrift(t *testing.T) {
	startFakeLifecycleServer(t, fakeDriftNoneHandler{})
	err := runDrift(nil, nil)
	require.NoError(t, err)
}

type fakeDriftItemsHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeDriftItemsHandler) CheckDrift(_ context.Context, _ *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	return connect.NewResponse(&specv1.DriftCheckResponse{
		Reports: []*specv1.DriftReport{
			{
				SpecSlug: "my-spec",
				Items: []*specv1.DriftItem{
					{
						Type:        specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
						Severity:    specv1.DriftSeverity_DRIFT_SEVERITY_MEDIUM,
						Description: "dependency changed",
					},
				},
			},
			{
				SpecSlug:     "err-spec",
				ErrorMessage: "unable to check drift",
			},
		},
	}), nil
}

func TestRunDrift_WithItemsAndErrors(t *testing.T) {
	startFakeLifecycleServer(t, fakeDriftItemsHandler{})
	err := runDrift(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift check completed with errors")
}

// --- runDrift clean report (spgr-myz.5) ---

type fakeDriftCleanReportHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeDriftCleanReportHandler) CheckDrift(_ context.Context, _ *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	return connect.NewResponse(&specv1.DriftCheckResponse{
		Reports: []*specv1.DriftReport{
			{SpecSlug: "clean-spec"},
		},
	}), nil
}

func TestRunDrift_CleanReport_NoDrift(t *testing.T) {
	startFakeLifecycleServer(t, fakeDriftCleanReportHandler{})
	err := runDrift(nil, nil)
	require.NoError(t, err)
}

// --- runDrift multi-report clean path (spgr-0fk.1) ---

// fakeDriftMultiCleanHandler returns 2 DriftReport entries both with no items and no error,
// exercising the !hasDrift && !hasErrors branch at line 200 (not the early-return at line 173).
type fakeDriftMultiCleanHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeDriftMultiCleanHandler) CheckDrift(_ context.Context, _ *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	return connect.NewResponse(&specv1.DriftCheckResponse{
		Reports: []*specv1.DriftReport{
			{SpecSlug: "clean-spec-a"},
			{SpecSlug: "clean-spec-b"},
		},
	}), nil
}

func TestRunDrift_MultiCleanReports_NoDrift(t *testing.T) {
	startFakeLifecycleServer(t, fakeDriftMultiCleanHandler{})
	err := runDrift(nil, nil)
	require.NoError(t, err)
}

// --- runDrift error-only report (spgr-jqc.5) ---

type fakeDriftErrorOnlyHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeDriftErrorOnlyHandler) CheckDrift(_ context.Context, _ *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	return connect.NewResponse(&specv1.DriftCheckResponse{
		Reports: []*specv1.DriftReport{
			{
				SpecSlug:     "infra-fail-spec",
				ErrorMessage: "database timeout during drift check",
			},
		},
	}), nil
}

func TestRunDrift_ErrorOnlyReport(t *testing.T) {
	startFakeLifecycleServer(t, fakeDriftErrorOnlyHandler{})
	err := runDrift(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift check completed with errors")
}

// --- runDrift drift-only (no errors) (spgr-5bl.1) ---

type fakeDriftOnlyHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeDriftOnlyHandler) CheckDrift(_ context.Context, _ *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	return connect.NewResponse(&specv1.DriftCheckResponse{
		Reports: []*specv1.DriftReport{
			{
				SpecSlug: "drifted-spec",
				Items: []*specv1.DriftItem{
					{
						Type:        specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
						Severity:    specv1.DriftSeverity_DRIFT_SEVERITY_LOW,
						Description: "dependency version changed",
					},
				},
			},
		},
	}), nil
}

func TestRunDrift_DriftOnly_NoErrors(t *testing.T) {
	startFakeLifecycleServer(t, fakeDriftOnlyHandler{})
	err := runDrift(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift detected")
}

// --- runDrift with slug (spgr-d1b.17) ---

type fakeDriftSlugCapture struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
	capturedSlug string
}

func (h *fakeDriftSlugCapture) CheckDrift(_ context.Context, req *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	h.capturedSlug = req.Msg.GetSlug()
	return connect.NewResponse(&specv1.DriftCheckResponse{}), nil
}

func TestRunDrift_HappyPath_WithSlug(t *testing.T) {
	h := &fakeDriftSlugCapture{}
	startFakeLifecycleServer(t, h)
	err := runDrift(nil, []string{"my-spec"})
	require.NoError(t, err)
	assert.Equal(t, "my-spec", h.capturedSlug)
}

type fakeLintEmptyHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeLintEmptyHandler) Lint(_ context.Context, _ *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	return connect.NewResponse(&specv1.LintResponse{Results: []*specv1.LintResult{}}), nil
}

func TestRunLint_EmptyResults(t *testing.T) {
	startFakeLifecycleServer(t, fakeLintEmptyHandler{})
	err := runLint(nil, nil)
	require.NoError(t, err)
}
