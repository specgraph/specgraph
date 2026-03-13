// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func syncClient() (specgraphv1connect.SyncServiceClient, error) {
	return newClient(specgraphv1connect.NewSyncServiceClient)
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync specs with external systems",
}

// --- sync beads ---

var syncBeadsCmd = &cobra.Command{
	Use:   "beads",
	Short: "Push approved specs to Beads as issues",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runSyncBeads(cmd)
	},
}

var (
	beadsFilterStage    string
	beadsFilterPriority string
	beadsDryRun         bool
)

var (
	ghFilterStage    string
	ghFilterPriority string
	ghDryRun         bool
)

func runSyncBeads(cmd *cobra.Command) error {
	client, err := syncClient()
	if err != nil {
		return err
	}

	resp, err := client.SyncBeads(context.Background(), connect.NewRequest(&specv1.SyncBeadsRequest{
		Config: &specv1.SyncConfig{
			Adapter:        specv1.SyncAdapter_SYNC_ADAPTER_BEADS,
			FilterStage:    beadsFilterStage,
			FilterPriority: beadsFilterPriority,
			DryRun:         beadsDryRun,
		},
	}))
	if err != nil {
		return fmt.Errorf("sync beads: %w", err)
	}

	return printSyncResponse(cmd, resp.Msg)
}

// --- sync github ---

var syncGitHubCmd = &cobra.Command{
	Use:   "github",
	Short: "Push specs as GitHub Issues",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runSyncGitHub(cmd)
	},
}

func runSyncGitHub(cmd *cobra.Command) error {
	client, err := syncClient()
	if err != nil {
		return err
	}

	resp, err := client.SyncGitHub(context.Background(), connect.NewRequest(&specv1.SyncGitHubRequest{
		Config: &specv1.SyncConfig{
			Adapter:        specv1.SyncAdapter_SYNC_ADAPTER_GITHUB,
			FilterStage:    ghFilterStage,
			FilterPriority: ghFilterPriority,
			DryRun:         ghDryRun,
		},
	}))
	if err != nil {
		return fmt.Errorf("sync github: %w", err)
	}

	return printSyncResponse(cmd, resp.Msg)
}

// --- sync status ---

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync state for all specs",
	RunE:  runSyncStatus,
}

var (
	statusAdapter string
	statusSpec    string
)

func runSyncStatus(cmd *cobra.Command, _ []string) error {
	client, err := syncClient()
	if err != nil {
		return err
	}

	adapter := specv1.SyncAdapter_SYNC_ADAPTER_UNSPECIFIED
	switch strings.ToLower(statusAdapter) {
	case "":
		// no filter
	case "beads":
		adapter = specv1.SyncAdapter_SYNC_ADAPTER_BEADS
	case "github":
		adapter = specv1.SyncAdapter_SYNC_ADAPTER_GITHUB
	default:
		return fmt.Errorf("unsupported adapter: %s (supported: beads, github)", statusAdapter)
	}

	resp, err := client.GetSyncStatus(context.Background(), connect.NewRequest(&specv1.SyncStatusRequest{
		Adapter:  adapter,
		SpecSlug: statusSpec,
	}))
	if err != nil {
		return fmt.Errorf("sync status: %w", err)
	}

	mappings := resp.Msg.Mappings
	if len(mappings) == 0 {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "No sync mappings found.")
		return err
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	tw := &tableWriter{w: w}
	tw.println("SPEC\tADAPTER\tEXTERNAL_ID\tSTATE\tLAST_SYNC")
	for _, m := range mappings {
		lastSync := ""
		if m.LastSync != nil {
			lastSync = m.LastSync.AsTime().Format("2006-01-02 15:04:05")
		}
		tw.printf("%s\t%s\t%s\t%s\t%s\n",
			m.SpecSlug,
			m.Adapter.String(),
			m.ExternalId,
			m.State.String(),
			lastSync,
		)
	}
	if tw.err != nil {
		return tw.err
	}
	return w.Flush()
}

func printSyncResponse(cmd *cobra.Command, resp *specv1.SyncResponse) error {
	w := cmd.OutOrStdout()
	tw := &tableWriter{w: w}
	tw.printf("Synced: %d  Skipped: %d  DryRun: %d  Errors: %d\n", resp.Synced, resp.Skipped, resp.Errors)
	for _, r := range resp.Results {
		stateIcon := " "
		switch r.State {
		case specv1.SyncState_SYNC_STATE_SYNCED:
			stateIcon = "+"
		case specv1.SyncState_SYNC_STATE_ERROR:
			stateIcon = "!"
		case specv1.SyncState_SYNC_STATE_PENDING:
			stateIcon = "~"
		}
		tw.printf("  [%s] %s", stateIcon, r.SpecSlug)
		if r.ExternalId != "" {
			tw.printf(" -> %s", r.ExternalId)
		}
		if r.Message != "" {
			tw.printf(" (%s)", r.Message)
		}
		tw.println("")
	}
	return tw.err
}

func init() {
	syncBeadsCmd.Flags().StringVar(&beadsFilterStage, "stage", "", "only sync specs at this stage")
	syncBeadsCmd.Flags().StringVar(&beadsFilterPriority, "priority", "", "only sync specs at this priority")
	syncBeadsCmd.Flags().BoolVar(&beadsDryRun, "dry-run", false, "show what would be synced without syncing")

	syncGitHubCmd.Flags().StringVar(&ghFilterStage, "stage", "", "only sync specs at this stage")
	syncGitHubCmd.Flags().StringVar(&ghFilterPriority, "priority", "", "only sync specs at this priority")
	syncGitHubCmd.Flags().BoolVar(&ghDryRun, "dry-run", false, "show what would be synced without syncing")

	syncStatusCmd.Flags().StringVar(&statusAdapter, "adapter", "", "filter by adapter (beads, github)")
	syncStatusCmd.Flags().StringVar(&statusSpec, "spec", "", "filter by spec slug")

	syncCmd.AddCommand(syncBeadsCmd)
	syncCmd.AddCommand(syncGitHubCmd)
	syncCmd.AddCommand(syncStatusCmd)
	rootCmd.AddCommand(syncCmd)
}
