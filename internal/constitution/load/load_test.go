// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package load_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/constitution/load"
	"github.com/specgraph/specgraph/internal/storage"
)

func TestLoadFromYAML_HappyPath(t *testing.T) {
	yaml := []byte(`name: test-constitution
layer: project
principles:
  - id: p1
    statement: Prefer explicit
constraints:
  - never use eval
`)
	c, err := load.LoadFromYAML(yaml)
	require.NoError(t, err)
	assert.Equal(t, "test-constitution", c.Name)
	assert.Equal(t, storage.ConstitutionLayerProject, c.Layer)
	require.Len(t, c.Principles, 1)
	assert.Equal(t, "p1", c.Principles[0].ID)
	assert.Equal(t, "Prefer explicit", c.Principles[0].Statement)
	require.Len(t, c.Constraints, 1)
	assert.Equal(t, "never use eval", c.Constraints[0])
}

func TestLoadFromYAML_MalformedYAML(t *testing.T) {
	yaml := []byte(`name: [unclosed`)
	_, err := load.LoadFromYAML(yaml)
	require.Error(t, err)
}

func TestLoadFromYAML_EmptyDoc(t *testing.T) {
	c, err := load.LoadFromYAML([]byte("{}"))
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "", c.Name)
	assert.Equal(t, storage.ConstitutionLayer(""), c.Layer)
}

func TestLoadFromYAML_InvalidLayer(t *testing.T) {
	yaml := []byte(`layer: not-a-real-layer`)
	_, err := load.LoadFromYAML(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid layer")
}

func TestLoadFromYAML_AllLayers(t *testing.T) {
	for _, layer := range []string{"user", "org", "project", "domain"} {
		yaml := []byte("layer: " + layer + "\n")
		c, err := load.LoadFromYAML(yaml)
		require.NoError(t, err, "layer=%s", layer)
		assert.Equal(t, storage.ConstitutionLayer(layer), c.Layer)
	}
}

func TestLoadFromYAML_FullConstitution(t *testing.T) {
	yaml := []byte(`name: full
layer: org
principles:
  - id: p1
    statement: S1
    rationale: R1
  - id: p2
    statement: S2
constraints:
  - c1
  - c2
antipatterns:
  - pattern: bad-pat
    why: because
    instead: do-this
references:
  - type: external
    path: https://example.com
tech:
  languages:
    primary: go
    allowed: [go, sql]
    forbidden: [javascript]
  frameworks:
    rpc: connect
process:
  spec_review: pr
`)
	c, err := load.LoadFromYAML(yaml)
	require.NoError(t, err)
	assert.Equal(t, "full", c.Name)
	assert.Equal(t, storage.ConstitutionLayerOrg, c.Layer)
	assert.Len(t, c.Principles, 2)
	assert.Equal(t, "R1", c.Principles[0].Rationale)
	assert.Len(t, c.Constraints, 2)
	require.Len(t, c.Antipatterns, 1)
	assert.Equal(t, "bad-pat", c.Antipatterns[0].Pattern)
	require.Len(t, c.References, 1)
	assert.Equal(t, "https://example.com", c.References[0].Path)
	require.NotNil(t, c.Tech)
	require.NotNil(t, c.Tech.Languages)
	assert.Equal(t, "go", c.Tech.Languages.Primary)
	assert.Equal(t, []string{"go", "sql"}, c.Tech.Languages.Allowed)
	assert.Equal(t, "connect", c.Tech.Frameworks["rpc"])
	require.NotNil(t, c.Process)
	assert.Equal(t, "pr", c.Process.SpecReview)
}

func TestLoadFromYAML_ToProto_RoundTrip(t *testing.T) {
	yaml := []byte(`name: rt
layer: project
principles:
  - id: p1
    statement: S
`)
	c, err := load.LoadFromYAML(yaml)
	require.NoError(t, err)

	pb := load.ToProto(c)
	require.NotNil(t, pb)
	assert.Equal(t, "rt", pb.GetName())
	assert.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT, pb.GetLayer())
	require.Len(t, pb.GetPrinciples(), 1)
	assert.Equal(t, "p1", pb.GetPrinciples()[0].GetId())
}
