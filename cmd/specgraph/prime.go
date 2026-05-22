// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var (
	primeShowProvenance bool
	primeJSON           bool
)

// runUpFn is the seam tests swap to verify that prime preserves the
// server-start side effect (Claude Code's SessionStart hook depends on
// it; design Section 10 line 681 mandates preservation). Production
// code points it at runUp.
var runUpFn = runUp

var primeCmd = &cobra.Command{
	Use:   "prime [slug]",
	Short: "Orient Claude Code to the current project (or to a specific spec)",
	Long: `Ensure the server is running, then print the project prime (constitution summary,
graph overview, ready specs, findings, skills) or, when a slug is given, the spec prime.
Used by Claude Code's SessionStart hook (no-arg form).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPrime,
}

func init() {
	primeCmd.Flags().BoolVar(&primeShowProvenance, "show-provenance", false, "Annotate constitution sections with provenance (set by: <layer>) markers")
	primeCmd.Flags().BoolVar(&primeJSON, "json", false, "Output the proto-native JSON form")
	rootCmd.AddCommand(primeCmd)
}

func runPrime(cmd *cobra.Command, args []string) error {
	// runUp is the SessionStart-hook ergonomic; removing it breaks
	// Claude Code's session prime (spgr-8ar Piece E design §10).
	if err := runUpFn(cmd, args); err != nil {
		// Non-fatal: server may already be running via manual mode.
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: up: %v\n", err) //nolint:errcheck // best-effort warning output
	}

	slug := ""
	if len(args) == 1 {
		slug = args[0]
	}

	client, err := executionClient()
	if err != nil {
		return fmt.Errorf("create execution client: %w", err)
	}
	resp, err := client.GetPrime(cmd.Context(), connect.NewRequest(&specv1.GetPrimeRequest{Slug: slug}))
	if err != nil {
		return fmt.Errorf("get prime: %w", err)
	}

	opts := render.RenderOpts{ShowProvenance: primeShowProvenance}

	switch v := resp.Msg.GetView().(type) {
	case *specv1.PrimeResponse_ProjectView:
		return writePrime(cmd, v.ProjectView, nil, opts)
	case *specv1.PrimeResponse_SpecView:
		return writePrime(cmd, nil, v.SpecView, opts)
	default:
		return fmt.Errorf("prime response missing view")
	}
}

func writePrime(cmd *cobra.Command, project *specv1.ProjectView, spec *specv1.SpecView, opts render.RenderOpts) error {
	if primeJSON {
		var msg proto.Message
		if project != nil {
			msg = render.ProjectViewForJSON(project, opts)
		} else {
			msg = render.SpecViewForJSON(spec, opts)
		}
		if err := printJSON(cmd.OutOrStdout(), msg); err != nil {
			return fmt.Errorf("render json: %w", err)
		}
		return nil
	}
	var body string
	if project != nil {
		body = render.RenderProjectMarkdown(project, opts)
	} else {
		body = render.RenderSpecMarkdown(spec, opts)
	}
	fmt.Fprint(cmd.OutOrStdout(), body) //nolint:errcheck // stdout write
	return nil
}
