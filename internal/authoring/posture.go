// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package authoring implements the SpecGraph authoring funnel — the
// multi-stage workflow that transforms raw user intent into fully
// specified, decomposed, and approved spec nodes.
package authoring

// Posture represents the collaboration posture between the AI and the human.
type Posture int

// Posture constants matching the proto Posture enum.
const (
	PostureUnspecified Posture = 0
	PostureDrive       Posture = 1
	PosturePartner     Posture = 2
	PostureSupport     Posture = 3
)

const (
	driveThreshold   = 20
	supportThreshold = 100
)

// DetectPosture infers an interaction posture from message lengths.
// Empty messages default to Partner. Short average (<driveThreshold chars)
// maps to Drive, long average (>supportThreshold chars) maps to Support,
// and everything else is Partner.
func DetectPosture(messages []string) Posture {
	if len(messages) == 0 {
		return PosturePartner
	}

	total := 0
	for _, m := range messages {
		total += len(m)
	}
	avg := float64(total) / float64(len(messages))

	if avg < driveThreshold {
		return PostureDrive
	}
	if avg > supportThreshold {
		return PostureSupport
	}
	return PosturePartner
}

// ResolvePosture returns the explicit posture when set, falling back to
// heuristic detection from messages when the caller passes PostureUnspecified.
func ResolvePosture(explicit Posture, messages []string) Posture {
	if explicit != PostureUnspecified {
		return explicit
	}
	return DetectPosture(messages)
}
