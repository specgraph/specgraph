// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- Spec ---

func specToProto(s *storage.Spec) (*specv1.Spec, error) {
	pt, err := specProvenanceToProto(s.Provenance)
	if err != nil {
		return nil, fmt.Errorf("spec %q: %w", s.Slug, err)
	}
	pb := &specv1.Spec{
		Id:                s.ID,
		Slug:              s.Slug,
		Intent:            s.Intent,
		Stage:             string(s.Stage),
		Priority:          string(s.Priority),
		Complexity:        string(s.Complexity),
		Version:           s.Version,
		CreatedAt:         timeToProto(s.CreatedAt),
		UpdatedAt:         timeToProto(s.UpdatedAt),
		ProvenanceType:    pt,
		SupersededBy:      s.SupersededBy,
		Supersedes:        s.Supersedes,
		Notes:             s.Notes,
		ContentHash:       s.ContentHash,
		ConversationCount: safeConvCount(s.ConversationCount),
	}
	setSpecProvenanceDetailOnProto(pb, s.ProvenanceDetail)
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

// --- Provenance ---

// specProvenanceToProtoMap maps storage provenance values to proto enums.
var specProvenanceToProtoMap = map[storage.SpecProvenanceType]specv1.SpecProvenance{
	storage.SpecProvenanceAuthored:          specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED,
	storage.SpecProvenanceRetroactiveFromPR: specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR,
	storage.SpecProvenanceDeclared:          specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED,
}

func specProvenanceToProto(p storage.SpecProvenanceType) (specv1.SpecProvenance, error) {
	if v, ok := specProvenanceToProtoMap[p]; ok {
		return v, nil
	}
	return specv1.SpecProvenance_SPEC_PROVENANCE_UNSPECIFIED, fmt.Errorf("unknown provenance: %q", p)
}

// setSpecProvenanceDetailOnProto sets the oneof field on the proto Spec based on
// which variant pointer (if any) is populated in the domain detail struct.
// The oneof interface (isSpec_ProvenanceDetail) is unexported from the gen
// package, so we assign directly rather than returning an interface value.
func setSpecProvenanceDetailOnProto(pb *specv1.Spec, d storage.SpecProvenanceDetail) {
	switch {
	case d.RetroactiveFromPR != nil:
		pb.ProvenanceDetail = &specv1.Spec_RetroactiveFromPr{
			RetroactiveFromPr: &specv1.RetroactiveFromPrProvenance{
				Url:      d.RetroactiveFromPR.URL,
				Sha:      d.RetroactiveFromPR.SHA,
				MergedAt: timeToProto(d.RetroactiveFromPR.MergedAt),
				Title:    d.RetroactiveFromPR.Title,
			},
		}
	case d.Declared != nil:
		pb.ProvenanceDetail = &specv1.Spec_Declared{
			Declared: &specv1.DeclaredProvenance{
				DeclaredBy: d.Declared.DeclaredBy,
				Note:       d.Declared.Note,
			},
		}
	default:
		// AUTHORED — empty payload.
		pb.ProvenanceDetail = &specv1.Spec_Authored{Authored: &specv1.AuthoredProvenance{}}
	}
}

// specProvenanceFromProto converts a proto enum back to the storage discriminator.
func specProvenanceFromProto(p specv1.SpecProvenance) (storage.SpecProvenanceType, error) {
	for domainVal, protoVal := range specProvenanceToProtoMap {
		if protoVal == p {
			return domainVal, nil
		}
	}
	return "", fmt.Errorf("unknown provenance enum: %q", p.String())
}

// specProvenanceDetailFromProto reads the oneof on a proto Spec and returns the
// domain detail struct. The caller passes pb.GetProvenanceDetail() — same
// unexported-interface caveat as above.
func specProvenanceDetailFromProto(pb *specv1.Spec) storage.SpecProvenanceDetail {
	switch v := pb.GetProvenanceDetail().(type) {
	case *specv1.Spec_RetroactiveFromPr:
		return storage.SpecProvenanceDetail{
			RetroactiveFromPR: &storage.RetroactivePRProvenance{
				URL:      v.RetroactiveFromPr.GetUrl(),
				SHA:      v.RetroactiveFromPr.GetSha(),
				MergedAt: v.RetroactiveFromPr.GetMergedAt().AsTime(),
				Title:    v.RetroactiveFromPr.GetTitle(),
			},
		}
	case *specv1.Spec_Declared:
		return storage.SpecProvenanceDetail{
			Declared: &storage.DeclaredProvenance{
				DeclaredBy: v.Declared.GetDeclaredBy(),
				Note:       v.Declared.GetNote(),
			},
		}
	case *specv1.Spec_Authored:
		return storage.SpecProvenanceDetail{}
	default:
		return storage.SpecProvenanceDetail{}
	}
}
