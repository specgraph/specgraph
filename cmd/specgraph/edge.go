// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func graphClient() (specgraphv1connect.GraphServiceClient, error) {
	baseURL, err := resolveBaseURL()
	if err != nil {
		return nil, err
	}
	return specgraphv1connect.NewGraphServiceClient(newHTTPClient(), baseURL), nil
}

var edgeTypeMap = map[string]specv1.EdgeType{
	"depends_on": specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	"blocks":     specv1.EdgeType_EDGE_TYPE_BLOCKS,
	"composes":   specv1.EdgeType_EDGE_TYPE_COMPOSES,
	"relates_to": specv1.EdgeType_EDGE_TYPE_RELATES_TO,
	"informs":    specv1.EdgeType_EDGE_TYPE_INFORMS,
}

// --- edge parent command ---

var edgeCmd = &cobra.Command{
	Use:   "edge",
	Short: "Manage graph edges",
}

func init() {
	rootCmd.AddCommand(edgeCmd)
}

// --- edge add ---

var edgeAddCmd = &cobra.Command{
	Use:   "add <from-slug> <to-slug>",
	Short: "Add an edge between two nodes",
	Args:  cobra.ExactArgs(2),
	RunE:  runEdgeAdd,
}

var edgeAddType string

func init() {
	edgeAddCmd.Flags().StringVar(&edgeAddType, "type", "depends_on", "edge type (depends_on, blocks, composes, relates_to, informs)")
	edgeCmd.AddCommand(edgeAddCmd)
}

func runEdgeAdd(_ *cobra.Command, args []string) error {
	client, err := graphClient()
	if err != nil {
		return err
	}
	et, ok := edgeTypeMap[edgeAddType]
	if !ok {
		return fmt.Errorf("unknown edge type: %s", edgeAddType)
	}

	resp, err := client.AddEdge(context.Background(), connect.NewRequest(&specv1.AddEdgeRequest{
		FromSlug: args[0],
		ToSlug:   args[1],
		EdgeType: et,
	}))
	if err != nil {
		return fmt.Errorf("add edge: %w", err)
	}
	fmt.Printf("Added %s edge: %s → %s\n", edgeAddType, resp.Msg.FromId, resp.Msg.ToId)
	return nil
}

// --- edge remove ---

var edgeRemoveCmd = &cobra.Command{
	Use:   "remove <from-slug> <to-slug>",
	Short: "Remove an edge between two nodes",
	Args:  cobra.ExactArgs(2),
	RunE:  runEdgeRemove,
}

var edgeRemoveType string

func init() {
	edgeRemoveCmd.Flags().StringVar(&edgeRemoveType, "type", "depends_on", "edge type")
	edgeCmd.AddCommand(edgeRemoveCmd)
}

func runEdgeRemove(_ *cobra.Command, args []string) error {
	client, err := graphClient()
	if err != nil {
		return err
	}
	et, ok := edgeTypeMap[edgeRemoveType]
	if !ok {
		return fmt.Errorf("unknown edge type: %s", edgeRemoveType)
	}

	_, err = client.RemoveEdge(context.Background(), connect.NewRequest(&specv1.RemoveEdgeRequest{
		FromSlug: args[0],
		ToSlug:   args[1],
		EdgeType: et,
	}))
	if err != nil {
		return fmt.Errorf("remove edge: %w", err)
	}
	fmt.Println("Edge removed.")
	return nil
}

// --- edge list ---

var edgeListCmd = &cobra.Command{
	Use:   "list <slug>",
	Short: "List edges for a node",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdgeList,
}

var edgeListType string

func init() {
	edgeListCmd.Flags().StringVar(&edgeListType, "type", "", "filter by edge type")
	edgeCmd.AddCommand(edgeListCmd)
}

func runEdgeList(cmd *cobra.Command, args []string) error {
	client, err := graphClient()
	if err != nil {
		return err
	}
	var et specv1.EdgeType
	if edgeListType != "" {
		var ok bool
		et, ok = edgeTypeMap[edgeListType]
		if !ok {
			return fmt.Errorf("unknown edge type: %s", edgeListType)
		}
	}

	resp, err := client.ListEdges(context.Background(), connect.NewRequest(&specv1.ListEdgesRequest{
		Slug:     args[0],
		EdgeType: et,
	}))
	if err != nil {
		return fmt.Errorf("list edges: %w", err)
	}
	edges := resp.Msg.Edges
	if len(edges) == 0 {
		fmt.Println("No edges found.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	tw := &tableWriter{w: w}
	tw.println("FROM\tTO\tTYPE")
	for _, e := range edges {
		tw.printf("%s\t%s\t%s\n", e.FromId, e.ToId, e.EdgeType)
	}
	if tw.err != nil {
		return tw.err
	}
	return w.Flush()
}
