<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { specClient } from '$lib/api/client';
  import type { Spec } from '$lib/api/gen/specgraph/v1/spec_pb';
  import type { Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
  import { EdgeType } from '$lib/api/gen/specgraph/v1/graph_pb';
  import { SliceStatus } from '$lib/api/gen/specgraph/v1/slice_pb';
  import { ScopeSniff, DecompositionStrategy } from '$lib/api/gen/specgraph/v1/authoring_pb';
  import type { ChangeLogEntry } from '$lib/api/gen/specgraph/v1/spec_pb';
  import AccordionSection from '$lib/components/AccordionSection.svelte';
  import MetadataBar from '$lib/components/MetadataBar.svelte';
  import FindingsSection from '$lib/components/FindingsSection.svelte';
  import ChangelogTimeline from '$lib/components/ChangelogTimeline.svelte';
  import VersionCompare from '$lib/components/VersionCompare.svelte';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { stageBadgeClass } from '$lib/components/badge-variants';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  // Changelog is lazy-loaded on demand (user-triggered), NOT part of the switch
  // refetch. Reset the cache whenever the load re-runs — `data.detail` is a fresh
  // promise on every slug change AND every invalidateAll(), so keying the reset on
  // its reference clears stale changelog on both switches (T-05-05).
  let changelogEntries = $state<ChangeLogEntry[]>([]);
  let changelogLoading = $state(false);
  let changelogLoaded = $state(false);

  $effect(() => {
    data.detail; // track the streamed promise reference
    changelogEntries = [];
    changelogLoading = false;
    changelogLoaded = false;
  });

  async function loadChangelog(slug: string) {
    if (changelogLoaded) return;
    changelogLoading = true;
    try {
      const resp = await specClient.listChanges({ slug, limit: 0 });
      changelogEntries = resp.entries;
    } catch {
      changelogEntries = [];
    } finally {
      changelogLoading = false;
      changelogLoaded = true;
    }
  }

  function shouldExpand(spec: Spec, stage: string): boolean {
    return spec.stage === stage;
  }

  function scopeSniffLabel(val: ScopeSniff): string {
    const labels: Record<number, string> = {
      [ScopeSniff.TINY]: 'tiny',
      [ScopeSniff.SMALL]: 'small',
      [ScopeSniff.MEDIUM]: 'medium',
      [ScopeSniff.LARGE]: 'large',
      [ScopeSniff.EPIC]: 'epic',
    };
    return labels[val] ?? '';
  }

  function strategyLabel(val: DecompositionStrategy): string {
    const labels: Record<number, string> = {
      [DecompositionStrategy.VERTICAL_SLICE]: 'vertical slice',
      [DecompositionStrategy.LAYER_CAKE]: 'layer cake',
      [DecompositionStrategy.SINGLE_UNIT]: 'single unit',
    };
    return labels[val] ?? '';
  }

  function sliceStatusBadge(status: SliceStatus): { label: string; color: string } {
    switch (status) {
      case SliceStatus.OPEN: return { label: 'open', color: '#4b5563' };
      case SliceStatus.CLAIMED: return { label: 'claimed', color: '#c2410c' };
      case SliceStatus.DONE: return { label: 'done', color: '#15803d' };
      default: return { label: 'unknown', color: '#4b5563' };
    }
  }

  function edgeTypeLabel(val: EdgeType): string {
    const labels: Record<number, string> = {
      [EdgeType.DEPENDS_ON]: 'Depends on',
      [EdgeType.BLOCKS]: 'Blocks',
      [EdgeType.COMPOSES]: 'Composes',
      [EdgeType.RELATES_TO]: 'Relates to',
      [EdgeType.INFORMS]: 'Informs',
      [EdgeType.DECIDED_IN]: 'Decision',
      [EdgeType.SUPERSEDES]: 'Supersedes',
    };
    return labels[val] ?? String(val);
  }

  interface EdgeDisplay {
    target: string;
    route: string; // '/spec/' or '/decision/'
    label: string; // direction-aware label
  }

  // Direction-aware edge grouping. A plain function (not $derived) so it composes
  // with the resolved load data inside {#await} — keyed on the spec's own slug.
  function groupEdges(edges: Edge[], slug: string): Record<string, EdgeDisplay[]> {
    return edges.reduce((acc, e) => {
      const isOutgoing = e.fromId === slug;
      const target = isOutgoing ? e.toId : e.fromId;
      // Decision edges link to /decision/, all others to /spec/
      const isDecisionEdge = e.edgeType === EdgeType.DECIDED_IN;
      const route = isDecisionEdge ? '/decision/' : '/spec/';
      // Direction-aware labels for directed relationships
      let label: string;
      switch (e.edgeType) {
        case EdgeType.DEPENDS_ON: label = isOutgoing ? 'Depends on' : 'Depended on by'; break;
        case EdgeType.BLOCKS: label = isOutgoing ? 'Blocks' : 'Blocked by'; break;
        case EdgeType.COMPOSES: label = isOutgoing ? 'Composes' : 'Composed by'; break;
        case EdgeType.INFORMS: label = isOutgoing ? 'Informs' : 'Informed by'; break;
        case EdgeType.DECIDED_IN: label = isOutgoing ? 'Decision' : 'Decided in'; break;
        case EdgeType.SUPERSEDES: label = isOutgoing ? 'Supersedes' : 'Superseded by'; break;
        default: label = edgeTypeLabel(e.edgeType); break;
      }
      if (!acc[label]) acc[label] = [];
      acc[label].push({ target, route, label });
      return acc;
    }, {} as Record<string, EdgeDisplay[]>);
  }
</script>

{#await data.detail}
  <!-- Loading: Skeleton title + metadata + section rows. Streamed promise
       re-suspends here on invalidateAll() so a switch returns to skeleton with no
       stale previous-project spec (Pitfall 3, T-05-05). -->
  <Skeleton class="mb-4 h-6 w-48" />
  <Skeleton class="mb-2 h-4 w-full max-w-md" />
  <Skeleton class="mb-6 h-4 w-40" />
  <div class="space-y-2">
    <Skeleton class="h-10 w-full" />
    <Skeleton class="h-10 w-full" />
    <Skeleton class="h-10 w-full" />
  </div>
{:then d}
  {#if d.loadError}
    <!-- Error: inline Retry card (do not reach +error.svelte, T-05-15). -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>Couldn't load spec.</Card.Title>
        <Card.Description>Check your connection and try again.</Card.Description>
      </Card.Header>
      <Card.Footer>
        <Button variant="outline" onclick={() => invalidateAll()}>Retry</Button>
      </Card.Footer>
    </Card.Root>
  {:else if !d.spec}
    <!-- Empty: spec not present in the current project (UI-SPEC copy). -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>Nothing here yet</Card.Title>
        <Card.Description>Spec not found in this project.</Card.Description>
      </Card.Header>
    </Card.Root>
  {:else}
    {@const spec = d.spec}
    {@const groupedEdges = groupEdges(d.edges, spec.slug)}
    <h1>{spec.slug}</h1>

    <MetadataBar
      createdAt={spec.createdAt}
      updatedAt={spec.updatedAt}
      provenanceType={spec.provenanceType}
      contentHash={spec.contentHash}
    />

    <table class="meta">
      <tbody>
        <tr><td class="label">Intent</td><td>{spec.intent}</td></tr>
        <tr><td class="label">Stage</td><td><Badge class={stageBadgeClass(spec.stage)}>{spec.stage}</Badge></td></tr>
        <tr><td class="label">Priority</td><td>{spec.priority || '—'}</td></tr>
        <tr><td class="label">Complexity</td><td>{spec.complexity || '—'}</td></tr>
        <tr><td class="label">Version</td><td>{spec.version}</td></tr>
      </tbody>
    </table>

    {#if spec.supersededBy}
      <div class="lifecycle-banner superseded-banner">
        This spec has been superseded by
        <a href="/spec/{spec.supersededBy}">{spec.supersededBy}</a>
      </div>
    {/if}
    {#if spec.supersedes}
      <div class="lifecycle-banner supersedes-banner">
        This spec supersedes
        <a href="/spec/{spec.supersedes}">{spec.supersedes}</a>
      </div>
    {/if}

    <div class="sections">
      {#if spec.notes}
        <AccordionSection title="Notes" expanded={true}>
          <p class="notes">{spec.notes}</p>
        </AccordionSection>
      {/if}

      {#if spec.sparkOutput}
        <AccordionSection title="Spark" expanded={shouldExpand(spec, 'spark')}>
          {#if spec.sparkOutput.seed}
            <blockquote><strong>Seed:</strong> {spec.sparkOutput.seed}</blockquote>
          {/if}
          {#if spec.sparkOutput.signal}
            <blockquote><strong>Signal:</strong> {spec.sparkOutput.signal}</blockquote>
          {/if}
          {#if scopeSniffLabel(spec.sparkOutput.scopeSniff)}
            <p><strong>Scope Sniff:</strong> {scopeSniffLabel(spec.sparkOutput.scopeSniff)}</p>
          {/if}
          {#if spec.sparkOutput.killTest}
            <p><strong>Kill Test:</strong> {spec.sparkOutput.killTest}</p>
          {/if}
          {#if spec.sparkOutput.questions.length > 0}
            <p><strong>Questions:</strong></p>
            <ul>{#each spec.sparkOutput.questions as q}<li>{q}</li>{/each}</ul>
          {/if}
        </AccordionSection>
      {/if}

      {#if spec.shapeOutput}
        <AccordionSection title="Shape" expanded={shouldExpand(spec, 'shape')}>
          {#if spec.shapeOutput.scopeIn.length > 0}
            <p><strong>Scope In:</strong></p>
            <ul>{#each spec.shapeOutput.scopeIn as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.scopeOut.length > 0}
            <p><strong>Scope Out:</strong></p>
            <ul>{#each spec.shapeOutput.scopeOut as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.approaches.length > 0}
            <h3>Approaches</h3>
            {#each spec.shapeOutput.approaches as approach}
              <div class="approach" class:chosen={approach.name === spec.shapeOutput?.chosenApproach}>
                <strong>{approach.name}</strong>
                {#if approach.name === spec.shapeOutput?.chosenApproach}<span class="chosen-badge">chosen</span>{/if}
                {#if approach.description}<p>{approach.description}</p>{/if}
                {#if approach.tradeoffs.length > 0}
                  <ul class="tradeoffs">{#each approach.tradeoffs as t}<li>{t}</li>{/each}</ul>
                {/if}
              </div>
            {/each}
          {/if}
          {#if spec.shapeOutput.risks.length > 0}
            <h3>Risks</h3>
            <ul>{#each spec.shapeOutput.risks as r}<li>{r}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.successMust.length > 0}
            <h3>Success Criteria</h3>
            <p><strong>Must:</strong></p>
            <ul>{#each spec.shapeOutput.successMust as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.successShould.length > 0}
            <p><strong>Should:</strong></p>
            <ul>{#each spec.shapeOutput.successShould as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.successWont.length > 0}
            <p><strong>Won't:</strong></p>
            <ul>{#each spec.shapeOutput.successWont as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.decisions.length > 0}
            <h3>Decisions</h3>
            <ul>
              {#each spec.shapeOutput.decisions as dec}
                <li>
                  {#if dec.slug}<a href="/decision/{dec.slug}">{dec.title || dec.slug}</a>{:else}{dec.title}{/if}
                  {#if dec.rationale} — {dec.rationale}{/if}
                </li>
              {/each}
            </ul>
          {/if}
        </AccordionSection>
      {/if}

      {#if spec.specifyOutput}
        <AccordionSection title="Specify" expanded={shouldExpand(spec, 'specify')}>
          {#if spec.specifyOutput.interfaces.length > 0}
            <h3>Interfaces</h3>
            {#each spec.specifyOutput.interfaces as iface}
              <div class="interface-section">
                <strong>{iface.name}</strong>
                <pre>{iface.body}</pre>
              </div>
            {/each}
          {/if}
          {#if spec.specifyOutput.verifyCriteria.length > 0}
            <h3>Verify Criteria</h3>
            <table class="detail-table">
              <thead><tr><th>Category</th><th>Description</th></tr></thead>
              <tbody>
                {#each spec.specifyOutput.verifyCriteria as vc}
                  <tr><td>{vc.category}</td><td>{vc.description}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
          {#if spec.specifyOutput.invariants.length > 0}
            <h3>Invariants</h3>
            <ul>{#each spec.specifyOutput.invariants as inv}<li>{inv}</li>{/each}</ul>
          {/if}
          {#if spec.specifyOutput.touches.length > 0}
            <h3>File Touches</h3>
            <table class="detail-table">
              <thead><tr><th>Path</th><th>Purpose</th><th>Action</th></tr></thead>
              <tbody>
                {#each spec.specifyOutput.touches as ft}
                  <tr><td><code>{ft.path}</code></td><td>{ft.purpose}</td><td>{ft.changeType}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </AccordionSection>
      {/if}

      {#if spec.decomposeOutput}
        <AccordionSection title="Decompose" badge={d.slices.length ? d.slices.length + ' slices' : 'output'} expanded={shouldExpand(spec, 'decompose')}>
          {#if strategyLabel(spec.decomposeOutput.strategy)}
            <p><strong>Strategy:</strong> {strategyLabel(spec.decomposeOutput.strategy)}</p>
          {/if}
          {#if d.slices.length > 0}
            {#each d.slices as slice}
              {@const badge = sliceStatusBadge(slice.status)}
              <div class="slice-card">
                <div class="slice-header">
                  <strong>{slice.sliceId}</strong>
                  <span class="slice-badge" style="background:{badge.color}">{badge.label}</span>
                </div>
                {#if slice.intent}<p>{slice.intent}</p>{/if}
                {#if slice.assignedTo}
                  <p class="slice-label">Assigned to: {slice.assignedTo}</p>
                {/if}
                {#if slice.verify.length > 0}
                  <p class="slice-label">Verify:</p>
                  <ul>{#each slice.verify as v}<li>{v}</li>{/each}</ul>
                {/if}
                {#if slice.dependsOn.length > 0}
                  <p class="slice-label">Depends on: {slice.dependsOn.join(', ')}</p>
                {/if}
              </div>
            {/each}
          {:else if spec.decomposeOutput.slices.length > 0}
            {#each spec.decomposeOutput.slices as slice}
              <div class="slice-card">
                <strong>{slice.id}</strong>
                {#if slice.intent}<p>{slice.intent}</p>{/if}
                {#if slice.verify.length > 0}
                  <p class="slice-label">Verify:</p>
                  <ul>{#each slice.verify as v}<li>{v}</li>{/each}</ul>
                {/if}
                {#if slice.dependsOn.length > 0}
                  <p class="slice-label">Depends on: {slice.dependsOn.join(', ')}</p>
                {/if}
              </div>
            {/each}
          {:else if spec.decomposeOutput.sliceSlugs.length > 0}
            <p class="slice-label">{spec.decomposeOutput.sliceSlugs.length} slice(s) — loading details</p>
          {/if}
        </AccordionSection>
      {/if}

      {#if d.edges.length > 0}
        <AccordionSection title="Edges" badge={String(d.edges.length)}>
          {#each Object.entries(groupedEdges) as [label, items]}
            <p><strong>{label}:</strong></p>
            <ul>
              {#each items as item}
                <li><a href="{item.route}{item.target}">{item.target}</a></li>
              {/each}
            </ul>
          {/each}
        </AccordionSection>
      {/if}

      {#if d.findings.length > 0}
        <AccordionSection title="Findings" badge={String(d.findings.length)}>
          <FindingsSection findings={d.findings} />
        </AccordionSection>
      {/if}

      {#if spec.conversationLogs.length > 0}
        <AccordionSection title="Conversations" badge={String(spec.conversationLogs.length)}>
          {#each spec.conversationLogs as log}
            <div class="conversation-log">
              <h4>{log.stage} (v{log.version}{log.isAmend ? ', amend' : ''})</h4>
              {#each log.exchanges as ex}
                <div class="exchange">
                  <span class="role" class:probe={ex.role === 'probe'} class:response={ex.role === 'response'}>
                    {ex.role === 'probe' ? 'Probe' : ex.role === 'response' ? 'Response' : ex.role}:
                  </span>
                  <span>{ex.content}</span>
                  {#if ex.decisionPoint}<span class="decision-marker">decision</span>{/if}
                </div>
              {/each}
            </div>
          {/each}
        </AccordionSection>
      {/if}

      <AccordionSection title="Changelog" badge={changelogLoaded ? String(changelogEntries.length) : '…'}>
        {#if !changelogLoaded}
          <button class="load-changelog-btn" onclick={() => loadChangelog(spec.slug)}>Load changelog</button>
        {:else}
          <VersionCompare slug={spec.slug} entries={changelogEntries} />
          <ChangelogTimeline entries={changelogEntries} loading={changelogLoading} />
        {/if}
      </AccordionSection>
    </div>
  {/if}
{/await}

<style>
  h1 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0 0 1rem;
    color: #1a1a2e;
  }

  .meta {
    border-collapse: collapse;
    font-size: 0.9rem;
    margin-bottom: 1.25rem;
  }

  .meta td {
    padding: 0.4rem 1rem 0.4rem 0;
    vertical-align: top;
  }

  .meta .label {
    color: #64748b;
    font-weight: 500;
    white-space: nowrap;
    min-width: 8rem;
  }

  .notes {
    color: #374151;
    font-size: 0.9rem;
    line-height: 1.6;
    white-space: pre-wrap;
  }

  .sections {
    margin-top: 1rem;
  }

  blockquote {
    margin: 0.5rem 0;
    padding: 0.5rem 0.75rem;
    border-left: 3px solid #e2e8f0;
    color: #475569;
    font-size: 0.9rem;
  }

  h3 {
    font-size: 0.9rem;
    font-weight: 600;
    color: #374151;
    margin: 0.75rem 0 0.25rem;
  }

  h4 {
    font-size: 0.85rem;
    font-weight: 600;
    color: #475569;
    margin: 0.5rem 0 0.25rem;
  }

  ul {
    margin: 0.25rem 0 0.5rem;
    padding-left: 1.25rem;
  }

  li {
    font-size: 0.9rem;
    margin-bottom: 0.15rem;
  }

  .approach {
    padding: 0.5rem;
    margin: 0.25rem 0;
    border-radius: 4px;
    background: #f8fafc;
  }

  .approach.chosen {
    background: #eff6ff;
    border-left: 3px solid #2563eb;
  }

  .chosen-badge {
    font-size: 0.7rem;
    background: #2563eb;
    color: white;
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
    margin-left: 0.5rem;
  }

  .tradeoffs {
    font-size: 0.85rem;
    color: #64748b;
  }

  .interface-section {
    margin: 0.5rem 0;
  }

  pre {
    background: #f8fafc;
    padding: 0.5rem;
    border-radius: 4px;
    font-size: 0.8rem;
    overflow-x: auto;
    white-space: pre-wrap;
  }

  .detail-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    margin: 0.25rem 0 0.5rem;
  }

  .detail-table th {
    text-align: left;
    padding: 0.3rem 0.5rem;
    background: #f1f5f9;
    color: #475569;
    font-weight: 600;
  }

  .detail-table td {
    padding: 0.3rem 0.5rem;
    border-bottom: 1px solid #f1f5f9;
  }

  code {
    background: #f1f5f9;
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
    font-size: 0.8rem;
  }

  .slice-card {
    padding: 0.5rem 0.75rem;
    margin: 0.25rem 0;
    background: #f8fafc;
    border-radius: 6px;
    border-left: 3px solid #ca8a04;
  }

  .slice-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .slice-badge {
    font-size: 0.7rem;
    color: white;
    padding: 0.1rem 0.4rem;
    border-radius: 9999px;
    font-weight: 500;
  }

  .slice-label {
    font-size: 0.85rem;
    color: #64748b;
    margin: 0.25rem 0 0;
  }

  .conversation-log {
    margin-bottom: 0.75rem;
  }

  .exchange {
    margin: 0.2rem 0;
    font-size: 0.85rem;
  }

  .role {
    font-weight: 600;
  }

  .role.probe {
    color: #7c3aed;
  }

  .role.response {
    color: #059669;
  }

  .lifecycle-banner {
    padding: 0.75rem 1rem;
    border-radius: 0.5rem;
    margin-bottom: 1rem;
    font-size: 0.9rem;
    font-weight: 500;
  }
  .lifecycle-banner a {
    font-weight: 700;
    text-decoration: underline;
  }
  .superseded-banner {
    background: #fef3c7;
    border: 1px solid #f59e0b;
    color: #92400e;
  }
  .supersedes-banner {
    background: #dbeafe;
    border: 1px solid #3b82f6;
    color: #1e40af;
  }
  .load-changelog-btn {
    padding: 0.375rem 0.75rem;
    font-size: 0.85rem;
    background: var(--accent-color, #6366f1);
    color: #fff;
    border: none;
    border-radius: 0.375rem;
    cursor: pointer;
  }

  .decision-marker {
    font-size: 0.7rem;
    background: #fef3c7;
    color: #b45309;
    padding: 0.05rem 0.3rem;
    border-radius: 3px;
    margin-left: 0.3rem;
  }
</style>
