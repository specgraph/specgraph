<script lang="ts">
  import * as Card from '$lib/components/ui/card/index.js';

  interface Props {
    totalSpecs: number;
    readyCount: number;
    driftCount: number;
    decisionCount: number;
    amendedCount?: number;
    supersededCount?: number;
  }

  let { totalSpecs, readyCount, driftCount, decisionCount, amendedCount = 0, supersededCount = 0 }: Props = $props();

  const cards = $derived([
    { label: 'Specs', value: totalSpecs, href: '/graph' },
    { label: 'Ready', value: readyCount, href: '/graph' },
    { label: 'Drift', value: driftCount, href: '/graph' },
    { label: 'Decisions', value: decisionCount, href: '/graph' },
    { label: 'Amended', value: amendedCount, href: '/graph' },
    { label: 'Superseded', value: supersededCount, href: '/graph' },
  ]);
</script>

<div class="grid grid-cols-[repeat(auto-fit,minmax(120px,1fr))] gap-6">
  {#each cards as card (card.label)}
    <a href={card.href} class="rounded-xl focus-visible:outline-2 focus-visible:outline-ring focus-visible:outline-offset-2">
      <Card.Root class="gap-1 transition-shadow hover:shadow-md">
        <Card.Content class="flex flex-col gap-1">
          <span class="text-3xl font-semibold leading-none">{card.value}</span>
          <span class="text-sm font-medium text-muted-foreground">{card.label}</span>
        </Card.Content>
      </Card.Root>
    </a>
  {/each}
</div>
