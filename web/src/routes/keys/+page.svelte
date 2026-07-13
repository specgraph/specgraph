<script lang="ts">
  import { onMount } from 'svelte';
  import { keys, listKeys, createKey, rotateKey, revokeKey } from '$lib/keys.svelte';
  import type { APIKey } from '$lib/api/gen/specgraph/v1/identity_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import RevealKeyModal from '$lib/components/RevealKeyModal.svelte';
  import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import * as Table from '$lib/components/ui/table/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';

  let label = $state('');
  let roleDowngrade = $state('');
  let expiresAt = $state('');
  let submitting = $state(false);

  // The one-time plaintext lives here only while the reveal modal is open; it is
  // cleared on close and never re-fetched.
  let revealed = $state<string | null>(null);

  // Destructive-revoke path is driven through the migrated RevealKeyModal
  // AlertDialog (05-07). We hold the pending key id and an open flag; the modal
  // binds `revokeOpen` and fires `onRevoke` on confirm.
  let revokeTarget = $state<string | null>(null);
  let revokeOpen = $state(false);

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

  function askRevoke(keyId: string) {
    revokeTarget = keyId;
    revokeOpen = true;
  }

  async function confirmRevoke() {
    if (revokeTarget) await revokeKey(revokeTarget);
    revokeTarget = null;
    revokeOpen = false;
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

<!-- Keys keeps its OWN user-scoped breadcrumb (D-09): no active-project indicator,
     and it is suppressed from the layout-owned {project} / {View} breadcrumb. -->
<Breadcrumb.Root class="mb-5">
  <Breadcrumb.List>
    <Breadcrumb.Item>
      <Breadcrumb.Link href="/">Dashboard</Breadcrumb.Link>
    </Breadcrumb.Item>
    <Breadcrumb.Separator />
    <Breadcrumb.Item>
      <Breadcrumb.Page class="text-muted-foreground">MCP Keys</Breadcrumb.Page>
    </Breadcrumb.Item>
  </Breadcrumb.List>
</Breadcrumb.Root>

<h1 class="mb-3 text-xl font-semibold text-foreground">MCP Keys</h1>

<p
  class="mb-5 rounded-lg border-l-2 border-primary bg-muted px-3.5 py-2.5 text-sm leading-relaxed text-muted-foreground"
>
  Self-minting keys requires an interactive <strong class="font-semibold text-foreground">OIDC</strong>
  or workspace (<code class="rounded bg-background px-1 py-0.5 font-mono text-xs text-foreground"
    >spgr_ws_</code
  >) session. If you signed in by pasting a raw API key, creating or rotating a key is intentionally
  blocked (anti key-chaining) — re-authenticate via your OIDC provider to manage keys.
</p>

{#if keys.error}
  <p
    class="mb-4 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
    role="alert"
  >
    {keys.error}
  </p>
{/if}

<Card.Root class="mb-7">
  <Card.Header>
    <Card.Title>Create a key</Card.Title>
  </Card.Header>
  <Card.Content>
    <form onsubmit={handleCreate} class="flex flex-wrap items-end gap-3">
      <label class="flex flex-col gap-1 text-xs font-medium text-muted-foreground">
        Label
        <Input
          type="text"
          bind:value={label}
          placeholder="ci-runner"
          disabled={submitting}
          class="w-44"
        />
      </label>
      <label class="flex flex-col gap-1 text-xs font-medium text-muted-foreground">
        Role downgrade <span class="font-normal text-muted-foreground/70">(optional)</span>
        <Input
          type="text"
          bind:value={roleDowngrade}
          placeholder="reader"
          disabled={submitting}
          class="w-44"
        />
      </label>
      <label class="flex flex-col gap-1 text-xs font-medium text-muted-foreground">
        Expires <span class="font-normal text-muted-foreground/70">(optional)</span>
        <Input type="date" bind:value={expiresAt} disabled={submitting} class="w-44" />
      </label>
      <Button type="submit" disabled={!label.trim() || submitting}>
        {submitting ? 'Creating…' : 'Create key'}
      </Button>
    </form>
  </Card.Content>
</Card.Root>

<Card.Root>
  <Card.Header>
    <Card.Title>Your keys</Card.Title>
  </Card.Header>
  <Card.Content>
    {#if keys.loading}
      <p class="text-sm text-muted-foreground">Loading…</p>
    {:else if keys.list.length === 0}
      <p class="text-sm text-muted-foreground">You have no API keys yet.</p>
    {:else}
      <Table.Root class="text-sm">
        <Table.Header>
          <Table.Row class="bg-muted">
            <Table.Head>Prefix</Table.Head>
            <Table.Head>Label</Table.Head>
            <Table.Head>Role downgrade</Table.Head>
            <Table.Head>Expires</Table.Head>
            <Table.Head>Status</Table.Head>
            <Table.Head></Table.Head>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {#each keys.list as k (k.id)}
            <Table.Row class={isRevoked(k) ? 'text-muted-foreground' : undefined}>
              <Table.Cell>
                <code class="font-mono text-xs text-foreground">{k.prefix}</code>
              </Table.Cell>
              <Table.Cell>{k.label || '—'}</Table.Cell>
              <Table.Cell>{k.roleDowngrade || '—'}</Table.Cell>
              <Table.Cell class="whitespace-nowrap">{fmt(k.expiresAt)}</Table.Cell>
              <Table.Cell>{isRevoked(k) ? 'Revoked' : 'Active'}</Table.Cell>
              <Table.Cell>
                {#if !isRevoked(k)}
                  <div class="flex gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onclick={() => handleRotate(k.id)}>Rotate</Button
                    >
                    <Button
                      type="button"
                      variant="destructive"
                      size="sm"
                      onclick={() => askRevoke(k.id)}>Revoke</Button
                    >
                  </div>
                {/if}
              </Table.Cell>
            </Table.Row>
          {/each}
        </Table.Body>
      </Table.Root>
    {/if}
  </Card.Content>
</Card.Root>

<RevealKeyModal
  plaintext={revealed}
  onClose={closeReveal}
  bind:revokeOpen
  onRevoke={confirmRevoke}
/>
