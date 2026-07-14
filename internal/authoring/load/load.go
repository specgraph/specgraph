// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package load parses friendly snake_case YAML for the four authoring funnel
// stages (spark, shape, specify, decompose) into their stage proto outputs
// (*specv1.SparkOutput/ShapeOutput/SpecifyOutput/DecomposeOutput). It mirrors
// the internal/constitution/load pipeline so an MCP-only agent authors from
// friendly YAML instead of raw protojson: the server stays the schema. Input
// is unmarshalled into fixed typed structs (never map[string]any) to bound the
// input shape, and unknown enum values are rejected with an error rather than
// silently written as UNSPECIFIED.
package load

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"gopkg.in/yaml.v3"
)

// --- Enum mappers ---

// scopeSniffFromString maps a friendly lowercase scope value (e.g. "medium")
// to a ScopeSniff enum. Unknown values map to SCOPE_SNIFF_UNSPECIFIED, which
// callers MUST convert into a returned error (never silent-write).
func scopeSniffFromString(s string) specv1.ScopeSniff {
	key := "SCOPE_SNIFF_" + strings.ToUpper(strings.ReplaceAll(s, "-", "_"))
	if v, ok := specv1.ScopeSniff_value[key]; ok {
		return specv1.ScopeSniff(v)
	}
	return specv1.ScopeSniff_SCOPE_SNIFF_UNSPECIFIED
}

// decompositionStrategyFromString maps a friendly lowercase multi-token
// strategy value (e.g. "vertical_slice") to a DecompositionStrategy enum.
// Unknown values map to DECOMPOSITION_STRATEGY_UNSPECIFIED, which callers MUST
// convert into a returned error (never silent-write).
func decompositionStrategyFromString(s string) specv1.DecompositionStrategy {
	key := "DECOMPOSITION_STRATEGY_" + strings.ToUpper(strings.ReplaceAll(s, "-", "_"))
	if v, ok := specv1.DecompositionStrategy_value[key]; ok {
		return specv1.DecompositionStrategy(v)
	}
	return specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_UNSPECIFIED
}

// --- Friendly-input structs (snake_case yaml tags match proto field names) ---

type sparkYAML struct {
	Seed       string   `yaml:"seed"`
	Signal     string   `yaml:"signal"`
	Questions  []string `yaml:"questions"`
	ScopeSniff string   `yaml:"scope_sniff"`
	KillTest   string   `yaml:"kill_test"`
}

type approachYAML struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tradeoffs   []string `yaml:"tradeoffs"`
}

type decisionYAML struct {
	Slug      string `yaml:"slug"`
	Title     string `yaml:"title"`
	Decision  string `yaml:"decision"`
	Rationale string `yaml:"rationale"`
}

type shapeYAML struct {
	ScopeIn        []string       `yaml:"scope_in"`
	ScopeOut       []string       `yaml:"scope_out"`
	Approaches     []approachYAML `yaml:"approaches"`
	ChosenApproach string         `yaml:"chosen_approach"`
	Risks          []string       `yaml:"risks"`
	SuccessMust    []string       `yaml:"success_must"`
	SuccessShould  []string       `yaml:"success_should"`
	SuccessWont    []string       `yaml:"success_wont"`
	Decisions      []decisionYAML `yaml:"decisions"`
}

type interfaceYAML struct {
	Name string `yaml:"name"`
	Body string `yaml:"body"`
}

type criterionYAML struct {
	Category    string `yaml:"category"`
	Description string `yaml:"description"`
}

type touchYAML struct {
	Path       string `yaml:"path"`
	Purpose    string `yaml:"purpose"`
	ChangeType string `yaml:"change_type"`
}

type specifyYAML struct {
	Interfaces     []interfaceYAML `yaml:"interfaces"`
	VerifyCriteria []criterionYAML `yaml:"verify_criteria"`
	Invariants     []string        `yaml:"invariants"`
	Touches        []touchYAML     `yaml:"touches"`
}

type sliceYAML struct {
	ID        string   `yaml:"id"`
	Intent    string   `yaml:"intent"`
	Verify    []string `yaml:"verify"`
	Touches   []string `yaml:"touches"`
	DependsOn []string `yaml:"depends_on"`
}

type decomposeYAML struct {
	Strategy string      `yaml:"strategy"`
	Slices   []sliceYAML `yaml:"slices"`
}

// --- Parsers ---

// SparkFromYAML parses friendly spark-stage YAML into a *specv1.SparkOutput.
// An unknown scope_sniff value is rejected with an error.
func SparkFromYAML(data []byte) (*specv1.SparkOutput, error) {
	var in sparkYAML
	if err := yaml.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("parse spark yaml: %w", err)
	}
	out := &specv1.SparkOutput{
		Seed:      in.Seed,
		Signal:    in.Signal,
		Questions: in.Questions,
		KillTest:  in.KillTest,
	}
	if in.ScopeSniff != "" {
		ss := scopeSniffFromString(in.ScopeSniff)
		if ss == specv1.ScopeSniff_SCOPE_SNIFF_UNSPECIFIED {
			return nil, fmt.Errorf("invalid scope_sniff: %q", in.ScopeSniff)
		}
		out.ScopeSniff = ss
	}
	return out, nil
}

// ShapeFromYAML parses friendly shape-stage YAML into a *specv1.ShapeOutput,
// including nested approaches (with tradeoffs) and decisions.
func ShapeFromYAML(data []byte) (*specv1.ShapeOutput, error) {
	var in shapeYAML
	if err := yaml.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("parse shape yaml: %w", err)
	}
	out := &specv1.ShapeOutput{
		ScopeIn:        in.ScopeIn,
		ScopeOut:       in.ScopeOut,
		ChosenApproach: in.ChosenApproach,
		Risks:          in.Risks,
		SuccessMust:    in.SuccessMust,
		SuccessShould:  in.SuccessShould,
		SuccessWont:    in.SuccessWont,
	}
	for _, a := range in.Approaches {
		out.Approaches = append(out.Approaches, &specv1.Approach{
			Name:        a.Name,
			Description: a.Description,
			Tradeoffs:   a.Tradeoffs,
		})
	}
	for _, d := range in.Decisions {
		out.Decisions = append(out.Decisions, &specv1.DecisionInput{
			Slug:      d.Slug,
			Title:     d.Title,
			Decision:  d.Decision,
			Rationale: d.Rationale,
		})
	}
	return out, nil
}

// SpecifyFromYAML parses friendly specify-stage YAML into a *specv1.SpecifyOutput,
// including nested interfaces, verify_criteria, and touches.
func SpecifyFromYAML(data []byte) (*specv1.SpecifyOutput, error) {
	var in specifyYAML
	if err := yaml.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("parse specify yaml: %w", err)
	}
	out := &specv1.SpecifyOutput{
		Invariants: in.Invariants,
	}
	for _, iface := range in.Interfaces {
		out.Interfaces = append(out.Interfaces, &specv1.InterfaceSection{
			Name: iface.Name,
			Body: iface.Body,
		})
	}
	for _, vc := range in.VerifyCriteria {
		out.VerifyCriteria = append(out.VerifyCriteria, &specv1.VerifyCriterion{
			Category:    vc.Category,
			Description: vc.Description,
		})
	}
	for _, t := range in.Touches {
		out.Touches = append(out.Touches, &specv1.FileTouch{
			Path:       t.Path,
			Purpose:    t.Purpose,
			ChangeType: t.ChangeType,
		})
	}
	return out, nil
}

// DecomposeFromYAML parses friendly decompose-stage YAML into a
// *specv1.DecomposeOutput. An unknown strategy value is rejected with an error.
func DecomposeFromYAML(data []byte) (*specv1.DecomposeOutput, error) {
	var in decomposeYAML
	if err := yaml.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("parse decompose yaml: %w", err)
	}
	out := &specv1.DecomposeOutput{}
	if in.Strategy != "" {
		strategy := decompositionStrategyFromString(in.Strategy)
		if strategy == specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_UNSPECIFIED {
			return nil, fmt.Errorf("invalid strategy: %q", in.Strategy)
		}
		out.Strategy = strategy
	}
	for _, s := range in.Slices {
		out.Slices = append(out.Slices, &specv1.DecompositionSlice{
			Id:        s.ID,
			Intent:    s.Intent,
			Verify:    s.Verify,
			Touches:   s.Touches,
			DependsOn: s.DependsOn,
		})
	}
	return out, nil
}
