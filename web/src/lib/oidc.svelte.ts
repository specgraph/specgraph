export interface OidcProvider {
  id: string;
  displayName: string;
}

// fetchProviders returns the configured interactive OIDC providers (empty on
// error or when none are configured).
export async function fetchProviders(): Promise<OidcProvider[]> {
  try {
    const resp = await fetch('/api/auth/oidc/providers');
    if (!resp.ok) return [];
    const data = await resp.json();
    return (data.providers ?? []).map((p: { id: string; display_name: string }) => ({
      id: p.id,
      displayName: p.display_name,
    }));
  } catch {
    return [];
  }
}

// authErrorMessage maps a backend auth_error reason token to a friendly message.
export function authErrorMessage(reason: string | null): string {
  switch (reason) {
    case 'denied': return 'Sign-in was cancelled or denied by the provider.';
    case 'unauthorized': return 'Your account is not permitted to sign in. Contact an administrator.';
    case 'expired': return 'The sign-in attempt expired. Please try again.';
    case 'state': return 'The sign-in could not be verified. Please try again.';
    case 'exchange': return 'Sign-in failed during authentication. Please try again.';
    case 'temporary': return 'The server is temporarily unavailable. Please try again shortly.';
    default: return 'Sign-in failed. Please try again.';
  }
}
