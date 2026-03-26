<script lang="ts">
  interface Props {
    tabs: string[];
    active: string;
    onchange: (tab: string) => void;
  }
  let { tabs, active, onchange }: Props = $props();
</script>

<div class="tab-bar">
  {#each tabs as tab}
    <button
      class="tab"
      class:active={tab === active}
      onclick={() => onchange(tab)}
    >
      {tab}
    </button>
  {/each}
</div>

<style>
  .tab-bar {
    display: flex;
    gap: 0;
    border-bottom: 2px solid #e2e8f0;
    margin-bottom: 1rem;
  }

  .tab {
    padding: 0.5rem 1rem;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    margin-bottom: -2px;
    cursor: pointer;
    font-size: 0.85rem;
    font-weight: 500;
    color: #64748b;
    transition: color 0.15s, border-color 0.15s;
  }

  .tab:hover {
    color: #1a1a2e;
  }

  .tab.active {
    color: #2563eb;
    border-bottom-color: #2563eb;
    font-weight: 600;
  }
</style>
