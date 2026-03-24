// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var rpcPermissions = map[string]string{
	// SpecService
	specgraphv1connect.SpecServiceGetSpecProcedure:    "spec:read",
	specgraphv1connect.SpecServiceListSpecsProcedure:  "spec:read",
	specgraphv1connect.SpecServiceCreateSpecProcedure: "spec:write",
	specgraphv1connect.SpecServiceUpdateSpecProcedure: "spec:write",
	// DecisionService
	specgraphv1connect.DecisionServiceGetDecisionProcedure:    "decision:read",
	specgraphv1connect.DecisionServiceListDecisionsProcedure:  "decision:read",
	specgraphv1connect.DecisionServiceCreateDecisionProcedure: "decision:write",
	specgraphv1connect.DecisionServiceUpdateDecisionProcedure: "decision:write",
	// GraphService
	specgraphv1connect.GraphServiceGetFullGraphProcedure:      "graph:read",
	specgraphv1connect.GraphServiceGetDependenciesProcedure:   "graph:read",
	specgraphv1connect.GraphServiceGetTransitiveDepsProcedure: "graph:read",
	specgraphv1connect.GraphServiceGetImpactProcedure:         "graph:read",
	specgraphv1connect.GraphServiceGetReadyProcedure:          "graph:read",
	specgraphv1connect.GraphServiceGetCriticalPathProcedure:   "graph:read",
	specgraphv1connect.GraphServiceListEdgesProcedure:         "graph:read",
	specgraphv1connect.GraphServiceAddEdgeProcedure:           "graph:write",
	specgraphv1connect.GraphServiceRemoveEdgeProcedure:        "graph:delete",
	// ClaimService
	specgraphv1connect.ClaimServiceClaimSpecProcedure:   "claim:write",
	specgraphv1connect.ClaimServiceHeartbeatProcedure:   "claim:write",
	specgraphv1connect.ClaimServiceUnclaimSpecProcedure: "claim:write",
	// ConstitutionService
	specgraphv1connect.ConstitutionServiceGetConstitutionProcedure:    "constitution:read",
	specgraphv1connect.ConstitutionServiceUpdateConstitutionProcedure: "constitution:write",
	specgraphv1connect.ConstitutionServiceEmitToolFilesProcedure:      "constitution:write",
	// AuthoringService
	specgraphv1connect.AuthoringServiceGetPromptsProcedure: "authoring:read",
	specgraphv1connect.AuthoringServiceSparkProcedure:      "authoring:write",
	specgraphv1connect.AuthoringServiceShapeProcedure:      "authoring:write",
	specgraphv1connect.AuthoringServiceSpecifyProcedure:    "authoring:write",
	specgraphv1connect.AuthoringServiceDecomposeProcedure:  "authoring:write",
	specgraphv1connect.AuthoringServiceApproveProcedure:    "authoring:write",
	specgraphv1connect.AuthoringServiceAmendProcedure:      "authoring:write",
	specgraphv1connect.AuthoringServiceSupersedeProcedure:  "authoring:write",
	// ExecutionService
	specgraphv1connect.ExecutionServiceGenerateBundleProcedure:     "execution:read",
	specgraphv1connect.ExecutionServiceGetPrimeProcedure:           "execution:read",
	specgraphv1connect.ExecutionServiceGetExecutionEventsProcedure: "execution:read",
	specgraphv1connect.ExecutionServiceReportProgressProcedure:     "execution:write",
	specgraphv1connect.ExecutionServiceReportBlockerProcedure:      "execution:write",
	specgraphv1connect.ExecutionServiceReportCompletionProcedure:   "execution:write",
	// LifecycleService
	specgraphv1connect.LifecycleServiceCheckDriftProcedure:          "lifecycle:read",
	specgraphv1connect.LifecycleServiceLintProcedure:                "lifecycle:read",
	specgraphv1connect.LifecycleServiceAcknowledgeDriftProcedure:    "lifecycle:write",
	specgraphv1connect.LifecycleServiceTransitionAmendProcedure:     "lifecycle:write",
	specgraphv1connect.LifecycleServiceTransitionSupersedeProcedure: "lifecycle:write",
	specgraphv1connect.LifecycleServiceTransitionAbandonProcedure:   "lifecycle:write",
	// SyncService
	specgraphv1connect.SyncServiceGetSyncStatusProcedure: "sync:read",
	specgraphv1connect.SyncServiceSyncBeadsProcedure:     "sync:write",
	specgraphv1connect.SyncServiceSyncGitHubProcedure:    "sync:write",
	specgraphv1connect.SyncServiceInjectProcedure:        "sync:write",
	// AnalyticalPassService
	specgraphv1connect.AnalyticalPassServiceRunAnalyticalPassProcedure: "analytical_pass:write",
	specgraphv1connect.AnalyticalPassServiceStoreFindingsProcedure:     "analytical_pass:write",
	specgraphv1connect.AnalyticalPassServiceListFindingsProcedure:      "analytical_pass:read",
}

var exemptProcedures = map[string]bool{
	specgraphv1connect.ServerServiceHealthProcedure: true,
}

// RPCPermission returns the required permission for a procedure.
func RPCPermission(procedure string) (string, bool) {
	perm, ok := rpcPermissions[procedure]
	return perm, ok
}

// IsExempt reports whether a procedure is exempt from authentication.
func IsExempt(procedure string) bool {
	return exemptProcedures[procedure]
}
