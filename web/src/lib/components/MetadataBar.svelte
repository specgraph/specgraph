<script lang="ts">
  import { SpecLifecycle } from '$lib/api/gen/specgraph/v1/spec_pb';

  interface Props {
    createdAt?: { seconds: bigint };
    updatedAt?: { seconds: bigint };
    lifecycle?: SpecLifecycle;
    contentHash?: string;
  }
  let { createdAt, updatedAt, lifecycle, contentHash }: Props = $props();

  function lifecycleLabel(lc: SpecLifecycle | undefined): string | undefined {
    if (lc === undefined || lc === SpecLifecycle.UNSPECIFIED) return undefined;
    const labels: Record<number, string> = {
      [SpecLifecycle.TASK]: 'task',
      [SpecLifecycle.LIVING]: 'living',
    };
    return labels[lc];
  }

  let displayLifecycle = $derived(lifecycleLabel(lifecycle));

  function formatDate(ts: { seconds: bigint } | undefined): string {
    if (!ts) return '—';
    return new Date(Number(ts.seconds) * 1000).toLocaleDateString('en-US', {
      month: 'short', day: 'numeric', year: 'numeric',
    });
  }

  function relativeTime(ts: { seconds: bigint } | undefined): string {
    if (!ts) return '—';
    const now = Date.now();
    const then = Number(ts.seconds) * 1000;
    const diff = now - then;
    if (diff < 60_000) return 'just now';
    if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
    if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
    return `${Math.floor(diff / 86_400_000)}d ago`;
  }
</script>

<div class="metadata-bar">
  <span>Created: <strong>{formatDate(createdAt)}</strong></span>
  <span class="sep">·</span>
  <span>Updated: <strong>{relativeTime(updatedAt)}</strong></span>
  {#if displayLifecycle}
    <span class="sep">·</span>
    <span>Lifecycle: <span class="lifecycle-badge">{displayLifecycle}</span></span>
  {/if}
  {#if contentHash}
    <span class="sep">·</span>
    <span class="hash" title={contentHash}>hash: {contentHash.slice(0, 8)}…</span>
  {/if}
</div>

<style>
  .metadata-bar {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    align-items: center;
    font-size: 0.8rem;
    color: #64748b;
    margin-bottom: 0.75rem;
  }

  .sep {
    color: #cbd5e1;
  }

  strong {
    color: #475569;
    font-weight: 500;
  }

  .lifecycle-badge {
    background: #dbeafe;
    color: #2563eb;
    padding: 0.05rem 0.3rem;
    border-radius: 3px;
    font-size: 0.7rem;
    font-weight: 600;
  }

  .hash {
    font-family: ui-monospace, monospace;
    font-size: 0.7rem;
    color: #94a3b8;
  }
</style>
