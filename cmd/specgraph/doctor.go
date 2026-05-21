// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

// DoctorReport is the canonical structure all output modes (text +
// JSON) emit. Schema is stable across versions; new fields may be
// added but existing ones don't change shape.
type DoctorReport struct {
	ConfigError string        `json:"configError,omitempty"`
	Binary      BinaryReport  `json:"binary"`
	Server      ServerReport  `json:"server"`
	Project     ProjectReport `json:"project"`
	Managed     ManagedReport `json:"managed"`
}

// runDoctor is doctorCmd's RunE entry point. It builds the report,
// renders it (text or JSON), and returns the final exit code as an
// error so cobra propagates it.
func runDoctor(cmd *cobra.Command, _ []string) error {
	jsonOut, err := cmd.Flags().GetBool("json")
	if err != nil {
		return fmt.Errorf("json flag: %w", err)
	}
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return fmt.Errorf("verbose flag: %w", err)
	}
	exitZero, err := cmd.Flags().GetBool("exit-zero")
	if err != nil {
		return fmt.Errorf("exit-zero flag: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return fmt.Errorf("timeout flag: %w", err)
	}

	fix, err := cmd.Flags().GetBool("fix")
	if err != nil {
		return fmt.Errorf("fix flag: %w", err)
	}
	harnessFlag, err := cmd.Flags().GetString("harness")
	if err != nil {
		return fmt.Errorf("harness flag: %w", err)
	}

	// Load the project config so we can resolve harnesses and server
	// URL for the Managed group. LoadProject derives a slug even when
	// no .specgraph.yaml exists, so the returned *ProjectConfig is
	// non-nil on success. Treat any load error as "no project config"
	// for the purposes of the Managed group — the Project group has
	// already surfaced the underlying problem.
	pc, pcErr := config.LoadProject(cwd)
	if pcErr != nil {
		pc = &config.ProjectConfig{}
	}
	harnesses, hErr := harnessesFromFlag(pc, harnessFlag)
	if hErr != nil {
		cmd.SilenceUsage = true
		return hErr
	}

	globalCfg, gErr := loadGlobalCfg()
	var serverURL string
	if gErr == nil {
		serverURL = globalCfg.ResolveServer(pc.Slug, pc.Server)
	}
	params := managedfiles.ProjectParams{Slug: pc.Slug, ServerURL: serverURL}

	rep := DoctorReport{
		Binary:  runBinaryGroup(),
		Project: runProjectConfigGroup(cwd),
		Server:  runServerGroup(timeout),
		Managed: runManagedGroup(cwd, harnesses, params),
	}
	if gErr != nil {
		rep.ConfigError = gErr.Error()
	}

	if fix {
		if err := runDoctorFix(cwd, rep.Managed, harnesses, params); err != nil {
			return fmt.Errorf("doctor --fix: %w", err)
		}
		// Re-inspect after fix so the exit code reflects the new state.
		rep.Managed = runManagedGroup(cwd, harnesses, params)
	}

	if jsonOut {
		renderJSON(os.Stdout, &rep)
	} else {
		renderText(os.Stdout, &rep, verbose)
	}
	final := finalExitCode(&rep, exitZero)
	if final == 0 {
		return nil
	}
	// Cobra exits 0 unless RunE returns non-nil; use SilenceUsage and
	// a sentinel error to propagate the code without printing the
	// usage banner.
	cmd.SilenceUsage = true
	return fmt.Errorf("doctor: exit %d", final)
}

// ExitCode returns 1 if any group reports non-OK or if the global
// config failed to load, else 0. Computed on demand rather than stored
// so callers constructing DoctorReport literals can't get the field out
// of sync with the underlying group OKs.
func (r *DoctorReport) ExitCode() int {
	if r.ConfigError != "" {
		return 1
	}
	if !r.Binary.OK || !r.Project.OK || !r.Server.OK || !r.Managed.OK {
		return 1
	}
	return 0
}

// finalExitCode applies the --exit-zero override.
func finalExitCode(rep *DoctorReport, exitZero bool) int {
	if exitZero {
		return 0
	}
	return rep.ExitCode()
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check SpecGraph integration health (binary, server, project config, managed files)",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().Bool("json", false, "Machine-readable output (full structure, never compacted)")
	doctorCmd.Flags().Bool("fix", false, "Auto-init for Stale/Missing; print guidance for Drifted")
	doctorCmd.Flags().String("harness", "", "Narrow Managed Files group to one harness (claude | cursor | opencode)")
	doctorCmd.Flags().Bool("verbose", false, "Force per-row expansion of all four groups")
	doctorCmd.Flags().Bool("exit-zero", false, "Always exit 0 (advisory-only mode)")
	doctorCmd.Flags().Duration("timeout", 2*time.Second, "Per-RPC timeout for the Server group")
	rootCmd.AddCommand(doctorCmd)
}
