// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func changeLogEntryToProto(e *storage.ChangeLogEntry) *specv1.ChangeLogEntry {
	changes := make([]*specv1.FieldChange, len(e.Changes))
	for i, c := range e.Changes {
		changes[i] = &specv1.FieldChange{
			Field:    c.Field,
			OldValue: c.OldValue,
			NewValue: c.NewValue,
		}
	}
	return &specv1.ChangeLogEntry{
		Id:          e.ID,
		Version:     e.Version,
		Stage:       e.Stage,
		ContentHash: e.ContentHash,
		Checkpoint:  e.Checkpoint,
		Summary:     e.Summary,
		Reason:      e.Reason,
		Changes:     changes,
		Date:        timestamppb.New(e.Date),
	}
}

func changeLogEntriesToProto(entries []*storage.ChangeLogEntry) []*specv1.ChangeLogEntry {
	pbs := make([]*specv1.ChangeLogEntry, len(entries))
	for i, e := range entries {
		pbs[i] = changeLogEntryToProto(e)
	}
	return pbs
}
