// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// B.1 — composerBackend.GetConstitution tests
// ---------------------------------------------------------------------------

// TestComposerAdapter_GetConstitution_NilConstitutionMapsToEmpty verifies that
// when GetConstitution returns a response with no Constitution field set, the
// adapter returns an empty (non-nil) ConstitutionSummary rather than nil.
func TestComposerAdapter_GetConstitution_NilConstitutionMapsToEmpty(t *testing.T) {
	c := &Client{
		Constitution: &mockConstitutionService{
			getConstitution: func() (*specv1.GetConstitutionResponse, error) {
				return &specv1.GetConstitutionResponse{}, nil
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetConstitution(context.Background())
	require.NoError(t, err)
	// Nil constitution maps to (nil, nil) per the GetConstitution docstring —
	// the adapter returns nil when no constitution is configured.
	require.Nil(t, summary, "nil Constitution in response should produce nil summary")
}

// TestComposerAdapter_GetConstitution_RPCErrorWrapped verifies that an RPC error
// is wrapped via "get constitution: %w" and the original error is reachable via errors.Is.
func TestComposerAdapter_GetConstitution_RPCErrorWrapped(t *testing.T) {
	inner := connect.NewError(connect.CodeInternal, fmt.Errorf("boom"))
	c := &Client{
		Constitution: &mockConstitutionService{
			getConstitution: func() (*specv1.GetConstitutionResponse, error) {
				return nil, inner
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetConstitution(context.Background())
	require.Nil(t, summary)
	require.Error(t, err)
	require.True(t, errors.Is(err, inner),
		"expected errors.Is(err, inner), got %v", err)
}

// TestComposerAdapter_GetConstitution_PopulatesFields verifies that a populated
// Constitution response is correctly mapped to ConstitutionSummary fields.
func TestComposerAdapter_GetConstitution_PopulatesFields(t *testing.T) {
	c := &Client{
		Constitution: &mockConstitutionService{
			getConstitution: func() (*specv1.GetConstitutionResponse, error) {
				return &specv1.GetConstitutionResponse{
					Constitution: &specv1.Constitution{
						Tech: &specv1.TechConfig{
							Languages: &specv1.LanguageConfig{
								Primary: "Go",
							},
						},
						Constraints:  []string{"c1", "c2"},
						Antipatterns: []*specv1.Antipattern{{Pattern: "ap1"}, {Pattern: "ap2"}},
					},
				}, nil
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetConstitution(context.Background())
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Equal(t, "Go", summary.PrimaryLanguage)
	require.Equal(t, []string{"c1", "c2"}, summary.KeyConstraints)
	require.Equal(t, []string{"ap1", "ap2"}, summary.Antipatterns)
}

// ---------------------------------------------------------------------------
// B.2 — composerBackend.GetRelatedSpecs tests
// ---------------------------------------------------------------------------

// TestComposerAdapter_GetRelatedSpecs_RPCErrorWrapped verifies that an RPC error
// from GetDependencies is wrapped via "get dependencies: %w".
func TestComposerAdapter_GetRelatedSpecs_RPCErrorWrapped(t *testing.T) {
	inner := connect.NewError(connect.CodeInternal, fmt.Errorf("deps boom"))
	c := &Client{
		Graph: &mockGraphService{
			getDeps: func(_ string) (*specv1.GetDependenciesResponse, error) {
				return nil, inner
			},
		},
	}
	b := &composerBackend{client: c}

	result, err := b.GetRelatedSpecs(context.Background(), "any-slug")
	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, errors.Is(err, inner),
		"expected errors.Is(err, inner) to hold through wrap, got %v", err)
}

// TestComposerAdapter_GetRelatedSpecs_TranslatesDependencies verifies that
// NodeRef.Slug/Label are mapped to RelatedSpec.Slug/Intent and that
// Relationship is set to RelationshipDependsOn for all entries.
func TestComposerAdapter_GetRelatedSpecs_TranslatesDependencies(t *testing.T) {
	c := &Client{
		Graph: &mockGraphService{
			getDeps: func(_ string) (*specv1.GetDependenciesResponse, error) {
				return &specv1.GetDependenciesResponse{
					Dependencies: []*specv1.NodeRef{
						{Slug: "alpha", Label: "Alpha intent"},
						{Slug: "beta", Label: "Beta intent"},
					},
				}, nil
			},
		},
	}
	b := &composerBackend{client: c}

	result, err := b.GetRelatedSpecs(context.Background(), "root")
	require.NoError(t, err)
	require.Len(t, result, 2)

	require.Equal(t, "alpha", result[0].Slug)
	require.Equal(t, "Alpha intent", result[0].Intent)
	require.Equal(t, authoring.RelationshipDependsOn, result[0].Relationship)

	require.Equal(t, "beta", result[1].Slug)
	require.Equal(t, "Beta intent", result[1].Intent)
	require.Equal(t, authoring.RelationshipDependsOn, result[1].Relationship)
}

// TestComposerAdapter_GetSpecSummary_NotFoundMaps verifies that a
// connect.CodeNotFound error from the underlying GetSpec RPC is translated
// into authoring.ErrSpecNotFound. Without this mapping the soft-miss path
// documented on authoring.ErrSpecNotFound would never trigger in production
// (the real server maps storage.ErrSpecNotFound → connect.CodeNotFound).
func TestComposerAdapter_GetSpecSummary_NotFoundMaps(t *testing.T) {
	c := &Client{
		Spec: &mockSpecService{
			getSpec: func(_ string) (*specv1.GetSpecResponse, error) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("spec not found"))
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetSpecSummary(context.Background(), "missing-slug")
	require.Nil(t, summary)
	require.Error(t, err)
	require.True(t, errors.Is(err, authoring.ErrSpecNotFound),
		"expected errors.Is(err, authoring.ErrSpecNotFound), got %v", err)
}

// TestComposerAdapter_GetSpecSummary_NilSpecMaps verifies the defense-in-depth
// nil-Spec branch: if a backend returns success with no spec payload, the
// adapter still produces ErrSpecNotFound rather than a nil-summary leak.
func TestComposerAdapter_GetSpecSummary_NilSpecMaps(t *testing.T) {
	c := &Client{
		Spec: &mockSpecService{
			getSpec: func(_ string) (*specv1.GetSpecResponse, error) {
				return &specv1.GetSpecResponse{}, nil
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetSpecSummary(context.Background(), "any-slug")
	require.Nil(t, summary)
	require.True(t, errors.Is(err, authoring.ErrSpecNotFound),
		"expected errors.Is(err, authoring.ErrSpecNotFound), got %v", err)
}

// TestComposerAdapter_GetSpecSummary_OtherErrorWrapped verifies that
// non-NotFound errors are wrapped opaquely (callers should not treat them
// as a soft miss).
func TestComposerAdapter_GetSpecSummary_OtherErrorWrapped(t *testing.T) {
	c := &Client{
		Spec: &mockSpecService{
			getSpec: func(_ string) (*specv1.GetSpecResponse, error) {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("boom"))
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetSpecSummary(context.Background(), "any-slug")
	require.Nil(t, summary)
	require.Error(t, err)
	require.False(t, errors.Is(err, authoring.ErrSpecNotFound),
		"non-NotFound error should not match ErrSpecNotFound, got %v", err)
}

// TestComposerAdapter_GetSpecSummary_Success verifies the happy path: a
// populated Spec produces a SpecSummary with Slug/Intent/Stage copied through.
func TestComposerAdapter_GetSpecSummary_Success(t *testing.T) {
	c := &Client{
		Spec: &mockSpecService{
			getSpec: func(slug string) (*specv1.GetSpecResponse, error) {
				return &specv1.GetSpecResponse{
					Spec: &specv1.Spec{Slug: slug, Intent: "the intent", Stage: "shape"},
				}, nil
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetSpecSummary(context.Background(), "my-spec")
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Equal(t, "my-spec", summary.Slug)
	require.Equal(t, "the intent", summary.Intent)
	require.Equal(t, "shape", summary.Stage)
}
