// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

func constitutionClient() (specgraphv1connect.ConstitutionServiceClient, error) {
	return newClient(specgraphv1connect.NewConstitutionServiceClient)
}

// constitutionClientWithProject creates a ConstitutionServiceClient that uses
// the given project slug for the X-Specgraph-Project header, bypassing the
// auto-derived slug from .specgraph.yaml.
func constitutionClientWithProject(project string) (specgraphv1connect.ConstitutionServiceClient, error) {
	return newClientWithProject(specgraphv1connect.NewConstitutionServiceClient, project)
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

var constitutionShowJSON bool

func runConstitutionShow(cmd *cobra.Command, _ []string) error {
	client, err := constitutionClient()
	if err != nil {
		return err
	}
	resp, err := client.GetConstitution(cmd.Context(), connect.NewRequest(&specv1.GetConstitutionRequest{}))
	if err != nil {
		return fmt.Errorf("get constitution: %w", err)
	}
	if constitutionShowJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.Constitution(resp.Msg.Constitution))
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

// outputFormatMap maps CLI flag strings to OutputFormat enum values.
var outputFormatMap = map[string]specv1.OutputFormat{
	"claude-md":   specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD,
	"cursorrules": specv1.OutputFormat_OUTPUT_FORMAT_CURSORRULES,
	"agents-md":   specv1.OutputFormat_OUTPUT_FORMAT_AGENTS_MD,
}

func runConstitutionEmit(cmd *cobra.Command, _ []string) error {
	format, ok := outputFormatMap[emitFormat]
	if !ok {
		return fmt.Errorf("unsupported format %q; valid values: claude-md, cursorrules, agents-md", emitFormat)
	}
	client, err := constitutionClient()
	if err != nil {
		return err
	}
	resp, err := client.EmitToolFiles(cmd.Context(), connect.NewRequest(&specv1.EmitToolFilesRequest{
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
		rel, err := filepath.Rel(cwd, absPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("output path %q is outside current directory %q", emitOutput, cwd)
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

var constitutionImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import a constitution from a YAML file (or stdin)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConstitutionImport,
}

var importProjectSlug string

func runConstitutionImport(cmd *cobra.Command, args []string) error {
	var data []byte
	var err error

	if len(args) > 0 {
		data, err = os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
	} else {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
	}

	cc, err := config.ParseConstitutionYAML(data)
	if err != nil {
		return fmt.Errorf("parse constitution: %w", err)
	}

	pb := constitutionConfigToProto(cc)

	var client specgraphv1connect.ConstitutionServiceClient
	if importProjectSlug != "" {
		client, err = constitutionClientWithProject(importProjectSlug)
	} else {
		client, err = constitutionClient()
	}
	if err != nil {
		return err
	}

	_, err = client.UpdateConstitution(cmd.Context(), connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: pb,
	}))
	if err != nil {
		return fmt.Errorf("update constitution: %w", err)
	}

	slug := importProjectSlug
	if slug == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			proj, lerr := config.LoadProject(cwd)
			if lerr == nil {
				slug = proj.Slug
			}
		}
	}

	fmt.Printf("Constitution imported for project %s\n", slug)
	return nil
}

// constitutionConfigToProto converts a config.ConstitutionConfig (YAML) to a proto Constitution.
func constitutionConfigToProto(cc *config.ConstitutionConfig) *specv1.Constitution {
	pb := &specv1.Constitution{
		Name:        cc.Name,
		Layer:       constitutionLayerStringToProto(cc.Layer),
		Constraints: cc.Constraints,
	}
	for _, p := range cc.Principles {
		pb.Principles = append(pb.Principles, &specv1.Principle{
			Id:         p.ID,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
	}
	for _, ap := range cc.Antipatterns {
		pb.Antipatterns = append(pb.Antipatterns, &specv1.Antipattern{
			Pattern: ap.Pattern,
			Why:     ap.Why,
			Instead: ap.Instead,
		})
	}
	for _, ref := range cc.References {
		pb.References = append(pb.References, &specv1.Reference{
			ReferenceType: constitutionRefTypeToProto(ref.Type),
			Path:          ref.Path,
		})
	}
	if t := &cc.Tech; t.Languages.Primary != "" || len(t.Languages.Allowed) > 0 || len(t.Languages.Forbidden) > 0 ||
		len(t.Frameworks) > 0 || len(t.Infrastructure) > 0 || len(t.APIStandards) > 0 || len(t.Data) > 0 {
		tc := &specv1.TechConfig{
			Frameworks:     t.Frameworks,
			Infrastructure: t.Infrastructure,
			ApiStandards:   t.APIStandards,
			Data:           t.Data,
		}
		if t.Languages.Primary != "" || len(t.Languages.Allowed) > 0 || len(t.Languages.Forbidden) > 0 {
			tc.Languages = &specv1.LanguageConfig{
				Primary:          t.Languages.Primary,
				Allowed:          t.Languages.Allowed,
				Forbidden:        t.Languages.Forbidden,
				ForbiddenReasons: t.Languages.ForbiddenReasons,
			}
		}
		pb.Tech = tc
	}
	return pb
}

func constitutionLayerStringToProto(layer string) specv1.ConstitutionLayer {
	switch strings.ToLower(layer) {
	case "user":
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER
	case "org":
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG
	case "project":
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT
	case "domain":
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN
	default:
		return specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED
	}
}

func constitutionRefTypeToProto(t string) specv1.ReferenceType {
	switch strings.ToLower(t) {
	case "adr":
		return specv1.ReferenceType_REFERENCE_TYPE_ADR
	case "spec":
		return specv1.ReferenceType_REFERENCE_TYPE_SPEC
	case "doc":
		return specv1.ReferenceType_REFERENCE_TYPE_DOC
	case "url":
		return specv1.ReferenceType_REFERENCE_TYPE_URL
	default:
		return specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED
	}
}

func init() {
	constitutionEmitCmd.Flags().StringVar(&emitFormat, "format", "claude-md", "output format (e.g. claude-md)")
	constitutionEmitCmd.Flags().StringVarP(&emitOutput, "output", "o", "", "write output to file instead of stdout")

	constitutionImportCmd.Flags().StringVar(&importProjectSlug, "project", "", "project slug (defaults to slug from .specgraph.yaml)")

	constitutionShowCmd.Flags().BoolVar(&constitutionShowJSON, "json", false, "output as JSON")
	constitutionCmd.AddCommand(constitutionShowCmd)
	constitutionCmd.AddCommand(constitutionEmitCmd)
	constitutionCmd.AddCommand(constitutionImportCmd)

	rootCmd.AddCommand(constitutionCmd)
}
