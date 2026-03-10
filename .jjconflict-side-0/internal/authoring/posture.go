// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package authoring implements the SpecGraph authoring funnel — the
// multi-stage workflow that transforms raw user intent into fully
// specified, decomposed, and approved spec nodes.
package authoring

import specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"

// Posture represents the collaboration posture between the AI and the human.
type Posture int

// Posture constants matching the proto Posture enum.
const (
	PostureUnspecified Posture = 0
	PostureDrive       Posture = 1
	PosturePartner     Posture = 2
	PostureSupport     Posture = 3
)

// postureToProtoMap maps domain Posture values to proto Posture values.
var postureToProtoMap = map[Posture]specv1.Posture{
	PostureUnspecified: specv1.Posture_POSTURE_UNSPECIFIED,
	PostureDrive:       specv1.Posture_POSTURE_DRIVE,
	PosturePartner:     specv1.Posture_POSTURE_PARTNER,
	PostureSupport:     specv1.Posture_POSTURE_SUPPORT,
}

// protoToPostureMap maps proto Posture values to domain Posture values.
var protoToPostureMap = map[specv1.Posture]Posture{
	specv1.Posture_POSTURE_UNSPECIFIED: PostureUnspecified,
	specv1.Posture_POSTURE_DRIVE:       PostureDrive,
	specv1.Posture_POSTURE_PARTNER:     PosturePartner,
	specv1.Posture_POSTURE_SUPPORT:     PostureSupport,
}

// PostureToProto converts a domain Posture to its proto equivalent.
// Unknown values map to POSTURE_UNSPECIFIED.
func PostureToProto(p Posture) specv1.Posture {
	return postureToProtoMap[p]
}

// ProtoToPosture converts a proto Posture to its domain equivalent.
// Unknown values map to PostureUnspecified.
func ProtoToPosture(p specv1.Posture) Posture {
	return protoToPostureMap[p]
}

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
