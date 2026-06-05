// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
)

// ErrInvalidStage is returned when ComposeInput.Stage is empty or not one of
// the five authoring-funnel stages (spark/shape/specify/decompose/approve).
// Callers can map this to connect.CodeInvalidArgument at the RPC boundary.
var ErrInvalidStage = errors.New("invalid authoring stage")

// ErrSpecNotFound is returned by ComposerBackend.GetSpecSummary when the
// requested slug has no corresponding spec. Callers should treat this as a
// soft miss: the composed prompt succeeds but omits the spec state block.
// Implementations should return this sentinel rather than (nil, nil) when the spec is absent.
var ErrSpecNotFound = errors.New("spec not found")

// Relationship is a typed edge-relationship label used in RelatedSpec.
type Relationship string

const (
	// RelationshipDependsOn indicates a spec depends on another.
	RelationshipDependsOn Relationship = "dependsOn"
	// RelationshipBlocks indicates a spec blocks another.
	RelationshipBlocks Relationship = "blocks"
	// RelationshipComposes indicates a spec composes another.
	RelationshipComposes Relationship = "composes"
)

// IsValid reports whether r is one of the three exported Relationship constants.
// Use a switch for O(1) performance with no allocations.
func (r Relationship) IsValid() bool {
	switch r {
	case RelationshipDependsOn, RelationshipBlocks, RelationshipComposes:
		return true
	default:
		return false
	}
}

// composerValidStages is the ordered set of stages the Composer accepts for
// content routing. This differs from authoringStages in stages.go: the
// composer accepts StageApprove ("approve") as a content-file routing key,
// while authoringStages uses StageApproved ("approved") for storage-side
// transition validation. The two lists intentionally diverge at the final step.
var composerValidStages = []Stage{StageSpark, StageShape, StageSpecify, StageDecompose, StageApprove}

var validStages = func() map[Stage]struct{} {
	m := make(map[Stage]struct{}, len(composerValidStages))
	for _, s := range composerValidStages {
		m[s] = struct{}{}
	}
	return m
}()

// ConstitutionSummary is a bounded digest of the current constitution for
// inclusion in composed prompts. Full constitution available at specgraph://constitution.
type ConstitutionSummary struct {
	PrimaryLanguage string
	KeyConstraints  []string
	Antipatterns    []string
}

// SpecSummary is a bounded view of a spec for composition.
type SpecSummary struct {
	Slug              string
	Intent            string
	Stage             string
	PriorStageSummary string
}

// RelatedSpec is a single related-spec reference for the state section.
type RelatedSpec struct {
	Slug         string
	Intent       string
	Relationship Relationship // RelationshipDependsOn, RelationshipBlocks, or RelationshipComposes
}

// ComposerBackend is the read-only storage surface the composer needs.
// GetSpecSummary returns ErrSpecNotFound when the slug has no spec.
// Implementations should return ErrSpecNotFound rather than (nil, nil) when
// the spec is absent; the composer treats both equivalently as soft-miss but
// the sentinel preserves the not-found / not-configured distinction for callers.
// GetConstitution may return (nil, nil) to indicate no constitution is
// configured; that is a distinct concept from "not found."
type ComposerBackend interface {
	GetConstitution(ctx context.Context) (*ConstitutionSummary, error)
	GetSpecSummary(ctx context.Context, slug string) (*SpecSummary, error)
	GetRelatedSpecs(ctx context.Context, slug string) ([]*RelatedSpec, error)
}

// Composer assembles stage prompts from embedded content plus dynamic state.
type Composer struct {
	backend ComposerBackend
}

// NewComposer returns a Composer wired to the given storage backend. Panics
// if backend is nil — the composer has no sensible behavior without one, and
// a constructor-time panic pins the failure to the wiring bug rather than to
// the first RPC that dereferences it.
func NewComposer(b ComposerBackend) *Composer {
	if b == nil {
		panic("authoring.NewComposer: nil backend")
	}
	return &Composer{backend: b}
}

// ComposeInput selects which stage prompt to compose.
type ComposeInput struct {
	Stage   Stage
	Slug    string
	Posture string
}

// ComposeResult carries the composed body plus observability counters.
type ComposeResult struct {
	Body           string
	StableTokens   int
	DynamicTokens  int
	TotalTokens    int
	TruncatedCount int
}

// ComposeStagePrompt assembles the full composed prompt for the given stage.
func (c *Composer) ComposeStagePrompt(ctx context.Context, in ComposeInput) (result *ComposeResult, retErr error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("compose stage %q: %w", string(in.Stage), err)
	}
	if _, ok := validStages[in.Stage]; !ok {
		validNames := make([]string, len(composerValidStages))
		for i, s := range composerValidStages {
			validNames[i] = string(s)
		}
		return nil, fmt.Errorf("stage %q (valid: %s): %w", string(in.Stage), strings.Join(validNames, ", "), ErrInvalidStage)
	}

	defer func() {
		if retErr != nil {
			slog.LogAttrs(ctx, slog.LevelError, "composer.invocation_failed",
				slog.String("stage", string(in.Stage)),
				slog.String("slug", in.Slug),
				slog.String("posture", in.Posture),
				slog.String("err", retErr.Error()),
			)
		}
	}()

	var b strings.Builder

	for _, name := range []string{
		"persona.md",
		"orchestration.md",
		"conversation-recording.md",
		"quality-heuristics.md",
		"stage-" + string(in.Stage) + ".md",
	} {
		data, err := Content(name)
		if err != nil {
			return nil, fmt.Errorf("load embedded content %s: %w", name, err)
		}
		b.Write(data)
		b.WriteString("\n\n")
	}
	stableLen := approxTokens(b.String())

	dynStart := b.Len()
	truncated, err := c.appendDynamicState(ctx, &b, in)
	if err != nil {
		return nil, fmt.Errorf("compose dynamic state: %w", err)
	}
	dynLen := approxTokens(b.String()[dynStart:])

	// Version footer (design §Embedded content versioning).
	fmt.Fprintf(&b, "\n---\nserver-version: %s\n", versionString())

	totalTokens := approxTokens(b.String())
	slog.LogAttrs(ctx, slog.LevelInfo, "composer.invocation",
		slog.String("stage", string(in.Stage)),
		slog.String("slug", in.Slug),
		slog.String("posture", in.Posture),
		slog.Int("stable_tokens", stableLen),
		slog.Int("dynamic_tokens", dynLen),
		slog.Int("total_tokens", totalTokens),
		slog.Int("truncated_count", truncated),
	)

	return &ComposeResult{
		Body:           b.String(),
		StableTokens:   stableLen,
		DynamicTokens:  dynLen,
		TotalTokens:    totalTokens,
		TruncatedCount: truncated,
	}, nil
}

func (c *Composer) appendDynamicState(ctx context.Context, b *strings.Builder, in ComposeInput) (int, error) {
	var truncated int
	b.WriteString("# Current State\n\n")

	con, err := c.backend.GetConstitution(ctx)
	if err != nil {
		return 0, fmt.Errorf("get constitution: %w", err)
	}
	if con != nil {
		fmt.Fprintf(b, "**Constitution summary**: primary language %s", con.PrimaryLanguage)
		if len(con.KeyConstraints) > 0 {
			constraints := con.KeyConstraints
			if len(constraints) > 5 {
				constraints = constraints[:5]
				truncated++
			}
			fmt.Fprintf(b, "; key constraints: %s", strings.Join(constraints, ", "))
		}
		if len(con.Antipatterns) > 0 {
			antipatterns := con.Antipatterns
			if len(antipatterns) > 5 {
				antipatterns = antipatterns[:5]
				truncated++
			}
			fmt.Fprintf(b, "; antipatterns: %s", strings.Join(antipatterns, ", "))
		}
		b.WriteString(". For full constitution, read `specgraph://constitution`.\n\n")
	}

	if in.Slug != "" {
		spec, err := c.backend.GetSpecSummary(ctx, in.Slug)
		if err != nil && !errors.Is(err, ErrSpecNotFound) {
			return truncated, fmt.Errorf("get spec summary: %w", err)
		}
		if spec != nil {
			fmt.Fprintf(b, "**Spec %s**: %s (stage: %s). For full spec, read `specgraph://spec/%s`.\n\n",
				spec.Slug, spec.Intent, spec.Stage, spec.Slug)
			if spec.PriorStageSummary != "" {
				fmt.Fprintf(b, "**Prior stage summary**: %s\n\n", spec.PriorStageSummary)
			}
		}

		related, err := c.backend.GetRelatedSpecs(ctx, in.Slug)
		if err != nil {
			return truncated, fmt.Errorf("get related specs: %w", err)
		}
		var validRelated []*RelatedSpec
		for _, r := range related {
			if !r.Relationship.IsValid() {
				slog.LogAttrs(ctx, slog.LevelWarn, "composer.invalid_relationship_skipped",
					slog.String("slug", r.Slug),
					slog.String("relationship", string(r.Relationship)),
				)
				continue
			}
			validRelated = append(validRelated, r)
		}
		if len(validRelated) > 0 {
			b.WriteString("**Related specs**: ")
			for i, r := range validRelated {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(b, "%s (%s)", r.Slug, r.Relationship)
			}
			b.WriteString(". Use `graph_query` for full traversal.\n\n")
		}
	}

	return truncated, nil
}

// approxTokens estimates token count using words * 0.75; used for observability, not hard enforcement.
func approxTokens(s string) int {
	words := strings.Fields(s)
	return (len(words) * 3) / 4
}

// versionString returns the runtime build version for the composer footer.
// Prefers module version, then short vcs.revision; falls back to "dev" when
// no build info is available (e.g. `go run`).
func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				if len(s.Value) > 12 {
					return s.Value[:12]
				}
				return s.Value
			}
		}
	}
	return "dev"
}
