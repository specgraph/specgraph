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
				"rpc":  "grpc",    // override
				"orm":  "sqlc",    // new key
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
