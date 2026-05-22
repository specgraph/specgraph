// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake handlers ---

// fakePrimeHandler returns a project view when slug is empty and a spec
// view otherwise. The project view carries a constitution and a single
// constraint so provenance assertions have something to chew on.
type fakePrimeHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakePrimeHandler) GetPrime(_ context.Context, req *connect.Request[specv1.GetPrimeRequest]) (*connect.Response[specv1.PrimeResponse], error) {
	if slug := req.Msg.GetSlug(); slug != "" {
		return connect.NewResponse(&specv1.PrimeResponse{
			View: &specv1.PrimeResponse_SpecView{
				SpecView: &specv1.SpecView{
					Spec: &specv1.Spec{
						Slug:     slug,
						Stage:    "specify",
						Priority: "high",
						Intent:   "a thing that does a thing",
					},
				},
			},
		}), nil
	}
	return connect.NewResponse(&specv1.PrimeResponse{
		View: &specv1.PrimeResponse_ProjectView{
			ProjectView: &specv1.ProjectView{
				Constitution: &specv1.Constitution{
					Tech: &specv1.TechConfig{
						Languages: &specv1.LanguageConfig{Primary: "go"},
					},
					Constraints: []string{"use-pgx-v5"},
				},
				ConstitutionProvenance: []*specv1.ProvenanceEntry{
					{Path: "tech_config.languages.primary", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
					{Path: "constraints[use-pgx-v5]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
				},
			},
		},
	}), nil
}

type fakePrimeNotFoundHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakePrimeNotFoundHandler) GetPrime(_ context.Context, _ *connect.Request[specv1.GetPrimeRequest]) (*connect.Response[specv1.PrimeResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

// --- helpers ---

// stubRunUp swaps runUpFn with a recorder for the duration of a test.
// It returns a pointer to the bool so callers can assert that runUp was
// invoked. This is the testability concession Task 7 calls out
// explicitly — runUp itself touches OS-level service state and is
// unsuitable for unit tests, but its invocation is load-bearing for
// Claude Code's SessionStart hook so the test verifies the call site.
func stubRunUp(t *testing.T) *bool {
	t.Helper()
	called := false
	old := runUpFn
	runUpFn = func(_ *cobra.Command, _ []string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { runUpFn = old })
	return &called
}

func newPrimeCmd(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	return cmd, &out, &errBuf
}

func resetPrimeFlags(t *testing.T) {
	t.Helper()
	oldProv, oldJSON := primeShowProvenance, primeJSON
	primeShowProvenance = false
	primeJSON = false
	t.Cleanup(func() {
		primeShowProvenance = oldProv
		primeJSON = oldJSON
	})
}

// --- tests ---

func TestRunPrime_PreservesRunUp(t *testing.T) {
	startFakeExecutionServer(t, fakePrimeHandler{})
	called := stubRunUp(t)
	resetPrimeFlags(t)

	cmd, _, _ := newPrimeCmd(t)
	err := runPrime(cmd, nil)
	require.NoError(t, err)
	assert.True(t, *called, "runUp must be invoked at the start of prime; Claude Code's SessionStart hook depends on it")
}

func TestRunPrime_RunUpErrorIsNonFatal(t *testing.T) {
	startFakeExecutionServer(t, fakePrimeHandler{})
	resetPrimeFlags(t)

	old := runUpFn
	runUpFn = func(_ *cobra.Command, _ []string) error {
		return assert.AnError
	}
	t.Cleanup(func() { runUpFn = old })

	cmd, _, errBuf := newPrimeCmd(t)
	err := runPrime(cmd, nil)
	require.NoError(t, err, "runUp failures must NOT abort prime; the server may already be running in manual mode")
	assert.Contains(t, errBuf.String(), "warning: up:")
}

func TestRunPrime_ProjectScope_Markdown(t *testing.T) {
	startFakeExecutionServer(t, fakePrimeHandler{})
	stubRunUp(t)
	resetPrimeFlags(t)

	cmd, out, _ := newPrimeCmd(t)
	err := runPrime(cmd, nil)
	require.NoError(t, err)

	body := out.String()
	assert.Contains(t, body, "# SpecGraph Session Prime")
	assert.Contains(t, body, "Top constraints:")
	assert.Contains(t, body, "use-pgx-v5")
	// Provenance is OFF by default — the (set by: …) markers must not appear.
	assert.NotContains(t, body, "(set by:")
}

func TestRunPrime_SpecScope_Markdown(t *testing.T) {
	startFakeExecutionServer(t, fakePrimeHandler{})
	stubRunUp(t)
	resetPrimeFlags(t)

	cmd, out, _ := newPrimeCmd(t)
	err := runPrime(cmd, []string{"my-spec"})
	require.NoError(t, err)

	body := out.String()
	assert.Contains(t, body, "# Prime: my-spec")
	assert.Contains(t, body, "Stage: specify")
	assert.Contains(t, body, "Priority: high")
}

func TestRunPrime_JSON_Project(t *testing.T) {
	startFakeExecutionServer(t, fakePrimeHandler{})
	stubRunUp(t)
	resetPrimeFlags(t)
	primeJSON = true

	cmd, out, _ := newPrimeCmd(t)
	err := runPrime(cmd, nil)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &decoded), "output must be valid JSON")
	assert.Contains(t, decoded, "constitution", "project JSON should carry the constitution field")
	// Provenance omitted by default.
	assert.NotContains(t, decoded, "constitutionProvenance")
}

func TestRunPrime_JSON_Spec(t *testing.T) {
	startFakeExecutionServer(t, fakePrimeHandler{})
	stubRunUp(t)
	resetPrimeFlags(t)
	primeJSON = true

	cmd, out, _ := newPrimeCmd(t)
	err := runPrime(cmd, []string{"my-spec"})
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &decoded), "output must be valid JSON")
	assert.Contains(t, decoded, "spec", "spec JSON should carry the spec field")
}

func TestRunPrime_ShowProvenance(t *testing.T) {
	startFakeExecutionServer(t, fakePrimeHandler{})
	stubRunUp(t)
	resetPrimeFlags(t)
	primeShowProvenance = true

	cmd, out, _ := newPrimeCmd(t)
	err := runPrime(cmd, nil)
	require.NoError(t, err)

	body := out.String()
	assert.Contains(t, body, "(set by:", "show-provenance must annotate fields with their resolved layer")
	assert.Contains(t, body, "project") // language entry layer
	assert.Contains(t, body, "org")     // constraint layer
}

func TestRunPrime_UnknownSlug(t *testing.T) {
	startFakeExecutionServer(t, fakePrimeNotFoundHandler{})
	stubRunUp(t)
	resetPrimeFlags(t)

	cmd, _, _ := newPrimeCmd(t)
	err := runPrime(cmd, []string{"does-not-exist"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get prime")
}

func TestRunPrime_ClientError(t *testing.T) {
	setMissingConfig(t)
	stubRunUp(t)
	resetPrimeFlags(t)

	cmd, _, _ := newPrimeCmd(t)
	err := runPrime(cmd, nil)
	require.Error(t, err)
}

func TestPrimeCmd_AcceptsZeroOrOneArg(t *testing.T) {
	require.NoError(t, primeCmd.Args(primeCmd, []string{}))
	require.NoError(t, primeCmd.Args(primeCmd, []string{"slug"}))
	require.Error(t, primeCmd.Args(primeCmd, []string{"a", "b"}))
}
