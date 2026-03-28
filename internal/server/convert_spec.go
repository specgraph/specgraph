// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- Spec ---

func specToProto(s *storage.Spec) (*specv1.Spec, error) {
	lc, err := lifecycleToProto(s.Lifecycle)
	if err != nil {
		return nil, fmt.Errorf("spec %q: %w", s.Slug, err)
	}
	pb := &specv1.Spec{
		Id:           s.ID,
		Slug:         s.Slug,
		Intent:       s.Intent,
		Stage:        string(s.Stage),
		Priority:     string(s.Priority),
		Complexity:   string(s.Complexity),
		Version:      s.Version,
		CreatedAt:    timeToProto(s.CreatedAt),
		UpdatedAt:    timeToProto(s.UpdatedAt),
		Lifecycle:    lc,
		SupersededBy: s.SupersededBy,
		Supersedes:   s.Supersedes,
		Notes:        s.Notes,
		ContentHash:       s.ContentHash,
		ConversationCount: safeConvCount(s.ConversationCount),
	}
	if s.ConversationLogs != nil {
		logs := make([]*specv1.ConversationLog, len(s.ConversationLogs))
		for i, entry := range s.ConversationLogs {
			logs[i] = conversationLogToProto(entry)
		}
		pb.ConversationLogs = logs
		if s.ConversationCount == 0 && len(logs) > 0 {
			pb.ConversationCount = safeConvCount(len(logs))
		}
	}
	pb.SparkOutput = sparkOutputToProto(s.SparkOutput)
	pb.ShapeOutput = shapeOutputToProto(s.ShapeOutput)
	pb.SpecifyOutput = specifyOutputToProto(s.SpecifyOutput)
	decompose, decompErr := decomposeOutputToProto(s.DecomposeOutput)
	if decompErr != nil {
		return nil, fmt.Errorf("spec %q: %w", s.Slug, decompErr)
	}
	pb.DecomposeOutput = decompose
	return pb, nil
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

// --- Lifecycle ---

// lifecycleToProtoMap maps storage lifecycle values to proto enums.
// The empty-string entry handles pre-lifecycle specs (created before the field existed)
// that have no lifecycle set in the graph — these map to UNSPECIFIED on the wire.
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
