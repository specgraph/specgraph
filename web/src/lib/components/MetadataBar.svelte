<script lang="ts">
  import { SpecProvenance } from '$lib/api/gen/specgraph/v1/spec_pb';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';

  interface Props {
    createdAt?: { seconds: bigint };
    updatedAt?: { seconds: bigint };
    provenanceType?: SpecProvenance;
    contentHash?: string;
  }
  let { createdAt, updatedAt, provenanceType, contentHash }: Props = $props();

  function provenanceLabel(p: SpecProvenance | undefined): string | undefined {
    if (p === undefined || p === SpecProvenance.UNSPECIFIED) return undefined;
    const labels: Record<number, string> = {
      [SpecProvenance.AUTHORED]: 'AUTHORED',
      [SpecProvenance.RETROACTIVE_FROM_PR]: 'RETROACTIVE_FROM_PR',
      [SpecProvenance.DECLARED]: 'DECLARED',
    };
    return labels[p];
  }

  let displayProvenance = $derived(provenanceLabel(provenanceType));

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

<Card.Root class="mb-3">
  <Card.Content class="p-3">
    <dl class="flex flex-wrap items-center gap-x-6 gap-y-2 text-xs">
      <div class="flex items-center gap-1.5">
        <dt class="text-muted-foreground">Created</dt>
        <dd class="font-medium text-foreground">{formatDate(createdAt)}</dd>
      </div>
      <div class="flex items-center gap-1.5">
        <dt class="text-muted-foreground">Updated</dt>
        <dd class="font-medium text-foreground">{relativeTime(updatedAt)}</dd>
      </div>
      {#if displayProvenance}
        <div class="flex items-center gap-1.5">
          <dt class="text-muted-foreground">Provenance</dt>
          <dd><Badge variant="secondary">{displayProvenance}</Badge></dd>
        </div>
      {/if}
      {#if contentHash}
        <div class="flex items-center gap-1.5">
          <dt class="text-muted-foreground">Hash</dt>
          <dd class="font-mono text-muted-foreground" title={contentHash}>{contentHash.slice(0, 8)}…</dd>
        </div>
      {/if}
    </dl>
  </Card.Content>
</Card.Root>
