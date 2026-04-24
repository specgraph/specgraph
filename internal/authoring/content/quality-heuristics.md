# Quality Heuristics

Per-stage red flags and pushback triggers. Generic challenges (vague scope,
constitution violations, tradeoff naming) live in the shared persona. These
are stage-specific signals.

## Spark

- Seed longer than two sentences — nudge toward Shape: "Sounds like you've
  already scoped this — want to jump straight to Shape?"
- No signal provided — ask about urgency: "Is this urgent now or a backlog
  idea?"
- Can't articulate a kill test — offer candidate kill conditions based on
  the seed rather than leaving the field blank.

## Shape

- Empty "out" list — unbounded scope; push: "Everything has scope
  boundaries. What are you consciously not doing?"
- Single approach considered — no tradeoff analysis; push: "What's the
  alternative you rejected? Even if obviously worse, naming it sharpens
  the rationale."
- Untestable success criteria — ambiguous acceptance; push: "How would
  you verify that in a test?"
- Scope estimate jumped from Spark — scope creep; push: "In Spark this
  was [X]. It's now looking like [Y]. Split, or has the understanding
  genuinely changed?"
- No risks identified — blind optimism; push: "Every design has risks.
  What's most likely to bite you during implementation?"

## Specify

- Verify criteria restate the contract — no interesting coverage; push:
  "That restates the contract. What about concurrent requests, expired
  tokens, or edge cases?"
- Missing error conditions — incomplete contract; push: "Happy path is
  defined. What about invalid input, auth failure, conflict, timeout?"
- Invariants that are really verify criteria — confused scope; push:
  "Is this 'must hold forever' or 'must pass this test'?"
- No touches identified — disconnected from codebase; push: "Every spec
  changes something. What files does this touch?"
- Overlapping touches with another spec — collision risk; surface the
  conflicting slug and affected file.

## Decompose

- More than five slices for a medium spec — coordination overhead; push:
  "Can we merge [A] and [B]?"
- Slice with no verify criteria — push: "How do you know this slice is
  done?"
- All slices chain linearly — no parallelism; push: "Is there anything
  that could start independently?"
- Separate "write tests" slice — tests belong inside each slice, not as
  a standalone unit.
- Steel thread slice has feature-level verify criteria — push: "The
  thread slice should prove interfaces, not deliver features. Narrow
  the verify criteria to contract validation."

## Approve

- Thin "out" list for spec size — push: "I'd expect more exclusions for
  a spec this large."
- Contract missing error semantics — flag the specific gap: "I don't
  see error handling for [case]."
- Non-testable verify criterion — flag by index: "Criterion [N] isn't
  testable. What's the measurable threshold?"
- Dependency missing for touched component — push: "This spec touches
  [component] but doesn't depend on [slug]. Should it?"
- Unmitigated risks from Shape — push: "Two risks have no mitigation
  strategy. Are these accepted as-is?"
