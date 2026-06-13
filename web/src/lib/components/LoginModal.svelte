<script lang="ts">
  import { login } from '$lib/auth.svelte';
  import { fetchProviders, authErrorMessage, type OidcProvider } from '$lib/oidc.svelte';
  import { onMount } from 'svelte';

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

<div class="overlay">
  <form class="login-card" onsubmit={handleSubmit}>
    <h2>SpecGraph</h2>
    {#if providers.length > 0}
      <div class="providers">
        {#each providers as p}
          <a class="oidc-btn" href={`/api/auth/oidc/${p.id}/start`}>Sign in with {p.displayName}</a>
        {/each}
      </div>
      <div class="divider"><span>or</span></div>
    {/if}
    <p>Enter your API key to continue.</p>
    <input
      type="password"
      bind:value={key}
      placeholder="spgr_sk_..."
      autocomplete="off"
      disabled={loading}
    />
    {#if error}
      <p class="error">{error}</p>
    {/if}
    <button type="submit" disabled={!key || loading}>
      {loading ? 'Authenticating...' : 'Sign in'}
    </button>
  </form>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .login-card {
    background: white;
    border-radius: 8px;
    padding: 2rem;
    width: 360px;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.15);
  }
  h2 { margin: 0 0 0.5rem; color: #1a1a2e; }
  p { color: #64748b; font-size: 0.9rem; margin: 0 0 1rem; }
  input {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid #d1d5db;
    border-radius: 4px;
    font-size: 0.9rem;
    box-sizing: border-box;
    margin-bottom: 0.75rem;
  }
  input:focus { outline: 2px solid #3b82f6; border-color: transparent; }
  .error { color: #ef4444; font-size: 0.85rem; }
  button {
    width: 100%;
    padding: 0.5rem;
    background: #1a1a2e;
    color: white;
    border: none;
    border-radius: 4px;
    font-size: 0.9rem;
    cursor: pointer;
  }
  button:hover:not(:disabled) { background: #2d2d4e; }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
  .providers { display: flex; flex-direction: column; gap: 0.5rem; margin-bottom: 0.75rem; }
  .oidc-btn {
    display: block; text-align: center; text-decoration: none;
    padding: 0.5rem; border: 1px solid #1a1a2e; border-radius: 4px;
    color: #1a1a2e; font-size: 0.9rem;
  }
  .oidc-btn:hover { background: #f1f5f9; }
  .divider { display: flex; align-items: center; text-align: center; color: #94a3b8; font-size: 0.8rem; margin: 0.25rem 0 0.75rem; }
  .divider::before, .divider::after { content: ''; flex: 1; border-bottom: 1px solid #e2e8f0; }
  .divider span { padding: 0 0.5rem; }
</style>
