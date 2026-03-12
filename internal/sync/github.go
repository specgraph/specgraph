// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/seanb4t/specgraph/internal/storage"
)

// GitHubAdapter syncs specs to GitHub Issues via the gh CLI.
type GitHubAdapter struct {
	runner CommandRunner
	repo   string // "owner/repo" format
}

// NewGitHubAdapter creates a GitHubAdapter with the given command runner and repo.
func NewGitHubAdapter(runner CommandRunner, repo string) *GitHubAdapter {
	return &GitHubAdapter{runner: runner, repo: repo}
}

// Name returns the adapter type identifier.
func (g *GitHubAdapter) Name() storage.SyncAdapterType {
	return storage.SyncAdapterGitHub
}

// Available checks whether the gh CLI is installed and reachable.
func (g *GitHubAdapter) Available() error {
	_, err := g.runner.Run(context.Background(), "gh", "--version")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAdapterNotAvailable, err)
	}
	return nil
}

// ghViewResponse captures the JSON output from gh issue view.
type ghViewResponse struct {
	State string `json:"state"`
}

// Push creates a GitHub issue from the given spec using the gh CLI.
func (g *GitHubAdapter) Push(ctx context.Context, spec *storage.Spec) (string, error) {
	if spec.Slug == "" {
		return "", fmt.Errorf("%w: spec slug is required", ErrPushFailed)
	}
	title := fmt.Sprintf("[spec] %s", spec.Slug)
	body := formatIssueBody(spec)
	labels := formatLabels(spec)

	args := []string{"issue", "create"}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}
	args = append(args, "--title", title, "--body", body, "--label", labels)
	out, err := g.runner.Run(ctx, "gh", args...)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPushFailed, err)
	}

	// gh issue create outputs the issue URL (e.g. https://github.com/owner/repo/issues/42)
	u, err := url.Parse(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("%w: failed to parse created issue URL: %w", ErrPushFailed, err)
	}
	number := path.Base(u.Path)
	if _, err := strconv.Atoi(number); err != nil {
		return "", fmt.Errorf("%w: invalid issue number in URL: %q", ErrPushFailed, number)
	}
	return number, nil
}

// Pull retrieves the current state of a GitHub issue by its number.
func (g *GitHubAdapter) Pull(ctx context.Context, externalID string) (string, error) {
	args := []string{"issue", "view", externalID}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}
	args = append(args, "--json", "state")
	out, err := g.runner.Run(ctx, "gh", args...)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPullFailed, err)
	}

	var resp ghViewResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("%w: failed to parse response: %w", ErrPullFailed, err)
	}

	return resp.State, nil
}

// formatIssueBody produces a markdown body for a GitHub issue from a spec.
func formatIssueBody(spec *storage.Spec) string {
	return fmt.Sprintf("## Spec: %s\n\n**Intent:** %s\n\n| Field | Value |\n|-------|-------|\n| Stage | %s |\n| Priority | %s |\n| Complexity | %s |\n| Version | %d |\n",
		spec.Slug, spec.Intent, spec.Stage, spec.Priority, spec.Complexity, spec.Version)
}

// formatLabels produces a comma-separated label string for a GitHub issue.
func formatLabels(spec *storage.Spec) string {
	return fmt.Sprintf("specgraph,%s,%s", spec.Stage, spec.Priority)
}
