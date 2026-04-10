// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package publish

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

type fakeRenderer struct{}

func (f *fakeRenderer) RenderPRD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{Kind: render.DocumentPRD, Title: "PRD", Body: []byte("{}")}, nil
}
func (f *fakeRenderer) RenderSDD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{Kind: render.DocumentSDD, Title: "SDD", Body: []byte("{}")}, nil
}
func (f *fakeRenderer) RenderADR(_ context.Context, _ *specv1.Decision) (render.Document, error) {
	return render.Document{Kind: render.DocumentADR, Title: "ADR", Body: []byte("{}")}, nil
}

type fakePublisher struct {
	published int
}

func (f *fakePublisher) Name() string { return "fake" }
func (f *fakePublisher) Publish(_ context.Context, _ string, docs []render.Document) (PublishResult, error) {
	f.published += len(docs)
	return PublishResult{}, nil
}
func (f *fakePublisher) Update(_ context.Context, _ string, docs []render.Document, _ *specv1.ChangeLogEntry) (PublishResult, error) {
	f.published += len(docs)
	return PublishResult{}, nil
}
func (f *fakePublisher) Unpublish(_ context.Context, _ string) error { return nil }
func (f *fakePublisher) Status(_ context.Context, _ string) (PublishStatus, error) {
	return PublishStatus{}, nil
}

func TestOrchestratorOnShapeComplete(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{Slug: "test", Stage: "shape"}
	err := orch.OnStageComplete(context.Background(), spec, "shape")
	if err != nil {
		t.Fatalf("OnStageComplete: %v", err)
	}
	if pub.published != 1 {
		t.Errorf("published = %d, want 1 (PRD)", pub.published)
	}
}

func TestOrchestratorOnSpecifyComplete(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{Slug: "test", Stage: "specify"}
	err := orch.OnStageComplete(context.Background(), spec, "specify")
	if err != nil {
		t.Fatalf("OnStageComplete: %v", err)
	}
	if pub.published != 1 {
		t.Errorf("published = %d, want 1 (SDD)", pub.published)
	}
}

func TestOrchestratorPublishAll(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{
		Slug:          "test",
		ShapeOutput:   &specv1.ShapeOutput{},
		SpecifyOutput: &specv1.SpecifyOutput{},
	}
	decisions := []*specv1.Decision{
		{Slug: "dec-1", Title: "Decision 1"},
	}
	err := orch.PublishAll(context.Background(), spec, decisions)
	if err != nil {
		t.Fatalf("PublishAll: %v", err)
	}
	// PRD + SDD + 1 ADR = 3
	if pub.published != 3 {
		t.Errorf("published = %d, want 3", pub.published)
	}
}

func TestOrchestratorUnknownStage(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{Slug: "test"}
	err := orch.OnStageComplete(context.Background(), spec, "spark")
	if err != nil {
		t.Fatalf("OnStageComplete(spark): %v", err)
	}
	if pub.published != 0 {
		t.Errorf("published = %d, want 0 (spark is not publishable)", pub.published)
	}
}
