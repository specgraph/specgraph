import { test, expect, request as playwrightRequest } from './fixtures';
import { seedSpec, seedEdge } from './helpers';

test.describe('Graph View', () => {
  test.beforeAll(async () => {
    const request = await playwrightRequest.newContext();
    await seedSpec(request, 'ui-parent', 'Parent feature');
    await seedSpec(request, 'ui-child', 'Child feature');
    await seedEdge(request, 'ui-child', 'ui-parent');
    await request.dispose();
  });

  test('renders SVG graph with nodes', async ({ page }) => {
    await page.goto('/graph');
    const svg = page.locator('svg.graph-svg');
    await expect(svg).toBeVisible({ timeout: 10_000 });
    const nodes = page.locator('svg.graph-svg .graph-node');
    await expect(nodes).not.toHaveCount(0);
  });

  test('search filter fades non-matching nodes', async ({ page }) => {
    await page.goto('/graph');
    await expect(page.locator('svg.graph-svg')).toBeVisible({ timeout: 10_000 });
    await page.fill('input[placeholder*="Filter"]', 'ui-parent');
    const fadedNodes = page.locator('svg.graph-svg .graph-node[opacity="0.2"]');
    await expect(fadedNodes).not.toHaveCount(0);
  });

  test('clicking a node navigates to detail page', async ({ page }) => {
    await page.goto('/graph');
    await expect(page.locator('svg.graph-svg')).toBeVisible({ timeout: 10_000 });
    const nodeLink = page.locator('svg.graph-svg a[href*="/spec/"]').first();
    await nodeLink.click();
    await expect(page).toHaveURL(/\/spec\//);
  });
});
