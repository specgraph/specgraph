<script lang="ts">
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { stageBadgeClass } from './badge-variants';

  interface Props {
    stageCounts: Record<string, number>;
  }

  let { stageCounts }: Props = $props();

  const stages = ['spark', 'shape', 'specify', 'decompose', 'approved', 'done'];

  // Funnel segments use the accent token at descending opacity (muted/accent
  // token rule, UI-SPEC FunnelBar). Stage identity/colour is carried by the
  // categorical Badge chips (D-10 palette), not the segment fill.
  const segmentFills = [
    'bg-primary',
    'bg-primary/75',
    'bg-primary/60',
    'bg-primary/45',
    'bg-primary/30',
    'bg-primary/20',
  ];

  let total = $derived(stages.reduce((sum, s) => sum + (stageCounts[s] ?? 0), 0));
</script>

<a href="/graph" class="block rounded-xl focus-visible:outline-2 focus-visible:outline-ring focus-visible:outline-offset-2">
  <div class="rounded-xl bg-card p-5 text-card-foreground ring-1 ring-foreground/10 transition-shadow hover:shadow-md">
    <h3 class="mb-3 text-sm font-semibold text-foreground">Authoring Funnel</h3>
    <div class="flex h-8 w-full overflow-hidden rounded-md border border-border">
      {#each stages as stage, i (stage)}
        {@const count = stageCounts[stage] ?? 0}
        {@const pct = total > 0 ? (count / total) * 100 : 0}
        {#if count > 0}
          <div
            class="flex min-w-0.5 items-center justify-center {segmentFills[i]}"
            style="width: {pct}%"
            title="{stage}: {count}"
          >
            {#if pct > 8}
              <span class="truncate px-1 text-xs font-semibold text-primary-foreground">{stage} ({count})</span>
            {/if}
          </div>
        {/if}
      {/each}
    </div>
    <div class="mt-3 flex flex-wrap gap-2">
      {#each stages as stage (stage)}
        <Badge class={stageBadgeClass(stage)}>{stage}: {stageCounts[stage] ?? 0}</Badge>
      {/each}
    </div>
  </div>
</a>
