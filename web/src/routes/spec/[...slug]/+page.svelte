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
    <Card.Root class="max-w-md" data-testid="error">
      <Card.Header>
        <Card.Title>Nothing here yet</Card.Title>
        <Card.Description>Spec not found in this project.</Card.Description>
      </Card.Header>
    </Card.Root>
  {:else}
    {@const spec = d.spec}
    {@const groupedEdges = groupEdges(d.edges, spec.slug)}
    <h1 class="mb-4 text-xl font-semibold text-foreground">{spec.slug}</h1>

    <MetadataBar
      createdAt={spec.createdAt}
      updatedAt={spec.updatedAt}
      provenanceType={spec.provenanceType}
      contentHash={spec.contentHash}
    />

    <table class="mb-5 border-collapse text-sm" data-testid="meta">
      <tbody>
        <tr><td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Intent</td><td class="py-1.5 pr-4 align-top">{spec.intent}</td></tr>
        <tr><td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Stage</td><td class="py-1.5 pr-4 align-top"><Badge data-testid="stage-badge" class={stageBadgeClass(spec.stage)}>{spec.stage}</Badge></td></tr>
        <tr><td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Priority</td><td class="py-1.5 pr-4 align-top">{spec.priority || '—'}</td></tr>
        <tr><td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Complexity</td><td class="py-1.5 pr-4 align-top">{spec.complexity || '—'}</td></tr>
        <tr><td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Version</td><td class="py-1.5 pr-4 align-top">{spec.version}</td></tr>
      </tbody>
    </table>

    {#if spec.supersededBy}
      <div data-testid="superseded-banner" class="mb-4 rounded-lg border border-amber-300 bg-amber-100 px-4 py-3 text-sm font-medium text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200">
        This spec has been superseded by
        <a class="font-bold underline" href="/spec/{spec.supersededBy}">{spec.supersededBy}</a>
      </div>
    {/if}
    {#if spec.supersedes}
      <div data-testid="supersedes-banner" class="mb-4 rounded-lg border border-blue-300 bg-blue-100 px-4 py-3 text-sm font-medium text-blue-900 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-200">
        This spec supersedes
        <a class="font-bold underline" href="/spec/{spec.supersedes}">{spec.supersedes}</a>
      </div>
    {/if}

    <div class="mt-4" data-testid="sections">
      {#if spec.notes}
        <AccordionSection title="Notes" expanded={true}>
          <p class="whitespace-pre-wrap text-sm leading-relaxed text-foreground">{spec.notes}</p>
        </AccordionSection>
      {/if}

      {#if spec.sparkOutput}
        <AccordionSection title="Spark" expanded={shouldExpand(spec, 'spark')}>
          {#if spec.sparkOutput.seed}
            <blockquote class="my-2 border-l-2 border-border py-2 pl-3 text-sm text-muted-foreground"><strong>Seed:</strong> {spec.sparkOutput.seed}</blockquote>
          {/if}
          {#if spec.sparkOutput.signal}
            <blockquote class="my-2 border-l-2 border-border py-2 pl-3 text-sm text-muted-foreground"><strong>Signal:</strong> {spec.sparkOutput.signal}</blockquote>
          {/if}
          {#if scopeSniffLabel(spec.sparkOutput.scopeSniff)}
            <p><strong>Scope Sniff:</strong> {scopeSniffLabel(spec.sparkOutput.scopeSniff)}</p>
          {/if}
          {#if spec.sparkOutput.killTest}
            <p><strong>Kill Test:</strong> {spec.sparkOutput.killTest}</p>
          {/if}
          {#if spec.sparkOutput.questions.length > 0}
            <p><strong>Questions:</strong></p>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each spec.sparkOutput.questions as q}<li>{q}</li>{/each}</ul>
          {/if}
        </AccordionSection>
      {/if}

      {#if spec.shapeOutput}
        <AccordionSection title="Shape" expanded={shouldExpand(spec, 'shape')}>
          {#if spec.shapeOutput.scopeIn.length > 0}
            <p><strong>Scope In:</strong></p>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each spec.shapeOutput.scopeIn as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.scopeOut.length > 0}
            <p><strong>Scope Out:</strong></p>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each spec.shapeOutput.scopeOut as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.approaches.length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Approaches</h3>
            {#each spec.shapeOutput.approaches as approach}
              <div class="my-1 rounded p-2 {approach.name === spec.shapeOutput?.chosenApproach ? 'border-l-2 border-primary bg-muted/50' : 'bg-muted'}">
                <strong>{approach.name}</strong>
                {#if approach.name === spec.shapeOutput?.chosenApproach}<span class="ml-2 rounded bg-primary px-1.5 py-0.5 text-[0.7rem] text-primary-foreground">chosen</span>{/if}
                {#if approach.description}<p>{approach.description}</p>{/if}
                {#if approach.tradeoffs.length > 0}
                  <ul class="my-1 ml-5 list-disc space-y-0.5 text-[0.85rem] text-muted-foreground">{#each approach.tradeoffs as t}<li>{t}</li>{/each}</ul>
                {/if}
              </div>
            {/each}
          {/if}
          {#if spec.shapeOutput.risks.length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Risks</h3>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each spec.shapeOutput.risks as r}<li>{r}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.successMust.length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Success Criteria</h3>
            <p><strong>Must:</strong></p>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each spec.shapeOutput.successMust as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.successShould.length > 0}
            <p><strong>Should:</strong></p>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each spec.shapeOutput.successShould as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.successWont.length > 0}
            <p><strong>Won't:</strong></p>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each spec.shapeOutput.successWont as s}<li>{s}</li>{/each}</ul>
          {/if}
          {#if spec.shapeOutput.decisions.length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Decisions</h3>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">
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
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Interfaces</h3>
            {#each spec.specifyOutput.interfaces as iface}
              <div class="my-2">
                <strong>{iface.name}</strong>
                <pre class="overflow-x-auto whitespace-pre-wrap rounded bg-muted p-2 text-xs">{iface.body}</pre>
              </div>
            {/each}
          {/if}
          {#if spec.specifyOutput.verifyCriteria.length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Verify Criteria</h3>
            <table class="my-1 w-full border-collapse text-sm">
              <thead><tr><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Category</th><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Description</th></tr></thead>
              <tbody>
                {#each spec.specifyOutput.verifyCriteria as vc}
                  <tr><td class="border-b border-border px-2 py-1.5">{vc.category}</td><td class="border-b border-border px-2 py-1.5">{vc.description}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
          {#if spec.specifyOutput.invariants.length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Invariants</h3>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each spec.specifyOutput.invariants as inv}<li>{inv}</li>{/each}</ul>
          {/if}
          {#if spec.specifyOutput.touches.length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">File Touches</h3>
            <table class="my-1 w-full border-collapse text-sm">
              <thead><tr><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Path</th><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Purpose</th><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Action</th></tr></thead>
              <tbody>
                {#each spec.specifyOutput.touches as ft}
                  <tr><td class="border-b border-border px-2 py-1.5"><code class="rounded bg-muted px-1 py-0.5 text-xs">{ft.path}</code></td><td class="border-b border-border px-2 py-1.5">{ft.purpose}</td><td class="border-b border-border px-2 py-1.5">{ft.changeType}</td></tr>
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
              <div class="my-1 rounded-md border-l-2 border-primary bg-muted px-3 py-2">
                <div class="flex items-center gap-2">
                  <strong>{slice.sliceId}</strong>
                  <span class="slice-badge rounded-full px-2 py-0.5 text-[0.7rem] font-medium text-white" style="background:{badge.color}">{badge.label}</span>
                </div>
                {#if slice.intent}<p>{slice.intent}</p>{/if}
                {#if slice.assignedTo}
                  <p class="mt-1 text-[0.85rem] text-muted-foreground">Assigned to: {slice.assignedTo}</p>
                {/if}
                {#if slice.verify.length > 0}
                  <p class="mt-1 text-[0.85rem] text-muted-foreground">Verify:</p>
                  <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each slice.verify as v}<li>{v}</li>{/each}</ul>
                {/if}
                {#if slice.dependsOn.length > 0}
                  <p class="mt-1 text-[0.85rem] text-muted-foreground">Depends on: {slice.dependsOn.join(', ')}</p>
                {/if}
              </div>
            {/each}
          {:else if spec.decomposeOutput.slices.length > 0}
            {#each spec.decomposeOutput.slices as slice}
              <div class="my-1 rounded-md border-l-2 border-primary bg-muted px-3 py-2">
                <strong>{slice.id}</strong>
                {#if slice.intent}<p>{slice.intent}</p>{/if}
                {#if slice.verify.length > 0}
                  <p class="mt-1 text-[0.85rem] text-muted-foreground">Verify:</p>
                  <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">{#each slice.verify as v}<li>{v}</li>{/each}</ul>
                {/if}
                {#if slice.dependsOn.length > 0}
                  <p class="mt-1 text-[0.85rem] text-muted-foreground">Depends on: {slice.dependsOn.join(', ')}</p>
                {/if}
              </div>
            {/each}
          {:else if spec.decomposeOutput.sliceSlugs.length > 0}
            <p class="mt-1 text-[0.85rem] text-muted-foreground">{spec.decomposeOutput.sliceSlugs.length} slice(s) — loading details</p>
          {/if}
        </AccordionSection>
      {/if}

      {#if d.edges.length > 0}
        <AccordionSection title="Edges" badge={String(d.edges.length)}>
          {#each Object.entries(groupedEdges) as [label, items]}
            <p><strong>{label}:</strong></p>
            <ul class="my-1 ml-5 list-disc space-y-0.5 text-sm">
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
            <div class="mb-3">
              <h4 class="mb-1 mt-2 text-[0.85rem] font-semibold text-muted-foreground">{log.stage} (v{log.version}{log.isAmend ? ', amend' : ''})</h4>
              {#each log.exchanges as ex}
                <div class="my-0.5 text-[0.85rem]">
                  <span
                    class="font-semibold {ex.role === 'probe'
                      ? 'text-violet-700 dark:text-violet-300'
                      : ex.role === 'response'
                        ? 'text-emerald-700 dark:text-emerald-300'
                        : ''}"
                  >
                    {ex.role === 'probe' ? 'Probe' : ex.role === 'response' ? 'Response' : ex.role}:
                  </span>
                  <span>{ex.content}</span>
                  {#if ex.decisionPoint}<span class="ml-1 rounded bg-amber-100 px-1.5 py-0.5 text-[0.7rem] text-amber-900 dark:bg-amber-950 dark:text-amber-200">decision</span>{/if}
                </div>
              {/each}
            </div>
          {/each}
        </AccordionSection>
      {/if}

      <AccordionSection title="Changelog" badge={changelogLoaded ? String(changelogEntries.length) : '…'}>
        {#if !changelogLoaded}
          <Button variant="outline" size="sm" onclick={() => loadChangelog(spec.slug)}>Load changelog</Button>
        {:else}
          <VersionCompare slug={spec.slug} entries={changelogEntries} />
          <ChangelogTimeline entries={changelogEntries} loading={changelogLoading} />
        {/if}
      </AccordionSection>
    </div>
  {/if}
{/await}
