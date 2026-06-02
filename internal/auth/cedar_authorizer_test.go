// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

// fakeEngine lets the authorizer tests control decisions without real cedar.
type fakeEngine struct {
	dec auth.PolicyDecision
	err error
	got auth.EvalRequest
}

func (f *fakeEngine) Evaluate(_ context.Context, req auth.EvalRequest) (auth.PolicyDecision, error) {
	f.got = req
	return f.dec, f.err
}
func (f *fakeEngine) Reload(context.Context) error { return nil }

func TestCedarAuthorizer_Allow(t *testing.T) {
	eng := &fakeEngine{dec: auth.PolicyDecision{Allowed: true, MatchedPolicies: []string{"embedded:base.cedar#policy0"}}}
	a := auth.NewCedarAuthorizer(eng)
	id := &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleReader}

	d, err := a.Authorize(context.Background(), id, specgraphv1connect.SpecServiceGetSpecProcedure, nil)
	require.NoError(t, err)
	require.True(t, d.Allowed)
	require.Equal(t, "cedar-allow:embedded:base.cedar#policy0", d.Reason)
	// The engine received the mapped action name, not the RPC method name.
	require.Equal(t, "spec.read", eng.got.Action)
}

func TestCedarAuthorizer_Deny(t *testing.T) {
	eng := &fakeEngine{dec: auth.PolicyDecision{Allowed: false}}
	a := auth.NewCedarAuthorizer(eng)
	id := &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleReader}

	d, err := a.Authorize(context.Background(), id, specgraphv1connect.SpecServiceCreateSpecProcedure, nil)
	require.NoError(t, err)
	require.False(t, d.Allowed)
	require.Equal(t, "cedar-deny:spec.write", d.Reason)
}

func TestCedarAuthorizer_UnconfiguredProcedureIsError(t *testing.T) {
	a := auth.NewCedarAuthorizer(&fakeEngine{})
	_, err := a.Authorize(context.Background(), &auth.Identity{}, "/no.such/Procedure", nil)
	require.Error(t, err)
}

func TestCedarAuthorizer_EngineErrorPropagates(t *testing.T) {
	sentinel := errors.New("engine down")
	a := auth.NewCedarAuthorizer(&fakeEngine{err: sentinel})
	_, err := a.Authorize(context.Background(), &auth.Identity{EffectiveRole: auth.RoleReader},
		specgraphv1connect.SpecServiceGetSpecProcedure, nil)
	require.ErrorIs(t, err, sentinel)
}

// Compile-time assertion that CedarAuthorizer satisfies Authorizer.
var _ auth.Authorizer = (*auth.CedarAuthorizer)(nil)
