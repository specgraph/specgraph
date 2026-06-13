import { describe, it, expect } from 'vitest';
import { authErrorMessage } from './oidc.svelte';

describe('authErrorMessage', () => {
  it('maps known reasons', () => {
    expect(authErrorMessage('denied')).toContain('cancelled');
    expect(authErrorMessage('unauthorized')).toContain('not permitted');
    expect(authErrorMessage('temporary')).toContain('temporarily');
  });
  it('falls back for unknown reasons', () => {
    expect(authErrorMessage('weird')).toBe('Sign-in failed. Please try again.');
    expect(authErrorMessage(null)).toBe('Sign-in failed. Please try again.');
  });
});
