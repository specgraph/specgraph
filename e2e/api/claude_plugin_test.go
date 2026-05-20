// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/e2e/testutil"
)

var _ = Describe("Claude plugin shim install", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "spgr-claude-e2e-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	})

	It("writes all 6 Claude-owned managed files on fresh init", func() {
		claudeRunInit(tmpDir)
		expected := []string{
			".claude/settings.json",
			".specgraph/agents/claude/.claude-plugin/plugin.json",
			".specgraph/agents/claude/.claude-plugin/marketplace.json",
			".specgraph/agents/claude/hooks/specgraph-session-start.sh",
			".specgraph/agents/claude/hooks/specgraph-post-stage.sh",
			".specgraph/agents/claude/routing-guide.md",
		}
		for _, p := range expected {
			_, err := os.Stat(filepath.Join(tmpDir, p))
			Expect(err).NotTo(HaveOccurred(), "expected %s to exist", p)
		}
	})

	It("preserves /plugin disable across re-init", func() {
		claudeRunInit(tmpDir)
		settingsPath := filepath.Join(tmpDir, ".claude/settings.json")
		body, err := os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())
		// Flip the enabled state to false via structural JSON mutation —
		// not strings.ReplaceAll — so the test doesn't silently no-op if
		// the formatter ever changes spacing or indentation.
		var settings map[string]any
		Expect(json.Unmarshal(body, &settings)).To(Succeed())
		enabled, ok := settings["enabledPlugins"].(map[string]any)
		Expect(ok).To(BeTrue(), "enabledPlugins should be an object")
		enabled["specgraph@specgraph-local"] = false
		body, err = json.MarshalIndent(settings, "", "  ")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(settingsPath, body, 0o644)).To(Succeed()) //nolint:gosec // test fixture in t.TempDir; permissions are intentional
		claudeRunInit(tmpDir)
		body2, err := os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())
		var doc map[string]any
		Expect(json.Unmarshal(body2, &doc)).To(Succeed())
		enabled, ok = doc["enabledPlugins"].(map[string]any)
		Expect(ok).To(BeTrue(), "enabledPlugins should be an object")
		Expect(enabled["specgraph@specgraph-local"]).To(BeFalse(),
			"expected user's disable to survive init")
	})

	It("registers the marketplace at the directory root, not .claude-plugin/", func() {
		claudeRunInit(tmpDir)
		body, err := os.ReadFile(filepath.Join(tmpDir, ".claude/settings.json"))
		Expect(err).NotTo(HaveOccurred())
		var doc map[string]any
		Expect(json.Unmarshal(body, &doc)).To(Succeed())
		ekm, ok := doc["extraKnownMarketplaces"].(map[string]any)
		Expect(ok).To(BeTrue(), "extraKnownMarketplaces should be an object")
		entry, ok := ekm["specgraph-local"].(map[string]any)
		Expect(ok).To(BeTrue(), "extraKnownMarketplaces.specgraph-local should be an object")
		source, ok := entry["source"].(map[string]any)
		Expect(ok).To(BeTrue(), "extraKnownMarketplaces.specgraph-local.source should be an object")
		Expect(source["path"]).To(Equal("./.specgraph/agents/claude"))
	})
})

// claudeRunInit shells out to the specgraph binary to execute `specgraph init`
// against tmpDir. Uses the suite-level cliBinaryPath and serverInfo.ConfigPath
// set up in BeforeSuite (api_suite_test.go).
func claudeRunInit(dir string) {
	cli := testutil.NewCLIRunner(cliBinaryPath, serverInfo.ConfigPath, "")
	result := cli.RunInDir(dir, "init", "spgr-claude-e2e", "--yes")
	Expect(result.ExitCode).To(Equal(0), "init stderr: %s", result.Stderr)
}

