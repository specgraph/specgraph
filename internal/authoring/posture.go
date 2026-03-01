// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package authoring implements the SpecGraph authoring funnel — the
// multi-stage workflow that transforms raw user intent into fully
// specified, decomposed, and approved spec nodes.
package authoring

import specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"

const (
	driveThreshold   = 20
	supportThreshold = 100
)

// DetectPosture infers an interaction posture from message lengths.
// Empty messages default to PARTNER. Short average (<20 chars) maps to DRIVE,
// long average (>100 chars) maps to SUPPORT, and everything else is PARTNER.
func DetectPosture(messages []string) specv1.Posture {
	if len(messages) == 0 {
		return specv1.Posture_POSTURE_PARTNER
	}

	total := 0
	for _, m := range messages {
		total += len(m)
	}
	avg := total / len(messages)

	if avg < driveThreshold {
		return specv1.Posture_POSTURE_DRIVE
	}
	if avg > supportThreshold {
		return specv1.Posture_POSTURE_SUPPORT
	}
	return specv1.Posture_POSTURE_PARTNER
}

// ResolvePosture returns the explicit posture when set, falling back to
// heuristic detection from messages when the caller passes UNSPECIFIED.
func ResolvePosture(explicit specv1.Posture, messages []string) specv1.Posture {
	if explicit != specv1.Posture_POSTURE_UNSPECIFIED {
		return explicit
	}
	return DetectPosture(messages)
}
