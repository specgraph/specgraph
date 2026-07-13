<!--
  SPDX-License-Identifier: MIT
  Copyright 2026 Sean Brandt
-->
<script lang="ts">
  import type { InlineDiff } from '$lib/api/gen/specgraph/v1/spec_pb';
  import { InlineDiff_Op } from '$lib/api/gen/specgraph/v1/spec_pb';
  import * as Card from '$lib/components/ui/card/index.js';

  interface Props {
    field: string;
    oldValue: string;
    newValue: string;
    hunks?: InlineDiff[];
  }

  let { field, oldValue, newValue, hunks = [] }: Props = $props();
</script>

<div class="mb-3" data-testid="diff-view">
  <div
    class="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground"
  >
    {field}
  </div>
  <div class="grid grid-cols-2 gap-2">
    <Card.Root class="overflow-hidden py-0">
      <div
        class="border-b border-border bg-red-50 px-2 py-1 text-[0.7rem] font-semibold uppercase text-red-800 dark:bg-red-950/40 dark:text-red-300"
      >
        Old
      </div>
      <div
        class="max-h-[300px] overflow-y-auto whitespace-pre-wrap break-words p-2 font-mono text-sm leading-relaxed"
      >
        {#if hunks.length > 0}
          {#each hunks as hunk}
            {#if hunk.op === InlineDiff_Op.EQUAL}
              <span>{hunk.text}</span>
            {:else if hunk.op === InlineDiff_Op.DELETE}
              <span
                class="rounded-sm bg-red-200 px-0.5 text-red-800 line-through dark:bg-red-900/60 dark:text-red-200"
                >{hunk.text}</span
              >
            {/if}
          {/each}
        {:else}
          <span>{oldValue}</span>
        {/if}
      </div>
    </Card.Root>
    <Card.Root class="overflow-hidden py-0">
      <div
        class="border-b border-border bg-green-50 px-2 py-1 text-[0.7rem] font-semibold uppercase text-green-800 dark:bg-green-950/40 dark:text-green-300"
      >
        New
      </div>
      <div
        class="max-h-[300px] overflow-y-auto whitespace-pre-wrap break-words p-2 font-mono text-sm leading-relaxed"
      >
        {#if hunks.length > 0}
          {#each hunks as hunk}
            {#if hunk.op === InlineDiff_Op.EQUAL}
              <span>{hunk.text}</span>
            {:else if hunk.op === InlineDiff_Op.INSERT}
              <span
                class="rounded-sm bg-green-200 px-0.5 text-green-800 dark:bg-green-900/60 dark:text-green-200"
                >{hunk.text}</span
              >
            {/if}
          {/each}
        {:else}
          <span>{newValue}</span>
        {/if}
      </div>
    </Card.Root>
  </div>
</div>
