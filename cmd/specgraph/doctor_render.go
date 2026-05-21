// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

// renderText writes the compact-when-green / expanded-when-problems text
// form of the report. verbose=true forces every group to expand.
func renderText(w io.Writer, rep *DoctorReport, verbose bool) {
	if rep.ConfigError != "" {
		_, _ = fmt.Fprintf(w, "Global config:  PROBLEM (%s)\n", rep.ConfigError) //nolint:errcheck // stdout write; not actionable
	}
	if rep.Binary.OK && !verbose {
		_, _ = fmt.Fprintf(w, "Binary:         OK (v%s from %s)\n", rep.Binary.Version, rep.Binary.Commit) //nolint:errcheck // stdout write; not actionable
	} else {
		_, _ = fmt.Fprintf(w, "Binary:         %s\n", binaryStatusText(rep.Binary)) //nolint:errcheck // stdout write; not actionable
		if verbose || !rep.Binary.OK {
			_, _ = fmt.Fprintf(w, "  Version: %s\n", rep.Binary.Version) //nolint:errcheck // stdout write; not actionable
			_, _ = fmt.Fprintf(w, "  Commit:  %s\n", rep.Binary.Commit)  //nolint:errcheck // stdout write; not actionable
		}
	}
	// Project group rendering
	if rep.Project.OK && !verbose {
		_, _ = fmt.Fprintf(w, "%s\n", projectStatusLine(rep.Project)) //nolint:errcheck // stdout write; not actionable
	} else {
		_, _ = fmt.Fprintf(w, "%s\n", projectStatusLine(rep.Project)) //nolint:errcheck // stdout write; not actionable
		if verbose || !rep.Project.OK {
			if rep.Project.StrictError != "" {
				_, _ = fmt.Fprintf(w, "  StrictError:  %s\n", rep.Project.StrictError) //nolint:errcheck // stdout write; not actionable
			}
			for _, name := range rep.Project.UnknownNames {
				_, _ = fmt.Fprintf(w, "  UnknownName:  %s\n", name) //nolint:errcheck // stdout write; not actionable
			}
		}
	}
	// Server group rendering
	if rep.Server.OK && !verbose {
		_, _ = fmt.Fprintf(w, "%s\n", serverStatusLine(rep.Server)) //nolint:errcheck // stdout write; not actionable
	} else {
		_, _ = fmt.Fprintf(w, "%s\n", serverStatusLine(rep.Server)) //nolint:errcheck // stdout write; not actionable
		if verbose || !rep.Server.OK {
			_, _ = fmt.Fprintf(w, "  Reachable:    %v\n", rep.Server.Reachable)    //nolint:errcheck // stdout write; not actionable
			_, _ = fmt.Fprintf(w, "  Version:      %s\n", rep.Server.Version)      //nolint:errcheck // stdout write; not actionable
			_, _ = fmt.Fprintf(w, "  MCPHandshake: %s\n", rep.Server.MCPHandshake) //nolint:errcheck // stdout write; not actionable
			_, _ = fmt.Fprintf(w, "  SkillsCount:  %d\n", rep.Server.SkillsCount)  //nolint:errcheck // stdout write; not actionable
			if rep.Server.Error != "" {
				_, _ = fmt.Fprintf(w, "  Error:        %s\n", rep.Server.Error) //nolint:errcheck // stdout write; not actionable
			}
		}
	}
	// Managed group rendering
	if rep.Managed.OK && !verbose {
		_, _ = fmt.Fprintf(w, "%s\n", managedStatusLine(rep.Managed)) //nolint:errcheck // stdout write; not actionable
	} else {
		_, _ = fmt.Fprintf(w, "%s\n", managedStatusLine(rep.Managed)) //nolint:errcheck // stdout write; not actionable
		if verbose || !rep.Managed.OK {
			renderManagedExpanded(w, rep.Managed)
		}
	}
}

// renderManagedExpanded prints the per-file table for the Managed
// group, partitioned into host-pinned (project-root) and
// SpecGraph-owned (under .specgraph/agents/) subsections. Within
// each subsection, rows preserve manifest order.
func renderManagedExpanded(w io.Writer, rep ManagedReport) {
	var hostPinned, owned []managedfiles.FileState
	for _, f := range rep.Files {
		if isHostPinned(f.Path) {
			hostPinned = append(hostPinned, f)
		} else {
			owned = append(owned, f)
		}
	}
	if len(hostPinned) > 0 {
		_, _ = fmt.Fprintln(w, "  Host-pinned:") //nolint:errcheck // stdout write; not actionable
		for i := range hostPinned {
			writeManagedRow(w, &hostPinned[i])
		}
	}
	if len(owned) > 0 {
		_, _ = fmt.Fprintln(w, "  SpecGraph-owned:") //nolint:errcheck // stdout write; not actionable
		for i := range owned {
			writeManagedRow(w, &owned[i])
		}
	}
}

// writeManagedRow renders a single FileState row in the expanded
// per-file table. When Detail is non-empty, it follows the State column
// in parentheses, mirroring `specgraph init`'s per-file output.
func writeManagedRow(w io.Writer, f *managedfiles.FileState) {
	if f.Detail != "" {
		_, _ = fmt.Fprintf(w, "    %-50s %s (%s)\n", f.Path, managedfiles.StateName(f.State), f.Detail) //nolint:errcheck // stdout write; not actionable
		return
	}
	_, _ = fmt.Fprintf(w, "    %-50s %s\n", f.Path, managedfiles.StateName(f.State)) //nolint:errcheck // stdout write; not actionable
}

func binaryStatusText(b BinaryReport) string {
	if b.OK {
		return fmt.Sprintf("OK (v%s)", b.Version)
	}
	return "PROBLEM (one or more identity fields empty)"
}

// renderJSON writes the canonical machine-readable form. Schema stays
// stable across versions; new fields may be added.
func renderJSON(w io.Writer, rep *DoctorReport) {
	wrapped := map[string]any{
		"exitCode": rep.ExitCode(),
		"groups": map[string]any{
			"binary":  rep.Binary,
			"server":  rep.Server,
			"project": rep.Project,
			"managed": rep.Managed,
		},
	}
	if rep.ConfigError != "" {
		wrapped["configError"] = rep.ConfigError
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(wrapped) //nolint:errcheck // stdout write; encoding a plain map can't fail
}
