<script lang="ts">
  interface Props {
    totalSpecs: number;
    readyCount: number;
    driftCount: number;
    decisionCount: number;
  }

  let { totalSpecs, readyCount, driftCount, decisionCount }: Props = $props();

  const cards = $derived([
    { label: 'Specs', value: totalSpecs, color: '#2563eb', href: '/graph' },
    { label: 'Ready', value: readyCount, color: '#16a34a', href: '/graph' },
    { label: 'Drift', value: driftCount, color: '#dc2626', href: '/graph' },
    { label: 'Decisions', value: decisionCount, color: '#7c3aed', href: '/graph' },
  ]);
</script>

<div class="stats-bar">
  {#each cards as card (card.label)}
    <a class="stat-card" style="border-top-color: {card.color}" href={card.href}>
      <span class="stat-value" style="color: {card.color}">{card.value}</span>
      <span class="stat-label">{card.label}</span>
    </a>
  {/each}
</div>

<style>
  .stats-bar {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1rem;
  }

  .stat-card {
    background: white;
    border-radius: 8px;
    padding: 1.25rem;
    border-top: 3px solid;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    text-decoration: none;
    color: inherit;
    transition: box-shadow 0.15s;
  }

  .stat-card:hover {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);
  }

  .stat-value {
    font-size: 2rem;
    font-weight: 700;
    line-height: 1;
  }

  .stat-label {
    font-size: 0.85rem;
    color: #64748b;
    font-weight: 500;
  }
</style>
