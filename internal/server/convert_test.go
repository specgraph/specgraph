// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/driftscope"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecToProto(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	spec := &storage.Spec{
		ID: "spec-abc", Slug: "login", Intent: "Login API",
		Stage: "spark", Priority: "p1", Complexity: "medium",
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	pb, err := specToProto(spec)
	require.NoError(t, err)
	assert.Equal(t, "spec-abc", pb.Id)
	assert.Equal(t, "login", pb.Slug)
	assert.Equal(t, "Login API", pb.Intent)
	assert.Equal(t, "spark", pb.Stage)
	assert.Equal(t, "p1", pb.Priority)
	assert.Equal(t, "medium", pb.Complexity)
	assert.Equal(t, int32(1), pb.Version)
	require.NotNil(t, pb.CreatedAt)
	assert.Equal(t, now.Unix(), pb.CreatedAt.AsTime().Unix())
}

func TestSpecsToProto(t *testing.T) {
	specs := []*storage.Spec{
		{ID: "a", Slug: "a"},
		{ID: "b", Slug: "b"},
	}
	pbs, err := specsToProto(specs)
	require.NoError(t, err)
	assert.Len(t, pbs, 2)
	assert.Equal(t, "a", pbs[0].Id)
}

func TestTimeToProto_ZeroValue(t *testing.T) {
	result := timeToProto(time.Time{})
	assert.Nil(t, result, "zero time should produce nil timestamp")
}

func TestSpecToProto_ZeroTimes(t *testing.T) {
	spec := &storage.Spec{
		ID: "spec-zero", Slug: "zero-time", Intent: "test",
		Stage: "spark", Priority: "p2", Complexity: "low",
	}
	pb, err := specToProto(spec)
	require.NoError(t, err)
	assert.Nil(t, pb.CreatedAt, "zero CreatedAt should produce nil timestamp")
	assert.Nil(t, pb.UpdatedAt, "zero UpdatedAt should produce nil timestamp")
}

func TestDecisionToProto_ZeroTimes(t *testing.T) {
	d := &storage.Decision{
		ID: "dec-zero", Slug: "zero-time", Title: "test",
		Status: storage.DecisionStatusProposed,
	}
	pb, err := decisionToProto(d)
	require.NoError(t, err)
	assert.Nil(t, pb.CreatedAt, "zero CreatedAt should produce nil timestamp")
	assert.Nil(t, pb.UpdatedAt, "zero UpdatedAt should produce nil timestamp")
}

func TestDecisionToProto(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	d := &storage.Decision{
		ID: "dec-abc", Slug: "use-memgraph", Title: "Use Memgraph",
		Status: storage.DecisionStatusAccepted, Body: "We chose Memgraph",
		Rationale: "Graph-native", CreatedAt: now, UpdatedAt: now,
	}
	pb, err := decisionToProto(d)
	require.NoError(t, err)
	assert.Equal(t, "dec-abc", pb.Id)
	assert.Equal(t, "use-memgraph", pb.Slug)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, pb.Status)
}

func TestDecisionStatusToProto(t *testing.T) {
	v, err := decisionStatusToProto(storage.DecisionStatusProposed)
	require.NoError(t, err)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_PROPOSED, v)

	v, err = decisionStatusToProto(storage.DecisionStatusAccepted)
	require.NoError(t, err)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, v)

	v, err = decisionStatusToProto(storage.DecisionStatusSuperseded)
	require.NoError(t, err)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED, v)

	v, err = decisionStatusToProto(storage.DecisionStatusDeprecated)
	require.NoError(t, err)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_DEPRECATED, v)

	_, err = decisionStatusToProto("unknown")
	assert.Error(t, err)
}

func TestDecisionStatusFromProto(t *testing.T) {
	got, err := decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_PROPOSED)
	require.NoError(t, err)
	assert.Equal(t, storage.DecisionStatusProposed, got)

	got, err = decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_ACCEPTED)
	require.NoError(t, err)
	assert.Equal(t, storage.DecisionStatusAccepted, got)

	got, err = decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED)
	require.NoError(t, err)
	assert.Equal(t, storage.DecisionStatusSuperseded, got)

	got, err = decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_DEPRECATED)
	require.NoError(t, err)
	assert.Equal(t, storage.DecisionStatusDeprecated, got)

	_, err = decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_UNSPECIFIED)
	assert.Error(t, err)
}

func TestEdgeToProto(t *testing.T) {
	e := &storage.Edge{FromID: "a", ToID: "b", EdgeType: storage.EdgeTypeDependsOn}
	pb, err := edgeToProto(e)
	require.NoError(t, err)
	assert.Equal(t, "a", pb.FromId)
	assert.Equal(t, "b", pb.ToId)
	assert.Equal(t, specv1.EdgeType_EDGE_TYPE_DEPENDS_ON, pb.EdgeType)
}

func TestEdgeTypeFromProto(t *testing.T) {
	tests := []struct {
		proto  specv1.EdgeType
		domain storage.EdgeType
	}{
		{specv1.EdgeType_EDGE_TYPE_DEPENDS_ON, storage.EdgeTypeDependsOn},
		{specv1.EdgeType_EDGE_TYPE_BLOCKS, storage.EdgeTypeBlocks},
		{specv1.EdgeType_EDGE_TYPE_COMPOSES, storage.EdgeTypeComposes},
		{specv1.EdgeType_EDGE_TYPE_RELATES_TO, storage.EdgeTypeRelatesTo},
		{specv1.EdgeType_EDGE_TYPE_INFORMS, storage.EdgeTypeInforms},
		{specv1.EdgeType_EDGE_TYPE_DECIDED_IN, storage.EdgeTypeDecidedIn},
		{specv1.EdgeType_EDGE_TYPE_SUPERSEDES, storage.EdgeTypeSupersedes},
	}
	for _, tt := range tests {
		got, err := edgeTypeFromProto(tt.proto)
		require.NoError(t, err)
		assert.Equal(t, tt.domain, got)
	}

	_, err := edgeTypeFromProto(specv1.EdgeType_EDGE_TYPE_UNSPECIFIED)
	assert.Error(t, err)
}

func TestEdgeTypeToProto(t *testing.T) {
	tests := []struct {
		domain storage.EdgeType
		proto  specv1.EdgeType
	}{
		{storage.EdgeTypeDependsOn, specv1.EdgeType_EDGE_TYPE_DEPENDS_ON},
		{storage.EdgeTypeBlocks, specv1.EdgeType_EDGE_TYPE_BLOCKS},
		{storage.EdgeTypeComposes, specv1.EdgeType_EDGE_TYPE_COMPOSES},
		{storage.EdgeTypeRelatesTo, specv1.EdgeType_EDGE_TYPE_RELATES_TO},
		{storage.EdgeTypeInforms, specv1.EdgeType_EDGE_TYPE_INFORMS},
		{storage.EdgeTypeDecidedIn, specv1.EdgeType_EDGE_TYPE_DECIDED_IN},
		{storage.EdgeTypeSupersedes, specv1.EdgeType_EDGE_TYPE_SUPERSEDES},
	}
	for _, tt := range tests {
		got, err := edgeTypeToProto(tt.domain)
		require.NoError(t, err)
		assert.Equal(t, tt.proto, got)
	}

	_, err := edgeTypeToProto("unknown")
	assert.Error(t, err)
}

func TestHistoryToProto(t *testing.T) {
	t.Run("nil/empty returns nil", func(t *testing.T) {
		assert.Nil(t, historyToProto(nil))
		assert.Nil(t, historyToProto([]storage.HistoryEntry{}))
	})

	t.Run("converts entries", func(t *testing.T) {
		now := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
		entries := []storage.HistoryEntry{
			{Version: 1, Stage: "spark", Summary: "Created", Reason: "init", Date: now},
			{Version: 2, Stage: "shape", Summary: "Shaped", Reason: "refined", Date: now.Add(time.Hour)},
		}
		pbs := historyToProto(entries)
		require.Len(t, pbs, 2)

		assert.Equal(t, int32(1), pbs[0].Version)
		assert.Equal(t, "spark", pbs[0].Stage)
		assert.Equal(t, "Created", pbs[0].Summary)
		assert.Equal(t, "init", pbs[0].Reason)
		require.NotNil(t, pbs[0].Date)
		assert.Equal(t, now.Unix(), pbs[0].Date.AsTime().Unix())

		assert.Equal(t, int32(2), pbs[1].Version)
		assert.Equal(t, "shape", pbs[1].Stage)
	})

	t.Run("zero date produces nil timestamp", func(t *testing.T) {
		entries := []storage.HistoryEntry{
			{Version: 1, Stage: "spark", Summary: "s"},
		}
		pbs := historyToProto(entries)
		require.Len(t, pbs, 1)
		assert.Nil(t, pbs[0].Date)
	})
}

func TestLifecycleToProto(t *testing.T) {
	t.Run("task lifecycle", func(t *testing.T) {
		got, err := lifecycleToProto(storage.SpecLifecycleTask)
		require.NoError(t, err)
		assert.Equal(t, specv1.SpecLifecycle_SPEC_LIFECYCLE_TASK, got)
	})

	t.Run("living lifecycle", func(t *testing.T) {
		got, err := lifecycleToProto(storage.SpecLifecycleLiving)
		require.NoError(t, err)
		assert.Equal(t, specv1.SpecLifecycle_SPEC_LIFECYCLE_LIVING, got)
	})

	t.Run("empty string maps to unspecified", func(t *testing.T) {
		got, err := lifecycleToProto("")
		require.NoError(t, err)
		assert.Equal(t, specv1.SpecLifecycle_SPEC_LIFECYCLE_UNSPECIFIED, got)
	})

	t.Run("unknown lifecycle returns error", func(t *testing.T) {
		_, err := lifecycleToProto("bogus")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown lifecycle")
	})
}

func TestDriftReportToProto(t *testing.T) {
	t.Run("full report", func(t *testing.T) {
		report := &storage.DriftReport{
			SpecSlug:        "login-api",
			Acknowledged:    true,
			AcknowledgeNote: "accepted risk",
			Items: []storage.DriftItem{
				{
					Type:            storage.DriftTypeDependency,
					Severity:        storage.DriftSeverityHigh,
					Description:     "version mismatch",
					SpecSlug:        "login-api",
					UpstreamSlug:    "auth-core",
					ExpectedVersion: 2,
					ActualVersion:   1,
				},
			},
		}
		pb, err := driftReportToProto(report)
		require.NoError(t, err)
		assert.Equal(t, "login-api", pb.SpecSlug)
		assert.True(t, pb.Acknowledged)
		assert.Equal(t, "accepted risk", pb.AcknowledgeNote)
		require.Len(t, pb.Items, 1)
	})

	t.Run("empty items", func(t *testing.T) {
		report := &storage.DriftReport{SpecSlug: "s"}
		pb, err := driftReportToProto(report)
		require.NoError(t, err)
		assert.Empty(t, pb.Items)
	})
}

func TestDriftItemToProto(t *testing.T) {
	tests := []struct {
		name     string
		item     storage.DriftItem
		wantType specv1.DriftType
		wantSev  specv1.DriftSeverity
	}{
		{
			name:     "dependency/high",
			item:     storage.DriftItem{Type: storage.DriftTypeDependency, Severity: storage.DriftSeverityHigh, Description: "d", SpecSlug: "a", UpstreamSlug: "b", ExpectedVersion: 3, ActualVersion: 1},
			wantType: specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
			wantSev:  specv1.DriftSeverity_DRIFT_SEVERITY_HIGH,
		},
		{
			name:     "interface/medium",
			item:     storage.DriftItem{Type: storage.DriftTypeInterface, Severity: storage.DriftSeverityMedium, Description: "iface changed"},
			wantType: specv1.DriftType_DRIFT_TYPE_INTERFACE,
			wantSev:  specv1.DriftSeverity_DRIFT_SEVERITY_MEDIUM,
		},
		{
			name:     "verify/low",
			item:     storage.DriftItem{Type: storage.DriftTypeVerify, Severity: storage.DriftSeverityLow},
			wantType: specv1.DriftType_DRIFT_TYPE_VERIFY,
			wantSev:  specv1.DriftSeverity_DRIFT_SEVERITY_LOW,
		},
		{
			name:     "info severity",
			item:     storage.DriftItem{Type: storage.DriftTypeDependency, Severity: storage.DriftSeverityInfo},
			wantType: specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
			wantSev:  specv1.DriftSeverity_DRIFT_SEVERITY_INFO,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use driftReportToProto since driftItemToProto is inline
			report := &storage.DriftReport{Items: []storage.DriftItem{tt.item}}
			pb, err := driftReportToProto(report)
			require.NoError(t, err)
			require.Len(t, pb.Items, 1)
			got := pb.Items[0]
			assert.Equal(t, tt.wantType, got.Type)
			assert.Equal(t, tt.wantSev, got.Severity)
			assert.Equal(t, tt.item.Description, got.Description)
			assert.Equal(t, tt.item.SpecSlug, got.SpecSlug)
			assert.Equal(t, tt.item.UpstreamSlug, got.UpstreamSlug)
			assert.Equal(t, tt.item.ExpectedVersion, got.ExpectedVersion)
			assert.Equal(t, tt.item.ActualVersion, got.ActualVersion)
		})
	}
}

func TestSpecToProto_InvalidLifecycle(t *testing.T) {
	spec := &storage.Spec{
		ID: "spec-bad", Slug: "bad-lifecycle", Intent: "test",
		Stage: "spark", Priority: "p1", Complexity: "low",
		Lifecycle: storage.SpecLifecycle("bogus"),
	}
	_, err := specToProto(spec)
	assert.Error(t, err)
}

func TestDriftReportToProto_UnknownType(t *testing.T) {
	report := &storage.DriftReport{
		SpecSlug: "s",
		Items: []storage.DriftItem{
			{Type: storage.DriftType("unknown-type"), Severity: storage.DriftSeverityHigh},
		},
	}
	_, err := driftReportToProto(report)
	assert.Error(t, err)
}

func TestDriftReportToProto_UnknownSeverity(t *testing.T) {
	report := &storage.DriftReport{
		SpecSlug: "s",
		Items: []storage.DriftItem{
			{Type: storage.DriftTypeDependency, Severity: storage.DriftSeverity("unknown-severity")},
		},
	}
	_, err := driftReportToProto(report)
	assert.Error(t, err)
}

func TestLintResultsToProto(t *testing.T) {
	t.Run("multiple results with violations", func(t *testing.T) {
		results := []storage.LintResult{
			{
				SpecSlug: "login-api",
				Passed:   false,
				Violations: []storage.LintViolation{
					{Rule: "no-empty-intent", Severity: storage.LintSeverityError, Message: "intent is empty", Location: "spec.intent"},
					{Rule: "slug-format", Severity: storage.LintSeverityWarning, Message: "slug has uppercase", Location: "spec.slug"},
				},
			},
			{
				SpecSlug:   "auth-core",
				Passed:     true,
				Violations: nil,
			},
		}
		pbs, err := lintResultsToProto(results)
		require.NoError(t, err)
		require.Len(t, pbs, 2)

		// First result
		assert.Equal(t, "login-api", pbs[0].SpecSlug)
		assert.False(t, pbs[0].Passed)
		require.Len(t, pbs[0].Violations, 2)
		assert.Equal(t, "no-empty-intent", pbs[0].Violations[0].Rule)
		assert.Equal(t, specv1.LintSeverity_LINT_SEVERITY_ERROR, pbs[0].Violations[0].Severity)
		assert.Equal(t, "intent is empty", pbs[0].Violations[0].Message)
		assert.Equal(t, "spec.intent", pbs[0].Violations[0].Location)
		assert.Equal(t, specv1.LintSeverity_LINT_SEVERITY_WARNING, pbs[0].Violations[1].Severity)

		// Second result
		assert.Equal(t, "auth-core", pbs[1].SpecSlug)
		assert.True(t, pbs[1].Passed)
		assert.Empty(t, pbs[1].Violations)
	})

	t.Run("empty input", func(t *testing.T) {
		pbs, err := lintResultsToProto([]storage.LintResult{})
		require.NoError(t, err)
		assert.Empty(t, pbs)
	})

	t.Run("info severity maps correctly", func(t *testing.T) {
		results := []storage.LintResult{
			{
				SpecSlug: "s",
				Violations: []storage.LintViolation{
					{Rule: "r", Severity: storage.LintSeverityInfo, Message: "m"},
				},
			},
		}
		pbs, err := lintResultsToProto(results)
		require.NoError(t, err)
		require.Len(t, pbs, 1)
		require.Len(t, pbs[0].Violations, 1)
		assert.Equal(t, specv1.LintSeverity_LINT_SEVERITY_INFO, pbs[0].Violations[0].Severity)
	})
}


func TestLintResultsToProto_UnknownSeverity(t *testing.T) {
	results := []storage.LintResult{
		{
			SpecSlug: "s",
			Violations: []storage.LintViolation{
				{Rule: "r", Severity: storage.LintSeverity("bogus"), Message: "m"},
			},
		},
	}
	_, err := lintResultsToProto(results)
	assert.Error(t, err)
}

func TestDriftScopeFromProto(t *testing.T) {
	tests := []struct {
		name  string
		scope specv1.DriftScope
		want  string
		ok    bool
	}{
		{"UNSPECIFIED defaults to all scopes", specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED, "", true},
		{"DEPS maps to deps", specv1.DriftScope_DRIFT_SCOPE_DEPS, "deps", true},
		{"INTERFACES maps to interfaces", specv1.DriftScope_DRIFT_SCOPE_INTERFACES, "interfaces", true},
		{"VERIFY maps to verify", specv1.DriftScope_DRIFT_SCOPE_VERIFY, "verify", true},
		{"unknown scope returns false", specv1.DriftScope(99), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := driftScopeFromProto(tt.scope)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDriftScopeFromProtoMap_SyncWithDriftscope(t *testing.T) {
	for _, scopeStr := range driftScopeFromProtoMap {
		assert.True(t, driftscope.IsValid(scopeStr),
			"server scope %q not recognized by driftscope.IsValid — tables out of sync", scopeStr)
	}
}
