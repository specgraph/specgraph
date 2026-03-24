import { test, expect } from '@playwright/test';

test.describe('Navigation', () => {
  test('loads the dashboard at /', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.locator('nav')).toContainText('Dashboard');
    await expect(page.locator('nav')).toContainText('Graph');
    await expect(page.locator('.brand')).toContainText('SpecGraph');
  });

  test('navigates from dashboard to graph view', async ({ page }) => {
    await page.goto('/');
    await page.click('nav a[href="/graph"]');
    await expect(page).toHaveURL('/graph');
    await expect(page.locator('h1')).toContainText('Dependency Graph');
  });

  test('deep links work for /graph', async ({ page }) => {
    await page.goto('/graph');
    await expect(page.locator('h1')).toContainText('Dependency Graph');
  });
});
