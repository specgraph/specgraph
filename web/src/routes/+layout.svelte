<script>
  import { page } from '$app/stores';

  let { children } = $props();
</script>

<nav>
  <a href="/" class:active={$page.url.pathname === '/'}>Dashboard</a>
  <a href="/graph" class:active={$page.url.pathname === '/graph'}>Graph</a>
  <span class="spacer"></span>
  <span class="brand">SpecGraph</span>
</nav>

<main>
  {@render children()}
</main>

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
</style>
