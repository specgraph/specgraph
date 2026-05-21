// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func specClient() (specgraphv1connect.SpecServiceClient, error) {
	return newClient(specgraphv1connect.NewSpecServiceClient)
}

// --- create ---

var createCmd = &cobra.Command{
	Use:   "create <slug>",
	Short: "Create a new spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runCreate,
}

var (
	createIntent   string
	createPriority string
)

func runCreate(cmd *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}

	provFlag, err := cmd.Flags().GetString("provenance")
	if err != nil {
		return fmt.Errorf("get --provenance flag: %w", err)
	}
	var provEnum specv1.SpecProvenance
	switch strings.ToLower(provFlag) {
	case "", "authored":
		provEnum = specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED
	case "retroactive_from_pr", "retroactive-from-pr", "retroactive":
		provEnum = specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR
	case "declared":
		provEnum = specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED
	default:
		return fmt.Errorf("invalid --provenance %q (valid: authored, retroactive_from_pr, declared)", provFlag)
	}

	req := &specv1.CreateSpecRequest{
		Slug:           args[0],
		Intent:         createIntent,
		Priority:       createPriority,
		ProvenanceType: provEnum,
	}

	switch provEnum {
	case specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR:
		if setErr := setRetroactiveProvenance(cmd, req); setErr != nil {
			return setErr
		}
	case specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED:
		if setErr := setDeclaredProvenance(cmd, req); setErr != nil {
			return setErr
		}
	}

	if provEnum != specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED {
		if loadErr := loadStageOutputs(cmd, req); loadErr != nil {
			return loadErr
		}
	}

	resp, err := client.CreateSpec(cmd.Context(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("create spec: %w", err)
	}
	fmt.Printf("Created: %s (%s)\n", resp.Msg.GetSpec().GetSlug(), resp.Msg.GetSpec().GetId())
	return nil
}

func setRetroactiveProvenance(cmd *cobra.Command, req *specv1.CreateSpecRequest) error {
	prURL, err := cmd.Flags().GetString("pr-url")
	if err != nil {
		return fmt.Errorf("get --pr-url flag: %w", err)
	}
	prSHA, err := cmd.Flags().GetString("pr-sha")
	if err != nil {
		return fmt.Errorf("get --pr-sha flag: %w", err)
	}
	prTitle, err := cmd.Flags().GetString("pr-title")
	if err != nil {
		return fmt.Errorf("get --pr-title flag: %w", err)
	}
	prMergedAtStr, err := cmd.Flags().GetString("pr-merged-at")
	if err != nil {
		return fmt.Errorf("get --pr-merged-at flag: %w", err)
	}
	var mergedAtPB *timestamppb.Timestamp
	if prMergedAtStr != "" {
		parsed, parseErr := time.Parse(time.RFC3339, prMergedAtStr)
		if parseErr != nil {
			return fmt.Errorf("invalid --pr-merged-at: %w", parseErr)
		}
		mergedAtPB = timestamppb.New(parsed)
	}
	req.ProvenanceDetail = &specv1.CreateSpecRequest_RetroactiveFromPr{
		RetroactiveFromPr: &specv1.RetroactiveFromPrProvenance{
			Url: prURL, Sha: prSHA, Title: prTitle, MergedAt: mergedAtPB,
		},
	}
	return nil
}

func setDeclaredProvenance(cmd *cobra.Command, req *specv1.CreateSpecRequest) error {
	declaredBy, err := cmd.Flags().GetString("declared-by")
	if err != nil {
		return fmt.Errorf("get --declared-by flag: %w", err)
	}
	note, err := cmd.Flags().GetString("declared-note")
	if err != nil {
		return fmt.Errorf("get --declared-note flag: %w", err)
	}
	req.ProvenanceDetail = &specv1.CreateSpecRequest_Declared{
		Declared: &specv1.DeclaredProvenance{DeclaredBy: declaredBy, Note: note},
	}
	return nil
}

func loadStageOutputs(cmd *cobra.Command, req *specv1.CreateSpecRequest) error {
	sparkPath, err := cmd.Flags().GetString("spark-json")
	if err != nil {
		return fmt.Errorf("get --spark-json flag: %w", err)
	}
	shapePath, err := cmd.Flags().GetString("shape-json")
	if err != nil {
		return fmt.Errorf("get --shape-json flag: %w", err)
	}
	specifyPath, err := cmd.Flags().GetString("specify-json")
	if err != nil {
		return fmt.Errorf("get --specify-json flag: %w", err)
	}
	decomposePath, err := cmd.Flags().GetString("decompose-json")
	if err != nil {
		return fmt.Errorf("get --decompose-json flag: %w", err)
	}

	if sparkPath != "" {
		req.SparkOutput = &specv1.SparkOutput{}
		if loadErr := loadJSONFile(sparkPath, req.SparkOutput); loadErr != nil {
			return loadErr
		}
	}
	if shapePath != "" {
		req.ShapeOutput = &specv1.ShapeOutput{}
		if loadErr := loadJSONFile(shapePath, req.ShapeOutput); loadErr != nil {
			return loadErr
		}
	}
	if specifyPath != "" {
		req.SpecifyOutput = &specv1.SpecifyOutput{}
		if loadErr := loadJSONFile(specifyPath, req.SpecifyOutput); loadErr != nil {
			return loadErr
		}
	}
	if decomposePath != "" {
		req.DecomposeOutput = &specv1.DecomposeOutput{}
		if loadErr := loadJSONFile(decomposePath, req.DecomposeOutput); loadErr != nil {
			return loadErr
		}
	}
	return nil
}


// --- update ---

var updateCmd = &cobra.Command{
	Use:   "update <slug>",
	Short: "Update an existing spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runUpdate,
}

var (
	updateIntent     string
	updateStage      string
	updatePriority   string
	updateComplexity string
	updateNotes      string
)

func runUpdate(cmd *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	req := &specv1.UpdateSpecRequest{Slug: args[0]}
	if cmd.Flags().Changed("intent") {
		req.Intent = &updateIntent
	}
	if cmd.Flags().Changed("stage") {
		req.Stage = &updateStage
	}
	if cmd.Flags().Changed("priority") {
		req.Priority = &updatePriority
	}
	if cmd.Flags().Changed("complexity") {
		req.Complexity = &updateComplexity
	}
	if cmd.Flags().Changed("notes") {
		req.Notes = &updateNotes
	}

	resp, err := client.UpdateSpec(cmd.Context(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("update spec: %w", err)
	}
	fmt.Printf("Updated: %s (version %d)\n", resp.Msg.GetSpec().GetSlug(), resp.Msg.GetSpec().GetVersion())
	return nil
}

// --- list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List specs",
	RunE:  runList,
}

var (
	listStage    string
	listPriority string
)

func runList(cmd *cobra.Command, _ []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.ListSpecs(cmd.Context(), connect.NewRequest(&specv1.ListSpecsRequest{
		Stage:    listStage,
		Priority: listPriority,
	}))
	if err != nil {
		return fmt.Errorf("list specs: %w", err)
	}
	if listJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.SpecList(resp.Msg.Specs))
	return nil
}

// --- show ---

var showCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: "Show spec details",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

var (
	showJSON bool
	listJSON bool
)

// registerCreateFlags adds all flags for the create subcommand to cmd.
// Extracted so tests can call it on a bare cobra.Command.
func registerCreateFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&createIntent, "intent", "", "intent for the spec (required)")
	cmd.Flags().StringVar(&createPriority, "priority", "p2", "priority (p0-p3)")
	cmd.Flags().String("provenance", "authored", "provenance type: authored | retroactive_from_pr | declared")
	cmd.Flags().String("pr-url", "", "PR URL (required when --provenance=retroactive_from_pr)")
	cmd.Flags().String("pr-sha", "", "merge commit SHA (required when --provenance=retroactive_from_pr)")
	cmd.Flags().String("pr-title", "", "PR title at import time (optional, retroactive_from_pr only)")
	cmd.Flags().String("pr-merged-at", "", "RFC3339 timestamp of PR merge (optional, retroactive_from_pr only)")
	cmd.Flags().String("declared-by", "", "human or system identifier (required when --provenance=declared)")
	cmd.Flags().String("declared-note", "", "free-text rationale (optional, declared only)")
	cmd.Flags().String("spark-json", "", "path to spark_output JSON file (required for non-AUTHORED)")
	cmd.Flags().String("shape-json", "", "path to shape_output JSON file (required for non-AUTHORED)")
	cmd.Flags().String("specify-json", "", "path to specify_output JSON file (required for non-AUTHORED)")
	cmd.Flags().String("decompose-json", "", "path to decompose_output JSON file (required for non-AUTHORED)")
}

func init() {
	registerCreateFlags(createCmd)
	cobra.CheckErr(createCmd.MarkFlagRequired("intent"))
	rootCmd.AddCommand(createCmd)

	updateCmd.Flags().StringVar(&updateIntent, "intent", "", "new intent")
	updateCmd.Flags().StringVar(&updateStage, "stage", "", "new stage")
	updateCmd.Flags().StringVar(&updatePriority, "priority", "", "new priority")
	updateCmd.Flags().StringVar(&updateComplexity, "complexity", "", "new complexity")
	updateCmd.Flags().StringVar(&updateNotes, "notes", "", "free-text notes")
	rootCmd.AddCommand(updateCmd)

	listCmd.Flags().StringVar(&listStage, "stage", "", "filter by stage")
	listCmd.Flags().StringVar(&listPriority, "priority", "", "filter by priority")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(listCmd)

	showCmd.Flags().BoolVar(&showJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.GetSpec(cmd.Context(), connect.NewRequest(&specv1.GetSpecRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("get spec: %w", err)
	}
	if showJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.Spec(resp.Msg.GetSpec()))
	return nil
}
