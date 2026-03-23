<script lang="ts">
  interface Props {
    totalSpecs: number;
    readyCount: number;
    driftCount: number;
    decisionCount: number;
  }

  let { totalSpecs, readyCount, driftCount, decisionCount }: Props = $props();

  const cards = $derived([
    { label: 'Specs', value: totalSpecs, color: '#2563eb' },
    { label: 'Ready', value: readyCount, color: '#16a34a' },
    { label: 'Drift', value: driftCount, color: '#dc2626' },
    { label: 'Decisions', value: decisionCount, color: '#7c3aed' },
  ]);
</script>

<div class="stats-bar">
  {#each cards as card (card.label)}
    <div class="stat-card" style="border-top-color: {card.color}">
      <span class="stat-value" style="color: {card.color}">{card.value}</span>
      <span class="stat-label">{card.label}</span>
    </div>
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
