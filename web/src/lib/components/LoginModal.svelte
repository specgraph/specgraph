<script lang="ts">
  import { login } from '$lib/auth.svelte';

  let key = $state('');
  let error = $state('');
  let loading = $state(false);

  let { onSuccess } = $props();

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
</style>
