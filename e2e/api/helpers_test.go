// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"net/http"

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
