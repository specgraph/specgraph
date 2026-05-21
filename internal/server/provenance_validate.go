// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	"github.com/specgraph/specgraph/internal/storage"
)

// validateProvenance enforces the create-time invariants per the
// spec-provenance design.
//
//   - AUTHORED: no provenance_detail (or only Authored variant), only
//     spark_output may be set; all later stage outputs must be nil.
//   - RETROACTIVE_FROM_PR: all 4 stage outputs required; retroactive_from_pr
//     detail required; url + sha must be non-empty.
//   - DECLARED: all 4 stage outputs required; declared detail required;
//     declared_by must be non-empty.
//
// Returns the matching sentinel error on violation.
func validateProvenance(
	pt storage.SpecProvenanceType,
	pd storage.SpecProvenanceDetail,
	spark *storage.SparkOutput,
	shape *storage.ShapeOutput,
	specify *storage.SpecifyOutput,
	decompose *storage.DecomposeOutput,
) error {
	switch pt {
	case storage.SpecProvenanceAuthored, "":
		// Empty string treated as AUTHORED (default).
		if shape != nil || specify != nil || decompose != nil {
			return storage.ErrAuthoredRequiresSparkOnly
		}
		if pd.RetroactiveFromPR != nil || pd.Declared != nil {
			return storage.ErrProvenanceMismatch
		}
	case storage.SpecProvenanceRetroactiveFromPR:
		if spark == nil || shape == nil || specify == nil || decompose == nil {
			return storage.ErrRetroactiveRequiresAllOutputs
		}
		if pd.RetroactiveFromPR == nil {
			return storage.ErrProvenanceMismatch
		}
		if pd.RetroactiveFromPR.URL == "" || pd.RetroactiveFromPR.SHA == "" {
			return storage.ErrRetroactiveRequiresPRRef
		}
		if pd.Declared != nil {
			return storage.ErrProvenanceMismatch
		}
	case storage.SpecProvenanceDeclared:
		if spark == nil || shape == nil || specify == nil || decompose == nil {
			return storage.ErrDeclaredRequiresAllOutputs
		}
		if pd.Declared == nil {
			return storage.ErrProvenanceMismatch
		}
		if pd.Declared.DeclaredBy == "" {
			return storage.ErrDeclaredRequiresDeclaredBy
		}
		if pd.RetroactiveFromPR != nil {
			return storage.ErrProvenanceMismatch
		}
	default:
		return fmt.Errorf("unknown provenance type %q", pt)
	}
	return nil
}
