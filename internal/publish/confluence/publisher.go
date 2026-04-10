// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package confluence

import (
	"context"
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/publish"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/specgraph/specgraph/internal/storage"
)

// PublishStore is the subset of storage.PublishBackend the publisher needs.
type PublishStore interface {
	UpsertPageMapping(ctx context.Context, m *storage.PageMapping) (*storage.PageMapping, error)
	GetPageMapping(ctx context.Context, specSlug string, kind storage.DocumentKind, decisionSlug string) (*storage.PageMapping, error)
	ListPageMappings(ctx context.Context, specSlug string) ([]*storage.PageMapping, error)
	DeletePageMappings(ctx context.Context, specSlug string) (int, error)
}

// Publisher implements publish.Publisher for Confluence.
type Publisher struct {
	client *Client
	store  PublishStore
	cfg    Config
}

// NewPublisher creates a Confluence publisher.
func NewPublisher(client *Client, store PublishStore, cfg Config) *Publisher {
	return &Publisher{client: client, store: store, cfg: cfg}
}

func (p *Publisher) Name() string { return "confluence" }

func (p *Publisher) Publish(ctx context.Context, slug string, docs []render.Document) (publish.PublishResult, error) {
	var result publish.PublishResult
	for _, doc := range docs {
		parentID := p.cfg.ParentPageID
		// SDDs and ADRs are children of the PRD page
		if doc.Kind != render.DocumentPRD {
			prdMapping, err := p.store.GetPageMapping(ctx, slug, storage.DocumentKindPRD, "")
			if err != nil {
				return result, fmt.Errorf("get PRD mapping: %w", err)
			}
			if prdMapping != nil {
				parentID = prdMapping.PageID
			}
		}

		page, err := p.client.CreatePage(ctx, doc.Title, parentID, doc.Body)
		if err != nil {
			return result, fmt.Errorf("create page %q: %w", doc.Title, err)
		}

		mapping := &storage.PageMapping{
			SpecSlug:     slug,
			DocKind:      storageDocKind(doc.Kind),
			DecisionSlug: doc.DecisionID,
			PageID:       page.ID,
			PageVersion:  page.Version,
			State:        storage.PublishStateSynced,
		}
		if _, err := p.store.UpsertPageMapping(ctx, mapping); err != nil {
			return result, fmt.Errorf("store mapping: %w", err)
		}

		result.Mappings = append(result.Mappings, publish.PageRef{
			DocKind:    doc.Kind,
			DecisionID: doc.DecisionID,
			PageID:     page.ID,
			Version:    page.Version,
			URL:        page.WebURL,
		})
	}
	return result, nil
}

func (p *Publisher) Update(ctx context.Context, slug string, docs []render.Document, _ *specv1.ChangeLogEntry) (publish.PublishResult, error) {
	var result publish.PublishResult
	for _, doc := range docs {
		existing, err := p.store.GetPageMapping(ctx, slug, storageDocKind(doc.Kind), doc.DecisionID)
		if err != nil {
			return result, fmt.Errorf("get mapping: %w", err)
		}
		if existing == nil {
			// Not yet published — create instead
			return p.Publish(ctx, slug, docs)
		}

		page, err := p.client.UpdatePage(ctx, existing.PageID, doc.Title, existing.PageVersion, doc.Body)
		if err != nil {
			return result, fmt.Errorf("update page %q: %w", doc.Title, err)
		}

		existing.PageVersion = page.Version
		existing.State = storage.PublishStateSynced
		if _, err := p.store.UpsertPageMapping(ctx, existing); err != nil {
			return result, fmt.Errorf("store mapping: %w", err)
		}

		result.Mappings = append(result.Mappings, publish.PageRef{
			DocKind: doc.Kind,
			PageID:  page.ID,
			Version: page.Version,
		})
	}
	return result, nil
}

func (p *Publisher) Unpublish(ctx context.Context, slug string) error {
	mappings, err := p.store.ListPageMappings(ctx, slug)
	if err != nil {
		return fmt.Errorf("list mappings: %w", err)
	}
	// Delete child pages first, then parent
	for i := len(mappings) - 1; i >= 0; i-- {
		if err := p.client.DeletePage(ctx, mappings[i].PageID); err != nil {
			return fmt.Errorf("delete page %s: %w", mappings[i].PageID, err)
		}
	}
	if _, err := p.store.DeletePageMappings(ctx, slug); err != nil {
		return fmt.Errorf("delete mappings: %w", err)
	}
	return nil
}

func (p *Publisher) Status(ctx context.Context, slug string) (publish.PublishStatus, error) {
	mappings, err := p.store.ListPageMappings(ctx, slug)
	if err != nil {
		return publish.PublishStatus{}, err
	}
	status := publish.PublishStatus{SpecSlug: slug}
	for _, m := range mappings {
		ps := &publish.PageState{
			PageID:      m.PageID,
			State:       string(m.State),
			SpecVersion: m.SpecVersion,
			LastSync:    m.LastSync,
		}
		switch m.DocKind {
		case storage.DocumentKindPRD:
			status.PRD = ps
		case storage.DocumentKindSDD:
			status.SDD = ps
		case storage.DocumentKindADR:
			status.ADRs = append(status.ADRs, *ps)
		}
		if m.LastSync.After(status.LastSync) {
			status.LastSync = m.LastSync
		}
	}
	return status, nil
}

func storageDocKind(k render.DocumentKind) storage.DocumentKind {
	switch k {
	case render.DocumentPRD:
		return storage.DocumentKindPRD
	case render.DocumentSDD:
		return storage.DocumentKindSDD
	case render.DocumentADR:
		return storage.DocumentKindADR
	default:
		return storage.DocumentKindPRD
	}
}
