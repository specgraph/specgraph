# The Problem

**Why specs-as-files fails — and why it matters more now**

---

## The Upstream Bottleneck

AI coding agents produce code at a rate no human team can match. The
bottleneck has moved upstream: specification, review, and verification.

Three independent studies point in the same direction:

- [Faros AI](https://www.faros.ai/blog/ai-software-engineering) (10,000+
  developers, 1,255 teams): task completion rose 21%, but code review time
  climbed 91% and bugs per developer increased 9%.
- [METR](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/)
  (16 experienced open-source developers, 246 tasks): developers were 19%
  *slower* with AI assistance — while believing they were 20% faster.
- [CodeRabbit](https://www.coderabbit.ai/blog/state-of-ai-vs-human-code-generation-report)
  (470 pull requests): AI-generated code produced 1.7x more issues per PR,
  with higher severity across logic, security, and performance categories.

Generating code is cheap. Knowing what code to generate, and verifying
that it does the right thing, is not.

Spec-Driven Development (SDD) flips the priority: the spec is the source
of truth, and code is what you generate from it. Get the spec right and
the code follows. Get it wrong and no amount of code review saves you.

---

## The Status Quo

Most teams manage specifications the same way: markdown files in a
`specs/` folder, Confluence pages, Jira epics, or Google Docs shared
across Slack threads. A product manager writes requirements, an engineer
adds technical details, and the document gets reviewed in a meeting or a
pull request. Then it sits there. The spec was useful for the two weeks it
took to align the team, but by the time implementation starts, it is
already drifting from reality.

Teams do write specs. The format just can't keep up. A markdown file does not know what other specs
depend on it. A Jira ticket can link to other tickets, but those links
carry no semantics: is this a hard dependency, a soft reference, or just
"related"? A wiki page can be updated, but nothing forces consistency
between the page and the codebase it describes.

When AI agents enter the picture, these gaps become structural barriers.
An agent that receives "implement the feature in spec-42.md" gets prose.
There's no queryable structure, no programmatic way to extract acceptance
criteria, and nothing connecting the spec to the project's architectural
constraints. The format that was adequate for human teams blocks
agent-assisted development.

---

## Five Gaps

### No Ground Truth

Every authoring session starts cold. The developer or AI agent writing a
new spec does not know the project's tech stack, architectural principles,
naming conventions, or constraints, unless someone explicitly provides
them. That context lives in people's heads, in scattered ADRs, in tribal
knowledge that never got written down, or in code patterns that no one
thought to document.

**Example:** An AI agent drafts a spec for a new microservice. The project
uses Go, ConnectRPC, and PostgreSQL exclusively. Nothing in the spec
environment encodes these constraints. The agent proposes a Java service
with REST endpoints and a MongoDB store — technically valid, architecturally
wrong. The review catches it. The rework costs a sprint.

A spec for a new API endpoint proposes a handler structure, middleware
chain, and test approach. But the codebase already has an established
router pattern, a standard middleware stack, and test helpers that handle
setup and teardown. The spec author (human or AI) did not know any of this. The resulting spec describes an implementation structurally
incompatible with the existing code. The implementing engineer reconciles
the two by hand, doing work that should have been done at authoring time.

### No Governance

Architectural decisions exist (ADRs, style guides, team agreements) but
nothing enforces them at the spec level. A constraint like "no
shared databases between services" or "all public APIs must have rate
limiting" lives in a document somewhere. The spec authoring process has no
mechanism to check new specs against these constraints. Violations surface
in code review if you are lucky, or in production if you are not.

**Example:** ADR-007 states that services must not share databases. A new
spec for a reporting feature includes a cross-service database join for
performance. The spec gets approved because the reviewer did not remember
ADR-007. The implementation ships, creates a hidden coupling between two
services, and the team discovers the violation six months later during a
migration that was supposed to be isolated.

### No Addressability

Specs reference each other by file path or ticket number — a brittle web
of string-based pointers that breaks whenever someone reorganizes a
directory, renames a file, or migrates from one tool to another. There is
no stable identity behind the reference, just a path that may or may not
resolve.

**Example:** Your auth spec lives at `specs/auth/login-spec.md` and twelve
other specs reference it by path. The team reorganizes `auth/` into
`identity/authn/` and `identity/authz/`. Every reference breaks silently.
The specs still render fine in the markdown viewer — the dangling references
quietly become lies.

### No Execution Interface

AI agents need structured task graphs with clear inputs, outputs,
dependencies, and completion criteria. Prose documents, even well-written
ones, do not provide that. An agent can read a spec, but it cannot
reliably extract what to build, what to build it on top of, what "done"
looks like, or where to report status.

**Example:** An orchestration agent receives "implement the feature
described in spec-42.md." It parses markdown and finds natural-language
requirements, a "Dependencies" section listing filenames it cannot resolve,
and acceptance criteria phrased as "the user should be able to..." Every
ambiguity becomes a decision the agent makes silently, without the context
to make it well.

### No Live Query

You cannot ask your spec repository a question. "What specs are currently
blocked?" "Which specs changed since the last release?" "Show me everything
downstream of the auth service." Answering these requires a custom script
that parses filenames, greps for keywords, and hopes the results are
accurate.

**Example:** A team lead preparing for a release wants the critical path —
which specs must complete before the release ships, and which are blocked.
With specs-as-files, they open each document, trace `depends_on`
references by hand, build a mental graph, and hope they have not missed a
transitive dependency three levels deep. With a live query layer, it is a
single traversal.

---

## The Cost

These gaps compound. A spec written without ground truth gets approved
without governance checks, handed to an agent that cannot query its
dependencies, which produces an implementation that conflicts with existing
patterns. Reviews catch some of the problems, rewrites follow, and the
cycle repeats until what should have been single-pass authoring has consumed
weeks. Multiply this across every spec in a growing project and teams stop
writing specs at all — not because specs lack value, but because maintaining
bad ones costs more than having none.

Agents amplify the problem. When agents make decisions without context,
they make them confidently and at scale. An agent that does not know your
architecture produces internally consistent work that is externally wrong.
The output looks correct until a human with project context reviews it and
discovers the agent built the right thing on the wrong foundation. At that
point you're not patching — you're redesigning.

---

## The Opportunity

How much this matters depends on team size. A solo developer with a
handful of specs can manage with markdown files in a repository, possibly
with a lightweight framework like
[Spec Kit](https://github.com/github/spec-kit) or
[GSD](https://github.com/gsd-build/get-shit-done). The gaps are real but
tolerable when one person holds the full context.

At team scale and above, specifications need infrastructure. When specs
live in a graph instead of a folder, the gaps close structurally:

- **Ground truth**: a layered constitution covering tech stack, principles,
  constraints. Agents query it before building. They never start cold.
- **Governance**: a spec that violates an architectural decision gets
  flagged before approval, not after deployment.
- **Addressability**: content-based identity. Rename files, reorganize
  folders — references hold.
- **Execution**: typed fields for dependencies, acceptance criteria, and
  completion status. Agents consume them directly.
- **Queryability**: "what's blocked?" is a graph traversal, not a grep
  script.

SpecGraph is an open-source implementation of these patterns. SDD
infrastructure you can run today. The patterns matter more than any
particular tool.

---

## What SDD Does About This

| Gap | SpecGraph answer |
|---|---|
| No Ground Truth | The constitution — layered architectural context every engineer and agent queries before building |
| No Governance | Constitution check and analytical passes enforce constraints at the spec layer, before code is written |
| No Addressability | Every spec has a stable ULID and slug — reorganise folders, rename files, references hold |
| No Execution Interface | Approved specs carry verify criteria, invariants, interface contracts, and typed dependencies — structured input for any executor |
| No Live Query | The spec graph answers `critical-path`, `impact`, `ready`, `drift` — direct traversal, not a custom script |

SpecGraph implements all five.

[:octicons-arrow-right-24: See how it works](how-it-works.md){ .md-button }
