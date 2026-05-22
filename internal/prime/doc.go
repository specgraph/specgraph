// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package prime composes the data shown in the SpecGraph "prime" responses.
//
// The Composer reads through a wide aggregate storage Backend interface and
// assembles ProjectView or SpecView values describing the current state of
// the project or a single spec. The composer does not render output —
// rendering (Markdown, text, etc.) is implemented separately by callers.
//
// All types in this package are domain types from internal/storage; proto
// conversion happens in the handler layer.
package prime
