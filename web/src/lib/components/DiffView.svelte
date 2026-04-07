<!--
  SPDX-License-Identifier: MIT
  Copyright 2026 Sean Brandt
-->
<script lang="ts">
  import type { InlineDiff } from '$lib/api/gen/specgraph/v1/spec_pb';
  import { InlineDiff_Op } from '$lib/api/gen/specgraph/v1/spec_pb';

  interface Props {
    field: string;
    oldValue: string;
    newValue: string;
    hunks?: InlineDiff[];
  }

  let { field, oldValue, newValue, hunks = [] }: Props = $props();
</script>

<div class="diff-view">
  <div class="diff-field-name">{field}</div>
  <div class="diff-panels">
    <div class="diff-panel diff-old">
      <div class="diff-panel-header">Old</div>
      <div class="diff-panel-content">
        {#if hunks.length > 0}
          {#each hunks as hunk}
            {#if hunk.op === InlineDiff_Op.EQUAL}
              <span>{hunk.text}</span>
            {:else if hunk.op === InlineDiff_Op.DELETE}
              <span class="diff-delete">{hunk.text}</span>
            {/if}
          {/each}
        {:else}
          <span>{oldValue}</span>
        {/if}
      </div>
    </div>
    <div class="diff-panel diff-new">
      <div class="diff-panel-header">New</div>
      <div class="diff-panel-content">
        {#if hunks.length > 0}
          {#each hunks as hunk}
            {#if hunk.op === InlineDiff_Op.EQUAL}
              <span>{hunk.text}</span>
            {:else if hunk.op === InlineDiff_Op.INSERT}
              <span class="diff-insert">{hunk.text}</span>
            {/if}
          {/each}
        {:else}
          <span>{newValue}</span>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .diff-view {
    margin-bottom: 0.75rem;
  }
  .diff-field-name {
    font-weight: 600;
    font-size: 0.8rem;
    color: var(--text-secondary, #6b7280);
    margin-bottom: 0.25rem;
    text-transform: uppercase;
    letter-spacing: 0.025em;
  }
  .diff-panels {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem;
  }
  .diff-panel {
    border: 1px solid var(--border-color, #e5e7eb);
    border-radius: 0.375rem;
    overflow: hidden;
  }
  .diff-panel-header {
    background: var(--bg-subtle, #f9fafb);
    padding: 0.25rem 0.5rem;
    font-size: 0.7rem;
    font-weight: 600;
    color: var(--text-secondary, #6b7280);
    text-transform: uppercase;
    border-bottom: 1px solid var(--border-color, #e5e7eb);
  }
  .diff-panel-content {
    padding: 0.5rem;
    font-size: 0.85rem;
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 300px;
    overflow-y: auto;
  }
  .diff-old .diff-panel-header {
    background: #fef2f2;
    color: #991b1b;
  }
  .diff-new .diff-panel-header {
    background: #f0fdf4;
    color: #166534;
  }
  .diff-delete {
    background: #fecaca;
    text-decoration: line-through;
    color: #991b1b;
    border-radius: 2px;
    padding: 0 2px;
  }
  .diff-insert {
    background: #bbf7d0;
    color: #166534;
    border-radius: 2px;
    padding: 0 2px;
  }
</style>
