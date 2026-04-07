// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package merge_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/constitution/merge"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge_ScalarOverride(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Process: &storage.ProcessConfig{
			SpecReview: "required",
			Deployment: &storage.DeploymentConfig{
				Strategy: "rolling",
				Rollback: "manual",
			},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Process: &storage.ProcessConfig{
			Deployment: &storage.DeploymentConfig{
				Strategy: "blue-green",
			},
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	// Project overrides org's deployment strategy.
	require.NotNil(t, result.Constitution.Process)
	assert.Equal(t, "blue-green", result.Constitution.Process.Deployment.Strategy)
	// Rollback not set by project — org value preserved.
	assert.Equal(t, "manual", result.Constitution.Process.Deployment.Rollback)
	// SpecReview not set by project — org value preserved.
	assert.Equal(t, "required", result.Constitution.Process.SpecReview)

	// Provenance.
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["process.spec_review"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["process.deployment.strategy"])
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["process.deployment.rollback"])
}

func TestMerge_StringListUnion(t *testing.T) {
	org := &storage.Constitution{
		Layer:       storage.ConstitutionLayerOrg,
		Constraints: []string{"no-global-state", "immutable-data"},
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Allowed:   []string{"go", "python"},
				Forbidden: []string{"php"},
			},
		},
	}
	project := &storage.Constitution{
		Layer:       storage.ConstitutionLayerProject,
		Constraints: []string{"immutable-data", "pure-functions"}, // "immutable-data" is a dupe
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Allowed:   []string{"go", "typescript"}, // "go" is a dupe
				Forbidden: []string{"ruby"},
			},
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	// Deduped union of constraints.
	assert.ElementsMatch(t, []string{"no-global-state", "immutable-data", "pure-functions"}, result.Constitution.Constraints)

	// Deduped union of allowed languages.
	assert.ElementsMatch(t, []string{"go", "python", "typescript"}, result.Constitution.Tech.Languages.Allowed)
	assert.ElementsMatch(t, []string{"php", "ruby"}, result.Constitution.Tech.Languages.Forbidden)
}

func TestMerge_KeyedObjectMerge(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "org-p1"},
			{ID: "p2", Statement: "org-p2"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p2", Statement: "project-p2"}, // override
			{ID: "p3", Statement: "project-p3"}, // new
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	require.Len(t, result.Constitution.Principles, 3)

	byID := map[string]storage.Principle{}
	for _, p := range result.Constitution.Principles {
		byID[p.ID] = p
	}

	// Org's p1 kept as-is.
	assert.Equal(t, "org-p1", byID["p1"].Statement)
	// Org's p2 overridden by project.
	assert.Equal(t, "project-p2", byID["p2"].Statement)
	// Project's p3 added.
	assert.Equal(t, "project-p3", byID["p3"].Statement)

	// Provenance.
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["principles[p1]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["principles[p2]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["principles[p3]"])
}

func TestMerge_DeleteDirective(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "org-p1"},
			{ID: "p2", Statement: "org-p2"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p2", Delete: true}, // delete org's p2
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	require.Len(t, result.Constitution.Principles, 1)
	assert.Equal(t, "p1", result.Constitution.Principles[0].ID)
	// p2 provenance should be removed.
	_, hasProv := result.Provenance["principles[p2]"]
	assert.False(t, hasProv)
}

func TestMerge_DeleteNonexistent_NoOp(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "org-p1"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p999", Delete: true}, // does not exist — no-op
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)
	require.Len(t, result.Constitution.Principles, 1)
	assert.Equal(t, "p1", result.Constitution.Principles[0].ID)
}

func TestMerge_SingleLayer(t *testing.T) {
	layer := &storage.Constitution{
		Layer:       storage.ConstitutionLayerProject,
		Constraints: []string{"immutable-data"},
		Principles: []storage.Principle{
			{ID: "p1", Statement: "project-p1"},
		},
		Process: &storage.ProcessConfig{
			SpecReview: "required",
		},
	}

	result, err := merge.Layers([]*storage.Constitution{layer})
	require.NoError(t, err)

	assert.Equal(t, []string{"immutable-data"}, result.Constitution.Constraints)
	require.Len(t, result.Constitution.Principles, 1)
	assert.Equal(t, "project-p1", result.Constitution.Principles[0].Statement)
	assert.Equal(t, "required", result.Constitution.Process.SpecReview)

	// All provenance points to project.
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["constraints[immutable-data]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["principles[p1]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["process.spec_review"])
}

func TestMerge_EmptyInput(t *testing.T) {
	result, err := merge.Layers(nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Constitution)
	require.NotNil(t, result.Provenance)
	assert.Empty(t, result.Provenance)
}

func TestMerge_FourLayers_DomainWins(t *testing.T) {
	user := &storage.Constitution{
		Layer: storage.ConstitutionLayerUser,
		Process: &storage.ProcessConfig{
			SpecReview: "user-value",
		},
	}
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Process: &storage.ProcessConfig{
			SpecReview: "org-value",
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Process: &storage.ProcessConfig{
			SpecReview: "project-value",
		},
	}
	domain := &storage.Constitution{
		Layer: storage.ConstitutionLayerDomain,
		Process: &storage.ProcessConfig{
			SpecReview: "domain-value",
		},
	}

	result, err := merge.Layers([]*storage.Constitution{user, org, project, domain})
	require.NoError(t, err)

	assert.Equal(t, "domain-value", result.Constitution.Process.SpecReview)
	assert.Equal(t, storage.ConstitutionLayerDomain, result.Provenance["process.spec_review"])
}

func TestMerge_DeleteOverrideThenDelete(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "org-p1"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "project-p1"}, // override
		},
	}
	domain := &storage.Constitution{
		Layer: storage.ConstitutionLayerDomain,
		Principles: []storage.Principle{
			{ID: "p1", Delete: true}, // delete entirely
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project, domain})
	require.NoError(t, err)

	assert.Empty(t, result.Constitution.Principles)
	_, hasProv := result.Provenance["principles[p1]"]
	assert.False(t, hasProv)
}

func TestMerge_DeleteOnlyItem(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "only"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p1", Delete: true},
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	// Must be empty slice, not nil.
	require.NotNil(t, result.Constitution.Principles)
	assert.Len(t, result.Constitution.Principles, 0)
}

func TestMerge_MapMerge(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Tech: &storage.TechStack{
			Frameworks: map[string]string{
				"rpc":  "connectrpc",
				"http": "chi",
			},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Tech: &storage.TechStack{
			Frameworks: map[string]string{
				"rpc": "grpc",  // override
				"orm": "sqlc",  // new key
			},
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	require.NotNil(t, result.Constitution.Tech)
	// Org's "http" key preserved.
	assert.Equal(t, "chi", result.Constitution.Tech.Frameworks["http"])
	// Project overrides org's "rpc" key.
	assert.Equal(t, "grpc", result.Constitution.Tech.Frameworks["rpc"])
	// Project's new "orm" key added.
	assert.Equal(t, "sqlc", result.Constitution.Tech.Frameworks["orm"])

	// Provenance.
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["tech_config.frameworks[http]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["tech_config.frameworks[rpc]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["tech_config.frameworks[orm]"])
}

func TestMerge_NilLayerSkipped(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Process: &storage.ProcessConfig{
			SpecReview: "required",
		},
	}
	domain := &storage.Constitution{
		Layer: storage.ConstitutionLayerDomain,
		Process: &storage.ProcessConfig{
			SpecReview: "mandatory",
		},
	}

	// nil layer between org and domain — must be silently skipped.
	result, err := merge.Layers([]*storage.Constitution{org, nil, domain})
	require.NoError(t, err)

	// Domain wins; nil layer caused no panic and no data loss.
	require.NotNil(t, result.Constitution.Process)
	assert.Equal(t, "mandatory", result.Constitution.Process.SpecReview)
	assert.Equal(t, storage.ConstitutionLayerDomain, result.Provenance["process.spec_review"])
}

func TestMerge_ProcessFullMerge(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Process: &storage.ProcessConfig{
			SecurityReview: &storage.SecurityReviewConfig{
				When: "always",
			},
			Documentation: &storage.DocumentationConfig{
				APIDocs: "required",
				Runbook: "required",
			},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Process: &storage.ProcessConfig{
			Documentation: &storage.DocumentationConfig{
				APIDocs: "optional", // override org's api_docs
			},
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	require.NotNil(t, result.Constitution.Process)

	// SecurityReview set by org, not overridden by project.
	require.NotNil(t, result.Constitution.Process.SecurityReview)
	assert.Equal(t, "always", result.Constitution.Process.SecurityReview.When)
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["process.security_review.when"])

	// Documentation: project wins on api_docs, org's runbook preserved.
	require.NotNil(t, result.Constitution.Process.Documentation)
	assert.Equal(t, "optional", result.Constitution.Process.Documentation.APIDocs)
	assert.Equal(t, "required", result.Constitution.Process.Documentation.Runbook)
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["process.documentation.api_docs"])
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["process.documentation.runbook"])
}

func TestMerge_ForbiddenLanguages(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Primary:   "go",
				Forbidden: []string{"java"},
				ForbiddenReasons: map[string]string{
					"java": "JVM startup cost",
				},
			},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Primary:   "typescript", // override primary
				Forbidden: []string{"ruby"},
				ForbiddenReasons: map[string]string{
					"ruby": "no typing",
				},
			},
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	require.NotNil(t, result.Constitution.Tech)
	require.NotNil(t, result.Constitution.Tech.Languages)

	// Project overrides primary language.
	assert.Equal(t, "typescript", result.Constitution.Tech.Languages.Primary)
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["tech_config.languages.primary"])

	// Both forbidden entries present (union).
	assert.ElementsMatch(t, []string{"java", "ruby"}, result.Constitution.Tech.Languages.Forbidden)

	// Both forbidden reasons present (map merge).
	require.NotNil(t, result.Constitution.Tech.Languages.ForbiddenReasons)
	assert.Equal(t, "JVM startup cost", result.Constitution.Tech.Languages.ForbiddenReasons["java"])
	assert.Equal(t, "no typing", result.Constitution.Tech.Languages.ForbiddenReasons["ruby"])

	// Provenance for forbidden entries.
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["tech_config.languages.forbidden[java]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["tech_config.languages.forbidden[ruby]"])
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["tech_config.languages.forbidden_reasons[java]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["tech_config.languages.forbidden_reasons[ruby]"])
}

func TestMerge_AntipatternMergeAndDelete(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Antipatterns: []storage.Antipattern{
			{Pattern: "god object", Why: "org-why", Instead: "org-instead"},
			{Pattern: "magic numbers", Why: "hard to read", Instead: "use constants"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Antipatterns: []storage.Antipattern{
			{Pattern: "god object", Why: "project-why", Instead: "project-instead"}, // override
		},
	}
	domain := &storage.Constitution{
		Layer: storage.ConstitutionLayerDomain,
		Antipatterns: []storage.Antipattern{
			{Pattern: "magic numbers", Delete: true}, // delete
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project, domain})
	require.NoError(t, err)

	// Only "god object" should remain.
	require.Len(t, result.Constitution.Antipatterns, 1)
	ap := result.Constitution.Antipatterns[0]
	assert.Equal(t, "god object", ap.Pattern)
	assert.Equal(t, "project-why", ap.Why)
	assert.Equal(t, "project-instead", ap.Instead)

	// Provenance: god object owned by project.
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["antipatterns[god object]"])

	// magic numbers deleted — no provenance entry.
	_, hasProv := result.Provenance["antipatterns[magic numbers]"]
	assert.False(t, hasProv)
}

func TestMerge_ReferenceMergeAndDelete(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		References: []storage.Reference{
			{Type: "api", Path: "/docs/api.md"},
			{Type: "arch", Path: "/docs/arch.md"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		References: []storage.Reference{
			{Delete: true, Path: "/docs/arch.md"},    // delete arch doc
			{Type: "setup", Path: "/docs/setup.md"},  // add new
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	// Expect /docs/api.md and /docs/setup.md; /docs/arch.md deleted.
	require.Len(t, result.Constitution.References, 2)

	paths := make(map[string]storage.Reference)
	for _, r := range result.Constitution.References {
		paths[r.Path] = r
	}

	assert.Contains(t, paths, "/docs/api.md")
	assert.Equal(t, "api", paths["/docs/api.md"].Type)
	assert.Contains(t, paths, "/docs/setup.md")
	assert.Equal(t, "setup", paths["/docs/setup.md"].Type)
	assert.NotContains(t, paths, "/docs/arch.md")

	// Provenance.
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["references[/docs/api.md]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["references[/docs/setup.md]"])
	_, hasProv := result.Provenance["references[/docs/arch.md]"]
	assert.False(t, hasProv)
}

func TestMerge_TechMapsComplete(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Tech: &storage.TechStack{
			Infrastructure: map[string]string{
				"cloud":    "aws",
				"registry": "ecr",
			},
			APIStandards: map[string]string{
				"style":  "rest",
				"format": "json",
			},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Tech: &storage.TechStack{
			Infrastructure: map[string]string{
				"cloud": "gcp",         // override
				"cdn":   "cloudfront",  // new key
			},
			APIStandards: map[string]string{
				"style":   "graphql",   // override
				"version": "semver",    // new key
			},
		},
	}

	result, err := merge.Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	require.NotNil(t, result.Constitution.Tech)

	// Infrastructure: project wins on "cloud", org's "registry" preserved, project adds "cdn".
	assert.Equal(t, "gcp", result.Constitution.Tech.Infrastructure["cloud"])
	assert.Equal(t, "ecr", result.Constitution.Tech.Infrastructure["registry"])
	assert.Equal(t, "cloudfront", result.Constitution.Tech.Infrastructure["cdn"])

	// APIStandards: project wins on "style", org's "format" preserved, project adds "version".
	assert.Equal(t, "graphql", result.Constitution.Tech.APIStandards["style"])
	assert.Equal(t, "json", result.Constitution.Tech.APIStandards["format"])
	assert.Equal(t, "semver", result.Constitution.Tech.APIStandards["version"])

	// Provenance for infrastructure.
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["tech_config.infrastructure[cloud]"])
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["tech_config.infrastructure[registry]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["tech_config.infrastructure[cdn]"])

	// Provenance for api_standards.
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["tech_config.api_standards[style]"])
	assert.Equal(t, storage.ConstitutionLayerOrg, result.Provenance["tech_config.api_standards[format]"])
	assert.Equal(t, storage.ConstitutionLayerProject, result.Provenance["tech_config.api_standards[version]"])
}
