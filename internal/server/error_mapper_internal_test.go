// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertConnectCode is a test helper that verifies err is a *connect.Error with the expected code.
func assertConnectCode(t *testing.T, err error, wantCode connect.Code) {
	t.Helper()
	var connErr *connect.Error
	require.True(t, errors.As(err, &connErr), "expected *connect.Error, got %T", err)
	assert.Equal(t, wantCode, connErr.Code())
}

func TestSpecError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{"not found", storage.ErrSpecNotFound, connect.CodeNotFound},
		{"already exists", storage.ErrSpecAlreadyExists, connect.CodeAlreadyExists},
		{"concurrent modification", storage.ErrConcurrentModification, connect.CodeAborted},
		{"connect error passthrough", connect.NewError(connect.CodePermissionDenied, errors.New("denied")), connect.CodePermissionDenied},
		{"unknown error", errors.New("boom"), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertConnectCode(t, specError(tt.err), tt.wantCode)
		})
	}
}

func TestClaimError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{"spec not found", storage.ErrSpecNotFound, connect.CodeNotFound},
		{"already claimed", storage.ErrSpecAlreadyClaimed, connect.CodeFailedPrecondition},
		{"not claim owner", storage.ErrNotClaimOwner, connect.CodePermissionDenied},
		{"spec not claimed", storage.ErrSpecNotClaimed, connect.CodeNotFound},
		{"connect error passthrough", connect.NewError(connect.CodeUnauthenticated, errors.New("no auth")), connect.CodeUnauthenticated},
		{"unknown error", errors.New("boom"), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertConnectCode(t, claimError(tt.err), tt.wantCode)
		})
	}
}

func TestDecisionError(t *testing.T) {
	h := &DecisionHandler{logger: slog.Default()}
	ctx := context.Background()
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{"decision not found", storage.ErrDecisionNotFound, connect.CodeNotFound},
		{"superseded_by required", storage.ErrSupersededByRequired, connect.CodeInvalidArgument},
		{"concurrent modification", storage.ErrConcurrentModification, connect.CodeAborted},
		{"connect error passthrough", connect.NewError(connect.CodeResourceExhausted, errors.New("too many")), connect.CodeResourceExhausted},
		{"unknown error", errors.New("boom"), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertConnectCode(t, h.decisionError(ctx, tt.err), tt.wantCode)
		})
	}
}

func TestGraphError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{"spec not found", storage.ErrSpecNotFound, connect.CodeNotFound},
		{"edge not found", storage.ErrEdgeNotFound, connect.CodeNotFound},
		{"connect error passthrough", connect.NewError(connect.CodeInvalidArgument, errors.New("bad")), connect.CodeInvalidArgument},
		{"unknown error", errors.New("boom"), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertConnectCode(t, graphError(tt.err), tt.wantCode)
		})
	}
}

func TestAnalyticalPassError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{"spec not found", storage.ErrSpecNotFound, connect.CodeNotFound},
		{"connect error passthrough", connect.NewError(connect.CodeDeadlineExceeded, errors.New("timeout")), connect.CodeDeadlineExceeded},
		{"unknown error", errors.New("boom"), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertConnectCode(t, analyticalPassError(tt.err), tt.wantCode)
		})
	}
}

func TestExecutionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{"spec not found", storage.ErrSpecNotFound, connect.CodeNotFound},
		{"spec not approved", storage.ErrSpecNotApproved, connect.CodeFailedPrecondition},
		{"agent not claim owner", storage.ErrAgentNotClaimOwner, connect.CodePermissionDenied},
		{"connect error passthrough", connect.NewError(connect.CodeCanceled, errors.New("canceled")), connect.CodeCanceled},
		{"unknown error", errors.New("boom"), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertConnectCode(t, executionError(tt.err), tt.wantCode)
		})
	}
}

func TestConstitutionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{"constitution not found", storage.ErrConstitutionNotFound, connect.CodeNotFound},
		{"connect error passthrough", connect.NewError(connect.CodeDataLoss, errors.New("lost")), connect.CodeDataLoss},
		{"unknown error", errors.New("boom"), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertConnectCode(t, constitutionError(tt.err), tt.wantCode)
		})
	}
}

func TestLifecycleError(t *testing.T) {
	h := &LifecycleHandler{logger: slog.Default()}
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{"spec not found", storage.ErrSpecNotFound, connect.CodeNotFound},
		{"spec not done", storage.ErrSpecNotDone, connect.CodeFailedPrecondition},
		{"spec ineligible stage", storage.ErrSpecIneligibleStage, connect.CodeFailedPrecondition},
		{"spec terminal", storage.ErrSpecTerminal, connect.CodeFailedPrecondition},
		{"spec ineligible for drift", storage.ErrSpecIneligibleForDrift, connect.CodeFailedPrecondition},
		{"new spec not found", storage.ErrNewSpecNotFound, connect.CodeNotFound},
		{"new spec terminal", storage.ErrNewSpecTerminal, connect.CodeFailedPrecondition},
		{"concurrent modification", storage.ErrConcurrentModification, connect.CodeAborted},
		{"internal guard failure", storage.ErrInternalGuardFailure, connect.CodeInternal},
		{"invalid re-entry stage", storage.ErrInvalidReEntryStage, connect.CodeInvalidArgument},
		{"spec not amendable", storage.ErrSpecNotAmendable, connect.CodeFailedPrecondition},
		{"re-entry stage required", storage.ErrReEntryStageRequired, connect.CodeInvalidArgument},
		{"edge not found", storage.ErrEdgeNotFound, connect.CodeNotFound},
		{"same slugs", storage.ErrSameSlugs, connect.CodeInvalidArgument},
		{"connect error passthrough", connect.NewError(connect.CodeAborted, errors.New("abort")), connect.CodeAborted},
		{"unknown error", errors.New("boom"), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertConnectCode(t, h.lifecycleError("TestOp", "test-slug", tt.err), tt.wantCode)
		})
	}
}
