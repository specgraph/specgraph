<!--
  SPDX-License-Identifier: MIT
  Copyright 2026 Sean Brandt
-->
<script lang="ts">
  import type { ChangeLogEntry, CompareVersionsResponse } from '$lib/api/gen/specgraph/v1/spec_pb';
  import { specClient } from '$lib/api/client';
  import DiffView from './DiffView.svelte';

  interface Props {
    slug: string;
    entries: ChangeLogEntry[];
  }

  let { slug, entries }: Props = $props();
  let fromVersion: number = $state(0);
  let toVersion: number = $state(0);
  let result: CompareVersionsResponse | null = $state(null);
  let comparing: boolean = $state(false);
  let error: string = $state('');

  let versions = $derived(entries.map((e) => e.version).sort((a, b) => b - a));

  async function compare() {
    if (fromVersion === 0 && toVersion === 0) return;
    comparing = true;
    error = '';
    result = null;
    try {
      const resp = await specClient.compareVersions({
        slug,
        fromVersion: fromVersion,
        toVersion: toVersion,
      });
      result = resp;
    } catch (e: unknown) {
      error = e instanceof Error ? e.message : 'Comparison failed';
    } finally {
      comparing = false;
    }
  }
</script>

<div class="version-compare">
  <div class="compare-controls">
    <label class="compare-label">
      From:
      <select bind:value={fromVersion}>
        <option value={0}>auto (previous)</option>
        {#each versions as v}
          <option value={v}>v{v}</option>
        {/each}
      </select>
    </label>
    <label class="compare-label">
      To:
      <select bind:value={toVersion}>
        <option value={0}>latest</option>
        {#each versions as v}
          <option value={v}>v{v}</option>
        {/each}
      </select>
    </label>
    <button class="compare-btn" onclick={compare} disabled={comparing}>
      {comparing ? 'Comparing...' : 'Compare'}
    </button>
  </div>

  {#if error}
    <div class="compare-error">{error}</div>
  {/if}

  {#if result}
    <div class="compare-result">
      <div class="compare-header">
        v{result.fromVersion} ({result.fromStage}) → v{result.toVersion} ({result.toStage})
      </div>
      {#if result.diffs.length === 0}
        <div class="compare-empty">No differences between these versions.</div>
      {:else}
        {#each result.diffs as d}
          <DiffView
            field={d.field}
            oldValue={d.oldValue}
            newValue={d.newValue}
            hunks={d.hunks}
          />
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .version-compare {
    margin-bottom: 1rem;
  }
  .compare-controls {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
    margin-bottom: 0.75rem;
  }
  .compare-label {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--text-secondary, #6b7280);
    display: flex;
    align-items: center;
    gap: 0.375rem;
  }
  .compare-label select {
    padding: 0.25rem 0.5rem;
    border: 1px solid var(--border-color, #d1d5db);
    border-radius: 0.375rem;
    font-size: 0.8rem;
    background: var(--bg-surface, #fff);
  }
  .compare-btn {
    padding: 0.375rem 0.75rem;
    font-size: 0.8rem;
    font-weight: 600;
    background: var(--accent-color, #6366f1);
    color: #fff;
    border: none;
    border-radius: 0.375rem;
    cursor: pointer;
  }
  .compare-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .compare-header {
    font-weight: 700;
    font-size: 0.9rem;
    margin-bottom: 0.75rem;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid var(--border-color, #e5e7eb);
  }
  .compare-error {
    color: #dc2626;
    font-size: 0.85rem;
    margin-bottom: 0.5rem;
  }
  .compare-empty {
    color: var(--text-secondary, #6b7280);
    font-size: 0.85rem;
    text-align: center;
    padding: 1rem;
  }
</style>
