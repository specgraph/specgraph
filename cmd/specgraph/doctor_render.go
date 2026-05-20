// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// renderText writes the compact-when-green / expanded-when-problems text
// form of the report. verbose=true forces every group to expand.
func renderText(w io.Writer, rep DoctorReport, verbose bool) {
	if rep.Binary.OK && !verbose {
		_, _ = fmt.Fprintf(w, "Binary:         OK (v%s from %s)\n", rep.Binary.Version, rep.Binary.Commit) //nolint:errcheck // stdout write; not actionable
	} else {
		_, _ = fmt.Fprintf(w, "Binary:         %s\n", binaryStatusText(rep.Binary)) //nolint:errcheck // stdout write; not actionable
		if verbose || !rep.Binary.OK {
			_, _ = fmt.Fprintf(w, "  Version: %s\n", rep.Binary.Version) //nolint:errcheck // stdout write; not actionable
			_, _ = fmt.Fprintf(w, "  Commit:  %s\n", rep.Binary.Commit)  //nolint:errcheck // stdout write; not actionable
		}
	}
	// Server, Project, Managed group rendering land in commits 4, 5, 6.
}

func binaryStatusText(b BinaryReport) string {
	if b.OK {
		return fmt.Sprintf("OK (v%s)", b.Version)
	}
	return "PROBLEM (one or more identity fields empty)"
}

// renderJSON writes the canonical machine-readable form. Schema stays
// stable across versions; new fields may be added.
func renderJSON(w io.Writer, rep DoctorReport) {
	wrapped := map[string]any{
		"exitCode": rep.ExitCode,
		"groups": map[string]any{
			"binary":  rep.Binary,
			"server":  rep.Server,
			"project": rep.Project,
			"managed": rep.Managed,
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(wrapped) //nolint:errcheck // stdout write; encoding a plain map can't fail
}
