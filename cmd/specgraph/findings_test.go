// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"io"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake handlers ---

type fakeFindingsListHandler struct {
	specgraphv1connect.UnimplementedAnalyticalPassServiceHandler
}

func (fakeFindingsListHandler) ListFindings(_ context.Context, _ *connect.Request[specv1.ListFindingsRequest]) (*connect.Response[specv1.ListFindingsResponse], error) {
	return connect.NewResponse(&specv1.ListFindingsResponse{
		Findings: []*specv1.AnalyticalFinding{
			{
				Id:       "finding-1",
				PassType: specv1.PassType_PASS_TYPE_RED_TEAM,
				Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
				Summary:  "Test finding",
				Detail:   "Details here",
			},
		},
	}), nil
}

type fakeFindingsListEmptyHandler struct {
	specgraphv1connect.UnimplementedAnalyticalPassServiceHandler
}

func (fakeFindingsListEmptyHandler) ListFindings(_ context.Context, _ *connect.Request[specv1.ListFindingsRequest]) (*connect.Response[specv1.ListFindingsResponse], error) {
	return connect.NewResponse(&specv1.ListFindingsResponse{}), nil
}

type fakeFindingsListErrorHandler struct {
	specgraphv1connect.UnimplementedAnalyticalPassServiceHandler
}

func (fakeFindingsListErrorHandler) ListFindings(_ context.Context, _ *connect.Request[specv1.ListFindingsRequest]) (*connect.Response[specv1.ListFindingsResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

// --- map tests ---

func TestPassTypeMap_Completeness(t *testing.T) {
	expected := []string{
		"constitution-check",
		"red-team",
		"peripheral-vision",
		"consistency",
		"simplicity",
	}
	for _, name := range expected {
		_, ok := passTypeMap[name]
		assert.True(t, ok, "expected pass type %q in passTypeMap", name)
	}
	assert.Len(t, passTypeMap, len(expected))
}

func TestPassTypeMap_AllEntriesAreValidProto(t *testing.T) {
	for name, pt := range passTypeMap {
		_, ok := specv1.PassType_name[int32(pt)]
		assert.True(t, ok, "passTypeMap[%q] = %d is not a valid PassType enum value", name, pt)
		assert.NotEqual(t, specv1.PassType_PASS_TYPE_UNSPECIFIED, pt,
			"passTypeMap[%q] should not map to UNSPECIFIED", name)
	}
}

// --- happy paths ---

func TestRunFindingsList_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakeFindingsListHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := findingsListPassType
	oldJSON := findingsListJSON
	findingsListPassType = ""
	findingsListJSON = false
	t.Cleanup(func() {
		findingsListPassType = oldPT
		findingsListJSON = oldJSON
	})

	err := runFindingsList(&cobra.Command{}, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunFindingsList_HappyPath_JSON(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakeFindingsListHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := findingsListPassType
	oldJSON := findingsListJSON
	findingsListPassType = ""
	findingsListJSON = true
	t.Cleanup(func() {
		findingsListPassType = oldPT
		findingsListJSON = oldJSON
	})

	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	err := runFindingsList(cmd, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunFindingsList_WithPassTypeFilter(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakeFindingsListHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := findingsListPassType
	oldJSON := findingsListJSON
	findingsListPassType = "red-team"
	findingsListJSON = false
	t.Cleanup(func() {
		findingsListPassType = oldPT
		findingsListJSON = oldJSON
	})

	err := runFindingsList(&cobra.Command{}, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunFindingsList_EmptyResults(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakeFindingsListEmptyHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := findingsListPassType
	oldJSON := findingsListJSON
	findingsListPassType = ""
	findingsListJSON = false
	t.Cleanup(func() {
		findingsListPassType = oldPT
		findingsListJSON = oldJSON
	})

	err := runFindingsList(&cobra.Command{}, []string{"my-spec"})
	require.NoError(t, err)
}

// --- negative paths ---

func TestRunFindingsList_InvalidPassType(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakeFindingsListHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := findingsListPassType
	oldJSON := findingsListJSON
	findingsListPassType = "bogus"
	findingsListJSON = false
	t.Cleanup(func() {
		findingsListPassType = oldPT
		findingsListJSON = oldJSON
	})

	err := runFindingsList(&cobra.Command{}, []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown pass type")
	assert.Contains(t, err.Error(), "bogus")
}

func TestRunFindingsList_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakeFindingsListErrorHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := findingsListPassType
	oldJSON := findingsListJSON
	findingsListPassType = ""
	findingsListJSON = false
	t.Cleanup(func() {
		findingsListPassType = oldPT
		findingsListJSON = oldJSON
	})

	err := runFindingsList(&cobra.Command{}, []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list findings")
}

func TestRunFindingsList_ClientError(t *testing.T) {
	setMissingConfig(t)

	oldPT := findingsListPassType
	oldJSON := findingsListJSON
	findingsListPassType = ""
	findingsListJSON = false
	t.Cleanup(func() {
		findingsListPassType = oldPT
		findingsListJSON = oldJSON
	})

	err := runFindingsList(nil, []string{"my-spec"})
	require.Error(t, err)
}

// --- cobra args ---

func TestFindingsListCmd_RequiresSlug(t *testing.T) {
	err := findingsListCmd.Args(findingsListCmd, []string{})
	require.Error(t, err)
}
