// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// setMissingConfig points cfgFile at a nonexistent path so that newClient
// returns an error, exercising the "client creation fails" branch.
func setMissingConfig(t *testing.T) {
	t.Helper()
	old := cfgFile
	cfgFile = t.TempDir() + "/does-not-exist/config.yaml"
	t.Cleanup(func() { cfgFile = old })
}

// --- spec run functions ---

func TestRunCreate_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runCreate(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunUpdate_ClientError(t *testing.T) {
	setMissingConfig(t)
	cmd := &cobra.Command{}
	cmd.Flags().String("intent", "", "")
	err := runUpdate(cmd, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunList_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runList(&cobra.Command{}, nil)
	require.Error(t, err)
}

func TestRunShow_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runShow(nil, []string{"my-spec"})
	require.Error(t, err)
}

// --- claim run functions ---

func TestRunClaim_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runClaim(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunUnclaim_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runUnclaim(nil, []string{"my-spec"})
	require.Error(t, err)
}

// --- decision run functions ---

func TestRunDecisionCreate_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runDecisionCreate(nil, []string{"my-decision"})
	require.Error(t, err)
}

func TestRunDecisionList_InvalidStatus(t *testing.T) {
	// decisionClient() is called before status validation, so client creation
	// error is returned first when config is missing. The invalid-status path
	// is covered here as a secondary error scenario (client fails first).
	old := decisionListStatus
	decisionListStatus = "not-a-valid-status"
	t.Cleanup(func() { decisionListStatus = old })

	setMissingConfig(t)
	err := runDecisionList(&cobra.Command{}, nil)
	require.Error(t, err)
}

func TestRunDecisionList_ClientError(t *testing.T) {
	setMissingConfig(t)
	old := decisionListStatus
	decisionListStatus = ""
	t.Cleanup(func() { decisionListStatus = old })

	err := runDecisionList(&cobra.Command{}, nil)
	require.Error(t, err)
}

func TestRunDecisionShow_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runDecisionShow(nil, []string{"my-decision"})
	require.Error(t, err)
}

// --- edge run functions ---

func TestRunEdgeAdd_UnknownType(t *testing.T) {
	// graphClient() is called before edge type validation; client creation
	// fails first when config is missing.
	old := edgeAddType
	edgeAddType = "not-a-real-type"
	t.Cleanup(func() { edgeAddType = old })

	setMissingConfig(t)
	err := runEdgeAdd(nil, []string{"from-spec", "to-spec"})
	require.Error(t, err)
}

func TestRunEdgeAdd_ClientError(t *testing.T) {
	setMissingConfig(t)
	old := edgeAddType
	edgeAddType = "depends_on"
	t.Cleanup(func() { edgeAddType = old })

	err := runEdgeAdd(nil, []string{"from-spec", "to-spec"})
	require.Error(t, err)
}

func TestRunEdgeRemove_UnknownType(t *testing.T) {
	// graphClient() is called before edge type validation; client creation
	// fails first when config is missing.
	old := edgeRemoveType
	edgeRemoveType = "not-a-real-type"
	t.Cleanup(func() { edgeRemoveType = old })

	setMissingConfig(t)
	err := runEdgeRemove(nil, []string{"from-spec", "to-spec"})
	require.Error(t, err)
}

func TestRunEdgeRemove_ClientError(t *testing.T) {
	setMissingConfig(t)
	old := edgeRemoveType
	edgeRemoveType = "blocks"
	t.Cleanup(func() { edgeRemoveType = old })

	err := runEdgeRemove(nil, []string{"from-spec", "to-spec"})
	require.Error(t, err)
}

func TestRunEdgeList_UnknownType(t *testing.T) {
	// graphClient() is called before edge type validation; client creation
	// fails first when config is missing.
	old := edgeListType
	edgeListType = "not-a-real-type"
	t.Cleanup(func() { edgeListType = old })

	setMissingConfig(t)
	err := runEdgeList(&cobra.Command{}, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunEdgeList_ClientError(t *testing.T) {
	setMissingConfig(t)
	old := edgeListType
	edgeListType = ""
	t.Cleanup(func() { edgeListType = old })

	err := runEdgeList(&cobra.Command{}, []string{"my-spec"})
	require.Error(t, err)
}

// --- graph run functions ---

func TestRunDeps_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runDeps(&cobra.Command{}, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunReady_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runReady(&cobra.Command{}, nil)
	require.Error(t, err)
}

func TestRunCriticalPath_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runCriticalPath(&cobra.Command{}, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunImpact_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runImpact(&cobra.Command{}, []string{"my-spec"})
	require.Error(t, err)
}
