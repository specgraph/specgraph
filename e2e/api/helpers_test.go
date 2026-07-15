// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// e2eProject is the project slug used by the E2E test server (set in testutil.StartServer).
const e2eProject = "e2e-test"

// projectClient returns an HTTP client that injects the X-Specgraph-Project header.
func projectClient() *http.Client {
	return &http.Client{
		Transport: &projectRoundTripper{
			base:    http.DefaultTransport,
			project: e2eProject,
		},
	}
}

type projectRoundTripper struct {
	base    http.RoundTripper
	project string
}

func (t *projectRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("X-Specgraph-Project", t.project)
	return t.base.RoundTrip(req)
}

func newSpecClient() specgraphv1connect.SpecServiceClient {
	return specgraphv1connect.NewSpecServiceClient(projectClient(), serverInfo.BaseURL)
}

func newClaimClient() specgraphv1connect.ClaimServiceClient {
	return specgraphv1connect.NewClaimServiceClient(projectClient(), serverInfo.BaseURL)
}

func newConstitutionClient() specgraphv1connect.ConstitutionServiceClient {
	return specgraphv1connect.NewConstitutionServiceClient(projectClient(), serverInfo.BaseURL)
}

func newGraphClient() specgraphv1connect.GraphServiceClient {
	return specgraphv1connect.NewGraphServiceClient(projectClient(), serverInfo.BaseURL)
}

func newAuthoringClient() specgraphv1connect.AuthoringServiceClient {
	return specgraphv1connect.NewAuthoringServiceClient(projectClient(), serverInfo.BaseURL)
}

func newDecisionClient() specgraphv1connect.DecisionServiceClient {
	return specgraphv1connect.NewDecisionServiceClient(projectClient(), serverInfo.BaseURL)
}

func newServerClient() specgraphv1connect.ServerServiceClient {
	return specgraphv1connect.NewServerServiceClient(http.DefaultClient, serverInfo.BaseURL)
}

func newLifecycleClient() specgraphv1connect.LifecycleServiceClient {
	return specgraphv1connect.NewLifecycleServiceClient(projectClient(), serverInfo.BaseURL)
}

func newExecutionClient() specgraphv1connect.ExecutionServiceClient {
	return specgraphv1connect.NewExecutionServiceClient(projectClient(), serverInfo.BaseURL)
}

func newSliceClient() specgraphv1connect.SliceServiceClient {
	return specgraphv1connect.NewSliceServiceClient(projectClient(), serverInfo.BaseURL)
}

// advanceStage advances a spec (already at "spark") through the authoring funnel
// to the target stage. Valid targets: "shape", "specify", "decompose", "approved",
// "in_progress" (claim without completing), "done" (claim + complete).
// The spec must have been created via CreateSpec or Spark before calling this.
// An optional http.Client may be passed to target a different project.
func advanceStage(ctx context.Context, slug, target string, httpClients ...*http.Client) error {
	hc := projectClient()
	if len(httpClients) > 0 && httpClients[0] != nil {
		hc = httpClients[0]
	}
	ac := specgraphv1connect.NewAuthoringServiceClient(hc, serverInfo.BaseURL)

	stages := []string{"shape", "specify", "decompose", "approved", "in_progress", "done"}
	targetIdx := -1
	for i, s := range stages {
		if s == target {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return fmt.Errorf("advanceStage: unknown target %q", target)
	}

	// shape
	if targetIdx >= 0 {
		_, err := ac.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug: slug,
			Output: &specv1.ShapeOutput{
				ScopeIn:        []string{"in-scope"},
				ScopeOut:       []string{"out-scope"},
				Approaches:     []*specv1.Approach{{Name: "default", Description: "test approach"}},
				ChosenApproach: "default",
			},
			ConversationExchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "what is in scope?", Stage: "shape", Sequence: 1},
				{Role: "response", Content: "in-scope only", Stage: "shape", Sequence: 2},
			},
		}))
		if err != nil {
			return fmt.Errorf("advanceStage shape: %w", err)
		}
	}
	if targetIdx < 1 {
		return nil
	}

	// specify
	_, err := ac.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
		Slug: slug,
		Output: &specv1.SpecifyOutput{
			Interfaces:     []*specv1.InterfaceSection{{Name: "API", Body: "test"}},
			VerifyCriteria: []*specv1.VerifyCriterion{{Description: "passes"}},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "what are the interfaces?", Stage: "specify", Sequence: 1},
			{Role: "response", Content: "API with test body", Stage: "specify", Sequence: 2},
		},
	}))
	if err != nil {
		return fmt.Errorf("advanceStage specify: %w", err)
	}
	if targetIdx < 2 {
		return nil
	}

	// decompose
	_, err = ac.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
		Slug: slug,
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT,
			Slices:   []*specv1.DecompositionSlice{{Id: "main", Intent: "test"}},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "how to decompose?", Stage: "decompose", Sequence: 1},
			{Role: "response", Content: "single unit", Stage: "decompose", Sequence: 2},
		},
	}))
	if err != nil {
		return fmt.Errorf("advanceStage decompose: %w", err)
	}
	if targetIdx < 3 {
		return nil
	}

	// approved
	_, err = ac.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
		Slug: slug,
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "ready to approve?", Stage: "approve", Sequence: 1},
			{Role: "response", Content: "approved", Stage: "approve", Sequence: 2},
		},
	}))
	if err != nil {
		return fmt.Errorf("advanceStage approve: %w", err)
	}
	if targetIdx < 4 {
		return nil
	}

	// in_progress: claim only (do not complete)
	const advanceAgent = "e2e-advance-agent"
	cc := specgraphv1connect.NewClaimServiceClient(hc, serverInfo.BaseURL)
	_, err = cc.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
		SpecSlug: slug,
		Agent:    advanceAgent,
	}))
	if err != nil {
		return fmt.Errorf("advanceStage claim: %w", err)
	}
	if targetIdx < 5 {
		return nil
	}

	// done: complete
	ec := specgraphv1connect.NewExecutionServiceClient(hc, serverInfo.BaseURL)
	_, err = ec.ReportCompletion(ctx, connect.NewRequest(&specv1.ReportCompletionRequest{
		Slug:  slug,
		Agent: advanceAgent,
	}))
	if err != nil {
		return fmt.Errorf("advanceStage complete: %w", err)
	}
	return nil
}

// claimAndComplete claims a spec (must be at approved or in_progress) and
// reports completion, advancing it to "done".
// An optional http.Client may be passed to target a different project.
func claimAndComplete(ctx context.Context, slug string, httpClients ...*http.Client) error {
	hc := projectClient()
	if len(httpClients) > 0 && httpClients[0] != nil {
		hc = httpClients[0]
	}
	const agent = "e2e-advance-agent"
	cc := specgraphv1connect.NewClaimServiceClient(hc, serverInfo.BaseURL)
	_, err := cc.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
		SpecSlug: slug,
		Agent:    agent,
	}))
	if err != nil {
		return fmt.Errorf("claimAndComplete claim: %w", err)
	}
	ec := specgraphv1connect.NewExecutionServiceClient(hc, serverInfo.BaseURL)
	_, err = ec.ReportCompletion(ctx, connect.NewRequest(&specv1.ReportCompletionRequest{
		Slug:  slug,
		Agent: agent,
	}))
	if err != nil {
		return fmt.Errorf("claimAndComplete done: %w", err)
	}
	return nil
}

// projectClientFor returns an HTTP client with a custom project header.
// Used by isolation tests to target different projects.
func projectClientFor(slug string) *http.Client {
	return &http.Client{
		Transport: &projectRoundTripper{
			base:    http.DefaultTransport,
			project: slug,
		},
	}
}
