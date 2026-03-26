<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { decisionClient, graphClient } from '$lib/api/client';
  import type { Decision } from '$lib/api/gen/specgraph/v1/decision_pb';
  import { DecisionStatus } from '$lib/api/gen/specgraph/v1/decision_pb';
  import { EdgeType } from '$lib/api/gen/specgraph/v1/graph_pb';

  let decision = $state<Decision | null>(null);
  let linkedSpecs = $state<string[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  let slug = $derived($page.params.slug);

  function statusLabel(status: DecisionStatus): string {
    switch (status) {
      case DecisionStatus.PROPOSED: return 'proposed';
      case DecisionStatus.ACCEPTED: return 'accepted';
      case DecisionStatus.DEPRECATED: return 'deprecated';
      case DecisionStatus.SUPERSEDED: return 'superseded';
      default: return 'unknown';
    }
  }

  async function loadDecision(s: string) {
    try {
      const resp = await decisionClient.getDecision({ slug: s });
      decision = resp.decision ?? null;
      // Linked specs are non-critical — fetch asynchronously without blocking render.
      graphClient.listEdges({ slug: s }).then(edgeResp => {
        linkedSpecs = edgeResp.edges
          .filter(e => e.edgeType === EdgeType.DECIDED_IN && e.toId === s)
          .map(e => e.fromId);
      }).catch(() => {
        linkedSpecs = [];
      });
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load decision';
    } finally {
      loading = false;
    }
  }

  onMount(() => { loadDecision(slug); });
</script>

<nav class="breadcrumb">
  <a href="/">Dashboard</a> / <a href="/graph">Graph</a> / <span>{slug}</span>
</nav>

{#if loading}
  <p class="status">Loading...</p>
{:else if error}
  <p class="status error">{error}</p>
{:else if decision}
  <h1>{decision.title || decision.slug}</h1>

  <table class="meta">
    <tbody>
      <tr><td class="label">Slug</td><td>{decision.slug}</td></tr>
      <tr>
        <td class="label">Status</td>
        <td><span class="badge status-{statusLabel(decision.status)}">{statusLabel(decision.status)}</span></td>
      </tr>
      {#if decision.supersededBy}
        <tr><td class="label">Superseded by</td><td>{decision.supersededBy}</td></tr>
      {/if}
      {#if decision.createdAt}
        <tr><td class="label">Created</td><td>{new Date(Number(decision.createdAt.seconds) * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}</td></tr>
      {/if}
      {#if decision.updatedAt}
        <tr><td class="label">Updated</td><td>{new Date(Number(decision.updatedAt.seconds) * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}</td></tr>
      {/if}
    </tbody>
  </table>

  {#if decision.decision}
    <section class="section">
      <h2>Decision</h2>
      <p class="body-text">{decision.decision}</p>
    </section>
  {/if}

  {#if decision.rationale}
    <section class="section">
      <h2>Rationale</h2>
      <p class="body-text">{decision.rationale}</p>
    </section>
  {/if}

  {#if linkedSpecs.length > 0}
    <section class="section">
      <h2>Referenced by</h2>
      <div class="spec-chips">
        {#each linkedSpecs as specSlug}
          <a href="/spec/{specSlug}" class="spec-chip">{specSlug}</a>
        {/each}
      </div>
    </section>
  {/if}
{/if}

<style>
  .breadcrumb {
    font-size: 0.85rem;
    color: #64748b;
    margin-bottom: 1.25rem;
  }

  .breadcrumb a {
    color: #2563eb;
    text-decoration: none;
  }

  .breadcrumb a:hover {
    text-decoration: underline;
  }

  .breadcrumb span {
    color: #1a1a2e;
    font-weight: 500;
  }

  h1 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0 0 1rem;
    color: #1a1a2e;
  }

  .status {
    color: #64748b;
    font-size: 0.95rem;
  }

  .status.error {
    color: #dc2626;
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

  .badge {
    display: inline-block;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.8rem;
    font-weight: 600;
    background: #f1f5f9;
    color: #475569;
  }

  .badge.status-proposed { background: #ede9fe; color: #7c3aed; }
  .badge.status-accepted { background: #ccfbf1; color: #0d9488; }
  .badge.status-deprecated { background: #f1f5f9; color: #6b7280; }
  .badge.status-superseded { background: #f3f4f6; color: #9ca3af; }

  .section {
    margin-top: 1.25rem;
  }

  h2 {
    font-size: 1rem;
    font-weight: 600;
    margin: 0 0 0.5rem;
    color: #1a1a2e;
  }

  .body-text {
    color: #374151;
    font-size: 0.9rem;
    line-height: 1.6;
    white-space: pre-wrap;
  }

  .spec-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .spec-chip {
    background: #eff6ff;
    color: #2563eb;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.8rem;
    text-decoration: none;
    font-weight: 500;
  }

  .spec-chip:hover {
    background: #dbeafe;
  }
</style>
