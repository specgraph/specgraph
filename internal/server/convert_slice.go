// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- Slice conversions ---

var sliceStatusToProtoMap = map[storage.SliceStatus]specv1.SliceStatus{
	storage.SliceStatusOpen:    specv1.SliceStatus_SLICE_STATUS_OPEN,
	storage.SliceStatusClaimed: specv1.SliceStatus_SLICE_STATUS_CLAIMED,
	storage.SliceStatusDone:    specv1.SliceStatus_SLICE_STATUS_DONE,
}

func sliceStatusToProto(s storage.SliceStatus) (specv1.SliceStatus, error) {
	if v, ok := sliceStatusToProtoMap[s]; ok {
		return v, nil
	}
	return specv1.SliceStatus_SLICE_STATUS_UNSPECIFIED, fmt.Errorf("unknown slice status: %q", s)
}

func sliceToProto(s *storage.Slice) (*specv1.Slice, error) {
	status, err := sliceStatusToProto(s.Status)
	if err != nil {
		return nil, err
	}
	return &specv1.Slice{
		Id:         s.ID,
		Slug:       s.Slug,
		ParentSlug: s.ParentSlug,
		SliceId:    s.SliceID,
		Intent:     s.Intent,
		Verify:     s.Verify,
		Touches:    s.Touches,
		DependsOn:  s.DependsOn,
		Status:     status,
		AssignedTo: s.AssignedTo,
		CreatedAt:  timeToProto(s.CreatedAt),
		UpdatedAt:  timeToProto(s.UpdatedAt),
	}, nil
}

func slicesToProto(slices []*storage.Slice) ([]*specv1.Slice, error) {
	result := make([]*specv1.Slice, len(slices))
	for i, s := range slices {
		pb, err := sliceToProto(s)
		if err != nil {
			return nil, err
		}
		result[i] = pb
	}
	return result, nil
}
