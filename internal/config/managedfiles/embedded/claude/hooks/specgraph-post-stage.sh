#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt
#
# PostToolUse hook: when the `author` MCP tool succeeds with a stage action,
# emit a hook output that asks the model to run the analytical passes
# registered for that stage. The contract across all three harnesses
# (Claude Code, Cursor, OpenCode) is "after stage transition, passes are
# surfaced" — the per-harness mechanism differs, but the user-visible effect
# is the same.

set -euo pipefail

# stdin is the JSON envelope Claude Code passes to PostToolUse hooks.
input="$(cat)"

# We only act on the specgraph author tool. Any other tool is a no-op.
tool_name="$(printf '%s' "$input" | jq -r '.tool_name // empty')"
if [ "$tool_name" != "mcp__specgraph__author" ]; then
  exit 0
fi

action="$(printf '%s' "$input" | jq -r '.tool_input.action // empty')"
case "$action" in
  spark|shape|specify|decompose|approve)
    # Emit a structured hook decision asking the model to run the registered
    # passes. `block` here is conventional for post-tool nudges in Claude
    # Code; the model receives the reason as model-visible text.
    cat <<EOF
{
  "decision": "block",
  "reason": "Run the analytical passes registered for the ${action} stage. Call the analytical_pass tool with action=run for each pass type returned by passes_for_stage for stage=${action}. Surface the findings inline; do not silently swallow them."
}
EOF
    ;;
  *)
    exit 0
    ;;
esac
