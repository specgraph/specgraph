// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

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
