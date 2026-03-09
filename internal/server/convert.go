// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"
	"log/slog"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// timeToProto converts a time.Time to a protobuf Timestamp, returning nil for zero values.
func timeToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// --- Spec ---

func specToProto(s *storage.Spec) (*specv1.Spec, error) {
	lc, err := lifecycleToProto(s.Lifecycle)
	if err != nil {
		return nil, fmt.Errorf("spec %q: %w", s.Slug, err)
	}
	return &specv1.Spec{
		Id:           s.ID,
		Slug:         s.Slug,
		Intent:       s.Intent,
		Stage:        string(s.Stage),
		Priority:     string(s.Priority),
		Complexity:   s.Complexity,
		Version:      s.Version,
		CreatedAt:    timeToProto(s.CreatedAt),
		UpdatedAt:    timeToProto(s.UpdatedAt),
		Lifecycle:    lc,
		SupersededBy: s.SupersededBy,
		Supersedes:   s.Supersedes,
		History:      historyToProto(s.History),
	}, nil
}

func specsToProto(specs []*storage.Spec) ([]*specv1.Spec, error) {
	result := make([]*specv1.Spec, len(specs))
	for i, s := range specs {
		pb, err := specToProto(s)
		if err != nil {
			return nil, err
		}
		result[i] = pb
	}
	return result, nil
}

// --- Decision ---

var decisionStatusToProtoMap = map[storage.DecisionStatus]specv1.DecisionStatus{
	storage.DecisionStatusProposed:   specv1.DecisionStatus_DECISION_STATUS_PROPOSED,
	storage.DecisionStatusAccepted:   specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
	storage.DecisionStatusSuperseded: specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED,
	storage.DecisionStatusDeprecated: specv1.DecisionStatus_DECISION_STATUS_DEPRECATED,
}

var decisionStatusFromProtoMap = map[specv1.DecisionStatus]storage.DecisionStatus{
	specv1.DecisionStatus_DECISION_STATUS_PROPOSED:   storage.DecisionStatusProposed,
	specv1.DecisionStatus_DECISION_STATUS_ACCEPTED:   storage.DecisionStatusAccepted,
	specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED: storage.DecisionStatusSuperseded,
	specv1.DecisionStatus_DECISION_STATUS_DEPRECATED: storage.DecisionStatusDeprecated,
}

func decisionStatusToProto(s storage.DecisionStatus) (specv1.DecisionStatus, error) {
	if v, ok := decisionStatusToProtoMap[s]; ok {
		return v, nil
	}
	return specv1.DecisionStatus_DECISION_STATUS_UNSPECIFIED, fmt.Errorf("unknown decision status: %q", s)
}

func decisionStatusFromProto(s specv1.DecisionStatus) (storage.DecisionStatus, error) {
	if v, ok := decisionStatusFromProtoMap[s]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown decision status: %v", s)
}

func decisionToProto(d *storage.Decision) (*specv1.Decision, error) {
	status, err := decisionStatusToProto(d.Status)
	if err != nil {
		return nil, err
	}
	return &specv1.Decision{
		Id:           d.ID,
		Slug:         d.Slug,
		Title:        d.Title,
		Status:       status,
		Decision:     d.Body,
		Rationale:    d.Rationale,
		SupersededBy: d.SupersededBy,
		CreatedAt:    timeToProto(d.CreatedAt),
		UpdatedAt:    timeToProto(d.UpdatedAt),
	}, nil
}

func decisionsToProto(decisions []*storage.Decision) ([]*specv1.Decision, error) {
	result := make([]*specv1.Decision, len(decisions))
	for i, d := range decisions {
		pb, err := decisionToProto(d)
		if err != nil {
			return nil, err
		}
		result[i] = pb
	}
	return result, nil
}

// --- Edge ---

var edgeTypeToProtoMap = map[storage.EdgeType]specv1.EdgeType{
	storage.EdgeTypeDependsOn:  specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	storage.EdgeTypeBlocks:     specv1.EdgeType_EDGE_TYPE_BLOCKS,
	storage.EdgeTypeComposes:   specv1.EdgeType_EDGE_TYPE_COMPOSES,
	storage.EdgeTypeRelatesTo:  specv1.EdgeType_EDGE_TYPE_RELATES_TO,
	storage.EdgeTypeInforms:    specv1.EdgeType_EDGE_TYPE_INFORMS,
	storage.EdgeTypeDecidedIn:  specv1.EdgeType_EDGE_TYPE_DECIDED_IN,
	storage.EdgeTypeSupersedes: specv1.EdgeType_EDGE_TYPE_SUPERSEDES,
}

var edgeTypeFromProtoMap = map[specv1.EdgeType]storage.EdgeType{
	specv1.EdgeType_EDGE_TYPE_DEPENDS_ON: storage.EdgeTypeDependsOn,
	specv1.EdgeType_EDGE_TYPE_BLOCKS:     storage.EdgeTypeBlocks,
	specv1.EdgeType_EDGE_TYPE_COMPOSES:   storage.EdgeTypeComposes,
	specv1.EdgeType_EDGE_TYPE_RELATES_TO: storage.EdgeTypeRelatesTo,
	specv1.EdgeType_EDGE_TYPE_INFORMS:    storage.EdgeTypeInforms,
	specv1.EdgeType_EDGE_TYPE_DECIDED_IN: storage.EdgeTypeDecidedIn,
	specv1.EdgeType_EDGE_TYPE_SUPERSEDES: storage.EdgeTypeSupersedes,
}

func edgeTypeToProto(e storage.EdgeType) (specv1.EdgeType, error) {
	if v, ok := edgeTypeToProtoMap[e]; ok {
		return v, nil
	}
	return specv1.EdgeType_EDGE_TYPE_UNSPECIFIED, fmt.Errorf("unknown edge type: %q", e)
}

func edgeTypeFromProto(e specv1.EdgeType) (storage.EdgeType, error) {
	if v, ok := edgeTypeFromProtoMap[e]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown edge type: %v", e)
}

func edgeToProto(e *storage.Edge) (*specv1.Edge, error) {
	et, err := edgeTypeToProto(e.EdgeType)
	if err != nil {
		return nil, err
	}
	return &specv1.Edge{
		FromId:   e.FromID,
		ToId:     e.ToID,
		EdgeType: et,
	}, nil
}

func edgesToProto(edges []*storage.Edge) ([]*specv1.Edge, error) {
	result := make([]*specv1.Edge, len(edges))
	for i, e := range edges {
		pb, err := edgeToProto(e)
		if err != nil {
			return nil, err
		}
		result[i] = pb
	}
	return result, nil
}

// --- Constitution ---

var constitutionLayerToProtoMap = map[storage.ConstitutionLayer]specv1.ConstitutionLayer{
	storage.ConstitutionLayerUser:    specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER,
	storage.ConstitutionLayerOrg:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
	storage.ConstitutionLayerProject: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
	storage.ConstitutionLayerDomain:  specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN,
}

var constitutionLayerFromProtoMap = map[specv1.ConstitutionLayer]storage.ConstitutionLayer{
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER:    storage.ConstitutionLayerUser,
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG:     storage.ConstitutionLayerOrg,
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT: storage.ConstitutionLayerProject,
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN:  storage.ConstitutionLayerDomain,
}

var violationSeverityToProtoMap = map[storage.ViolationSeverity]specv1.ViolationSeverity{
	storage.ViolationSeverityError:   specv1.ViolationSeverity_VIOLATION_SEVERITY_ERROR,
	storage.ViolationSeverityWarning: specv1.ViolationSeverity_VIOLATION_SEVERITY_WARNING,
	storage.ViolationSeverityInfo:    specv1.ViolationSeverity_VIOLATION_SEVERITY_INFO,
}

func constitutionToProto(c *storage.Constitution) *specv1.Constitution {
	if c == nil {
		return nil
	}
	pb := &specv1.Constitution{
		Id:          c.ID,
		Layer:       constitutionLayerToProtoMap[c.Layer],
		Name:        c.Name,
		Version:     c.Version,
		Constraints: c.Constraints,
		CreatedAt:   timeToProto(c.CreatedAt),
		UpdatedAt:   timeToProto(c.UpdatedAt),
	}

	if c.Tech != nil {
		pb.Tech = techStackToProto(c.Tech)
	}
	for _, p := range c.Principles {
		pb.Principles = append(pb.Principles, &specv1.Principle{
			Id:         p.ID,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
	}
	if c.Process != nil {
		pb.Process = processConfigToProto(c.Process)
	}
	for _, ap := range c.Antipatterns {
		pb.Antipatterns = append(pb.Antipatterns, &specv1.Antipattern{
			Pattern: ap.Pattern,
			Why:     ap.Why,
			Instead: ap.Instead,
		})
	}
	for _, ref := range c.References {
		pb.References = append(pb.References, &specv1.Reference{
			ReferenceType: referenceTypeToProto(ref.Type),
			Path:          ref.Path,
		})
	}

	return pb
}

func techStackToProto(t *storage.TechStack) *specv1.TechConfig {
	pb := &specv1.TechConfig{
		Frameworks:     t.Frameworks,
		Infrastructure: t.Infrastructure,
		ApiStandards:   t.APIStandards,
		Data:           t.Data,
	}
	if t.Languages != nil {
		pb.Languages = &specv1.LanguageConfig{
			Primary:          t.Languages.Primary,
			Allowed:          t.Languages.Allowed,
			Forbidden:        t.Languages.Forbidden,
			ForbiddenReasons: t.Languages.ForbiddenReasons,
		}
	}
	return pb
}

func processConfigToProto(p *storage.ProcessConfig) *specv1.ProcessConfig {
	pb := &specv1.ProcessConfig{
		SpecReview: p.SpecReview,
	}
	if p.SecurityReview != nil {
		pb.SecurityReview = &specv1.SecurityReviewConfig{When: p.SecurityReview.When}
	}
	if p.Deployment != nil {
		pb.Deployment = &specv1.DeploymentConfig{
			Strategy: p.Deployment.Strategy,
			Rollback: p.Deployment.Rollback,
		}
	}
	if p.Documentation != nil {
		pb.Documentation = &specv1.DocumentationConfig{
			ApiDocs: p.Documentation.APIDocs,
			Runbook: p.Documentation.Runbook,
		}
	}
	return pb
}

func constitutionFromProto(pb *specv1.Constitution) *storage.Constitution {
	if pb == nil {
		return nil
	}
	c := &storage.Constitution{
		ID:          pb.Id,
		Layer:       constitutionLayerFromProtoMap[pb.Layer],
		Name:        pb.Name,
		Version:     pb.Version,
		Constraints: pb.Constraints,
	}
	if pb.CreatedAt != nil {
		c.CreatedAt = pb.CreatedAt.AsTime()
	}
	if pb.UpdatedAt != nil {
		c.UpdatedAt = pb.UpdatedAt.AsTime()
	}
	if pb.Tech != nil {
		c.Tech = techStackFromProto(pb.Tech)
	}
	for _, p := range pb.Principles {
		c.Principles = append(c.Principles, storage.Principle{
			ID:         p.Id,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
	}
	if pb.Process != nil {
		c.Process = processConfigFromProto(pb.Process)
	}
	for _, ap := range pb.Antipatterns {
		c.Antipatterns = append(c.Antipatterns, storage.Antipattern{
			Pattern: ap.Pattern,
			Why:     ap.Why,
			Instead: ap.Instead,
		})
	}
	for _, ref := range pb.References {
		c.References = append(c.References, storage.Reference{
			Type: referenceTypeFromProto(ref.ReferenceType),
			Path: ref.Path,
		})
	}
	return c
}

func referenceTypeToProto(t string) specv1.ReferenceType {
	switch t {
	case "adr":
		return specv1.ReferenceType_REFERENCE_TYPE_ADR
	case "spec":
		return specv1.ReferenceType_REFERENCE_TYPE_SPEC
	case "doc":
		return specv1.ReferenceType_REFERENCE_TYPE_DOC
	case "url":
		return specv1.ReferenceType_REFERENCE_TYPE_URL
	default:
		return specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED
	}
}

func referenceTypeFromProto(t specv1.ReferenceType) string {
	switch t {
	case specv1.ReferenceType_REFERENCE_TYPE_ADR:
		return "adr"
	case specv1.ReferenceType_REFERENCE_TYPE_SPEC:
		return "spec"
	case specv1.ReferenceType_REFERENCE_TYPE_DOC:
		return "doc"
	case specv1.ReferenceType_REFERENCE_TYPE_URL:
		return "url"
	default:
		return ""
	}
}

func techStackFromProto(pb *specv1.TechConfig) *storage.TechStack {
	t := &storage.TechStack{
		Frameworks:     pb.Frameworks,
		Infrastructure: pb.Infrastructure,
		APIStandards:   pb.ApiStandards,
		Data:           pb.Data,
	}
	if pb.Languages != nil {
		t.Languages = &storage.Languages{
			Primary:          pb.Languages.Primary,
			Allowed:          pb.Languages.Allowed,
			Forbidden:        pb.Languages.Forbidden,
			ForbiddenReasons: pb.Languages.ForbiddenReasons,
		}
	}
	return t
}

func processConfigFromProto(pb *specv1.ProcessConfig) *storage.ProcessConfig {
	p := &storage.ProcessConfig{
		SpecReview: pb.SpecReview,
	}
	if pb.SecurityReview != nil {
		p.SecurityReview = &storage.SecurityReviewConfig{When: pb.SecurityReview.When}
	}
	if pb.Deployment != nil {
		p.Deployment = &storage.DeploymentConfig{
			Strategy: pb.Deployment.Strategy,
			Rollback: pb.Deployment.Rollback,
		}
	}
	if pb.Documentation != nil {
		p.Documentation = &storage.DocumentationConfig{
			APIDocs: pb.Documentation.ApiDocs,
			Runbook: pb.Documentation.Runbook,
		}
	}
	return p
}

var outputFormatToString = map[specv1.OutputFormat]string{
	specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD:   "claude-md",
	specv1.OutputFormat_OUTPUT_FORMAT_CURSORRULES: "cursorrules",
	specv1.OutputFormat_OUTPUT_FORMAT_AGENTS_MD:   "agents-md",
}

func violationsToProto(violations []storage.Violation) []*specv1.Violation {
	result := make([]*specv1.Violation, len(violations))
	for i, v := range violations {
		result[i] = &specv1.Violation{
			Rule:     v.Rule,
			Severity: violationSeverityToProtoMap[v.Severity],
			Message:  v.Message,
			SpecSlug: v.SpecSlug,
		}
	}
	return result
}

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

func executionEventToProto(e *storage.ExecutionEvent) *specv1.ExecutionEvent {
	return &specv1.ExecutionEvent{
		Id:        e.ID,
		SpecSlug:  e.SpecSlug,
		Agent:     e.Agent,
		Type:      executionEventTypeToProtoMap[e.Type],
		Message:   e.Message,
		CreatedAt: timeToProto(e.CreatedAt),
	}
}

func executionEventsToProto(events []*storage.ExecutionEvent) []*specv1.ExecutionEvent {
	result := make([]*specv1.ExecutionEvent, len(events))
	for i, e := range events {
		result[i] = executionEventToProto(e)
	}
	return result
}

// --- History ---

func historyToProto(entries []storage.HistoryEntry) []*specv1.HistoryEntry {
	if len(entries) == 0 {
		return nil
	}
	result := make([]*specv1.HistoryEntry, len(entries))
	for i, e := range entries {
		result[i] = &specv1.HistoryEntry{
			Version: e.Version,
			Stage:   string(e.Stage),
			Summary: e.Summary,
			Reason:  e.Reason,
			Date:    timeToProto(e.Date),
		}
	}
	return result
}

// --- Drift ---

var driftScopeFromProtoMap = map[specv1.DriftScope]string{
	specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED: "",
	specv1.DriftScope_DRIFT_SCOPE_DEPS:        "deps",
	specv1.DriftScope_DRIFT_SCOPE_INTERFACES:  "interfaces",
	specv1.DriftScope_DRIFT_SCOPE_VERIFY:      "verify",
}

func driftScopeFromProto(scope specv1.DriftScope) string {
	if s, ok := driftScopeFromProtoMap[scope]; ok {
		return s
	}
	return ""
}

var driftTypeToProtoMap = map[storage.DriftType]specv1.DriftType{
	storage.DriftTypeDependency: specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
	storage.DriftTypeInterface:  specv1.DriftType_DRIFT_TYPE_INTERFACE,
	storage.DriftTypeVerify:     specv1.DriftType_DRIFT_TYPE_VERIFY,
}

var driftSeverityToProtoMap = map[storage.DriftSeverity]specv1.DriftSeverity{
	storage.DriftSeverityHigh:   specv1.DriftSeverity_DRIFT_SEVERITY_HIGH,
	storage.DriftSeverityMedium: specv1.DriftSeverity_DRIFT_SEVERITY_MEDIUM,
	storage.DriftSeverityLow:    specv1.DriftSeverity_DRIFT_SEVERITY_LOW,
	storage.DriftSeverityInfo:   specv1.DriftSeverity_DRIFT_SEVERITY_INFO,
}

func driftReportToProto(r *storage.DriftReport) (*specv1.DriftReport, error) {
	items := make([]*specv1.DriftItem, len(r.Items))
	for i, item := range r.Items {
		dt, ok := driftTypeToProtoMap[item.Type]
		if !ok {
			return nil, fmt.Errorf("unknown drift type: %q", item.Type)
		}
		ds, ok := driftSeverityToProtoMap[item.Severity]
		if !ok {
			return nil, fmt.Errorf("unknown drift severity: %q", item.Severity)
		}
		items[i] = &specv1.DriftItem{
			Type:            dt,
			Severity:        ds,
			Description:     item.Description,
			SpecSlug:        item.SpecSlug,
			UpstreamSlug:    item.UpstreamSlug,
			ExpectedVersion: item.ExpectedVersion,
			ActualVersion:   item.ActualVersion,
		}
	}
	return &specv1.DriftReport{
		SpecSlug:        r.SpecSlug,
		Items:           items,
		Acknowledged:    r.Acknowledged,
		AcknowledgeNote: r.AcknowledgeNote,
		ItemsStale:      r.ItemsStale,
	}, nil
}

func driftReportsToProto(reports []storage.DriftReport) ([]*specv1.DriftReport, error) {
	result := make([]*specv1.DriftReport, len(reports))
	for i := range reports {
		r, err := driftReportToProto(&reports[i])
		if err != nil {
			return nil, err
		}
		result[i] = r
	}
	return result, nil
}

// --- Lifecycle ---

var lifecycleToProtoMap = map[storage.SpecLifecycle]specv1.SpecLifecycle{
	"":                          specv1.SpecLifecycle_SPEC_LIFECYCLE_UNSPECIFIED,
	storage.SpecLifecycleTask:   specv1.SpecLifecycle_SPEC_LIFECYCLE_TASK,
	storage.SpecLifecycleLiving: specv1.SpecLifecycle_SPEC_LIFECYCLE_LIVING,
}

func lifecycleToProto(l storage.SpecLifecycle) (specv1.SpecLifecycle, error) {
	if v, ok := lifecycleToProtoMap[l]; ok {
		return v, nil
	}
	return specv1.SpecLifecycle_SPEC_LIFECYCLE_UNSPECIFIED, fmt.Errorf("unknown lifecycle: %q", l)
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
		slog.Warn("lintViolationToProto: unknown severity",
			slog.String("severity", string(v.Severity)), slog.String("rule", v.Rule))
		return nil, fmt.Errorf("unknown lint severity: %q", v.Severity)
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
		pb, err := lintViolationToProto(&r.Violations[i])
		if err != nil {
			return nil, err
		}
		violations[i] = pb
	}
	return &specv1.LintResult{
		SpecSlug:   r.SpecSlug,
		Violations: violations,
		Passed:     r.Passed,
	}, nil
}

func lintResultsToProto(results []storage.LintResult) ([]*specv1.LintResult, error) {
	result := make([]*specv1.LintResult, len(results))
	for i := range results {
		pb, err := lintResultToProto(&results[i])
		if err != nil {
			return nil, err
		}
		result[i] = pb
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
