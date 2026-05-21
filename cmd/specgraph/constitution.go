// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/constitution/load"
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

var constitutionShowLayer string

func runConstitutionShow(cmd *cobra.Command, _ []string) error {
	client, err := constitutionClient()
	if err != nil {
		return err
	}
	req := &specv1.GetConstitutionRequest{}
	if constitutionShowLayer != "" {
		resolved := constitutionLayerStringToProto(constitutionShowLayer)
		if resolved == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
			return fmt.Errorf("invalid layer %q; must be user, org, project, or domain", constitutionShowLayer)
		}
		req.Layer = resolved
	}
	resp, err := client.GetConstitution(cmd.Context(), connect.NewRequest(req))
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

var importLayerFlag string

var importFromURLFlag string

func runConstitutionImport(cmd *cobra.Command, args []string) error {
	if importFromURLFlag != "" {
		if len(args) > 0 {
			return fmt.Errorf("cannot specify both <path> argument and --from-url flag")
		}
		if importLayerFlag == "" {
			return fmt.Errorf("--layer is required when using --from-url")
		}
		return runImportFromURL(cmd, importFromURLFlag, importLayerFlag)
	}

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

	c, err := load.FromYAML(data)
	if err != nil {
		return fmt.Errorf("parse constitution: %w", err)
	}

	pb := load.ToProto(c)

	// Layer resolution: --layer flag > YAML layer field > default "project".
	if importLayerFlag != "" {
		resolved := constitutionLayerStringToProto(importLayerFlag)
		if resolved == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
			return fmt.Errorf("invalid layer %q; must be user, org, project, or domain", importLayerFlag)
		}
		pb.Layer = resolved
	}
	if pb.Layer == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
		pb.Layer = specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT
	}

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

func runImportFromURL(cmd *cobra.Command, sourceURL, layerStr string) error {
	layer := constitutionLayerStringToProto(layerStr)
	if layer == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
		return fmt.Errorf("invalid layer %q; must be user, org, project, or domain", layerStr)
	}

	var (
		client specgraphv1connect.ConstitutionServiceClient
		err    error
	)
	if importProjectSlug != "" {
		client, err = constitutionClientWithProject(importProjectSlug)
	} else {
		client, err = constitutionClient()
	}
	if err != nil {
		return err
	}

	resp, err := client.RefreshConstitutionLayer(cmd.Context(), connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
		Layer:     layer,
		SourceUrl: sourceURL,
	}))
	if err != nil {
		return fmt.Errorf("refresh constitution layer: %w", err)
	}

	sha := resp.Msg.GetNewSourceHash()
	if len(sha) > 12 {
		sha = sha[:12] + "..."
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Imported as '%s' layer (sha: %s)\n", layerStr, sha) //nolint:errcheck // stdout write
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

func constitutionLayerProtoToString(l specv1.ConstitutionLayer) string {
	switch l {
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER:
		return "user"
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG:
		return "org"
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT:
		return "project"
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN:
		return "domain"
	default:
		return "unspecified"
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

var (
	syncLayerFlag string
	syncDryRun    bool
	syncCheck     bool
)

var constitutionSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Re-fetch remote constitution layers and detect drift",
	Long: `Re-fetch each constitution layer that has a configured source_url
and detect drift via content hash comparison.

By default exits 0 regardless of drift. Use --check to exit 1 when drift
is detected (useful in CI). Always exits 2 if any fetch fails.

Layers without a configured source_url are reported but not synced.`,
	RunE: runConstitutionSync,
}

// syncLayerInfo holds per-layer metadata used during the sync pass.
type syncLayerInfo struct {
	name       string
	layerProto specv1.ConstitutionLayer
	sourceURL  string
}

func runConstitutionSync(cmd *cobra.Command, _ []string) error {
	client, err := constitutionClient()
	if err != nil {
		return err
	}

	// 1. Discover layers with source_url.
	layers, err := listLayersWithSource(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list layers: %w", err)
	}

	// 2. Filter to --layer if set.
	if syncLayerFlag != "" {
		target := constitutionLayerStringToProto(syncLayerFlag)
		if target == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
			return fmt.Errorf("invalid layer %q", syncLayerFlag)
		}
		var filtered []syncLayerInfo
		for _, li := range layers {
			if li.layerProto == target {
				filtered = append(filtered, li)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("layer %s has no source_url; nothing to sync", syncLayerFlag)
		}
		layers = filtered
	}

	if len(layers) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no remote layers configured; nothing to sync") //nolint:errcheck // stdout write
		return nil
	}

	// 3. Refresh each layer.
	var driftCount, updateCount, failCount int
	for _, li := range layers {
		resp, refreshErr := client.RefreshConstitutionLayer(cmd.Context(), connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     li.layerProto,
			SourceUrl: li.sourceURL,
			DryRun:    syncDryRun,
		}))
		if refreshErr != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s  error           %v\n", li.name, refreshErr) //nolint:errcheck // stdout write
			failCount++
			continue
		}
		if !resp.Msg.GetChanged() {
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s  unchanged       (sha %s)\n", //nolint:errcheck // stdout write
				li.name, shortHash(resp.Msg.GetNewSourceHash()))
			continue
		}
		driftCount++
		prefix := "changed         "
		if syncDryRun {
			prefix = "would-change    "
		} else {
			updateCount++
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-8s  %s(sha %s -> %s)\n", //nolint:errcheck // stdout write
			li.name, prefix,
			shortHash(resp.Msg.GetPreviousSourceHash()),
			shortHash(resp.Msg.GetNewSourceHash()))
	}

	fmt.Fprintf(cmd.OutOrStdout(), //nolint:errcheck // stdout write
		"\n%d of %d remote layers checked, %d updated.\n",
		len(layers), len(layers), updateCount)

	// 4. Exit codes.
	if failCount > 0 {
		os.Exit(2) //nolint:nolintlint // intentional early exit for CI integration; exit code 2 on fetch failure
	}
	if syncCheck && driftCount > 0 {
		os.Exit(1) //nolint:nolintlint // intentional early exit for CI integration; --check semantics
	}
	return nil
}

// listLayersWithSource queries each known layer and returns those with a
// non-empty source_url. Returns layers in canonical precedence order
// (user, org, project, domain).
func listLayersWithSource(ctx context.Context, client specgraphv1connect.ConstitutionServiceClient) ([]syncLayerInfo, error) {
	var result []syncLayerInfo
	for _, l := range []specv1.ConstitutionLayer{
		specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER,
		specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
		specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN,
	} {
		resp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{Layer: l}))
		if err != nil {
			if connect.CodeOf(err) == connect.CodeNotFound {
				continue
			}
			return nil, fmt.Errorf("get layer %s: %w", constitutionLayerProtoToString(l), err)
		}
		c := resp.Msg.GetConstitution()
		if c != nil && c.GetSourceUrl() != "" {
			result = append(result, syncLayerInfo{
				name:       constitutionLayerProtoToString(l),
				layerProto: l,
				sourceURL:  c.GetSourceUrl(),
			})
		}
	}
	return result, nil
}

// shortHash returns the first 8 chars of a hash for display purposes,
// suffixed with "..." if truncated.
func shortHash(h string) string {
	if len(h) <= 8 {
		return h
	}
	return h[:8] + "..."
}

func init() {
	constitutionEmitCmd.Flags().StringVar(&emitFormat, "format", "claude-md", "output format (e.g. claude-md)")
	constitutionEmitCmd.Flags().StringVarP(&emitOutput, "output", "o", "", "write output to file instead of stdout")

	constitutionImportCmd.Flags().StringVar(&importProjectSlug, "project", "", "project slug (defaults to slug from .specgraph.yaml)")
	constitutionImportCmd.Flags().StringVar(&importLayerFlag, "layer", "", "constitution layer (user|org|project|domain; default: project)")
	constitutionImportCmd.Flags().StringVar(&importFromURLFlag, "from-url", "", "fetch constitution from URL (alternative to local file argument)")

	constitutionSyncCmd.Flags().StringVar(&syncLayerFlag, "layer", "",
		"sync only this layer (default: all layers with source_url)")
	constitutionSyncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false,
		"fetch and compare but do not write")
	constitutionSyncCmd.Flags().BoolVar(&syncCheck, "check", false,
		"exit 1 if drift detected; useful in CI")

	constitutionShowCmd.Flags().BoolVar(&constitutionShowJSON, "json", false, "output as JSON")
	constitutionShowCmd.Flags().StringVar(&constitutionShowLayer, "layer", "", "show specific layer (user|org|project|domain; default: merged)")
	constitutionCmd.AddCommand(constitutionShowCmd)
	constitutionCmd.AddCommand(constitutionEmitCmd)
	constitutionCmd.AddCommand(constitutionImportCmd)
	constitutionCmd.AddCommand(constitutionSyncCmd)

	rootCmd.AddCommand(constitutionCmd)
}
