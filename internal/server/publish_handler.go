// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/publish"
	"github.com/specgraph/specgraph/internal/render/adf"
	"github.com/specgraph/specgraph/internal/storage"
)

var _ specgraphv1connect.PublishServiceHandler = (*PublishHandler)(nil)

// PublishHandler implements the ConnectRPC PublishService.
type PublishHandler struct {
	scoper       storage.Scoper
	orchestrator *publish.Orchestrator
	publisher    publish.Publisher
	feedback     publish.FeedbackSource
}

// RegisterPublishService registers the PublishService on the given mux.
func RegisterPublishService(
	mux *http.ServeMux,
	scoper storage.Scoper,
	publisher publish.Publisher,
	feedback publish.FeedbackSource,
	opts ...connect.HandlerOption,
) {
	renderer := adf.NewRenderer()
	orch := publish.NewOrchestrator(renderer, publisher)
	h := &PublishHandler{
		scoper:       scoper,
		orchestrator: orch,
		publisher:    publisher,
		feedback:     feedback,
	}
	path, handler := specgraphv1connect.NewPublishServiceHandler(h, opts...)
	mux.Handle(path, handler)
}

// Publish renders and publishes all available documents for a spec and its linked decisions.
func (h *PublishHandler) Publish(ctx context.Context, req *connect.Request[specv1.PublishRequest]) (*connect.Response[specv1.PublishResponse], error) {
	slug := req.Msg.GetSlug()
	if err := validateSlug(slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}

	// Load spec and convert to proto.
	spec, err := store.GetSpec(ctx, slug)
	if err != nil {
		return nil, publishError(err)
	}
	protoSpec, err := specToProto(spec)
	if err != nil {
		return nil, publishError(err)
	}

	// Fetch linked decisions via DECIDED_IN edges.
	protoDecisions, err := getLinkedDecisions(ctx, store, slug)
	if err != nil {
		return nil, publishError(err)
	}

	// Orchestrate render + publish.
	err = h.orchestrator.PublishAll(ctx, protoSpec, protoDecisions)
	if err != nil {
		return nil, publishError(err)
	}

	// Retrieve page mappings to return.
	mappings, err := store.ListPageMappings(ctx, slug)
	if err != nil {
		return nil, publishError(err)
	}

	return connect.NewResponse(&specv1.PublishResponse{
		Mappings: pageMappingsToProto(mappings),
	}), nil
}

// GetPublishStatus returns the publish status for one or all specs.
func (h *PublishHandler) GetPublishStatus(ctx context.Context, req *connect.Request[specv1.GetPublishStatusRequest]) (*connect.Response[specv1.GetPublishStatusResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}

	slug := req.Msg.GetSlug()
	if slug != "" {
		if vErr := validateSlug(slug); vErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, vErr)
		}
	}

	mappings, err := store.ListPageMappings(ctx, slug)
	if err != nil {
		return nil, publishError(err)
	}

	entries := groupMappingsToEntries(mappings)

	// Attach new-comment counts for each spec.
	for _, entry := range entries {
		count, cErr := store.CountNewFeedback(ctx, entry.GetSpecSlug())
		if cErr != nil {
			slog.ErrorContext(ctx, "publish: count feedback", slog.Any("error", cErr))
			continue
		}
		entry.NewComments = int32(count) //nolint:gosec // count is bounded by DB rows
	}

	return connect.NewResponse(&specv1.GetPublishStatusResponse{
		Entries: entries,
	}), nil
}

// SyncComments polls the configured feedback source and stores entries.
func (h *PublishHandler) SyncComments(ctx context.Context, req *connect.Request[specv1.SyncCommentsRequest]) (*connect.Response[specv1.SyncCommentsResponse], error) {
	if h.feedback == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no feedback source configured"))
	}
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}

	slug := req.Msg.GetSlug()
	if slug != "" {
		if vErr := validateSlug(slug); vErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, vErr)
		}
	}

	// Determine which specs to sync.
	slugs, err := resolveSyncSlugs(ctx, store, slug)
	if err != nil {
		return nil, publishError(err)
	}

	var allFeedback []*specv1.Feedback
	var specCount int32
	for _, s := range slugs {
		comments, pollErr := h.feedback.Poll(ctx, s)
		if pollErr != nil {
			slog.ErrorContext(ctx, "publish: poll feedback",
				slog.String("slug", s), slog.Any("error", pollErr))
			continue
		}
		if len(comments) == 0 {
			continue
		}
		specCount++
		for i := range comments {
			entry := feedbackToStorage(&comments[i], s)
			if _, storeErr := store.StoreFeedback(ctx, entry); storeErr != nil {
				slog.ErrorContext(ctx, "publish: store feedback",
					slog.String("slug", s), slog.Any("error", storeErr))
				continue
			}
			allFeedback = append(allFeedback, feedbackToProto(&comments[i]))
		}
	}

	return connect.NewResponse(&specv1.SyncCommentsResponse{
		Feedback:  allFeedback,
		NewCount:  int32(len(allFeedback)), //nolint:gosec // bounded by feedback entries
		SpecCount: specCount,
	}), nil
}

// Unpublish removes published documents for a spec.
func (h *PublishHandler) Unpublish(ctx context.Context, req *connect.Request[specv1.UnpublishRequest]) (*connect.Response[specv1.UnpublishResponse], error) {
	slug := req.Msg.GetSlug()
	if err := validateSlug(slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if h.publisher == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no publisher configured"))
	}

	if err := h.publisher.Unpublish(ctx, slug); err != nil {
		return nil, publishError(err)
	}

	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	removed, err := store.DeletePageMappings(ctx, slug)
	if err != nil {
		return nil, publishError(err)
	}

	return connect.NewResponse(&specv1.UnpublishResponse{
		PagesRemoved: int32(removed), //nolint:gosec // bounded by number of doc kinds
	}), nil
}

// --- helpers ---

// getLinkedDecisions fetches decisions linked to a spec via DECIDED_IN edges.
func getLinkedDecisions(ctx context.Context, store storage.ScopedBackend, slug string) ([]*specv1.Decision, error) {
	edges, err := store.ListEdges(ctx, slug, storage.EdgeTypeDecidedIn)
	if err != nil {
		return nil, fmt.Errorf("list DECIDED_IN edges: %w", err)
	}
	var decisions []*specv1.Decision
	for _, edge := range edges {
		// DECIDED_IN edges: from=spec, to=decision
		if edge.FromID != slug {
			continue
		}
		d, dErr := store.GetDecision(ctx, edge.ToID)
		if dErr != nil {
			if errors.Is(dErr, storage.ErrDecisionNotFound) {
				slog.WarnContext(ctx, "publish: linked decision not found",
					slog.String("slug", slug), slog.String("decision", edge.ToID))
				continue
			}
			return nil, fmt.Errorf("get decision %s: %w", edge.ToID, dErr)
		}
		pb, cErr := decisionToProto(d)
		if cErr != nil {
			return nil, fmt.Errorf("convert decision %s: %w", edge.ToID, cErr)
		}
		decisions = append(decisions, pb)
	}
	return decisions, nil
}

// resolveSyncSlugs determines which spec slugs to sync feedback for.
// If slug is non-empty, returns a single-element slice. Otherwise, extracts
// distinct slugs from existing page mappings.
func resolveSyncSlugs(ctx context.Context, store storage.ScopedBackend, slug string) ([]string, error) {
	if slug != "" {
		return []string{slug}, nil
	}
	mappings, err := store.ListPageMappings(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list page mappings: %w", err)
	}
	seen := make(map[string]struct{})
	var slugs []string
	for _, m := range mappings {
		if _, ok := seen[m.SpecSlug]; ok {
			continue
		}
		seen[m.SpecSlug] = struct{}{}
		slugs = append(slugs, m.SpecSlug)
	}
	return slugs, nil
}

// --- converters ---

// publishStateToProto maps storage publish state to proto enum.
var publishStateToProtoMap = map[storage.PublishState]specv1.PublishState{
	storage.PublishStateDraft:       specv1.PublishState_PUBLISH_STATE_DRAFT,
	storage.PublishStateSynced:      specv1.PublishState_PUBLISH_STATE_SYNCED,
	storage.PublishStateError:       specv1.PublishState_PUBLISH_STATE_ERROR,
	storage.PublishStateUnpublished: specv1.PublishState_PUBLISH_STATE_UNPUBLISHED,
}

// docKindToProto maps storage document kind to proto enum.
var docKindToProtoMap = map[storage.DocumentKind]specv1.DocumentKind{
	storage.DocumentKindPRD: specv1.DocumentKind_DOCUMENT_KIND_PRD,
	storage.DocumentKindSDD: specv1.DocumentKind_DOCUMENT_KIND_SDD,
	storage.DocumentKindADR: specv1.DocumentKind_DOCUMENT_KIND_ADR,
}

// feedbackKindToProto maps publish feedback kind to proto enum.
var feedbackKindToProtoMap = map[publish.FeedbackKind]specv1.FeedbackKind{
	publish.FeedbackInline: specv1.FeedbackKind_FEEDBACK_KIND_INLINE,
	publish.FeedbackFooter: specv1.FeedbackKind_FEEDBACK_KIND_FOOTER,
}

// pageMappingToProto converts a single storage PageMapping to proto.
func pageMappingToProto(m *storage.PageMapping) *specv1.PageMapping {
	return &specv1.PageMapping{
		SpecSlug:     m.SpecSlug,
		DocKind:      docKindToProtoMap[m.DocKind],
		DecisionSlug: m.DecisionSlug,
		PageId:       m.PageID,
		PageVersion:  int32(m.PageVersion), //nolint:gosec // page version is small
		SpecVersion:  m.SpecVersion,
		State:        publishStateToProtoMap[m.State],
		ErrorMessage: m.ErrorMessage,
		LastSync:     timeToProto(m.LastSync),
	}
}

// pageMappingsToProto converts a slice of storage PageMappings to proto.
func pageMappingsToProto(mappings []*storage.PageMapping) []*specv1.PageMapping {
	if len(mappings) == 0 {
		return nil
	}
	result := make([]*specv1.PageMapping, len(mappings))
	for i, m := range mappings {
		result[i] = pageMappingToProto(m)
	}
	return result
}

// groupMappingsToEntries groups page mappings by spec slug into status entries.
func groupMappingsToEntries(mappings []*storage.PageMapping) []*specv1.PublishStatusEntry {
	if len(mappings) == 0 {
		return nil
	}
	// Collect by spec slug, preserving insertion order.
	type entryData struct {
		entry *specv1.PublishStatusEntry
		order int
	}
	bySlug := make(map[string]*entryData)
	var idx int
	for _, m := range mappings {
		d, ok := bySlug[m.SpecSlug]
		if !ok {
			d = &entryData{
				entry: &specv1.PublishStatusEntry{SpecSlug: m.SpecSlug},
				order: idx,
			}
			bySlug[m.SpecSlug] = d
			idx++
		}
		pm := pageMappingToProto(m)
		switch m.DocKind {
		case storage.DocumentKindPRD:
			d.entry.Prd = pm
		case storage.DocumentKindSDD:
			d.entry.Sdd = pm
		case storage.DocumentKindADR:
			d.entry.Adrs = append(d.entry.Adrs, pm)
		}
		// Track the latest sync across all mappings.
		if ts := timeToProto(m.LastSync); ts != nil {
			if d.entry.LastSync == nil || m.LastSync.After(d.entry.LastSync.AsTime()) {
				d.entry.LastSync = ts
			}
		}
	}

	// Build result in insertion order.
	result := make([]*specv1.PublishStatusEntry, len(bySlug))
	for _, d := range bySlug {
		result[d.order] = d.entry
	}
	return result
}

// feedbackToProto converts a publish.Feedback to proto.
func feedbackToProto(f *publish.Feedback) *specv1.Feedback {
	return &specv1.Feedback{
		ExternalId: f.ExternalID,
		Author:     f.Author,
		Body:       f.Body,
		Timestamp:  timeToProto(f.Timestamp),
		Kind:       feedbackKindToProtoMap[f.Kind],
		Stage:      f.Stage,
		IsQuestion: f.IsQuestion,
		ParentId:   f.ParentID,
		SpecSlug:   f.SpecSlug,
	}
}

// feedbackToStorage converts a publish.Feedback to a storage FeedbackEntry.
func feedbackToStorage(f *publish.Feedback, specSlug string) *storage.FeedbackEntry {
	var kind storage.FeedbackKind
	switch f.Kind {
	case publish.FeedbackInline:
		kind = storage.FeedbackKindInline
	case publish.FeedbackFooter:
		kind = storage.FeedbackKindFooter
	}
	return &storage.FeedbackEntry{
		ExternalID: f.ExternalID,
		SpecSlug:   specSlug,
		Author:     f.Author,
		Body:       f.Body,
		Timestamp:  f.Timestamp,
		Kind:       kind,
		Stage:      f.Stage,
		IsQuestion: f.IsQuestion,
		ParentID:   f.ParentID,
	}
}

// publishError maps errors to sanitized connect error codes.
func publishError(err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	switch {
	case errors.Is(err, storage.ErrSpecNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("spec not found"))
	case errors.Is(err, storage.ErrDecisionNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("decision not found"))
	default:
		slog.Error("publishError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
