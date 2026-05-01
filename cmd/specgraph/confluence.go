// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render/markdown"
	"github.com/spf13/cobra"
)

var confluenceCmd = &cobra.Command{
	Use:   "confluence",
	Short: "Manage Confluence publishing",
}

var (
	confluencePublishJSON      bool
	confluenceStatusJSON       bool
	confluenceSyncCommentsJSON bool
	confluenceUnpublishJSON    bool
)

var confluencePublishCmd = &cobra.Command{
	Use:   "publish <slug>",
	Short: "Publish or re-publish a spec to Confluence",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfluencePublish,
}

var confluenceStatusCmd = &cobra.Command{
	Use:   "status [slug]",
	Short: "Show Confluence publish status",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConfluenceStatus,
}

var confluenceSyncCommentsCmd = &cobra.Command{
	Use:   "sync-comments [slug]",
	Short: "Poll Confluence comments for published specs",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConfluenceSyncComments,
}

var confluenceUnpublishCmd = &cobra.Command{
	Use:   "unpublish <slug>",
	Short: "Remove published pages from Confluence",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfluenceUnpublish,
}

func init() {
	confluencePublishCmd.Flags().BoolVar(&confluencePublishJSON, "json", false, "Output as JSON")
	confluenceStatusCmd.Flags().BoolVar(&confluenceStatusJSON, "json", false, "Output as JSON")
	confluenceSyncCommentsCmd.Flags().BoolVar(&confluenceSyncCommentsJSON, "json", false, "Output as JSON")
	confluenceUnpublishCmd.Flags().BoolVar(&confluenceUnpublishJSON, "json", false, "Output as JSON")

	confluenceCmd.AddCommand(confluencePublishCmd)
	confluenceCmd.AddCommand(confluenceStatusCmd)
	confluenceCmd.AddCommand(confluenceSyncCommentsCmd)
	confluenceCmd.AddCommand(confluenceUnpublishCmd)

	rootCmd.AddCommand(confluenceCmd)
}

func publishClient() (specgraphv1connect.PublishServiceClient, error) {
	return newClient(specgraphv1connect.NewPublishServiceClient)
}

func runConfluencePublish(cmd *cobra.Command, args []string) error {
	client, err := publishClient()
	if err != nil {
		return err
	}
	resp, err := client.Publish(cmd.Context(), connect.NewRequest(&specv1.PublishRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	if confluencePublishJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Published %d pages for %s\n", len(resp.Msg.GetMappings()), args[0]) //nolint:errcheck // stdout write
	for _, m := range resp.Msg.GetMappings() {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s: page %s (v%d)\n", m.GetDocKind(), m.GetPageId(), m.GetPageVersion()) //nolint:errcheck // stdout write
	}
	return nil
}

func runConfluenceStatus(cmd *cobra.Command, args []string) error {
	client, err := publishClient()
	if err != nil {
		return err
	}
	slug := ""
	if len(args) > 0 {
		slug = args[0]
	}
	resp, err := client.GetPublishStatus(cmd.Context(), connect.NewRequest(&specv1.GetPublishStatusRequest{
		Slug: slug,
	}))
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if confluenceStatusJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Fprint(cmd.OutOrStdout(), renderPublishStatus(resp.Msg.GetEntries())) //nolint:errcheck // stdout write
	return nil
}

func runConfluenceSyncComments(cmd *cobra.Command, args []string) error {
	client, err := publishClient()
	if err != nil {
		return err
	}
	slug := ""
	if len(args) > 0 {
		slug = args[0]
	}
	resp, err := client.SyncComments(cmd.Context(), connect.NewRequest(&specv1.SyncCommentsRequest{
		Slug: slug,
	}))
	if err != nil {
		return fmt.Errorf("sync comments: %w", err)
	}
	if confluenceSyncCommentsJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Synced %d new comments\n", resp.Msg.GetNewCount()) //nolint:errcheck // stdout write
	return nil
}

func runConfluenceUnpublish(cmd *cobra.Command, args []string) error {
	client, err := publishClient()
	if err != nil {
		return err
	}
	resp, err := client.Unpublish(cmd.Context(), connect.NewRequest(&specv1.UnpublishRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("unpublish: %w", err)
	}
	if confluenceUnpublishJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed %d pages for %s\n", resp.Msg.GetPagesRemoved(), args[0]) //nolint:errcheck // stdout write
	return nil
}

func renderPublishStatus(entries []*specv1.PublishStatusEntry) string {
	if len(entries) == 0 {
		return "No published specs.\n"
	}
	headers := []string{"Slug", "PRD", "SDD", "ADRs", "Last Sync", "Comments"}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		prd := "-"
		if e.GetPrd() != nil {
			prd = publishStateString(e.GetPrd().GetState())
		}
		sdd := "-"
		if e.GetSdd() != nil {
			sdd = publishStateString(e.GetSdd().GetState())
		}
		adrs := fmt.Sprintf("%d", len(e.GetAdrs()))
		lastSync := "-"
		if e.GetLastSync() != nil {
			lastSync = e.GetLastSync().AsTime().Format("2006-01-02 15:04")
		}
		comments := fmt.Sprintf("%d new", e.GetNewComments())
		rows[i] = []string{e.GetSpecSlug(), prd, sdd, adrs, lastSync, comments}
	}
	return markdown.ItemTable(headers, rows)
}

func publishStateString(s specv1.PublishState) string {
	switch s {
	case specv1.PublishState_PUBLISH_STATE_SYNCED:
		return "synced"
	case specv1.PublishState_PUBLISH_STATE_DRAFT:
		return "draft"
	case specv1.PublishState_PUBLISH_STATE_ERROR:
		return "error"
	case specv1.PublishState_PUBLISH_STATE_UNPUBLISHED:
		return "unpublished"
	default:
		return "-"
	}
}
