// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- Decision Confidence ---

var decisionConfidenceToProtoMap = map[storage.DecisionConfidence]specv1.DecisionConfidence{
	storage.DecisionConfidenceHigh:   specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
	storage.DecisionConfidenceMedium: specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM,
	storage.DecisionConfidenceLow:    specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW,
}

var decisionConfidenceFromProtoMap = map[specv1.DecisionConfidence]storage.DecisionConfidence{
	specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH:   storage.DecisionConfidenceHigh,
	specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM: storage.DecisionConfidenceMedium,
	specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW:    storage.DecisionConfidenceLow,
}

func decisionConfidenceToProto(c storage.DecisionConfidence) specv1.DecisionConfidence {
	if v, ok := decisionConfidenceToProtoMap[c]; ok {
		return v
	}
	return specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED
}

func decisionConfidenceFromProto(c specv1.DecisionConfidence) storage.DecisionConfidence {
	if v, ok := decisionConfidenceFromProtoMap[c]; ok {
		return v
	}
	return ""
}

// --- Decision Scope ---

var decisionScopeToProtoMap = map[storage.DecisionScope]specv1.DecisionScope{
	storage.DecisionScopeProject: specv1.DecisionScope_DECISION_SCOPE_PROJECT,
	storage.DecisionScopeTeam:    specv1.DecisionScope_DECISION_SCOPE_TEAM,
	storage.DecisionScopeOrg:     specv1.DecisionScope_DECISION_SCOPE_ORG,
}

var decisionScopeFromProtoMap = map[specv1.DecisionScope]storage.DecisionScope{
	specv1.DecisionScope_DECISION_SCOPE_PROJECT: storage.DecisionScopeProject,
	specv1.DecisionScope_DECISION_SCOPE_TEAM:    storage.DecisionScopeTeam,
	specv1.DecisionScope_DECISION_SCOPE_ORG:     storage.DecisionScopeOrg,
}

func decisionScopeToProto(s storage.DecisionScope) specv1.DecisionScope {
	if v, ok := decisionScopeToProtoMap[s]; ok {
		return v
	}
	return specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED
}

func decisionScopeFromProto(s specv1.DecisionScope) storage.DecisionScope {
	if v, ok := decisionScopeFromProtoMap[s]; ok {
		return v
	}
	return ""
}

// --- Rejected Alternatives ---

func rejectedAltsToProto(alts []storage.RejectedAlternative) []*specv1.RejectedAlternative {
	if len(alts) == 0 {
		return nil
	}
	result := make([]*specv1.RejectedAlternative, len(alts))
	for i, a := range alts {
		result[i] = &specv1.RejectedAlternative{Option: a.Option, Reason: a.Reason}
	}
	return result
}

func rejectedAltsFromProto(alts []*specv1.RejectedAlternative) []storage.RejectedAlternative {
	if len(alts) == 0 {
		return nil
	}
	result := make([]storage.RejectedAlternative, len(alts))
	for i, a := range alts {
		result[i] = storage.RejectedAlternative{Option: a.GetOption(), Reason: a.GetReason()}
	}
	return result
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
		Id:                   d.ID,
		Slug:                 d.Slug,
		Title:                d.Title,
		Status:               status,
		Decision:             d.Body,
		Rationale:            d.Rationale,
		SupersededBy:         d.SupersededBy,
		Question:             d.Question,
		RejectedAlternatives: rejectedAltsToProto(d.RejectedAlternatives),
		Confidence:           decisionConfidenceToProto(d.Confidence),
		Tags:                 d.Tags,
		Scope:                decisionScopeToProto(d.Scope),
		OriginSpec:           d.OriginSpec,
		OriginStage:          d.OriginStage,
		Version:              int32(d.Version), //nolint:gosec // version is a small monotonic counter; overflow impossible
		ContentHash:          d.ContentHash,
		CreatedAt:            timeToProto(d.CreatedAt),
		UpdatedAt:            timeToProto(d.UpdatedAt),
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
