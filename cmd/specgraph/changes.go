// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/diff"
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
	changesDiff         bool
	changesFrom         int32
	changesTo           int32
)

func init() {
	changesCmd.Flags().BoolVar(&changesCheckpoints, "checkpoints", false, "show only checkpoint entries")
	changesCmd.Flags().Int32Var(&changesSinceVersion, "since-version", 0, "show entries after this version")
	changesCmd.Flags().Int32Var(&changesLimit, "limit", 0, "maximum number of entries (0 = all)")
	changesCmd.Flags().BoolVar(&changesJSON, "json", false, "output as JSON")
	changesCmd.Flags().BoolVar(&changesDiff, "diff", false, "show inline word-level diffs")
	changesCmd.Flags().Int32Var(&changesFrom, "from", 0, "compare from this version (requires --diff)")
	changesCmd.Flags().Int32Var(&changesTo, "to", 0, "compare to this version (requires --diff)")
	rootCmd.AddCommand(changesCmd)
}

func runChanges(cmd *cobra.Command, args []string) error {
	client, err := newClient(specgraphv1connect.NewSpecServiceClient)
	if err != nil {
		return err
	}

	// Version comparison mode: --diff with --from or --to
	if changesDiff && (changesFrom != 0 || changesTo != 0) {
		resp, err := client.CompareVersions(cmd.Context(), connect.NewRequest(&specv1.CompareVersionsRequest{
			Slug:        args[0],
			FromVersion: changesFrom,
			ToVersion:   changesTo,
		}))
		if err != nil {
			return fmt.Errorf("compare versions: %w", err)
		}
		if changesJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), render.VersionComparison(resp.Msg))
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

	// Inline diff mode: --diff without --from/--to
	if changesDiff {
		hunksProvider := func(oldVal, newVal string) []*specv1.InlineDiff {
			hunks := diff.ComputeHunks(oldVal, newVal)
			pbs := make([]*specv1.InlineDiff, len(hunks))
			for i, h := range hunks {
				var op specv1.InlineDiff_Op
				switch h.Op {
				case diff.OpInsert:
					op = specv1.InlineDiff_INSERT
				case diff.OpDelete:
					op = specv1.InlineDiff_DELETE
				default:
					op = specv1.InlineDiff_EQUAL
				}
				pbs[i] = &specv1.InlineDiff{Op: op, Text: h.Text}
			}
			return pbs
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), render.ChangesWithDiff(resp.Msg.Entries, hunksProvider))
		return err
	}

	_, err = fmt.Fprint(cmd.OutOrStdout(), render.Changes(resp.Msg.Entries))
	return err
}
