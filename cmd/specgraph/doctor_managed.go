// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"strings"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

// ManagedReport describes the per-file managed-files inspection result
// for the doctor command's Managed group.
type ManagedReport struct {
	OK     bool                     `json:"ok"`
	Synced int                      `json:"synced"`
	Total  int                      `json:"total"`
	Files  []managedfiles.FileState `json:"files"`
}

// runManagedGroup inspects every managed file for the resolved harness
// set and returns a ManagedReport. The group is OK only when every
// file is Synced. An InspectAll error is treated as not-OK with a
// synthetic "inspect failure" file row carrying the error in Detail.
func runManagedGroup(cwd string, harnesses []managedfiles.Harness, params managedfiles.ProjectParams) ManagedReport {
	states, err := managedfiles.InspectAll(cwd, harnesses, params)
	if err != nil {
		return ManagedReport{
			OK:    false,
			Total: 0,
			Files: []managedfiles.FileState{
				{Path: "(inspect)", State: managedfiles.StateDrifted, Detail: err.Error()},
			},
		}
	}
	synced := 0
	for _, s := range states {
		if s.State == managedfiles.StateSynced {
			synced++
		}
	}
	return ManagedReport{
		OK:     synced == len(states),
		Synced: synced,
		Total:  len(states),
		Files:  states,
	}
}

// managedStatusLine renders the compact single-line form of the
// Managed group. All-synced → "Managed files: 14/14 synced". When any
// file is off, it appends a breakdown like
// "12/14 synced — 1 missing, 1 drifted".
func managedStatusLine(rep ManagedReport) string {
	if rep.Total == 0 && rep.OK {
		return "Managed files: 0/0 synced"
	}
	if rep.OK {
		return fmt.Sprintf("Managed files: %d/%d synced", rep.Synced, rep.Total)
	}
	var missing, stale, drifted int
	for _, s := range rep.Files {
		switch s.State {
		case managedfiles.StateMissing:
			missing++
		case managedfiles.StateStale:
			stale++
		case managedfiles.StateDrifted:
			drifted++
		case managedfiles.StateSynced:
			// already counted in rep.Synced
		}
	}
	var parts []string
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", missing))
	}
	if stale > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", stale))
	}
	if drifted > 0 {
		parts = append(parts, fmt.Sprintf("%d drifted", drifted))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Managed files: %d/%d synced", rep.Synced, rep.Total)
	}
	return fmt.Sprintf("Managed files: %d/%d synced — %s",
		rep.Synced, rep.Total, strings.Join(parts, ", "))
}

// isHostPinned reports whether path is host-pinned (lives at the
// project root in a path required by the harness, like AGENTS.md or
// .cursor/rules/…) rather than under the SpecGraph-owned
// .specgraph/agents/ tree.
func isHostPinned(path string) bool {
	return !strings.HasPrefix(path, ".specgraph/agents/")
}

// runDoctorFix re-syncs every Stale or Missing managed file via
// managedfiles.Sync and prints actionable guidance per Drifted file.
// `specgraph init` does not expose --force / --keep-edits flags, so the
// guidance describes the manual reconciliation paths users actually
// have. Synced rows are left alone.
func runDoctorFix(cwd string, rep ManagedReport, harnesses []managedfiles.Harness, params managedfiles.ProjectParams) error {
	mfsByPath := map[string]managedfiles.ManagedFile{}
	for _, mf := range managedfiles.Manifest(harnesses) {
		mfsByPath[mf.Path] = mf
	}
	var drifted []string
	for _, s := range rep.Files {
		switch s.State {
		case managedfiles.StateStale, managedfiles.StateMissing:
			mf, ok := mfsByPath[s.Path]
			if !ok {
				continue
			}
			if _, err := managedfiles.Sync(cwd, mf, params, managedfiles.SyncOptions{}); err != nil {
				return fmt.Errorf("sync %s: %w", s.Path, err)
			}
		case managedfiles.StateDrifted:
			drifted = append(drifted, s.Path)
		case managedfiles.StateSynced:
			// nothing to do
		}
	}
	for _, path := range drifted {
		fmt.Printf("%s (drifted): the file has local edits that don't match the canonical form.\n", path)
		fmt.Printf("  Inspect the diff (e.g. `specgraph doctor --json --verbose` and compare against the embedded canonical),\n")
		fmt.Printf("  then either edit %s in place to reconcile, or delete it and run `specgraph init` to regenerate.\n", path)
	}
	return nil
}

// harnessesFromFlag turns the --harness flag value into a slice of
// Harness enum values. Empty flag falls back to the harness set
// resolved from the project config (via harnessSliceFromConfig from
// the init command). An unknown name yields an empty slice; the
// Managed group will then report 0/0 synced.
func harnessesFromFlag(pc *config.ProjectConfig, flag string) []managedfiles.Harness {
	switch flag {
	case "":
		if pc == nil {
			return harnessSliceFromConfig(nil)
		}
		return harnessSliceFromConfig(pc.Harnesses)
	case "claude":
		return []managedfiles.Harness{managedfiles.HarnessClaude}
	case "cursor":
		return []managedfiles.Harness{managedfiles.HarnessCursor}
	case "opencode":
		return []managedfiles.Harness{managedfiles.HarnessOpenCode}
	default:
		return nil
	}
}
