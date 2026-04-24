// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
)

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
	Relationship string // "dependsOn", "blocks", "composes", etc.
}

// ComposerBackend is the read-only storage surface the composer needs.
type ComposerBackend interface {
	GetConstitution(ctx context.Context) (*ConstitutionSummary, error)
	GetSpecSummary(ctx context.Context, slug string) (*SpecSummary, error)
	GetRelatedSpecs(ctx context.Context, slug string) ([]*RelatedSpec, error)
}

// Composer assembles stage prompts from embedded content plus dynamic state.
type Composer struct {
	backend ComposerBackend
}

// NewComposer returns a Composer wired to the given storage backend.
func NewComposer(b ComposerBackend) *Composer { return &Composer{backend: b} }

// ComposeInput selects which stage prompt to compose.
type ComposeInput struct {
	Stage   string
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
func (c *Composer) ComposeStagePrompt(ctx context.Context, in ComposeInput) (*ComposeResult, error) {
	var b strings.Builder

	for _, name := range []string{
		"persona.md",
		"orchestration.md",
		"conversation-recording.md",
		"quality-heuristics.md",
		"stage-" + in.Stage + ".md",
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

	return &ComposeResult{
		Body:           b.String(),
		StableTokens:   stableLen,
		DynamicTokens:  dynLen,
		TotalTokens:    approxTokens(b.String()),
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
		if err != nil {
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
		if len(related) > 0 {
			b.WriteString("**Related specs**: ")
			for i, r := range related {
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
