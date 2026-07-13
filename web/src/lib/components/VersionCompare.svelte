<!--
  SPDX-License-Identifier: MIT
  Copyright 2026 Sean Brandt
-->
<script lang="ts">
  import type { ChangeLogEntry, CompareVersionsResponse } from '$lib/api/gen/specgraph/v1/spec_pb';
  import { specClient } from '$lib/api/client';
  import DiffView from './DiffView.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import { Button } from '$lib/components/ui/button/index.js';

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

  // shadcn Select binds string values; bridge to the numeric version state.
  let fromValue = $derived(String(fromVersion));
  let toValue = $derived(String(toVersion));
  let fromLabel = $derived(fromVersion === 0 ? 'auto (previous)' : `v${fromVersion}`);
  let toLabel = $derived(toVersion === 0 ? 'latest' : `v${toVersion}`);

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

<div class="mb-4">
  <div class="mb-3 flex flex-wrap items-end gap-3">
    <div class="flex flex-col gap-1.5">
      <span class="text-xs font-semibold text-muted-foreground">From</span>
      <Select.Root
        type="single"
        value={fromValue}
        onValueChange={(v) => (fromVersion = Number(v))}
      >
        <Select.Trigger class="w-[160px]" aria-label="From version">
          {fromLabel}
        </Select.Trigger>
        <Select.Content>
          <Select.Item value="0" label="auto (previous)">auto (previous)</Select.Item>
          {#each versions as v (v)}
            <Select.Item value={String(v)} label={`v${v}`}>v{v}</Select.Item>
          {/each}
        </Select.Content>
      </Select.Root>
    </div>
    <div class="flex flex-col gap-1.5">
      <span class="text-xs font-semibold text-muted-foreground">To</span>
      <Select.Root
        type="single"
        value={toValue}
        onValueChange={(v) => (toVersion = Number(v))}
      >
        <Select.Trigger class="w-[160px]" aria-label="To version">
          {toLabel}
        </Select.Trigger>
        <Select.Content>
          <Select.Item value="0" label="latest">latest</Select.Item>
          {#each versions as v (v)}
            <Select.Item value={String(v)} label={`v${v}`}>v{v}</Select.Item>
          {/each}
        </Select.Content>
      </Select.Root>
    </div>
    <Button onclick={compare} disabled={comparing}>
      {comparing ? 'Comparing...' : 'Compare'}
    </Button>
  </div>

  {#if error}
    <div class="mb-2 text-sm text-destructive">{error}</div>
  {/if}

  {#if result}
    <Card.Root data-testid="compare-result">
      <Card.Header>
        <Card.Title class="text-sm font-bold">
          v{result.fromVersion} ({result.fromStage}) → v{result.toVersion} ({result.toStage})
        </Card.Title>
      </Card.Header>
      <Card.Content>
        {#if result.diffs.length === 0}
          <div class="py-4 text-center text-sm text-muted-foreground">
            No differences between these versions.
          </div>
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
      </Card.Content>
    </Card.Root>
  {/if}
</div>
