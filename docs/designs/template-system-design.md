# [System Name]

| | |
|---|---|
| **Status** | DRAFT / RFC / APPROVED / SUPERSEDED |
| **Owner** | [Name] |
| **Date** | [Created date] |
| **Intent** | [One sentence: what does this system/change achieve and for whom?] |

## Context & Signal

*Why are we doing this now? What triggered this work?*

[2-4 sentences. Link to the audit finding, incident, customer feedback, roadmap item, or technical constraint that made this work necessary. If this work could reasonably be killed or deferred, say what would cause that.]

## Scope

### In

- [What this design delivers — concrete, bounded outcomes]
- [...]

### Out

- [What this explicitly does NOT deliver — and why]
- [Reference related work by name if it's tracked elsewhere]

## Approach & Decisions

*What options did we consider and what did we choose?*

| Approach | Description | Tradeoffs |
|----------|-------------|-----------|
| [option-1] | [Brief description] | [Pros and cons] |
| [option-2] | [Brief description] | [Pros and cons] |

**Chosen:** [which option] — [1-2 sentences on why this won over the alternatives]

### Key Decisions

*Record decisions that affect other teams or have long-term consequences. These prevent "why didn't we just..." conversations months from now.*

| Decision | Rationale |
|----------|-----------|
| [e.g., "gRPC over REST"] | [Why — the constraint or tradeoff that drove this] |

## Success Criteria

### Must (ship-blockers)

- [If this doesn't work, the project isn't done]

### Should (strong expectations)

- [We'd fix this before calling it good, but it won't block launch]

### Won't (explicitly deferred)

- [We considered this and decided not to do it — here's why]

## Dependencies & Risks

*What external things could block, change, or break this?*

| Dependency / Risk | Impact | Mitigation |
|-------------------|--------|------------|
| [e.g., "Upstream API not finalized"] | [Blocks implementation of X] | [Design to interface; stub until available] |

## Open Questions

*What's unresolved? Mark owners and target dates so these don't stall.*

| # | Question | Owner | Target Date | Resolution |
|---|----------|-------|-------------|------------|
| 1 | [e.g., "Cache technology — Redis vs Memcached?"] | [Name] | [Date] | [Fill in when decided] |

---

*When this design is approved, add deeper topics (API design, data model, deployment, failure modes, etc.) as the team needs them — not before.*
