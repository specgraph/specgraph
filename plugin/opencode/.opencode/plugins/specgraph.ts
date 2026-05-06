// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
//
// SpecGraph OpenCode plugin.
//
// Surface area (matched against `@opencode-ai/plugin` ≥ 1.3 — see Hooks
// interface in the package's index.d.ts):
//   - experimental.chat.system.transform: prepend `specgraph://prime` to the
//     system prompt so the model starts each turn with the same priming the
//     Claude Code and Cursor harnesses get. Runs on every turn, which keeps
//     the priming current rather than going stale after the first message.
//   - tool.execute.after: when the `mcp__specgraph__author` tool succeeds with
//     a stage action (spark/shape/specify/decompose/approve), record the stage
//     so the next system-prompt transform appends a nudge to run the
//     analytical passes registered for that stage. The cross-harness contract
//     is "after stage transition, passes are surfaced", not "identical
//     mechanism" — Claude uses a PostToolUse hook, Cursor uses a rule, we
//     thread it through the system-prompt transform here.
//
// We use execFile (argv array, no shell) to avoid shell injection if the
// argument list ever grows beyond the fixed `specgraph://prime` URI.

import type { Plugin } from "@opencode-ai/plugin";
import { execFile } from "node:child_process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

const STAGE_ACTIONS = new Set([
  "spark",
  "shape",
  "specify",
  "decompose",
  "approve",
]);

const plugin: Plugin = async () => {
  // Cache the prime output for the lifetime of this plugin instance. Reading
  // it once on first system-prompt transform avoids a CLI invocation per
  // turn; the prime is a session-priming digest, not per-turn live data.
  let cachedPrime: string | null = null;
  let primeAttempted = false;

  // One-shot stage nudge: set by tool.execute.after, consumed by the next
  // chat.system.transform. Cleared after consumption so the nudge doesn't
  // repeat indefinitely.
  let pendingStageNudge: string | null = null;

  const loadPrime = async (): Promise<string> => {
    if (cachedPrime !== null) return cachedPrime;
    if (primeAttempted) return "";
    primeAttempted = true;
    try {
      const { stdout } = await execFileAsync("specgraph", [
        "read-mcp-resource",
        "specgraph://prime",
      ]);
      cachedPrime = stdout.trim();
      return cachedPrime;
    } catch (err) {
      // Soft-fail: missing CLI or unreachable server should not block the
      // session. Match the bash session-start hook in the Claude shim.
      const msg = err instanceof Error ? err.message : String(err);
      cachedPrime = `# specgraph prime unavailable (${msg}); continuing without prime.`;
      return cachedPrime;
    }
  };

  return {
    "experimental.chat.system.transform": async (_input, output) => {
      const prime = await loadPrime();
      if (prime.length > 0) {
        output.system.push(prime);
      }
      if (pendingStageNudge !== null) {
        output.system.push(pendingStageNudge);
        pendingStageNudge = null;
      }
    },
    "tool.execute.after": async (input, _output) => {
      if (input.tool !== "mcp__specgraph__author") return;
      const action = input.args?.action;
      if (typeof action !== "string" || !STAGE_ACTIONS.has(action)) return;
      pendingStageNudge =
        `Run the analytical passes registered for the ${action} stage. ` +
        `Call the analytical_pass tool with action=run for each pass type ` +
        `returned by passes_for_stage for stage=${action}. Surface findings ` +
        `inline; do not silently swallow them.`;
    },
  };
};

export default plugin;
