import { identityClient } from '$lib/api/client';
import type { APIKey } from '$lib/api/gen/specgraph/v1/identity_pb';
import { ConnectError, Code } from '@connectrpc/connect';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

// Reactive state for the caller's own API keys. The one-time plaintext secret is
// intentionally NOT held here — create/rotate return it to the caller (the reveal
// modal) and it is never stored, re-fetched, or serialized into `list`.
let list = $state<APIKey[]>([]);
let loading = $state(false);
let error = $state<string | null>(null);

export const keys = {
  get list(): APIKey[] {
    return list;
  },
  get loading(): boolean {
    return loading;
  },
  get error(): string | null {
    return error;
  },
};

// friendlyError turns a Connect error into a human-readable message. The
// anti-key-chaining gate (Plan 05) rejects self-mint from a raw-API-key session
// (Source=="apikey") with PermissionDenied — surface that as guidance rather than
// a raw code (cursor #4a).
function friendlyError(err: unknown): string {
  if (err instanceof ConnectError) {
    if (err.code === Code.PermissionDenied) {
      return 'Self-minting keys requires an OIDC or workspace (spgr_ws_) session. This dashboard session appears to be authenticated with a raw API key, which cannot mint further keys (anti key-chaining). Re-authenticate via your OIDC provider to manage keys.';
    }
    return err.message;
  }
  return err instanceof Error ? err.message : 'Request failed';
}

export interface CreateKeyInput {
  label: string;
  roleDowngrade?: string;
  expiresAt?: Date;
}

// listKeys loads the caller's keys into reactive state.
export async function listKeys(): Promise<void> {
  loading = true;
  error = null;
  try {
    const resp = await identityClient.listMyAPIKeys({});
    list = resp.keys ?? [];
  } catch (err) {
    error = friendlyError(err);
  } finally {
    loading = false;
  }
}

// createKey mints a new key and returns its one-time plaintext (or null on
// error). The list is refreshed so metadata (prefix/expiry) appears immediately;
// the plaintext is returned to the caller and never stored.
export async function createKey(input: CreateKeyInput): Promise<string | null> {
  error = null;
  try {
    const resp = await identityClient.createMyAPIKey({
      label: input.label,
      roleDowngrade: input.roleDowngrade ?? '',
      expiresAt: input.expiresAt ? timestampFromDate(input.expiresAt) : undefined,
    });
    await listKeys();
    return resp.plaintext;
  } catch (err) {
    error = friendlyError(err);
    return null;
  }
}

// rotateKey mints a fresh secret for an existing key and returns the new one-time
// plaintext (or null on error). Optional expiresAt chooses the new validity
// window; omitted inherits the old key's expiry (server-side).
export async function rotateKey(keyId: string, expiresAt?: Date): Promise<string | null> {
  error = null;
  try {
    const resp = await identityClient.rotateMyAPIKey({
      keyId,
      expiresAt: expiresAt ? timestampFromDate(expiresAt) : undefined,
    });
    await listKeys();
    return resp.plaintext;
  } catch (err) {
    error = friendlyError(err);
    return null;
  }
}

// revokeKey revokes one of the caller's keys and refreshes the list.
export async function revokeKey(keyId: string): Promise<boolean> {
  error = null;
  try {
    await identityClient.revokeMyAPIKey({ keyId });
    await listKeys();
    return true;
  } catch (err) {
    error = friendlyError(err);
    return false;
  }
}
