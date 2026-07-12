<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { DecisionStatus } from '$lib/api/gen/specgraph/v1/decision_pb';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { statusBadgeClass } from '$lib/components/badge-variants';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  function statusLabel(status: DecisionStatus): string {
    switch (status) {
      case DecisionStatus.PROPOSED: return 'proposed';
      case DecisionStatus.ACCEPTED: return 'accepted';
      case DecisionStatus.DEPRECATED: return 'deprecated';
      case DecisionStatus.SUPERSEDED: return 'superseded';
      default: return 'unknown';
    }
  }
</script>

{#await data.detail}
  <!-- Loading: Skeleton title + metadata. Streamed promise re-suspends here on
       invalidateAll() so a switch returns to skeleton with no stale previous-project
       decision (Pitfall 3, T-05-05). -->
  <Skeleton class="mb-4 h-6 w-48" />
  <Skeleton class="mb-2 h-4 w-40" />
  <Skeleton class="mb-2 h-4 w-32" />
  <Skeleton class="h-4 w-36" />
{:then d}
  {#if d.loadError}
    <!-- Error: inline Retry card (do not reach +error.svelte, T-05-15). -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>Couldn't load decision.</Card.Title>
        <Card.Description>Check your connection and try again.</Card.Description>
      </Card.Header>
      <Card.Footer>
        <Button variant="outline" onclick={() => invalidateAll()}>Retry</Button>
      </Card.Footer>
    </Card.Root>
  {:else if !d.decision}
    <!-- Empty: decision not present in the current project (UI-SPEC copy). -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>Nothing here yet</Card.Title>
        <Card.Description>Decision not found in this project.</Card.Description>
      </Card.Header>
    </Card.Root>
  {:else}
    {@const decision = d.decision}
    <h1>{decision.title || decision.slug}</h1>

    <table class="meta">
      <tbody>
        <tr><td class="label">Slug</td><td>{decision.slug}</td></tr>
        <tr>
          <td class="label">Status</td>
          <td><Badge class={statusBadgeClass(statusLabel(decision.status))}>{statusLabel(decision.status)}</Badge></td>
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

    {#if d.linkedSpecs.length > 0}
      <section class="section">
        <h2>Referenced by</h2>
        <div class="spec-chips">
          {#each d.linkedSpecs as specSlug}
            <a href="/spec/{specSlug}" class="spec-chip">{specSlug}</a>
          {/each}
        </div>
      </section>
    {/if}
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
