# Confluence Publishing for Specs

| | |
|---|---|
| **Status** | RFC / DRAFT |
| **Owner** | Sean Brandt |
| **Date** | 2026-04-10 |
| **Intent** | Publish specs to Confluence as PRDs, SDDs, and ADRs in native ADF format, with bidirectional comment flow, so Product and Engineering stakeholders can review specs in a familiar environment while SpecGraph remains the source of truth. |

## Context & Signal

SpecGraph captures rich structured specs through its authoring funnel, but the audience for those specs — Product Managers, Tech Leads, stakeholders — lives in Confluence. Today there's no way to project specs into Confluence without manual copy-paste, which drifts immediately.

The existing [Confluence-to-SpecGraph Design Bridge](2026-03-26-confluence-to-specgraph-design-bridge.md) covers the inbound direction (teams authoring designs in Confluence before SpecGraph is ready). This design covers the outbound direction: SpecGraph as source of truth, Confluence as a read-optimized projection.

Using Atlassian Document Format (ADF) rather than Confluence wiki markup or converted Markdown ensures full fidelity — native macros (status lozenges, panels, expand sections, page-properties), proper table structures, and code blocks with language hints.

This work could be killed if teams don't use Confluence or if a lighter-weight export (e.g., static-site markdown) proves sufficient.

## Scope

### In

- Renderer interface with Markdown and ADF implementations
- Three document types: PRD (Spark + Shape), SDD (Specify + Decompose), ADR (MADR format)
- Publisher interface with Confluence as first implementation
- FeedbackSource interface for comment ingestion
- Auto-publish on stage transitions, idempotent updates with appended changelogs
- Comment ingestion: inline comments routed by ADF section to funnel stage, footer comments as general notes, question heuristic for open questions
- CLI commands: `confluence publish`, `confluence status`, `confluence sync-comments`, `confluence unpublish`
- JSON and Markdown output for all read commands
- Proto service definition (`PublishService`)
- Skill updates for CLI discoverability

### Out

- Binary plugin system (go-plugin or similar) — interfaces are designed for future extraction but dispatch is in-process for now
- Real-time Confluence webhooks — polling is sufficient; webhooks can be added later without interface changes
- Confluence-to-SpecGraph ingestion (covered by the Design Bridge doc)
- Publishing to systems other than Confluence — interfaces support it, but only Confluence is implemented
- Automated spec state changes from comments — comments are informational input, humans decide what to act on

## Approach & Decisions

| Approach | Description | Tradeoffs |
|----------|-------------|-----------|
| Extend existing sync adapter | Add `ConfluenceAdapter` implementing the existing `sync.Adapter` interface (Push/Pull/FindOrCreate) | Follows existing patterns. But the 1:1 interface (one spec = one external entity) doesn't fit Confluence's 1:N page tree model. Comment ingestion is structurally different from issue-status polling. |
| Dedicated publish package (chosen) | New `internal/publish/` with `Renderer`, `Publisher`, `FeedbackSource` interfaces. `internal/render/` refactored with `markdown/` and `adf/` sub-packages both implementing `Renderer`. `internal/publish/confluence/` as first Publisher implementation. | Purpose-built for 1:N document trees. Clean separation: rendering is pure transformation, publishing is external system management, feedback is inbound flow. More surface area, but it's warranted — these are genuinely different capabilities from issue-tracker sync. |
| Render backend + thin sync | Extend `internal/render/` with format parameter, keep sync adapter thin | Markdown and ADF are structurally different (string concat vs. JSON tree). Shared interface would serve neither format well. Splits Confluence concern across three packages. |

**Chosen:** Dedicated publish package — the Confluence integration's requirements (1:N page trees, bidirectional comments, ADF format) are different enough from issue-tracker sync to warrant their own domain.

### Key Decisions

| Decision | Rationale |
|----------|-----------|
| ADF format, not wiki markup or converted Markdown | Full fidelity with Confluence's native rendering. Enables macros (status lozenges, panels, expand, page-properties) that make PRDs look like hand-crafted Confluence pages, not imported docs. |
| Go interfaces, not binary plugins | Clean package boundaries give the option to extract into go-plugin later. But with one implementation, binary plugins would be optimizing for a distribution problem that doesn't exist yet. The interface IS the plugin contract; dispatch mechanism is plumbing. |
| Renderer interface shared by Markdown and ADF | Both implement the same interface for document-level rendering (PRD, SDD, ADR). Enables CLI preview (`specgraph render prd <slug>` in Markdown) using the same composition logic as Confluence publishing. |
| SpecGraph is source of truth, Confluence is projection | Confluence pages are overwritten on spec updates (with changelog appended). Page ID mapping lives in SpecGraph storage. Comments are ingested as feedback, not as authoritative edits. |
| PRD = Spark + Shape, SDD = Specify + Decompose | Natural split between "should we build this?" (PM audience) and "how do we build it?" (engineering audience). Mirrors the Design Bridge's deliberate omission of Specify/Decompose from the PM-facing template. |
| MADR format for ADRs | SpecGraph's Decision proto already has the fields for MADR (rejected alternatives, confidence, scope). Classic Nygard format would discard data the system already captures. |
| Comments don't mutate spec state | Comments are informational input surfaced to authors. Automatic state changes from external comments would be dangerous — a casual question shouldn't trigger a stage transition. |

## Architecture

### Package Layout

```
internal/
├── render/
│   ├── render.go              # Renderer interface
│   ├── document.go            # Document type, DocumentKind (PRD, SDD, ADR)
│   ├── markdown/
│   │   ├── prd.go             # PRD document renderer
│   │   ├── sdd.go             # SDD document renderer
│   │   ├── adr.go             # ADR document renderer (MADR)
│   │   ├── spec.go            # Entity-level (existing, for CLI show commands)
│   │   ├── decision.go        # Entity-level (existing)
│   │   ├── ...                # Rest of existing entity renderers
│   │   └── helpers.go         # metadataTable(), section(), etc.
│   └── adf/
│       ├── builder.go         # ADF node tree builder (fluent API)
│       ├── prd.go             # PRD document renderer
│       ├── sdd.go             # SDD document renderer
│       ├── adr.go             # ADR document renderer (MADR)
│       └── macros.go          # Confluence macro nodes (status, panel, expand, page-properties)
├── publish/
│   ├── publisher.go           # Publisher interface
│   ├── feedback.go            # FeedbackSource interface
│   ├── orchestrator.go        # Stage transitions → render → publish
│   ├── registry.go            # Registry of publishers by name
│   └── confluence/
│       ├── client.go          # Confluence REST API client (ADF-native)
│       ├── publisher.go       # Publisher impl — page tree management
│       ├── feedback.go        # FeedbackSource impl — comment polling + routing
│       ├── config.go          # Cloud ID, space key, parent page, auth
│       └── mapping.go         # Page ID ↔ spec slug tracking
```

### Render Package Migration

The existing `internal/render/` package's entity-level renderers (spec.go, decision.go, etc.) move into `internal/render/markdown/`. Existing callers (CLI `show`, `list` commands) update their imports from `render.RenderSpec(...)` to `markdown.RenderSpec(...)`. This is a mechanical refactor — function signatures and behavior are unchanged. The `internal/render/` package root becomes the home for the `Renderer` interface and `Document` types only.

### Core Interfaces

```go
// Renderer transforms spec data into a structured document.
// Implementations are format-specific (ADF, Markdown, etc.)
type Renderer interface {
    RenderPRD(ctx context.Context, spec *v1.Spec) (Document, error)
    RenderSDD(ctx context.Context, spec *v1.Spec) (Document, error)
    RenderADR(ctx context.Context, decision *v1.Decision) (Document, error)
}

// Publisher manages document lifecycle in an external system.
type Publisher interface {
    Name() string
    Publish(ctx context.Context, slug string, docs []Document) (PublishResult, error)
    Update(ctx context.Context, slug string, docs []Document, changelog *v1.ChangeLogEntry) (PublishResult, error)
    Unpublish(ctx context.Context, slug string) error
    Status(ctx context.Context, slug string) (PublishStatus, error)
}

// FeedbackSource ingests external feedback back into SpecGraph.
type FeedbackSource interface {
    Poll(ctx context.Context, slug string) ([]Feedback, error)
}
```

### Document Types

**Document** is a format-agnostic container. The Publisher doesn't need to know whether it's ADF or Markdown:

```go
type DocumentKind int

const (
    DocumentPRD DocumentKind = iota
    DocumentSDD
    DocumentADR
)

type Document struct {
    Kind       DocumentKind
    Title      string
    Body       []byte       // Rendered content (ADF JSON, Markdown, etc.)
    ParentSlug string       // For SDD/ADR: the spec slug (determines parent page)
    DecisionID string       // For ADR only: the decision slug
    Metadata   map[string]string
}
```

### Page Tree Structure

```
[Space Root or Configured Parent Page]
└── PRD: <Spec Title>              ← auto-published on Shape completion
    ├── SDD: <Spec Title>          ← auto-published on Specify completion
    ├── ADR-001: <Decision Title>  ← auto-published when Decision linked
    └── ADR-002: ...
```

## Document Rendering

### PRD (Spark + Shape)

Targets Product stakeholders. Uses Confluence macros for rich presentation.

| Section | Source Fields | ADF Treatment |
|---------|-------------|---------------|
| Title + Status | `spec.slug`, `spec.stage` | Page title + `status` macro (color-coded by stage) |
| Intent | `spark.seed` | Opening paragraph |
| Context & Signal | `spark.signal`, `spark.kill_test` | Panel macro with signal + kill conditions |
| Scope | `shape.scope.in`, `shape.scope.out` | Two-column layout — In / Out |
| Approaches | `shape.approaches[]` | Expand macros — chosen approach open, alternatives collapsed |
| Decisions | Linked `Decision` nodes via DECIDED_IN edges | Table with links to child ADR pages |
| Success Criteria | `shape.success_criteria` | MoSCoW table (Must/Should/Won't) |
| Risks & Dependencies | `shape.risks[]`, DEPENDS_ON edges | Table with severity + mitigation |
| Change History | `changelog[]` | Appended section — newest first, `expand` macro per version |

### SDD (Specify + Decompose)

Targets Engineering. Child page of the PRD.

| Section | Source Fields | ADF Treatment |
|---------|-------------|---------------|
| Interface Contracts | `specify.sections[]` | Code blocks with language hints per interface |
| Acceptance Criteria | `specify.acceptance_criteria[]` | Checklist (task list nodes) |
| Invariants | `specify.invariants[]` | Warning panel macro |
| File Touches | `specify.file_touches[]` | Table with path + rationale |
| Decomposition Strategy | `decompose.strategy` | Info panel |
| Slices | `decompose.slices[]` | Table — intent, dependencies, verify criteria, status |
| Slice Dependency Graph | Computed from slice deps | PlantUML macro or smart-link diagram |

### ADR (MADR Format)

One child page per Decision node linked to the spec.

| Section | Source Fields | ADF Treatment |
|---------|-------------|---------------|
| Title | `decision.title` | `ADR-NNN: <title>` page title |
| Status | `decision.status` | Status macro (proposed/accepted/deprecated/superseded) |
| Context | `decision.question` | The problem statement |
| Decision | `decision.title` + body | What was decided |
| Considered Options | `decision.rejected_alternatives[]` + chosen | Table with pros/cons |
| Consequences | Derived from linked specs | Impact on dependent specs |
| Confidence | `decision.confidence` | Info panel with confidence level + scope |

### ADF Builder

Fluent API for constructing ADF node trees:

```go
doc := adf.NewDocument().
    Heading(1, "Scope").
    Table(
        adf.Row(adf.HeaderCell("In"), adf.HeaderCell("Out")),
        adf.Row(adf.Cell(inContent), adf.Cell(outContent)),
    ).
    Panel(adf.PanelInfo, riskContent).
    Expand("Rejected: Option B", rejectedDetail)
```

Output is the ADF JSON tree that goes directly into the Confluence REST API `body.atlas_doc_format` field.

## Publishing Lifecycle

### Auto-Publish Triggers

| Event | Action |
|-------|--------|
| Shape completed | Publish PRD page (create) |
| Specify completed | Publish SDD as child of PRD (create) |
| Decision linked (DECIDED_IN edge created) | Publish ADR as child of PRD (create) |
| Spec updated (new version) | Re-render affected pages, append changelog entry |
| Decision status change | Update ADR page |
| Spec lifecycle → done/abandoned | Update status macros on all pages in tree |

### Update Flow

```
Stage transition / spec update
  → Orchestrator detects change
  → Renderer produces updated Document(s)
  → Publisher.Update() called with docs + changelog entry
  → Publisher:
      1. Fetches current page version (Confluence requires version increment)
      2. Replaces body with new ADF
      3. Appends changelog entry to Change History section
      4. Confluence native versioning captures the before/after automatically
```

### Page ID Mapping

Stored in SpecGraph's storage layer (new table):

```go
type PageMapping struct {
    SpecSlug    string
    DocKind     render.DocumentKind
    DecisionID  string              // only for ADRs
    PageID      string              // Confluence page ID
    Version     int                 // last published Confluence version
    SpecVersion int32               // spec version at last publish
    LastSync    time.Time
}
```

### Idempotency

If a publish fails partway through (e.g., PRD updated but ADR create fails), the orchestrator can re-run safely. Each document in the tree is published independently. The mapping table tracks what succeeded.

## Comment Ingestion

### Inline Comments (Anchored to Text)

Routed by ADF section to the appropriate funnel stage:

| Comment Anchored In | Routes To | SpecGraph Entity |
|---|---|---|
| Intent / Context & Signal | Spark stage | Conversation log entry on spec |
| Scope / Approaches / Success Criteria | Shape stage | Conversation log entry on spec |
| Interface Contracts / Invariants | Specify stage | Conversation log entry on spec |
| Slices / Decomposition | Decompose stage | Conversation log entry on spec |
| Decision Context / Considered Options | Decision | Conversation log entry on decision |
| Contains `?` + unresolved | Any | Open question on spec (with commenter as owner) |

Section-to-stage mapping is deterministic because SpecGraph controls the ADF structure.

### Footer Comments (Page-Level)

No text anchor. Become general notes on the spec, tagged with the commenter's name and timestamp. Replies inherit the routing of their parent comment.

### Feedback Model

```go
type Feedback struct {
    ExternalID   string              // Confluence comment ID (dedup key)
    Author       string
    Body         string
    Timestamp    time.Time
    Kind         FeedbackKind        // Inline or Footer
    Stage        v1.Stage            // Routed stage (inline only)
    IsQuestion   bool                // Heuristic: contains '?' + unresolved
    ParentID     string              // Reply threading
}
```

### Polling

- Configurable per-publisher, default 15 minutes for specs in active stages
- Stopped for done/abandoned specs
- Deduplication by Confluence comment ID — re-polling returns only new/changed comments

### What Comments Do NOT Do

- Comments don't automatically change spec state or trigger stage transitions
- Comments don't modify spec fields
- The system surfaces feedback; humans (or authoring skills) decide what to act on

## Configuration

```yaml
publish:
  confluence:
    cloud_id: "abc-123"
    space_key: "ENG"
    parent_page_id: "12345"
    auth:
      method: "api_token"           # or "oauth2"
      # credentials via env vars: CONFLUENCE_API_TOKEN, CONFLUENCE_USER_EMAIL
    poll_interval: "15m"
    auto_publish: true
    labels:
      - "specgraph"
      - "auto-generated"
```

Auth credentials come from environment variables or system keyring — never in the config file.

## CLI Commands

```
specgraph confluence publish <slug>       # Manual publish/re-publish
specgraph confluence status [slug]        # Show publish state (all or one)
specgraph confluence sync-comments        # Poll comments now (all published specs)
specgraph confluence unpublish <slug>     # Remove pages from Confluence
```

All read commands support `--json` flag (via `printJSON`) and default table/markdown output, consistent with existing CLI patterns.

### Status Output (Default Table)

```
SLUG              PRD     SDD     ADRS    LAST SYNC            COMMENTS
auth-redesign     synced  synced  2/2     2026-04-10 14:30     3 new
payment-flow      synced  -       1/1     2026-04-10 14:30     0 new
search-v2         draft   -       0/0     2026-04-10 12:00     -
```

## Proto Service

New `PublishService` in `proto/specgraph/v1/publish.proto`:

```protobuf
service PublishService {
  rpc Publish(PublishRequest) returns (PublishResponse);
  rpc GetPublishStatus(GetPublishStatusRequest) returns (GetPublishStatusResponse);
  rpc SyncComments(SyncCommentsRequest) returns (SyncCommentsResponse);
  rpc Unpublish(UnpublishRequest) returns (UnpublishResponse);
}
```

Sits alongside existing services. The orchestrator's auto-publish logic calls the same RPCs internally.

## Plugin / Skill Updates

- Existing `specgraph-show` skill updated to note `specgraph confluence status` for publishing state
- New lightweight `specgraph-confluence` skill routes queries about publishing status and comments to the appropriate CLI commands

## Dependencies & Risks

| Dependency / Risk | Impact | Mitigation |
|-------------------|--------|------------|
| Confluence REST API v2 stability | Breaking API changes could require client updates | Pin to v2 API version, use versioned endpoints. ADF format itself is stable. |
| ADF schema evolution | New node types or macro changes | ADF builder is our code — add node types as needed. Existing nodes are stable. |
| Proto schema changes (Spark/Shape/Specify/Decompose outputs) | Renderer field mappings drift | Renderers reference proto fields directly — compiler catches removed/renamed fields. New fields require manual renderer updates. |
| Confluence rate limits | Bulk publishing could hit API limits | Publish documents sequentially within a page tree, with configurable backoff. Initial use case is individual spec transitions, not bulk operations. |
| Inline comment anchoring | If page content changes significantly, existing inline comments may lose their anchor | Confluence handles this natively (orphaned comments become page-level). Polling would pick them up as footer comments. Acceptable degradation. |
| Polling latency for comments | Up to 15 minutes delay for comment visibility in SpecGraph | Configurable interval. Webhooks can replace polling later without interface changes. |

## Open Questions

| # | Question | Owner | Target Date | Resolution |
|---|----------|-------|-------------|------------|
| 1 | Should the page-properties macro on PRDs enable a Page Properties Report on the parent page (dashboard of all published specs)? | Sean | | |
| 2 | Should published pages include a "Generated by SpecGraph — do not edit directly" warning banner? | Sean | | |
| 3 | Should the SDD include a rendered slice dependency graph (PlantUML macro), or is the table sufficient? | Sean | | |
| 4 | What Confluence permission model should published pages use — inherit from parent, or explicit restrictions? | Sean | | |
