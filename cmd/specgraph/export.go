// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"io"
	"os"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func exportClient() (specgraphv1connect.ExportServiceClient, error) {
	return newClient(specgraphv1connect.NewExportServiceClient)
}

// fprintf writes to w, discarding the byte count. Write errors to stdout/stderr are not actionable in a CLI.
func fprintf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, format, a...) //nolint:errcheck // write errors to stdout/stderr are not actionable
}

// fprintln writes to w, discarding the byte count. Write errors to stdout/stderr are not actionable in a CLI.
func fprintln(w io.Writer, a ...any) {
	fmt.Fprintln(w, a...) //nolint:errcheck // write errors to stdout/stderr are not actionable
}

// --- export ---

var exportCmd = &cobra.Command{
	Use:   "export <project-slug>",
	Short: "Export a project to a JSON backup file",
	Args:  cobra.ExactArgs(1),
	RunE:  runExport,
}

var exportOutput string

func runExport(cmd *cobra.Command, args []string) error {
	client, err := exportClient()
	if err != nil {
		return err
	}
	resp, err := client.ExportProject(cmd.Context(), connect.NewRequest(&specv1.ExportProjectRequest{
		ProjectSlug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	data := resp.Msg.GetData()
	if exportOutput != "" {
		if writeErr := os.WriteFile(exportOutput, data, 0o644); writeErr != nil { //nolint:gosec // export artifact, world-readable
			return fmt.Errorf("write output file: %w", writeErr)
		}
		fprintf(cmd.ErrOrStderr(), "Exported to %s (%d bytes)\n", exportOutput, len(data))
		return nil
	}

	_, err = cmd.OutOrStdout().Write(data)
	return err
}

// --- import ---

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a project from a JSON backup file",
	Args:  cobra.ExactArgs(1),
	RunE:  runImport,
}

var (
	importForce            bool
	importRequireSignature bool
)

func runImport(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}

	client, err := exportClient()
	if err != nil {
		return err
	}
	resp, err := client.ImportProject(cmd.Context(), connect.NewRequest(&specv1.ImportProjectRequest{
		Data:             data,
		Force:            importForce,
		RequireSignature: importRequireSignature,
	}))
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	r := resp.Msg.GetResult()
	fprintf(cmd.OutOrStdout(), "Import complete:\n")
	fprintf(cmd.OutOrStdout(), "  Specs:            %d\n", r.GetSpecsCreated())
	fprintf(cmd.OutOrStdout(), "  Decisions:        %d\n", r.GetDecisionsCreated())
	fprintf(cmd.OutOrStdout(), "  Slices:           %d\n", r.GetSlicesCreated())
	fprintf(cmd.OutOrStdout(), "  Edges:            %d\n", r.GetEdgesCreated())
	fprintf(cmd.OutOrStdout(), "  Findings:         %d\n", r.GetFindingsCreated())
	fprintf(cmd.OutOrStdout(), "  ChangeLogs:       %d\n", r.GetChangelogsCreated())
	fprintf(cmd.OutOrStdout(), "  Conversations:    %d\n", r.GetConversationsCreated())
	fprintf(cmd.OutOrStdout(), "  Sync Mappings:    %d\n", r.GetSyncMappingsCreated())
	fprintf(cmd.OutOrStdout(), "  Execution Events: %d\n", r.GetExecutionEventsCreated())

	for _, w := range r.GetWarnings() {
		fprintf(cmd.ErrOrStderr(), "  warning: %s\n", w)
	}
	return nil
}

// --- verify ---

var verifyCmd = &cobra.Command{
	Use:   "verify <file>",
	Short: "Verify an export matches current server state",
	Args:  cobra.ExactArgs(1),
	RunE:  runVerify,
}

func runVerify(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read verify file: %w", err)
	}

	client, err := exportClient()
	if err != nil {
		return err
	}
	resp, err := client.VerifyExport(cmd.Context(), connect.NewRequest(&specv1.VerifyExportRequest{
		Data: data,
	}))
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	if resp.Msg.GetMatch() {
		fprintln(cmd.OutOrStdout(), "Verification passed: export matches current state.")
		return nil
	}

	fprintln(cmd.OutOrStdout(), "Verification failed: differences found.")
	for _, d := range resp.Msg.GetDiffs() {
		if d.GetMissing() > 0 || d.GetExtra() > 0 {
			fprintf(cmd.OutOrStdout(), "  %-20s matched=%d missing=%d extra=%d\n",
				d.GetEntityType(), d.GetMatched(), d.GetMissing(), d.GetExtra())
		}
	}
	return fmt.Errorf("verification failed")
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file path (default: stdout)")

	importCmd.Flags().BoolVar(&importForce, "force", false, "overwrite existing project data")
	importCmd.Flags().BoolVar(&importRequireSignature, "require-signature", false, "require a valid HMAC signature")

	rootCmd.AddCommand(exportCmd, importCmd, verifyCmd)
}
