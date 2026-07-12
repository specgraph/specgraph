import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { project, loadProjects } from './project.svelte';

// project.svelte.ts holds `current` / `available` / `loaded` in MODULE-LEVEL
// `$state` that persists across cases in this file, and the `specgraph-project`
// localStorage entry likewise leaks between cases. Reset both before each test so
// every D-04 precedence tier is exercised from a clean slate and the suite is
// order-independent (review Round-2 LOW #5).
function makeStorage(): Storage {
  const map = new Map<string, string>();
  return {
    getItem: (k: string) => (map.has(k) ? (map.get(k) as string) : null),
    setItem: (k: string, v: string) => void map.set(k, String(v)),
    removeItem: (k: string) => void map.delete(k),
    clear: () => map.clear(),
    key: (i: number) => Array.from(map.keys())[i] ?? null,
    get length() {
      return map.size;
    },
  } as Storage;
}

function mockProjects(list: string[]): void {
  globalThis.fetch = vi.fn(async () => ({
    ok: true,
    json: async () => ({ projects: list }),
  })) as unknown as typeof fetch;
}

beforeEach(() => {
  // Clear the localStorage shim, then reset the module-level rune. Assigning
  // through the setter also writes '' into the fresh shim, leaving no stale slug.
  (globalThis as { localStorage?: Storage }).localStorage = makeStorage();
  project.current = '';
});

afterEach(() => {
  delete (globalThis as { localStorage?: Storage }).localStorage;
  delete (globalThis as { fetch?: unknown }).fetch;
});

describe('loadProjects', () => {
  it('D-05: sorts the project list case-insensitively', async () => {
    mockProjects(['Zebra', 'alpha', 'Beta']);
    await loadProjects();
    expect(project.available).toEqual(['alpha', 'Beta', 'Zebra']);
  });

  it('D-04 tier 1: keeps a valid saved project present in the list', async () => {
    project.current = 'Beta'; // saved slug that is in the returned list
    mockProjects(['Zebra', 'alpha', 'Beta']);
    await loadProjects();
    expect(project.current).toBe('Beta');
  });

  it("D-04 tier 2: with no valid saved project, prefers a project named 'default'", async () => {
    mockProjects(['zebra', 'default', 'alpha']);
    await loadProjects();
    expect(project.current).toBe('default');
  });

  it('D-04 tier 3: with no saved and no default, picks the alphabetically-first project', async () => {
    mockProjects(['zebra', 'alpha', 'Beta']);
    await loadProjects();
    expect(project.current).toBe('alpha');
  });

  it('D-06: a saved project absent from the list falls back per precedence', async () => {
    project.current = 'ghost'; // no longer present in the returned list
    mockProjects(['zebra', 'alpha', 'Beta']);
    await loadProjects();
    expect(project.current).toBe('alpha');
    expect(project.available).not.toContain('ghost');
  });

  it('D-07 seam: an empty project list leaves current empty', async () => {
    mockProjects([]);
    await loadProjects();
    expect(project.current).toBe('');
    expect(project.available).toEqual([]);
  });
});
