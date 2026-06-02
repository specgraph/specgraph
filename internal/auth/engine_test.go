// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

// stubSource is an in-memory PolicySource for engine unit tests, so the
// engine is exercised independently of the embedded file.
type stubSource struct {
	name    string
	docs    []auth.PolicyDocument
	loadErr error
}

func (s stubSource) Name() string { return s.name }
func (s stubSource) Load(context.Context) ([]auth.PolicyDocument, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.docs, nil
}

// basePolicies is the three-policy verb-group set reused across engine tests.
const basePolicies = `
permit (principal, action in SpecGraph::Action::"read", resource)
when { principal has role && (principal.role == "reader" || principal.role == "writer" || principal.role == "admin") };
permit (principal, action in SpecGraph::Action::"write", resource)
when { principal has role && (principal.role == "writer" || principal.role == "admin") };
permit (principal, action in SpecGraph::Action::"delete", resource)
when { principal has role && principal.role == "admin" };
`

func baseSource() auth.PolicySource {
	return stubSource{name: "test", docs: []auth.PolicyDocument{{Source: "test:base.cedar", Text: basePolicies}}}
}

func testActions() []string { return []string{"spec.read", "spec.write", "graph.delete"} }

func TestNewCedarEngine_LoadsPolicies(t *testing.T) {
	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{baseSource()}, testActions())
	require.NoError(t, err)
	require.NotNil(t, eng)
}

func TestNewCedarEngine_NoPoliciesIsError(t *testing.T) {
	empty := stubSource{name: "empty", docs: nil}
	_, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{empty}, testActions())
	require.Error(t, err, "no loaded policies must refuse construction")
}

func TestNewCedarEngine_BadPolicyTextIsError(t *testing.T) {
	bad := stubSource{name: "bad", docs: []auth.PolicyDocument{{Source: "bad:x.cedar", Text: "this is not cedar"}}}
	_, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{bad}, testActions())
	require.Error(t, err)
}

func TestNewCedarEngine_SourceLoadErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")
	failing := stubSource{name: "failing", loadErr: sentinel}
	_, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{failing}, testActions())
	require.ErrorIs(t, err, sentinel)
}

func TestCedarEngine_Reload(t *testing.T) {
	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{baseSource()}, testActions())
	require.NoError(t, err)
	require.NoError(t, eng.Reload(context.Background()))
}

func TestNewCedarEngine_DuplicateDocSourceIsError(t *testing.T) {
	same := stubSource{name: "s1", docs: []auth.PolicyDocument{
		{Source: "shared:base.cedar", Text: basePolicies},
	}}
	again := stubSource{name: "s2", docs: []auth.PolicyDocument{
		{Source: "shared:base.cedar", Text: basePolicies},
	}}
	_, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{same, again}, testActions())
	require.Error(t, err)
	require.ErrorContains(t, err, "duplicate policy id")
}
