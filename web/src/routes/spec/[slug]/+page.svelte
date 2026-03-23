<script lang="ts">
  import { page } from '$app/stores';
  import { specClient } from '$lib/api/client';
  import type { Spec } from '$lib/api/gen/specgraph/v1/spec_pb';

  let spec = $state<Spec | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);

  let slug = $derived($page.params.slug);

  async function loadSpec(s: string) {
    try {
      const resp = await specClient.getSpec({ slug: s });
      spec = resp.spec ?? null;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load spec';
    } finally {
      loading = false;
    }
  }

  $effect(() => { loadSpec(slug); });
</script>

<nav class="breadcrumb">
  <a href="/">Dashboard</a> / <a href="/graph">Graph</a> / <span>{slug}</span>
</nav>

{#if loading}
  <p class="status">Loading...</p>
{:else if error}
  <p class="status error">{error}</p>
{:else if spec}
  <h1>{spec.slug}</h1>

  <table class="meta">
    <tbody>
      <tr><td class="label">Intent</td><td>{spec.intent}</td></tr>
      <tr><td class="label">Stage</td><td><span class="badge stage-{spec.stage}">{spec.stage}</span></td></tr>
      <tr><td class="label">Priority</td><td>{spec.priority || '—'}</td></tr>
      <tr><td class="label">Complexity</td><td>{spec.complexity || '—'}</td></tr>
      <tr><td class="label">Version</td><td>{spec.version}</td></tr>
    </tbody>
  </table>

  {#if spec.notes}
    <section class="section">
      <h2>Notes</h2>
      <p class="notes">{spec.notes}</p>
    </section>
  {/if}
{/if}

<style>
  .breadcrumb {
    font-size: 0.85rem;
    color: #64748b;
    margin-bottom: 1.25rem;
  }

  .breadcrumb a {
    color: #2563eb;
    text-decoration: none;
  }

  .breadcrumb a:hover {
    text-decoration: underline;
  }

  .breadcrumb span {
    color: #1a1a2e;
    font-weight: 500;
  }

  h1 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0 0 1rem;
    color: #1a1a2e;
  }

  .status {
    color: #64748b;
    font-size: 0.95rem;
  }

  .status.error {
    color: #dc2626;
  }

  .meta {
    border-collapse: collapse;
    font-size: 0.9rem;
    margin-bottom: 1.25rem;
  }

  .meta td {
    padding: 0.4rem 1rem 0.4rem 0;
    vertical-align: top;
  }

  .meta .label {
    color: #64748b;
    font-weight: 500;
    white-space: nowrap;
    min-width: 8rem;
  }

  .badge {
    display: inline-block;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.8rem;
    font-weight: 600;
    background: #f1f5f9;
    color: #475569;
  }

  .badge.stage-spark { background: #ede9fe; color: #7c3aed; }
  .badge.stage-shape { background: #dbeafe; color: #2563eb; }
  .badge.stage-specify { background: #dcfce7; color: #16a34a; }
  .badge.stage-decompose { background: #fef9c3; color: #ca8a04; }
  .badge.stage-approved { background: #ccfbf1; color: #0d9488; }
  .badge.stage-in_progress { background: #ffedd5; color: #ea580c; }
  .badge.stage-done { background: #f1f5f9; color: #6b7280; }

  .section {
    margin-top: 1rem;
  }

  h2 {
    font-size: 1rem;
    font-weight: 600;
    margin: 0 0 0.5rem;
    color: #1a1a2e;
  }

  .notes {
    color: #374151;
    font-size: 0.9rem;
    line-height: 1.6;
    white-space: pre-wrap;
  }
</style>
