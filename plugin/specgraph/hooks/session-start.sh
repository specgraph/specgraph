#!/usr/bin/env bash
# Copyright 2026 SpecGraph Contributors
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

if ! command -v specgraph >/dev/null 2>&1; then
  echo "specgraph CLI not found; skipping session prime" >&2
  exit 0
fi

specgraph read-mcp-resource specgraph://prime 2>&1 || {
  echo "specgraph prime failed (server unreachable?); session starts without prime" >&2
  exit 0
}
