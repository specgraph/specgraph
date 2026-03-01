// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/durationpb"
)

func claimClient() (specgraphv1connect.ClaimServiceClient, error) {
	baseURL, err := resolveBaseURL()
	if err != nil {
		return nil, err
	}
	return specgraphv1connect.NewClaimServiceClient(newHTTPClient(), baseURL), nil
}

// --- claim ---

var claimCmd = &cobra.Command{
	Use:   "claim <slug>",
	Short: "Claim a spec for an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runClaim,
}

var (
	claimAgent    string
	claimDuration time.Duration
)

func init() {
	claimCmd.Flags().StringVar(&claimAgent, "agent", "", "agent identifier (required)")
	claimCmd.Flags().DurationVar(&claimDuration, "duration", 15*time.Minute, "lease duration")
	cobra.CheckErr(claimCmd.MarkFlagRequired("agent"))
	rootCmd.AddCommand(claimCmd)
}

func runClaim(_ *cobra.Command, args []string) error {
	client, err := claimClient()
	if err != nil {
		return err
	}
	resp, err := client.ClaimSpec(context.Background(), connect.NewRequest(&specv1.ClaimSpecRequest{
		SpecSlug:      args[0],
		Agent:         claimAgent,
		LeaseDuration: durationpb.New(claimDuration),
	}))
	if err != nil {
		return fmt.Errorf("claim spec: %w", err)
	}
	fmt.Printf("Claimed: %s by %s (expires %s)\n",
		resp.Msg.SpecSlug, resp.Msg.Agent,
		resp.Msg.LeaseExpires.AsTime().Format(time.RFC3339))
	return nil
}

// --- unclaim ---

var unclaimCmd = &cobra.Command{
	Use:   "unclaim <slug>",
	Short: "Release a claim on a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runUnclaim,
}

var unclaimAgent string

func init() {
	unclaimCmd.Flags().StringVar(&unclaimAgent, "agent", "", "agent identifier (required)")
	cobra.CheckErr(unclaimCmd.MarkFlagRequired("agent"))
	rootCmd.AddCommand(unclaimCmd)
}

func runUnclaim(_ *cobra.Command, args []string) error {
	client, err := claimClient()
	if err != nil {
		return err
	}
	_, err = client.UnclaimSpec(context.Background(), connect.NewRequest(&specv1.UnclaimSpecRequest{
		SpecSlug: args[0],
		Agent:    unclaimAgent,
	}))
	if err != nil {
		return fmt.Errorf("unclaim spec: %w", err)
	}
	fmt.Println("Claim released.")
	return nil
}
