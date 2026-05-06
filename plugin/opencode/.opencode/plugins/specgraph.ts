// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
//
// SpecGraph OpenCode plugin.
//
// Surface area:
//   - session.start: read specgraph://prime via the specgraph CLI and append
//     it to the session output, so the model starts with the same priming the
//     Claude Code and Cursor harnesses get.
//   - tool.use (after a successful mcp__specgraph__author with a stage action):
//     surface a suggestion to run the analytical passes registered for that
//     stage. The contract is "after stage transition, passes are surfaced",
//     not "identical mechanism across harnesses".
//
// We intentionally use execFile (argv array, no shell) to avoid shell
// injection on user-supplied data. specgraph://prime is a fixed URI here, but
// using execFile keeps this plugin safe by construction if arguments grow
// later.

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

const plugin: Plugin = {
  name: "specgraph",
  hooks: {
    "session.start": async (_ctx, output) => {
      try {
        const { stdout } = await execFileAsync("specgraph", [
          "read-mcp-resource",
          "specgraph://prime",
        ]);
        const trimmed = stdout.trim();
        if (trimmed.length > 0) {
          output.append(trimmed + "\n");
        }
      } catch (err) {
        // Soft-fail: missing CLI or unreachable server should not block the
        // session start. Match the bash session-start hook in the Claude
        // shim.
        const msg = err instanceof Error ? err.message : String(err);
        output.append(
          `# specgraph prime unavailable (${msg}); continuing without prime.\n`,
        );
      }
    },
    "tool.use": async (ctx) => {
      // Field names are best-effort against @opencode-ai/plugin; verify
      // against the version pinned in package.json. If the API uses
      // different names (e.g., ctx.tool.name vs ctx.tool), update here.
      const toolName = (ctx as { tool?: string }).tool;
      const phase = (ctx as { phase?: string }).phase;
      const action = (ctx as { input?: { action?: string } }).input?.action;

      if (
        toolName === "mcp__specgraph__author" &&
        phase === "after" &&
        typeof action === "string" &&
        STAGE_ACTIONS.has(action)
      ) {
        const suggest = (ctx as { suggest?: (msg: string) => void }).suggest;
        if (typeof suggest === "function") {
          suggest(
            `Run analytical passes for the ${action} stage by calling analytical_pass with action=run for each pass type returned by passes_for_stage.`,
          );
        }
      }
    },
  },
};

export default plugin;
