// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "time"

// SpecStage represents the authoring lifecycle stage of a spec.
type SpecStage string

// Spec stage lifecycle values.
const (
	SpecStageSpark     SpecStage = "spark"
	SpecStageShape     SpecStage = "shape"
	SpecStageSpecify   SpecStage = "specify"
	SpecStageDecompose SpecStage = "decompose"
	SpecStageApproved  SpecStage = "approved"
)

// IsValid reports whether s is a known spec stage.
func (s SpecStage) IsValid() bool {
	switch s {
	case SpecStageSpark, SpecStageShape, SpecStageSpecify,
		SpecStageDecompose, SpecStageApproved:
		return true
	default:
		return false
	}
}

// SpecPriority represents the priority level of a spec.
type SpecPriority string

// Spec priority values.
const (
	SpecPriorityP0 SpecPriority = "p0"
	SpecPriorityP1 SpecPriority = "p1"
	SpecPriorityP2 SpecPriority = "p2"
	SpecPriorityP3 SpecPriority = "p3"
)

// IsValid reports whether p is a known spec priority.
func (p SpecPriority) IsValid() bool {
	switch p {
	case SpecPriorityP0, SpecPriorityP1, SpecPriorityP2, SpecPriorityP3:
		return true
	default:
		return false
	}
}

// Spec is the storage-layer domain type for specifications.
// Handlers convert between this type and the proto Spec message.
type Spec struct {
	ID         string
	Slug       string
	Intent     string
	Stage      SpecStage
	Priority   SpecPriority
	Complexity string
	Version    int32
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
