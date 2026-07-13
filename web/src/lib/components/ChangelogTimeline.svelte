<!--
  SPDX-License-Identifier: MIT
  Copyright 2026 Sean Brandt
-->
<script lang="ts">
  import type { ChangeLogEntry } from '$lib/api/gen/specgraph/v1/spec_pb';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { stageBadgeClass } from './badge-variants';
  import DiffView from './DiffView.svelte';

  interface Props {
    entries: ChangeLogEntry[];
    loading?: boolean;
  }

  let { entries, loading = false }: Props = $props();
  let expandedVersions: Set<number> = $state(new Set());

  function toggleEntry(version: number) {
    const next = new Set(expandedVersions);
    if (next.has(version)) {
      next.delete(version);
    } else {
      next.add(version);
    }
    expandedVersions = next;
  }

  function formatDate(entry: ChangeLogEntry): string {
    if (!entry.date) return '';
    const d = new Date(Number(entry.date.seconds) * 1000);
    return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
  }
</script>

{#if loading}
  <div class="p-4 text-center text-sm text-muted-foreground">Loading changelog...</div>
{:else if entries.length === 0}
  <div class="p-4 text-center text-sm text-muted-foreground">No changelog entries.</div>
{:else}
  <div class="relative pl-6">
    <Separator orientation="vertical" class="absolute left-1 top-0 h-full" />
    {#each entries as entry}
      <div class="relative mb-4" data-testid="timeline-entry">
        <span
          class="absolute -left-5 top-3 z-10 size-2.5 rounded-full border-2 {entry.checkpoint
            ? 'border-primary bg-primary'
            : 'border-border bg-background'}"
        ></span>
        <button
          data-testid="timeline-card"
          class="block w-full cursor-pointer rounded-lg border border-border bg-card p-3 text-left transition-colors hover:border-primary"
          onclick={() => toggleEntry(entry.version)}
        >
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-sm font-bold text-foreground">v{entry.version}</span>
            <Badge class={stageBadgeClass(entry.stage)}>{entry.stage}</Badge>
            {#if entry.checkpoint}
              <Badge class="text-[0.65rem] uppercase tracking-wide">checkpoint</Badge>
            {/if}
            <span class="ml-auto text-xs text-muted-foreground">{formatDate(entry)}</span>
            <span class="text-xs text-muted-foreground"
              >{expandedVersions.has(entry.version) ? '▾' : '▸'}</span
            >
          </div>
          {#if entry.reason}
            <div class="mt-1.5 text-xs italic text-muted-foreground">{entry.reason}</div>
          {/if}
          {#if entry.summary}
            <div class="mt-1 text-sm text-foreground">{entry.summary}</div>
          {/if}
        </button>

        {#if expandedVersions.has(entry.version) && entry.changes.length > 0}
          <div class="mt-0 rounded-b-lg border border-t-0 border-border bg-muted/50 p-3">
            {#each entry.changes as change}
              <DiffView
                field={change.field}
                oldValue={change.oldValue}
                newValue={change.newValue}
              />
            {/each}
          </div>
        {/if}
      </div>
    {/each}
  </div>
{/if}
