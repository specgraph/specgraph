import { test, expect } from '@playwright/test';
import { seedSpec, seedEdge } from './helpers';

test.describe('Graph View', () => {
  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await seedSpec(page, 'ui-parent', 'Parent feature');
    await seedSpec(page, 'ui-child', 'Child feature');
    await seedEdge(page, 'ui-child', 'ui-parent');
    await page.close();
  });

  test('renders SVG graph with nodes', async ({ page }) => {
    await page.goto('/graph');
    const svg = page.locator('svg.graph');
    await expect(svg).toBeVisible({ timeout: 10_000 });
    const nodes = page.locator('svg.graph .graph-node');
    await expect(nodes).not.toHaveCount(0);
  });

  test('search filter fades non-matching nodes', async ({ page }) => {
    await page.goto('/graph');
    await expect(page.locator('svg.graph')).toBeVisible({ timeout: 10_000 });
    await page.fill('input[placeholder*="Filter"]', 'ui-parent');
    const fadedNodes = page.locator('svg.graph .graph-node[opacity="0.2"]');
    await expect(fadedNodes).not.toHaveCount(0);
  });

  test('clicking a node navigates to detail page', async ({ page }) => {
    await page.goto('/graph');
    await expect(page.locator('svg.graph')).toBeVisible({ timeout: 10_000 });
    const nodeLink = page.locator('svg.graph a[href*="/spec/"]').first();
    await nodeLink.click();
    await expect(page).toHaveURL(/\/spec\//);
  });
});
