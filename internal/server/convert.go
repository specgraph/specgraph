// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- Spec ---

func specToProto(s *storage.Spec) *specv1.Spec {
	return &specv1.Spec{
		Id:         s.ID,
		Slug:       s.Slug,
		Intent:     s.Intent,
		Stage:      s.Stage,
		Priority:   s.Priority,
		Complexity: s.Complexity,
		Version:    s.Version,
		CreatedAt:  timestamppb.New(s.CreatedAt),
		UpdatedAt:  timestamppb.New(s.UpdatedAt),
	}
}

func specsToProto(specs []*storage.Spec) []*specv1.Spec {
	result := make([]*specv1.Spec, len(specs))
	for i, s := range specs {
		result[i] = specToProto(s)
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
		Id:           d.ID,
		Slug:         d.Slug,
		Title:        d.Title,
		Status:       status,
		Decision:     d.Body,
		Rationale:    d.Rationale,
		SupersededBy: d.SupersededBy,
		CreatedAt:    timestamppb.New(d.CreatedAt),
		UpdatedAt:    timestamppb.New(d.UpdatedAt),
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
	storage.EdgeTypeDependsOn: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	storage.EdgeTypeBlocks:    specv1.EdgeType_EDGE_TYPE_BLOCKS,
	storage.EdgeTypeComposes:  specv1.EdgeType_EDGE_TYPE_COMPOSES,
	storage.EdgeTypeRelatesTo: specv1.EdgeType_EDGE_TYPE_RELATES_TO,
	storage.EdgeTypeInforms:   specv1.EdgeType_EDGE_TYPE_INFORMS,
	storage.EdgeTypeDecidedIn: specv1.EdgeType_EDGE_TYPE_DECIDED_IN,
}

var edgeTypeFromProtoMap = map[specv1.EdgeType]storage.EdgeType{
	specv1.EdgeType_EDGE_TYPE_DEPENDS_ON: storage.EdgeTypeDependsOn,
	specv1.EdgeType_EDGE_TYPE_BLOCKS:     storage.EdgeTypeBlocks,
	specv1.EdgeType_EDGE_TYPE_COMPOSES:   storage.EdgeTypeComposes,
	specv1.EdgeType_EDGE_TYPE_RELATES_TO: storage.EdgeTypeRelatesTo,
	specv1.EdgeType_EDGE_TYPE_INFORMS:    storage.EdgeTypeInforms,
	specv1.EdgeType_EDGE_TYPE_DECIDED_IN: storage.EdgeTypeDecidedIn,
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
