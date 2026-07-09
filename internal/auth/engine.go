// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

// Cedar entity-type namespaces. Defined here (first use) and reused by the
// principal/resource helpers in Task 8 and Evaluate in Task 9.
const (
	entityTypeUser     = "SpecGraph::User"
	entityTypeResource = "SpecGraph::Resource"
	entityTypeAction   = "SpecGraph::Action"
)

// knownVerbs are the action suffixes that map to verb groups. The base
// policies gate roles per verb; an action whose suffix is not here cannot be
// authorized and is a programming error (caught at engine construction).
//
// The "self" verb authorizes an authenticated principal acting on their own
// resources (apikey.self). base.cedar permits it for any authenticated role;
// the handler further restricts it (rejects Source == "apikey", floors the
// minted role via RoleMin) because principalEntity exposes only role/id/email.
// This entry MUST land in the same commit as the base.cedar permit and the
// apikey.self action map entries (actions.go) or NewCedarEngine fails at boot.
var knownVerbs = map[string]bool{"read": true, "write": true, "delete": true, "manage": true, "self": true}

// actionDomain returns the domain prefix of an action name
// ("spec.read" -> "spec"). Used to derive the placeholder resource id.
func actionDomain(action string) string {
	if idx := strings.Index(action, "."); idx >= 0 {
		return action[:idx]
	}
	return action
}

// actionVerb returns the verb suffix of an action name ("spec.read" -> "read").
func actionVerb(action string) (string, error) {
	idx := strings.LastIndex(action, ".")
	if idx < 0 || idx == len(action)-1 {
		return "", fmt.Errorf("action %q has no verb suffix", action)
	}
	verb := action[idx+1:]
	if !knownVerbs[verb] {
		return "", fmt.Errorf("action %q has unknown verb %q", action, verb)
	}
	return verb, nil
}

// buildActionEntities turns action names into the cedar entity graph: each
// verb group (SpecGraph::Action::"read") and each concrete action
// (SpecGraph::Action::"spec.read") parented to its group. cedar resolves
// "action in <group>" through these Parents at Authorize time.
func buildActionEntities(actionNames []string) (cedar.EntityMap, error) {
	ents := cedar.EntityMap{}
	for _, name := range actionNames {
		verb, err := actionVerb(name)
		if err != nil {
			return nil, err
		}
		groupUID := cedar.NewEntityUID(entityTypeAction, cedar.String(verb))
		if _, ok := ents[groupUID]; !ok {
			ents[groupUID] = cedar.Entity{
				UID:        groupUID,
				Parents:    cedar.NewEntityUIDSet(),
				Attributes: cedar.NewRecord(nil),
			}
		}
		actionUID := cedar.NewEntityUID(entityTypeAction, cedar.String(name))
		ents[actionUID] = cedar.Entity{
			UID:        actionUID,
			Parents:    cedar.NewEntityUIDSet(groupUID),
			Attributes: cedar.NewRecord(nil),
		}
	}
	return ents, nil
}

// principalEntity projects the resolved Identity into a Cedar principal.
// principal.role is EffectiveRole (the authz-relevant, possibly-downgraded
// role). UserID is the entity id; legacy/edge identities without a UserID
// fall back to Subject so the principal still has a stable id.
func principalEntity(id *Identity) (uid cedar.EntityUID, ent cedar.Entity) {
	pid := id.UserID
	if pid == "" {
		pid = id.Subject
	}
	uid = cedar.NewEntityUID(entityTypeUser, cedar.String(pid))
	attrs := cedar.NewRecord(cedar.RecordMap{
		"role":  cedar.String(string(id.EffectiveRole)),
		"id":    cedar.String(id.UserID),
		"email": cedar.String(id.Email),
	})
	ent = cedar.Entity{UID: uid, Parents: cedar.NewEntityUIDSet(), Attributes: attrs}
	return uid, ent
}

// resourceEntity projects a ResourceRef into a Cedar resource. For the
// migration the id is a placeholder ("unspecified"); stories #3/#4 pass real
// ids and attributes (e.g. owner_user_id) that ownership policies read.
func resourceEntity(r ResourceRef) (uid cedar.EntityUID, ent cedar.Entity) {
	id := r.ID
	if id == "" {
		id = "unspecified"
	}
	uid = cedar.NewEntityUID(entityTypeResource, cedar.String(id))
	rm := make(cedar.RecordMap, len(r.Attributes))
	for k, v := range r.Attributes {
		rm[cedar.String(k)] = cedar.String(v)
	}
	ent = cedar.Entity{UID: uid, Parents: cedar.NewEntityUIDSet(), Attributes: cedar.NewRecord(rm)}
	return uid, ent
}

// Evaluate maps the EvalRequest into a cedar.Request, merges the principal,
// resource, and precomputed action entities, and calls cedar.Authorize
// against the current policy set (loaded lock-free from the atomic pointer).
func (e *cedarEngine) Evaluate(ctx context.Context, req EvalRequest) (PolicyDecision, error) {
	if req.Identity == nil {
		return PolicyDecision{}, fmt.Errorf("cedar: Evaluate: nil Identity")
	}
	ps := e.policies.Load()
	if ps == nil {
		return PolicyDecision{}, fmt.Errorf("cedar: Evaluate: no policy set loaded")
	}

	principalUID, principalEnt := principalEntity(req.Identity)
	resourceUID, resourceEnt := resourceEntity(req.Resource)
	actionUID := cedar.NewEntityUID(entityTypeAction, cedar.String(req.Action))

	// Merge precomputed action entities (immutable) with the per-request
	// principal and resource into a fresh EntityMap.
	entities := make(cedar.EntityMap, len(e.actionEntities)+2)
	for uid, ent := range e.actionEntities {
		entities[uid] = ent
	}
	entities[principalUID] = principalEnt
	entities[resourceUID] = resourceEnt

	ctxRecord := make(cedar.RecordMap, len(req.Context))
	for k, v := range req.Context {
		ctxRecord[cedar.String(k)] = cedar.String(v)
	}

	cedarReq := cedar.Request{
		Principal: principalUID,
		Action:    actionUID,
		Resource:  resourceUID,
		Context:   cedar.NewRecord(ctxRecord),
	}

	decision, diag := cedar.Authorize(ps, entities, cedarReq)

	matched := make([]string, 0, len(diag.Reasons))
	for _, r := range diag.Reasons {
		matched = append(matched, string(r.PolicyID))
	}
	var evalErrs []string
	for _, de := range diag.Errors {
		evalErrs = append(evalErrs, de.String())
	}

	slog.LogAttrs(ctx, slog.LevelDebug, "cedar decision",
		slog.String("action", req.Action),
		slog.String("principal", req.Identity.Subject),
		slog.String("role", string(req.Identity.EffectiveRole)),
		slog.Bool("allowed", decision == cedar.Allow),
		slog.Any("policies", matched),
		slog.Any("errors", evalErrs),
	)

	return PolicyDecision{
		Allowed:         decision == cedar.Allow,
		MatchedPolicies: matched,
		Errors:          evalErrs,
	}, nil
}
