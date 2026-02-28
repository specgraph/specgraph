package main

import (
	"context"
	"fmt"
	"net/http"
	"text/tabwriter"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/config"
	"github.com/spf13/cobra"
)

func specClient() (specgraphv1connect.SpecServiceClient, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, err
	}
	baseURL := cfg.Server.Remote
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	}
	return specgraphv1connect.NewSpecServiceClient(http.DefaultClient, baseURL), nil
}

// --- create ---

var createCmd = &cobra.Command{
	Use:   "create <slug>",
	Short: "Create a new spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runCreate,
}

var (
	createIntent   string
	createPriority string
)

func init() {
	createCmd.Flags().StringVar(&createIntent, "intent", "", "intent for the spec (required)")
	createCmd.Flags().StringVar(&createPriority, "priority", "p2", "priority (p0-p3)")
	createCmd.MarkFlagRequired("intent")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.CreateSpec(context.Background(), connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:     args[0],
		Intent:   createIntent,
		Priority: createPriority,
	}))
	if err != nil {
		return fmt.Errorf("create spec: %w", err)
	}
	fmt.Printf("Created: %s (%s)\n", resp.Msg.Slug, resp.Msg.Id)
	return nil
}

// --- list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List specs",
	RunE:  runList,
}

var (
	listStage    string
	listPriority string
)

func init() {
	listCmd.Flags().StringVar(&listStage, "stage", "", "filter by stage")
	listCmd.Flags().StringVar(&listPriority, "priority", "", "filter by priority")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.ListSpecs(context.Background(), connect.NewRequest(&specv1.ListSpecsRequest{
		Stage:    listStage,
		Priority: listPriority,
	}))
	if err != nil {
		return fmt.Errorf("list specs: %w", err)
	}
	specs := resp.Msg.Specs
	if len(specs) == 0 {
		fmt.Println("No specs found.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPRIORITY\tSTAGE\tSLUG")
	for _, s := range specs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Id, s.Priority, s.Stage, s.Slug)
	}
	return w.Flush()
}

// --- show ---

var showCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: "Show spec details",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.GetSpec(context.Background(), connect.NewRequest(&specv1.GetSpecRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("get spec: %w", err)
	}
	s := resp.Msg
	fmt.Printf("ID:         %s\n", s.Id)
	fmt.Printf("Slug:       %s\n", s.Slug)
	fmt.Printf("Intent:     %s\n", s.Intent)
	fmt.Printf("Stage:      %s\n", s.Stage)
	fmt.Printf("Priority:   %s\n", s.Priority)
	fmt.Printf("Complexity: %s\n", s.Complexity)
	fmt.Printf("Version:    %d\n", s.Version)
	return nil
}
