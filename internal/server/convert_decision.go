// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

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
		ContentHash:  d.ContentHash,
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
