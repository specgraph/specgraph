<script lang="ts">
  import { stageColors } from '$lib/colors';

  interface Props {
    stageCounts: Record<string, number>;
  }

  let { stageCounts }: Props = $props();

  const stages = ['spark', 'shape', 'specify', 'decompose', 'approved', 'done'];

  let total = $derived(stages.reduce((sum, s) => sum + (stageCounts[s] ?? 0), 0));
</script>

<a class="funnel-bar" href="/graph">
  <h3>Authoring Funnel</h3>
  <div class="bar-container">
    {#each stages as stage (stage)}
      {@const count = stageCounts[stage] ?? 0}
      {@const pct = total > 0 ? (count / total) * 100 : 0}
      {#if count > 0}
        <div
          class="bar-segment"
          style="width: {pct}%; background: {stageColors[stage] ?? '#6b7280'}"
          title="{stage}: {count}"
        >
          {#if pct > 8}
            <span class="bar-label">{stage} ({count})</span>
          {/if}
        </div>
      {/if}
    {/each}
  </div>
  <div class="legend">
    {#each stages as stage (stage)}
      <span class="legend-item">
        <span class="legend-dot" style="background: {stageColors[stage]}"></span>
        {stage}: {stageCounts[stage] ?? 0}
      </span>
    {/each}
  </div>
</a>

<style>
  .funnel-bar {
    display: block;
    background: white;
    border-radius: 8px;
    padding: 1.25rem;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
    text-decoration: none;
    color: inherit;
    transition: box-shadow 0.15s;
  }

  .funnel-bar:hover {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);
  }

  h3 {
    margin: 0 0 0.75rem;
    font-size: 0.95rem;
    color: #1a1a2e;
  }

  .bar-container {
    display: flex;
    height: 32px;
    border-radius: 6px;
    overflow: hidden;
  }

  .bar-segment {
    display: flex;
    align-items: center;
    justify-content: center;
    min-width: 2px;
    transition: width 0.3s ease;
  }

  .bar-label {
    font-size: 0.7rem;
    color: white;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    padding: 0 4px;
  }

  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem;
    margin-top: 0.75rem;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.8rem;
    color: #64748b;
  }

  .legend-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
</style>
