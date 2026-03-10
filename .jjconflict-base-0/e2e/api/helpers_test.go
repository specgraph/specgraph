// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"net/http"

	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

func newSpecClient() specgraphv1connect.SpecServiceClient {
	return specgraphv1connect.NewSpecServiceClient(http.DefaultClient, serverInfo.BaseURL)
}

func newClaimClient() specgraphv1connect.ClaimServiceClient {
	return specgraphv1connect.NewClaimServiceClient(http.DefaultClient, serverInfo.BaseURL)
}

func newConstitutionClient() specgraphv1connect.ConstitutionServiceClient {
	return specgraphv1connect.NewConstitutionServiceClient(http.DefaultClient, serverInfo.BaseURL)
}

func newGraphClient() specgraphv1connect.GraphServiceClient {
	return specgraphv1connect.NewGraphServiceClient(http.DefaultClient, serverInfo.BaseURL)
}

func newAuthoringClient() specgraphv1connect.AuthoringServiceClient {
	return specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, serverInfo.BaseURL)
}

func newDecisionClient() specgraphv1connect.DecisionServiceClient {
	return specgraphv1connect.NewDecisionServiceClient(http.DefaultClient, serverInfo.BaseURL)
}

func newServerClient() specgraphv1connect.ServerServiceClient {
	return specgraphv1connect.NewServerServiceClient(http.DefaultClient, serverInfo.BaseURL)
}

func newLifecycleClient() specgraphv1connect.LifecycleServiceClient {
	return specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, serverInfo.BaseURL)
}
