// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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

func TestComposer_EmitsInvocationLog(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	c := NewComposer(&fakeComposerBackend{})
	if _, err := c.ComposeStagePrompt(context.Background(), ComposeInput{
		Stage:   "shape",
		Slug:    "oauth-refresh",
		Posture: "partner",
	}); err != nil {
		t.Fatalf("ComposeStagePrompt: %v", err)
	}

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record); err != nil {
		t.Fatalf("decode log record: %v; raw=%q", err, buf.String())
	}
	if record["msg"] != "composer.invocation" {
		t.Errorf("msg = %v, want composer.invocation", record["msg"])
	}
	for _, key := range []string{"stage", "slug", "posture", "stable_tokens", "dynamic_tokens", "total_tokens", "truncated_count"} {
		if _, ok := record[key]; !ok {
			t.Errorf("log record missing key %q; record=%v", key, record)
		}
	}
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
