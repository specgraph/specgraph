// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestComposeGolden(t *testing.T) {
	cases := []struct {
		name      string
		stage     Stage
		slug      string
		maxStable int
		maxTotal  int
	}{
		{"spark", StageSpark, "", 4000, 6000},
		{"shape", StageShape, "oauth-refresh", 4500, 7000},
		{"specify", StageSpecify, "oauth-refresh", 4500, 7000},
		{"decompose", StageDecompose, "oauth-refresh", 4500, 7000},
		{"approve", StageApprove, "oauth-refresh", 4500, 7000},
	}
	c := NewComposer(&fakeComposerBackend{
		constitution: &ConstitutionSummary{
			PrimaryLanguage: "Go",
			KeyConstraints:  []string{"No panics", "Transactional writes"},
		},
		specSummary: &SpecSummary{Slug: "oauth-refresh", Intent: "Refresh tokens", Stage: "shape"},
	})
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: tt.stage, Slug: tt.slug, Posture: "partner"})
			if err != nil {
				t.Fatalf("compose: %v", err)
			}
			if result.StableTokens > tt.maxStable {
				t.Errorf("stable tokens %d > budget %d (restructure per design validation prerequisite)", result.StableTokens, tt.maxStable)
			}
			if result.TotalTokens > tt.maxTotal {
				t.Errorf("total tokens %d > budget %d", result.TotalTokens, tt.maxTotal)
			}
			goldenPath := filepath.Join("testdata", "golden", tt.name+".md")
			if *update {
				if mkErr := os.MkdirAll(filepath.Dir(goldenPath), 0o750); mkErr != nil {
					t.Fatalf("mkdir: %v", mkErr)
				}
				if wrErr := os.WriteFile(goldenPath, []byte(result.Body), 0o600); wrErr != nil {
					t.Fatalf("write golden: %v", wrErr)
				}
				return
			}
			want, readErr := os.ReadFile(goldenPath)
			if readErr != nil {
				t.Fatalf("read golden: %v (run with -update to create)", readErr)
			}
			if string(want) != result.Body {
				t.Errorf("composed output differs from golden %s", goldenPath)
			}
		})
	}
}
