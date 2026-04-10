// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package publish

import (
	"context"
	"fmt"
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
func (f *fakePublisher) Publish(_ context.Context, _ string, docs []render.Document) (Result, error) {
	f.published += len(docs)
	return Result{}, nil
}
func (f *fakePublisher) Update(_ context.Context, _ string, docs []render.Document, _ *specv1.ChangeLogEntry) (Result, error) {
	f.published += len(docs)
	return Result{}, nil
}
func (f *fakePublisher) Unpublish(_ context.Context, _ string) error { return nil }
func (f *fakePublisher) Status(_ context.Context, _ string) (Status, error) {
	return Status{}, nil
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

func TestOrchestratorOnDecisionLinked(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	decision := &specv1.Decision{Slug: "dec-1", Title: "Decision 1"}
	err := orch.OnDecisionLinked(context.Background(), "test-spec", decision)
	if err != nil {
		t.Fatalf("OnDecisionLinked: %v", err)
	}
	if pub.published != 1 {
		t.Errorf("published = %d, want 1 (ADR)", pub.published)
	}
}

func TestOrchestratorOnSpecUpdatedBothOutputs(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{
		Slug:          "test",
		ShapeOutput:   &specv1.ShapeOutput{},
		SpecifyOutput: &specv1.SpecifyOutput{},
	}
	changelog := &specv1.ChangeLogEntry{}
	err := orch.OnSpecUpdated(context.Background(), spec, changelog)
	if err != nil {
		t.Fatalf("OnSpecUpdated: %v", err)
	}
	// PRD + SDD = 2 docs via Update
	if pub.published != 2 {
		t.Errorf("published = %d, want 2 (PRD + SDD)", pub.published)
	}
}

func TestOrchestratorOnSpecUpdatedShapeOnly(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{
		Slug:        "test",
		ShapeOutput: &specv1.ShapeOutput{},
	}
	changelog := &specv1.ChangeLogEntry{}
	err := orch.OnSpecUpdated(context.Background(), spec, changelog)
	if err != nil {
		t.Fatalf("OnSpecUpdated(shape only): %v", err)
	}
	// PRD only = 1 doc via Update
	if pub.published != 1 {
		t.Errorf("published = %d, want 1 (PRD only)", pub.published)
	}
}

func TestOrchestratorOnSpecUpdatedNoOutputs(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	// No ShapeOutput or SpecifyOutput — should be a no-op
	spec := &specv1.Spec{Slug: "test"}
	changelog := &specv1.ChangeLogEntry{}
	err := orch.OnSpecUpdated(context.Background(), spec, changelog)
	if err != nil {
		t.Fatalf("OnSpecUpdated(no outputs): %v", err)
	}
	if pub.published != 0 {
		t.Errorf("published = %d, want 0 (no outputs, nothing to update)", pub.published)
	}
}

func TestOrchestratorPublishAllEmpty(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	// No outputs, no decisions — empty publish
	spec := &specv1.Spec{Slug: "test"}
	err := orch.PublishAll(context.Background(), spec, nil)
	if err != nil {
		t.Fatalf("PublishAll(empty): %v", err)
	}
	if pub.published != 0 {
		t.Errorf("published = %d, want 0 (no outputs, no decisions)", pub.published)
	}
}

func TestOrchestratorOnStageCompleteNilSpec(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	err := orch.OnStageComplete(context.Background(), nil, "shape")
	if err == nil {
		t.Fatal("OnStageComplete(nil spec): expected error, got nil")
	}
}

func TestOrchestratorOnDecisionLinkedNilDecision(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	err := orch.OnDecisionLinked(context.Background(), "test-spec", nil)
	if err == nil {
		t.Fatal("OnDecisionLinked(nil decision): expected error, got nil")
	}
}

func TestOrchestratorOnSpecUpdatedNilSpec(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	err := orch.OnSpecUpdated(context.Background(), nil, &specv1.ChangeLogEntry{})
	if err == nil {
		t.Fatal("OnSpecUpdated(nil spec): expected error, got nil")
	}
}

func TestOrchestratorPublishAllNilSpec(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	err := orch.PublishAll(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("PublishAll(nil spec): expected error, got nil")
	}
}

func TestOrchestratorPublishAllRenderADRError(t *testing.T) {
	pub := &fakePublisher{}
	// Use an error renderer that always fails on RenderADR.
	orch := NewOrchestrator(&alwaysErrorADRRenderer{}, pub)
	spec := &specv1.Spec{
		Slug: "test",
	}
	// Pass a valid decision; the renderer will return an error.
	err := orch.PublishAll(context.Background(), spec, []*specv1.Decision{
		{Slug: "dec-1", Title: "Some Decision"},
	})
	if err == nil {
		t.Fatal("PublishAll(render ADR error): expected error, got nil")
	}
}

func TestOrchestratorOnSpecUpdatedUpdateError(t *testing.T) {
	pub := &errorPublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{
		Slug:          "test",
		ShapeOutput:   &specv1.ShapeOutput{},
		SpecifyOutput: &specv1.SpecifyOutput{},
	}
	err := orch.OnSpecUpdated(context.Background(), spec, &specv1.ChangeLogEntry{})
	if err == nil {
		t.Fatal("OnSpecUpdated with errorPublisher: expected error, got nil")
	}
}

func TestOrchestratorOnDecisionLinkedSuccess(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	decision := &specv1.Decision{Slug: "adr-use-grpc", Title: "Use ConnectRPC"}
	err := orch.OnDecisionLinked(context.Background(), "test-spec", decision)
	if err != nil {
		t.Fatalf("OnDecisionLinked success: %v", err)
	}
	if pub.published != 1 {
		t.Errorf("published = %d, want 1 (ADR doc)", pub.published)
	}
}

func TestOrchestratorOnStageCompleteRenderError(t *testing.T) {
	// errorRenderer always returns an error from RenderPRD.
	pub := &fakePublisher{}
	orch := NewOrchestrator(&errorRenderer{}, pub)
	spec := &specv1.Spec{Slug: "test", Stage: "shape"}
	err := orch.OnStageComplete(context.Background(), spec, "shape")
	if err == nil {
		t.Fatal("OnStageComplete(shape): expected error from RenderPRD, got nil")
	}
}

func TestOrchestratorOnStageCompletePublishError(t *testing.T) {
	// fakeRenderer succeeds but publishErrorPublisher.Publish fails.
	pub := &publishErrorPublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{Slug: "test", Stage: "shape"}
	err := orch.OnStageComplete(context.Background(), spec, "shape")
	if err == nil {
		t.Fatal("OnStageComplete(shape): expected error from Publish, got nil")
	}
}

func TestOrchestratorOnSpecifyCompleteRenderError(t *testing.T) {
	// errorRenderer always returns an error from RenderSDD.
	pub := &fakePublisher{}
	orch := NewOrchestrator(&errorSDDRenderer{}, pub)
	spec := &specv1.Spec{Slug: "test", Stage: "specify"}
	err := orch.OnStageComplete(context.Background(), spec, "specify")
	if err == nil {
		t.Fatal("OnStageComplete(specify): expected error from RenderSDD, got nil")
	}
}

// alwaysErrorADRRenderer always returns an error from RenderADR.
type alwaysErrorADRRenderer struct{}

func (f *alwaysErrorADRRenderer) RenderPRD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{Kind: render.DocumentPRD, Title: "PRD", Body: []byte("{}")}, nil
}
func (f *alwaysErrorADRRenderer) RenderSDD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{Kind: render.DocumentSDD, Title: "SDD", Body: []byte("{}")}, nil
}
func (f *alwaysErrorADRRenderer) RenderADR(_ context.Context, _ *specv1.Decision) (render.Document, error) {
	return render.Document{}, fmt.Errorf("render ADR failed")
}

// errorPublisher always returns an error from Update.
type errorPublisher struct{}

func (f *errorPublisher) Name() string { return "error" }
func (f *errorPublisher) Publish(_ context.Context, _ string, _ []render.Document) (Result, error) {
	return Result{}, nil
}
func (f *errorPublisher) Update(_ context.Context, _ string, _ []render.Document, _ *specv1.ChangeLogEntry) (Result, error) {
	return Result{}, fmt.Errorf("update failed")
}
func (f *errorPublisher) Unpublish(_ context.Context, _ string) error { return nil }
func (f *errorPublisher) Status(_ context.Context, _ string) (Status, error) {
	return Status{}, nil
}

// publishErrorPublisher always returns an error from Publish (not Update).
type publishErrorPublisher struct{}

func (f *publishErrorPublisher) Name() string { return "publish-error" }
func (f *publishErrorPublisher) Publish(_ context.Context, _ string, _ []render.Document) (Result, error) {
	return Result{}, fmt.Errorf("publish failed")
}
func (f *publishErrorPublisher) Update(_ context.Context, _ string, _ []render.Document, _ *specv1.ChangeLogEntry) (Result, error) {
	return Result{}, nil
}
func (f *publishErrorPublisher) Unpublish(_ context.Context, _ string) error { return nil }
func (f *publishErrorPublisher) Status(_ context.Context, _ string) (Status, error) {
	return Status{}, nil
}

// errorRenderer always returns an error from RenderPRD.
type errorRenderer struct{}

func (f *errorRenderer) RenderPRD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{}, fmt.Errorf("render PRD failed")
}
func (f *errorRenderer) RenderSDD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{Kind: render.DocumentSDD, Title: "SDD", Body: []byte("{}")}, nil
}
func (f *errorRenderer) RenderADR(_ context.Context, _ *specv1.Decision) (render.Document, error) {
	return render.Document{Kind: render.DocumentADR, Title: "ADR", Body: []byte("{}")}, nil
}

// errorSDDRenderer always returns an error from RenderSDD.
type errorSDDRenderer struct{}

func (f *errorSDDRenderer) RenderPRD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{Kind: render.DocumentPRD, Title: "PRD", Body: []byte("{}")}, nil
}
func (f *errorSDDRenderer) RenderSDD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{}, fmt.Errorf("render SDD failed")
}
func (f *errorSDDRenderer) RenderADR(_ context.Context, _ *specv1.Decision) (render.Document, error) {
	return render.Document{Kind: render.DocumentADR, Title: "ADR", Body: []byte("{}")}, nil
}
