// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/prime"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- Claim ---

func claimToProto(c *storage.Claim) *specv1.Claim {
	return &specv1.Claim{
		SpecSlug:     c.Slug,
		Agent:        c.Agent,
		ClaimedAt:    timeToProto(c.ClaimedAt),
		LeaseExpires: timeToProto(c.LeaseExpires),
	}
}

// --- Execution ---

var executionEventTypeToProtoMap = map[storage.ExecutionEventType]specv1.ExecutionEventType{
	storage.ExecutionEventTypeProgress:   specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS,
	storage.ExecutionEventTypeBlocker:    specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER,
	storage.ExecutionEventTypeCompletion: specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_COMPLETION,
}

func executionEventToProto(e *storage.ExecutionEvent) (*specv1.ExecutionEvent, error) {
	t, ok := executionEventTypeToProtoMap[e.Type]
	if !ok {
		return nil, fmt.Errorf("executionEventToProto: unknown execution event type %v for event %q", e.Type, e.ID)
	}
	return &specv1.ExecutionEvent{
		Id:        e.ID,
		SpecSlug:  e.SpecSlug,
		Agent:     e.Agent,
		Type:      t,
		Message:   e.Message,
		CreatedAt: timeToProto(e.CreatedAt),
	}, nil
}

func executionEventsToProto(events []*storage.ExecutionEvent) ([]*specv1.ExecutionEvent, error) {
	result := make([]*specv1.ExecutionEvent, len(events))
	for i, e := range events {
		pb, err := executionEventToProto(e)
		if err != nil {
			return nil, err
		}
		result[i] = pb
	}
	return result, nil
}

// --- Lint ---

var lintSeverityToProtoMap = map[storage.LintSeverity]specv1.LintSeverity{
	storage.LintSeverityError:   specv1.LintSeverity_LINT_SEVERITY_ERROR,
	storage.LintSeverityWarning: specv1.LintSeverity_LINT_SEVERITY_WARNING,
	storage.LintSeverityInfo:    specv1.LintSeverity_LINT_SEVERITY_INFO,
}

func lintViolationToProto(v *storage.LintViolation) (*specv1.LintViolation, error) {
	sev, ok := lintSeverityToProtoMap[v.Severity]
	if !ok {
		return nil, fmt.Errorf("lintViolationToProto: unknown lint severity %q for rule %q", v.Severity, v.Rule)
	}
	return &specv1.LintViolation{
		Rule:     v.Rule,
		Severity: sev,
		Message:  v.Message,
		Location: v.Location,
	}, nil
}

func lintResultToProto(r *storage.LintResult) (*specv1.LintResult, error) {
	violations := make([]*specv1.LintViolation, len(r.Violations))
	for i := range r.Violations {
		v, err := lintViolationToProto(&r.Violations[i])
		if err != nil {
			return nil, err
		}
		violations[i] = v
	}
	return &specv1.LintResult{
		SpecSlug:   r.SpecSlug,
		Violations: violations,
		Passed:     r.Passed,
		Error:      r.Error,
	}, nil
}

func lintResultsToProto(results []storage.LintResult) ([]*specv1.LintResult, error) {
	result := make([]*specv1.LintResult, len(results))
	for i := range results {
		r, err := lintResultToProto(&results[i])
		if err != nil {
			return nil, err
		}
		result[i] = r
	}
	return result, nil
}

func bundleToProto(b *storage.Bundle) (*specv1.Bundle, error) {
	spec, err := specToProto(b.Spec)
	if err != nil {
		return nil, err
	}
	decisions, err := decisionsToProto(b.Decisions)
	if err != nil {
		return nil, err
	}

	var callbacks *specv1.CallbackConfig
	if b.Callbacks != nil {
		callbacks = &specv1.CallbackConfig{
			Endpoint:   b.Callbacks.Endpoint,
			Prime:      b.Callbacks.Prime,
			Progress:   b.Callbacks.Progress,
			Blocker:    b.Callbacks.Blocker,
			Completion: b.Callbacks.Completion,
		}
	}

	return &specv1.Bundle{
		Version:   b.Version,
		Spec:      spec,
		Decisions: decisions,
		Bootstrap: b.Bootstrap,
		Callbacks: callbacks,
	}, nil
}

// --- Prime views ---

// provenanceEntriesToProto converts an ordered slice of domain
// ProvenanceEntry values to their proto representation. Order is
// preserved so callers can rely on backend-defined sort.
func provenanceEntriesToProto(entries []storage.ProvenanceEntry) []*specv1.ProvenanceEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]*specv1.ProvenanceEntry, 0, len(entries))
	for _, e := range entries {
		layer, ok := constitutionLayerToProtoMap[e.Layer]
		if !ok {
			layer = specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED
		}
		out = append(out, &specv1.ProvenanceEntry{
			Path:  e.Path,
			Layer: layer,
		})
	}
	return out
}

// convertStageCounts casts a domain map[string]int to the proto
// map[string]int32 shape.
func convertStageCounts(in map[string]int) map[string]int32 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int32, len(in))
	for k, v := range in {
		out[k] = int32(v) //nolint:gosec // bucket counts are bounded by spec count and non-negative
	}
	return out
}

// convertFindingsBySeverity converts a domain map keyed by
// FindingSeverity (a string) to the proto map keyed by the
// FindingSeverity enum's int32 value. Unknown severities are mapped to
// FINDING_SEVERITY_UNSPECIFIED so the conversion is lossy-but-total.
func convertFindingsBySeverity(in map[storage.FindingSeverity]int) map[int32]int32 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int32]int32, len(in))
	for k, v := range in {
		pb, err := findingSeverityToProto(k)
		if err != nil {
			pb = specv1.FindingSeverity_FINDING_SEVERITY_UNSPECIFIED
		}
		out[int32(pb)] += int32(v) //nolint:gosec // finding counts are non-negative and bounded
	}
	return out
}

// primeProjectViewToProto converts a domain ProjectView to its proto
// representation for inclusion in a PrimeResponse.
func primeProjectViewToProto(v *prime.ProjectView) (*specv1.ProjectView, error) {
	if v == nil {
		return nil, nil
	}
	ready, err := specsToProto(v.Ready)
	if err != nil {
		return nil, fmt.Errorf("primeProjectViewToProto: ready: %w", err)
	}
	out := &specv1.ProjectView{
		Constitution:           constitutionToProto(v.Constitution),
		ConstitutionProvenance: provenanceEntriesToProto(v.ConstitutionProvenance),
		GraphOverview: &specv1.GraphOverview{
			CountsByStage: convertStageCounts(v.GraphOverview.CountsByStage),
		},
		Ready:              ready,
		FindingsBySeverity: convertFindingsBySeverity(v.FindingsBySeverity),
		SkillsCount:        int32(v.SkillsCount), //nolint:gosec // skills count is bounded and non-negative
	}
	return out, nil
}

// primeSpecViewToProto converts a domain SpecView to its proto
// representation. The semantically 1:1 Claims slice carries zero or one
// active claim (populated by Composer.Spec from ClaimBackend.GetActiveClaim).
func primeSpecViewToProto(v *prime.SpecView) (*specv1.SpecView, error) {
	if v == nil {
		return nil, nil
	}
	spec, err := specToProto(v.Spec)
	if err != nil {
		return nil, fmt.Errorf("primeSpecViewToProto: spec: %w", err)
	}
	decisions, err := decisionsToProto(v.Decisions)
	if err != nil {
		return nil, fmt.Errorf("primeSpecViewToProto: decisions: %w", err)
	}
	slices, err := slicesToProto(v.Slices)
	if err != nil {
		return nil, fmt.Errorf("primeSpecViewToProto: slices: %w", err)
	}
	blockers, err := executionEventsToProto(v.Blockers)
	if err != nil {
		return nil, fmt.Errorf("primeSpecViewToProto: blockers: %w", err)
	}
	claims := make([]*specv1.Claim, 0, len(v.Claims))
	for _, c := range v.Claims {
		if c == nil {
			continue
		}
		claims = append(claims, claimToProto(c))
	}
	return &specv1.SpecView{
		Spec:                   spec,
		Constitution:           constitutionToProto(v.Constitution),
		ConstitutionProvenance: provenanceEntriesToProto(v.ConstitutionProvenance),
		Decisions:              decisions,
		Slices:                 slices,
		Claims:                 claims,
		Blockers:               blockers,
	}, nil
}
