// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package publish

import (
	"context"
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// Orchestrator coordinates rendering and publishing on spec events.
type Orchestrator struct {
	renderer  render.Renderer
	publisher Publisher
}

// NewOrchestrator creates a publish orchestrator.
func NewOrchestrator(r render.Renderer, p Publisher) *Orchestrator {
	return &Orchestrator{renderer: r, publisher: p}
}

// OnStageComplete is called when a spec completes an authoring stage.
func (o *Orchestrator) OnStageComplete(ctx context.Context, spec *specv1.Spec, stage string) error {
	if spec == nil {
		return fmt.Errorf("spec is nil")
	}
	switch stage {
	case "shape":
		doc, err := o.renderer.RenderPRD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render PRD: %w", err)
		}
		if _, err = o.publisher.Publish(ctx, spec.Slug, []render.Document{doc}); err != nil {
			return fmt.Errorf("publish: %w", err)
		}
		return nil
	case "specify":
		doc, err := o.renderer.RenderSDD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render SDD: %w", err)
		}
		if _, err = o.publisher.Publish(ctx, spec.Slug, []render.Document{doc}); err != nil {
			return fmt.Errorf("publish: %w", err)
		}
		return nil
	default:
		return nil
	}
}

// OnDecisionLinked is called when a decision is linked to a spec.
func (o *Orchestrator) OnDecisionLinked(ctx context.Context, specSlug string, decision *specv1.Decision) error {
	if decision == nil {
		return fmt.Errorf("decision is nil")
	}
	doc, err := o.renderer.RenderADR(ctx, decision)
	if err != nil {
		return fmt.Errorf("render ADR: %w", err)
	}
	if _, err = o.publisher.Publish(ctx, specSlug, []render.Document{doc}); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}

// OnSpecUpdated is called when a spec is updated (new version).
func (o *Orchestrator) OnSpecUpdated(ctx context.Context, spec *specv1.Spec, changelog *specv1.ChangeLogEntry) error {
	if spec == nil {
		return fmt.Errorf("spec is nil")
	}
	var docs []render.Document
	if spec.ShapeOutput != nil {
		doc, err := o.renderer.RenderPRD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render PRD: %w", err)
		}
		docs = append(docs, doc)
	}
	if spec.SpecifyOutput != nil {
		doc, err := o.renderer.RenderSDD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render SDD: %w", err)
		}
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		return nil
	}
	if _, err := o.publisher.Update(ctx, spec.Slug, docs, changelog); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

// PublishAll renders and publishes all available documents for a spec.
func (o *Orchestrator) PublishAll(ctx context.Context, spec *specv1.Spec, decisions []*specv1.Decision) error {
	if spec == nil {
		return fmt.Errorf("spec is nil")
	}
	var docs []render.Document
	if spec.ShapeOutput != nil {
		doc, err := o.renderer.RenderPRD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render PRD: %w", err)
		}
		docs = append(docs, doc)
	}
	if spec.SpecifyOutput != nil {
		doc, err := o.renderer.RenderSDD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render SDD: %w", err)
		}
		docs = append(docs, doc)
	}
	if len(docs) > 0 {
		if _, err := o.publisher.Publish(ctx, spec.Slug, docs); err != nil {
			return fmt.Errorf("publish: %w", err)
		}
	}
	for _, d := range decisions {
		doc, err := o.renderer.RenderADR(ctx, d)
		if err != nil {
			return fmt.Errorf("render ADR %s: %w", d.Slug, err)
		}
		if _, err := o.publisher.Publish(ctx, spec.Slug, []render.Document{doc}); err != nil {
			return fmt.Errorf("publish ADR %s: %w", d.Slug, err)
		}
	}
	return nil
}
