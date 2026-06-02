// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"fmt"
	"sync/atomic"

	cedar "github.com/cedar-policy/cedar-go"
)

// PolicyEngine evaluates an authorization request against a loaded policy
// set. Cedar is wrapped behind this interface so the rest of the codebase
// never imports cedar-go directly. The only implementation is cedarEngine
// (this file); the only file importing cedar-go is engine.go.
type PolicyEngine interface {
	// Evaluate decides the request. Returns an error only for operational
	// failures (the engine could not reach a decision); a clean Deny is a
	// successful evaluation with PolicyDecision.Allowed == false.
	Evaluate(ctx context.Context, req EvalRequest) (PolicyDecision, error)
	// Reload re-reads every PolicySource and atomically swaps the active
	// policy set. Not wired to any signal in v1 (restart applies policy
	// changes); present for the future reload story and exercised by tests.
	Reload(ctx context.Context) error
}

// EvalRequest is a SpecGraph-shaped authorization question, mapped to a
// cedar.Request inside the engine.
type EvalRequest struct {
	// Identity is the resolved principal. Its EffectiveRole becomes the
	// Cedar principal's "role" attribute.
	Identity *Identity
	// Action is the stable action name (e.g. "spec.read"), already mapped
	// from the RPC procedure by the caller. Decoupled from method names.
	Action string
	// Resource describes what is being acted on. For the migration this is
	// a placeholder derived from the action's domain; stories #3/#4 populate
	// real resource attributes here.
	Resource ResourceRef
	// Context carries transient request attributes (e.g. project slug).
	// Empty for the migration; the Cedar context Record is built from it.
	Context map[string]string
}

// ResourceRef projects a resource into Cedar's (type, id, attributes) shape.
type ResourceRef struct {
	Type       string            // Cedar resource entity id namespace, e.g. "spec"
	ID         string            // resource id; "" → "unspecified"
	Attributes map[string]string // e.g. {"owner_user_id": "..."}; empty in migration
}

// PolicyDecision is the engine's full result: the allow/deny plus the
// policy IDs that drove it (for decision logs / future audit) and any
// evaluation errors Cedar surfaced.
type PolicyDecision struct {
	Allowed         bool
	MatchedPolicies []string // cedar policy IDs from Diagnostic.Reasons
	Errors          []string // stringified Diagnostic.Errors (rare; bad policy/attr)
}

// cedarEngine is the sole PolicyEngine implementation and the sole file
// importing cedar-go. Policies live in an atomic.Pointer for a lock-free
// eval hot path; Reload builds a fresh set and swaps it.
type cedarEngine struct {
	sources        []PolicySource
	actionEntities cedar.EntityMap // precomputed action + verb-group entities
	policies       atomic.Pointer[cedar.PolicySet]
}

// NewCedarEngine loads every source into one merged policy set and
// precomputes the action-group entity graph from actionNames. The built-in
// source MUST contribute at least one parseable policy: a zero-policy result
// is a build error (the binary shipped without its base policies) and
// construction fails.
func NewCedarEngine(ctx context.Context, sources []PolicySource, actionNames []string) (PolicyEngine, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("cedar: NewCedarEngine: at least one PolicySource required")
	}
	actionEntities, err := buildActionEntities(actionNames)
	if err != nil {
		return nil, fmt.Errorf("cedar: build action entities: %w", err)
	}
	eng := &cedarEngine{sources: sources, actionEntities: actionEntities}
	if err := eng.Reload(ctx); err != nil {
		return nil, err
	}
	return eng, nil
}

// Reload re-reads all sources, parses + merges them, and atomically swaps
// the active policy set. Safe to call concurrently with Evaluate.
func (e *cedarEngine) Reload(ctx context.Context) error {
	set, err := loadPolicySet(ctx, e.sources)
	if err != nil {
		return err
	}
	count := 0
	for range set.All() {
		count++
	}
	if count == 0 {
		return fmt.Errorf("cedar: no policies loaded from %d source(s); refusing to start", len(e.sources))
	}
	e.policies.Store(set)
	return nil
}

// loadPolicySet parses each source's documents and merges them into one
// PolicySet. Policy IDs are prefixed with the document source so decision
// logs can name the origin and IDs never collide across documents.
func loadPolicySet(ctx context.Context, sources []PolicySource) (*cedar.PolicySet, error) {
	combined := cedar.NewPolicySet()
	for _, src := range sources {
		docs, err := src.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("cedar: load source %s: %w", src.Name(), err)
		}
		for _, doc := range docs {
			ps, parseErr := cedar.NewPolicySetFromBytes(doc.Source, []byte(doc.Text))
			if parseErr != nil {
				return nil, fmt.Errorf("cedar: parse %s: %w", doc.Source, parseErr)
			}
			for id, p := range ps.All() {
				// Prefix the per-document policy id with the source so ids are
				// globally unique and a decision log names the origin.
				// PolicySet.Add upserts: it returns false when the id already
				// exists and silently REPLACES the prior policy. Treat any
				// collision as a programming error rather than letting a policy
				// be overwritten.
				mergedID := cedar.PolicyID(doc.Source + "#" + string(id))
				if !combined.Add(mergedID, p) {
					return nil, fmt.Errorf("cedar: duplicate policy id %q while merging %s", mergedID, doc.Source)
				}
			}
		}
	}
	return combined, nil
}

// Temporary stubs — real impls land in Tasks 7 and 9.
func buildActionEntities(_ []string) (cedar.EntityMap, error) { return cedar.EntityMap{}, nil }
func (e *cedarEngine) Evaluate(_ context.Context, _ EvalRequest) (PolicyDecision, error) {
	return PolicyDecision{}, fmt.Errorf("Evaluate not implemented")
}
