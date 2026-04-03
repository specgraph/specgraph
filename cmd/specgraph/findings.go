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
	"google.golang.org/protobuf/encoding/protojson"
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

var findingsStoreCmd = &cobra.Command{
	Use:   "store <slug>",
	Short: "Store analytical findings for a spec",
	Long:  "Reads findings from a JSON file (proto3 format) and stores them via the StoreFindings RPC.",
	Args:  cobra.ExactArgs(1),
	RunE:  runFindingsStore,
}

var (
	findingsStorePassType string
	findingsStoreJSON     bool
	findingsStoreFile     string
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

	findingsStoreCmd.Flags().StringVar(&findingsStorePassType, "pass-type", "", "pass type (constitution-check, red-team, peripheral-vision, consistency, simplicity) (required)")
	findingsStoreCmd.Flags().BoolVar(&findingsStoreJSON, "json", false, "output as JSON")
	findingsStoreCmd.Flags().StringVar(&findingsStoreFile, "json-file", "", "path to findings JSON file (required)")
	findingsCmd.AddCommand(findingsStoreCmd)

	rootCmd.AddCommand(findingsCmd)
}

func runFindingsList(cmd *cobra.Command, args []string) error {
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

	resp, err := client.ListFindings(cmd.Context(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("list findings: %w", err)
	}
	if findingsListJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.Findings(resp.Msg.Findings))
	return nil
}

func runFindingsStore(cmd *cobra.Command, args []string) error {
	if findingsStorePassType == "" {
		return fmt.Errorf("--pass-type is required")
	}
	if findingsStoreFile == "" {
		return fmt.Errorf("--json-file is required")
	}
	pt, ok := passTypeMap[findingsStorePassType]
	if !ok {
		return fmt.Errorf("unknown pass type %q; valid: constitution-check, red-team, peripheral-vision, consistency, simplicity", findingsStorePassType)
	}

	data, err := os.ReadFile(findingsStoreFile)
	if err != nil {
		return fmt.Errorf("read json file: %w", err)
	}

	var req specv1.StoreFindingsRequest
	if err := protojson.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("parse json file: %w", err)
	}
	req.Slug = args[0]
	req.PassType = pt

	client, err := analyticalPassClient()
	if err != nil {
		return err
	}

	resp, err := client.StoreFindings(cmd.Context(), connect.NewRequest(&req))
	if err != nil {
		return fmt.Errorf("store findings: %w", err)
	}

	if findingsStoreJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	for _, id := range resp.Msg.Ids {
		fmt.Fprintln(cmd.OutOrStdout(), id)
	}
	return nil
}
