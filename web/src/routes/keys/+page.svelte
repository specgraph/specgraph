<script lang="ts">
  import { onMount } from 'svelte';
  import { keys, listKeys, createKey, rotateKey, revokeKey } from '$lib/keys.svelte';
  import type { APIKey } from '$lib/api/gen/specgraph/v1/identity_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import RevealKeyModal from '$lib/components/RevealKeyModal.svelte';

  let label = $state('');
  let roleDowngrade = $state('');
  let expiresAt = $state('');
  let submitting = $state(false);

  // The one-time plaintext lives here only while the reveal modal is open; it is
  // cleared on close and never re-fetched.
  let revealed = $state<string | null>(null);

  onMount(() => {
    listKeys();
  });

  function parseExpiry(): Date | undefined {
    if (!expiresAt) return undefined;
    const d = new Date(expiresAt);
    return Number.isNaN(d.getTime()) ? undefined : d;
  }

  async function handleCreate(e: Event) {
    e.preventDefault();
    if (!label.trim() || submitting) return;
    submitting = true;
    try {
      const pt = await createKey({
        label: label.trim(),
        roleDowngrade: roleDowngrade.trim() || undefined,
        expiresAt: parseExpiry(),
      });
      if (pt) {
        revealed = pt;
        label = '';
        roleDowngrade = '';
        expiresAt = '';
      }
    } finally {
      submitting = false;
    }
  }

  async function handleRotate(keyId: string) {
    const pt = await rotateKey(keyId);
    if (pt) revealed = pt;
  }

  async function handleRevoke(keyId: string) {
    await revokeKey(keyId);
  }

  function closeReveal() {
    revealed = null;
  }

  function fmt(ts: APIKey['expiresAt']): string {
    if (!ts) return '—';
    try {
      return timestampDate(ts).toLocaleDateString();
    } catch {
      return '—';
    }
  }

  function isRevoked(k: APIKey): boolean {
    return !!k.revokedAt;
  }
</script>

<nav class="breadcrumb">
  <a href="/">Dashboard</a> / <span>MCP Keys</span>
</nav>

<h1>MCP Keys</h1>

<p class="eligibility">
  Self-minting keys requires an interactive <strong>OIDC</strong> or workspace
  (<code>spgr_ws_</code>) session. If you signed in by pasting a raw API key, creating or rotating a
  key is intentionally blocked (anti key-chaining) — re-authenticate via your OIDC provider to
  manage keys.
</p>

{#if keys.error}
  <p class="status error">{keys.error}</p>
{/if}

<section class="create">
  <h2>Create a key</h2>
  <form onsubmit={handleCreate}>
    <label>
      Label
      <input type="text" bind:value={label} placeholder="ci-runner" disabled={submitting} />
    </label>
    <label>
      Role downgrade <span class="opt">(optional)</span>
      <input type="text" bind:value={roleDowngrade} placeholder="reader" disabled={submitting} />
    </label>
    <label>
      Expires <span class="opt">(optional)</span>
      <input type="date" bind:value={expiresAt} disabled={submitting} />
    </label>
    <button type="submit" disabled={!label.trim() || submitting}>
      {submitting ? 'Creating…' : 'Create key'}
    </button>
  </form>
</section>

<section class="list">
  <h2>Your keys</h2>
  {#if keys.loading}
    <p class="status">Loading…</p>
  {:else if keys.list.length === 0}
    <p class="status">You have no API keys yet.</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th>Prefix</th>
          <th>Label</th>
          <th>Role downgrade</th>
          <th>Expires</th>
          <th>Status</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each keys.list as k (k.id)}
          <tr class:revoked={isRevoked(k)}>
            <td><code>{k.prefix}</code></td>
            <td>{k.label || '—'}</td>
            <td>{k.roleDowngrade || '—'}</td>
            <td>{fmt(k.expiresAt)}</td>
            <td>{isRevoked(k) ? 'Revoked' : 'Active'}</td>
            <td class="actions">
              {#if !isRevoked(k)}
                <button type="button" class="rotate" onclick={() => handleRotate(k.id)}>Rotate</button>
                <button type="button" class="revoke" onclick={() => handleRevoke(k.id)}>Revoke</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

{#if revealed}
  <RevealKeyModal plaintext={revealed} onClose={closeReveal} />
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
    margin: 0 0 0.75rem;
    color: #1a1a2e;
  }
  h2 {
    font-size: 0.95rem;
    font-weight: 600;
    color: #374151;
    margin: 0 0 0.75rem;
  }
  .eligibility {
    background: #eff6ff;
    border-left: 3px solid #2563eb;
    border-radius: 4px;
    padding: 0.6rem 0.85rem;
    font-size: 0.85rem;
    color: #334155;
    line-height: 1.5;
    margin: 0 0 1.25rem;
  }
  .eligibility code {
    background: #dbeafe;
    padding: 0.05rem 0.3rem;
    border-radius: 3px;
    font-size: 0.8rem;
  }
  .status {
    color: #64748b;
    font-size: 0.9rem;
  }
  .status.error {
    color: #dc2626;
    background: #fef2f2;
    border-radius: 4px;
    padding: 0.5rem 0.75rem;
    margin-bottom: 1rem;
  }
  section {
    margin-bottom: 1.75rem;
  }
  form {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem;
    align-items: flex-end;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.8rem;
    color: #475569;
  }
  .opt {
    color: #94a3b8;
    font-weight: 400;
  }
  input {
    padding: 0.4rem 0.5rem;
    border: 1px solid #d1d5db;
    border-radius: 4px;
    font-size: 0.85rem;
  }
  input:focus {
    outline: 2px solid #3b82f6;
    border-color: transparent;
  }
  form button {
    padding: 0.45rem 1rem;
    background: #1a1a2e;
    color: white;
    border: none;
    border-radius: 4px;
    font-size: 0.85rem;
    cursor: pointer;
    height: fit-content;
  }
  form button:hover:not(:disabled) {
    background: #2d2d4e;
  }
  form button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }
  th {
    text-align: left;
    padding: 0.4rem 0.6rem;
    background: #f1f5f9;
    color: #475569;
    font-weight: 600;
  }
  td {
    padding: 0.4rem 0.6rem;
    border-bottom: 1px solid #f1f5f9;
  }
  tr.revoked {
    color: #94a3b8;
  }
  .actions {
    display: flex;
    gap: 0.4rem;
  }
  .actions button {
    padding: 0.25rem 0.6rem;
    border: 1px solid #d1d5db;
    border-radius: 4px;
    background: white;
    font-size: 0.8rem;
    cursor: pointer;
  }
  .actions .rotate:hover {
    border-color: #2563eb;
    color: #2563eb;
  }
  .actions .revoke:hover {
    border-color: #dc2626;
    color: #dc2626;
  }
</style>
