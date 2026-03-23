import { test, expect, request as playwrightRequest } from '@playwright/test';
import { seedSpec, seedDecision } from './helpers';

test.describe('Detail Pages', () => {
  test.beforeAll(async () => {
    const request = await playwrightRequest.newContext();
    await seedSpec(request, 'detail-spec', 'Detail test spec', 'p1');
    await seedDecision(request, 'detail-dec', 'Detail test decision');
    await request.dispose();
  });

  test('spec detail page shows metadata', async ({ page }) => {
    await page.goto('/spec/detail-spec');
    await expect(page.locator('h1')).toContainText('detail-spec');
    await expect(page.locator('.meta')).toContainText('Detail test spec');
    await expect(page.locator('.meta')).toContainText('p1');
    await expect(page.locator('.breadcrumb')).toContainText('Dashboard');
    await expect(page.locator('.breadcrumb')).toContainText('Graph');
  });

  test('decision detail page shows metadata', async ({ page }) => {
    await page.goto('/decision/detail-dec');
    await expect(page.locator('h1')).toContainText('detail-dec');
    await expect(page.locator('.meta')).toContainText('Detail test decision');
  });

  test('spec detail 404 shows error', async ({ page }) => {
    await page.goto('/spec/nonexistent-slug-xyz');
    await expect(page.locator('.error')).toBeVisible({ timeout: 10_000 });
  });
});
