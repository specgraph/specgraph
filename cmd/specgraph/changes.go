// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

var changesCmd = &cobra.Command{
	Use:   "changes <slug>",
	Short: "List changelog entries for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runChanges,
}

var (
	changesCheckpoints  bool
	changesSinceVersion int32
	changesLimit        int32
	changesJSON         bool
)

func init() {
	changesCmd.Flags().BoolVar(&changesCheckpoints, "checkpoints", false, "show only checkpoint entries")
	changesCmd.Flags().Int32Var(&changesSinceVersion, "since-version", 0, "show entries after this version")
	changesCmd.Flags().Int32Var(&changesLimit, "limit", 0, "maximum number of entries (0 = all)")
	changesCmd.Flags().BoolVar(&changesJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(changesCmd)
}

func runChanges(cmd *cobra.Command, args []string) error {
	client, err := newClient(specgraphv1connect.NewSpecServiceClient)
	if err != nil {
		return err
	}

	resp, err := client.ListChanges(cmd.Context(), connect.NewRequest(&specv1.ListChangesRequest{
		Slug:            args[0],
		CheckpointsOnly: changesCheckpoints,
		SinceVersion:    changesSinceVersion,
		Limit:           changesLimit,
	}))
	if err != nil {
		return fmt.Errorf("list changes: %w", err)
	}
	if changesJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), render.Changes(resp.Msg.Entries))
	return err
}
