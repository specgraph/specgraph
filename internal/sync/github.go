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

	"github.com/specgraph/specgraph/internal/storage"
)

// GitHubAdapter syncs specs to GitHub Issues via the gh CLI.
type GitHubAdapter struct {
	runner CommandRunner
	repo   string // "owner/repo" format
}

// NewGitHubAdapter creates a GitHubAdapter with the given command runner and repo.
// repo must be in "owner/repo" format.
func NewGitHubAdapter(runner CommandRunner, repo string) *GitHubAdapter {
	return &GitHubAdapter{runner: runner, repo: repo}
}

// Name returns the adapter type identifier.
func (g *GitHubAdapter) Name() storage.SyncAdapterType {
	return storage.SyncAdapterGitHub
}

// Available checks whether the gh CLI is installed, authenticated, and the
// adapter has a configured repo.
func (g *GitHubAdapter) Available(ctx context.Context) error {
	if g.repo == "" {
		return fmt.Errorf("%w: repo not configured", ErrAdapterNotAvailable)
	}
	_, err := g.runner.Run(ctx, "gh", "--version")
	if err != nil {
		return fmt.Errorf("%w: gh CLI not found: %w", ErrAdapterNotAvailable, err)
	}
	_, err = g.runner.Run(ctx, "gh", "auth", "status")
	if err != nil {
		return fmt.Errorf("%w: gh CLI not authenticated: %w", ErrAdapterNotAvailable, err)
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
		return "", fmt.Errorf("%w: spec slug is required", errPushFailed)
	}
	if g.repo == "" {
		return "", fmt.Errorf("%w: repo is required", errPushFailed)
	}
	if !spec.Stage.IsValid() {
		return "", fmt.Errorf("%w: invalid spec stage: %q", errPushFailed, spec.Stage)
	}
	if !spec.Priority.IsValid() {
		return "", fmt.Errorf("%w: invalid spec priority: %q", errPushFailed, spec.Priority)
	}
	title := fmt.Sprintf("[spec] %s", spec.Slug)
	body := formatIssueBody(spec)
	labels := formatLabels(spec)

	args := []string{"issue", "create"}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}
	args = append(args, "--title", title, "--body", body)
	for _, label := range labels {
		args = append(args, "--label", label)
	}
	out, err := g.runner.Run(ctx, "gh", args...)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errPushFailed, err)
	}

	// gh issue create outputs the issue URL (e.g. https://github.com/owner/repo/issues/42)
	issueURL := strings.TrimSpace(string(out))
	if issueURL == "" {
		return "", fmt.Errorf("%w: gh issue create returned empty output", errPushFailed)
	}
	u, err := url.Parse(issueURL)
	if err != nil {
		return "", fmt.Errorf("%w: failed to parse created issue URL: %w", errPushFailed, err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("%w: unexpected URL scheme in created issue URL: %q", errPushFailed, issueURL)
	}
	if u.Host != "github.com" {
		return "", fmt.Errorf("%w: unexpected host in created issue URL: %q", errPushFailed, issueURL)
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) < 4 {
		return "", fmt.Errorf("%w: unexpected URL structure in created issue URL: %q", errPushFailed, issueURL)
	}
	number := segments[len(segments)-1]
	if _, err := strconv.Atoi(number); err != nil {
		return "", fmt.Errorf("%w: invalid issue number in URL: %q", errPushFailed, number)
	}
	return issueURL, nil
}

// Pull retrieves the current state of a GitHub issue by its URL or number.
func (g *GitHubAdapter) Pull(ctx context.Context, externalID string) (string, error) {
	if externalID == "" {
		return "", fmt.Errorf("%w: external ID is empty", errPullFailed)
	}
	// Extract issue number from URL if externalID is a full URL.
	issueRef := externalID
	if u, parseErr := url.Parse(externalID); parseErr == nil && u.Scheme != "" {
		if u.Host != "github.com" {
			return "", fmt.Errorf("%w: unexpected host in external ID URL: %q", errPullFailed, externalID)
		}
		issueRef = path.Base(u.Path)
	}
	if _, err := strconv.Atoi(issueRef); err != nil {
		return "", fmt.Errorf("%w: invalid issue reference: %q", errPullFailed, issueRef)
	}
	args := []string{"issue", "view", issueRef}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}
	args = append(args, "--json", "state")
	out, err := g.runner.Run(ctx, "gh", args...)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errPullFailed, err)
	}

	var resp ghViewResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("%w: failed to parse response: %w", errPullFailed, err)
	}
	if resp.State == "" {
		return "", fmt.Errorf("%w: missing issue state in response", errPullFailed)
	}

	return resp.State, nil
}

// escapeCell escapes pipe characters and newlines for markdown table cells.
func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// formatIssueBody produces a markdown body for a GitHub issue from a spec.
func formatIssueBody(spec *storage.Spec) string {
	return fmt.Sprintf("## Spec: %s\n\n**Intent:** %s\n\n| Field | Value |\n|-------|-------|\n| Stage | %s |\n| Priority | %s |\n| Complexity | %s |\n| Version | %d |\n",
		escapeCell(spec.Slug), escapeCell(spec.Intent),
		escapeCell(string(spec.Stage)), escapeCell(string(spec.Priority)),
		escapeCell(spec.Complexity), spec.Version)
}

// formatLabels produces individual label strings for a GitHub issue.
// Each label is passed as a separate --label argument to avoid comma-parsing
// issues in the gh CLI when label values contain special characters.
func formatLabels(spec *storage.Spec) []string {
	return []string{"specgraph", string(spec.Stage), string(spec.Priority)}
}
