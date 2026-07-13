import { test, expect } from './fixtures';

test.describe('Navigation', () => {
  test('loads the dashboard at /', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByTestId('primary-nav')).toBeVisible();
    await expect(page.getByTestId('primary-nav')).toContainText('Dashboard');
    await expect(page.getByTestId('primary-nav')).toContainText('Graph');
    await expect(page.getByTestId('brand')).toContainText('SpecGraph');
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
