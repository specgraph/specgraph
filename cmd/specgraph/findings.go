// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

func analyticalPassClient() (specgraphv1connect.AnalyticalPassServiceClient, error) {
	return newClient(specgraphv1connect.NewAnalyticalPassServiceClient)
}

var findingsCmd = &cobra.Command{
	Use:   "findings",
	Short: "Manage analytical findings",
}

var findingsListCmd = &cobra.Command{
	Use:   "list <slug>",
	Short: "List findings for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runFindingsList,
}

var (
	findingsListPassType string
	findingsListJSON     bool
)

// passTypeMap maps friendly CLI names to proto enum values,
// following the same pattern as driftScopeToProtoMap in lifecycle.go.
var passTypeMap = map[string]specv1.PassType{
	"constitution-check": specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
	"red-team":           specv1.PassType_PASS_TYPE_RED_TEAM,
	"peripheral-vision":  specv1.PassType_PASS_TYPE_PERIPHERAL_VISION,
	"consistency":        specv1.PassType_PASS_TYPE_CONSISTENCY,
	"simplicity":         specv1.PassType_PASS_TYPE_SIMPLICITY,
}

func init() {
	findingsListCmd.Flags().StringVar(&findingsListPassType, "pass-type", "", "filter by pass type (constitution-check, red-team, peripheral-vision, consistency, simplicity)")
	findingsListCmd.Flags().BoolVar(&findingsListJSON, "json", false, "output as JSON")
	findingsCmd.AddCommand(findingsListCmd)
	rootCmd.AddCommand(findingsCmd)
}

func runFindingsList(_ *cobra.Command, args []string) error {
	client, err := analyticalPassClient()
	if err != nil {
		return err
	}

	req := &specv1.ListFindingsRequest{Slug: args[0]}
	if findingsListPassType != "" {
		pt, ok := passTypeMap[findingsListPassType]
		if !ok {
			return fmt.Errorf("unknown pass type %q; valid: constitution-check, red-team, peripheral-vision, consistency, simplicity", findingsListPassType)
		}
		req.PassType = pt
	}

	resp, err := client.ListFindings(context.Background(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("list findings: %w", err)
	}
	if findingsListJSON {
		return printJSON(resp.Msg)
	}
	fmt.Print(render.Findings(resp.Msg.Findings))
	return nil
}
