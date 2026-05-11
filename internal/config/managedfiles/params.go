// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"fmt"
	"net/url"
)

// ProjectParams carries the per-project values strategy Build closures
// interpolate into managed file content. Construct once at init time;
// thread the same value through InspectAll and SyncAll.
//
// Build closures MUST be pure functions of ProjectParams — see
// docs/plans/2026-05-11-spgr-rwrp-pr-b-port-managed-files-design.md
// §"Build closure purity".
type ProjectParams struct {
	Slug      string
	ServerURL string // resolved, http or https, no /mcp/ suffix
}

// Validate rejects malformed slug or server URL. Lifted from
// pointers.NewOptions (deleted in this PR).
func (p ProjectParams) Validate() error {
	if !safeSlugPattern.MatchString(p.Slug) {
		return fmt.Errorf("project slug %q does not match %s", p.Slug, safeSlugPattern)
	}
	parsed, perr := url.Parse(p.ServerURL)
	if perr != nil {
		return fmt.Errorf("server URL %q: %w", p.ServerURL, perr)
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("server URL %q must be an absolute http or https URL", p.ServerURL)
	}
	return nil
}
