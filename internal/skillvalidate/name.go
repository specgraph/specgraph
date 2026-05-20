// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skillvalidate

import "regexp"

// NameRegex is the canonical pattern for skill directory and frontmatter
// names. Kebab-case ASCII: lowercase alphanumerics with single hyphens
// between segments. Used by:
//   - the validator (this package) to reject malformed names at build time
//   - internal/mcp/skills.NewEmbedded() to reject malformed names at server
//     startup
//   - internal/mcp/resources.go's extractSkillName helper to reject malformed
//     URIs at request time
//
// One regex, three callers — keeps "what counts as a valid skill name" in
// one place.
var NameRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
