// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/config/managedfiles"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project-slug]",
	Short: "Initialize a SpecGraph project in the current directory",
	Long: "Writes .specgraph.yaml and the per-harness managed files " +
		"(.cursor/mcp.json, .mcp.json, opencode.json, AGENTS.md, " +
		".cursor/rules/specgraph-bootstrap.mdc) for the current project. " +
		"Idempotent: safe to re-run on an already-initialized project. " +
		"JSON managed keys are reset to canonical values on every run; " +
		"user-added sibling keys are preserved. Markdown managed blocks " +
		"(AGENTS.md, .mdc) are rewritten only when canonical or stale — " +
		"user-edited (drifted) blocks are SKIPPED to preserve hand edits. " +
		"runInit calls SyncAll with zero-value SyncOptions, so there is no " +
		"--force path in this command; use `specgraph doctor --fix` " +
		"to overwrite drifted blocks.",
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

var (
	initYes   bool
	initCheck bool
	initQuiet bool
)

func init() {
	initCmd.Flags().BoolVar(&initYes, "yes", false, "non-interactive (accepted for backward compat; init is always non-interactive)")
	initCmd.Flags().BoolVar(&initCheck, "check", false, "Exit non-zero if any managed file would be modified (no writes)")
	initCmd.Flags().BoolVar(&initQuiet, "quiet", false, "Suppress per-file action lines")
	rootCmd.AddCommand(initCmd)
}

func runInit(_ *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	var argSlug string
	if len(args) > 0 {
		argSlug = args[0]
	}

	// Resolve project state: load existing .specgraph.yaml if present.
	var existing *config.ProjectConfig
	if root, findErr := config.FindProjectRoot(cwd); findErr == nil {
		loaded, loadErr := config.LoadProject(root)
		if loadErr != nil {
			return fmt.Errorf("load existing project config: %w", loadErr)
		}
		existing = loaded
		cwd = root
	} else if !errors.Is(findErr, config.ErrProjectNotFound) {
		return fmt.Errorf("find project root: %w", findErr)
	}

	// Slug-consistency check: if both an arg and an existing config are
	// present and the slugs differ, refuse. The slug is identity-defining
	// (storage partition key, X-Specgraph-Project header value) and silent
	// mutation would orphan project data.
	if argSlug != "" && existing != nil && argSlug != existing.Slug {
		return fmt.Errorf(
			"cannot change project slug from %q to %q; edit .specgraph.yaml directly or remove it",
			existing.Slug, argSlug,
		)
	}

	// Determine the slug for this run.
	var pc *config.ProjectConfig
	switch {
	case existing != nil:
		pc = existing
	case argSlug != "":
		pc = &config.ProjectConfig{Slug: argSlug}
	default:
		// Derive from git remote / dir name (config.LoadProject already does
		// this when no .specgraph.yaml exists).
		derived, derErr := config.LoadProject(cwd)
		if derErr != nil {
			return fmt.Errorf("derive project slug: %w", derErr)
		}
		pc = &config.ProjectConfig{Slug: derived.Slug}
	}

	// Reject malformed/relative server URLs before any writes. url.Parse
	// is lenient — bare "/api", "example.com", and "localhost:3000" all
	// parse — so NewOptions requires Scheme ∈ {http,https} AND non-empty Host.
	globalCfg, err := loadGlobalCfg()
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}
	serverURL := globalCfg.ResolveServer(pc.Slug, pc.Server)
	params := managedfiles.ProjectParams{Slug: pc.Slug, ServerURL: serverURL}
	if err := params.Validate(); err != nil {
		return fmt.Errorf("validate project params: %w", err)
	}

	// Write .specgraph.yaml only if it doesn't exist; idempotent.
	projectCreated := false
	if existing == nil {
		if writeErr := config.WriteProject(cwd, pc); writeErr != nil {
			return fmt.Errorf("write project config: %w", writeErr)
		}
		projectCreated = true
	}

	// Read harnesses from .specgraph.yaml when present; fall back to all
	// three when the list is empty (legacy configs and no-config case).
	harnesses := harnessSliceFromConfig(pc.Harnesses)

	// --check: inspect without writing; exit non-zero if any tracked managed
	// file is not Synced. Init-only destinations that the repo's .gitignore
	// covers (harness shims under .specgraph/agents/ and the cursor .mdc rules)
	// are skipped: they're meant to materialize on a contributor's machine via
	// `specgraph init`, never to be checked into git, so reporting them as
	// "missing" on a fresh checkout would be a false positive that fails CI.
	if initCheck {
		states, err := managedfiles.InspectAll(cwd, harnesses, params)
		if err != nil {
			return fmt.Errorf("inspect for --check: %w", err)
		}
		nonSynced := 0
		checked := 0
		for _, s := range states {
			if isCheckIgnored(s.Path) {
				continue
			}
			checked++
			if s.State != managedfiles.StateSynced {
				nonSynced++
				if !initQuiet {
					fmt.Printf("%s: %s\n", s.Path, managedfiles.StateName(s.State))
				}
			}
		}
		if nonSynced > 0 {
			return fmt.Errorf("%d managed file(s) not in sync", nonSynced)
		}
		if !initQuiet {
			fmt.Printf("init --check: all %d tracked managed file(s) synced\n", checked)
		}
		return nil
	}

	results, syncErr := managedfiles.SyncAll(cwd, harnesses, params, managedfiles.SyncOptions{})
	var failedPaths []string
	for _, r := range results {
		if r.Action == managedfiles.ActionError {
			fmt.Fprintf(os.Stderr, "%s: error: %v\n", r.Path, r.Err)
			failedPaths = append(failedPaths, r.Path)
		} else if !initQuiet {
			line := fmt.Sprintf("%s: %s", r.Path, managedfiles.ActionName(r.Action))
			if r.Detail != "" {
				line += " (" + r.Detail + ")"
			}
			fmt.Println(line)
		}
	}
	if syncErr != nil {
		return fmt.Errorf("sync managed files: %w", syncErr)
	}
	if len(failedPaths) > 0 {
		return fmt.Errorf("sync managed files: %d failed: %s",
			len(failedPaths), strings.Join(failedPaths, ", "))
	}

	if projectCreated {
		fmt.Printf("Initialized project %s. Config written to .specgraph.yaml\n", pc.Slug)
	}

	return nil
}

// checkIgnoredPrefixes lists path prefixes for init-only destinations that
// the repo's .gitignore covers: harness shims under .specgraph/agents/ and
// the two cursor .mdc rules. `init --check` skips these because they're
// expected to be absent on a fresh checkout (CI, new contributor clone) and
// only materialize when the contributor runs `specgraph init`. Keep in sync
// with the relevant blocks in .gitignore.
var checkIgnoredPrefixes = []string{
	".specgraph/agents/",
	".cursor/rules/specgraph.mdc",
	".cursor/rules/specgraph-post-stage.mdc",
}

// isCheckIgnored returns true if path matches one of the .gitignore-covered
// init-only destinations enumerated in checkIgnoredPrefixes. Used by
// init --check so plugin:check (in `task check`) doesn't fail CI for files
// that aren't tracked in git.
func isCheckIgnored(path string) bool {
	for _, p := range checkIgnoredPrefixes {
		if path == p || strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// harnessSliceFromConfig maps strings from cfg.Harnesses to Harness enum
// values. Unknown names are silently dropped (doctor's Project config
// group surfaces them as drift). Empty input returns all three harnesses
// — the legacy default when no harnesses are configured.
func harnessSliceFromConfig(names []string) []managedfiles.Harness {
	if len(names) == 0 {
		return []managedfiles.Harness{
			managedfiles.HarnessClaude,
			managedfiles.HarnessCursor,
			managedfiles.HarnessOpenCode,
		}
	}
	var out []managedfiles.Harness
	for _, n := range names {
		switch n {
		case "claude":
			out = append(out, managedfiles.HarnessClaude)
		case "cursor":
			out = append(out, managedfiles.HarnessCursor)
		case "opencode":
			out = append(out, managedfiles.HarnessOpenCode)
		}
	}
	return out
}
