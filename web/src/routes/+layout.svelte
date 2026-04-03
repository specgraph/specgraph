<script>
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { auth, checkAuth } from '$lib/auth.svelte';
  import { project, loadProjects } from '$lib/project.svelte';
  import LoginModal from '$lib/components/LoginModal.svelte';

  let { children } = $props();
  let ready = $state(false);

  onMount(async () => {
    await checkAuth();
    if (auth.authenticated) {
      await loadProjects();
    }
    ready = true;
  });

  async function handleLoginSuccess() {
    await loadProjects();
  }
</script>

{#if !ready}
  <main><p class="loading">Connecting...</p></main>
{:else if !auth.authenticated}
  <LoginModal onSuccess={handleLoginSuccess} />
{:else}
  <nav>
    <a href="/" class:active={$page.url.pathname === '/'}>Dashboard</a>
    <a href="/graph" class:active={$page.url.pathname === '/graph'}>Graph</a>
    <a href="/constitution" class:active={$page.url.pathname === '/constitution'}>Constitution</a>
    <span class="spacer"></span>
    {#if project.available.length > 1}
      <select bind:value={project.current} class="project-picker">
        {#each project.available as slug}
          <option value={slug}>{slug}</option>
        {/each}
      </select>
    {:else if project.current}
      <span class="project-name">{project.current}</span>
    {/if}
    <span class="brand">SpecGraph</span>
  </nav>

  <main>
    {#if project.loaded}
      {@render children()}
    {:else}
      <p class="loading">Loading projects...</p>
    {/if}
  </main>
{/if}

<style>
  :global(body) {
    margin: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    color: #1a1a2e;
    background: #f8f9fa;
  }

  nav {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.75rem 1.5rem;
    background: #1a1a2e;
    color: white;
  }

  nav a {
    color: rgba(255, 255, 255, 0.7);
    text-decoration: none;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.9rem;
  }

  nav a:hover, nav a.active {
    color: white;
    background: rgba(255, 255, 255, 0.1);
  }

  .spacer { flex: 1; }

  .project-picker {
    background: rgba(255, 255, 255, 0.1);
    color: white;
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 4px;
    padding: 0.25rem 0.5rem;
    font-size: 0.85rem;
  }

  .project-name {
    color: rgba(255, 255, 255, 0.6);
    font-size: 0.85rem;
  }

  .brand {
    font-weight: 600;
    font-size: 0.9rem;
    opacity: 0.8;
  }

  main {
    padding: 1.5rem;
    max-width: 1400px;
    margin: 0 auto;
  }

  .loading {
    color: #64748b;
  }
</style>
