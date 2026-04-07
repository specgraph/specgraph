// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package merge implements strategic merge for layered constitutions.
// Layers are applied in order from lowest to highest precedence
// (user < org < project < domain), with higher layers winning on conflicts.
package merge

import (
	"slices"

	"github.com/specgraph/specgraph/internal/storage"
)

// Result holds the merged constitution and provenance tracking.
type Result struct {
	// Constitution is the merged result of all input layers.
	Constitution *storage.Constitution
	// Provenance maps dot/bracket-notation keys to the layer that last set them.
	// Examples: "process.spec_review", "principles[p1]", "tech_config.frameworks[rpc]"
	Provenance map[string]storage.ConstitutionLayer
}

// Layers merges a slice of constitutions ordered lowest-to-highest precedence
// (user < org < project < domain). It returns the merged Result or an error.
// A nil or empty input returns an empty Constitution and empty Provenance map.
func Layers(layers []*storage.Constitution) (*Result, error) {
	result := &Result{
		Constitution: &storage.Constitution{},
		Provenance:   map[string]storage.ConstitutionLayer{},
	}

	if len(layers) == 0 {
		return result, nil
	}

	// Keyed state tracked across layers.
	principleMap := map[string]storage.Principle{}
	principleOrder := []string{}
	antipatternMap := map[string]storage.Antipattern{}
	antipatternOrder := []string{}
	referenceMap := map[string]storage.Reference{}
	referenceOrder := []string{}

	// String list dedup sets.
	constraintSet := map[string]bool{}
	constraintOrder := []string{}

	// Tech string list dedup sets.
	langAllowedSet := map[string]bool{}
	langAllowedOrder := []string{}
	langForbiddenSet := map[string]bool{}
	langForbiddenOrder := []string{}

	for _, layer := range layers {
		if layer == nil {
			continue
		}
		lyr := layer.Layer

		// Carry forward identity/metadata from highest-precedence layer.
		if layer.Name != "" {
			result.Constitution.Name = layer.Name
		}
		if layer.Layer != "" {
			result.Constitution.Layer = layer.Layer
		}

		// --- Scalar: process fields ---
		if layer.Process != nil {
			if result.Constitution.Process == nil {
				result.Constitution.Process = &storage.ProcessConfig{}
			}
			p := layer.Process
			if p.SpecReview != "" {
				result.Constitution.Process.SpecReview = p.SpecReview
				result.Provenance["process.spec_review"] = lyr
			}
			if p.SecurityReview != nil {
				if result.Constitution.Process.SecurityReview == nil {
					result.Constitution.Process.SecurityReview = &storage.SecurityReviewConfig{}
				}
				if p.SecurityReview.When != "" {
					result.Constitution.Process.SecurityReview.When = p.SecurityReview.When
					result.Provenance["process.security_review.when"] = lyr
				}
			}
			if p.Deployment != nil {
				if result.Constitution.Process.Deployment == nil {
					result.Constitution.Process.Deployment = &storage.DeploymentConfig{}
				}
				if p.Deployment.Strategy != "" {
					result.Constitution.Process.Deployment.Strategy = p.Deployment.Strategy
					result.Provenance["process.deployment.strategy"] = lyr
				}
				if p.Deployment.Rollback != "" {
					result.Constitution.Process.Deployment.Rollback = p.Deployment.Rollback
					result.Provenance["process.deployment.rollback"] = lyr
				}
			}
			if p.Documentation != nil {
				if result.Constitution.Process.Documentation == nil {
					result.Constitution.Process.Documentation = &storage.DocumentationConfig{}
				}
				if p.Documentation.APIDocs != "" {
					result.Constitution.Process.Documentation.APIDocs = p.Documentation.APIDocs
					result.Provenance["process.documentation.api_docs"] = lyr
				}
				if p.Documentation.Runbook != "" {
					result.Constitution.Process.Documentation.Runbook = p.Documentation.Runbook
					result.Provenance["process.documentation.runbook"] = lyr
				}
			}
		}

		// --- String list: Constraints (union, deduped) ---
		for _, c := range layer.Constraints {
			if !constraintSet[c] {
				constraintSet[c] = true
				constraintOrder = append(constraintOrder, c)
				result.Provenance["constraints["+c+"]"] = lyr
			}
		}

		// --- Tech stack ---
		if layer.Tech != nil {
			if result.Constitution.Tech == nil {
				result.Constitution.Tech = &storage.TechStack{}
			}
			tech := layer.Tech

			// Languages scalar + lists
			if tech.Languages != nil {
				if result.Constitution.Tech.Languages == nil {
					result.Constitution.Tech.Languages = &storage.Languages{}
				}
				lang := tech.Languages
				if lang.Primary != "" {
					result.Constitution.Tech.Languages.Primary = lang.Primary
					result.Provenance["tech_config.languages.primary"] = lyr
				}
				for _, a := range lang.Allowed {
					if !langAllowedSet[a] {
						langAllowedSet[a] = true
						langAllowedOrder = append(langAllowedOrder, a)
						result.Provenance["tech_config.languages.allowed["+a+"]"] = lyr
					}
				}
				for _, f := range lang.Forbidden {
					if !langForbiddenSet[f] {
						langForbiddenSet[f] = true
						langForbiddenOrder = append(langForbiddenOrder, f)
						result.Provenance["tech_config.languages.forbidden["+f+"]"] = lyr
					}
				}
				// ForbiddenReasons: map merge
				if len(lang.ForbiddenReasons) > 0 {
					if result.Constitution.Tech.Languages.ForbiddenReasons == nil {
						result.Constitution.Tech.Languages.ForbiddenReasons = map[string]string{}
					}
					for k, v := range lang.ForbiddenReasons {
						result.Constitution.Tech.Languages.ForbiddenReasons[k] = v
						result.Provenance["tech_config.languages.forbidden_reasons["+k+"]"] = lyr
					}
				}
			}

			// Map fields: Frameworks, Infrastructure, APIStandards, Data
			if len(tech.Frameworks) > 0 {
				if result.Constitution.Tech.Frameworks == nil {
					result.Constitution.Tech.Frameworks = map[string]string{}
				}
				for k, v := range tech.Frameworks {
					result.Constitution.Tech.Frameworks[k] = v
					result.Provenance["tech_config.frameworks["+k+"]"] = lyr
				}
			}
			if len(tech.Infrastructure) > 0 {
				if result.Constitution.Tech.Infrastructure == nil {
					result.Constitution.Tech.Infrastructure = map[string]string{}
				}
				for k, v := range tech.Infrastructure {
					result.Constitution.Tech.Infrastructure[k] = v
					result.Provenance["tech_config.infrastructure["+k+"]"] = lyr
				}
			}
			if len(tech.APIStandards) > 0 {
				if result.Constitution.Tech.APIStandards == nil {
					result.Constitution.Tech.APIStandards = map[string]string{}
				}
				for k, v := range tech.APIStandards {
					result.Constitution.Tech.APIStandards[k] = v
					result.Provenance["tech_config.api_standards["+k+"]"] = lyr
				}
			}
			if len(tech.Data) > 0 {
				if result.Constitution.Tech.Data == nil {
					result.Constitution.Tech.Data = map[string]string{}
				}
				for k, v := range tech.Data {
					result.Constitution.Tech.Data[k] = v
					result.Provenance["tech_config.data["+k+"]"] = lyr
				}
			}
		}

		// --- Keyed object list: Principles (merge by ID) ---
		for _, p := range layer.Principles {
			if p.ID == "" {
				continue
			}
			if p.Delete {
				if _, exists := principleMap[p.ID]; exists {
					delete(principleMap, p.ID)
					// Remove from order slice.
					principleOrder = removeString(principleOrder, p.ID)
					delete(result.Provenance, "principles["+p.ID+"]")
				}
				continue
			}
			if _, exists := principleMap[p.ID]; !exists {
				principleOrder = append(principleOrder, p.ID)
			}
			principleMap[p.ID] = p
			result.Provenance["principles["+p.ID+"]"] = lyr
		}

		// --- Keyed object list: Antipatterns (merge by Pattern) ---
		for _, a := range layer.Antipatterns {
			if a.Pattern == "" {
				continue
			}
			if a.Delete {
				if _, exists := antipatternMap[a.Pattern]; exists {
					delete(antipatternMap, a.Pattern)
					antipatternOrder = removeString(antipatternOrder, a.Pattern)
					delete(result.Provenance, "antipatterns["+a.Pattern+"]")
				}
				continue
			}
			if _, exists := antipatternMap[a.Pattern]; !exists {
				antipatternOrder = append(antipatternOrder, a.Pattern)
			}
			antipatternMap[a.Pattern] = a
			result.Provenance["antipatterns["+a.Pattern+"]"] = lyr
		}

		// --- Keyed object list: References (merge by Path) ---
		for _, r := range layer.References {
			if r.Path == "" {
				continue
			}
			if r.Delete {
				if _, exists := referenceMap[r.Path]; exists {
					delete(referenceMap, r.Path)
					referenceOrder = removeString(referenceOrder, r.Path)
					delete(result.Provenance, "references["+r.Path+"]")
				}
				continue
			}
			if _, exists := referenceMap[r.Path]; !exists {
				referenceOrder = append(referenceOrder, r.Path)
			}
			referenceMap[r.Path] = r
			result.Provenance["references["+r.Path+"]"] = lyr
		}
	}

	// Reconstruct slices from ordered keys (ensures non-nil empty slices).
	result.Constitution.Constraints = append(make([]string, 0, len(constraintOrder)), constraintOrder...)

	principles := make([]storage.Principle, 0, len(principleOrder))
	for _, id := range principleOrder {
		principles = append(principles, principleMap[id])
	}
	result.Constitution.Principles = principles

	antipatterns := make([]storage.Antipattern, 0, len(antipatternOrder))
	for _, pat := range antipatternOrder {
		antipatterns = append(antipatterns, antipatternMap[pat])
	}
	result.Constitution.Antipatterns = antipatterns

	references := make([]storage.Reference, 0, len(referenceOrder))
	for _, path := range referenceOrder {
		references = append(references, referenceMap[path])
	}
	result.Constitution.References = references

	// Reconstruct tech language lists from ordered dedup tracking (ensures non-nil).
	if result.Constitution.Tech != nil && result.Constitution.Tech.Languages != nil {
		result.Constitution.Tech.Languages.Allowed = append(make([]string, 0, len(langAllowedOrder)), langAllowedOrder...)
		result.Constitution.Tech.Languages.Forbidden = append(make([]string, 0, len(langForbiddenOrder)), langForbiddenOrder...)
	}

	return result, nil
}

// removeString removes the first occurrence of s from slice and returns the result.
func removeString(slice []string, s string) []string {
	if i := slices.Index(slice, s); i >= 0 {
		return slices.Delete(slice, i, i+1)
	}
	return slice
}
