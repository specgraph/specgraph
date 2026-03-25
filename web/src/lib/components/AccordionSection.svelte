<script lang="ts">
  interface Props {
    title: string;
    expanded?: boolean;
    badge?: string;
    children: import('svelte').Snippet;
  }
  let { title, expanded = false, badge = '', children }: Props = $props();
  let toggled = $state<boolean | null>(null);
  let open = $derived(toggled !== null ? toggled : expanded);
</script>

<div class="accordion">
  <button class="accordion-header" onclick={() => toggled = !open}>
    <span class="chevron" class:open>&#x25B6;</span>
    <span class="accordion-title">{title}</span>
    {#if badge}<span class="accordion-badge">{badge}</span>{/if}
  </button>
  {#if open}
    <div class="accordion-body">
      {@render children()}
    </div>
  {/if}
</div>

<style>
  .accordion {
    border-bottom: 1px solid #e2e8f0;
    margin-bottom: 0.25rem;
  }

  .accordion-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.6rem 0;
    background: none;
    border: none;
    cursor: pointer;
    font-size: 0.95rem;
    font-weight: 600;
    color: #1a1a2e;
    text-align: left;
  }

  .accordion-header:hover {
    color: #2563eb;
  }

  .chevron {
    font-size: 0.7rem;
    transition: transform 0.15s ease;
    color: #94a3b8;
  }

  .chevron.open {
    transform: rotate(90deg);
  }

  .accordion-badge {
    font-size: 0.75rem;
    font-weight: 500;
    color: #64748b;
    background: #f1f5f9;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
  }

  .accordion-body {
    padding: 0 0 0.75rem 1.25rem;
    font-size: 0.9rem;
    color: #374151;
    line-height: 1.6;
  }
</style>
