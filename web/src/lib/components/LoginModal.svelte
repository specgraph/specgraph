<script lang="ts">
  import { login } from '$lib/auth.svelte';
  import { fetchProviders, authErrorMessage, type OidcProvider } from '$lib/oidc.svelte';
  import { onMount } from 'svelte';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Button } from '$lib/components/ui/button/index.js';

  let key = $state('');
  let error = $state('');
  let loading = $state(false);
  let providers = $state<OidcProvider[]>([]);

  let { onSuccess, authError = null }: { onSuccess: () => Promise<void>; authError?: string | null } = $props();

  onMount(async () => {
    providers = await fetchProviders();
    if (authError) error = authErrorMessage(authError);
  });

  async function handleSubmit(e: Event) {
    e.preventDefault();
    error = '';
    loading = true;
    try {
      const ok = await login(key);
      if (ok) {
        await onSuccess();
      } else {
        error = 'Invalid API key. Check your key and try again.';
        key = '';
      }
    } catch {
      error = 'Connection error. Please try again.';
    } finally {
      loading = false;
    }
  }
</script>

<!-- Auth gate: open is controlled (not bindable) so the login dialog cannot be
     dismissed into a blank page. bits-ui still provides focus-trap; the layout
     unmounts this component once authenticated. -->
<Dialog.Root open={true}>
  <Dialog.Content showCloseButton={false} class="sm:max-w-sm">
    <Dialog.Header>
      <Dialog.Title>SpecGraph</Dialog.Title>
      <Dialog.Description>Enter your API key to continue.</Dialog.Description>
    </Dialog.Header>

    {#if providers.length > 0}
      <div class="flex flex-col gap-2">
        {#each providers as p}
          <Button variant="outline" href={`/api/auth/oidc/${p.id}/start`}>
            Sign in with {p.displayName}
          </Button>
        {/each}
      </div>
      <div class="flex items-center gap-2 text-xs text-muted-foreground">
        <span class="h-px flex-1 bg-border"></span>
        <span>or</span>
        <span class="h-px flex-1 bg-border"></span>
      </div>
    {/if}

    <form class="flex flex-col gap-3" onsubmit={handleSubmit}>
      <Input
        type="password"
        bind:value={key}
        placeholder="spgr_sk_..."
        autocomplete="off"
        disabled={loading}
      />
      {#if error}
        <p class="text-sm text-destructive">{error}</p>
      {/if}
      <Button type="submit" disabled={!key || loading}>
        {loading ? 'Authenticating...' : 'Sign in'}
      </Button>
    </form>
  </Dialog.Content>
</Dialog.Root>
