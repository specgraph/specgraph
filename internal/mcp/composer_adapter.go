// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcp

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
)

// composerBackend bridges the MCP Client's ConnectRPC calls to the
// authoring.ComposerBackend interface required by authoring.Composer.
type composerBackend struct {
	client *Client
}

// GetConstitution fetches the active constitution and returns a bounded summary.
// Returns (nil, nil) when the server reports no constitution is configured.
func (b *composerBackend) GetConstitution(ctx context.Context) (*authoring.ConstitutionSummary, error) {
	resp, err := b.client.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
	if err != nil {
		return nil, fmt.Errorf("get constitution: %w", err)
	}
	c := resp.Msg.GetConstitution()
	if c == nil {
		return nil, nil
	}
	var lang string
	if c.GetTech() != nil && c.GetTech().GetLanguages() != nil {
		lang = c.GetTech().GetLanguages().GetPrimary()
	}
	var antipatterns []string
	for _, ap := range c.GetAntipatterns() {
		antipatterns = append(antipatterns, ap.GetPattern())
	}
	return &authoring.ConstitutionSummary{
		PrimaryLanguage: lang,
		KeyConstraints:  c.GetConstraints(),
		Antipatterns:    antipatterns,
	}, nil
}

// GetSpecSummary fetches a bounded view of the spec identified by slug.
// Returns authoring.ErrSpecNotFound when the server reports the spec is absent
// (either via connect.CodeNotFound or — defensively — via a nil Spec payload).
func (b *composerBackend) GetSpecSummary(ctx context.Context, slug string) (*authoring.SpecSummary, error) {
	resp, err := b.client.Spec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{Slug: slug}))
	if err != nil {
		// The real server maps storage.ErrSpecNotFound → connect.CodeNotFound
		// (see internal/server/authoring_handler.go). Translate that into the
		// composer's soft-miss sentinel so callers can skip the spec block.
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, authoring.ErrSpecNotFound
		}
		return nil, fmt.Errorf("get spec: %w", err)
	}
	s := resp.Msg.GetSpec()
	if s == nil {
		// Defense in depth: the real server never returns (success, nil-Spec),
		// but a fake or future implementation might.
		return nil, authoring.ErrSpecNotFound
	}
	return &authoring.SpecSummary{
		Slug:   s.GetSlug(),
		Intent: s.GetIntent(),
		Stage:  s.GetStage(),
		// PriorStageSummary intentionally left zero: no proto field yet (deferred).
	}, nil
}

// GetRelatedSpecs fetches the direct dependencies of the spec identified by slug.
func (b *composerBackend) GetRelatedSpecs(ctx context.Context, slug string) ([]*authoring.RelatedSpec, error) {
	resp, err := b.client.Graph.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{Slug: slug}))
	if err != nil {
		return nil, fmt.Errorf("get dependencies: %w", err)
	}
	var out []*authoring.RelatedSpec
	for _, dep := range resp.Msg.GetDependencies() {
		out = append(out, &authoring.RelatedSpec{
			Slug:         dep.GetSlug(),
			Intent:       dep.GetLabel(),
			Relationship: authoring.RelationshipDependsOn,
		})
	}
	return out, nil
}
