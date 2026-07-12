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
    <h1 class="mb-4 text-xl font-semibold text-foreground">{decision.title || decision.slug}</h1>

    <table class="mb-5 border-collapse text-sm">
      <tbody>
        <tr><td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Slug</td><td class="py-1.5 pr-4 align-top">{decision.slug}</td></tr>
        <tr>
          <td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Status</td>
          <td class="py-1.5 pr-4 align-top"><Badge class={statusBadgeClass(statusLabel(decision.status))}>{statusLabel(decision.status)}</Badge></td>
        </tr>
        {#if decision.supersededBy}
          <tr><td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Superseded by</td><td class="py-1.5 pr-4 align-top">{decision.supersededBy}</td></tr>
        {/if}
        {#if decision.createdAt}
          <tr><td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Created</td><td class="py-1.5 pr-4 align-top">{new Date(Number(decision.createdAt.seconds) * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}</td></tr>
        {/if}
        {#if decision.updatedAt}
          <tr><td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Updated</td><td class="py-1.5 pr-4 align-top">{new Date(Number(decision.updatedAt.seconds) * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}</td></tr>
        {/if}
      </tbody>
    </table>

    {#if decision.decision}
      <section class="mt-5">
        <h2 class="mb-2 text-base font-semibold text-foreground">Decision</h2>
        <p class="whitespace-pre-wrap text-sm leading-relaxed text-foreground">{decision.decision}</p>
      </section>
    {/if}

    {#if decision.rationale}
      <section class="mt-5">
        <h2 class="mb-2 text-base font-semibold text-foreground">Rationale</h2>
        <p class="whitespace-pre-wrap text-sm leading-relaxed text-foreground">{decision.rationale}</p>
      </section>
    {/if}

    {#if d.linkedSpecs.length > 0}
      <section class="mt-5">
        <h2 class="mb-2 text-base font-semibold text-foreground">Referenced by</h2>
        <div class="flex flex-wrap gap-1.5">
          {#each d.linkedSpecs as specSlug}
            <a
              href="/spec/{specSlug}"
              class="rounded px-2 py-0.5 text-xs font-medium no-underline bg-blue-100 text-blue-800 hover:bg-blue-200 dark:bg-blue-950 dark:text-blue-300 dark:hover:bg-blue-900"
              >{specSlug}</a
            >
          {/each}
        </div>
      </section>
    {/if}
  {/if}
{/await}
