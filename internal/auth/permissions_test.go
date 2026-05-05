// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

var allProcedures = []string{
	// SpecService
	specgraphv1connect.SpecServiceCreateSpecProcedure,
	specgraphv1connect.SpecServiceGetSpecProcedure,
	specgraphv1connect.SpecServiceListSpecsProcedure,
	specgraphv1connect.SpecServiceUpdateSpecProcedure,
	specgraphv1connect.SpecServiceListChangesProcedure,
	// DecisionService
	specgraphv1connect.DecisionServiceCreateDecisionProcedure,
	specgraphv1connect.DecisionServiceGetDecisionProcedure,
	specgraphv1connect.DecisionServiceListDecisionsProcedure,
	specgraphv1connect.DecisionServiceUpdateDecisionProcedure,
	// GraphService
	specgraphv1connect.GraphServiceGetFullGraphProcedure,
	specgraphv1connect.GraphServiceAddEdgeProcedure,
	specgraphv1connect.GraphServiceRemoveEdgeProcedure,
	specgraphv1connect.GraphServiceListEdgesProcedure,
	specgraphv1connect.GraphServiceGetDependenciesProcedure,
	specgraphv1connect.GraphServiceGetTransitiveDepsProcedure,
	specgraphv1connect.GraphServiceGetImpactProcedure,
	specgraphv1connect.GraphServiceGetReadyProcedure,
	specgraphv1connect.GraphServiceGetCriticalPathProcedure,
	// ClaimService
	specgraphv1connect.ClaimServiceClaimSpecProcedure,
	specgraphv1connect.ClaimServiceUnclaimSpecProcedure,
	specgraphv1connect.ClaimServiceHeartbeatProcedure,
	// ConstitutionService
	specgraphv1connect.ConstitutionServiceGetConstitutionProcedure,
	specgraphv1connect.ConstitutionServiceUpdateConstitutionProcedure,
	specgraphv1connect.ConstitutionServiceEmitToolFilesProcedure,
	// AuthoringService
	specgraphv1connect.AuthoringServiceSparkProcedure,
	specgraphv1connect.AuthoringServiceShapeProcedure,
	specgraphv1connect.AuthoringServiceSpecifyProcedure,
	specgraphv1connect.AuthoringServiceDecomposeProcedure,
	specgraphv1connect.AuthoringServiceApproveProcedure,
	specgraphv1connect.AuthoringServiceAmendProcedure,
	specgraphv1connect.AuthoringServiceSupersedeProcedure,
	specgraphv1connect.AuthoringServiceGetPromptsProcedure,
	specgraphv1connect.AuthoringServiceRecordConversationProcedure,
	specgraphv1connect.AuthoringServiceListConversationsProcedure,
	// ExecutionService
	specgraphv1connect.ExecutionServiceGenerateBundleProcedure,
	specgraphv1connect.ExecutionServiceGetPrimeProcedure,
	specgraphv1connect.ExecutionServiceReportProgressProcedure,
	specgraphv1connect.ExecutionServiceReportBlockerProcedure,
	specgraphv1connect.ExecutionServiceReportCompletionProcedure,
	specgraphv1connect.ExecutionServiceGetExecutionEventsProcedure,
	// LifecycleService
	specgraphv1connect.LifecycleServiceTransitionAmendProcedure,
	specgraphv1connect.LifecycleServiceTransitionSupersedeProcedure,
	specgraphv1connect.LifecycleServiceTransitionAbandonProcedure,
	specgraphv1connect.LifecycleServiceCheckDriftProcedure,
	specgraphv1connect.LifecycleServiceAcknowledgeDriftProcedure,
	specgraphv1connect.LifecycleServiceLintProcedure,
	// SyncService
	specgraphv1connect.SyncServiceSyncBeadsProcedure,
	specgraphv1connect.SyncServiceSyncGitHubProcedure,
	specgraphv1connect.SyncServiceGetSyncStatusProcedure,
	specgraphv1connect.SyncServiceInjectProcedure,
	// AnalyticalPassService
	specgraphv1connect.AnalyticalPassServiceRunAnalyticalPassProcedure,
	specgraphv1connect.AnalyticalPassServiceStoreFindingsProcedure,
	specgraphv1connect.AnalyticalPassServiceListFindingsProcedure,
	specgraphv1connect.AnalyticalPassServiceListProjectFindingsProcedure,
	// ServerService (exempt)
	specgraphv1connect.ServerServiceHealthProcedure,
}

func TestPermissionTable_Completeness(t *testing.T) {
	for _, proc := range allProcedures {
		if auth.IsExempt(proc) {
			continue
		}
		if _, ok := auth.RPCPermission(proc); !ok {
			t.Errorf("procedure %s is not in rpcPermissions and not exempt", proc)
		}
	}
}

func TestPermissionTable_NoExemptInPermissions(t *testing.T) {
	for _, proc := range allProcedures {
		if !auth.IsExempt(proc) {
			continue
		}
		if _, ok := auth.RPCPermission(proc); ok {
			t.Errorf("exempt procedure %s should not be in rpcPermissions", proc)
		}
	}
}
