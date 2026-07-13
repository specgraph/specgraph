<script lang="ts">
  import '../app.css';
  import { page } from '$app/stores';
  import { invalidateAll } from '$app/navigation';
  import { onMount } from 'svelte';
  import { ModeWatcher } from 'mode-watcher';
  import { project } from '$lib/project.svelte';
  import { cn } from '$lib/utils.js';
  import LoginModal from '$lib/components/LoginModal.svelte';
  import ModeToggle from '$lib/components/ModeToggle.svelte';
  import * as Select from '$lib/components/ui/select/index.js';
  import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
  import type { LayoutData } from './$types';

  let { children, data }: { children: import('svelte').Snippet; data: LayoutData } = $props();
  let authError = $state<string | null>(null);

  // Auth + project bootstrap now runs in +layout.ts load() (D-02). Only the
  // window-dependent auth_error query cleanup stays here (needs history/window).
  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const ae = params.get('auth_error');
    if (ae) {
      authError = ae;
      params.delete('auth_error');
      const qs = params.toString();
      history.replaceState({}, '', window.location.pathname + (qs ? '?' + qs : ''));
    }
  });

  // On login the session cookie is set; re-run load() so data.authenticated and
  // the resolved project default refresh reactively (D-02 seam).
  async function handleLoginSuccess() {
    await invalidateAll();
  }

  // D-03/D-01 switch seam: persist selection (setter writes localStorage) then
  // invalidateAll() re-runs +layout.ts and every +page.ts load() with the new
  // X-Specgraph-Project header. Wired via explicit onValueChange (not bind:value
  // alone) so every user change guarantees the invalidation.
  async function switchProject(slug: string | undefined) {
    if (!slug) return;
    project.current = slug;
    await invalidateAll();
  }

  // D-11 active-project indicator: {View} is a static label derived from the
  // pathname. Detail-route slugs stay in each page's <h1>, not the breadcrumb.
  function viewLabel(pathname: string): string {
    if (pathname === '/') return 'Dashboard';
    if (pathname === '/graph') return 'Graph';
    if (pathname === '/constitution') return 'Constitution';
    if (pathname.startsWith('/spec')) return 'Spec';
    if (pathname.startsWith('/decision')) return 'Decision';
    return '';
  }

  const pathname = $derived($page.url.pathname);
  const view = $derived(viewLabel(pathname));
  // Suppress the project breadcrumb on the user-scoped /keys route (D-09) and
  // when no project is resolved (zero-projects empty state owns the main area).
  const showBreadcrumb = $derived(Boolean(project.current) && pathname !== '/keys');
</script>

{#snippet navLink(href: string, label: string)}
  <a
    {href}
    class={cn(
      'rounded-md px-2 py-1 text-sm hover:text-foreground',
      pathname === href ? 'text-primary font-medium' : 'text-muted-foreground'
    )}
  >{label}</a>
{/snippet}

<ModeWatcher />

{#if !data.authenticated}
  <LoginModal onSuccess={handleLoginSuccess} {authError} />
{:else}
  <nav data-testid="primary-nav" class="flex items-center gap-4 border-b border-border px-6 py-3">
    {@render navLink('/', 'Dashboard')}
    {@render navLink('/graph', 'Graph')}
    {@render navLink('/constitution', 'Constitution')}
    {@render navLink('/keys', 'Keys')}
    <span class="flex-1"></span>
    {#if project.available.length > 1}
      <Select.Root type="single" value={project.current} onValueChange={switchProject}>
        <Select.Trigger class="w-[180px]" aria-label="Select project">
          {project.current}
        </Select.Trigger>
        <Select.Content>
          {#each project.available as slug (slug)}
            <Select.Item value={slug} label={slug}>{slug}</Select.Item>
          {/each}
        </Select.Content>
      </Select.Root>
    {:else if project.current}
      <span class="text-sm text-muted-foreground">{project.current}</span>
    {/if}
    <ModeToggle />
    <span data-testid="brand" class="text-sm font-semibold text-muted-foreground">SpecGraph</span>
  </nav>

  <main class="mx-auto max-w-[1400px] p-6">
    {#if project.available.length === 0}
      <div class="py-16 text-center">
        <h2 class="text-lg font-semibold text-foreground">No projects found</h2>
        <p class="mt-2 text-sm text-muted-foreground">
          Create a project with the SpecGraph CLI or authoring flow, then reload this page.
        </p>
      </div>
    {:else}
      {#if showBreadcrumb}
        <Breadcrumb.Root class="mb-5">
          <Breadcrumb.List>
            <Breadcrumb.Item>
              <span class="text-foreground font-semibold">{project.current}</span>
            </Breadcrumb.Item>
            {#if view}
              <Breadcrumb.Separator />
              <Breadcrumb.Item>
                <Breadcrumb.Page class="text-muted-foreground">{view}</Breadcrumb.Page>
              </Breadcrumb.Item>
            {/if}
          </Breadcrumb.List>
        </Breadcrumb.Root>
      {/if}
      {@render children()}
    {/if}
  </main>
{/if}
