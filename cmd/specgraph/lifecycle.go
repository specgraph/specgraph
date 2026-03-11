// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

// errDriftItemsStale is returned when AcknowledgeDrift succeeds but the
// server reports that drift items may be stale (re-check failed after
// acknowledgment). Callers should treat this as a warning-level exit.
var errDriftItemsStale = errors.New("drift items are stale: re-check failed after acknowledgment")

func lifecycleClient() (specgraphv1connect.LifecycleServiceClient, error) {
	return newClient(specgraphv1connect.NewLifecycleServiceClient)
}

// --- amend ---

var amendCmd = &cobra.Command{
	Use:   "amend <slug>",
	Short: "Amend a spec, returning it to an earlier authoring stage",
	Args:  cobra.ExactArgs(1),
	RunE:  runAmend,
}

var (
	amendReason  string
	amendReEntry string
)

func runAmend(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         args[0],
		Reason:       amendReason,
		ReEntryStage: amendReEntry,
	}))
	if err != nil {
		return fmt.Errorf("amend: %w", err)
	}
	s := resp.Msg.GetSpec()
	fmt.Printf("Amended: %s (stage=%s, lifecycle=%s, version=%d)\n", s.GetSlug(), s.GetStage(), s.GetLifecycle().String(), s.GetVersion())
	return nil
}

// --- supersede ---

var supersedeCmd = &cobra.Command{
	Use:   "supersede <slug>",
	Short: "Supersede a spec with a new one",
	Args:  cobra.ExactArgs(1),
	RunE:  runSupersede,
}

var supersedeWith string

func runSupersede(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    args[0],
		NewSlug: supersedeWith,
	}))
	if err != nil {
		return fmt.Errorf("supersede: %w", err)
	}
	old := resp.Msg.GetOldSpec()
	newSpec := resp.Msg.GetNewSpec()
	fmt.Printf("Superseded: %s (lifecycle=%s)\n", old.GetSlug(), old.GetLifecycle().String())
	fmt.Printf("Created:    %s (lifecycle=%s, stage=%s)\n", newSpec.GetSlug(), newSpec.GetLifecycle().String(), newSpec.GetStage())
	return nil
}

// --- abandon ---

var abandonCmd = &cobra.Command{
	Use:   "abandon <slug>",
	Short: "Abandon a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runAbandon,
}

var abandonReason string

func runAbandon(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   args[0],
		Reason: abandonReason,
	}))
	if err != nil {
		return fmt.Errorf("abandon: %w", err)
	}
	s := resp.Msg.GetSpec()
	fmt.Printf("Abandoned: %s (lifecycle=%s, version=%d)\n", s.GetSlug(), s.GetLifecycle().String(), s.GetVersion())
	return nil
}

// --- drift ---

var driftCmd = &cobra.Command{
	Use:   "drift [slug]",
	Short: "Check specs for drift",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDrift,
}

var driftScope string

// SYNC: keep in sync with validScopes (internal/driftscope/scope.go)
// and driftScopeFromProtoMap (internal/server/convert.go).
var driftScopeToProtoMap = map[string]specv1.DriftScope{
	"":           specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED,
	"deps":       specv1.DriftScope_DRIFT_SCOPE_DEPS,
	"interfaces": specv1.DriftScope_DRIFT_SCOPE_INTERFACES,
	"verify":     specv1.DriftScope_DRIFT_SCOPE_VERIFY,
}

func driftScopeToProto(s string) (specv1.DriftScope, error) {
	if v, ok := driftScopeToProtoMap[s]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("invalid scope %q (valid: unspecified/all, deps, interfaces, verify)", s)
}

func runDrift(_ *cobra.Command, args []string) error {
	scope, err := driftScopeToProto(driftScope)
	if err != nil {
		return err
	}
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	req := &specv1.DriftCheckRequest{
		Scope: scope,
	}
	if len(args) > 0 {
		req.Slug = args[0]
	}
	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("drift check: %w", err)
	}
	reports := resp.Msg.GetReports()
	if len(reports) == 0 {
		fmt.Println("No drift detected.")
		return nil
	}
	var hasErrors bool
	var hasDrift bool
	for _, r := range reports {
		ack := ""
		if r.GetAcknowledged() {
			ack = " (acknowledged)"
		}
		fmt.Printf("Spec: %s%s\n", r.GetSpecSlug(), ack)
		for _, item := range r.GetItems() {
			fmt.Printf("  [%s] %s: %s\n", item.GetSeverity(), item.GetType(), item.GetDescription())
			hasDrift = true
		}
		if r.GetErrorMessage() != "" {
			fmt.Fprintf(os.Stderr, "  [error] %s\n", r.GetErrorMessage())
			hasErrors = true
		}
	}
	if hasErrors {
		return fmt.Errorf("drift check completed with errors")
	}
	if hasDrift {
		return fmt.Errorf("drift detected")
	}
	return nil
}

// --- drift acknowledge ---

var driftAckCmd = &cobra.Command{
	Use:   "acknowledge <slug>",
	Short: "Acknowledge drift for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runDriftAck,
}

var driftAckNote string

func runDriftAck(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: args[0],
		Note: driftAckNote,
	}))
	if err != nil {
		return fmt.Errorf("acknowledge drift: %w", err)
	}
	r := resp.Msg.GetReport()
	fmt.Printf("Acknowledged drift for: %s\n", r.GetSpecSlug())
	if r.GetItemsStale() {
		fmt.Fprintf(os.Stderr, "Warning: drift items may be stale (re-check failed after acknowledgment)\n")
		return errDriftItemsStale
	}
	return nil
}

// --- lint ---

var lintCmd = &cobra.Command{
	Use:   "lint [slug]",
	Short: "Lint specs for violations",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLint,
}

func runLint(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	req := &specv1.LintRequest{}
	if len(args) > 0 {
		req.Slug = args[0]
	}
	resp, err := client.Lint(context.Background(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("lint: %w", err)
	}
	results := resp.Msg.GetResults()
	if len(results) == 0 {
		fmt.Println("No lint results.")
		return nil
	}
	failCount := 0
	for _, r := range results {
		if r.GetPassed() {
			fmt.Printf("Spec: %s — PASSED\n", r.GetSpecSlug())
			continue
		}
		failCount++
		fmt.Printf("Spec: %s — FAILED\n", r.GetSpecSlug())
		for _, v := range r.GetViolations() {
			loc := ""
			if v.GetLocation() != "" {
				loc = fmt.Sprintf(" (%s)", v.GetLocation())
			}
			fmt.Printf("  [%s] %s: %s%s\n", v.GetSeverity(), v.GetRule(), v.GetMessage(), loc)
		}
		if r.GetError() != "" {
			fmt.Fprintf(os.Stderr, "  [error] %s\n", r.GetError())
		}
	}
	if failCount == 0 {
		fmt.Println("All specs passed lint checks.")
		return nil
	}
	return fmt.Errorf("lint: %d spec(s) failed", failCount)
}

// --- init ---

func init() {
	amendCmd.Flags().StringVar(&amendReason, "reason", "", "reason for amendment (required)")
	cobra.CheckErr(amendCmd.MarkFlagRequired("reason"))
	amendCmd.Flags().StringVar(&amendReEntry, "re-entry", "", "authoring stage to re-enter (spark|shape|specify|decompose|approved|in_progress|review)")
	rootCmd.AddCommand(amendCmd)

	supersedeCmd.Flags().StringVar(&supersedeWith, "with", "", "slug for the replacement spec (required)")
	cobra.CheckErr(supersedeCmd.MarkFlagRequired("with"))
	rootCmd.AddCommand(supersedeCmd)

	abandonCmd.Flags().StringVar(&abandonReason, "reason", "", "reason for abandonment (required)")
	cobra.CheckErr(abandonCmd.MarkFlagRequired("reason"))
	rootCmd.AddCommand(abandonCmd)

	driftCmd.Flags().StringVar(&driftScope, "scope", "", "drift check scope (deps|interfaces|verify); omit for all")
	driftAckCmd.Flags().StringVar(&driftAckNote, "note", "", "acknowledgement note (required)")
	cobra.CheckErr(driftAckCmd.MarkFlagRequired("note"))
	driftCmd.AddCommand(driftAckCmd)
	rootCmd.AddCommand(driftCmd)

	rootCmd.AddCommand(lintCmd)
}
