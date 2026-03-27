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

func sliceClient() (specgraphv1connect.SliceServiceClient, error) {
	return newClient(specgraphv1connect.NewSliceServiceClient)
}

// --- slice parent command ---

var sliceCmd = &cobra.Command{
	Use:   "slice",
	Short: "Manage decomposition slices",
}

// --- slice list ---

var sliceListCmd = &cobra.Command{
	Use:   "list <parent-slug>",
	Short: "List slices for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runSliceList,
}

var sliceListJSON bool

func runSliceList(cmd *cobra.Command, args []string) error {
	client, err := sliceClient()
	if err != nil {
		return err
	}
	resp, err := client.ListSlices(context.Background(), connect.NewRequest(&specv1.ListSlicesRequest{
		ParentSlug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("list slices: %w", err)
	}
	if sliceListJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.SliceList(resp.Msg.Slices))
	return nil
}

// --- slice get ---

var sliceGetCmd = &cobra.Command{
	Use:   "get <slug>",
	Short: "Show slice details",
	Args:  cobra.ExactArgs(1),
	RunE:  runSliceGet,
}

var sliceGetJSON bool

func runSliceGet(cmd *cobra.Command, args []string) error {
	client, err := sliceClient()
	if err != nil {
		return err
	}
	resp, err := client.GetSlice(context.Background(), connect.NewRequest(&specv1.GetSliceRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("get slice: %w", err)
	}
	if sliceGetJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.SliceDetail(resp.Msg.Slice))
	return nil
}

// --- slice claim ---

var sliceClaimCmd = &cobra.Command{
	Use:   "claim <slug>",
	Short: "Claim a slice for work",
	Args:  cobra.ExactArgs(1),
	RunE:  runSliceClaim,
}

var sliceClaimAssignee string

func runSliceClaim(_ *cobra.Command, args []string) error {
	client, err := sliceClient()
	if err != nil {
		return err
	}
	resp, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug:     args[0],
		Assignee: sliceClaimAssignee,
	}))
	if err != nil {
		return fmt.Errorf("claim slice: %w", err)
	}
	fmt.Printf("Claimed: %s by %s\n", resp.Msg.GetSlice().GetSlug(), resp.Msg.GetSlice().GetAssignedTo())
	return nil
}

// --- slice complete ---

var sliceCompleteCmd = &cobra.Command{
	Use:   "complete <slug>",
	Short: "Mark a slice as done",
	Args:  cobra.ExactArgs(1),
	RunE:  runSliceComplete,
}

func runSliceComplete(_ *cobra.Command, args []string) error {
	client, err := sliceClient()
	if err != nil {
		return err
	}
	resp, err := client.CompleteSlice(context.Background(), connect.NewRequest(&specv1.CompleteSliceRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("complete slice: %w", err)
	}
	fmt.Printf("Completed: %s\n", resp.Msg.GetSlice().GetSlug())
	return nil
}

// --- registration ---

func init() {
	rootCmd.AddCommand(sliceCmd)

	sliceListCmd.Flags().BoolVar(&sliceListJSON, "json", false, "output as JSON")
	sliceCmd.AddCommand(sliceListCmd)

	sliceGetCmd.Flags().BoolVar(&sliceGetJSON, "json", false, "output as JSON")
	sliceCmd.AddCommand(sliceGetCmd)

	sliceClaimCmd.Flags().StringVar(&sliceClaimAssignee, "assignee", "", "who is claiming (required)")
	cobra.CheckErr(sliceClaimCmd.MarkFlagRequired("assignee"))
	sliceCmd.AddCommand(sliceClaimCmd)

	sliceCmd.AddCommand(sliceCompleteCmd)
}
