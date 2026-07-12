import { describe, it, expect, vi, beforeEach } from 'vitest';

// Load-seam unit test for the constitution universal load() (review Round-2
// MEDIUM #3). This is a plain load-FUNCTION test — NO jsdom, NO Svelte component
// harness, NO browser runner (the component harness stays out of appetite per
// VALIDATION). It pins the load() contract structurally, beyond the `node -e`
// string greps:
//   - depends('app:project') registration — the dependency invalidateAll() /
//     invalidate('app:project') re-runs after a project switch (D-01/D-02).
//   - response → { constitution, provenance } mapping (D-10).
//   - empty seam → { constitution: null, provenance: [] } (D-10).
//   - RPC rejection → a `loadError` field, never a throw to +error.svelte
//     (RESEARCH L279, T-05-15).
// It does NOT assert framework re-run behavior (that is SvelteKit's contract,
// guaranteed once depends('app:project') is registered and 05-03's switchProject
// calls invalidateAll()).

// vi.hoisted so the stub exists before the hoisted vi.mock factory runs.
const { getConstitution } = vi.hoisted(() => ({ getConstitution: vi.fn() }));

vi.mock('$lib/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/api/client')>();
  return {
    ...actual,
    constitutionClient: { getConstitution },
  };
});

import { load } from '../routes/constitution/+page';

// A hand-rolled fake SvelteKit load event — only the three members the load
// touches (parent/depends/fetch). Cast `as any` since we do not need the full
// LoadEvent surface for a seam test.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function fakeEvent(): { event: any; depends: ReturnType<typeof vi.fn> } {
  const depends = vi.fn();
  const event = {
    parent: vi.fn().mockResolvedValue({}),
    depends,
    fetch: vi.fn(),
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;
  return { event, depends };
}

beforeEach(() => {
  getConstitution.mockReset();
});

describe('constitution load()', () => {
  it("registers depends('app:project') exactly once (the D-01/D-02 switch mechanism)", async () => {
    getConstitution.mockResolvedValue({ constitution: { name: 'C' }, provenance: [] });
    const { event, depends } = fakeEvent();

    await load(event);

    expect(depends).toHaveBeenCalledWith('app:project');
    expect(depends).toHaveBeenCalledTimes(1);
  });

  it('maps a resolved response to { constitution, provenance } with no loadError (D-10)', async () => {
    const constitution = { name: 'Project Constitution', layer: 3, version: '1' };
    const provenance = [
      { path: 'principles[P1]', layer: 1 },
      { path: 'constraints[C1]', layer: 3 },
    ];
    getConstitution.mockResolvedValue({ constitution, provenance });
    const { event } = fakeEvent();

    const result = await load(event);

    expect(await result.constitution).toEqual(constitution);
    expect(await result.provenance).toEqual(provenance);
    expect(await result.loadError).toBeNull();
  });

  it('maps an empty constitution to { constitution: null, provenance: [] } without throwing (D-10 empty seam)', async () => {
    getConstitution.mockResolvedValue({ constitution: undefined, provenance: undefined });
    const { event } = fakeEvent();

    const result = await load(event);

    expect(await result.constitution).toBeNull();
    expect(await result.provenance).toEqual([]);
    expect(await result.loadError).toBeNull();
  });

  it('returns a loadError (does NOT throw to +error.svelte) when the RPC rejects (RESEARCH L279)', async () => {
    getConstitution.mockRejectedValue(new Error('connection refused'));
    const { event } = fakeEvent();

    // load() itself must resolve (the error is caught inside the streamed promise).
    const result = await load(event);

    expect(await result.loadError).toBe('connection refused');
    expect(await result.constitution).toBeNull();
    expect(await result.provenance).toEqual([]);
  });
});
