interface Identity {
  subject: string;
  displayName: string;
  role: string;
}

let authenticated = $state(false);
let identity = $state<Identity | null>(null);

export const auth = {
  get authenticated() { return authenticated; },
  get identity() { return identity; },
};

export async function checkAuth(): Promise<void> {
  try {
    const resp = await fetch('/api/auth/whoami');
    if (resp.ok) {
      const data = await resp.json();
      identity = data.identity;
      authenticated = true;
    } else if (resp.status === 401) {
      identity = null;
      authenticated = false;
    }
    // Non-401 errors (5xx, etc.) leave state unchanged — a transient server
    // error shouldn't force the user back to the login screen.
  } catch {
    // Network errors also leave state unchanged.
  }
}

export async function login(key: string): Promise<boolean> {
  const resp = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key }),
  });
  if (resp.ok) {
    const data = await resp.json();
    identity = data.identity;
    authenticated = true;
    return true;
  }
  if (resp.status === 401) {
    return false;
  }
  throw new Error(`login failed: ${resp.status}`);
}

export async function logout(): Promise<void> {
  await fetch('/api/auth/logout', { method: 'POST' });
  identity = null;
  authenticated = false;
}

export function onUnauthenticated(): void {
  identity = null;
  authenticated = false;
}
