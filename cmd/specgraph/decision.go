// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

func decisionClient() (specgraphv1connect.DecisionServiceClient, error) {
	return newClient(specgraphv1connect.NewDecisionServiceClient)
}

// --- decision parent command ---

var decisionCmd = &cobra.Command{
	Use:   "decision",
	Short: "Manage decisions",
}

// --- decision create ---

var decisionCreateCmd = &cobra.Command{
	Use:   "create <slug>",
	Short: "Create a new decision",
	Args:  cobra.ExactArgs(1),
	RunE:  runDecisionCreate,
}

var (
	decisionTitle       string
	decisionText        string
	decisionRationale   string
	decisionQuestion    string
	decisionConfidence  string
	decisionTags        string
	decisionScope       string
	decisionOriginSpec  string
	decisionOriginStage string
	decisionRejected    []string
)

func runDecisionCreate(cmd *cobra.Command, args []string) error {
	client, err := decisionClient()
	if err != nil {
		return err
	}

	confidence, err := parseDecisionConfidence(decisionConfidence)
	if err != nil {
		return err
	}
	scope, err := parseDecisionScope(decisionScope)
	if err != nil {
		return err
	}

	var tags []string
	if decisionTags != "" {
		for _, t := range strings.Split(decisionTags, ",") {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	var rejected []*specv1.RejectedAlternative
	for _, r := range decisionRejected {
		option, reason, found := strings.Cut(r, ":")
		if !found || strings.TrimSpace(option) == "" || strings.TrimSpace(reason) == "" {
			return fmt.Errorf("invalid --rejected value %q: expected \"Option:Reason\"", r)
		}
		rejected = append(rejected, &specv1.RejectedAlternative{
			Option: strings.TrimSpace(option),
			Reason: strings.TrimSpace(reason),
		})
	}

	resp, err := client.CreateDecision(cmd.Context(), connect.NewRequest(&specv1.CreateDecisionRequest{
		Slug:                 args[0],
		Title:                decisionTitle,
		Decision:             decisionText,
		Rationale:            decisionRationale,
		Question:             decisionQuestion,
		Confidence:           confidence,
		Tags:                 tags,
		Scope:                scope,
		OriginSpec:           decisionOriginSpec,
		OriginStage:          decisionOriginStage,
		RejectedAlternatives: rejected,
	}))
	if err != nil {
		return fmt.Errorf("create decision: %w", err)
	}
	fmt.Printf("Created: %s (%s)\n", resp.Msg.GetDecision().GetSlug(), resp.Msg.GetDecision().GetId())
	return nil
}

var decisionConfidenceMap = map[string]specv1.DecisionConfidence{
	"":       specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED,
	"high":   specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
	"medium": specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM,
	"low":    specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW,
}

func parseDecisionConfidence(s string) (specv1.DecisionConfidence, error) {
	if v, ok := decisionConfidenceMap[s]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("unknown confidence %q; valid values: high, medium, low", s)
}

var decisionScopeMap = map[string]specv1.DecisionScope{
	"":        specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED,
	"project": specv1.DecisionScope_DECISION_SCOPE_PROJECT,
	"team":    specv1.DecisionScope_DECISION_SCOPE_TEAM,
	"org":     specv1.DecisionScope_DECISION_SCOPE_ORG,
}

func parseDecisionScope(s string) (specv1.DecisionScope, error) {
	if v, ok := decisionScopeMap[s]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("unknown scope %q; valid values: project, team, org", s)
}

// --- decision list ---

var decisionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List decisions",
	RunE:  runDecisionList,
}

var (
	decisionListStatus string
	decisionListJSON   bool
	decisionShowJSON   bool
)

func runDecisionList(cmd *cobra.Command, _ []string) error {
	client, err := decisionClient()
	if err != nil {
		return err
	}

	var statusFilter specv1.DecisionStatus
	if decisionListStatus != "" {
		decisionStatusToProtoMap := map[string]specv1.DecisionStatus{
			"proposed":   specv1.DecisionStatus_DECISION_STATUS_PROPOSED,
			"accepted":   specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
			"deprecated": specv1.DecisionStatus_DECISION_STATUS_DEPRECATED,
			"superseded": specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED,
		}
		val, ok := decisionStatusToProtoMap[decisionListStatus]
		if !ok {
			return fmt.Errorf("unknown status %q; valid values: proposed, accepted, deprecated, superseded", decisionListStatus)
		}
		statusFilter = val
	}

	resp, err := client.ListDecisions(cmd.Context(), connect.NewRequest(&specv1.ListDecisionsRequest{
		Status: statusFilter,
	}))
	if err != nil {
		return fmt.Errorf("list decisions: %w", err)
	}
	if decisionListJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.DecisionList(resp.Msg.Decisions))
	return nil
}

// --- decision show ---

var decisionShowCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: "Show decision details",
	Args:  cobra.ExactArgs(1),
	RunE:  runDecisionShow,
}

func init() {
	rootCmd.AddCommand(decisionCmd)

	decisionCreateCmd.Flags().StringVar(&decisionTitle, "title", "", "decision title (required)")
	decisionCreateCmd.Flags().StringVar(&decisionText, "decision", "", "decision text")
	decisionCreateCmd.Flags().StringVar(&decisionRationale, "rationale", "", "rationale")
	decisionCreateCmd.Flags().StringVar(&decisionQuestion, "question", "", "the question being decided")
	decisionCreateCmd.Flags().StringVar(&decisionConfidence, "confidence", "", "confidence level (high|medium|low)")
	decisionCreateCmd.Flags().StringVar(&decisionTags, "tags", "", "comma-separated tags")
	decisionCreateCmd.Flags().StringVar(&decisionScope, "scope", "", "decision scope (project|team|org)")
	decisionCreateCmd.Flags().StringVar(&decisionOriginSpec, "origin-spec", "", "slug of originating spec")
	decisionCreateCmd.Flags().StringVar(&decisionOriginStage, "origin-stage", "", "authoring stage")
	decisionCreateCmd.Flags().StringArrayVar(&decisionRejected, "rejected", nil, `rejected alternative "Option:Reason" (repeatable)`)
	cobra.CheckErr(decisionCreateCmd.MarkFlagRequired("title"))
	decisionCmd.AddCommand(decisionCreateCmd)

	decisionListCmd.Flags().StringVar(&decisionListStatus, "status", "", "filter by status")
	decisionListCmd.Flags().BoolVar(&decisionListJSON, "json", false, "output as JSON")
	decisionCmd.AddCommand(decisionListCmd)

	decisionShowCmd.Flags().BoolVar(&decisionShowJSON, "json", false, "output as JSON")
	decisionCmd.AddCommand(decisionShowCmd)
}

func runDecisionShow(cmd *cobra.Command, args []string) error {
	client, err := decisionClient()
	if err != nil {
		return err
	}
	resp, err := client.GetDecision(cmd.Context(), connect.NewRequest(&specv1.GetDecisionRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("get decision: %w", err)
	}
	if decisionShowJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.Decision(resp.Msg.GetDecision()))
	return nil
}
