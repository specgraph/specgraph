---
description: After a SpecGraph stage transition, run the analytical passes registered for that stage.
alwaysApply: false
---

# SpecGraph Post-Stage

After a successful `author` tool call (`action`: spark / shape / specify /
decompose / approve), call the `analytical_pass` tool with `action: "run"`
for each pass type returned by `passes_for_stage` for the stage that just
completed.

The pass registry maps stages to passes:

| Stage | Auto-passes |
|---|---|
| `spark` | constitution-check |
| `shape` | constitution-check, peripheral-vision |
| `specify` | constitution-check, red-team, consistency |
| `decompose` | constitution-check, simplicity |
| `approve` | constitution-check |

Surface the findings; don't silently swallow them. The user decides what to
fix and what to defer.
