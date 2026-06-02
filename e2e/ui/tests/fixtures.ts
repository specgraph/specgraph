import { test as base } from '@playwright/test';

const E2E_API_KEY = 'spgr_sk_e2eadmin_e2esecret32charsfixedpaddingaaa0'; // matches helpers.ts + e2e/ui/seed.sql

// Extend the base test to log in via the UI before each test.
export const test = base.extend({
  page: async ({ page }, use) => {
    // Navigate to trigger the login modal.
    await page.goto('/');
    // Fill the API key via accessibility locator and submit.
    await page.getByPlaceholder('spgr_sk_...').fill(E2E_API_KEY);
    await page.getByRole('button', { name: 'Sign in' }).click();
    // Wait for the nav to appear (login succeeded, dashboard loaded).
    await page.waitForSelector('nav', { timeout: 30_000 });
    await use(page);
  },
});

export { expect, request } from '@playwright/test';
