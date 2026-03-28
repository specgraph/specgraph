// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
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
