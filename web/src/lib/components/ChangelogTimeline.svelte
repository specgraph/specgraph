<!--
  SPDX-License-Identifier: MIT
  Copyright 2026 Sean Brandt
-->
<script lang="ts">
  import type { ChangeLogEntry } from '$lib/api/gen/specgraph/v1/spec_pb';
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

  function stageBadgeClass(stage: string): string {
    const map: Record<string, string> = {
      spark: 'badge-purple',
      shape: 'badge-blue',
      specify: 'badge-green',
      decompose: 'badge-yellow',
      approved: 'badge-teal',
      in_progress: 'badge-orange',
      review: 'badge-orange',
      done: 'badge-gray',
      amended: 'badge-amber',
      superseded: 'badge-gray-strike',
      abandoned: 'badge-red',
    };
    return map[stage] || 'badge-gray';
  }
</script>

{#if loading}
  <div class="loading">Loading changelog...</div>
{:else if entries.length === 0}
  <div class="empty">No changelog entries.</div>
{:else}
  <div class="timeline">
    {#each entries as entry}
      <div class="timeline-entry" class:checkpoint={entry.checkpoint}>
        <div class="timeline-marker" class:checkpoint-marker={entry.checkpoint}></div>
        <button class="timeline-card" onclick={() => toggleEntry(entry.version)}>
          <div class="card-header">
            <span class="version-badge">v{entry.version}</span>
            <span class="stage-badge {stageBadgeClass(entry.stage)}">{entry.stage}</span>
            {#if entry.checkpoint}
              <span class="checkpoint-badge">checkpoint</span>
            {/if}
            <span class="date">{formatDate(entry)}</span>
            <span class="expand-icon">{expandedVersions.has(entry.version) ? '▾' : '▸'}</span>
          </div>
          {#if entry.reason}
            <div class="card-reason">{entry.reason}</div>
          {/if}
          {#if entry.summary}
            <div class="card-summary">{entry.summary}</div>
          {/if}
        </button>

        {#if expandedVersions.has(entry.version) && entry.changes.length > 0}
          <div class="card-diffs">
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

<style>
  .timeline {
    position: relative;
    padding-left: 1.5rem;
  }
  .timeline::before {
    content: '';
    position: absolute;
    left: 0.5rem;
    top: 0;
    bottom: 0;
    width: 2px;
    background: var(--border-color, #e5e7eb);
  }
  .timeline-entry {
    position: relative;
    margin-bottom: 1rem;
  }
  .timeline-marker {
    position: absolute;
    left: -1.25rem;
    top: 0.75rem;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    border: 2px solid var(--border-color, #d1d5db);
    background: var(--bg-surface, #fff);
    z-index: 1;
  }
  .checkpoint-marker {
    background: var(--accent-color, #6366f1);
    border-color: var(--accent-color, #6366f1);
  }
  .timeline-card {
    display: block;
    width: 100%;
    text-align: left;
    background: var(--bg-surface, #fff);
    border: 1px solid var(--border-color, #e5e7eb);
    border-radius: 0.5rem;
    padding: 0.75rem;
    cursor: pointer;
    transition: border-color 0.15s;
    font: inherit;
    color: inherit;
  }
  .timeline-card:hover {
    border-color: var(--accent-color, #6366f1);
  }
  .card-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .version-badge {
    font-weight: 700;
    font-size: 0.85rem;
  }
  .stage-badge {
    font-size: 0.75rem;
    padding: 0.125rem 0.5rem;
    border-radius: 9999px;
    font-weight: 500;
  }
  .badge-purple { background: #ede9fe; color: #5b21b6; }
  .badge-blue { background: #dbeafe; color: #1e40af; }
  .badge-green { background: #dcfce7; color: #166534; }
  .badge-yellow { background: #fef9c3; color: #854d0e; }
  .badge-teal { background: #ccfbf1; color: #115e59; }
  .badge-orange { background: #ffedd5; color: #9a3412; }
  .badge-gray { background: #f3f4f6; color: #374151; }
  .badge-amber { background: #fef3c7; color: #92400e; }
  .badge-gray-strike { background: #f3f4f6; color: #6b7280; text-decoration: line-through; }
  .badge-red { background: #fee2e2; color: #991b1b; }
  .checkpoint-badge {
    font-size: 0.65rem;
    padding: 0.1rem 0.375rem;
    border-radius: 0.25rem;
    background: var(--accent-color, #6366f1);
    color: #fff;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .date {
    font-size: 0.8rem;
    color: var(--text-secondary, #6b7280);
    margin-left: auto;
  }
  .expand-icon {
    font-size: 0.75rem;
    color: var(--text-secondary, #9ca3af);
  }
  .card-reason {
    margin-top: 0.375rem;
    font-size: 0.8rem;
    color: var(--text-secondary, #6b7280);
    font-style: italic;
  }
  .card-summary {
    margin-top: 0.25rem;
    font-size: 0.85rem;
  }
  .card-diffs {
    margin-top: 0.5rem;
    padding: 0.75rem;
    background: var(--bg-subtle, #f9fafb);
    border: 1px solid var(--border-color, #e5e7eb);
    border-radius: 0 0 0.5rem 0.5rem;
    border-top: none;
  }
  .loading, .empty {
    padding: 1rem;
    text-align: center;
    color: var(--text-secondary, #6b7280);
    font-size: 0.85rem;
  }
</style>
