// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
		Stage:   StageShape,
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

func TestComposer_InvalidStageReturnsErrInvalidStage(t *testing.T) {
	c := NewComposer(&fakeComposerBackend{})
	for _, stage := range []Stage{"", "Shape", "bogus", "APPROVE"} {
		_, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: stage, Slug: "s"})
		if !errors.Is(err, ErrInvalidStage) {
			t.Errorf("Stage=%q: err = %v, want errors.Is(err, ErrInvalidStage)", stage, err)
		}
	}
}

func TestNewComposer_NilBackendPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewComposer(nil) did not panic")
		}
	}()
	NewComposer(nil)
}

func TestComposer_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewComposer(&fakeComposerBackend{})
	_, err := c.ComposeStagePrompt(ctx, ComposeInput{Stage: StageShape, Slug: "s"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want errors.Is(err, context.Canceled)", err)
	}
}

type errorBackend struct {
	constitutionErr error
	specErr         error
	relatedErr      error
}

func (e *errorBackend) GetConstitution(_ context.Context) (*ConstitutionSummary, error) {
	if e.constitutionErr != nil {
		return nil, e.constitutionErr
	}
	return &ConstitutionSummary{PrimaryLanguage: "Go"}, nil
}
func (e *errorBackend) GetSpecSummary(_ context.Context, _ string) (*SpecSummary, error) {
	if e.specErr != nil {
		return nil, e.specErr
	}
	return &SpecSummary{Slug: "s", Intent: "i"}, nil
}
func (e *errorBackend) GetRelatedSpecs(_ context.Context, _ string) ([]*RelatedSpec, error) {
	if e.relatedErr != nil {
		return nil, e.relatedErr
	}
	return nil, nil
}

func TestComposer_BackendErrorsPropagate(t *testing.T) {
	sentinel := errors.New("backend boom")
	cases := []struct {
		name    string
		backend ComposerBackend
	}{
		{"constitution", &errorBackend{constitutionErr: sentinel}},
		{"spec", &errorBackend{specErr: sentinel}},
		{"related", &errorBackend{relatedErr: sentinel}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewComposer(tc.backend)
			result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: StageShape, Slug: "s"})
			if result != nil {
				t.Errorf("result = %v, want nil on error", result)
			}
			if !errors.Is(err, sentinel) {
				t.Errorf("err = %v, want errors.Is(err, sentinel)", err)
			}
		})
	}
}

func TestComposer_SpecNotFoundIsSoftMiss(t *testing.T) {
	backend := &errorBackend{specErr: ErrSpecNotFound}
	c := NewComposer(backend)
	result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: StageShape, Slug: "missing-slug"})
	if err != nil {
		t.Fatalf("ComposeStagePrompt: expected no error on ErrSpecNotFound, got %v", err)
	}
	if strings.Contains(result.Body, "**Spec missing-slug**") {
		t.Error("body should not contain spec block when ErrSpecNotFound is returned")
	}
}

type bulkConstitutionBackend struct{ fakeComposerBackend }

func (b *bulkConstitutionBackend) GetConstitution(_ context.Context) (*ConstitutionSummary, error) {
	return &ConstitutionSummary{
		PrimaryLanguage: "Go",
		KeyConstraints:  []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7"}, // 7 → truncate 2
		Antipatterns:    []string{"a1", "a2", "a3", "a4", "a5", "a6"},       // 6 → truncate 1
	}, nil
}

func TestComposer_TruncationCounter(t *testing.T) {
	c := NewComposer(&bulkConstitutionBackend{})
	result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: StageShape, Slug: "s"})
	if err != nil {
		t.Fatalf("ComposeStagePrompt: %v", err)
	}
	// Both constraints and antipatterns were truncated, so the section-level
	// counter increments twice. If this assertion fails, the truncation logic
	// regressed (wrong operator, off-by-one, counter dropped).
	if result.TruncatedCount != 2 {
		t.Errorf("TruncatedCount = %d, want 2", result.TruncatedCount)
	}
	// Dropped items must not appear in the body.
	for _, dropped := range []string{"c6", "c7", "a6"} {
		if strings.Contains(result.Body, dropped) {
			t.Errorf("body contains dropped item %q", dropped)
		}
	}
	// First-5 items must appear.
	for _, kept := range []string{"c1", "c5", "a1", "a5"} {
		if !strings.Contains(result.Body, kept) {
			t.Errorf("body missing kept item %q", kept)
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
		Stage:   StageShape,
		Slug:    "oauth-refresh",
		Posture: "partner",
	})
	if err != nil {
		t.Fatalf("ComposeStagePrompt: %v", err)
	}
	for _, marker := range []string{"# Persona", "# Orchestration", "# Conversation", "# Quality Heuristics", "# Stage: Shape", "oauth-refresh"} {
		if !strings.Contains(result.Body, marker) {
			t.Errorf("composed body missing marker %q", marker)
		}
	}
}

// TestComposer_PriorStageSummaryRendered verifies that a non-empty PriorStageSummary
// produces the expected "**Prior stage summary**: <value>" line in the body.
func TestComposer_PriorStageSummaryRendered(t *testing.T) {
	b := &fakeComposerBackend{
		specSummary: &SpecSummary{
			Slug:              "my-spec",
			Intent:            "Do something useful",
			Stage:             "shape",
			PriorStageSummary: "The spark identified a caching problem.",
		},
	}
	c := NewComposer(b)
	result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: StageShape, Slug: "my-spec"})
	if err != nil {
		t.Fatalf("ComposeStagePrompt: %v", err)
	}
	want := "**Prior stage summary**: The spark identified a caching problem."
	if !strings.Contains(result.Body, want) {
		t.Errorf("body missing %q", want)
	}
}

// TestComposer_MultipleRelatedSpecs verifies that three related specs appear
// comma-separated in the body.
func TestComposer_MultipleRelatedSpecs(t *testing.T) {
	b := &fakeComposerBackend{
		related: []*RelatedSpec{
			{Slug: "alpha", Relationship: RelationshipDependsOn},
			{Slug: "beta", Relationship: RelationshipBlocks},
			{Slug: "gamma", Relationship: RelationshipComposes},
		},
	}
	c := NewComposer(b)
	result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: StageShape, Slug: "root-spec"})
	if err != nil {
		t.Fatalf("ComposeStagePrompt: %v", err)
	}
	for _, slug := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(result.Body, slug) {
			t.Errorf("body missing related spec slug %q", slug)
		}
	}
	// Verify the ", " joiner between entries.
	if !strings.Contains(result.Body, "alpha (dependsOn), beta (blocks), gamma (composes)") {
		t.Errorf("related specs not comma-joined correctly")
	}
}

// TestComposer_NilConstitutionSkipped verifies that a (nil, nil) return from
// GetConstitution causes the body to have "# Current State" but no
// "**Constitution summary**" line.
func TestComposer_NilConstitutionSkipped(t *testing.T) {
	b := &nilConstitutionBackend{}
	c := NewComposer(b)
	result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: StageShape, Slug: "any-spec"})
	if err != nil {
		t.Fatalf("ComposeStagePrompt: %v", err)
	}
	if !strings.Contains(result.Body, "# Current State") {
		t.Error("body missing '# Current State' section")
	}
	if strings.Contains(result.Body, "**Constitution summary**") {
		t.Error("body should not contain '**Constitution summary**' when constitution is nil")
	}
}

type nilConstitutionBackend struct{ fakeComposerBackend }

func (n *nilConstitutionBackend) GetConstitution(_ context.Context) (*ConstitutionSummary, error) {
	return nil, nil
}

// TestRelationship_IsValid verifies that IsValid returns true for the three
// exported constants and false for everything else (empty, unknown, case-mismatch).
func TestRelationship_IsValid(t *testing.T) {
	cases := []struct {
		name string
		r    Relationship
		want bool
	}{
		{"DependsOn constant", RelationshipDependsOn, true},
		{"Blocks constant", RelationshipBlocks, true},
		{"Composes constant", RelationshipComposes, true},
		{"empty string", Relationship(""), false},
		{"unknown value", Relationship("BlockedBy"), false},
		{"case mismatch", Relationship("DependsOn"), false}, // constant is "dependsOn" (lowercase d)
		{"trailing whitespace", Relationship("dependsOn "), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.IsValid(); got != tc.want {
				t.Errorf("Relationship(%q).IsValid() = %v, want %v", string(tc.r), got, tc.want)
			}
		})
	}
}

// TestComposer_InvalidRelationshipSkipped verifies that a RelatedSpec with an
// unrecognised Relationship is silently dropped from the body and that a
// slog.Warn record is emitted for observability.
func TestComposer_InvalidRelationshipSkipped(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	b := &fakeComposerBackend{
		related: []*RelatedSpec{
			{Slug: "alpha", Relationship: RelationshipDependsOn},
			{Slug: "invalid-spec", Relationship: Relationship("BlockedBy")}, // capital B — invalid
			{Slug: "gamma", Relationship: RelationshipBlocks},
		},
	}
	c := NewComposer(b)
	result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: StageShape, Slug: "root-spec"})
	if err != nil {
		t.Fatalf("ComposeStagePrompt: %v", err)
	}

	// Valid entries must appear.
	if !strings.Contains(result.Body, "alpha (dependsOn)") {
		t.Errorf("body missing %q", "alpha (dependsOn)")
	}
	if !strings.Contains(result.Body, "gamma (blocks)") {
		t.Errorf("body missing %q", "gamma (blocks)")
	}

	// The invalid relationship must NOT leak into the body.
	if strings.Contains(result.Body, "BlockedBy") {
		t.Error("body must not contain invalid relationship value \"BlockedBy\"")
	}

	// Decode all log records — there may be the invocation log plus the warn.
	var warnFound bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode log record: %v; raw=%q", err, line)
		}
		if record["msg"] == "composer.invalid_relationship_skipped" {
			warnFound = true
			if record["level"] != "WARN" {
				t.Errorf("warn record level = %v, want WARN", record["level"])
			}
			if record["relationship"] != "BlockedBy" {
				t.Errorf("warn record relationship = %v, want BlockedBy", record["relationship"])
			}
		}
	}
	if !warnFound {
		t.Errorf("expected slog WARN record with msg %q; log output: %q",
			"composer.invalid_relationship_skipped", buf.String())
	}
}

// TestComposer_InvalidStageErrorFormat verifies that the error message for an
// invalid stage contains both the offending value and the word "valid",
// and that the error wraps ErrInvalidStage so callers can sentinel-check it.
func TestComposer_InvalidStageErrorFormat(t *testing.T) {
	c := NewComposer(&fakeComposerBackend{})
	_, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: "bogus-stage", Slug: "s"})
	if err == nil {
		t.Fatal("expected error for invalid stage, got nil")
	}
	if !strings.Contains(err.Error(), "bogus-stage") {
		t.Errorf("error %q does not contain offending stage value %q", err.Error(), "bogus-stage")
	}
	if !strings.Contains(err.Error(), "valid") {
		t.Errorf("error %q does not contain the word 'valid'", err.Error())
	}
	// B.4: The error must wrap ErrInvalidStage so callers can use errors.Is.
	if !errors.Is(err, ErrInvalidStage) {
		t.Errorf("errors.Is(err, ErrInvalidStage) = false; got %v", err)
	}
}

