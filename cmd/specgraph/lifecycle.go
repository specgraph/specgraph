// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

// exitError is returned from RunE when the command should exit with a specific
// code without Cobra printing the error (SilenceErrors is set on rootCmd).
// The warning/message is printed by the command itself before returning.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

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

func runAmend(cmd *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.TransitionAmend(cmd.Context(), connect.NewRequest(&specv1.TransitionAmendRequest{
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

func runSupersede(cmd *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.TransitionSupersede(cmd.Context(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
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

func runAbandon(cmd *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.TransitionAbandon(cmd.Context(), connect.NewRequest(&specv1.TransitionAbandonRequest{
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

var (
	driftScope string
	driftJSON  bool
)

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

func runDrift(cmd *cobra.Command, args []string) error {
	scope, err := driftScopeToProto(driftScope)
	if err != nil {
		return err
	}
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	req := &specv1.DriftCheckRequest{Scope: scope}
	if len(args) > 0 {
		req.Slug = args[0]
	}
	resp, err := client.CheckDrift(cmd.Context(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("drift check: %w", err)
	}
	reports := resp.Msg.GetReports()

	// Render output (JSON or markdown).
	if driftJSON {
		if jsonErr := printJSON(cmd.OutOrStdout(), resp.Msg); jsonErr != nil {
			return jsonErr
		}
	} else {
		if _, printErr := fmt.Fprint(cmd.OutOrStdout(), render.DriftReport(reports)); printErr != nil {
			return printErr
		}
	}

	// Exit code logic: non-zero exit for errors or drift regardless of output format.
	var hasErrors, hasDrift bool
	for _, r := range reports {
		if r.GetErrorMessage() != "" {
			hasErrors = true
		}
		if len(r.GetItems()) > 0 {
			hasDrift = true
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

var (
	driftAckNote     string
	driftAckUpstream string
	driftAckAll      bool
)

func runDriftAck(cmd *cobra.Command, args []string) error {
	if driftAckUpstream == "" && !driftAckAll {
		return fmt.Errorf("specify --upstream <slug> or --all")
	}
	if driftAckUpstream != "" && driftAckAll {
		return fmt.Errorf("cannot specify both --upstream and --all")
	}
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.AcknowledgeDrift(cmd.Context(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug:         args[0],
		Note:         driftAckNote,
		UpstreamSlug: driftAckUpstream,
		All:          driftAckAll,
	}))
	if err != nil {
		return fmt.Errorf("acknowledge drift: %w", err)
	}
	r := resp.Msg.GetReport()
	if driftAckAll {
		fmt.Printf("Acknowledged drift for %s (all upstreams)\n", r.GetSpecSlug())
	} else {
		fmt.Printf("Acknowledged drift for %s (upstream: %s)\n", r.GetSpecSlug(), driftAckUpstream)
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

func runLint(cmd *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	req := &specv1.LintRequest{}
	if len(args) > 0 {
		req.Slug = args[0]
	}
	resp, err := client.Lint(cmd.Context(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("lint: %w", err)
	}
	results := resp.Msg.GetResults()
	if len(results) == 0 {
		fmt.Println("No lint results.")
		return nil
	}
	failCount := 0
	infraErrCount := 0
	for _, r := range results {
		if r.GetPassed() {
			fmt.Printf("Spec: %s — PASSED\n", r.GetSpecSlug())
			continue
		}
		failCount++
		isInfraError := r.GetError() != "" && len(r.GetViolations()) == 0
		if isInfraError {
			infraErrCount++
		}
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
	if infraErrCount > 0 {
		return fmt.Errorf("lint: %d spec(s) failed (including %d infrastructure error(s))", failCount, infraErrCount)
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
	driftCmd.Flags().BoolVar(&driftJSON, "json", false, "output as JSON")
	driftAckCmd.Flags().StringVar(&driftAckNote, "note", "", "acknowledgement note (required)")
	cobra.CheckErr(driftAckCmd.MarkFlagRequired("note"))
	driftAckCmd.Flags().StringVar(&driftAckUpstream, "upstream", "", "specific upstream slug to acknowledge")
	driftAckCmd.Flags().BoolVar(&driftAckAll, "all", false, "acknowledge all upstream dependencies")
	driftCmd.AddCommand(driftAckCmd)
	rootCmd.AddCommand(driftCmd)

	rootCmd.AddCommand(lintCmd)
}
