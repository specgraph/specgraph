// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"context"
	"strings"
	"testing"
)

type fakeComposerBackend struct {
	constitution *ConstitutionSummary
	specSummary  *SpecSummary
	related      []*RelatedSpec
}

func (f *fakeComposerBackend) GetConstitution(_ context.Context) (*ConstitutionSummary, error) {
	if f.constitution != nil {
		return f.constitution, nil
	}
	return &ConstitutionSummary{PrimaryLanguage: "Go"}, nil
}
func (f *fakeComposerBackend) GetSpecSummary(_ context.Context, slug string) (*SpecSummary, error) {
	if f.specSummary != nil {
		return f.specSummary, nil
	}
	return &SpecSummary{Slug: slug, Intent: "test"}, nil
}
func (f *fakeComposerBackend) GetRelatedSpecs(_ context.Context, _ string) ([]*RelatedSpec, error) {
	return f.related, nil
}

func TestVersionString_RealOrDev(t *testing.T) {
	v := versionString()
	if v == "" {
		t.Error("versionString returned empty")
	}
}

func TestComposer_StageSectionsPresent(t *testing.T) {
	c := NewComposer(&fakeComposerBackend{})
	result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{
		Stage:   "shape",
		Slug:    "oauth-refresh",
		Posture: "partner",
	})
	if err != nil {
		t.Fatalf("ComposeStagePrompt: %v", err)
	}
	for _, marker := range []string{"# Persona", "# Orchestration", "# Conversation", "# Quality Heuristics", "# Shape", "oauth-refresh"} {
		if !strings.Contains(result.Body, marker) {
			t.Errorf("composed body missing marker %q", marker)
		}
	}
}
