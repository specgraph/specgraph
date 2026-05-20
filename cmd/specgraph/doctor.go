// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// DoctorReport is the canonical structure all output modes (text +
// JSON) emit. Schema is stable across versions; new fields may be
// added but existing ones don't change shape.
type DoctorReport struct {
	ExitCode int           `json:"exitCode"`
	Binary   BinaryReport  `json:"binary"`
	Server   ServerReport  `json:"server"`  // populated in commit 5
	Project  ProjectReport `json:"project"` // populated in commit 4
	Managed  ManagedReport `json:"managed"` // populated in commit 6
}

// ServerReport is a placeholder until commit 5 wires in the Server group.
type ServerReport struct {
	OK bool `json:"ok"`
}

// ManagedReport is a placeholder until commit 6 wires in the Managed group.
type ManagedReport struct {
	OK bool `json:"ok"`
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

	rep := DoctorReport{
		Binary:  runBinaryGroup(),
		Project: runProjectConfigGroup(cwd),
		// Server, Managed wired in later commits.
	}
	rep.ExitCode = computeExitCode(&rep)

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

// computeExitCode picks among 0 (clean), 1 (any group unhealthy), 2
// (infrastructure failure — reserved for the Server group's dial
// errors etc.; filled in by commit 5).
func computeExitCode(rep *DoctorReport) int {
	if !rep.Binary.OK || !rep.Project.OK {
		return 1
	}
	return 0
}

// finalExitCode applies the --exit-zero override.
func finalExitCode(rep *DoctorReport, exitZero bool) int {
	if exitZero {
		return 0
	}
	return rep.ExitCode
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
