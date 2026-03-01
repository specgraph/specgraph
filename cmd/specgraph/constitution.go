// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func constitutionClient() (specgraphv1connect.ConstitutionServiceClient, error) {
	return newClient(specgraphv1connect.NewConstitutionServiceClient)
}

var constitutionCmd = &cobra.Command{
	Use:   "constitution",
	Short: "Manage the project constitution",
}

var constitutionShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the project constitution",
	RunE:  runConstitutionShow,
}

func runConstitutionShow(_ *cobra.Command, _ []string) error {
	client, err := constitutionClient()
	if err != nil {
		return err
	}
	resp, err := client.GetConstitution(context.Background(), connect.NewRequest(&specv1.GetConstitutionRequest{}))
	if err != nil {
		return fmt.Errorf("get constitution: %w", err)
	}
	c := resp.Msg.Constitution
	if c == nil {
		fmt.Println("No constitution found.")
		return nil
	}
	fmt.Printf("Name:    %s\n", c.GetName())
	fmt.Printf("Layer:   %s\n", c.GetLayer())
	fmt.Printf("Version: %d\n", c.GetVersion())
	if tech := c.GetTech(); tech != nil {
		if langs := tech.GetLanguages(); langs != nil {
			fmt.Printf("Tech:    primary=%s\n", langs.GetPrimary())
		}
	}
	if principles := c.GetPrinciples(); len(principles) > 0 {
		fmt.Println("Principles:")
		for _, p := range principles {
			fmt.Printf("  - %s\n", p.GetStatement())
		}
	}
	if constraints := c.GetConstraints(); len(constraints) > 0 {
		fmt.Println("Constraints:")
		for _, ct := range constraints {
			fmt.Printf("  - %s\n", ct)
		}
	}
	if antipatterns := c.GetAntipatterns(); len(antipatterns) > 0 {
		fmt.Println("Antipatterns:")
		for _, ap := range antipatterns {
			fmt.Printf("  - %s: %s\n", ap.GetPattern(), ap.GetWhy())
		}
	}
	return nil
}

var constitutionCheckCmd = &cobra.Command{
	Use:   "check <slug>",
	Short: "Check a spec for constitution violations",
	Args:  cobra.ExactArgs(1),
	RunE:  runConstitutionCheck,
}

func runConstitutionCheck(_ *cobra.Command, args []string) error {
	client, err := constitutionClient()
	if err != nil {
		return err
	}
	resp, err := client.CheckViolation(context.Background(), connect.NewRequest(&specv1.CheckViolationRequest{
		SpecSlug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("check violation: %w", err)
	}
	violations := resp.Msg.GetViolations()
	if len(violations) == 0 {
		fmt.Println("No violations found.")
		return nil
	}
	for _, v := range violations {
		fmt.Printf("[%s] %s: %s\n", v.GetSeverity(), v.GetRule(), v.GetMessage())
	}
	return nil
}

var constitutionEmitCmd = &cobra.Command{
	Use:   "emit",
	Short: "Emit constitution as tool files",
	RunE:  runConstitutionEmit,
}

var (
	emitFormat string
	emitOutput string
)

// outputFormatFromString converts a CLI flag string to the OutputFormat enum.
var outputFormatMap = map[string]specv1.OutputFormat{
	"claude-md":   specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD,
	"cursorrules": specv1.OutputFormat_OUTPUT_FORMAT_CURSORRULES,
	"agents-md":   specv1.OutputFormat_OUTPUT_FORMAT_AGENTS_MD,
}

func runConstitutionEmit(_ *cobra.Command, _ []string) error {
	format, ok := outputFormatMap[emitFormat]
	if !ok {
		return fmt.Errorf("unsupported format %q; valid values: claude-md, cursorrules, agents-md", emitFormat)
	}
	client, err := constitutionClient()
	if err != nil {
		return err
	}
	resp, err := client.EmitToolFiles(context.Background(), connect.NewRequest(&specv1.EmitToolFilesRequest{
		Format: format,
	}))
	if err != nil {
		return fmt.Errorf("emit tool files: %w", err)
	}
	content := resp.Msg.Content
	filename := resp.Msg.Filename
	if emitOutput != "" {
		absPath, err := filepath.Abs(emitOutput)
		if err != nil {
			return fmt.Errorf("resolve output path: %w", err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		if !strings.HasPrefix(absPath, cwd) {
			fmt.Fprintf(os.Stderr, "warning: output path %s is outside the current directory\n", absPath)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Printf("Written to %s\n", absPath)
		return nil
	}
	if filename != "" {
		fmt.Printf("# %s\n", filename)
	}
	fmt.Print(content)
	return nil
}

func init() {
	constitutionEmitCmd.Flags().StringVar(&emitFormat, "format", "claude-md", "output format (e.g. claude-md)")
	constitutionEmitCmd.Flags().StringVarP(&emitOutput, "output", "o", "", "write output to file instead of stdout")

	constitutionCmd.AddCommand(constitutionShowCmd)
	constitutionCmd.AddCommand(constitutionCheckCmd)
	constitutionCmd.AddCommand(constitutionEmitCmd)

	rootCmd.AddCommand(constitutionCmd)
}
