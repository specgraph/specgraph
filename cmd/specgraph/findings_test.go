// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"io"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
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

	err := runFindingsList(newCmdWithCtx(), []string{"my-spec"})
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

	cmd := newCmdWithCtx()
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

	err := runFindingsList(newCmdWithCtx(), []string{"my-spec"})
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

	err := runFindingsList(newCmdWithCtx(), []string{"my-spec"})
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

	err := runFindingsList(newCmdWithCtx(), []string{"my-spec"})
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

	err := runFindingsList(newCmdWithCtx(), []string{"my-spec"})
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

	err := runFindingsList(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

// --- cobra args ---

func TestFindingsListCmd_RequiresSlug(t *testing.T) {
	err := findingsListCmd.Args(findingsListCmd, []string{})
	require.Error(t, err)
}

// --- findings store fake handlers ---

type fakeFindingsStoreHandler struct {
	specgraphv1connect.UnimplementedAnalyticalPassServiceHandler
}

func (fakeFindingsStoreHandler) StoreFindings(_ context.Context, _ *connect.Request[specv1.StoreFindingsRequest]) (*connect.Response[specv1.StoreFindingsResponse], error) {
	return connect.NewResponse(&specv1.StoreFindingsResponse{
		Ids: []string{"finding-abc", "finding-def"},
	}), nil
}

type fakeFindingsStoreErrorHandler struct {
	specgraphv1connect.UnimplementedAnalyticalPassServiceHandler
}

func (fakeFindingsStoreErrorHandler) StoreFindings(_ context.Context, _ *connect.Request[specv1.StoreFindingsRequest]) (*connect.Response[specv1.StoreFindingsResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

// --- findings store tests ---

func TestRunFindingsStore_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakeFindingsStoreHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	f := writeJSONFile(t, `{"findings":[{"summary":"test finding"}]}`)

	oldPT := findingsStorePassType
	oldJSON := findingsStoreJSON
	oldFile := findingsStoreFile
	findingsStorePassType = "red-team"
	findingsStoreJSON = false
	findingsStoreFile = f
	t.Cleanup(func() {
		findingsStorePassType = oldPT
		findingsStoreJSON = oldJSON
		findingsStoreFile = oldFile
	})

	err := runFindingsStore(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunFindingsStore_JSON(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakeFindingsStoreHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	f := writeJSONFile(t, `{"findings":[{"summary":"test finding"}]}`)

	oldPT := findingsStorePassType
	oldJSON := findingsStoreJSON
	oldFile := findingsStoreFile
	findingsStorePassType = "red-team"
	findingsStoreJSON = true
	findingsStoreFile = f
	t.Cleanup(func() {
		findingsStorePassType = oldPT
		findingsStoreJSON = oldJSON
		findingsStoreFile = oldFile
	})

	cmd := newCmdWithCtx()
	cmd.SetOut(io.Discard)
	err := runFindingsStore(cmd, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunFindingsStore_MissingPassType(t *testing.T) {
	oldPT := findingsStorePassType
	oldJSON := findingsStoreJSON
	oldFile := findingsStoreFile
	findingsStorePassType = ""
	findingsStoreJSON = false
	findingsStoreFile = "some-file.json"
	t.Cleanup(func() {
		findingsStorePassType = oldPT
		findingsStoreJSON = oldJSON
		findingsStoreFile = oldFile
	})

	err := runFindingsStore(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pass-type is required")
}

func TestRunFindingsStore_MissingJSONFile(t *testing.T) {
	oldPT := findingsStorePassType
	oldJSON := findingsStoreJSON
	oldFile := findingsStoreFile
	findingsStorePassType = "red-team"
	findingsStoreJSON = false
	findingsStoreFile = ""
	t.Cleanup(func() {
		findingsStorePassType = oldPT
		findingsStoreJSON = oldJSON
		findingsStoreFile = oldFile
	})

	err := runFindingsStore(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "json-file is required")
}

func TestRunFindingsStore_InvalidPassType(t *testing.T) {
	oldPT := findingsStorePassType
	oldJSON := findingsStoreJSON
	oldFile := findingsStoreFile
	findingsStorePassType = "bogus"
	findingsStoreJSON = false
	findingsStoreFile = "some-file.json"
	t.Cleanup(func() {
		findingsStorePassType = oldPT
		findingsStoreJSON = oldJSON
		findingsStoreFile = oldFile
	})

	err := runFindingsStore(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown pass type")
}

func TestRunFindingsStore_FileNotFound(t *testing.T) {
	oldPT := findingsStorePassType
	oldJSON := findingsStoreJSON
	oldFile := findingsStoreFile
	findingsStorePassType = "red-team"
	findingsStoreJSON = false
	findingsStoreFile = "/tmp/does-not-exist-specgraph-test.json"
	t.Cleanup(func() {
		findingsStorePassType = oldPT
		findingsStoreJSON = oldJSON
		findingsStoreFile = oldFile
	})

	err := runFindingsStore(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read json file")
}

func TestRunFindingsStore_MalformedJSON(t *testing.T) {
	f := writeJSONFile(t, `{not valid json}`)

	oldPT := findingsStorePassType
	oldJSON := findingsStoreJSON
	oldFile := findingsStoreFile
	findingsStorePassType = "red-team"
	findingsStoreJSON = false
	findingsStoreFile = f
	t.Cleanup(func() {
		findingsStorePassType = oldPT
		findingsStoreJSON = oldJSON
		findingsStoreFile = oldFile
	})

	err := runFindingsStore(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse json file")
}

func TestRunFindingsStore_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakeFindingsStoreErrorHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	f := writeJSONFile(t, `{"findings":[{"summary":"test"}]}`)

	oldPT := findingsStorePassType
	oldJSON := findingsStoreJSON
	oldFile := findingsStoreFile
	findingsStorePassType = "red-team"
	findingsStoreJSON = false
	findingsStoreFile = f
	t.Cleanup(func() {
		findingsStorePassType = oldPT
		findingsStoreJSON = oldJSON
		findingsStoreFile = oldFile
	})

	err := runFindingsStore(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store findings")
}

func TestRunFindingsStore_ClientError(t *testing.T) {
	setMissingConfig(t)

	f := writeJSONFile(t, `{"findings":[{"summary":"test"}]}`)

	oldPT := findingsStorePassType
	oldJSON := findingsStoreJSON
	oldFile := findingsStoreFile
	findingsStorePassType = "red-team"
	findingsStoreJSON = false
	findingsStoreFile = f
	t.Cleanup(func() {
		findingsStorePassType = oldPT
		findingsStoreJSON = oldJSON
		findingsStoreFile = oldFile
	})

	err := runFindingsStore(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

func TestFindingsStoreCmd_RequiresSlug(t *testing.T) {
	err := findingsStoreCmd.Args(findingsStoreCmd, []string{})
	require.Error(t, err)
}
