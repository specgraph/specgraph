// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package load parses constitution YAML/JSON into the *storage.Constitution
// domain struct. Single source of YAML parsing for both the CLI's
// 'constitution import' command and the server's RefreshConstitutionLayer
// RPC handler.
package load

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// LoadFromYAML parses YAML or JSON bytes into *storage.Constitution.
// Layer validation: if a layer field is present, it must be one of
// user|org|project|domain. Empty layer is allowed (caller may set it).
func LoadFromYAML(data []byte) (*storage.Constitution, error) {
	cc, err := config.ParseConstitutionConfig(data)
	if err != nil {
		// Map the config package's "unknown constitution layer" error to the
		// "invalid layer" phrasing that the tests (and callers) expect.
		if strings.Contains(err.Error(), "unknown constitution layer") {
			return nil, fmt.Errorf("invalid layer: %w", err)
		}
		return nil, fmt.Errorf("parse constitution: %w", err)
	}
	return cc.ToDomain(), nil
}

// ToProto converts a *storage.Constitution to the proto representation
// for use with RPC calls (e.g., UpdateConstitution). Inverse of the
// server's constitutionFromProto.
func ToProto(c *storage.Constitution) *specv1.Constitution {
	if c == nil {
		return nil
	}
	pb := &specv1.Constitution{
		Name:        c.Name,
		Layer:       layerToProto(c.Layer),
		Constraints: c.Constraints,
		SourceUrl:   c.SourceURL,
		SourceHash:  c.SourceHash,
	}
	for _, p := range c.Principles {
		pb.Principles = append(pb.Principles, &specv1.Principle{
			Id:         p.ID,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
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
	if c.Tech != nil {
		pb.Tech = techStackToProto(c.Tech)
	}
	if c.Process != nil {
		pb.Process = processConfigToProto(c.Process)
	}
	return pb
}

func layerToProto(l storage.ConstitutionLayer) specv1.ConstitutionLayer {
	switch l {
	case storage.ConstitutionLayerUser:
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER
	case storage.ConstitutionLayerOrg:
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG
	case storage.ConstitutionLayerProject:
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT
	case storage.ConstitutionLayerDomain:
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN
	default:
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED
	}
}

func referenceTypeToProto(t string) specv1.ReferenceType {
	switch strings.ToLower(t) {
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
