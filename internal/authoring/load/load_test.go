// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package load_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring/load"
)

// --- Spark ---

func TestSparkFromYAML_Full(t *testing.T) {
	yaml := []byte(`seed: build a webhook dispatcher
signal: customers keep asking for callbacks
questions:
  - which events?
  - retry policy?
scope_sniff: medium
kill_test: nobody uses webhooks after 30 days
`)
	out, err := load.SparkFromYAML(yaml)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "build a webhook dispatcher", out.GetSeed())
	assert.Equal(t, "customers keep asking for callbacks", out.GetSignal())
	assert.Equal(t, []string{"which events?", "retry policy?"}, out.GetQuestions())
	assert.Equal(t, specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM, out.GetScopeSniff())
	assert.Equal(t, "nobody uses webhooks after 30 days", out.GetKillTest())
}

func TestSparkFromYAML_Minimal(t *testing.T) {
	// Mirrors the minimal 06-05 e2e payload: only seed/signal/scope_sniff.
	yaml := []byte(`seed: idea
signal: why now
scope_sniff: tiny
`)
	out, err := load.SparkFromYAML(yaml)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "idea", out.GetSeed())
	assert.Equal(t, "why now", out.GetSignal())
	assert.Equal(t, specv1.ScopeSniff_SCOPE_SNIFF_TINY, out.GetScopeSniff())
	assert.Empty(t, out.GetQuestions())
	assert.Equal(t, "", out.GetKillTest())
}

func TestSparkFromYAML_AllScopeSniffValues(t *testing.T) {
	cases := map[string]specv1.ScopeSniff{
		"tiny":   specv1.ScopeSniff_SCOPE_SNIFF_TINY,
		"small":  specv1.ScopeSniff_SCOPE_SNIFF_SMALL,
		"medium": specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM,
		"large":  specv1.ScopeSniff_SCOPE_SNIFF_LARGE,
		"epic":   specv1.ScopeSniff_SCOPE_SNIFF_EPIC,
	}
	for in, want := range cases {
		yaml := []byte("seed: s\nscope_sniff: " + in + "\n")
		out, err := load.SparkFromYAML(yaml)
		require.NoError(t, err, "scope_sniff=%s", in)
		assert.Equal(t, want, out.GetScopeSniff(), "scope_sniff=%s", in)
	}
}

func TestSparkFromYAML_InvalidScopeSniff(t *testing.T) {
	yaml := []byte(`seed: s
scope_sniff: bogus
`)
	out, err := load.SparkFromYAML(yaml)
	require.Error(t, err)
	assert.Nil(t, out)
}

func TestSparkFromYAML_Malformed(t *testing.T) {
	_, err := load.SparkFromYAML([]byte(`seed: [unclosed`))
	require.Error(t, err)
}

// --- Shape ---

func TestShapeFromYAML_FullNested(t *testing.T) {
	yaml := []byte(`scope_in:
  - deliver webhooks
scope_out:
  - inbound events
approaches:
  - name: queue-based
    description: enqueue and drain with workers
    tradeoffs:
      - pro durable
      - con more infra
chosen_approach: queue-based
risks:
  - poison messages
success_must:
  - at-least-once delivery
success_should:
  - dashboard
success_wont:
  - exactly-once
decisions:
  - slug: retry-policy
    title: Retry policy
    decision: exponential backoff
    rationale: avoids thundering herd
`)
	out, err := load.ShapeFromYAML(yaml)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, []string{"deliver webhooks"}, out.GetScopeIn())
	assert.Equal(t, []string{"inbound events"}, out.GetScopeOut())
	assert.Equal(t, "queue-based", out.GetChosenApproach())
	assert.Equal(t, []string{"poison messages"}, out.GetRisks())
	assert.Equal(t, []string{"at-least-once delivery"}, out.GetSuccessMust())
	assert.Equal(t, []string{"dashboard"}, out.GetSuccessShould())
	assert.Equal(t, []string{"exactly-once"}, out.GetSuccessWont())

	require.Len(t, out.GetApproaches(), 1)
	ap := out.GetApproaches()[0]
	assert.Equal(t, "queue-based", ap.GetName())
	assert.Equal(t, "enqueue and drain with workers", ap.GetDescription())
	assert.Equal(t, []string{"pro durable", "con more infra"}, ap.GetTradeoffs())

	require.Len(t, out.GetDecisions(), 1)
	d := out.GetDecisions()[0]
	assert.Equal(t, "retry-policy", d.GetSlug())
	assert.Equal(t, "Retry policy", d.GetTitle())
	assert.Equal(t, "exponential backoff", d.GetDecision())
	assert.Equal(t, "avoids thundering herd", d.GetRationale())
}

func TestShapeFromYAML_Minimal(t *testing.T) {
	yaml := []byte(`scope_in:
  - a
scope_out:
  - b
approaches:
  - name: only
chosen_approach: only
`)
	out, err := load.ShapeFromYAML(yaml)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, []string{"a"}, out.GetScopeIn())
	assert.Equal(t, []string{"b"}, out.GetScopeOut())
	require.Len(t, out.GetApproaches(), 1)
	assert.Equal(t, "only", out.GetApproaches()[0].GetName())
	assert.Empty(t, out.GetApproaches()[0].GetTradeoffs())
	assert.Equal(t, "only", out.GetChosenApproach())
	assert.Empty(t, out.GetDecisions())
	assert.Empty(t, out.GetRisks())
}

func TestShapeFromYAML_Malformed(t *testing.T) {
	_, err := load.ShapeFromYAML([]byte(`approaches: [unclosed`))
	require.Error(t, err)
}

// --- Specify ---

func TestSpecifyFromYAML_FullNested(t *testing.T) {
	yaml := []byte(`interfaces:
  - name: WebhookService proto
    body: rpc Dispatch(...) returns (...)
verify_criteria:
  - category: emission
    description: emits an event on dispatch
invariants:
  - never drops an accepted event
touches:
  - path: internal/webhook/dispatch.go
    purpose: core dispatch loop
    change_type: new
`)
	out, err := load.SpecifyFromYAML(yaml)
	require.NoError(t, err)
	require.NotNil(t, out)

	require.Len(t, out.GetInterfaces(), 1)
	iface := out.GetInterfaces()[0]
	assert.Equal(t, "WebhookService proto", iface.GetName())
	assert.Equal(t, "rpc Dispatch(...) returns (...)", iface.GetBody())

	require.Len(t, out.GetVerifyCriteria(), 1)
	vc := out.GetVerifyCriteria()[0]
	assert.Equal(t, "emission", vc.GetCategory())
	assert.Equal(t, "emits an event on dispatch", vc.GetDescription())

	assert.Equal(t, []string{"never drops an accepted event"}, out.GetInvariants())

	require.Len(t, out.GetTouches(), 1)
	tc := out.GetTouches()[0]
	assert.Equal(t, "internal/webhook/dispatch.go", tc.GetPath())
	assert.Equal(t, "core dispatch loop", tc.GetPurpose())
	assert.Equal(t, "new", tc.GetChangeType())
}

func TestSpecifyFromYAML_Minimal(t *testing.T) {
	yaml := []byte(`interfaces:
  - name: only
    body: sig
verify_criteria:
  - category: c
    description: d
`)
	out, err := load.SpecifyFromYAML(yaml)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, out.GetInterfaces(), 1)
	assert.Equal(t, "only", out.GetInterfaces()[0].GetName())
	require.Len(t, out.GetVerifyCriteria(), 1)
	assert.Equal(t, "c", out.GetVerifyCriteria()[0].GetCategory())
	assert.Empty(t, out.GetInvariants())
	assert.Empty(t, out.GetTouches())
}

func TestSpecifyFromYAML_Malformed(t *testing.T) {
	_, err := load.SpecifyFromYAML([]byte(`interfaces: [unclosed`))
	require.Error(t, err)
}

// --- Decompose ---

func TestDecomposeFromYAML_FullNested(t *testing.T) {
	yaml := []byte(`strategy: steel_thread
slices:
  - id: thread
    intent: prove integration
    verify:
      - end-to-end passes
    touches:
      - internal/webhook/
    depends_on: []
  - id: second
    intent: add retries
    verify:
      - retries on failure
    touches:
      - internal/webhook/retry.go
    depends_on:
      - thread
`)
	out, err := load.DecomposeFromYAML(yaml)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD, out.GetStrategy())
	require.Len(t, out.GetSlices(), 2)

	first := out.GetSlices()[0]
	assert.Equal(t, "thread", first.GetId())
	assert.Equal(t, "prove integration", first.GetIntent())
	assert.Equal(t, []string{"end-to-end passes"}, first.GetVerify())
	assert.Equal(t, []string{"internal/webhook/"}, first.GetTouches())
	assert.Empty(t, first.GetDependsOn())

	second := out.GetSlices()[1]
	assert.Equal(t, "second", second.GetId())
	assert.Equal(t, []string{"thread"}, second.GetDependsOn())
}

func TestDecomposeFromYAML_Minimal(t *testing.T) {
	// Mirrors the minimal 06-05 e2e payload: single_unit + one slice with id+intent.
	yaml := []byte(`strategy: single_unit
slices:
  - id: all
    intent: ship it all at once
`)
	out, err := load.DecomposeFromYAML(yaml)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT, out.GetStrategy())
	require.Len(t, out.GetSlices(), 1)
	assert.Equal(t, "all", out.GetSlices()[0].GetId())
	assert.Equal(t, "ship it all at once", out.GetSlices()[0].GetIntent())
	assert.Empty(t, out.GetSlices()[0].GetVerify())
	assert.Empty(t, out.GetSlices()[0].GetTouches())
	assert.Empty(t, out.GetSlices()[0].GetDependsOn())
}

func TestDecomposeFromYAML_MultiTokenStrategies(t *testing.T) {
	cases := map[string]specv1.DecompositionStrategy{
		"vertical_slice": specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
		"layer_cake":     specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE,
		"steel_thread":   specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
		"single_unit":    specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT,
	}
	for in, want := range cases {
		yaml := []byte("strategy: " + in + "\n")
		out, err := load.DecomposeFromYAML(yaml)
		require.NoError(t, err, "strategy=%s", in)
		assert.Equal(t, want, out.GetStrategy(), "strategy=%s", in)
	}
}

func TestDecomposeFromYAML_InvalidStrategy(t *testing.T) {
	yaml := []byte(`strategy: bogus
`)
	out, err := load.DecomposeFromYAML(yaml)
	require.Error(t, err)
	assert.Nil(t, out)
}

func TestDecomposeFromYAML_Malformed(t *testing.T) {
	_, err := load.DecomposeFromYAML([]byte(`slices: [unclosed`))
	require.Error(t, err)
}
