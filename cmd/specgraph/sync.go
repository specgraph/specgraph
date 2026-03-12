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
	RunE:  runSyncBeads,
}

var (
	syncFilterStage    string
	syncFilterPriority string
	syncDryRun         bool
)

func runSyncBeads(_ *cobra.Command, _ []string) error {
	client, err := syncClient()
	if err != nil {
		return err
	}

	resp, err := client.SyncBeads(context.Background(), connect.NewRequest(&specv1.SyncBeadsRequest{
		Config: &specv1.SyncConfig{
			Adapter:        specv1.SyncAdapter_SYNC_ADAPTER_BEADS,
			FilterStage:    syncFilterStage,
			FilterPriority: syncFilterPriority,
			DryRun:         syncDryRun,
		},
	}))
	if err != nil {
		return fmt.Errorf("sync beads: %w", err)
	}

	printSyncResponse(resp.Msg)
	return nil
}

// --- sync github ---

var syncGitHubCmd = &cobra.Command{
	Use:   "github",
	Short: "Push specs as GitHub Issues",
	RunE:  runSyncGitHub,
}

func runSyncGitHub(_ *cobra.Command, _ []string) error {
	client, err := syncClient()
	if err != nil {
		return err
	}

	resp, err := client.SyncGitHub(context.Background(), connect.NewRequest(&specv1.SyncGitHubRequest{
		Config: &specv1.SyncConfig{
			Adapter:        specv1.SyncAdapter_SYNC_ADAPTER_GITHUB,
			FilterStage:    syncFilterStage,
			FilterPriority: syncFilterPriority,
			DryRun:         syncDryRun,
		},
	}))
	if err != nil {
		return fmt.Errorf("sync github: %w", err)
	}

	printSyncResponse(resp.Msg)
	return nil
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
		fmt.Println("No sync mappings found.")
		return nil
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

func printSyncResponse(resp *specv1.SyncResponse) {
	fmt.Printf("Synced: %d  Skipped: %d  Errors: %d\n", resp.Synced, resp.Skipped, resp.Errors)
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
		fmt.Printf("  [%s] %s", stateIcon, r.SpecSlug)
		if r.ExternalId != "" {
			fmt.Printf(" -> %s", r.ExternalId)
		}
		if r.Message != "" {
			fmt.Printf(" (%s)", r.Message)
		}
		fmt.Println()
	}
}

func init() {
	syncBeadsCmd.Flags().StringVar(&syncFilterStage, "stage", "", "only sync specs at this stage")
	syncBeadsCmd.Flags().StringVar(&syncFilterPriority, "priority", "", "only sync specs at this priority")
	syncBeadsCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "show what would be synced without syncing")

	syncGitHubCmd.Flags().StringVar(&syncFilterStage, "stage", "", "only sync specs at this stage")
	syncGitHubCmd.Flags().StringVar(&syncFilterPriority, "priority", "", "only sync specs at this priority")
	syncGitHubCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "show what would be synced without syncing")

	syncStatusCmd.Flags().StringVar(&statusAdapter, "adapter", "", "filter by adapter (beads, github)")
	syncStatusCmd.Flags().StringVar(&statusSpec, "spec", "", "filter by spec slug")

	syncCmd.AddCommand(syncBeadsCmd)
	syncCmd.AddCommand(syncGitHubCmd)
	syncCmd.AddCommand(syncStatusCmd)
	rootCmd.AddCommand(syncCmd)
}
