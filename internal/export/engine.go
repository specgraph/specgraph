// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package export

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// Backend is the subset of storage needed for export/import operations.
type Backend interface {
	storage.Backend
	storage.GraphBackend
	storage.DecisionBackend
	storage.ConstitutionBackend
	storage.FindingsBackend
	storage.ChangeLogBackend
	storage.ConversationBackend
	storage.ExecutionBackend
	storage.SyncBackend
	storage.SliceBackend
	storage.ProjectBackend
}

// Engine performs project export and import operations.
type Engine struct {
	backend    Backend
	signingKey string
	version    string
}

// NewEngine creates an Engine that reads from backend and signs with the given key.
// An empty signingKey disables signature generation.
func NewEngine(backend Backend, signingKey, version string) *Engine {
	return &Engine{backend: backend, signingKey: signingKey, version: version}
}

// Export collects all project entities and returns a signed JSON document.
func (e *Engine) Export(ctx context.Context, projectSlug string) ([]byte, error) {
	doc, err := e.collect(ctx, projectSlug)
	if err != nil {
		return nil, fmt.Errorf("export collect: %w", err)
	}

	dataBytes, err := json.Marshal(doc.Data)
	if err != nil {
		return nil, fmt.Errorf("export marshal data: %w", err)
	}

	if e.signingKey != "" {
		mac := hmac.New(sha256.New, []byte(e.signingKey))
		mac.Write(dataBytes)
		doc.Signature = &Signature{
			Algorithm: "hmac-sha256",
			Digest:    hex.EncodeToString(mac.Sum(nil)),
		}
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("export marshal: %w", err)
	}
	return out, nil
}

// collect reads all entities from the backend and assembles a Document.
func (e *Engine) collect(ctx context.Context, projectSlug string) (*Document, error) {
	doc := &Document{
		SchemaVersion:    CurrentSchemaVersion,
		ExportedAt:       time.Now().UTC(),
		SpecGraphVersion: e.version,
		ProjectSlug:      projectSlug,
	}

	// Project
	proj, err := e.backend.GetProject(ctx, projectSlug)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	doc.Data.Project = proj

	// Constitution (optional — may not exist)
	constitution, err := e.backend.GetConstitution(ctx)
	if err == nil {
		doc.Data.Constitution = constitution
	}

	// Specs — list summaries, then fetch full data for each
	specSummaries, err := e.backend.ListSpecs(ctx, "", "", 0)
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}
	specs := make([]*storage.Spec, 0, len(specSummaries))
	for _, s := range specSummaries {
		full, getErr := e.backend.GetSpec(ctx, s.Slug)
		if getErr != nil {
			return nil, fmt.Errorf("get spec %q: %w", s.Slug, getErr)
		}
		specs = append(specs, full)
	}
	doc.Data.Specs = specs

	// Decisions
	decisions, err := e.backend.ListDecisions(ctx, "", 0)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}
	doc.Data.Decisions = decisions

	// Slices — per spec
	var allSlices []*storage.Slice
	for _, s := range specs {
		slices, sliceErr := e.backend.ListSlices(ctx, s.Slug)
		if sliceErr != nil {
			return nil, fmt.Errorf("list slices for %q: %w", s.Slug, sliceErr)
		}
		allSlices = append(allSlices, slices...)
	}
	doc.Data.Slices = allSlices

	// Edges — from full graph
	fg, err := e.backend.GetFullGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("get full graph: %w", err)
	}
	edges := make([]Edge, 0, len(fg.Edges))
	for _, ge := range fg.Edges {
		edges = append(edges, Edge{
			FromSlug:          ge.FromID,
			ToSlug:            ge.ToID,
			Type:              string(ge.EdgeType),
			ContentHashAtLink: ge.ContentHashAtLink,
		})
	}
	doc.Data.Edges = edges

	// Findings
	findings, err := e.backend.ListAllFindings(ctx)
	if err != nil {
		return nil, fmt.Errorf("list findings: %w", err)
	}
	doc.Data.Findings = findings

	// ChangeLogs
	changeLogs, err := e.backend.ListAllChanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("list changes: %w", err)
	}
	doc.Data.ChangeLogs = changeLogs

	// Conversations
	conversations, err := e.backend.ListAllConversations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	doc.Data.Conversations = conversations

	// SyncMappings
	mappings, err := e.backend.ListSyncMappings(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("list sync mappings: %w", err)
	}
	doc.Data.SyncMappings = mappings

	// ExecutionEvents — per spec
	var allEvents []*storage.ExecutionEvent
	for _, s := range specs {
		events, evErr := e.backend.GetExecutionEvents(ctx, s.Slug, 0)
		if evErr != nil {
			return nil, fmt.Errorf("get execution events for %q: %w", s.Slug, evErr)
		}
		allEvents = append(allEvents, events...)
	}
	doc.Data.ExecutionEvents = allEvents

	return doc, nil
}
