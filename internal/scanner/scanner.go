// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package scanner implements Tier 0 codebase scanning for constitution drafting.
package scanner //nolint:revive // package-comments: "scanner" is clearer than alternatives for this domain

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

const maxFileSize = 1 << 20 // 1 MiB

// Scan performs a Tier 0 scan of the given directory, detecting languages,
// frameworks, infrastructure, and CI configuration.
// It always returns a non-nil *Constitution with detected fields populated
// (empty strings/maps if nothing found). The error return is reserved for
// future use and is currently always nil.
//
// Note: Kubernetes detection walks the directory tree but skips hidden
// directories, vendor, and node_modules.
func Scan(dir string) (*specv1.Constitution, error) {
	c := &specv1.Constitution{
		Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Tech: &specv1.TechConfig{
			Languages:      &specv1.LanguageConfig{},
			Frameworks:     map[string]string{},
			Infrastructure: map[string]string{},
		},
	}

	detectLanguage(dir, c)
	detectFrameworks(dir, c)
	detectInfrastructure(dir, c)
	detectCI(dir, c)

	return c, nil
}

func detectLanguage(dir string, c *specv1.Constitution) {
	type langEntry struct{ file, lang string }
	langFiles := []langEntry{
		{"go.mod", "go"},
		{"Cargo.toml", "rust"},
		{"pom.xml", "java"},
		{"build.gradle", "java"},
		{"setup.py", "python"},
		{"pyproject.toml", "python"},
		{"Gemfile", "ruby"},
	}

	for _, entry := range langFiles {
		if fileExists(filepath.Join(dir, entry.file)) {
			c.Tech.Languages.Primary = entry.lang
			return
		}
	}

	// Check package.json — could be JS or TS
	if fileExists(filepath.Join(dir, "package.json")) {
		if fileExists(filepath.Join(dir, "tsconfig.json")) {
			c.Tech.Languages.Primary = "typescript"
		} else {
			c.Tech.Languages.Primary = "javascript"
		}
	}
}

func detectFrameworks(dir string, c *specv1.Constitution) {
	// Go frameworks
	if gomod := readFile(filepath.Join(dir, "go.mod")); gomod != "" {
		if strings.Contains(gomod, "connectrpc.com/connect") {
			c.Tech.Frameworks["api"] = "ConnectRPC"
		}
		if strings.Contains(gomod, "github.com/go-chi/chi") {
			c.Tech.Frameworks["api"] = "chi"
		}
		if strings.Contains(gomod, "github.com/gin-gonic/gin") {
			c.Tech.Frameworks["api"] = "gin"
		}
		if strings.Contains(gomod, "github.com/spf13/cobra") {
			c.Tech.Frameworks["cli"] = "cobra"
		}
		if strings.Contains(gomod, "github.com/stretchr/testify") {
			c.Tech.Frameworks["testing"] = "testify"
		}
	}

	// Node frameworks
	if pkgJSON := readFile(filepath.Join(dir, "package.json")); pkgJSON != "" && len(pkgJSON) <= maxFileSize {
		var pkg map[string]any
		if err := json.Unmarshal([]byte(pkgJSON), &pkg); err == nil {
			deps := mergeDeps(pkg)
			if _, ok := deps["react"]; ok {
				c.Tech.Frameworks["ui"] = "React"
			}
			if _, ok := deps["next"]; ok {
				c.Tech.Frameworks["ui"] = "Next.js"
			}
			if _, ok := deps["express"]; ok {
				c.Tech.Frameworks["api"] = "Express"
			}
			if _, ok := deps["fastify"]; ok {
				c.Tech.Frameworks["api"] = "Fastify"
			}
		}
		// If unmarshal fails, skip framework detection for this file silently.
		// TODO(scanner): surface parse warnings when Scan gains a warning accumulator.
	}
}

func detectInfrastructure(dir string, c *specv1.Constitution) {
	if fileExists(filepath.Join(dir, "Dockerfile")) {
		c.Tech.Infrastructure["runtime"] = "Docker"
	}

	if fileExists(filepath.Join(dir, "docker-compose.yaml")) || fileExists(filepath.Join(dir, "docker-compose.yml")) {
		c.Tech.Infrastructure["orchestration"] = "Docker Compose"
	}

	// Kubernetes: walk directory tree looking for K8s manifests.
	// Errors during the walk (e.g. permission denied) are intentionally
	// skipped so scanning continues for accessible directories.
	//nolint:errcheck // WalkDir errors are handled inside the callback
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries gracefully
		}
		if d.IsDir() {
			// Skip hidden dirs and vendor
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		content := readFile(path)
		if strings.Contains(content, "apiVersion:") && strings.Contains(content, "kind:") {
			c.Tech.Infrastructure["runtime"] = "Kubernetes"
			return filepath.SkipAll
		}
		return nil
	})
}

func detectCI(dir string, c *specv1.Constitution) {
	if dirExists(filepath.Join(dir, ".github", "workflows")) {
		c.Tech.Infrastructure["ci"] = "GitHub Actions"
	}
	if fileExists(filepath.Join(dir, ".gitlab-ci.yml")) {
		c.Tech.Infrastructure["ci"] = "GitLab CI"
	}
	if fileExists(filepath.Join(dir, "Jenkinsfile")) {
		c.Tech.Infrastructure["ci"] = "Jenkins"
	}
	if fileExists(filepath.Join(dir, ".circleci", "config.yml")) {
		c.Tech.Infrastructure["ci"] = "CircleCI"
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func readFile(path string) string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > maxFileSize {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func mergeDeps(pkg map[string]any) map[string]any {
	merged := map[string]any{}
	for _, key := range []string{"dependencies", "devDependencies"} {
		if deps, ok := pkg[key].(map[string]any); ok {
			for k, v := range deps {
				merged[k] = v
			}
		}
	}
	return merged
}
