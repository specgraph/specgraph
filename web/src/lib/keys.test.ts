import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Partial-mock the client module: keep the real csrfInterceptor / readCsrfToken
// (unit under test for the CSRF echo) while replacing identityClient's four self
// RPCs with vi.fn stubs so the state module can be exercised without a transport.
// vi.hoisted lets these stubs exist before the hoisted vi.mock factory runs.
const { listMyAPIKeys, createMyAPIKey, rotateMyAPIKey, revokeMyAPIKey } = vi.hoisted(() => ({
  listMyAPIKeys: vi.fn(),
  createMyAPIKey: vi.fn(),
  rotateMyAPIKey: vi.fn(),
  revokeMyAPIKey: vi.fn(),
}));

vi.mock('$lib/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/api/client')>();
  return {
    ...actual,
    identityClient: { listMyAPIKeys, createMyAPIKey, rotateMyAPIKey, revokeMyAPIKey },
  };
});

import { csrfInterceptor } from '$lib/api/client';
import { keys, listKeys, createKey, rotateKey, revokeKey } from '$lib/keys.svelte';

beforeEach(() => {
  listMyAPIKeys.mockReset();
  createMyAPIKey.mockReset();
  rotateMyAPIKey.mockReset();
  revokeMyAPIKey.mockReset();
});

afterEach(() => {
  // Remove the injected document double so tests stay isolated.
  delete (globalThis as { document?: unknown }).document;
});

describe('csrfInterceptor', () => {
  it('echoes the specgraph_csrf cookie value into X-CSRF-Token (double-submit)', async () => {
    (globalThis as { document?: unknown }).document = {
      cookie: 'foo=bar; specgraph_csrf=tok-123; baz=qux',
    };
    const req = { header: new Headers() } as unknown as { header: Headers };
    const next = vi.fn(async (r: unknown) => r);
    await csrfInterceptor(next as never)(req as never);
    expect(req.header.get('X-CSRF-Token')).toBe('tok-123');
    expect(next).toHaveBeenCalledOnce();
  });

  it('sets no X-CSRF-Token header when the cookie is absent', async () => {
    (globalThis as { document?: unknown }).document = { cookie: 'foo=bar' };
    const req = { header: new Headers() } as unknown as { header: Headers };
    await csrfInterceptor((async (r: unknown) => r) as never)(req as never);
    expect(req.header.get('X-CSRF-Token')).toBeNull();
  });
});

describe('keys state module', () => {
  it('listKeys populates the reactive list', async () => {
    listMyAPIKeys.mockResolvedValue({
      keys: [
        { id: 'k1', prefix: 'abc' },
        { id: 'k2', prefix: 'def' },
      ],
    });
    await listKeys();
    expect(keys.list.map((k) => k.id)).toEqual(['k1', 'k2']);
    expect(keys.error).toBeNull();
  });

  it('createKey surfaces the plaintext exactly once and never persists it in state', async () => {
    createMyAPIKey.mockResolvedValue({
      key: { id: 'k9', prefix: 'zzz' },
      plaintext: 'spgr_sk_zzz_secret',
    });
    listMyAPIKeys.mockResolvedValue({ keys: [{ id: 'k9', prefix: 'zzz' }] });
    const plaintext = await createKey({ label: 'ci' });
    expect(plaintext).toBe('spgr_sk_zzz_secret');
    // The one-time secret must never leak into the persisted key list.
    expect(JSON.stringify(keys.list)).not.toContain('spgr_sk_zzz_secret');
  });

  it('createKey sends the label + role downgrade and refreshes the list', async () => {
    createMyAPIKey.mockResolvedValue({ key: { id: 'k1' }, plaintext: 'pt' });
    listMyAPIKeys.mockResolvedValue({ keys: [{ id: 'k1' }] });
    await createKey({ label: 'ci', roleDowngrade: 'reader' });
    expect(createMyAPIKey).toHaveBeenCalledWith(
      expect.objectContaining({ label: 'ci', roleDowngrade: 'reader' }),
    );
    expect(listMyAPIKeys).toHaveBeenCalled();
  });

  it('rotateKey returns the freshly minted plaintext', async () => {
    rotateMyAPIKey.mockResolvedValue({ key: { id: 'k1' }, plaintext: 'new-pt' });
    listMyAPIKeys.mockResolvedValue({ keys: [] });
    const pt = await rotateKey('k1');
    expect(pt).toBe('new-pt');
    expect(rotateMyAPIKey).toHaveBeenCalledWith(expect.objectContaining({ keyId: 'k1' }));
  });

  it('revokeKey calls the RPC and refreshes the list', async () => {
    revokeMyAPIKey.mockResolvedValue({});
    listMyAPIKeys.mockResolvedValue({ keys: [] });
    await revokeKey('k1');
    expect(revokeMyAPIKey).toHaveBeenCalledWith(expect.objectContaining({ keyId: 'k1' }));
    expect(listMyAPIKeys).toHaveBeenCalled();
  });

  it('maps a PermissionDenied (anti key-chaining) failure to a readable message', async () => {
    const { ConnectError, Code } = await import('@connectrpc/connect');
    createMyAPIKey.mockRejectedValue(new ConnectError('source apikey blocked', Code.PermissionDenied));
    const pt = await createKey({ label: 'x' });
    expect(pt).toBeNull();
    expect(keys.error).toContain('OIDC');
    expect(keys.error).not.toContain('PermissionDenied');
  });
});
