// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"

// Prompt is a named template used during a funnel stage.
type Prompt struct {
	Name     string
	Template string
}

// promptRegistry maps funnel stage names to their ordered prompt lists.
// promptRegistry is effectively immutable after init; do not modify at runtime.
var promptRegistry = map[Stage][]Prompt{
	StageSpark: {
		{Name: "seed", Template: "Describe the core idea or need in one sentence."},
		{Name: "signal", Template: "What signal or evidence prompted this idea?"},
		{Name: "scope_sniff", Template: "Roughly how large is this? (tiny / small / medium / large / epic)"},
		{Name: "unknowns", Template: "List the biggest unknowns or open questions."},
		{Name: "kill_test", Template: "What would make you kill this idea right now?"},
	},
	StageShape: {
		{Name: "bound_scope", Template: "Define what is explicitly in scope and out of scope."},
		{Name: "explore_solutions", Template: "List 2-3 candidate solution approaches with trade-offs."},
		{Name: "identify_edges", Template: "Identify dependencies, blockers, and compositions with other specs."},
		{Name: "surface_risks", Template: "What are the top risks and how might you mitigate them?"},
		{Name: "define_success", Template: "What does success look like? Define measurable acceptance criteria."},
	},
	StageSpecify: {
		{Name: "interface_contract", Template: "Define the public interface: inputs, outputs, error cases."},
		{Name: "verify_criteria", Template: "Write verification criteria that a reviewer can check mechanically."},
		{Name: "invariants", Template: "State the invariants that must hold before, during, and after execution."},
	},
	StageDecompose: {
		{Name: "strategy", Template: "Choose a decomposition strategy: vertical slices, horizontal layers, or risk-first."},
		{Name: "slices", Template: "Break the spec into ordered, independently-deliverable slices."},
	},
}

// GetPrompts returns a copy of the prompts for the given funnel stage.
// Returns nil if the stage is not found.
func GetPrompts(stage Stage) []Prompt {
	src := promptRegistry[stage]
	if len(src) == 0 {
		return nil
	}
	out := make([]Prompt, len(src))
	copy(out, src)
	return out
}

// stageToProtoEnum maps domain stage values to proto AuthoringStage values.
var stageToProtoEnum = map[Stage]specv1.AuthoringStage{
	StageSpark:     specv1.AuthoringStage_AUTHORING_STAGE_SPARK,
	StageShape:     specv1.AuthoringStage_AUTHORING_STAGE_SHAPE,
	StageSpecify:   specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY,
	StageDecompose: specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE,
	StageApproved:  specv1.AuthoringStage_AUTHORING_STAGE_APPROVED,
}

// PromptsToProto converts the prompts for a stage into protobuf PromptTemplate messages.
// Returns nil if the stage is not found.
func PromptsToProto(stage Stage) []*specv1.PromptTemplate {
	prompts := promptRegistry[stage]
	if len(prompts) == 0 {
		return nil
	}
	protoStage := stageToProtoEnum[stage]
	out := make([]*specv1.PromptTemplate, len(prompts))
	for i, p := range prompts {
		out[i] = &specv1.PromptTemplate{
			Stage:    protoStage,
			Name:     p.Name,
			Template: p.Template,
		}
	}
	return out
}
