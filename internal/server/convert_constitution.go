// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- Constitution ---

var constitutionLayerToProtoMap = map[storage.ConstitutionLayer]specv1.ConstitutionLayer{
	storage.ConstitutionLayerUser:    specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER,
	storage.ConstitutionLayerOrg:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
	storage.ConstitutionLayerProject: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
	storage.ConstitutionLayerDomain:  specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN,
}

var constitutionLayerFromProtoMap = map[specv1.ConstitutionLayer]storage.ConstitutionLayer{
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER:    storage.ConstitutionLayerUser,
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG:     storage.ConstitutionLayerOrg,
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT: storage.ConstitutionLayerProject,
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN:  storage.ConstitutionLayerDomain,
}

func constitutionToProto(c *storage.Constitution) *specv1.Constitution {
	if c == nil {
		return nil
	}
	pb := &specv1.Constitution{
		Id:          c.ID,
		Layer:       constitutionLayerToProtoMap[c.Layer],
		Name:        c.Name,
		Version:     c.Version,
		Constraints: c.Constraints,
		CreatedAt:   timeToProto(c.CreatedAt),
		UpdatedAt:   timeToProto(c.UpdatedAt),
	}

	if c.Tech != nil {
		pb.Tech = techStackToProto(c.Tech)
	}
	for _, p := range c.Principles {
		pb.Principles = append(pb.Principles, &specv1.Principle{
			Id:         p.ID,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
	}
	if c.Process != nil {
		pb.Process = processConfigToProto(c.Process)
	}
	for _, ap := range c.Antipatterns {
		pb.Antipatterns = append(pb.Antipatterns, &specv1.Antipattern{
			Pattern: ap.Pattern,
			Why:     ap.Why,
			Instead: ap.Instead,
		})
	}
	for _, ref := range c.References {
		pb.References = append(pb.References, &specv1.Reference{
			ReferenceType: referenceTypeToProto(ref.Type),
			Path:          ref.Path,
		})
	}

	return pb
}

func techStackToProto(t *storage.TechStack) *specv1.TechConfig {
	pb := &specv1.TechConfig{
		Frameworks:     t.Frameworks,
		Infrastructure: t.Infrastructure,
		ApiStandards:   t.APIStandards,
		Data:           t.Data,
	}
	if t.Languages != nil {
		pb.Languages = &specv1.LanguageConfig{
			Primary:          t.Languages.Primary,
			Allowed:          t.Languages.Allowed,
			Forbidden:        t.Languages.Forbidden,
			ForbiddenReasons: t.Languages.ForbiddenReasons,
		}
	}
	return pb
}

func processConfigToProto(p *storage.ProcessConfig) *specv1.ProcessConfig {
	pb := &specv1.ProcessConfig{
		SpecReview: p.SpecReview,
	}
	if p.SecurityReview != nil {
		pb.SecurityReview = &specv1.SecurityReviewConfig{When: p.SecurityReview.When}
	}
	if p.Deployment != nil {
		pb.Deployment = &specv1.DeploymentConfig{
			Strategy: p.Deployment.Strategy,
			Rollback: p.Deployment.Rollback,
		}
	}
	if p.Documentation != nil {
		pb.Documentation = &specv1.DocumentationConfig{
			ApiDocs: p.Documentation.APIDocs,
			Runbook: p.Documentation.Runbook,
		}
	}
	return pb
}

func constitutionFromProto(pb *specv1.Constitution) (*storage.Constitution, error) {
	if pb == nil {
		return nil, nil
	}
	layer, ok := constitutionLayerFromProtoMap[pb.Layer]
	if !ok && pb.Layer != specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
		return nil, fmt.Errorf("unsupported constitution layer: %v", pb.Layer)
	}
	c := &storage.Constitution{
		ID:          pb.Id,
		Layer:       layer,
		Name:        pb.Name,
		Version:     pb.Version,
		Constraints: pb.Constraints,
	}
	if pb.CreatedAt != nil {
		c.CreatedAt = pb.CreatedAt.AsTime()
	}
	if pb.UpdatedAt != nil {
		c.UpdatedAt = pb.UpdatedAt.AsTime()
	}
	if pb.Tech != nil {
		c.Tech = techStackFromProto(pb.Tech)
	}
	for _, p := range pb.Principles {
		c.Principles = append(c.Principles, storage.Principle{
			ID:         p.Id,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
	}
	if pb.Process != nil {
		c.Process = processConfigFromProto(pb.Process)
	}
	for _, ap := range pb.Antipatterns {
		c.Antipatterns = append(c.Antipatterns, storage.Antipattern{
			Pattern: ap.Pattern,
			Why:     ap.Why,
			Instead: ap.Instead,
		})
	}
	for _, ref := range pb.References {
		rt := referenceTypeFromProto(ref.ReferenceType)
		if rt == "" && ref.ReferenceType != specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED {
			return nil, fmt.Errorf("unsupported reference type: %v", ref.ReferenceType)
		}
		c.References = append(c.References, storage.Reference{
			Type: rt,
			Path: ref.Path,
		})
	}
	return c, nil
}

func referenceTypeToProto(t string) specv1.ReferenceType {
	switch t {
	case "adr":
		return specv1.ReferenceType_REFERENCE_TYPE_ADR
	case "spec":
		return specv1.ReferenceType_REFERENCE_TYPE_SPEC
	case "doc":
		return specv1.ReferenceType_REFERENCE_TYPE_DOC
	case "url":
		return specv1.ReferenceType_REFERENCE_TYPE_URL
	default:
		return specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED
	}
}

func referenceTypeFromProto(t specv1.ReferenceType) string {
	switch t {
	case specv1.ReferenceType_REFERENCE_TYPE_ADR:
		return "adr"
	case specv1.ReferenceType_REFERENCE_TYPE_SPEC:
		return "spec"
	case specv1.ReferenceType_REFERENCE_TYPE_DOC:
		return "doc"
	case specv1.ReferenceType_REFERENCE_TYPE_URL:
		return "url"
	default:
		return ""
	}
}

func techStackFromProto(pb *specv1.TechConfig) *storage.TechStack {
	t := &storage.TechStack{
		Frameworks:     pb.Frameworks,
		Infrastructure: pb.Infrastructure,
		APIStandards:   pb.ApiStandards,
		Data:           pb.Data,
	}
	if pb.Languages != nil {
		t.Languages = &storage.Languages{
			Primary:          pb.Languages.Primary,
			Allowed:          pb.Languages.Allowed,
			Forbidden:        pb.Languages.Forbidden,
			ForbiddenReasons: pb.Languages.ForbiddenReasons,
		}
	}
	return t
}

func processConfigFromProto(pb *specv1.ProcessConfig) *storage.ProcessConfig {
	p := &storage.ProcessConfig{
		SpecReview: pb.SpecReview,
	}
	if pb.SecurityReview != nil {
		p.SecurityReview = &storage.SecurityReviewConfig{When: pb.SecurityReview.When}
	}
	if pb.Deployment != nil {
		p.Deployment = &storage.DeploymentConfig{
			Strategy: pb.Deployment.Strategy,
			Rollback: pb.Deployment.Rollback,
		}
	}
	if pb.Documentation != nil {
		p.Documentation = &storage.DocumentationConfig{
			APIDocs: pb.Documentation.ApiDocs,
			Runbook: pb.Documentation.Runbook,
		}
	}
	return p
}

var outputFormatToString = map[specv1.OutputFormat]string{
	specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD:   "claude-md",
	specv1.OutputFormat_OUTPUT_FORMAT_CURSORRULES: "cursorrules",
	specv1.OutputFormat_OUTPUT_FORMAT_AGENTS_MD:   "agents-md",
}

func injectToolFromProto(t specv1.InjectTool) (storage.InjectToolType, error) {
	switch t {
	case specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE:
		return storage.InjectToolClaudeCode, nil
	case specv1.InjectTool_INJECT_TOOL_CURSOR:
		return storage.InjectToolCursor, nil
	case specv1.InjectTool_INJECT_TOOL_AGENTS_MD:
		return storage.InjectToolAgentsMD, nil
	default:
		return "", fmt.Errorf("unknown inject tool: %v", t)
	}
}
