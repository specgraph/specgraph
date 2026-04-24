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
